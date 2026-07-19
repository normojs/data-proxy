package controller

import (
	"encoding/json"
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

func setupTokenUsageTestDB(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	origSQLite := common.UsingSQLite
	origMySQL := common.UsingMySQL
	origPG := common.UsingPostgreSQL
	origRedis := common.RedisEnabled
	t.Cleanup(func() {
		common.UsingSQLite = origSQLite
		common.UsingMySQL = origMySQL
		common.UsingPostgreSQL = origPG
		common.RedisEnabled = origRedis
	})
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false
	dsn := "file:token_usage_" + strings.ReplaceAll(t.Name(), "/", "_") + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db
	require.NoError(t, db.AutoMigrate(
		&model.User{},
		&model.Token{},
		&model.ModelTokenPackage{},
		&model.UserSubscription{},
	))
	require.NoError(t, db.Create(&model.User{
		Id:          9101,
		Username:    "usage-user",
		DisplayName: "Usage User",
		Status:      common.UserStatusEnabled,
		Role:        common.RoleCommonUser,
		Group:       "default",
		Quota:       12345,
		UsedQuota:   100,
		AffCode:     "usageu",
	}).Error)
	require.NoError(t, db.Create(&model.Token{
		Id:             1,
		UserId:         9101,
		Key:            "usage-token-key-abcdef",
		Name:           "desktop",
		Status:         common.TokenStatusEnabled,
		RemainQuota:    0,
		UsedQuota:      0,
		UnlimitedQuota: true,
		ExpiredTime:    -1,
	}).Error)
}

func TestGetTokenUsageIncludesAccountSummary(t *testing.T) {
	setupTokenUsageTestDB(t)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/usage/token", nil)
	c.Request.Header.Set("Authorization", "Bearer sk-usage-token-key-abcdef")
	GetTokenUsage(c)
	require.Equal(t, http.StatusOK, w.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, true, body["code"])
	data, ok := body["data"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, true, data["unlimited_quota"])
	account, ok := data["account"].(map[string]any)
	require.True(t, ok, "account summary missing: %s", w.Body.String())
	require.EqualValues(t, 9101, account["user_id"])
	require.Equal(t, "usage-user", account["username"])
	wallet, ok := account["wallet"].(map[string]any)
	require.True(t, ok)
	require.EqualValues(t, 12345, wallet["quota"])
}

func TestGetAccountUsage(t *testing.T) {
	setupTokenUsageTestDB(t)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/usage/account", nil)
	c.Request.Header.Set("Authorization", "Bearer sk-usage-token-key-abcdef")
	c.Set("id", 9101)
	GetAccountUsage(c)
	require.Equal(t, http.StatusOK, w.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, true, body["success"])
	data, ok := body["data"].(map[string]any)
	require.True(t, ok)
	require.EqualValues(t, 9101, data["user_id"])
	require.EqualValues(t, 12345, data["quota_remaining"])
}
