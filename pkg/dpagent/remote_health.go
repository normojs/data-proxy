package dpagent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

func checkBridgeWebSocketAuth(rawURL string, token string, timeout time.Duration) error {
	rawURL = strings.TrimSpace(rawURL)
	token = strings.TrimSpace(token)
	if rawURL == "" {
		return errors.New("bridge websocket URL is empty")
	}
	if token == "" {
		return errors.New("agent token is missing")
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	headers := http.Header{}
	headers.Set("Authorization", "Bearer "+token)
	headers.Set("User-Agent", "data-proxy-agent/"+DefaultAgentVersion+" doctor")
	dialer := websocket.Dialer{HandshakeTimeout: timeout}
	conn, resp, err := dialer.DialContext(ctx, rawURL, headers)
	if err != nil {
		return bridgeAuthDialError(err, resp)
	}
	_ = conn.Close()
	return nil
}

func bridgeAuthDialError(err error, resp *http.Response) error {
	if resp == nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	detail := strings.TrimSpace(string(body))
	if detail == "" {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return fmt.Errorf("HTTP %d: %s", resp.StatusCode, detail)
}
