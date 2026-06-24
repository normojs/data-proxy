package dpagent

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/dto"
)

func TestHandleMCPProxyTest(t *testing.T) {
	mcp := newFakeMCPServer(t)
	defer mcp.Close()

	cfg := DefaultConfig()
	cfg.Runtime.HTTPTimeoutMS = 5000
	client := BridgeClient{Config: cfg}
	result, err := client.handleMCPProxy(context.Background(), BridgeToolMCPProxyTest, map[string]any{
		"target": mcp.URL,
		"server": map[string]any{"name": "fallback-name"},
	})
	if err != nil {
		t.Fatal(err)
	}
	payload := mapFromAny(result.Metadata["result"])
	if payload["server_name"] != "fake-mcp" {
		t.Fatalf("unexpected payload: %#v", payload)
	}
	if payload["protocol_version"] != dto.MCPProtocolVersion {
		t.Fatalf("unexpected protocol version: %#v", payload)
	}
}

func TestHandleMCPProxyListTools(t *testing.T) {
	mcp := newFakeMCPServer(t)
	defer mcp.Close()

	cfg := DefaultConfig()
	cfg.Runtime.HTTPTimeoutMS = 5000
	client := BridgeClient{Config: cfg}
	result, err := client.handleMCPProxy(context.Background(), BridgeToolMCPProxyListTools, map[string]any{
		"endpoint": "bridge://test-agent?target=" + mcp.URL,
	})
	if err != nil {
		t.Fatal(err)
	}
	payload := mapFromAny(result.Metadata["result"])
	tools, ok := payload["tools"].([]any)
	if !ok || len(tools) != 1 {
		t.Fatalf("unexpected tools payload: %#v", payload)
	}
	tool := mapFromAny(tools[0])
	if tool["name"] != "echo" {
		t.Fatalf("unexpected tool: %#v", tool)
	}
}

func TestHandleMCPProxyCallTool(t *testing.T) {
	mcp := newFakeMCPServer(t)
	defer mcp.Close()

	cfg := DefaultConfig()
	cfg.Runtime.HTTPTimeoutMS = 5000
	client := BridgeClient{Config: cfg}
	result, err := client.handleMCPProxy(context.Background(), BridgeToolMCPProxyCallTool, map[string]any{
		"target":    mcp.URL,
		"name":      "echo",
		"arguments": map[string]any{"message": "hello"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Content) != 1 || result.Content[0].Text != "echo: hello" {
		t.Fatalf("unexpected content: %#v", result.Content)
	}
	if result.Metadata["target"] != mcp.URL || result.Metadata["tool_name"] != "echo" {
		t.Fatalf("unexpected metadata: %#v", result.Metadata)
	}
}

func TestHandleMCPProxyRPC(t *testing.T) {
	mcp := newFakeMCPServer(t)
	defer mcp.Close()

	cfg := DefaultConfig()
	cfg.Runtime.HTTPTimeoutMS = 5000
	client := BridgeClient{Config: cfg}
	result, err := client.handleMCPProxy(context.Background(), BridgeToolMCPProxyRPC, map[string]any{
		"target": mcp.URL,
		"method": "resources/list",
		"params": map[string]any{},
	})
	if err != nil {
		t.Fatal(err)
	}
	payload := mapFromAny(result.Metadata["result"])
	resources, ok := payload["resources"].([]any)
	if !ok || len(resources) != 1 {
		t.Fatalf("unexpected rpc result: %#v", payload)
	}
	if result.Summary != "resources/list forwarded" {
		t.Fatalf("unexpected summary: %s", result.Summary)
	}
}

func TestHandleMCPProxyRejectsNonLoopbackByDefault(t *testing.T) {
	cfg := DefaultConfig()
	client := BridgeClient{Config: cfg}
	_, err := client.handleMCPProxy(context.Background(), BridgeToolMCPProxyTest, map[string]any{
		"target": "https://example.com/mcp",
	})
	if err == nil {
		t.Fatal("expected non-loopback MCP target to be rejected")
	}
	toolErr, ok := err.(ToolError)
	if !ok {
		t.Fatalf("expected ToolError, got %T", err)
	}
	if toolErr.Code != "MCP_PROXY_FORBIDDEN_TARGET" {
		t.Fatalf("unexpected error: %#v", toolErr)
	}
}

func TestHandleMCPProxyStdioTestListAndCall(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Runtime.HTTPTimeoutMS = 5000
	cfg.MCPServers = []MCPServer{{
		Name:      "coding",
		Transport: "stdio",
		Command:   fakeStdioMCPCommand(),
	}}
	defer defaultMCPStdioSessions.Forget("stdio:coding")

	client := BridgeClient{Config: cfg}
	testResult, err := client.handleMCPProxy(context.Background(), BridgeToolMCPProxyTest, map[string]any{
		"transport": "bridge",
		"endpoint":  "bridge://client-1",
		"server":    map[string]any{"name": "coding"},
	})
	if err != nil {
		t.Fatal(err)
	}
	testPayload := mapFromAny(testResult.Metadata["result"])
	if testPayload["server_name"] != "fake-stdio-mcp" {
		t.Fatalf("unexpected stdio test payload: %#v", testPayload)
	}

	listResult, err := client.handleMCPProxy(context.Background(), BridgeToolMCPProxyListTools, map[string]any{
		"transport": "bridge",
		"endpoint":  "bridge://client-1",
		"server":    map[string]any{"name": "coding"},
	})
	if err != nil {
		t.Fatal(err)
	}
	listPayload := mapFromAny(listResult.Metadata["result"])
	tools, ok := listPayload["tools"].([]any)
	if !ok || len(tools) != 1 {
		t.Fatalf("unexpected stdio tools payload: %#v", listPayload)
	}
	if mapFromAny(tools[0])["name"] != "echo" {
		t.Fatalf("unexpected stdio tool: %#v", tools[0])
	}

	callResult, err := client.handleMCPProxy(context.Background(), BridgeToolMCPProxyCallTool, map[string]any{
		"transport": "bridge",
		"endpoint":  "bridge://client-1",
		"server":    map[string]any{"name": "coding"},
		"name":      "echo",
		"arguments": map[string]any{"message": "hello stdio"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(callResult.Content) != 1 || callResult.Content[0].Text != "stdio echo: hello stdio" {
		t.Fatalf("unexpected stdio content: %#v", callResult.Content)
	}
	if callResult.Metadata["transport"] != "stdio" || callResult.Metadata["target"] != "stdio:coding" {
		t.Fatalf("unexpected stdio metadata: %#v", callResult.Metadata)
	}
}

func TestHandleMCPProxyStdioRejectsUnconfiguredCommand(t *testing.T) {
	cfg := DefaultConfig()
	client := BridgeClient{Config: cfg}
	_, err := client.handleMCPProxy(context.Background(), BridgeToolMCPProxyListTools, map[string]any{
		"transport": "stdio",
		"endpoint":  "bridge://client-1",
		"server":    map[string]any{"name": "remote-command"},
		"command":   fakeStdioMCPCommand(),
	})
	if err == nil {
		t.Fatal("expected unconfigured stdio MCP server to be rejected")
	}
	toolErr, ok := err.(ToolError)
	if !ok {
		t.Fatalf("expected ToolError, got %T", err)
	}
	if toolErr.Code != "MCP_PROXY_STDIO_NOT_CONFIGURED" {
		t.Fatalf("unexpected error: %#v", toolErr)
	}
}

func TestParseMCPSseResponse(t *testing.T) {
	object, err := parseMCPResponseText("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"ok\":true}}\n\n")
	if err != nil {
		t.Fatal(err)
	}
	result := mapFromAny(object["result"])
	if result["ok"] != true {
		t.Fatalf("unexpected SSE result: %#v", object)
	}
}

func TestEffectiveCapabilitiesAddsMCPProxyForServers(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MCPServers = []MCPServer{{Name: "coding", Transport: "streamable_http", Endpoint: "http://127.0.0.1:30837/mcp"}}
	capabilities := strings.Join(EffectiveCapabilities(cfg), ",")
	if !strings.Contains(capabilities, BridgeCapabilityMCPProxy) {
		t.Fatalf("mcp_proxy capability missing: %s", capabilities)
	}
}

func fakeStdioMCPCommand() string {
	executable := os.Args[0]
	if runtime.GOOS == "windows" {
		return fmt.Sprintf("set DPAGENT_FAKE_STDIO_MCP=1&& %s -test.run=TestFakeStdioMCP --", windowsCommandQuote(executable))
	}
	return fmt.Sprintf("DPAGENT_FAKE_STDIO_MCP=1 %s -test.run=TestFakeStdioMCP --", strconv.Quote(executable))
}

func windowsCommandQuote(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `\"`) + `"`
}

func TestFakeStdioMCP(t *testing.T) {
	if os.Getenv("DPAGENT_FAKE_STDIO_MCP") != "1" {
		return
	}
	reader := bufio.NewReader(os.Stdin)
	writer := bufio.NewWriter(os.Stdout)
	for {
		body, err := readMCPStdioFrame(reader, DefaultMaxResultBytes)
		if err != nil {
			os.Exit(0)
		}
		var request map[string]any
		if err := json.Unmarshal(body, &request); err != nil {
			os.Exit(1)
		}
		id, hasID := request["id"]
		if !hasID {
			continue
		}
		response := fakeStdioMCPResponse(id, request)
		responseBytes, err := json.Marshal(response)
		if err != nil {
			os.Exit(1)
		}
		if err := writeMCPStdioFrame(writer, responseBytes); err != nil {
			os.Exit(1)
		}
	}
}

func fakeStdioMCPResponse(id any, request map[string]any) map[string]any {
	switch request["method"] {
	case "initialize":
		return map[string]any{
			"jsonrpc": "2.0",
			"id":      id,
			"result": map[string]any{
				"protocolVersion": dto.MCPProtocolVersion,
				"capabilities":    map[string]any{"tools": map[string]any{}},
				"serverInfo":      map[string]any{"name": "fake-stdio-mcp", "version": "1.0.0"},
			},
		}
	case "ping":
		return map[string]any{"jsonrpc": "2.0", "id": id, "result": map[string]any{}}
	case "tools/list":
		return map[string]any{
			"jsonrpc": "2.0",
			"id":      id,
			"result": map[string]any{
				"tools": []map[string]any{{
					"name":        "echo",
					"description": "Echo input",
					"inputSchema": map[string]any{"type": "object"},
				}},
			},
		}
	case "tools/call":
		params := mapFromAny(request["params"])
		arguments := mapFromAny(params["arguments"])
		return map[string]any{
			"jsonrpc": "2.0",
			"id":      id,
			"result": map[string]any{
				"content": []map[string]any{{"type": "text", "text": "stdio echo: " + fmt.Sprint(arguments["message"])}},
				"metadata": map[string]any{
					"ok": true,
				},
			},
		}
	default:
		return map[string]any{
			"jsonrpc": "2.0",
			"id":      id,
			"error": map[string]any{
				"code":    -32601,
				"message": "method not found",
			},
		}
	}
}

func newFakeMCPServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.Header.Get("MCP-Protocol-Version") != dto.MCPProtocolVersion {
			t.Fatalf("missing MCP protocol version header: %#v", r.Header)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		method := body["method"]
		switch method {
		case "initialize":
			w.Header().Set("Mcp-Session-Id", "mcp-session-1")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      body["id"],
				"result": map[string]any{
					"protocolVersion": dto.MCPProtocolVersion,
					"capabilities":    map[string]any{"tools": true},
					"serverInfo":      map[string]any{"name": "fake-mcp", "version": "1.0.0"},
				},
			})
		case "notifications/initialized":
			if r.Header.Get("Mcp-Session-Id") != "mcp-session-1" {
				t.Fatalf("missing initialized session header: %#v", r.Header)
			}
			w.WriteHeader(http.StatusAccepted)
		case "ping":
			if r.Header.Get("Mcp-Session-Id") != "mcp-session-1" {
				t.Fatalf("missing ping session header: %#v", r.Header)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      body["id"],
				"result":  map[string]any{},
			})
		case "tools/list":
			if r.Header.Get("Mcp-Session-Id") != "mcp-session-1" {
				t.Fatalf("missing tools/list session header: %#v", r.Header)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      body["id"],
				"result": map[string]any{
					"tools": []map[string]any{{
						"name":        "echo",
						"description": "Echo input",
						"inputSchema": map[string]any{"type": "object"},
					}},
				},
			})
		case "tools/call":
			if r.Header.Get("Mcp-Session-Id") != "mcp-session-1" {
				t.Fatalf("missing tools/call session header: %#v", r.Header)
			}
			params := mapFromAny(body["params"])
			if params["name"] != "echo" {
				t.Fatalf("unexpected tool name: %#v", params)
			}
			arguments := mapFromAny(params["arguments"])
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      body["id"],
				"result": map[string]any{
					"content": []map[string]any{{"type": "text", "text": "echo: " + arguments["message"].(string)}},
					"metadata": map[string]any{
						"ok": true,
					},
				},
			})
		case "resources/list":
			if r.Header.Get("Mcp-Session-Id") != "mcp-session-1" {
				t.Fatalf("missing resources/list session header: %#v", r.Header)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      body["id"],
				"result": map[string]any{
					"resources": []map[string]any{{
						"uri":  "file:///tmp/example.txt",
						"name": "example.txt",
					}},
				},
			})
		default:
			t.Fatalf("unexpected MCP method: %v", method)
		}
	}))
}
