package controller

import (
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/bridge"
	"github.com/QuantumNous/new-api/pkg/bridgepolicy"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func TunnelHTTP(c *gin.Context) {
	body, err := readTunnelHTTPBody(c)
	if err != nil {
		if errors.Is(err, common.ErrRequestBodyTooLarge) {
			c.String(http.StatusRequestEntityTooLarge, "tunnel http request body too large")
			return
		}
		c.String(http.StatusBadRequest, err.Error())
		return
	}
	resp, err := service.ForwardTunnelHTTPRequest(service.TunnelHTTPForwardRequest{
		Context:       c.Request.Context(),
		ConnectionKey: c.Param("connection_key"),
		Slug:          c.Param("slug"),
		Method:        c.Request.Method,
		Host:          c.Request.Host,
		ProxyPath:     c.Param("proxy_path"),
		RawQuery:      c.Request.URL.RawQuery,
		Headers:       c.Request.Header,
		Body:          body,
		RequestId:     c.GetString(common.RequestIdKey),
		ClientIP:      c.ClientIP(),
	})
	if err != nil {
		writeTunnelHTTPError(c, err)
		return
	}
	writeTunnelHTTPResponse(c, resp)
}

func readTunnelHTTPBody(c *gin.Context) ([]byte, error) {
	if c == nil || c.Request == nil || c.Request.Body == nil {
		return nil, nil
	}
	limit := int64(service.DefaultTunnelHTTPMaxRequestBytes) + 1
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, limit))
	if err != nil {
		return nil, err
	}
	if len(body) > service.DefaultTunnelHTTPMaxRequestBytes {
		return nil, common.ErrRequestBodyTooLarge
	}
	return body, nil
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

func tunnelHTTPControllerDropHeader(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "connection", "proxy-connection", "keep-alive", "proxy-authenticate", "proxy-authorization",
		"te", "trailer", "transfer-encoding", "upgrade", "content-length":
		return true
	default:
		return false
	}
}
