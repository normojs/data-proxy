package service

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/stretchr/testify/require"
)

func TestBackfillBillingEventsDryRunDoesNotWrite(t *testing.T) {
	truncate(t)
	withServiceTestQuotaPerUnit(t, 100)
	seedBackfillWalletTopUp(t, "backfill-dryrun-topup", 901, model.PaymentProviderWaffo, 3, 3.00)

	result, err := BackfillBillingEvents(BillingEventBackfillParams{
		Sources: []string{BillingEventBackfillSourceWalletTopUp},
		Limit:   10,
		DryRun:  true,
	})
	require.NoError(t, err)
	require.Equal(t, 1, result.TotalScanned)
	require.Equal(t, 1, result.TotalWouldCreate)
	require.Equal(t, 0, result.TotalCreated)

	var count int64
	require.NoError(t, model.DB.Model(&model.BillingEvent{}).Count(&count).Error)
	require.EqualValues(t, 0, count)
}

func TestBillingEventSourceHandlersCoverDefaultSources(t *testing.T) {
	require.NotEmpty(t, defaultBillingEventBackfillSources)
	require.Equal(t, len(defaultBillingEventBackfillSources), len(billingEventSourceHandlers))

	for _, source := range defaultBillingEventBackfillSources {
		handler, ok := getBillingEventSourceHandler(source)
		require.True(t, ok, "missing handler for %s", source)
		require.Equal(t, source, handler.Source)
		require.NotNil(t, handler.Backfill, "missing backfill handler for %s", source)
		require.NotNil(t, handler.Reconcile, "missing reconcile handler for %s", source)
		require.NotNil(t, handler.MissingBackfillInput, "missing missing-backfill handler for %s", source)
	}
}

func TestBackfillBillingEventsCreatesFundingEventsIdempotently(t *testing.T) {
	truncate(t)
	withServiceTestQuotaPerUnit(t, 100)

	seedBackfillWalletTopUp(t, "backfill-wallet", 902, model.PaymentProviderWaffo, 4, 4.00)
	seedBackfillSubscriptionTopUpMirror(t, "backfill-sub-purchase", 903)
	plan := seedBackfillPlan(t, 910, 1000, 9.99)
	seedBackfillSubscriptionOrder(t, "backfill-sub-purchase", 903, plan.Id, model.PaymentProviderStripe, model.PaymentMethodStripe, 9.99)
	seedBackfillUserSubscription(t, 920, 903, plan.Id, "order")
	seedBackfillSubscriptionOrder(t, "backfill-sub-balance", 904, plan.Id, model.PaymentProviderBalance, model.PaymentMethodBalance, 9.99)
	seedBackfillUserSubscription(t, 921, 904, plan.Id, model.PaymentMethodBalance)
	seedBackfillUserSubscription(t, 922, 905, plan.Id, "admin")
	seedBackfillRedemption(t, 930, 906, 321)

	result, err := BackfillBillingEvents(BillingEventBackfillParams{
		Sources: []string{"all"},
		Limit:   20,
		DryRun:  false,
	})
	require.NoError(t, err)
	require.Equal(t, 5, result.TotalScanned)
	require.Equal(t, 6, result.TotalCreated)
	require.Equal(t, 0, result.TotalErrorCount)

	assertBillingEventCount(t, model.BillingEventSourceWalletTopUp, "backfill-wallet", 1)
	assertBillingEventCount(t, model.BillingEventSourceWalletTopUp, "backfill-sub-purchase", 0)
	assertBillingEventCount(t, model.BillingEventSourceWalletTopUp, "redemption:930", 1)
	assertBillingEventCount(t, model.BillingEventSourceSubscription, "backfill-sub-purchase", 1)
	assertBillingEventCount(t, model.BillingEventSourceSubscription, "backfill-sub-balance", 2)
	assertBillingEventCount(t, model.BillingEventSourceSubscription, "admin_bind:922", 1)

	again, err := BackfillBillingEvents(BillingEventBackfillParams{
		Sources: []string{"all"},
		Limit:   20,
		DryRun:  false,
	})
	require.NoError(t, err)
	require.Equal(t, 0, again.TotalCreated)

	var total int64
	require.NoError(t, model.DB.Model(&model.BillingEvent{}).Count(&total).Error)
	require.EqualValues(t, 6, total)
}

func TestBackfillBillingEventsCreatesWalletAdjustEvents(t *testing.T) {
	truncate(t)
	withServiceTestQuotaPerUnit(t, 100)

	seedBackfillWalletAdjustment(t, "wallet-adjust-backfill-existing", 931, 7, "add", model.BillingEventTypeCredit, 123, 1000, 1123)
	seedBackfillWalletAdjustment(t, "wallet-adjust-backfill-missing", 932, 7, "override", model.BillingEventTypeDebit, 45, 1123, 1078)
	created, err := model.RecordFundingBillingEventIfNotExists(nil, model.FundingBillingEventInput{
		Source:        model.BillingEventSourceWalletAdjust,
		SourceId:      "wallet-adjust-backfill-existing",
		Phase:         "adjust",
		UserId:        931,
		RequestId:     "wallet-adjust-backfill-existing",
		BillingSource: BillingSourceWallet,
		PriceUnit:     "manual_adjust",
		EventType:     model.BillingEventTypeCredit,
		AmountQuota:   123,
	})
	require.NoError(t, err)
	require.True(t, created)

	preview, err := BackfillBillingEvents(BillingEventBackfillParams{
		Sources: []string{BillingEventBackfillSourceWalletAdjust},
		Limit:   20,
		DryRun:  true,
	})
	require.NoError(t, err)
	require.Equal(t, 1, preview.TotalScanned)
	require.Equal(t, 1, preview.TotalWouldCreate)
	require.Equal(t, 0, preview.TotalCreated)

	result, err := BackfillBillingEvents(BillingEventBackfillParams{
		Sources: []string{BillingEventBackfillSourceWalletAdjust},
		Limit:   20,
		DryRun:  false,
	})
	require.NoError(t, err)
	require.Equal(t, 1, result.TotalScanned)
	require.Equal(t, 1, result.TotalCreated)

	event := requireFundingBillingEvent(t, model.BillingEventSourceWalletAdjust, "wallet-adjust-backfill-missing", "adjust")
	require.Equal(t, 932, event.UserId)
	require.Equal(t, model.BillingEventTypeDebit, event.EventType)
	require.Equal(t, 45, event.AmountQuota)
	require.Equal(t, -45, event.QuotaDelta)
	require.Equal(t, "manual_adjust", event.PriceUnit)
	requireBillingEventMetadataValue(t, event, "wallet_adjustment_id", float64(2))
	requireBillingEventMetadataValue(t, event, "admin_id", float64(7))
	requireBillingEventMetadataValue(t, event, "mode", "override")
}

func TestBackfillBillingEventsCreatesAsyncTaskEvents(t *testing.T) {
	truncate(t)
	withServiceTestQuotaPerUnit(t, 100)

	seedBackfillTaskBillingRecord(t, "task-backfill-existing", taskBillingEventPhaseInitialSettlement, 951, 51, BillingSourceWallet, model.BillingEventTypeDebit, 123, -123)
	seedBackfillTaskBillingRecord(t, "task-backfill-missing", taskBillingEventPhaseFailureRefund, 952, 52, BillingSourceSubscription, model.BillingEventTypeCredit, 45, 45)
	created, err := model.RecordFundingBillingEventIfNotExists(nil, model.FundingBillingEventInput{
		Source:        model.BillingEventSourceAsyncTask,
		SourceId:      "task-backfill-existing",
		Phase:         taskBillingEventPhaseInitialSettlement,
		UserId:        951,
		TokenId:       51,
		RequestId:     "task-backfill-existing",
		Group:         "default",
		BillingSource: BillingSourceWallet,
		PriceUnit:     "task",
		EventType:     model.BillingEventTypeDebit,
		AmountQuota:   123,
	})
	require.NoError(t, err)
	require.True(t, created)

	preview, err := BackfillBillingEvents(BillingEventBackfillParams{
		Sources: []string{BillingEventBackfillSourceAsyncTask},
		Limit:   20,
		DryRun:  true,
	})
	require.NoError(t, err)
	require.Equal(t, 1, preview.TotalScanned)
	require.Equal(t, 1, preview.TotalWouldCreate)
	require.Equal(t, 0, preview.TotalCreated)

	result, err := BackfillBillingEvents(BillingEventBackfillParams{
		Sources: []string{BillingEventBackfillSourceAsyncTask},
		Limit:   20,
		DryRun:  false,
	})
	require.NoError(t, err)
	require.Equal(t, 1, result.TotalScanned)
	require.Equal(t, 1, result.TotalCreated)

	event := requireFundingBillingEvent(t, model.BillingEventSourceAsyncTask, "task-backfill-missing", taskBillingEventPhaseFailureRefund)
	require.Equal(t, 952, event.UserId)
	require.Equal(t, 52, event.TokenId)
	require.Equal(t, model.BillingEventTypeCredit, event.EventType)
	require.Equal(t, 45, event.AmountQuota)
	require.Equal(t, 45, event.QuotaDelta)
	require.Equal(t, BillingSourceSubscription, event.BillingSource)
	require.Equal(t, "task_refund", event.PriceUnit)
	requireBillingEventMetadataValue(t, event, "task_id", "task-backfill-missing")
	requireBillingEventMetadataValue(t, event, "phase", taskBillingEventPhaseFailureRefund)
	requireBillingEventMetadataValue(t, event, "task_billing_record_id", float64(2))
}

func TestBackfillBillingEventsCreatesViolationFeeEvents(t *testing.T) {
	truncate(t)
	withServiceTestQuotaPerUnit(t, 100)

	seedBackfillViolationFeeRecord(t, "violation-backfill-existing", "violation_fee.grok_csam", 953, 53, BillingSourceWallet, 123)
	seedBackfillViolationFeeRecord(t, "violation-backfill-missing", "violation_fee.grok_csam", 954, 54, BillingSourceSubscription, 45)
	created, err := model.RecordFundingBillingEventIfNotExists(nil, model.FundingBillingEventInput{
		Source:        model.BillingEventSourceViolationFee,
		SourceId:      "violation-backfill-existing",
		Phase:         "violation_fee.grok_csam",
		UserId:        953,
		TokenId:       53,
		RequestId:     "violation-backfill-existing",
		Group:         "default",
		BillingSource: BillingSourceWallet,
		PriceUnit:     "violation_fee",
		EventType:     model.BillingEventTypeDebit,
		AmountQuota:   123,
	})
	require.NoError(t, err)
	require.True(t, created)

	preview, err := BackfillBillingEvents(BillingEventBackfillParams{
		Sources: []string{BillingEventBackfillSourceViolationFee},
		Limit:   20,
		DryRun:  true,
	})
	require.NoError(t, err)
	require.Equal(t, 1, preview.TotalScanned)
	require.Equal(t, 1, preview.TotalWouldCreate)
	require.Equal(t, 0, preview.TotalCreated)

	result, err := BackfillBillingEvents(BillingEventBackfillParams{
		Sources: []string{BillingEventBackfillSourceViolationFee},
		Limit:   20,
		DryRun:  false,
	})
	require.NoError(t, err)
	require.Equal(t, 1, result.TotalScanned)
	require.Equal(t, 1, result.TotalCreated)

	event := requireFundingBillingEvent(t, model.BillingEventSourceViolationFee, "violation-backfill-missing", "violation_fee.grok_csam")
	require.Equal(t, 954, event.UserId)
	require.Equal(t, 54, event.TokenId)
	require.Equal(t, model.BillingEventTypeDebit, event.EventType)
	require.Equal(t, 45, event.AmountQuota)
	require.Equal(t, -45, event.QuotaDelta)
	require.Equal(t, BillingSourceSubscription, event.BillingSource)
	require.Equal(t, "violation_fee", event.PriceUnit)
	requireBillingEventMetadataValue(t, event, "violation_fee", true)
	requireBillingEventMetadataValue(t, event, "violation_fee_code", "violation_fee.grok_csam")
	requireBillingEventMetadataValue(t, event, "violation_fee_record_id", float64(2))
}

func TestBackfillBillingEventsCreatesMCPToolCallEvents(t *testing.T) {
	truncate(t)
	withServiceTestQuotaPerUnit(t, 100)

	user, token := seedMCPBillingUserAndToken(t, 10000, 10000, false)
	tool := setMCPToolPriceForTest(t, "remote_read", 0.01)
	call := seedBackfillMCPToolCall(t, user.Id, token.Id, tool, "mcp-backfill-missing", model.MCPToolCallStatusSuccess, 100, tool.PricePerCall, 1234)

	preview, err := BackfillBillingEvents(BillingEventBackfillParams{
		Sources: []string{BillingEventBackfillSourceMCPToolCall},
		Limit:   20,
		DryRun:  true,
	})
	require.NoError(t, err)
	require.Equal(t, 1, preview.TotalScanned)
	require.Equal(t, 1, preview.TotalWouldCreate)
	require.Equal(t, 0, preview.TotalCreated)

	result, err := BackfillBillingEvents(BillingEventBackfillParams{
		Sources: []string{BillingEventBackfillSourceMCPToolCall},
		Limit:   20,
		DryRun:  false,
	})
	require.NoError(t, err)
	require.Equal(t, 1, result.TotalScanned)
	require.Equal(t, 1, result.TotalCreated)

	event := requireFundingBillingEvent(t, model.BillingEventSourceMCPToolCall, fmt.Sprintf("%d", call.Id), "settlement")
	require.Equal(t, user.Id, event.UserId)
	require.Equal(t, token.Id, event.TokenId)
	require.Equal(t, model.BillingEventTypeDebit, event.EventType)
	require.Equal(t, 100, event.AmountQuota)
	require.Equal(t, -100, event.QuotaDelta)
	require.Equal(t, model.MCPToolPriceUnitPerCall, event.PriceUnit)
	requireBillingEventMetadataValue(t, event, "mcp_tool_call_id", float64(call.Id))
	requireBillingEventMetadataValue(t, event, "tool_name", tool.Name)
}

func TestBackfillBillingEventsUsesSubscriptionSnapshotQuota(t *testing.T) {
	truncate(t)
	withServiceTestQuotaPerUnit(t, 100)

	plan := seedBackfillPlan(t, 940, 5000, 9.99)
	seedBackfillSubscriptionOrder(t, "backfill-snapshot-purchase", 941, plan.Id, model.PaymentProviderStripe, model.PaymentMethodStripe, 9.99)
	seedBackfillUserSubscriptionWithAmount(t, 942, 941, plan.Id, "order", 1234)
	seedBackfillSubscriptionOrder(t, "backfill-snapshot-balance", 943, plan.Id, model.PaymentProviderBalance, model.PaymentMethodBalance, 9.99)
	seedBackfillUserSubscriptionWithAmount(t, 944, 943, plan.Id, model.PaymentMethodBalance, 2345)
	seedBackfillUserSubscriptionWithAmount(t, 945, 946, plan.Id, "admin", 3456)

	result, err := BackfillBillingEvents(BillingEventBackfillParams{
		Sources: []string{BillingEventBackfillSourceSubscription},
		Limit:   10,
		DryRun:  false,
	})
	require.NoError(t, err)
	require.Equal(t, 3, result.TotalScanned)
	require.Equal(t, 4, result.TotalCreated)

	purchase := requireFundingBillingEvent(t, model.BillingEventSourceSubscription, "backfill-snapshot-purchase", "purchase")
	require.Equal(t, 1234, purchase.AmountQuota)
	require.Equal(t, 1234, purchase.QuotaDelta)
	requireBillingEventMetadataValue(t, purchase, "quota_from", "user_subscription")

	balancePayment := requireFundingBillingEvent(t, model.BillingEventSourceSubscription, "backfill-snapshot-balance", "balance_payment")
	require.Equal(t, 999, balancePayment.AmountQuota)
	require.Equal(t, -999, balancePayment.QuotaDelta)

	balanceGrant := requireFundingBillingEvent(t, model.BillingEventSourceSubscription, "backfill-snapshot-balance", "grant")
	require.Equal(t, 2345, balanceGrant.AmountQuota)
	require.Equal(t, 2345, balanceGrant.QuotaDelta)
	requireBillingEventMetadataValue(t, balanceGrant, "quota_from", "user_subscription")

	adminGrant := requireFundingBillingEvent(t, model.BillingEventSourceSubscription, "admin_bind:945", "admin_bind")
	require.Equal(t, 3456, adminGrant.AmountQuota)
	require.Equal(t, 3456, adminGrant.QuotaDelta)
	requireBillingEventMetadataValue(t, adminGrant, "quota_from", "user_subscription")

	reconciled, err := ReconcileBillingEvents(BillingEventReconciliationParams{
		Sources: []string{BillingEventBackfillSourceSubscription},
		Limit:   10,
	})
	require.NoError(t, err)
	require.Equal(t, 3, reconciled.TotalScanned)
	require.Equal(t, 4, reconciled.TotalExpected)
	require.Equal(t, 4, reconciled.TotalLedgered)
	require.Equal(t, 0, reconciled.TotalMismatched)
	require.Equal(t, 0, reconciled.TotalMissing)
}

func TestBackfillBillingEventsMatchesSubscriptionSnapshotByOrderTime(t *testing.T) {
	truncate(t)
	withServiceTestQuotaPerUnit(t, 100)

	userId := 946
	plan := seedBackfillPlan(t, 947, 5000, 9.99)
	firstOrder := seedBackfillSubscriptionOrderWithTimes(t, "backfill-snapshot-first-order", userId, plan.Id, model.PaymentProviderStripe, model.PaymentMethodStripe, 9.99, 1000, 1100)
	secondOrder := seedBackfillSubscriptionOrderWithTimes(t, "backfill-snapshot-second-order", userId, plan.Id, model.PaymentProviderStripe, model.PaymentMethodStripe, 9.99, 2000, 2100)
	seedBackfillUserSubscriptionWithTimes(t, 948, userId, plan.Id, "order", 1111, firstOrder.CompleteTime+10)
	seedBackfillUserSubscriptionWithTimes(t, 949, userId, plan.Id, "order", 2222, secondOrder.CompleteTime+10)

	result, err := BackfillBillingEvents(BillingEventBackfillParams{
		Sources: []string{BillingEventBackfillSourceSubscriptionPurchase},
		Limit:   10,
		DryRun:  false,
	})
	require.NoError(t, err)
	require.Equal(t, 2, result.TotalScanned)
	require.Equal(t, 2, result.TotalCreated)

	first := requireFundingBillingEvent(t, model.BillingEventSourceSubscription, firstOrder.TradeNo, "purchase")
	require.Equal(t, 1111, first.AmountQuota)
	requireBillingEventMetadataValue(t, first, "subscription_id", float64(948))
	second := requireFundingBillingEvent(t, model.BillingEventSourceSubscription, secondOrder.TradeNo, "purchase")
	require.Equal(t, 2222, second.AmountQuota)
	requireBillingEventMetadataValue(t, second, "subscription_id", float64(949))
}

func TestBackfillBillingEventsSkipsLongExistingSourceIdsWithinBatch(t *testing.T) {
	truncate(t)
	withServiceTestQuotaPerUnit(t, 100)

	existingTradeNo := "backfill-long-existing-" + strings.Repeat("x", 170)
	missingTradeNo := "backfill-long-missing-" + strings.Repeat("y", 170)
	seedBackfillWalletTopUp(t, existingTradeNo, 947, model.PaymentProviderStripe, 2, 2.00)
	seedBackfillWalletTopUp(t, missingTradeNo, 948, model.PaymentProviderStripe, 3, 3.00)

	created, err := model.RecordFundingBillingEventIfNotExists(nil, model.FundingBillingEventInput{
		Source:        model.BillingEventSourceWalletTopUp,
		SourceId:      existingTradeNo,
		Phase:         "success",
		UserId:        947,
		RequestId:     existingTradeNo,
		BillingSource: BillingSourceWallet,
		PriceUnit:     "topup",
		EventType:     model.BillingEventTypeCredit,
		AmountQuota:   200,
	})
	require.NoError(t, err)
	require.True(t, created)

	result, err := BackfillBillingEvents(BillingEventBackfillParams{
		Sources: []string{BillingEventBackfillSourceWalletTopUp},
		Limit:   1,
		DryRun:  false,
	})
	require.NoError(t, err)
	require.Equal(t, 1, result.TotalScanned)
	require.Equal(t, 1, result.TotalCreated)

	existing := requireFundingBillingEvent(t, model.BillingEventSourceWalletTopUp, existingTradeNo, "success")
	require.Equal(t, 200, existing.AmountQuota)
	missing := requireFundingBillingEvent(t, model.BillingEventSourceWalletTopUp, missingTradeNo, "success")
	require.Equal(t, 300, missing.AmountQuota)
}

func TestBackfillBillingEventsSkipsExistingModelRequestLogsWithinBatch(t *testing.T) {
	truncate(t)
	withServiceTestQuotaPerUnit(t, 100)

	seedBackfillConsumeLog(t, model.Log{
		UserId:    950,
		TokenId:   1950,
		RequestId: "backfill-model-existing",
		ModelName: "gpt-existing",
		Quota:     111,
	})
	seedBackfillConsumeLog(t, model.Log{
		UserId:    951,
		TokenId:   1951,
		RequestId: "backfill-model-missing",
		ModelName: "gpt-missing",
		Quota:     222,
	})
	created, err := model.RecordFundingBillingEventIfNotExists(nil, model.FundingBillingEventInput{
		Source:        model.BillingEventSourceModelRequest,
		SourceId:      "backfill-model-existing",
		Phase:         "settlement",
		UserId:        950,
		TokenId:       1950,
		RequestId:     "backfill-model-existing",
		Group:         "default",
		BillingSource: BillingSourceWallet,
		PriceUnit:     "token_usage",
		EventType:     model.BillingEventTypeDebit,
		AmountQuota:   111,
	})
	require.NoError(t, err)
	require.True(t, created)

	result, err := BackfillBillingEvents(BillingEventBackfillParams{
		Sources: []string{BillingEventBackfillSourceModelRequest},
		Limit:   1,
		DryRun:  false,
	})
	require.NoError(t, err)
	require.Equal(t, 1, result.TotalScanned)
	require.Equal(t, 1, result.TotalCreated)

	existing := requireFundingBillingEvent(t, model.BillingEventSourceModelRequest, "backfill-model-existing", "settlement")
	require.Equal(t, 111, existing.AmountQuota)
	missing := requireFundingBillingEvent(t, model.BillingEventSourceModelRequest, "backfill-model-missing", "settlement")
	require.Equal(t, 222, missing.AmountQuota)
}

func TestGetBillingEventHealthReportsBackfillAndReconciliationWithoutWriting(t *testing.T) {
	truncate(t)
	withServiceTestQuotaPerUnit(t, 100)
	seedBackfillWalletTopUp(t, "billing-health-wallet", 949, model.PaymentProviderWaffo, 4, 4.00)

	health, err := GetBillingEventHealth(BillingEventReconciliationParams{
		Sources: []string{BillingEventBackfillSourceWalletTopUp},
		Limit:   10,
	})
	require.NoError(t, err)
	require.True(t, health.NeedsReview)
	require.Equal(t, 1, health.TotalWouldCreate)
	require.Equal(t, 1, health.TotalMissing)
	require.Equal(t, 0, health.TotalMismatched)
	require.Equal(t, 0, health.TotalInvalid)
	require.Equal(t, 0, health.TotalErrorCount)
	require.Equal(t, 1, health.Backfill.TotalWouldCreate)
	require.Equal(t, 1, health.Reconciliation.TotalMissing)

	var count int64
	require.NoError(t, model.DB.Model(&model.BillingEvent{}).Count(&count).Error)
	require.EqualValues(t, 0, count)
}

func TestBackfillBillingEventsCreatesModelRequestEventsFromConsumeLogs(t *testing.T) {
	truncate(t)
	withServiceTestQuotaPerUnit(t, 100)

	seedBackfillConsumeLog(t, model.Log{
		UserId:            908,
		TokenId:           1908,
		RequestId:         "backfill-model-request",
		UpstreamRequestId: "upstream-model-request",
		ModelName:         "gpt-backfill",
		TokenName:         "model-token",
		Group:             "vip",
		Quota:             345,
		PromptTokens:      100,
		CompletionTokens:  23,
		ChannelId:         7,
		UseTime:           2,
		IsStream:          true,
		Other: common.MapToJsonStr(map[string]interface{}{
			"billing_source":  "subscription",
			"subscription_id": 12,
		}),
	})

	result, err := BackfillBillingEvents(BillingEventBackfillParams{
		Sources: []string{BillingEventBackfillSourceModelRequest},
		Limit:   10,
		DryRun:  false,
	})
	require.NoError(t, err)
	require.Equal(t, 1, result.TotalScanned)
	require.Equal(t, 1, result.TotalCreated)

	event, exists, err := model.GetFundingBillingEvent(nil, model.BillingEventSourceModelRequest, "backfill-model-request", "settlement")
	require.NoError(t, err)
	require.True(t, exists)
	require.Equal(t, 908, event.UserId)
	require.Equal(t, 1908, event.TokenId)
	require.Equal(t, model.BillingEventTypeDebit, event.EventType)
	require.Equal(t, "subscription", event.BillingSource)
	require.Equal(t, "vip", event.Group)
	require.Equal(t, "token_usage", event.PriceUnit)
	require.Equal(t, 345, event.AmountQuota)
	require.Equal(t, -345, event.QuotaDelta)
	require.InDelta(t, 3.45, event.Cost, 0.000001)

	var metadata map[string]any
	require.NoError(t, common.UnmarshalJsonStr(event.Metadata, &metadata))
	require.Equal(t, true, metadata["backfill"])
	require.Equal(t, "text", metadata["usage_kind"])
	require.Equal(t, "gpt-backfill", metadata["model_name"])
	require.Equal(t, "upstream-model-request", metadata["upstream_request_id"])

	again, err := BackfillBillingEvents(BillingEventBackfillParams{
		Sources: []string{BillingEventBackfillSourceModelRequest},
		Limit:   10,
		DryRun:  false,
	})
	require.NoError(t, err)
	require.Equal(t, 0, again.TotalCreated)
	require.Equal(t, 0, again.TotalScanned)
}

func TestBackfillBillingEventsCreatesMidjourneySubscriptionModelRequest(t *testing.T) {
	truncate(t)
	withServiceTestQuotaPerUnit(t, 100)

	seedBackfillConsumeLog(t, model.Log{
		UserId:    909,
		TokenId:   1909,
		RequestId: "backfill-midjourney-subscription",
		ModelName: "mj_imagine",
		TokenName: "mj-token",
		Group:     "vip",
		Quota:     450,
		ChannelId: 8,
		UseTime:   1,
		Other: common.MapToJsonStr(map[string]interface{}{
			"usage_kind":                  "midjourney",
			"billing_source":              "subscription",
			"billing_preference":          "subscription_first",
			"subscription_id":             42,
			"subscription_plan_id":        7,
			"subscription_plan_title":     "Pro",
			"subscription_consumed":       450,
			"subscription_remain":         9550,
			"wallet_quota_deducted":       0,
			"midjourney_action":           "IMAGINE",
			"midjourney_upstream_task_id": "mj-upstream-subscription",
			"request_path":                "/mj/submit/imagine",
		}),
	})

	result, err := BackfillBillingEvents(BillingEventBackfillParams{
		Sources: []string{BillingEventBackfillSourceModelRequest},
		Limit:   10,
		DryRun:  false,
	})
	require.NoError(t, err)
	require.Equal(t, 1, result.TotalCreated)

	event, exists, err := model.GetFundingBillingEvent(nil, model.BillingEventSourceModelRequest, "backfill-midjourney-subscription", "settlement")
	require.NoError(t, err)
	require.True(t, exists)
	require.Equal(t, "subscription", event.BillingSource)
	require.Equal(t, "per_call", event.PriceUnit)
	require.Equal(t, 450, event.AmountQuota)
	require.Equal(t, -450, event.QuotaDelta)

	var metadata map[string]any
	require.NoError(t, common.UnmarshalJsonStr(event.Metadata, &metadata))
	require.Equal(t, "midjourney", metadata["usage_kind"])
	require.Equal(t, "mj_imagine", metadata["model_name"])
	require.Equal(t, "subscription_first", metadata["billing_preference"])
	require.Equal(t, float64(42), metadata["subscription_id"])
	require.Equal(t, float64(7), metadata["subscription_plan_id"])
	require.Equal(t, "Pro", metadata["subscription_plan_title"])
	require.Equal(t, float64(450), metadata["subscription_consumed"])
	require.Equal(t, float64(9550), metadata["subscription_remain"])
	require.Equal(t, float64(0), metadata["wallet_quota_deducted"])
	require.Equal(t, "IMAGINE", metadata["midjourney_action"])
	require.Equal(t, "mj-upstream-subscription", metadata["midjourney_upstream_task_id"])
	require.Equal(t, "/mj/submit/imagine", metadata["request_path"])

	items, total, err := ListBillingEvents(BillingEventListParams{
		Source:    model.BillingEventSourceModelRequest,
		UsageKind: model.BillingEventUsageKindMidjourney,
		Offset:    0,
		Limit:     10,
	})
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, items, 1)
	require.Equal(t, event.EventId, items[0].EventId)

	items, total, err = ListBillingEvents(BillingEventListParams{
		Source:    model.BillingEventSourceModelRequest,
		UsageKind: model.BillingEventUsageKindText,
		Offset:    0,
		Limit:     10,
	})
	require.NoError(t, err)
	require.EqualValues(t, 0, total)
	require.Empty(t, items)
}

func TestBackfillBillingEventsSkipsMissingSubscriptionPlan(t *testing.T) {
	truncate(t)
	withServiceTestQuotaPerUnit(t, 100)
	seedBackfillSubscriptionOrder(t, "backfill-missing-plan", 907, 9999, model.PaymentProviderStripe, model.PaymentMethodStripe, 1.23)

	result, err := BackfillBillingEvents(BillingEventBackfillParams{
		Sources: []string{BillingEventBackfillSourceSubscriptionPurchase},
		Limit:   10,
		DryRun:  false,
	})
	require.NoError(t, err)
	require.Equal(t, 1, result.TotalScanned)
	require.Equal(t, 0, result.TotalCreated)
	require.Equal(t, 1, result.TotalSkippedInvalid)
	require.NotEmpty(t, result.Results[0].Errors)
}

func withServiceTestQuotaPerUnit(t *testing.T, quotaPerUnit float64) {
	t.Helper()
	original := common.QuotaPerUnit
	common.QuotaPerUnit = quotaPerUnit
	t.Cleanup(func() {
		common.QuotaPerUnit = original
	})
}

func seedBackfillWalletTopUp(t *testing.T, tradeNo string, userId int, provider string, amount int64, money float64) {
	t.Helper()
	seedBackfillUser(t, userId)
	topUp := &model.TopUp{
		UserId:          userId,
		Amount:          amount,
		Money:           money,
		TradeNo:         tradeNo,
		PaymentMethod:   provider,
		PaymentProvider: provider,
		CreateTime:      time.Now().Unix() - 60,
		CompleteTime:    time.Now().Unix(),
		Status:          common.TopUpStatusSuccess,
	}
	require.NoError(t, model.DB.Create(topUp).Error)
}

func seedBackfillSubscriptionTopUpMirror(t *testing.T, tradeNo string, userId int) {
	t.Helper()
	seedBackfillUser(t, userId)
	topUp := &model.TopUp{
		UserId:          userId,
		Amount:          0,
		Money:           9.99,
		TradeNo:         tradeNo,
		PaymentMethod:   model.PaymentMethodStripe,
		PaymentProvider: model.PaymentProviderStripe,
		CreateTime:      time.Now().Unix() - 60,
		CompleteTime:    time.Now().Unix(),
		Status:          common.TopUpStatusSuccess,
	}
	require.NoError(t, model.DB.Create(topUp).Error)
}

func seedBackfillSubscriptionOrder(t *testing.T, tradeNo string, userId int, planId int, provider string, method string, money float64) {
	t.Helper()
	seedBackfillSubscriptionOrderWithTimes(t, tradeNo, userId, planId, provider, method, money, time.Now().Unix()-60, time.Now().Unix())
}

func seedBackfillSubscriptionOrderWithTimes(t *testing.T, tradeNo string, userId int, planId int, provider string, method string, money float64, createTime int64, completeTime int64) *model.SubscriptionOrder {
	t.Helper()
	seedBackfillUser(t, userId)
	order := &model.SubscriptionOrder{
		UserId:          userId,
		PlanId:          planId,
		Money:           money,
		TradeNo:         tradeNo,
		PaymentMethod:   method,
		PaymentProvider: provider,
		Status:          common.TopUpStatusSuccess,
		CreateTime:      createTime,
		CompleteTime:    completeTime,
		ProviderPayload: "charged_quota=999",
	}
	require.NoError(t, model.DB.Create(order).Error)
	return order
}

func seedBackfillPlan(t *testing.T, id int, totalAmount int64, priceAmount float64) *model.SubscriptionPlan {
	t.Helper()
	plan := &model.SubscriptionPlan{
		Id:            id,
		Title:         fmt.Sprintf("Backfill Plan %d", id),
		PriceAmount:   priceAmount,
		Currency:      "USD",
		DurationUnit:  model.SubscriptionDurationMonth,
		DurationValue: 1,
		Enabled:       true,
		TotalAmount:   totalAmount,
	}
	require.NoError(t, model.DB.Create(plan).Error)
	return plan
}

func seedBackfillUserSubscription(t *testing.T, id int, userId int, planId int, source string) {
	t.Helper()
	seedBackfillUserSubscriptionWithAmount(t, id, userId, planId, source, 1000)
}

func seedBackfillUserSubscriptionWithAmount(t *testing.T, id int, userId int, planId int, source string, amountTotal int64) {
	t.Helper()
	seedBackfillUserSubscriptionWithTimes(t, id, userId, planId, source, amountTotal, time.Now().Unix()-60)
}

func seedBackfillUserSubscriptionWithTimes(t *testing.T, id int, userId int, planId int, source string, amountTotal int64, createdAt int64) {
	t.Helper()
	seedBackfillUser(t, userId)
	sub := &model.UserSubscription{
		Id:          id,
		UserId:      userId,
		PlanId:      planId,
		AmountTotal: amountTotal,
		Status:      "active",
		Source:      source,
		StartTime:   createdAt,
		EndTime:     createdAt + int64(30*24*time.Hour/time.Second),
		CreatedAt:   createdAt,
	}
	require.NoError(t, model.DB.Create(sub).Error)
	require.NoError(t, model.DB.Model(sub).Updates(map[string]any{
		"start_time": createdAt,
		"end_time":   createdAt + int64(30*24*time.Hour/time.Second),
		"created_at": createdAt,
		"updated_at": createdAt,
	}).Error)
}

func seedBackfillRedemption(t *testing.T, id int, usedUserId int, quota int) {
	t.Helper()
	seedBackfillUser(t, usedUserId)
	redemption := &model.Redemption{
		Id:           id,
		Key:          fmt.Sprintf("%032d", id),
		Status:       common.RedemptionCodeStatusUsed,
		Name:         fmt.Sprintf("Backfill Redemption %d", id),
		Quota:        quota,
		CreatedTime:  time.Now().Unix() - 60,
		RedeemedTime: time.Now().Unix(),
		UsedUserId:   usedUserId,
	}
	require.NoError(t, model.DB.Create(redemption).Error)
}

func seedBackfillWalletAdjustment(t *testing.T, sourceId string, userId int, adminId int, mode string, eventType string, amount int, oldQuota int, newQuota int) {
	t.Helper()
	seedBackfillUser(t, userId)
	metadata := map[string]any{
		"admin_info": map[string]any{
			"admin_id":   adminId,
			"admin_name": "backfill_admin",
		},
		"mode":      mode,
		"old_quota": oldQuota,
		"new_quota": newQuota,
	}
	metadataBytes, err := common.Marshal(metadata)
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.WalletAdjustment{
		SourceId:  sourceId,
		UserId:    userId,
		AdminId:   adminId,
		Mode:      mode,
		EventType: eventType,
		Amount:    amount,
		OldQuota:  oldQuota,
		NewQuota:  newQuota,
		Metadata:  string(metadataBytes),
		CreatedAt: time.Now().Unix(),
	}).Error)
}

func seedBackfillTaskBillingRecord(t *testing.T, sourceId string, phase string, userId int, tokenId int, billingSource string, eventType string, amount int, quotaDelta int) {
	t.Helper()
	seedBackfillUser(t, userId)
	metadata := map[string]any{
		"phase":          phase,
		"task_id":        sourceId,
		"billing_source": billingSource,
	}
	metadataBytes, err := common.Marshal(metadata)
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.TaskBillingRecord{
		SourceId:      sourceId,
		TaskId:        sourceId,
		Phase:         phase,
		UserId:        userId,
		TokenId:       tokenId,
		Group:         "default",
		BillingSource: billingSource,
		PriceUnit:     expectedTaskPriceUnitForPhase(phase),
		EventType:     eventType,
		AmountQuota:   amount,
		QuotaDelta:    quotaDelta,
		RequestId:     sourceId,
		Metadata:      string(metadataBytes),
		CreatedAt:     time.Now().Unix(),
	}).Error)
}

func seedBackfillViolationFeeRecord(t *testing.T, sourceId string, phase string, userId int, tokenId int, billingSource string, amount int) {
	t.Helper()
	seedBackfillUser(t, userId)
	metadata := map[string]any{
		"violation_fee":      true,
		"violation_fee_code": phase,
		"fee_quota":          amount,
		"billing_source":     billingSource,
	}
	metadataBytes, err := common.Marshal(metadata)
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.ViolationFeeRecord{
		SourceId:      sourceId,
		Phase:         phase,
		UserId:        userId,
		TokenId:       tokenId,
		Group:         "default",
		BillingSource: billingSource,
		PriceUnit:     "violation_fee",
		EventType:     model.BillingEventTypeDebit,
		AmountQuota:   amount,
		QuotaDelta:    -amount,
		RequestId:     sourceId,
		Metadata:      string(metadataBytes),
		CreatedAt:     time.Now().Unix(),
	}).Error)
}

func seedBackfillMCPToolCall(t *testing.T, userId int, tokenId int, tool *model.MCPTool, requestId string, status string, quota int, cost float64, settledAt int64) model.MCPToolCall {
	t.Helper()
	call := model.MCPToolCall{
		UserId:        userId,
		TokenId:       tokenId,
		ToolId:        tool.Id,
		ToolName:      tool.Name,
		RequestId:     requestId,
		RequestParams: "{}",
		RequestIP:     "127.0.0.1",
		Status:        status,
		Cost:          cost,
		Quota:         quota,
		SettledAt:     settledAt,
	}
	require.NoError(t, model.DB.Create(&call).Error)
	return call
}

func expectedTaskPriceUnitForPhase(phase string) string {
	switch phase {
	case taskBillingEventPhaseFailureRefund:
		return "task_refund"
	case taskBillingEventPhaseDeltaDebit, taskBillingEventPhaseDeltaCredit:
		return "task_recalculation"
	default:
		return "task"
	}
}

func seedBackfillConsumeLog(t *testing.T, log model.Log) {
	t.Helper()
	if log.CreatedAt == 0 {
		log.CreatedAt = time.Now().Unix()
	}
	log.Type = model.LogTypeConsume
	require.NoError(t, model.LOG_DB.Create(&log).Error)
}

func seedBackfillUser(t *testing.T, id int) {
	t.Helper()
	var count int64
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", id).Count(&count).Error)
	if count > 0 {
		return
	}
	user := &model.User{
		Id:       id,
		Username: fmt.Sprintf("backfill_user_%d", id),
		AffCode:  fmt.Sprintf("bfaff%d", id),
		Status:   common.UserStatusEnabled,
		Quota:    0,
	}
	require.NoError(t, model.DB.Create(user).Error)
}

func assertBillingEventCount(t *testing.T, source string, sourceId string, expected int64) {
	t.Helper()
	var count int64
	require.NoError(t, model.DB.Model(&model.BillingEvent{}).
		Where("source = ? AND source_id = ?", source, sourceId).
		Count(&count).Error)
	require.EqualValues(t, expected, count)
}

func requireFundingBillingEvent(t *testing.T, source string, sourceId string, phase string) model.BillingEvent {
	t.Helper()
	event, exists, err := model.GetFundingBillingEvent(nil, source, sourceId, phase)
	require.NoError(t, err)
	require.True(t, exists)
	return event
}

func requireBillingEventMetadataValue(t *testing.T, event model.BillingEvent, key string, expected any) {
	t.Helper()
	var metadata map[string]any
	require.NoError(t, common.UnmarshalJsonStr(event.Metadata, &metadata))
	require.Equal(t, expected, metadata[key])
}
