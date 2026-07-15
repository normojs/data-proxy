package service

import (
	"crypto/sha256"
	"fmt"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"gorm.io/gorm"
)

const EnterprisePolicyAlertEventType = "policy.alert"

type EnterprisePolicyAlertNotificationPayload struct {
	RequestId     string            `json:"request_id"`
	PolicyId      int               `json:"policy_id"`
	Action        string            `json:"action"`
	Trigger       string            `json:"trigger"`
	Model         string            `json:"model"`
	Ability       string            `json:"ability"`
	ChannelId     int               `json:"channel_id"`
	TokenId       int               `json:"token_id"`
	UserId        int               `json:"user_id"`
	OrgUnitId     int               `json:"org_unit_id"`
	ProjectId     int               `json:"project_id"`
	DryRun        bool              `json:"dry_run"`
	TitleKey      string            `json:"title_key"`
	ContentKey    string            `json:"content_key"`
	ContentParams map[string]string `json:"content_params"`
}

// EnqueueEnterprisePolicyAlertOutbox writes deduped in-app/email/webhook outbox
// rows for non-dry-run policy alert observations. Dry-run callers must not call this.
func EnqueueEnterprisePolicyAlertOutbox(
	enterpriseCtx *EnterpriseContext,
	req PolicyEvaluationRequest,
	decision PolicyDecision,
) (int, error) {
	if enterpriseCtx == nil || decision.DryRun {
		return 0, nil
	}
	return EnqueueEnterprisePolicyAlertOutboxWithDB(model.DB, enterpriseCtx, req, decision)
}

func EnqueueEnterprisePolicyAlertOutboxWithDB(
	tx *gorm.DB,
	enterpriseCtx *EnterpriseContext,
	req PolicyEvaluationRequest,
	decision PolicyDecision,
) (int, error) {
	if tx == nil || enterpriseCtx == nil || decision.DryRun {
		return 0, nil
	}
	created := 0
	for _, observation := range decision.ActionObservations {
		if observation.Action != model.PolicyActionAlert {
			continue
		}
		n, err := enqueueEnterprisePolicyAlertOutboxForObservation(tx, enterpriseCtx, req, decision, observation)
		if err != nil {
			return created, err
		}
		created += n
	}
	return created, nil
}

func enqueueEnterprisePolicyAlertOutboxForObservation(
	tx *gorm.DB,
	enterpriseCtx *EnterpriseContext,
	req PolicyEvaluationRequest,
	decision PolicyDecision,
	observation PolicyActionObservation,
) (int, error) {
	action := EnterprisePolicyAlertEventType
	payload := enterprisePolicyAlertNotificationPayload(enterpriseCtx, req, decision, observation)
	created := 0

	recipients, err := listEnterpriseNotificationAdminUserIdsWithDB(tx)
	if err != nil {
		return created, err
	}
	if enterpriseCtx.UserId > 0 {
		recipients = append(recipients, enterpriseCtx.UserId)
	}
	recipients = uniquePositiveInts(recipients)

	for _, recipientUserId := range recipients {
		rowCreated, err := EnqueueEnterpriseNotificationOutboxWithDB(tx, EnterpriseNotificationOutboxInput{
			EventKey:        enterprisePolicyAlertOutboxEventKey(req.RequestId, observation.PolicyId, "in_app", "user:"+strconv.Itoa(recipientUserId)),
			EventType:       action,
			EnterpriseId:    enterpriseCtx.EnterpriseId,
			RecipientUserId: recipientUserId,
			Channel:         model.EnterpriseNotificationOutboxChannelInApp,
			TargetType:      "quota_policy",
			TargetId:        observation.PolicyId,
			Payload:         payload,
		})
		if err != nil {
			return created, err
		}
		if rowCreated {
			created++
		}
	}

	emailScope, emailEnabled, err := EnterpriseNotificationEmailScopeWithDB(tx, enterpriseCtx.EnterpriseId, action)
	if err != nil {
		return created, err
	}
	if emailEnabled {
		emailByUserId, err := enterpriseQuotaRequestOutboxRecipientEmails(tx, recipients)
		if err != nil {
			return created, err
		}
		if emailScope.Applicant && enterpriseCtx.UserId > 0 {
			if email := strings.TrimSpace(emailByUserId[enterpriseCtx.UserId]); email != "" {
				rowCreated, err := EnqueueEnterpriseNotificationOutboxWithDB(tx, EnterpriseNotificationOutboxInput{
					EventKey:        enterprisePolicyAlertOutboxEventKey(req.RequestId, observation.PolicyId, "email", "user:"+strconv.Itoa(enterpriseCtx.UserId)),
					EventType:       action,
					EnterpriseId:    enterpriseCtx.EnterpriseId,
					RecipientUserId: enterpriseCtx.UserId,
					RecipientEmail:  email,
					Channel:         model.EnterpriseNotificationOutboxChannelEmail,
					TargetType:      "quota_policy",
					TargetId:        observation.PolicyId,
					Payload:         payload,
				})
				if err != nil {
					return created, err
				}
				if rowCreated {
					created++
				}
			}
		}
		adminEmails, err := enterpriseQuotaRequestOutboxAdminEmails(tx, emailScope)
		if err != nil {
			return created, err
		}
		for _, email := range adminEmails {
			rowCreated, err := EnqueueEnterpriseNotificationOutboxWithDB(tx, EnterpriseNotificationOutboxInput{
				EventKey:       enterprisePolicyAlertOutboxEventKey(req.RequestId, observation.PolicyId, "email", "email:"+email),
				EventType:      action,
				EnterpriseId:   enterpriseCtx.EnterpriseId,
				RecipientEmail: email,
				Channel:        model.EnterpriseNotificationOutboxChannelEmail,
				TargetType:     "quota_policy",
				TargetId:       observation.PolicyId,
				Payload:        payload,
			})
			if err != nil {
				return created, err
			}
			if rowCreated {
				created++
			}
		}
	}

	webhookEnabled, err := EnterpriseNotificationChannelEnabledWithDB(tx, enterpriseCtx.EnterpriseId, model.EnterpriseNotificationPreferenceChannelWebhook, action)
	if err != nil {
		return created, err
	}
	if webhookEnabled {
		rowCreated, err := EnqueueEnterpriseNotificationOutboxWithDB(tx, EnterpriseNotificationOutboxInput{
			EventKey:     enterprisePolicyAlertOutboxEventKey(req.RequestId, observation.PolicyId, "webhook", "enterprise"),
			EventType:    action,
			EnterpriseId: enterpriseCtx.EnterpriseId,
			Channel:      model.EnterpriseNotificationOutboxChannelWebhook,
			TargetType:   "quota_policy",
			TargetId:     observation.PolicyId,
			Payload:      payload,
		})
		if err != nil {
			return created, err
		}
		if rowCreated {
			created++
		}
	}
	return created, nil
}

func enterprisePolicyAlertNotificationPayload(
	enterpriseCtx *EnterpriseContext,
	req PolicyEvaluationRequest,
	decision PolicyDecision,
	observation PolicyActionObservation,
) EnterprisePolicyAlertNotificationPayload {
	policyId := observation.PolicyId
	return EnterprisePolicyAlertNotificationPayload{
		RequestId:  req.RequestId,
		PolicyId:   policyId,
		Action:     observation.Action,
		Trigger:    observation.Trigger,
		Model:      req.ModelName,
		Ability:    req.Ability,
		ChannelId:  req.ChannelId,
		TokenId:    enterpriseCtx.TokenId,
		UserId:     enterpriseCtx.UserId,
		OrgUnitId:  enterpriseCtx.PrimaryOrgUnitId,
		ProjectId:  enterpriseCtx.ProjectId,
		DryRun:     decision.DryRun,
		TitleKey:   "notification.enterprise_policy_alert.title",
		ContentKey: "notification.enterprise_policy_alert.content",
		ContentParams: map[string]string{
			"policyId":  strconv.Itoa(policyId),
			"requestId": req.RequestId,
			"trigger":   observation.Trigger,
			"model":     req.ModelName,
		},
	}
}

func enterprisePolicyAlertOutboxEventKey(requestId string, policyId int, channel string, recipientRef string) string {
	digest := sha256.Sum256([]byte(strings.TrimSpace(channel) + ":" + strings.TrimSpace(recipientRef)))
	return EnterprisePolicyAlertEventType + ":" + strings.TrimSpace(requestId) + ":" + strconv.Itoa(policyId) + ":" + fmt.Sprintf("%x", digest[:8])
}

func listEnterpriseNotificationAdminUserIdsWithDB(db *gorm.DB) ([]int, error) {
	if db == nil {
		db = model.DB
	}
	var users []model.User
	if err := db.Select("id").Where("status = ? AND role >= ?", common.UserStatusEnabled, common.RoleAdminUser).Find(&users).Error; err != nil {
		return nil, err
	}
	ids := make([]int, 0, len(users))
	for _, user := range users {
		if user.Id > 0 {
			ids = append(ids, user.Id)
		}
	}
	return ids, nil
}

func uniquePositiveInts(values []int) []int {
	result := make([]int, 0, len(values))
	seen := map[int]struct{}{}
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
