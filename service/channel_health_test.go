package service

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func TestChannelHealthHardDisableOnlyForHardErrors(t *testing.T) {
	withAutomaticDisableForTest(t)

	invalidKeyErr := types.NewOpenAIError(errors.New("invalid api key"), types.ErrorCodeChannelInvalidKey, http.StatusUnauthorized)
	require.True(t, ShouldHardDisableChannel(invalidKeyErr))

	rateLimitErr := types.NewOpenAIError(errors.New("rate limited"), types.ErrorCodeBadResponseStatusCode, http.StatusTooManyRequests)
	require.False(t, ShouldHardDisableChannel(rateLimitErr))
	require.True(t, IsTransientChannelError(rateLimitErr))

	upstreamErr := types.NewOpenAIError(errors.New("upstream temporarily unavailable"), types.ErrorCodeBadResponseStatusCode, http.StatusBadGateway)
	require.False(t, ShouldHardDisableChannel(upstreamErr))
	require.True(t, IsTransientChannelError(upstreamErr))
}

func TestChannelHealthTransientFailuresOpenCooldown(t *testing.T) {
	withAutomaticDisableForTest(t)
	resetChannelHealthForTest()
	t.Cleanup(resetChannelHealthForTest)

	channelError := *types.NewChannelError(1001, 0, "temporary-channel", false, "", true)
	err := types.NewOpenAIError(errors.New("rate limited"), types.ErrorCodeBadResponseStatusCode, http.StatusTooManyRequests)

	decision := HandleChannelFailure(channelError, err)
	require.Equal(t, ChannelErrorActionRecordTransient, decision.Action)
	require.Equal(t, 1, decision.FailureCount)
	require.False(t, IsChannelTemporarilyUnavailable(channelError.ChannelId))

	decision = HandleChannelFailure(channelError, err)
	require.Equal(t, ChannelErrorActionRecordTransient, decision.Action)
	require.Equal(t, 2, decision.FailureCount)
	require.False(t, IsChannelTemporarilyUnavailable(channelError.ChannelId))

	decision = HandleChannelFailure(channelError, err)
	require.Equal(t, ChannelErrorActionRecordTransient, decision.Action)
	require.Equal(t, 3, decision.FailureCount)
	require.False(t, decision.CooldownUntil.IsZero())
	require.True(t, IsChannelTemporarilyUnavailable(channelError.ChannelId))

	RecordChannelSuccess(channelError.ChannelId)
	require.False(t, IsChannelTemporarilyUnavailable(channelError.ChannelId))
}

func TestChannelRuntimeHealthSnapshotAndClear(t *testing.T) {
	withAutomaticDisableForTest(t)
	resetChannelHealthForTest()
	t.Cleanup(resetChannelHealthForTest)

	channelError := *types.NewChannelError(1002, 0, "snapshot-channel", false, "", true)
	err := types.NewOpenAIError(errors.New("rate limited"), types.ErrorCodeBadResponseStatusCode, http.StatusTooManyRequests)

	HandleChannelFailure(channelError, err)
	HandleChannelFailure(channelError, err)
	HandleChannelFailure(channelError, err)

	snapshot := ChannelRuntimeHealthSnapshot(channelError.ChannelId)
	require.Equal(t, "temporarily_unavailable", snapshot.Status)
	require.True(t, snapshot.TemporarilyUnavailable)
	require.Equal(t, 3, snapshot.ConsecutiveFailures)
	require.Equal(t, http.StatusTooManyRequests, snapshot.LastStatusCode)
	require.NotZero(t, snapshot.CooldownUntil)
	require.Contains(t, snapshot.LastReason, "rate limited")

	require.True(t, ClearChannelTemporaryHealth(channelError.ChannelId))
	require.False(t, ClearChannelTemporaryHealth(channelError.ChannelId))

	snapshot = ChannelRuntimeHealthSnapshot(channelError.ChannelId)
	require.Equal(t, "healthy", snapshot.Status)
	require.False(t, snapshot.TemporarilyUnavailable)
}

func TestChannelRuntimeHealthSnapshotExpiresStaleDegradedState(t *testing.T) {
	withAutomaticDisableForTest(t)
	resetChannelHealthForTest()
	t.Cleanup(resetChannelHealthForTest)

	channelError := *types.NewChannelError(1003, 0, "stale-channel", false, "", true)
	err := types.NewOpenAIError(errors.New("temporary upstream outage"), types.ErrorCodeBadResponseStatusCode, http.StatusBadGateway)

	oldFailureTime := time.Now().Add(-10 * time.Minute)
	decision := recordTransientChannelFailure(channelError, err, oldFailureTime)
	require.Equal(t, ChannelErrorActionRecordTransient, decision.Action)
	require.Equal(t, 1, decision.FailureCount)

	snapshot := ChannelRuntimeHealthSnapshot(channelError.ChannelId)
	require.Equal(t, "healthy", snapshot.Status)
	require.False(t, snapshot.TemporarilyUnavailable)
	require.False(t, ClearChannelTemporaryHealth(channelError.ChannelId))
}

func TestChannelHealthUsesConfiguredTransientRules(t *testing.T) {
	withAutomaticDisableForTest(t)
	originalTransientRanges := append([]operation_setting.StatusCodeRange(nil), operation_setting.ChannelHealthTransientStatusCodeRanges...)
	originalTransientKeywords := append([]string(nil), operation_setting.ChannelHealthTransientKeywords...)
	t.Cleanup(func() {
		operation_setting.ChannelHealthTransientStatusCodeRanges = originalTransientRanges
		operation_setting.ChannelHealthTransientKeywords = originalTransientKeywords
	})

	operation_setting.ChannelHealthTransientStatusCodeRanges = []operation_setting.StatusCodeRange{{Start: http.StatusTooManyRequests, End: http.StatusTooManyRequests}}
	operation_setting.ChannelHealthTransientKeywords = []string{"temporarily overloaded"}

	badGatewayErr := types.NewOpenAIError(errors.New("bad gateway"), types.ErrorCodeBadResponseStatusCode, http.StatusBadGateway)
	require.False(t, IsTransientChannelError(badGatewayErr))

	rateLimitErr := types.NewOpenAIError(errors.New("rate limited"), types.ErrorCodeBadResponseStatusCode, http.StatusTooManyRequests)
	require.True(t, IsTransientChannelError(rateLimitErr))

	keywordErr := types.NewOpenAIError(errors.New("provider temporarily overloaded"), types.ErrorCodeBadResponseStatusCode, http.StatusBadRequest)
	require.True(t, IsTransientChannelError(keywordErr))
}

func withAutomaticDisableForTest(t *testing.T) {
	t.Helper()
	originalEnabled := common.AutomaticDisableChannelEnabled
	originalRanges := append([]operation_setting.StatusCodeRange(nil), operation_setting.AutomaticDisableStatusCodeRanges...)
	originalKeywords := append([]string(nil), operation_setting.AutomaticDisableKeywords...)
	originalThreshold := common.ChannelHealthFailureThreshold
	originalWindow := common.ChannelHealthFailureWindowMinutes
	originalCooldown := common.ChannelHealthCooldownMinutes
	originalMaxCooldown := common.ChannelHealthMaxCooldownMinutes

	common.AutomaticDisableChannelEnabled = true
	operation_setting.AutomaticDisableStatusCodeRanges = []operation_setting.StatusCodeRange{{Start: http.StatusUnauthorized, End: http.StatusUnauthorized}}
	operation_setting.AutomaticDisableKeywords = []string{"credit balance is too low", "permission denied"}
	common.ChannelHealthFailureThreshold = 3
	common.ChannelHealthFailureWindowMinutes = 5
	common.ChannelHealthCooldownMinutes = 2
	common.ChannelHealthMaxCooldownMinutes = 10

	t.Cleanup(func() {
		common.AutomaticDisableChannelEnabled = originalEnabled
		operation_setting.AutomaticDisableStatusCodeRanges = originalRanges
		operation_setting.AutomaticDisableKeywords = originalKeywords
		common.ChannelHealthFailureThreshold = originalThreshold
		common.ChannelHealthFailureWindowMinutes = originalWindow
		common.ChannelHealthCooldownMinutes = originalCooldown
		common.ChannelHealthMaxCooldownMinutes = originalMaxCooldown
	})
}
