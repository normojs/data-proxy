package controller

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestChannelFailoverTraceRecordsSelectionFailureAndAdminInfo(t *testing.T) {
	originalRetryTimes := common.RetryTimes
	common.RetryTimes = 1
	t.Cleanup(func() {
		common.RetryTimes = originalRetryTimes
	})

	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())

	retry := 0
	autoBan := 1
	retryParam := &service.RetryParam{
		Ctx:               ctx,
		TokenGroup:        "default",
		ModelName:         "deepseek-ai/DeepSeek-V4-Flash",
		Retry:             &retry,
		ExcludeChannelIds: map[int]bool{9: true},
	}
	relayInfo := &relaycommon.RelayInfo{
		TokenGroup:      "default",
		UsingGroup:      "default",
		OriginModelName: "deepseek-ai/DeepSeek-V4-Flash",
	}
	channel := &model.Channel{
		Id:      42,
		Name:    "primary",
		Type:    1,
		AutoBan: &autoBan,
	}

	recordChannelFailoverSelection(ctx, relayInfo, retryParam, channel, "default")
	updateChannelFailoverRetryDecision(ctx, retryParam, true)
	recordChannelFailoverFailure(
		ctx,
		*types.NewChannelError(channel.Id, channel.Type, channel.Name, false, "", channel.GetAutoBan()),
		types.NewOpenAIError(errors.New("upstream returned 502"), types.ErrorCodeBadResponse, http.StatusBadGateway),
		service.ChannelHealthDecision{
			Action:       service.ChannelErrorActionRecordTransient,
			FailureCount: 1,
			Reason:       "upstream returned 502",
		},
	)

	events, ok := common.GetContextKeyType[[]map[string]interface{}](ctx, constant.ContextKeyChannelFailoverTrace)
	require.True(t, ok)
	require.Len(t, events, 2)

	require.Equal(t, "selected", events[0]["event"])
	require.Equal(t, 42, events[0]["channel_id"])
	require.Equal(t, "default", events[0]["selected_group"])
	require.Equal(t, []int{9}, events[0]["excluded_channel_ids"])

	require.Equal(t, "failed", events[1]["event"])
	require.Equal(t, "bad_response", events[1]["error_code"])
	require.Equal(t, http.StatusBadGateway, events[1]["status_code"])
	require.Equal(t, true, events[1]["retry_planned"])
	require.Equal(t, "record_transient", events[1]["health_action"])
	require.Equal(t, 1, events[1]["health_failure_count"])

	adminInfo := map[string]interface{}{}
	service.AppendChannelFailoverAdminInfo(ctx, adminInfo)
	require.Equal(t, events, adminInfo["channel_failover"])
}

func TestChannelFailoverTraceKeepsNoRetryDecision(t *testing.T) {
	originalRetryTimes := common.RetryTimes
	common.RetryTimes = 0
	t.Cleanup(func() {
		common.RetryTimes = originalRetryTimes
	})

	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())

	retry := 0
	autoBan := 1
	retryParam := &service.RetryParam{
		Ctx:        ctx,
		TokenGroup: "default",
		ModelName:  "deepseek-ai/DeepSeek-V4-Flash",
		Retry:      &retry,
	}
	relayInfo := &relaycommon.RelayInfo{
		TokenGroup:      "default",
		UsingGroup:      "default",
		OriginModelName: "deepseek-ai/DeepSeek-V4-Flash",
	}
	channel := &model.Channel{
		Id:      42,
		Name:    "primary",
		Type:    1,
		AutoBan: &autoBan,
	}

	recordChannelFailoverSelection(ctx, relayInfo, retryParam, channel, "default")
	updateChannelFailoverRetryDecision(ctx, retryParam, false)
	recordChannelFailoverFailure(
		ctx,
		*types.NewChannelError(channel.Id, channel.Type, channel.Name, false, "", channel.GetAutoBan()),
		types.NewOpenAIError(errors.New("upstream returned 502"), types.ErrorCodeBadResponse, http.StatusBadGateway),
		service.ChannelHealthDecision{Action: service.ChannelErrorActionRecordTransient},
	)

	events, ok := common.GetContextKeyType[[]map[string]interface{}](ctx, constant.ContextKeyChannelFailoverTrace)
	require.True(t, ok)
	require.Len(t, events, 2)
	require.Equal(t, false, events[1]["retry_planned"])
	require.Equal(t, "failed", events[1]["event"])
}
