package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestHTTPClientInitializeListAndCall(t *testing.T) {
	var seenMethods []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		var req struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		seenMethods = append(seenMethods, req.Method)
		switch req.Method {
		case "initialize":
			writeJSONRPCResult(t, w, req.ID, map[string]any{
				"protocolVersion": "2025-06-18",
				"capabilities":    map[string]any{"tools": true},
				"serverInfo":      map[string]any{"name": "fake-mcp", "version": "1.0.0"},
			})
		case "notifications/initialized":
			w.WriteHeader(http.StatusAccepted)
		case "ping":
			writeJSONRPCResult(t, w, req.ID, map[string]any{})
		case "tools/list":
			writeJSONRPCResult(t, w, req.ID, map[string]any{
				"tools": []map[string]any{
					{
						"name":        "search_repos",
						"title":       "Search Repos",
						"description": "Search repositories",
						"inputSchema": map[string]any{"type": "object"},
					},
				},
			})
		case "tools/call":
			var params struct {
				Name      string         `json:"name"`
				Arguments map[string]any `json:"arguments"`
			}
			require.NoError(t, json.Unmarshal(req.Params, &params))
			require.Equal(t, "search_repos", params.Name)
			require.Equal(t, "data-proxy", params.Arguments["query"])
			writeJSONRPCResult(t, w, req.ID, map[string]any{
				"content": []map[string]any{{"type": "text", "text": "ok"}},
				"metadata": map[string]any{
					"downstream_request_id": "req-1",
				},
			})
		default:
			t.Fatalf("unexpected method %s", req.Method)
		}
	}))
	defer server.Close()

	t.Setenv("MCP_PROXY_HTTP_CLIENT_TOKEN", "test-token")
	client := NewHTTPClient(server.Client())
	proxyServer := model.MCPProxyServer{
		Endpoint: server.URL,
		AuthType: model.MCPProxyAuthTypeBearer,
		AuthRef:  "env:MCP_PROXY_HTTP_CLIENT_TOKEN",
	}

	testResult, err := client.Test(context.Background(), proxyServer)
	require.NoError(t, err)
	require.Equal(t, "2025-06-18", testResult.ProtocolVersion)
	require.Equal(t, "fake-mcp", testResult.ServerName)

	tools, err := client.ListTools(context.Background(), proxyServer)
	require.NoError(t, err)
	require.Len(t, tools, 1)
	require.Equal(t, "search_repos", tools[0].Name)
	require.Equal(t, "Search Repos", tools[0].Title)

	callResult, err := client.CallTool(context.Background(), proxyServer, CallRequest{
		ToolName:  "search_repos",
		Arguments: map[string]any{"query": "data-proxy"},
	})
	require.NoError(t, err)
	require.Equal(t, "ok", callResult.Content[0].Text)
	require.Equal(t, "req-1", callResult.Metadata["downstream_request_id"])
	require.Positive(t, callResult.ResultSize)
	require.Equal(t, []string{"initialize", "notifications/initialized", "ping", "tools/list", "tools/call"}, seenMethods)
}

func TestHTTPClientJSONRPCError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID json.RawMessage `json:"id"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		_ = json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0",
			"id":      req.ID,
			"error": map[string]any{
				"code":    -32000,
				"message": "downstream failed",
			},
		})
	}))
	defer server.Close()

	client := NewHTTPClient(server.Client())
	_, err := client.ListTools(context.Background(), model.MCPProxyServer{Endpoint: server.URL})
	require.Error(t, err)
	require.Contains(t, err.Error(), "downstream failed")
}

func TestHTTPClientListToolsRetriesRetryableHTTPStatus(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt := attempts.Add(1)
		var req struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		require.Equal(t, dto.MCPMethodToolsList, req.Method)
		if attempt == 1 {
			http.Error(w, "temporarily unavailable", http.StatusServiceUnavailable)
			return
		}
		writeJSONRPCResult(t, w, req.ID, map[string]any{
			"tools": []map[string]any{
				{
					"name":        "retry_tool",
					"description": "Retried tool",
					"inputSchema": map[string]any{"type": "object"},
				},
			},
		})
	}))
	defer server.Close()

	client := NewHTTPClient(server.Client())
	client.RetryMaxAttempts = 2
	client.RetryBaseDelay = time.Millisecond
	tools, err := client.ListTools(context.Background(), model.MCPProxyServer{
		Endpoint: server.URL,
	})
	require.NoError(t, err)
	require.Len(t, tools, 1)
	require.Equal(t, "retry_tool", tools[0].Name)
	require.EqualValues(t, 2, attempts.Load())
}

func TestApplyHTTPAuthUsesEnvSecretReferences(t *testing.T) {
	t.Setenv("MCP_PROXY_AUTH_BEARER", "bearer-token")
	t.Setenv("MCP_PROXY_AUTH_HEADER", "X-API-Key=header-token")
	t.Setenv("MCP_PROXY_AUTH_BASIC", "user:pass")

	bearerReq, err := http.NewRequest(http.MethodPost, "https://example.test/mcp", nil)
	require.NoError(t, err)
	require.NoError(t, applyHTTPAuth(bearerReq, model.MCPProxyServer{
		AuthType: model.MCPProxyAuthTypeBearer,
		AuthRef:  "env:MCP_PROXY_AUTH_BEARER",
	}))
	require.Equal(t, "Bearer bearer-token", bearerReq.Header.Get("Authorization"))

	headerReq, err := http.NewRequest(http.MethodPost, "https://example.test/mcp", nil)
	require.NoError(t, err)
	require.NoError(t, applyHTTPAuth(headerReq, model.MCPProxyServer{
		AuthType: model.MCPProxyAuthTypeHeader,
		AuthRef:  "env:MCP_PROXY_AUTH_HEADER",
	}))
	require.Equal(t, "header-token", headerReq.Header.Get("X-API-Key"))

	basicReq, err := http.NewRequest(http.MethodPost, "https://example.test/mcp", nil)
	require.NoError(t, err)
	require.NoError(t, applyHTTPAuth(basicReq, model.MCPProxyServer{
		AuthType: model.MCPProxyAuthTypeBasic,
		AuthRef:  "env:MCP_PROXY_AUTH_BASIC",
	}))
	username, password, ok := basicReq.BasicAuth()
	require.True(t, ok)
	require.Equal(t, "user", username)
	require.Equal(t, "pass", password)
}

func TestApplyHTTPAuthRejectsNonEnvSecretReference(t *testing.T) {
	for _, authRef := range []string{"raw:test-token", "env:", "env:MCP-PROXY-TOKEN"} {
		t.Run(authRef, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodPost, "https://example.test/mcp", nil)
			require.NoError(t, err)
			err = applyHTTPAuth(req, model.MCPProxyServer{
				AuthType: model.MCPProxyAuthTypeBearer,
				AuthRef:  authRef,
			})
			require.Error(t, err)
			require.Contains(t, err.Error(), "env:NAME")
		})
	}
}

func TestApplyHTTPAuthMissingEnvDoesNotLeakSecretReference(t *testing.T) {
	req, err := http.NewRequest(http.MethodPost, "https://example.test/mcp", nil)
	require.NoError(t, err)
	err = applyHTTPAuth(req, model.MCPProxyServer{
		AuthType: model.MCPProxyAuthTypeBearer,
		AuthRef:  "env:MCP_PROXY_MISSING_AUTH_TOKEN",
	})
	require.Error(t, err)
	require.NotContains(t, err.Error(), "MCP_PROXY_MISSING_AUTH_TOKEN")
	require.NotContains(t, err.Error(), "env:")
}

func TestHTTPClientStreamableHTTPHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Contains(t, r.Header.Get("Accept"), "text/event-stream")
		require.Equal(t, "2025-06-18", r.Header.Get("MCP-Protocol-Version"))
		var req struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		switch req.Method {
		case "initialize":
			writeJSONRPCResult(t, w, req.ID, map[string]any{
				"protocolVersion": "2025-06-18",
				"capabilities":    map[string]any{"tools": true},
				"serverInfo":      map[string]any{"name": "fake-mcp"},
			})
		case "notifications/initialized":
			w.WriteHeader(http.StatusNoContent)
		case "tools/list":
			writeJSONRPCResult(t, w, req.ID, map[string]any{
				"tools": []map[string]any{},
			})
		default:
			t.Fatalf("unexpected method %s", req.Method)
		}
	}))
	defer server.Close()

	client := NewHTTPClient(server.Client())
	tools, err := client.ListTools(context.Background(), model.MCPProxyServer{
		Endpoint:  server.URL,
		Transport: model.MCPProxyTransportStreamableHTTP,
	})
	require.NoError(t, err)
	require.Empty(t, tools)
}

func TestHTTPClientStreamableHTTPSessionHeader(t *testing.T) {
	var seenMethods []string
	var listSession string
	var initializedSession string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Contains(t, r.Header.Get("Accept"), "text/event-stream")
		require.Equal(t, "2025-06-18", r.Header.Get("MCP-Protocol-Version"))
		var req struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		seenMethods = append(seenMethods, req.Method)
		switch req.Method {
		case "initialize":
			require.Empty(t, r.Header.Get("Mcp-Session-Id"))
			w.Header().Set("Mcp-Session-Id", "session-1")
			writeJSONRPCResult(t, w, req.ID, map[string]any{
				"protocolVersion": "2025-06-18",
				"capabilities":    map[string]any{"tools": true},
				"serverInfo":      map[string]any{"name": "fake-mcp"},
			})
		case "notifications/initialized":
			initializedSession = r.Header.Get("Mcp-Session-Id")
			w.WriteHeader(http.StatusAccepted)
		case "tools/list":
			listSession = r.Header.Get("Mcp-Session-Id")
			writeJSONRPCResult(t, w, req.ID, map[string]any{
				"tools": []map[string]any{},
			})
		default:
			t.Fatalf("unexpected method %s", req.Method)
		}
	}))
	defer server.Close()

	client := NewHTTPClient(server.Client())
	tools, err := client.ListTools(context.Background(), model.MCPProxyServer{
		Endpoint:  server.URL,
		Transport: model.MCPProxyTransportStreamableHTTP,
	})
	require.NoError(t, err)
	require.Empty(t, tools)
	require.Equal(t, []string{"initialize", "notifications/initialized", "tools/list"}, seenMethods)
	require.Equal(t, "session-1", initializedSession)
	require.Equal(t, "session-1", listSession)

	snapshot := client.SessionSnapshot(model.MCPProxyServer{
		Endpoint:  server.URL,
		Transport: model.MCPProxyTransportStreamableHTTP,
	})
	require.True(t, snapshot.HasSession)
	require.True(t, snapshot.Initialized)
	require.True(t, snapshot.StreamableSession)
	require.Equal(t, 1, snapshot.ActiveSessions)
	require.Equal(t, 0, snapshot.PendingRequests)
	require.Positive(t, snapshot.LastActivityAt)
}

func TestHTTPClientStreamableHTTPSessionExpiredRetriesInitialize(t *testing.T) {
	var seenMethods []string
	var expiredOnce bool
	var finalListSession string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		seenMethods = append(seenMethods, req.Method)
		switch req.Method {
		case "initialize":
			if r.Header.Get("Mcp-Session-Id") == "" && !expiredOnce {
				w.Header().Set("Mcp-Session-Id", "expired-session")
			} else {
				w.Header().Set("Mcp-Session-Id", "fresh-session")
			}
			writeJSONRPCResult(t, w, req.ID, map[string]any{
				"protocolVersion": "2025-06-18",
				"capabilities":    map[string]any{"tools": true},
				"serverInfo":      map[string]any{"name": "fake-mcp"},
			})
		case "notifications/initialized":
			w.WriteHeader(http.StatusAccepted)
		case "tools/list":
			if r.Header.Get("Mcp-Session-Id") == "expired-session" {
				expiredOnce = true
				http.Error(w, "session expired", http.StatusNotFound)
				return
			}
			finalListSession = r.Header.Get("Mcp-Session-Id")
			writeJSONRPCResult(t, w, req.ID, map[string]any{
				"tools": []map[string]any{},
			})
		default:
			t.Fatalf("unexpected method %s", req.Method)
		}
	}))
	defer server.Close()

	client := NewHTTPClient(server.Client())
	tools, err := client.ListTools(context.Background(), model.MCPProxyServer{
		Endpoint:  server.URL,
		Transport: model.MCPProxyTransportStreamableHTTP,
	})
	require.NoError(t, err)
	require.Empty(t, tools)
	require.Equal(t, []string{"initialize", "notifications/initialized", "tools/list", "initialize", "notifications/initialized", "tools/list"}, seenMethods)
	require.True(t, expiredOnce)
	require.Equal(t, "fresh-session", finalListSession)
}

func TestHTTPClientStreamableHTTPIdleSessionReinitializes(t *testing.T) {
	var seenMethods []string
	var initializeCount atomic.Int64
	var listSessions []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		seenMethods = append(seenMethods, req.Method)
		switch req.Method {
		case "initialize":
			sessionId := "session-" + fmt.Sprint(initializeCount.Add(1))
			require.Empty(t, r.Header.Get("Mcp-Session-Id"))
			w.Header().Set("Mcp-Session-Id", sessionId)
			writeJSONRPCResult(t, w, req.ID, map[string]any{
				"protocolVersion": "2025-06-18",
				"capabilities":    map[string]any{"tools": true},
				"serverInfo":      map[string]any{"name": "fake-mcp"},
			})
		case "notifications/initialized":
			w.WriteHeader(http.StatusAccepted)
		case "tools/list":
			listSessions = append(listSessions, r.Header.Get("Mcp-Session-Id"))
			writeJSONRPCResult(t, w, req.ID, map[string]any{
				"tools": []map[string]any{},
			})
		default:
			t.Fatalf("unexpected method %s", req.Method)
		}
	}))
	defer server.Close()

	client := NewHTTPClient(server.Client())
	client.SessionIdleTimeout = time.Minute
	proxyServer := model.MCPProxyServer{
		Endpoint:  server.URL,
		Transport: model.MCPProxyTransportStreamableHTTP,
	}
	_, err := client.ListTools(context.Background(), proxyServer)
	require.NoError(t, err)

	sessionKey := mcpProxySessionKey(proxyServer)
	client.sessionMu.Lock()
	client.streamableLastActive[sessionKey] = time.Now().Add(-time.Hour).Unix()
	client.sessionMu.Unlock()

	_, err = client.ListTools(context.Background(), proxyServer)
	require.NoError(t, err)
	require.Equal(t, []string{"initialize", "notifications/initialized", "tools/list", "initialize", "notifications/initialized", "tools/list"}, seenMethods)
	require.Equal(t, []string{"session-1", "session-2"}, listSessions)
}

func TestHTTPClientCloseIdleSessions(t *testing.T) {
	client := NewHTTPClient(nil)
	now := time.Now().Unix()
	client.streamableSessions["streamable"] = "session-1"
	client.streamableInitialized["streamable"] = true
	client.streamableLastActive["streamable"] = now - int64(time.Hour.Seconds())
	sse := newSSESession("sse", "https://example.test/message", io.NopCloser(strings.NewReader("")))
	sse.lastActive = now - int64(time.Hour.Seconds())
	client.sseSessions["sse"] = sse

	closed := client.CloseIdleSessions(time.Minute)
	require.Equal(t, 2, closed)
	require.Empty(t, client.streamableSessions)
	require.Empty(t, client.streamableInitialized)
	require.Empty(t, client.streamableLastActive)
	require.Empty(t, client.sseSessions)
	require.True(t, sse.isClosed())
}

func TestHTTPClientInitializedNotificationMethodNotFoundIsIgnored(t *testing.T) {
	var seenMethods []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		seenMethods = append(seenMethods, req.Method)
		switch req.Method {
		case "initialize":
			writeJSONRPCResult(t, w, req.ID, map[string]any{
				"protocolVersion": "2025-06-18",
				"capabilities":    map[string]any{"tools": true},
				"serverInfo":      map[string]any{"name": "legacy-mcp"},
			})
		case "notifications/initialized":
			writeJSONRPCError(t, w, req.ID, -32601, "method not found")
		case "tools/list":
			writeJSONRPCResult(t, w, req.ID, map[string]any{
				"tools": []map[string]any{},
			})
		default:
			t.Fatalf("unexpected method %s", req.Method)
		}
	}))
	defer server.Close()

	client := NewHTTPClient(server.Client())
	tools, err := client.ListTools(context.Background(), model.MCPProxyServer{
		Endpoint:  server.URL,
		Transport: model.MCPProxyTransportStreamableHTTP,
	})
	require.NoError(t, err)
	require.Empty(t, tools)
	require.Equal(t, []string{"initialize", "notifications/initialized", "tools/list"}, seenMethods)
}

func TestHTTPClientPingMethodNotFoundIsIgnored(t *testing.T) {
	var seenMethods []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		seenMethods = append(seenMethods, req.Method)
		switch req.Method {
		case "initialize":
			writeJSONRPCResult(t, w, req.ID, map[string]any{
				"protocolVersion": "2025-06-18",
				"capabilities":    map[string]any{"tools": true},
				"serverInfo":      map[string]any{"name": "legacy-mcp"},
			})
		case "notifications/initialized":
			w.WriteHeader(http.StatusAccepted)
		case "ping":
			writeJSONRPCError(t, w, req.ID, dto.MCPErrorCodeMethodNotFound, "method not found")
		default:
			t.Fatalf("unexpected method %s", req.Method)
		}
	}))
	defer server.Close()

	client := NewHTTPClient(server.Client())
	result, err := client.Test(context.Background(), model.MCPProxyServer{
		Endpoint:  server.URL,
		Transport: model.MCPProxyTransportHTTP,
	})
	require.NoError(t, err)
	require.Equal(t, "legacy-mcp", result.ServerName)
	require.Equal(t, []string{"initialize", "notifications/initialized", "ping"}, seenMethods)
}

func TestHTTPClientParsesStreamableSSEResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Contains(t, r.Header.Get("Accept"), "text/event-stream")
		var req struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		switch req.Method {
		case "initialize":
			writeJSONRPCResult(t, w, req.ID, map[string]any{
				"protocolVersion": "2025-06-18",
				"capabilities":    map[string]any{"tools": true},
				"serverInfo":      map[string]any{"name": "fake-mcp"},
			})
		case "notifications/initialized":
			w.WriteHeader(http.StatusAccepted)
		case "tools/list":
			w.Header().Set("Content-Type", "text/event-stream")
			_, err := w.Write([]byte(`event: message
data: {"jsonrpc":"2.0","id":` + string(req.ID) + `,"result":{"tools":[{"name":"sse_tool","description":"SSE tool","inputSchema":{"type":"object"}}]}}

`))
			require.NoError(t, err)
		default:
			t.Fatalf("unexpected method %s", req.Method)
		}
	}))
	defer server.Close()

	client := NewHTTPClient(server.Client())
	tools, err := client.ListTools(context.Background(), model.MCPProxyServer{
		Endpoint:  server.URL,
		Transport: model.MCPProxyTransportStreamableHTTP,
	})
	require.NoError(t, err)
	require.Len(t, tools, 1)
	require.Equal(t, "sse_tool", tools[0].Name)
}

func TestHTTPClientSSEDualChannelListTools(t *testing.T) {
	events := make(chan string, 4)
	var sseConnections atomic.Int64
	var seenMethods []string
	mux := http.NewServeMux()
	mux.HandleFunc("/sse", func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Contains(t, r.Header.Get("Accept"), "text/event-stream")
		sseConnections.Add(1)
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		require.True(t, ok)
		_, err := w.Write([]byte("event: endpoint\ndata: /message\n\n"))
		require.NoError(t, err)
		flusher.Flush()
		for {
			select {
			case event := <-events:
				_, err := w.Write([]byte(event))
				require.NoError(t, err)
				flusher.Flush()
			case <-r.Context().Done():
				return
			}
		}
	})
	mux.HandleFunc("/message", func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		var req struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		seenMethods = append(seenMethods, req.Method)
		switch req.Method {
		case "initialize":
			events <- `event: message
data: {"jsonrpc":"2.0","id":` + string(req.ID) + `,"result":{"protocolVersion":"2025-06-18","capabilities":{"tools":true},"serverInfo":{"name":"fake-sse"}}}

`
		case "notifications/initialized":
			w.WriteHeader(http.StatusAccepted)
			return
		case "tools/list":
			events <- `event: message
data: {"jsonrpc":"2.0","id":` + string(req.ID) + `,"result":{"tools":[{"name":"dual_sse_tool","description":"Dual SSE tool","inputSchema":{"type":"object"}}]}}

`
		default:
			t.Fatalf("unexpected method %s", req.Method)
		}
		w.WriteHeader(http.StatusAccepted)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client := NewHTTPClient(server.Client())
	defer client.CloseSessions()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	tools, err := client.ListTools(ctx, model.MCPProxyServer{
		Endpoint:  server.URL + "/sse",
		Transport: model.MCPProxyTransportSSE,
	})
	cancel()
	require.NoError(t, err)
	require.Len(t, tools, 1)
	require.Equal(t, "dual_sse_tool", tools[0].Name)
	require.Equal(t, []string{"initialize", "notifications/initialized", "tools/list"}, seenMethods)
	require.EqualValues(t, 1, sseConnections.Load())

	snapshot := client.SessionSnapshot(model.MCPProxyServer{
		Endpoint:  server.URL + "/sse",
		Transport: model.MCPProxyTransportSSE,
	})
	require.True(t, snapshot.HasSession)
	require.True(t, snapshot.Initialized)
	require.True(t, snapshot.SSEConnected)
	require.Equal(t, 1, snapshot.ActiveSessions)
	require.Equal(t, 0, snapshot.PendingRequests)
	require.NotEmpty(t, snapshot.MessageEndpoint)
	require.Positive(t, snapshot.LastActivityAt)
}

func writeJSONRPCResult(t *testing.T, w http.ResponseWriter, id json.RawMessage, result any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  result,
	}))
}

func writeJSONRPCError(t *testing.T, w http.ResponseWriter, id json.RawMessage, code int, message string) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"error": map[string]any{
			"code":    code,
			"message": message,
		},
	}))
}
