package controller

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestUpdateUserTokenGroupsPreserveAndClear(t *testing.T) {
	setupUserTokenGroupsControllerTestDB(t)
	require.NoError(t, model.DB.Create(&model.User{
		Id:          7001,
		Username:    "tg-user",
		DisplayName: "TG User",
		Password:    "hashed-password",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		Group:       "default",
		TokenGroups: `["default","vip"]`,
		AffCode:     "tg-user-aff",
	}).Error)

	preserveResp := requestUpdateUserForTokenGroupsTest(t, map[string]any{
		"id":           7001,
		"username":     "tg-user",
		"display_name": "Renamed",
		"role":         common.RoleCommonUser,
		"status":       common.UserStatusEnabled,
		"group":        "default",
	})
	require.True(t, preserveResp.Success, preserveResp.Message)
	var afterPreserve model.User
	require.NoError(t, model.DB.First(&afterPreserve, 7001).Error)
	require.Equal(t, []string{"default", "vip"}, afterPreserve.GetTokenGroups())

	clearResp := requestUpdateUserForTokenGroupsTest(t, map[string]any{
		"id":           7001,
		"username":     "tg-user",
		"display_name": "Renamed Again",
		"role":         common.RoleCommonUser,
		"status":       common.UserStatusEnabled,
		"group":        "default",
		"token_groups": []string{},
	})
	require.True(t, clearResp.Success, clearResp.Message)
	var afterClear model.User
	require.NoError(t, model.DB.First(&afterClear, 7001).Error)
	require.Empty(t, afterClear.GetTokenGroups())
}

func TestCreateUserPersistsInitialGroupBindings(t *testing.T) {
	setupUserTokenGroupsControllerTestDB(t)

	resp := requestCreateUserForTokenGroupsTest(t, map[string]any{
		"username":     "tg-create-user",
		"display_name": "TG Create User",
		"password":     "password1",
		"role":         common.RoleCommonUser,
		"group":        "vip",
		"token_groups": []string{"vip", "default", "vip", ""},
	})
	require.True(t, resp.Success, resp.Message)

	var created model.User
	require.NoError(t, model.DB.Where("username = ?", "tg-create-user").First(&created).Error)
	require.Equal(t, "vip", created.Group)
	require.Equal(t, []string{"vip", "default"}, created.GetTokenGroups())
}

type userTokenGroupsAPIResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func setupUserTokenGroupsControllerTestDB(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	common.UsingSQLite = true
	common.RedisEnabled = false
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}))

	originalDB := model.DB
	originalLOGDB := model.LOG_DB
	model.DB = db
	model.LOG_DB = db
	t.Cleanup(func() {
		model.DB = originalDB
		model.LOG_DB = originalLOGDB
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
}

func requestUpdateUserForTokenGroupsTest(t *testing.T, body map[string]any) userTokenGroupsAPIResponse {
	t.Helper()
	payload, err := common.Marshal(body)
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPut, "/api/user/", bytes.NewReader(payload))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set("role", common.RoleRootUser)

	UpdateUser(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response userTokenGroupsAPIResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	return response
}

func requestCreateUserForTokenGroupsTest(t *testing.T, body map[string]any) userTokenGroupsAPIResponse {
	t.Helper()
	payload, err := common.Marshal(body)
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/user/", bytes.NewReader(payload))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set("role", common.RoleRootUser)

	CreateUser(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response userTokenGroupsAPIResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	return response
}
