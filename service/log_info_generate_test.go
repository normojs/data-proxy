package service

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGenerateTextOtherInfoIncludesStreamFailureClassification(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	start := time.Now().Add(-2 * time.Second)
	streamStatus := relaycommon.NewStreamStatus()
	streamStatus.RecordError("decode failed")
	streamStatus.SetEndReason(relaycommon.StreamEndReasonHandlerStop, fmt.Errorf("handler stopped"))
	relayInfo := &relaycommon.RelayInfo{
		StartTime:             start,
		FirstResponseTime:     start.Add(200 * time.Millisecond),
		IsStream:              true,
		StreamStatus:          streamStatus,
		ReceivedResponseCount: 3,
		ChannelMeta:           &relaycommon.ChannelMeta{},
	}

	other := GenerateTextOtherInfo(ctx, relayInfo, 1, 1, 1, 0, 0, 0, 1)

	streamInfo, ok := other["stream_status"].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, "error", streamInfo["status"])
	require.Equal(t, "handler_stop", streamInfo["end_reason"])
	require.Equal(t, "stream_handler_error", streamInfo["failure_category"])
	require.Equal(t, "proxy", streamInfo["failure_source"])
	require.Equal(t, "after_first_response", streamInfo["failure_stage"])
	require.Equal(t, false, streamInfo["channel_failure_candidate"])
	require.Equal(t, true, streamInfo["has_first_response"])
	require.Equal(t, 3, streamInfo["received_response_count"])
	require.Equal(t, 1, streamInfo["error_count"])
	require.Equal(t, []string{"decode failed"}, streamInfo["errors"])
	require.Equal(t, "handler stopped", streamInfo["end_error"])
}

func TestGenerateTextOtherInfoIncludesMappedStreamError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	start := time.Now().Add(-2 * time.Second)
	streamStatus := relaycommon.NewStreamStatus()
	streamStatus.RecordError("upstream_key_sleeping: 上游公益 token 睡眠中")
	streamStatus.SetMappedError(429, "upstream_key_sleeping", "上游公益 token 睡眠中", "公益 token 睡眠", true)
	streamStatus.SetEndReason(relaycommon.StreamEndReasonMappedError, fmt.Errorf("上游公益 token 睡眠中"))
	relayInfo := &relaycommon.RelayInfo{
		StartTime:             start,
		FirstResponseTime:     start.Add(-time.Second),
		IsStream:              true,
		StreamStatus:          streamStatus,
		ReceivedResponseCount: 0,
		ChannelMeta:           &relaycommon.ChannelMeta{},
	}

	other := GenerateTextOtherInfo(ctx, relayInfo, 1, 1, 1, 0, 0, 0, 1)

	streamInfo, ok := other["stream_status"].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, "error", streamInfo["status"])
	require.Equal(t, "mapped_error", streamInfo["end_reason"])
	require.Equal(t, "upstream_mapped_error", streamInfo["failure_category"])
	require.Equal(t, "upstream", streamInfo["failure_source"])
	require.Equal(t, "before_first_response", streamInfo["failure_stage"])
	require.Equal(t, true, streamInfo["channel_failure_candidate"])
	require.Equal(t, false, streamInfo["has_first_response"])
	require.Equal(t, "upstream_key_sleeping", streamInfo["mapped_error_code"])
	require.Equal(t, 429, streamInfo["mapped_status_code"])
	require.Equal(t, "上游公益 token 睡眠中", streamInfo["mapped_message"])
	require.Equal(t, "公益 token 睡眠", streamInfo["mapped_rule"])
}
