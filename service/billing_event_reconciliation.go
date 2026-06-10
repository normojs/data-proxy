package service

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const maxBillingEventReconciliationSamples = 20
const defaultBillingEventReconciliationDetailLimit = 50
const maxBillingEventReconciliationDetailLimit = 500

type BillingEventReconciliationParams struct {
	Sources []string
	Limit   int
}

type BillingEventReconciliationMismatchParams struct {
	Sources     []string
	Limit       int
	DetailLimit int
}

type BillingEventReconciliationMissingParams struct {
	Sources     []string
	Limit       int
	DetailLimit int
}

type BillingEventReconciliationRepairParams struct {
	Source   string
	Label    string
	Limit    int
	Reason   string
	AdminId  int
	ActualId int64
	Expected *dto.BillingEventReconciliationExpectedEvent
}

type BillingEventReconciliationBackfillMissingParams struct {
	Source   string
	Label    string
	Limit    int
	Reason   string
	AdminId  int
	Expected *dto.BillingEventReconciliationExpectedEvent
}

func ReconcileBillingEvents(params BillingEventReconciliationParams) (dto.BillingEventReconciliationResponse, error) {
	limit := normalizeBillingEventBackfillLimit(params.Limit)
	sources, err := normalizeBillingEventBackfillSources(params.Sources)
	if err != nil {
		return dto.BillingEventReconciliationResponse{}, err
	}
	response := dto.BillingEventReconciliationResponse{
		Limit:   limit,
		Sources: sources,
		Results: make([]dto.BillingEventReconciliationSourceResult, 0, len(sources)),
	}

	for _, source := range sources {
		result, err := reconcileBillingEventSource(source, limit, nil)
		if err != nil {
			return response, err
		}
		response.Results = append(response.Results, result)
		response.TotalScanned += result.Scanned
		response.TotalExpected += result.Expected
		response.TotalLedgered += result.Ledgered
		response.TotalMissing += result.Missing
		response.TotalMismatched += result.Mismatched
		response.TotalInvalid += result.Invalid
		response.TotalErrorCount += result.ErrorCount
		response.HasMore = response.HasMore || result.HasMore
	}
	response.ScanComplete = !response.HasMore

	return response, nil
}

func ListBillingEventReconciliationMismatches(params BillingEventReconciliationMismatchParams) (dto.BillingEventReconciliationMismatchResponse, error) {
	limit := normalizeBillingEventBackfillLimit(params.Limit)
	detailLimit := normalizeBillingEventReconciliationDetailLimit(params.DetailLimit)
	sources, err := normalizeBillingEventBackfillSources(params.Sources)
	if err != nil {
		return dto.BillingEventReconciliationMismatchResponse{}, err
	}
	response := dto.BillingEventReconciliationMismatchResponse{
		Limit:       limit,
		DetailLimit: detailLimit,
		Sources:     sources,
		Items:       []dto.BillingEventReconciliationMismatchItem{},
	}

	for _, source := range sources {
		details := &billingEventMismatchDetails{
			ReconciliationSource: source,
			DetailLimit:          detailLimit,
			Items:                &response.Items,
		}
		result, err := reconcileBillingEventSource(source, limit, details)
		if err != nil {
			return response, err
		}
		response.TotalScanned += result.Scanned
		response.TotalExpected += result.Expected
		response.TotalMismatched += result.Mismatched
		response.TotalMissing += result.Missing
		response.TotalInvalid += result.Invalid
		response.TotalErrorCount += result.ErrorCount
		response.HasMore = response.HasMore || result.HasMore
	}
	response.HasMore = response.HasMore || len(response.Items) < response.TotalMismatched
	response.ScanComplete = !response.HasMore
	return response, nil
}

func ListBillingEventReconciliationMissing(params BillingEventReconciliationMissingParams) (dto.BillingEventReconciliationMissingResponse, error) {
	limit := normalizeBillingEventBackfillLimit(params.Limit)
	detailLimit := normalizeBillingEventReconciliationDetailLimit(params.DetailLimit)
	sources, err := normalizeBillingEventBackfillSources(params.Sources)
	if err != nil {
		return dto.BillingEventReconciliationMissingResponse{}, err
	}
	response := dto.BillingEventReconciliationMissingResponse{
		Limit:       limit,
		DetailLimit: detailLimit,
		Sources:     sources,
		Items:       []dto.BillingEventReconciliationMissingItem{},
	}

	for _, source := range sources {
		details := &billingEventMismatchDetails{
			ReconciliationSource: source,
			DetailLimit:          detailLimit,
			MissingItems:         &response.Items,
		}
		result, err := reconcileBillingEventSource(source, limit, details)
		if err != nil {
			return response, err
		}
		response.TotalScanned += result.Scanned
		response.TotalExpected += result.Expected
		response.TotalMissing += result.Missing
		response.TotalMismatched += result.Mismatched
		response.TotalInvalid += result.Invalid
		response.TotalErrorCount += result.ErrorCount
		response.HasMore = response.HasMore || result.HasMore
	}
	response.HasMore = response.HasMore || len(response.Items) < response.TotalMissing
	response.ScanComplete = !response.HasMore
	return response, nil
}

func normalizeBillingEventReconciliationDetailLimit(limit int) int {
	if limit <= 0 {
		return defaultBillingEventReconciliationDetailLimit
	}
	if limit > maxBillingEventReconciliationDetailLimit {
		return maxBillingEventReconciliationDetailLimit
	}
	return limit
}

func RepairBillingEventReconciliationMismatch(params BillingEventReconciliationRepairParams) (dto.BillingEventReconciliationRepairResponse, error) {
	source := strings.TrimSpace(params.Source)
	label := strings.TrimSpace(params.Label)
	reason := strings.TrimSpace(params.Reason)
	if source == "" || label == "" {
		return dto.BillingEventReconciliationRepairResponse{}, fmt.Errorf("source and label are required")
	}
	if reason == "" {
		return dto.BillingEventReconciliationRepairResponse{}, fmt.Errorf("repair reason is required")
	}

	item, found, err := resolveBillingEventReconciliationMismatch(params, source, label)
	if err != nil {
		return dto.BillingEventReconciliationRepairResponse{}, err
	}
	if !found {
		return dto.BillingEventReconciliationRepairResponse{}, fmt.Errorf("billing event mismatch not found: %s", label)
	}
	if item.Actual == nil {
		return dto.BillingEventReconciliationRepairResponse{}, fmt.Errorf("missing billing event cannot be repaired by mismatch repair: %s", label)
	}

	expected := reconciliationExpectedEventToExpectation(item.Expected)
	response := dto.BillingEventReconciliationRepairResponse{
		Repaired: true,
		Label:    item.Label,
		Source:   item.Source,
		Expected: item.Expected,
		Diffs:    item.Diffs,
		Before:   item.Actual,
	}

	err = model.DB.Transaction(func(tx *gorm.DB) error {
		var current model.BillingEvent
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", item.Actual.Id).
			First(&current).Error; err != nil {
			return err
		}

		diffs := compareBillingEventExpectation(expected, current)
		if len(diffs) == 0 {
			return fmt.Errorf("billing event mismatch already repaired: %s", label)
		}
		beforeDTO := billingEventToDTO(current)
		response.Before = &beforeDTO
		response.Diffs = diffs

		updates := map[string]any{
			"user_id":      expected.UserId,
			"source":       expected.Source,
			"source_id":    truncateBillingEventString(expected.SourceId, 128),
			"event_type":   expected.EventType,
			"status":       expected.Status,
			"amount_quota": expected.AmountQuota,
			"quota_delta":  expected.QuotaDelta,
			"cost":         billingEventCost(expected.AmountQuota),
		}
		if expected.CheckTokenId {
			updates["token_id"] = expected.TokenId
		}
		if strings.TrimSpace(expected.RequestId) != "" {
			updates["request_id"] = truncateBillingEventString(expected.RequestId, 128)
		}
		if strings.TrimSpace(expected.Group) != "" {
			updates["group"] = expected.Group
		}
		if strings.TrimSpace(expected.BillingSource) != "" {
			updates["billing_source"] = expected.BillingSource
		}
		if strings.TrimSpace(expected.PriceUnit) != "" {
			updates["price_unit"] = expected.PriceUnit
		}
		if strings.TrimSpace(expected.Currency) != "" {
			updates["currency"] = expected.Currency
		}
		if err := tx.Model(&model.BillingEvent{}).Where("id = ?", current.Id).Updates(updates).Error; err != nil {
			return err
		}

		var updated model.BillingEvent
		if err := tx.Where("id = ?", current.Id).First(&updated).Error; err != nil {
			return err
		}
		afterDTO := billingEventToDTO(updated)
		response.After = &afterDTO

		audit, err := createBillingEventRepairAudit(tx, params.AdminId, reason, item, beforeDTO, afterDTO, diffs)
		if err != nil {
			return err
		}
		if _, err := model.CreateBillingEventRelationIfNotExists(tx, &model.BillingEventRelation{
			SourceEventId: audit.Id,
			TargetEventId: afterDTO.Id,
			RelationType:  model.BillingEventRelationTypeReconciliationRepair,
			Reason:        billingEventRelationReason(reason),
			Label:         billingEventRelationLabel(item.Label),
			AdminId:       params.AdminId,
		}); err != nil {
			return err
		}
		auditDTO := billingEventToDTO(audit)
		response.AuditEvent = &auditDTO
		return nil
	})
	if err != nil {
		return dto.BillingEventReconciliationRepairResponse{}, err
	}
	return response, nil
}

type billingEventReconciliationScanStop struct {
	reachedLimit bool
}

func (s *billingEventReconciliationScanStop) shouldStop() bool {
	return s != nil && s.reachedLimit
}

func (s *billingEventReconciliationScanStop) markIfLimitReached(result *dto.BillingEventReconciliationSourceResult, limit int) {
	if s == nil || result == nil || limit <= 0 {
		return
	}
	if result.Missing+result.Mismatched+result.Invalid+result.ErrorCount >= limit {
		s.reachedLimit = true
	}
}

func finishBillingEventReconciliationScan(result *dto.BillingEventReconciliationSourceResult, stop billingEventReconciliationScanStop, sawMore bool) {
	if result == nil {
		return
	}
	result.HasMore = sawMore
	result.ScanComplete = !result.HasMore
}

func findBillingEventReconciliationMismatch(source string, label string, limit int) (dto.BillingEventReconciliationMismatchItem, bool, error) {
	label = normalizeBillingEventReconciliationTargetLabel(source, label)
	items := []dto.BillingEventReconciliationMismatchItem{}
	details := &billingEventMismatchDetails{
		ReconciliationSource: source,
		DetailLimit:          1,
		TargetLabel:          label,
		Items:                &items,
	}
	_, err := reconcileBillingEventSource(source, limit, details)
	if err != nil {
		return dto.BillingEventReconciliationMismatchItem{}, false, err
	}
	if len(items) == 0 {
		return dto.BillingEventReconciliationMismatchItem{}, false, nil
	}
	return items[0], true, nil
}

func findBillingEventReconciliationMissing(source string, label string, limit int) (dto.BillingEventReconciliationMissingItem, bool, error) {
	label = normalizeBillingEventReconciliationTargetLabel(source, label)
	items := []dto.BillingEventReconciliationMissingItem{}
	details := &billingEventMismatchDetails{
		ReconciliationSource: source,
		DetailLimit:          1,
		TargetLabel:          label,
		MissingItems:         &items,
	}
	_, err := reconcileBillingEventSource(source, limit, details)
	if err != nil {
		return dto.BillingEventReconciliationMissingItem{}, false, err
	}
	if len(items) == 0 {
		return dto.BillingEventReconciliationMissingItem{}, false, nil
	}
	return items[0], true, nil
}

func resolveBillingEventReconciliationMismatch(params BillingEventReconciliationRepairParams, source string, label string) (dto.BillingEventReconciliationMismatchItem, bool, error) {
	if params.Expected == nil || params.ActualId <= 0 {
		return findBillingEventReconciliationMismatch(source, label, normalizeBillingEventBackfillLimit(params.Limit))
	}
	expectedDTO := *params.Expected
	if strings.TrimSpace(expectedDTO.Label) == "" {
		expectedDTO.Label = label
	}
	expected := reconciliationExpectedEventToExpectation(expectedDTO)

	var actual model.BillingEvent
	if err := model.DB.Where("id = ?", params.ActualId).First(&actual).Error; err != nil {
		return dto.BillingEventReconciliationMismatchItem{}, false, err
	}
	diffs := compareBillingEventExpectation(expected, actual)
	if len(diffs) == 0 {
		return dto.BillingEventReconciliationMismatchItem{}, false, nil
	}
	actualDTO := billingEventToDTO(actual)
	return dto.BillingEventReconciliationMismatchItem{
		Source:   source,
		Label:    billingEventExpectationLabel(expected),
		Expected: billingEventExpectedEventToDTO(expected),
		Actual:   &actualDTO,
		Diffs:    diffs,
	}, true, nil
}

func resolveBillingEventReconciliationMissing(params BillingEventReconciliationBackfillMissingParams, source string, label string) (dto.BillingEventReconciliationMissingItem, bool, error) {
	if params.Expected == nil {
		return findBillingEventReconciliationMissing(source, label, normalizeBillingEventBackfillLimit(params.Limit))
	}
	expectedDTO := *params.Expected
	if strings.TrimSpace(expectedDTO.Label) == "" {
		expectedDTO.Label = label
	}
	expected := reconciliationExpectedEventToExpectation(expectedDTO)
	return dto.BillingEventReconciliationMissingItem{
		Source:   source,
		Label:    billingEventExpectationLabel(expected),
		Expected: billingEventExpectedEventToDTO(expected),
	}, true, nil
}

func normalizeBillingEventReconciliationTargetLabel(source string, label string) string {
	label = strings.TrimSpace(label)
	if handler, ok := getBillingEventSourceHandler(source); ok && handler.LegacyTargetNormalize != nil {
		return handler.LegacyTargetNormalize(label)
	}
	return label
}

func normalizeWalletTopUpReconciliationTargetLabel(label string) string {
	label = strings.TrimSpace(label)
	if strings.HasPrefix(label, model.BillingEventSourceWalletTopUp+":") &&
		!strings.HasSuffix(label, ":success") {
		return label + ":success"
	}
	return label
}

func reconciliationExpectedEventToExpectation(expected dto.BillingEventReconciliationExpectedEvent) billingEventExpectation {
	return normalizeBillingEventExpectation(billingEventExpectation{
		Source:        expected.Source,
		SourceId:      expected.SourceId,
		Phase:         expected.Phase,
		UserId:        expected.UserId,
		TokenId:       expected.TokenId,
		CheckTokenId:  expected.TokenId > 0,
		EventType:     expected.EventType,
		AmountQuota:   expected.AmountQuota,
		QuotaDelta:    expected.QuotaDelta,
		Status:        expected.Status,
		RequestId:     expected.RequestId,
		Group:         expected.Group,
		BillingSource: expected.BillingSource,
		PriceUnit:     expected.PriceUnit,
		Currency:      expected.Currency,
	})
}

func BackfillBillingEventReconciliationMissing(params BillingEventReconciliationBackfillMissingParams) (dto.BillingEventReconciliationBackfillMissingResponse, error) {
	source := strings.TrimSpace(params.Source)
	label := strings.TrimSpace(params.Label)
	reason := strings.TrimSpace(params.Reason)
	if source == "" || label == "" {
		return dto.BillingEventReconciliationBackfillMissingResponse{}, fmt.Errorf("source and label are required")
	}
	if reason == "" {
		return dto.BillingEventReconciliationBackfillMissingResponse{}, fmt.Errorf("backfill reason is required")
	}

	item, found, err := resolveBillingEventReconciliationMissing(params, source, label)
	if err != nil {
		return dto.BillingEventReconciliationBackfillMissingResponse{}, err
	}
	if !found {
		return dto.BillingEventReconciliationBackfillMissingResponse{}, fmt.Errorf("missing billing event not found: %s", label)
	}

	input, err := billingEventMissingBackfillInput(item, params.AdminId, reason)
	if err != nil {
		return dto.BillingEventReconciliationBackfillMissingResponse{}, err
	}
	response := dto.BillingEventReconciliationBackfillMissingResponse{
		Backfilled: true,
		Label:      item.Label,
		Source:     item.Source,
		Expected:   item.Expected,
	}

	err = model.DB.Transaction(func(tx *gorm.DB) error {
		created, err := model.RecordFundingBillingEventIfNotExists(tx, input)
		if err != nil {
			return err
		}
		if !created {
			return fmt.Errorf("missing billing event already exists: %s", label)
		}
		event, exists, err := model.GetFundingBillingEvent(tx, input.Source, input.SourceId, input.Phase)
		if err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("created billing event could not be loaded: %s", label)
		}
		eventDTO := billingEventToDTO(event)
		response.Event = &eventDTO
		audit, err := createBillingEventMissingBackfillAudit(tx, params.AdminId, reason, item, eventDTO)
		if err != nil {
			return err
		}
		if _, err := model.CreateBillingEventRelationIfNotExists(tx, &model.BillingEventRelation{
			SourceEventId: audit.Id,
			TargetEventId: event.Id,
			RelationType:  model.BillingEventRelationTypeReconciliationBackfillMissing,
			Reason:        billingEventRelationReason(reason),
			Label:         billingEventRelationLabel(item.Label),
			AdminId:       params.AdminId,
		}); err != nil {
			return err
		}
		auditDTO := billingEventToDTO(audit)
		response.AuditEvent = &auditDTO
		return nil
	})
	if err != nil {
		return dto.BillingEventReconciliationBackfillMissingResponse{}, err
	}
	return response, nil
}

func billingEventMissingBackfillInput(item dto.BillingEventReconciliationMissingItem, adminId int, reason string) (model.FundingBillingEventInput, error) {
	expected := reconciliationExpectedEventToExpectation(item.Expected)
	extraMetadata := map[string]any{
		"reconciliation_backfill": true,
		"admin_id":                adminId,
		"reason":                  truncateBillingEventRepairReason(reason, 512),
		"label":                   item.Label,
		"reconciliation_source":   item.Source,
	}

	input, invalid, err := missingBackfillInputForBillingEventSource(item.Source, expected, extraMetadata)
	if err != nil {
		return input, err
	}
	if invalid != "" {
		return model.FundingBillingEventInput{}, fmt.Errorf("%s", invalid)
	}
	if diffs := compareBillingEventExpectationToInput(expected, input); len(diffs) > 0 {
		return model.FundingBillingEventInput{}, fmt.Errorf("source data changed for %s: %s", item.Label, formatBillingEventMismatch(expected, diffs))
	}
	return input, nil
}

func walletTopUpMissingBackfillInput(expected billingEventExpectation, extraMetadata map[string]any) (model.FundingBillingEventInput, string, error) {
	var topUp model.TopUp
	err := model.DB.Model(&model.TopUp{}).
		Joins("LEFT JOIN subscription_orders ON subscription_orders.trade_no = top_ups.trade_no").
		Where("top_ups.status = ? AND subscription_orders.id IS NULL", common.TopUpStatusSuccess).
		Where("top_ups.trade_no = ?", expected.SourceId).
		First(&topUp).Error
	if err != nil {
		return model.FundingBillingEventInput{}, "", err
	}
	input, invalid := walletTopUpBackfillInput(topUp, extraMetadata)
	return input, invalid, nil
}

func redemptionMissingBackfillInput(expected billingEventExpectation, extraMetadata map[string]any) (model.FundingBillingEventInput, string, error) {
	redemptionId, err := parseBillingEventPrefixedId(expected.SourceId, "redemption:")
	if err != nil {
		return model.FundingBillingEventInput{}, "", err
	}
	var redemption model.Redemption
	err = model.DB.Unscoped().
		Where("status = ? AND used_user_id > 0", common.RedemptionCodeStatusUsed).
		Where("id = ?", redemptionId).
		First(&redemption).Error
	if err != nil {
		return model.FundingBillingEventInput{}, "", err
	}
	input, invalid := redemptionBackfillInput(redemption, extraMetadata)
	return input, invalid, nil
}

func walletAdjustMissingBackfillInput(expected billingEventExpectation, extraMetadata map[string]any) (model.FundingBillingEventInput, string, error) {
	adjustment, exists, err := model.GetWalletAdjustmentBySourceId(nil, expected.SourceId)
	if err != nil {
		return model.FundingBillingEventInput{}, "", err
	}
	if !exists {
		return model.FundingBillingEventInput{}, "", gorm.ErrRecordNotFound
	}
	input, invalid := walletAdjustmentBackfillInput(adjustment, extraMetadata)
	return input, invalid, nil
}

func modelRequestMissingBackfillInput(expected billingEventExpectation, extraMetadata map[string]any) (model.FundingBillingEventInput, string, error) {
	if model.LOG_DB == nil {
		return model.FundingBillingEventInput{}, "", fmt.Errorf("log database is not initialized")
	}
	var log model.Log
	err := model.LOG_DB.Model(&model.Log{}).
		Where("type = ? AND quota > 0 AND request_id = ?", model.LogTypeConsume, expected.SourceId).
		Order("id asc").
		First(&log).Error
	if err != nil {
		return model.FundingBillingEventInput{}, "", err
	}
	input, invalid := modelRequestBackfillInput(log, extraMetadata)
	return input, invalid, nil
}

func mcpToolCallMissingBackfillInput(expected billingEventExpectation, extraMetadata map[string]any) (model.FundingBillingEventInput, string, error) {
	callId, err := strconv.ParseInt(expected.SourceId, 10, 64)
	if err != nil || callId <= 0 {
		return model.FundingBillingEventInput{}, "", fmt.Errorf("invalid mcp_tool_call source_id %s", expected.SourceId)
	}
	call, exists, err := model.GetMCPToolCallById(nil, callId)
	if err != nil {
		return model.FundingBillingEventInput{}, "", err
	}
	if !exists {
		return model.FundingBillingEventInput{}, "", gorm.ErrRecordNotFound
	}
	input, invalid := mcpToolCallBackfillInput(call, expected.Phase, extraMetadata)
	return input, invalid, nil
}

func taskBillingRecordMissingBackfillInput(expected billingEventExpectation, extraMetadata map[string]any) (model.FundingBillingEventInput, string, error) {
	record, exists, err := model.GetTaskBillingRecordBySourcePhase(nil, expected.SourceId, expected.Phase)
	if err != nil {
		return model.FundingBillingEventInput{}, "", err
	}
	if !exists {
		return model.FundingBillingEventInput{}, "", gorm.ErrRecordNotFound
	}
	input, invalid := taskBillingRecordBackfillInput(record, extraMetadata)
	return input, invalid, nil
}

func violationFeeRecordMissingBackfillInput(expected billingEventExpectation, extraMetadata map[string]any) (model.FundingBillingEventInput, string, error) {
	record, exists, err := model.GetViolationFeeRecordBySourcePhase(nil, expected.SourceId, expected.Phase)
	if err != nil {
		return model.FundingBillingEventInput{}, "", err
	}
	if !exists {
		return model.FundingBillingEventInput{}, "", gorm.ErrRecordNotFound
	}
	input, invalid := violationFeeRecordBackfillInput(record, extraMetadata)
	return input, invalid, nil
}

func subscriptionPurchaseMissingBackfillInputFromExpectation(expected billingEventExpectation, extraMetadata map[string]any) (model.FundingBillingEventInput, string, error) {
	var order model.SubscriptionOrder
	err := model.DB.Where("status = ? AND payment_provider <> ? AND payment_method <> ?",
		common.TopUpStatusSuccess, model.PaymentProviderBalance, model.PaymentMethodBalance).
		Where("trade_no = ?", expected.SourceId).
		First(&order).Error
	if err != nil {
		return model.FundingBillingEventInput{}, "", err
	}
	input, invalid := subscriptionPurchaseMissingBackfillInput(order, extraMetadata)
	return input, invalid, nil
}

func subscriptionBalanceMissingBackfillInputFromExpectation(expected billingEventExpectation, extraMetadata map[string]any) (model.FundingBillingEventInput, string, error) {
	var order model.SubscriptionOrder
	err := model.DB.Where("status = ? AND (payment_provider = ? OR payment_method = ?)",
		common.TopUpStatusSuccess, model.PaymentProviderBalance, model.PaymentMethodBalance).
		Where("trade_no = ?", expected.SourceId).
		First(&order).Error
	if err != nil {
		return model.FundingBillingEventInput{}, "", err
	}
	input, invalid := subscriptionBalanceMissingBackfillInput(order, expected.Phase, extraMetadata)
	return input, invalid, nil
}

func adminSubscriptionMissingBackfillInputFromExpectation(expected billingEventExpectation, extraMetadata map[string]any) (model.FundingBillingEventInput, string, error) {
	subscriptionId, err := parseBillingEventPrefixedId(expected.SourceId, "admin_bind:")
	if err != nil {
		return model.FundingBillingEventInput{}, "", err
	}
	var sub model.UserSubscription
	err = model.DB.Where("source = ? AND id = ?", "admin", subscriptionId).First(&sub).Error
	if err != nil {
		return model.FundingBillingEventInput{}, "", err
	}
	input, invalid := adminSubscriptionMissingBackfillInput(sub, extraMetadata)
	return input, invalid, nil
}

func subscriptionPurchaseMissingBackfillInput(order model.SubscriptionOrder, extraMetadata map[string]any) (model.FundingBillingEventInput, string) {
	tradeNo := strings.TrimSpace(order.TradeNo)
	if tradeNo == "" || order.UserId <= 0 || order.PlanId <= 0 {
		return model.FundingBillingEventInput{}, fmt.Sprintf("subscription_order:%d invalid purchase data", order.Id)
	}
	plan, err := model.GetSubscriptionPlanById(order.PlanId)
	if err != nil {
		return model.FundingBillingEventInput{}, fmt.Sprintf("subscription_order:%s missing plan %d: %v", tradeNo, order.PlanId, err)
	}
	subscription := findBackfillSubscription(order, "order")
	return subscriptionPurchaseBackfillInput(order, plan, subscription, extraMetadata), ""
}

func subscriptionBalanceMissingBackfillInput(order model.SubscriptionOrder, phase string, extraMetadata map[string]any) (model.FundingBillingEventInput, string) {
	tradeNo := strings.TrimSpace(order.TradeNo)
	if tradeNo == "" || order.UserId <= 0 || order.PlanId <= 0 {
		return model.FundingBillingEventInput{}, fmt.Sprintf("subscription_order:%d invalid balance data", order.Id)
	}
	plan, err := model.GetSubscriptionPlanById(order.PlanId)
	if err != nil {
		return model.FundingBillingEventInput{}, fmt.Sprintf("subscription_order:%s missing plan %d: %v", tradeNo, order.PlanId, err)
	}
	subscription := findBackfillSubscription(order, model.PaymentMethodBalance)
	createdAt := backfillEventTime(order.CompleteTime, order.CreateTime)
	switch phase {
	case "balance_payment":
		requiredQuota, err := backfillSubscriptionBalanceQuota(order, plan)
		if err != nil {
			return model.FundingBillingEventInput{}, fmt.Sprintf("subscription_order:%s invalid balance quota: %v", tradeNo, err)
		}
		if requiredQuota <= 0 {
			return model.FundingBillingEventInput{}, fmt.Sprintf("subscription_order:%s balance_payment quota is zero", tradeNo)
		}
		return subscriptionBalancePaymentBackfillInput(order, plan, subscription.Id, requiredQuota, createdAt, extraMetadata), ""
	case "grant":
		return subscriptionGrantBackfillInput(order, plan, subscription, createdAt, extraMetadata), ""
	default:
		return model.FundingBillingEventInput{}, fmt.Sprintf("subscription_order:%s unsupported balance phase %s", tradeNo, phase)
	}
}

func adminSubscriptionMissingBackfillInput(sub model.UserSubscription, extraMetadata map[string]any) (model.FundingBillingEventInput, string) {
	if sub.Id <= 0 || sub.UserId <= 0 || sub.PlanId <= 0 {
		return model.FundingBillingEventInput{}, fmt.Sprintf("user_subscription:%d invalid admin subscription data", sub.Id)
	}
	plan, err := model.GetSubscriptionPlanById(sub.PlanId)
	if err != nil {
		return model.FundingBillingEventInput{}, fmt.Sprintf("user_subscription:%d missing plan %d: %v", sub.Id, sub.PlanId, err)
	}
	return adminSubscriptionBackfillInput(sub, plan, extraMetadata), ""
}

func walletAdjustmentBillingExpectation(adjustment model.WalletAdjustment) (billingEventExpectation, string) {
	sourceId := strings.TrimSpace(adjustment.SourceId)
	if sourceId == "" || adjustment.UserId <= 0 || adjustment.Amount <= 0 {
		return billingEventExpectation{}, fmt.Sprintf("wallet_adjustment:%d invalid adjustment data", adjustment.Id)
	}
	eventType := strings.TrimSpace(adjustment.EventType)
	if eventType != model.BillingEventTypeCredit && eventType != model.BillingEventTypeDebit {
		return billingEventExpectation{}, fmt.Sprintf("wallet_adjustment:%s invalid event_type %s", sourceId, adjustment.EventType)
	}
	quotaDelta := adjustment.Amount
	if eventType == model.BillingEventTypeDebit {
		quotaDelta = -adjustment.Amount
	}
	return billingEventExpectation{
		Source:        model.BillingEventSourceWalletAdjust,
		SourceId:      sourceId,
		Phase:         "adjust",
		UserId:        adjustment.UserId,
		EventType:     eventType,
		AmountQuota:   adjustment.Amount,
		QuotaDelta:    quotaDelta,
		Status:        model.BillingEventStatusSettled,
		RequestId:     sourceId,
		BillingSource: BillingSourceWallet,
		PriceUnit:     "manual_adjust",
		Currency:      "quota",
	}, ""
}

func mcpToolCallBillingExpectations(call model.MCPToolCall) ([]billingEventExpectation, string) {
	if call.Id <= 0 || call.UserId <= 0 || strings.TrimSpace(call.RequestId) == "" {
		return nil, fmt.Sprintf("mcp_tool_call:%d invalid call data", call.Id)
	}
	expectations := []billingEventExpectation{}
	settlementEvent, settlementExists, err := model.GetFundingBillingEvent(nil, model.BillingEventSourceMCPToolCall, fmt.Sprintf("%d", call.Id), "settlement")
	if err != nil {
		return nil, fmt.Sprintf("mcp_tool_call:%d settlement lookup failed: %v", call.Id, err)
	}
	if call.SettledAt > 0 && call.Quota > 0 {
		expectations = append(expectations, mcpToolCallSettlementExpectation(call, call.Quota, call.Cost, call.PriceUnit))
	} else if settlementExists {
		expectations = append(expectations, mcpToolCallSettlementExpectation(call, settlementEvent.AmountQuota, settlementEvent.Cost, settlementEvent.PriceUnit))
	}
	if mcpToolCallShouldExpectRefund(call) && settlementExists {
		expectations = append(expectations, mcpToolCallRefundExpectation(call, settlementEvent.AmountQuota))
	}
	return expectations, ""
}

func mcpToolCallSettlementExpectation(call model.MCPToolCall, quota int, cost float64, priceUnit string) billingEventExpectation {
	if strings.TrimSpace(priceUnit) == "" {
		priceUnit = model.MCPToolPriceUnitPerCall
	}
	return billingEventExpectation{
		Source:        model.BillingEventSourceMCPToolCall,
		SourceId:      fmt.Sprintf("%d", call.Id),
		Phase:         "settlement",
		UserId:        call.UserId,
		TokenId:       call.TokenId,
		CheckTokenId:  call.TokenId > 0,
		EventType:     model.BillingEventTypeDebit,
		AmountQuota:   quota,
		QuotaDelta:    -quota,
		Status:        model.BillingEventStatusSettled,
		RequestId:     call.RequestId,
		BillingSource: BillingSourceWallet,
		PriceUnit:     priceUnit,
		Currency:      "quota",
		Cost:          cost,
		CheckCost:     true,
	}
}

func mcpToolCallRefundExpectation(call model.MCPToolCall, quota int) billingEventExpectation {
	return billingEventExpectation{
		Source:        model.BillingEventSourceMCPToolCall,
		SourceId:      fmt.Sprintf("%d", call.Id),
		Phase:         "refund",
		UserId:        call.UserId,
		TokenId:       call.TokenId,
		CheckTokenId:  call.TokenId > 0,
		EventType:     model.BillingEventTypeCredit,
		AmountQuota:   quota,
		QuotaDelta:    quota,
		Status:        model.BillingEventStatusSettled,
		RequestId:     call.RequestId,
		BillingSource: BillingSourceWallet,
		PriceUnit:     "mcp_refund",
		Currency:      "quota",
	}
}

func mcpToolCallShouldExpectRefund(call model.MCPToolCall) bool {
	if call.SettledAt <= 0 {
		return false
	}
	return call.Status == model.MCPToolCallStatusError || call.Status == model.MCPToolCallStatusTimeout
}

func mcpToolCallBackfillInput(call model.MCPToolCall, phase string, extraMetadata map[string]any) (model.FundingBillingEventInput, string) {
	expectations, invalid := mcpToolCallBillingExpectations(call)
	if invalid != "" {
		return model.FundingBillingEventInput{}, invalid
	}
	var expected billingEventExpectation
	for _, item := range expectations {
		if item.Phase == phase {
			expected = item
			break
		}
	}
	if strings.TrimSpace(expected.SourceId) == "" {
		return model.FundingBillingEventInput{}, fmt.Sprintf("mcp_tool_call:%d no expected %s event", call.Id, phase)
	}
	metadata := mcpToolCallMetadata(call, phase, extraMetadata)
	cost := expected.Cost
	return model.FundingBillingEventInput{
		Source:        model.BillingEventSourceMCPToolCall,
		SourceId:      expected.SourceId,
		Phase:         expected.Phase,
		UserId:        expected.UserId,
		TokenId:       expected.TokenId,
		RequestId:     expected.RequestId,
		BillingSource: expected.BillingSource,
		PriceUnit:     expected.PriceUnit,
		EventType:     expected.EventType,
		AmountQuota:   expected.AmountQuota,
		Cost:          &cost,
		CreatedAt:     call.SettledAt,
		Metadata:      metadata,
	}, ""
}

func mcpToolCallMetadata(call model.MCPToolCall, phase string, extraMetadata map[string]any) map[string]any {
	metadata := map[string]any{
		"mcp_tool_call_id":  call.Id,
		"tool_id":           call.ToolId,
		"tool_name":         call.ToolName,
		"status":            call.Status,
		"phase":             phase,
		"free_used":         call.FreeUsed,
		"bridge_session_id": call.BridgeSessionId,
		"target_client":     call.TargetClient,
		"result_size":       call.ResultSize,
		"duration_ms":       call.DurationMS,
		"error_code":        call.ErrorCode,
		"error_message":     call.ErrorMessage,
	}
	for key, value := range extraMetadata {
		metadata[key] = value
	}
	return metadata
}

func taskBillingRecordExpectation(record model.TaskBillingRecord) (billingEventExpectation, string) {
	sourceId := strings.TrimSpace(record.SourceId)
	phase := strings.TrimSpace(record.Phase)
	if sourceId == "" || phase == "" || record.UserId <= 0 || record.AmountQuota <= 0 {
		return billingEventExpectation{}, fmt.Sprintf("task_billing_record:%d invalid task billing data", record.Id)
	}
	eventType := strings.TrimSpace(record.EventType)
	if eventType != model.BillingEventTypeCredit && eventType != model.BillingEventTypeDebit {
		return billingEventExpectation{}, fmt.Sprintf("task_billing_record:%d invalid event_type %s", record.Id, record.EventType)
	}
	expectedDelta := record.AmountQuota
	if eventType == model.BillingEventTypeDebit {
		expectedDelta = -record.AmountQuota
	}
	if record.QuotaDelta != expectedDelta {
		return billingEventExpectation{}, fmt.Sprintf("task_billing_record:%d invalid quota_delta %d for %s %d", record.Id, record.QuotaDelta, eventType, record.AmountQuota)
	}
	billingSource := strings.TrimSpace(record.BillingSource)
	if billingSource == "" {
		billingSource = BillingSourceWallet
	}
	requestId := strings.TrimSpace(record.RequestId)
	if requestId == "" {
		requestId = sourceId
	}
	priceUnit := strings.TrimSpace(record.PriceUnit)
	if priceUnit == "" {
		priceUnit = expectedPriceUnitForExpectation(billingEventExpectation{
			Source: model.BillingEventSourceAsyncTask,
			Phase:  phase,
		})
	}
	return billingEventExpectation{
		Source:        model.BillingEventSourceAsyncTask,
		SourceId:      sourceId,
		Phase:         phase,
		UserId:        record.UserId,
		TokenId:       record.TokenId,
		CheckTokenId:  record.TokenId > 0,
		EventType:     eventType,
		AmountQuota:   record.AmountQuota,
		QuotaDelta:    record.QuotaDelta,
		Status:        model.BillingEventStatusSettled,
		RequestId:     requestId,
		Group:         record.Group,
		BillingSource: billingSource,
		PriceUnit:     priceUnit,
		Currency:      "quota",
	}, ""
}

func taskBillingRecordBackfillInput(record model.TaskBillingRecord, extraMetadata map[string]any) (model.FundingBillingEventInput, string) {
	expected, invalid := taskBillingRecordExpectation(record)
	if invalid != "" {
		return model.FundingBillingEventInput{}, invalid
	}
	metadata := taskBillingRecordMetadata(record, extraMetadata)
	return model.FundingBillingEventInput{
		Source:        model.BillingEventSourceAsyncTask,
		SourceId:      expected.SourceId,
		Phase:         expected.Phase,
		UserId:        expected.UserId,
		TokenId:       expected.TokenId,
		RequestId:     expected.RequestId,
		Group:         expected.Group,
		BillingSource: expected.BillingSource,
		PriceUnit:     expected.PriceUnit,
		EventType:     expected.EventType,
		AmountQuota:   expected.AmountQuota,
		CreatedAt:     record.CreatedAt,
		Metadata:      metadata,
	}, ""
}

func violationFeeRecordExpectation(record model.ViolationFeeRecord) (billingEventExpectation, string) {
	sourceId := strings.TrimSpace(record.SourceId)
	phase := strings.TrimSpace(record.Phase)
	if sourceId == "" || phase == "" || record.UserId <= 0 || record.AmountQuota <= 0 {
		return billingEventExpectation{}, fmt.Sprintf("violation_fee_record:%d invalid violation fee data", record.Id)
	}
	eventType := strings.TrimSpace(record.EventType)
	if eventType == "" {
		eventType = model.BillingEventTypeDebit
	}
	if eventType != model.BillingEventTypeDebit {
		return billingEventExpectation{}, fmt.Sprintf("violation_fee_record:%d invalid event_type %s", record.Id, record.EventType)
	}
	if record.QuotaDelta != -record.AmountQuota {
		return billingEventExpectation{}, fmt.Sprintf("violation_fee_record:%d invalid quota_delta %d for debit %d", record.Id, record.QuotaDelta, record.AmountQuota)
	}
	billingSource := strings.TrimSpace(record.BillingSource)
	if billingSource == "" {
		billingSource = BillingSourceWallet
	}
	requestId := strings.TrimSpace(record.RequestId)
	if requestId == "" {
		requestId = sourceId
	}
	priceUnit := strings.TrimSpace(record.PriceUnit)
	if priceUnit == "" {
		priceUnit = "violation_fee"
	}
	return billingEventExpectation{
		Source:        model.BillingEventSourceViolationFee,
		SourceId:      sourceId,
		Phase:         phase,
		UserId:        record.UserId,
		TokenId:       record.TokenId,
		CheckTokenId:  record.TokenId > 0,
		EventType:     model.BillingEventTypeDebit,
		AmountQuota:   record.AmountQuota,
		QuotaDelta:    record.QuotaDelta,
		Status:        model.BillingEventStatusSettled,
		RequestId:     requestId,
		Group:         record.Group,
		BillingSource: billingSource,
		PriceUnit:     priceUnit,
		Currency:      "quota",
	}, ""
}

func violationFeeRecordBackfillInput(record model.ViolationFeeRecord, extraMetadata map[string]any) (model.FundingBillingEventInput, string) {
	expected, invalid := violationFeeRecordExpectation(record)
	if invalid != "" {
		return model.FundingBillingEventInput{}, invalid
	}
	metadata := violationFeeRecordMetadata(record, extraMetadata)
	return model.FundingBillingEventInput{
		Source:        model.BillingEventSourceViolationFee,
		SourceId:      expected.SourceId,
		Phase:         expected.Phase,
		UserId:        expected.UserId,
		TokenId:       expected.TokenId,
		RequestId:     expected.RequestId,
		Group:         expected.Group,
		BillingSource: expected.BillingSource,
		PriceUnit:     expected.PriceUnit,
		EventType:     expected.EventType,
		AmountQuota:   expected.AmountQuota,
		CreatedAt:     record.CreatedAt,
		Metadata:      metadata,
	}, ""
}

func violationFeeRecordMetadata(record model.ViolationFeeRecord, extraMetadata map[string]any) map[string]any {
	metadata := map[string]any{}
	if strings.TrimSpace(record.Metadata) != "" {
		if err := common.UnmarshalJsonStr(record.Metadata, &metadata); err != nil {
			metadata = map[string]any{
				"source_metadata":             record.Metadata,
				"source_metadata_parse_error": err.Error(),
			}
		}
	}
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadata["violation_fee_record_id"] = record.Id
	metadata["phase"] = record.Phase
	metadata["source"] = model.BillingEventSourceViolationFee
	for key, value := range extraMetadata {
		metadata[key] = value
	}
	return metadata
}

func taskBillingRecordMetadata(record model.TaskBillingRecord, extraMetadata map[string]any) map[string]any {
	metadata := map[string]any{}
	if strings.TrimSpace(record.Metadata) != "" {
		if err := common.UnmarshalJsonStr(record.Metadata, &metadata); err != nil {
			metadata = map[string]any{
				"source_metadata":             record.Metadata,
				"source_metadata_parse_error": err.Error(),
			}
		}
	}
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadata["task_billing_record_id"] = record.Id
	metadata["task_id"] = record.TaskId
	metadata["phase"] = record.Phase
	metadata["source"] = model.BillingEventSourceAsyncTask
	for key, value := range extraMetadata {
		metadata[key] = value
	}
	return metadata
}

func parseBillingEventPrefixedId(value string, prefix string) (int, error) {
	if !strings.HasPrefix(value, prefix) {
		return 0, fmt.Errorf("invalid source_id %s, expected prefix %s", value, prefix)
	}
	id, err := strconv.Atoi(strings.TrimPrefix(value, prefix))
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("invalid source_id %s", value)
	}
	return id, nil
}

func compareBillingEventExpectationToInput(expected billingEventExpectation, input model.FundingBillingEventInput) []dto.BillingEventReconciliationDiff {
	expected = normalizeBillingEventExpectation(expected)
	inputPhase := strings.TrimSpace(input.Phase)
	if inputPhase == "" {
		inputPhase = "settlement"
	}
	inputEventType := strings.TrimSpace(input.EventType)
	if inputEventType == "" {
		inputEventType = model.BillingEventTypeCredit
	}
	inputQuotaDelta := input.AmountQuota
	if inputEventType == model.BillingEventTypeDebit {
		inputQuotaDelta = -input.AmountQuota
	}
	inputCost := billingEventCost(input.AmountQuota)
	if input.Cost != nil {
		inputCost = *input.Cost
	}
	inputRequestId := strings.TrimSpace(input.RequestId)
	if inputRequestId == "" {
		inputRequestId = input.SourceId
	}
	inputPriceUnit := strings.TrimSpace(input.PriceUnit)
	if inputPriceUnit == "" {
		inputPriceUnit = expectedPriceUnitForExpectation(billingEventExpectation{
			Source: input.Source,
			Phase:  input.Phase,
		})
	}
	inputCurrency := "quota"

	diffs := []dto.BillingEventReconciliationDiff{}
	if strings.TrimSpace(input.Source) != strings.TrimSpace(expected.Source) {
		diffs = appendBillingEventReconciliationDiff(diffs, "source", expected.Source, input.Source)
	}
	if strings.TrimSpace(input.SourceId) != strings.TrimSpace(expected.SourceId) {
		diffs = appendBillingEventReconciliationDiff(diffs, "source_id", expected.SourceId, input.SourceId)
	}
	if strings.TrimSpace(expected.Phase) != "" && inputPhase != strings.TrimSpace(expected.Phase) {
		diffs = appendBillingEventReconciliationDiff(diffs, "phase", expected.Phase, inputPhase)
	}
	if input.UserId != expected.UserId {
		diffs = appendBillingEventReconciliationDiff(diffs, "user_id", expected.UserId, input.UserId)
	}
	if expected.CheckTokenId && input.TokenId != expected.TokenId {
		diffs = appendBillingEventReconciliationDiff(diffs, "token_id", expected.TokenId, input.TokenId)
	}
	if inputEventType != strings.TrimSpace(expected.EventType) {
		diffs = appendBillingEventReconciliationDiff(diffs, "event_type", expected.EventType, inputEventType)
	}
	if input.AmountQuota != expected.AmountQuota {
		diffs = appendBillingEventReconciliationDiff(diffs, "amount_quota", expected.AmountQuota, input.AmountQuota)
	}
	if inputQuotaDelta != expected.QuotaDelta {
		diffs = appendBillingEventReconciliationDiff(diffs, "quota_delta", expected.QuotaDelta, inputQuotaDelta)
	}
	if expected.CheckCost && inputCost != expected.Cost {
		diffs = appendBillingEventReconciliationDiff(diffs, "cost", expected.Cost, inputCost)
	}
	if strings.TrimSpace(expected.RequestId) != "" && inputRequestId != strings.TrimSpace(expected.RequestId) {
		diffs = appendBillingEventReconciliationDiff(diffs, "request_id", expected.RequestId, inputRequestId)
	}
	if strings.TrimSpace(expected.Group) != "" && strings.TrimSpace(input.Group) != strings.TrimSpace(expected.Group) {
		diffs = appendBillingEventReconciliationDiff(diffs, "group", expected.Group, input.Group)
	}
	if strings.TrimSpace(expected.BillingSource) != "" && strings.TrimSpace(input.BillingSource) != strings.TrimSpace(expected.BillingSource) {
		diffs = appendBillingEventReconciliationDiff(diffs, "billing_source", expected.BillingSource, input.BillingSource)
	}
	if strings.TrimSpace(expected.PriceUnit) != "" && inputPriceUnit != strings.TrimSpace(expected.PriceUnit) {
		diffs = appendBillingEventReconciliationDiff(diffs, "price_unit", expected.PriceUnit, inputPriceUnit)
	}
	if strings.TrimSpace(expected.Currency) != "" && inputCurrency != strings.TrimSpace(expected.Currency) {
		diffs = appendBillingEventReconciliationDiff(diffs, "currency", expected.Currency, inputCurrency)
	}
	return diffs
}

func expectedRequestId(expected billingEventExpectation) string {
	if strings.TrimSpace(expected.RequestId) != "" {
		return expected.RequestId
	}
	return expected.SourceId
}

func expectedPriceUnit(expected billingEventExpectation) string {
	return expectedPriceUnitForExpectation(expected)
}

func expectedPriceUnitForExpectation(expected billingEventExpectation) string {
	switch expected.Source {
	case model.BillingEventSourceWalletTopUp:
		if expected.Phase == "redemption" {
			return "redemption"
		}
		return "topup"
	case model.BillingEventSourceWalletAdjust:
		return "manual_adjust"
	case model.BillingEventSourceModelRequest:
		return "token_usage"
	case model.BillingEventSourceMCPToolCall:
		if expected.Phase == "refund" {
			return "mcp_refund"
		}
		return model.MCPToolPriceUnitPerCall
	case model.BillingEventSourceAsyncTask:
		switch expected.Phase {
		case taskBillingEventPhaseInitialSettlement:
			return "task"
		case taskBillingEventPhaseFailureRefund:
			return "task_refund"
		case taskBillingEventPhaseDeltaDebit, taskBillingEventPhaseDeltaCredit:
			return "task_recalculation"
		default:
			return "task"
		}
	case model.BillingEventSourceViolationFee:
		return "violation_fee"
	case model.BillingEventSourceSubscription:
		switch expected.Phase {
		case "balance_payment":
			return "subscription_balance_payment"
		default:
			return "subscription"
		}
	default:
		return "quota"
	}
}

func createBillingEventRepairAudit(tx *gorm.DB, adminId int, reason string, item dto.BillingEventReconciliationMismatchItem, before dto.BillingEventItem, after dto.BillingEventItem, diffs []dto.BillingEventReconciliationDiff) (model.BillingEvent, error) {
	repairSourceId := fmt.Sprintf("event:%d:%d", before.Id, time.Now().UnixNano())
	metadata := map[string]any{
		"admin_id":              adminId,
		"reason":                truncateBillingEventRepairReason(reason, 512),
		"reconciliation_source": item.Source,
		"label":                 item.Label,
		"target_event_pk":       before.Id,
		"target_event_id":       before.EventId,
		"expected":              item.Expected,
		"before":                before,
		"after":                 after,
		"diffs":                 diffs,
	}
	metadataBytes, err := common.Marshal(metadata)
	if err != nil {
		return model.BillingEvent{}, err
	}
	audit := model.BillingEvent{
		EventId:       billingEventID(model.BillingEventSourceLedgerRepair, repairSourceId, "repair"),
		UserId:        after.UserId,
		TokenId:       after.TokenId,
		Source:        model.BillingEventSourceLedgerRepair,
		SourceId:      truncateBillingEventString(repairSourceId, 128),
		EventType:     model.BillingEventTypeAudit,
		Status:        model.BillingEventStatusSettled,
		RequestId:     truncateBillingEventString(repairSourceId, 128),
		BillingSource: "ledger",
		PriceUnit:     "reconciliation_repair",
		Currency:      "quota",
		AmountQuota:   0,
		QuotaDelta:    0,
		Cost:          0,
		Metadata:      string(metadataBytes),
		CreatedAt:     common.GetTimestamp(),
	}
	if err := tx.Create(&audit).Error; err != nil {
		return model.BillingEvent{}, err
	}
	return audit, nil
}

func createBillingEventMissingBackfillAudit(tx *gorm.DB, adminId int, reason string, item dto.BillingEventReconciliationMissingItem, event dto.BillingEventItem) (model.BillingEvent, error) {
	repairSourceId := fmt.Sprintf("missing:%d:%d", event.Id, time.Now().UnixNano())
	metadata := map[string]any{
		"admin_id":              adminId,
		"reason":                truncateBillingEventRepairReason(reason, 512),
		"reconciliation_source": item.Source,
		"label":                 item.Label,
		"expected":              item.Expected,
		"created_event_pk":      event.Id,
		"created_event_id":      event.EventId,
		"created_event":         event,
	}
	metadataBytes, err := common.Marshal(metadata)
	if err != nil {
		return model.BillingEvent{}, err
	}
	audit := model.BillingEvent{
		EventId:       billingEventID(model.BillingEventSourceLedgerRepair, repairSourceId, "backfill_missing"),
		UserId:        event.UserId,
		TokenId:       event.TokenId,
		Source:        model.BillingEventSourceLedgerRepair,
		SourceId:      truncateBillingEventString(repairSourceId, 128),
		EventType:     model.BillingEventTypeAudit,
		Status:        model.BillingEventStatusSettled,
		RequestId:     truncateBillingEventString(repairSourceId, 128),
		BillingSource: "ledger",
		PriceUnit:     "reconciliation_backfill_missing",
		Currency:      "quota",
		AmountQuota:   0,
		QuotaDelta:    0,
		Cost:          0,
		Metadata:      string(metadataBytes),
		CreatedAt:     common.GetTimestamp(),
	}
	if err := tx.Create(&audit).Error; err != nil {
		return model.BillingEvent{}, err
	}
	return audit, nil
}

func billingEventRelationReason(reason string) string {
	return truncateBillingEventRepairReason(reason, 512)
}

func billingEventRelationLabel(label string) string {
	return truncateBillingEventString(label, 256)
}

func truncateBillingEventRepairReason(reason string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(reason)
	if len(runes) <= limit {
		return reason
	}
	return string(runes[:limit])
}

func reconcileMCPToolCalls(limit int, details *billingEventMismatchDetails) (dto.BillingEventReconciliationSourceResult, error) {
	result := newBillingEventReconciliationSourceResult(BillingEventBackfillSourceMCPToolCall)
	lastId := int64(0)
	scanLimit := backfillCandidateScanLimit(limit)
	stop := billingEventReconciliationScanStop{}
	sawMore := false
	for !stop.shouldStop() {
		calls, err := model.ListMCPToolCallsAfterId(lastId, scanLimit)
		if err != nil {
			return result, err
		}
		if len(calls) == 0 {
			break
		}
		for _, call := range calls {
			lastId = call.Id
			result.Scanned++
			expectations, invalid := mcpToolCallBillingExpectations(call)
			if invalid != "" {
				addBillingEventReconciliationInvalid(&result, invalid)
				stop.markIfLimitReached(&result, limit)
				if stop.shouldStop() {
					break
				}
				continue
			}
			for _, expected := range expectations {
				checkBillingEventPhase(&result, expected, details)
				stop.markIfLimitReached(&result, limit)
				if stop.shouldStop() {
					break
				}
			}
			if stop.shouldStop() {
				break
			}
		}
		if len(calls) < scanLimit {
			break
		}
	}
	if stop.shouldStop() {
		count, err := model.MCPToolCallsCountAfterId(lastId)
		if err != nil {
			return result, err
		}
		sawMore = count > 0
	}
	finishBillingEventReconciliationScan(&result, stop, sawMore)
	return result, nil
}

func reconcileWalletTopUps(limit int, details *billingEventMismatchDetails) (dto.BillingEventReconciliationSourceResult, error) {
	result := newBillingEventReconciliationSourceResult(BillingEventBackfillSourceWalletTopUp)
	lastId := 0
	scanLimit := backfillCandidateScanLimit(limit)
	stop := billingEventReconciliationScanStop{}
	sawMore := false
	for !stop.shouldStop() {
		var topUps []model.TopUp
		err := model.DB.Model(&model.TopUp{}).
			Joins("LEFT JOIN subscription_orders ON subscription_orders.trade_no = top_ups.trade_no").
			Where("top_ups.status = ? AND subscription_orders.id IS NULL", common.TopUpStatusSuccess).
			Where("top_ups.id > ?", lastId).
			Order("top_ups.id asc").
			Limit(scanLimit).
			Find(&topUps).Error
		if err != nil {
			return result, err
		}
		if len(topUps) == 0 {
			break
		}
		for _, topUp := range topUps {
			lastId = topUp.Id
			result.Scanned++
			tradeNo := strings.TrimSpace(topUp.TradeNo)
			if tradeNo == "" || topUp.UserId <= 0 {
				addBillingEventReconciliationInvalid(&result, fmt.Sprintf("topup:%d missing trade_no or user_id", topUp.Id))
				stop.markIfLimitReached(&result, limit)
				if stop.shouldStop() {
					break
				}
				continue
			}
			quota := calculateBackfillWalletTopUpQuota(topUp)
			if quota <= 0 {
				addBillingEventReconciliationInvalid(&result, fmt.Sprintf("topup:%s invalid quota", tradeNo))
				stop.markIfLimitReached(&result, limit)
				if stop.shouldStop() {
					break
				}
				continue
			}
			checkBillingEventPhase(&result, billingEventExpectation{
				Source:      model.BillingEventSourceWalletTopUp,
				SourceId:    tradeNo,
				Phase:       "success",
				UserId:      topUp.UserId,
				EventType:   model.BillingEventTypeCredit,
				AmountQuota: quota,
				QuotaDelta:  quota,
				Status:      model.BillingEventStatusSettled,
			}, details)
			stop.markIfLimitReached(&result, limit)
			if stop.shouldStop() {
				break
			}
		}
		if len(topUps) < scanLimit {
			break
		}
	}
	if stop.shouldStop() {
		var count int64
		if err := model.DB.Model(&model.TopUp{}).
			Joins("LEFT JOIN subscription_orders ON subscription_orders.trade_no = top_ups.trade_no").
			Where("top_ups.status = ? AND subscription_orders.id IS NULL", common.TopUpStatusSuccess).
			Where("top_ups.id > ?", lastId).
			Limit(1).
			Count(&count).Error; err != nil {
			return result, err
		}
		sawMore = count > 0
	}
	finishBillingEventReconciliationScan(&result, stop, sawMore)
	return result, nil
}

func reconcileRedemptions(limit int, details *billingEventMismatchDetails) (dto.BillingEventReconciliationSourceResult, error) {
	result := newBillingEventReconciliationSourceResult(BillingEventBackfillSourceRedemption)
	lastId := 0
	scanLimit := backfillCandidateScanLimit(limit)
	stop := billingEventReconciliationScanStop{}
	sawMore := false
	for !stop.shouldStop() {
		var redemptions []model.Redemption
		err := model.DB.Unscoped().
			Where("status = ? AND used_user_id > 0", common.RedemptionCodeStatusUsed).
			Where("id > ?", lastId).
			Order("id asc").
			Limit(scanLimit).
			Find(&redemptions).Error
		if err != nil {
			return result, err
		}
		if len(redemptions) == 0 {
			break
		}
		for _, redemption := range redemptions {
			lastId = redemption.Id
			result.Scanned++
			sourceId := fmt.Sprintf("redemption:%d", redemption.Id)
			if redemption.Id <= 0 || redemption.UsedUserId <= 0 || redemption.Quota <= 0 {
				addBillingEventReconciliationInvalid(&result, fmt.Sprintf("redemption:%d invalid redemption data", redemption.Id))
				stop.markIfLimitReached(&result, limit)
				if stop.shouldStop() {
					break
				}
				continue
			}
			checkBillingEventPhase(&result, billingEventExpectation{
				Source:      model.BillingEventSourceWalletTopUp,
				SourceId:    sourceId,
				Phase:       "redemption",
				UserId:      redemption.UsedUserId,
				EventType:   model.BillingEventTypeCredit,
				AmountQuota: redemption.Quota,
				QuotaDelta:  redemption.Quota,
				Status:      model.BillingEventStatusSettled,
			}, details)
			stop.markIfLimitReached(&result, limit)
			if stop.shouldStop() {
				break
			}
		}
		if len(redemptions) < scanLimit {
			break
		}
	}
	if stop.shouldStop() {
		var count int64
		if err := model.DB.Unscoped().
			Model(&model.Redemption{}).
			Where("status = ? AND used_user_id > 0", common.RedemptionCodeStatusUsed).
			Where("id > ?", lastId).
			Limit(1).
			Count(&count).Error; err != nil {
			return result, err
		}
		sawMore = count > 0
	}
	finishBillingEventReconciliationScan(&result, stop, sawMore)
	return result, nil
}

func reconcileWalletAdjustments(limit int, details *billingEventMismatchDetails) (dto.BillingEventReconciliationSourceResult, error) {
	result := newBillingEventReconciliationSourceResult(BillingEventBackfillSourceWalletAdjust)
	lastId := int64(0)
	scanLimit := backfillCandidateScanLimit(limit)
	stop := billingEventReconciliationScanStop{}
	sawMore := false
	for !stop.shouldStop() {
		adjustments, err := model.ListWalletAdjustmentsAfterId(lastId, scanLimit)
		if err != nil {
			return result, err
		}
		if len(adjustments) == 0 {
			break
		}
		for _, adjustment := range adjustments {
			lastId = adjustment.Id
			result.Scanned++
			expected, invalid := walletAdjustmentBillingExpectation(adjustment)
			if invalid != "" {
				addBillingEventReconciliationInvalid(&result, invalid)
				stop.markIfLimitReached(&result, limit)
				if stop.shouldStop() {
					break
				}
				continue
			}
			checkBillingEventPhase(&result, expected, details)
			stop.markIfLimitReached(&result, limit)
			if stop.shouldStop() {
				break
			}
		}
		if len(adjustments) < scanLimit {
			break
		}
	}
	if stop.shouldStop() {
		count, err := model.WalletAdjustmentsCountAfterId(lastId)
		if err != nil {
			return result, err
		}
		sawMore = count > 0
	}
	finishBillingEventReconciliationScan(&result, stop, sawMore)
	return result, nil
}

func reconcileModelRequests(limit int, details *billingEventMismatchDetails) (dto.BillingEventReconciliationSourceResult, error) {
	result := newBillingEventReconciliationSourceResult(BillingEventBackfillSourceModelRequest)
	if model.LOG_DB == nil {
		return result, fmt.Errorf("log database is not initialized")
	}
	lastId := 0
	scanLimit := backfillCandidateScanLimit(limit)
	stop := billingEventReconciliationScanStop{}
	sawMore := false
	for !stop.shouldStop() {
		var logs []model.Log
		err := model.LOG_DB.Model(&model.Log{}).
			Where("type = ? AND quota > 0 AND request_id <> ?", model.LogTypeConsume, "").
			Where("id > ?", lastId).
			Order("id asc").
			Limit(scanLimit).
			Find(&logs).Error
		if err != nil {
			return result, err
		}
		if len(logs) == 0 {
			break
		}
		for _, log := range logs {
			lastId = log.Id
			result.Scanned++
			expected, _, invalid := modelRequestBillingExpectationFromLog(log)
			if invalid != "" {
				addBillingEventReconciliationInvalid(&result, invalid)
				stop.markIfLimitReached(&result, limit)
				if stop.shouldStop() {
					break
				}
				continue
			}
			checkBillingEventPhase(&result, expected, details)
			stop.markIfLimitReached(&result, limit)
			if stop.shouldStop() {
				break
			}
		}
		if len(logs) < scanLimit {
			break
		}
	}
	if stop.shouldStop() {
		var count int64
		if err := model.LOG_DB.Model(&model.Log{}).
			Where("type = ? AND quota > 0 AND request_id <> ?", model.LogTypeConsume, "").
			Where("id > ?", lastId).
			Limit(1).
			Count(&count).Error; err != nil {
			return result, err
		}
		sawMore = count > 0
	}
	finishBillingEventReconciliationScan(&result, stop, sawMore)
	return result, nil
}

func reconcileTaskBillingRecords(limit int, details *billingEventMismatchDetails) (dto.BillingEventReconciliationSourceResult, error) {
	result := newBillingEventReconciliationSourceResult(BillingEventBackfillSourceAsyncTask)
	lastId := int64(0)
	scanLimit := backfillCandidateScanLimit(limit)
	stop := billingEventReconciliationScanStop{}
	sawMore := false
	for !stop.shouldStop() {
		records, err := model.ListTaskBillingRecordsAfterId(lastId, scanLimit)
		if err != nil {
			return result, err
		}
		if len(records) == 0 {
			break
		}
		for _, record := range records {
			lastId = record.Id
			result.Scanned++
			expected, invalid := taskBillingRecordExpectation(record)
			if invalid != "" {
				addBillingEventReconciliationInvalid(&result, invalid)
				stop.markIfLimitReached(&result, limit)
				if stop.shouldStop() {
					break
				}
				continue
			}
			checkBillingEventPhase(&result, expected, details)
			stop.markIfLimitReached(&result, limit)
			if stop.shouldStop() {
				break
			}
		}
		if len(records) < scanLimit {
			break
		}
	}
	if stop.shouldStop() {
		count, err := model.TaskBillingRecordsCountAfterId(lastId)
		if err != nil {
			return result, err
		}
		sawMore = count > 0
	}
	finishBillingEventReconciliationScan(&result, stop, sawMore)
	return result, nil
}

func reconcileViolationFeeRecords(limit int, details *billingEventMismatchDetails) (dto.BillingEventReconciliationSourceResult, error) {
	result := newBillingEventReconciliationSourceResult(BillingEventBackfillSourceViolationFee)
	lastId := int64(0)
	scanLimit := backfillCandidateScanLimit(limit)
	stop := billingEventReconciliationScanStop{}
	sawMore := false
	for !stop.shouldStop() {
		records, err := model.ListViolationFeeRecordsAfterId(lastId, scanLimit)
		if err != nil {
			return result, err
		}
		if len(records) == 0 {
			break
		}
		for _, record := range records {
			lastId = record.Id
			result.Scanned++
			expected, invalid := violationFeeRecordExpectation(record)
			if invalid != "" {
				addBillingEventReconciliationInvalid(&result, invalid)
				stop.markIfLimitReached(&result, limit)
				if stop.shouldStop() {
					break
				}
				continue
			}
			checkBillingEventPhase(&result, expected, details)
			stop.markIfLimitReached(&result, limit)
			if stop.shouldStop() {
				break
			}
		}
		if len(records) < scanLimit {
			break
		}
	}
	if stop.shouldStop() {
		count, err := model.ViolationFeeRecordsCountAfterId(lastId)
		if err != nil {
			return result, err
		}
		sawMore = count > 0
	}
	finishBillingEventReconciliationScan(&result, stop, sawMore)
	return result, nil
}

func reconcileSubscriptionPurchases(limit int, details *billingEventMismatchDetails) (dto.BillingEventReconciliationSourceResult, error) {
	result := newBillingEventReconciliationSourceResult(BillingEventBackfillSourceSubscriptionPurchase)
	lastId := 0
	scanLimit := backfillCandidateScanLimit(limit)
	stop := billingEventReconciliationScanStop{}
	sawMore := false
	for !stop.shouldStop() {
		var orders []model.SubscriptionOrder
		err := model.DB.Where("status = ? AND payment_provider <> ? AND payment_method <> ?",
			common.TopUpStatusSuccess, model.PaymentProviderBalance, model.PaymentMethodBalance).
			Where("id > ?", lastId).
			Order("id asc").
			Limit(scanLimit).
			Find(&orders).Error
		if err != nil {
			return result, err
		}
		if len(orders) == 0 {
			break
		}
		for _, order := range orders {
			lastId = order.Id
			result.Scanned++
			tradeNo := strings.TrimSpace(order.TradeNo)
			if tradeNo == "" || order.UserId <= 0 || order.PlanId <= 0 {
				addBillingEventReconciliationInvalid(&result, fmt.Sprintf("subscription_order:%d invalid purchase data", order.Id))
				stop.markIfLimitReached(&result, limit)
				if stop.shouldStop() {
					break
				}
				continue
			}
			plan, err := model.GetSubscriptionPlanById(order.PlanId)
			if err != nil {
				addBillingEventReconciliationInvalid(&result, fmt.Sprintf("subscription_order:%s missing plan %d: %v", tradeNo, order.PlanId, err))
				stop.markIfLimitReached(&result, limit)
				if stop.shouldStop() {
					break
				}
				continue
			}
			subscription := findBackfillSubscription(order, "order")
			amountQuota := backfillSubscriptionGrantQuota(subscription, plan)
			checkBillingEventPhase(&result, billingEventExpectation{
				Source:      model.BillingEventSourceSubscription,
				SourceId:    tradeNo,
				Phase:       "purchase",
				UserId:      order.UserId,
				EventType:   model.BillingEventTypeCredit,
				AmountQuota: amountQuota,
				QuotaDelta:  amountQuota,
				Status:      model.BillingEventStatusSettled,
			}, details)
			stop.markIfLimitReached(&result, limit)
			if stop.shouldStop() {
				break
			}
		}
		if len(orders) < scanLimit {
			break
		}
	}
	if stop.shouldStop() {
		var count int64
		if err := model.DB.Model(&model.SubscriptionOrder{}).
			Where("status = ? AND payment_provider <> ? AND payment_method <> ?",
				common.TopUpStatusSuccess, model.PaymentProviderBalance, model.PaymentMethodBalance).
			Where("id > ?", lastId).
			Limit(1).
			Count(&count).Error; err != nil {
			return result, err
		}
		sawMore = count > 0
	}
	finishBillingEventReconciliationScan(&result, stop, sawMore)
	return result, nil
}

func reconcileSubscriptionBalanceOrders(limit int, details *billingEventMismatchDetails) (dto.BillingEventReconciliationSourceResult, error) {
	result := newBillingEventReconciliationSourceResult(BillingEventBackfillSourceSubscriptionBalance)
	lastId := 0
	scanLimit := backfillCandidateScanLimit(limit)
	stop := billingEventReconciliationScanStop{}
	sawMore := false
	for !stop.shouldStop() {
		var orders []model.SubscriptionOrder
		err := model.DB.Where("status = ? AND (payment_provider = ? OR payment_method = ?)",
			common.TopUpStatusSuccess, model.PaymentProviderBalance, model.PaymentMethodBalance).
			Where("id > ?", lastId).
			Order("id asc").
			Limit(scanLimit).
			Find(&orders).Error
		if err != nil {
			return result, err
		}
		if len(orders) == 0 {
			break
		}
		for _, order := range orders {
			lastId = order.Id
			result.Scanned++
			tradeNo := strings.TrimSpace(order.TradeNo)
			if tradeNo == "" || order.UserId <= 0 || order.PlanId <= 0 {
				addBillingEventReconciliationInvalid(&result, fmt.Sprintf("subscription_order:%d invalid balance data", order.Id))
				stop.markIfLimitReached(&result, limit)
				if stop.shouldStop() {
					break
				}
				continue
			}
			plan, err := model.GetSubscriptionPlanById(order.PlanId)
			if err != nil {
				addBillingEventReconciliationInvalid(&result, fmt.Sprintf("subscription_order:%s missing plan %d: %v", tradeNo, order.PlanId, err))
				stop.markIfLimitReached(&result, limit)
				if stop.shouldStop() {
					break
				}
				continue
			}
			requiredQuota, err := backfillSubscriptionBalanceQuota(order, plan)
			if err != nil {
				addBillingEventReconciliationInvalid(&result, fmt.Sprintf("subscription_order:%s invalid balance quota: %v", tradeNo, err))
				stop.markIfLimitReached(&result, limit)
				if stop.shouldStop() {
					break
				}
				continue
			}
			if requiredQuota > 0 {
				checkBillingEventPhase(&result, billingEventExpectation{
					Source:      model.BillingEventSourceSubscription,
					SourceId:    tradeNo,
					Phase:       "balance_payment",
					UserId:      order.UserId,
					EventType:   model.BillingEventTypeDebit,
					AmountQuota: requiredQuota,
					QuotaDelta:  -requiredQuota,
					Status:      model.BillingEventStatusSettled,
				}, details)
				stop.markIfLimitReached(&result, limit)
				if stop.shouldStop() {
					break
				}
			}
			subscription := findBackfillSubscription(order, model.PaymentMethodBalance)
			grantQuota := backfillSubscriptionGrantQuota(subscription, plan)
			checkBillingEventPhase(&result, billingEventExpectation{
				Source:      model.BillingEventSourceSubscription,
				SourceId:    tradeNo,
				Phase:       "grant",
				UserId:      order.UserId,
				EventType:   model.BillingEventTypeCredit,
				AmountQuota: grantQuota,
				QuotaDelta:  grantQuota,
				Status:      model.BillingEventStatusSettled,
			}, details)
			stop.markIfLimitReached(&result, limit)
			if stop.shouldStop() {
				break
			}
		}
		if len(orders) < scanLimit {
			break
		}
	}
	if stop.shouldStop() {
		var count int64
		if err := model.DB.Model(&model.SubscriptionOrder{}).
			Where("status = ? AND (payment_provider = ? OR payment_method = ?)",
				common.TopUpStatusSuccess, model.PaymentProviderBalance, model.PaymentMethodBalance).
			Where("id > ?", lastId).
			Limit(1).
			Count(&count).Error; err != nil {
			return result, err
		}
		sawMore = count > 0
	}
	finishBillingEventReconciliationScan(&result, stop, sawMore)
	return result, nil
}

func reconcileAdminSubscriptions(limit int, details *billingEventMismatchDetails) (dto.BillingEventReconciliationSourceResult, error) {
	result := newBillingEventReconciliationSourceResult(BillingEventBackfillSourceSubscriptionAdmin)
	lastId := 0
	scanLimit := backfillCandidateScanLimit(limit)
	stop := billingEventReconciliationScanStop{}
	sawMore := false
	for !stop.shouldStop() {
		var subscriptions []model.UserSubscription
		err := model.DB.Where("source = ?", "admin").
			Where("id > ?", lastId).
			Order("id asc").
			Limit(scanLimit).
			Find(&subscriptions).Error
		if err != nil {
			return result, err
		}
		if len(subscriptions) == 0 {
			break
		}
		for _, sub := range subscriptions {
			lastId = sub.Id
			result.Scanned++
			sourceId := fmt.Sprintf("admin_bind:%d", sub.Id)
			if sub.Id <= 0 || sub.UserId <= 0 || sub.PlanId <= 0 {
				addBillingEventReconciliationInvalid(&result, fmt.Sprintf("user_subscription:%d invalid admin subscription data", sub.Id))
				stop.markIfLimitReached(&result, limit)
				if stop.shouldStop() {
					break
				}
				continue
			}
			plan, err := model.GetSubscriptionPlanById(sub.PlanId)
			if err != nil {
				addBillingEventReconciliationInvalid(&result, fmt.Sprintf("user_subscription:%d missing plan %d: %v", sub.Id, sub.PlanId, err))
				stop.markIfLimitReached(&result, limit)
				if stop.shouldStop() {
					break
				}
				continue
			}
			amountQuota := backfillSubscriptionGrantQuota(backfillSubscriptionSnapshot{
				Id:          sub.Id,
				Source:      sub.Source,
				AmountTotal: sub.AmountTotal,
				QuotaSource: "user_subscription",
			}, plan)
			checkBillingEventPhase(&result, billingEventExpectation{
				Source:      model.BillingEventSourceSubscription,
				SourceId:    sourceId,
				Phase:       "admin_bind",
				UserId:      sub.UserId,
				EventType:   model.BillingEventTypeCredit,
				AmountQuota: amountQuota,
				QuotaDelta:  amountQuota,
				Status:      model.BillingEventStatusSettled,
			}, details)
			stop.markIfLimitReached(&result, limit)
			if stop.shouldStop() {
				break
			}
		}
		if len(subscriptions) < scanLimit {
			break
		}
	}
	if stop.shouldStop() {
		var count int64
		if err := model.DB.Model(&model.UserSubscription{}).
			Where("source = ?", "admin").
			Where("id > ?", lastId).
			Limit(1).
			Count(&count).Error; err != nil {
			return result, err
		}
		sawMore = count > 0
	}
	finishBillingEventReconciliationScan(&result, stop, sawMore)
	return result, nil
}

func newBillingEventReconciliationSourceResult(source string) dto.BillingEventReconciliationSourceResult {
	return dto.BillingEventReconciliationSourceResult{
		Source:           source,
		SampleMissing:    []string{},
		SampleMismatched: []string{},
		SampleInvalid:    []string{},
		Errors:           []string{},
	}
}

type billingEventExpectation struct {
	Source        string
	SourceId      string
	Phase         string
	UserId        int
	TokenId       int
	CheckTokenId  bool
	EventType     string
	AmountQuota   int
	QuotaDelta    int
	Status        string
	RequestId     string
	Group         string
	BillingSource string
	PriceUnit     string
	Currency      string
	Cost          float64
	CheckCost     bool
}

type billingEventMismatchDetails struct {
	ReconciliationSource string
	DetailLimit          int
	TargetLabel          string
	Items                *[]dto.BillingEventReconciliationMismatchItem
	MissingItems         *[]dto.BillingEventReconciliationMissingItem
}

func checkBillingEventSource(result *dto.BillingEventReconciliationSourceResult, expected billingEventExpectation, details *billingEventMismatchDetails) {
	result.Expected++
	events, err := model.ListFundingBillingEventsBySource(nil, expected.Source, expected.SourceId, maxBillingEventReconciliationSamples+1)
	if err != nil {
		result.ErrorCount++
		addBillingEventReconciliationError(result, fmt.Sprintf("%s source check failed: %v", billingEventExpectationLabel(expected), err))
		return
	}
	if len(events) == 0 {
		addBillingEventReconciliationMissingDetail(result, expected, details)
		return
	}
	var firstDiffs []dto.BillingEventReconciliationDiff
	var firstEvent model.BillingEvent
	for _, event := range events {
		diffs := compareBillingEventExpectation(expected, event)
		if len(diffs) == 0 {
			result.Ledgered++
			return
		}
		if firstDiffs == nil {
			firstDiffs = diffs
			firstEvent = event
		}
	}
	addBillingEventReconciliationMismatch(result, expected, &firstEvent, firstDiffs, details)
}

func checkBillingEventPhase(result *dto.BillingEventReconciliationSourceResult, expected billingEventExpectation, details *billingEventMismatchDetails) {
	result.Expected++
	event, exists, err := model.GetFundingBillingEvent(nil, expected.Source, expected.SourceId, expected.Phase)
	if err != nil {
		result.ErrorCount++
		addBillingEventReconciliationError(result, fmt.Sprintf("%s exists check failed: %v", billingEventExpectationLabel(expected), err))
		return
	}
	if !exists {
		addBillingEventReconciliationMissingDetail(result, expected, details)
		return
	}
	diffs := compareBillingEventExpectation(expected, event)
	if len(diffs) == 0 {
		result.Ledgered++
		return
	}
	addBillingEventReconciliationMismatch(result, expected, &event, diffs, details)
}

func compareBillingEventExpectation(expected billingEventExpectation, event model.BillingEvent) []dto.BillingEventReconciliationDiff {
	expected = normalizeBillingEventExpectation(expected)
	expectedStatus := strings.TrimSpace(expected.Status)
	if expectedStatus == "" {
		expectedStatus = model.BillingEventStatusSettled
	}
	expectedEventType := strings.TrimSpace(expected.EventType)
	if expectedEventType == "" {
		expectedEventType = model.BillingEventTypeCredit
	}
	expectedCurrency := strings.TrimSpace(expected.Currency)
	if expectedCurrency == "" {
		expectedCurrency = "quota"
	}
	diffs := []dto.BillingEventReconciliationDiff{}
	if event.UserId != expected.UserId {
		diffs = appendBillingEventReconciliationDiff(diffs, "user_id", expected.UserId, event.UserId)
	}
	if expected.CheckTokenId && event.TokenId != expected.TokenId {
		diffs = appendBillingEventReconciliationDiff(diffs, "token_id", expected.TokenId, event.TokenId)
	}
	if strings.TrimSpace(event.Source) != strings.TrimSpace(expected.Source) {
		diffs = appendBillingEventReconciliationDiff(diffs, "source", expected.Source, event.Source)
	}
	if strings.TrimSpace(event.SourceId) != truncateBillingEventString(expected.SourceId, 128) {
		diffs = appendBillingEventReconciliationDiff(diffs, "source_id", truncateBillingEventString(expected.SourceId, 128), event.SourceId)
	}
	if strings.TrimSpace(event.EventType) != expectedEventType {
		diffs = appendBillingEventReconciliationDiff(diffs, "event_type", expectedEventType, event.EventType)
	}
	if strings.TrimSpace(event.Status) != expectedStatus {
		diffs = appendBillingEventReconciliationDiff(diffs, "status", expectedStatus, event.Status)
	}
	if event.AmountQuota != expected.AmountQuota {
		diffs = appendBillingEventReconciliationDiff(diffs, "amount_quota", expected.AmountQuota, event.AmountQuota)
	}
	if event.QuotaDelta != expected.QuotaDelta {
		diffs = appendBillingEventReconciliationDiff(diffs, "quota_delta", expected.QuotaDelta, event.QuotaDelta)
	}
	if expected.CheckCost && event.Cost != expected.Cost {
		diffs = appendBillingEventReconciliationDiff(diffs, "cost", expected.Cost, event.Cost)
	}
	if strings.TrimSpace(expected.RequestId) != "" && strings.TrimSpace(event.RequestId) != truncateBillingEventString(expected.RequestId, 128) {
		diffs = appendBillingEventReconciliationDiff(diffs, "request_id", truncateBillingEventString(expected.RequestId, 128), event.RequestId)
	}
	if strings.TrimSpace(expected.Group) != "" && strings.TrimSpace(event.Group) != strings.TrimSpace(expected.Group) {
		diffs = appendBillingEventReconciliationDiff(diffs, "group", expected.Group, event.Group)
	}
	if strings.TrimSpace(expected.BillingSource) != "" && strings.TrimSpace(event.BillingSource) != strings.TrimSpace(expected.BillingSource) {
		diffs = appendBillingEventReconciliationDiff(diffs, "billing_source", expected.BillingSource, event.BillingSource)
	}
	if strings.TrimSpace(expected.PriceUnit) != "" && strings.TrimSpace(event.PriceUnit) != strings.TrimSpace(expected.PriceUnit) {
		diffs = appendBillingEventReconciliationDiff(diffs, "price_unit", expected.PriceUnit, event.PriceUnit)
	}
	if strings.TrimSpace(event.Currency) != expectedCurrency {
		diffs = appendBillingEventReconciliationDiff(diffs, "currency", expectedCurrency, event.Currency)
	}
	return diffs
}

func appendBillingEventReconciliationDiff(diffs []dto.BillingEventReconciliationDiff, field string, expected any, actual any) []dto.BillingEventReconciliationDiff {
	return append(diffs, dto.BillingEventReconciliationDiff{
		Field:    field,
		Expected: fmt.Sprint(expected),
		Actual:   fmt.Sprint(actual),
	})
}

func billingEventExpectationLabel(expected billingEventExpectation) string {
	if strings.TrimSpace(expected.Phase) == "" {
		return fmt.Sprintf("%s:%s", expected.Source, expected.SourceId)
	}
	return fmt.Sprintf("%s:%s:%s", expected.Source, expected.SourceId, expected.Phase)
}

func billingEventExpectedEventToDTO(expected billingEventExpectation) dto.BillingEventReconciliationExpectedEvent {
	expected = normalizeBillingEventExpectation(expected)
	expectedStatus := strings.TrimSpace(expected.Status)
	if expectedStatus == "" {
		expectedStatus = model.BillingEventStatusSettled
	}
	expectedEventType := strings.TrimSpace(expected.EventType)
	if expectedEventType == "" {
		expectedEventType = model.BillingEventTypeCredit
	}
	expectedPriceUnit := strings.TrimSpace(expected.PriceUnit)
	if expectedPriceUnit == "" {
		expectedPriceUnit = expectedPriceUnitForExpectation(expected)
	}
	expectedCurrency := strings.TrimSpace(expected.Currency)
	if expectedCurrency == "" {
		expectedCurrency = "quota"
	}
	return dto.BillingEventReconciliationExpectedEvent{
		Label:         billingEventExpectationLabel(expected),
		Source:        expected.Source,
		SourceId:      expected.SourceId,
		Phase:         expected.Phase,
		UserId:        expected.UserId,
		TokenId:       expected.TokenId,
		EventType:     expectedEventType,
		Status:        expectedStatus,
		AmountQuota:   expected.AmountQuota,
		QuotaDelta:    expected.QuotaDelta,
		RequestId:     expected.RequestId,
		Group:         expected.Group,
		BillingSource: expected.BillingSource,
		PriceUnit:     expectedPriceUnit,
		Currency:      expectedCurrency,
	}
}

func normalizeBillingEventExpectation(expected billingEventExpectation) billingEventExpectation {
	if strings.TrimSpace(expected.PriceUnit) == "" {
		expected.PriceUnit = expectedPriceUnitForExpectation(expected)
	}
	if strings.TrimSpace(expected.Currency) == "" {
		expected.Currency = "quota"
	}
	return expected
}

func formatBillingEventMismatch(expected billingEventExpectation, diffs []dto.BillingEventReconciliationDiff) string {
	parts := make([]string, 0, len(diffs))
	for _, diff := range diffs {
		parts = append(parts, fmt.Sprintf("%s expected %s got %s", diff.Field, diff.Expected, diff.Actual))
	}
	return fmt.Sprintf("%s mismatch: %s", billingEventExpectationLabel(expected), strings.Join(parts, "; "))
}

func addBillingEventReconciliationInvalid(result *dto.BillingEventReconciliationSourceResult, message string) {
	result.Invalid++
	if len(result.SampleInvalid) < maxBillingEventReconciliationSamples {
		result.SampleInvalid = append(result.SampleInvalid, message)
	}
}

func addBillingEventReconciliationMismatch(result *dto.BillingEventReconciliationSourceResult, expected billingEventExpectation, actual *model.BillingEvent, diffs []dto.BillingEventReconciliationDiff, details *billingEventMismatchDetails) {
	result.Mismatched++
	message := formatBillingEventMismatch(expected, diffs)
	if len(result.SampleMismatched) < maxBillingEventReconciliationSamples {
		result.SampleMismatched = append(result.SampleMismatched, message)
	}
	if details == nil || details.Items == nil || len(*details.Items) >= details.DetailLimit {
		return
	}
	label := billingEventExpectationLabel(expected)
	if details.TargetLabel != "" && label != details.TargetLabel {
		return
	}
	item := dto.BillingEventReconciliationMismatchItem{
		Source:   details.ReconciliationSource,
		Label:    label,
		Expected: billingEventExpectedEventToDTO(expected),
		Diffs:    diffs,
	}
	if actual != nil {
		actualDTO := billingEventToDTO(*actual)
		item.Actual = &actualDTO
	}
	*details.Items = append(*details.Items, item)
}

func addBillingEventReconciliationMissing(result *dto.BillingEventReconciliationSourceResult, message string) {
	if len(result.SampleMissing) < maxBillingEventReconciliationSamples {
		result.SampleMissing = append(result.SampleMissing, message)
	}
}

func addBillingEventReconciliationMissingDetail(result *dto.BillingEventReconciliationSourceResult, expected billingEventExpectation, details *billingEventMismatchDetails) {
	result.Missing++
	label := billingEventExpectationLabel(expected)
	addBillingEventReconciliationMissing(result, label)
	if details == nil || details.MissingItems == nil || len(*details.MissingItems) >= details.DetailLimit {
		return
	}
	if details.TargetLabel != "" && label != details.TargetLabel {
		return
	}
	*details.MissingItems = append(*details.MissingItems, dto.BillingEventReconciliationMissingItem{
		Source:   details.ReconciliationSource,
		Label:    label,
		Expected: billingEventExpectedEventToDTO(expected),
	})
}

func addBillingEventReconciliationError(result *dto.BillingEventReconciliationSourceResult, message string) {
	if len(result.Errors) < maxBillingEventReconciliationSamples {
		result.Errors = append(result.Errors, message)
	}
}
