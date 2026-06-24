package controller

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

func TunnelMCP(c *gin.Context) {
	handleTunnelMCPPost(c, false)
}

func TunnelMCPMessage(c *gin.Context) {
	handleTunnelMCPPost(c, true)
}

func TunnelMCPDelete(c *gin.Context) {
	err := service.CloseTunnelMCPSession(c.GetInt("id"), c.Param("slug"), c.Param("connection_key"), tunnelMCPRequestSessionId(c))
	if err != nil {
		if errors.Is(err, service.ErrTunnelMCPSessionNotFound) {
			c.String(http.StatusNotFound, "tunnel mcp session not found")
			return
		}
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}

func handleTunnelMCPPost(c *gin.Context, sseMessageEndpoint bool) {
	var req dto.MCPRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		writeTunnelMCPResponse(c, http.StatusOK, &dto.MCPResponse{
			JSONRPC: dto.MCPJSONRPCVersion,
			Error: &dto.MCPError{
				Code:    dto.MCPErrorCodeParseError,
				Message: "Parse error",
				Data:    err.Error(),
			},
		})
		return
	}

	if status, handled := prepareTunnelMCPGatewaySession(c, req); handled {
		if status == http.StatusAccepted || status == http.StatusNoContent {
			c.Status(status)
			return
		}
		c.String(status, "tunnel mcp session not found")
		return
	}

	resp, status := tunnelMCPProcessRequest(c, req)
	if sseMessageEndpoint {
		if resp == nil {
			c.Status(status)
			return
		}
		sessionId := tunnelMCPRequestSessionId(c)
		if service.SendTunnelMCPSSE(sessionId, *resp) {
			c.Status(http.StatusAccepted)
			return
		}
		c.String(http.StatusConflict, "tunnel mcp sse session is not bound")
		return
	}
	writeTunnelMCPResponse(c, status, resp)
}

func TunnelMCPSSE(c *gin.Context) {
	sessionId := tunnelMCPRequestSessionId(c)
	events, _, nextSessionId, err := service.BindTunnelMCPSSESession(c.GetInt("id"), c.Param("slug"), c.Param("connection_key"), sessionId, tunnelMCPSessionContext(c, nil))
	if err != nil {
		if errors.Is(err, service.ErrTunnelMCPSessionNotFound) {
			c.String(http.StatusNotFound, "tunnel mcp session not found")
			return
		}
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache, no-transform")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Header("Mcp-Session-Id", nextSessionId)
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.String(http.StatusInternalServerError, "streaming is not supported")
		return
	}
	fmt.Fprintf(c.Writer, "event: endpoint\ndata: %s\n\n", tunnelMCPMessageEndpoint(c, nextSessionId))
	flusher.Flush()
	defer service.UnbindTunnelMCPSSESession(nextSessionId, events)

	heartbeat := time.NewTicker(25 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-heartbeat.C:
			fmt.Fprint(c.Writer, ": keepalive\n\n")
			flusher.Flush()
		case body, ok := <-events:
			if !ok {
				return
			}
			fmt.Fprintf(c.Writer, "event: message\ndata: %s\n\n", string(body))
			flusher.Flush()
		}
	}
}

func tunnelMCPProcessRequest(c *gin.Context, req dto.MCPRequest) (*dto.MCPResponse, int) {
	if req.JSONRPC != dto.MCPJSONRPCVersion || req.Method == "" {
		resp := mcpErrorResponse(req.ID, dto.MCPErrorCodeInvalidRequest, "Invalid Request", nil)
		return &resp, http.StatusOK
	}
	if req.Method == dto.MCPMethodInitialized {
		return nil, http.StatusAccepted
	}
	if strings.HasPrefix(req.Method, "notifications/") && len(req.ID) == 0 {
		return nil, http.StatusAccepted
	}
	if len(req.ID) == 0 {
		resp := mcpErrorResponse(req.ID, dto.MCPErrorCodeInvalidRequest, "Invalid Request", nil)
		return &resp, http.StatusOK
	}

	switch req.Method {
	case dto.MCPMethodInitialize:
		resp := handleTunnelMCPInitialize(c, req)
		return &resp, http.StatusOK
	case dto.MCPMethodPing:
		return &dto.MCPResponse{
			JSONRPC: dto.MCPJSONRPCVersion,
			ID:      req.ID,
			Result:  map[string]any{},
		}, http.StatusOK
	case dto.MCPMethodToolsList:
		resp := handleTunnelMCPToolsList(c, req)
		return &resp, http.StatusOK
	case dto.MCPMethodToolsCall:
		resp := handleTunnelMCPToolCall(c, req)
		return &resp, http.StatusOK
	case dto.MCPMethodResourcesList,
		dto.MCPMethodResourcesRead,
		dto.MCPMethodPromptsList,
		dto.MCPMethodPromptsGet:
		resp := handleTunnelMCPRawForward(c, req)
		return &resp, http.StatusOK
	default:
		resp := mcpErrorResponse(req.ID, dto.MCPErrorCodeMethodNotFound, "Method not found", req.Method)
		return &resp, http.StatusOK
	}
}

func handleTunnelMCPInitialize(c *gin.Context, req dto.MCPRequest) dto.MCPResponse {
	var params *dto.MCPInitializeParams
	if len(bytes.TrimSpace(req.Params)) > 0 {
		params = &dto.MCPInitializeParams{}
		if err := common.Unmarshal(req.Params, params); err != nil {
			return mcpErrorResponse(req.ID, dto.MCPErrorCodeInvalidParams, "Invalid params", err.Error())
		}
	}
	_, sessionId, err := service.EnsureTunnelMCPSession(c.GetInt("id"), c.Param("slug"), c.Param("connection_key"), tunnelMCPRequestSessionId(c), tunnelMCPSessionContext(c, params))
	if err != nil {
		return mcpErrorResponse(req.ID, dto.MCPErrorCodeInternalError, "Internal error", err.Error())
	}
	c.Header("Mcp-Session-Id", sessionId)
	return dto.MCPResponse{
		JSONRPC: dto.MCPJSONRPCVersion,
		ID:      req.ID,
		Result: dto.MCPInitializeResult{
			ProtocolVersion: dto.MCPProtocolVersion,
			Capabilities: map[string]any{
				"tools": map[string]any{
					"listChanged": true,
				},
				"resources": map[string]any{
					"listChanged": true,
				},
				"prompts": map[string]any{
					"listChanged": true,
				},
			},
			ServerInfo: &dto.MCPImplementationInfo{
				Name:    "data-proxy-tunnel-mcp",
				Version: "0.2.0",
			},
		},
	}
}

func handleTunnelMCPToolsList(c *gin.Context, req dto.MCPRequest) dto.MCPResponse {
	tools, err := service.ListTunnelMCPTools(service.TunnelMCPToolsListRequest{
		Context:       c.Request.Context(),
		UserId:        c.GetInt("id"),
		TokenId:       c.GetInt("token_id"),
		Slug:          c.Param("slug"),
		ConnectionKey: c.Param("connection_key"),
		RequestId:     common.JsonRawMessageToString(req.ID),
		SessionId:     tunnelMCPRequestSessionId(c),
	})
	if err != nil {
		if errors.Is(err, service.ErrTunnelRateLimited) {
			return mcpErrorResponse(req.ID, dto.MCPErrorCodeInvalidRequest, "MCP gateway rate limit exceeded", err.Error())
		}
		return mcpErrorResponse(req.ID, dto.MCPErrorCodeInternalError, "Internal error", err.Error())
	}
	return dto.MCPResponse{
		JSONRPC: dto.MCPJSONRPCVersion,
		ID:      req.ID,
		Result: dto.MCPToolsListResult{
			Tools: tools,
		},
	}
}

func handleTunnelMCPToolCall(c *gin.Context, req dto.MCPRequest) dto.MCPResponse {
	var params dto.MCPToolCallParams
	if len(bytes.TrimSpace(req.Params)) == 0 {
		return mcpErrorResponse(req.ID, dto.MCPErrorCodeInvalidParams, "Invalid params", "params is required")
	}
	if err := common.Unmarshal(req.Params, &params); err != nil {
		return mcpErrorResponse(req.ID, dto.MCPErrorCodeInvalidParams, "Invalid params", err.Error())
	}
	resp, err := service.CallTunnelMCPTool(service.TunnelMCPToolCallRequest{
		Context:       c.Request.Context(),
		UserId:        c.GetInt("id"),
		TokenId:       c.GetInt("token_id"),
		Slug:          c.Param("slug"),
		ConnectionKey: c.Param("connection_key"),
		RequestId:     common.JsonRawMessageToString(req.ID),
		SessionId:     tunnelMCPRequestSessionId(c),
		Params:        params,
	})
	if err != nil {
		if errors.Is(err, service.ErrTunnelRateLimited) {
			return mcpErrorResponse(req.ID, dto.MCPErrorCodeInvalidRequest, "MCP gateway rate limit exceeded", err.Error())
		}
		return mcpErrorResponse(req.ID, dto.MCPErrorCodeInternalError, "Internal error", err.Error())
	}
	if resp != nil && resp.Result != nil {
		return dto.MCPResponse{
			JSONRPC: dto.MCPJSONRPCVersion,
			ID:      req.ID,
			Result:  resp.Result,
		}
	}
	if resp == nil {
		return mcpErrorResponse(req.ID, dto.MCPErrorCodeInternalError, "Internal error", "empty tunnel mcp tool call response")
	}
	return mcpErrorResponse(req.ID, resp.ErrorCode, resp.ErrorMessage, resp.ErrorData)
}

func handleTunnelMCPRawForward(c *gin.Context, req dto.MCPRequest) dto.MCPResponse {
	result, err := service.CallTunnelMCPRaw(service.TunnelMCPRawRequest{
		Context:       c.Request.Context(),
		UserId:        c.GetInt("id"),
		TokenId:       c.GetInt("token_id"),
		Slug:          c.Param("slug"),
		ConnectionKey: c.Param("connection_key"),
		RequestId:     common.JsonRawMessageToString(req.ID),
		SessionId:     tunnelMCPRequestSessionId(c),
		Method:        req.Method,
		Params:        req.Params,
	})
	if err != nil {
		return mcpErrorResponse(req.ID, dto.MCPErrorCodeInternalError, "Internal error", err.Error())
	}
	return dto.MCPResponse{
		JSONRPC: dto.MCPJSONRPCVersion,
		ID:      req.ID,
		Result:  result,
	}
}

func prepareTunnelMCPGatewaySession(c *gin.Context, req dto.MCPRequest) (int, bool) {
	sessionId := tunnelMCPRequestSessionId(c)
	if req.Method == dto.MCPMethodInitialize {
		return 0, false
	}
	if req.Method != dto.MCPMethodInitialize && sessionId == "" {
		return 0, false
	}
	_, nextSessionId, err := service.EnsureTunnelMCPSession(c.GetInt("id"), c.Param("slug"), c.Param("connection_key"), sessionId, tunnelMCPSessionContext(c, nil))
	if err != nil {
		if errors.Is(err, service.ErrTunnelMCPSessionNotFound) {
			return http.StatusNotFound, true
		}
		return http.StatusInternalServerError, true
	}
	c.Header("Mcp-Session-Id", nextSessionId)
	return 0, false
}

func writeTunnelMCPResponse(c *gin.Context, status int, resp *dto.MCPResponse) {
	if resp == nil {
		c.Status(status)
		return
	}
	if tunnelMCPWantsSSE(c) {
		body, err := json.Marshal(resp)
		if err != nil {
			c.JSON(http.StatusOK, mcpErrorResponse(resp.ID, dto.MCPErrorCodeInternalError, "Internal error", err.Error()))
			return
		}
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache, no-transform")
		c.Header("X-Accel-Buffering", "no")
		c.String(status, "event: message\ndata: %s\n\n", string(body))
		return
	}
	c.JSON(status, resp)
}

func tunnelMCPWantsSSE(c *gin.Context) bool {
	return strings.Contains(strings.ToLower(c.GetHeader("Accept")), "text/event-stream")
}

func tunnelMCPSessionContext(c *gin.Context, params *dto.MCPInitializeParams) service.TunnelMCPSessionContext {
	clientVersion := ""
	if params != nil && params.ClientInfo != nil {
		clientVersion = strings.TrimSpace(params.ClientInfo.Name)
		if params.ClientInfo.Version != "" {
			if clientVersion != "" {
				clientVersion += "@"
			}
			clientVersion += strings.TrimSpace(params.ClientInfo.Version)
		}
	}
	return service.TunnelMCPSessionContext{
		ClientVersion: clientVersion,
		ClientIP:      c.ClientIP(),
		UserAgent:     c.Request.UserAgent(),
	}
}

func tunnelMCPRequestSessionId(c *gin.Context) string {
	if value := strings.TrimSpace(c.GetHeader("Mcp-Session-Id")); value != "" {
		return value
	}
	return strings.TrimSpace(c.Query("session_id"))
}

func tunnelMCPMessageEndpoint(c *gin.Context, sessionId string) string {
	path := strings.TrimRight(c.Request.URL.Path, "/") + "/message"
	values := url.Values{}
	values.Set("session_id", sessionId)
	return path + "?" + values.Encode()
}
