package dpagent

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type ReportOptions struct {
	ConfigPath  string
	OutputPath  string
	Timeout     time.Duration
	SkipNetwork bool
	Version     string
}

type ReportResult struct {
	OutputPath string `json:"output_path"`
	Bytes      int64  `json:"bytes"`
}

type reportMeta struct {
	GeneratedAt  string `json:"generated_at"`
	Version      string `json:"version"`
	Platform     string `json:"platform"`
	ConfigPath   string `json:"config_path"`
	ConfigLoaded bool   `json:"config_loaded"`
}

type reportCheck struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

func (c CLI) runReport(args []string) int {
	fs := flag.NewFlagSet("report", flag.ContinueOnError)
	fs.SetOutput(c.Err)
	configPath := fs.String("config", "", "config path")
	outputPath := fs.String("output", "", "output zip path")
	timeout := fs.Duration("timeout", 5*time.Second, "network check timeout")
	skipNetwork := fs.Bool("skip-network", false, "skip DNS and base URL checks")
	jsonOutput := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	result, err := WriteDiagnosticReport(ReportOptions{
		ConfigPath:  *configPath,
		OutputPath:  *outputPath,
		Timeout:     *timeout,
		SkipNetwork: *skipNetwork,
		Version:     c.Version,
	})
	if err != nil {
		fmt.Fprintln(c.Err, err)
		return 1
	}
	if *jsonOutput {
		bytes, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			fmt.Fprintln(c.Err, err)
			return 1
		}
		fmt.Fprintln(c.Out, string(bytes))
		return 0
	}
	fmt.Fprintf(c.Out, "diagnostic report saved: %s\n", result.OutputPath)
	return 0
}

func WriteDiagnosticReport(opts ReportOptions) (ReportResult, error) {
	cfg, loaded, err := LoadConfig(opts.ConfigPath)
	if err != nil {
		return ReportResult{}, err
	}
	configPath := opts.ConfigPath
	if strings.TrimSpace(configPath) == "" {
		configPath, err = ConfigPath()
		if err != nil {
			return ReportResult{}, err
		}
	}
	outputPath := strings.TrimSpace(opts.OutputPath)
	if outputPath == "" {
		outputPath = defaultReportPath()
	}
	outputPath = expandPath(outputPath)
	if err := os.MkdirAll(filepath.Dir(outputPath), DefaultConfigFolderMode); err != nil {
		return ReportResult{}, err
	}
	file, err := os.OpenFile(outputPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, DefaultConfigFileMode)
	if err != nil {
		return ReportResult{}, err
	}
	defer file.Close()

	archive := zip.NewWriter(file)
	if err := writeReportEntries(archive, cfg, loaded, configPath, opts); err != nil {
		_ = archive.Close()
		return ReportResult{}, err
	}
	if err := archive.Close(); err != nil {
		return ReportResult{}, err
	}
	info, err := file.Stat()
	if err != nil {
		return ReportResult{}, err
	}
	return ReportResult{OutputPath: outputPath, Bytes: info.Size()}, nil
}

func writeReportEntries(archive *zip.Writer, cfg Config, loaded bool, configPath string, opts ReportOptions) error {
	version := strings.TrimSpace(opts.Version)
	if version == "" {
		version = DefaultAgentVersion
	}
	meta := reportMeta{
		GeneratedAt:  time.Now().UTC().Format(time.RFC3339),
		Version:      version,
		Platform:     agentPlatform(),
		ConfigPath:   configPath,
		ConfigLoaded: loaded,
	}
	if err := writeZipJSON(archive, "meta.json", meta); err != nil {
		return err
	}
	redacted := RedactedConfig(cfg)
	configYAML, err := yaml.Marshal(redacted)
	if err != nil {
		return err
	}
	if err := writeZipBytes(archive, "config.redacted.yaml", configYAML); err != nil {
		return err
	}
	if err := writeZipJSON(archive, "validation.json", ValidateConfig(cfg, false)); err != nil {
		return err
	}
	if err := writeZipBytes(archive, "status.txt", []byte(renderStatusText(cfg, loaded))); err != nil {
		return err
	}
	if err := writeZipJSON(archive, "checks.json", buildReportChecks(cfg, opts)); err != nil {
		return err
	}
	return nil
}

func renderStatusText(cfg Config, loaded bool) string {
	var builder strings.Builder
	bridgeURL, _ := EffectiveBridgeWSURL(cfg)
	fmt.Fprintf(&builder, "config_loaded: %t\n", loaded)
	fmt.Fprintf(&builder, "client_id: %s\n", cfg.Agent.ClientID)
	fmt.Fprintf(&builder, "name: %s\n", cfg.Agent.Name)
	fmt.Fprintf(&builder, "version: %s\n", cfg.Agent.Version)
	fmt.Fprintf(&builder, "workspace: %s\n", cfg.Agent.Workspace)
	fmt.Fprintf(&builder, "bridge_ws_url: %s\n", bridgeURL)
	fmt.Fprintf(&builder, "token_configured: %t\n", ResolveToken(cfg) != "")
	fmt.Fprintf(&builder, "capabilities: %s\n", strings.Join(EffectiveCapabilities(cfg), ","))
	fmt.Fprintf(&builder, "mcp_servers: %d\n", len(cfg.MCPServers))
	fmt.Fprintf(&builder, "http_routes: %d\n", len(cfg.HTTPRoutes))
	return builder.String()
}

func buildReportChecks(cfg Config, opts ReportOptions) []reportCheck {
	checks := []reportCheck{}
	validation := ValidateConfig(cfg, false)
	if validation.OK() {
		checks = append(checks, reportCheck{Name: "config", Status: "ok"})
	} else {
		checks = append(checks, reportCheck{Name: "config", Status: "fail", Message: validation.Error().Error()})
	}
	if ResolveToken(cfg) == "" {
		checks = append(checks, reportCheck{Name: "token", Status: "missing"})
	} else {
		checks = append(checks, reportCheck{Name: "token", Status: "configured"})
	}
	if opts.SkipNetwork {
		checks = append(checks, reportCheck{Name: "network", Status: "skipped"})
		return checks
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	if bridgeURL, err := EffectiveBridgeWSURL(cfg); err != nil {
		checks = append(checks, reportCheck{Name: "bridge_url", Status: "fail", Message: err.Error()})
	} else if err := checkDNS(bridgeURL, timeout); err != nil {
		checks = append(checks, reportCheck{Name: "dns", Status: "fail", Message: err.Error()})
	} else {
		checks = append(checks, reportCheck{Name: "dns", Status: "ok"})
	}
	if strings.TrimSpace(cfg.Server.BaseURL) != "" {
		if err := checkBaseURL(cfg.Server.BaseURL, timeout); err != nil {
			checks = append(checks, reportCheck{Name: "base_url", Status: "warn", Message: err.Error()})
		} else {
			checks = append(checks, reportCheck{Name: "base_url", Status: "ok"})
		}
	}
	return checks
}

func defaultReportPath() string {
	name := "data-proxy-agent-diagnostic-" + time.Now().Format("20060102-150405") + ".zip"
	if runtime.GOOS == "windows" {
		return filepath.Join(os.TempDir(), name)
	}
	if cwd, err := os.Getwd(); err == nil && cwd != "" {
		return filepath.Join(cwd, name)
	}
	return filepath.Join(os.TempDir(), name)
}

func writeZipJSON(archive *zip.Writer, name string, value any) error {
	bytes, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	bytes = append(bytes, '\n')
	return writeZipBytes(archive, name, bytes)
}

func writeZipBytes(archive *zip.Writer, name string, data []byte) error {
	header := &zip.FileHeader{Name: name, Method: zip.Deflate}
	header.SetMode(0o600)
	writer, err := archive.CreateHeader(header)
	if err != nil {
		return err
	}
	_, err = io.Copy(writer, bytes.NewReader(data))
	return err
}
