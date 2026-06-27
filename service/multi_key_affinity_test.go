package service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func resetMultiKeyAffinityTestState() {
	multiKeyAffinityBindingCacheOnce = sync.Once{}
	multiKeyAffinityBindingCache = nil
	multiKeyLoadMu.Lock()
	multiKeyLoadCounters = map[string]*multiKeyLoadCounter{}
	multiKeyLoadMu.Unlock()
}

func buildMultiKeyAffinityTestChannel() *model.Channel {
	return &model.Channel{
		Id: 321,
		Key: strings.Join([]string{
			"sub2api-key-a",
			"sub2api-key-b",
			"sub2api-key-c",
		}, "\n"),
		ChannelInfo: model.ChannelInfo{
			IsMultiKey:   true,
			MultiKeySize: 3,
			MultiKeyMode: constant.MultiKeyModeStickyHashBounded,
			MultiKeyAffinityPolicy: model.MultiKeyAffinityPolicy{
				Enabled:                           true,
				BindingTTLSeconds:                 3600,
				MoveCooldownSeconds:               900,
				SoftLoadFactor:                    1.25,
				HardLoadFactor:                    1.8,
				ExistingBindingStayOnSoftOverload: common.GetPointer(true),
			},
		},
	}
}

func buildMultiKeyAffinityTestContext(seed string) *gin.Context {
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(fmt.Sprintf(`{"prompt_cache_key":%q}`, seed)))
	ctx.Request.Header.Set("Content-Type", "application/json")
	common.SetContextKey(ctx, constant.ContextKeyTokenId, 9527)
	common.SetContextKey(ctx, constant.ContextKeyUserId, 1001)
	return ctx
}

func TestSelectChannelMultiKeyStickyStableForSameSeed(t *testing.T) {
	resetMultiKeyAffinityTestState()
	channel := buildMultiKeyAffinityTestChannel()

	ctx1 := buildMultiKeyAffinityTestContext("workspace-a")
	first, apiErr := SelectChannelMultiKey(ctx1, channel, "deepseek-ai/DeepSeek-V4-Flash", "default")
	require.Nil(t, apiErr)
	require.NotNil(t, first)
	FinishChannelMultiKeyRequest(ctx1)

	ctx2 := buildMultiKeyAffinityTestContext("workspace-a")
	second, apiErr := SelectChannelMultiKey(ctx2, channel, "deepseek-ai/DeepSeek-V4-Flash", "default")
	require.Nil(t, apiErr)
	require.NotNil(t, second)
	FinishChannelMultiKeyRequest(ctx2)

	require.Equal(t, first.Index, second.Index)
	require.Equal(t, first.Key, second.Key)
	info, ok := ctx2.Get(ginKeyMultiKeyAffinityLogInfo)
	require.True(t, ok)
	logInfo := info.(map[string]interface{})
	require.Equal(t, true, logInfo["binding_hit"])
	require.Equal(t, first.Index, logInfo["selected_key_index"])
}

func TestSelectChannelMultiKeySkipsDisabledPrimaryKey(t *testing.T) {
	resetMultiKeyAffinityTestState()
	channel := buildMultiKeyAffinityTestChannel()
	seed := "disabled-primary"
	seedFP := common.GenerateHMAC("multi-key-affinity-seed:" + seed)
	scope := strings.Join([]string{fmt.Sprint(channel.Id), "default", "qwen-plus", seedFP}, ":")
	candidates := enabledMultiKeyCandidates(channel, channel.GetKeys())
	rankMultiKeyCandidates(candidates, scope)
	primary := candidates[0]

	channel.ChannelInfo.MultiKeyStatusList = map[int]int{
		primary.Index: common.ChannelStatusManuallyDisabled,
	}

	ctx := buildMultiKeyAffinityTestContext(seed)
	selected, apiErr := SelectChannelMultiKey(ctx, channel, "qwen-plus", "default")
	require.Nil(t, apiErr)
	require.NotNil(t, selected)
	FinishChannelMultiKeyRequest(ctx)

	require.NotEqual(t, primary.Index, selected.Index)
	require.NotEqual(t, primary.Key, selected.Key)
}

func TestSelectChannelMultiKeyLogInfoRedactsSeedAndKey(t *testing.T) {
	resetMultiKeyAffinityTestState()
	channel := buildMultiKeyAffinityTestChannel()
	rawSeed := "super-secret-workspace"

	ctx := buildMultiKeyAffinityTestContext(rawSeed)
	selected, apiErr := SelectChannelMultiKey(ctx, channel, "qwen-plus", "default")
	require.Nil(t, apiErr)
	require.NotNil(t, selected)
	FinishChannelMultiKeyRequest(ctx)

	info, ok := ctx.Get(ginKeyMultiKeyAffinityLogInfo)
	require.True(t, ok)
	bytes, err := json.Marshal(info)
	require.NoError(t, err)
	logJSON := string(bytes)
	require.NotContains(t, logJSON, rawSeed)
	require.NotContains(t, logJSON, selected.Key)
	require.Contains(t, logJSON, `"seed_fp"`)
	require.Contains(t, logJSON, `"selected_key_fp"`)
}

func TestExtractMultiKeyAffinitySeedPriority(t *testing.T) {
	ctx := buildMultiKeyAffinityTestContext("prompt-seed")
	ctx.Request.Header.Set("Session_id", "session-seed")
	source, value := ExtractMultiKeyAffinitySeed(ctx)
	require.Equal(t, "prompt_cache_key", source)
	require.Equal(t, "prompt-seed", value)

	ctx = buildMultiKeyAffinityTestContext("")
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"metadata":{"user_id":"meta-user"}}`))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Request.Header.Set("Session_id", "session-seed")
	source, value = ExtractMultiKeyAffinitySeed(ctx)
	require.Equal(t, "metadata.user_id", source)
	require.Equal(t, "meta-user", value)

	ctx = buildMultiKeyAffinityTestContext("")
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{}`))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Request.Header.Set("Session_id", "session-seed")
	source, value = ExtractMultiKeyAffinitySeed(ctx)
	require.Equal(t, "header:Session_id", source)
	require.Equal(t, "session-seed", value)

	ctx = buildMultiKeyAffinityTestContext("")
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{}`))
	ctx.Request.Header.Set("Content-Type", "application/json")
	common.SetContextKey(ctx, constant.ContextKeyTokenId, 9527)
	common.SetContextKey(ctx, constant.ContextKeyUserId, 1001)
	source, value = ExtractMultiKeyAffinitySeed(ctx)
	require.Equal(t, "token_id", source)
	require.Equal(t, "9527", value)
}

func TestSelectChannelMultiKeyNewSeedAvoidsSoftOverloadedKey(t *testing.T) {
	resetMultiKeyAffinityTestState()
	channel := buildMultiKeyAffinityTestChannel()
	policy := normalizeMultiKeyAffinityPolicy(channel.ChannelInfo.MultiKeyAffinityPolicy)
	now := time.Now()
	primarySeed := "heavy-workspace"
	primaryFP := common.GenerateHMAC("multi-key-affinity-seed:" + primarySeed)
	scope := strings.Join([]string{fmt.Sprint(channel.Id), "default", "qwen-plus", primaryFP}, ":")
	candidates := enabledMultiKeyCandidates(channel, channel.GetKeys())
	rankMultiKeyCandidates(candidates, scope)
	primary := candidates[0]

	multiKeyLoadMu.Lock()
	counter := getMultiKeyLoadCounterLocked(channel.Id, primary.Index, now)
	counter.Requests = defaultMultiKeyAffinityMinRPMForOverload
	counter.Inflight = defaultMultiKeyAffinityMinInflightForOverload
	for _, candidate := range candidates {
		if candidate.Index == primary.Index {
			continue
		}
		other := getMultiKeyLoadCounterLocked(channel.Id, candidate.Index, now)
		other.Requests = 3
		other.Inflight = defaultMultiKeyAffinityMinInflightForOverload
	}
	multiKeyLoadMu.Unlock()

	state := loadStateForMultiKey(channel.Id, candidates, primary.Index, policy)
	require.True(t, state.SoftOverloaded)

	ctx := buildMultiKeyAffinityTestContext(primarySeed)
	selected, apiErr := SelectChannelMultiKey(ctx, channel, "qwen-plus", "default")
	require.Nil(t, apiErr)
	require.NotNil(t, selected)
	FinishChannelMultiKeyRequest(ctx)

	require.NotEqual(t, primary.Index, selected.Index)
	info, ok := ctx.Get(ginKeyMultiKeyAffinityLogInfo)
	require.True(t, ok)
	logInfo := info.(map[string]interface{})
	require.Equal(t, "soft_overload_diverted", logInfo["fallback_reason"])
}

func TestSelectChannelMultiKeyExistingBindingStaysOnSoftOverload(t *testing.T) {
	resetMultiKeyAffinityTestState()
	channel := buildMultiKeyAffinityTestChannel()
	seed := "sticky-existing"

	ctx1 := buildMultiKeyAffinityTestContext(seed)
	first, apiErr := SelectChannelMultiKey(ctx1, channel, "qwen-plus", "default")
	require.Nil(t, apiErr)
	require.NotNil(t, first)
	FinishChannelMultiKeyRequest(ctx1)

	now := time.Now()
	multiKeyLoadMu.Lock()
	counter := getMultiKeyLoadCounterLocked(channel.Id, first.Index, now)
	counter.Requests = defaultMultiKeyAffinityMinRPMForOverload
	counter.Inflight = defaultMultiKeyAffinityMinInflightForOverload
	for _, candidate := range enabledMultiKeyCandidates(channel, channel.GetKeys()) {
		if candidate.Index == first.Index {
			continue
		}
		other := getMultiKeyLoadCounterLocked(channel.Id, candidate.Index, now)
		other.Requests = 3
		other.Inflight = defaultMultiKeyAffinityMinInflightForOverload
	}
	multiKeyLoadMu.Unlock()

	ctx2 := buildMultiKeyAffinityTestContext(seed)
	second, apiErr := SelectChannelMultiKey(ctx2, channel, "qwen-plus", "default")
	require.Nil(t, apiErr)
	require.NotNil(t, second)
	FinishChannelMultiKeyRequest(ctx2)

	require.Equal(t, first.Index, second.Index)
	info, ok := ctx2.Get(ginKeyMultiKeyAffinityLogInfo)
	require.True(t, ok)
	logInfo := info.(map[string]interface{})
	require.Equal(t, true, logInfo["binding_hit"])
	require.Equal(t, "binding", logInfo["fallback_reason"])
}

func TestSelectChannelMultiKeyExistingBindingCanMoveOnSoftOverload(t *testing.T) {
	resetMultiKeyAffinityTestState()
	channel := buildMultiKeyAffinityTestChannel()
	channel.ChannelInfo.MultiKeyAffinityPolicy.ExistingBindingStayOnSoftOverload = common.GetPointer(false)
	seed := "sticky-soft-move"

	ctx1 := buildMultiKeyAffinityTestContext(seed)
	first, apiErr := SelectChannelMultiKey(ctx1, channel, "qwen-plus", "default")
	require.Nil(t, apiErr)
	require.NotNil(t, first)
	FinishChannelMultiKeyRequest(ctx1)

	now := time.Now()
	multiKeyLoadMu.Lock()
	counter := getMultiKeyLoadCounterLocked(channel.Id, first.Index, now)
	counter.Requests = defaultMultiKeyAffinityMinRPMForOverload
	counter.Inflight = defaultMultiKeyAffinityMinInflightForOverload
	for _, candidate := range enabledMultiKeyCandidates(channel, channel.GetKeys()) {
		if candidate.Index == first.Index {
			continue
		}
		other := getMultiKeyLoadCounterLocked(channel.Id, candidate.Index, now)
		other.Requests = 3
		other.Inflight = defaultMultiKeyAffinityMinInflightForOverload
	}
	multiKeyLoadMu.Unlock()

	ctx2 := buildMultiKeyAffinityTestContext(seed)
	second, apiErr := SelectChannelMultiKey(ctx2, channel, "qwen-plus", "default")
	require.Nil(t, apiErr)
	require.NotNil(t, second)
	FinishChannelMultiKeyRequest(ctx2)

	require.NotEqual(t, first.Index, second.Index)
	info, ok := ctx2.Get(ginKeyMultiKeyAffinityLogInfo)
	require.True(t, ok)
	logInfo := info.(map[string]interface{})
	require.Equal(t, false, logInfo["binding_hit"])
	require.Equal(t, "soft_overload_moved", logInfo["fallback_reason"])
}

func TestSelectChannelMultiKeyExistingBindingMovesOnHardOverload(t *testing.T) {
	resetMultiKeyAffinityTestState()
	channel := buildMultiKeyAffinityTestChannel()
	channel.ChannelInfo.MultiKeyAffinityPolicy.MoveCooldownSeconds = 1
	seed := "sticky-hard-overload"

	ctx1 := buildMultiKeyAffinityTestContext(seed)
	first, apiErr := SelectChannelMultiKey(ctx1, channel, "qwen-plus", "default")
	require.Nil(t, apiErr)
	require.NotNil(t, first)
	FinishChannelMultiKeyRequest(ctx1)

	now := time.Now()
	multiKeyLoadMu.Lock()
	counter := getMultiKeyLoadCounterLocked(channel.Id, first.Index, now)
	counter.Requests = 100
	counter.Inflight = 10
	for _, candidate := range enabledMultiKeyCandidates(channel, channel.GetKeys()) {
		if candidate.Index == first.Index {
			continue
		}
		other := getMultiKeyLoadCounterLocked(channel.Id, candidate.Index, now)
		other.Requests = 1
		other.Inflight = 1
	}
	multiKeyLoadMu.Unlock()

	ctx2 := buildMultiKeyAffinityTestContext(seed)
	second, apiErr := SelectChannelMultiKey(ctx2, channel, "qwen-plus", "default")
	require.Nil(t, apiErr)
	require.NotNil(t, second)
	FinishChannelMultiKeyRequest(ctx2)

	require.NotEqual(t, first.Index, second.Index)
	info, ok := ctx2.Get(ginKeyMultiKeyAffinityLogInfo)
	require.True(t, ok)
	logInfo := info.(map[string]interface{})
	require.Equal(t, false, logInfo["binding_hit"])
	require.Equal(t, "hard_overload_moved", logInfo["fallback_reason"])
}

func TestFinishChannelMultiKeyRequestReleasesStackedSelections(t *testing.T) {
	resetMultiKeyAffinityTestState()
	channel := buildMultiKeyAffinityTestChannel()

	ctx := buildMultiKeyAffinityTestContext("stacked-selection-a")
	incMultiKeyLoad(channel.Id, 0)
	pushMultiKeyLoadSelection(ctx, channel.Id, 0)
	incMultiKeyLoad(channel.Id, 1)
	pushMultiKeyLoadSelection(ctx, channel.Id, 1)

	FinishChannelMultiKeyRequest(ctx)
	multiKeyLoadMu.Lock()
	firstCounter := getMultiKeyLoadCounterLocked(channel.Id, 0, time.Now())
	secondCounter := getMultiKeyLoadCounterLocked(channel.Id, 1, time.Now())
	require.Equal(t, 1, firstCounter.Inflight)
	require.Equal(t, 0, secondCounter.Inflight)
	multiKeyLoadMu.Unlock()

	FinishChannelMultiKeyRequest(ctx)
	multiKeyLoadMu.Lock()
	firstCounter = getMultiKeyLoadCounterLocked(channel.Id, 0, time.Now())
	secondCounter = getMultiKeyLoadCounterLocked(channel.Id, 1, time.Now())
	require.Equal(t, 0, firstCounter.Inflight)
	require.Equal(t, 0, secondCounter.Inflight)
	multiKeyLoadMu.Unlock()

	FinishChannelMultiKeyRequest(ctx)
	multiKeyLoadMu.Lock()
	firstCounter = getMultiKeyLoadCounterLocked(channel.Id, 0, time.Now())
	secondCounter = getMultiKeyLoadCounterLocked(channel.Id, 1, time.Now())
	require.Equal(t, 0, firstCounter.Inflight)
	require.Equal(t, 0, secondCounter.Inflight)
	multiKeyLoadMu.Unlock()
}

func TestFinishAllChannelMultiKeyRequestsReleasesAllSelections(t *testing.T) {
	resetMultiKeyAffinityTestState()
	channel := buildMultiKeyAffinityTestChannel()
	ctx := buildMultiKeyAffinityTestContext("finish-all")

	incMultiKeyLoad(channel.Id, 0)
	pushMultiKeyLoadSelection(ctx, channel.Id, 0)
	incMultiKeyLoad(channel.Id, 1)
	pushMultiKeyLoadSelection(ctx, channel.Id, 1)

	FinishAllChannelMultiKeyRequests(ctx)

	multiKeyLoadMu.Lock()
	firstCounter := getMultiKeyLoadCounterLocked(channel.Id, 0, time.Now())
	secondCounter := getMultiKeyLoadCounterLocked(channel.Id, 1, time.Now())
	require.Equal(t, 0, firstCounter.Inflight)
	require.Equal(t, 0, secondCounter.Inflight)
	multiKeyLoadMu.Unlock()
	require.False(t, ctx.GetBool(ginKeyMultiKeyAffinityCounted))
}

func TestMultiKeyLoadWindowKeepsInflightAcrossMinute(t *testing.T) {
	resetMultiKeyAffinityTestState()
	channel := buildMultiKeyAffinityTestChannel()
	start := time.Unix(1_700_000_000, 0)

	multiKeyLoadMu.Lock()
	counter := getMultiKeyLoadCounterLocked(channel.Id, 0, start)
	counter.Requests = 7
	counter.Inflight = 3
	nextCounter := getMultiKeyLoadCounterLocked(channel.Id, 0, start.Add(2*time.Minute))
	require.Equal(t, 0, nextCounter.Requests)
	require.Equal(t, 3, nextCounter.Inflight)
	multiKeyLoadMu.Unlock()
}
