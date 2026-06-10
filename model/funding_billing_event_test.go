package model

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func withTestQuotaPerUnit(t *testing.T, quotaPerUnit float64) {
	t.Helper()
	original := common.QuotaPerUnit
	common.QuotaPerUnit = quotaPerUnit
	t.Cleanup(func() {
		common.QuotaPerUnit = original
	})
}

func getBillingEventsForSource(t *testing.T, source string, sourceId string) []BillingEvent {
	t.Helper()
	var events []BillingEvent
	require.NoError(t, DB.Where("source = ? AND source_id = ?", source, sourceId).Order("id asc").Find(&events).Error)
	return events
}

func getBillingEventForPhase(t *testing.T, source string, sourceId string, phase string) BillingEvent {
	t.Helper()
	var event BillingEvent
	require.NoError(t, DB.Where("event_id = ?", modelBillingEventID(source, sourceId, phase)).First(&event).Error)
	return event
}

func decodeFundingBillingMetadata(t *testing.T, event BillingEvent) map[string]any {
	t.Helper()
	var metadata map[string]any
	require.NoError(t, json.Unmarshal([]byte(event.Metadata), &metadata))
	return metadata
}

func TestRechargeWaffoRecordsWalletTopUpBillingEvent(t *testing.T) {
	truncateTables(t)
	withTestQuotaPerUnit(t, 100)

	insertUserForPaymentGuardTest(t, 501, 0)
	insertTopUpForPaymentGuardTest(t, "waffo-ledger", 501, PaymentProviderWaffo)

	require.NoError(t, RechargeWaffo("waffo-ledger", "127.0.0.1"))

	event := getBillingEventForPhase(t, BillingEventSourceWalletTopUp, "waffo-ledger", "success")
	assert.Equal(t, 501, event.UserId)
	assert.Equal(t, BillingEventTypeCredit, event.EventType)
	assert.Equal(t, "wallet", event.BillingSource)
	assert.Equal(t, "topup", event.PriceUnit)
	assert.Equal(t, 200, event.AmountQuota)
	assert.Equal(t, 200, event.QuotaDelta)

	metadata := decodeFundingBillingMetadata(t, event)
	assert.Equal(t, PaymentProviderWaffo, metadata["channel"])
	assert.Equal(t, "waffo-ledger", metadata["trade_no"])

	require.NoError(t, RechargeWaffo("waffo-ledger", "127.0.0.1"))
	events := getBillingEventsForSource(t, BillingEventSourceWalletTopUp, "waffo-ledger")
	assert.Len(t, events, 1)
	assert.Equal(t, 200, getUserQuotaForPaymentGuardTest(t, 501))
}

func TestRedeemRecordsWalletTopUpBillingEvent(t *testing.T) {
	truncateTables(t)
	withTestQuotaPerUnit(t, 100)

	insertUserForPaymentGuardTest(t, 502, 0)
	redemption := &Redemption{
		UserId:      0,
		Key:         "1234567890abcdef1234567890abcdef",
		Status:      common.RedemptionCodeStatusEnabled,
		Name:        "Test Redemption",
		Quota:       321,
		CreatedTime: time.Now().Unix(),
	}
	require.NoError(t, redemption.Insert())

	quota, err := Redeem(redemption.Key, 502)
	require.NoError(t, err)
	assert.Equal(t, 321, quota)

	sourceId := fmt.Sprintf("redemption:%d", redemption.Id)
	event := getBillingEventForPhase(t, BillingEventSourceWalletTopUp, sourceId, "redemption")
	assert.Equal(t, 502, event.UserId)
	assert.Equal(t, BillingEventTypeCredit, event.EventType)
	assert.Equal(t, "redemption", event.PriceUnit)
	assert.Equal(t, 321, event.AmountQuota)
	assert.Equal(t, 321, event.QuotaDelta)

	metadata := decodeFundingBillingMetadata(t, event)
	assert.Equal(t, "redemption", metadata["channel"])
	assert.Equal(t, "Test Redemption", metadata["name"])
}

func TestCompleteSubscriptionOrderRecordsSubscriptionBillingEvent(t *testing.T) {
	truncateTables(t)
	withTestQuotaPerUnit(t, 100)

	insertUserForPaymentGuardTest(t, 503, 0)
	plan := insertSubscriptionPlanForPaymentGuardTest(t, 601)
	insertSubscriptionOrderForPaymentGuardTest(t, "sub-order-ledger", 503, plan.Id, PaymentProviderStripe)

	require.NoError(t, CompleteSubscriptionOrder("sub-order-ledger", `{"provider":"stripe"}`, PaymentProviderStripe, "card"))

	event := getBillingEventForPhase(t, BillingEventSourceSubscription, "sub-order-ledger", "purchase")
	assert.Equal(t, 503, event.UserId)
	assert.Equal(t, BillingEventTypeCredit, event.EventType)
	assert.Equal(t, "subscription", event.BillingSource)
	assert.Equal(t, "subscription", event.PriceUnit)
	assert.Equal(t, int(plan.TotalAmount), event.AmountQuota)
	assert.Equal(t, int(plan.TotalAmount), event.QuotaDelta)

	metadata := decodeFundingBillingMetadata(t, event)
	assert.Equal(t, "card", metadata["payment_method"])
	assert.Equal(t, PaymentProviderStripe, metadata["payment_provider"])
	assert.Equal(t, float64(plan.Id), metadata["plan_id"])

	require.NoError(t, CompleteSubscriptionOrder("sub-order-ledger", `{"provider":"stripe"}`, PaymentProviderStripe, "card"))
	events := getBillingEventsForSource(t, BillingEventSourceSubscription, "sub-order-ledger")
	assert.Len(t, events, 1)
	assert.EqualValues(t, 1, countUserSubscriptionsForPaymentGuardTest(t, 503))
}

func TestPurchaseSubscriptionWithBalanceRecordsDebitAndGrantEvents(t *testing.T) {
	truncateTables(t)
	withTestQuotaPerUnit(t, 100)

	insertUserForPaymentGuardTest(t, 504, 2000)
	plan := insertSubscriptionPlanForPaymentGuardTest(t, 602)
	requiredQuota, err := calcSubscriptionBalanceQuota(plan.PriceAmount)
	require.NoError(t, err)

	require.NoError(t, PurchaseSubscriptionWithBalance(504, plan.Id))

	var order SubscriptionOrder
	require.NoError(t, DB.Where("user_id = ? AND payment_provider = ?", 504, PaymentProviderBalance).First(&order).Error)

	paymentEvent := getBillingEventForPhase(t, BillingEventSourceSubscription, order.TradeNo, "balance_payment")
	assert.Equal(t, BillingEventTypeDebit, paymentEvent.EventType)
	assert.Equal(t, "wallet", paymentEvent.BillingSource)
	assert.Equal(t, "subscription_balance_payment", paymentEvent.PriceUnit)
	assert.Equal(t, requiredQuota, paymentEvent.AmountQuota)
	assert.Equal(t, -requiredQuota, paymentEvent.QuotaDelta)

	grantEvent := getBillingEventForPhase(t, BillingEventSourceSubscription, order.TradeNo, "grant")
	assert.Equal(t, BillingEventTypeCredit, grantEvent.EventType)
	assert.Equal(t, "subscription", grantEvent.BillingSource)
	assert.Equal(t, int(plan.TotalAmount), grantEvent.AmountQuota)
	assert.Equal(t, int(plan.TotalAmount), grantEvent.QuotaDelta)

	assert.Equal(t, 2000-requiredQuota, getUserQuotaForPaymentGuardTest(t, 504))
}

func TestAdminBindSubscriptionRecordsSubscriptionBillingEvent(t *testing.T) {
	truncateTables(t)
	withTestQuotaPerUnit(t, 100)

	insertUserForPaymentGuardTest(t, 505, 0)
	plan := insertSubscriptionPlanForPaymentGuardTest(t, 603)

	_, err := AdminBindSubscription(505, plan.Id, "manual grant")
	require.NoError(t, err)

	var sub UserSubscription
	require.NoError(t, DB.Where("user_id = ? AND plan_id = ?", 505, plan.Id).First(&sub).Error)

	event := getBillingEventForPhase(t, BillingEventSourceSubscription, fmt.Sprintf("admin_bind:%d", sub.Id), "admin_bind")
	assert.Equal(t, 505, event.UserId)
	assert.Equal(t, BillingEventTypeCredit, event.EventType)
	assert.Equal(t, int(plan.TotalAmount), event.AmountQuota)

	metadata := decodeFundingBillingMetadata(t, event)
	assert.Equal(t, "manual grant", metadata["source_note"])
}

func TestRecordWalletAdjustBillingEvent(t *testing.T) {
	truncateTables(t)
	withTestQuotaPerUnit(t, 100)

	insertUserForPaymentGuardTest(t, 506, 0)

	require.NoError(t, RecordWalletAdjustBillingEvent(506, "admin-adjust-ledger", BillingEventTypeDebit, 77, map[string]any{
		"mode":      "subtract",
		"old_quota": 100,
		"new_quota": 23,
	}))

	event := getBillingEventForPhase(t, BillingEventSourceWalletAdjust, "admin-adjust-ledger", "adjust")
	assert.Equal(t, 506, event.UserId)
	assert.Equal(t, BillingEventTypeDebit, event.EventType)
	assert.Equal(t, "wallet", event.BillingSource)
	assert.Equal(t, "manual_adjust", event.PriceUnit)
	assert.Equal(t, 77, event.AmountQuota)
	assert.Equal(t, -77, event.QuotaDelta)

	metadata := decodeFundingBillingMetadata(t, event)
	assert.Equal(t, "subtract", metadata["mode"])
}
