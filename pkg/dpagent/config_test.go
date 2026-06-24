package dpagent

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/dto"
)

func TestBridgeURLFromBaseURL(t *testing.T) {
	got, err := BridgeURLFromBaseURL("https://dp.example.com")
	if err != nil {
		t.Fatal(err)
	}
	if got != "wss://dp.example.com/bridge/ws" {
		t.Fatalf("unexpected bridge URL: %s", got)
	}

	got, err = BridgeURLFromBaseURL("http://127.0.0.1:13002/base")
	if err != nil {
		t.Fatal(err)
	}
	if got != "ws://127.0.0.1:13002/base/bridge/ws" {
		t.Fatalf("unexpected bridge URL with path: %s", got)
	}
}

func TestValidateConfigRequiresServer(t *testing.T) {
	cfg := DefaultConfig()
	result := ValidateConfig(cfg, false)
	if result.OK() {
		t.Fatal("expected validation to fail without server URL")
	}
	if !strings.Contains(strings.Join(result.Errors, "\n"), "server.bridge_ws_url") {
		t.Fatalf("unexpected errors: %#v", result.Errors)
	}
}

func TestResolveTokenPrefersConfigAndEnvironment(t *testing.T) {
	t.Setenv("DATA_PROXY_API_KEY", "env-token")
	cfg := DefaultConfig()
	if got := ResolveToken(cfg); got != "env-token" {
		t.Fatalf("expected env token, got %q", got)
	}
	cfg.Agent.Token = "config-token"
	if got := ResolveToken(cfg); got != "config-token" {
		t.Fatalf("expected config token, got %q", got)
	}
}

func TestRedactedConfigMasksToken(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Agent.Token = "sk-1234567890abcdef"
	redacted := RedactedConfig(cfg)
	if redacted.Agent.Token == cfg.Agent.Token {
		t.Fatal("token was not redacted")
	}
	if !strings.Contains(redacted.Agent.Token, "...") {
		t.Fatalf("unexpected redacted token: %q", redacted.Agent.Token)
	}
}

func TestCLISelfTest(t *testing.T) {
	var out, errOut bytes.Buffer
	code := RunCLI([]string{"self-test"}, &out, &errOut, "test-version")
	if code != 0 {
		t.Fatalf("self-test failed with code %d: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "self-test: ok") {
		t.Fatalf("unexpected self-test output: %s", out.String())
	}
}

func TestCLIConfigShowRedactsToken(t *testing.T) {
	tmp := t.TempDir()
	configPath := tmp + "/config.yaml"
	cfg := DefaultConfig()
	cfg.Server.BaseURL = "https://dp.example.com"
	cfg.Agent.Token = "sk-secret-token-value"
	if err := SaveConfig(configPath, cfg); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	code := RunCLI([]string{"config", "show", "--config", configPath}, &out, &errOut, "test-version")
	if code != 0 {
		t.Fatalf("config show failed with code %d: %s", code, errOut.String())
	}
	if strings.Contains(out.String(), "sk-secret-token-value") {
		t.Fatalf("config show leaked token: %s", out.String())
	}
	if !strings.Contains(out.String(), "sk-s...alue") {
		t.Fatalf("config show did not include masked token: %s", out.String())
	}
}

func TestCLIMCPAddListRemove(t *testing.T) {
	configPath := t.TempDir() + "/config.yaml"
	cfg := DefaultConfig()
	cfg.Server.BaseURL = "https://dp.example.com"
	if err := SaveConfig(configPath, cfg); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	code := RunCLI([]string{"mcp", "add", "coding", "--url", "http://127.0.0.1:30837/mcp", "--config", configPath}, &out, &errOut, "test-version")
	if code != 0 {
		t.Fatalf("mcp add failed with code %d: %s", code, errOut.String())
	}
	loaded, _, err := LoadConfig(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.MCPServers) != 1 || loaded.MCPServers[0].Name != "coding" || loaded.MCPServers[0].Transport != "streamable_http" {
		t.Fatalf("unexpected mcp servers: %#v", loaded.MCPServers)
	}

	out.Reset()
	errOut.Reset()
	code = RunCLI([]string{"mcp", "list", "--config", configPath}, &out, &errOut, "test-version")
	if code != 0 {
		t.Fatalf("mcp list failed with code %d: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "coding") {
		t.Fatalf("mcp list did not include server: %s", out.String())
	}

	out.Reset()
	errOut.Reset()
	code = RunCLI([]string{"mcp", "remove", "coding", "--config", configPath}, &out, &errOut, "test-version")
	if code != 0 {
		t.Fatalf("mcp remove failed with code %d: %s", code, errOut.String())
	}
	loaded, _, err = LoadConfig(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.MCPServers) != 0 {
		t.Fatalf("mcp server was not removed: %#v", loaded.MCPServers)
	}
}

func TestCLIMCPTestSupportsStdio(t *testing.T) {
	configPath := t.TempDir() + "/config.yaml"
	cfg := DefaultConfig()
	cfg.Server.BaseURL = "https://dp.example.com"
	if err := SaveConfig(configPath, cfg); err != nil {
		t.Fatal(err)
	}
	defer defaultMCPStdioSessions.Forget("stdio:filesystem")

	var out, errOut bytes.Buffer
	code := RunCLI([]string{
		"mcp", "add", "filesystem",
		"--transport", "stdio",
		"--command", fakeStdioMCPCommand(),
		"--config", configPath,
	}, &out, &errOut, "test-version")
	if code != 0 {
		t.Fatalf("mcp stdio add failed with code %d: %s", code, errOut.String())
	}

	out.Reset()
	errOut.Reset()
	code = RunCLI([]string{"mcp", "test", "filesystem", "--config", configPath}, &out, &errOut, "test-version")
	if code != 0 {
		t.Fatalf("mcp stdio test failed with code %d: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "mcp server ok: filesystem (fake-stdio-mcp)") {
		t.Fatalf("unexpected mcp stdio test output: %s", out.String())
	}
}

func TestCLITunnelRouteAddListRemove(t *testing.T) {
	configPath := t.TempDir() + "/config.yaml"
	cfg := DefaultConfig()
	cfg.Server.BaseURL = "https://dp.example.com"
	if err := SaveConfig(configPath, cfg); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	code := RunCLI([]string{
		"tunnel", "route", "add", "http", "local-web",
		"--url", "http://127.0.0.1:3000",
		"--allow-websocket",
		"--config", configPath,
	}, &out, &errOut, "test-version")
	if code != 0 {
		t.Fatalf("tunnel route add failed with code %d: %s", code, errOut.String())
	}
	loaded, _, err := LoadConfig(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.HTTPRoutes) != 1 || loaded.HTTPRoutes[0].Name != "local-web" || !loaded.HTTPRoutes[0].AllowWebSocket {
		t.Fatalf("unexpected routes: %#v", loaded.HTTPRoutes)
	}

	out.Reset()
	errOut.Reset()
	code = RunCLI([]string{"tunnel", "route", "list", "--config", configPath}, &out, &errOut, "test-version")
	if code != 0 {
		t.Fatalf("tunnel route list failed with code %d: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "local-web") {
		t.Fatalf("tunnel route list did not include route: %s", out.String())
	}

	out.Reset()
	errOut.Reset()
	code = RunCLI([]string{"tunnel", "route", "remove", "local-web", "--config", configPath}, &out, &errOut, "test-version")
	if code != 0 {
		t.Fatalf("tunnel route remove failed with code %d: %s", code, errOut.String())
	}
	loaded, _, err = LoadConfig(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.HTTPRoutes) != 0 {
		t.Fatalf("route was not removed: %#v", loaded.HTTPRoutes)
	}
}

func TestLocalAuditWritesMetadataOnly(t *testing.T) {
	auditPath := t.TempDir() + "/audit.jsonl"
	cfg := DefaultConfig()
	cfg.Logging.LocalAuditJSONL = auditPath
	result := dto.BridgeToolCallResult{
		ResultSize: 123,
		Metadata: map[string]any{
			"target":    "stdio:coding",
			"transport": "stdio",
			"result":    map[string]any{"secret": true},
			"custom":    "should not be copied",
		},
	}
	err := (BridgeClient{Config: cfg}).auditBridgeToolCall("req-1", BridgeToolMCPProxyCallTool, result, nil, 15*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	bytes, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatal(err)
	}
	var entry map[string]any
	if err := json.Unmarshal(bytes, &entry); err != nil {
		t.Fatal(err)
	}
	if entry["request_id"] != "req-1" || entry["tool_name"] != BridgeToolMCPProxyCallTool || entry["success"] != true {
		t.Fatalf("unexpected audit entry: %#v", entry)
	}
	metadata := mapFromAny(entry["metadata"])
	if metadata["target"] != "stdio:coding" || metadata["transport"] != "stdio" {
		t.Fatalf("unexpected audit metadata: %#v", metadata)
	}
	if _, ok := metadata["result"]; ok {
		t.Fatalf("audit metadata leaked result: %#v", metadata)
	}
	if _, ok := metadata["custom"]; ok {
		t.Fatalf("audit metadata copied custom field: %#v", metadata)
	}
}

func TestCLILogsPathAndTail(t *testing.T) {
	tmp := t.TempDir()
	configPath := tmp + "/config.yaml"
	auditPath := tmp + "/audit.jsonl"
	cfg := DefaultConfig()
	cfg.Server.BaseURL = "https://dp.example.com"
	cfg.Logging.LocalAuditJSONL = auditPath
	if err := SaveConfig(configPath, cfg); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(auditPath, []byte("one\ntwo\nthree\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	code := RunCLI([]string{"logs", "path", "--config", configPath}, &out, &errOut, "test-version")
	if code != 0 {
		t.Fatalf("logs path failed with code %d: %s", code, errOut.String())
	}
	if strings.TrimSpace(out.String()) != auditPath {
		t.Fatalf("unexpected logs path output: %q", out.String())
	}

	out.Reset()
	errOut.Reset()
	code = RunCLI([]string{"logs", "tail", "--lines", "2", "--config", configPath}, &out, &errOut, "test-version")
	if code != 0 {
		t.Fatalf("logs tail failed with code %d: %s", code, errOut.String())
	}
	if out.String() != "two\nthree\n" {
		t.Fatalf("unexpected logs tail output: %q", out.String())
	}
}

func TestFollowLocalAuditPrintsAppendedLines(t *testing.T) {
	auditPath := t.TempDir() + "/audit.jsonl"
	if err := os.WriteFile(auditPath, []byte("old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	go func() {
		time.Sleep(50 * time.Millisecond)
		file, err := os.OpenFile(auditPath, os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			return
		}
		_, _ = file.WriteString("new\n")
		_ = file.Close()
	}()
	var out bytes.Buffer
	err := followLocalAudit(ctx, auditPath, &out, 10*time.Millisecond)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context deadline from test follow loop, got %v", err)
	}
	if out.String() != "new\n" {
		t.Fatalf("unexpected follow output: %q", out.String())
	}
}

func TestCLIEnrollWritesPrivateConfig(t *testing.T) {
	configPath := t.TempDir() + "/config.yaml"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/bridge/agent-setup" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "access-token" {
			t.Fatalf("unexpected authorization header: %s", got)
		}
		if got := r.Header.Get("New-Api-User"); got != "42" {
			t.Fatalf("unexpected user id header: %s", got)
		}
		var req dto.BridgeAgentSetupRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		if req.ClientName != "Laptop Agent" || req.Version != "test-version" {
			t.Fatalf("unexpected setup request: %#v", req)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(apiEnvelope[dto.BridgeAgentSetupResponse]{
			Success: true,
			Data: dto.BridgeAgentSetupResponse{
				BaseURL:        serverURLFromRequest(r),
				BridgeWSURL:    "ws" + strings.TrimPrefix(serverURLFromRequest(r), "http") + "/bridge/ws",
				ClientId:       "client-123",
				APIKey:         "sk-agent-token-secret",
				APIKeyOnce:     true,
				TokenMaskedKey: "sk-age...cret",
				Client: dto.BridgeClientItem{
					ClientId:  "client-123",
					Name:      "Laptop Agent",
					Workspace: "/workspace/project",
				},
			},
		})
	}))
	defer server.Close()

	var out, errOut bytes.Buffer
	code := RunCLI([]string{
		"enroll",
		"--server", server.URL,
		"--access-token", "access-token",
		"--user-id", "42",
		"--name", "Laptop Agent",
		"--workspace", "/workspace/project",
		"--config", configPath,
	}, &out, &errOut, "test-version")
	if code != 0 {
		t.Fatalf("enroll failed with code %d: %s", code, errOut.String())
	}
	if strings.Contains(out.String(), "sk-agent-token-secret") {
		t.Fatalf("enroll leaked agent token: %s", out.String())
	}
	loaded, _, err := LoadConfig(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Server.BaseURL != server.URL {
		t.Fatalf("unexpected base url: %s", loaded.Server.BaseURL)
	}
	if loaded.Agent.ClientID != "client-123" || loaded.Agent.Token != "sk-agent-token-secret" {
		t.Fatalf("unexpected enrolled config: %#v", loaded.Agent)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(configPath)
		if err != nil {
			t.Fatal(err)
		}
		if mode := info.Mode().Perm(); mode != DefaultConfigFileMode {
			t.Fatalf("unexpected config mode: %o", mode)
		}
	}
}

func TestCLIEnrollWithSetupTokenWritesPrivateConfig(t *testing.T) {
	configPath := t.TempDir() + "/config.yaml"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/bridge/agent-setup/consume" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "" {
			t.Fatalf("unexpected authorization header: %s", got)
		}
		if got := r.Header.Get("New-Api-User"); got != "" {
			t.Fatalf("unexpected user id header: %s", got)
		}
		var req dto.BridgeAgentSetupTokenConsumeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		if req.SetupToken != "setup-token-123" || req.ClientName != "Laptop Agent" || req.Version != "test-version" {
			t.Fatalf("unexpected setup token request: %#v", req)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(apiEnvelope[dto.BridgeAgentSetupResponse]{
			Success: true,
			Data: dto.BridgeAgentSetupResponse{
				BaseURL:        serverURLFromRequest(r),
				BridgeWSURL:    "ws" + strings.TrimPrefix(serverURLFromRequest(r), "http") + "/bridge/ws",
				ClientId:       "client-setup-token",
				APIKey:         "sk-agent-token-secret",
				APIKeyOnce:     true,
				TokenMaskedKey: "sk-age...cret",
				Client: dto.BridgeClientItem{
					ClientId:  "client-setup-token",
					Name:      "Laptop Agent",
					Workspace: "/workspace/project",
				},
			},
		})
	}))
	defer server.Close()

	var out, errOut bytes.Buffer
	code := RunCLI([]string{
		"enroll",
		"--server", server.URL,
		"--setup-token", "setup-token-123",
		"--name", "Laptop Agent",
		"--workspace", "/workspace/project",
		"--config", configPath,
	}, &out, &errOut, "test-version")
	if code != 0 {
		t.Fatalf("enroll failed with code %d: %s", code, errOut.String())
	}
	if strings.Contains(out.String(), "sk-agent-token-secret") {
		t.Fatalf("enroll leaked agent token: %s", out.String())
	}
	loaded, _, err := LoadConfig(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Agent.ClientID != "client-setup-token" || loaded.Agent.Token != "sk-agent-token-secret" {
		t.Fatalf("unexpected enrolled config: %#v", loaded.Agent)
	}
}

func TestCLIReportWritesRedactedDiagnosticZip(t *testing.T) {
	tmp := t.TempDir()
	configPath := tmp + "/config.yaml"
	reportPath := tmp + "/report.zip"
	local := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer local.Close()
	cfg := DefaultConfig()
	cfg.Server.BaseURL = "https://dp.example.com"
	cfg.Agent.ClientID = "report-agent"
	cfg.Agent.Token = "sk-report-token-secret"
	cfg.Agent.Workspace = tmp
	cfg.Logging.LocalAuditJSONL = tmp + "/audit.jsonl"
	cfg.HTTPRoutes = []HTTPRoute{{Name: "local-web", Target: local.URL}}
	cfg.MCPServers = []MCPServer{{Name: "coding", Transport: "streamable_http", Endpoint: local.URL + "/mcp"}}
	if err := SaveConfig(configPath, cfg); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	code := RunCLI([]string{"report", "--config", configPath, "--output", reportPath, "--skip-network"}, &out, &errOut, "test-version")
	if code != 0 {
		t.Fatalf("report failed with code %d: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "diagnostic report saved") {
		t.Fatalf("unexpected report output: %s", out.String())
	}
	reader, err := zip.OpenReader(reportPath)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	combined := readZipEntries(t, reader.File)
	if !strings.Contains(combined, "config.redacted.yaml") {
		t.Fatalf("report did not include config entry names: %s", combined)
	}
	if strings.Contains(combined, "sk-report-token-secret") {
		t.Fatalf("report leaked token: %s", combined)
	}
	if !strings.Contains(combined, "sk-r...cret") {
		t.Fatalf("report did not include redacted token: %s", combined)
	}
	for _, want := range []string{"workspace", "http_route.local-web", "mcp_server.coding"} {
		if !strings.Contains(combined, want) {
			t.Fatalf("report missing health check %q: %s", want, combined)
		}
	}
}

func TestServiceDefinitionsRenderPlatformManifests(t *testing.T) {
	configPath := writeTestServiceConfig(t)

	linux, err := BuildServiceDefinition(ServiceOptions{
		ConfigPath: configPath,
		BinaryPath: "/usr/local/bin/data-proxy-agent",
		Platform:   "linux",
		Scope:      "user",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(linux.InstallPath, "data-proxy-agent.service") {
		t.Fatalf("unexpected linux install path: %s", linux.InstallPath)
	}
	if !strings.Contains(linux.Content, `[Unit]`) || !strings.Contains(linux.Content, `ExecStart="/usr/local/bin/data-proxy-agent" "run" --config`) {
		t.Fatalf("unexpected linux unit:\n%s", linux.Content)
	}
	if !strings.Contains(linux.Content, configPath) {
		t.Fatalf("linux unit missing config path:\n%s", linux.Content)
	}

	darwin, err := BuildServiceDefinition(ServiceOptions{
		ConfigPath: configPath,
		BinaryPath: "/usr/local/bin/data-proxy-agent",
		Platform:   "darwin",
		Scope:      "user",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(darwin.InstallPath, "ltd.mbu.dataproxy.agent.plist") {
		t.Fatalf("unexpected launchd path: %s", darwin.InstallPath)
	}
	if !strings.Contains(darwin.Content, `<key>ProgramArguments</key>`) || !strings.Contains(darwin.Content, `<string>run</string>`) {
		t.Fatalf("unexpected launchd plist:\n%s", darwin.Content)
	}

	windows, err := BuildServiceDefinition(ServiceOptions{
		ConfigPath: configPath,
		BinaryPath: `C:\Program Files\DataProxy\data-proxy-agent.exe`,
		Platform:   "windows",
		Scope:      "system",
	})
	if err != nil {
		t.Fatal(err)
	}
	if windows.InstallPath != `HKLM\SYSTEM\CurrentControlSet\Services\DataProxyAgent` {
		t.Fatalf("unexpected windows install path: %s", windows.InstallPath)
	}
	if !strings.Contains(windows.Content, `"run" --config`) {
		t.Fatalf("unexpected windows command line: %s", windows.Content)
	}
}

func TestCLIServiceInstallDryRun(t *testing.T) {
	configPath := writeTestServiceConfig(t)
	var out, errOut bytes.Buffer
	code := RunCLI([]string{
		"service", "install",
		"--dry-run",
		"--platform", "linux",
		"--scope", "user",
		"--binary", "/usr/local/bin/data-proxy-agent",
		"--config", configPath,
	}, &out, &errOut, "test-version")
	if code != 0 {
		t.Fatalf("service install dry-run failed with code %d: %s", code, errOut.String())
	}
	output := out.String()
	for _, want := range []string{"service_action: install", "platform: linux", "[Unit]", "ExecStart="} {
		if !strings.Contains(output, want) {
			t.Fatalf("dry-run output missing %q:\n%s", want, output)
		}
	}
}

func TestRunServiceCommandUsesPlatformRunner(t *testing.T) {
	configPath := writeTestServiceConfig(t)
	var gotName string
	var gotArgs []string
	err := RunServiceCommand(context.Background(), ServiceOptions{
		Command:    "status",
		ConfigPath: configPath,
		BinaryPath: "/usr/local/bin/data-proxy-agent",
		Platform:   "linux",
		Scope:      "user",
		CommandExec: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			gotName = name
			gotArgs = append([]string(nil), args...)
			return []byte("ok\n"), nil
		},
		Out: io.Discard,
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotName != "systemctl" {
		t.Fatalf("unexpected command name: %s", gotName)
	}
	got := strings.Join(gotArgs, " ")
	if got != "--user status data-proxy-agent.service" {
		t.Fatalf("unexpected command args: %s", got)
	}
}

func TestConfigPathEnvOverride(t *testing.T) {
	t.Setenv("DATA_PROXY_AGENT_CONFIG", "/tmp/custom-agent.yaml")
	got, err := ConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	if got != "/tmp/custom-agent.yaml" {
		t.Fatalf("unexpected config path: %s", got)
	}
}

func writeTestServiceConfig(t *testing.T) string {
	t.Helper()
	configPath := t.TempDir() + "/config.yaml"
	cfg := DefaultConfig()
	cfg.Server.BaseURL = "https://dp.example.com"
	cfg.Agent.Token = "sk-service-test-token"
	if err := SaveConfig(configPath, cfg); err != nil {
		t.Fatal(err)
	}
	return configPath
}

func serverURLFromRequest(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

func readZipEntries(t *testing.T, files []*zip.File) string {
	t.Helper()
	var builder strings.Builder
	for _, file := range files {
		builder.WriteString(file.Name)
		builder.WriteString("\n")
		reader, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(reader)
		_ = reader.Close()
		if err != nil {
			t.Fatal(err)
		}
		builder.Write(body)
		builder.WriteString("\n")
	}
	return builder.String()
}

func TestLoadConfigMissingAppliesEnvironment(t *testing.T) {
	t.Setenv("DATA_PROXY_BASE_URL", "https://dp.example.com")
	t.Setenv("DATA_PROXY_API_KEY", "env-token")
	cfg, loaded, err := LoadConfig(t.TempDir() + "/missing.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if loaded {
		t.Fatal("missing config should not be marked loaded")
	}
	if cfg.Server.BaseURL != "https://dp.example.com" {
		t.Fatalf("env base url not applied: %s", cfg.Server.BaseURL)
	}
	if ResolveToken(cfg) != "env-token" {
		t.Fatal("env token not applied")
	}
}

func TestSaveConfigUsesPrivateMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file mode semantics differ on windows")
	}
	configPath := t.TempDir() + "/config.yaml"
	if err := SaveConfig(configPath, DefaultConfig()); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if mode := info.Mode().Perm(); mode != DefaultConfigFileMode {
		t.Fatalf("unexpected config mode: %o", mode)
	}
}
