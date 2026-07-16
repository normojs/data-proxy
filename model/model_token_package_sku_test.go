package model

import (
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupModelTokenPackageSkuTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:model_token_package_sku_%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	DB = db
	LOG_DB = db
	require.NoError(t, db.AutoMigrate(
		&User{},
		&ModelTokenPackage{},
		&ModelTokenPackageLedger{},
		&ModelTokenPackageSku{},
		&BillingEvent{},
	))
	return db
}

func TestPurchaseModelTokenPackageSkuWithWallet(t *testing.T) {
	setupModelTokenPackageSkuTestDB(t)

	user := &User{Id: 42, Username: "buyer", Quota: 5000, Status: 1}
	require.NoError(t, DB.Create(user).Error)

	sku, err := CreateModelTokenPackageSku(ModelTokenPackageSkuInput{
		Name:        "GPT Pack",
		Models:      []string{"gpt-4o-mini"},
		TotalTokens: 10000,
		PriceQuota:  1200,
		Status:      ModelTokenPackageSkuStatusEnabled,
		CreatedBy:   1,
	})
	require.NoError(t, err)
	require.True(t, sku.IsEnabled())

	pkg, purchasedSku, err := PurchaseModelTokenPackageSkuWithWallet(42, sku.Id)
	require.NoError(t, err)
	require.NotNil(t, pkg)
	require.Equal(t, sku.Id, purchasedSku.Id)
	require.Equal(t, ModelTokenPackageSourcePurchase, pkg.Source)
	require.EqualValues(t, 10000, pkg.RemainingTokens)

	var updated User
	require.NoError(t, DB.Select("quota").First(&updated, 42).Error)
	require.Equal(t, 3800, updated.Quota)

	// insufficient balance
	user.Quota = 100
	require.NoError(t, DB.Model(&User{}).Where("id = ?", 42).Update("quota", 100).Error)
	_, _, err = PurchaseModelTokenPackageSkuWithWallet(42, sku.Id)
	require.ErrorIs(t, err, ErrModelTokenPackageSkuInsufficientQ)
}

func TestPurchaseModelTokenPackageSkuFreeDoesNotDebitWallet(t *testing.T) {
	setupModelTokenPackageSkuTestDB(t)

	user := &User{Id: 7, Username: "free-buyer", Quota: 50, Status: 1}
	require.NoError(t, DB.Create(user).Error)

	sku, err := CreateModelTokenPackageSku(ModelTokenPackageSkuInput{
		Name:        "Free Trial",
		Models:      []string{"gpt-4o-mini"},
		TotalTokens: 500,
		PriceQuota:  0,
		Status:      ModelTokenPackageSkuStatusEnabled,
	})
	require.NoError(t, err)

	pkg, _, err := PurchaseModelTokenPackageSkuWithWallet(7, sku.Id)
	require.NoError(t, err)
	require.EqualValues(t, 500, pkg.TotalTokens)

	var updated User
	require.NoError(t, DB.Select("quota").First(&updated, 7).Error)
	require.Equal(t, 50, updated.Quota)
}
