package controller

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/dto"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestMCPInitialize(t *testing.T) {
	recorder := postMCPRequest(t, `{
		"jsonrpc":"2.0",
		"id":1,
		"method":"initialize",
		"params":{
			"protocolVersion":"2025-06-18",
			"capabilities":{},
			"clientInfo":{"name":"test-client","version":"0.0.1"}
		}
	}`)

	require.Equal(t, http.StatusOK, recorder.Code)

	var resp struct {
		JSONRPC string                  `json:"jsonrpc"`
		ID      json.RawMessage         `json:"id"`
		Result  dto.MCPInitializeResult `json:"result"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, dto.MCPJSONRPCVersion, resp.JSONRPC)
	require.JSONEq(t, `1`, string(resp.ID))
	require.Equal(t, dto.MCPProtocolVersion, resp.Result.ProtocolVersion)
	require.Equal(t, "data-proxy", resp.Result.ServerInfo.Name)
	require.Contains(t, resp.Result.Capabilities, "tools")
}

func TestMCPInitializedNotificationReturnsNoResponse(t *testing.T) {
	recorder := postMCPRequest(t, `{
		"jsonrpc":"2.0",
		"method":"notifications/initialized"
	}`)

	require.Equal(t, http.StatusAccepted, recorder.Code)
	require.Empty(t, recorder.Body.String())
}

func TestMCPRequestWithoutIDIsInvalidUnlessNotification(t *testing.T) {
	recorder := postMCPRequest(t, `{
		"jsonrpc":"2.0",
		"method":"tools/list"
	}`)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp dto.MCPResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.NotNil(t, resp.Error)
	require.Equal(t, dto.MCPErrorCodeInvalidRequest, resp.Error.Code)
}

func postMCPRequest(t *testing.T, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/mcp", MCP)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	return recorder
}
