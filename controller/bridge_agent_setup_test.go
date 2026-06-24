package controller

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupBridgeAgentSetupControllerDB(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.Token{},
		&model.BridgeClient{},
		&model.BridgeAgentSetupToken{},
	))

	previousDB := model.DB
	previousServerAddress := system_setting.ServerAddress
	model.DB = db
	system_setting.ServerAddress = "https://dp.example"

	t.Cleanup(func() {
		model.DB = previousDB
		system_setting.ServerAddress = previousServerAddress
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
}

func TestEnsureBridgeAgentSetupControllerCreatesAgentConfig(t *testing.T) {
	setupBridgeAgentSetupControllerDB(t)
	gin.SetMode(gin.TestMode)

	body, err := common.Marshal(dto.BridgeAgentSetupRequest{
		ClientName: "Desktop Bridge Agent",
		Platform:   "darwin",
		Workspace:  "/workspace/project",
		Version:    "1.0.0",
	})
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/bridge/agent-setup", bytes.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set("id", 100)

	EnsureBridgeAgentSetup(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success bool                         `json:"success"`
		Data    dto.BridgeAgentSetupResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	require.True(t, response.Data.Created)
	require.True(t, response.Data.APIKeyOnce)
	require.NotEmpty(t, response.Data.APIKey)
	require.Equal(t, "https://dp.example", response.Data.BaseURL)
	require.Equal(t, "wss://dp.example/bridge/ws", response.Data.BridgeWSURL)
	require.Equal(t, "Bearer "+response.Data.APIKey, response.Data.Headers["Authorization"])
	require.Equal(t, response.Data.TokenId, response.Data.Client.TokenId)
	require.Equal(t, "Desktop Bridge Agent", response.Data.Client.Name)

	var client model.BridgeClient
	require.NoError(t, model.DB.First(&client, "client_id = ?", response.Data.ClientId).Error)
	require.Equal(t, 100, client.UserId)
	require.Equal(t, response.Data.TokenId, client.TokenId)
}

func TestBridgeAgentSetupTokenControllerCreateAndConsume(t *testing.T) {
	setupBridgeAgentSetupControllerDB(t)
	gin.SetMode(gin.TestMode)

	createBody, err := common.Marshal(dto.BridgeAgentSetupTokenRequest{
		ClientName: "Desktop Bridge Agent",
		Workspace:  "/workspace/project",
	})
	require.NoError(t, err)
	createRecorder := httptest.NewRecorder()
	createCtx, _ := gin.CreateTestContext(createRecorder)
	createCtx.Request = httptest.NewRequest(http.MethodPost, "/api/bridge/agent-setup-tokens", bytes.NewReader(createBody))
	createCtx.Request.Header.Set("Content-Type", "application/json")
	createCtx.Set("id", 100)

	CreateBridgeAgentSetupToken(createCtx)

	require.Equal(t, http.StatusOK, createRecorder.Code)
	var createResponse struct {
		Success bool                              `json:"success"`
		Data    dto.BridgeAgentSetupTokenResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(createRecorder.Body.Bytes(), &createResponse))
	require.True(t, createResponse.Success)
	require.NotEmpty(t, createResponse.Data.SetupToken)
	require.Contains(t, createResponse.Data.FullCommand, "data-proxy-agent enroll")

	consumeBody, err := common.Marshal(dto.BridgeAgentSetupTokenConsumeRequest{
		SetupToken: createResponse.Data.SetupToken,
		Version:    "1.0.0",
		Platform:   "darwin",
	})
	require.NoError(t, err)
	consumeRecorder := httptest.NewRecorder()
	consumeCtx, _ := gin.CreateTestContext(consumeRecorder)
	consumeCtx.Request = httptest.NewRequest(http.MethodPost, "/api/bridge/agent-setup/consume", bytes.NewReader(consumeBody))
	consumeCtx.Request.Header.Set("Content-Type", "application/json")

	ConsumeBridgeAgentSetupToken(consumeCtx)

	require.Equal(t, http.StatusOK, consumeRecorder.Code)
	var consumeResponse struct {
		Success bool                         `json:"success"`
		Data    dto.BridgeAgentSetupResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(consumeRecorder.Body.Bytes(), &consumeResponse))
	require.True(t, consumeResponse.Success)
	require.Equal(t, "Desktop Bridge Agent", consumeResponse.Data.Client.Name)
	require.Equal(t, "darwin", consumeResponse.Data.Client.Platform)
	require.NotEmpty(t, consumeResponse.Data.APIKey)
}
