package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestApplyAdminWalletAdjustRollsBackWhenLedgerFails(t *testing.T) {
	setupUserWalletAdjustTestDB(t)
	seedWalletAdjustUser(t, 5001, 1000)

	_, _, err := applyAdminWalletAdjust(5001, map[string]interface{}{
		"admin_id": 7,
		"invalid":  func() {},
	}, "add", model.BillingEventTypeCredit, 100)
	require.Error(t, err)
	require.Equal(t, 1000, walletAdjustUserQuota(t, 5001))

	var count int64
	require.NoError(t, model.DB.Model(&model.BillingEvent{}).Count(&count).Error)
	require.EqualValues(t, 0, count)
	require.NoError(t, model.DB.Model(&model.WalletAdjustment{}).Count(&count).Error)
	require.EqualValues(t, 0, count)
}

func TestAdminWalletAdjustWritesQuotaAndLedgerTogether(t *testing.T) {
	setupUserWalletAdjustTestDB(t)
	seedWalletAdjustUser(t, 5002, 1000)

	adminInfo := map[string]interface{}{"admin_id": 8, "admin_name": "tester"}
	oldQuota, newQuota, err := applyAdminWalletAdjust(5002, adminInfo, "add", model.BillingEventTypeCredit, 250)
	require.NoError(t, err)
	require.Equal(t, 1000, oldQuota)
	require.Equal(t, 1250, newQuota)
	require.Equal(t, 1250, walletAdjustUserQuota(t, 5002))

	credit := walletAdjustBillingEvent(t, 5002, model.BillingEventTypeCredit)
	require.Equal(t, 250, credit.AmountQuota)
	require.Equal(t, 250, credit.QuotaDelta)
	require.Equal(t, model.BillingEventStatusSettled, credit.Status)
	creditAdjustment := walletAdjustmentSource(t, credit.SourceId)
	require.Equal(t, 5002, creditAdjustment.UserId)
	require.Equal(t, 8, creditAdjustment.AdminId)
	require.Equal(t, "add", creditAdjustment.Mode)
	require.Equal(t, 1000, creditAdjustment.OldQuota)
	require.Equal(t, 1250, creditAdjustment.NewQuota)
	require.Equal(t, credit.AmountQuota, creditAdjustment.Amount)

	oldQuota, err = overrideAdminWalletQuota(5002, adminInfo, 1200)
	require.NoError(t, err)
	require.Equal(t, 1250, oldQuota)
	require.Equal(t, 1200, walletAdjustUserQuota(t, 5002))

	debit := walletAdjustBillingEvent(t, 5002, model.BillingEventTypeDebit)
	require.Equal(t, 50, debit.AmountQuota)
	require.Equal(t, -50, debit.QuotaDelta)
	require.Equal(t, model.BillingEventStatusSettled, debit.Status)
	debitAdjustment := walletAdjustmentSource(t, debit.SourceId)
	require.Equal(t, 5002, debitAdjustment.UserId)
	require.Equal(t, 8, debitAdjustment.AdminId)
	require.Equal(t, "override", debitAdjustment.Mode)
	require.Equal(t, 1250, debitAdjustment.OldQuota)
	require.Equal(t, 1200, debitAdjustment.NewQuota)
	require.Equal(t, debit.AmountQuota, debitAdjustment.Amount)
}

func setupUserWalletAdjustTestDB(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.BillingEvent{}, &model.WalletAdjustment{}))
	model.DB = db
	model.LOG_DB = db
	common.UsingSQLite = true
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
}

func seedWalletAdjustUser(t *testing.T, userId int, quota int) {
	t.Helper()
	require.NoError(t, model.DB.Create(&model.User{
		Id:       userId,
		Username: "wallet_adjust_user",
		Status:   common.UserStatusEnabled,
		Quota:    quota,
	}).Error)
}

func walletAdjustUserQuota(t *testing.T, userId int) int {
	t.Helper()
	var user model.User
	require.NoError(t, model.DB.Select("quota").Where("id = ?", userId).First(&user).Error)
	return user.Quota
}

func walletAdjustBillingEvent(t *testing.T, userId int, eventType string) model.BillingEvent {
	t.Helper()
	var event model.BillingEvent
	require.NoError(t, model.DB.Where("user_id = ? AND source = ? AND event_type = ?",
		userId, model.BillingEventSourceWalletAdjust, eventType).
		First(&event).Error)
	return event
}

func walletAdjustmentSource(t *testing.T, sourceId string) model.WalletAdjustment {
	t.Helper()
	var adjustment model.WalletAdjustment
	require.NoError(t, model.DB.Where("source_id = ?", sourceId).First(&adjustment).Error)
	return adjustment
}
