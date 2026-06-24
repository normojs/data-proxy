package dpagent

import (
	"bytes"
	"os"
	"runtime"
	"strings"
	"testing"
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
