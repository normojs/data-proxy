package dpagent

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strings"
	"testing"

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

func TestCLIReportWritesRedactedDiagnosticZip(t *testing.T) {
	tmp := t.TempDir()
	configPath := tmp + "/config.yaml"
	reportPath := tmp + "/report.zip"
	cfg := DefaultConfig()
	cfg.Server.BaseURL = "https://dp.example.com"
	cfg.Agent.ClientID = "report-agent"
	cfg.Agent.Token = "sk-report-token-secret"
	cfg.MCPServers = []MCPServer{{Name: "coding", Transport: "streamable_http", Endpoint: "http://127.0.0.1:30837/mcp"}}
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
