package model

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/bytedance/gopkg/util/gopool"
	"gorm.io/gorm"
)

type FundingBillingEventInput struct {
	Source        string
	SourceId      string
	Phase         string
	UserId        int
	TokenId       int
	RequestId     string
	Group         string
	BillingSource string
	PriceUnit     string
	EventType     string
	AmountQuota   int
	Cost          *float64
	AllowZero     bool
	CreatedAt     int64
	Metadata      map[string]any
}

func RecordFundingBillingEvent(tx *gorm.DB, input FundingBillingEventInput) error {
	_, err := RecordFundingBillingEventIfNotExists(tx, input)
	return err
}

func RecordFundingBillingEventIfNotExists(tx *gorm.DB, input FundingBillingEventInput) (bool, error) {
	if input.AmountQuota < 0 || (!input.AllowZero && input.AmountQuota == 0) || strings.TrimSpace(input.Source) == "" || strings.TrimSpace(input.SourceId) == "" {
		return false, nil
	}
	if strings.TrimSpace(input.Phase) == "" {
		input.Phase = "settlement"
	}
	if strings.TrimSpace(input.EventType) == "" {
		input.EventType = BillingEventTypeCredit
	}
	if strings.TrimSpace(input.BillingSource) == "" {
		input.BillingSource = "wallet"
	}
	if strings.TrimSpace(input.PriceUnit) == "" {
		input.PriceUnit = "quota"
	}
	if input.Metadata == nil {
		input.Metadata = map[string]any{}
	}
	input.Metadata["source"] = input.Source
	input.Metadata["phase"] = input.Phase

	quotaDelta := input.AmountQuota
	if input.EventType == BillingEventTypeDebit {
		quotaDelta = -input.AmountQuota
	}
	cost := modelBillingEventCost(input.AmountQuota)
	if input.Cost != nil {
		cost = *input.Cost
	}

	metadataBytes, err := common.Marshal(input.Metadata)
	if err != nil {
		return false, err
	}
	createdAt := input.CreatedAt
	if createdAt == 0 {
		createdAt = common.GetTimestamp()
	}
	return CreateBillingEventIfNotExists(tx, &BillingEvent{
		EventId:       modelBillingEventID(input.Source, input.SourceId, input.Phase),
		UserId:        input.UserId,
		TokenId:       input.TokenId,
		Source:        input.Source,
		SourceId:      modelBillingEventSourceId(input.SourceId),
		EventType:     input.EventType,
		Status:        BillingEventStatusSettled,
		RequestId:     modelBillingEventSourceId(input.RequestId),
		Group:         input.Group,
		BillingSource: input.BillingSource,
		PriceUnit:     input.PriceUnit,
		Currency:      "quota",
		AmountQuota:   input.AmountQuota,
		QuotaDelta:    quotaDelta,
		Cost:          cost,
		Metadata:      string(metadataBytes),
		CreatedAt:     createdAt,
	})
}

func RecordFundingDebitEventAndDecreaseUserQuotaIfNotExists(input FundingBillingEventInput) (bool, error) {
	if input.EventType == "" {
		input.EventType = BillingEventTypeDebit
	}
	if input.EventType != BillingEventTypeDebit || input.AmountQuota <= 0 || input.UserId <= 0 {
		return RecordFundingBillingEventIfNotExists(nil, input)
	}
	created := false
	err := DB.Transaction(func(tx *gorm.DB) error {
		nextCreated, err := RecordFundingBillingEventIfNotExists(tx, input)
		if err != nil {
			return err
		}
		if !nextCreated {
			return nil
		}
		result := tx.Model(&User{}).Where("id = ?", input.UserId).Updates(map[string]any{
			"quota":         gorm.Expr("quota - ?", input.AmountQuota),
			"used_quota":    gorm.Expr("used_quota + ?", input.AmountQuota),
			"request_count": gorm.Expr("request_count + ?", 1),
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		created = true
		return nil
	})
	if err != nil {
		return false, err
	}
	if created {
		gopool.Go(func() {
			if err := cacheDecrUserQuota(input.UserId, int64(input.AmountQuota)); err != nil {
				common.SysLog("failed to decrease user quota after billing event settlement: " + err.Error())
			}
		})
	}
	return created, nil
}

func FundingBillingEventExists(tx *gorm.DB, source string, sourceId string, phase string) (bool, error) {
	if tx == nil {
		tx = DB
	}
	if strings.TrimSpace(source) == "" || strings.TrimSpace(sourceId) == "" {
		return false, nil
	}
	if strings.TrimSpace(phase) == "" {
		phase = "settlement"
	}
	var count int64
	err := tx.Model(&BillingEvent{}).
		Where("event_id = ?", modelBillingEventID(source, sourceId, phase)).
		Count(&count).Error
	return count > 0, err
}

func FundingBillingEventSourceExists(tx *gorm.DB, source string, sourceId string) (bool, error) {
	if tx == nil {
		tx = DB
	}
	if strings.TrimSpace(source) == "" || strings.TrimSpace(sourceId) == "" {
		return false, nil
	}
	var count int64
	err := tx.Model(&BillingEvent{}).
		Where("source = ? AND source_id = ?", strings.TrimSpace(source), modelBillingEventSourceId(sourceId)).
		Count(&count).Error
	return count > 0, err
}

func GetFundingBillingEvent(tx *gorm.DB, source string, sourceId string, phase string) (BillingEvent, bool, error) {
	if tx == nil {
		tx = DB
	}
	if strings.TrimSpace(source) == "" || strings.TrimSpace(sourceId) == "" {
		return BillingEvent{}, false, nil
	}
	if strings.TrimSpace(phase) == "" {
		phase = "settlement"
	}
	var event BillingEvent
	err := tx.Where("event_id = ?", modelBillingEventID(source, sourceId, phase)).First(&event).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return BillingEvent{}, false, nil
	}
	return event, err == nil, err
}

func ListFundingBillingEventsBySource(tx *gorm.DB, source string, sourceId string, limit int) ([]BillingEvent, error) {
	if tx == nil {
		tx = DB
	}
	if strings.TrimSpace(source) == "" || strings.TrimSpace(sourceId) == "" {
		return []BillingEvent{}, nil
	}
	if limit <= 0 {
		limit = 20
	}
	var events []BillingEvent
	err := tx.Where("source = ? AND source_id = ?", strings.TrimSpace(source), modelBillingEventSourceId(sourceId)).
		Order("id asc").
		Limit(limit).
		Find(&events).Error
	return events, err
}

func RecordWalletTopUpBillingEvent(tx *gorm.DB, topUp *TopUp, quota int, phase string, metadata map[string]any) error {
	if topUp == nil || quota <= 0 {
		return nil
	}
	if strings.TrimSpace(phase) == "" {
		phase = "success"
	}
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadata["trade_no"] = topUp.TradeNo
	metadata["payment_method"] = topUp.PaymentMethod
	metadata["payment_provider"] = topUp.PaymentProvider
	metadata["money"] = topUp.Money
	metadata["amount"] = topUp.Amount
	metadata["status"] = topUp.Status
	metadata["complete_time"] = topUp.CompleteTime
	return RecordFundingBillingEvent(tx, FundingBillingEventInput{
		Source:        BillingEventSourceWalletTopUp,
		SourceId:      topUp.TradeNo,
		Phase:         phase,
		UserId:        topUp.UserId,
		RequestId:     topUp.TradeNo,
		BillingSource: "wallet",
		PriceUnit:     "topup",
		EventType:     BillingEventTypeCredit,
		AmountQuota:   quota,
		Metadata:      metadata,
	})
}

func RecordWalletAdjustBillingEvent(userId int, sourceId string, eventType string, quota int, metadata map[string]any) error {
	return RecordWalletAdjustBillingEventTx(nil, userId, sourceId, eventType, quota, metadata)
}

func RecordWalletAdjustBillingEventTx(tx *gorm.DB, userId int, sourceId string, eventType string, quota int, metadata map[string]any) error {
	if sourceId == "" {
		sourceId = fmt.Sprintf("user:%d:%d", userId, time.Now().UnixNano())
	}
	return RecordFundingBillingEvent(tx, FundingBillingEventInput{
		Source:        BillingEventSourceWalletAdjust,
		SourceId:      sourceId,
		Phase:         "adjust",
		UserId:        userId,
		RequestId:     sourceId,
		BillingSource: "wallet",
		PriceUnit:     "manual_adjust",
		EventType:     eventType,
		AmountQuota:   quota,
		Metadata:      metadata,
	})
}

func RecordSubscriptionBillingEvent(tx *gorm.DB, sourceId string, phase string, userId int, amountQuota int, eventType string, metadata map[string]any) error {
	_, err := RecordSubscriptionBillingEventIfNotExists(tx, sourceId, phase, userId, amountQuota, eventType, metadata)
	return err
}

func RecordSubscriptionBillingEventIfNotExists(tx *gorm.DB, sourceId string, phase string, userId int, amountQuota int, eventType string, metadata map[string]any) (bool, error) {
	return RecordSubscriptionBillingEventWithCreatedAt(tx, sourceId, phase, userId, amountQuota, eventType, 0, metadata)
}

func RecordSubscriptionBillingEventWithCreatedAt(tx *gorm.DB, sourceId string, phase string, userId int, amountQuota int, eventType string, createdAt int64, metadata map[string]any) (bool, error) {
	return RecordFundingBillingEventIfNotExists(tx, FundingBillingEventInput{
		Source:        BillingEventSourceSubscription,
		SourceId:      sourceId,
		Phase:         phase,
		UserId:        userId,
		RequestId:     sourceId,
		BillingSource: "subscription",
		PriceUnit:     "subscription",
		EventType:     eventType,
		AmountQuota:   amountQuota,
		AllowZero:     true,
		CreatedAt:     createdAt,
		Metadata:      metadata,
	})
}
