package service

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"

	"github.com/shopspring/decimal"
)

const (
	BillingEventBackfillSourceWalletTopUp          = "wallet_topup"
	BillingEventBackfillSourceMCPToolCall          = "mcp_tool_call"
	BillingEventBackfillSourceWalletAdjust         = "wallet_adjust"
	BillingEventBackfillSourceRedemption           = "redemption"
	BillingEventBackfillSourceModelRequest         = "model_request"
	BillingEventBackfillSourceAsyncTask            = "async_task"
	BillingEventBackfillSourceViolationFee         = "violation_fee"
	BillingEventBackfillSourceSubscription         = "subscription"
	BillingEventBackfillSourceSubscriptionPurchase = "subscription_purchase"
	BillingEventBackfillSourceSubscriptionBalance  = "subscription_balance"
	BillingEventBackfillSourceSubscriptionAdmin    = "subscription_admin_bind"

	defaultBillingEventBackfillLimit = 500
	maxBillingEventBackfillLimit     = 5000
	maxBillingEventBackfillErrors    = 20
)

type BillingEventBackfillParams struct {
	Sources []string
	Limit   int
	DryRun  bool
}

func BackfillBillingEvents(params BillingEventBackfillParams) (dto.BillingEventBackfillResponse, error) {
	limit := normalizeBillingEventBackfillLimit(params.Limit)
	sources, err := normalizeBillingEventBackfillSources(params.Sources)
	if err != nil {
		return dto.BillingEventBackfillResponse{}, err
	}
	response := dto.BillingEventBackfillResponse{
		DryRun:  params.DryRun,
		Limit:   limit,
		Sources: sources,
		Results: make([]dto.BillingEventBackfillSourceResult, 0, len(sources)),
	}

	for _, source := range sources {
		result, err := backfillBillingEventSource(source, limit, params.DryRun)
		if err != nil {
			return response, err
		}
		response.Results = append(response.Results, result)
		response.TotalScanned += result.Scanned
		response.TotalCreated += result.Created
		response.TotalWouldCreate += result.WouldCreate
		response.TotalSkippedExisting += result.SkippedExisting
		response.TotalSkippedInvalid += result.SkippedInvalid
		response.TotalErrorCount += result.ErrorCount
	}

	return response, nil
}

func normalizeBillingEventBackfillLimit(limit int) int {
	if limit <= 0 {
		return defaultBillingEventBackfillLimit
	}
	if limit > maxBillingEventBackfillLimit {
		return maxBillingEventBackfillLimit
	}
	return limit
}

func normalizeBillingEventBackfillSources(sources []string) ([]string, error) {
	if len(sources) == 0 {
		return append([]string{}, defaultBillingEventBackfillSources...), nil
	}
	seen := make(map[string]bool, len(sources))
	normalized := make([]string, 0, len(sources))
	add := func(source string) {
		if !seen[source] {
			seen[source] = true
			normalized = append(normalized, source)
		}
	}
	for _, source := range sources {
		source = strings.ToLower(strings.TrimSpace(source))
		switch source {
		case "", "all":
			for _, defaultSource := range defaultBillingEventBackfillSources {
				add(defaultSource)
			}
		case BillingEventBackfillSourceSubscription:
			add(BillingEventBackfillSourceSubscriptionPurchase)
			add(BillingEventBackfillSourceSubscriptionBalance)
			add(BillingEventBackfillSourceSubscriptionAdmin)
		default:
			if _, ok := getBillingEventSourceHandler(source); !ok {
				return nil, fmt.Errorf("unsupported billing event backfill source: %s", source)
			}
			add(source)
		}
	}
	return normalized, nil
}

func backfillCandidateScanLimit(limit int) int {
	if limit < 100 {
		return 100
	}
	if limit > 1000 {
		return 1000
	}
	return limit
}

func listBackfillWalletTopUpCandidates(limit int) ([]model.TopUp, error) {
	candidates := make([]model.TopUp, 0, limit)
	lastId := 0
	scanLimit := backfillCandidateScanLimit(limit)
	for len(candidates) < limit {
		var batch []model.TopUp
		err := model.DB.Model(&model.TopUp{}).
			Joins("LEFT JOIN subscription_orders ON subscription_orders.trade_no = top_ups.trade_no").
			Where("top_ups.status = ? AND subscription_orders.id IS NULL", common.TopUpStatusSuccess).
			Where("top_ups.id > ?", lastId).
			Order("top_ups.id asc").
			Limit(scanLimit).
			Find(&batch).Error
		if err != nil {
			return nil, err
		}
		if len(batch) == 0 {
			break
		}
		for _, topUp := range batch {
			lastId = topUp.Id
			tradeNo := strings.TrimSpace(topUp.TradeNo)
			if tradeNo == "" || topUp.UserId <= 0 {
				candidates = append(candidates, topUp)
			} else if exists, err := model.FundingBillingEventSourceExists(nil, model.BillingEventSourceWalletTopUp, tradeNo); err != nil || !exists {
				candidates = append(candidates, topUp)
			}
			if len(candidates) >= limit {
				break
			}
		}
	}
	return candidates, nil
}

func listBackfillRedemptionCandidates(limit int) ([]model.Redemption, error) {
	candidates := make([]model.Redemption, 0, limit)
	lastId := 0
	scanLimit := backfillCandidateScanLimit(limit)
	for len(candidates) < limit {
		var batch []model.Redemption
		err := model.DB.Unscoped().
			Where("status = ? AND used_user_id > 0", common.RedemptionCodeStatusUsed).
			Where("id > ?", lastId).
			Order("id asc").
			Limit(scanLimit).
			Find(&batch).Error
		if err != nil {
			return nil, err
		}
		if len(batch) == 0 {
			break
		}
		for _, redemption := range batch {
			lastId = redemption.Id
			sourceId := fmt.Sprintf("redemption:%d", redemption.Id)
			if redemption.Id <= 0 || redemption.UsedUserId <= 0 || redemption.Quota <= 0 {
				candidates = append(candidates, redemption)
			} else if exists, err := model.FundingBillingEventExists(nil, model.BillingEventSourceWalletTopUp, sourceId, "redemption"); err != nil || !exists {
				candidates = append(candidates, redemption)
			}
			if len(candidates) >= limit {
				break
			}
		}
	}
	return candidates, nil
}

func listBackfillSubscriptionPurchaseCandidates(limit int) ([]model.SubscriptionOrder, error) {
	candidates := make([]model.SubscriptionOrder, 0, limit)
	lastId := 0
	scanLimit := backfillCandidateScanLimit(limit)
	for len(candidates) < limit {
		var batch []model.SubscriptionOrder
		err := model.DB.Where("status = ? AND payment_provider <> ? AND payment_method <> ?",
			common.TopUpStatusSuccess, model.PaymentProviderBalance, model.PaymentMethodBalance).
			Where("id > ?", lastId).
			Order("id asc").
			Limit(scanLimit).
			Find(&batch).Error
		if err != nil {
			return nil, err
		}
		if len(batch) == 0 {
			break
		}
		for _, order := range batch {
			lastId = order.Id
			tradeNo := strings.TrimSpace(order.TradeNo)
			if tradeNo == "" || order.UserId <= 0 || order.PlanId <= 0 {
				candidates = append(candidates, order)
			} else if exists, err := model.FundingBillingEventExists(nil, model.BillingEventSourceSubscription, tradeNo, "purchase"); err != nil || !exists {
				candidates = append(candidates, order)
			}
			if len(candidates) >= limit {
				break
			}
		}
	}
	return candidates, nil
}

func listBackfillSubscriptionBalanceCandidates(limit int) ([]model.SubscriptionOrder, error) {
	candidates := make([]model.SubscriptionOrder, 0, limit)
	lastId := 0
	scanLimit := backfillCandidateScanLimit(limit)
	for len(candidates) < limit {
		var batch []model.SubscriptionOrder
		err := model.DB.Where("status = ? AND (payment_provider = ? OR payment_method = ?)",
			common.TopUpStatusSuccess, model.PaymentProviderBalance, model.PaymentMethodBalance).
			Where("id > ?", lastId).
			Order("id asc").
			Limit(scanLimit).
			Find(&batch).Error
		if err != nil {
			return nil, err
		}
		if len(batch) == 0 {
			break
		}
		for _, order := range batch {
			lastId = order.Id
			tradeNo := strings.TrimSpace(order.TradeNo)
			if tradeNo == "" || order.UserId <= 0 || order.PlanId <= 0 {
				candidates = append(candidates, order)
			} else if subscriptionBalanceBackfillMissing(order) {
				candidates = append(candidates, order)
			}
			if len(candidates) >= limit {
				break
			}
		}
	}
	return candidates, nil
}

func subscriptionBalanceBackfillMissing(order model.SubscriptionOrder) bool {
	tradeNo := strings.TrimSpace(order.TradeNo)
	grantExists, err := model.FundingBillingEventExists(nil, model.BillingEventSourceSubscription, tradeNo, "grant")
	if err != nil || !grantExists {
		return true
	}
	plan, err := model.GetSubscriptionPlanById(order.PlanId)
	if err != nil {
		return true
	}
	requiredQuota, err := backfillSubscriptionBalanceQuota(order, plan)
	if err != nil || requiredQuota <= 0 {
		return false
	}
	balanceExists, err := model.FundingBillingEventExists(nil, model.BillingEventSourceSubscription, tradeNo, "balance_payment")
	return err != nil || !balanceExists
}

func listBackfillAdminSubscriptionCandidates(limit int) ([]model.UserSubscription, error) {
	candidates := make([]model.UserSubscription, 0, limit)
	lastId := 0
	scanLimit := backfillCandidateScanLimit(limit)
	for len(candidates) < limit {
		var batch []model.UserSubscription
		err := model.DB.Where("source = ?", "admin").
			Where("id > ?", lastId).
			Order("id asc").
			Limit(scanLimit).
			Find(&batch).Error
		if err != nil {
			return nil, err
		}
		if len(batch) == 0 {
			break
		}
		for _, sub := range batch {
			lastId = sub.Id
			sourceId := fmt.Sprintf("admin_bind:%d", sub.Id)
			if sub.Id <= 0 || sub.UserId <= 0 || sub.PlanId <= 0 {
				candidates = append(candidates, sub)
			} else if exists, err := model.FundingBillingEventExists(nil, model.BillingEventSourceSubscription, sourceId, "admin_bind"); err != nil || !exists {
				candidates = append(candidates, sub)
			}
			if len(candidates) >= limit {
				break
			}
		}
	}
	return candidates, nil
}

func listBackfillModelRequestCandidates(limit int) ([]model.Log, error) {
	if model.LOG_DB == nil {
		return nil, fmt.Errorf("log database is not initialized")
	}
	candidates := make([]model.Log, 0, limit)
	lastId := 0
	scanLimit := backfillCandidateScanLimit(limit)
	for len(candidates) < limit {
		var batch []model.Log
		err := model.LOG_DB.Model(&model.Log{}).
			Where("type = ? AND quota > 0 AND request_id <> ?", model.LogTypeConsume, "").
			Where("id > ?", lastId).
			Order("id asc").
			Limit(scanLimit).
			Find(&batch).Error
		if err != nil {
			return nil, err
		}
		if len(batch) == 0 {
			break
		}
		for _, log := range batch {
			lastId = log.Id
			expected, _, invalid := modelRequestBillingExpectationFromLog(log)
			if invalid != "" {
				candidates = append(candidates, log)
			} else if exists, err := model.FundingBillingEventExists(nil, model.BillingEventSourceModelRequest, expected.SourceId, expected.Phase); err != nil || !exists {
				candidates = append(candidates, log)
			}
			if len(candidates) >= limit {
				break
			}
		}
	}
	return candidates, nil
}

func listBackfillWalletAdjustmentCandidates(limit int) ([]model.WalletAdjustment, error) {
	candidates := make([]model.WalletAdjustment, 0, limit)
	lastId := int64(0)
	scanLimit := backfillCandidateScanLimit(limit)
	for len(candidates) < limit {
		batch, err := model.ListWalletAdjustmentsAfterId(lastId, scanLimit)
		if err != nil {
			return nil, err
		}
		if len(batch) == 0 {
			break
		}
		for _, adjustment := range batch {
			lastId = adjustment.Id
			sourceId := strings.TrimSpace(adjustment.SourceId)
			if sourceId == "" || adjustment.UserId <= 0 || adjustment.Amount <= 0 {
				candidates = append(candidates, adjustment)
				continue
			}
			exists, err := model.FundingBillingEventExists(nil, model.BillingEventSourceWalletAdjust, sourceId, "adjust")
			if err != nil {
				return nil, err
			}
			if !exists {
				candidates = append(candidates, adjustment)
			}
			if len(candidates) >= limit {
				break
			}
		}
		if len(batch) < scanLimit {
			break
		}
	}
	return candidates, nil
}

func listBackfillTaskBillingRecordCandidates(limit int) ([]model.TaskBillingRecord, error) {
	candidates := make([]model.TaskBillingRecord, 0, limit)
	lastId := int64(0)
	scanLimit := backfillCandidateScanLimit(limit)
	for len(candidates) < limit {
		batch, err := model.ListTaskBillingRecordsAfterId(lastId, scanLimit)
		if err != nil {
			return nil, err
		}
		if len(batch) == 0 {
			break
		}
		for _, record := range batch {
			lastId = record.Id
			if strings.TrimSpace(record.SourceId) == "" || strings.TrimSpace(record.Phase) == "" || record.UserId <= 0 || record.AmountQuota <= 0 {
				candidates = append(candidates, record)
				continue
			}
			exists, err := model.FundingBillingEventExists(nil, model.BillingEventSourceAsyncTask, record.SourceId, record.Phase)
			if err != nil {
				return nil, err
			}
			if !exists {
				candidates = append(candidates, record)
			}
			if len(candidates) >= limit {
				break
			}
		}
		if len(batch) < scanLimit {
			break
		}
	}
	return candidates, nil
}

func listBackfillViolationFeeRecordCandidates(limit int) ([]model.ViolationFeeRecord, error) {
	candidates := make([]model.ViolationFeeRecord, 0, limit)
	lastId := int64(0)
	scanLimit := backfillCandidateScanLimit(limit)
	for len(candidates) < limit {
		batch, err := model.ListViolationFeeRecordsAfterId(lastId, scanLimit)
		if err != nil {
			return nil, err
		}
		if len(batch) == 0 {
			break
		}
		for _, record := range batch {
			lastId = record.Id
			if strings.TrimSpace(record.SourceId) == "" || strings.TrimSpace(record.Phase) == "" || record.UserId <= 0 || record.AmountQuota <= 0 {
				candidates = append(candidates, record)
				continue
			}
			exists, err := model.FundingBillingEventExists(nil, model.BillingEventSourceViolationFee, record.SourceId, record.Phase)
			if err != nil {
				return nil, err
			}
			if !exists {
				candidates = append(candidates, record)
			}
			if len(candidates) >= limit {
				break
			}
		}
		if len(batch) < scanLimit {
			break
		}
	}
	return candidates, nil
}

func listBackfillMCPToolCallCandidates(limit int) ([]model.MCPToolCall, error) {
	candidates := make([]model.MCPToolCall, 0, limit)
	lastId := int64(0)
	scanLimit := backfillCandidateScanLimit(limit)
	for len(candidates) < limit {
		batch, err := model.ListMCPToolCallsAfterId(lastId, scanLimit)
		if err != nil {
			return nil, err
		}
		if len(batch) == 0 {
			break
		}
		for _, call := range batch {
			lastId = call.Id
			expectations, invalid := mcpToolCallBillingExpectations(call)
			if invalid != "" {
				candidates = append(candidates, call)
				continue
			}
			missing := false
			for _, expected := range expectations {
				exists, err := model.FundingBillingEventExists(nil, expected.Source, expected.SourceId, expected.Phase)
				if err != nil {
					return nil, err
				}
				if !exists {
					missing = true
					break
				}
			}
			if missing {
				candidates = append(candidates, call)
			}
			if len(candidates) >= limit {
				break
			}
		}
		if len(batch) < scanLimit {
			break
		}
	}
	return candidates, nil
}

func backfillWalletTopUps(limit int, dryRun bool) (dto.BillingEventBackfillSourceResult, error) {
	result := newBillingEventBackfillSourceResult(BillingEventBackfillSourceWalletTopUp)
	topUps, err := listBackfillWalletTopUpCandidates(limit)
	if err != nil {
		return result, err
	}

	for _, topUp := range topUps {
		result.Scanned++
		tradeNo := strings.TrimSpace(topUp.TradeNo)
		if tradeNo == "" || topUp.UserId <= 0 {
			result.SkippedInvalid++
			addBillingEventBackfillError(&result, fmt.Sprintf("topup:%d missing trade_no or user_id", topUp.Id))
			continue
		}
		exists, err := model.FundingBillingEventSourceExists(nil, model.BillingEventSourceWalletTopUp, tradeNo)
		if err != nil {
			result.ErrorCount++
			addBillingEventBackfillError(&result, fmt.Sprintf("topup:%s exists check failed: %v", tradeNo, err))
			continue
		}
		if exists {
			result.SkippedExisting++
			continue
		}

		quota := calculateBackfillWalletTopUpQuota(topUp)
		if quota <= 0 {
			result.SkippedInvalid++
			addBillingEventBackfillError(&result, fmt.Sprintf("topup:%s invalid quota", tradeNo))
			continue
		}
		if dryRun {
			result.WouldCreate++
			continue
		}
		input, invalid := walletTopUpBackfillInput(topUp, nil)
		if invalid != "" {
			result.SkippedInvalid++
			addBillingEventBackfillError(&result, invalid)
			continue
		}
		created, err := model.RecordFundingBillingEventIfNotExists(nil, input)
		if err != nil {
			result.ErrorCount++
			addBillingEventBackfillError(&result, fmt.Sprintf("topup:%s create failed: %v", tradeNo, err))
			continue
		}
		if created {
			result.Created++
		} else {
			result.SkippedExisting++
		}
	}
	return result, nil
}

func backfillMCPToolCalls(limit int, dryRun bool) (dto.BillingEventBackfillSourceResult, error) {
	result := newBillingEventBackfillSourceResult(BillingEventBackfillSourceMCPToolCall)
	calls, err := listBackfillMCPToolCallCandidates(limit)
	if err != nil {
		return result, err
	}

	for _, call := range calls {
		result.Scanned++
		expectations, invalid := mcpToolCallBillingExpectations(call)
		if invalid != "" {
			result.SkippedInvalid++
			addBillingEventBackfillError(&result, invalid)
			continue
		}
		for _, expected := range expectations {
			exists, err := model.FundingBillingEventExists(nil, expected.Source, expected.SourceId, expected.Phase)
			if err != nil {
				result.ErrorCount++
				addBillingEventBackfillError(&result, fmt.Sprintf("mcp_tool_call:%d %s exists check failed: %v", call.Id, expected.Phase, err))
				continue
			}
			if exists {
				result.SkippedExisting++
				continue
			}
			if dryRun {
				result.WouldCreate++
				continue
			}
			input, invalid := mcpToolCallBackfillInput(call, expected.Phase, nil)
			if invalid != "" {
				result.SkippedInvalid++
				addBillingEventBackfillError(&result, invalid)
				continue
			}
			created, err := model.RecordFundingBillingEventIfNotExists(nil, input)
			if err != nil {
				result.ErrorCount++
				addBillingEventBackfillError(&result, fmt.Sprintf("mcp_tool_call:%d %s create failed: %v", call.Id, expected.Phase, err))
				continue
			}
			if created {
				result.Created++
			} else {
				result.SkippedExisting++
			}
		}
	}
	return result, nil
}

func backfillRedemptions(limit int, dryRun bool) (dto.BillingEventBackfillSourceResult, error) {
	result := newBillingEventBackfillSourceResult(BillingEventBackfillSourceRedemption)
	redemptions, err := listBackfillRedemptionCandidates(limit)
	if err != nil {
		return result, err
	}

	for _, redemption := range redemptions {
		result.Scanned++
		sourceId := fmt.Sprintf("redemption:%d", redemption.Id)
		if redemption.Id <= 0 || redemption.UsedUserId <= 0 || redemption.Quota <= 0 {
			result.SkippedInvalid++
			addBillingEventBackfillError(&result, fmt.Sprintf("redemption:%d invalid redemption data", redemption.Id))
			continue
		}
		exists, err := model.FundingBillingEventExists(nil, model.BillingEventSourceWalletTopUp, sourceId, "redemption")
		if err != nil {
			result.ErrorCount++
			addBillingEventBackfillError(&result, fmt.Sprintf("redemption:%d exists check failed: %v", redemption.Id, err))
			continue
		}
		if exists {
			result.SkippedExisting++
			continue
		}
		if dryRun {
			result.WouldCreate++
			continue
		}
		input, invalid := redemptionBackfillInput(redemption, nil)
		if invalid != "" {
			result.SkippedInvalid++
			addBillingEventBackfillError(&result, invalid)
			continue
		}
		created, err := model.RecordFundingBillingEventIfNotExists(nil, input)
		if err != nil {
			result.ErrorCount++
			addBillingEventBackfillError(&result, fmt.Sprintf("redemption:%d create failed: %v", redemption.Id, err))
			continue
		}
		if created {
			result.Created++
		} else {
			result.SkippedExisting++
		}
	}
	return result, nil
}

func backfillWalletAdjustments(limit int, dryRun bool) (dto.BillingEventBackfillSourceResult, error) {
	result := newBillingEventBackfillSourceResult(BillingEventBackfillSourceWalletAdjust)
	adjustments, err := listBackfillWalletAdjustmentCandidates(limit)
	if err != nil {
		return result, err
	}

	for _, adjustment := range adjustments {
		result.Scanned++
		sourceId := strings.TrimSpace(adjustment.SourceId)
		if sourceId == "" || adjustment.UserId <= 0 || adjustment.Amount <= 0 {
			result.SkippedInvalid++
			addBillingEventBackfillError(&result, fmt.Sprintf("wallet_adjustment:%d invalid adjustment data", adjustment.Id))
			continue
		}
		exists, err := model.FundingBillingEventExists(nil, model.BillingEventSourceWalletAdjust, sourceId, "adjust")
		if err != nil {
			result.ErrorCount++
			addBillingEventBackfillError(&result, fmt.Sprintf("wallet_adjustment:%s exists check failed: %v", sourceId, err))
			continue
		}
		if exists {
			result.SkippedExisting++
			continue
		}
		if dryRun {
			result.WouldCreate++
			continue
		}
		input, invalid := walletAdjustmentBackfillInput(adjustment, nil)
		if invalid != "" {
			result.SkippedInvalid++
			addBillingEventBackfillError(&result, invalid)
			continue
		}
		created, err := model.RecordFundingBillingEventIfNotExists(nil, input)
		if err != nil {
			result.ErrorCount++
			addBillingEventBackfillError(&result, fmt.Sprintf("wallet_adjustment:%s create failed: %v", sourceId, err))
			continue
		}
		if created {
			result.Created++
		} else {
			result.SkippedExisting++
		}
	}
	return result, nil
}

func backfillTaskBillingRecords(limit int, dryRun bool) (dto.BillingEventBackfillSourceResult, error) {
	result := newBillingEventBackfillSourceResult(BillingEventBackfillSourceAsyncTask)
	records, err := listBackfillTaskBillingRecordCandidates(limit)
	if err != nil {
		return result, err
	}

	for _, record := range records {
		result.Scanned++
		expected, invalid := taskBillingRecordExpectation(record)
		if invalid != "" {
			result.SkippedInvalid++
			addBillingEventBackfillError(&result, invalid)
			continue
		}
		exists, err := model.FundingBillingEventExists(nil, model.BillingEventSourceAsyncTask, expected.SourceId, expected.Phase)
		if err != nil {
			result.ErrorCount++
			addBillingEventBackfillError(&result, fmt.Sprintf("task_billing_record:%d exists check failed: %v", record.Id, err))
			continue
		}
		if exists {
			result.SkippedExisting++
			continue
		}
		if dryRun {
			result.WouldCreate++
			continue
		}
		input, invalid := taskBillingRecordBackfillInput(record, nil)
		if invalid != "" {
			result.SkippedInvalid++
			addBillingEventBackfillError(&result, invalid)
			continue
		}
		created, err := model.RecordFundingBillingEventIfNotExists(nil, input)
		if err != nil {
			result.ErrorCount++
			addBillingEventBackfillError(&result, fmt.Sprintf("task_billing_record:%d create failed: %v", record.Id, err))
			continue
		}
		if created {
			result.Created++
		} else {
			result.SkippedExisting++
		}
	}
	return result, nil
}

func backfillViolationFeeRecords(limit int, dryRun bool) (dto.BillingEventBackfillSourceResult, error) {
	result := newBillingEventBackfillSourceResult(BillingEventBackfillSourceViolationFee)
	records, err := listBackfillViolationFeeRecordCandidates(limit)
	if err != nil {
		return result, err
	}

	for _, record := range records {
		result.Scanned++
		expected, invalid := violationFeeRecordExpectation(record)
		if invalid != "" {
			result.SkippedInvalid++
			addBillingEventBackfillError(&result, invalid)
			continue
		}
		exists, err := model.FundingBillingEventExists(nil, model.BillingEventSourceViolationFee, expected.SourceId, expected.Phase)
		if err != nil {
			result.ErrorCount++
			addBillingEventBackfillError(&result, fmt.Sprintf("violation_fee_record:%d exists check failed: %v", record.Id, err))
			continue
		}
		if exists {
			result.SkippedExisting++
			continue
		}
		if dryRun {
			result.WouldCreate++
			continue
		}
		input, invalid := violationFeeRecordBackfillInput(record, nil)
		if invalid != "" {
			result.SkippedInvalid++
			addBillingEventBackfillError(&result, invalid)
			continue
		}
		created, err := model.RecordFundingBillingEventIfNotExists(nil, input)
		if err != nil {
			result.ErrorCount++
			addBillingEventBackfillError(&result, fmt.Sprintf("violation_fee_record:%d create failed: %v", record.Id, err))
			continue
		}
		if created {
			result.Created++
		} else {
			result.SkippedExisting++
		}
	}
	return result, nil
}

func backfillModelRequests(limit int, dryRun bool) (dto.BillingEventBackfillSourceResult, error) {
	result := newBillingEventBackfillSourceResult(BillingEventBackfillSourceModelRequest)
	logs, err := listBackfillModelRequestCandidates(limit)
	if err != nil {
		return result, err
	}

	for _, log := range logs {
		result.Scanned++
		expected, _, invalid := modelRequestBillingExpectationFromLog(log)
		if invalid != "" {
			result.SkippedInvalid++
			addBillingEventBackfillError(&result, invalid)
			continue
		}
		exists, err := model.FundingBillingEventExists(nil, model.BillingEventSourceModelRequest, expected.SourceId, expected.Phase)
		if err != nil {
			result.ErrorCount++
			addBillingEventBackfillError(&result, fmt.Sprintf("model_request:%s exists check failed: %v", expected.SourceId, err))
			continue
		}
		if exists {
			result.SkippedExisting++
			continue
		}
		if dryRun {
			result.WouldCreate++
			continue
		}
		input, invalid := modelRequestBackfillInput(log, nil)
		if invalid != "" {
			result.SkippedInvalid++
			addBillingEventBackfillError(&result, invalid)
			continue
		}
		created, err := model.RecordFundingBillingEventIfNotExists(nil, input)
		if err != nil {
			result.ErrorCount++
			addBillingEventBackfillError(&result, fmt.Sprintf("model_request:%s create failed: %v", expected.SourceId, err))
			continue
		}
		if created {
			result.Created++
		} else {
			result.SkippedExisting++
		}
	}
	return result, nil
}

func backfillSubscriptionPurchases(limit int, dryRun bool) (dto.BillingEventBackfillSourceResult, error) {
	result := newBillingEventBackfillSourceResult(BillingEventBackfillSourceSubscriptionPurchase)
	orders, err := listBackfillSubscriptionPurchaseCandidates(limit)
	if err != nil {
		return result, err
	}

	for _, order := range orders {
		result.Scanned++
		tradeNo := strings.TrimSpace(order.TradeNo)
		if tradeNo == "" || order.UserId <= 0 || order.PlanId <= 0 {
			result.SkippedInvalid++
			addBillingEventBackfillError(&result, fmt.Sprintf("subscription_order:%d invalid purchase data", order.Id))
			continue
		}
		exists, err := model.FundingBillingEventExists(nil, model.BillingEventSourceSubscription, tradeNo, "purchase")
		if err != nil {
			result.ErrorCount++
			addBillingEventBackfillError(&result, fmt.Sprintf("subscription_order:%s exists check failed: %v", tradeNo, err))
			continue
		}
		if exists {
			result.SkippedExisting++
			continue
		}
		plan, err := model.GetSubscriptionPlanById(order.PlanId)
		if err != nil {
			result.SkippedInvalid++
			addBillingEventBackfillError(&result, fmt.Sprintf("subscription_order:%s missing plan %d: %v", tradeNo, order.PlanId, err))
			continue
		}
		subscription := findBackfillSubscription(order, "order")
		if dryRun {
			result.WouldCreate++
			continue
		}
		created, err := model.RecordFundingBillingEventIfNotExists(nil, subscriptionPurchaseBackfillInput(order, plan, subscription, nil))
		if err != nil {
			result.ErrorCount++
			addBillingEventBackfillError(&result, fmt.Sprintf("subscription_order:%s create failed: %v", tradeNo, err))
			continue
		}
		if created {
			result.Created++
		} else {
			result.SkippedExisting++
		}
	}
	return result, nil
}

func backfillSubscriptionBalanceOrders(limit int, dryRun bool) (dto.BillingEventBackfillSourceResult, error) {
	result := newBillingEventBackfillSourceResult(BillingEventBackfillSourceSubscriptionBalance)
	orders, err := listBackfillSubscriptionBalanceCandidates(limit)
	if err != nil {
		return result, err
	}

	for _, order := range orders {
		result.Scanned++
		tradeNo := strings.TrimSpace(order.TradeNo)
		if tradeNo == "" || order.UserId <= 0 || order.PlanId <= 0 {
			result.SkippedInvalid++
			addBillingEventBackfillError(&result, fmt.Sprintf("subscription_order:%d invalid balance data", order.Id))
			continue
		}
		plan, err := model.GetSubscriptionPlanById(order.PlanId)
		if err != nil {
			result.SkippedInvalid++
			addBillingEventBackfillError(&result, fmt.Sprintf("subscription_order:%s missing plan %d: %v", tradeNo, order.PlanId, err))
			continue
		}
		subscription := findBackfillSubscription(order, model.PaymentMethodBalance)
		createdAt := backfillEventTime(order.CompleteTime, order.CreateTime)
		requiredQuota, err := backfillSubscriptionBalanceQuota(order, plan)
		if err != nil {
			result.SkippedInvalid++
			addBillingEventBackfillError(&result, fmt.Sprintf("subscription_order:%s invalid balance quota: %v", tradeNo, err))
			continue
		}

		if requiredQuota > 0 {
			backfillSubscriptionBalancePaymentEvent(&result, order, plan, subscription.Id, dryRun, requiredQuota, createdAt)
		}
		backfillSubscriptionGrantEvent(&result, order, plan, subscription, dryRun, createdAt)
	}
	return result, nil
}

func backfillAdminSubscriptions(limit int, dryRun bool) (dto.BillingEventBackfillSourceResult, error) {
	result := newBillingEventBackfillSourceResult(BillingEventBackfillSourceSubscriptionAdmin)
	subscriptions, err := listBackfillAdminSubscriptionCandidates(limit)
	if err != nil {
		return result, err
	}

	for _, sub := range subscriptions {
		result.Scanned++
		sourceId := fmt.Sprintf("admin_bind:%d", sub.Id)
		if sub.Id <= 0 || sub.UserId <= 0 || sub.PlanId <= 0 {
			result.SkippedInvalid++
			addBillingEventBackfillError(&result, fmt.Sprintf("user_subscription:%d invalid admin subscription data", sub.Id))
			continue
		}
		exists, err := model.FundingBillingEventExists(nil, model.BillingEventSourceSubscription, sourceId, "admin_bind")
		if err != nil {
			result.ErrorCount++
			addBillingEventBackfillError(&result, fmt.Sprintf("user_subscription:%d exists check failed: %v", sub.Id, err))
			continue
		}
		if exists {
			result.SkippedExisting++
			continue
		}
		plan, err := model.GetSubscriptionPlanById(sub.PlanId)
		if err != nil {
			result.SkippedInvalid++
			addBillingEventBackfillError(&result, fmt.Sprintf("user_subscription:%d missing plan %d: %v", sub.Id, sub.PlanId, err))
			continue
		}
		if dryRun {
			result.WouldCreate++
			continue
		}
		created, err := model.RecordFundingBillingEventIfNotExists(nil, adminSubscriptionBackfillInput(sub, plan, nil))
		if err != nil {
			result.ErrorCount++
			addBillingEventBackfillError(&result, fmt.Sprintf("user_subscription:%d create failed: %v", sub.Id, err))
			continue
		}
		if created {
			result.Created++
		} else {
			result.SkippedExisting++
		}
	}
	return result, nil
}

func newBillingEventBackfillSourceResult(source string) dto.BillingEventBackfillSourceResult {
	return dto.BillingEventBackfillSourceResult{
		Source: source,
		Errors: []string{},
	}
}

func addBillingEventBackfillError(result *dto.BillingEventBackfillSourceResult, message string) {
	if len(result.Errors) < maxBillingEventBackfillErrors {
		result.Errors = append(result.Errors, message)
	}
}

func calculateBackfillWalletTopUpQuota(topUp model.TopUp) int {
	provider := strings.TrimSpace(topUp.PaymentProvider)
	method := strings.TrimSpace(topUp.PaymentMethod)
	switch {
	case provider == model.PaymentProviderCreem || method == model.PaymentMethodCreem:
		return int(topUp.Amount)
	case provider == model.PaymentProviderStripe || method == model.PaymentMethodStripe:
		return int(decimal.NewFromFloat(topUp.Money).Mul(decimal.NewFromFloat(common.QuotaPerUnit)).IntPart())
	default:
		return int(decimal.NewFromInt(topUp.Amount).Mul(decimal.NewFromFloat(common.QuotaPerUnit)).IntPart())
	}
}

func backfillPaymentChannel(provider string, method string) string {
	provider = strings.TrimSpace(provider)
	if provider != "" {
		return provider
	}
	method = strings.TrimSpace(method)
	if method != "" {
		return method
	}
	return "unknown"
}

func backfillEventTime(values ...int64) int64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return common.GetTimestamp()
}

func mergeBillingEventMetadata(base map[string]any, overlays ...map[string]any) map[string]any {
	merged := map[string]any{}
	for key, value := range base {
		merged[key] = value
	}
	for _, overlay := range overlays {
		for key, value := range overlay {
			merged[key] = value
		}
	}
	return merged
}

func walletTopUpBackfillInput(topUp model.TopUp, extraMetadata map[string]any) (model.FundingBillingEventInput, string) {
	tradeNo := strings.TrimSpace(topUp.TradeNo)
	if tradeNo == "" || topUp.UserId <= 0 {
		return model.FundingBillingEventInput{}, fmt.Sprintf("topup:%d missing trade_no or user_id", topUp.Id)
	}
	quota := calculateBackfillWalletTopUpQuota(topUp)
	if quota <= 0 {
		return model.FundingBillingEventInput{}, fmt.Sprintf("topup:%s invalid quota", tradeNo)
	}
	return model.FundingBillingEventInput{
		Source:        model.BillingEventSourceWalletTopUp,
		SourceId:      tradeNo,
		Phase:         "success",
		UserId:        topUp.UserId,
		RequestId:     tradeNo,
		BillingSource: BillingSourceWallet,
		PriceUnit:     "topup",
		EventType:     model.BillingEventTypeCredit,
		AmountQuota:   quota,
		CreatedAt:     backfillEventTime(topUp.CompleteTime, topUp.CreateTime),
		Metadata: mergeBillingEventMetadata(map[string]any{
			"backfill":         true,
			"channel":          backfillPaymentChannel(topUp.PaymentProvider, topUp.PaymentMethod),
			"trade_no":         tradeNo,
			"topup_id":         topUp.Id,
			"payment_method":   topUp.PaymentMethod,
			"payment_provider": topUp.PaymentProvider,
			"money":            topUp.Money,
			"amount":           topUp.Amount,
			"status":           topUp.Status,
			"complete_time":    topUp.CompleteTime,
			"create_time":      topUp.CreateTime,
		}, extraMetadata),
	}, ""
}

func redemptionBackfillInput(redemption model.Redemption, extraMetadata map[string]any) (model.FundingBillingEventInput, string) {
	sourceId := fmt.Sprintf("redemption:%d", redemption.Id)
	if redemption.Id <= 0 || redemption.UsedUserId <= 0 || redemption.Quota <= 0 {
		return model.FundingBillingEventInput{}, fmt.Sprintf("redemption:%d invalid redemption data", redemption.Id)
	}
	return model.FundingBillingEventInput{
		Source:        model.BillingEventSourceWalletTopUp,
		SourceId:      sourceId,
		Phase:         "redemption",
		UserId:        redemption.UsedUserId,
		RequestId:     sourceId,
		BillingSource: BillingSourceWallet,
		PriceUnit:     "redemption",
		EventType:     model.BillingEventTypeCredit,
		AmountQuota:   redemption.Quota,
		CreatedAt:     backfillEventTime(redemption.RedeemedTime, redemption.CreatedTime),
		Metadata: mergeBillingEventMetadata(map[string]any{
			"backfill":        true,
			"channel":         "redemption",
			"redemption_id":   redemption.Id,
			"name":            redemption.Name,
			"quota":           redemption.Quota,
			"created_time":    redemption.CreatedTime,
			"redeemed_time":   redemption.RedeemedTime,
			"used_user_id":    redemption.UsedUserId,
			"was_soft_delete": redemption.DeletedAt.Valid,
		}, extraMetadata),
	}, ""
}

func walletAdjustmentBackfillInput(adjustment model.WalletAdjustment, extraMetadata map[string]any) (model.FundingBillingEventInput, string) {
	sourceId := strings.TrimSpace(adjustment.SourceId)
	if sourceId == "" || adjustment.UserId <= 0 || adjustment.Amount <= 0 {
		return model.FundingBillingEventInput{}, fmt.Sprintf("wallet_adjustment:%d invalid adjustment data", adjustment.Id)
	}
	eventType := strings.TrimSpace(adjustment.EventType)
	if eventType != model.BillingEventTypeCredit && eventType != model.BillingEventTypeDebit {
		return model.FundingBillingEventInput{}, fmt.Sprintf("wallet_adjustment:%s invalid event_type %s", sourceId, adjustment.EventType)
	}
	metadata := map[string]any{}
	if strings.TrimSpace(adjustment.Metadata) != "" {
		_ = common.UnmarshalJsonStr(adjustment.Metadata, &metadata)
	}
	return model.FundingBillingEventInput{
		Source:        model.BillingEventSourceWalletAdjust,
		SourceId:      sourceId,
		Phase:         "adjust",
		UserId:        adjustment.UserId,
		RequestId:     sourceId,
		BillingSource: BillingSourceWallet,
		PriceUnit:     "manual_adjust",
		EventType:     eventType,
		AmountQuota:   adjustment.Amount,
		CreatedAt:     backfillEventTime(adjustment.LedgeredAt, adjustment.CreatedAt),
		Metadata: mergeBillingEventMetadata(map[string]any{
			"backfill":             true,
			"wallet_adjustment_id": adjustment.Id,
			"admin_id":             adjustment.AdminId,
			"mode":                 adjustment.Mode,
			"old_quota":            adjustment.OldQuota,
			"new_quota":            adjustment.NewQuota,
		}, metadata, extraMetadata),
	}, ""
}

func modelRequestBackfillInput(log model.Log, extraMetadata map[string]any) (model.FundingBillingEventInput, string) {
	expected, metadata, invalid := modelRequestBillingExpectationFromLog(log)
	if invalid != "" {
		return model.FundingBillingEventInput{}, invalid
	}
	priceUnit := "token_usage"
	if expected.PriceUnit != "" {
		priceUnit = expected.PriceUnit
	}
	return model.FundingBillingEventInput{
		Source:        expected.Source,
		SourceId:      expected.SourceId,
		Phase:         expected.Phase,
		UserId:        expected.UserId,
		TokenId:       expected.TokenId,
		RequestId:     expected.RequestId,
		Group:         expected.Group,
		BillingSource: expected.BillingSource,
		PriceUnit:     priceUnit,
		EventType:     expected.EventType,
		AmountQuota:   expected.AmountQuota,
		CreatedAt:     log.CreatedAt,
		Metadata:      mergeBillingEventMetadata(metadata, extraMetadata),
	}, ""
}

func subscriptionPurchaseBackfillInput(order model.SubscriptionOrder, plan *model.SubscriptionPlan, subscription backfillSubscriptionSnapshot, extraMetadata map[string]any) model.FundingBillingEventInput {
	tradeNo := strings.TrimSpace(order.TradeNo)
	amountQuota := backfillSubscriptionGrantQuota(subscription, plan)
	return model.FundingBillingEventInput{
		Source:        model.BillingEventSourceSubscription,
		SourceId:      tradeNo,
		Phase:         "purchase",
		UserId:        order.UserId,
		RequestId:     tradeNo,
		BillingSource: BillingSourceSubscription,
		PriceUnit:     "subscription",
		EventType:     model.BillingEventTypeCredit,
		AmountQuota:   amountQuota,
		AllowZero:     true,
		CreatedAt:     backfillEventTime(order.CompleteTime, order.CreateTime),
		Metadata: mergeBillingEventMetadata(map[string]any{
			"backfill":          true,
			"trade_no":          tradeNo,
			"order_id":          order.Id,
			"subscription_id":   subscription.Id,
			"plan_id":           plan.Id,
			"plan_title":        plan.Title,
			"plan_subtitle":     plan.Subtitle,
			"payment_method":    order.PaymentMethod,
			"payment_provider":  order.PaymentProvider,
			"money":             order.Money,
			"price_amount":      plan.PriceAmount,
			"currency":          plan.Currency,
			"duration_unit":     plan.DurationUnit,
			"duration_value":    plan.DurationValue,
			"quota_reset":       plan.QuotaResetPeriod,
			"upgrade_group":     plan.UpgradeGroup,
			"subscription_from": subscription.Source,
			"quota_from":        subscription.QuotaSource,
		}, extraMetadata),
	}
}

func subscriptionBalancePaymentBackfillInput(order model.SubscriptionOrder, plan *model.SubscriptionPlan, subscriptionId int, requiredQuota int, createdAt int64, extraMetadata map[string]any) model.FundingBillingEventInput {
	tradeNo := strings.TrimSpace(order.TradeNo)
	return model.FundingBillingEventInput{
		Source:        model.BillingEventSourceSubscription,
		SourceId:      tradeNo,
		Phase:         "balance_payment",
		UserId:        order.UserId,
		RequestId:     tradeNo,
		BillingSource: BillingSourceWallet,
		PriceUnit:     "subscription_balance_payment",
		EventType:     model.BillingEventTypeDebit,
		AmountQuota:   requiredQuota,
		CreatedAt:     createdAt,
		Metadata: mergeBillingEventMetadata(map[string]any{
			"backfill":         true,
			"trade_no":         tradeNo,
			"order_id":         order.Id,
			"subscription_id":  subscriptionId,
			"plan_id":          plan.Id,
			"plan_title":       plan.Title,
			"payment_method":   model.PaymentMethodBalance,
			"payment_provider": model.PaymentProviderBalance,
			"money":            order.Money,
			"price_amount":     plan.PriceAmount,
			"currency":         plan.Currency,
		}, extraMetadata),
	}
}

func subscriptionGrantBackfillInput(order model.SubscriptionOrder, plan *model.SubscriptionPlan, subscription backfillSubscriptionSnapshot, createdAt int64, extraMetadata map[string]any) model.FundingBillingEventInput {
	tradeNo := strings.TrimSpace(order.TradeNo)
	amountQuota := backfillSubscriptionGrantQuota(subscription, plan)
	return model.FundingBillingEventInput{
		Source:        model.BillingEventSourceSubscription,
		SourceId:      tradeNo,
		Phase:         "grant",
		UserId:        order.UserId,
		RequestId:     tradeNo,
		BillingSource: BillingSourceSubscription,
		PriceUnit:     "subscription",
		EventType:     model.BillingEventTypeCredit,
		AmountQuota:   amountQuota,
		AllowZero:     true,
		CreatedAt:     createdAt,
		Metadata: mergeBillingEventMetadata(map[string]any{
			"backfill":          true,
			"trade_no":          tradeNo,
			"order_id":          order.Id,
			"subscription_id":   subscription.Id,
			"plan_id":           plan.Id,
			"plan_title":        plan.Title,
			"plan_subtitle":     plan.Subtitle,
			"payment_method":    model.PaymentMethodBalance,
			"payment_provider":  model.PaymentProviderBalance,
			"money":             order.Money,
			"price_amount":      plan.PriceAmount,
			"currency":          plan.Currency,
			"duration_unit":     plan.DurationUnit,
			"duration_value":    plan.DurationValue,
			"quota_reset":       plan.QuotaResetPeriod,
			"upgrade_group":     plan.UpgradeGroup,
			"subscription_from": subscription.Source,
			"quota_from":        subscription.QuotaSource,
		}, extraMetadata),
	}
}

func adminSubscriptionBackfillInput(sub model.UserSubscription, plan *model.SubscriptionPlan, extraMetadata map[string]any) model.FundingBillingEventInput {
	sourceId := fmt.Sprintf("admin_bind:%d", sub.Id)
	snapshot := backfillSubscriptionSnapshot{
		Id:          sub.Id,
		Source:      sub.Source,
		AmountTotal: sub.AmountTotal,
		QuotaSource: "user_subscription",
	}
	amountQuota := backfillSubscriptionGrantQuota(snapshot, plan)
	return model.FundingBillingEventInput{
		Source:        model.BillingEventSourceSubscription,
		SourceId:      sourceId,
		Phase:         "admin_bind",
		UserId:        sub.UserId,
		RequestId:     sourceId,
		BillingSource: BillingSourceSubscription,
		PriceUnit:     "subscription",
		EventType:     model.BillingEventTypeCredit,
		AmountQuota:   amountQuota,
		AllowZero:     true,
		CreatedAt:     backfillEventTime(sub.CreatedAt, sub.StartTime),
		Metadata: mergeBillingEventMetadata(map[string]any{
			"backfill":        true,
			"subscription_id": sub.Id,
			"plan_id":         plan.Id,
			"plan_title":      plan.Title,
			"plan_subtitle":   plan.Subtitle,
			"source_note":     "backfill",
			"price_amount":    plan.PriceAmount,
			"currency":        plan.Currency,
			"duration_unit":   plan.DurationUnit,
			"duration_value":  plan.DurationValue,
			"quota_reset":     plan.QuotaResetPeriod,
			"upgrade_group":   plan.UpgradeGroup,
			"quota_from":      "user_subscription",
		}, extraMetadata),
	}
}

type backfillSubscriptionSnapshot struct {
	Id          int
	Source      string
	AmountTotal int64
	QuotaSource string
}

func findBackfillSubscription(order model.SubscriptionOrder, source string) backfillSubscriptionSnapshot {
	createdAt := backfillEventTime(order.CompleteTime, order.CreateTime)
	var sub model.UserSubscription
	query := model.DB.Where("user_id = ? AND plan_id = ?", order.UserId, order.PlanId)
	if strings.TrimSpace(source) != "" {
		query = query.Where("source = ?", source)
	}
	result := query.
		Where("created_at <= ?", createdAt+60).
		Order("created_at desc, id desc").
		Limit(1).
		Find(&sub)
	if result.Error != nil {
		return backfillSubscriptionSnapshot{Source: source, QuotaSource: "plan"}
	}
	if result.RowsAffected == 0 {
		sub = model.UserSubscription{}
		fallback := model.DB.Where("user_id = ? AND plan_id = ?", order.UserId, order.PlanId)
		if strings.TrimSpace(source) != "" {
			fallback = fallback.Where("source = ?", source)
		}
		result = fallback.
			Order("created_at asc, id asc").
			Limit(1).
			Find(&sub)
	}
	if result.Error != nil || result.RowsAffected == 0 {
		return backfillSubscriptionSnapshot{Source: source, QuotaSource: "plan"}
	}
	return backfillSubscriptionSnapshot{
		Id:          sub.Id,
		Source:      sub.Source,
		AmountTotal: sub.AmountTotal,
		QuotaSource: "user_subscription",
	}
}

func backfillSubscriptionGrantQuota(snapshot backfillSubscriptionSnapshot, plan *model.SubscriptionPlan) int {
	if snapshot.AmountTotal >= 0 && snapshot.QuotaSource == "user_subscription" {
		return int(snapshot.AmountTotal)
	}
	if plan == nil {
		return 0
	}
	return int(plan.TotalAmount)
}

func backfillSubscriptionBalanceQuota(order model.SubscriptionOrder, plan *model.SubscriptionPlan) (int, error) {
	if quota, ok := parseBackfillChargedQuota(order.ProviderPayload); ok {
		return quota, nil
	}
	return model.CalculateSubscriptionBalanceQuota(plan.PriceAmount)
}

func parseBackfillChargedQuota(providerPayload string) (int, bool) {
	const prefix = "charged_quota="
	for _, part := range strings.FieldsFunc(providerPayload, func(r rune) bool {
		return r == '&' || r == ';' || r == ',' || r == '\n' || r == '\r' || r == '\t' || r == ' '
	}) {
		part = strings.TrimSpace(part)
		if !strings.HasPrefix(part, prefix) {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(part, prefix))
		quota, err := strconv.Atoi(value)
		if err != nil || quota < 0 {
			return 0, false
		}
		return quota, true
	}
	return 0, false
}

func backfillSubscriptionBalancePaymentEvent(result *dto.BillingEventBackfillSourceResult, order model.SubscriptionOrder, plan *model.SubscriptionPlan, subscriptionId int, dryRun bool, requiredQuota int, createdAt int64) {
	exists, err := model.FundingBillingEventExists(nil, model.BillingEventSourceSubscription, order.TradeNo, "balance_payment")
	if err != nil {
		result.ErrorCount++
		addBillingEventBackfillError(result, fmt.Sprintf("subscription_order:%s balance_payment exists check failed: %v", order.TradeNo, err))
		return
	}
	if exists {
		result.SkippedExisting++
		return
	}
	if dryRun {
		result.WouldCreate++
		return
	}
	created, err := model.RecordFundingBillingEventIfNotExists(nil, subscriptionBalancePaymentBackfillInput(order, plan, subscriptionId, requiredQuota, createdAt, nil))
	if err != nil {
		result.ErrorCount++
		addBillingEventBackfillError(result, fmt.Sprintf("subscription_order:%s balance_payment create failed: %v", order.TradeNo, err))
		return
	}
	if created {
		result.Created++
	} else {
		result.SkippedExisting++
	}
}

func backfillSubscriptionGrantEvent(result *dto.BillingEventBackfillSourceResult, order model.SubscriptionOrder, plan *model.SubscriptionPlan, subscription backfillSubscriptionSnapshot, dryRun bool, createdAt int64) {
	exists, err := model.FundingBillingEventExists(nil, model.BillingEventSourceSubscription, order.TradeNo, "grant")
	if err != nil {
		result.ErrorCount++
		addBillingEventBackfillError(result, fmt.Sprintf("subscription_order:%s grant exists check failed: %v", order.TradeNo, err))
		return
	}
	if exists {
		result.SkippedExisting++
		return
	}
	if dryRun {
		result.WouldCreate++
		return
	}
	created, err := model.RecordFundingBillingEventIfNotExists(nil, subscriptionGrantBackfillInput(order, plan, subscription, createdAt, nil))
	if err != nil {
		result.ErrorCount++
		addBillingEventBackfillError(result, fmt.Sprintf("subscription_order:%s grant create failed: %v", order.TradeNo, err))
		return
	}
	if created {
		result.Created++
	} else {
		result.SkippedExisting++
	}
}

func listModelRequestConsumeLogs(limit int) ([]model.Log, error) {
	if model.LOG_DB == nil {
		return nil, fmt.Errorf("log database is not initialized")
	}
	var logs []model.Log
	err := model.LOG_DB.Model(&model.Log{}).
		Where("type = ? AND quota > 0 AND request_id <> ?", model.LogTypeConsume, "").
		Order("id asc").
		Limit(limit).
		Find(&logs).Error
	return logs, err
}

func modelRequestBillingExpectationFromLog(log model.Log) (billingEventExpectation, map[string]any, string) {
	requestId := strings.TrimSpace(log.RequestId)
	if requestId == "" {
		return billingEventExpectation{}, nil, fmt.Sprintf("consume_log:%d missing request_id", log.Id)
	}
	if log.UserId <= 0 || log.Quota <= 0 {
		return billingEventExpectation{}, nil, fmt.Sprintf("consume_log:%d invalid user_id or quota", log.Id)
	}

	other, otherParseError := consumeLogOtherMap(log.Other)
	billingSource := modelRequestBillingSourceFromLogOther(other)
	usageKind := modelRequestUsageKindFromLogOther(other)
	priceUnit := "token_usage"
	if usageKind == "midjourney" {
		priceUnit = "per_call"
	}
	metadata := map[string]any{
		"backfill":          true,
		"usage_kind":        usageKind,
		"log_id":            log.Id,
		"log_created_at":    log.CreatedAt,
		"log_content":       log.Content,
		"model_name":        log.ModelName,
		"token_name":        log.TokenName,
		"channel_id":        log.ChannelId,
		"prompt_tokens":     log.PromptTokens,
		"completion_tokens": log.CompletionTokens,
		"total_tokens":      log.PromptTokens + log.CompletionTokens,
		"use_time":          log.UseTime,
		"is_stream":         log.IsStream,
		"group":             log.Group,
		"billing_source":    billingSource,
	}
	if log.UpstreamRequestId != "" {
		metadata["upstream_request_id"] = log.UpstreamRequestId
	}
	if otherParseError != "" {
		metadata["log_other_parse_error"] = otherParseError
	} else if len(other) > 0 {
		metadata["log_other"] = other
		appendModelRequestMetadataFromLogOther(metadata, other)
	}

	return billingEventExpectation{
		Source:        model.BillingEventSourceModelRequest,
		SourceId:      requestId,
		Phase:         "settlement",
		UserId:        log.UserId,
		TokenId:       log.TokenId,
		CheckTokenId:  log.TokenId > 0,
		EventType:     model.BillingEventTypeDebit,
		AmountQuota:   log.Quota,
		QuotaDelta:    -log.Quota,
		Status:        model.BillingEventStatusSettled,
		RequestId:     requestId,
		Group:         log.Group,
		BillingSource: billingSource,
		PriceUnit:     priceUnit,
	}, metadata, ""
}

func appendModelRequestMetadataFromLogOther(metadata map[string]any, other map[string]any) {
	if metadata == nil || other == nil {
		return
	}
	copyKeys := []string{
		"billing_preference",
		"subscription_id",
		"subscription_plan_id",
		"subscription_plan_title",
		"subscription_pre_consumed",
		"subscription_post_delta",
		"subscription_total",
		"subscription_used",
		"subscription_remain",
		"subscription_consumed",
		"wallet_quota_deducted",
		"request_path",
		"model_price",
		"group_ratio",
		"user_group_ratio",
		"midjourney_action",
		"midjourney_upstream_task_id",
	}
	for _, key := range copyKeys {
		value, ok := other[key]
		if !ok || value == nil || fmt.Sprint(value) == "" {
			continue
		}
		metadata[key] = value
	}
}

func consumeLogOtherMap(other string) (map[string]any, string) {
	other = strings.TrimSpace(other)
	if other == "" {
		return map[string]any{}, ""
	}
	parsed, err := common.StrToMap(other)
	if err != nil {
		return map[string]any{}, err.Error()
	}
	return parsed, ""
}

func modelRequestBillingSourceFromLogOther(other map[string]any) string {
	if source, ok := other["billing_source"].(string); ok {
		source = strings.TrimSpace(source)
		if source == BillingSourceWallet || source == BillingSourceSubscription {
			return source
		}
	}
	if metadataNumberGreaterThanZero(other["subscription_id"]) {
		return BillingSourceSubscription
	}
	return BillingSourceWallet
}

func modelRequestUsageKindFromLogOther(other map[string]any) string {
	if usageKind, ok := other["usage_kind"].(string); ok {
		usageKind = strings.TrimSpace(usageKind)
		if usageKind != "" {
			return usageKind
		}
	}
	if metadataBool(other["ws"]) {
		return "realtime"
	}
	if metadataBool(other["audio"]) {
		return "audio"
	}
	return "text"
}

func metadataBool(value any) bool {
	boolValue, ok := value.(bool)
	return ok && boolValue
}

func metadataNumberGreaterThanZero(value any) bool {
	if value == nil {
		return false
	}
	parsed, err := strconv.ParseFloat(fmt.Sprint(value), 64)
	return err == nil && parsed > 0
}

func concatSQL(parts ...string) string {
	if common.UsingPostgreSQL || common.UsingSQLite {
		return strings.Join(parts, " || ")
	}
	return "CONCAT(" + strings.Join(parts, ", ") + ")"
}

func redemptionIdSQL() string {
	return intColumnStringSQL("redemptions.id")
}

func userSubscriptionIdSQL() string {
	return intColumnStringSQL("user_subscriptions.id")
}

func intColumnStringSQL(column string) string {
	switch {
	case common.UsingPostgreSQL:
		return column + "::text"
	case common.UsingSQLite:
		return "CAST(" + column + " AS TEXT)"
	default:
		return "CAST(" + column + " AS CHAR)"
	}
}
