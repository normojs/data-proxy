package executor

import (
	"context"
	"errors"
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
	if !errors.Is(err, ErrExecutorNotImplemented) {
		t.Fatalf("expected ErrExecutorNotImplemented, got %v", err)
	}
	if ErrorCode(err) != ErrorCodeNotImplemented {
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
