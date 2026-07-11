package executor

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/bridge"
)

type fakeExecutor struct {
	supported bool
	result    Result
	err       error
}

func (f fakeExecutor) Supports(tool model.MCPTool) bool {
	return f.supported
}

func (f fakeExecutor) Execute(ctx context.Context, req Request) (Result, error) {
	return f.result, f.err
}

func TestRegistryFallsBackToNoopExecutor(t *testing.T) {
	registry := NewRegistry()
	resolved := registry.Resolve(model.MCPTool{Name: "remote_read"})
	result, err := resolved.Execute(context.Background(), Request{})
	if err == nil {
		t.Fatal("expected noop executor error")
	}
	if !errors.Is(err, ErrExecutorUnsupported) {
		t.Fatalf("expected ErrExecutorUnsupported, got %v", err)
	}
	if ErrorCode(err) != ErrorCodeUnsupported {
		t.Fatalf("error code mismatch, got %s", ErrorCode(err))
	}
	if len(result.Content) != 0 {
		t.Fatalf("expected empty result, got %#v", result)
	}
}

func TestRegistryUsesSupportingExecutor(t *testing.T) {
	expected := Result{
		Content: []dto.MCPContentBlock{{Type: "text", Text: "ok"}},
	}
	registry := NewRegistry(
		fakeExecutor{supported: false},
		fakeExecutor{supported: true, result: expected},
	)

	resolved := registry.Resolve(model.MCPTool{Name: "remote_read"})
	result, err := resolved.Execute(context.Background(), Request{})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if len(result.Content) != 1 || result.Content[0].Text != "ok" {
		t.Fatalf("result mismatch, got %#v", result)
	}
}

func TestBuiltinExecutorRejectsUnsupportedBuiltinTool(t *testing.T) {
	executor := NewBuiltinExecutor()
	_, err := executor.Execute(context.Background(), Request{
		Tool: model.MCPTool{
			Name:     "unknown_builtin",
			Source:   model.MCPToolSourceBuiltin,
			IsRemote: false,
		},
	})
	if err == nil {
		t.Fatal("expected unsupported builtin tool error")
	}
	if !errors.Is(err, ErrExecutorUnsupported) {
		t.Fatalf("expected ErrExecutorUnsupported, got %v", err)
	}
	if ErrorCode(err) != ErrorCodeUnsupported {
		t.Fatalf("error code mismatch, got %s", ErrorCode(err))
	}
}

func TestRemoteBridgeExecutorSupportsRemoteTools(t *testing.T) {
	executor := NewRemoteBridgeExecutor(bridge.NewHub())
	if !executor.Supports(model.MCPTool{IsRemote: true}) {
		t.Fatal("expected remote bridge executor to support remote tool")
	}
	if executor.Supports(model.MCPTool{IsRemote: false}) {
		t.Fatal("did not expect remote bridge executor to support local tool")
	}
}

func TestRemoteBridgeExecutorReadsTimeoutEnv(t *testing.T) {
	t.Setenv("MCP_REMOTE_BRIDGE_TIMEOUT_MS", "1234")
	executor := NewRemoteBridgeExecutor(bridge.NewHub())
	if executor.Timeout != 1234*time.Millisecond {
		t.Fatalf("timeout mismatch, got %s", executor.Timeout)
	}
}

func TestBuiltinExecutorServerTime(t *testing.T) {
	executor := NewBuiltinExecutor()
	tool := model.MCPTool{
		Name:     BuiltinToolServerTime,
		Source:   model.MCPToolSourceBuiltin,
		IsRemote: false,
	}
	if !executor.Supports(tool) {
		t.Fatal("expected builtin executor to support server_time")
	}
	result, err := executor.Execute(context.Background(), Request{
		Tool:      tool,
		Arguments: map[string]any{"timezone": "UTC"},
	})
	if err != nil {
		t.Fatalf("execute server_time failed: %v", err)
	}
	if len(result.Content) != 1 || result.Content[0].Text == "" {
		t.Fatalf("server_time content mismatch: %#v", result.Content)
	}
	if result.Metadata["executor"] != "builtin" || result.Metadata["timezone"] != "UTC" {
		t.Fatalf("server_time metadata mismatch: %#v", result.Metadata)
	}
}

func TestBuiltinExecutorJSONPretty(t *testing.T) {
	executor := NewBuiltinExecutor()
	tool := model.MCPTool{
		Name:     BuiltinToolJSONPretty,
		Source:   model.MCPToolSourceBuiltin,
		IsRemote: false,
	}
	result, err := executor.Execute(context.Background(), Request{
		Tool:      tool,
		Arguments: map[string]any{"json": `{"b":2,"a":1}`, "indent": 2},
	})
	if err != nil {
		t.Fatalf("execute json_pretty failed: %v", err)
	}
	if len(result.Content) != 1 || result.Content[0].Text != "{\n  \"a\": 1,\n  \"b\": 2\n}" {
		t.Fatalf("json_pretty content mismatch: %#v", result.Content)
	}
}

func TestBuiltinExecutorRejectsInvalidJSON(t *testing.T) {
	executor := NewBuiltinExecutor()
	_, err := executor.Execute(context.Background(), Request{
		Tool: model.MCPTool{
			Name:     BuiltinToolJSONPretty,
			Source:   model.MCPToolSourceBuiltin,
			IsRemote: false,
		},
		Arguments: map[string]any{"json": `{bad`},
	})
	if err == nil {
		t.Fatal("expected invalid json error")
	}
	if ErrorCode(err) != ErrorCodeFailed {
		t.Fatalf("error code mismatch, got %s", ErrorCode(err))
	}
}

func TestBuiltinExecutorJSONQuery(t *testing.T) {
	executor := NewBuiltinExecutor()
	tool := model.MCPTool{
		Name:     BuiltinToolJSONQuery,
		Source:   model.MCPToolSourceBuiltin,
		IsRemote: false,
	}
	if !executor.Supports(tool) {
		t.Fatal("expected builtin executor to support json_query")
	}
	result, err := executor.Execute(context.Background(), Request{
		Tool:      tool,
		Arguments: map[string]any{"json": `{"items":[{"name":"alpha"},{"name":"beta"}]}`, "pointer": "/items/1/name"},
	})
	if err != nil {
		t.Fatalf("execute json_query failed: %v", err)
	}
	if len(result.Content) != 1 ||
		!strings.Contains(result.Content[0].Text, `"exists": true`) ||
		!strings.Contains(result.Content[0].Text, `"value": "beta"`) {
		t.Fatalf("json_query content mismatch: %#v", result.Content)
	}
	if result.Metadata["executor"] != "builtin" || result.Metadata["tool"] != BuiltinToolJSONQuery || result.Metadata["exists"] != true {
		t.Fatalf("json_query metadata mismatch: %#v", result.Metadata)
	}
	if result.Summary != "json pointer /items/1/name (string)" {
		t.Fatalf("json_query summary mismatch: %s", result.Summary)
	}
}

func TestBuiltinExecutorJSONQueryMissingPath(t *testing.T) {
	executor := NewBuiltinExecutor()
	result, err := executor.Execute(context.Background(), Request{
		Tool: model.MCPTool{
			Name:     BuiltinToolJSONQuery,
			Source:   model.MCPToolSourceBuiltin,
			IsRemote: false,
		},
		Arguments: map[string]any{"json": `{"items":[]}`, "pointer": "/items/0/name"},
	})
	if err != nil {
		t.Fatalf("execute json_query missing path failed: %v", err)
	}
	if len(result.Content) != 1 ||
		!strings.Contains(result.Content[0].Text, `"exists": false`) ||
		!strings.Contains(result.Content[0].Text, `"type": "missing"`) {
		t.Fatalf("json_query missing path content mismatch: %#v", result.Content)
	}
}

func TestBuiltinExecutorJSONQueryRejectsInvalidPointer(t *testing.T) {
	executor := NewBuiltinExecutor()
	_, err := executor.Execute(context.Background(), Request{
		Tool: model.MCPTool{
			Name:     BuiltinToolJSONQuery,
			Source:   model.MCPToolSourceBuiltin,
			IsRemote: false,
		},
		Arguments: map[string]any{"json": `{"items":[]}`, "pointer": "items/0"},
	})
	if err == nil {
		t.Fatal("expected invalid json pointer error")
	}
	if ErrorCode(err) != ErrorCodeFailed {
		t.Fatalf("error code mismatch, got %s", ErrorCode(err))
	}
}

func TestBuiltinExecutorTextHash(t *testing.T) {
	executor := NewBuiltinExecutor()
	tool := model.MCPTool{
		Name:     BuiltinToolTextHash,
		Source:   model.MCPToolSourceBuiltin,
		IsRemote: false,
	}
	if !executor.Supports(tool) {
		t.Fatal("expected builtin executor to support text_hash")
	}
	result, err := executor.Execute(context.Background(), Request{
		Tool:      tool,
		Arguments: map[string]any{"text": "hello", "algorithm": "sha-256"},
	})
	if err != nil {
		t.Fatalf("execute text_hash failed: %v", err)
	}
	if len(result.Content) != 1 || !strings.Contains(result.Content[0].Text, "2cf24dba5fb0a30e26e83b2ac5b9e29e") {
		t.Fatalf("text_hash content mismatch: %#v", result.Content)
	}
	if result.Metadata["executor"] != "builtin" || result.Metadata["algorithm"] != "sha256" {
		t.Fatalf("text_hash metadata mismatch: %#v", result.Metadata)
	}
}

func TestBuiltinExecutorTextHashRejectsUnsupportedAlgorithm(t *testing.T) {
	executor := NewBuiltinExecutor()
	_, err := executor.Execute(context.Background(), Request{
		Tool: model.MCPTool{
			Name:     BuiltinToolTextHash,
			Source:   model.MCPToolSourceBuiltin,
			IsRemote: false,
		},
		Arguments: map[string]any{"text": "hello", "algorithm": "crc32"},
	})
	if err == nil {
		t.Fatal("expected unsupported hash algorithm error")
	}
	if ErrorCode(err) != ErrorCodeFailed {
		t.Fatalf("error code mismatch, got %s", ErrorCode(err))
	}
}

func TestBuiltinExecutorTextStats(t *testing.T) {
	executor := NewBuiltinExecutor()
	tool := model.MCPTool{
		Name:     BuiltinToolTextStats,
		Source:   model.MCPToolSourceBuiltin,
		IsRemote: false,
	}
	if !executor.Supports(tool) {
		t.Fatal("expected builtin executor to support text_stats")
	}
	result, err := executor.Execute(context.Background(), Request{
		Tool:      tool,
		Arguments: map[string]any{"text": " hello 世界\nsecond line\n ", "trim": true},
	})
	if err != nil {
		t.Fatalf("execute text_stats failed: %v", err)
	}
	if len(result.Content) != 1 ||
		!strings.Contains(result.Content[0].Text, `"words": 4`) ||
		!strings.Contains(result.Content[0].Text, `"lines": 2`) {
		t.Fatalf("text_stats content mismatch: %#v", result.Content)
	}
	if result.Metadata["executor"] != "builtin" || result.Metadata["tool"] != BuiltinToolTextStats {
		t.Fatalf("text_stats metadata mismatch: %#v", result.Metadata)
	}
	if result.Summary != "text stats (4 words, 2 lines)" {
		t.Fatalf("text_stats summary mismatch: %s", result.Summary)
	}
}

func TestBuiltinExecutorTextStatsRequiresText(t *testing.T) {
	executor := NewBuiltinExecutor()
	_, err := executor.Execute(context.Background(), Request{
		Tool: model.MCPTool{
			Name:     BuiltinToolTextStats,
			Source:   model.MCPToolSourceBuiltin,
			IsRemote: false,
		},
		Arguments: map[string]any{},
	})
	if err == nil {
		t.Fatal("expected missing text error")
	}
	if ErrorCode(err) != ErrorCodeFailed {
		t.Fatalf("error code mismatch, got %s", ErrorCode(err))
	}
}

func TestBuiltinExecutorBase64CodecEncode(t *testing.T) {
	executor := NewBuiltinExecutor()
	tool := model.MCPTool{
		Name:     BuiltinToolBase64,
		Source:   model.MCPToolSourceBuiltin,
		IsRemote: false,
	}
	if !executor.Supports(tool) {
		t.Fatal("expected builtin executor to support base64_codec")
	}
	result, err := executor.Execute(context.Background(), Request{
		Tool:      tool,
		Arguments: map[string]any{"input": "hello"},
	})
	if err != nil {
		t.Fatalf("execute base64 encode failed: %v", err)
	}
	if len(result.Content) != 1 || !strings.Contains(result.Content[0].Text, `"base64": "aGVsbG8="`) {
		t.Fatalf("base64 encode content mismatch: %#v", result.Content)
	}
	if result.Metadata["executor"] != "builtin" || result.Metadata["operation"] != "encode" {
		t.Fatalf("base64 encode metadata mismatch: %#v", result.Metadata)
	}
}

func TestBuiltinExecutorBase64CodecDecode(t *testing.T) {
	executor := NewBuiltinExecutor()
	result, err := executor.Execute(context.Background(), Request{
		Tool: model.MCPTool{
			Name:     BuiltinToolBase64,
			Source:   model.MCPToolSourceBuiltin,
			IsRemote: false,
		},
		Arguments: map[string]any{"input": "aGVsbG8=", "operation": "decode"},
	})
	if err != nil {
		t.Fatalf("execute base64 decode failed: %v", err)
	}
	if len(result.Content) != 1 || !strings.Contains(result.Content[0].Text, `"text": "hello"`) {
		t.Fatalf("base64 decode content mismatch: %#v", result.Content)
	}
}

func TestBuiltinExecutorBase64CodecRejectsUnsupportedOperation(t *testing.T) {
	executor := NewBuiltinExecutor()
	_, err := executor.Execute(context.Background(), Request{
		Tool: model.MCPTool{
			Name:     BuiltinToolBase64,
			Source:   model.MCPToolSourceBuiltin,
			IsRemote: false,
		},
		Arguments: map[string]any{"input": "hello", "operation": "rotate"},
	})
	if err == nil {
		t.Fatal("expected unsupported base64 operation error")
	}
	if ErrorCode(err) != ErrorCodeFailed {
		t.Fatalf("error code mismatch, got %s", ErrorCode(err))
	}
}

func TestBuiltinExecutorURLCodecEncode(t *testing.T) {
	executor := NewBuiltinExecutor()
	tool := model.MCPTool{
		Name:     BuiltinToolURLCodec,
		Source:   model.MCPToolSourceBuiltin,
		IsRemote: false,
	}
	if !executor.Supports(tool) {
		t.Fatal("expected builtin executor to support url_codec")
	}
	result, err := executor.Execute(context.Background(), Request{
		Tool:      tool,
		Arguments: map[string]any{"input": "hello world+plus"},
	})
	if err != nil {
		t.Fatalf("execute url encode failed: %v", err)
	}
	if len(result.Content) != 1 || !strings.Contains(result.Content[0].Text, `"encoded": "hello+world%2Bplus"`) {
		t.Fatalf("url encode content mismatch: %#v", result.Content)
	}
	if result.Metadata["executor"] != "builtin" || result.Metadata["operation"] != "encode" || result.Metadata["mode"] != "query" {
		t.Fatalf("url encode metadata mismatch: %#v", result.Metadata)
	}
}

func TestBuiltinExecutorURLCodecDecodePath(t *testing.T) {
	executor := NewBuiltinExecutor()
	result, err := executor.Execute(context.Background(), Request{
		Tool: model.MCPTool{
			Name:     BuiltinToolURLCodec,
			Source:   model.MCPToolSourceBuiltin,
			IsRemote: false,
		},
		Arguments: map[string]any{"input": "hello%2Fworld", "operation": "decode", "mode": "path"},
	})
	if err != nil {
		t.Fatalf("execute url decode failed: %v", err)
	}
	if len(result.Content) != 1 || !strings.Contains(result.Content[0].Text, `"decoded": "hello/world"`) {
		t.Fatalf("url decode content mismatch: %#v", result.Content)
	}
}

func TestBuiltinExecutorURLCodecRejectsUnsupportedOperation(t *testing.T) {
	executor := NewBuiltinExecutor()
	_, err := executor.Execute(context.Background(), Request{
		Tool: model.MCPTool{
			Name:     BuiltinToolURLCodec,
			Source:   model.MCPToolSourceBuiltin,
			IsRemote: false,
		},
		Arguments: map[string]any{"input": "hello", "operation": "rotate"},
	})
	if err == nil {
		t.Fatal("expected unsupported url operation error")
	}
	if ErrorCode(err) != ErrorCodeFailed {
		t.Fatalf("error code mismatch, got %s", ErrorCode(err))
	}
}
