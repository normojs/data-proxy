package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestTokenAuthRejectsLegacyTokenOutsideUserBoundGroups(t *testing.T) {
	setupTokenAuthGroupMiddlewareTestDB(t)

	require.NoError(t, model.DB.Create(&model.User{
		Id:          9701,
		Username:    "bound-relay-user",
		Status:      common.UserStatusEnabled,
		Group:       "default",
		TokenGroups: `["default"]`,
		AffCode:     "bound-relay-user-aff",
	}).Error)
	require.NoError(t, model.DB.Create(&model.Token{
		Id:             9801,
		UserId:         9701,
		Key:            "legacyviptoken",
		Name:           "legacy vip token",
		Status:         common.TokenStatusEnabled,
		ExpiredTime:    -1,
		UnlimitedQuota: true,
		Group:          "vip",
	}).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-test","messages":[]}`))
	ctx.Request.Header.Set("Authorization", "Bearer sk-legacyviptoken")

	nextCalled := false
	TokenAuth()(ctx)
	if !ctx.IsAborted() {
		nextCalled = true
	}

	require.False(t, nextCalled)
	require.Equal(t, http.StatusForbidden, recorder.Code, recorder.Body.String())
	require.Contains(t, recorder.Body.String(), "vip")
}

func setupTokenAuthGroupMiddlewareTestDB(t *testing.T) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	originalDB := model.DB
	originalLogDB := model.LOG_DB
	originalRedisEnabled := common.RedisEnabled
	originalUsingSQLite := common.UsingSQLite

	common.RedisEnabled = false
	common.UsingSQLite = true
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Token{}))

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
		model.DB = originalDB
		model.LOG_DB = originalLogDB
		common.RedisEnabled = originalRedisEnabled
		common.UsingSQLite = originalUsingSQLite
	})
}
