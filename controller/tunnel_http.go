package controller

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/bridge"
	"github.com/QuantumNous/new-api/pkg/bridgepolicy"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"gorm.io/gorm"
)

func TunnelHTTP(c *gin.Context) {
	if isTunnelHTTPWebSocketUpgrade(c.Request) {
		tunnelHTTPWebSocket(c)
		return
	}
	req := service.TunnelHTTPForwardRequest{
		Context:       c.Request.Context(),
		ConnectionKey: c.Param("connection_key"),
		Slug:          c.Param("slug"),
		Method:        c.Request.Method,
		Host:          c.Request.Host,
		ProxyPath:     c.Param("proxy_path"),
		RawQuery:      c.Request.URL.RawQuery,
		Headers:       c.Request.Header,
		BodyReader:    c.Request.Body,
		ContentLength: c.Request.ContentLength,
		RequestId:     c.GetString(common.RequestIdKey),
		ClientIP:      c.ClientIP(),
	}
	wroteHeader := false
	resp, err := service.ForwardTunnelHTTPStream(req, func(event service.TunnelHTTPStreamEvent) error {
		if !wroteHeader && event.StatusCode > 0 {
			writeTunnelHTTPStreamHeaders(c, event)
			wroteHeader = true
		}
		if len(event.Body) > 0 {
			if _, err := c.Writer.Write(event.Body); err != nil {
				return err
			}
		}
		if event.Flush {
			if flusher, ok := c.Writer.(http.Flusher); ok {
				flusher.Flush()
			}
		}
		return nil
	})
	if err != nil {
		if wroteHeader {
			return
		}
		writeTunnelHTTPError(c, err)
		return
	}
	if !wroteHeader {
		writeTunnelHTTPResponse(c, resp)
	}
}

func tunnelHTTPWebSocket(c *gin.Context) {
	ws, err := bridgeUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	req := service.TunnelHTTPForwardRequest{
		Context:       c.Request.Context(),
		ConnectionKey: c.Param("connection_key"),
		Slug:          c.Param("slug"),
		Method:        c.Request.Method,
		Host:          c.Request.Host,
		ProxyPath:     c.Param("proxy_path"),
		RawQuery:      c.Request.URL.RawQuery,
		Headers:       c.Request.Header,
		RequestId:     c.GetString(common.RequestIdKey),
		ClientIP:      c.ClientIP(),
	}
	if _, err := service.ForwardTunnelHTTPWebSocket(req, tunnelHTTPWebSocketPeer{conn: ws}); err != nil {
		_ = ws.WriteControl(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseTryAgainLater, limitTunnelHTTPWebSocketCloseReason(err.Error())),
			time.Now().Add(time.Second),
		)
	}
}

type tunnelHTTPWebSocketPeer struct {
	conn *websocket.Conn
}

func (p tunnelHTTPWebSocketPeer) ReadFrame() (service.TunnelHTTPWebSocketFrame, error) {
	messageType, data, err := p.conn.ReadMessage()
	if err != nil {
		var closeErr *websocket.CloseError
		if errors.As(err, &closeErr) {
			return service.TunnelHTTPWebSocketFrame{
				FrameType:   service.TunnelWebSocketFrameClose,
				CloseCode:   closeErr.Code,
				CloseReason: closeErr.Text,
			}, err
		}
		return service.TunnelHTTPWebSocketFrame{}, err
	}
	switch messageType {
	case websocket.TextMessage:
		return service.TunnelHTTPWebSocketFrame{FrameType: service.TunnelWebSocketFrameText, Data: data}, nil
	case websocket.BinaryMessage:
		return service.TunnelHTTPWebSocketFrame{FrameType: service.TunnelWebSocketFrameBinary, Data: data}, nil
	case websocket.CloseMessage:
		return service.TunnelHTTPWebSocketFrame{FrameType: service.TunnelWebSocketFrameClose, Data: data}, nil
	case websocket.PingMessage:
		return service.TunnelHTTPWebSocketFrame{FrameType: service.TunnelWebSocketFramePing, Data: data}, nil
	case websocket.PongMessage:
		return service.TunnelHTTPWebSocketFrame{FrameType: service.TunnelWebSocketFramePong, Data: data}, nil
	default:
		return service.TunnelHTTPWebSocketFrame{FrameType: service.TunnelWebSocketFrameBinary, Data: data}, nil
	}
}

func (p tunnelHTTPWebSocketPeer) WriteFrame(frame service.TunnelHTTPWebSocketFrame) error {
	switch frame.FrameType {
	case service.TunnelWebSocketFrameText:
		return p.conn.WriteMessage(websocket.TextMessage, frame.Data)
	case service.TunnelWebSocketFramePing:
		return p.conn.WriteControl(websocket.PingMessage, frame.Data, time.Now().Add(time.Second))
	case service.TunnelWebSocketFramePong:
		return p.conn.WriteControl(websocket.PongMessage, frame.Data, time.Now().Add(time.Second))
	case service.TunnelWebSocketFrameClose:
		code := frame.CloseCode
		if code == 0 {
			code = websocket.CloseNormalClosure
		}
		return p.conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(code, limitTunnelHTTPWebSocketCloseReason(frame.CloseReason)), time.Now().Add(time.Second))
	case service.TunnelWebSocketFrameBinary:
		fallthrough
	default:
		return p.conn.WriteMessage(websocket.BinaryMessage, frame.Data)
	}
}

func (p tunnelHTTPWebSocketPeer) Close() error {
	return p.conn.Close()
}

func writeTunnelHTTPResponse(c *gin.Context, resp service.TunnelHTTPForwardResponse) {
	for key, values := range resp.Headers {
		if tunnelHTTPControllerDropHeader(key) {
			continue
		}
		for _, value := range values {
			c.Writer.Header().Add(key, value)
		}
	}
	if resp.RequestId != "" {
		c.Header("X-Tunnel-Request-Id", resp.RequestId)
	}
	if resp.BridgeSessionId != "" {
		c.Header("X-Tunnel-Bridge-Session-Id", resp.BridgeSessionId)
	}
	if resp.TargetClient != "" {
		c.Header("X-Tunnel-Target-Client", resp.TargetClient)
	}
	c.Writer.WriteHeader(resp.StatusCode)
	_, _ = c.Writer.Write(resp.Body)
}

func writeTunnelHTTPStreamHeaders(c *gin.Context, event service.TunnelHTTPStreamEvent) {
	for key, values := range event.Headers {
		if tunnelHTTPControllerDropHeader(key) {
			continue
		}
		for _, value := range values {
			c.Writer.Header().Add(key, value)
		}
	}
	if event.RequestId != "" {
		c.Header("X-Tunnel-Request-Id", event.RequestId)
	}
	if event.BridgeSessionId != "" {
		c.Header("X-Tunnel-Bridge-Session-Id", event.BridgeSessionId)
	}
	if event.TargetClient != "" {
		c.Header("X-Tunnel-Target-Client", event.TargetClient)
	}
	c.Writer.WriteHeader(event.StatusCode)
}

func writeTunnelHTTPError(c *gin.Context, err error) {
	status := http.StatusBadGateway
	message := err.Error()
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		status = http.StatusNotFound
	case errors.Is(err, service.ErrTunnelHTTPAuthRequired):
		status = http.StatusUnauthorized
		c.Header("WWW-Authenticate", `Bearer realm="tunnel-http"`)
	case errors.Is(err, service.ErrTunnelHTTPAuthForbidden),
		errors.Is(err, service.ErrTunnelHTTPRouteForbidden),
		errors.Is(err, service.ErrTunnelHTTPRouteConfigInvalid):
		status = http.StatusForbidden
	case errors.Is(err, service.ErrTunnelHTTPRequestTooLarge),
		errors.Is(err, service.ErrTunnelHTTPResponseTooLarge):
		status = http.StatusRequestEntityTooLarge
	case errors.Is(err, service.ErrTunnelRateLimited):
		status = http.StatusTooManyRequests
	case errors.Is(err, service.ErrTunnelBillingInsufficient):
		status = http.StatusPaymentRequired
	case errors.Is(err, bridge.ErrClientNotFound), errors.Is(err, bridge.ErrClientUnavailable), errors.Is(err, bridge.ErrClientDisconnected):
		status = http.StatusBadGateway
	case bridgepolicy.ErrorCode(err) == bridgepolicy.ErrorCodeToolNotAllowed,
		bridgepolicy.ErrorCode(err) == bridgepolicy.ErrorCodeHTTPTargetForbidden:
		status = http.StatusForbidden
	case strings.Contains(strings.ToLower(message), "not approved"),
		strings.Contains(strings.ToLower(message), "not active"),
		strings.Contains(strings.ToLower(message), "expired"):
		status = http.StatusForbidden
	case strings.Contains(strings.ToLower(message), "not found"):
		status = http.StatusNotFound
	}
	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.String(status, message)
}

func isTunnelHTTPWebSocketUpgrade(req *http.Request) bool {
	if req == nil {
		return false
	}
	if !strings.EqualFold(strings.TrimSpace(req.Header.Get("Upgrade")), "websocket") {
		return false
	}
	for _, item := range strings.Split(req.Header.Get("Connection"), ",") {
		if strings.EqualFold(strings.TrimSpace(item), "upgrade") {
			return true
		}
	}
	return false
}

func limitTunnelHTTPWebSocketCloseReason(reason string) string {
	reason = strings.TrimSpace(reason)
	if len(reason) > 120 {
		return reason[:120]
	}
	return reason
}

func tunnelHTTPControllerDropHeader(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "connection", "proxy-connection", "keep-alive", "proxy-authenticate", "proxy-authorization",
		"te", "trailer", "transfer-encoding", "upgrade", "content-length":
		return true
	default:
		return false
	}
}
