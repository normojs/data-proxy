package service

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"gorm.io/gorm"
)

const (
	taskBillingEventPhaseInitialSettlement = "initial_settlement"
	taskBillingEventPhaseDeltaDebit        = "delta_debit"
	taskBillingEventPhaseDeltaCredit       = "delta_credit"
	taskBillingEventPhaseFailureRefund     = "failure_refund"
)

func RecordTaskInitialBillingEvent(relayInfo *relaycommon.RelayInfo, task *model.Task, quota int) error {
	if relayInfo == nil || task == nil || quota <= 0 {
		return nil
	}
	billingSource := strings.TrimSpace(relayInfo.BillingSource)
	if billingSource == "" {
		billingSource = BillingSourceWallet
	}
	metadata := map[string]any{
		"phase":                   taskBillingEventPhaseInitialSettlement,
		"task_id":                 task.TaskID,
		"upstream_task_id":        task.PrivateData.UpstreamTaskID,
		"platform":                task.Platform,
		"action":                  task.Action,
		"status":                  task.Status,
		"model_name":              relayInfo.OriginModelName,
		"upstream_model_name":     upstreamModelName(relayInfo),
		"channel_id":              billingEventChannelId(relayInfo),
		"channel_type":            billingEventChannelType(relayInfo),
		"model_ratio":             relayInfo.PriceData.ModelRatio,
		"group_ratio":             relayInfo.PriceData.GroupRatioInfo.GroupRatio,
		"model_price":             relayInfo.PriceData.ModelPrice,
		"use_price":               relayInfo.PriceData.UsePrice,
		"other_ratios":            relayInfo.PriceData.OtherRatios,
		"billing_source":          billingSource,
		"subscription_id":         relayInfo.SubscriptionId,
		"subscription_plan_id":    relayInfo.SubscriptionPlanId,
		"subscription_plan_title": relayInfo.SubscriptionPlanTitle,
	}
	return recordTaskBillingEvent(TaskBillingEventInput{
		SourceId:      task.TaskID,
		Phase:         taskBillingEventPhaseInitialSettlement,
		UserId:        task.UserId,
		SubsiteId:     relayInfo.SubsiteId,
		TokenId:       task.PrivateData.TokenId,
		RequestId:     task.TaskID,
		Group:         task.Group,
		BillingSource: billingSource,
		PriceUnit:     "task",
		EventType:     model.BillingEventTypeDebit,
		AmountQuota:   quota,
		QuotaDelta:    -quota,
		Metadata:      metadata,
	})
}

func RecordTaskRefundBillingEvent(task *model.Task, quota int, reason string) error {
	if task == nil || quota <= 0 {
		return nil
	}
	metadata := taskBillingEventMetadata(task, taskBillingEventPhaseFailureRefund, reason)
	return recordTaskBillingEvent(TaskBillingEventInput{
		SourceId:      task.TaskID,
		Phase:         taskBillingEventPhaseFailureRefund,
		UserId:        task.UserId,
		TokenId:       task.PrivateData.TokenId,
		RequestId:     task.TaskID,
		Group:         task.Group,
		BillingSource: taskBillingSource(task),
		PriceUnit:     "task_refund",
		EventType:     model.BillingEventTypeCredit,
		AmountQuota:   quota,
		QuotaDelta:    quota,
		Metadata:      metadata,
	})
}

func RecordTaskRecalculationBillingEvent(task *model.Task, preConsumedQuota int, actualQuota int, quotaDelta int, reason string) error {
	if task == nil || quotaDelta == 0 {
		return nil
	}
	phase := taskBillingEventPhaseDeltaDebit
	eventType := model.BillingEventTypeDebit
	amountQuota := quotaDelta
	eventQuotaDelta := -amountQuota
	if quotaDelta < 0 {
		phase = taskBillingEventPhaseDeltaCredit
		eventType = model.BillingEventTypeCredit
		amountQuota = -quotaDelta
		eventQuotaDelta = amountQuota
	}
	metadata := taskBillingEventMetadata(task, phase, reason)
	metadata["pre_consumed_quota"] = preConsumedQuota
	metadata["actual_quota"] = actualQuota
	metadata["quota_delta"] = quotaDelta
	return recordTaskBillingEvent(TaskBillingEventInput{
		SourceId:      task.TaskID,
		Phase:         phase,
		UserId:        task.UserId,
		TokenId:       task.PrivateData.TokenId,
		RequestId:     task.TaskID,
		Group:         task.Group,
		BillingSource: taskBillingSource(task),
		PriceUnit:     "task_recalculation",
		EventType:     eventType,
		AmountQuota:   amountQuota,
		QuotaDelta:    eventQuotaDelta,
		Metadata:      metadata,
	})
}

type TaskBillingEventInput struct {
	SourceId      string
	Phase         string
	UserId        int
	SubsiteId     int64
	TokenId       int
	RequestId     string
	Group         string
	BillingSource string
	PriceUnit     string
	EventType     string
	AmountQuota   int
	QuotaDelta    int
	Metadata      map[string]any
}

func recordTaskBillingEvent(input TaskBillingEventInput) error {
	return recordTaskBillingEventWithTx(nil, input)
}

func recordTaskBillingEventWithTx(tx *gorm.DB, input TaskBillingEventInput) error {
	if strings.TrimSpace(input.SourceId) == "" || input.AmountQuota <= 0 {
		return nil
	}
	if strings.TrimSpace(input.BillingSource) == "" {
		input.BillingSource = BillingSourceWallet
	}
	if strings.TrimSpace(input.EventType) == "" {
		input.EventType = model.BillingEventTypeDebit
	}
	if strings.TrimSpace(input.PriceUnit) == "" {
		input.PriceUnit = "task"
	}
	if input.Metadata == nil {
		input.Metadata = map[string]any{}
	}
	input.Metadata["source"] = model.BillingEventSourceAsyncTask
	metadataBytes, err := common.Marshal(input.Metadata)
	if err != nil {
		return err
	}
	if tx == nil {
		tx = model.DB
	}
	return tx.Transaction(func(tx *gorm.DB) error {
		if _, err := model.CreateTaskBillingRecordIfNotExists(tx, &model.TaskBillingRecord{
			SourceId:      input.SourceId,
			TaskId:        input.SourceId,
			Phase:         input.Phase,
			UserId:        input.UserId,
			TokenId:       input.TokenId,
			Group:         input.Group,
			BillingSource: input.BillingSource,
			PriceUnit:     input.PriceUnit,
			EventType:     input.EventType,
			AmountQuota:   input.AmountQuota,
			QuotaDelta:    input.QuotaDelta,
			RequestId:     input.RequestId,
			Metadata:      string(metadataBytes),
		}); err != nil {
			return err
		}
		_, err := model.CreateBillingEventIfNotExists(tx, taskBillingEventFromInput(input, string(metadataBytes), 0))
		return err
	})
}

func taskBillingEventFromInput(input TaskBillingEventInput, metadata string, createdAt int64) *model.BillingEvent {
	return &model.BillingEvent{
		EventId:       billingEventID(model.BillingEventSourceAsyncTask, input.SourceId, input.Phase),
		SubsiteId:     input.SubsiteId,
		UserId:        input.UserId,
		TokenId:       input.TokenId,
		Source:        model.BillingEventSourceAsyncTask,
		SourceId:      truncateBillingEventString(input.SourceId, 128),
		EventType:     input.EventType,
		Status:        model.BillingEventStatusSettled,
		RequestId:     truncateBillingEventString(input.RequestId, 128),
		Group:         input.Group,
		BillingSource: input.BillingSource,
		PriceUnit:     input.PriceUnit,
		Currency:      "quota",
		AmountQuota:   input.AmountQuota,
		QuotaDelta:    input.QuotaDelta,
		Cost:          billingEventCost(input.AmountQuota),
		Metadata:      metadata,
		CreatedAt:     createdAt,
	}
}

func taskBillingEventMetadata(task *model.Task, phase string, reason string) map[string]any {
	metadata := map[string]any{
		"phase":                 phase,
		"task_id":               task.TaskID,
		"upstream_task_id":      task.PrivateData.UpstreamTaskID,
		"platform":              task.Platform,
		"action":                task.Action,
		"status":                task.Status,
		"reason":                reason,
		"model_name":            taskModelName(task),
		"origin_model_name":     task.Properties.OriginModelName,
		"upstream_model_name":   task.Properties.UpstreamModelName,
		"channel_id":            task.ChannelId,
		"billing_source":        taskBillingSource(task),
		"subscription_id":       task.PrivateData.SubscriptionId,
		"current_task_quota":    task.Quota,
		"task_created_at":       task.CreatedAt,
		"task_updated_at":       task.UpdatedAt,
		"task_submit_time":      task.SubmitTime,
		"task_start_time":       task.StartTime,
		"task_finish_time":      task.FinishTime,
		"task_progress":         task.Progress,
		"task_private_token_id": task.PrivateData.TokenId,
	}
	if bc := task.PrivateData.BillingContext; bc != nil {
		metadata["model_price"] = bc.ModelPrice
		metadata["group_ratio"] = bc.GroupRatio
		metadata["model_ratio"] = bc.ModelRatio
		metadata["other_ratios"] = bc.OtherRatios
		metadata["per_call_billing"] = bc.PerCallBilling
	}
	return metadata
}

func taskBillingSource(task *model.Task) string {
	if task == nil {
		return BillingSourceWallet
	}
	billingSource := strings.TrimSpace(task.PrivateData.BillingSource)
	if billingSource == "" {
		return BillingSourceWallet
	}
	return billingSource
}
