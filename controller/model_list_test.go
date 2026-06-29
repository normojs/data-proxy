package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type listModelsResponse struct {
	Success bool               `json:"success"`
	Data    []dto.OpenAIModels `json:"data"`
	Object  string             `json:"object"`
}

func setupModelListControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	initModelListColumnNames(t)

	gin.SetMode(gin.TestMode)
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db

	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Channel{}, &model.Ability{}, &model.Model{}, &model.Vendor{}))

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func initModelListColumnNames(t *testing.T) {
	t.Helper()

	originalIsMasterNode := common.IsMasterNode
	originalSQLitePath := common.SQLitePath
	originalUsingSQLite := common.UsingSQLite
	originalUsingMySQL := common.UsingMySQL
	originalUsingPostgreSQL := common.UsingPostgreSQL
	originalSQLDSN, hadSQLDSN := os.LookupEnv("SQL_DSN")
	defer func() {
		common.IsMasterNode = originalIsMasterNode
		common.SQLitePath = originalSQLitePath
		common.UsingSQLite = originalUsingSQLite
		common.UsingMySQL = originalUsingMySQL
		common.UsingPostgreSQL = originalUsingPostgreSQL
		if hadSQLDSN {
			require.NoError(t, os.Setenv("SQL_DSN", originalSQLDSN))
		} else {
			require.NoError(t, os.Unsetenv("SQL_DSN"))
		}
	}()

	common.IsMasterNode = false
	common.SQLitePath = fmt.Sprintf("file:%s_init?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	common.UsingSQLite = false
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	require.NoError(t, os.Setenv("SQL_DSN", "local"))

	require.NoError(t, model.InitDB())
	if model.DB != nil {
		sqlDB, err := model.DB.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	}
}

func withTieredBillingConfig(t *testing.T, modes map[string]string, exprs map[string]string) {
	t.Helper()

	saved := map[string]string{}
	require.NoError(t, config.GlobalConfig.SaveToDB(func(key, value string) error {
		if strings.HasPrefix(key, "billing_setting.") {
			saved[key] = value
		}
		return nil
	}))
	t.Cleanup(func() {
		require.NoError(t, config.GlobalConfig.LoadFromDB(saved))
		model.InvalidatePricingCache()
	})

	modeBytes, err := common.Marshal(modes)
	require.NoError(t, err)
	exprBytes, err := common.Marshal(exprs)
	require.NoError(t, err)

	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"billing_setting.billing_mode": string(modeBytes),
		"billing_setting.billing_expr": string(exprBytes),
	}))
	model.InvalidatePricingCache()
}

func withSelfUseModeDisabled(t *testing.T) {
	t.Helper()

	original := operation_setting.SelfUseModeEnabled
	operation_setting.SelfUseModeEnabled = false
	t.Cleanup(func() {
		operation_setting.SelfUseModeEnabled = original
	})
}

func decodeListModelsResponse(t *testing.T, recorder *httptest.ResponseRecorder) map[string]struct{} {
	t.Helper()

	require.Equal(t, http.StatusOK, recorder.Code)
	var payload listModelsResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.True(t, payload.Success)
	require.Equal(t, "list", payload.Object)

	ids := make(map[string]struct{}, len(payload.Data))
	for _, item := range payload.Data {
		ids[item.Id] = struct{}{}
	}
	return ids
}

func pricingByModelName(pricings []model.Pricing) map[string]model.Pricing {
	byName := make(map[string]model.Pricing, len(pricings))
	for _, pricing := range pricings {
		byName[pricing.ModelName] = pricing
	}
	return byName
}

func seedModelListChannelAbility(t *testing.T, db *gorm.DB, channelId int, subsiteId int64, group string, modelName string, channelType int) {
	t.Helper()
	priority := int64(10)
	require.NoError(t, db.Create(&model.Channel{
		Id:        channelId,
		SubsiteId: subsiteId,
		Type:      channelType,
		Key:       fmt.Sprintf("sk-model-list-%d", channelId),
		Status:    common.ChannelStatusEnabled,
		Name:      fmt.Sprintf("model-list-channel-%d", channelId),
		Models:    modelName,
		Group:     group,
		Priority:  &priority,
	}).Error)
	require.NoError(t, db.Create(&model.Ability{
		Group:     group,
		Model:     modelName,
		ChannelId: channelId,
		Enabled:   true,
		Priority:  &priority,
		Weight:    100,
	}).Error)
}

func setModelListChannelDisplayNames(t *testing.T, db *gorm.DB, channelId int, displayNames map[string]string) {
	t.Helper()
	channel := model.Channel{Id: channelId}
	settings := channel.GetOtherSettings()
	settings.SubsiteModelDisplayNames = displayNames
	channel.SetOtherSettings(settings)
	require.NoError(t, db.Model(&model.Channel{}).Where("id = ?", channelId).Update("settings", channel.OtherSettings).Error)
}

func TestListModelsIncludesTieredBillingModel(t *testing.T) {
	withSelfUseModeDisabled(t)
	withTieredBillingConfig(t, map[string]string{
		"zz-tiered-visible-model":      "tiered_expr",
		"zz-tiered-empty-expr-model":   "tiered_expr",
		"zz-tiered-missing-expr-model": "tiered_expr",
	}, map[string]string{
		"zz-tiered-visible-model":    `tier("base", p * 1 + c * 2)`,
		"zz-tiered-empty-expr-model": "   ",
	})

	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.Create(&model.User{
		Id:       1001,
		Username: "model-list-user",
		Password: "password",
		Group:    "default",
		Status:   common.UserStatusEnabled,
	}).Error)
	require.NoError(t, db.Create(&[]model.Ability{
		{Group: "default", Model: "zz-tiered-visible-model", ChannelId: 1, Enabled: true},
		{Group: "default", Model: "zz-tiered-empty-expr-model", ChannelId: 1, Enabled: true},
		{Group: "default", Model: "zz-tiered-missing-expr-model", ChannelId: 1, Enabled: true},
		{Group: "default", Model: "zz-unpriced-model", ChannelId: 1, Enabled: true},
	}).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	ctx.Set("id", 1001)

	ListModels(ctx, constant.ChannelTypeOpenAI)

	ids := decodeListModelsResponse(t, recorder)
	require.Contains(t, ids, "zz-tiered-visible-model")
	require.NotContains(t, ids, "zz-tiered-empty-expr-model")
	require.NotContains(t, ids, "zz-tiered-missing-expr-model")
	require.NotContains(t, ids, "zz-unpriced-model")

	pricingByName := pricingByModelName(model.GetPricing())
	visiblePricing, ok := pricingByName["zz-tiered-visible-model"]
	require.True(t, ok)
	require.Equal(t, "tiered_expr", visiblePricing.BillingMode)
	require.NotEmpty(t, visiblePricing.BillingExpr)

	emptyExprPricing, ok := pricingByName["zz-tiered-empty-expr-model"]
	require.True(t, ok)
	require.Empty(t, emptyExprPricing.BillingMode)
	require.Empty(t, emptyExprPricing.BillingExpr)

	missingExprPricing, ok := pricingByName["zz-tiered-missing-expr-model"]
	require.True(t, ok)
	require.Empty(t, missingExprPricing.BillingMode)
	require.Empty(t, missingExprPricing.BillingExpr)
}

func TestListModelsTokenLimitIncludesTieredBillingModel(t *testing.T) {
	withSelfUseModeDisabled(t)
	withTieredBillingConfig(t, map[string]string{
		"zz-token-tiered-visible-model":      "tiered_expr",
		"zz-token-tiered-empty-expr-model":   "tiered_expr",
		"zz-token-tiered-missing-expr-model": "tiered_expr",
	}, map[string]string{
		"zz-token-tiered-visible-model":    `tier("base", p * 1 + c * 2)`,
		"zz-token-tiered-empty-expr-model": "",
	})
	setupModelListControllerTestDB(t)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	common.SetContextKey(ctx, constant.ContextKeyTokenModelLimitEnabled, true)
	common.SetContextKey(ctx, constant.ContextKeyTokenModelLimit, map[string]bool{
		"zz-token-tiered-visible-model":      true,
		"zz-token-tiered-empty-expr-model":   true,
		"zz-token-tiered-missing-expr-model": true,
		"zz-token-unpriced-model":            true,
	})

	ListModels(ctx, constant.ChannelTypeOpenAI)

	ids := decodeListModelsResponse(t, recorder)
	require.Contains(t, ids, "zz-token-tiered-visible-model")
	require.NotContains(t, ids, "zz-token-tiered-empty-expr-model")
	require.NotContains(t, ids, "zz-token-tiered-missing-expr-model")
	require.NotContains(t, ids, "zz-token-unpriced-model")
}

func TestListModelsHonorsBoundTokenGroups(t *testing.T) {
	withSelfUseModeDisabled(t)
	withTieredBillingConfig(t, map[string]string{
		"zz-bound-default-model": "tiered_expr",
		"zz-bound-vip-model":     "tiered_expr",
	}, map[string]string{
		"zz-bound-default-model": `tier("base", p + c)`,
		"zz-bound-vip-model":     `tier("base", p + c)`,
	})
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.Create(&model.User{
		Id:          1002,
		Username:    "bound-model-user",
		Password:    "password",
		Group:       "vip",
		TokenGroups: `["default"]`,
		Status:      common.UserStatusEnabled,
	}).Error)
	require.NoError(t, db.Create(&[]model.Ability{
		{Group: "default", Model: "zz-bound-default-model", ChannelId: 1, Enabled: true},
		{Group: "vip", Model: "zz-bound-vip-model", ChannelId: 2, Enabled: true},
	}).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	ctx.Set("id", 1002)

	ListModels(ctx, constant.ChannelTypeOpenAI)

	ids := decodeListModelsResponse(t, recorder)
	require.Contains(t, ids, "zz-bound-default-model")
	require.NotContains(t, ids, "zz-bound-vip-model")
}

func TestListModelsReturnsEmptyForUnavailableTokenGroup(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.Create(&model.User{
		Id:          1003,
		Username:    "unavailable-model-user",
		Password:    "password",
		Group:       "vip",
		TokenGroups: `["default"]`,
		Status:      common.UserStatusEnabled,
	}).Error)
	require.NoError(t, db.Create(&model.Ability{
		Group: "vip", Model: "zz-hidden-vip-model", ChannelId: 1, Enabled: true,
	}).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	ctx.Set("id", 1003)
	common.SetContextKey(ctx, constant.ContextKeyUserGroup, "vip")
	common.SetContextKey(ctx, constant.ContextKeyUserTokenGroups, []string{"default"})
	common.SetContextKey(ctx, constant.ContextKeyTokenGroup, "vip")

	ListModels(ctx, constant.ChannelTypeOpenAI)

	ids := decodeListModelsResponse(t, recorder)
	require.Empty(t, ids)
}

func TestListModelsScopesSubsiteChannelModels(t *testing.T) {
	withSelfUseModeDisabled(t)
	withTieredBillingConfig(t, map[string]string{
		"zz-main-scope-model":    "tiered_expr",
		"zz-subsite-scope-model": "tiered_expr",
	}, map[string]string{
		"zz-main-scope-model":    `tier("base", p + c)`,
		"zz-subsite-scope-model": `tier("base", p + c)`,
	})
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.Create(&model.User{
		Id:       1004,
		Username: "subsite-model-user",
		Password: "password",
		Group:    "default",
		Status:   common.UserStatusEnabled,
	}).Error)
	seedModelListChannelAbility(t, db, 3001, 0, "default", "zz-main-scope-model", constant.ChannelTypeOpenAI)
	seedModelListChannelAbility(t, db, 3002, 42, "default", "zz-subsite-scope-model", constant.ChannelTypeOpenAI)

	mainRecorder := httptest.NewRecorder()
	mainCtx, _ := gin.CreateTestContext(mainRecorder)
	mainCtx.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	mainCtx.Set("id", 1004)
	ListModels(mainCtx, constant.ChannelTypeOpenAI)

	mainIds := decodeListModelsResponse(t, mainRecorder)
	require.Contains(t, mainIds, "zz-main-scope-model")
	require.NotContains(t, mainIds, "zz-subsite-scope-model")

	subsiteRecorder := httptest.NewRecorder()
	subsiteCtx, _ := gin.CreateTestContext(subsiteRecorder)
	subsiteCtx.Request = httptest.NewRequest(http.MethodGet, "/s/site-a/v1/models", nil)
	subsiteCtx.Set("id", 1004)
	subsiteCtx.Set("subsite_id", int64(42))
	ListModels(subsiteCtx, constant.ChannelTypeOpenAI)

	subsiteIds := decodeListModelsResponse(t, subsiteRecorder)
	require.Contains(t, subsiteIds, "zz-subsite-scope-model")
	require.NotContains(t, subsiteIds, "zz-main-scope-model")
}

func TestListModelsIncludesSubsiteModelDisplayName(t *testing.T) {
	withSelfUseModeDisabled(t)
	withTieredBillingConfig(t, map[string]string{
		"zz-subsite-display-model": "tiered_expr",
	}, map[string]string{
		"zz-subsite-display-model": `tier("base", p + c)`,
	})
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.Create(&model.User{
		Id:       1006,
		Username: "subsite-display-user",
		Password: "password",
		Group:    "default",
		Status:   common.UserStatusEnabled,
	}).Error)
	seedModelListChannelAbility(t, db, 3201, 42, "default", "zz-subsite-display-model", constant.ChannelTypeOpenAI)
	setModelListChannelDisplayNames(t, db, 3201, map[string]string{
		"zz-subsite-display-model": "Display Model",
	})

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/s/site-a/v1/models", nil)
	ctx.Set("id", 1006)
	ctx.Set("subsite_id", int64(42))
	ListModels(ctx, constant.ChannelTypeOpenAI)

	require.Equal(t, http.StatusOK, recorder.Code)
	var payload listModelsResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.True(t, payload.Success)
	require.Len(t, payload.Data, 1)
	require.Equal(t, "zz-subsite-display-model", payload.Data[0].Id)
	require.Equal(t, "Display Model", payload.Data[0].DisplayName)
}

func TestRetrieveModelHonorsSubsiteChannelScope(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NotEmpty(t, openAIModels)
	mainModel := openAIModels[0].Id
	subsiteModel := openAIModels[1].Id
	require.NotEqual(t, mainModel, subsiteModel)
	require.NoError(t, db.Create(&model.User{
		Id:       1005,
		Username: "subsite-retrieve-user",
		Password: "password",
		Group:    "default",
		Status:   common.UserStatusEnabled,
	}).Error)
	seedModelListChannelAbility(t, db, 3101, 0, "default", mainModel, constant.ChannelTypeOpenAI)
	seedModelListChannelAbility(t, db, 3102, 42, "default", subsiteModel, constant.ChannelTypeOpenAI)

	allowed := httptest.NewRecorder()
	allowedCtx, _ := gin.CreateTestContext(allowed)
	allowedCtx.Request = httptest.NewRequest(http.MethodGet, "/s/site-a/v1/models/"+subsiteModel, nil)
	allowedCtx.Params = gin.Params{{Key: "model", Value: subsiteModel}}
	allowedCtx.Set("id", 1005)
	allowedCtx.Set("subsite_id", int64(42))
	RetrieveModel(allowedCtx, constant.ChannelTypeOpenAI)
	require.Equal(t, http.StatusOK, allowed.Code)
	require.NotContains(t, allowed.Body.String(), "model_not_found")

	rejected := httptest.NewRecorder()
	rejectedCtx, _ := gin.CreateTestContext(rejected)
	rejectedCtx.Request = httptest.NewRequest(http.MethodGet, "/s/site-a/v1/models/"+mainModel, nil)
	rejectedCtx.Params = gin.Params{{Key: "model", Value: mainModel}}
	rejectedCtx.Set("id", 1005)
	rejectedCtx.Set("subsite_id", int64(42))
	RetrieveModel(rejectedCtx, constant.ChannelTypeOpenAI)
	require.Equal(t, http.StatusOK, rejected.Code)
	require.Contains(t, rejected.Body.String(), "model_not_found")
}
