package controller

import (
	"net/http"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

func TunnelTCP(c *gin.Context) {
	if !isTunnelHTTPWebSocketUpgrade(c.Request) {
		c.String(http.StatusUpgradeRequired, "tcp tunnel requires websocket upgrade")
		return
	}
	ws, err := bridgeUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	req := service.TunnelTCPForwardRequest{
		Context:       c.Request.Context(),
		ConnectionKey: c.Param("connection_key"),
		Slug:          c.Param("slug"),
		RequestId:     c.GetString(common.RequestIdKey),
		ClientIP:      c.ClientIP(),
	}
	if _, err := service.ForwardTunnelTCPWebSocket(req, tunnelHTTPWebSocketPeer{conn: ws}); err != nil {
		_ = ws.WriteControl(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseTryAgainLater, limitTunnelHTTPWebSocketCloseReason(err.Error())),
			time.Now().Add(time.Second),
		)
	}
}
