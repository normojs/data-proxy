package service

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupConnectedAppNotificationServiceTestDB(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(
		&model.User{},
		&model.ConnectedApp{},
		&model.ConnectedAppRequest{},
		&model.ConnectedAppAuditLog{},
		&model.ConnectedAppDeviceSession{},
		&model.ConnectedAppNotificationPreference{},
		&model.ConnectedAppWebhook{},
		&model.ConnectedAppNotificationOutbox{},
	))
	originalDB := model.DB
	model.DB = db
	t.Cleanup(func() {
		model.DB = originalDB
		_ = sqlDB.Close()
	})
}

func TestEnqueueConnectedAppRequestReviewOutboxRespectsPreferencesAndIsIdempotent(t *testing.T) {
	setupConnectedAppNotificationServiceTestDB(t)
	require.NoError(t, model.DB.Create(&model.User{
		Id:       9501,
		Username: "connected-app-applicant",
		Email:    "connected-app-applicant@example.com",
		Status:   common.UserStatusEnabled,
		AffCode:  "connected-app-applicant",
	}).Error)
	app := model.ConnectedApp{
		Id:                9510,
		Slug:              "review-app",
		Name:              "Review App",
		AllowedScopes:     "openai.chat",
		DefaultScopes:     "openai.chat",
		AuthorizationFlow: model.ConnectedAppAuthorizationFlowDeviceCode,
		Trusted:           true,
		Status:            model.ConnectedAppStatusEnabled,
	}
	require.NoError(t, model.DB.Create(&app).Error)
	request := model.ConnectedAppRequest{
		Id:              9520,
		ApplicantUserId: 9501,
		AppId:           app.Id,
		Slug:            app.Slug,
		Name:            app.Name,
		Status:          model.ConnectedAppRequestStatusApproved,
		ReviewerUserId:  9502,
	}
	audit := model.ConnectedAppAuditLog{
		Id:         9530,
		Action:     model.ConnectedAppAuditActionApprove,
		TargetType: model.ConnectedAppAuditTargetRequest,
		TargetId:   request.Id,
	}
	require.NoError(t, model.DB.Create(&model.ConnectedAppNotificationPreference{
		AppId:              0,
		Channel:            model.ConnectedAppNotificationOutboxChannelEmail,
		EventType:          model.ConnectedAppAuditActionApprove,
		Enabled:            true,
		RecipientScopeJson: `{"applicant":true}`,
	}).Error)
	require.NoError(t, model.DB.Create(&model.ConnectedAppNotificationPreference{
		AppId:              0,
		Channel:            model.ConnectedAppNotificationOutboxChannelWebhook,
		EventType:          model.ConnectedAppAuditActionApprove,
		Enabled:            true,
		RecipientScopeJson: `{}`,
	}).Error)
	require.NoError(t, model.DB.Create(&model.ConnectedAppWebhook{
		AppId:          0,
		Name:           "review webhook",
		Url:            "https://example.com/review",
		EventTypesJson: `["connected_app_request.approve"]`,
		Status:         model.ConnectedAppWebhookStatusEnabled,
	}).Error)

	require.NoError(t, EnqueueConnectedAppRequestReviewOutboxWithDB(model.DB, request, app, audit))
	require.NoError(t, EnqueueConnectedAppRequestReviewOutboxWithDB(model.DB, request, app, audit))

	var rows []model.ConnectedAppNotificationOutbox
	require.NoError(t, model.DB.Order("channel asc").Find(&rows).Error)
	require.Len(t, rows, 2)
	channels := []string{rows[0].Channel, rows[1].Channel}
	assert.ElementsMatch(t, []string{model.ConnectedAppNotificationOutboxChannelEmail, model.ConnectedAppNotificationOutboxChannelWebhook}, channels)
	for _, row := range rows {
		assert.Equal(t, app.Id, row.AppId)
		assert.Equal(t, model.ConnectedAppAuditActionApprove, row.EventType)
		assert.Equal(t, model.ConnectedAppNotificationOutboxStatusPending, row.Status)
		assert.Contains(t, row.PayloadJson, ConnectedAppWebhookPayloadVersion)
		if row.Channel == model.ConnectedAppNotificationOutboxChannelEmail {
			assert.Equal(t, "connected-app-applicant@example.com", row.RecipientEmail)
		}
		if row.Channel == model.ConnectedAppNotificationOutboxChannelWebhook {
			assert.Equal(t, "webhook:1", row.RecipientEmail)
		}
	}
}

func TestSendConnectedAppWebhookWithSignatureAndPayloadVersion(t *testing.T) {
	setupConnectedAppNotificationServiceTestDB(t)
	fetchSetting := system_setting.GetFetchSetting()
	originalSSRF := fetchSetting.EnableSSRFProtection
	fetchSetting.EnableSSRFProtection = false
	t.Cleanup(func() {
		fetchSetting.EnableSSRFProtection = originalSSRF
	})

	var receivedSignature string
	var receivedPayload ConnectedAppWebhookPayload
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedSignature = r.Header.Get("X-Connected-App-Webhook-Signature")
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.NoError(t, common.Unmarshal(body, &receivedPayload))
		expected := ConnectedAppWebhookSignature("secret", body)
		assert.Equal(t, expected, receivedSignature)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	row := model.ConnectedAppNotificationOutbox{
		EventKey:    "connected_app_device.authorized:session:1:0:webhook:test",
		EventType:   model.ConnectedAppNotificationEventDeviceAuthorized,
		AppId:       42,
		Channel:     model.ConnectedAppNotificationOutboxChannelWebhook,
		TargetType:  "connected_app_device_session",
		TargetId:    1001,
		PayloadJson: `{"status":"authorized"}`,
		Status:      model.ConnectedAppNotificationOutboxStatusProcessing,
		CreatedAt:   1781901000,
	}
	result, err := SendConnectedAppWebhookWithResult(model.ConnectedAppWebhook{
		AppId:  42,
		Name:   "signed webhook",
		Url:    server.URL,
		Secret: "secret",
		Status: model.ConnectedAppWebhookStatusEnabled,
	}, row)
	require.NoError(t, err)
	require.True(t, result.Success)
	assert.NotEmpty(t, receivedSignature)
	assert.Equal(t, ConnectedAppWebhookPayloadVersion, receivedPayload.Version)
	assert.Equal(t, row.EventKey, receivedPayload.EventId)
	assert.Equal(t, row.EventType, receivedPayload.EventType)
	assert.Equal(t, row.AppId, receivedPayload.AppId)
	assert.Equal(t, row.PayloadJson, receivedPayload.PayloadJson)
}

func TestEnqueueConnectedAppTokenLifecycleOutboxRespectsPreferencesAndIsIdempotent(t *testing.T) {
	setupConnectedAppNotificationServiceTestDB(t)
	require.NoError(t, model.DB.Create(&model.User{
		Id:       9601,
		Username: "token-lifecycle-dev",
		Email:    "token-lifecycle-dev@example.com",
		Status:   common.UserStatusEnabled,
		AffCode:  "token-lifecycle-dev",
	}).Error)
	app := model.ConnectedApp{
		Id:                9610,
		Slug:              "token-lifecycle-app",
		Name:              "Token Lifecycle App",
		AllowedScopes:     "openai.chat",
		DefaultScopes:     "openai.chat",
		AuthorizationFlow: model.ConnectedAppAuthorizationFlowDeviceCode,
		Trusted:           true,
		Status:            model.ConnectedAppStatusEnabled,
	}
	require.NoError(t, model.DB.Create(&app).Error)
	require.NoError(t, model.DB.Create(&model.ConnectedAppRequest{
		Id:              9620,
		ApplicantUserId: 9601,
		AppId:           app.Id,
		Slug:            app.Slug,
		Name:            app.Name,
		Status:          model.ConnectedAppRequestStatusApproved,
	}).Error)
	require.NoError(t, model.DB.Create(&model.ConnectedAppNotificationPreference{
		AppId:              app.Id,
		Channel:            model.ConnectedAppNotificationOutboxChannelEmail,
		EventType:          model.ConnectedAppNotificationEventTokenRotated,
		Enabled:            true,
		RecipientScopeJson: `{"app_developers":true}`,
	}).Error)
	require.NoError(t, model.DB.Create(&model.ConnectedAppNotificationPreference{
		AppId:              app.Id,
		Channel:            model.ConnectedAppNotificationOutboxChannelWebhook,
		EventType:          model.ConnectedAppNotificationEventTokenRotated,
		Enabled:            true,
		RecipientScopeJson: `{}`,
	}).Error)
	require.NoError(t, model.DB.Create(&model.ConnectedAppWebhook{
		AppId:          app.Id,
		Name:           "token lifecycle webhook",
		Url:            "https://example.com/token-lifecycle",
		EventTypesJson: `["connected_app_token.rotated"]`,
		Status:         model.ConnectedAppWebhookStatusEnabled,
	}).Error)

	input := ConnectedAppTokenLifecycleNotificationInput{
		EventType:         model.ConnectedAppNotificationEventTokenRotated,
		App:               app,
		UserId:            9701,
		GrantId:           9630,
		BindingId:         9640,
		TokenId:           9660,
		PreviousTokenId:   9650,
		NewTokenId:        9660,
		DeviceFingerprint: "device-fingerprint",
		DeviceName:        "Developer Mac",
		Platform:          "macos",
		AppVersion:        "1.0.0",
		OccurredAt:        1781902000,
	}
	require.NoError(t, EnqueueConnectedAppTokenLifecycleOutboxWithDB(model.DB, input))
	require.NoError(t, EnqueueConnectedAppTokenLifecycleOutboxWithDB(model.DB, input))

	var rows []model.ConnectedAppNotificationOutbox
	require.NoError(t, model.DB.Order("channel asc").Find(&rows).Error)
	require.Len(t, rows, 2)
	channels := []string{rows[0].Channel, rows[1].Channel}
	assert.ElementsMatch(t, []string{model.ConnectedAppNotificationOutboxChannelEmail, model.ConnectedAppNotificationOutboxChannelWebhook}, channels)
	for _, row := range rows {
		assert.Equal(t, app.Id, row.AppId)
		assert.Equal(t, model.ConnectedAppNotificationEventTokenRotated, row.EventType)
		assert.Equal(t, "connected_app_token_binding", row.TargetType)
		assert.Equal(t, 9640, row.TargetId)
		assert.Equal(t, model.ConnectedAppNotificationOutboxStatusPending, row.Status)
		assert.Contains(t, row.PayloadJson, `"previous_token_id":9650`)
		assert.Contains(t, row.PayloadJson, `"new_token_id":9660`)
		if row.Channel == model.ConnectedAppNotificationOutboxChannelEmail {
			assert.Equal(t, "token-lifecycle-dev@example.com", row.RecipientEmail)
		}
		if row.Channel == model.ConnectedAppNotificationOutboxChannelWebhook {
			assert.Equal(t, "webhook:1", row.RecipientEmail)
		}
	}
}

func TestConnectedAppNotificationRejectsInvalidAppIdAndWebhookEvent(t *testing.T) {
	setupConnectedAppNotificationServiceTestDB(t)

	_, _, err := UpsertConnectedAppNotificationPreference(ConnectedAppNotificationPreferenceUpsertInput{
		AppId:     -1,
		Channel:   model.ConnectedAppNotificationOutboxChannelEmail,
		EventType: model.ConnectedAppAuditActionApprove,
		Enabled:   true,
	})
	require.ErrorContains(t, err, "invalid connected app id")

	_, err = CreateConnectedAppWebhook(ConnectedAppWebhookUpsertInput{
		AppId:      -1,
		Name:       "invalid app",
		Url:        "https://example.com/webhook",
		EventTypes: []string{model.ConnectedAppNotificationEventDeviceAuthorized},
	})
	require.ErrorContains(t, err, "invalid connected app id")

	_, err = CreateConnectedAppWebhook(ConnectedAppWebhookUpsertInput{
		AppId:      0,
		Name:       "invalid event",
		Url:        "https://example.com/webhook",
		EventTypes: []string{"connected_app.unknown"},
	})
	require.ErrorContains(t, err, "invalid webhook event type")

	_, err = CreateConnectedAppWebhook(ConnectedAppWebhookUpsertInput{
		AppId:      0,
		Name:       "wildcard",
		Url:        "https://example.com/webhook",
		EventTypes: []string{"*"},
	})
	require.NoError(t, err)

	_, err = CreateConnectedAppWebhook(ConnectedAppWebhookUpsertInput{
		AppId:      0,
		Name:       "token lifecycle",
		Url:        "https://example.com/token-lifecycle",
		EventTypes: []string{model.ConnectedAppNotificationEventTokenRotated},
	})
	require.NoError(t, err)
}
