package controller

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
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

func TestShouldRetryUsesTransientFailureRulesForFailover(t *testing.T) {
	originalRetryRanges := append([]operation_setting.StatusCodeRange(nil), operation_setting.AutomaticRetryStatusCodeRanges...)
	originalTransientRanges := append([]operation_setting.StatusCodeRange(nil), operation_setting.ChannelHealthTransientStatusCodeRanges...)
	originalTransientKeywords := append([]string(nil), operation_setting.ChannelHealthTransientKeywords...)
	t.Cleanup(func() {
		operation_setting.AutomaticRetryStatusCodeRanges = originalRetryRanges
		operation_setting.ChannelHealthTransientStatusCodeRanges = originalTransientRanges
		operation_setting.ChannelHealthTransientKeywords = originalTransientKeywords
	})

	operation_setting.AutomaticRetryStatusCodeRanges = []operation_setting.StatusCodeRange{
		{Start: http.StatusTooManyRequests, End: http.StatusTooManyRequests},
	}
	operation_setting.ChannelHealthTransientStatusCodeRanges = []operation_setting.StatusCodeRange{
		{Start: http.StatusRequestTimeout, End: http.StatusRequestTimeout},
	}
	operation_setting.ChannelHealthTransientKeywords = []string{"connection reset"}

	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())

	requestTimeoutErr := types.NewOpenAIError(errors.New("provider request timeout"), types.ErrorCodeBadResponseStatusCode, http.StatusRequestTimeout)
	require.True(t, shouldRetry(ctx, requestTimeoutErr, 1))

	connectionResetErr := types.NewOpenAIError(errors.New("connection reset by peer"), types.ErrorCodeDoRequestFailed, 0)
	require.True(t, shouldRetry(ctx, connectionResetErr, 1))

	require.False(t, shouldRetry(ctx, requestTimeoutErr, 0))

	ctx.Set("specific_channel_id", 42)
	require.False(t, shouldRetry(ctx, requestTimeoutErr, 1))
}

func TestProcessChannelErrorRecordsStreamStatusInErrorLog(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Log{}, &model.User{}))

	originalDB := model.DB
	originalLogDB := model.LOG_DB
	model.DB = db
	model.LOG_DB = db
	t.Cleanup(func() {
		model.DB = originalDB
		model.LOG_DB = originalLogDB
	})

	originalErrorLogEnabled := constant.ErrorLogEnabled
	originalAutomaticDisable := common.AutomaticDisableChannelEnabled
	originalRedisEnabled := common.RedisEnabled
	constant.ErrorLogEnabled = true
	common.AutomaticDisableChannelEnabled = false
	common.RedisEnabled = false
	t.Cleanup(func() {
		constant.ErrorLogEnabled = originalErrorLogEnabled
		common.AutomaticDisableChannelEnabled = originalAutomaticDisable
		common.RedisEnabled = originalRedisEnabled
	})

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Set(common.RequestIdKey, "req-stream-mapped-error")
	ctx.Set("id", 7)
	ctx.Set("username", "alice")
	ctx.Set("token_name", "admin-smoke")
	ctx.Set("token_id", 21)
	ctx.Set("group", "GPT免费")
	ctx.Set("original_model", "gpt-5.5")
	ctx.Set("channel_id", 12)
	ctx.Set("channel_name", "GTP免费2")
	ctx.Set("channel_type", 1)
	ctx.Set("use_channel", []string{"12"})
	common.SetContextKey(ctx, constant.ContextKeyIsStream, true)

	apiErr := types.WithOpenAIError(types.OpenAIError{
		Message: "上游公益 token 睡眠中，请稍后重试或切换 key",
		Type:    "upstream_stream_error",
		Code:    "upstream_key_sleeping",
	}, http.StatusServiceUnavailable)

	streamStatus := relaycommon.NewStreamStatus()
	streamStatus.RecordError("upstream_key_sleeping: 上游公益 token 睡眠中，请稍后重试或切换 key")
	streamStatus.SetMappedError(http.StatusTooManyRequests, "upstream_key_sleeping", "上游公益 token 睡眠中，请稍后重试或切换 key", "公益 token 睡眠", true)
	streamStatus.SetEndReason(relaycommon.StreamEndReasonMappedError, apiErr)
	relayInfo := &relaycommon.RelayInfo{
		IsStream:     true,
		StreamStatus: streamStatus,
	}

	processChannelError(ctx, *types.NewChannelError(12, 1, "GTP免费2", false, "", false), apiErr, relayInfo)

	var log model.Log
	require.NoError(t, model.LOG_DB.First(&log).Error)
	require.Equal(t, model.LogTypeError, log.Type)
	require.Equal(t, "req-stream-mapped-error", log.RequestId)
	require.True(t, log.IsStream)

	var other map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(log.Other), &other))
	require.Equal(t, "upstream_key_sleeping", other["error_code"])
	require.Equal(t, float64(http.StatusServiceUnavailable), other["status_code"])

	streamInfo, ok := other["stream_status"].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, "error", streamInfo["status"])
	require.Equal(t, "mapped_error", streamInfo["end_reason"])
	require.Equal(t, "upstream_mapped_error", streamInfo["failure_category"])
	require.Equal(t, "upstream", streamInfo["failure_source"])
	require.Equal(t, "before_first_response", streamInfo["failure_stage"])
	require.Equal(t, false, streamInfo["has_first_response"])
	require.Equal(t, "upstream_key_sleeping", streamInfo["mapped_error_code"])
	require.Equal(t, float64(http.StatusTooManyRequests), streamInfo["mapped_status_code"])
	require.Equal(t, "公益 token 睡眠", streamInfo["mapped_rule"])
}
