package dpagent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/dto"
	"github.com/gorilla/websocket"
)

type BridgeClient struct {
	Config Config
	Out    io.Writer
	Err    io.Writer
}

type BridgeRunResult struct {
	Opened      bool
	Registered  bool
	SessionID   string
	ClientID    string
	CloseReason string
}

func (c BridgeClient) Run(ctx context.Context) error {
	cfg := c.Config
	fillConfigDefaults(&cfg)
	if result := ValidateConfig(cfg, true); !result.OK() {
		return result.Error()
	}
	if ctx == nil {
		ctx = context.Background()
	}
	attempt := 0
	for {
		attempt++
		result, err := c.runOnce(ctx, attempt)
		if err == nil && result.CloseReason == "server requested close" {
			return nil
		}
		if !cfg.Runtime.Reconnect {
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		delay := reconnectDelay(cfg, attempt)
		logf(c.Err, "WARN", "bridge connection closed; reconnecting in %s", delay)
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (c BridgeClient) runOnce(ctx context.Context, attempt int) (BridgeRunResult, error) {
	bridgeURL, err := EffectiveBridgeWSURL(c.Config)
	if err != nil {
		return BridgeRunResult{}, err
	}
	token := ResolveToken(c.Config)
	if token == "" {
		return BridgeRunResult{}, errors.New("agent token is required")
	}
	headers := http.Header{}
	headers.Set("Authorization", "Bearer "+token)
	headers.Set("User-Agent", "data-proxy-agent/"+agentVersion(c.Config))
	dialer := websocket.DefaultDialer
	conn, _, err := dialer.DialContext(ctx, bridgeURL, headers)
	if err != nil {
		return BridgeRunResult{}, err
	}
	defer conn.Close()

	result := BridgeRunResult{Opened: true}
	logf(c.Err, "INFO", "connected to data-proxy bridge", "url", redactBridgeURL(bridgeURL), "attempt", attempt)
	if err := conn.WriteJSON(dto.BridgeWSMessage{
		Type: "register",
		Data: dto.BridgeClientRegisterRequest{
			ClientId:     c.Config.Agent.ClientID,
			Name:         c.Config.Agent.Name,
			Version:      agentVersion(c.Config),
			Platform:     agentPlatform(),
			Workspace:    c.Config.Agent.Workspace,
			Capabilities: append([]string(nil), c.Config.Agent.Capabilities...),
		},
	}); err != nil {
		return result, err
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var writeMu sync.Mutex
	writeJSON := func(msg dto.BridgeWSMessage) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		return conn.WriteJSON(msg)
	}
	if c.Config.Runtime.PingIntervalMS > 0 {
		go func() {
			ticker := time.NewTicker(time.Duration(c.Config.Runtime.PingIntervalMS) * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					_ = writeJSON(dto.BridgeWSMessage{Type: "ping", Id: fmt.Sprintf("ping-%d", time.Now().UnixMilli())})
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	for {
		var msg dto.BridgeWSMessage
		if err := conn.ReadJSON(&msg); err != nil {
			cancel()
			return result, err
		}
		switch msg.Type {
		case "registered":
			sessionID, clientID := registeredIDs(msg.Data)
			result.Registered = true
			result.SessionID = sessionID
			result.ClientID = clientID
			logf(c.Out, "INFO", "registered bridge client", "client_id", clientID, "session_id", sessionID)
		case "pong":
		case "close":
			cancel()
			result.CloseReason = "server requested close"
			logf(c.Err, "WARN", "server requested bridge close")
			return result, nil
		case "error":
			logf(c.Err, "ERROR", "server returned bridge error", "data", fmt.Sprintf("%v", msg.Data))
		case "tool_call":
			go func(message dto.BridgeWSMessage) {
				requestID := bridgeRequestID(message)
				toolName := bridgeToolName(message.Data)
				logf(c.Err, "WARN", "tool call rejected by early Go agent", "request_id", requestID, "tool_name", toolName)
				_ = writeJSON(dto.BridgeWSMessage{
					Type: "tool_error",
					Id:   requestID,
					Data: dto.BridgeToolCallError{
						Code:    "TOOL_NOT_SUPPORTED",
						Message: fmt.Sprintf("data-proxy-agent Go CLI has not implemented tool %q yet", toolName),
					},
				})
			}(msg)
		default:
			logf(c.Err, "WARN", "ignored bridge message", "type", msg.Type)
		}
	}
}

func bridgeRequestID(msg dto.BridgeWSMessage) string {
	if msg.Id != "" {
		return msg.Id
	}
	var call dto.BridgeToolCallRequest
	if decodeBridgeData(msg.Data, &call) == nil && call.RequestId != "" {
		return call.RequestId
	}
	return fmt.Sprintf("bridge-%d", time.Now().UnixMilli())
}

func bridgeToolName(data any) string {
	var call dto.BridgeToolCallRequest
	if decodeBridgeData(data, &call) == nil {
		return call.ToolName
	}
	return ""
}

func decodeBridgeData(data any, target any) error {
	bytes, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return json.Unmarshal(bytes, target)
}

func registeredIDs(data any) (string, string) {
	object, ok := data.(map[string]any)
	if !ok {
		bytes, err := json.Marshal(data)
		if err == nil {
			_ = json.Unmarshal(bytes, &object)
		}
	}
	sessionID, _ := object["session_id"].(string)
	clientID, _ := object["client_id"].(string)
	return sessionID, clientID
}

func reconnectDelay(cfg Config, attempt int) time.Duration {
	base := cfg.Runtime.ReconnectBaseMS
	if base <= 0 {
		base = DefaultReconnectBaseMS
	}
	maxValue := cfg.Runtime.ReconnectMaxMS
	if maxValue <= 0 {
		maxValue = DefaultReconnectMaxMS
	}
	delay := time.Duration(base) * time.Millisecond
	for i := 1; i < attempt && delay < time.Duration(maxValue)*time.Millisecond; i++ {
		delay *= 2
	}
	maxDelay := time.Duration(maxValue) * time.Millisecond
	if delay > maxDelay {
		return maxDelay
	}
	return delay
}

func agentVersion(cfg Config) string {
	if strings.TrimSpace(cfg.Agent.Version) != "" {
		return strings.TrimSpace(cfg.Agent.Version)
	}
	return DefaultAgentVersion
}

func agentPlatform() string {
	return runtime.GOOS + "-" + runtime.GOARCH
}

func logf(w io.Writer, level string, message string, kv ...any) {
	if w == nil {
		return
	}
	if len(kv) == 0 {
		fmt.Fprintf(w, "[%s] [%s] %s\n", time.Now().Format(time.RFC3339), level, message)
		return
	}
	parts := make([]string, 0, len(kv)/2)
	for i := 0; i+1 < len(kv); i += 2 {
		parts = append(parts, fmt.Sprintf("%v=%v", kv[i], kv[i+1]))
	}
	fmt.Fprintf(w, "[%s] [%s] %s %s\n", time.Now().Format(time.RFC3339), level, message, strings.Join(parts, " "))
}

func redactBridgeURL(value string) string {
	if idx := strings.Index(value, "?"); idx >= 0 {
		return value[:idx] + "?..."
	}
	return value
}
