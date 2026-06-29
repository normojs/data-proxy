package service

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChargeViolationFeeRecordsBillingEvent(t *testing.T) {
	truncate(t)
	gin.SetMode(gin.TestMode)

	const userID, tokenID, channelID = 50, 50, 50
	const initQuota, tokenRemain = 100000, 100000
	seedUser(t, userID, initQuota)
	seedToken(t, tokenID, userID, "sk-violation-fee", tokenRemain)
	seedChannel(t, channelID)

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Set(common.RequestIdKey, "violation-fee-request")
	ctx.Set("token_name", "violation-token")

	relayInfo := &relaycommon.RelayInfo{
		RequestId:       "violation-fee-request",
		UserId:          userID,
		SubsiteId:       44,
		TokenId:         tokenID,
		TokenKey:        "sk-violation-fee",
		UsingGroup:      "default",
		OriginModelName: "grok-test",
		RequestURLPath:  "/v1/chat/completions",
		StartTime:       time.Now(),
		PriceData: types.PriceData{
			GroupRatioInfo: types.GroupRatioInfo{GroupRatio: 1},
		},
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelId:   channelID,
			ChannelType: 1,
		},
	}
	apiErr := types.NewOpenAIError(
		errors.New(CSAMViolationMarker),
		types.ErrorCodeViolationFeeGrokCSAM,
		http.StatusBadRequest,
		types.ErrOptionWithSkipRetry(),
	)

	require.True(t, ChargeViolationFeeIfNeeded(ctx, relayInfo, apiErr))

	var events []model.BillingEvent
	require.NoError(t, model.DB.Where("source = ? AND request_id = ?", model.BillingEventSourceViolationFee, relayInfo.RequestId).Find(&events).Error)
	require.Len(t, events, 1)

	event := events[0]
	assert.Equal(t, model.BillingEventTypeDebit, event.EventType)
	assert.Equal(t, int64(44), event.SubsiteId)
	assert.Equal(t, "violation_fee", event.PriceUnit)
	assert.Equal(t, -event.AmountQuota, event.QuotaDelta)
	assert.Equal(t, BillingSourceWallet, event.BillingSource)
	assert.Equal(t, initQuota-event.AmountQuota, getUserQuota(t, userID))
	assert.Equal(t, tokenRemain-event.AmountQuota, getTokenRemainQuota(t, tokenID))

	metadata := decodeBillingEventMetadata(t, event)
	assert.Equal(t, true, metadata["violation_fee"])
	assert.Equal(t, string(types.ErrorCodeViolationFeeGrokCSAM), metadata["violation_fee_code"])
	assert.Equal(t, CSAMViolationMarker, metadata["violation_fee_marker"])
	assert.Equal(t, "grok-test", metadata["model_name"])

	var records []model.ViolationFeeRecord
	require.NoError(t, model.DB.Where("source_id = ? AND phase = ?", relayInfo.RequestId, string(types.ErrorCodeViolationFeeGrokCSAM)).Find(&records).Error)
	require.Len(t, records, 1)
	assert.Equal(t, relayInfo.RequestId, records[0].SourceId)
	assert.Equal(t, string(types.ErrorCodeViolationFeeGrokCSAM), records[0].Phase)
	assert.Equal(t, event.AmountQuota, records[0].AmountQuota)
	assert.Equal(t, event.QuotaDelta, records[0].QuotaDelta)

	require.NoError(t, RecordViolationFeeBillingEvent(relayInfo, ViolationFeeBillingEventInput{
		FeeQuota:      event.AmountQuota,
		BaseAmount:    0.05,
		GroupRatio:    1,
		StatusCode:    http.StatusBadRequest,
		UpstreamType:  string(types.ErrorCodeViolationFeeGrokCSAM),
		UpstreamCode:  string(types.ErrorCodeViolationFeeGrokCSAM),
		ViolationCode: types.ErrorCodeViolationFeeGrokCSAM,
		Marker:        CSAMViolationMarker,
	}))
	require.NoError(t, model.DB.Where("source = ? AND request_id = ?", model.BillingEventSourceViolationFee, relayInfo.RequestId).Find(&events).Error)
	require.Len(t, events, 1)
	require.NoError(t, model.DB.Where("source_id = ? AND phase = ?", relayInfo.RequestId, string(types.ErrorCodeViolationFeeGrokCSAM)).Find(&records).Error)
	require.Len(t, records, 1)
}
