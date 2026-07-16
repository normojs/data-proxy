package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func withRedemptionTestDB(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&User{},
		&Redemption{},
		&ModelTokenPackage{},
		&ModelTokenPackageLedger{},
		&BillingEvent{},
		&Log{},
	))
	original := DB
	modelDB := db
	DB = modelDB
	t.Cleanup(func() {
		DB = original
	})
}

func TestRedeemPackageCode(t *testing.T) {
	withRedemptionTestDB(t)
	require.NoError(t, DB.Create(&User{
		Id:       11,
		Username: "redeem-user",
		Password: "password123",
		Status:   1,
		Role:     1,
		Group:    "default",
		AffCode:  "rd11",
		Quota:    100,
	}).Error)
	modelsJSON, err := EncodeModelTokenPackageModels([]string{"gpt-4o-mini"})
	require.NoError(t, err)
	code := &Redemption{
		UserId:             1,
		Key:                "pkg-redeem-key-001",
		Status:             common.RedemptionCodeStatusEnabled,
		Name:               "GPT Pack",
		RewardType:         RedemptionRewardTypeModelTokenPackage,
		PackageModelsJson:  modelsJSON,
		PackageTokens:      2500,
		PackageInputRatio:  1,
		PackageOutputRatio: 1,
		PackageCacheRatio:  1,
		CreatedTime:        common.GetTimestamp(),
	}
	require.NoError(t, code.Insert())

	result, err := Redeem(code.Key, 11)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, RedemptionRewardTypeModelTokenPackage, result.RewardType)
	assert.EqualValues(t, 2500, result.TotalTokens)
	assert.Greater(t, result.PackageId, 0)

	var user User
	require.NoError(t, DB.First(&user, 11).Error)
	assert.Equal(t, 100, user.Quota) // wallet unchanged

	packages, err := ListModelTokenPackagesByUser(11, true)
	require.NoError(t, err)
	require.Len(t, packages, 1)
	assert.EqualValues(t, 2500, packages[0].RemainingTokens)
	assert.Equal(t, ModelTokenPackageSourceRedeem, packages[0].Source)

	// second redeem fails
	_, err = Redeem(code.Key, 11)
	require.Error(t, err)
}

func TestRedeemQuotaCodeStillWorks(t *testing.T) {
	withRedemptionTestDB(t)
	require.NoError(t, DB.Create(&User{
		Id:       12,
		Username: "quota-user",
		Password: "password123",
		Status:   1,
		Role:     1,
		Group:    "default",
		AffCode:  "rd12",
		Quota:    10,
	}).Error)
	code := &Redemption{
		UserId:      1,
		Key:         "quota-redeem-key-001",
		Status:      common.RedemptionCodeStatusEnabled,
		Name:        "Wallet Code",
		RewardType:  RedemptionRewardTypeQuota,
		Quota:       500,
		CreatedTime: common.GetTimestamp(),
	}
	require.NoError(t, code.Insert())
	result, err := Redeem(code.Key, 12)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, RedemptionRewardTypeQuota, result.RewardType)
	assert.Equal(t, 500, result.Quota)
	var user User
	require.NoError(t, DB.First(&user, 12).Error)
	assert.Equal(t, 510, user.Quota)
}
