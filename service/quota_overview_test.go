package service

import (
	"testing"

	"github.com/QuantumNous/new-api/model"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func withQuotaOverviewTestDB(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.User{},
		&model.ModelTokenPackage{},
		&model.ModelTokenPackageLedger{},
		&model.UserSubscription{},
		&model.Token{},
	))
	original := model.DB
	model.DB = db
	t.Cleanup(func() {
		model.DB = original
	})
}

func TestBuildUserQuotaOverviewEmptyUser(t *testing.T) {
	withQuotaOverviewTestDB(t)
	require.NoError(t, model.DB.Create(&model.User{
		Id:           42,
		Username:     "overview-user",
		Password:     "password123",
		Quota:        1000,
		UsedQuota:    200,
		RequestCount: 3,
		Status:       1,
		Role:         1,
		Group:        "default",
		AffCode:      "ov42",
	}).Error)

	overview, err := BuildUserQuotaOverview(42)
	require.NoError(t, err)
	require.NotNil(t, overview)
	assert.Equal(t, 1000, overview.Wallet.Quota)
	assert.Equal(t, 200, overview.Wallet.UsedQuota)
	assert.Equal(t, "quota_points", overview.Units.Wallet)
	assert.Equal(t, "llm_tokens", overview.Units.ModelTokenPackage)
	assert.Equal(t, "empty", overview.ModelTokenPackages.Status)
	assert.Equal(t, "empty", overview.Subscriptions.Status)
	assert.Equal(t, "empty", overview.APIKeyHardLimits.Status)
	assert.Equal(t, "/wallet", overview.Links.Wallet)
	assert.Equal(t, "/keys", overview.Links.APIKeys)
}

func TestBuildUserQuotaOverviewWithPackageAndKey(t *testing.T) {
	withQuotaOverviewTestDB(t)
	require.NoError(t, model.DB.Create(&model.User{
		Id:       43,
		Username: "pack-user",
		Password: "password123",
		Quota:    50,
		Status:   1,
		Role:     1,
		Group:    "default",
		AffCode:  "ov43",
	}).Error)
	_, err := model.CreateModelTokenPackage(model.ModelTokenPackageCreateInput{
		UserId:      43,
		Name:        "gpt pack",
		Models:      []string{"gpt-4o"},
		TotalTokens: 5000,
		InputRatio:  1,
		OutputRatio: 1,
		CacheRatio:  1,
	})
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.Token{
		UserId:                43,
		Key:                   "sk-overview-test-key-0001",
		Name:                  "limited",
		Status:                1,
		RemainQuota:           300,
		UsedQuota:             100,
		UnlimitedQuota:        false,
		QuotaHardLimitEnabled: false,
	}).Error)

	overview, err := BuildUserQuotaOverview(43)
	require.NoError(t, err)
	assert.Equal(t, "ok", overview.ModelTokenPackages.Status)
	assert.Equal(t, 1, overview.ModelTokenPackages.ActiveCount)
	assert.EqualValues(t, 5000, overview.ModelTokenPackages.RemainingTokens)
	assert.Equal(t, "ok", overview.APIKeyHardLimits.Status)
	assert.Equal(t, 1, overview.APIKeyHardLimits.LimitedCount)
	assert.EqualValues(t, 300, overview.APIKeyHardLimits.RemainingQuota)
	require.Len(t, overview.APIKeyHardLimits.Items, 1)
	assert.Equal(t, "limited", overview.APIKeyHardLimits.Items[0].Name)
}
