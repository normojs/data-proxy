package service

import (
	"strconv"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnqueueEnterpriseNotificationOutboxIsIdempotentByEventKey(t *testing.T) {
	setupEnterprisePolicyServiceTestDB(t)
	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)

	input := EnterpriseNotificationOutboxInput{
		EventKey:        "quota_request.approve:1:2:3",
		EventType:       "quota_request.approve",
		EnterpriseId:    enterprise.Id,
		RecipientUserId: 3,
		Channel:         model.EnterpriseNotificationOutboxChannelInApp,
		TargetType:      "quota_request",
		TargetId:        1,
		Payload: map[string]any{
			"quota_request_id": 1,
			"status":           model.EnterpriseQuotaRequestStatusApproved,
		},
	}

	created, err := EnqueueEnterpriseNotificationOutbox(input)
	require.NoError(t, err)
	assert.True(t, created)
	createdAgain, err := EnqueueEnterpriseNotificationOutbox(input)
	require.NoError(t, err)
	assert.False(t, createdAgain)

	var rows []model.EnterpriseNotificationOutbox
	require.NoError(t, model.DB.Find(&rows).Error)
	require.Len(t, rows, 1)
	assert.Equal(t, input.EventKey, rows[0].EventKey)
	assert.Equal(t, model.EnterpriseNotificationOutboxStatusPending, rows[0].Status)
	assert.Contains(t, rows[0].PayloadJson, model.EnterpriseQuotaRequestStatusApproved)
}

func TestListEnterpriseNotificationOutboxFiltersAndSanitizes(t *testing.T) {
	setupEnterprisePolicyServiceTestDB(t)
	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	now := int64(1781901000)
	rows := []model.EnterpriseNotificationOutbox{
		{
			EventKey:       "webhook-failed",
			EventType:      "quota_request.approve",
			EnterpriseId:   enterprise.Id,
			RecipientEmail: "webhook:12",
			Channel:        model.EnterpriseNotificationOutboxChannelWebhook,
			TargetType:     "quota_request",
			TargetId:       101,
			PayloadJson:    `{"secret":"payload"}`,
			Status:         model.EnterpriseNotificationOutboxStatusFailed,
			RetryCount:     2,
			NextRetryAt:    now + 60,
			LastError:      "POST https://example.com/hook?token=abc&safe=1 failed",
			CreatedAt:      now,
			UpdatedAt:      now,
		},
		{
			EventKey:       "webhook-other",
			EventType:      "quota_request.reject",
			EnterpriseId:   enterprise.Id,
			RecipientEmail: "webhook:99",
			Channel:        model.EnterpriseNotificationOutboxChannelWebhook,
			TargetType:     "quota_request",
			TargetId:       102,
			PayloadJson:    `{}`,
			Status:         model.EnterpriseNotificationOutboxStatusSent,
			CreatedAt:      now - 10,
			UpdatedAt:      now - 10,
		},
		{
			EventKey:       "email-sent",
			EventType:      "quota_request.approve",
			EnterpriseId:   enterprise.Id,
			RecipientEmail: "person@example.com",
			Channel:        model.EnterpriseNotificationOutboxChannelEmail,
			TargetType:     "quota_request",
			TargetId:       101,
			PayloadJson:    `{}`,
			Status:         model.EnterpriseNotificationOutboxStatusSent,
			CreatedAt:      now - 20,
			UpdatedAt:      now - 20,
		},
	}
	require.NoError(t, model.DB.Create(&rows).Error)

	items, total, err := ListEnterpriseNotificationOutbox(EnterpriseNotificationOutboxListParams{
		EnterpriseId: enterprise.Id,
		WebhookId:    12,
		Status:       model.EnterpriseNotificationOutboxStatusFailed,
		Limit:        10,
	})
	require.NoError(t, err)
	assert.EqualValues(t, 1, total)
	require.Len(t, items, 1)
	assert.Equal(t, "webhook-failed", items[0].EventKey)
	assert.Equal(t, "webhook:12", items[0].RecipientEmail)
	assert.Contains(t, items[0].LastError, "token=redacted")
	assert.NotContains(t, items[0].LastError, "abc")

	items, total, err = ListEnterpriseNotificationOutbox(EnterpriseNotificationOutboxListParams{
		EnterpriseId: enterprise.Id,
		Channel:      model.EnterpriseNotificationOutboxChannelEmail,
		Limit:        10,
	})
	require.NoError(t, err)
	assert.EqualValues(t, 1, total)
	require.Len(t, items, 1)
	assert.Equal(t, "p***@example.com", items[0].RecipientEmail)
}

func TestRetryEnterpriseNotificationOutboxResetsFailedRows(t *testing.T) {
	setupEnterprisePolicyServiceTestDB(t)
	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	now := int64(1781903000)
	row := model.EnterpriseNotificationOutbox{
		EventKey:       "retry-me",
		EventType:      "quota_request.reject",
		EnterpriseId:   enterprise.Id,
		RecipientEmail: "retry@example.com",
		Channel:        model.EnterpriseNotificationOutboxChannelEmail,
		TargetType:     "quota_request",
		TargetId:       201,
		PayloadJson:    `{}`,
		Status:         model.EnterpriseNotificationOutboxStatusPermanentFailed,
		RetryCount:     EnterpriseNotificationOutboxMaxRetryCount,
		NextRetryAt:    now + 3600,
		LastError:      "SMTP server is not configured",
		CreatedAt:      now - 10,
		UpdatedAt:      now - 5,
	}
	require.NoError(t, model.DB.Create(&row).Error)

	before, after, err := RetryEnterpriseNotificationOutbox(enterprise.Id, row.Id)
	require.NoError(t, err)
	assert.Equal(t, model.EnterpriseNotificationOutboxStatusPermanentFailed, before.Status)
	assert.Equal(t, model.EnterpriseNotificationOutboxStatusPending, after.Status)
	assert.Equal(t, 0, after.RetryCount)
	assert.Equal(t, int64(0), after.NextRetryAt)
	assert.Empty(t, after.LastError)

	var reloaded model.EnterpriseNotificationOutbox
	require.NoError(t, model.DB.First(&reloaded, row.Id).Error)
	assert.Equal(t, model.EnterpriseNotificationOutboxStatusPending, reloaded.Status)
	assert.Equal(t, 0, reloaded.RetryCount)
	assert.Equal(t, int64(0), reloaded.NextRetryAt)
	assert.Empty(t, reloaded.LastError)

	_, _, err = RetryEnterpriseNotificationOutbox(enterprise.Id, row.Id)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not retryable")
}

func TestEnqueueEnterpriseQuotaRequestOutboxRespectsExternalNotificationPreferences(t *testing.T) {
	setupEnterprisePolicyServiceTestDB(t)
	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.User{Id: 9101, Username: "pref-applicant", Email: "Applicant@Example.com", Status: common.UserStatusEnabled, AffCode: "eg-pref-applicant"}).Error)
	require.NoError(t, model.DB.Create(&model.User{Id: 9199, Username: "pref-admin", Email: "Admin@Example.com", Role: common.RoleAdminUser, Status: common.UserStatusEnabled, AffCode: "eg-pref-admin"}).Error)
	webhook := model.EnterpriseWebhook{
		EnterpriseId:   enterprise.Id,
		Name:           "preference webhook",
		Url:            "https://example.com/preference-webhook",
		EventTypesJson: `["quota_request.approve"]`,
		Status:         model.EnterpriseWebhookStatusEnabled,
	}
	require.NoError(t, model.DB.Create(&webhook).Error)
	request := model.EnterpriseQuotaRequest{
		Id:              91001,
		EnterpriseId:    enterprise.Id,
		ApplicantUserId: 9101,
		ApproverUserId:  9199,
		PolicyId:        7,
		TargetType:      model.PolicyTargetEnterprise,
		TargetId:        enterprise.Id,
		Metric:          model.PolicyMetricRequestCount,
		Period:          model.PolicyPeriodDay,
		LimitDelta:      5,
		Status:          model.EnterpriseQuotaRequestStatusApproved,
		EffectiveAt:     time.Now().Unix(),
		ExpiresAt:       time.Now().Add(time.Hour).Unix(),
	}
	audit := model.EnterpriseAuditLog{Id: 91002, EnterpriseId: enterprise.Id, Action: "quota_request.approve", TargetType: "quota_request", TargetId: request.Id}

	require.NoError(t, EnqueueEnterpriseQuotaRequestOutboxWithDB(model.DB, request, audit, "quota_request.approve"))
	var rows []model.EnterpriseNotificationOutbox
	require.NoError(t, model.DB.Where("target_id = ?", request.Id).Order("id asc").Find(&rows).Error)
	require.Len(t, rows, 1)
	assert.Equal(t, model.EnterpriseNotificationOutboxChannelInApp, rows[0].Channel)

	applicantScope, err := common.Marshal(EnterpriseNotificationRecipientScope{Applicant: true})
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.EnterpriseNotificationPreference{
		EnterpriseId:       enterprise.Id,
		Channel:            model.EnterpriseNotificationPreferenceChannelEmail,
		EventType:          "quota_request.approve",
		Enabled:            true,
		RecipientScopeJson: string(applicantScope),
	}).Error)
	require.NoError(t, model.DB.Create(&model.EnterpriseNotificationPreference{
		EnterpriseId:       enterprise.Id,
		Channel:            model.EnterpriseNotificationPreferenceChannelWebhook,
		EventType:          "quota_request.approve",
		Enabled:            true,
		RecipientScopeJson: `{}`,
	}).Error)
	request.Id = 91003
	audit.Id = 91004
	audit.TargetId = request.Id
	require.NoError(t, EnqueueEnterpriseQuotaRequestOutboxWithDB(model.DB, request, audit, "quota_request.approve"))

	rows = nil
	require.NoError(t, model.DB.Where("target_id = ?", request.Id).Order("channel asc").Find(&rows).Error)
	require.Len(t, rows, 3)
	channels := []string{rows[0].Channel, rows[1].Channel, rows[2].Channel}
	assert.ElementsMatch(t, []string{model.EnterpriseNotificationOutboxChannelInApp, model.EnterpriseNotificationOutboxChannelEmail, model.EnterpriseNotificationOutboxChannelWebhook}, channels)
	for _, row := range rows {
		if row.Channel == model.EnterpriseNotificationOutboxChannelEmail {
			assert.Equal(t, "Applicant@Example.com", row.RecipientEmail)
			assert.NotContains(t, row.EventKey, "Applicant@Example.com")
		}
		if row.Channel == model.EnterpriseNotificationOutboxChannelWebhook {
			assert.Equal(t, "webhook:1", row.RecipientEmail)
		}
	}
}

func TestEnqueueEnterpriseQuotaRequestOutboxRespectsUserEmailPreference(t *testing.T) {
	setupEnterprisePolicyServiceTestDB(t)
	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	setting, err := common.Marshal(dto.UserSetting{EnterpriseQuotaRequestEmailEnabled: false})
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.User{
		Id:       9151,
		Username: "pref-disabled-applicant",
		Email:    "pref-disabled@example.com",
		Status:   common.UserStatusEnabled,
		AffCode:  "eg-pref-disabled-applicant",
		Setting:  string(setting),
	}).Error)
	scopeJson, err := common.Marshal(EnterpriseNotificationRecipientScope{Applicant: true})
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.EnterpriseNotificationPreference{
		EnterpriseId:       enterprise.Id,
		Channel:            model.EnterpriseNotificationPreferenceChannelEmail,
		EventType:          "quota_request.approve",
		Enabled:            true,
		RecipientScopeJson: string(scopeJson),
	}).Error)
	request := model.EnterpriseQuotaRequest{
		Id:              91501,
		EnterpriseId:    enterprise.Id,
		ApplicantUserId: 9151,
		PolicyId:        7,
		TargetType:      model.PolicyTargetEnterprise,
		TargetId:        enterprise.Id,
		Metric:          model.PolicyMetricRequestCount,
		Period:          model.PolicyPeriodDay,
		LimitDelta:      5,
		Status:          model.EnterpriseQuotaRequestStatusApproved,
		EffectiveAt:     time.Now().Unix(),
		ExpiresAt:       time.Now().Add(time.Hour).Unix(),
	}
	audit := model.EnterpriseAuditLog{Id: 91502, EnterpriseId: enterprise.Id, Action: "quota_request.approve", TargetType: "quota_request", TargetId: request.Id}

	require.NoError(t, EnqueueEnterpriseQuotaRequestOutboxWithDB(model.DB, request, audit, "quota_request.approve"))
	var rows []model.EnterpriseNotificationOutbox
	require.NoError(t, model.DB.Where("target_id = ?", request.Id).Order("id asc").Find(&rows).Error)
	require.Len(t, rows, 1)
	assert.Equal(t, model.EnterpriseNotificationOutboxChannelInApp, rows[0].Channel)
}

func TestEnqueueEnterpriseQuotaRequestOutboxEmailsAdminScopeOnSubmit(t *testing.T) {
	setupEnterprisePolicyServiceTestDB(t)
	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.User{Id: 9201, Username: "submit-applicant", Email: "submit-applicant@example.com", Status: common.UserStatusEnabled, AffCode: "eg-submit-applicant"}).Error)
	require.NoError(t, model.DB.Create(&model.User{Id: 9299, Username: "submit-admin", Email: "Submit-Admin@Example.com", Role: common.RoleAdminUser, Status: common.UserStatusEnabled, AffCode: "eg-submit-admin"}).Error)
	scopeJson, err := common.Marshal(EnterpriseNotificationRecipientScope{
		EnterpriseAdmins: true,
		ExplicitEmails:   []string{"desk@example.com", "DESK@example.com", ""},
	})
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.EnterpriseNotificationPreference{
		EnterpriseId:       enterprise.Id,
		Channel:            model.EnterpriseNotificationPreferenceChannelEmail,
		EventType:          "quota_request.submit",
		Enabled:            true,
		RecipientScopeJson: string(scopeJson),
	}).Error)
	request := model.EnterpriseQuotaRequest{
		Id:              92001,
		EnterpriseId:    enterprise.Id,
		ApplicantUserId: 9201,
		PolicyId:        8,
		TargetType:      model.PolicyTargetEnterprise,
		TargetId:        enterprise.Id,
		Metric:          model.PolicyMetricRequestCount,
		Period:          model.PolicyPeriodDay,
		LimitDelta:      3,
		Status:          model.EnterpriseQuotaRequestStatusPending,
		ExpiresAt:       time.Now().Add(time.Hour).Unix(),
	}
	audit := model.EnterpriseAuditLog{Id: 92002, EnterpriseId: enterprise.Id, Action: "quota_request.submit", TargetType: "quota_request", TargetId: request.Id}

	require.NoError(t, EnqueueEnterpriseQuotaRequestOutboxWithDB(model.DB, request, audit, "quota_request.submit"))
	var emailRows []model.EnterpriseNotificationOutbox
	require.NoError(t, model.DB.Where("target_id = ? AND channel = ?", request.Id, model.EnterpriseNotificationOutboxChannelEmail).Order("recipient_email asc").Find(&emailRows).Error)
	require.Len(t, emailRows, 2)
	assert.Equal(t, "desk@example.com", emailRows[0].RecipientEmail)
	assert.Equal(t, "submit-admin@example.com", emailRows[1].RecipientEmail)
}

func TestEnqueueExpiringSoonEnterpriseQuotaRequestOutboxIsWindowedAndIdempotent(t *testing.T) {
	setupEnterprisePolicyServiceTestDB(t)
	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.User{Id: 9301, Username: "expiring-applicant", Email: "expiring@example.com", Status: common.UserStatusEnabled, AffCode: "eg-expiring-applicant"}).Error)
	webhook := model.EnterpriseWebhook{
		EnterpriseId:   enterprise.Id,
		Name:           "expiring webhook",
		Url:            "https://example.com/expiring-webhook",
		EventTypesJson: `["quota_request.expiring_soon"]`,
		Status:         model.EnterpriseWebhookStatusEnabled,
	}
	require.NoError(t, model.DB.Create(&webhook).Error)
	applicantScope, err := common.Marshal(EnterpriseNotificationRecipientScope{Applicant: true})
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.EnterpriseNotificationPreference{
		EnterpriseId:       enterprise.Id,
		Channel:            model.EnterpriseNotificationPreferenceChannelEmail,
		EventType:          "quota_request.expiring_soon",
		Enabled:            true,
		RecipientScopeJson: string(applicantScope),
	}).Error)
	require.NoError(t, model.DB.Create(&model.EnterpriseNotificationPreference{
		EnterpriseId:       enterprise.Id,
		Channel:            model.EnterpriseNotificationPreferenceChannelWebhook,
		EventType:          "quota_request.expiring_soon",
		Enabled:            true,
		RecipientScopeJson: `{}`,
	}).Error)
	now := int64(1781902000)
	notSoon := model.EnterpriseQuotaRequest{
		EnterpriseId:    enterprise.Id,
		ApplicantUserId: 9301,
		ApproverUserId:  9399,
		PolicyId:        9,
		TargetType:      model.PolicyTargetEnterprise,
		TargetId:        enterprise.Id,
		Metric:          model.PolicyMetricRequestCount,
		Period:          model.PolicyPeriodDay,
		LimitDelta:      5,
		Status:          model.EnterpriseQuotaRequestStatusApproved,
		EffectiveAt:     now - 3600,
		ExpiresAt:       now + int64(48*time.Hour/time.Second),
	}
	soon := model.EnterpriseQuotaRequest{
		EnterpriseId:    enterprise.Id,
		ApplicantUserId: 9301,
		ApproverUserId:  9399,
		PolicyId:        10,
		TargetType:      model.PolicyTargetEnterprise,
		TargetId:        enterprise.Id,
		Metric:          model.PolicyMetricRequestCount,
		Period:          model.PolicyPeriodDay,
		LimitDelta:      7,
		Status:          model.EnterpriseQuotaRequestStatusApproved,
		EffectiveAt:     now - 3600,
		ExpiresAt:       now + 3600,
	}
	require.NoError(t, model.DB.Create(&notSoon).Error)
	require.NoError(t, model.DB.Create(&soon).Error)

	created, err := EnqueueExpiringSoonEnterpriseQuotaRequestOutbox(now, 10)
	require.NoError(t, err)
	assert.Equal(t, 2, created)
	var rows []model.EnterpriseNotificationOutbox
	require.NoError(t, model.DB.Where("event_type = ?", "quota_request.expiring_soon").Order("channel asc").Find(&rows).Error)
	require.Len(t, rows, 2)
	for _, row := range rows {
		assert.Equal(t, soon.Id, row.TargetId)
		assert.Contains(t, row.EventKey, strconv.Itoa(soon.Id))
		assert.Contains(t, row.EventKey, strconv.FormatInt(soon.ExpiresAt, 10))
		assert.Contains(t, row.EventKey, EnterpriseQuotaRequestExpiringSoonOutboxWindowLabel)
		assert.Contains(t, row.PayloadJson, "expiring_soon")
	}

	createdAgain, err := EnqueueExpiringSoonEnterpriseQuotaRequestOutbox(now, 10)
	require.NoError(t, err)
	assert.Equal(t, 0, createdAgain)
	var count int64
	require.NoError(t, model.DB.Model(&model.EnterpriseNotificationOutbox{}).Where("event_type = ?", "quota_request.expiring_soon").Count(&count).Error)
	assert.EqualValues(t, 2, count)

	newExpiresAt := now + 7200
	require.NoError(t, model.DB.Model(&model.EnterpriseQuotaRequest{}).Where("id = ?", soon.Id).Update("expires_at", newExpiresAt).Error)
	createdAfterExtension, err := EnqueueExpiringSoonEnterpriseQuotaRequestOutbox(now, 10)
	require.NoError(t, err)
	assert.Equal(t, 2, createdAfterExtension)
	require.NoError(t, model.DB.Model(&model.EnterpriseNotificationOutbox{}).Where("event_type = ?", "quota_request.expiring_soon").Count(&count).Error)
	assert.EqualValues(t, 4, count)
}

func TestDeliverEnterpriseNotificationOutboxEmailUsesConfiguredSender(t *testing.T) {
	setupEnterprisePolicyServiceTestDB(t)
	originalSMTPServer := common.SMTPServer
	originalSMTPAccount := common.SMTPAccount
	originalSendEmail := enterpriseNotificationOutboxSendEmail
	common.SMTPServer = "smtp.example.com"
	common.SMTPAccount = "sender@example.com"
	t.Cleanup(func() {
		common.SMTPServer = originalSMTPServer
		common.SMTPAccount = originalSMTPAccount
		enterpriseNotificationOutboxSendEmail = originalSendEmail
	})

	var sentSubject string
	var sentReceiver string
	var sentContent string
	enterpriseNotificationOutboxSendEmail = func(subject string, receiver string, content string) error {
		sentSubject = subject
		sentReceiver = receiver
		sentContent = content
		return nil
	}

	err := DeliverEnterpriseNotificationOutbox(model.EnterpriseNotificationOutbox{
		EventType:      "quota_request.approve",
		RecipientEmail: "applicant@example.com",
		Channel:        model.EnterpriseNotificationOutboxChannelEmail,
		TargetType:     "quota_request",
		TargetId:       99,
		PayloadJson:    "{}",
	})
	require.NoError(t, err)
	assert.Equal(t, "Quota request approved", sentSubject)
	assert.Equal(t, "applicant@example.com", sentReceiver)
	assert.Contains(t, sentContent, "quota_request.approve")
	assert.Contains(t, sentContent, "quota_request #99")
}

func TestDeliverEnterpriseNotificationOutboxEmailRequiresSMTPAndReceiver(t *testing.T) {
	setupEnterprisePolicyServiceTestDB(t)
	originalSMTPServer := common.SMTPServer
	originalSMTPAccount := common.SMTPAccount
	common.SMTPServer = ""
	common.SMTPAccount = ""
	t.Cleanup(func() {
		common.SMTPServer = originalSMTPServer
		common.SMTPAccount = originalSMTPAccount
	})

	err := DeliverEnterpriseNotificationOutbox(model.EnterpriseNotificationOutbox{
		EventType:      "quota_request.reject",
		RecipientEmail: "applicant@example.com",
		Channel:        model.EnterpriseNotificationOutboxChannelEmail,
		TargetType:     "quota_request",
		TargetId:       100,
		PayloadJson:    "{}",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "SMTP")

	common.SMTPServer = "smtp.example.com"
	err = DeliverEnterpriseNotificationOutbox(model.EnterpriseNotificationOutbox{
		EventType:   "quota_request.reject",
		Channel:     model.EnterpriseNotificationOutboxChannelEmail,
		TargetType:  "quota_request",
		TargetId:    100,
		PayloadJson: "{}",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "receiver")
}

func TestProcessEnterpriseNotificationOutboxBatchMarksInAppSentAndRetriesFailures(t *testing.T) {
	setupEnterprisePolicyServiceTestDB(t)
	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	now := int64(1781900000)
	rows := []model.EnterpriseNotificationOutbox{
		{
			EventKey:        "in-app-due",
			EventType:       "quota_request.approve",
			EnterpriseId:    enterprise.Id,
			RecipientUserId: 1,
			Channel:         model.EnterpriseNotificationOutboxChannelInApp,
			TargetType:      "quota_request",
			TargetId:        11,
			PayloadJson:     "{}",
			Status:          model.EnterpriseNotificationOutboxStatusPending,
			NextRetryAt:     0,
			CreatedAt:       now - 10,
			UpdatedAt:       now - 10,
		},
		{
			EventKey:        "email-due",
			EventType:       "quota_request.approve",
			EnterpriseId:    enterprise.Id,
			RecipientUserId: 2,
			Channel:         model.EnterpriseNotificationOutboxChannelEmail,
			TargetType:      "quota_request",
			TargetId:        12,
			PayloadJson:     "{}",
			Status:          model.EnterpriseNotificationOutboxStatusPending,
			NextRetryAt:     0,
			CreatedAt:       now - 10,
			UpdatedAt:       now - 10,
		},
		{
			EventKey:        "email-permanent",
			EventType:       "quota_request.reject",
			EnterpriseId:    enterprise.Id,
			RecipientUserId: 4,
			Channel:         model.EnterpriseNotificationOutboxChannelEmail,
			TargetType:      "quota_request",
			TargetId:        16,
			PayloadJson:     "{}",
			Status:          model.EnterpriseNotificationOutboxStatusPending,
			RetryCount:      EnterpriseNotificationOutboxMaxRetryCount - 1,
			NextRetryAt:     0,
			CreatedAt:       now - 10,
			UpdatedAt:       now - 10,
		},
		{
			EventKey:        "email-later",
			EventType:       "quota_request.approve",
			EnterpriseId:    enterprise.Id,
			RecipientUserId: 3,
			Channel:         model.EnterpriseNotificationOutboxChannelEmail,
			TargetType:      "quota_request",
			TargetId:        13,
			PayloadJson:     "{}",
			Status:          model.EnterpriseNotificationOutboxStatusFailed,
			RetryCount:      1,
			NextRetryAt:     now + 3600,
			CreatedAt:       now - 10,
			UpdatedAt:       now - 10,
		},
	}
	require.NoError(t, model.DB.Create(&rows).Error)

	claimed, err := ClaimDueEnterpriseNotificationOutbox(10, now)
	require.NoError(t, err)
	require.Len(t, claimed, 3)
	assert.Equal(t, "in-app-due", claimed[0].EventKey)
	assert.Equal(t, "email-due", claimed[1].EventKey)
	assert.Equal(t, "email-permanent", claimed[2].EventKey)

	require.NoError(t, MarkEnterpriseNotificationOutboxSent(claimed[0].Id, now+1))
	require.NoError(t, MarkEnterpriseNotificationOutboxFailed(claimed[1].Id, assert.AnError, now+1))
	require.NoError(t, MarkEnterpriseNotificationOutboxFailed(claimed[2].Id, assert.AnError, now+1))

	var inApp model.EnterpriseNotificationOutbox
	require.NoError(t, model.DB.Where("event_key = ?", "in-app-due").First(&inApp).Error)
	assert.Equal(t, model.EnterpriseNotificationOutboxStatusSent, inApp.Status)
	assert.Equal(t, int64(0), inApp.NextRetryAt)

	var failed model.EnterpriseNotificationOutbox
	require.NoError(t, model.DB.Where("event_key = ?", "email-due").First(&failed).Error)
	assert.Equal(t, model.EnterpriseNotificationOutboxStatusFailed, failed.Status)
	assert.Equal(t, 1, failed.RetryCount)
	assert.Greater(t, failed.NextRetryAt, now)
	assert.Contains(t, failed.LastError, assert.AnError.Error())

	var later model.EnterpriseNotificationOutbox
	require.NoError(t, model.DB.Where("event_key = ?", "email-later").First(&later).Error)
	assert.Equal(t, model.EnterpriseNotificationOutboxStatusFailed, later.Status)

	var permanent model.EnterpriseNotificationOutbox
	require.NoError(t, model.DB.Where("event_key = ?", "email-permanent").First(&permanent).Error)
	assert.Equal(t, model.EnterpriseNotificationOutboxStatusPermanentFailed, permanent.Status)
}

func TestProcessEnterpriseNotificationOutboxBatchWithStatsRecordsMetrics(t *testing.T) {
	setupEnterprisePolicyServiceTestDB(t)
	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	now := int64(1781904000)
	require.NoError(t, model.DB.Create(&[]model.EnterpriseNotificationOutbox{
		{
			EventKey:        "stats-in-app",
			EventType:       "quota_request.approve",
			EnterpriseId:    enterprise.Id,
			RecipientUserId: 10,
			Channel:         model.EnterpriseNotificationOutboxChannelInApp,
			TargetType:      "quota_request",
			TargetId:        21,
			PayloadJson:     "{}",
			Status:          model.EnterpriseNotificationOutboxStatusPending,
			NextRetryAt:     0,
			CreatedAt:       now,
			UpdatedAt:       now,
		},
		{
			EventKey:        "stats-email-fail",
			EventType:       "quota_request.reject",
			EnterpriseId:    enterprise.Id,
			RecipientUserId: 11,
			Channel:         model.EnterpriseNotificationOutboxChannelEmail,
			TargetType:      "quota_request",
			TargetId:        22,
			PayloadJson:     "{}",
			Status:          model.EnterpriseNotificationOutboxStatusPending,
			RetryCount:      EnterpriseNotificationOutboxMaxRetryCount - 1,
			NextRetryAt:     0,
			CreatedAt:       now,
			UpdatedAt:       now,
		},
	}).Error)

	stats, err := ProcessEnterpriseNotificationOutboxBatchWithStats(10)
	require.NoError(t, err)
	assert.Equal(t, 2, stats.Claimed)
	assert.Equal(t, 1, stats.Sent)
	assert.Equal(t, 1, stats.Failed)
	assert.Equal(t, 1, stats.PermanentFailed)
	assert.GreaterOrEqual(t, stats.DurationMs, int64(0))
	assert.Greater(t, stats.StartedAt, int64(0))
	assert.Greater(t, stats.FinishedAt, int64(0))

	metrics := GetEnterpriseNotificationOutboxWorkerMetrics()
	assert.Equal(t, stats, metrics.LastRun)
	assert.GreaterOrEqual(t, metrics.TotalRuns, int64(1))
	assert.GreaterOrEqual(t, metrics.TotalClaimed, int64(2))
	assert.GreaterOrEqual(t, metrics.TotalSent, int64(1))
	assert.GreaterOrEqual(t, metrics.TotalFailed, int64(1))
	assert.GreaterOrEqual(t, metrics.TotalPermanentFailed, int64(1))
}

func TestMarkEnterpriseNotificationOutboxFailedPermanentAfterMaxRetries(t *testing.T) {
	setupEnterprisePolicyServiceTestDB(t)
	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	now := int64(1781900000)
	row := model.EnterpriseNotificationOutbox{
		EventKey:        "email-final",
		EventType:       "quota_request.reject",
		EnterpriseId:    enterprise.Id,
		RecipientUserId: 4,
		Channel:         model.EnterpriseNotificationOutboxChannelEmail,
		TargetType:      "quota_request",
		TargetId:        14,
		PayloadJson:     "{}",
		Status:          model.EnterpriseNotificationOutboxStatusProcessing,
		RetryCount:      EnterpriseNotificationOutboxMaxRetryCount - 1,
		NextRetryAt:     0,
		CreatedAt:       now - 10,
		UpdatedAt:       now - 10,
	}
	require.NoError(t, model.DB.Create(&row).Error)

	require.NoError(t, MarkEnterpriseNotificationOutboxFailed(row.Id, assert.AnError, now))

	var reloaded model.EnterpriseNotificationOutbox
	require.NoError(t, model.DB.First(&reloaded, row.Id).Error)
	assert.Equal(t, model.EnterpriseNotificationOutboxStatusPermanentFailed, reloaded.Status)
	assert.Equal(t, EnterpriseNotificationOutboxMaxRetryCount, reloaded.RetryCount)
	assert.Equal(t, int64(0), reloaded.NextRetryAt)
}

func TestProcessEnterpriseNotificationOutboxBatchDeliversInApp(t *testing.T) {
	setupEnterprisePolicyServiceTestDB(t)
	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.EnterpriseNotificationOutbox{
		EventKey:        "batch-in-app",
		EventType:       "quota_request.approve",
		EnterpriseId:    enterprise.Id,
		RecipientUserId: 5,
		Channel:         model.EnterpriseNotificationOutboxChannelInApp,
		TargetType:      "quota_request",
		TargetId:        15,
		PayloadJson:     "{}",
		Status:          model.EnterpriseNotificationOutboxStatusPending,
	}).Error)

	sent, failed, err := ProcessEnterpriseNotificationOutboxBatch(10)
	require.NoError(t, err)
	assert.Equal(t, 1, sent)
	assert.Equal(t, 0, failed)

	var row model.EnterpriseNotificationOutbox
	require.NoError(t, model.DB.Where("event_key = ?", "batch-in-app").First(&row).Error)
	assert.Equal(t, model.EnterpriseNotificationOutboxStatusSent, row.Status)
}
