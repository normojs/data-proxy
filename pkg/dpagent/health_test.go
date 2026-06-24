package dpagent

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCLIDoctorPrintsLocalHealth(t *testing.T) {
	local := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer local.Close()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	cfg := DefaultConfig()
	cfg.Server.BaseURL = local.URL
	cfg.Agent.Token = "sk-doctor-test"
	cfg.Agent.Workspace = t.TempDir()
	cfg.Logging.LocalAuditJSONL = filepath.Join(t.TempDir(), "audit.jsonl")
	cfg.HTTPRoutes = []HTTPRoute{{Name: "local-web", Target: local.URL}}
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
		"workspace: ok:",
		"local_audit: ok:",
		"http_route.local-web: ok:",
		"doctor: ok",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("doctor output missing %q:\n%s", want, output)
		}
	}
}

func TestAgentLocalHealthChecksRoutesAndMCP(t *testing.T) {
	local := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer local.Close()

	cfg := DefaultConfig()
	cfg.Agent.Workspace = t.TempDir()
	cfg.Logging.LocalAuditJSONL = filepath.Join(t.TempDir(), "audit.jsonl")
	cfg.HTTPRoutes = []HTTPRoute{{Name: "local-web", Target: local.URL}}
	cfg.MCPServers = []MCPServer{{Name: "coding", Transport: "streamable_http", Endpoint: local.URL + "/mcp"}}

	checks := AgentLocalHealthChecks(cfg, 2*time.Second)
	if statusForHealthCheck(checks, "workspace") != HealthStatusOK {
		t.Fatalf("workspace should be ok: %#v", checks)
	}
	if statusForHealthCheck(checks, "http_route.local-web") != HealthStatusOK {
		t.Fatalf("http route should be ok: %#v", checks)
	}
	if statusForHealthCheck(checks, "mcp_server.coding") != HealthStatusOK {
		t.Fatalf("mcp server should be ok: %#v", checks)
	}
}

func TestAgentLocalHealthChecksRejectForbiddenNonLoopback(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Agent.Workspace = t.TempDir()
	cfg.HTTPRoutes = []HTTPRoute{{Name: "public", Target: "http://example.com"}}
	cfg.MCPServers = []MCPServer{{Name: "remote-mcp", Transport: "streamable_http", Endpoint: "https://example.com/mcp"}}

	checks := AgentLocalHealthChecks(cfg, 10*time.Millisecond)
	if statusForHealthCheck(checks, "http_route.public") != HealthStatusFail {
		t.Fatalf("public http route should fail: %#v", checks)
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

func statusForHealthCheck(checks []AgentHealthCheck, name string) string {
	for _, check := range checks {
		if check.Name == name {
			return check.Status
		}
	}
	return ""
}
