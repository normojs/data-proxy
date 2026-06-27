package service

import (
	"errors"
	"fmt"
	"hash/fnv"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/cachex"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/samber/hot"
	"github.com/tidwall/gjson"
)

const (
	ginKeyMultiKeyAffinityLogInfo = "multi_key_affinity_log_info"
	ginKeyMultiKeyAffinityCounted = "multi_key_affinity_counted"
	ginKeyMultiKeyAffinityLoads   = "multi_key_affinity_loads"

	multiKeyAffinityBindingNamespace = "new-api:multi_key_affinity_binding:v1"

	defaultMultiKeyAffinityBindingTTLSeconds      = 3600
	defaultMultiKeyAffinityMoveCooldownSeconds    = 900
	defaultMultiKeyAffinitySoftLoadFactor         = 1.25
	defaultMultiKeyAffinityHardLoadFactor         = 1.8
	defaultMultiKeyAffinityMinRPMForOverload      = 5
	defaultMultiKeyAffinityMinInflightForOverload = 2

	multiKeyAffinityLoadWindow = time.Minute
)

type MultiKeySelectionResult struct {
	Key   string
	Index int
}

type multiKeyCandidate struct {
	Index int
	Key   string
	KeyFP string
	Score uint64
}

type multiKeyAffinityBinding struct {
	SelectedKeyFP     string `json:"selected_key_fp"`
	SelectedKeyIndex  int    `json:"selected_key_index"`
	PrimaryKeyFP      string `json:"primary_key_fp"`
	Reason            string `json:"reason"`
	MoveCooldownUntil int64  `json:"move_cooldown_until"`
	UpdatedAt         int64  `json:"updated_at"`
}

type multiKeyLoadCounter struct {
	WindowStart int64
	Requests    int
	Inflight    int
}

type multiKeyLoadSelection struct {
	ChannelID int
	KeyIndex  int
}

type multiKeyLoadState struct {
	State          string
	Load           float64
	AvgRPM         float64
	AvgInflight    float64
	RPM            int
	Inflight       int
	SoftOverloaded bool
	HardOverloaded bool
}

var (
	multiKeyAffinityBindingCacheOnce sync.Once
	multiKeyAffinityBindingCache     *cachex.HybridCache[multiKeyAffinityBinding]

	multiKeyLoadMu       sync.Mutex
	multiKeyLoadCounters = map[string]*multiKeyLoadCounter{}
)

func getMultiKeyAffinityBindingCache() *cachex.HybridCache[multiKeyAffinityBinding] {
	multiKeyAffinityBindingCacheOnce.Do(func() {
		multiKeyAffinityBindingCache = cachex.NewHybridCache[multiKeyAffinityBinding](cachex.HybridCacheConfig[multiKeyAffinityBinding]{
			Namespace: cachex.Namespace(multiKeyAffinityBindingNamespace),
			Redis:     common.RDB,
			RedisEnabled: func() bool {
				return common.RedisEnabled && common.RDB != nil
			},
			RedisCodec: cachex.JSONCodec[multiKeyAffinityBinding]{},
			Memory: func() *hot.HotCache[string, multiKeyAffinityBinding] {
				return hot.NewHotCache[string, multiKeyAffinityBinding](hot.LRU, 100_000).
					WithTTL(time.Duration(defaultMultiKeyAffinityBindingTTLSeconds) * time.Second).
					WithJanitor().
					Build()
			},
		})
	})
	return multiKeyAffinityBindingCache
}

func SelectChannelMultiKey(c *gin.Context, channel *model.Channel, modelName string, usingGroup string) (*MultiKeySelectionResult, *types.NewAPIError) {
	if channel == nil {
		return nil, types.NewError(errors.New("channel is nil"), types.ErrorCodeGetChannelFailed, types.ErrOptionWithSkipRetry())
	}
	if !channel.ChannelInfo.IsMultiKey || channel.ChannelInfo.MultiKeyMode != constant.MultiKeyModeStickyHashBounded {
		key, index, apiErr := channel.GetNextEnabledKey()
		if apiErr != nil {
			return nil, apiErr
		}
		return &MultiKeySelectionResult{Key: key, Index: index}, nil
	}

	keys := channel.GetKeys()
	if len(keys) == 0 {
		return nil, types.NewError(errors.New("no keys available"), types.ErrorCodeChannelNoAvailableKey)
	}
	enabled := enabledMultiKeyCandidates(channel, keys)
	if len(enabled) == 0 {
		return nil, types.NewError(errors.New("no enabled keys"), types.ErrorCodeChannelNoAvailableKey)
	}

	policy := normalizeMultiKeyAffinityPolicy(channel.ChannelInfo.MultiKeyAffinityPolicy)
	seedSource, seedValue := ExtractMultiKeyAffinitySeed(c)
	seedFP := ""
	if seedValue != "" {
		seedFP = common.GenerateHMAC("multi-key-affinity-seed:" + seedValue)
	}
	if seedFP == "" {
		seedSource = "channel"
		seedFP = common.GenerateHMAC(fmt.Sprintf("multi-key-affinity-channel:%d:%s:%s", channel.Id, usingGroup, modelName))
	}
	scope := strings.Join([]string{strconv.Itoa(channel.Id), strings.TrimSpace(usingGroup), strings.TrimSpace(modelName), seedFP}, ":")
	rankMultiKeyCandidates(enabled, scope)
	primary := enabled[0]
	bindingKey := strings.Join([]string{strconv.Itoa(channel.Id), strings.TrimSpace(usingGroup), strings.TrimSpace(modelName), seedFP}, ":")
	now := time.Now()

	selected := primary
	reason := "primary"
	bindingHit := false
	loadState := loadStateForMultiKey(channel.Id, enabled, primary.Index, policy)
	binding, found, err := getMultiKeyAffinityBindingCache().Get(bindingKey)
	if err != nil {
		common.SysError(fmt.Sprintf("multi-key affinity binding get failed: channel=%d err=%v", channel.Id, err))
	}
	if found {
		if candidate, ok := findCandidateByFP(enabled, binding.SelectedKeyFP); ok {
			candidateState := loadStateForMultiKey(channel.Id, enabled, candidate.Index, policy)
			cooldownActive := binding.MoveCooldownUntil > now.Unix()
			if candidateState.HardOverloaded && !cooldownActive {
				if next, nextState, ok := firstUsableCandidate(channel.Id, enabled, policy, candidate.KeyFP); ok {
					selected = next
					loadState = nextState
					reason = "hard_overload_moved"
				} else {
					selected = candidate
					loadState = candidateState
					reason = "binding_hard_overload_no_alternative"
				}
			} else if candidateState.SoftOverloaded && !policyExistingBindingStayOnSoftOverload(policy) && !cooldownActive {
				if next, nextState, ok := firstUsableCandidate(channel.Id, enabled, policy, candidate.KeyFP); ok {
					selected = next
					loadState = nextState
					reason = "soft_overload_moved"
				} else {
					selected = candidate
					loadState = candidateState
					bindingHit = true
					reason = "binding_soft_overload_no_alternative"
				}
			} else {
				selected = candidate
				loadState = candidateState
				bindingHit = true
				reason = "binding"
			}
		}
	}
	if !bindingHit && reason == "primary" {
		if loadState.SoftOverloaded {
			if next, nextState, ok := firstUsableCandidate(channel.Id, enabled, policy, ""); ok {
				selected = next
				loadState = nextState
				reason = "soft_overload_diverted"
			}
		}
	}

	nextBinding := multiKeyAffinityBinding{
		SelectedKeyFP:    selected.KeyFP,
		SelectedKeyIndex: selected.Index,
		PrimaryKeyFP:     primary.KeyFP,
		Reason:           reason,
		UpdatedAt:        now.Unix(),
	}
	if !bindingHit && found {
		nextBinding.MoveCooldownUntil = now.Add(time.Duration(policy.MoveCooldownSeconds) * time.Second).Unix()
	} else if found {
		nextBinding.MoveCooldownUntil = binding.MoveCooldownUntil
	}
	if err := getMultiKeyAffinityBindingCache().SetWithTTL(bindingKey, nextBinding, time.Duration(policy.BindingTTLSeconds)*time.Second); err != nil {
		common.SysError(fmt.Sprintf("multi-key affinity binding set failed: channel=%d err=%v", channel.Id, err))
	}

	c.Set(ginKeyMultiKeyAffinityLogInfo, map[string]interface{}{
		"enabled":            true,
		"mode":               string(constant.MultiKeyModeStickyHashBounded),
		"seed_source":        seedSource,
		"seed_fp":            shortFingerprint(seedFP),
		"binding_hit":        bindingHit,
		"selected_key_index": selected.Index,
		"selected_key_fp":    shortFingerprint(selected.KeyFP),
		"primary_key_index":  primary.Index,
		"load_state":         loadState.State,
		"key_load":           roundFloat(loadState.Load, 2),
		"avg_rpm":            roundFloat(loadState.AvgRPM, 2),
		"avg_inflight":       roundFloat(loadState.AvgInflight, 2),
		"fallback_reason":    reason,
	})

	incMultiKeyLoad(channel.Id, selected.Index)
	pushMultiKeyLoadSelection(c, channel.Id, selected.Index)
	return &MultiKeySelectionResult{Key: selected.Key, Index: selected.Index}, nil
}

func FinishChannelMultiKeyRequest(c *gin.Context) {
	if c == nil {
		return
	}
	if anySelections, ok := c.Get(ginKeyMultiKeyAffinityLoads); ok {
		if selections, ok := anySelections.([]multiKeyLoadSelection); ok && len(selections) > 0 {
			selection := selections[len(selections)-1]
			selections = selections[:len(selections)-1]
			c.Set(ginKeyMultiKeyAffinityLoads, selections)
			c.Set(ginKeyMultiKeyAffinityCounted, len(selections) > 0)
			if selection.ChannelID > 0 && selection.KeyIndex >= 0 {
				decMultiKeyLoad(selection.ChannelID, selection.KeyIndex)
			}
			return
		}
	}
	if !c.GetBool(ginKeyMultiKeyAffinityCounted) {
		return
	}
	channelID := common.GetContextKeyInt(c, constant.ContextKeyChannelId)
	keyIndex := common.GetContextKeyInt(c, constant.ContextKeyChannelMultiKeyIndex)
	if channelID <= 0 || keyIndex < 0 {
		return
	}
	decMultiKeyLoad(channelID, keyIndex)
	c.Set(ginKeyMultiKeyAffinityCounted, false)
}

func FinishAllChannelMultiKeyRequests(c *gin.Context) {
	if c == nil {
		return
	}
	for {
		if anySelections, ok := c.Get(ginKeyMultiKeyAffinityLoads); ok {
			if selections, ok := anySelections.([]multiKeyLoadSelection); ok && len(selections) > 0 {
				FinishChannelMultiKeyRequest(c)
				continue
			}
		}
		break
	}
	FinishChannelMultiKeyRequest(c)
}

func AppendMultiKeyAffinityAdminInfo(c *gin.Context, adminInfo map[string]interface{}) {
	if c == nil || adminInfo == nil {
		return
	}
	anyInfo, ok := c.Get(ginKeyMultiKeyAffinityLogInfo)
	if !ok || anyInfo == nil {
		return
	}
	adminInfo["multi_key_affinity"] = anyInfo
}

func ExtractMultiKeyAffinitySeed(c *gin.Context) (string, string) {
	if c == nil {
		return "", ""
	}
	if c.Request != nil {
		contentType := strings.ToLower(c.Request.Header.Get("Content-Type"))
		if strings.HasPrefix(contentType, "application/json") {
			if storage, err := common.GetBodyStorage(c); err == nil && storage != nil && !storage.IsDisk() {
				if body, err := storage.Bytes(); err == nil && len(body) > 0 {
					if value := strings.TrimSpace(gjsonGet(body, "prompt_cache_key")); value != "" {
						return "prompt_cache_key", value
					}
					if value := strings.TrimSpace(gjsonGet(body, "metadata.user_id")); value != "" {
						return "metadata.user_id", value
					}
				}
			}
		}
		for _, header := range []string{"Session_id", "Originator", "X-Data-Proxy-Affinity-Key"} {
			if value := strings.TrimSpace(c.Request.Header.Get(header)); value != "" {
				return "header:" + header, value
			}
		}
	}
	if tokenID := common.GetContextKeyInt(c, constant.ContextKeyTokenId); tokenID > 0 {
		return "token_id", strconv.Itoa(tokenID)
	}
	if userID := common.GetContextKeyInt(c, constant.ContextKeyUserId); userID > 0 {
		return "user_id", strconv.Itoa(userID)
	}
	return "", ""
}

func gjsonGet(body []byte, path string) string {
	res := gjson.GetBytes(body, path)
	if !res.Exists() {
		return ""
	}
	switch res.Type {
	case gjson.String, gjson.Number, gjson.True, gjson.False:
		return res.String()
	default:
		return res.Raw
	}
}

func enabledMultiKeyCandidates(channel *model.Channel, keys []string) []multiKeyCandidate {
	statusList := channel.ChannelInfo.MultiKeyStatusList
	getStatus := func(idx int) int {
		if statusList == nil {
			return common.ChannelStatusEnabled
		}
		if status, ok := statusList[idx]; ok {
			return status
		}
		return common.ChannelStatusEnabled
	}
	candidates := make([]multiKeyCandidate, 0, len(keys))
	for i, key := range keys {
		if strings.TrimSpace(key) == "" {
			continue
		}
		if getStatus(i) != common.ChannelStatusEnabled {
			continue
		}
		keyFP := common.GenerateHMAC(fmt.Sprintf("multi-key-affinity-key:%d:%s", channel.Id, key))
		candidates = append(candidates, multiKeyCandidate{Index: i, Key: key, KeyFP: keyFP})
	}
	return candidates
}

func rankMultiKeyCandidates(candidates []multiKeyCandidate, scope string) {
	for i := range candidates {
		candidates[i].Score = hash64(scope + ":" + candidates[i].KeyFP + ":" + strconv.Itoa(candidates[i].Index))
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Score == candidates[j].Score {
			return candidates[i].Index < candidates[j].Index
		}
		return candidates[i].Score > candidates[j].Score
	})
}

func findCandidateByFP(candidates []multiKeyCandidate, keyFP string) (multiKeyCandidate, bool) {
	for _, candidate := range candidates {
		if candidate.KeyFP == keyFP {
			return candidate, true
		}
	}
	return multiKeyCandidate{}, false
}

func firstUsableCandidate(channelID int, candidates []multiKeyCandidate, policy model.MultiKeyAffinityPolicy, excludedKeyFP string) (multiKeyCandidate, multiKeyLoadState, bool) {
	filtered := make([]multiKeyCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if excludedKeyFP != "" && candidate.KeyFP == excludedKeyFP {
			continue
		}
		filtered = append(filtered, candidate)
		state := loadStateForMultiKey(channelID, candidates, candidate.Index, policy)
		if state.SoftOverloaded || state.HardOverloaded {
			continue
		}
		return candidate, state, true
	}
	if len(filtered) == 0 {
		return multiKeyCandidate{}, multiKeyLoadState{}, false
	}
	best := filtered[0]
	bestState := loadStateForMultiKey(channelID, candidates, best.Index, policy)
	for _, candidate := range filtered[1:] {
		state := loadStateForMultiKey(channelID, candidates, candidate.Index, policy)
		if state.Load < bestState.Load {
			best = candidate
			bestState = state
		}
	}
	return best, bestState, true
}

func pushMultiKeyLoadSelection(c *gin.Context, channelID int, keyIndex int) {
	if c == nil || channelID <= 0 || keyIndex < 0 {
		return
	}
	selections := make([]multiKeyLoadSelection, 0, 1)
	if anySelections, ok := c.Get(ginKeyMultiKeyAffinityLoads); ok {
		if existing, ok := anySelections.([]multiKeyLoadSelection); ok {
			selections = existing
		}
	}
	selections = append(selections, multiKeyLoadSelection{ChannelID: channelID, KeyIndex: keyIndex})
	c.Set(ginKeyMultiKeyAffinityLoads, selections)
	c.Set(ginKeyMultiKeyAffinityCounted, true)
}

func normalizeMultiKeyAffinityPolicy(policy model.MultiKeyAffinityPolicy) model.MultiKeyAffinityPolicy {
	if policy.BindingTTLSeconds <= 0 {
		policy.BindingTTLSeconds = defaultMultiKeyAffinityBindingTTLSeconds
	}
	if policy.MoveCooldownSeconds <= 0 {
		policy.MoveCooldownSeconds = defaultMultiKeyAffinityMoveCooldownSeconds
	}
	if policy.SoftLoadFactor <= 0 {
		policy.SoftLoadFactor = defaultMultiKeyAffinitySoftLoadFactor
	}
	if policy.HardLoadFactor <= 0 {
		policy.HardLoadFactor = defaultMultiKeyAffinityHardLoadFactor
	}
	policy.Enabled = true
	if policy.ExistingBindingStayOnSoftOverload == nil {
		policy.ExistingBindingStayOnSoftOverload = common.GetPointer(true)
	}
	return policy
}

func policyExistingBindingStayOnSoftOverload(policy model.MultiKeyAffinityPolicy) bool {
	if policy.ExistingBindingStayOnSoftOverload == nil {
		return true
	}
	return *policy.ExistingBindingStayOnSoftOverload
}

func loadKey(channelID int, keyIndex int) string {
	return fmt.Sprintf("%d:%d", channelID, keyIndex)
}

func incMultiKeyLoad(channelID int, keyIndex int) {
	multiKeyLoadMu.Lock()
	defer multiKeyLoadMu.Unlock()
	counter := getMultiKeyLoadCounterLocked(channelID, keyIndex, time.Now())
	counter.Requests++
	counter.Inflight++
}

func decMultiKeyLoad(channelID int, keyIndex int) {
	multiKeyLoadMu.Lock()
	defer multiKeyLoadMu.Unlock()
	counter := getMultiKeyLoadCounterLocked(channelID, keyIndex, time.Now())
	if counter.Inflight > 0 {
		counter.Inflight--
	}
}

func getMultiKeyLoadCounterLocked(channelID int, keyIndex int, now time.Time) *multiKeyLoadCounter {
	key := loadKey(channelID, keyIndex)
	windowStart := now.Truncate(multiKeyAffinityLoadWindow).Unix()
	counter := multiKeyLoadCounters[key]
	if counter == nil || counter.WindowStart != windowStart {
		inflight := 0
		if counter != nil {
			inflight = counter.Inflight
		}
		counter = &multiKeyLoadCounter{WindowStart: windowStart, Inflight: inflight}
		multiKeyLoadCounters[key] = counter
	}
	return counter
}

func loadStateForMultiKey(channelID int, candidates []multiKeyCandidate, keyIndex int, policy model.MultiKeyAffinityPolicy) multiKeyLoadState {
	multiKeyLoadMu.Lock()
	defer multiKeyLoadMu.Unlock()
	now := time.Now()
	totalRPM := 0
	totalInflight := 0
	count := 0
	currentRPM := 0
	currentInflight := 0
	for _, candidate := range candidates {
		counter := getMultiKeyLoadCounterLocked(channelID, candidate.Index, now)
		totalRPM += counter.Requests
		totalInflight += counter.Inflight
		count++
		if candidate.Index == keyIndex {
			currentRPM = counter.Requests
			currentInflight = counter.Inflight
		}
	}
	if count <= 0 {
		return multiKeyLoadState{State: "normal", Load: 1}
	}
	avgRPM := float64(totalRPM) / float64(count)
	avgInflight := float64(totalInflight) / float64(count)
	load := 1.0
	if avgRPM > 0 {
		load = math.Max(load, float64(currentRPM)/avgRPM)
	}
	if avgInflight > 0 {
		load = math.Max(load, float64(currentInflight)/avgInflight)
	}
	soft := false
	hard := false
	if totalRPM >= defaultMultiKeyAffinityMinRPMForOverload && currentRPM >= defaultMultiKeyAffinityMinRPMForOverload {
		soft = load > policy.SoftLoadFactor
		hard = load > policy.HardLoadFactor
	}
	if totalInflight >= defaultMultiKeyAffinityMinInflightForOverload && currentInflight >= defaultMultiKeyAffinityMinInflightForOverload {
		soft = soft || load > policy.SoftLoadFactor
		hard = hard || load > policy.HardLoadFactor
	}
	state := "normal"
	if hard {
		state = "hard_overload"
	} else if soft {
		state = "soft_overload"
	}
	return multiKeyLoadState{
		State:          state,
		Load:           load,
		AvgRPM:         avgRPM,
		AvgInflight:    avgInflight,
		RPM:            currentRPM,
		Inflight:       currentInflight,
		SoftOverloaded: soft,
		HardOverloaded: hard,
	}
}

func hash64(s string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(s))
	return h.Sum64()
}

func shortFingerprint(fp string) string {
	fp = strings.TrimSpace(fp)
	if len(fp) <= 12 {
		return fp
	}
	return fp[:12]
}

func roundFloat(value float64, places int) float64 {
	if places < 0 {
		return value
	}
	factor := math.Pow10(places)
	return math.Round(value*factor) / factor
}
