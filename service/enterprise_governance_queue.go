package service

import (
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/gin-gonic/gin"
)

const (
	enterpriseGovernanceAuditActionQueueAdmission = "enterprise_governance.queue_admission"
	enterpriseQueueStatusAdmitted                 = "admitted"
	enterpriseQueueStatusTimeout                  = "timeout"
	enterpriseQueueStatusCanceled                 = "canceled"
	enterpriseQueueStatusHeader                   = "X-Data-Proxy-Enterprise-Queue-Status"
	enterpriseQueueWaitMsHeader                   = "X-Data-Proxy-Enterprise-Queue-Wait-Ms"
	enterpriseQueueTimeoutMsHeader                = "X-Data-Proxy-Enterprise-Queue-Timeout-Ms"
)

var (
	enterprisePolicyQueueMaxConcurrent = 1
	enterprisePolicyQueueTimeout       = 30 * time.Second
	enterprisePolicyQueues             sync.Map
)

var ErrEnterpriseGovernanceQueueTimeout = errors.New("enterprise governance queue timeout")

type EnterpriseGovernanceQueueResult struct {
	Applied   bool
	Status    string
	WaitMs    int64
	TimeoutMs int64
}

type enterprisePolicyQueue struct {
	slots chan struct{}
}

func ApplyEnterpriseGovernanceQueue(c *gin.Context, relayInfo *relaycommon.RelayInfo) (EnterpriseGovernanceQueueResult, func(), error) {
	result := EnterpriseGovernanceQueueResult{}
	if !common.EnterpriseGovernanceEnabled || c == nil || relayInfo == nil {
		return result, nil, nil
	}
	decision, ok := common.GetContextKeyType[PolicyDecision](c, constant.ContextKeyEnterpriseGovernanceDecision)
	if !ok || !hasEnterprisePolicyQueueAction(decision.ActionObservations) {
		return result, nil, nil
	}
	enterpriseCtx, ok := common.GetContextKeyType[*EnterpriseContext](c, constant.ContextKeyEnterpriseGovernanceContext)
	if !ok || enterpriseCtx == nil {
		var err error
		enterpriseCtx, err = resolveEnterpriseContextFromRelay(c, relayInfo)
		if err != nil {
			return result, nil, err
		}
	}
	if enterpriseCtx == nil || !enterpriseCtx.Enabled {
		return result, nil, nil
	}

	result.Applied = true
	result.TimeoutMs = durationMillis(enterprisePolicyQueueTimeout)
	queue := getEnterprisePolicyQueue(enterprisePolicyQueueKey(enterpriseCtx, relayInfo))
	start := time.Now()
	timer := time.NewTimer(enterprisePolicyQueueTimeout)
	defer timer.Stop()
	var requestDone <-chan struct{}
	if c.Request != nil {
		requestDone = c.Request.Context().Done()
	}
	select {
	case queue.slots <- struct{}{}:
		result.Status = enterpriseQueueStatusAdmitted
		result.WaitMs = durationMillis(time.Since(start))
		setEnterpriseQueueHeaders(c, result)
		recordEnterpriseGovernanceQueueAudit(c, enterpriseCtx, relayInfo, decision, result)
		var once sync.Once
		release := func() {
			once.Do(func() {
				<-queue.slots
			})
		}
		return result, release, nil
	case <-timer.C:
		result.Status = enterpriseQueueStatusTimeout
		result.WaitMs = durationMillis(time.Since(start))
		setEnterpriseQueueHeaders(c, result)
		recordEnterpriseGovernanceQueueAudit(c, enterpriseCtx, relayInfo, decision, result)
		logger.LogWarn(c, fmt.Sprintf("enterprise governance queue timeout after %dms", result.WaitMs))
		return result, nil, ErrEnterpriseGovernanceQueueTimeout
	case <-requestDone:
		result.Status = enterpriseQueueStatusCanceled
		result.WaitMs = durationMillis(time.Since(start))
		setEnterpriseQueueHeaders(c, result)
		recordEnterpriseGovernanceQueueAudit(c, enterpriseCtx, relayInfo, decision, result)
		return result, nil, c.Request.Context().Err()
	}
}

func hasEnterprisePolicyQueueAction(observations []PolicyActionObservation) bool {
	for _, observation := range observations {
		if observation.Action == model.PolicyActionQueue {
			return true
		}
	}
	return false
}

func getEnterprisePolicyQueue(key string) *enterprisePolicyQueue {
	if queue, ok := enterprisePolicyQueues.Load(key); ok {
		return queue.(*enterprisePolicyQueue)
	}
	maxConcurrent := enterprisePolicyQueueMaxConcurrent
	if maxConcurrent <= 0 {
		maxConcurrent = 1
	}
	queue := &enterprisePolicyQueue{
		slots: make(chan struct{}, maxConcurrent),
	}
	actual, _ := enterprisePolicyQueues.LoadOrStore(key, queue)
	return actual.(*enterprisePolicyQueue)
}

func enterprisePolicyQueueKey(enterpriseCtx *EnterpriseContext, relayInfo *relaycommon.RelayInfo) string {
	if enterpriseCtx != nil {
		if enterpriseCtx.EnterpriseId > 0 {
			return fmt.Sprintf("enterprise:%d", enterpriseCtx.EnterpriseId)
		}
		if enterpriseCtx.TokenId > 0 {
			return fmt.Sprintf("token:%d", enterpriseCtx.TokenId)
		}
		if enterpriseCtx.UserId > 0 {
			return fmt.Sprintf("user:%d", enterpriseCtx.UserId)
		}
	}
	if relayInfo != nil {
		if relayInfo.TokenId > 0 {
			return fmt.Sprintf("token:%d", relayInfo.TokenId)
		}
		if relayInfo.UserId > 0 {
			return fmt.Sprintf("user:%d", relayInfo.UserId)
		}
	}
	return "global"
}

func setEnterpriseQueueHeaders(c *gin.Context, result EnterpriseGovernanceQueueResult) {
	if c == nil || !result.Applied {
		return
	}
	c.Header(enterpriseQueueStatusHeader, result.Status)
	c.Header(enterpriseQueueWaitMsHeader, strconv.FormatInt(result.WaitMs, 10))
	c.Header(enterpriseQueueTimeoutMsHeader, strconv.FormatInt(result.TimeoutMs, 10))
}

func recordEnterpriseGovernanceQueueAudit(c *gin.Context, enterpriseCtx *EnterpriseContext, relayInfo *relaycommon.RelayInfo, decision PolicyDecision, result EnterpriseGovernanceQueueResult) {
	if enterpriseCtx == nil || !result.Applied {
		return
	}
	requestId := enterpriseRequestIdFromRelay(c, relayInfo)
	err := model.RecordEnterpriseAuditLog(model.EnterpriseAuditInput{
		EnterpriseId:   enterpriseCtx.EnterpriseId,
		ActorUserId:    enterpriseCtx.UserId,
		Action:         enterpriseGovernanceAuditActionQueueAdmission,
		TargetType:     "quota_policy",
		TargetId:       firstEnterpriseQueuePolicyActionObservationId(decision.ActionObservations),
		ScopeUserId:    enterpriseCtx.UserId,
		ScopeOrgUnitId: enterpriseCtx.PrimaryOrgUnitId,
		ScopeProjectId: enterpriseCtx.ProjectId,
		After:          enterpriseGovernanceQueueAuditPayload(c, enterpriseCtx, relayInfo, decision, result, requestId),
		RequestId:      requestId,
	})
	if err != nil {
		logger.LogError(c, "error recording enterprise governance queue audit: "+err.Error())
	}
}

func enterpriseGovernanceQueueAuditPayload(c *gin.Context, enterpriseCtx *EnterpriseContext, relayInfo *relaycommon.RelayInfo, decision PolicyDecision, result EnterpriseGovernanceQueueResult, requestId string) map[string]any {
	modelName := ""
	channelId := 0
	if relayInfo != nil {
		modelName = relayInfo.OriginModelName
		channelId = enterpriseChannelIdFromRelay(c, relayInfo)
	}
	userMessageKey := "enterprise_governance.policy_action_observed"
	if result.Status == enterpriseQueueStatusTimeout {
		userMessageKey = "enterprise_governance.queue_timeout"
	}
	return map[string]any{
		"request_id":         requestId,
		"model":              modelName,
		"channel_id":         channelId,
		"token_id":           enterpriseCtx.TokenId,
		"org_unit_id":        enterpriseCtx.PrimaryOrgUnitId,
		"project_id":         enterpriseCtx.ProjectId,
		"policy_group_ids":   cloneIntSlice(enterpriseCtx.PolicyGroupIds),
		"matched_policy_ids": cloneIntSlice(decision.MatchedPolicyIds),
		"counter_policy_ids": cloneIntSlice(decision.CounterPolicyIds),
		"policy_actions":     cloneEnterprisePolicyActionObservations(decision.ActionObservations),
		"queue_status":       result.Status,
		"wait_ms":            result.WaitMs,
		"timeout_ms":         result.TimeoutMs,
		"user_message_key":   userMessageKey,
		"dry_run":            decision.DryRun,
	}
}

func firstEnterpriseQueuePolicyActionObservationId(observations []PolicyActionObservation) int {
	for _, observation := range observations {
		if observation.Action == model.PolicyActionQueue {
			return observation.PolicyId
		}
	}
	return firstEnterprisePolicyActionObservationId(observations)
}

func durationMillis(duration time.Duration) int64 {
	return int64(duration / time.Millisecond)
}
