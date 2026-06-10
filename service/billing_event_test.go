package service

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestListBillingEventsFiltersBySourceId(t *testing.T) {
	truncate(t)

	require.NoError(t, model.DB.Create(&model.BillingEvent{
		EventId:   "billing-source-id-a",
		UserId:    1,
		Source:    model.BillingEventSourceMCPToolCall,
		SourceId:  "call-a",
		EventType: model.BillingEventTypeDebit,
		Status:    model.BillingEventStatusSettled,
		RequestId: "request-a",
	}).Error)
	require.NoError(t, model.DB.Create(&model.BillingEvent{
		EventId:   "billing-source-id-b",
		UserId:    1,
		Source:    model.BillingEventSourceMCPToolCall,
		SourceId:  "call-b",
		EventType: model.BillingEventTypeDebit,
		Status:    model.BillingEventStatusSettled,
		RequestId: "request-b",
	}).Error)

	items, total, err := ListBillingEvents(BillingEventListParams{
		Source:   model.BillingEventSourceMCPToolCall,
		SourceId: "call-b",
		Limit:    10,
	})
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, items, 1)
	require.Equal(t, "call-b", items[0].SourceId)
	require.Equal(t, "request-b", items[0].RequestId)
}

func TestGetBillingEventSummaryAggregatesWindow(t *testing.T) {
	truncate(t)

	dayOne := int64(1735689600)
	dayTwo := dayOne + 86400
	events := []model.BillingEvent{
		{
			EventId:     "billing-summary-credit",
			UserId:      1,
			Source:      model.BillingEventSourceWalletTopUp,
			SourceId:    "topup-1",
			EventType:   model.BillingEventTypeCredit,
			Status:      model.BillingEventStatusSettled,
			AmountQuota: 100,
			QuotaDelta:  100,
			Cost:        12.5,
			CreatedAt:   dayOne + 60,
		},
		{
			EventId:     "billing-summary-mcp",
			UserId:      1,
			Source:      model.BillingEventSourceMCPToolCall,
			SourceId:    "call-1",
			EventType:   model.BillingEventTypeDebit,
			Status:      model.BillingEventStatusSettled,
			AmountQuota: 30,
			QuotaDelta:  -30,
			Cost:        0.03,
			Metadata:    `{"usage_kind":"text"}`,
			CreatedAt:   dayOne + 120,
		},
		{
			EventId:     "billing-summary-model",
			UserId:      1,
			Source:      model.BillingEventSourceModelRequest,
			SourceId:    "log-1",
			EventType:   model.BillingEventTypeDebit,
			Status:      model.BillingEventStatusSettled,
			AmountQuota: 20,
			QuotaDelta:  -20,
			Cost:        0.02,
			Metadata:    `{"usage_kind":"audio"}`,
			CreatedAt:   dayTwo + 60,
		},
		{
			EventId:   "billing-summary-audit",
			UserId:    1,
			Source:    model.BillingEventSourceLedgerRepair,
			SourceId:  "repair-1",
			EventType: model.BillingEventTypeAudit,
			Status:    model.BillingEventStatusSettled,
			CreatedAt: dayTwo + 120,
		},
		{
			EventId:     "billing-summary-outside-user",
			UserId:      2,
			Source:      model.BillingEventSourceWalletTopUp,
			SourceId:    "topup-2",
			EventType:   model.BillingEventTypeCredit,
			Status:      model.BillingEventStatusSettled,
			AmountQuota: 999,
			QuotaDelta:  999,
			CreatedAt:   dayOne + 180,
		},
	}
	require.NoError(t, model.DB.Create(&events).Error)

	summary, err := GetBillingEventSummary(BillingEventListParams{
		UserId:    1,
		StartTime: dayOne,
		EndTime:   dayTwo + 86399,
	})
	require.NoError(t, err)
	require.EqualValues(t, 4, summary.Totals.TotalEvents)
	require.EqualValues(t, 1, summary.Totals.CreditEvents)
	require.EqualValues(t, 2, summary.Totals.DebitEvents)
	require.EqualValues(t, 1, summary.Totals.AuditEvents)
	require.EqualValues(t, 50, summary.Totals.NetQuotaDelta)
	require.EqualValues(t, 100, summary.Totals.CreditQuotaDelta)
	require.EqualValues(t, -50, summary.Totals.DebitQuotaDelta)
	require.InDelta(t, 12.55, summary.Totals.TotalCost, 0.001)
	require.Len(t, summary.DailyTrend, 2)
	require.EqualValues(t, dayOne, summary.DailyTrend[0].BucketStart)
	require.EqualValues(t, 2, summary.DailyTrend[0].TotalEvents)
	require.EqualValues(t, dayTwo, summary.DailyTrend[1].BucketStart)
	require.EqualValues(t, 2, summary.DailyTrend[1].TotalEvents)

	bySource := billingSummaryDimensionsByKey(summary.BySource)
	require.EqualValues(t, 1, bySource[model.BillingEventSourceWalletTopUp].TotalEvents)
	require.EqualValues(t, 1, bySource[model.BillingEventSourceMCPToolCall].TotalEvents)
	require.EqualValues(t, 1, bySource[model.BillingEventSourceModelRequest].TotalEvents)
	require.EqualValues(t, 1, bySource[model.BillingEventSourceLedgerRepair].TotalEvents)

	textSummary, err := GetBillingEventSummary(BillingEventListParams{
		UserId:    1,
		UsageKind: model.BillingEventUsageKindText,
		StartTime: dayOne,
		EndTime:   dayTwo + 86399,
	})
	require.NoError(t, err)
	require.EqualValues(t, 1, textSummary.Totals.TotalEvents)
	require.EqualValues(t, -30, textSummary.Totals.NetQuotaDelta)
}

func billingSummaryDimensionsByKey(items []dto.BillingEventSummaryDimension) map[string]dto.BillingEventSummaryDimension {
	result := make(map[string]dto.BillingEventSummaryDimension, len(items))
	for _, item := range items {
		result[item.Key] = item
	}
	return result
}
