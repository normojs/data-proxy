package dpagent

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/dto"
)

const (
	HealthStatusOK   = "ok"
	HealthStatusWarn = "warn"
	HealthStatusFail = "fail"
)

type AgentHealthCheck struct {
	Name   string
	Status string
	Detail string
}

func AgentLocalHealthChecks(cfg Config, timeout time.Duration) []AgentHealthCheck {
	var checks []AgentHealthCheck
	checks = append(checks, checkAgentWorkspace(cfg)...)
	checks = append(checks, checkAgentAuditLog(cfg)...)
	for _, route := range cfg.HTTPRoutes {
		checks = append(checks, checkHTTPRoute(cfg, route, timeout))
	}
	for _, server := range cfg.MCPServers {
		checks = append(checks, checkMCPServer(cfg, server, timeout))
	}
	return checks
}

func BuildAgentHealthReport(cfg Config, timeout time.Duration) dto.BridgeAgentHealthReport {
	fillConfigDefaults(&cfg)
	checks := AgentLocalHealthChecks(cfg, timeout)
	reportChecks := make([]dto.BridgeAgentHealthCheck, 0, len(checks))
	summary := dto.BridgeAgentHealthSummary{
		Status: HealthStatusOK,
		Total:  len(checks),
	}
	for _, check := range checks {
		status := normalizeAgentHealthStatus(check.Status)
		switch status {
		case HealthStatusFail:
			summary.Fail++
		case HealthStatusWarn:
			summary.Warn++
		default:
			summary.OK++
		}
		reportChecks = append(reportChecks, dto.BridgeAgentHealthCheck{
			Name:   check.Name,
			Status: status,
			Detail: check.Detail,
		})
	}
	if summary.Fail > 0 {
		summary.Status = HealthStatusFail
	} else if summary.Warn > 0 {
		summary.Status = HealthStatusWarn
	}
	return dto.BridgeAgentHealthReport{
		GeneratedAt: time.Now().Unix(),
		Version:     agentVersion(cfg),
		Platform:    agentPlatform(),
		Workspace:   cfg.Agent.Workspace,
		Summary:     summary,
		Checks:      reportChecks,
	}
}

func normalizeAgentHealthStatus(status string) string {
	switch strings.TrimSpace(strings.ToLower(status)) {
	case HealthStatusFail:
		return HealthStatusFail
	case HealthStatusWarn:
		return HealthStatusWarn
	default:
		return HealthStatusOK
	}
}

func checkAgentWorkspace(cfg Config) []AgentHealthCheck {
	workspace := strings.TrimSpace(cfg.Agent.Workspace)
	if workspace == "" {
		return []AgentHealthCheck{{Name: "workspace", Status: HealthStatusWarn, Detail: "agent.workspace is empty"}}
	}
	path := expandPath(workspace)
	info, err := os.Stat(path)
	if err != nil {
		return []AgentHealthCheck{{Name: "workspace", Status: HealthStatusFail, Detail: err.Error()}}
	}
	if !info.IsDir() {
		return []AgentHealthCheck{{Name: "workspace", Status: HealthStatusFail, Detail: "not a directory: " + path}}
	}
	checks := []AgentHealthCheck{{Name: "workspace", Status: HealthStatusOK, Detail: path}}
	for _, allowed := range cfg.Policy.AllowedWorkspaces {
		allowed = strings.TrimSpace(allowed)
		if allowed == "" {
			continue
		}
		if _, err := os.Stat(expandPath(allowed)); err != nil {
			checks = append(checks, AgentHealthCheck{Name: "allowed_workspace", Status: HealthStatusWarn, Detail: allowed + ": " + err.Error()})
		}
	}
	return checks
}

func checkAgentAuditLog(cfg Config) []AgentHealthCheck {
	path := localAuditPath(cfg)
	if path == "" {
		return []AgentHealthCheck{{Name: "local_audit", Status: HealthStatusWarn, Detail: "disabled; set logging.local_audit_jsonl to keep metadata-only local audit"}}
	}
	dir := filepath.Dir(path)
	info, err := os.Stat(dir)
	if err != nil {
		return []AgentHealthCheck{{Name: "local_audit", Status: HealthStatusWarn, Detail: "directory will be created on first write: " + dir}}
	}
	if !info.IsDir() {
		return []AgentHealthCheck{{Name: "local_audit", Status: HealthStatusFail, Detail: "parent is not a directory: " + dir}}
	}
	return []AgentHealthCheck{{Name: "local_audit", Status: HealthStatusOK, Detail: path}}
}

func checkHTTPRoute(cfg Config, route HTTPRoute, timeout time.Duration) AgentHealthCheck {
	name := "http_route." + safeHealthName(route.Name)
	target, err := allowedHTTPTarget(cfg, route.Target)
	if err != nil {
		return AgentHealthCheck{Name: name, Status: HealthStatusFail, Detail: err.Error()}
	}
	if err := checkTCPURL(target, timeout); err != nil {
		return AgentHealthCheck{Name: name, Status: HealthStatusFail, Detail: err.Error()}
	}
	return AgentHealthCheck{Name: name, Status: HealthStatusOK, Detail: target}
}

func checkMCPServer(cfg Config, server MCPServer, timeout time.Duration) AgentHealthCheck {
	name := "mcp_server." + safeHealthName(server.Name)
	transport := normalizeMCPTransport(server.Transport, server.Endpoint, server.Command)
	switch transport {
	case "stdio":
		return checkStdioMCPServer(name, server)
	case "streamable_http", "http", "sse":
		target, err := allowedMCPHealthTarget(cfg, server.Endpoint)
		if err != nil {
			return AgentHealthCheck{Name: name, Status: HealthStatusFail, Detail: err.Error()}
		}
		if err := checkTCPURL(target, timeout); err != nil {
			return AgentHealthCheck{Name: name, Status: HealthStatusFail, Detail: err.Error()}
		}
		return AgentHealthCheck{Name: name, Status: HealthStatusOK, Detail: transport + " " + target}
	case "":
		return AgentHealthCheck{Name: name, Status: HealthStatusFail, Detail: "transport is empty"}
	default:
		return AgentHealthCheck{Name: name, Status: HealthStatusFail, Detail: "unsupported transport: " + transport}
	}
}

func checkStdioMCPServer(name string, server MCPServer) AgentHealthCheck {
	command := strings.TrimSpace(server.Command)
	if command == "" {
		return AgentHealthCheck{Name: name, Status: HealthStatusFail, Detail: "stdio command is empty"}
	}
	if err := checkStdioShell(); err != nil {
		return AgentHealthCheck{Name: name, Status: HealthStatusFail, Detail: err.Error()}
	}
	prefix, ok := stdioCommandPrefix(command)
	if !ok {
		return AgentHealthCheck{Name: name, Status: HealthStatusWarn, Detail: "shell command configured; unable to identify executable prefix"}
	}
	if _, err := exec.LookPath(prefix); err != nil {
		return AgentHealthCheck{Name: name, Status: HealthStatusWarn, Detail: "shell ok; executable prefix not found in PATH: " + prefix}
	}
	detail := "stdio command prefix found: " + prefix
	status := defaultMCPStdioSessions.Status("stdio:" + server.Name)
	if !status.Exists {
		return AgentHealthCheck{Name: name, Status: HealthStatusOK, Detail: detail + "; process not started"}
	}
	if status.Alive {
		initialized := "not initialized"
		if status.Initialized {
			initialized = "initialized"
		}
		return AgentHealthCheck{Name: name, Status: HealthStatusOK, Detail: fmt.Sprintf("%s; process running pid=%d %s", detail, status.PID, initialized)}
	}
	if status.ExitError != "" {
		return AgentHealthCheck{Name: name, Status: HealthStatusWarn, Detail: detail + "; previous process exited: " + status.ExitError}
	}
	return AgentHealthCheck{Name: name, Status: HealthStatusWarn, Detail: detail + "; previous process exited; next call will restart"}
}

func checkStdioShell() error {
	if runtime.GOOS == "windows" {
		if _, err := exec.LookPath("cmd"); err != nil {
			return fmt.Errorf("cmd shell not found: %w", err)
		}
		return nil
	}
	if _, err := os.Stat("/bin/sh"); err != nil {
		return fmt.Errorf("/bin/sh not found: %w", err)
	}
	return nil
}

func stdioCommandPrefix(command string) (string, bool) {
	fields := strings.Fields(command)
	for len(fields) > 0 {
		first := strings.Trim(fields[0], "\"'")
		if first == "" {
			fields = fields[1:]
			continue
		}
		if first == "env" || strings.Contains(first, "=") {
			fields = fields[1:]
			continue
		}
		if isShellBuiltinPrefix(first) {
			return "", false
		}
		return first, true
	}
	return "", false
}

func isShellBuiltinPrefix(value string) bool {
	switch value {
	case "cd", "export", "source", ".", "set", "ulimit":
		return true
	default:
		return strings.Contains(value, "&&") || strings.Contains(value, ";")
	}
}

func allowedMCPHealthTarget(cfg Config, target string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(target))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		if err == nil {
			err = fmt.Errorf("missing scheme or host")
		}
		return "", fmt.Errorf("invalid MCP target: %s", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("only http/https MCP targets are supported")
	}
	if cfg.Policy.AllowNonLoopbackMCP || isLoopbackHost(parsed.Hostname()) {
		return parsed.String(), nil
	}
	return "", fmt.Errorf("MCP proxy target must be loopback unless allow_non_loopback_mcp is enabled")
}

func checkTCPURL(rawURL string, timeout time.Duration) error {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return err
	}
	host := parsed.Hostname()
	if host == "" {
		return fmt.Errorf("missing host")
	}
	port := parsed.Port()
	if port == "" {
		switch parsed.Scheme {
		case "https", "wss":
			port = "443"
		default:
			port = "80"
		}
	}
	address := net.JoinHostPort(host, port)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return fmt.Errorf("tcp %s: %w", address, err)
	}
	_ = conn.Close()
	return nil
}

func safeHealthName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unnamed"
	}
	return strings.ReplaceAll(value, " ", "_")
}
