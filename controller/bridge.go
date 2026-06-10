package controller

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/bridge"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

const bridgeWSRegisterTimeout = 10 * time.Second

var bridgeUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func BridgeWebSocket(c *gin.Context) {
	ws, err := bridgeUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer ws.Close()

	if err := ws.SetReadDeadline(time.Now().Add(bridgeWSRegisterTimeout)); err != nil {
		return
	}
	var register dto.BridgeClientRegisterRequest
	if err := readBridgeRegisterMessage(ws, &register); err != nil {
		_ = ws.WriteJSON(dto.BridgeWSMessage{Type: "error", Data: gin.H{"message": err.Error()}})
		return
	}
	if err := ws.SetReadDeadline(time.Time{}); err != nil {
		return
	}

	result, err := service.RegisterBridgeClient(service.BridgeRegisterInput{
		UserId:       c.GetInt("id"),
		TokenId:      c.GetInt("token_id"),
		RequestIP:    c.ClientIP(),
		UserAgent:    c.Request.UserAgent(),
		ClientId:     register.ClientId,
		Name:         register.Name,
		Version:      register.Version,
		Platform:     register.Platform,
		Workspace:    register.Workspace,
		Capabilities: register.Capabilities,
	})
	if err != nil {
		_ = ws.WriteJSON(dto.BridgeWSMessage{Type: "error", Data: gin.H{"message": err.Error()}})
		return
	}
	outbound := make(chan bridge.OutboundMessage, 32)
	done := make(chan struct{})
	defer func() {
		_ = service.CloseBridgeClientSession(result.SessionId, "websocket closed")
		close(done)
	}()
	bridge.DefaultHub.Register(bridge.Session{
		SessionId:    result.SessionId,
		ClientId:     result.ClientId,
		UserId:       c.GetInt("id"),
		TokenId:      c.GetInt("token_id"),
		Name:         register.Name,
		Version:      register.Version,
		Platform:     register.Platform,
		Workspace:    register.Workspace,
		Capabilities: register.Capabilities,
		ConnectedAt:  time.Now(),
		LastSeenAt:   time.Now(),
		Send:         outbound,
	})

	if err := ws.WriteJSON(dto.BridgeWSMessage{
		Type: "registered",
		Data: gin.H{
			"session_id": result.SessionId,
			"client_id":  result.ClientId,
		},
	}); err != nil {
		return
	}
	go writeBridgeOutbound(ws, outbound, done)

	for {
		var msg dto.BridgeWSMessage
		if err := ws.ReadJSON(&msg); err != nil {
			return
		}
		switch msg.Type {
		case "ping":
			if err := service.TouchBridgeClientSession(result.SessionId); err != nil {
				_ = ws.WriteJSON(dto.BridgeWSMessage{Type: "error", Data: gin.H{"message": err.Error()}})
				return
			}
			outbound <- bridge.OutboundMessage{Type: "pong", Id: msg.Id}
		case "tool_result":
			result, err := decodeBridgeToolResult(msg.Data)
			if err != nil {
				outbound <- bridge.OutboundMessage{Type: "error", Id: msg.Id, Data: gin.H{"message": err.Error()}}
				continue
			}
			if !bridge.DefaultHub.CompleteToolCall(msg.Id, result) {
				outbound <- bridge.OutboundMessage{Type: "error", Id: msg.Id, Data: gin.H{"message": "unknown bridge request"}}
			}
		case "tool_error":
			clientErr, err := decodeBridgeToolError(msg.Data)
			if err != nil {
				outbound <- bridge.OutboundMessage{Type: "error", Id: msg.Id, Data: gin.H{"message": err.Error()}}
				continue
			}
			if !bridge.DefaultHub.FailToolCall(msg.Id, clientErr.Code, clientErr.Message) {
				outbound <- bridge.OutboundMessage{Type: "error", Id: msg.Id, Data: gin.H{"message": "unknown bridge request"}}
			}
		default:
			outbound <- bridge.OutboundMessage{Type: "error", Id: msg.Id, Data: gin.H{"message": "unsupported message type"}}
		}
	}
}

func writeBridgeOutbound(ws *websocket.Conn, outbound <-chan bridge.OutboundMessage, done <-chan struct{}) {
	for {
		select {
		case <-done:
			return
		case msg := <-outbound:
			if err := ws.WriteJSON(dto.BridgeWSMessage{
				Type: msg.Type,
				Id:   msg.Id,
				Data: msg.Data,
			}); err != nil {
				return
			}
		}
	}
}

func readBridgeRegisterMessage(ws *websocket.Conn, register *dto.BridgeClientRegisterRequest) error {
	var msg dto.BridgeWSMessage
	if err := ws.ReadJSON(&msg); err != nil {
		return err
	}
	if msg.Type != "register" {
		return errors.New("first bridge message must be register")
	}
	bytes, err := json.Marshal(msg.Data)
	if err != nil {
		return err
	}
	return json.Unmarshal(bytes, register)
}

func decodeBridgeToolResult(data any) (dto.BridgeToolCallResult, error) {
	var result dto.BridgeToolCallResult
	bytes, err := json.Marshal(data)
	if err != nil {
		return result, err
	}
	return result, json.Unmarshal(bytes, &result)
}

func decodeBridgeToolError(data any) (dto.BridgeToolCallError, error) {
	var result dto.BridgeToolCallError
	bytes, err := json.Marshal(data)
	if err != nil {
		return result, err
	}
	return result, json.Unmarshal(bytes, &result)
}

func GetBridgeClients(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	var status *int
	if statusQuery := c.Query("status"); statusQuery != "" {
		statusValue, err := strconv.Atoi(statusQuery)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		status = &statusValue
	}
	items, total, err := service.ListBridgeClients(service.BridgeClientListParams{
		UserId:  bridgeViewerUserId(c),
		Status:  status,
		Keyword: c.Query("keyword"),
		Offset:  pageInfo.GetStartIdx(),
		Limit:   pageInfo.GetPageSize(),
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

func GetBridgeClient(c *gin.Context) {
	item, err := service.GetBridgeClientDetail(service.BridgeClientDetailParams{
		UserId:   bridgeViewerUserId(c),
		ClientId: c.Param("client_id"),
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, item)
}

func GetBridgeClientHealth(c *gin.Context) {
	windowSeconds, err := parseOptionalInt64Query(c, "window_seconds")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	item, err := service.GetBridgeClientHealth(service.BridgeClientHealthParams{
		UserId:        bridgeViewerUserId(c),
		ClientId:      c.Param("client_id"),
		WindowSeconds: windowSeconds,
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, item)
}

func UpdateBridgeClient(c *gin.Context) {
	var req dto.BridgeClientUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	item, err := service.UpdateBridgeClient(service.BridgeClientUpdateParams{
		UserId:   bridgeViewerUserId(c),
		ClientId: c.Param("client_id"),
		Request:  req,
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, item)
}

func DeleteBridgeClient(c *gin.Context) {
	item, err := service.ArchiveBridgeClient(service.BridgeClientDetailParams{
		UserId:   bridgeViewerUserId(c),
		ClientId: c.Param("client_id"),
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, item)
}

func CloseBridgeSession(c *gin.Context) {
	var req dto.BridgeSessionCloseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	item, err := service.CloseBridgeSessionForAdmin(service.BridgeSessionCloseParams{
		UserId:    bridgeViewerUserId(c),
		SessionId: c.Param("session_id"),
		Reason:    req.Reason,
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, item)
}

func GetBridgeAuditLogs(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	tokenId, err := parseOptionalIntQuery(c, "token_id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	startTime, err := parseOptionalInt64Query(c, "start_time")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	endTime, err := parseOptionalInt64Query(c, "end_time")
	if err != nil {
		common.ApiError(c, err)
		return
	}

	items, total, err := service.ListBridgeAuditLogs(service.BridgeAuditLogListParams{
		UserId:    bridgeViewerUserId(c),
		TokenId:   tokenId,
		ClientId:  c.Query("client_id"),
		SessionId: c.Query("session_id"),
		ToolName:  c.Query("tool_name"),
		Status:    c.Query("status"),
		RequestId: c.Query("request_id"),
		StartTime: startTime,
		EndTime:   endTime,
		Keyword:   c.Query("keyword"),
		Offset:    pageInfo.GetStartIdx(),
		Limit:     pageInfo.GetPageSize(),
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

func bridgeViewerUserId(c *gin.Context) int {
	userId := c.GetInt("id")
	if model.IsAdmin(userId) && c.Query("scope") == "all" {
		return 0
	}
	return userId
}
