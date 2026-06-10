package service

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
)

const (
	BillingEventSourceCapabilityReady      = "ready"
	BillingEventSourceCapabilityRecordOnly = "record_only"
	BillingEventSourceCapabilityPlanned    = "planned"
	BillingEventSourceCapabilityAuditOnly  = "audit_only"
)

func GetBillingEventSourceMatrix() dto.BillingEventSourceMatrixResponse {
	items := []dto.BillingEventSourceCapabilityItem{
		{
			Source:                  model.BillingEventSourceMCPToolCall,
			EventSource:             model.BillingEventSourceMCPToolCall,
			Label:                   "MCP Tool Call",
			Status:                  BillingEventSourceCapabilityReady,
			BackfillSources:         []string{BillingEventBackfillSourceMCPToolCall},
			SupportsRecording:       true,
			SupportsBackfill:        true,
			SupportsReconciliation:  true,
			SupportsMissingBackfill: true,
			SupportsRefundOrDelta:   true,
			SupportsRepairAudit:     true,
			SupportsAuditRelation:   true,
			Notes: []string{
				"Successful MCP tool calls write settlement ledger events; executor and billing precheck failures do not deduct quota.",
				"Refund events are recorded for post-settlement executor failures.",
				"QidianBrowser real client integration is intentionally last; current local-client path remains mock-backed.",
				"Backfill and reconciliation rebuild expected settlement/refund events from mcp_tool_calls.",
			},
		},
		{
			Source:                  model.BillingEventSourceModelRequest,
			EventSource:             model.BillingEventSourceModelRequest,
			Label:                   "Model Request",
			Status:                  BillingEventSourceCapabilityReady,
			BackfillSources:         []string{BillingEventBackfillSourceModelRequest},
			SupportsRecording:       true,
			SupportsBackfill:        true,
			SupportsReconciliation:  true,
			SupportsMissingBackfill: true,
			SupportsRepairAudit:     true,
			SupportsAuditRelation:   true,
			Notes: []string{
				"Runtime text, audio, realtime, and Midjourney per-call success paths write model_request settlement events.",
				"Midjourney post-submit failures write model_request refund events when the original task had charged quota.",
				"Backfill and reconciliation rebuild expected settlement events from consume logs with request IDs.",
			},
		},
		{
			Source:                  model.BillingEventSourceAsyncTask,
			EventSource:             model.BillingEventSourceAsyncTask,
			Label:                   "Async Task",
			Status:                  BillingEventSourceCapabilityReady,
			BackfillSources:         []string{BillingEventBackfillSourceAsyncTask},
			SupportsRecording:       true,
			SupportsBackfill:        true,
			SupportsReconciliation:  true,
			SupportsMissingBackfill: true,
			SupportsRefundOrDelta:   true,
			SupportsRepairAudit:     true,
			SupportsAuditRelation:   true,
			Notes: []string{
				"Task initial settlement, recalculation delta, and failure refund events write task_billing_records and billing_events.",
				"Backfill and reconciliation rebuild expected events from task_billing_records rather than guessing from final task rows.",
			},
		},
		{
			Source:                  model.BillingEventSourceViolationFee,
			EventSource:             model.BillingEventSourceViolationFee,
			Label:                   "Violation Fee",
			Status:                  BillingEventSourceCapabilityReady,
			BackfillSources:         []string{BillingEventBackfillSourceViolationFee},
			SupportsRecording:       true,
			SupportsBackfill:        true,
			SupportsReconciliation:  true,
			SupportsMissingBackfill: true,
			SupportsRepairAudit:     true,
			SupportsAuditRelation:   true,
			Notes: []string{
				"Violation fee debits write violation_fee_records and billing_events.",
				"Backfill and reconciliation rebuild expected events from violation_fee_records.",
			},
		},
		{
			Source:                  model.BillingEventSourceWalletTopUp,
			EventSource:             model.BillingEventSourceWalletTopUp,
			Label:                   "Wallet Top-up",
			Status:                  BillingEventSourceCapabilityReady,
			BackfillSources:         []string{BillingEventBackfillSourceWalletTopUp, BillingEventBackfillSourceRedemption},
			SupportsRecording:       true,
			SupportsBackfill:        true,
			SupportsReconciliation:  true,
			SupportsMissingBackfill: true,
			SupportsRepairAudit:     true,
			SupportsAuditRelation:   true,
			Notes: []string{
				"Top-up success and redemption credits share the wallet_topup ledger source.",
				"Dedicated source handlers cover backfill, mismatch detection, and missing-event repair.",
			},
		},
		{
			Source:                  model.BillingEventSourceWalletAdjust,
			EventSource:             model.BillingEventSourceWalletAdjust,
			Label:                   "Wallet Adjust",
			Status:                  BillingEventSourceCapabilityReady,
			BackfillSources:         []string{BillingEventBackfillSourceWalletAdjust},
			SupportsRecording:       true,
			SupportsBackfill:        true,
			SupportsReconciliation:  true,
			SupportsMissingBackfill: true,
			SupportsRefundOrDelta:   true,
			SupportsRepairAudit:     true,
			SupportsAuditRelation:   true,
			Notes: []string{
				"Admin wallet adjustments write wallet_adjustments and billing_events in one transaction.",
				"Backfill and reconciliation rebuild expected events from wallet_adjustments.",
			},
		},
		{
			Source:                  model.BillingEventSourceSubscription,
			EventSource:             model.BillingEventSourceSubscription,
			Label:                   "Subscription",
			Status:                  BillingEventSourceCapabilityReady,
			BackfillSources:         []string{BillingEventBackfillSourceSubscriptionPurchase, BillingEventBackfillSourceSubscriptionBalance, BillingEventBackfillSourceSubscriptionAdmin},
			SupportsRecording:       true,
			SupportsBackfill:        true,
			SupportsReconciliation:  true,
			SupportsMissingBackfill: true,
			SupportsRefundOrDelta:   true,
			SupportsRepairAudit:     true,
			SupportsAuditRelation:   true,
			Notes: []string{
				"Purchase, balance payment/grant, and admin bind flows are covered by separate source handlers.",
				"Subscription invalidation and refund paths keep their own source metadata for audit.",
			},
		},
		{
			Source:                model.BillingEventSourceLedgerRepair,
			EventSource:           model.BillingEventSourceLedgerRepair,
			Label:                 "Billing Event Repair",
			Status:                BillingEventSourceCapabilityAuditOnly,
			SupportsRecording:     true,
			SupportsRepairAudit:   true,
			SupportsAuditRelation: true,
			Notes: []string{
				"Repair and missing-event backfill write audit-only ledger events.",
				"Relation inspection owns audit-target link health and orphan cleanup.",
			},
		},
	}

	response := dto.BillingEventSourceMatrixResponse{
		CheckedAt:    common.GetTimestamp(),
		Items:        items,
		TotalSources: len(items),
	}
	for index := range response.Items {
		if response.Items[index].BackfillSources == nil {
			response.Items[index].BackfillSources = []string{}
		}
		if response.Items[index].Notes == nil {
			response.Items[index].Notes = []string{}
		}
		item := response.Items[index]
		switch item.Status {
		case BillingEventSourceCapabilityReady:
			response.ReadySources++
		case BillingEventSourceCapabilityRecordOnly:
			response.RecordOnlySources++
		case BillingEventSourceCapabilityPlanned:
			response.PlannedSources++
		case BillingEventSourceCapabilityAuditOnly:
			response.AuditOnlySources++
		}
	}
	return response
}
