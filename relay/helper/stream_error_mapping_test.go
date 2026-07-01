package helper

import (
	"net/http"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func boolPtr(value bool) *bool {
	return &value
}

func streamErrorMappingInfo(rules ...dto.StreamErrorMappingRule) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelOtherSettings: dto.ChannelOtherSettings{
				StreamErrorMapping: rules,
			},
		},
	}
}

func TestMatchStreamErrorMapping_TextCandidatesFromJSON(t *testing.T) {
	t.Parallel()

	info := streamErrorMappingInfo(dto.StreamErrorMappingRule{
		Name:          "sleeping-key",
		Target:        "text",
		Operator:      "contains",
		Pattern:       "公益token睡眠中",
		StatusCode:    http.StatusTooManyRequests,
		ErrorCode:     "upstream_key_sleeping",
		Message:       "上游公益 token 睡眠中",
		Retryable:     boolPtr(true),
		MaxChunks:     3,
		CaseSensitive: false,
	})

	match, ok := matchStreamErrorMapping(
		`{"choices":[{"delta":{"content":"公益token睡眠中 https://dc.hhhl.cc/chat/room/amlc1bekzi"}}]}`,
		info,
		1,
	)

	require.True(t, ok)
	require.Equal(t, "sleeping-key", match.Rule.Name)
	require.Equal(t, http.StatusTooManyRequests, match.StatusCode)
	require.Equal(t, "upstream_key_sleeping", match.ErrorCode)
	require.Equal(t, "上游公益 token 睡眠中", match.Message)
	require.True(t, match.Retryable)
}

func TestMatchStreamErrorMapping_TextTargetFallsBackToPlainChunk(t *testing.T) {
	t.Parallel()

	info := streamErrorMappingInfo(dto.StreamErrorMappingRule{
		Target:   "text",
		Operator: "contains",
		Pattern:  "公益token睡眠中",
	})

	_, ok := matchStreamErrorMapping(
		"公益token睡眠中 https://dc.hhhl.cc/chat/room/amlc1bekzi",
		info,
		1,
	)

	require.True(t, ok)
}

func TestMatchStreamErrorMapping_OperatorsAndCaseSensitivity(t *testing.T) {
	t.Parallel()

	require.True(t, matchStreamErrorMappingText("Quota Sleeping", "quota sleeping", "equals", false))
	require.False(t, matchStreamErrorMappingText("Quota Sleeping", "quota sleeping", "equals", true))
	require.True(t, matchStreamErrorMappingText("prefix-value", "prefix", "starts_with", false))
	require.True(t, matchStreamErrorMappingText("value-suffix", "suffix", "ends_with", false))
	require.True(t, matchStreamErrorMappingText("KEY sleeping for 10m", `key\s+sleeping`, "regex", false))
	require.False(t, matchStreamErrorMappingText("KEY sleeping for 10m", `[`, "regex", false))
}

func TestMatchStreamErrorMapping_MaxChunksAndDisabledRules(t *testing.T) {
	t.Parallel()

	info := streamErrorMappingInfo(
		dto.StreamErrorMappingRule{
			Enabled:  boolPtr(false),
			Pattern:  "disabled-match",
			Operator: "contains",
		},
		dto.StreamErrorMappingRule{
			Pattern:   "late-match",
			Operator:  "contains",
			MaxChunks: 1,
		},
	)

	_, ok := matchStreamErrorMapping("disabled-match", info, 1)
	require.False(t, ok)

	_, ok = matchStreamErrorMapping("late-match", info, 2)
	require.False(t, ok)

	_, ok = matchStreamErrorMapping("late-match", info, 1)
	require.True(t, ok)
}

func TestMatchStreamErrorMapping_MaxContentChars(t *testing.T) {
	t.Parallel()

	info := streamErrorMappingInfo(dto.StreamErrorMappingRule{
		Target:          "text",
		Operator:        "contains",
		Pattern:         "公益token睡眠中",
		MaxContentChars: 8,
	})

	_, ok := matchStreamErrorMapping(`{"choices":[{"delta":{"content":"公益token睡眠中"}}]}`, info, 1)
	require.False(t, ok)

	info.ChannelOtherSettings.StreamErrorMapping[0].MaxContentChars = 20
	_, ok = matchStreamErrorMapping(`{"choices":[{"delta":{"content":"公益token睡眠中"}}]}`, info, 1)
	require.True(t, ok)
}

func TestMatchStreamErrorMapping_MaxRawChars(t *testing.T) {
	t.Parallel()

	info := streamErrorMappingInfo(dto.StreamErrorMappingRule{
		Target:      "raw",
		Operator:    "contains",
		Pattern:     "公益token睡眠中",
		MaxRawChars: 8,
	})

	_, ok := matchStreamErrorMapping(`{"error":"公益token睡眠中"}`, info, 1)
	require.False(t, ok)

	info.ChannelOtherSettings.StreamErrorMapping[0].MaxRawChars = 30
	_, ok = matchStreamErrorMapping(`{"error":"公益token睡眠中"}`, info, 1)
	require.True(t, ok)
}

func TestStreamErrorMappingPreFlushConfig_DefaultTimeout(t *testing.T) {
	t.Parallel()

	info := streamErrorMappingInfo(dto.StreamErrorMappingRule{
		Target:            "text",
		Operator:          "contains",
		Pattern:           "公益token睡眠中",
		PreFlushMaxChunks: 3,
		PreFlushTimeoutMs: 0,
	})

	config := streamErrorMappingPreFlushConfigFromInfo(info)
	require.True(t, config.Enabled)
	require.Equal(t, 3, config.MaxChunks)
	require.Equal(t, 60*time.Second, config.Timeout)
}

func TestBuildStreamErrorMappingMatch_DefaultsAndSkipRetry(t *testing.T) {
	t.Parallel()

	match := buildStreamErrorMappingMatch(dto.StreamErrorMappingRule{
		Pattern:                 "sleep",
		StatusCode:              999,
		Retryable:               boolPtr(false),
		ChannelFailureCandidate: boolPtr(false),
	})

	require.Equal(t, defaultStreamErrorMappingStatusCode, match.StatusCode)
	require.Equal(t, defaultStreamErrorMappingCode, match.ErrorCode)
	require.Equal(t, defaultStreamErrorMappingMessage, match.Message)
	require.False(t, match.Retryable)
	require.False(t, match.ChannelFailureCandidate)

	apiErr := match.NewAPIError()
	require.NotNil(t, apiErr)
	require.Equal(t, defaultStreamErrorMappingStatusCode, apiErr.StatusCode)
	require.Equal(t, types.ErrorCode(defaultStreamErrorMappingCode), apiErr.GetErrorCode())
	require.True(t, types.IsSkipRetryError(apiErr))
}
