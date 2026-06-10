package proxy

import (
	"context"
	"errors"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
)

var ErrClientNotConfigured = errors.New("mcp proxy client is not configured")

type ToolDefinition struct {
	Name        string
	Title       string
	Description string
	InputSchema map[string]any
}

type TestResult struct {
	ProtocolVersion string
	ServerName      string
	Capabilities    map[string]any
}

type CallRequest struct {
	ToolName  string
	Arguments map[string]any
	RequestId string
	UserId    int
	TokenId   int
}

type CallResult struct {
	Content         []dto.MCPContentBlock
	Metadata        map[string]any
	Summary         string
	DurationMS      int
	ResultSize      int
	BridgeSessionId string
	TargetClient    string
}

type SessionSnapshot struct {
	Transport         string
	HasSession        bool
	Initialized       bool
	MessageEndpoint   string
	LastError         string
	StreamableSession bool
	SSEConnected      bool
	ActiveSessions    int
	PendingRequests   int
	LastActivityAt    int64
}

type Client interface {
	Test(ctx context.Context, server model.MCPProxyServer) (TestResult, error)
	ListTools(ctx context.Context, server model.MCPProxyServer) ([]ToolDefinition, error)
	CallTool(ctx context.Context, server model.MCPProxyServer, req CallRequest) (CallResult, error)
}

type UnconfiguredClient struct{}

func (UnconfiguredClient) Test(ctx context.Context, server model.MCPProxyServer) (TestResult, error) {
	return TestResult{}, ErrClientNotConfigured
}

func (UnconfiguredClient) ListTools(ctx context.Context, server model.MCPProxyServer) ([]ToolDefinition, error) {
	return nil, ErrClientNotConfigured
}

func (UnconfiguredClient) CallTool(ctx context.Context, server model.MCPProxyServer, req CallRequest) (CallResult, error) {
	return CallResult{}, ErrClientNotConfigured
}
