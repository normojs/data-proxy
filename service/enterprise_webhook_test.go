package service

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnterpriseWebhookMatchesEventAndSignature(t *testing.T) {
	setupEnterprisePolicyServiceTestDB(t)
	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.EnterpriseWebhook{
		EnterpriseId:   enterprise.Id,
		Name:           "approval webhook",
		Url:            "https://example.com/webhook",
		Secret:         "secret",
		EventTypesJson: `["quota_request.approve"]`,
		Status:         model.EnterpriseWebhookStatusEnabled,
	}).Error)
	require.NoError(t, model.DB.Create(&model.EnterpriseWebhook{
		EnterpriseId:   enterprise.Id,
		Name:           "disabled webhook",
		Url:            "https://example.com/disabled",
		EventTypesJson: `["*"]`,
		Status:         model.EnterpriseWebhookStatusDisabled,
	}).Error)

	matched, err := ListEnabledEnterpriseWebhooksForEvent(enterprise.Id, "quota_request.approve")
	require.NoError(t, err)
	require.Len(t, matched, 1)
	assert.Equal(t, "approval webhook", matched[0].Name)

	notMatched, err := ListEnabledEnterpriseWebhooksForEvent(enterprise.Id, "quota_request.reject")
	require.NoError(t, err)
	assert.Empty(t, notMatched)

	assert.Equal(t, "sha256=b82fcb791acec57859b989b430a826488ce2e479fdf92326bd0a2e8375a42ba4", EnterpriseWebhookSignature("secret", []byte("payload")))
}

func TestCreateUpdateEnterpriseWebhookSanitizesItemAndPreservesSecret(t *testing.T) {
	setupEnterprisePolicyServiceTestDB(t)
	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	secret := "initial-secret"
	webhook, err := CreateEnterpriseWebhook(enterprise.Id, EnterpriseWebhookUpsertInput{
		Name:       "Ops Hook",
		Url:        "https://example.com/hook?token=secret",
		Secret:     &secret,
		EventTypes: []string{"quota_request.approve", "quota_request.approve", ""},
	})
	require.NoError(t, err)
	assert.Equal(t, model.EnterpriseWebhookStatusEnabled, webhook.Status)
	assert.Equal(t, "initial-secret", webhook.Secret)
	item := EnterpriseWebhookToItem(webhook)
	assert.True(t, item.HasSecret)
	assert.ElementsMatch(t, []string{"quota_request.approve"}, item.EventTypes)

	before, after, err := UpdateEnterpriseWebhook(enterprise.Id, webhook.Id, EnterpriseWebhookUpsertInput{
		Name:       "Ops Hook Updated",
		Url:        "https://example.com/hook2",
		EventTypes: []string{"*"},
		Status:     model.EnterpriseWebhookStatusDisabled,
	})
	require.NoError(t, err)
	assert.Equal(t, "initial-secret", before.Secret)
	assert.Equal(t, "initial-secret", after.Secret)
	assert.Equal(t, model.EnterpriseWebhookStatusDisabled, after.Status)

	reset := "next-secret"
	_, after, err = UpdateEnterpriseWebhook(enterprise.Id, webhook.Id, EnterpriseWebhookUpsertInput{
		Name:       "Ops Hook Reset",
		Url:        "https://example.com/hook3",
		Secret:     &reset,
		EventTypes: []string{"quota_request.reject"},
		Status:     model.EnterpriseWebhookStatusEnabled,
	})
	require.NoError(t, err)
	assert.Equal(t, "next-secret", after.Secret)
}

func TestSendEnterpriseWebhookPostsSignedPayload(t *testing.T) {
	setupEnterprisePolicyServiceTestDB(t)
	disableWebhookTestSSRFProtection(t)
	var receivedSignature string
	var receivedBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedSignature = r.Header.Get("X-Enterprise-Webhook-Signature")
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		receivedBody = body
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	row := model.EnterpriseNotificationOutbox{
		EventKey:     "quota_request.approve:1:2:3",
		EventType:    "quota_request.approve",
		EnterpriseId: 1,
		TargetType:   "quota_request",
		TargetId:     7,
		PayloadJson:  `{"status":"approved"}`,
		CreatedAt:    1781900000,
	}
	err := SendEnterpriseWebhook(model.EnterpriseWebhook{
		Id:           1,
		EnterpriseId: 1,
		Name:         "test webhook",
		Url:          server.URL,
		Secret:       "secret",
		Status:       model.EnterpriseWebhookStatusEnabled,
	}, row)
	require.NoError(t, err)
	assert.NotEmpty(t, receivedBody)
	assert.Equal(t, EnterpriseWebhookSignature("secret", receivedBody), receivedSignature)
	assert.Contains(t, string(receivedBody), "quota_request.approve")
	assert.Contains(t, string(receivedBody), "quota_request")
}

func TestSendEnterpriseWebhookWithResultCapturesFailureSummary(t *testing.T) {
	setupEnterprisePolicyServiceTestDB(t)
	disableWebhookTestSSRFProtection(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("upstream unavailable"))
	}))
	defer server.Close()

	result, err := SendEnterpriseWebhookWithResult(model.EnterpriseWebhook{
		Id:           1,
		EnterpriseId: 1,
		Name:         "test webhook",
		Url:          server.URL,
		Secret:       "secret",
		Status:       model.EnterpriseWebhookStatusEnabled,
	}, model.EnterpriseNotificationOutbox{
		EventKey:     "enterprise.webhook.test:1",
		EventType:    "enterprise.webhook.test",
		EnterpriseId: 1,
		TargetType:   "enterprise_webhook",
		TargetId:     1,
		PayloadJson:  `{}`,
		CreatedAt:    1781900000,
	})
	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Equal(t, http.StatusBadGateway, result.StatusCode)
	assert.Contains(t, result.Error, "502")
	assert.Contains(t, result.Error, "upstream unavailable")
	assert.NotEmpty(t, result.SignatureHeader)
}

func TestDeliverEnterpriseWebhookOutboxUsesConfiguredWebhook(t *testing.T) {
	setupEnterprisePolicyServiceTestDB(t)
	disableWebhookTestSSRFProtection(t)
	enterprise, err := model.GetDefaultEnterprise()
	require.NoError(t, err)
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	webhook := model.EnterpriseWebhook{
		EnterpriseId:   enterprise.Id,
		Name:           "delivery webhook",
		Url:            server.URL,
		Secret:         "secret",
		EventTypesJson: `["quota_request.approve"]`,
		Status:         model.EnterpriseWebhookStatusEnabled,
	}
	require.NoError(t, model.DB.Create(&webhook).Error)

	err = DeliverEnterpriseWebhookOutbox(model.EnterpriseNotificationOutbox{
		EventKey:       "quota_request.approve:1:2:webhook",
		EventType:      "quota_request.approve",
		EnterpriseId:   enterprise.Id,
		RecipientEmail: "webhook:" + strconv.Itoa(webhook.Id),
		Channel:        model.EnterpriseNotificationOutboxChannelWebhook,
		TargetType:     "quota_request",
		TargetId:       1,
		PayloadJson:    "{}",
	})
	require.NoError(t, err)
	assert.True(t, called)
}

func disableWebhookTestSSRFProtection(t *testing.T) {
	t.Helper()
	fetchSetting := system_setting.GetFetchSetting()
	originalEnabled := fetchSetting.EnableSSRFProtection
	originalAllowPrivate := fetchSetting.AllowPrivateIp
	fetchSetting.EnableSSRFProtection = false
	fetchSetting.AllowPrivateIp = true
	t.Cleanup(func() {
		fetchSetting.EnableSSRFProtection = originalEnabled
		fetchSetting.AllowPrivateIp = originalAllowPrivate
	})
}
