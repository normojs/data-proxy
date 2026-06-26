package service

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
)

const (
	defaultChannelHealthFailureWindowMinutes = 5
	defaultChannelHealthCooldownMinutes      = 2
	defaultChannelHealthMaxCooldownMinutes   = 10
	defaultChannelHealthThreshold            = 3
)

type ChannelErrorAction string

const (
	ChannelErrorActionNone             ChannelErrorAction = "none"
	ChannelErrorActionRecordTransient  ChannelErrorAction = "record_transient"
	ChannelErrorActionHardAutoDisabled ChannelErrorAction = "hard_auto_disabled"
)

type ChannelHealthDecision struct {
	Action        ChannelErrorAction
	FailureCount  int
	CooldownUntil time.Time
	Reason        string
}

type channelHealthState struct {
	ConsecutiveFailures int
	FirstFailureAt      time.Time
	LastFailureAt       time.Time
	CooldownUntil       time.Time
	LastStatusCode      int
	LastErrorCode       types.ErrorCode
	LastReason          string
}

var channelHealthRegistry = struct {
	sync.Mutex
	states map[int]*channelHealthState
}{
	states: map[int]*channelHealthState{},
}

func ShouldDisableChannel(err *types.NewAPIError) bool {
	return ShouldHardDisableChannel(err)
}

func ShouldHardDisableChannel(err *types.NewAPIError) bool {
	if !common.AutomaticDisableChannelEnabled {
		return false
	}
	if err == nil {
		return false
	}
	if isHardChannelErrorCode(err.GetErrorCode()) {
		return true
	}
	if types.IsSkipRetryError(err) {
		return false
	}
	if operationShouldHardDisable(err) {
		return true
	}
	return containsAutomaticDisableKeyword(err)
}

func isHardChannelErrorCode(code types.ErrorCode) bool {
	return strings.HasPrefix(string(code), "channel:")
}

func operationShouldHardDisable(err *types.NewAPIError) bool {
	if err == nil {
		return false
	}
	return operation_setting.ShouldDisableByStatusCode(err.StatusCode)
}

func containsAutomaticDisableKeyword(err *types.NewAPIError) bool {
	if err == nil {
		return false
	}
	lowerMessage := strings.ToLower(err.Error())
	search, _ := AcSearch(lowerMessage, operation_setting.AutomaticDisableKeywords, true)
	return search
}

func IsTransientChannelError(err *types.NewAPIError) bool {
	if err == nil || types.IsSkipRetryError(err) {
		return false
	}
	if ShouldHardDisableChannel(err) {
		return false
	}
	code := err.StatusCode
	if operation_setting.ShouldTreatTransientByStatusCode(code) {
		return true
	}
	switch err.GetErrorCode() {
	case types.ErrorCodeDoRequestFailed,
		types.ErrorCodeReadResponseBodyFailed,
		types.ErrorCodeBadResponse,
		types.ErrorCodeBadResponseBody,
		types.ErrorCodeEmptyResponse,
		types.ErrorCodeAwsInvokeError:
		return true
	}
	lowerMessage := strings.ToLower(err.Error())
	search, _ := AcSearch(lowerMessage, operation_setting.ChannelHealthTransientKeywords, true)
	return search
}

func HandleChannelFailure(channelError types.ChannelError, err *types.NewAPIError) ChannelHealthDecision {
	if !common.AutomaticDisableChannelEnabled || err == nil {
		return ChannelHealthDecision{Action: ChannelErrorActionNone}
	}
	if ShouldHardDisableChannel(err) {
		return ChannelHealthDecision{
			Action: ChannelErrorActionHardAutoDisabled,
			Reason: err.ErrorWithStatusCode(),
		}
	}
	if !channelError.AutoBan || !IsTransientChannelError(err) {
		return ChannelHealthDecision{Action: ChannelErrorActionNone}
	}
	return recordTransientChannelFailure(channelError, err, time.Now())
}

func RecordChannelSuccess(channelId int) {
	if channelId <= 0 {
		return
	}
	channelHealthRegistry.Lock()
	delete(channelHealthRegistry.states, channelId)
	channelHealthRegistry.Unlock()
}

func ClearChannelTemporaryHealth(channelId int) bool {
	if channelId <= 0 {
		return false
	}
	channelHealthRegistry.Lock()
	_, existed := channelHealthRegistry.states[channelId]
	delete(channelHealthRegistry.states, channelId)
	channelHealthRegistry.Unlock()
	return existed
}

func ChannelRuntimeHealthSnapshot(channelId int) dto.ChannelRuntimeHealth {
	if channelId <= 0 {
		return dto.ChannelRuntimeHealth{Status: "healthy"}
	}

	channelHealthRegistry.Lock()
	defer channelHealthRegistry.Unlock()

	state := channelHealthRegistry.states[channelId]
	if state == nil {
		return dto.ChannelRuntimeHealth{Status: "healthy"}
	}

	now := time.Now()
	if pruneExpiredChannelHealthStateLocked(channelId, state, now) {
		return dto.ChannelRuntimeHealth{Status: "healthy"}
	}

	status := "healthy"
	temporarilyUnavailable := false
	if state.CooldownUntil.After(now) {
		status = "temporarily_unavailable"
		temporarilyUnavailable = true
	} else if state.ConsecutiveFailures > 0 {
		status = "degraded"
	}

	snapshot := dto.ChannelRuntimeHealth{
		Status:                 status,
		TemporarilyUnavailable: temporarilyUnavailable,
		ConsecutiveFailures:    state.ConsecutiveFailures,
		LastStatusCode:         state.LastStatusCode,
		LastErrorCode:          string(state.LastErrorCode),
		LastReason:             state.LastReason,
	}
	if !state.FirstFailureAt.IsZero() {
		snapshot.FirstFailureAt = state.FirstFailureAt.Unix()
	}
	if !state.LastFailureAt.IsZero() {
		snapshot.LastFailureAt = state.LastFailureAt.Unix()
	}
	if !state.CooldownUntil.IsZero() {
		snapshot.CooldownUntil = state.CooldownUntil.Unix()
	}
	return snapshot
}

func IsChannelTemporarilyUnavailable(channelId int) bool {
	if channelId <= 0 {
		return false
	}
	channelHealthRegistry.Lock()
	defer channelHealthRegistry.Unlock()
	state := channelHealthRegistry.states[channelId]
	if state == nil {
		return false
	}
	now := time.Now()
	if pruneExpiredChannelHealthStateLocked(channelId, state, now) {
		return false
	}
	if state.CooldownUntil.IsZero() {
		return false
	}
	return true
}

func TemporarilyUnavailableChannelIDs() []int {
	channelHealthRegistry.Lock()
	defer channelHealthRegistry.Unlock()
	now := time.Now()
	ids := make([]int, 0)
	for id, state := range channelHealthRegistry.states {
		if state == nil {
			continue
		}
		if pruneExpiredChannelHealthStateLocked(id, state, now) {
			continue
		}
		if state.CooldownUntil.After(now) {
			ids = append(ids, id)
		}
	}
	return ids
}

func pruneExpiredChannelHealthStateLocked(channelId int, state *channelHealthState, now time.Time) bool {
	if state == nil {
		return true
	}
	if !state.CooldownUntil.IsZero() {
		if state.CooldownUntil.After(now) {
			return false
		}
		delete(channelHealthRegistry.states, channelId)
		return true
	}
	if !state.LastFailureAt.IsZero() && now.Sub(state.LastFailureAt) > channelHealthFailureWindow() {
		delete(channelHealthRegistry.states, channelId)
		return true
	}
	return false
}

func resetChannelHealthForTest() {
	channelHealthRegistry.Lock()
	channelHealthRegistry.states = map[int]*channelHealthState{}
	channelHealthRegistry.Unlock()
}

func recordTransientChannelFailure(channelError types.ChannelError, err *types.NewAPIError, now time.Time) ChannelHealthDecision {
	channelHealthRegistry.Lock()
	defer channelHealthRegistry.Unlock()

	state := channelHealthRegistry.states[channelError.ChannelId]
	if state == nil {
		state = &channelHealthState{}
		channelHealthRegistry.states[channelError.ChannelId] = state
	}
	if state.FirstFailureAt.IsZero() || now.Sub(state.FirstFailureAt) > channelHealthFailureWindow() {
		state.ConsecutiveFailures = 0
		state.FirstFailureAt = now
	}
	state.ConsecutiveFailures++
	state.LastFailureAt = now
	state.LastStatusCode = err.StatusCode
	state.LastErrorCode = err.GetErrorCode()
	state.LastReason = err.ErrorWithStatusCode()

	decision := ChannelHealthDecision{
		Action:       ChannelErrorActionRecordTransient,
		FailureCount: state.ConsecutiveFailures,
		Reason:       state.LastReason,
	}
	threshold := channelHealthThreshold()
	if state.ConsecutiveFailures < threshold {
		return decision
	}

	cooldown := channelHealthCooldown() * time.Duration(state.ConsecutiveFailures-threshold+1)
	maxCooldown := channelHealthMaxCooldown()
	if cooldown > maxCooldown {
		cooldown = maxCooldown
	}
	state.CooldownUntil = now.Add(cooldown)
	decision.CooldownUntil = state.CooldownUntil
	common.SysLog(fmt.Sprintf("通道「%s」（#%d）连续出现临时错误 %d 次，临时熔断至 %s，原因：%s",
		channelError.ChannelName,
		channelError.ChannelId,
		state.ConsecutiveFailures,
		state.CooldownUntil.Format(time.RFC3339),
		common.LocalLogPreview(state.LastReason),
	))
	return decision
}

func channelHealthThreshold() int {
	if common.ChannelHealthFailureThreshold <= 0 {
		return defaultChannelHealthThreshold
	}
	return common.ChannelHealthFailureThreshold
}

func channelHealthFailureWindow() time.Duration {
	minutes := common.ChannelHealthFailureWindowMinutes
	if minutes <= 0 {
		minutes = defaultChannelHealthFailureWindowMinutes
	}
	return time.Duration(minutes) * time.Minute
}

func channelHealthCooldown() time.Duration {
	minutes := common.ChannelHealthCooldownMinutes
	if minutes <= 0 {
		minutes = defaultChannelHealthCooldownMinutes
	}
	return time.Duration(minutes) * time.Minute
}

func channelHealthMaxCooldown() time.Duration {
	minutes := common.ChannelHealthMaxCooldownMinutes
	if minutes <= 0 {
		minutes = defaultChannelHealthMaxCooldownMinutes
	}
	cooldown := time.Duration(minutes) * time.Minute
	baseCooldown := channelHealthCooldown()
	if cooldown < baseCooldown {
		return baseCooldown
	}
	return cooldown
}
