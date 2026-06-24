package proxy

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/model"
)

type CompositeClient struct {
	HTTP   Client
	Bridge Client
}

func NewDefaultClient(httpClient *http.Client) *CompositeClient {
	return &CompositeClient{
		HTTP:   NewHTTPClient(httpClient),
		Bridge: NewBridgeClient(nil),
	}
}

func (c *CompositeClient) Test(ctx context.Context, server model.MCPProxyServer) (TestResult, error) {
	return c.clientForTransport(server.Transport).Test(ctx, server)
}

func (c *CompositeClient) ListTools(ctx context.Context, server model.MCPProxyServer) ([]ToolDefinition, error) {
	return c.clientForTransport(server.Transport).ListTools(ctx, server)
}

func (c *CompositeClient) ListToolsForUser(ctx context.Context, server model.MCPProxyServer, userId int) ([]ToolDefinition, error) {
	client := c.clientForTransport(server.Transport)
	if scoped, ok := client.(UserScopedListToolsClient); ok {
		return scoped.ListToolsForUser(ctx, server, userId)
	}
	if userId > 0 {
		return nil, ErrClientNotConfigured
	}
	return client.ListTools(ctx, server)
}

func (c *CompositeClient) CallTool(ctx context.Context, server model.MCPProxyServer, req CallRequest) (CallResult, error) {
	return c.clientForTransport(server.Transport).CallTool(ctx, server, req)
}

func (c *CompositeClient) CallRaw(ctx context.Context, server model.MCPProxyServer, req RawRequest) (RawResult, error) {
	client := c.clientForTransport(server.Transport)
	if rawCaller, ok := client.(RawCaller); ok {
		return rawCaller.CallRaw(ctx, server, req)
	}
	return RawResult{}, ErrClientNotConfigured
}

func (c *CompositeClient) SessionSnapshot(server model.MCPProxyServer) SessionSnapshot {
	client := c.clientForTransport(server.Transport)
	if provider, ok := client.(interface {
		SessionSnapshot(server model.MCPProxyServer) SessionSnapshot
	}); ok {
		return provider.SessionSnapshot(server)
	}
	return SessionSnapshot{Transport: strings.TrimSpace(server.Transport)}
}

func (c *CompositeClient) CloseIdleSessions(idleTimeout time.Duration) int {
	if c == nil {
		return 0
	}
	closed := 0
	for _, client := range []Client{c.HTTP, c.Bridge} {
		if closer, ok := client.(interface {
			CloseIdleSessions(idleTimeout time.Duration) int
		}); ok && closer != nil {
			closed += closer.CloseIdleSessions(idleTimeout)
		}
	}
	return closed
}

func (c *CompositeClient) clientForTransport(transport string) Client {
	if c == nil {
		return UnconfiguredClient{}
	}
	switch strings.TrimSpace(transport) {
	case model.MCPProxyTransportBridge, model.MCPProxyTransportQidianBrowser:
		if c.Bridge != nil {
			return c.Bridge
		}
	default:
		if c.HTTP != nil {
			return c.HTTP
		}
	}
	return UnconfiguredClient{}
}
