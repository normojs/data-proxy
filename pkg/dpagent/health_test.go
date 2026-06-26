package dpagent

import (
	"bytes"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestCLIDoctorPrintsLocalHealth(t *testing.T) {
	local := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer local.Close()
	tcpListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer tcpListener.Close()
	bridge := newAgentAuthWebSocketServer(t, "sk-doctor-test")
	defer bridge.Close()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	cfg := DefaultConfig()
	cfg.Server.BaseURL = local.URL
	cfg.Server.BridgeWSURL = "ws" + strings.TrimPrefix(bridge.URL, "http")
	cfg.Agent.Token = "sk-doctor-test"
	cfg.Agent.Workspace = t.TempDir()
	cfg.Logging.LocalAuditJSONL = filepath.Join(t.TempDir(), "audit.jsonl")
	cfg.HTTPRoutes = []HTTPRoute{{Name: "local-web", Target: local.URL}}
	cfg.TCPRoutes = []TCPRoute{{Name: "local-tcp", TargetHost: "127.0.0.1", TargetPort: tcpListener.Addr().(*net.TCPAddr).Port}}
	if err := SaveConfig(configPath, cfg); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	code := RunCLI([]string{"doctor", "--config", configPath, "--timeout", "2s"}, &out, &errOut, "test-version")
	if code != 0 {
		t.Fatalf("doctor failed with code %d: %s\n%s", code, errOut.String(), out.String())
	}
	output := out.String()
	for _, want := range []string{
		"validation: ok",
		"token: configured",
		"bridge_auth: ok",
		"workspace: ok:",
		"local_audit: ok:",
		"http_route.local-web: ok:",
		"tcp_route.local-tcp: ok:",
		"doctor: ok",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("doctor output missing %q:\n%s", want, output)
		}
	}
}

func TestCLIDoctorJSON(t *testing.T) {
	local := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer local.Close()
	tcpListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer tcpListener.Close()
	bridge := newAgentAuthWebSocketServer(t, "sk-doctor-json")
	defer bridge.Close()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	cfg := DefaultConfig()
	cfg.Server.BaseURL = local.URL
	cfg.Server.BridgeWSURL = "ws" + strings.TrimPrefix(bridge.URL, "http")
	cfg.Agent.Token = "sk-doctor-json"
	cfg.Agent.Workspace = t.TempDir()
	cfg.Logging.LocalAuditJSONL = filepath.Join(t.TempDir(), "audit.jsonl")
	cfg.HTTPRoutes = []HTTPRoute{{Name: "local-web", Target: local.URL}}
	cfg.TCPRoutes = []TCPRoute{{Name: "local-tcp", TargetHost: "127.0.0.1", TargetPort: tcpListener.Addr().(*net.TCPAddr).Port}}
	if err := SaveConfig(configPath, cfg); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	code := RunCLI([]string{"doctor", "--json", "--config", configPath, "--timeout", "2s"}, &out, &errOut, "test-version")
	if code != 0 {
		t.Fatalf("doctor --json failed with code %d: %s\n%s", code, errOut.String(), out.String())
	}
	if strings.Contains(out.String(), "sk-doctor-json") {
		t.Fatalf("doctor --json leaked token: %s", out.String())
	}

	var report AgentDoctorReport
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("doctor --json output is invalid JSON: %s\n%s", err, out.String())
	}
	if !report.OK || !report.ConfigLoaded || report.Version != "test-version" || !report.Validation.OK() {
		t.Fatalf("unexpected doctor report: %#v", report)
	}
	expected := map[string]string{
		"dns":                  HealthStatusOK,
		"base_url":             HealthStatusOK,
		"token":                "configured",
		"bridge_auth":          HealthStatusOK,
		"workspace":            HealthStatusOK,
		"local_audit":          HealthStatusOK,
		"http_route.local-web": HealthStatusOK,
		"tcp_route.local-tcp":  HealthStatusOK,
		"update":               "skipped",
	}
	for name, want := range expected {
		if got := statusForDoctorCheck(report.Checks, name); got != want {
			t.Fatalf("doctor check %s = %q, want %q; checks=%#v", name, got, want, report.Checks)
		}
	}
}

func TestCLIDoctorCheckUpdateFromManifest(t *testing.T) {
	local := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer local.Close()
	bridge := newAgentAuthWebSocketServer(t, "sk-doctor-update")
	defer bridge.Close()
	assetName := "data-proxy-agent-v1.2.4-" + runtime.GOOS + "-" + runtime.GOARCH + agentTestArchiveExt(runtime.GOOS)
	manifest := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(agentUpdateManifest{
			Version: "v1.2.4",
			Assets: []agentUpdateAsset{{
				Name: assetName,
				URL:  serverURLFromRequest(r) + "/" + assetName,
				OS:   runtime.GOOS,
				Arch: runtime.GOARCH,
			}},
		})
	}))
	defer manifest.Close()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	cfg := DefaultConfig()
	cfg.Server.BaseURL = local.URL
	cfg.Server.BridgeWSURL = "ws" + strings.TrimPrefix(bridge.URL, "http")
	cfg.Agent.Token = "sk-doctor-update"
	cfg.Agent.Workspace = t.TempDir()
	cfg.Logging.LocalAuditJSONL = filepath.Join(t.TempDir(), "audit.jsonl")
	if err := SaveConfig(configPath, cfg); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	code := RunCLI([]string{
		"doctor",
		"--config", configPath,
		"--timeout", "2s",
		"--check-update",
		"--manifest-url", manifest.URL,
	}, &out, &errOut, "v1.2.3")
	if code != 0 {
		t.Fatalf("doctor check-update failed with code %d: %s\n%s", code, errOut.String(), out.String())
	}
	output := out.String()
	if !strings.Contains(output, "update: warn: v1.2.4 available") || !strings.Contains(output, assetName) {
		t.Fatalf("doctor output missing update warning:\n%s", output)
	}
	if !strings.Contains(output, "doctor: ok") {
		t.Fatalf("update warning should not fail doctor:\n%s", output)
	}
}

func TestCheckBridgeWebSocketAuth(t *testing.T) {
	bridge := newAgentAuthWebSocketServer(t, "sk-valid")
	defer bridge.Close()
	bridgeURL := "ws" + strings.TrimPrefix(bridge.URL, "http")

	if err := checkBridgeWebSocketAuth(bridgeURL, "sk-valid", 2*time.Second); err != nil {
		t.Fatalf("expected bridge auth to pass: %s", err)
	}
	err := checkBridgeWebSocketAuth(bridgeURL, "sk-invalid", 2*time.Second)
	if err == nil || !strings.Contains(err.Error(), "HTTP 401") {
		t.Fatalf("expected HTTP 401 error, got %v", err)
	}
}

func TestBuildReportChecksIncludesBridgeAuth(t *testing.T) {
	bridge := newAgentAuthWebSocketServer(t, "sk-report")
	defer bridge.Close()

	cfg := DefaultConfig()
	cfg.Server.BridgeWSURL = "ws" + strings.TrimPrefix(bridge.URL, "http")
	cfg.Agent.Token = "sk-report"
	checks := buildReportChecks(cfg, ReportOptions{Timeout: 2 * time.Second})
	if statusForReportCheck(checks, "bridge_auth") != "ok" {
		t.Fatalf("expected bridge_auth ok: %#v", checks)
	}
}

func TestAgentLocalHealthChecksRoutesAndMCP(t *testing.T) {
	local := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer local.Close()
	tcpListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer tcpListener.Close()

	cfg := DefaultConfig()
	cfg.Agent.Workspace = t.TempDir()
	cfg.Logging.LocalAuditJSONL = filepath.Join(t.TempDir(), "audit.jsonl")
	cfg.HTTPRoutes = []HTTPRoute{{Name: "local-web", Target: local.URL}}
	cfg.TCPRoutes = []TCPRoute{{Name: "local-tcp", TargetHost: "127.0.0.1", TargetPort: tcpListener.Addr().(*net.TCPAddr).Port}}
	cfg.MCPServers = []MCPServer{{Name: "coding", Transport: "streamable_http", Endpoint: local.URL + "/mcp"}}

	checks := AgentLocalHealthChecks(cfg, 2*time.Second)
	if statusForHealthCheck(checks, "workspace") != HealthStatusOK {
		t.Fatalf("workspace should be ok: %#v", checks)
	}
	if statusForHealthCheck(checks, "http_route.local-web") != HealthStatusOK {
		t.Fatalf("http route should be ok: %#v", checks)
	}
	if statusForHealthCheck(checks, "tcp_route.local-tcp") != HealthStatusOK {
		t.Fatalf("tcp route should be ok: %#v", checks)
	}
	if statusForHealthCheck(checks, "mcp_server.coding") != HealthStatusOK {
		t.Fatalf("mcp server should be ok: %#v", checks)
	}
}

func TestAgentLocalHealthChecksRejectForbiddenNonLoopback(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Agent.Workspace = t.TempDir()
	cfg.HTTPRoutes = []HTTPRoute{{Name: "public", Target: "http://example.com"}}
	cfg.TCPRoutes = []TCPRoute{{Name: "public-tcp", TargetHost: "192.0.2.10", TargetPort: 22}}
	cfg.MCPServers = []MCPServer{{Name: "remote-mcp", Transport: "streamable_http", Endpoint: "https://example.com/mcp"}}

	checks := AgentLocalHealthChecks(cfg, 10*time.Millisecond)
	if statusForHealthCheck(checks, "http_route.public") != HealthStatusFail {
		t.Fatalf("public http route should fail: %#v", checks)
	}
	if statusForHealthCheck(checks, "tcp_route.public-tcp") != HealthStatusFail {
		t.Fatalf("public tcp route should fail: %#v", checks)
	}
	if statusForHealthCheck(checks, "mcp_server.remote-mcp") != HealthStatusFail {
		t.Fatalf("public mcp target should fail: %#v", checks)
	}
}

func TestStdioCommandPrefixSkipsEnvAssignments(t *testing.T) {
	got, ok := stdioCommandPrefix("env FOO=bar npx -y @modelcontextprotocol/server-filesystem /tmp")
	if !ok || got != "npx" {
		t.Fatalf("unexpected stdio prefix: %q ok=%t", got, ok)
	}
	if _, ok := stdioCommandPrefix("cd /tmp && npx -y server"); ok {
		t.Fatal("shell builtin command should not be treated as a concrete executable prefix")
	}
}

func TestClassifyMCPStdioStderr(t *testing.T) {
	cases := map[string]string{
		"Error: Cannot find module '@modelcontextprotocol/server-filesystem'": "dependency",
		"sh: mcp-server: command not found":                                   "command_not_found",
		"listen EADDRINUSE: address already in use 127.0.0.1:30837":           "port_in_use",
		"panic: runtime error":                                                "crash",
		"permission denied opening workspace":                                 "permission",
		"random stderr line":                                                  "stderr",
		"":                                                                    "",
	}
	for input, expected := range cases {
		if got := classifyMCPStdioStderr(input); got != expected {
			t.Fatalf("classifyMCPStdioStderr(%q) = %q, want %q", input, got, expected)
		}
	}
}

func TestAgentLocalHealthChecksClassifiesExitedStdioStderr(t *testing.T) {
	previous := defaultMCPStdioSessions
	defaultMCPStdioSessions = newMCPStdioSessionCache()
	t.Cleanup(func() {
		defaultMCPStdioSessions = previous
	})

	done := make(chan struct{})
	close(done)
	stderr := &limitedBuffer{limit: 1024}
	_, _ = stderr.Write([]byte("Error: Cannot find module '@modelcontextprotocol/server-filesystem'\n"))
	session := &mcpStdioSession{
		key:    "stdio:filesystem",
		server: MCPServer{Name: "filesystem"},
		stderr: stderr,
		done:   done,
		pid:    12345,
	}
	session.setWaitErr(errors.New("exit status 1"))
	defaultMCPStdioSessions.sessions["stdio:filesystem"] = session

	command := "sh -c fake-mcp"
	if runtime.GOOS == "windows" {
		command = "cmd /C fake-mcp"
	}
	cfg := DefaultConfig()
	cfg.MCPServers = []MCPServer{{Name: "filesystem", Transport: "stdio", Command: command}}

	checks := AgentLocalHealthChecks(cfg, 0)
	detail := detailForHealthCheck(checks, "mcp_server.filesystem")
	if statusForHealthCheck(checks, "mcp_server.filesystem") != HealthStatusWarn {
		t.Fatalf("stdio exited process should warn: %#v", checks)
	}
	if !strings.Contains(detail, "stderr_class=dependency") {
		t.Fatalf("stdio health did not include stderr classification: %s", detail)
	}
	report := BuildAgentHealthReport(cfg, 0)
	if len(report.MCPProcesses) != 1 {
		t.Fatalf("expected one MCP process summary: %#v", report.MCPProcesses)
	}
	process := report.MCPProcesses[0]
	if process.Name != "filesystem" || process.Transport != "stdio" || process.Status != "exited" {
		t.Fatalf("unexpected MCP process summary: %#v", process)
	}
	if process.PID != 12345 || process.StderrClass != "dependency" || !strings.Contains(process.ExitError, "exit status 1") {
		t.Fatalf("missing stdio process diagnostics: %#v", process)
	}
}

func TestMCPStdioWatchdogReapsExitedSessions(t *testing.T) {
	cache := newMCPStdioSessionCache()
	auditPath := filepath.Join(t.TempDir(), "audit.jsonl")

	exitedDone := make(chan struct{})
	close(exitedDone)
	exitedStderr := &limitedBuffer{limit: 1024}
	_, _ = exitedStderr.Write([]byte("panic: boom\n"))
	exited := &mcpStdioSession{
		key:       "stdio:crashy",
		server:    MCPServer{Name: "crashy", Transport: "stdio", Command: "fake-mcp"},
		stderr:    exitedStderr,
		done:      exitedDone,
		pid:       4242,
		auditPath: auditPath,
	}
	exited.setWaitErr(errors.New("exit status 2"))

	aliveDone := make(chan struct{})
	alive := &mcpStdioSession{
		key:       "stdio:alive",
		server:    MCPServer{Name: "alive", Transport: "stdio", Command: "fake-mcp"},
		stderr:    &limitedBuffer{limit: 1024},
		done:      aliveDone,
		pid:       4343,
		auditPath: auditPath,
	}

	cache.sessions["stdio:crashy"] = exited
	cache.sessions["stdio:alive"] = alive

	summary := cache.ReapExited()
	if summary.Reaped != 1 || len(summary.Keys) != 1 || summary.Keys[0] != "stdio:crashy" {
		t.Fatalf("unexpected watchdog summary: %#v", summary)
	}
	if cache.Status("stdio:crashy").Exists {
		t.Fatal("exited stdio session was not reaped")
	}
	if !cache.Status("stdio:alive").Exists {
		t.Fatal("alive stdio session was reaped")
	}

	auditBytes, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatal(err)
	}
	auditText := string(auditBytes)
	if !strings.Contains(auditText, `"tool_name":"mcp_stdio.watchdog_reap"`) ||
		!strings.Contains(auditText, `"session_key":"stdio:crashy"`) ||
		!strings.Contains(auditText, `"stderr_class":"crash"`) {
		t.Fatalf("watchdog audit missing expected metadata: %s", auditText)
	}
}

func statusForHealthCheck(checks []AgentHealthCheck, name string) string {
	for _, check := range checks {
		if check.Name == name {
			return check.Status
		}
	}
	return ""
}

func detailForHealthCheck(checks []AgentHealthCheck, name string) string {
	for _, check := range checks {
		if check.Name == name {
			return check.Detail
		}
	}
	return ""
}

func statusForReportCheck(checks []reportCheck, name string) string {
	for _, check := range checks {
		if check.Name == name {
			return check.Status
		}
	}
	return ""
}

func statusForDoctorCheck(checks []AgentDoctorCheck, name string) string {
	for _, check := range checks {
		if check.Name == name {
			return check.Status
		}
	}
	return ""
}

func newAgentAuthWebSocketServer(t *testing.T, token string) *httptest.Server {
	t.Helper()
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer "+token {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatal(err)
		}
		_ = conn.Close()
	}))
}
