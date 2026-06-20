package service

import (
	"crypto/sha256"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"

	"gorm.io/gorm"
)

const EnterpriseQuotaRequestExpiringSoonOutboxWindowLabel = "24h"

type EnterpriseQuotaRequestNotificationPayload struct {
	QuotaRequestId  int               `json:"quota_request_id"`
	AuditLogId      int64             `json:"audit_log_id"`
	Action          string            `json:"action"`
	Status          string            `json:"status"`
	PolicyId        int               `json:"policy_id"`
	TargetType      string            `json:"target_type"`
	TargetId        int               `json:"target_id"`
	Metric          string            `json:"metric"`
	Period          string            `json:"period"`
	LimitDelta      int64             `json:"limit_delta"`
	ApplicantUserId int               `json:"applicant_user_id"`
	ApproverUserId  int               `json:"approver_user_id"`
	EffectiveAt     int64             `json:"effective_at"`
	ExpiresAt       int64             `json:"expires_at"`
	TitleKey        string            `json:"title_key"`
	ContentKey      string            `json:"content_key"`
	ContentParams   map[string]string `json:"content_params"`
}

func EnqueueEnterpriseQuotaRequestOutboxWithDB(tx *gorm.DB, request model.EnterpriseQuotaRequest, audit model.EnterpriseAuditLog, action string) error {
	recipients := enterpriseQuotaRequestOutboxRecipients(request, action)
	if len(recipients) == 0 {
		return nil
	}
	payload := enterpriseQuotaRequestNotificationPayload(request, audit.Id, action)
	emailByUserId, err := enterpriseQuotaRequestOutboxRecipientEmails(tx, recipients)
	if err != nil {
		return err
	}
	emailScope, emailEnabled, err := EnterpriseNotificationEmailScopeWithDB(tx, request.EnterpriseId, action)
	if err != nil {
		return err
	}
	for _, recipientUserId := range recipients {
		_, err := EnqueueEnterpriseNotificationOutboxWithDB(tx, EnterpriseNotificationOutboxInput{
			EventKey:        enterpriseQuotaRequestOutboxEventKey(action, request.Id, audit.Id, recipientUserId),
			EventType:       action,
			EnterpriseId:    request.EnterpriseId,
			RecipientUserId: recipientUserId,
			Channel:         model.EnterpriseNotificationOutboxChannelInApp,
			TargetType:      "quota_request",
			TargetId:        request.Id,
			Payload:         payload,
		})
		if err != nil {
			return err
		}
		if emailEnabled && emailScope.Applicant && enterpriseQuotaRequestUserEmailEnabled(tx, recipientUserId) {
			if email := strings.TrimSpace(emailByUserId[recipientUserId]); email != "" {
				_, err := EnqueueEnterpriseNotificationOutboxWithDB(tx, EnterpriseNotificationOutboxInput{
					EventKey:        enterpriseQuotaRequestOutboxEventKeyForRecipient(action+".email", request.Id, audit.Id, "user:"+strconv.Itoa(recipientUserId)),
					EventType:       action,
					EnterpriseId:    request.EnterpriseId,
					RecipientUserId: recipientUserId,
					RecipientEmail:  email,
					Channel:         model.EnterpriseNotificationOutboxChannelEmail,
					TargetType:      "quota_request",
					TargetId:        request.Id,
					Payload:         payload,
				})
				if err != nil {
					return err
				}
			}
		}
	}
	if emailEnabled {
		adminEmails, err := enterpriseQuotaRequestOutboxAdminEmails(tx, emailScope)
		if err != nil {
			return err
		}
		for _, email := range adminEmails {
			_, err := EnqueueEnterpriseNotificationOutboxWithDB(tx, EnterpriseNotificationOutboxInput{
				EventKey:       enterpriseQuotaRequestOutboxEventKeyForRecipient(action+".email", request.Id, audit.Id, "email:"+email),
				EventType:      action,
				EnterpriseId:   request.EnterpriseId,
				RecipientEmail: email,
				Channel:        model.EnterpriseNotificationOutboxChannelEmail,
				TargetType:     "quota_request",
				TargetId:       request.Id,
				Payload:        payload,
			})
			if err != nil {
				return err
			}
		}
	}
	webhookEnabled, err := EnterpriseNotificationChannelEnabledWithDB(tx, request.EnterpriseId, model.EnterpriseNotificationPreferenceChannelWebhook, action)
	if err != nil {
		return err
	}
	if !webhookEnabled {
		return nil
	}
	webhooks, err := ListEnabledEnterpriseWebhooksForEventWithDB(tx, request.EnterpriseId, action)
	if err != nil {
		return err
	}
	for _, webhook := range webhooks {
		_, err := EnqueueEnterpriseNotificationOutboxWithDB(tx, EnterpriseNotificationOutboxInput{
			EventKey:       enterpriseQuotaRequestOutboxEventKey(action+".webhook", request.Id, audit.Id, webhook.Id),
			EventType:      action,
			EnterpriseId:   request.EnterpriseId,
			RecipientEmail: "webhook:" + strconv.Itoa(webhook.Id),
			Channel:        model.EnterpriseNotificationOutboxChannelWebhook,
			TargetType:     "quota_request",
			TargetId:       request.Id,
			Payload:        payload,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func EnqueueExpiringSoonEnterpriseQuotaRequestOutbox(now int64, batchSize int) (int, error) {
	if now <= 0 {
		now = common.GetTimestamp()
	}
	if batchSize <= 0 {
		batchSize = EnterpriseQuotaRequestExpiryBatchSize
	}
	windowEnd := now + int64(EnterpriseQuotaRequestExpiringSoonWindow/time.Second)
	var requests []model.EnterpriseQuotaRequest
	if err := model.DB.
		Where("status = ?", model.EnterpriseQuotaRequestStatusApproved).
		Where("effective_at <= ? AND expires_at > ? AND expires_at <= ?", now, now, windowEnd).
		Order("expires_at asc, id asc").
		Limit(batchSize).
		Find(&requests).Error; err != nil {
		return 0, err
	}
	enqueued := 0
	for _, request := range requests {
		created, err := enqueueExpiringSoonEnterpriseQuotaRequestOutboxForRequest(model.DB, request)
		if err != nil {
			return enqueued, err
		}
		enqueued += created
	}
	return enqueued, nil
}

func enqueueExpiringSoonEnterpriseQuotaRequestOutboxForRequest(tx *gorm.DB, request model.EnterpriseQuotaRequest) (int, error) {
	action := "quota_request.expiring_soon"
	recipients := enterpriseQuotaRequestOutboxRecipients(request, action)
	if len(recipients) == 0 {
		return 0, nil
	}
	payload := enterpriseQuotaRequestNotificationPayload(request, 0, action)
	created := 0
	emailByUserId, err := enterpriseQuotaRequestOutboxRecipientEmails(tx, recipients)
	if err != nil {
		return created, err
	}
	emailScope, emailEnabled, err := EnterpriseNotificationEmailScopeWithDB(tx, request.EnterpriseId, action)
	if err != nil {
		return created, err
	}
	for _, recipientUserId := range recipients {
		if emailEnabled && emailScope.Applicant && enterpriseQuotaRequestUserEmailEnabled(tx, recipientUserId) {
			if email := strings.TrimSpace(emailByUserId[recipientUserId]); email != "" {
				rowCreated, err := EnqueueEnterpriseNotificationOutboxWithDB(tx, EnterpriseNotificationOutboxInput{
					EventKey:        enterpriseQuotaRequestExpiringSoonOutboxEventKeyForRecipient(request, "email", "user:"+strconv.Itoa(recipientUserId)),
					EventType:       action,
					EnterpriseId:    request.EnterpriseId,
					RecipientUserId: recipientUserId,
					RecipientEmail:  email,
					Channel:         model.EnterpriseNotificationOutboxChannelEmail,
					TargetType:      "quota_request",
					TargetId:        request.Id,
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
	}
	if emailEnabled {
		adminEmails, err := enterpriseQuotaRequestOutboxAdminEmails(tx, emailScope)
		if err != nil {
			return created, err
		}
		for _, email := range adminEmails {
			rowCreated, err := EnqueueEnterpriseNotificationOutboxWithDB(tx, EnterpriseNotificationOutboxInput{
				EventKey:       enterpriseQuotaRequestExpiringSoonOutboxEventKeyForRecipient(request, "email", "email:"+email),
				EventType:      action,
				EnterpriseId:   request.EnterpriseId,
				RecipientEmail: email,
				Channel:        model.EnterpriseNotificationOutboxChannelEmail,
				TargetType:     "quota_request",
				TargetId:       request.Id,
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
	webhookEnabled, err := EnterpriseNotificationChannelEnabledWithDB(tx, request.EnterpriseId, model.EnterpriseNotificationPreferenceChannelWebhook, action)
	if err != nil {
		return created, err
	}
	if !webhookEnabled {
		return created, nil
	}
	webhooks, err := ListEnabledEnterpriseWebhooksForEventWithDB(tx, request.EnterpriseId, action)
	if err != nil {
		return created, err
	}
	for _, webhook := range webhooks {
		rowCreated, err := EnqueueEnterpriseNotificationOutboxWithDB(tx, EnterpriseNotificationOutboxInput{
			EventKey:       enterpriseQuotaRequestExpiringSoonOutboxEventKeyForRecipient(request, "webhook", "webhook:"+strconv.Itoa(webhook.Id)),
			EventType:      action,
			EnterpriseId:   request.EnterpriseId,
			RecipientEmail: "webhook:" + strconv.Itoa(webhook.Id),
			Channel:        model.EnterpriseNotificationOutboxChannelWebhook,
			TargetType:     "quota_request",
			TargetId:       request.Id,
			Payload:        payload,
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

func enterpriseQuotaRequestNotificationPayload(request model.EnterpriseQuotaRequest, auditLogId int64, action string) EnterpriseQuotaRequestNotificationPayload {
	status := enterpriseQuotaRequestOutboxStatus(request, action)
	return EnterpriseQuotaRequestNotificationPayload{
		QuotaRequestId:  request.Id,
		AuditLogId:      auditLogId,
		Action:          action,
		Status:          status,
		PolicyId:        request.PolicyId,
		TargetType:      request.TargetType,
		TargetId:        request.TargetId,
		Metric:          request.Metric,
		Period:          request.Period,
		LimitDelta:      request.LimitDelta,
		ApplicantUserId: request.ApplicantUserId,
		ApproverUserId:  request.ApproverUserId,
		EffectiveAt:     request.EffectiveAt,
		ExpiresAt:       request.ExpiresAt,
		TitleKey:        quotaRequestNotificationTitleKey(status),
		ContentKey:      quotaRequestNotificationContentKey(status, "actor"),
		ContentParams: map[string]string{
			"policyId": strconv.Itoa(request.PolicyId),
		},
	}
}

func enterpriseQuotaRequestUserEmailEnabled(tx *gorm.DB, userId int) bool {
	if userId <= 0 {
		return false
	}
	var user model.User
	if err := tx.Select("id, setting").Where("id = ?", userId).First(&user).Error; err != nil {
		return true
	}
	settingValue := strings.TrimSpace(user.Setting)
	if settingValue == "" || !strings.Contains(settingValue, "enterprise_quota_request_email_enabled") {
		return true
	}
	setting := dto.UserSetting{}
	if err := common.Unmarshal([]byte(settingValue), &setting); err != nil {
		return true
	}
	return setting.EnterpriseQuotaRequestEmailEnabled
}

func enterpriseQuotaRequestOutboxAdminEmails(tx *gorm.DB, scope EnterpriseNotificationRecipientScope) ([]string, error) {
	emails := make([]string, 0, len(scope.ExplicitEmails))
	emails = append(emails, scope.ExplicitEmails...)
	if scope.EnterpriseAdmins {
		adminEmails, err := ListEnterpriseNotificationAdminEmailsWithDB(tx)
		if err != nil {
			return nil, err
		}
		emails = append(emails, adminEmails...)
	}
	return normalizeEnterpriseNotificationEmails(emails), nil
}

func enterpriseQuotaRequestOutboxRecipientEmails(tx *gorm.DB, recipients []int) (map[int]string, error) {
	result := map[int]string{}
	userIds := make([]int, 0, len(recipients))
	seen := map[int]struct{}{}
	for _, userId := range recipients {
		if userId <= 0 {
			continue
		}
		if _, ok := seen[userId]; ok {
			continue
		}
		seen[userId] = struct{}{}
		userIds = append(userIds, userId)
	}
	if len(userIds) == 0 {
		return result, nil
	}
	var users []model.User
	if err := tx.Select("id, email").Where("id IN ?", userIds).Find(&users).Error; err != nil {
		return nil, err
	}
	for _, user := range users {
		if strings.TrimSpace(user.Email) != "" {
			result[user.Id] = strings.TrimSpace(user.Email)
		}
	}
	return result, nil
}

func enterpriseQuotaRequestOutboxRecipients(request model.EnterpriseQuotaRequest, action string) []int {
	switch action {
	case "quota_request.submit":
		return []int{0}
	case "quota_request.approve", "quota_request.reject", "quota_request.withdraw", "quota_request.expire", "quota_request.expiring_soon":
		if request.ApplicantUserId > 0 {
			return []int{request.ApplicantUserId}
		}
	}
	return nil
}

func enterpriseQuotaRequestOutboxEventKey(action string, requestId int, auditLogId int64, recipientUserId int) string {
	return action + ":" + strconv.Itoa(requestId) + ":" + strconv.FormatInt(auditLogId, 10) + ":" + strconv.Itoa(recipientUserId)
}

func enterpriseQuotaRequestOutboxEventKeyForRecipient(action string, requestId int, auditLogId int64, recipientRef string) string {
	digest := sha256.Sum256([]byte(strings.TrimSpace(recipientRef)))
	return action + ":" + strconv.Itoa(requestId) + ":" + strconv.FormatInt(auditLogId, 10) + ":" + fmt.Sprintf("%x", digest[:8])
}

func enterpriseQuotaRequestExpiringSoonOutboxEventKeyForRecipient(request model.EnterpriseQuotaRequest, channel string, recipientRef string) string {
	digest := sha256.Sum256([]byte(strings.TrimSpace(channel) + ":" + strings.TrimSpace(recipientRef)))
	return "quota_request.expiring_soon:" + strconv.Itoa(request.Id) + ":" + strconv.FormatInt(request.ExpiresAt, 10) + ":" + EnterpriseQuotaRequestExpiringSoonOutboxWindowLabel + ":" + fmt.Sprintf("%x", digest[:8])
}

func enterpriseQuotaRequestOutboxStatus(request model.EnterpriseQuotaRequest, action string) string {
	switch action {
	case "quota_request.submit":
		return model.EnterpriseQuotaRequestStatusPending
	case "quota_request.approve":
		return model.EnterpriseQuotaRequestStatusApproved
	case "quota_request.reject":
		return model.EnterpriseQuotaRequestStatusRejected
	case "quota_request.withdraw":
		return model.EnterpriseQuotaRequestStatusWithdrawn
	case "quota_request.expire":
		return model.EnterpriseQuotaRequestStatusExpired
	case "quota_request.expiring_soon":
		return "expiring_soon"
	default:
		return request.Status
	}
}
