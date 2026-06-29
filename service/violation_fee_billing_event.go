package service

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"gorm.io/gorm"
)

type ViolationFeeBillingEventInput struct {
	FeeQuota      int
	BaseAmount    float64
	GroupRatio    float64
	StatusCode    int
	UpstreamType  string
	UpstreamCode  string
	ViolationCode types.ErrorCode
	Marker        string
}

func RecordViolationFeeBillingEvent(relayInfo *relaycommon.RelayInfo, input ViolationFeeBillingEventInput) error {
	if relayInfo == nil || input.FeeQuota <= 0 {
		return nil
	}
	requestId := strings.TrimSpace(relayInfo.RequestId)
	if requestId == "" {
		requestId = fmt.Sprintf("user:%d:token:%d:start:%d:violation", relayInfo.UserId, relayInfo.TokenId, relayInfo.StartTime.UnixNano())
	}
	billingSource := strings.TrimSpace(relayInfo.BillingSource)
	if billingSource == "" {
		billingSource = BillingSourceWallet
	}
	phase := strings.TrimSpace(string(input.ViolationCode))
	if phase == "" {
		phase = "violation"
	}
	metadata := map[string]any{
		"violation_fee":        true,
		"violation_fee_code":   phase,
		"fee_quota":            input.FeeQuota,
		"base_amount":          input.BaseAmount,
		"group_ratio":          input.GroupRatio,
		"status_code":          input.StatusCode,
		"upstream_error_type":  input.UpstreamType,
		"upstream_error_code":  input.UpstreamCode,
		"violation_fee_marker": input.Marker,
		"model_name":           relayInfo.OriginModelName,
		"upstream_model_name":  upstreamModelName(relayInfo),
		"channel_id":           billingEventChannelId(relayInfo),
		"channel_type":         billingEventChannelType(relayInfo),
		"request_url_path":     relayInfo.RequestURLPath,
		"billing_source":       billingSource,
		"subscription_id":      relayInfo.SubscriptionId,
	}
	metadataBytes, err := common.Marshal(metadata)
	if err != nil {
		return err
	}
	createdAt := common.GetTimestamp()
	return model.DB.Transaction(func(tx *gorm.DB) error {
		if _, err := model.CreateViolationFeeRecordIfNotExists(tx, &model.ViolationFeeRecord{
			SourceId:      requestId,
			Phase:         phase,
			UserId:        relayInfo.UserId,
			TokenId:       relayInfo.TokenId,
			Group:         relayInfo.UsingGroup,
			BillingSource: billingSource,
			PriceUnit:     "violation_fee",
			EventType:     model.BillingEventTypeDebit,
			AmountQuota:   input.FeeQuota,
			QuotaDelta:    -input.FeeQuota,
			RequestId:     requestId,
			Metadata:      string(metadataBytes),
			CreatedAt:     createdAt,
		}); err != nil {
			return err
		}
		_, err := model.CreateBillingEventIfNotExists(tx, &model.BillingEvent{
			EventId:       billingEventID(model.BillingEventSourceViolationFee, requestId, phase),
			SubsiteId:     relayInfo.SubsiteId,
			UserId:        relayInfo.UserId,
			TokenId:       relayInfo.TokenId,
			Source:        model.BillingEventSourceViolationFee,
			SourceId:      truncateBillingEventString(requestId, 128),
			EventType:     model.BillingEventTypeDebit,
			Status:        model.BillingEventStatusSettled,
			RequestId:     truncateBillingEventString(requestId, 128),
			Group:         relayInfo.UsingGroup,
			BillingSource: billingSource,
			PriceUnit:     "violation_fee",
			Currency:      "quota",
			AmountQuota:   input.FeeQuota,
			QuotaDelta:    -input.FeeQuota,
			Cost:          billingEventCost(input.FeeQuota),
			Metadata:      string(metadataBytes),
			CreatedAt:     createdAt,
		})
		return err
	})
}
