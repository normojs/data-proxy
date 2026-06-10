package service

import (
	"fmt"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
)

type billingEventSourceHandler struct {
	Source                string
	Backfill              func(limit int, dryRun bool) (dto.BillingEventBackfillSourceResult, error)
	Reconcile             func(limit int, details *billingEventMismatchDetails) (dto.BillingEventReconciliationSourceResult, error)
	MissingBackfillInput  func(expected billingEventExpectation, extraMetadata map[string]any) (model.FundingBillingEventInput, string, error)
	LegacyTargetNormalize func(label string) string
}

var billingEventSourceHandlers = map[string]billingEventSourceHandler{
	BillingEventBackfillSourceMCPToolCall: {
		Source:               BillingEventBackfillSourceMCPToolCall,
		Backfill:             backfillMCPToolCalls,
		Reconcile:            reconcileMCPToolCalls,
		MissingBackfillInput: mcpToolCallMissingBackfillInput,
	},
	BillingEventBackfillSourceWalletTopUp: {
		Source:                BillingEventBackfillSourceWalletTopUp,
		Backfill:              backfillWalletTopUps,
		Reconcile:             reconcileWalletTopUps,
		MissingBackfillInput:  walletTopUpMissingBackfillInput,
		LegacyTargetNormalize: normalizeWalletTopUpReconciliationTargetLabel,
	},
	BillingEventBackfillSourceWalletAdjust: {
		Source:               BillingEventBackfillSourceWalletAdjust,
		Backfill:             backfillWalletAdjustments,
		Reconcile:            reconcileWalletAdjustments,
		MissingBackfillInput: walletAdjustMissingBackfillInput,
	},
	BillingEventBackfillSourceRedemption: {
		Source:               BillingEventBackfillSourceRedemption,
		Backfill:             backfillRedemptions,
		Reconcile:            reconcileRedemptions,
		MissingBackfillInput: redemptionMissingBackfillInput,
	},
	BillingEventBackfillSourceModelRequest: {
		Source:               BillingEventBackfillSourceModelRequest,
		Backfill:             backfillModelRequests,
		Reconcile:            reconcileModelRequests,
		MissingBackfillInput: modelRequestMissingBackfillInput,
	},
	BillingEventBackfillSourceAsyncTask: {
		Source:               BillingEventBackfillSourceAsyncTask,
		Backfill:             backfillTaskBillingRecords,
		Reconcile:            reconcileTaskBillingRecords,
		MissingBackfillInput: taskBillingRecordMissingBackfillInput,
	},
	BillingEventBackfillSourceViolationFee: {
		Source:               BillingEventBackfillSourceViolationFee,
		Backfill:             backfillViolationFeeRecords,
		Reconcile:            reconcileViolationFeeRecords,
		MissingBackfillInput: violationFeeRecordMissingBackfillInput,
	},
	BillingEventBackfillSourceSubscriptionPurchase: {
		Source:               BillingEventBackfillSourceSubscriptionPurchase,
		Backfill:             backfillSubscriptionPurchases,
		Reconcile:            reconcileSubscriptionPurchases,
		MissingBackfillInput: subscriptionPurchaseMissingBackfillInputFromExpectation,
	},
	BillingEventBackfillSourceSubscriptionBalance: {
		Source:               BillingEventBackfillSourceSubscriptionBalance,
		Backfill:             backfillSubscriptionBalanceOrders,
		Reconcile:            reconcileSubscriptionBalanceOrders,
		MissingBackfillInput: subscriptionBalanceMissingBackfillInputFromExpectation,
	},
	BillingEventBackfillSourceSubscriptionAdmin: {
		Source:               BillingEventBackfillSourceSubscriptionAdmin,
		Backfill:             backfillAdminSubscriptions,
		Reconcile:            reconcileAdminSubscriptions,
		MissingBackfillInput: adminSubscriptionMissingBackfillInputFromExpectation,
	},
}

var defaultBillingEventBackfillSources = []string{
	BillingEventBackfillSourceMCPToolCall,
	BillingEventBackfillSourceWalletTopUp,
	BillingEventBackfillSourceWalletAdjust,
	BillingEventBackfillSourceRedemption,
	BillingEventBackfillSourceModelRequest,
	BillingEventBackfillSourceAsyncTask,
	BillingEventBackfillSourceViolationFee,
	BillingEventBackfillSourceSubscriptionPurchase,
	BillingEventBackfillSourceSubscriptionBalance,
	BillingEventBackfillSourceSubscriptionAdmin,
}

func getBillingEventSourceHandler(source string) (billingEventSourceHandler, bool) {
	handler, ok := billingEventSourceHandlers[source]
	return handler, ok
}

func backfillBillingEventSource(source string, limit int, dryRun bool) (dto.BillingEventBackfillSourceResult, error) {
	handler, ok := getBillingEventSourceHandler(source)
	if !ok || handler.Backfill == nil {
		return dto.BillingEventBackfillSourceResult{}, fmt.Errorf("unsupported billing event backfill source: %s", source)
	}
	return handler.Backfill(limit, dryRun)
}

func reconcileBillingEventSource(source string, limit int, details *billingEventMismatchDetails) (dto.BillingEventReconciliationSourceResult, error) {
	handler, ok := getBillingEventSourceHandler(source)
	if !ok || handler.Reconcile == nil {
		return dto.BillingEventReconciliationSourceResult{}, fmt.Errorf("unsupported billing event reconciliation source: %s", source)
	}
	return handler.Reconcile(limit, details)
}

func missingBackfillInputForBillingEventSource(source string, expected billingEventExpectation, extraMetadata map[string]any) (model.FundingBillingEventInput, string, error) {
	handler, ok := getBillingEventSourceHandler(source)
	if !ok || handler.MissingBackfillInput == nil {
		return model.FundingBillingEventInput{}, "", fmt.Errorf("unsupported billing event reconciliation source: %s", source)
	}
	return handler.MissingBackfillInput(expected, extraMetadata)
}
