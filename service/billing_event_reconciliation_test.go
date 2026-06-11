package service

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"

	"github.com/stretchr/testify/require"
)

func TestReconcileBillingEventsReportsMissingAndLedgeredEvents(t *testing.T) {
	truncate(t)
	withServiceTestQuotaPerUnit(t, 100)

	seedBackfillWalletTopUp(t, "reconcile-wallet", 1001, model.PaymentProviderWaffo, 4, 4.00)
	seedBackfillSubscriptionTopUpMirror(t, "reconcile-sub-purchase", 1002)
	plan := seedBackfillPlan(t, 1010, 1000, 9.99)
	seedBackfillSubscriptionOrder(t, "reconcile-sub-purchase", 1002, plan.Id, model.PaymentProviderStripe, model.PaymentMethodStripe, 9.99)
	seedBackfillUserSubscription(t, 1020, 1002, plan.Id, "order")
	seedBackfillSubscriptionOrder(t, "reconcile-sub-balance", 1003, plan.Id, model.PaymentProviderBalance, model.PaymentMethodBalance, 9.99)
	seedBackfillUserSubscription(t, 1021, 1003, plan.Id, model.PaymentMethodBalance)
	seedBackfillUserSubscription(t, 1022, 1004, plan.Id, "admin")
	seedBackfillRedemption(t, 1030, 1005, 321)

	before, err := ReconcileBillingEvents(BillingEventReconciliationParams{
		Sources: []string{"all"},
		Limit:   20,
	})
	require.NoError(t, err)
	require.Equal(t, 5, before.TotalScanned)
	require.Equal(t, 6, before.TotalExpected)
	require.Equal(t, 0, before.TotalLedgered)
	require.Equal(t, 6, before.TotalMissing)
	require.Equal(t, 0, before.TotalInvalid)

	_, err = BackfillBillingEvents(BillingEventBackfillParams{
		Sources: []string{"all"},
		Limit:   20,
		DryRun:  false,
	})
	require.NoError(t, err)

	after, err := ReconcileBillingEvents(BillingEventReconciliationParams{
		Sources: []string{"all"},
		Limit:   20,
	})
	require.NoError(t, err)
	require.Equal(t, 5, after.TotalScanned)
	require.Equal(t, 6, after.TotalExpected)
	require.Equal(t, 6, after.TotalLedgered)
	require.Equal(t, 0, after.TotalMissing)
	require.Equal(t, 0, after.TotalInvalid)
}

func TestReconcileBillingEventsReportsInvalidRows(t *testing.T) {
	truncate(t)
	withServiceTestQuotaPerUnit(t, 100)

	seedBackfillUser(t, 1101)
	require.NoError(t, model.DB.Create(&model.SubscriptionOrder{
		UserId:          1101,
		PlanId:          9999,
		Money:           9.99,
		TradeNo:         "reconcile-missing-plan",
		PaymentMethod:   model.PaymentMethodStripe,
		PaymentProvider: model.PaymentProviderStripe,
		Status:          common.TopUpStatusSuccess,
		CreateTime:      100,
		CompleteTime:    200,
	}).Error)

	result, err := ReconcileBillingEvents(BillingEventReconciliationParams{
		Sources: []string{BillingEventBackfillSourceSubscriptionPurchase},
		Limit:   20,
	})
	require.NoError(t, err)
	require.Equal(t, 1, result.TotalScanned)
	require.Equal(t, 0, result.TotalExpected)
	require.Equal(t, 0, result.TotalMissing)
	require.Equal(t, 1, result.TotalInvalid)
	require.NotEmpty(t, result.Results[0].SampleInvalid)
}

func TestReconcileBillingEventsFindsMissingModelRequestBeyondLimit(t *testing.T) {
	truncate(t)
	withServiceTestQuotaPerUnit(t, 100)

	seedBackfillConsumeLog(t, model.Log{
		UserId:    1151,
		TokenId:   2151,
		RequestId: "reconcile-model-existing",
		ModelName: "gpt-existing",
		Quota:     111,
	})
	seedBackfillConsumeLog(t, model.Log{
		UserId:    1152,
		TokenId:   2152,
		RequestId: "reconcile-model-missing",
		ModelName: "gpt-missing",
		Quota:     222,
	})
	created, err := model.RecordFundingBillingEventIfNotExists(nil, model.FundingBillingEventInput{
		Source:        model.BillingEventSourceModelRequest,
		SourceId:      "reconcile-model-existing",
		Phase:         "settlement",
		UserId:        1151,
		TokenId:       2151,
		RequestId:     "reconcile-model-existing",
		BillingSource: BillingSourceWallet,
		PriceUnit:     "token_usage",
		EventType:     model.BillingEventTypeDebit,
		AmountQuota:   111,
	})
	require.NoError(t, err)
	require.True(t, created)

	result, err := ReconcileBillingEvents(BillingEventReconciliationParams{
		Sources: []string{BillingEventBackfillSourceModelRequest},
		Limit:   1,
	})
	require.NoError(t, err)
	require.Equal(t, 2, result.TotalScanned)
	require.Equal(t, 2, result.TotalExpected)
	require.Equal(t, 1, result.TotalLedgered)
	require.Equal(t, 1, result.TotalMissing)
	require.False(t, result.HasMore)
	require.True(t, result.ScanComplete)
	require.Contains(t, result.Results[0].SampleMissing[0], "reconcile-model-missing")
}

func TestBillingEventReconciliationMissingDetailsAndBackfill(t *testing.T) {
	truncate(t)
	withServiceTestQuotaPerUnit(t, 100)

	tradeNo := "reconcile-missing-wallet"
	seedBackfillWalletTopUp(t, tradeNo, 1161, model.PaymentProviderWaffo, 4, 4.00)

	missing, err := ListBillingEventReconciliationMissing(BillingEventReconciliationMissingParams{
		Sources:     []string{BillingEventBackfillSourceWalletTopUp},
		Limit:       20,
		DetailLimit: 10,
	})
	require.NoError(t, err)
	require.Equal(t, 1, missing.TotalMissing)
	require.Len(t, missing.Items, 1)
	item := missing.Items[0]
	require.Equal(t, BillingEventBackfillSourceWalletTopUp, item.Source)
	require.Equal(t, model.BillingEventSourceWalletTopUp+":"+tradeNo+":success", item.Label)
	require.Equal(t, model.BillingEventSourceWalletTopUp, item.Expected.Source)
	require.Equal(t, tradeNo, item.Expected.SourceId)
	require.Equal(t, "success", item.Expected.Phase)
	require.Equal(t, 1161, item.Expected.UserId)
	require.Equal(t, 400, item.Expected.AmountQuota)

	backfilled, err := BackfillBillingEventReconciliationMissing(BillingEventReconciliationBackfillMissingParams{
		Source:  BillingEventBackfillSourceWalletTopUp,
		Label:   item.Label,
		Limit:   20,
		Reason:  "backfill missing wallet test",
		AdminId: 1,
	})
	require.NoError(t, err)
	require.True(t, backfilled.Backfilled)
	require.NotNil(t, backfilled.Event)
	require.NotNil(t, backfilled.AuditEvent)
	require.Equal(t, tradeNo, backfilled.Event.SourceId)
	require.Equal(t, 400, backfilled.Event.AmountQuota)
	requireBillingEventMetadataValue(t, requireFundingBillingEvent(t, model.BillingEventSourceWalletTopUp, tradeNo, "success"), "reconciliation_backfill", true)
	requireBillingEventMetadataValue(t, requireFundingBillingEvent(t, model.BillingEventSourceWalletTopUp, tradeNo, "success"), "trade_no", tradeNo)
	requireBillingEventMetadataValue(t, requireFundingBillingEvent(t, model.BillingEventSourceWalletTopUp, tradeNo, "success"), "channel", model.PaymentProviderWaffo)
	require.Equal(t, model.BillingEventSourceLedgerRepair, backfilled.AuditEvent.Source)
	require.Equal(t, "reconciliation_backfill_missing", backfilled.AuditEvent.PriceUnit)
	requireBillingEventMetadataValue(t, requireFundingBillingEvent(t, model.BillingEventSourceLedgerRepair, backfilled.AuditEvent.SourceId, "backfill_missing"), "created_event_pk", float64(backfilled.Event.Id))
	requireBillingEventRelation(t, backfilled.AuditEvent.Id, backfilled.Event.Id, model.BillingEventRelationTypeReconciliationBackfillMissing, "backfill missing wallet test", item.Label, 1)

	items, total, err := ListBillingEvents(BillingEventListParams{
		Source:  model.BillingEventSourceWalletTopUp,
		Keyword: tradeNo,
		Offset:  0,
		Limit:   10,
	})
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, items, 1)
	require.Len(t, items[0].RelatedAuditEvents, 1)
	require.Equal(t, backfilled.AuditEvent.Id, items[0].RelatedAuditEvents[0].Id)
	require.Equal(t, "reconciliation_backfill_missing", items[0].RelatedAuditEvents[0].PriceUnit)
	require.Equal(t, "backfill missing wallet test", items[0].RelatedAuditEvents[0].Reason)

	audits, total, err := ListBillingEvents(BillingEventListParams{
		Source: model.BillingEventSourceLedgerRepair,
		Offset: 0,
		Limit:  10,
	})
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, audits, 1)
	require.NotNil(t, audits[0].RelatedTargetEvent)
	require.Equal(t, backfilled.Event.Id, audits[0].RelatedTargetEvent.Id)
	require.Equal(t, tradeNo, audits[0].RelatedTargetEvent.SourceId)

	reconciled, err := ReconcileBillingEvents(BillingEventReconciliationParams{
		Sources: []string{BillingEventBackfillSourceWalletTopUp},
		Limit:   20,
	})
	require.NoError(t, err)
	require.Equal(t, 1, reconciled.TotalLedgered)
	require.Equal(t, 0, reconciled.TotalMissing)

	_, err = BackfillBillingEventReconciliationMissing(BillingEventReconciliationBackfillMissingParams{
		Source:  BillingEventBackfillSourceWalletTopUp,
		Label:   item.Label,
		Limit:   20,
		Reason:  "repeat missing wallet test",
		AdminId: 1,
	})
	require.Error(t, err)
}

func TestBillingEventReconciliationWalletAdjustMissingBackfill(t *testing.T) {
	truncate(t)
	withServiceTestQuotaPerUnit(t, 100)

	sourceId := "wallet-adjust-missing"
	seedBackfillWalletAdjustment(t, sourceId, 1162, 9, "add", model.BillingEventTypeCredit, 88, 1000, 1088)

	missing, err := ListBillingEventReconciliationMissing(BillingEventReconciliationMissingParams{
		Sources:     []string{BillingEventBackfillSourceWalletAdjust},
		Limit:       20,
		DetailLimit: 10,
	})
	require.NoError(t, err)
	require.Equal(t, 1, missing.TotalMissing)
	item := requireMissingBillingEventItem(t, missing.Items, model.BillingEventSourceWalletAdjust+":"+sourceId+":adjust")
	require.Equal(t, BillingEventBackfillSourceWalletAdjust, item.Source)
	require.Equal(t, sourceId, item.Expected.SourceId)
	require.Equal(t, model.BillingEventTypeCredit, item.Expected.EventType)
	require.Equal(t, 88, item.Expected.AmountQuota)
	require.Equal(t, 88, item.Expected.QuotaDelta)
	require.Equal(t, "manual_adjust", item.Expected.PriceUnit)

	backfilled, err := BackfillBillingEventReconciliationMissing(BillingEventReconciliationBackfillMissingParams{
		Source:  BillingEventBackfillSourceWalletAdjust,
		Label:   item.Label,
		Limit:   20,
		Reason:  "backfill missing wallet adjust test",
		AdminId: 1,
	})
	require.NoError(t, err)
	require.True(t, backfilled.Backfilled)
	require.NotNil(t, backfilled.Event)
	require.NotNil(t, backfilled.AuditEvent)
	require.Equal(t, sourceId, backfilled.Event.SourceId)
	require.Equal(t, model.BillingEventSourceWalletAdjust, backfilled.Event.Source)
	require.Equal(t, model.BillingEventTypeCredit, backfilled.Event.EventType)
	require.Equal(t, 88, backfilled.Event.AmountQuota)
	require.Equal(t, 88, backfilled.Event.QuotaDelta)
	event := requireFundingBillingEvent(t, model.BillingEventSourceWalletAdjust, sourceId, "adjust")
	requireBillingEventMetadataValue(t, event, "reconciliation_backfill", true)
	requireBillingEventMetadataValue(t, event, "wallet_adjustment_id", float64(1))
	requireBillingEventMetadataValue(t, event, "admin_id", float64(1))
	var metadata map[string]any
	require.NoError(t, common.UnmarshalJsonStr(event.Metadata, &metadata))
	adminInfo, ok := metadata["admin_info"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, float64(9), adminInfo["admin_id"])
	requireBillingEventRelation(t, backfilled.AuditEvent.Id, backfilled.Event.Id, model.BillingEventRelationTypeReconciliationBackfillMissing, "backfill missing wallet adjust test", item.Label, 1)

	reconciled, err := ReconcileBillingEvents(BillingEventReconciliationParams{
		Sources: []string{BillingEventBackfillSourceWalletAdjust},
		Limit:   20,
	})
	require.NoError(t, err)
	require.Equal(t, 1, reconciled.TotalLedgered)
	require.Equal(t, 0, reconciled.TotalMissing)
	require.Equal(t, 0, reconciled.TotalMismatched)
}

func TestBillingEventReconciliationAsyncTaskMissingBackfill(t *testing.T) {
	truncate(t)
	withServiceTestQuotaPerUnit(t, 100)

	sourceId := "task-reconcile-missing"
	seedBackfillTaskBillingRecord(t, sourceId, taskBillingEventPhaseDeltaDebit, 1163, 63, BillingSourceWallet, model.BillingEventTypeDebit, 88, -88)

	missing, err := ListBillingEventReconciliationMissing(BillingEventReconciliationMissingParams{
		Sources:     []string{BillingEventBackfillSourceAsyncTask},
		Limit:       20,
		DetailLimit: 10,
	})
	require.NoError(t, err)
	require.Equal(t, 1, missing.TotalMissing)
	item := requireMissingBillingEventItem(t, missing.Items, model.BillingEventSourceAsyncTask+":"+sourceId+":"+taskBillingEventPhaseDeltaDebit)
	require.Equal(t, BillingEventBackfillSourceAsyncTask, item.Source)
	require.Equal(t, sourceId, item.Expected.SourceId)
	require.Equal(t, model.BillingEventTypeDebit, item.Expected.EventType)
	require.Equal(t, 88, item.Expected.AmountQuota)
	require.Equal(t, -88, item.Expected.QuotaDelta)
	require.Equal(t, "task_recalculation", item.Expected.PriceUnit)

	backfilled, err := BackfillBillingEventReconciliationMissing(BillingEventReconciliationBackfillMissingParams{
		Source:  BillingEventBackfillSourceAsyncTask,
		Label:   item.Label,
		Limit:   20,
		Reason:  "backfill missing async task test",
		AdminId: 1,
	})
	require.NoError(t, err)
	require.True(t, backfilled.Backfilled)
	require.NotNil(t, backfilled.Event)
	require.NotNil(t, backfilled.AuditEvent)
	require.Equal(t, sourceId, backfilled.Event.SourceId)
	require.Equal(t, model.BillingEventSourceAsyncTask, backfilled.Event.Source)

	event := requireFundingBillingEvent(t, model.BillingEventSourceAsyncTask, sourceId, taskBillingEventPhaseDeltaDebit)
	requireBillingEventMetadataValue(t, event, "reconciliation_backfill", true)
	requireBillingEventMetadataValue(t, event, "task_id", sourceId)
	requireBillingEventMetadataValue(t, event, "phase", taskBillingEventPhaseDeltaDebit)
	requireBillingEventRelation(t, backfilled.AuditEvent.Id, backfilled.Event.Id, model.BillingEventRelationTypeReconciliationBackfillMissing, "backfill missing async task test", item.Label, 1)

	reconciled, err := ReconcileBillingEvents(BillingEventReconciliationParams{
		Sources: []string{BillingEventBackfillSourceAsyncTask},
		Limit:   20,
	})
	require.NoError(t, err)
	require.Equal(t, 1, reconciled.TotalLedgered)
	require.Equal(t, 0, reconciled.TotalMissing)
}

func TestBillingEventReconciliationViolationFeeMissingBackfill(t *testing.T) {
	truncate(t)
	withServiceTestQuotaPerUnit(t, 100)

	sourceId := "violation-reconcile-missing"
	phase := "violation_fee.grok_csam"
	seedBackfillViolationFeeRecord(t, sourceId, phase, 1164, 64, BillingSourceWallet, 77)

	missing, err := ListBillingEventReconciliationMissing(BillingEventReconciliationMissingParams{
		Sources:     []string{BillingEventBackfillSourceViolationFee},
		Limit:       20,
		DetailLimit: 10,
	})
	require.NoError(t, err)
	require.Equal(t, 1, missing.TotalMissing)
	item := requireMissingBillingEventItem(t, missing.Items, model.BillingEventSourceViolationFee+":"+sourceId+":"+phase)
	require.Equal(t, BillingEventBackfillSourceViolationFee, item.Source)
	require.Equal(t, sourceId, item.Expected.SourceId)
	require.Equal(t, model.BillingEventTypeDebit, item.Expected.EventType)
	require.Equal(t, 77, item.Expected.AmountQuota)
	require.Equal(t, -77, item.Expected.QuotaDelta)
	require.Equal(t, "violation_fee", item.Expected.PriceUnit)

	backfilled, err := BackfillBillingEventReconciliationMissing(BillingEventReconciliationBackfillMissingParams{
		Source:  BillingEventBackfillSourceViolationFee,
		Label:   item.Label,
		Limit:   20,
		Reason:  "backfill missing violation fee test",
		AdminId: 1,
	})
	require.NoError(t, err)
	require.True(t, backfilled.Backfilled)
	require.NotNil(t, backfilled.Event)
	require.NotNil(t, backfilled.AuditEvent)
	require.Equal(t, sourceId, backfilled.Event.SourceId)
	require.Equal(t, model.BillingEventSourceViolationFee, backfilled.Event.Source)

	event := requireFundingBillingEvent(t, model.BillingEventSourceViolationFee, sourceId, phase)
	requireBillingEventMetadataValue(t, event, "reconciliation_backfill", true)
	requireBillingEventMetadataValue(t, event, "violation_fee", true)
	requireBillingEventMetadataValue(t, event, "violation_fee_code", phase)
	requireBillingEventRelation(t, backfilled.AuditEvent.Id, backfilled.Event.Id, model.BillingEventRelationTypeReconciliationBackfillMissing, "backfill missing violation fee test", item.Label, 1)

	reconciled, err := ReconcileBillingEvents(BillingEventReconciliationParams{
		Sources: []string{BillingEventBackfillSourceViolationFee},
		Limit:   20,
	})
	require.NoError(t, err)
	require.Equal(t, 1, reconciled.TotalLedgered)
	require.Equal(t, 0, reconciled.TotalMissing)
}

func TestBillingEventReconciliationMCPToolCallMissingBackfill(t *testing.T) {
	truncate(t)
	withServiceTestQuotaPerUnit(t, 100)

	user, token := seedMCPBillingUserAndToken(t, 10000, 10000, false)
	tool := setMCPToolPriceForTest(t, "remote_read", 0.01)
	call := seedBackfillMCPToolCall(t, user.Id, token.Id, tool, "mcp-reconcile-missing", model.MCPToolCallStatusSuccess, 100, tool.PricePerCall, 1234)

	missing, err := ListBillingEventReconciliationMissing(BillingEventReconciliationMissingParams{
		Sources:     []string{BillingEventBackfillSourceMCPToolCall},
		Limit:       20,
		DetailLimit: 10,
	})
	require.NoError(t, err)
	require.Equal(t, 1, missing.TotalMissing)
	item := requireMissingBillingEventItem(t, missing.Items, model.BillingEventSourceMCPToolCall+":"+fmt.Sprintf("%d", call.Id)+":settlement")
	require.Equal(t, BillingEventBackfillSourceMCPToolCall, item.Source)
	require.Equal(t, fmt.Sprintf("%d", call.Id), item.Expected.SourceId)
	require.Equal(t, model.BillingEventTypeDebit, item.Expected.EventType)
	require.Equal(t, 100, item.Expected.AmountQuota)
	require.Equal(t, -100, item.Expected.QuotaDelta)

	backfilled, err := BackfillBillingEventReconciliationMissing(BillingEventReconciliationBackfillMissingParams{
		Source:  BillingEventBackfillSourceMCPToolCall,
		Label:   item.Label,
		Limit:   20,
		Reason:  "backfill missing mcp settlement test",
		AdminId: 1,
	})
	require.NoError(t, err)
	require.True(t, backfilled.Backfilled)
	require.NotNil(t, backfilled.Event)
	require.NotNil(t, backfilled.AuditEvent)
	require.Equal(t, fmt.Sprintf("%d", call.Id), backfilled.Event.SourceId)

	event := requireFundingBillingEvent(t, model.BillingEventSourceMCPToolCall, fmt.Sprintf("%d", call.Id), "settlement")
	requireBillingEventMetadataValue(t, event, "reconciliation_backfill", true)
	requireBillingEventMetadataValue(t, event, "mcp_tool_call_id", float64(call.Id))
	requireBillingEventRelation(t, backfilled.AuditEvent.Id, backfilled.Event.Id, model.BillingEventRelationTypeReconciliationBackfillMissing, "backfill missing mcp settlement test", item.Label, 1)

	reconciled, err := ReconcileBillingEvents(BillingEventReconciliationParams{
		Sources: []string{BillingEventBackfillSourceMCPToolCall},
		Limit:   20,
	})
	require.NoError(t, err)
	require.Equal(t, 1, reconciled.TotalLedgered)
	require.Equal(t, 0, reconciled.TotalMissing)
}

func TestBillingEventReconciliationMCPToolCallRefundMissingBackfill(t *testing.T) {
	truncate(t)
	withServiceTestQuotaPerUnit(t, 100)

	user, token := seedMCPBillingUserAndToken(t, 10000, 10000, false)
	tool := setMCPToolPriceForTest(t, "remote_read", 0.01)
	call := seedBackfillMCPToolCall(t, user.Id, token.Id, tool, "mcp-reconcile-refund-missing", model.MCPToolCallStatusError, 0, 0, 1234)
	created, err := model.RecordFundingBillingEventIfNotExists(nil, model.FundingBillingEventInput{
		Source:        model.BillingEventSourceMCPToolCall,
		SourceId:      fmt.Sprintf("%d", call.Id),
		Phase:         "settlement",
		UserId:        user.Id,
		TokenId:       token.Id,
		RequestId:     call.RequestId,
		BillingSource: BillingSourceWallet,
		PriceUnit:     model.MCPToolPriceUnitPerCall,
		EventType:     model.BillingEventTypeDebit,
		AmountQuota:   100,
	})
	require.NoError(t, err)
	require.True(t, created)

	missing, err := ListBillingEventReconciliationMissing(BillingEventReconciliationMissingParams{
		Sources:     []string{BillingEventBackfillSourceMCPToolCall},
		Limit:       20,
		DetailLimit: 10,
	})
	require.NoError(t, err)
	require.Equal(t, 1, missing.TotalMissing)
	item := requireMissingBillingEventItem(t, missing.Items, model.BillingEventSourceMCPToolCall+":"+fmt.Sprintf("%d", call.Id)+":refund")
	require.Equal(t, model.BillingEventTypeCredit, item.Expected.EventType)
	require.Equal(t, 100, item.Expected.AmountQuota)
	require.Equal(t, 100, item.Expected.QuotaDelta)

	backfilled, err := BackfillBillingEventReconciliationMissing(BillingEventReconciliationBackfillMissingParams{
		Source:  BillingEventBackfillSourceMCPToolCall,
		Label:   item.Label,
		Limit:   20,
		Reason:  "backfill missing mcp refund test",
		AdminId: 1,
	})
	require.NoError(t, err)
	require.True(t, backfilled.Backfilled)
	require.NotNil(t, backfilled.Event)

	event := requireFundingBillingEvent(t, model.BillingEventSourceMCPToolCall, fmt.Sprintf("%d", call.Id), "refund")
	require.Equal(t, model.BillingEventTypeCredit, event.EventType)
	require.Equal(t, 100, event.QuotaDelta)
	requireBillingEventMetadataValue(t, event, "reconciliation_backfill", true)
	requireBillingEventMetadataValue(t, event, "mcp_tool_call_id", float64(call.Id))
}

func TestBillingEventReconciliationReportsMCPRefundMissingWhenRefundFailed(t *testing.T) {
	truncate(t)
	withServiceTestQuotaPerUnit(t, 100)

	user, token := seedMCPBillingUserAndToken(t, 10000, 10000, false)
	tool := setMCPToolPriceForTest(t, "remote_read", 0.01)
	call := seedBackfillMCPToolCall(t, user.Id, token.Id, tool, "mcp-reconcile-refund-failed", model.MCPToolCallStatusError, 100, 0.01, 1234)
	created, err := model.RecordFundingBillingEventIfNotExists(nil, model.FundingBillingEventInput{
		Source:        model.BillingEventSourceMCPToolCall,
		SourceId:      fmt.Sprintf("%d", call.Id),
		Phase:         "settlement",
		UserId:        user.Id,
		TokenId:       token.Id,
		RequestId:     call.RequestId,
		BillingSource: BillingSourceWallet,
		PriceUnit:     model.MCPToolPriceUnitPerCall,
		EventType:     model.BillingEventTypeDebit,
		AmountQuota:   100,
	})
	require.NoError(t, err)
	require.True(t, created)

	missing, err := ListBillingEventReconciliationMissing(BillingEventReconciliationMissingParams{
		Sources:     []string{BillingEventBackfillSourceMCPToolCall},
		Limit:       20,
		DetailLimit: 10,
	})
	require.NoError(t, err)
	require.Equal(t, 1, missing.TotalMissing)
	item := requireMissingBillingEventItem(t, missing.Items, model.BillingEventSourceMCPToolCall+":"+fmt.Sprintf("%d", call.Id)+":refund")
	require.Equal(t, model.BillingEventTypeCredit, item.Expected.EventType)
	require.Equal(t, 100, item.Expected.AmountQuota)
	require.Equal(t, 100, item.Expected.QuotaDelta)
}

func TestBillingEventReconciliationBackfillMissingModelRequestMetadata(t *testing.T) {
	truncate(t)
	withServiceTestQuotaPerUnit(t, 100)

	requestId := "reconcile-missing-model-metadata"
	seedBackfillConsumeLog(t, model.Log{
		UserId:            1171,
		TokenId:           2171,
		RequestId:         requestId,
		UpstreamRequestId: "upstream-reconcile-missing",
		ModelName:         "gpt-reconcile-missing",
		TokenName:         "model-token",
		Group:             "pro",
		Quota:             678,
		PromptTokens:      321,
		CompletionTokens:  45,
		ChannelId:         9,
		UseTime:           3,
		IsStream:          true,
		Other: common.MapToJsonStr(map[string]interface{}{
			"billing_source":  "subscription",
			"subscription_id": 22,
			"ws":              true,
		}),
	})

	missing, err := ListBillingEventReconciliationMissing(BillingEventReconciliationMissingParams{
		Sources:     []string{BillingEventBackfillSourceModelRequest},
		Limit:       20,
		DetailLimit: 10,
	})
	require.NoError(t, err)
	require.Equal(t, 1, missing.TotalMissing)
	require.Len(t, missing.Items, 1)

	backfilled, err := BackfillBillingEventReconciliationMissing(BillingEventReconciliationBackfillMissingParams{
		Source:  BillingEventBackfillSourceModelRequest,
		Label:   missing.Items[0].Label,
		Limit:   20,
		Reason:  "backfill missing model metadata test",
		AdminId: 2,
	})
	require.NoError(t, err)
	require.NotNil(t, backfilled.Event)
	require.Equal(t, model.BillingEventSourceModelRequest, backfilled.Event.Source)
	require.Equal(t, requestId, backfilled.Event.SourceId)
	require.Equal(t, model.BillingEventTypeDebit, backfilled.Event.EventType)
	require.Equal(t, "subscription", backfilled.Event.BillingSource)
	require.Equal(t, "pro", backfilled.Event.Group)
	require.Equal(t, 678, backfilled.Event.AmountQuota)
	require.Equal(t, -678, backfilled.Event.QuotaDelta)

	event := requireFundingBillingEvent(t, model.BillingEventSourceModelRequest, requestId, "settlement")
	var metadata map[string]any
	require.NoError(t, common.UnmarshalJsonStr(event.Metadata, &metadata))
	require.Equal(t, true, metadata["reconciliation_backfill"])
	require.Equal(t, "gpt-reconcile-missing", metadata["model_name"])
	require.Equal(t, "model-token", metadata["token_name"])
	require.Equal(t, "upstream-reconcile-missing", metadata["upstream_request_id"])
	require.Equal(t, float64(321), metadata["prompt_tokens"])
	require.Equal(t, float64(45), metadata["completion_tokens"])
	require.Equal(t, "subscription", metadata["billing_source"])
	require.Equal(t, float64(22), metadata["subscription_id"])
	require.Equal(t, "realtime", metadata["usage_kind"])
}

func TestBillingEventReconciliationBackfillMissingSubscriptionPurchaseMetadata(t *testing.T) {
	truncate(t)
	withServiceTestQuotaPerUnit(t, 100)

	tradeNo := "reconcile-missing-subscription-metadata"
	seedBackfillSubscriptionTopUpMirror(t, tradeNo, 1181)
	plan := seedBackfillPlan(t, 1182, 5000, 19.99)
	seedBackfillSubscriptionOrder(t, tradeNo, 1181, plan.Id, model.PaymentProviderStripe, model.PaymentMethodStripe, 19.99)
	seedBackfillUserSubscriptionWithAmount(t, 1183, 1181, plan.Id, "order", 4321)

	missing, err := ListBillingEventReconciliationMissing(BillingEventReconciliationMissingParams{
		Sources:     []string{BillingEventBackfillSourceSubscriptionPurchase},
		Limit:       20,
		DetailLimit: 10,
	})
	require.NoError(t, err)
	require.Equal(t, 1, missing.TotalMissing)
	require.Len(t, missing.Items, 1)

	backfilled, err := BackfillBillingEventReconciliationMissing(BillingEventReconciliationBackfillMissingParams{
		Source:  BillingEventBackfillSourceSubscriptionPurchase,
		Label:   missing.Items[0].Label,
		Limit:   20,
		Reason:  "backfill missing subscription metadata test",
		AdminId: 3,
	})
	require.NoError(t, err)
	require.NotNil(t, backfilled.Event)
	require.Equal(t, model.BillingEventSourceSubscription, backfilled.Event.Source)
	require.Equal(t, tradeNo, backfilled.Event.SourceId)
	require.Equal(t, 4321, backfilled.Event.AmountQuota)
	require.Equal(t, 4321, backfilled.Event.QuotaDelta)

	event := requireFundingBillingEvent(t, model.BillingEventSourceSubscription, tradeNo, "purchase")
	var metadata map[string]any
	require.NoError(t, common.UnmarshalJsonStr(event.Metadata, &metadata))
	require.Equal(t, true, metadata["reconciliation_backfill"])
	require.Equal(t, tradeNo, metadata["trade_no"])
	require.Equal(t, float64(1183), metadata["subscription_id"])
	require.Equal(t, float64(plan.Id), metadata["plan_id"])
	require.Equal(t, plan.Title, metadata["plan_title"])
	require.Equal(t, model.PaymentMethodStripe, metadata["payment_method"])
	require.Equal(t, "user_subscription", metadata["quota_from"])
}

func TestBillingEventReconciliationBackfillMissingRedemptionMetadata(t *testing.T) {
	truncate(t)
	withServiceTestQuotaPerUnit(t, 100)

	redemptionId := 1191
	seedBackfillRedemption(t, redemptionId, 1192, 765)
	sourceId := fmt.Sprintf("redemption:%d", redemptionId)

	missing, err := ListBillingEventReconciliationMissing(BillingEventReconciliationMissingParams{
		Sources:     []string{BillingEventBackfillSourceRedemption},
		Limit:       20,
		DetailLimit: 10,
	})
	require.NoError(t, err)
	require.Equal(t, 1, missing.TotalMissing)
	item := requireMissingBillingEventItem(t, missing.Items, model.BillingEventSourceWalletTopUp+":"+sourceId+":redemption")

	backfilled, err := BackfillBillingEventReconciliationMissing(BillingEventReconciliationBackfillMissingParams{
		Source:  BillingEventBackfillSourceRedemption,
		Label:   item.Label,
		Limit:   20,
		Reason:  "backfill missing redemption metadata test",
		AdminId: 4,
	})
	require.NoError(t, err)
	require.NotNil(t, backfilled.Event)
	require.Equal(t, model.BillingEventSourceWalletTopUp, backfilled.Event.Source)
	require.Equal(t, sourceId, backfilled.Event.SourceId)
	require.Equal(t, model.BillingEventTypeCredit, backfilled.Event.EventType)
	require.Equal(t, 765, backfilled.Event.AmountQuota)
	require.Equal(t, 765, backfilled.Event.QuotaDelta)

	event := requireFundingBillingEvent(t, model.BillingEventSourceWalletTopUp, sourceId, "redemption")
	var metadata map[string]any
	require.NoError(t, common.UnmarshalJsonStr(event.Metadata, &metadata))
	require.Equal(t, true, metadata["reconciliation_backfill"])
	require.Equal(t, "redemption", metadata["channel"])
	require.Equal(t, float64(redemptionId), metadata["redemption_id"])
	require.Equal(t, float64(765), metadata["quota"])
	require.Equal(t, float64(1192), metadata["used_user_id"])
	require.Equal(t, fmt.Sprintf("Backfill Redemption %d", redemptionId), metadata["name"])
}

func TestBillingEventReconciliationBackfillMissingSubscriptionBalanceMetadata(t *testing.T) {
	truncate(t)
	withServiceTestQuotaPerUnit(t, 100)

	tradeNo := "reconcile-missing-balance-metadata"
	userId := 1193
	plan := seedBackfillPlan(t, 1194, 5000, 19.99)
	seedBackfillSubscriptionOrder(t, tradeNo, userId, plan.Id, model.PaymentProviderBalance, model.PaymentMethodBalance, 19.99)
	seedBackfillUserSubscriptionWithAmount(t, 1195, userId, plan.Id, model.PaymentMethodBalance, 3456)

	missing, err := ListBillingEventReconciliationMissing(BillingEventReconciliationMissingParams{
		Sources:     []string{BillingEventBackfillSourceSubscriptionBalance},
		Limit:       20,
		DetailLimit: 10,
	})
	require.NoError(t, err)
	require.Equal(t, 2, missing.TotalMissing)
	require.Len(t, missing.Items, 2)
	balancePayment := requireMissingBillingEventItem(t, missing.Items, model.BillingEventSourceSubscription+":"+tradeNo+":balance_payment")
	grant := requireMissingBillingEventItem(t, missing.Items, model.BillingEventSourceSubscription+":"+tradeNo+":grant")

	backfilledPayment, err := BackfillBillingEventReconciliationMissing(BillingEventReconciliationBackfillMissingParams{
		Source:  BillingEventBackfillSourceSubscriptionBalance,
		Label:   balancePayment.Label,
		Limit:   20,
		Reason:  "backfill missing balance payment metadata test",
		AdminId: 5,
	})
	require.NoError(t, err)
	require.NotNil(t, backfilledPayment.Event)
	require.Equal(t, model.BillingEventTypeDebit, backfilledPayment.Event.EventType)
	require.Equal(t, "wallet", backfilledPayment.Event.BillingSource)
	require.Equal(t, "subscription_balance_payment", backfilledPayment.Event.PriceUnit)
	require.Equal(t, 999, backfilledPayment.Event.AmountQuota)
	require.Equal(t, -999, backfilledPayment.Event.QuotaDelta)

	paymentEvent := requireFundingBillingEvent(t, model.BillingEventSourceSubscription, tradeNo, "balance_payment")
	var paymentMetadata map[string]any
	require.NoError(t, common.UnmarshalJsonStr(paymentEvent.Metadata, &paymentMetadata))
	require.Equal(t, true, paymentMetadata["reconciliation_backfill"])
	require.Equal(t, tradeNo, paymentMetadata["trade_no"])
	require.Equal(t, float64(1195), paymentMetadata["subscription_id"])
	require.Equal(t, float64(plan.Id), paymentMetadata["plan_id"])
	require.Equal(t, model.PaymentMethodBalance, paymentMetadata["payment_method"])
	require.Equal(t, model.PaymentProviderBalance, paymentMetadata["payment_provider"])

	backfilledGrant, err := BackfillBillingEventReconciliationMissing(BillingEventReconciliationBackfillMissingParams{
		Source:  BillingEventBackfillSourceSubscriptionBalance,
		Label:   grant.Label,
		Limit:   20,
		Reason:  "backfill missing balance grant metadata test",
		AdminId: 5,
	})
	require.NoError(t, err)
	require.NotNil(t, backfilledGrant.Event)
	require.Equal(t, model.BillingEventTypeCredit, backfilledGrant.Event.EventType)
	require.Equal(t, "subscription", backfilledGrant.Event.BillingSource)
	require.Equal(t, "subscription", backfilledGrant.Event.PriceUnit)
	require.Equal(t, 3456, backfilledGrant.Event.AmountQuota)
	require.Equal(t, 3456, backfilledGrant.Event.QuotaDelta)

	grantEvent := requireFundingBillingEvent(t, model.BillingEventSourceSubscription, tradeNo, "grant")
	var grantMetadata map[string]any
	require.NoError(t, common.UnmarshalJsonStr(grantEvent.Metadata, &grantMetadata))
	require.Equal(t, true, grantMetadata["reconciliation_backfill"])
	require.Equal(t, tradeNo, grantMetadata["trade_no"])
	require.Equal(t, float64(1195), grantMetadata["subscription_id"])
	require.Equal(t, float64(plan.Id), grantMetadata["plan_id"])
	require.Equal(t, plan.Title, grantMetadata["plan_title"])
	require.Equal(t, model.PaymentMethodBalance, grantMetadata["payment_method"])
	require.Equal(t, model.PaymentProviderBalance, grantMetadata["payment_provider"])
	require.Equal(t, model.PaymentMethodBalance, grantMetadata["subscription_from"])
	require.Equal(t, "user_subscription", grantMetadata["quota_from"])

	reconciled, err := ReconcileBillingEvents(BillingEventReconciliationParams{
		Sources: []string{BillingEventBackfillSourceSubscriptionBalance},
		Limit:   20,
	})
	require.NoError(t, err)
	require.Equal(t, 2, reconciled.TotalLedgered)
	require.Equal(t, 0, reconciled.TotalMissing)
}

func TestBillingEventReconciliationBackfillMissingAdminSubscriptionMetadata(t *testing.T) {
	truncate(t)
	withServiceTestQuotaPerUnit(t, 100)

	plan := seedBackfillPlan(t, 1196, 5000, 29.99)
	seedBackfillUserSubscriptionWithAmount(t, 1197, 1198, plan.Id, "admin", 2468)
	sourceId := "admin_bind:1197"

	missing, err := ListBillingEventReconciliationMissing(BillingEventReconciliationMissingParams{
		Sources:     []string{BillingEventBackfillSourceSubscriptionAdmin},
		Limit:       20,
		DetailLimit: 10,
	})
	require.NoError(t, err)
	require.Equal(t, 1, missing.TotalMissing)
	item := requireMissingBillingEventItem(t, missing.Items, model.BillingEventSourceSubscription+":"+sourceId+":admin_bind")

	backfilled, err := BackfillBillingEventReconciliationMissing(BillingEventReconciliationBackfillMissingParams{
		Source:  BillingEventBackfillSourceSubscriptionAdmin,
		Label:   item.Label,
		Limit:   20,
		Reason:  "backfill missing admin subscription metadata test",
		AdminId: 6,
	})
	require.NoError(t, err)
	require.NotNil(t, backfilled.Event)
	require.Equal(t, model.BillingEventSourceSubscription, backfilled.Event.Source)
	require.Equal(t, sourceId, backfilled.Event.SourceId)
	require.Equal(t, model.BillingEventTypeCredit, backfilled.Event.EventType)
	require.Equal(t, 2468, backfilled.Event.AmountQuota)
	require.Equal(t, 2468, backfilled.Event.QuotaDelta)

	event := requireFundingBillingEvent(t, model.BillingEventSourceSubscription, sourceId, "admin_bind")
	var metadata map[string]any
	require.NoError(t, common.UnmarshalJsonStr(event.Metadata, &metadata))
	require.Equal(t, true, metadata["reconciliation_backfill"])
	require.Equal(t, float64(1197), metadata["subscription_id"])
	require.Equal(t, float64(plan.Id), metadata["plan_id"])
	require.Equal(t, plan.Title, metadata["plan_title"])
	require.Equal(t, "backfill", metadata["source_note"])
	require.Equal(t, "user_subscription", metadata["quota_from"])
}

func TestReconcileBillingEventsReportsMismatchedLedgerEvents(t *testing.T) {
	truncate(t)
	withServiceTestQuotaPerUnit(t, 100)

	tradeNo := "reconcile-mismatch-wallet"
	seedBackfillWalletTopUp(t, tradeNo, 1201, model.PaymentProviderWaffo, 4, 4.00)
	created, err := model.RecordFundingBillingEventIfNotExists(nil, model.FundingBillingEventInput{
		Source:        model.BillingEventSourceWalletTopUp,
		SourceId:      tradeNo,
		Phase:         "success",
		UserId:        1201,
		RequestId:     tradeNo,
		BillingSource: BillingSourceWallet,
		PriceUnit:     "topup",
		EventType:     model.BillingEventTypeCredit,
		AmountQuota:   1,
	})
	require.NoError(t, err)
	require.True(t, created)
	require.NoError(t, model.DB.Model(&model.BillingEvent{}).
		Where("source = ? AND source_id = ?", model.BillingEventSourceWalletTopUp, tradeNo).
		Update("status", "pending").Error)
	require.NoError(t, model.DB.Model(&model.BillingEvent{}).
		Where("source = ? AND source_id = ?", model.BillingEventSourceWalletTopUp, tradeNo).
		Update("price_unit", "quota").Error)

	result, err := ReconcileBillingEvents(BillingEventReconciliationParams{
		Sources: []string{BillingEventBackfillSourceWalletTopUp},
		Limit:   20,
	})
	require.NoError(t, err)
	require.Equal(t, 1, result.TotalScanned)
	require.Equal(t, 1, result.TotalExpected)
	require.Equal(t, 0, result.TotalLedgered)
	require.Equal(t, 0, result.TotalMissing)
	require.Equal(t, 1, result.TotalMismatched)
	require.NotEmpty(t, result.Results[0].SampleMismatched)
	require.Contains(t, result.Results[0].SampleMismatched[0], "status expected settled got pending")
	require.Contains(t, result.Results[0].SampleMismatched[0], "amount_quota expected 400 got 1")
	require.Contains(t, result.Results[0].SampleMismatched[0], "price_unit expected topup got quota")

	details, err := ListBillingEventReconciliationMismatches(BillingEventReconciliationMismatchParams{
		Sources:     []string{BillingEventBackfillSourceWalletTopUp},
		Limit:       20,
		DetailLimit: 10,
	})
	require.NoError(t, err)
	require.False(t, details.HasMore)
	require.Equal(t, 1, details.TotalMismatched)
	require.Len(t, details.Items, 1)
	item := details.Items[0]
	require.Equal(t, BillingEventBackfillSourceWalletTopUp, item.Source)
	require.Equal(t, "wallet_topup:reconcile-mismatch-wallet:success", item.Label)
	require.Equal(t, model.BillingEventSourceWalletTopUp, item.Expected.Source)
	require.Equal(t, tradeNo, item.Expected.SourceId)
	require.Equal(t, 1201, item.Expected.UserId)
	require.Equal(t, 400, item.Expected.AmountQuota)
	require.NotNil(t, item.Actual)
	require.Equal(t, "pending", item.Actual.Status)
	require.Equal(t, 1, item.Actual.AmountQuota)
	require.NotEmpty(t, item.Diffs)
	require.Contains(t, item.Diffs, dtoBillingEventDiff("status", "settled", "pending"))
	require.Contains(t, item.Diffs, dtoBillingEventDiff("amount_quota", "400", "1"))
	require.Contains(t, item.Diffs, dtoBillingEventDiff("price_unit", "topup", "quota"))

	legacyItem, found, err := findBillingEventReconciliationMismatch(BillingEventBackfillSourceWalletTopUp, "wallet_topup:"+tradeNo, 20)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, item.Label, legacyItem.Label)

	repair, err := RepairBillingEventReconciliationMismatch(BillingEventReconciliationRepairParams{
		Source:  BillingEventBackfillSourceWalletTopUp,
		Label:   item.Label,
		Limit:   20,
		Reason:  "fix test mismatch",
		AdminId: 1,
	})
	require.NoError(t, err)
	require.True(t, repair.Repaired)
	require.NotNil(t, repair.Before)
	require.NotNil(t, repair.After)
	require.NotNil(t, repair.AuditEvent)
	require.Equal(t, model.BillingEventSourceLedgerRepair, repair.AuditEvent.Source)
	require.Equal(t, model.BillingEventTypeAudit, repair.AuditEvent.EventType)
	require.Equal(t, 400, repair.After.AmountQuota)
	require.Equal(t, model.BillingEventStatusSettled, repair.After.Status)
	require.Equal(t, "topup", repair.After.PriceUnit)
	requireBillingEventRelation(t, repair.AuditEvent.Id, repair.After.Id, model.BillingEventRelationTypeReconciliationRepair, "fix test mismatch", item.Label, 1)

	items, total, err := ListBillingEvents(BillingEventListParams{
		Source:  model.BillingEventSourceWalletTopUp,
		Keyword: tradeNo,
		Offset:  0,
		Limit:   10,
	})
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, items, 1)
	require.Len(t, items[0].RelatedAuditEvents, 1)
	require.Equal(t, repair.AuditEvent.Id, items[0].RelatedAuditEvents[0].Id)
	require.Equal(t, "reconciliation_repair", items[0].RelatedAuditEvents[0].PriceUnit)
	require.Equal(t, "fix test mismatch", items[0].RelatedAuditEvents[0].Reason)

	audits, total, err := ListBillingEvents(BillingEventListParams{
		Source: model.BillingEventSourceLedgerRepair,
		Offset: 0,
		Limit:  10,
	})
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, audits, 1)
	require.NotNil(t, audits[0].RelatedTargetEvent)
	require.Equal(t, repair.After.Id, audits[0].RelatedTargetEvent.Id)
	require.Equal(t, tradeNo, audits[0].RelatedTargetEvent.SourceId)

	repaired, err := ListBillingEventReconciliationMismatches(BillingEventReconciliationMismatchParams{
		Sources:     []string{BillingEventBackfillSourceWalletTopUp},
		Limit:       20,
		DetailLimit: 10,
	})
	require.NoError(t, err)
	require.Equal(t, 0, repaired.TotalMismatched)
	require.Empty(t, repaired.Items)

	var auditCount int64
	require.NoError(t, model.DB.Model(&model.BillingEvent{}).
		Where("source = ?", model.BillingEventSourceLedgerRepair).
		Count(&auditCount).Error)
	require.EqualValues(t, 1, auditCount)
}

func TestFindBillingEventReconciliationMismatchSupportsColonTradeNoLegacyLabel(t *testing.T) {
	truncate(t)
	withServiceTestQuotaPerUnit(t, 100)

	tradeNo := "reconcile:mismatch:wallet"
	seedBackfillWalletTopUp(t, tradeNo, 1202, model.PaymentProviderWaffo, 5, 5.00)
	created, err := model.RecordFundingBillingEventIfNotExists(nil, model.FundingBillingEventInput{
		Source:        model.BillingEventSourceWalletTopUp,
		SourceId:      tradeNo,
		Phase:         "success",
		UserId:        1202,
		RequestId:     tradeNo,
		BillingSource: BillingSourceWallet,
		PriceUnit:     "topup",
		EventType:     model.BillingEventTypeCredit,
		AmountQuota:   1,
	})
	require.NoError(t, err)
	require.True(t, created)

	item, found, err := findBillingEventReconciliationMismatch(BillingEventBackfillSourceWalletTopUp, "wallet_topup:"+tradeNo, 20)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "wallet_topup:"+tradeNo+":success", item.Label)
	require.Equal(t, 500, item.Expected.AmountQuota)
	require.NotNil(t, item.Actual)
	require.Equal(t, 1, item.Actual.AmountQuota)
}

func TestReconcileBillingEventsRepairsModelRequestMismatch(t *testing.T) {
	truncate(t)
	withServiceTestQuotaPerUnit(t, 100)

	requestId := "reconcile-model-request"
	seedBackfillConsumeLog(t, model.Log{
		UserId:           1301,
		TokenId:          2301,
		RequestId:        requestId,
		ModelName:        "gpt-reconcile",
		TokenName:        "reconcile-token",
		Group:            "vip",
		Quota:            777,
		PromptTokens:     200,
		CompletionTokens: 50,
		Other: common.MapToJsonStr(map[string]interface{}{
			"billing_source":  "subscription",
			"subscription_id": 88,
		}),
	})
	created, err := model.RecordFundingBillingEventIfNotExists(nil, model.FundingBillingEventInput{
		Source:        model.BillingEventSourceModelRequest,
		SourceId:      requestId,
		Phase:         "settlement",
		UserId:        1301,
		TokenId:       1,
		RequestId:     requestId,
		Group:         "default",
		BillingSource: BillingSourceWallet,
		PriceUnit:     "token_usage",
		EventType:     model.BillingEventTypeDebit,
		AmountQuota:   1,
	})
	require.NoError(t, err)
	require.True(t, created)
	require.NoError(t, model.DB.Model(&model.BillingEvent{}).
		Where("source = ? AND source_id = ?", model.BillingEventSourceModelRequest, requestId).
		Update("status", "pending").Error)

	result, err := ReconcileBillingEvents(BillingEventReconciliationParams{
		Sources: []string{BillingEventBackfillSourceModelRequest},
		Limit:   20,
	})
	require.NoError(t, err)
	require.Equal(t, 1, result.TotalScanned)
	require.Equal(t, 1, result.TotalExpected)
	require.Equal(t, 1, result.TotalMismatched)
	require.Contains(t, result.Results[0].SampleMismatched[0], "token_id expected 2301 got 1")
	require.Contains(t, result.Results[0].SampleMismatched[0], "amount_quota expected 777 got 1")
	require.Contains(t, result.Results[0].SampleMismatched[0], "billing_source expected subscription got wallet")

	details, err := ListBillingEventReconciliationMismatches(BillingEventReconciliationMismatchParams{
		Sources:     []string{BillingEventBackfillSourceModelRequest},
		Limit:       20,
		DetailLimit: 10,
	})
	require.NoError(t, err)
	require.Len(t, details.Items, 1)
	item := details.Items[0]
	require.Equal(t, BillingEventBackfillSourceModelRequest, item.Source)
	require.Equal(t, model.BillingEventSourceModelRequest+":"+requestId+":settlement", item.Label)
	require.Equal(t, 2301, item.Expected.TokenId)
	require.Equal(t, "vip", item.Expected.Group)
	require.Equal(t, "subscription", item.Expected.BillingSource)
	require.Contains(t, item.Diffs, dtoBillingEventDiff("billing_source", "subscription", "wallet"))

	repair, err := RepairBillingEventReconciliationMismatch(BillingEventReconciliationRepairParams{
		Source:  BillingEventBackfillSourceModelRequest,
		Label:   item.Label,
		Limit:   20,
		Reason:  "fix model request mismatch",
		AdminId: 1,
	})
	require.NoError(t, err)
	require.True(t, repair.Repaired)
	require.NotNil(t, repair.After)
	require.Equal(t, 2301, repair.After.TokenId)
	require.Equal(t, 777, repair.After.AmountQuota)
	require.Equal(t, -777, repair.After.QuotaDelta)
	require.Equal(t, model.BillingEventStatusSettled, repair.After.Status)
	require.Equal(t, "vip", repair.After.Group)
	require.Equal(t, "subscription", repair.After.BillingSource)
	require.NotNil(t, repair.AuditEvent)
	require.Equal(t, model.BillingEventSourceLedgerRepair, repair.AuditEvent.Source)

	repaired, err := ReconcileBillingEvents(BillingEventReconciliationParams{
		Sources: []string{BillingEventBackfillSourceModelRequest},
		Limit:   20,
	})
	require.NoError(t, err)
	require.Equal(t, 1, repaired.TotalLedgered)
	require.Equal(t, 0, repaired.TotalMismatched)
}

func dtoBillingEventDiff(field string, expected string, actual string) dto.BillingEventReconciliationDiff {
	return dto.BillingEventReconciliationDiff{Field: field, Expected: expected, Actual: actual}
}

func TestListBillingEventsKeepsMetadataFallbackForLegacyAuditRelations(t *testing.T) {
	truncate(t)
	withServiceTestQuotaPerUnit(t, 100)

	created, err := model.RecordFundingBillingEventIfNotExists(nil, model.FundingBillingEventInput{
		Source:        model.BillingEventSourceWalletTopUp,
		SourceId:      "legacy-audit-relation",
		Phase:         "success",
		UserId:        1701,
		RequestId:     "legacy-audit-relation",
		BillingSource: BillingSourceWallet,
		PriceUnit:     "topup",
		EventType:     model.BillingEventTypeCredit,
		AmountQuota:   123,
	})
	require.NoError(t, err)
	require.True(t, created)
	target := requireFundingBillingEvent(t, model.BillingEventSourceWalletTopUp, "legacy-audit-relation", "success")

	metadataBytes, err := common.Marshal(map[string]any{
		"admin_id":        17,
		"reason":          "legacy metadata relation",
		"label":           "wallet_topup:legacy-audit-relation:success",
		"target_event_pk": target.Id,
		"target_event_id": target.EventId,
	})
	require.NoError(t, err)
	audit := model.BillingEvent{
		EventId:       "billing_event_repair:legacy-audit-relation:repair",
		UserId:        target.UserId,
		Source:        model.BillingEventSourceLedgerRepair,
		SourceId:      "legacy-audit-relation-repair",
		EventType:     model.BillingEventTypeAudit,
		Status:        model.BillingEventStatusSettled,
		RequestId:     "legacy-audit-relation-repair",
		BillingSource: "ledger",
		PriceUnit:     "reconciliation_repair",
		Currency:      "quota",
		Metadata:      string(metadataBytes),
	}
	require.NoError(t, model.DB.Create(&audit).Error)

	var relationCount int64
	require.NoError(t, model.DB.Model(&model.BillingEventRelation{}).Count(&relationCount).Error)
	require.Zero(t, relationCount)

	items, total, err := ListBillingEvents(BillingEventListParams{
		Source:  model.BillingEventSourceWalletTopUp,
		Keyword: "legacy-audit-relation",
		Offset:  0,
		Limit:   10,
	})
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, items, 1)
	require.Len(t, items[0].RelatedAuditEvents, 1)
	require.Equal(t, audit.Id, items[0].RelatedAuditEvents[0].Id)
	require.Equal(t, "legacy metadata relation", items[0].RelatedAuditEvents[0].Reason)
	require.Equal(t, 17, items[0].RelatedAuditEvents[0].AdminId)

	audits, total, err := ListBillingEvents(BillingEventListParams{
		Source: model.BillingEventSourceLedgerRepair,
		Offset: 0,
		Limit:  10,
	})
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, audits, 1)
	require.NotNil(t, audits[0].RelatedTargetEvent)
	require.Equal(t, target.Id, audits[0].RelatedTargetEvent.Id)
}

func TestBackfillBillingEventRelationsCreatesRelationsForLegacyAudits(t *testing.T) {
	truncate(t)
	withServiceTestQuotaPerUnit(t, 100)

	target := seedLegacyRelationAudit(t, "legacy-relation-backfill", "reconciliation_repair", "target_event_pk")
	seedInvalidRelationAudit(t, "legacy-relation-invalid")

	health, err := GetBillingEventRelationHealth(BillingEventRelationMaintenanceParams{Limit: 10})
	require.NoError(t, err)
	require.True(t, health.NeedsReview)
	require.Equal(t, 2, health.ScannedAuditEvents)
	require.Equal(t, 1, health.MissingRelations)
	require.Equal(t, 1, health.InvalidAuditEvents)
	require.Len(t, health.SampleMissingRelations, 1)
	require.Equal(t, target.Id, health.SampleMissingRelations[0].TargetEventId)
	require.Len(t, health.SampleInvalidAudits, 1)
	require.NotEmpty(t, health.SampleInvalidAudits[0].Error)

	preview, err := BackfillBillingEventRelations(BillingEventRelationMaintenanceParams{Limit: 10, DryRun: true})
	require.NoError(t, err)
	require.True(t, preview.DryRun)
	require.Equal(t, 1, preview.WouldCreate)
	require.Equal(t, 0, preview.Created)
	require.Equal(t, 1, preview.SkippedInvalid)
	require.Len(t, preview.Items, 1)

	var relationCount int64
	require.NoError(t, model.DB.Model(&model.BillingEventRelation{}).Count(&relationCount).Error)
	require.Zero(t, relationCount)

	result, err := BackfillBillingEventRelations(BillingEventRelationMaintenanceParams{Limit: 10})
	require.NoError(t, err)
	require.False(t, result.DryRun)
	require.Equal(t, 1, result.Created)
	require.Equal(t, 1, result.SkippedInvalid)
	requireBillingEventRelation(t, result.Items[0].AuditEventId, target.Id, model.BillingEventRelationTypeReconciliationRepair, "legacy relation backfill", "wallet_topup:legacy-relation-backfill:success", 18)

	after, err := GetBillingEventRelationHealth(BillingEventRelationMaintenanceParams{Limit: 10})
	require.NoError(t, err)
	require.Equal(t, 0, after.MissingRelations)
	require.Equal(t, 1, after.InvalidAuditEvents)
	require.True(t, after.NeedsReview)

	repeated, err := BackfillBillingEventRelations(BillingEventRelationMaintenanceParams{Limit: 10})
	require.NoError(t, err)
	require.Equal(t, 0, repeated.Created)
	require.Equal(t, 1, repeated.SkippedExisting)
}

func TestRepairSelectedBillingEventRelations(t *testing.T) {
	truncate(t)
	withServiceTestQuotaPerUnit(t, 100)

	target := seedLegacyRelationAudit(t, "legacy-relation-selected", "reconciliation_repair", "target_event_pk")
	health, err := GetBillingEventRelationHealth(BillingEventRelationMaintenanceParams{Limit: 10})
	require.NoError(t, err)
	require.Equal(t, 1, health.MissingRelations)
	require.Len(t, health.SampleMissingRelations, 1)
	item := health.SampleMissingRelations[0]
	require.Equal(t, target.Id, item.TargetEventId)

	noop, err := RepairSelectedBillingEventRelations(BillingEventRelationSelectedRepairParams{})
	require.NoError(t, err)
	require.False(t, noop.DryRun)
	require.Zero(t, noop.Selected)
	require.Zero(t, noop.Created)

	preview, err := RepairSelectedBillingEventRelations(BillingEventRelationSelectedRepairParams{
		DryRun: true,
		Items:  []dto.BillingEventRelationMaintenanceItem{item},
	})
	require.NoError(t, err)
	require.True(t, preview.DryRun)
	require.Equal(t, 1, preview.Selected)
	require.Equal(t, 1, preview.WouldCreate)
	require.Zero(t, preview.Created)
	require.Zero(t, preview.SkippedExisting)
	require.Len(t, preview.Items, 1)

	var relationCount int64
	require.NoError(t, model.DB.Model(&model.BillingEventRelation{}).Count(&relationCount).Error)
	require.Zero(t, relationCount)

	repaired, err := RepairSelectedBillingEventRelations(BillingEventRelationSelectedRepairParams{
		Items: []dto.BillingEventRelationMaintenanceItem{item},
	})
	require.NoError(t, err)
	require.False(t, repaired.DryRun)
	require.Equal(t, 1, repaired.Selected)
	require.Equal(t, 1, repaired.Created)
	require.Zero(t, repaired.WouldCreate)
	require.Zero(t, repaired.SkippedExisting)
	require.Len(t, repaired.Items, 1)
	requireBillingEventRelation(t, repaired.Items[0].AuditEventId, target.Id, model.BillingEventRelationTypeReconciliationRepair, "legacy relation backfill", "wallet_topup:legacy-relation-selected:success", 18)

	repeated, err := RepairSelectedBillingEventRelations(BillingEventRelationSelectedRepairParams{
		Items: []dto.BillingEventRelationMaintenanceItem{item},
	})
	require.NoError(t, err)
	require.Zero(t, repeated.Created)
	require.Equal(t, 1, repeated.SkippedExisting)

	stale := item
	stale.TargetEventId = target.Id + 1000
	invalid, err := RepairSelectedBillingEventRelations(BillingEventRelationSelectedRepairParams{
		Items: []dto.BillingEventRelationMaintenanceItem{stale},
	})
	require.NoError(t, err)
	require.Zero(t, invalid.Created)
	require.Equal(t, 1, invalid.SkippedInvalid)
	require.NotEmpty(t, invalid.Errors)
}

func TestBillingEventRelationMaintenanceUsesCursor(t *testing.T) {
	truncate(t)
	withServiceTestQuotaPerUnit(t, 100)

	oldTarget := seedLegacyRelationAudit(t, "legacy-relation-cursor-old", "reconciliation_repair", "target_event_pk")
	newTarget := seedLegacyRelationAudit(t, "legacy-relation-cursor-new", "reconciliation_repair", "target_event_pk")

	first, err := GetBillingEventRelationHealth(BillingEventRelationMaintenanceParams{Limit: 1})
	require.NoError(t, err)
	require.Equal(t, int64(2), first.TotalAuditEvents)
	require.Equal(t, 1, first.ScannedAuditEvents)
	require.Equal(t, 1, first.MissingRelations)
	require.True(t, first.HasMore)
	require.False(t, first.ScanComplete)
	require.NotZero(t, first.NextCursor)
	require.Equal(t, newTarget.Id, first.SampleMissingRelations[0].TargetEventId)

	second, err := GetBillingEventRelationHealth(BillingEventRelationMaintenanceParams{Limit: 1, Cursor: first.NextCursor})
	require.NoError(t, err)
	require.Equal(t, 1, second.ScannedAuditEvents)
	require.Equal(t, 1, second.MissingRelations)
	require.False(t, second.HasMore)
	require.True(t, second.ScanComplete)
	require.Zero(t, second.NextCursor)
	require.Equal(t, oldTarget.Id, second.SampleMissingRelations[0].TargetEventId)

	backfilled, err := BackfillBillingEventRelations(BillingEventRelationMaintenanceParams{Limit: 1, Cursor: first.NextCursor})
	require.NoError(t, err)
	require.Equal(t, 1, backfilled.Created)
	require.False(t, backfilled.HasMore)
	requireBillingEventRelation(t, backfilled.Items[0].AuditEventId, oldTarget.Id, model.BillingEventRelationTypeReconciliationRepair, "legacy relation backfill", "wallet_topup:legacy-relation-cursor-old:success", 18)

	remaining, err := GetBillingEventRelationHealth(BillingEventRelationMaintenanceParams{Limit: 10})
	require.NoError(t, err)
	require.Equal(t, 1, remaining.MissingRelations)
	require.Equal(t, newTarget.Id, remaining.SampleMissingRelations[0].TargetEventId)
}

func TestCleanupBillingEventRelationOrphans(t *testing.T) {
	truncate(t)
	withServiceTestQuotaPerUnit(t, 100)

	target := seedLegacyRelationAudit(t, "legacy-relation-orphan-target", "reconciliation_repair", "target_event_pk")
	relation := model.BillingEventRelation{
		SourceEventId: 999001,
		TargetEventId: target.Id,
		RelationType:  model.BillingEventRelationTypeReconciliationRepair,
		Reason:        "orphan source",
	}
	require.NoError(t, model.DB.Create(&relation).Error)
	doubleOrphan := model.BillingEventRelation{
		SourceEventId: 999002,
		TargetEventId: 999003,
		RelationType:  model.BillingEventRelationTypeReconciliationRepair,
		Reason:        "double orphan",
	}
	require.NoError(t, model.DB.Create(&doubleOrphan).Error)

	preview, err := CleanupBillingEventRelationOrphans(BillingEventRelationMaintenanceParams{DryRun: true})
	require.NoError(t, err)
	require.True(t, preview.DryRun)
	require.Equal(t, 2, preview.SourceOrphans)
	require.Equal(t, 1, preview.TargetOrphans)
	require.Equal(t, 2, preview.WouldDelete)
	require.Zero(t, preview.Deleted)

	var count int64
	require.NoError(t, model.DB.Model(&model.BillingEventRelation{}).Count(&count).Error)
	require.EqualValues(t, 2, count)

	cleaned, err := CleanupBillingEventRelationOrphans(BillingEventRelationMaintenanceParams{})
	require.NoError(t, err)
	require.False(t, cleaned.DryRun)
	require.Equal(t, 2, cleaned.WouldDelete)
	require.Equal(t, 2, cleaned.Deleted)

	require.NoError(t, model.DB.Model(&model.BillingEventRelation{}).Count(&count).Error)
	require.Zero(t, count)
}

func TestBillingEventRelationInspectionSettingsPersistAndNormalize(t *testing.T) {
	truncate(t)

	cursor := int64(-12)
	status, err := UpdateBillingEventRelationInspectionSettings(dto.BillingEventRelationInspectionSettingsRequest{
		Enabled:            true,
		IntervalMinutes:    0,
		Limit:              999999,
		AutoBackfill:       true,
		AutoCleanupOrphans: true,
		Cursor:             &cursor,
	})
	require.NoError(t, err)
	require.True(t, status.Settings.Enabled)
	require.Equal(t, defaultBillingEventRelationInspectionIntervalMinutes, status.Settings.IntervalMinutes)
	require.Equal(t, maxBillingEventBackfillLimit, status.Settings.Limit)
	require.True(t, status.Settings.AutoBackfill)
	require.True(t, status.Settings.AutoCleanupOrphans)
	require.Equal(t, defaultBillingEventRelationInspectionMaxAutoBackfill, status.Settings.MaxAutoBackfill)
	require.Equal(t, defaultBillingEventRelationInspectionMaxAutoCleanup, status.Settings.MaxAutoCleanupOrphans)
	require.Zero(t, status.Settings.Cursor)

	loaded := GetBillingEventRelationInspectionStatus()
	require.Equal(t, status.Settings, loaded.Settings)
}

func TestRunBillingEventRelationInspectionBackfillsAndPersistsResult(t *testing.T) {
	truncate(t)
	withServiceTestQuotaPerUnit(t, 100)

	target := seedLegacyRelationAudit(t, "legacy-relation-inspection", "reconciliation_repair", "target_event_pk")
	_, err := UpdateBillingEventRelationInspectionSettings(dto.BillingEventRelationInspectionSettingsRequest{
		Enabled:         false,
		IntervalMinutes: 15,
		Limit:           10,
		AutoBackfill:    true,
	})
	require.NoError(t, err)

	run, err := RunBillingEventRelationInspectionOnce(true)
	require.NoError(t, err)
	require.True(t, run.Manual)
	require.Equal(t, "success", run.Status)
	require.Equal(t, 1, run.Health.MissingRelations)
	require.NotNil(t, run.Backfill)
	require.Equal(t, 1, run.Backfill.Created)
	require.NotEmpty(t, run.Backfill.Items)
	require.Zero(t, run.Settings.Cursor)
	requireBillingEventRelation(t, run.Backfill.Items[0].AuditEventId, target.Id, model.BillingEventRelationTypeReconciliationRepair, "legacy relation backfill", "wallet_topup:legacy-relation-inspection:success", 18)

	status := GetBillingEventRelationInspectionStatus()
	require.False(t, status.Running)
	require.Equal(t, "success", status.LastRunStatus)
	require.NotZero(t, status.LastRunAt)
	require.Contains(t, status.LastRunMessage, "backfilled=1")
	require.NotNil(t, status.LastHealth)
	require.NotNil(t, status.LastBackfill)
	require.Equal(t, 1, status.LastBackfill.Created)
	require.Zero(t, status.Settings.Cursor)
	require.Len(t, status.RecentRuns, 1)
	require.Equal(t, model.BillingEventRelationInspectionStatusSuccess, status.RecentRuns[0].Status)
	require.Equal(t, model.BillingEventRelationInspectionTriggerManual, status.RecentRuns[0].Trigger)
	require.Equal(t, 1, status.RecentRuns[0].BackfillCreated)
}

func TestRunBillingEventRelationInspectionCleansOrphans(t *testing.T) {
	truncate(t)
	withServiceTestQuotaPerUnit(t, 100)

	target := seedLegacyRelationAudit(t, "legacy-relation-inspection-clean", "reconciliation_repair", "target_event_pk")
	relation := model.BillingEventRelation{
		SourceEventId: 800001,
		TargetEventId: target.Id,
		RelationType:  model.BillingEventRelationTypeReconciliationRepair,
		Reason:        "inspection cleanup",
	}
	require.NoError(t, model.DB.Create(&relation).Error)
	_, err := UpdateBillingEventRelationInspectionSettings(dto.BillingEventRelationInspectionSettingsRequest{
		Enabled:            false,
		IntervalMinutes:    15,
		Limit:              10,
		AutoCleanupOrphans: true,
	})
	require.NoError(t, err)

	run, err := RunBillingEventRelationInspectionOnce(true)
	require.NoError(t, err)
	require.Equal(t, "success", run.Status)
	require.NotNil(t, run.Cleanup)
	require.Equal(t, 1, run.Cleanup.Deleted)

	var count int64
	require.NoError(t, model.DB.Model(&model.BillingEventRelation{}).Count(&count).Error)
	require.Zero(t, count)
	status := GetBillingEventRelationInspectionStatus()
	require.NotNil(t, status.LastCleanup)
	require.Equal(t, 1, status.LastCleanup.Deleted)
	require.Len(t, status.RecentRuns, 1)
	require.Equal(t, 1, status.RecentRuns[0].CleanupDeleted)
}

func TestRunBillingEventRelationInspectionBlocksAutoBackfillOverLimit(t *testing.T) {
	truncate(t)
	withServiceTestQuotaPerUnit(t, 100)

	seedLegacyRelationAudit(t, "legacy-relation-inspection-block-1", "reconciliation_repair", "target_event_pk")
	seedLegacyRelationAudit(t, "legacy-relation-inspection-block-2", "reconciliation_repair", "target_event_pk")
	_, err := UpdateBillingEventRelationInspectionSettings(dto.BillingEventRelationInspectionSettingsRequest{
		Enabled:         false,
		IntervalMinutes: 15,
		Limit:           10,
		AutoBackfill:    true,
		MaxAutoBackfill: 1,
	})
	require.NoError(t, err)

	run, err := RunBillingEventRelationInspectionOnce(true)
	require.NoError(t, err)
	require.Equal(t, model.BillingEventRelationInspectionStatusBlocked, run.Status)
	require.Contains(t, run.Message, "auto backfill blocked")
	require.Nil(t, run.Backfill)

	var relationCount int64
	require.NoError(t, model.DB.Model(&model.BillingEventRelation{}).Count(&relationCount).Error)
	require.Zero(t, relationCount)
	status := GetBillingEventRelationInspectionStatus()
	require.Equal(t, model.BillingEventRelationInspectionStatusBlocked, status.LastRunStatus)
	require.Len(t, status.RecentRuns, 1)
	require.True(t, status.RecentRuns[0].BackfillBlocked)
	require.Equal(t, 2, status.RecentRuns[0].BackfillWouldCreate)
	require.Zero(t, status.Settings.Cursor)
}

func TestListBillingEventRelationInspectionRunsPage(t *testing.T) {
	truncate(t)

	for i := 1; i <= 3; i++ {
		run := &model.BillingEventRelationInspectionRun{
			Trigger:             model.BillingEventRelationInspectionTriggerManual,
			Status:              model.BillingEventRelationInspectionStatusSuccess,
			Message:             fmt.Sprintf("paged inspection %d", i),
			Limit:               10,
			ScannedAuditEvents:  i,
			MissingRelations:    i + 10,
			StartedAt:           int64(100 + i),
			FinishedAt:          int64(200 + i),
			BackfillWouldCreate: i + 20,
		}
		require.NoError(t, model.CreateBillingEventRelationInspectionRun(run))
	}

	firstPage, total, err := ListBillingEventRelationInspectionRunsPage(0, 2)
	require.NoError(t, err)
	require.EqualValues(t, 3, total)
	require.Len(t, firstPage, 2)
	require.Equal(t, "paged inspection 3", firstPage[0].Message)
	require.Equal(t, 3, firstPage[0].ScannedAuditEvents)
	require.Equal(t, 23, firstPage[0].BackfillWouldCreate)
	require.Equal(t, "paged inspection 2", firstPage[1].Message)

	secondPage, total, err := ListBillingEventRelationInspectionRunsPage(2, 2)
	require.NoError(t, err)
	require.EqualValues(t, 3, total)
	require.Len(t, secondPage, 1)
	require.Equal(t, "paged inspection 1", secondPage[0].Message)
}

func requireBillingEventRelation(t *testing.T, sourceEventId int64, targetEventId int64, relationType string, reason string, label string, adminId int) model.BillingEventRelation {
	t.Helper()
	var relation model.BillingEventRelation
	require.NoError(t, model.DB.Where("source_event_id = ? AND target_event_id = ? AND relation_type = ?", sourceEventId, targetEventId, relationType).
		First(&relation).Error)
	require.Equal(t, reason, relation.Reason)
	require.Equal(t, billingEventRelationLabel(label), relation.Label)
	require.Equal(t, adminId, relation.AdminId)
	require.NotZero(t, relation.CreatedAt)
	return relation
}

func seedLegacyRelationAudit(t *testing.T, sourceId string, priceUnit string, targetKey string) model.BillingEvent {
	t.Helper()
	created, err := model.RecordFundingBillingEventIfNotExists(nil, model.FundingBillingEventInput{
		Source:        model.BillingEventSourceWalletTopUp,
		SourceId:      sourceId,
		Phase:         "success",
		UserId:        1801,
		RequestId:     sourceId,
		BillingSource: BillingSourceWallet,
		PriceUnit:     "topup",
		EventType:     model.BillingEventTypeCredit,
		AmountQuota:   123,
	})
	require.NoError(t, err)
	require.True(t, created)
	target := requireFundingBillingEvent(t, model.BillingEventSourceWalletTopUp, sourceId, "success")

	metadata := map[string]any{
		"admin_id":        18,
		"reason":          "legacy relation backfill",
		"label":           "wallet_topup:" + sourceId + ":success",
		"target_event_id": target.EventId,
	}
	switch targetKey {
	case "created_event_pk":
		metadata["created_event_pk"] = target.Id
	case "created_event":
		metadata["created_event"] = map[string]any{"id": target.Id}
	default:
		metadata["target_event_pk"] = target.Id
	}
	metadataBytes, err := common.Marshal(metadata)
	require.NoError(t, err)
	audit := model.BillingEvent{
		EventId:       "billing_event_repair:" + sourceId + ":repair",
		UserId:        target.UserId,
		Source:        model.BillingEventSourceLedgerRepair,
		SourceId:      sourceId + ":repair",
		EventType:     model.BillingEventTypeAudit,
		Status:        model.BillingEventStatusSettled,
		RequestId:     sourceId + ":repair",
		BillingSource: "ledger",
		PriceUnit:     priceUnit,
		Currency:      "quota",
		Metadata:      string(metadataBytes),
	}
	require.NoError(t, model.DB.Create(&audit).Error)
	return target
}

func seedInvalidRelationAudit(t *testing.T, sourceId string) model.BillingEvent {
	t.Helper()
	audit := model.BillingEvent{
		EventId:       "billing_event_repair:" + sourceId + ":repair",
		UserId:        1802,
		Source:        model.BillingEventSourceLedgerRepair,
		SourceId:      sourceId + ":repair",
		EventType:     model.BillingEventTypeAudit,
		Status:        model.BillingEventStatusSettled,
		RequestId:     sourceId + ":repair",
		BillingSource: "ledger",
		PriceUnit:     model.BillingEventRelationTypeReconciliationRepair,
		Currency:      "quota",
		Metadata:      `{"admin_id":18}`,
	}
	require.NoError(t, model.DB.Create(&audit).Error)
	return audit
}

func requireMissingBillingEventItem(t *testing.T, items []dto.BillingEventReconciliationMissingItem, label string) dto.BillingEventReconciliationMissingItem {
	t.Helper()
	for _, item := range items {
		if item.Label == label {
			return item
		}
	}
	require.Failf(t, "missing billing event item not found", "label=%s items=%v", label, items)
	return dto.BillingEventReconciliationMissingItem{}
}

func TestBillingEventReconciliationRepairMySQLSmoke(t *testing.T) {
	if os.Getenv("MCP_MIGRATION_TEST") != "1" {
		t.Skip("set MCP_MIGRATION_TEST=1 to run the billing event reconciliation MySQL smoke test")
	}
	if os.Getenv("SQL_DSN") == "" {
		t.Fatal("SQL_DSN is required")
	}

	common.InitEnv()
	logger.SetupLogger()
	if err := model.InitDB(); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	if err := model.InitLogDB(); err != nil {
		t.Fatalf("InitLogDB failed: %v", err)
	}

	withServiceTestQuotaPerUnit(t, 100)

	suffix := fmt.Sprintf("reconcile-mysql-smoke-%d", time.Now().UnixNano())
	tradeNo := suffix + ":topup"
	var userId int
	t.Cleanup(func() {
		_ = model.DB.Where(
			"source_event_id IN (?) OR target_event_id IN (?)",
			model.DB.Model(&model.BillingEvent{}).Select("id").Where("source = ? AND metadata LIKE ?", model.BillingEventSourceLedgerRepair, "%"+tradeNo+"%"),
			model.DB.Model(&model.BillingEvent{}).Select("id").Where("source_id = ?", tradeNo),
		).Delete(&model.BillingEventRelation{}).Error
		_ = model.DB.Where("source = ? AND metadata LIKE ?", model.BillingEventSourceLedgerRepair, "%"+tradeNo+"%").Delete(&model.BillingEvent{}).Error
		_ = model.DB.Where("source_id = ?", tradeNo).Delete(&model.BillingEvent{}).Error
		_ = model.DB.Where("trade_no = ?", tradeNo).Delete(&model.TopUp{}).Error
		if userId > 0 {
			_ = model.DB.Unscoped().Where("id = ?", userId).Delete(&model.User{}).Error
		}
		_ = model.CloseDB()
	})

	user := &model.User{
		Username: suffix,
		Status:   common.UserStatusEnabled,
		Quota:    0,
	}
	require.NoError(t, model.DB.Create(user).Error)
	userId = user.Id

	topUp := &model.TopUp{
		UserId:          user.Id,
		Amount:          4,
		Money:           4,
		TradeNo:         tradeNo,
		PaymentMethod:   model.PaymentMethodWaffo,
		PaymentProvider: model.PaymentProviderWaffo,
		CreateTime:      common.GetTimestamp() - 10,
		CompleteTime:    common.GetTimestamp(),
		Status:          common.TopUpStatusSuccess,
	}
	require.NoError(t, model.DB.Create(topUp).Error)

	created, err := model.RecordFundingBillingEventIfNotExists(nil, model.FundingBillingEventInput{
		Source:        model.BillingEventSourceWalletTopUp,
		SourceId:      tradeNo,
		Phase:         "success",
		UserId:        user.Id,
		RequestId:     tradeNo,
		BillingSource: BillingSourceWallet,
		PriceUnit:     "topup",
		EventType:     model.BillingEventTypeCredit,
		AmountQuota:   1,
	})
	require.NoError(t, err)
	require.True(t, created)
	require.NoError(t, model.DB.Model(&model.BillingEvent{}).
		Where("source = ? AND source_id = ?", model.BillingEventSourceWalletTopUp, tradeNo).
		Update("status", "pending").Error)

	item, found, err := findBillingEventReconciliationMismatch(BillingEventBackfillSourceWalletTopUp, model.BillingEventSourceWalletTopUp+":"+tradeNo, 5000)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, model.BillingEventSourceWalletTopUp+":"+tradeNo+":success", item.Label)
	require.Equal(t, 400, item.Expected.AmountQuota)
	require.NotNil(t, item.Actual)
	require.Equal(t, "pending", item.Actual.Status)

	repair, err := RepairBillingEventReconciliationMismatch(BillingEventReconciliationRepairParams{
		Source:  BillingEventBackfillSourceWalletTopUp,
		Label:   item.Label,
		Limit:   5000,
		Reason:  "mysql smoke repair",
		AdminId: user.Id,
	})
	require.NoError(t, err)
	require.True(t, repair.Repaired)
	require.NotNil(t, repair.After)
	require.Equal(t, 400, repair.After.AmountQuota)
	require.Equal(t, model.BillingEventStatusSettled, repair.After.Status)
	require.NotNil(t, repair.AuditEvent)
	require.Equal(t, model.BillingEventSourceLedgerRepair, repair.AuditEvent.Source)
	requireBillingEventRelation(t, repair.AuditEvent.Id, repair.After.Id, model.BillingEventRelationTypeReconciliationRepair, "mysql smoke repair", item.Label, user.Id)

	_, found, err = findBillingEventReconciliationMismatch(BillingEventBackfillSourceWalletTopUp, item.Label, 5000)
	require.NoError(t, err)
	require.False(t, found)
}

func TestBillingEventRelationInspectionMySQLSmoke(t *testing.T) {
	if os.Getenv("MCP_MIGRATION_TEST") != "1" {
		t.Skip("set MCP_MIGRATION_TEST=1 to run the billing event relation inspection MySQL smoke test")
	}
	if os.Getenv("SQL_DSN") == "" {
		t.Fatal("SQL_DSN is required")
	}

	common.InitEnv()
	logger.SetupLogger()
	if err := model.InitDB(); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	if err := model.InitLogDB(); err != nil {
		t.Fatalf("InitLogDB failed: %v", err)
	}

	suffix := fmt.Sprintf("inspection-mysql-smoke-%d", time.Now().UnixNano())
	var targetId int64
	t.Cleanup(func() {
		_ = model.DB.Where(
			"source_event_id IN (?) OR target_event_id IN (?)",
			model.DB.Model(&model.BillingEvent{}).Select("id").Where("source = ? AND source_id LIKE ?", model.BillingEventSourceLedgerRepair, suffix+"%"),
			model.DB.Model(&model.BillingEvent{}).Select("id").Where("source_id LIKE ?", suffix+"%"),
		).Delete(&model.BillingEventRelation{}).Error
		_ = model.DB.Where("source_id LIKE ?", suffix+"%").Delete(&model.BillingEvent{}).Error
		_ = model.DB.Where("source = ? AND source_id LIKE ?", model.BillingEventSourceLedgerRepair, suffix+"%").Delete(&model.BillingEvent{}).Error
		_ = model.DB.Where("id = ?", targetId).Delete(&model.BillingEvent{}).Error
		_ = model.DB.Where("message LIKE ?", "%"+suffix+"%").Delete(&model.BillingEventRelationInspectionRun{}).Error
		_ = model.DB.Where("`key` LIKE ?", "BillingEventRelationInspection%").Delete(&model.Option{}).Error
		common.OptionMapRWMutex.Lock()
		common.OptionMap = map[string]string{}
		common.OptionMapRWMutex.Unlock()
		_ = model.CloseDB()
	})

	created, err := model.RecordFundingBillingEventIfNotExists(nil, model.FundingBillingEventInput{
		Source:        model.BillingEventSourceWalletTopUp,
		SourceId:      suffix + ":target",
		Phase:         "success",
		UserId:        88001,
		RequestId:     suffix,
		BillingSource: BillingSourceWallet,
		PriceUnit:     "topup",
		EventType:     model.BillingEventTypeCredit,
		AmountQuota:   123,
	})
	require.NoError(t, err)
	require.True(t, created)
	target := requireFundingBillingEvent(t, model.BillingEventSourceWalletTopUp, suffix+":target", "success")
	targetId = target.Id

	metadataBytes, err := common.Marshal(map[string]any{
		"admin_id":        88,
		"reason":          "mysql inspection smoke " + suffix,
		"label":           "wallet_topup:" + suffix + ":target:success",
		"target_event_pk": target.Id,
	})
	require.NoError(t, err)
	audit := model.BillingEvent{
		EventId:       "billing_event_repair:" + suffix + ":repair",
		UserId:        target.UserId,
		Source:        model.BillingEventSourceLedgerRepair,
		SourceId:      suffix + ":repair",
		EventType:     model.BillingEventTypeAudit,
		Status:        model.BillingEventStatusSettled,
		RequestId:     suffix + ":repair",
		BillingSource: "ledger",
		PriceUnit:     model.BillingEventRelationTypeReconciliationRepair,
		Currency:      "quota",
		Metadata:      string(metadataBytes),
	}
	require.NoError(t, model.DB.Create(&audit).Error)

	_, err = UpdateBillingEventRelationInspectionSettings(dto.BillingEventRelationInspectionSettingsRequest{
		Enabled:         false,
		IntervalMinutes: 30,
		Limit:           10,
		AutoBackfill:    true,
		MaxAutoBackfill: 10,
	})
	require.NoError(t, err)
	run, err := RunBillingEventRelationInspectionOnce(true)
	require.NoError(t, err)
	require.Equal(t, model.BillingEventRelationInspectionStatusSuccess, run.Status)
	require.NotNil(t, run.Backfill)
	require.GreaterOrEqual(t, run.Backfill.Created, 1)
	requireBillingEventRelation(t, audit.Id, target.Id, model.BillingEventRelationTypeReconciliationRepair, "mysql inspection smoke "+suffix, "wallet_topup:"+suffix+":target:success", 88)

	status := GetBillingEventRelationInspectionStatus()
	require.Equal(t, model.BillingEventRelationInspectionStatusSuccess, status.LastRunStatus)
	require.NotEmpty(t, status.RecentRuns)
}
