package service

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
)

func TestRequestCapturePolicyDisabledByDefault(t *testing.T) {
	policy := LoadRequestCapturePolicyFromEnv()

	assert.False(t, policy.Enabled)
	assert.Equal(t, model.RequestCaptureLevelMetadata, policy.Level)
	assert.Equal(t, "disabled", policy.Decide(RequestCaptureDecisionInput{RequestId: "req-disabled"}).Reason)
}

func TestRequestCapturePolicyMatchesConfiguredFilters(t *testing.T) {
	t.Setenv("CAPTURE_ENABLED", "true")
	t.Setenv("CAPTURE_OBJECT_BACKEND", "s3")
	t.Setenv("CAPTURE_LEVEL", model.RequestCaptureLevelFullBundle)
	t.Setenv("CAPTURE_MODEL_PATTERNS", "deepseek-*,qwen-*")
	t.Setenv("CAPTURE_PATH_PREFIXES", "/v1/responses,/v1/chat")
	t.Setenv("CAPTURE_USER_IDS", "10,11")
	t.Setenv("CAPTURE_TOKEN_IDS", "20")
	t.Setenv("CAPTURE_CHANNEL_IDS", "30")
	t.Setenv("CAPTURE_CONNECTED_APP_IDS", "40")
	t.Setenv("CAPTURE_SAMPLE_RATE", "1")
	t.Setenv("CAPTURE_MAX_ARTIFACT_BYTES", "1048576")

	policy := LoadRequestCapturePolicyFromEnv()
	assert.Equal(t, int64(1048576), policy.MaxArtifactBytes)
	decision := policy.Decide(RequestCaptureDecisionInput{
		RequestId:      "req-capture-match",
		UserId:         10,
		TokenId:        20,
		ChannelId:      30,
		ConnectedAppId: 40,
		ModelName:      "deepseek-ai/DeepSeek-V4-Flash",
		RequestPath:    "/v1/responses",
		ProtocolChain:  "responses->chat",
		IsStream:       true,
	})

	assert.True(t, decision.Enabled)
	assert.Equal(t, model.RequestCaptureLevelFullBundle, decision.Level)
	assert.Equal(t, "matched", decision.Reason)
}

func TestRequestCapturePolicyRejectsUnmatchedFilters(t *testing.T) {
	policy := RequestCapturePolicy{
		Enabled:       true,
		Level:         model.RequestCaptureLevelSanitizedBundle,
		SampleRate:    1,
		ModelPatterns: []string{"qwen-*"},
		PathPrefixes:  []string{"/v1/chat"},
		UserIds:       map[int]struct{}{10: {}},
	}

	assert.Equal(t, "user_not_matched", policy.Decide(RequestCaptureDecisionInput{UserId: 9}).Reason)
	assert.Equal(t, "model_not_matched", policy.Decide(RequestCaptureDecisionInput{
		UserId:      10,
		ModelName:   "deepseek-ai/DeepSeek-V4-Flash",
		RequestPath: "/v1/chat/completions",
		RequestId:   "req-model",
	}).Reason)
	assert.Equal(t, "path_not_matched", policy.Decide(RequestCaptureDecisionInput{
		UserId:      10,
		ModelName:   "qwen-plus",
		RequestPath: "/v1/responses",
		RequestId:   "req-path",
	}).Reason)
}

func TestRequestCapturePolicySampleRateBounds(t *testing.T) {
	always := RequestCapturePolicy{Enabled: true, Level: model.RequestCaptureLevelMetadata, SampleRate: 1}
	assert.True(t, always.Decide(RequestCaptureDecisionInput{RequestId: "req"}).Enabled)

	never := RequestCapturePolicy{Enabled: true, Level: model.RequestCaptureLevelMetadata, SampleRate: 0}
	decision := never.Decide(RequestCaptureDecisionInput{RequestId: "req"})
	assert.False(t, decision.Enabled)
	assert.Equal(t, "sample_not_matched", decision.Reason)
}

func TestRequestCaptureWildcardMatch(t *testing.T) {
	assert.True(t, requestCaptureWildcardMatch("deepseek-*", "deepseek-ai/DeepSeek-V4-Flash"))
	assert.True(t, requestCaptureWildcardMatch("*flash", "deepseek-ai/deepseek-v4-flash"))
	assert.True(t, requestCaptureWildcardMatch("*v4*", "deepseek-ai/deepseek-v4-flash"))
	assert.False(t, requestCaptureWildcardMatch("qwen-*", "deepseek-ai/deepseek-v4-flash"))
}
