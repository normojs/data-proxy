package controller

import (
	"bytes"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

func MCP(c *gin.Context) {
	var req dto.MCPRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		c.JSON(http.StatusOK, mcpErrorResponse(nil, dto.MCPErrorCodeParseError, "Parse error", err.Error()))
		return
	}

	if req.JSONRPC != dto.MCPJSONRPCVersion || req.Method == "" {
		c.JSON(http.StatusOK, mcpErrorResponse(req.ID, dto.MCPErrorCodeInvalidRequest, "Invalid Request", nil))
		return
	}
	if req.Method == dto.MCPMethodInitialized {
		c.Status(http.StatusAccepted)
		return
	}
	if len(req.ID) == 0 {
		c.JSON(http.StatusOK, mcpErrorResponse(req.ID, dto.MCPErrorCodeInvalidRequest, "Invalid Request", nil))
		return
	}

	switch req.Method {
	case dto.MCPMethodInitialize:
		handleMCPInitialize(c, req)
	case dto.MCPMethodToolsList:
		tools, err := service.ListMCPTools()
		if err != nil {
			c.JSON(http.StatusOK, mcpErrorResponse(req.ID, dto.MCPErrorCodeInternalError, "Internal error", err.Error()))
			return
		}
		c.JSON(http.StatusOK, dto.MCPResponse{
			JSONRPC: dto.MCPJSONRPCVersion,
			ID:      req.ID,
			Result: dto.MCPToolsListResult{
				Tools: tools,
			},
		})
	case dto.MCPMethodToolsCall:
		handleMCPToolCall(c, req)
	default:
		c.JSON(http.StatusOK, mcpErrorResponse(req.ID, dto.MCPErrorCodeMethodNotFound, "Method not found", req.Method))
	}
}

func handleMCPInitialize(c *gin.Context, req dto.MCPRequest) {
	if len(bytes.TrimSpace(req.Params)) > 0 {
		var params dto.MCPInitializeParams
		if err := common.Unmarshal(req.Params, &params); err != nil {
			c.JSON(http.StatusOK, mcpErrorResponse(req.ID, dto.MCPErrorCodeInvalidParams, "Invalid params", err.Error()))
			return
		}
	}
	c.JSON(http.StatusOK, dto.MCPResponse{
		JSONRPC: dto.MCPJSONRPCVersion,
		ID:      req.ID,
		Result: dto.MCPInitializeResult{
			ProtocolVersion: dto.MCPProtocolVersion,
			Capabilities: map[string]any{
				"tools": map[string]any{
					"listChanged": false,
				},
			},
			ServerInfo: &dto.MCPImplementationInfo{
				Name:    "data-proxy",
				Version: "0.1.0",
			},
		},
	})
}

func handleMCPToolCall(c *gin.Context, req dto.MCPRequest) {
	var params dto.MCPToolCallParams
	if len(bytes.TrimSpace(req.Params)) == 0 {
		c.JSON(http.StatusOK, mcpErrorResponse(req.ID, dto.MCPErrorCodeInvalidParams, "Invalid params", "params is required"))
		return
	}
	if err := common.Unmarshal(req.Params, &params); err != nil {
		c.JSON(http.StatusOK, mcpErrorResponse(req.ID, dto.MCPErrorCodeInvalidParams, "Invalid params", err.Error()))
		return
	}
	if params.Name == "" {
		c.JSON(http.StatusOK, mcpErrorResponse(req.ID, dto.MCPErrorCodeInvalidParams, "Invalid params", "tool name is required"))
		return
	}
	requestId := common.JsonRawMessageToString(req.ID)
	callResp, err := service.CallMCPTool(service.MCPToolCallRequest{
		Context:                    c.Request.Context(),
		UserId:                     c.GetInt("id"),
		TokenId:                    c.GetInt("token_id"),
		TokenKey:                   c.GetString("token_key"),
		TokenUnlimited:             c.GetBool("token_unlimited_quota"),
		TokenQuotaHardLimitEnabled: c.GetBool("token_quota_hard_limit_enabled"),
		TokenQuota:                 c.GetInt("token_quota"),
		UsingGroup:                 common.GetContextKeyString(c, constant.ContextKeyUsingGroup),
		RequestId:                  requestId,
		RequestIP:                  c.ClientIP(),
		Params:                     params,
	})
	if err != nil {
		c.JSON(http.StatusOK, mcpErrorResponse(req.ID, dto.MCPErrorCodeInternalError, "Internal error", err.Error()))
		return
	}
	if callResp != nil && callResp.Result != nil {
		c.JSON(http.StatusOK, dto.MCPResponse{
			JSONRPC: dto.MCPJSONRPCVersion,
			ID:      req.ID,
			Result:  callResp.Result,
		})
		return
	}
	if callResp == nil {
		c.JSON(http.StatusOK, mcpErrorResponse(req.ID, dto.MCPErrorCodeInternalError, "Internal error", "empty tool call response"))
		return
	}
	c.JSON(http.StatusOK, mcpErrorResponse(req.ID, callResp.ErrorCode, callResp.ErrorMessage, callResp.ErrorData))
}

func mcpErrorResponse(id []byte, code int, message string, data any) dto.MCPResponse {
	return dto.MCPResponse{
		JSONRPC: dto.MCPJSONRPCVersion,
		ID:      id,
		Error: &dto.MCPError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}
}
