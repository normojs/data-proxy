package proxy

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
)

const (
	defaultHTTPTimeout            = 30 * time.Second
	defaultHTTPSessionIdleTimeout = 15 * time.Minute
	defaultHTTPRetryMaxAttempts   = 3
	defaultHTTPRetryBaseDelay     = 200 * time.Millisecond
	defaultHTTPRetryMaxDelay      = 2 * time.Second
)

var errStreamableSessionExpired = errors.New("mcp proxy streamable http session expired")

type HTTPClient struct {
	Client                *http.Client
	SessionIdleTimeout    time.Duration
	RetryMaxAttempts      int
	RetryBaseDelay        time.Duration
	nextId                atomic.Int64
	sessionMu             sync.Mutex
	streamableSessions    map[string]string
	streamableInitialized map[string]bool
	streamableLastActive  map[string]int64
	sseSessions           map[string]*sseSession
	initLocks             map[string]*sync.Mutex
	oauthMu               sync.Mutex
	oauthTokens           map[string]oauthCachedToken
}

func NewHTTPClient(client *http.Client) *HTTPClient {
	if client == nil {
		client = &http.Client{Timeout: defaultHTTPTimeout}
	}
	return &HTTPClient{
		Client:                client,
		streamableSessions:    map[string]string{},
		streamableInitialized: map[string]bool{},
		streamableLastActive:  map[string]int64{},
		sseSessions:           map[string]*sseSession{},
		initLocks:             map[string]*sync.Mutex{},
		oauthTokens:           map[string]oauthCachedToken{},
	}
}

func (c *HTTPClient) CloseSessions() {
	if c == nil {
		return
	}
	c.sessionMu.Lock()
	sseSessions := c.sseSessions
	c.sseSessions = map[string]*sseSession{}
	c.streamableSessions = map[string]string{}
	c.streamableInitialized = map[string]bool{}
	c.streamableLastActive = map[string]int64{}
	c.sessionMu.Unlock()
	for _, session := range sseSessions {
		if session != nil {
			session.close()
		}
	}
}

func (c *HTTPClient) CloseIdleSessions(idleTimeout time.Duration) int {
	if c == nil {
		return 0
	}
	if idleTimeout <= 0 {
		idleTimeout = c.sessionIdleTimeout()
	}
	if idleTimeout <= 0 {
		return 0
	}
	now := time.Now().Unix()
	closed := 0
	staleSSE := []*sseSession{}
	c.sessionMu.Lock()
	for key := range c.streamableSessions {
		if c.streamableSessionExpiredLocked(key, now, idleTimeout) {
			delete(c.streamableSessions, key)
			delete(c.streamableInitialized, key)
			delete(c.streamableLastActive, key)
			closed++
		}
	}
	for key, session := range c.sseSessions {
		if session == nil || session.isIdleExpired(idleTimeout, now) {
			delete(c.sseSessions, key)
			if session != nil {
				staleSSE = append(staleSSE, session)
			}
			closed++
		}
	}
	c.sessionMu.Unlock()
	for _, session := range staleSSE {
		session.close()
	}
	return closed
}

func (c *HTTPClient) SessionSnapshot(server model.MCPProxyServer) SessionSnapshot {
	snapshot := SessionSnapshot{Transport: strings.TrimSpace(server.Transport)}
	if c == nil {
		snapshot.LastError = ErrClientNotConfigured.Error()
		return snapshot
	}
	key := mcpProxySessionKey(server)
	c.sessionMu.Lock()
	switch server.Transport {
	case model.MCPProxyTransportStreamableHTTP:
		snapshot.StreamableSession = strings.TrimSpace(c.streamableSessions[key]) != ""
		snapshot.HasSession = snapshot.StreamableSession
		snapshot.Initialized = c.streamableInitialized[key]
		snapshot.LastActivityAt = c.streamableLastActive[key]
		if snapshot.HasSession {
			snapshot.ActiveSessions = 1
		}
	case model.MCPProxyTransportSSE:
		session := c.sseSessions[key]
		c.sessionMu.Unlock()
		if session == nil {
			return snapshot
		}
		return session.snapshot(snapshot.Transport)
	default:
		c.sessionMu.Unlock()
		return snapshot
	}
	c.sessionMu.Unlock()
	return snapshot
}

func (c *HTTPClient) sessionIdleTimeout() time.Duration {
	if c == nil || c.SessionIdleTimeout <= 0 {
		return defaultHTTPSessionIdleTimeout
	}
	return c.SessionIdleTimeout
}

func (c *HTTPClient) Test(ctx context.Context, server model.MCPProxyServer) (TestResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	var lastErr error
	for attempt := 0; attempt < c.retryMaxAttempts(); attempt++ {
		if attempt > 0 {
			c.clearSession(server)
			if err := c.waitBeforeRetry(ctx, attempt); err != nil {
				return TestResult{}, err
			}
		}
		var result initializeResult
		if err := c.rpc(ctx, server, "initialize", defaultMCPInitializeParams(), &result); err != nil {
			lastErr = err
			if !isMCPProxyRetryableError(err) {
				return TestResult{}, err
			}
			continue
		}
		if err := c.completeInitializedHandshake(ctx, server); err != nil {
			lastErr = err
			if !isMCPProxyRetryableError(err) {
				return TestResult{}, err
			}
			continue
		}
		if err := c.ping(ctx, server); err != nil && !isIgnoredPingError(err) {
			lastErr = err
			if !isMCPProxyRetryableError(err) {
				return TestResult{}, err
			}
			continue
		}
		return TestResult{
			ProtocolVersion: result.ProtocolVersion,
			ServerName:      result.ServerInfo.Name,
			Capabilities:    result.Capabilities,
		}, nil
	}
	return TestResult{}, lastErr
}

func (c *HTTPClient) ping(ctx context.Context, server model.MCPProxyServer) error {
	var result map[string]any
	return c.rpc(ctx, server, "ping", map[string]any{}, &result)
}

func (c *HTTPClient) ListTools(ctx context.Context, server model.MCPProxyServer) ([]ToolDefinition, error) {
	var result toolsListResult
	if err := c.rpcWithBackoff(ctx, server, dto.MCPMethodToolsList, map[string]any{}, &result); err != nil {
		return nil, err
	}
	tools := make([]ToolDefinition, 0, len(result.Tools))
	for _, tool := range result.Tools {
		tools = append(tools, ToolDefinition{
			Name:        tool.Name,
			Title:       tool.Title,
			Description: tool.Description,
			InputSchema: tool.InputSchema,
		})
	}
	return tools, nil
}

func (c *HTTPClient) CallTool(ctx context.Context, server model.MCPProxyServer, req CallRequest) (CallResult, error) {
	startedAt := time.Now()
	var result dto.MCPToolCallResult
	if err := c.rpc(ctx, server, dto.MCPMethodToolsCall, dto.MCPToolCallParams{
		Name:      req.ToolName,
		Arguments: req.Arguments,
	}, &result); err != nil {
		return CallResult{DurationMS: int(time.Since(startedAt).Milliseconds())}, err
	}
	resultSize := 0
	if bytes, err := common.Marshal(result); err == nil {
		resultSize = len(bytes)
	}
	return CallResult{
		Content:    result.Content,
		Metadata:   result.Metadata,
		Summary:    summarizeHTTPCallResult(result.Content),
		DurationMS: int(time.Since(startedAt).Milliseconds()),
		ResultSize: resultSize,
	}, nil
}

func (c *HTTPClient) rpc(ctx context.Context, server model.MCPProxyServer, method string, params any, out any) error {
	if c == nil {
		return ErrClientNotConfigured
	}
	if c.Client == nil {
		c.Client = &http.Client{Timeout: defaultHTTPTimeout}
	}
	endpoint := strings.TrimSpace(server.Endpoint)
	if endpoint == "" {
		return errors.New("mcp proxy endpoint is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	requestBody, id, err := c.buildJSONRPCRequest(method, params)
	if err != nil {
		return err
	}
	if server.Transport == model.MCPProxyTransportSSE {
		if method != "initialize" {
			if err := c.ensureSSEInitialized(ctx, server); err != nil {
				return err
			}
		}
		return c.rpcSSE(ctx, server, method, requestBody, id, out)
	}
	if server.Transport == model.MCPProxyTransportStreamableHTTP && method != "initialize" {
		if err := c.ensureStreamableInitialized(ctx, server); err != nil {
			return err
		}
		err := c.rpcHTTPPost(ctx, server, method, requestBody, id, out)
		if !errors.Is(err, errStreamableSessionExpired) {
			return err
		}
		if err := c.ensureStreamableInitialized(ctx, server); err != nil {
			return err
		}
		return c.rpcHTTPPost(ctx, server, method, requestBody, id, out)
	}
	return c.rpcHTTPPost(ctx, server, method, requestBody, id, out)
}

func (c *HTTPClient) rpcWithBackoff(ctx context.Context, server model.MCPProxyServer, method string, params any, out any) error {
	if ctx == nil {
		ctx = context.Background()
	}
	var lastErr error
	for attempt := 0; attempt < c.retryMaxAttempts(); attempt++ {
		if attempt > 0 {
			c.clearSession(server)
			if err := c.waitBeforeRetry(ctx, attempt); err != nil {
				return err
			}
		}
		err := c.rpc(ctx, server, method, params, out)
		if err == nil {
			return nil
		}
		lastErr = err
		if !isMCPProxyRetryableError(err) {
			return err
		}
	}
	return lastErr
}

func (c *HTTPClient) retryMaxAttempts() int {
	if c == nil || c.RetryMaxAttempts <= 0 {
		return defaultHTTPRetryMaxAttempts
	}
	if c.RetryMaxAttempts > 5 {
		return 5
	}
	return c.RetryMaxAttempts
}

func (c *HTTPClient) retryBaseDelay() time.Duration {
	if c == nil || c.RetryBaseDelay <= 0 {
		return defaultHTTPRetryBaseDelay
	}
	return c.RetryBaseDelay
}

func (c *HTTPClient) waitBeforeRetry(ctx context.Context, attempt int) error {
	delay := c.retryBaseDelay()
	for i := 1; i < attempt; i++ {
		delay *= 2
		if delay >= defaultHTTPRetryMaxDelay {
			delay = defaultHTTPRetryMaxDelay
			break
		}
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (c *HTTPClient) buildJSONRPCRequest(method string, params any) ([]byte, json.RawMessage, error) {
	paramsBytes, err := common.Marshal(params)
	if err != nil {
		return nil, nil, err
	}
	id := c.nextId.Add(1)
	requestBody, err := common.Marshal(dto.MCPRequest{
		JSONRPC: dto.MCPJSONRPCVersion,
		ID:      json.RawMessage(fmt.Sprintf("%d", id)),
		Method:  method,
		Params:  paramsBytes,
	})
	if err != nil {
		return nil, nil, err
	}
	return requestBody, json.RawMessage(fmt.Sprintf("%d", id)), nil
}

func (c *HTTPClient) buildJSONRPCNotification(method string, params any) ([]byte, error) {
	var paramsBytes json.RawMessage
	if params != nil {
		bytes, err := common.Marshal(params)
		if err != nil {
			return nil, err
		}
		paramsBytes = bytes
	}
	request := dto.MCPRequest{
		JSONRPC: dto.MCPJSONRPCVersion,
		Method:  method,
		Params:  paramsBytes,
	}
	return common.Marshal(request)
}

func (c *HTTPClient) sendInitializedNotification(ctx context.Context, server model.MCPProxyServer) error {
	body, err := c.buildJSONRPCNotification(dto.MCPMethodInitialized, map[string]any{})
	if err != nil {
		return err
	}
	if server.Transport == model.MCPProxyTransportSSE {
		return c.rpcSSENotification(ctx, server, body)
	}
	return c.rpcHTTPPostNotification(ctx, server, body)
}

func (c *HTTPClient) completeInitializedHandshake(ctx context.Context, server model.MCPProxyServer) error {
	if err := c.sendInitializedNotification(ctx, server); err != nil {
		if !isIgnoredInitializedNotificationError(err) {
			return err
		}
	}
	sessionKey := mcpProxySessionKey(server)
	switch server.Transport {
	case model.MCPProxyTransportStreamableHTTP:
		c.markStreamableInitialized(sessionKey)
	case model.MCPProxyTransportSSE:
		if session := c.getSSESession(sessionKey); session != nil {
			session.markInitialized()
		}
	}
	return nil
}

func isIgnoredInitializedNotificationError(err error) bool {
	var rpcErr *jsonRPCError
	return errors.As(err, &rpcErr) && rpcErr.Code == dto.MCPErrorCodeMethodNotFound
}

func isIgnoredPingError(err error) bool {
	var rpcErr *jsonRPCError
	return errors.As(err, &rpcErr) && rpcErr.Code == dto.MCPErrorCodeMethodNotFound
}

func isMCPProxyRetryableError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	if errors.Is(err, errStreamableSessionExpired) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return true
	}
	message := strings.ToLower(err.Error())
	for _, marker := range []string{
		"connection reset",
		"connection refused",
		"broken pipe",
		"unexpected eof",
		"upstream http status 429",
		"upstream http status 500",
		"upstream http status 502",
		"upstream http status 503",
		"upstream http status 504",
	} {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}

func (c *HTTPClient) rpcHTTPPost(ctx context.Context, server model.MCPProxyServer, method string, requestBody []byte, id json.RawMessage, out any) error {
	endpoint := strings.TrimSpace(server.Endpoint)
	sessionKey := ""
	sentSession := ""
	if server.Transport == model.MCPProxyTransportStreamableHTTP {
		sessionKey = mcpProxySessionKey(server)
		if method == "initialize" {
			c.clearStreamableSession(sessionKey)
		} else {
			sentSession = c.getStreamableSession(sessionKey)
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(requestBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	applyMCPProxyTransportHeaders(req, server)
	if sentSession != "" {
		req.Header.Set("Mcp-Session-Id", sentSession)
	}
	if err := c.applyHTTPAuth(ctx, req, server); err != nil {
		return err
	}
	resp, err := c.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if sessionKey != "" {
		if nextSession := strings.TrimSpace(resp.Header.Get("Mcp-Session-Id")); nextSession != "" {
			c.setStreamableSession(sessionKey, nextSession)
		}
	}
	limit := int64(maxPositive(server.MaxResultSize, 1048576)) + 1
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if sessionKey != "" && resp.StatusCode == http.StatusNotFound && sentSession != "" {
			c.clearStreamableSession(sessionKey)
			return fmt.Errorf("%w: upstream http status %d: %s", errStreamableSessionExpired, resp.StatusCode, truncateHTTPErrorBody(string(body), 256))
		}
		return fmt.Errorf("mcp proxy upstream http status %d: %s", resp.StatusCode, truncateHTTPErrorBody(string(body), 256))
	}
	if sessionKey != "" && sentSession != "" {
		c.touchStreamableSession(sessionKey)
	}
	body, err = normalizeMCPProxyHTTPResponseBody(resp.Header.Get("Content-Type"), body, id)
	if err != nil {
		return err
	}
	if err := decodeJSONRPCResponse(body, out); err != nil {
		return err
	}
	return nil
}

func (c *HTTPClient) rpcHTTPPostNotification(ctx context.Context, server model.MCPProxyServer, requestBody []byte) error {
	endpoint := strings.TrimSpace(server.Endpoint)
	sessionKey := ""
	sentSession := ""
	if server.Transport == model.MCPProxyTransportStreamableHTTP {
		sessionKey = mcpProxySessionKey(server)
		sentSession = c.getStreamableSession(sessionKey)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(requestBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	applyMCPProxyTransportHeaders(req, server)
	if sentSession != "" {
		req.Header.Set("Mcp-Session-Id", sentSession)
	}
	if err := c.applyHTTPAuth(ctx, req, server); err != nil {
		return err
	}
	resp, err := c.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if sessionKey != "" {
		if nextSession := strings.TrimSpace(resp.Header.Get("Mcp-Session-Id")); nextSession != "" {
			c.setStreamableSession(sessionKey, nextSession)
		}
	}
	limit := int64(maxPositive(server.MaxResultSize, 1048576)) + 1
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if sessionKey != "" && resp.StatusCode == http.StatusNotFound && sentSession != "" {
			c.clearStreamableSession(sessionKey)
			return fmt.Errorf("%w: upstream http status %d: %s", errStreamableSessionExpired, resp.StatusCode, truncateHTTPErrorBody(string(body), 256))
		}
		return fmt.Errorf("mcp proxy upstream notification http status %d: %s", resp.StatusCode, truncateHTTPErrorBody(string(body), 256))
	}
	if sessionKey != "" && sentSession != "" {
		c.touchStreamableSession(sessionKey)
	}
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		return nil
	}
	if strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "text/event-stream") {
		return nil
	}
	if !bytes.HasPrefix(body, []byte("{")) {
		return nil
	}
	var rpcResp jsonRPCResponse
	if err := common.Unmarshal(body, &rpcResp); err != nil {
		return nil
	}
	if rpcResp.Error != nil {
		return rpcResp.Error
	}
	return nil
}

func decodeJSONRPCResponse(body []byte, out any) error {
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		return errors.New("mcp proxy upstream returned an empty json-rpc response")
	}
	var rpcResp jsonRPCResponse
	if err := common.Unmarshal(body, &rpcResp); err != nil {
		return err
	}
	if rpcResp.Error != nil {
		return rpcResp.Error
	}
	if out == nil {
		return nil
	}
	if len(rpcResp.Result) == 0 || string(rpcResp.Result) == "null" {
		return nil
	}
	return common.Unmarshal(rpcResp.Result, out)
}

func (c *HTTPClient) ensureStreamableInitialized(ctx context.Context, server model.MCPProxyServer) error {
	sessionKey := mcpProxySessionKey(server)
	lock := c.initLock("streamable:" + sessionKey)
	lock.Lock()
	defer lock.Unlock()
	if c.isStreamableInitialized(sessionKey) {
		return nil
	}
	var result initializeResult
	if err := c.rpc(ctx, server, "initialize", defaultMCPInitializeParams(), &result); err != nil {
		return err
	}
	if err := c.completeInitializedHandshake(ctx, server); err != nil {
		return err
	}
	return nil
}

func (c *HTTPClient) ensureSSEInitialized(ctx context.Context, server model.MCPProxyServer) error {
	sessionKey := mcpProxySessionKey(server)
	lock := c.initLock("sse:" + sessionKey)
	lock.Lock()
	defer lock.Unlock()
	session, err := c.getOrCreateSSESession(ctx, server)
	if err != nil {
		return err
	}
	if session.isInitialized() {
		return nil
	}
	var result initializeResult
	if err := c.rpc(ctx, server, "initialize", defaultMCPInitializeParams(), &result); err != nil {
		return err
	}
	if err := c.completeInitializedHandshake(ctx, server); err != nil {
		return err
	}
	return nil
}

func (c *HTTPClient) initLock(key string) *sync.Mutex {
	c.sessionMu.Lock()
	defer c.sessionMu.Unlock()
	if c.initLocks == nil {
		c.initLocks = map[string]*sync.Mutex{}
	}
	lock := c.initLocks[key]
	if lock == nil {
		lock = &sync.Mutex{}
		c.initLocks[key] = lock
	}
	return lock
}

func defaultMCPInitializeParams() map[string]any {
	return map[string]any{
		"protocolVersion": "2025-06-18",
		"capabilities":    map[string]any{},
		"clientInfo": map[string]any{
			"name":    "data-proxy",
			"version": "0.1.0",
		},
	}
}

func mcpProxySessionKey(server model.MCPProxyServer) string {
	authSum := sha256.Sum256([]byte(strings.TrimSpace(server.AuthType) + "\x00" + strings.TrimSpace(server.AuthRef)))
	return strings.TrimSpace(server.Transport) + "\x00" + strings.TrimSpace(server.Endpoint) + "\x00" + hex.EncodeToString(authSum[:])
}

func (c *HTTPClient) getStreamableSession(key string) string {
	c.sessionMu.Lock()
	defer c.sessionMu.Unlock()
	if c.streamableSessionExpiredLocked(key, time.Now().Unix(), c.sessionIdleTimeout()) {
		delete(c.streamableSessions, key)
		delete(c.streamableInitialized, key)
		delete(c.streamableLastActive, key)
		return ""
	}
	return c.streamableSessions[key]
}

func (c *HTTPClient) streamableSessionExpiredLocked(key string, now int64, idleTimeout time.Duration) bool {
	if strings.TrimSpace(c.streamableSessions[key]) == "" || idleTimeout <= 0 {
		return false
	}
	lastActive := c.streamableLastActive[key]
	if lastActive <= 0 {
		return false
	}
	return now-lastActive >= int64(idleTimeout.Seconds())
}

func (c *HTTPClient) getSSESession(key string) *sseSession {
	c.sessionMu.Lock()
	defer c.sessionMu.Unlock()
	return c.sseSessions[key]
}

func (c *HTTPClient) setStreamableSession(key string, sessionId string) {
	c.sessionMu.Lock()
	defer c.sessionMu.Unlock()
	if c.streamableSessions == nil {
		c.streamableSessions = map[string]string{}
	}
	if c.streamableLastActive == nil {
		c.streamableLastActive = map[string]int64{}
	}
	c.streamableSessions[key] = sessionId
	c.streamableLastActive[key] = time.Now().Unix()
}

func (c *HTTPClient) clearStreamableSession(key string) {
	c.sessionMu.Lock()
	defer c.sessionMu.Unlock()
	delete(c.streamableSessions, key)
	delete(c.streamableInitialized, key)
	delete(c.streamableLastActive, key)
}

func (c *HTTPClient) clearSession(server model.MCPProxyServer) {
	if c == nil {
		return
	}
	key := mcpProxySessionKey(server)
	var sse *sseSession
	c.sessionMu.Lock()
	delete(c.streamableSessions, key)
	delete(c.streamableInitialized, key)
	delete(c.streamableLastActive, key)
	if c.sseSessions != nil {
		sse = c.sseSessions[key]
		delete(c.sseSessions, key)
	}
	c.sessionMu.Unlock()
	if sse != nil {
		sse.close()
	}
}

func (c *HTTPClient) isStreamableInitialized(key string) bool {
	c.sessionMu.Lock()
	defer c.sessionMu.Unlock()
	if c.streamableSessionExpiredLocked(key, time.Now().Unix(), c.sessionIdleTimeout()) {
		delete(c.streamableSessions, key)
		delete(c.streamableInitialized, key)
		delete(c.streamableLastActive, key)
		return false
	}
	return c.streamableInitialized[key]
}

func (c *HTTPClient) markStreamableInitialized(key string) {
	c.sessionMu.Lock()
	defer c.sessionMu.Unlock()
	if c.streamableInitialized == nil {
		c.streamableInitialized = map[string]bool{}
	}
	if c.streamableLastActive == nil {
		c.streamableLastActive = map[string]int64{}
	}
	c.streamableInitialized[key] = true
	c.streamableLastActive[key] = time.Now().Unix()
}

func (c *HTTPClient) touchStreamableSession(key string) {
	c.sessionMu.Lock()
	defer c.sessionMu.Unlock()
	if c.streamableLastActive == nil {
		c.streamableLastActive = map[string]int64{}
	}
	if strings.TrimSpace(c.streamableSessions[key]) != "" {
		c.streamableLastActive[key] = time.Now().Unix()
	}
}

func (c *HTTPClient) rpcSSE(ctx context.Context, server model.MCPProxyServer, method string, requestBody []byte, id json.RawMessage, out any) error {
	sessionKey := mcpProxySessionKey(server)
	session, err := c.getOrCreateSSESession(ctx, server)
	if err != nil {
		return err
	}
	body, err := session.call(ctx, c, server, requestBody, id)
	if err != nil {
		if session.isClosed() {
			c.clearSSESession(sessionKey, session)
		}
		return err
	}
	if err := decodeJSONRPCResponse(body, out); err != nil {
		return err
	}
	return nil
}

func (c *HTTPClient) rpcSSENotification(ctx context.Context, server model.MCPProxyServer, requestBody []byte) error {
	sessionKey := mcpProxySessionKey(server)
	session, err := c.getOrCreateSSESession(ctx, server)
	if err != nil {
		return err
	}
	if err := session.notify(ctx, c, server, requestBody); err != nil {
		if session.isClosed() {
			c.clearSSESession(sessionKey, session)
		}
		return err
	}
	return nil
}

func (c *HTTPClient) getOrCreateSSESession(ctx context.Context, server model.MCPProxyServer) (*sseSession, error) {
	sessionKey := mcpProxySessionKey(server)
	var staleSession *sseSession
	c.sessionMu.Lock()
	if session := c.sseSessions[sessionKey]; session != nil && !session.isClosed() {
		if !session.isIdleExpired(c.sessionIdleTimeout(), time.Now().Unix()) {
			c.sessionMu.Unlock()
			return session, nil
		}
		staleSession = session
		delete(c.sseSessions, sessionKey)
	}
	c.sessionMu.Unlock()
	if staleSession != nil {
		staleSession.close()
	}

	session, err := c.openSSESession(ctx, server, sessionKey)
	if err != nil {
		return nil, err
	}

	c.sessionMu.Lock()
	if c.sseSessions == nil {
		c.sseSessions = map[string]*sseSession{}
	}
	if existing := c.sseSessions[sessionKey]; existing != nil && !existing.isClosed() {
		c.sessionMu.Unlock()
		session.close()
		return existing, nil
	}
	c.sseSessions[sessionKey] = session
	c.sessionMu.Unlock()
	return session, nil
}

func (c *HTTPClient) openSSESession(ctx context.Context, server model.MCPProxyServer, sessionKey string) (*sseSession, error) {
	endpoint := strings.TrimSpace(server.Endpoint)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/event-stream")
	if err := c.applyHTTPAuth(ctx, req, server); err != nil {
		return nil, err
	}
	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		limit := int64(maxPositive(server.MaxResultSize, 1048576)) + 1
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, limit))
		if readErr != nil {
			return nil, readErr
		}
		return nil, fmt.Errorf("mcp proxy upstream sse status %d: %s", resp.StatusCode, truncateHTTPErrorBody(string(body), 256))
	}
	if !strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "text/event-stream") {
		defer resp.Body.Close()
		return nil, fmt.Errorf("mcp proxy upstream sse endpoint returned content-type %s", resp.Header.Get("Content-Type"))
	}

	scanner := newSSEScanner(resp.Body, maxPositive(server.MaxResultSize, 1048576))
	var messageEndpoint string
	for scanner.Scan() {
		event := parseSSEEvent(scanner.Text())
		if event.Name != "endpoint" {
			continue
		}
		messageEndpoint = strings.TrimSpace(event.Data)
		break
	}
	if err := scanner.Err(); err != nil {
		resp.Body.Close()
		return nil, err
	}
	if messageEndpoint == "" {
		resp.Body.Close()
		return nil, errors.New("mcp proxy upstream sse did not send an endpoint event")
	}
	messageURL, err := resolveSSEMessageEndpoint(endpoint, messageEndpoint)
	if err != nil {
		resp.Body.Close()
		return nil, err
	}

	session := newSSESession(sessionKey, messageURL, resp.Body)
	go session.readLoop(scanner)
	return session, nil
}

func (c *HTTPClient) clearSSESession(key string, session *sseSession) {
	c.sessionMu.Lock()
	defer c.sessionMu.Unlock()
	if existing := c.sseSessions[key]; existing == session {
		delete(c.sseSessions, key)
	}
}

type sseSession struct {
	key             string
	messageEndpoint string
	body            io.Closer

	mu          sync.Mutex
	pending     map[string]chan sseResponse
	initialized bool
	closed      bool
	err         error
	createdAt   int64
	lastActive  int64
}

type sseResponse struct {
	body []byte
	err  error
}

func newSSESession(key string, messageEndpoint string, body io.Closer) *sseSession {
	now := time.Now().Unix()
	return &sseSession{
		key:             key,
		messageEndpoint: messageEndpoint,
		body:            body,
		pending:         map[string]chan sseResponse{},
		createdAt:       now,
		lastActive:      now,
	}
}

func (s *sseSession) snapshot(transport string) SessionSnapshot {
	if s == nil {
		return SessionSnapshot{Transport: transport}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	activeSessions := 0
	if !s.closed {
		activeSessions = 1
	}
	snapshot := SessionSnapshot{
		Transport:       transport,
		HasSession:      true,
		Initialized:     s.initialized && !s.closed,
		MessageEndpoint: s.messageEndpoint,
		SSEConnected:    !s.closed,
		ActiveSessions:  activeSessions,
		PendingRequests: len(s.pending),
		LastActivityAt:  s.lastActive,
	}
	if s.err != nil {
		snapshot.LastError = truncateHTTPErrorBody(s.err.Error(), 256)
	}
	return snapshot
}

func (s *sseSession) call(ctx context.Context, client *HTTPClient, server model.MCPProxyServer, requestBody []byte, id json.RawMessage) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	key := jsonRPCIDKey(id)
	if key == "" {
		return nil, errors.New("mcp proxy sse request id is required")
	}
	ch, err := s.registerPending(key)
	if err != nil {
		return nil, err
	}
	defer s.unregisterPending(key, ch)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.messageEndpoint, bytes.NewReader(requestBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if err := client.applyHTTPAuth(ctx, req, server); err != nil {
		return nil, err
	}
	resp, err := client.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	limit := int64(maxPositive(server.MaxResultSize, 1048576)) + 1
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("mcp proxy upstream sse message status %d: %s", resp.StatusCode, truncateHTTPErrorBody(string(body), 256))
	}
	s.touch()
	if direct := bytes.TrimSpace(body); len(direct) > 0 && strings.HasPrefix(string(direct), "{") {
		return direct, nil
	}

	select {
	case result := <-ch:
		if result.err != nil {
			return nil, result.err
		}
		return result.body, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (s *sseSession) notify(ctx context.Context, client *HTTPClient, server model.MCPProxyServer, requestBody []byte) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := s.ensureOpen(); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.messageEndpoint, bytes.NewReader(requestBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if err := client.applyHTTPAuth(ctx, req, server); err != nil {
		return err
	}
	resp, err := client.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	limit := int64(maxPositive(server.MaxResultSize, 1048576)) + 1
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("mcp proxy upstream sse notification status %d: %s", resp.StatusCode, truncateHTTPErrorBody(string(body), 256))
	}
	s.touch()
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		return nil
	}
	if !bytes.HasPrefix(body, []byte("{")) {
		return nil
	}
	var rpcResp jsonRPCResponse
	if err := common.Unmarshal(body, &rpcResp); err != nil {
		return nil
	}
	if rpcResp.Error != nil {
		return rpcResp.Error
	}
	return nil
}

func (s *sseSession) registerPending(key string) (<-chan sseResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		if s.err != nil {
			return nil, s.err
		}
		return nil, errors.New("mcp proxy sse session is closed")
	}
	if _, exists := s.pending[key]; exists {
		return nil, fmt.Errorf("mcp proxy sse duplicate pending request id %s", key)
	}
	ch := make(chan sseResponse, 1)
	s.pending[key] = ch
	s.lastActive = time.Now().Unix()
	return ch, nil
}

func (s *sseSession) ensureOpen() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		if s.err != nil {
			return s.err
		}
		return errors.New("mcp proxy sse session is closed")
	}
	return nil
}

func (s *sseSession) unregisterPending(key string, ch <-chan sseResponse) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing := s.pending[key]; existing == ch {
		delete(s.pending, key)
	}
}

func (s *sseSession) dispatch(body []byte, id json.RawMessage) {
	key := jsonRPCIDKey(id)
	if key == "" {
		return
	}
	s.mu.Lock()
	ch := s.pending[key]
	s.lastActive = time.Now().Unix()
	s.mu.Unlock()
	if ch == nil {
		return
	}
	select {
	case ch <- sseResponse{body: bytes.TrimSpace(body)}:
	default:
	}
}

func (s *sseSession) readLoop(scanner *bufio.Scanner) {
	for scanner.Scan() {
		event := parseSSEEvent(scanner.Text())
		data := []byte(strings.TrimSpace(event.Data))
		if len(data) == 0 || !bytes.HasPrefix(data, []byte("{")) {
			continue
		}
		var response jsonRPCResponse
		if err := common.Unmarshal(data, &response); err != nil {
			continue
		}
		s.dispatch(data, response.ID)
	}
	if err := scanner.Err(); err != nil {
		s.closeWithError(fmt.Errorf("mcp proxy upstream sse read failed: %w", err))
		return
	}
	s.closeWithError(errors.New("mcp proxy upstream sse stream closed"))
}

func (s *sseSession) isInitialized() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.initialized && !s.closed
}

func (s *sseSession) markInitialized() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.closed {
		s.initialized = true
		s.lastActive = time.Now().Unix()
	}
}

func (s *sseSession) touch() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.closed {
		s.lastActive = time.Now().Unix()
	}
}

func (s *sseSession) isClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}

func (s *sseSession) isIdleExpired(idleTimeout time.Duration, now int64) bool {
	if s == nil || idleTimeout <= 0 {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return true
	}
	if len(s.pending) > 0 {
		return false
	}
	if s.lastActive <= 0 {
		return false
	}
	return now-s.lastActive >= int64(idleTimeout.Seconds())
}

func (s *sseSession) close() {
	s.closeWithError(errors.New("mcp proxy sse session closed"))
}

func (s *sseSession) closeWithError(err error) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	s.err = err
	pending := s.pending
	s.pending = map[string]chan sseResponse{}
	body := s.body
	s.mu.Unlock()

	if body != nil {
		_ = body.Close()
	}
	for _, ch := range pending {
		select {
		case ch <- sseResponse{err: err}:
		default:
		}
	}
}

func applyMCPProxyTransportHeaders(req *http.Request, server model.MCPProxyServer) {
	switch server.Transport {
	case model.MCPProxyTransportSSE, model.MCPProxyTransportStreamableHTTP:
		req.Header.Set("Accept", "application/json, text/event-stream")
	default:
		req.Header.Set("Accept", "application/json")
	}
	if server.Transport == model.MCPProxyTransportStreamableHTTP {
		req.Header.Set("MCP-Protocol-Version", "2025-06-18")
	}
}

func normalizeMCPProxyHTTPResponseBody(contentType string, body []byte, id json.RawMessage) ([]byte, error) {
	if !strings.Contains(strings.ToLower(contentType), "text/event-stream") {
		return bytes.TrimSpace(body), nil
	}
	eventBody, err := readJSONRPCFromSSE(body, id)
	if err != nil {
		return nil, err
	}
	return eventBody, nil
}

func readJSONRPCFromSSE(body []byte, id json.RawMessage) ([]byte, error) {
	scanner := newSSEScanner(bytes.NewReader(body), len(body)+1024)
	for scanner.Scan() {
		event := parseSSEEvent(scanner.Text())
		if event.Data == "" || !strings.HasPrefix(strings.TrimSpace(event.Data), "{") {
			continue
		}
		if responseMatchesJSONRPCID([]byte(event.Data), id) {
			return []byte(strings.TrimSpace(event.Data)), nil
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return nil, errors.New("mcp proxy upstream sse response did not include a json-rpc data event")
}

type sseEvent struct {
	Name string
	Data string
}

func newSSEScanner(reader io.Reader, maxSize int) *bufio.Scanner {
	scanner := bufio.NewScanner(reader)
	if maxSize <= 0 {
		maxSize = 1048576
	}
	scanner.Buffer(make([]byte, 0, 64*1024), maxSize+1024)
	scanner.Split(splitSSEEvents)
	return scanner
}

func splitSSEEvents(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if index := bytes.Index(data, []byte("\n\n")); index >= 0 {
		return index + 2, data[:index], nil
	}
	if index := bytes.Index(data, []byte("\r\n\r\n")); index >= 0 {
		return index + 4, data[:index], nil
	}
	if atEOF && len(data) > 0 {
		return len(data), data, nil
	}
	return 0, nil, nil
}

func parseSSEEvent(raw string) sseEvent {
	var event sseEvent
	var dataLines []string
	lines := strings.Split(raw, "\n")
	for _, line := range lines {
		line = strings.TrimRight(line, "\r")
		if strings.HasPrefix(line, ":") || strings.TrimSpace(line) == "" {
			continue
		}
		field, value, ok := strings.Cut(line, ":")
		if ok && strings.HasPrefix(value, " ") {
			value = strings.TrimPrefix(value, " ")
		}
		if !ok {
			field = line
			value = ""
		}
		switch field {
		case "event":
			event.Name = value
		case "data":
			dataLines = append(dataLines, value)
		}
	}
	event.Data = strings.Join(dataLines, "\n")
	return event
}

func responseMatchesJSONRPCID(body []byte, id json.RawMessage) bool {
	if len(id) == 0 {
		return true
	}
	var resp jsonRPCResponse
	if err := common.Unmarshal(body, &resp); err != nil {
		return false
	}
	return rawJSONEqual(resp.ID, id)
}

func rawJSONEqual(a json.RawMessage, b json.RawMessage) bool {
	a = bytes.TrimSpace(a)
	b = bytes.TrimSpace(b)
	if bytes.Equal(a, b) {
		return true
	}
	var av any
	var bv any
	if common.Unmarshal(a, &av) != nil || common.Unmarshal(b, &bv) != nil {
		return false
	}
	return fmt.Sprintf("%v", av) == fmt.Sprintf("%v", bv)
}

func jsonRPCIDKey(id json.RawMessage) string {
	id = bytes.TrimSpace(id)
	if len(id) == 0 {
		return ""
	}
	var value any
	if err := common.Unmarshal(id, &value); err == nil {
		switch typed := value.(type) {
		case string:
			return "s:" + typed
		case float64:
			return fmt.Sprintf("n:%.0f", typed)
		}
	}
	return "r:" + string(id)
}

func resolveSSEMessageEndpoint(baseEndpoint string, messageEndpoint string) (string, error) {
	baseURL, err := url.Parse(strings.TrimSpace(baseEndpoint))
	if err != nil {
		return "", err
	}
	messageURL, err := url.Parse(strings.TrimSpace(messageEndpoint))
	if err != nil {
		return "", err
	}
	return baseURL.ResolveReference(messageURL).String(), nil
}

type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jsonRPCError   `json:"error,omitempty"`
}

type jsonRPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (e *jsonRPCError) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return fmt.Sprintf("mcp proxy upstream error %d: %s", e.Code, e.Message)
	}
	return fmt.Sprintf("mcp proxy upstream error %d", e.Code)
}

type initializeResult struct {
	ProtocolVersion string         `json:"protocolVersion"`
	Capabilities    map[string]any `json:"capabilities"`
	ServerInfo      struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"serverInfo"`
}

type toolsListResult struct {
	Tools []struct {
		Name        string         `json:"name"`
		Title       string         `json:"title,omitempty"`
		Description string         `json:"description"`
		InputSchema map[string]any `json:"inputSchema"`
	} `json:"tools"`
}

func summarizeHTTPCallResult(content []dto.MCPContentBlock) string {
	for _, block := range content {
		if block.Text != "" {
			return block.Text
		}
		if block.Type != "" {
			return block.Type
		}
	}
	return ""
}

func truncateHTTPErrorBody(value string, limit int) string {
	if limit <= 0 || len(value) <= limit {
		return value
	}
	if limit <= 3 {
		return value[:limit]
	}
	return value[:limit-3] + "..."
}

func maxPositive(value int, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}
