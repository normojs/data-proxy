package dpagent

import (
	"archive/zip"
	"bytes"
	"context"
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
	ConfigPath        string
	OutputPath        string
	Timeout           time.Duration
	SkipNetwork       bool
	CheckUpdate       bool
	UpdateManifestURL string
	UpdateRepo        string
	UpdateGitHubAPI   string
	AllowPrerelease   bool
	Version           string
}

type ReportResult struct {
	OutputPath string `json:"output_path"`
	Bytes      int64  `json:"bytes"`
}

type AgentStatusSummary struct {
	ConfigLoaded    bool                `json:"config_loaded"`
	ClientID        string              `json:"client_id"`
	Name            string              `json:"name"`
	Version         string              `json:"version"`
	Workspace       string              `json:"workspace"`
	BridgeWSURL     string              `json:"bridge_ws_url"`
	TokenConfigured bool                `json:"token_configured"`
	TokenSource     string              `json:"token_source,omitempty"`
	Capabilities    []string            `json:"capabilities"`
	MCPServers      int                 `json:"mcp_servers"`
	HTTPRoutes      int                 `json:"http_routes"`
	TCPRoutes       int                 `json:"tcp_routes"`
	Routes          []AgentRouteSummary `json:"routes,omitempty"`
	LocalHealth     []AgentHealthCheck  `json:"local_health,omitempty"`
}

type AgentRouteSummary struct {
	Name           string `json:"name"`
	Type           string `json:"type"`
	Target         string `json:"target,omitempty"`
	Transport      string `json:"transport,omitempty"`
	AllowWebSocket bool   `json:"allow_websocket"`
	AllowSSE       bool   `json:"allow_sse"`
	AllowPublic    bool   `json:"allow_public"`
}

type AgentDoctorReport struct {
	OK           bool               `json:"ok"`
	Version      string             `json:"version"`
	ConfigLoaded bool               `json:"config_loaded"`
	Validation   ValidationResult   `json:"validation"`
	Checks       []AgentDoctorCheck `json:"checks"`
}

type AgentDoctorCheck struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
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
	checkUpdate := fs.Bool("check-update", false, "check latest dpa release metadata")
	updateManifestURL := fs.String("manifest-url", "", "custom update manifest URL for --check-update")
	updateRepo := fs.String("repo", DefaultAgentUpdateRepo, "GitHub repo in owner/name form for --check-update")
	updateGitHubAPI := fs.String("github-api", DefaultAgentUpdateGitHubAPI, "GitHub API base URL for --check-update")
	allowPrerelease := fs.Bool("allow-prerelease", false, "allow prerelease versions for --check-update")
	jsonOutput := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	result, err := WriteDiagnosticReport(ReportOptions{
		ConfigPath:        *configPath,
		OutputPath:        *outputPath,
		Timeout:           *timeout,
		SkipNetwork:       *skipNetwork,
		CheckUpdate:       *checkUpdate,
		UpdateManifestURL: *updateManifestURL,
		UpdateRepo:        *updateRepo,
		UpdateGitHubAPI:   *updateGitHubAPI,
		AllowPrerelease:   *allowPrerelease,
		Version:           c.Version,
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
	if err := writeZipJSON(archive, "status.json", BuildAgentStatusSummary(cfg, loaded)); err != nil {
		return err
	}
	if err := writeZipJSON(archive, "local_health.json", AgentLocalHealthChecks(cfg, reportTimeout(opts))); err != nil {
		return err
	}
	if err := writeZipJSON(archive, "checks.json", buildReportChecks(cfg, opts)); err != nil {
		return err
	}
	return nil
}

func renderStatusText(cfg Config, loaded bool) string {
	return RenderAgentStatusText(BuildAgentStatusSummary(cfg, loaded))
}

func BuildAgentStatusSummary(cfg Config, loaded bool) AgentStatusSummary {
	return BuildAgentStatusSummaryWithHealth(cfg, loaded, false, 0)
}

func BuildAgentStatusSummaryWithHealth(cfg Config, loaded bool, includeHealth bool, timeout time.Duration) AgentStatusSummary {
	fillConfigDefaults(&cfg)
	bridgeURL, _ := EffectiveBridgeWSURL(cfg)
	token, tokenSource := ResolveTokenWithSource(cfg)
	summary := AgentStatusSummary{
		ConfigLoaded:    loaded,
		ClientID:        cfg.Agent.ClientID,
		Name:            cfg.Agent.Name,
		Version:         agentVersion(cfg),
		Workspace:       cfg.Agent.Workspace,
		BridgeWSURL:     bridgeURL,
		TokenConfigured: strings.TrimSpace(token) != "",
		TokenSource:     tokenSource,
		Capabilities:    EffectiveCapabilities(cfg),
		MCPServers:      len(cfg.MCPServers),
		HTTPRoutes:      len(cfg.HTTPRoutes),
		TCPRoutes:       len(cfg.TCPRoutes),
		Routes:          BuildAgentRouteSummaries(cfg),
	}
	if includeHealth {
		summary.LocalHealth = AgentLocalHealthChecks(cfg, timeout)
	}
	return summary
}

func BuildAgentRouteSummaries(cfg Config) []AgentRouteSummary {
	routes := make([]AgentRouteSummary, 0, len(cfg.HTTPRoutes)+len(cfg.TCPRoutes)+len(cfg.MCPServers))
	for _, route := range cfg.HTTPRoutes {
		routes = append(routes, AgentRouteSummary{
			Name:           route.Name,
			Type:           "http",
			Target:         route.Target,
			AllowWebSocket: route.AllowWebSocket,
			AllowSSE:       route.AllowSSE,
		})
	}
	for _, route := range cfg.TCPRoutes {
		routes = append(routes, AgentRouteSummary{
			Name:        route.Name,
			Type:        "tcp",
			Target:      fmt.Sprintf("%s:%d", route.TargetHost, route.TargetPort),
			AllowPublic: route.AllowPublic,
		})
	}
	for _, server := range cfg.MCPServers {
		transport := normalizeMCPTransport(server.Transport, server.Endpoint, server.Command)
		target := strings.TrimSpace(server.Endpoint)
		if transport == "stdio" {
			target = "stdio:configured"
			if prefix, ok := stdioCommandPrefix(server.Command); ok {
				target = "stdio:" + prefix
			}
		}
		routes = append(routes, AgentRouteSummary{
			Name:      server.Name,
			Type:      "mcp",
			Target:    target,
			Transport: transport,
		})
	}
	return routes
}

func RenderAgentStatusText(summary AgentStatusSummary) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "config_loaded: %t\n", summary.ConfigLoaded)
	fmt.Fprintf(&builder, "client_id: %s\n", summary.ClientID)
	fmt.Fprintf(&builder, "name: %s\n", summary.Name)
	fmt.Fprintf(&builder, "version: %s\n", summary.Version)
	fmt.Fprintf(&builder, "workspace: %s\n", summary.Workspace)
	fmt.Fprintf(&builder, "bridge_ws_url: %s\n", summary.BridgeWSURL)
	fmt.Fprintf(&builder, "token_configured: %t\n", summary.TokenConfigured)
	if summary.TokenSource != "" {
		fmt.Fprintf(&builder, "token_source: %s\n", summary.TokenSource)
	}
	fmt.Fprintf(&builder, "capabilities: %s\n", strings.Join(summary.Capabilities, ","))
	fmt.Fprintf(&builder, "mcp_servers: %d\n", summary.MCPServers)
	fmt.Fprintf(&builder, "http_routes: %d\n", summary.HTTPRoutes)
	fmt.Fprintf(&builder, "tcp_routes: %d\n", summary.TCPRoutes)
	for _, route := range summary.Routes {
		detail := strings.TrimSpace(route.Target)
		if route.Transport != "" {
			detail = strings.TrimSpace(route.Transport + " " + detail)
		}
		if detail == "" {
			fmt.Fprintf(&builder, "route: %s %s\n", route.Type, route.Name)
		} else {
			fmt.Fprintf(&builder, "route: %s %s %s\n", route.Type, route.Name, detail)
		}
	}
	for _, check := range summary.LocalHealth {
		if strings.TrimSpace(check.Detail) == "" {
			fmt.Fprintf(&builder, "health: %s %s\n", check.Name, check.Status)
			continue
		}
		fmt.Fprintf(&builder, "health: %s %s %s\n", check.Name, check.Status, check.Detail)
	}
	return builder.String()
}

func (report *AgentDoctorReport) addCheck(name string, status string, message string) {
	if report == nil {
		return
	}
	status = strings.TrimSpace(status)
	if status == "" {
		status = HealthStatusOK
	}
	report.Checks = append(report.Checks, AgentDoctorCheck{
		Name:    strings.TrimSpace(name),
		Status:  status,
		Message: strings.TrimSpace(message),
	})
	switch status {
	case HealthStatusFail, "missing":
		report.OK = false
	}
}

func (report *AgentDoctorReport) addHealthCheck(check AgentHealthCheck) {
	if report == nil {
		return
	}
	report.addCheck(check.Name, check.Status, check.Detail)
}

func buildReportChecks(cfg Config, opts ReportOptions) []reportCheck {
	checks := []reportCheck{}
	validation := ValidateConfig(cfg, false)
	if validation.OK() {
		checks = append(checks, reportCheck{Name: "config", Status: "ok"})
	} else {
		checks = append(checks, reportCheck{Name: "config", Status: "fail", Message: validation.Error().Error()})
	}
	token := ResolveToken(cfg)
	if token == "" {
		checks = append(checks, reportCheck{Name: "token", Status: "missing"})
	} else {
		checks = append(checks, reportCheck{Name: "token", Status: "configured"})
	}
	if opts.SkipNetwork {
		checks = append(checks, reportCheck{Name: "network", Status: "skipped"})
	} else {
		bridgeURL := ""
		bridgeURLValid := false
		if resolvedBridgeURL, err := EffectiveBridgeWSURL(cfg); err != nil {
			checks = append(checks, reportCheck{Name: "bridge_url", Status: "fail", Message: err.Error()})
		} else if err := checkDNS(resolvedBridgeURL, reportTimeout(opts)); err != nil {
			bridgeURL = resolvedBridgeURL
			bridgeURLValid = true
			checks = append(checks, reportCheck{Name: "dns", Status: "fail", Message: err.Error()})
		} else {
			bridgeURL = resolvedBridgeURL
			bridgeURLValid = true
			checks = append(checks, reportCheck{Name: "dns", Status: "ok"})
		}
		if token == "" {
			checks = append(checks, reportCheck{Name: "bridge_auth", Status: "skipped", Message: "token missing"})
		} else if bridgeURLValid {
			if err := checkBridgeWebSocketAuth(bridgeURL, token, reportTimeout(opts)); err != nil {
				checks = append(checks, reportCheck{Name: "bridge_auth", Status: "fail", Message: err.Error()})
			} else {
				checks = append(checks, reportCheck{Name: "bridge_auth", Status: "ok"})
			}
		}
		if strings.TrimSpace(cfg.Server.BaseURL) != "" {
			if err := checkBaseURL(cfg.Server.BaseURL, reportTimeout(opts)); err != nil {
				checks = append(checks, reportCheck{Name: "base_url", Status: "warn", Message: err.Error()})
			} else {
				checks = append(checks, reportCheck{Name: "base_url", Status: "ok"})
			}
		}
	}
	checks = append(checks, buildUpdateReportCheck(opts))
	for _, check := range AgentLocalHealthChecks(cfg, reportTimeout(opts)) {
		checks = append(checks, reportCheck{Name: check.Name, Status: check.Status, Message: check.Detail})
	}
	serviceCheck := CheckServiceStatus(context.Background(), ServiceHealthOptions{
		ConfigPath: opts.ConfigPath,
		Timeout:    reportTimeout(opts),
	})
	checks = append(checks, reportCheck{Name: serviceCheck.Name, Status: serviceCheck.Status, Message: serviceCheck.Detail})
	return checks
}

func buildUpdateReportCheck(opts ReportOptions) reportCheck {
	if !opts.CheckUpdate {
		return reportCheck{Name: "update", Status: "skipped", Message: "pass --check-update to query release metadata"}
	}
	result, err := CheckAgentUpdate(context.Background(), AgentUpdateCheckOptions{
		CurrentVersion:  opts.Version,
		Version:         "latest",
		Repo:            opts.UpdateRepo,
		ManifestURL:     opts.UpdateManifestURL,
		GitHubAPIBase:   opts.UpdateGitHubAPI,
		AllowPrerelease: opts.AllowPrerelease,
		Timeout:         reportTimeout(opts),
	})
	if err != nil {
		return reportCheck{Name: "update", Status: "warn", Message: err.Error()}
	}
	if result.UpdateAvailable {
		return reportCheck{Name: "update", Status: "warn", Message: fmt.Sprintf("%s available; current %s; asset %s", result.LatestVersion, result.CurrentVersion, result.AssetName)}
	}
	return reportCheck{Name: "update", Status: "ok", Message: result.CurrentVersion + " is current"}
}

func reportTimeout(opts ReportOptions) time.Duration {
	if opts.Timeout > 0 {
		return opts.Timeout
	}
	return 5 * time.Second
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
