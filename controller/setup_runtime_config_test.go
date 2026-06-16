package controller

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestPostSetupRuntimeConfigRejectsAfterSetup(t *testing.T) {
	withSetupRuntimeConfigTestState(t)
	constant.Setup = true

	recorder := postSetupRuntimeConfig(t, map[string]any{
		"database_type": "sqlite",
	})

	require.False(t, decodeSetupRuntimeConfigResponse(t, recorder).Success)
	require.Contains(t, recorder.Body.String(), "系统已经初始化完成")
}

func TestPostSetupRuntimeConfigRejectsMismatchedDSN(t *testing.T) {
	withSetupRuntimeConfigTestState(t)

	recorder := postSetupRuntimeConfig(t, map[string]any{
		"database_type": "postgres",
		"sql_dsn":       "root:pass@tcp(127.0.0.1:3306)/data_proxy",
	})

	require.False(t, decodeSetupRuntimeConfigResponse(t, recorder).Success)
	require.Contains(t, recorder.Body.String(), "不是 PostgreSQL")
}

func TestPostSetupRuntimeConfigRequiresRedisConnectionString(t *testing.T) {
	tempDir := withSetupRuntimeConfigTestState(t)

	recorder := postSetupRuntimeConfig(t, map[string]any{
		"database_type":     "sqlite",
		"sqlite_path":       filepath.Join(tempDir, "data-proxy.db"),
		"redis_enabled":     true,
		"redis_conn_string": "",
	})

	require.False(t, decodeSetupRuntimeConfigResponse(t, recorder).Success)
	require.Contains(t, recorder.Body.String(), "启用 Redis 时必须填写连接字符串")
}

func TestPostSetupRuntimeConfigSavesSQLiteConfigAndRequiresRestart(t *testing.T) {
	tempDir := withSetupRuntimeConfigTestState(t)
	runtimeConfigPath := filepath.Join(tempDir, "runtime-config.json")
	sqlitePath := filepath.Join(tempDir, "data-proxy.db")
	t.Setenv("DATA_PROXY_RUNTIME_CONFIG", runtimeConfigPath)

	recorder := postSetupRuntimeConfig(t, map[string]any{
		"database_type": "sqlite",
		"sqlite_path":   sqlitePath,
	})

	response := decodeSetupRuntimeConfigResponse(t, recorder)
	require.True(t, response.Success)
	require.Contains(t, response.Message, "请重启 Data Proxy")
	require.True(t, common.RuntimeConfigLoaded)
	require.True(t, common.RuntimeConfigRestartRequired)
	require.Equal(t, runtimeConfigPath, common.RuntimeConfigPath)

	data, err := os.ReadFile(runtimeConfigPath)
	require.NoError(t, err)

	var cfg common.RuntimeConfig
	require.NoError(t, json.Unmarshal(data, &cfg))
	require.Equal(t, "local", cfg.SQLDSN)
	require.Equal(t, sqlitePath, cfg.SQLitePath)
	require.Empty(t, cfg.RedisConnString)
	require.NotZero(t, cfg.UpdatedAt)
}

func withSetupRuntimeConfigTestState(t *testing.T) string {
	t.Helper()
	gin.SetMode(gin.TestMode)

	originalSetup := constant.Setup
	originalRuntimeConfigLoaded := common.RuntimeConfigLoaded
	originalRuntimeConfigPath := common.RuntimeConfigPath
	originalRuntimeConfigRestartRequired := common.RuntimeConfigRestartRequired
	tempDir := t.TempDir()

	constant.Setup = false
	common.RuntimeConfigLoaded = false
	common.RuntimeConfigPath = ""
	common.RuntimeConfigRestartRequired = false
	t.Setenv("SQL_DSN", "")
	t.Setenv("LOG_SQL_DSN", "")
	t.Setenv("REDIS_CONN_STRING", "")
	t.Setenv("SQLITE_PATH", "")

	t.Cleanup(func() {
		constant.Setup = originalSetup
		common.RuntimeConfigLoaded = originalRuntimeConfigLoaded
		common.RuntimeConfigPath = originalRuntimeConfigPath
		common.RuntimeConfigRestartRequired = originalRuntimeConfigRestartRequired
	})
	return tempDir
}

func postSetupRuntimeConfig(t *testing.T, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()

	payload, err := json.Marshal(body)
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/setup/runtime-config", bytes.NewReader(payload))
	ctx.Request.Header.Set("Content-Type", "application/json")

	PostSetupRuntimeConfig(ctx)
	return recorder
}

func decodeSetupRuntimeConfigResponse(t *testing.T, recorder *httptest.ResponseRecorder) struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
} {
	t.Helper()

	var response struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	require.Equal(t, http.StatusOK, recorder.Code)
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	return response
}
