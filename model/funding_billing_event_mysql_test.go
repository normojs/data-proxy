package model

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/stretchr/testify/require"
)

func TestFundingBillingEventMySQLSmoke(t *testing.T) {
	if os.Getenv("MCP_MIGRATION_TEST") != "1" {
		t.Skip("set MCP_MIGRATION_TEST=1 to run the funding billing event MySQL smoke test")
	}
	if os.Getenv("SQL_DSN") == "" {
		t.Fatal("SQL_DSN is required")
	}

	common.InitEnv()
	logger.SetupLogger()

	if err := InitDB(); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	if err := InitLogDB(); err != nil {
		t.Fatalf("InitLogDB failed: %v", err)
	}
	t.Cleanup(func() {
		_ = CloseDB()
	})

	suffix := fmt.Sprintf("funding-mysql-smoke-%d", time.Now().UnixNano())
	sourceIds := []string{
		suffix + ":topup",
		suffix + ":adjust",
		suffix + ":subscription",
	}
	t.Cleanup(func() {
		_ = DB.Where("source_id IN ?", sourceIds).Delete(&BillingEvent{}).Error
		_ = DB.Unscoped().Where("username = ?", suffix).Delete(&User{}).Error
	})

	user := &User{
		Username: suffix,
		Status:   common.UserStatusEnabled,
		Quota:    0,
	}
	require.NoError(t, DB.Create(user).Error)

	topUp := &TopUp{
		UserId:          user.Id,
		Amount:          1,
		Money:           1,
		TradeNo:         sourceIds[0],
		PaymentMethod:   PaymentMethodWaffo,
		PaymentProvider: PaymentProviderWaffo,
		Status:          common.TopUpStatusSuccess,
		CompleteTime:    common.GetTimestamp(),
	}
	require.NoError(t, RecordWalletTopUpBillingEvent(nil, topUp, 100, "mysql_smoke", map[string]any{
		"channel": "mysql_smoke",
	}))
	require.NoError(t, RecordWalletAdjustBillingEvent(user.Id, sourceIds[1], BillingEventTypeDebit, 40, map[string]any{
		"mode": "mysql_smoke",
	}))
	require.NoError(t, RecordSubscriptionBillingEvent(nil, sourceIds[2], "mysql_smoke", user.Id, 0, BillingEventTypeCredit, map[string]any{
		"plan_title": "MySQL Smoke",
	}))

	var events []BillingEvent
	require.NoError(t, DB.Where("source_id IN ?", sourceIds).Order("id asc").Find(&events).Error)
	require.Len(t, events, 3)
	require.Equal(t, BillingEventSourceWalletTopUp, events[0].Source)
	require.Equal(t, BillingEventTypeCredit, events[0].EventType)
	require.Equal(t, 100, events[0].QuotaDelta)
	require.Equal(t, BillingEventSourceWalletAdjust, events[1].Source)
	require.Equal(t, BillingEventTypeDebit, events[1].EventType)
	require.Equal(t, -40, events[1].QuotaDelta)
	require.Equal(t, BillingEventSourceSubscription, events[2].Source)
	require.Equal(t, BillingEventTypeCredit, events[2].EventType)
	require.Equal(t, 0, events[2].AmountQuota)
}
