package service

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	ConnectedAppNotificationOutboxWorkerBatchSize      = 100
	ConnectedAppNotificationOutboxMaxRetryCount        = 5
	ConnectedAppNotificationOutboxProcessingStaleAfter = 10 * time.Minute
	ConnectedAppWebhookPayloadVersion                  = "v1"
)

var ConnectedAppNotificationPreferenceEventTypes = []string{
	model.ConnectedAppAuditActionApprove,
	model.ConnectedAppAuditActionReject,
	model.ConnectedAppNotificationEventDeviceAuthorized,
	model.ConnectedAppNotificationEventDeviceDenied,
	model.ConnectedAppNotificationEventDeviceRevoked,
	model.ConnectedAppNotificationEventTokenRotated,
	model.ConnectedAppNotificationEventTokenRevoked,
	model.ConnectedAppNotificationEventGrantRevoked,
	model.ConnectedAppNotificationEventHealthWarning,
}

var ConnectedAppNotificationPreferenceChannels = []string{
	model.ConnectedAppNotificationOutboxChannelEmail,
	model.ConnectedAppNotificationOutboxChannelWebhook,
}

var connectedAppNotificationOutboxSendEmail = common.SendEmail

var connectedAppWebhookHTTPClient = GetHttpClient

var connectedAppNotificationOutboxMetricsMu sync.RWMutex

var connectedAppNotificationOutboxMetrics ConnectedAppNotificationOutboxWorkerMetrics

type ConnectedAppNotificationRecipientScope struct {
	Applicant       bool     `json:"applicant"`
	AuthorizingUser bool     `json:"authorizing_user"`
	AppDevelopers   bool     `json:"app_developers"`
	ExplicitEmails  []string `json:"explicit_emails"`
}

type ConnectedAppNotificationPreferenceItem struct {
	Id                 int                                    `json:"id"`
	AppId              int                                    `json:"app_id"`
	Channel            string                                 `json:"channel"`
	EventType          string                                 `json:"event_type"`
	Enabled            bool                                   `json:"enabled"`
	RecipientScope     ConnectedAppNotificationRecipientScope `json:"recipient_scope"`
	RecipientScopeJson string                                 `json:"recipient_scope_json"`
	CreatedAt          int64                                  `json:"created_at"`
	UpdatedAt          int64                                  `json:"updated_at"`
}

type ConnectedAppNotificationPreferenceUpsertInput struct {
	AppId          int
	Channel        string
	EventType      string
	Enabled        bool
	RecipientScope ConnectedAppNotificationRecipientScope
}

type ConnectedAppWebhookItem struct {
	Id             int      `json:"id"`
	AppId          int      `json:"app_id"`
	Name           string   `json:"name"`
	Url            string   `json:"url"`
	HasSecret      bool     `json:"has_secret"`
	EventTypes     []string `json:"event_types"`
	EventTypesJson string   `json:"event_types_json"`
	Status         int      `json:"status"`
	CreatedAt      int64    `json:"created_at"`
	UpdatedAt      int64    `json:"updated_at"`
}

type ConnectedAppWebhookUpsertInput struct {
	AppId      int
	Name       string
	Url        string
	Secret     *string
	EventTypes []string
	Status     int
}

type ConnectedAppWebhookPayload struct {
	Version     string `json:"version"`
	EventId     string `json:"event_id"`
	EventType   string `json:"event_type"`
	AppId       int    `json:"app_id"`
	TargetType  string `json:"target_type"`
	TargetId    int    `json:"target_id"`
	CreatedAt   int64  `json:"created_at"`
	PayloadJson string `json:"payload_json"`
}

type ConnectedAppWebhookTestResult struct {
	Success         bool   `json:"success"`
	StatusCode      int    `json:"status_code"`
	DurationMs      int64  `json:"duration_ms"`
	Error           string `json:"error"`
	SignatureHeader string `json:"signature_header"`
}

type ConnectedAppNotificationOutboxInput struct {
	EventKey        string
	EventType       string
	AppId           int
	RecipientUserId int
	RecipientEmail  string
	Channel         string
	TargetType      string
	TargetId        int
	Payload         any
	NextRetryAt     int64
}

type ConnectedAppNotificationOutboxListParams struct {
	AppId       int
	FilterAppId bool
	Channel     string
	EventType   string
	Status      string
	TargetType  string
	TargetId    int
	WebhookId   int
	StartTime   int64
	EndTime     int64
	Offset      int
	Limit       int
}

type ConnectedAppNotificationOutboxItem struct {
	Id              int64  `json:"id"`
	EventKey        string `json:"event_key"`
	EventType       string `json:"event_type"`
	AppId           int    `json:"app_id"`
	RecipientUserId int    `json:"recipient_user_id"`
	RecipientEmail  string `json:"recipient_email"`
	Channel         string `json:"channel"`
	TargetType      string `json:"target_type"`
	TargetId        int    `json:"target_id"`
	Status          string `json:"status"`
	RetryCount      int    `json:"retry_count"`
	NextRetryAt     int64  `json:"next_retry_at"`
	LastError       string `json:"last_error"`
	CreatedAt       int64  `json:"created_at"`
	UpdatedAt       int64  `json:"updated_at"`
}

type ConnectedAppNotificationOutboxBatchStats struct {
	Claimed         int   `json:"claimed"`
	Sent            int   `json:"sent"`
	Failed          int   `json:"failed"`
	PermanentFailed int   `json:"permanent_failed"`
	DurationMs      int64 `json:"duration_ms"`
	StartedAt       int64 `json:"started_at"`
	FinishedAt      int64 `json:"finished_at"`
}

type ConnectedAppNotificationOutboxWorkerMetrics struct {
	LastRun              ConnectedAppNotificationOutboxBatchStats `json:"last_run"`
	TotalRuns            int64                                    `json:"total_runs"`
	TotalClaimed         int64                                    `json:"total_claimed"`
	TotalSent            int64                                    `json:"total_sent"`
	TotalFailed          int64                                    `json:"total_failed"`
	TotalPermanentFailed int64                                    `json:"total_permanent_failed"`
}

type ConnectedAppRequestReviewNotificationPayload struct {
	Version         string `json:"version"`
	RequestId       int    `json:"request_id"`
	AuditLogId      int64  `json:"audit_log_id"`
	Action          string `json:"action"`
	Status          string `json:"status"`
	AppId           int    `json:"app_id"`
	AppSlug         string `json:"app_slug"`
	AppName         string `json:"app_name"`
	ApplicantUserId int    `json:"applicant_user_id"`
	ReviewerUserId  int    `json:"reviewer_user_id"`
	ReviewNote      string `json:"review_note"`
}

type ConnectedAppDeviceAuthorizationNotificationPayload struct {
	Version           string `json:"version"`
	SessionId         int64  `json:"session_id"`
	AppId             int    `json:"app_id"`
	AppSlug           string `json:"app_slug"`
	AppName           string `json:"app_name"`
	UserId            int    `json:"user_id"`
	TokenId           int    `json:"token_id"`
	Status            string `json:"status"`
	DeviceFingerprint string `json:"device_fingerprint"`
	DeviceName        string `json:"device_name"`
	Platform          string `json:"platform"`
	AppVersion        string `json:"app_version"`
	Client            string `json:"client"`
}

type ConnectedAppHealthWarningInput struct {
	App       model.ConnectedApp
	UserId    int
	Session   *model.ConnectedAppDeviceSession
	Status    string
	Checks    map[string]bool
	CreatedAt int64
}

type ConnectedAppHealthWarningNotificationPayload struct {
	Version           string          `json:"version"`
	AppId             int             `json:"app_id"`
	AppSlug           string          `json:"app_slug"`
	AppName           string          `json:"app_name"`
	UserId            int             `json:"user_id"`
	SessionId         int64           `json:"session_id"`
	Status            string          `json:"status"`
	Checks            map[string]bool `json:"checks"`
	DeviceFingerprint string          `json:"device_fingerprint"`
	DeviceName        string          `json:"device_name"`
	Platform          string          `json:"platform"`
	AppVersion        string          `json:"app_version"`
	Client            string          `json:"client"`
}

type ConnectedAppTokenLifecycleNotificationInput struct {
	EventType         string
	App               model.ConnectedApp
	UserId            int
	GrantId           int64
	BindingId         int64
	TokenId           int
	PreviousTokenId   int
	NewTokenId        int
	Status            string
	DeviceFingerprint string
	DeviceName        string
	Platform          string
	AppVersion        string
	OccurredAt        int64
}

type ConnectedAppTokenLifecycleNotificationPayload struct {
	Version           string `json:"version"`
	AppId             int    `json:"app_id"`
	AppSlug           string `json:"app_slug"`
	AppName           string `json:"app_name"`
	UserId            int    `json:"user_id"`
	GrantId           int64  `json:"grant_id"`
	BindingId         int64  `json:"binding_id"`
	TokenId           int    `json:"token_id"`
	PreviousTokenId   int    `json:"previous_token_id"`
	NewTokenId        int    `json:"new_token_id"`
	Status            string `json:"status"`
	DeviceFingerprint string `json:"device_fingerprint"`
	DeviceName        string `json:"device_name"`
	Platform          string `json:"platform"`
	AppVersion        string `json:"app_version"`
	OccurredAt        int64  `json:"occurred_at"`
}

func ListConnectedAppNotificationPreferences(appId int) ([]ConnectedAppNotificationPreferenceItem, error) {
	var preferences []model.ConnectedAppNotificationPreference
	if err := model.DB.Where("app_id = ?", appId).Find(&preferences).Error; err != nil {
		return nil, err
	}
	byKey := map[string]model.ConnectedAppNotificationPreference{}
	for _, preference := range preferences {
		byKey[connectedAppNotificationPreferenceKey(preference.Channel, preference.EventType)] = preference
	}
	items := make([]ConnectedAppNotificationPreferenceItem, 0, len(ConnectedAppNotificationPreferenceChannels)*len(ConnectedAppNotificationPreferenceEventTypes))
	for _, channel := range ConnectedAppNotificationPreferenceChannels {
		for _, eventType := range ConnectedAppNotificationPreferenceEventTypes {
			preference, ok := byKey[connectedAppNotificationPreferenceKey(channel, eventType)]
			if ok {
				item, err := ConnectedAppNotificationPreferenceToItem(preference)
				if err != nil {
					return nil, err
				}
				items = append(items, item)
				continue
			}
			items = append(items, ConnectedAppNotificationPreferenceItem{
				AppId:              appId,
				Channel:            channel,
				EventType:          eventType,
				Enabled:            false,
				RecipientScope:     ConnectedAppNotificationRecipientScope{},
				RecipientScopeJson: "{}",
			})
		}
	}
	return items, nil
}

func UpsertConnectedAppNotificationPreference(input ConnectedAppNotificationPreferenceUpsertInput) (model.ConnectedAppNotificationPreference, model.ConnectedAppNotificationPreference, error) {
	if input.AppId < 0 {
		return model.ConnectedAppNotificationPreference{}, model.ConnectedAppNotificationPreference{}, errors.New("invalid connected app id")
	}
	channel := strings.TrimSpace(input.Channel)
	eventType := strings.TrimSpace(input.EventType)
	if !isConnectedAppNotificationPreferenceChannel(channel) {
		return model.ConnectedAppNotificationPreference{}, model.ConnectedAppNotificationPreference{}, errors.New("invalid notification channel")
	}
	if !isConnectedAppNotificationPreferenceEventType(eventType) {
		return model.ConnectedAppNotificationPreference{}, model.ConnectedAppNotificationPreference{}, errors.New("invalid notification event type")
	}
	scope := input.RecipientScope
	scope.ExplicitEmails = normalizeConnectedAppNotificationEmails(scope.ExplicitEmails)
	scopeJson, err := common.Marshal(scope)
	if err != nil {
		return model.ConnectedAppNotificationPreference{}, model.ConnectedAppNotificationPreference{}, err
	}
	preference, ok, err := GetConnectedAppNotificationPreferenceWithDB(model.DB, input.AppId, channel, eventType)
	if err != nil {
		return model.ConnectedAppNotificationPreference{}, model.ConnectedAppNotificationPreference{}, err
	}
	before := preference
	if !ok {
		preference = model.ConnectedAppNotificationPreference{
			AppId:     input.AppId,
			Channel:   channel,
			EventType: eventType,
		}
	}
	preference.Enabled = input.Enabled
	preference.RecipientScopeJson = string(scopeJson)
	if !ok {
		if err := model.DB.Create(&preference).Error; err != nil {
			return model.ConnectedAppNotificationPreference{}, model.ConnectedAppNotificationPreference{}, err
		}
		return before, preference, nil
	}
	if err := model.DB.Save(&preference).Error; err != nil {
		return model.ConnectedAppNotificationPreference{}, model.ConnectedAppNotificationPreference{}, err
	}
	return before, preference, nil
}

func ConnectedAppNotificationPreferenceToItem(preference model.ConnectedAppNotificationPreference) (ConnectedAppNotificationPreferenceItem, error) {
	scope, err := parseConnectedAppNotificationRecipientScope(preference.RecipientScopeJson)
	if err != nil {
		return ConnectedAppNotificationPreferenceItem{}, err
	}
	return ConnectedAppNotificationPreferenceItem{
		Id:                 preference.Id,
		AppId:              preference.AppId,
		Channel:            preference.Channel,
		EventType:          preference.EventType,
		Enabled:            preference.Enabled,
		RecipientScope:     scope,
		RecipientScopeJson: preference.RecipientScopeJson,
		CreatedAt:          preference.CreatedAt,
		UpdatedAt:          preference.UpdatedAt,
	}, nil
}

func GetConnectedAppNotificationPreferenceWithDB(db *gorm.DB, appId int, channel string, eventType string) (model.ConnectedAppNotificationPreference, bool, error) {
	var preference model.ConnectedAppNotificationPreference
	err := db.Where("app_id = ? AND channel = ? AND event_type = ?", appId, strings.TrimSpace(channel), strings.TrimSpace(eventType)).First(&preference).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.ConnectedAppNotificationPreference{}, false, nil
		}
		return model.ConnectedAppNotificationPreference{}, false, err
	}
	return preference, true, nil
}

func ConnectedAppNotificationPreferenceWithFallbackDB(db *gorm.DB, appId int, channel string, eventType string) (model.ConnectedAppNotificationPreference, bool, error) {
	if appId > 0 {
		preference, ok, err := GetConnectedAppNotificationPreferenceWithDB(db, appId, channel, eventType)
		if err != nil || ok {
			return preference, ok, err
		}
	}
	return GetConnectedAppNotificationPreferenceWithDB(db, 0, channel, eventType)
}

func ConnectedAppNotificationChannelEnabledWithDB(db *gorm.DB, appId int, channel string, eventType string) (bool, error) {
	preference, ok, err := ConnectedAppNotificationPreferenceWithFallbackDB(db, appId, channel, eventType)
	if err != nil || !ok {
		return false, err
	}
	return preference.Enabled, nil
}

func ConnectedAppNotificationEmailScopeWithDB(db *gorm.DB, appId int, eventType string) (ConnectedAppNotificationRecipientScope, bool, error) {
	preference, ok, err := ConnectedAppNotificationPreferenceWithFallbackDB(db, appId, model.ConnectedAppNotificationOutboxChannelEmail, eventType)
	if err != nil || !ok || !preference.Enabled {
		return ConnectedAppNotificationRecipientScope{}, false, err
	}
	scope, err := parseConnectedAppNotificationRecipientScope(preference.RecipientScopeJson)
	if err != nil {
		return ConnectedAppNotificationRecipientScope{}, false, err
	}
	return scope, true, nil
}

func connectedAppNotificationPreferenceKey(channel string, eventType string) string {
	return strings.TrimSpace(channel) + ":" + strings.TrimSpace(eventType)
}

func isConnectedAppNotificationPreferenceChannel(channel string) bool {
	for _, allowed := range ConnectedAppNotificationPreferenceChannels {
		if channel == allowed {
			return true
		}
	}
	return false
}

func isConnectedAppNotificationPreferenceEventType(eventType string) bool {
	for _, allowed := range ConnectedAppNotificationPreferenceEventTypes {
		if eventType == allowed {
			return true
		}
	}
	return false
}

func parseConnectedAppNotificationRecipientScope(value string) (ConnectedAppNotificationRecipientScope, error) {
	scope := ConnectedAppNotificationRecipientScope{}
	value = strings.TrimSpace(value)
	if value == "" {
		return scope, nil
	}
	if err := common.Unmarshal([]byte(value), &scope); err != nil {
		return ConnectedAppNotificationRecipientScope{}, err
	}
	scope.ExplicitEmails = normalizeConnectedAppNotificationEmails(scope.ExplicitEmails)
	return scope, nil
}

func normalizeConnectedAppNotificationEmails(emails []string) []string {
	result := make([]string, 0, len(emails))
	seen := map[string]struct{}{}
	for _, email := range emails {
		normalized := strings.ToLower(strings.TrimSpace(email))
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	return result
}

func ListConnectedAppWebhooks(appId int, includeGlobal bool) ([]ConnectedAppWebhookItem, error) {
	var webhooks []model.ConnectedAppWebhook
	query := model.DB.Model(&model.ConnectedAppWebhook{})
	if includeGlobal && appId > 0 {
		query = query.Where("app_id IN ?", []int{0, appId})
	} else {
		query = query.Where("app_id = ?", appId)
	}
	if err := query.Order("app_id asc, id desc").Find(&webhooks).Error; err != nil {
		return nil, err
	}
	items := make([]ConnectedAppWebhookItem, 0, len(webhooks))
	for _, webhook := range webhooks {
		items = append(items, ConnectedAppWebhookToItem(webhook))
	}
	return items, nil
}

func CreateConnectedAppWebhook(input ConnectedAppWebhookUpsertInput) (model.ConnectedAppWebhook, error) {
	webhook, err := connectedAppWebhookFromInput(model.ConnectedAppWebhook{}, input, true)
	if err != nil {
		return model.ConnectedAppWebhook{}, err
	}
	return webhook, model.DB.Create(&webhook).Error
}

func UpdateConnectedAppWebhook(appId int, id int, input ConnectedAppWebhookUpsertInput) (model.ConnectedAppWebhook, model.ConnectedAppWebhook, error) {
	var before model.ConnectedAppWebhook
	if err := model.DB.Where("id = ? AND app_id = ?", id, appId).First(&before).Error; err != nil {
		return model.ConnectedAppWebhook{}, model.ConnectedAppWebhook{}, err
	}
	input.AppId = appId
	after, err := connectedAppWebhookFromInput(before, input, false)
	if err != nil {
		return model.ConnectedAppWebhook{}, model.ConnectedAppWebhook{}, err
	}
	after.Id = before.Id
	after.CreatedAt = before.CreatedAt
	return before, after, model.DB.Save(&after).Error
}

func DisableConnectedAppWebhook(appId int, id int) (model.ConnectedAppWebhook, model.ConnectedAppWebhook, error) {
	var before model.ConnectedAppWebhook
	if err := model.DB.Where("id = ? AND app_id = ?", id, appId).First(&before).Error; err != nil {
		return model.ConnectedAppWebhook{}, model.ConnectedAppWebhook{}, err
	}
	after := before
	after.Status = model.ConnectedAppWebhookStatusDisabled
	return before, after, model.DB.Save(&after).Error
}

func ConnectedAppWebhookToItem(webhook model.ConnectedAppWebhook) ConnectedAppWebhookItem {
	events, _ := parseConnectedAppWebhookEventTypes(webhook.EventTypesJson)
	return ConnectedAppWebhookItem{
		Id:             webhook.Id,
		AppId:          webhook.AppId,
		Name:           webhook.Name,
		Url:            webhook.Url,
		HasSecret:      strings.TrimSpace(webhook.Secret) != "",
		EventTypes:     events,
		EventTypesJson: webhook.EventTypesJson,
		Status:         webhook.Status,
		CreatedAt:      webhook.CreatedAt,
		UpdatedAt:      webhook.UpdatedAt,
	}
}

func connectedAppWebhookFromInput(existing model.ConnectedAppWebhook, input ConnectedAppWebhookUpsertInput, create bool) (model.ConnectedAppWebhook, error) {
	if input.AppId < 0 {
		return model.ConnectedAppWebhook{}, errors.New("invalid connected app id")
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return model.ConnectedAppWebhook{}, errors.New("webhook name is required")
	}
	webhookUrl := strings.TrimSpace(input.Url)
	if err := validateConnectedAppWebhookURL(webhookUrl); err != nil {
		return model.ConnectedAppWebhook{}, err
	}
	events := normalizeConnectedAppWebhookEventTypes(input.EventTypes)
	if err := validateConnectedAppWebhookEventTypes(events); err != nil {
		return model.ConnectedAppWebhook{}, err
	}
	eventsJson, err := common.Marshal(events)
	if err != nil {
		return model.ConnectedAppWebhook{}, err
	}
	status := input.Status
	if status == 0 {
		if create {
			status = model.ConnectedAppWebhookStatusEnabled
		} else {
			status = existing.Status
		}
	}
	if status != model.ConnectedAppWebhookStatusEnabled && status != model.ConnectedAppWebhookStatusDisabled {
		return model.ConnectedAppWebhook{}, errors.New("invalid webhook status")
	}
	secret := existing.Secret
	if input.Secret != nil {
		secret = strings.TrimSpace(*input.Secret)
	}
	return model.ConnectedAppWebhook{
		Id:             existing.Id,
		AppId:          input.AppId,
		Name:           name,
		Url:            webhookUrl,
		Secret:         secret,
		EventTypesJson: string(eventsJson),
		Status:         status,
		CreatedAt:      existing.CreatedAt,
	}, nil
}

func validateConnectedAppWebhookURL(value string) error {
	if value == "" {
		return errors.New("webhook URL is required")
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return errors.New("invalid webhook URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errors.New("webhook URL must use http or https")
	}
	return nil
}

func normalizeConnectedAppWebhookEventTypes(values []string) []string {
	result := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func validateConnectedAppWebhookEventTypes(events []string) error {
	for _, event := range events {
		if event == "*" || isConnectedAppNotificationPreferenceEventType(event) {
			continue
		}
		return fmt.Errorf("invalid webhook event type: %s", event)
	}
	return nil
}

func parseConnectedAppWebhookEventTypes(value string) ([]string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, nil
	}
	var events []string
	if err := common.Unmarshal([]byte(trimmed), &events); err != nil {
		return nil, err
	}
	return normalizeConnectedAppWebhookEventTypes(events), nil
}

func ListEnabledConnectedAppWebhooksForEventWithDB(db *gorm.DB, appId int, eventType string) ([]model.ConnectedAppWebhook, error) {
	var webhooks []model.ConnectedAppWebhook
	query := db.Where("status = ?", model.ConnectedAppWebhookStatusEnabled)
	if appId > 0 {
		query = query.Where("app_id IN ?", []int{0, appId})
	} else {
		query = query.Where("app_id = ?", 0)
	}
	if err := query.Find(&webhooks).Error; err != nil {
		return nil, err
	}
	matched := make([]model.ConnectedAppWebhook, 0, len(webhooks))
	for _, webhook := range webhooks {
		if connectedAppWebhookMatchesEvent(webhook, eventType) {
			matched = append(matched, webhook)
		}
	}
	return matched, nil
}

func connectedAppWebhookMatchesEvent(webhook model.ConnectedAppWebhook, eventType string) bool {
	trimmed := strings.TrimSpace(webhook.EventTypesJson)
	if trimmed == "" || trimmed == "[]" {
		return true
	}
	events, err := parseConnectedAppWebhookEventTypes(trimmed)
	if err != nil {
		return false
	}
	for _, event := range events {
		if strings.TrimSpace(event) == eventType || strings.TrimSpace(event) == "*" {
			return true
		}
	}
	return false
}

func ConnectedAppWebhookSignature(secret string, payload []byte) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(payload)
	return "sha256=" + hex.EncodeToString(h.Sum(nil))
}

func BuildConnectedAppWebhookPayload(row model.ConnectedAppNotificationOutbox) ConnectedAppWebhookPayload {
	return ConnectedAppWebhookPayload{
		Version:     ConnectedAppWebhookPayloadVersion,
		EventId:     row.EventKey,
		EventType:   row.EventType,
		AppId:       row.AppId,
		TargetType:  row.TargetType,
		TargetId:    row.TargetId,
		CreatedAt:   row.CreatedAt,
		PayloadJson: row.PayloadJson,
	}
}

func DeliverConnectedAppWebhookOutbox(row model.ConnectedAppNotificationOutbox) error {
	webhookId, err := connectedAppWebhookIdFromRecipient(row.RecipientEmail)
	if err != nil {
		return err
	}
	var webhook model.ConnectedAppWebhook
	query := model.DB.Where("id = ? AND status = ?", webhookId, model.ConnectedAppWebhookStatusEnabled)
	if row.AppId > 0 {
		query = query.Where("app_id IN ?", []int{0, row.AppId})
	} else {
		query = query.Where("app_id = ?", 0)
	}
	if err := query.First(&webhook).Error; err != nil {
		return err
	}
	if !connectedAppWebhookMatchesEvent(webhook, row.EventType) {
		return fmt.Errorf("connected app webhook %d is not subscribed to %s", webhook.Id, row.EventType)
	}
	return SendConnectedAppWebhook(webhook, row)
}

func SendConnectedAppWebhook(webhook model.ConnectedAppWebhook, row model.ConnectedAppNotificationOutbox) error {
	result, err := SendConnectedAppWebhookWithResult(webhook, row)
	if err != nil {
		return err
	}
	if !result.Success {
		return errors.New(result.Error)
	}
	return nil
}

func SendConnectedAppWebhookWithResult(webhook model.ConnectedAppWebhook, row model.ConnectedAppNotificationOutbox) (ConnectedAppWebhookTestResult, error) {
	urlValue := strings.TrimSpace(webhook.Url)
	if urlValue == "" {
		return ConnectedAppWebhookTestResult{}, errors.New("connected app webhook URL is empty")
	}
	payloadBytes, err := common.Marshal(BuildConnectedAppWebhookPayload(row))
	if err != nil {
		return ConnectedAppWebhookTestResult{}, err
	}
	signature := ""
	if strings.TrimSpace(webhook.Secret) != "" {
		signature = ConnectedAppWebhookSignature(webhook.Secret, payloadBytes)
	}
	startedAt := time.Now()
	result := ConnectedAppWebhookTestResult{SignatureHeader: signature}
	if system_setting.EnableWorker() {
		workerReq := &WorkerRequest{
			URL:    urlValue,
			Key:    system_setting.WorkerValidKey,
			Method: http.MethodPost,
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			Body: payloadBytes,
		}
		if signature != "" {
			workerReq.Headers["X-Connected-App-Webhook-Signature"] = signature
		}
		resp, err := DoWorkerRequest(workerReq)
		result.DurationMs = time.Since(startedAt).Milliseconds()
		if err != nil {
			result.Error = err.Error()
			return result, nil
		}
		defer resp.Body.Close()
		result.StatusCode = resp.StatusCode
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			result.Error = fmt.Sprintf("connected app webhook request failed with status code: %d", resp.StatusCode)
			return result, nil
		}
		result.Success = true
		return result, nil
	}
	fetchSetting := system_setting.GetFetchSetting()
	if err := common.ValidateURLWithFetchSetting(urlValue, fetchSetting.EnableSSRFProtection, fetchSetting.AllowPrivateIp, fetchSetting.DomainFilterMode, fetchSetting.IpFilterMode, fetchSetting.DomainList, fetchSetting.IpList, fetchSetting.AllowedPorts, fetchSetting.ApplyIPFilterForDomain); err != nil {
		return ConnectedAppWebhookTestResult{}, err
	}
	req, err := http.NewRequest(http.MethodPost, urlValue, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return ConnectedAppWebhookTestResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	if signature != "" {
		req.Header.Set("X-Connected-App-Webhook-Signature", signature)
	}
	client := connectedAppWebhookHTTPClient()
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	result.DurationMs = time.Since(startedAt).Milliseconds()
	if err != nil {
		result.Error = err.Error()
		return result, nil
	}
	defer resp.Body.Close()
	result.StatusCode = resp.StatusCode
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		result.Error = connectedAppWebhookResponseError(resp)
		return result, nil
	}
	result.Success = true
	return result, nil
}

func connectedAppWebhookResponseError(resp *http.Response) string {
	message := fmt.Sprintf("connected app webhook request failed with status code: %d", resp.StatusCode)
	if resp.Body == nil {
		return message
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 512))
	if err != nil || len(bytes.TrimSpace(body)) == 0 {
		return message
	}
	return message + ": " + string(bytes.TrimSpace(body))
}

func BuildConnectedAppWebhookTestOutbox(appId int, webhookId int) model.ConnectedAppNotificationOutbox {
	now := common.GetTimestamp()
	return model.ConnectedAppNotificationOutbox{
		EventKey:       fmt.Sprintf("connected_app.webhook.test:%d:%d", webhookId, now),
		EventType:      "connected_app.webhook.test",
		AppId:          appId,
		RecipientEmail: fmt.Sprintf("webhook:%d", webhookId),
		Channel:        model.ConnectedAppNotificationOutboxChannelWebhook,
		TargetType:     "connected_app_webhook",
		TargetId:       webhookId,
		PayloadJson:    `{"message":"test"}`,
		Status:         model.ConnectedAppNotificationOutboxStatusProcessing,
		NextRetryAt:    0,
		RetryCount:     0,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

func connectedAppWebhookIdFromRecipient(value string) (int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, errors.New("connected app webhook recipient is empty")
	}
	var id int
	_, err := fmt.Sscanf(value, "webhook:%d", &id)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("invalid connected app webhook recipient: %s", value)
	}
	return id, nil
}

func EnqueueConnectedAppNotificationOutbox(input ConnectedAppNotificationOutboxInput) (bool, error) {
	return EnqueueConnectedAppNotificationOutboxWithDB(model.DB, input)
}

func EnqueueConnectedAppNotificationOutboxWithDB(db *gorm.DB, input ConnectedAppNotificationOutboxInput) (bool, error) {
	row, err := buildConnectedAppNotificationOutbox(input)
	if err != nil {
		return false, err
	}
	result := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "event_key"}},
		DoNothing: true,
	}).Create(&row)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func buildConnectedAppNotificationOutbox(input ConnectedAppNotificationOutboxInput) (model.ConnectedAppNotificationOutbox, error) {
	eventKey := strings.TrimSpace(input.EventKey)
	if eventKey == "" {
		return model.ConnectedAppNotificationOutbox{}, errors.New("connected app notification outbox event key is empty")
	}
	eventType := strings.TrimSpace(input.EventType)
	if eventType == "" {
		return model.ConnectedAppNotificationOutbox{}, errors.New("connected app notification outbox event type is empty")
	}
	targetType := strings.TrimSpace(input.TargetType)
	if targetType == "" {
		return model.ConnectedAppNotificationOutbox{}, errors.New("connected app notification outbox target type is empty")
	}
	payloadJson, err := connectedAppNotificationOutboxPayloadJson(input.Payload)
	if err != nil {
		return model.ConnectedAppNotificationOutbox{}, err
	}
	channel := strings.TrimSpace(input.Channel)
	if channel == "" {
		channel = model.ConnectedAppNotificationOutboxChannelInApp
	}
	return model.ConnectedAppNotificationOutbox{
		EventKey:        eventKey,
		EventType:       eventType,
		AppId:           input.AppId,
		RecipientUserId: input.RecipientUserId,
		RecipientEmail:  strings.TrimSpace(input.RecipientEmail),
		Channel:         channel,
		TargetType:      targetType,
		TargetId:        input.TargetId,
		PayloadJson:     payloadJson,
		Status:          model.ConnectedAppNotificationOutboxStatusPending,
		NextRetryAt:     input.NextRetryAt,
	}, nil
}

func connectedAppNotificationOutboxPayloadJson(value any) (string, error) {
	if value == nil {
		return "{}", nil
	}
	bytes, err := common.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func ListConnectedAppNotificationOutbox(params ConnectedAppNotificationOutboxListParams) ([]ConnectedAppNotificationOutboxItem, int64, error) {
	if params.Limit <= 0 {
		params.Limit = 20
	}
	if params.Limit > 100 {
		params.Limit = 100
	}
	query := model.DB.Model(&model.ConnectedAppNotificationOutbox{})
	if params.FilterAppId {
		query = query.Where("app_id = ?", params.AppId)
	}
	if channel := strings.TrimSpace(params.Channel); channel != "" {
		query = query.Where("channel = ?", channel)
	}
	if eventType := strings.TrimSpace(params.EventType); eventType != "" {
		query = query.Where("event_type = ?", eventType)
	}
	if status := strings.TrimSpace(params.Status); status != "" {
		query = query.Where("status = ?", status)
	}
	if targetType := strings.TrimSpace(params.TargetType); targetType != "" {
		query = query.Where("target_type = ?", targetType)
	}
	if params.TargetId > 0 {
		query = query.Where("target_id = ?", params.TargetId)
	}
	if params.WebhookId > 0 {
		query = query.Where("channel = ? AND recipient_email = ?", model.ConnectedAppNotificationOutboxChannelWebhook, fmt.Sprintf("webhook:%d", params.WebhookId))
	}
	if params.StartTime > 0 {
		query = query.Where("created_at >= ?", params.StartTime)
	}
	if params.EndTime > 0 {
		query = query.Where("created_at <= ?", params.EndTime)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []model.ConnectedAppNotificationOutbox
	if err := query.Order("created_at desc, id desc").Limit(params.Limit).Offset(params.Offset).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	items := make([]ConnectedAppNotificationOutboxItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, ConnectedAppNotificationOutboxToItem(row))
	}
	return items, total, nil
}

func ConnectedAppNotificationOutboxToItem(row model.ConnectedAppNotificationOutbox) ConnectedAppNotificationOutboxItem {
	return ConnectedAppNotificationOutboxItem{
		Id:              row.Id,
		EventKey:        row.EventKey,
		EventType:       row.EventType,
		AppId:           row.AppId,
		RecipientUserId: row.RecipientUserId,
		RecipientEmail:  maskEnterpriseNotificationRecipient(row.Channel, row.RecipientEmail),
		Channel:         row.Channel,
		TargetType:      row.TargetType,
		TargetId:        row.TargetId,
		Status:          row.Status,
		RetryCount:      row.RetryCount,
		NextRetryAt:     row.NextRetryAt,
		LastError:       sanitizeEnterpriseNotificationOutboxError(row.LastError),
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
	}
}

func RetryConnectedAppNotificationOutbox(id int64) (model.ConnectedAppNotificationOutbox, model.ConnectedAppNotificationOutbox, error) {
	var before model.ConnectedAppNotificationOutbox
	if err := model.DB.Where("id = ?", id).First(&before).Error; err != nil {
		return model.ConnectedAppNotificationOutbox{}, model.ConnectedAppNotificationOutbox{}, err
	}
	if before.Status != model.ConnectedAppNotificationOutboxStatusFailed && before.Status != model.ConnectedAppNotificationOutboxStatusPermanentFailed {
		return model.ConnectedAppNotificationOutbox{}, model.ConnectedAppNotificationOutbox{}, fmt.Errorf("connected app notification outbox row %d is not retryable", id)
	}
	now := common.GetTimestamp()
	after := before
	after.Status = model.ConnectedAppNotificationOutboxStatusPending
	after.RetryCount = 0
	after.NextRetryAt = 0
	after.LastError = ""
	after.UpdatedAt = now
	if err := model.DB.Model(&model.ConnectedAppNotificationOutbox{}).
		Where("id = ? AND status IN ?", id, []string{model.ConnectedAppNotificationOutboxStatusFailed, model.ConnectedAppNotificationOutboxStatusPermanentFailed}).
		Updates(map[string]any{
			"status":        after.Status,
			"retry_count":   after.RetryCount,
			"next_retry_at": after.NextRetryAt,
			"last_error":    after.LastError,
			"updated_at":    after.UpdatedAt,
		}).Error; err != nil {
		return model.ConnectedAppNotificationOutbox{}, model.ConnectedAppNotificationOutbox{}, err
	}
	return before, after, nil
}

func GetConnectedAppNotificationOutboxWorkerMetrics() ConnectedAppNotificationOutboxWorkerMetrics {
	connectedAppNotificationOutboxMetricsMu.RLock()
	defer connectedAppNotificationOutboxMetricsMu.RUnlock()
	return connectedAppNotificationOutboxMetrics
}

func recordConnectedAppNotificationOutboxWorkerMetrics(stats ConnectedAppNotificationOutboxBatchStats) {
	connectedAppNotificationOutboxMetricsMu.Lock()
	defer connectedAppNotificationOutboxMetricsMu.Unlock()
	connectedAppNotificationOutboxMetrics.LastRun = stats
	connectedAppNotificationOutboxMetrics.TotalRuns++
	connectedAppNotificationOutboxMetrics.TotalClaimed += int64(stats.Claimed)
	connectedAppNotificationOutboxMetrics.TotalSent += int64(stats.Sent)
	connectedAppNotificationOutboxMetrics.TotalFailed += int64(stats.Failed)
	connectedAppNotificationOutboxMetrics.TotalPermanentFailed += int64(stats.PermanentFailed)
}

func ClaimDueConnectedAppNotificationOutbox(batchSize int, now int64) ([]model.ConnectedAppNotificationOutbox, error) {
	if batchSize <= 0 {
		batchSize = ConnectedAppNotificationOutboxWorkerBatchSize
	}
	if now <= 0 {
		now = common.GetTimestamp()
	}
	processingStaleBefore := now - int64(ConnectedAppNotificationOutboxProcessingStaleAfter/time.Second)
	var candidates []model.ConnectedAppNotificationOutbox
	if err := model.DB.
		Where("(status IN ? AND next_retry_at <= ?) OR (status = ? AND updated_at <= ?)", []string{model.ConnectedAppNotificationOutboxStatusPending, model.ConnectedAppNotificationOutboxStatusFailed}, now, model.ConnectedAppNotificationOutboxStatusProcessing, processingStaleBefore).
		Order("next_retry_at asc, id asc").
		Limit(batchSize).
		Find(&candidates).Error; err != nil {
		return nil, err
	}
	claimed := make([]model.ConnectedAppNotificationOutbox, 0, len(candidates))
	for _, candidate := range candidates {
		result := model.DB.Model(&model.ConnectedAppNotificationOutbox{}).
			Where("id = ? AND ((status IN ? AND next_retry_at <= ?) OR (status = ? AND updated_at <= ?))", candidate.Id, []string{model.ConnectedAppNotificationOutboxStatusPending, model.ConnectedAppNotificationOutboxStatusFailed}, now, model.ConnectedAppNotificationOutboxStatusProcessing, processingStaleBefore).
			Updates(map[string]any{
				"status":     model.ConnectedAppNotificationOutboxStatusProcessing,
				"updated_at": now,
			})
		if result.Error != nil {
			return claimed, result.Error
		}
		if result.RowsAffected == 0 {
			continue
		}
		candidate.Status = model.ConnectedAppNotificationOutboxStatusProcessing
		candidate.UpdatedAt = now
		claimed = append(claimed, candidate)
	}
	return claimed, nil
}

func MarkConnectedAppNotificationOutboxSent(id int64, now int64) error {
	if now <= 0 {
		now = common.GetTimestamp()
	}
	return model.DB.Model(&model.ConnectedAppNotificationOutbox{}).
		Where("id = ? AND status = ?", id, model.ConnectedAppNotificationOutboxStatusProcessing).
		Updates(map[string]any{
			"status":        model.ConnectedAppNotificationOutboxStatusSent,
			"last_error":    "",
			"next_retry_at": int64(0),
			"updated_at":    now,
		}).Error
}

func MarkConnectedAppNotificationOutboxFailed(id int64, err error, now int64) error {
	if now <= 0 {
		now = common.GetTimestamp()
	}
	errorMessage := "connected app notification outbox delivery failed"
	if err != nil {
		errorMessage = err.Error()
	}
	var row model.ConnectedAppNotificationOutbox
	if loadErr := model.DB.Where("id = ?", id).First(&row).Error; loadErr != nil {
		return loadErr
	}
	nextRetryCount := row.RetryCount + 1
	status := model.ConnectedAppNotificationOutboxStatusFailed
	nextRetryAt := now + connectedAppNotificationOutboxRetryDelaySeconds(nextRetryCount)
	if nextRetryCount >= ConnectedAppNotificationOutboxMaxRetryCount {
		status = model.ConnectedAppNotificationOutboxStatusPermanentFailed
		nextRetryAt = 0
	}
	return model.DB.Model(&model.ConnectedAppNotificationOutbox{}).
		Where("id = ? AND status = ?", id, model.ConnectedAppNotificationOutboxStatusProcessing).
		Updates(map[string]any{
			"status":        status,
			"retry_count":   nextRetryCount,
			"next_retry_at": nextRetryAt,
			"last_error":    errorMessage,
			"updated_at":    now,
		}).Error
}

func connectedAppNotificationOutboxRetryDelaySeconds(retryCount int) int64 {
	if retryCount <= 0 {
		return 60
	}
	delay := int64(60)
	for i := 1; i < retryCount; i++ {
		delay *= 2
		if delay >= 3600 {
			return 3600
		}
	}
	return delay
}

func DeliverConnectedAppNotificationOutbox(row model.ConnectedAppNotificationOutbox) error {
	switch row.Channel {
	case model.ConnectedAppNotificationOutboxChannelInApp:
		return nil
	case model.ConnectedAppNotificationOutboxChannelEmail:
		return deliverConnectedAppNotificationOutboxEmail(row)
	case model.ConnectedAppNotificationOutboxChannelWebhook:
		return DeliverConnectedAppWebhookOutbox(row)
	default:
		return fmt.Errorf("unsupported connected app notification outbox channel: %s", row.Channel)
	}
}

func deliverConnectedAppNotificationOutboxEmail(row model.ConnectedAppNotificationOutbox) error {
	receiver := strings.TrimSpace(row.RecipientEmail)
	if receiver == "" && row.RecipientUserId > 0 {
		var user model.User
		if err := model.DB.Select("id, email").Where("id = ?", row.RecipientUserId).First(&user).Error; err != nil {
			return err
		}
		receiver = strings.TrimSpace(user.Email)
	}
	if receiver == "" {
		return errors.New("connected app email notification receiver is empty")
	}
	if strings.TrimSpace(common.SMTPServer) == "" && strings.TrimSpace(common.SMTPAccount) == "" {
		return errors.New("SMTP server is not configured")
	}
	subject, content := connectedAppNotificationOutboxEmailMessage(row)
	return connectedAppNotificationOutboxSendEmail(subject, receiver, content)
}

func connectedAppNotificationOutboxEmailMessage(row model.ConnectedAppNotificationOutbox) (string, string) {
	subject := "Connected app notification"
	switch row.EventType {
	case model.ConnectedAppAuditActionApprove:
		subject = "Connected app request approved"
	case model.ConnectedAppAuditActionReject:
		subject = "Connected app request rejected"
	case model.ConnectedAppNotificationEventDeviceAuthorized:
		subject = "Connected app device authorized"
	case model.ConnectedAppNotificationEventDeviceDenied:
		subject = "Connected app device denied"
	case model.ConnectedAppNotificationEventDeviceRevoked:
		subject = "Connected app device revoked"
	case model.ConnectedAppNotificationEventGrantRevoked:
		subject = "Connected app grant revoked"
	case model.ConnectedAppNotificationEventHealthWarning:
		subject = "Connected app health warning"
	case model.ConnectedAppNotificationEventTokenRevoked:
		subject = "Connected app token revoked"
	case model.ConnectedAppNotificationEventTokenRotated:
		subject = "Connected app token rotated"
	}
	content := fmt.Sprintf(
		`<p>%s</p><p>Event: %s</p><p>Target: %s #%d</p>`,
		html.EscapeString(subject),
		html.EscapeString(row.EventType),
		html.EscapeString(row.TargetType),
		row.TargetId,
	)
	return subject, content
}

func ProcessConnectedAppNotificationOutboxBatch(batchSize int) (int, int, error) {
	stats, err := ProcessConnectedAppNotificationOutboxBatchWithStats(batchSize)
	if err != nil {
		return stats.Sent, stats.Failed, err
	}
	return stats.Sent, stats.Failed, nil
}

func ProcessConnectedAppNotificationOutboxBatchWithStats(batchSize int) (stats ConnectedAppNotificationOutboxBatchStats, err error) {
	started := time.Now()
	now := common.GetTimestamp()
	stats = ConnectedAppNotificationOutboxBatchStats{StartedAt: now}
	defer func() {
		stats.DurationMs = time.Since(started).Milliseconds()
		stats.FinishedAt = common.GetTimestamp()
		recordConnectedAppNotificationOutboxWorkerMetrics(stats)
	}()
	rows, err := ClaimDueConnectedAppNotificationOutbox(batchSize, now)
	if err != nil {
		return stats, err
	}
	stats.Claimed = len(rows)
	for _, row := range rows {
		if err := DeliverConnectedAppNotificationOutbox(row); err != nil {
			stats.Failed++
			if markErr := MarkConnectedAppNotificationOutboxFailed(row.Id, err, common.GetTimestamp()); markErr != nil {
				return stats, markErr
			}
			var failedRow model.ConnectedAppNotificationOutbox
			if loadErr := model.DB.Select("id, status").Where("id = ?", row.Id).First(&failedRow).Error; loadErr == nil && failedRow.Status == model.ConnectedAppNotificationOutboxStatusPermanentFailed {
				stats.PermanentFailed++
			}
			continue
		}
		if err := MarkConnectedAppNotificationOutboxSent(row.Id, common.GetTimestamp()); err != nil {
			return stats, err
		}
		stats.Sent++
	}
	return stats, nil
}

func EnqueueConnectedAppRequestReviewOutboxWithDB(db *gorm.DB, request model.ConnectedAppRequest, app model.ConnectedApp, audit model.ConnectedAppAuditLog) error {
	eventType := strings.TrimSpace(audit.Action)
	if eventType == "" {
		if request.Status == model.ConnectedAppRequestStatusApproved {
			eventType = model.ConnectedAppAuditActionApprove
		} else {
			eventType = model.ConnectedAppAuditActionReject
		}
	}
	appId := request.AppId
	appSlug := request.Slug
	appName := request.Name
	if app.Id > 0 {
		appId = app.Id
		appSlug = app.Slug
		appName = app.Name
	}
	payload := ConnectedAppRequestReviewNotificationPayload{
		Version:         ConnectedAppWebhookPayloadVersion,
		RequestId:       request.Id,
		AuditLogId:      audit.Id,
		Action:          eventType,
		Status:          request.Status,
		AppId:           appId,
		AppSlug:         appSlug,
		AppName:         appName,
		ApplicantUserId: request.ApplicantUserId,
		ReviewerUserId:  request.ReviewerUserId,
		ReviewNote:      request.ReviewNote,
	}
	emailScope, emailEnabled, err := ConnectedAppNotificationEmailScopeWithDB(db, appId, eventType)
	if err != nil {
		return err
	}
	if emailEnabled {
		recipients := make([]int, 0, 1)
		if emailScope.Applicant && request.ApplicantUserId > 0 {
			recipients = append(recipients, request.ApplicantUserId)
		}
		emailByUserId, err := connectedAppNotificationRecipientEmails(db, recipients)
		if err != nil {
			return err
		}
		for _, recipientUserId := range recipients {
			if email := strings.TrimSpace(emailByUserId[recipientUserId]); email != "" {
				if _, err := EnqueueConnectedAppNotificationOutboxWithDB(db, ConnectedAppNotificationOutboxInput{
					EventKey:        connectedAppNotificationOutboxEventKey(eventType, "connected_app_request", request.Id, audit.Id, model.ConnectedAppNotificationOutboxChannelEmail, "user:"+strconv.Itoa(recipientUserId)),
					EventType:       eventType,
					AppId:           appId,
					RecipientUserId: recipientUserId,
					RecipientEmail:  email,
					Channel:         model.ConnectedAppNotificationOutboxChannelEmail,
					TargetType:      "connected_app_request",
					TargetId:        request.Id,
					Payload:         payload,
				}); err != nil {
					return err
				}
			}
		}
		if err := enqueueConnectedAppExplicitEmailRows(db, appId, eventType, "connected_app_request", request.Id, audit.Id, payload, emailScope.ExplicitEmails); err != nil {
			return err
		}
	}
	return enqueueConnectedAppWebhookRows(db, appId, eventType, "connected_app_request", request.Id, audit.Id, payload)
}

func EnqueueConnectedAppDeviceAuthorizationOutboxWithDB(db *gorm.DB, app model.ConnectedApp, session model.ConnectedAppDeviceSession) error {
	eventType := model.ConnectedAppNotificationEventDeviceAuthorized
	if session.Status == model.ConnectedAppDeviceSessionStatusDenied {
		eventType = model.ConnectedAppNotificationEventDeviceDenied
	}
	payload := ConnectedAppDeviceAuthorizationNotificationPayload{
		Version:           ConnectedAppWebhookPayloadVersion,
		SessionId:         session.Id,
		AppId:             app.Id,
		AppSlug:           app.Slug,
		AppName:           app.Name,
		UserId:            session.UserId,
		TokenId:           session.TokenId,
		Status:            session.Status,
		DeviceFingerprint: session.DeviceFingerprint,
		DeviceName:        session.DeviceName,
		Platform:          session.Platform,
		AppVersion:        session.AppVersion,
		Client:            session.Client,
	}
	emailScope, emailEnabled, err := ConnectedAppNotificationEmailScopeWithDB(db, app.Id, eventType)
	if err != nil {
		return err
	}
	if emailEnabled {
		recipients, err := connectedAppDeviceNotificationRecipientUserIds(db, app.Id, session.UserId, emailScope)
		if err != nil {
			return err
		}
		emailByUserId, err := connectedAppNotificationRecipientEmails(db, recipients)
		if err != nil {
			return err
		}
		for _, recipientUserId := range recipients {
			if email := strings.TrimSpace(emailByUserId[recipientUserId]); email != "" {
				if _, err := EnqueueConnectedAppNotificationOutboxWithDB(db, ConnectedAppNotificationOutboxInput{
					EventKey:        connectedAppNotificationOutboxEventKey(eventType, "connected_app_device_session", int(session.Id), 0, model.ConnectedAppNotificationOutboxChannelEmail, "user:"+strconv.Itoa(recipientUserId)),
					EventType:       eventType,
					AppId:           app.Id,
					RecipientUserId: recipientUserId,
					RecipientEmail:  email,
					Channel:         model.ConnectedAppNotificationOutboxChannelEmail,
					TargetType:      "connected_app_device_session",
					TargetId:        int(session.Id),
					Payload:         payload,
				}); err != nil {
					return err
				}
			}
		}
		if err := enqueueConnectedAppExplicitEmailRows(db, app.Id, eventType, "connected_app_device_session", int(session.Id), 0, payload, emailScope.ExplicitEmails); err != nil {
			return err
		}
	}
	return enqueueConnectedAppWebhookRows(db, app.Id, eventType, "connected_app_device_session", int(session.Id), 0, payload)
}

func EnqueueConnectedAppHealthWarningOutboxWithDB(db *gorm.DB, input ConnectedAppHealthWarningInput) error {
	if strings.TrimSpace(input.Status) == "" || input.Status == "ok" {
		return nil
	}
	now := input.CreatedAt
	if now <= 0 {
		now = common.GetTimestamp()
	}
	sessionId := int64(0)
	deviceFingerprint := ""
	deviceName := ""
	platform := ""
	appVersion := ""
	client := ""
	if input.Session != nil {
		sessionId = input.Session.Id
		deviceFingerprint = input.Session.DeviceFingerprint
		deviceName = input.Session.DeviceName
		platform = input.Session.Platform
		appVersion = input.Session.AppVersion
		client = input.Session.Client
	}
	payload := ConnectedAppHealthWarningNotificationPayload{
		Version:           ConnectedAppWebhookPayloadVersion,
		AppId:             input.App.Id,
		AppSlug:           input.App.Slug,
		AppName:           input.App.Name,
		UserId:            input.UserId,
		SessionId:         sessionId,
		Status:            input.Status,
		Checks:            input.Checks,
		DeviceFingerprint: deviceFingerprint,
		DeviceName:        deviceName,
		Platform:          platform,
		AppVersion:        appVersion,
		Client:            client,
	}
	targetId := input.UserId
	targetType := "connected_app_health"
	if sessionId > 0 {
		targetId = int(sessionId)
		targetType = "connected_app_device_session"
	}
	dailyAuditKey := int64(0)
	if !time.Unix(now, 0).IsZero() {
		day := time.Unix(now, 0).UTC().Format("20060102")
		if parsed, err := strconv.ParseInt(day, 10, 64); err == nil {
			dailyAuditKey = parsed
		}
	}
	emailScope, emailEnabled, err := ConnectedAppNotificationEmailScopeWithDB(db, input.App.Id, model.ConnectedAppNotificationEventHealthWarning)
	if err != nil {
		return err
	}
	if emailEnabled {
		recipients, err := connectedAppDeviceNotificationRecipientUserIds(db, input.App.Id, input.UserId, emailScope)
		if err != nil {
			return err
		}
		emailByUserId, err := connectedAppNotificationRecipientEmails(db, recipients)
		if err != nil {
			return err
		}
		for _, recipientUserId := range recipients {
			if email := strings.TrimSpace(emailByUserId[recipientUserId]); email != "" {
				if _, err := EnqueueConnectedAppNotificationOutboxWithDB(db, ConnectedAppNotificationOutboxInput{
					EventKey:        connectedAppNotificationOutboxEventKey(model.ConnectedAppNotificationEventHealthWarning+"."+input.Status, targetType, targetId, dailyAuditKey, model.ConnectedAppNotificationOutboxChannelEmail, "user:"+strconv.Itoa(recipientUserId)),
					EventType:       model.ConnectedAppNotificationEventHealthWarning,
					AppId:           input.App.Id,
					RecipientUserId: recipientUserId,
					RecipientEmail:  email,
					Channel:         model.ConnectedAppNotificationOutboxChannelEmail,
					TargetType:      targetType,
					TargetId:        targetId,
					Payload:         payload,
				}); err != nil {
					return err
				}
			}
		}
		if err := enqueueConnectedAppExplicitEmailRows(db, input.App.Id, model.ConnectedAppNotificationEventHealthWarning+"."+input.Status, targetType, targetId, dailyAuditKey, payload, emailScope.ExplicitEmails); err != nil {
			return err
		}
	}
	return enqueueConnectedAppWebhookRows(db, input.App.Id, model.ConnectedAppNotificationEventHealthWarning+"."+input.Status, targetType, targetId, dailyAuditKey, payload)
}

func EnqueueConnectedAppTokenLifecycleOutboxWithDB(db *gorm.DB, input ConnectedAppTokenLifecycleNotificationInput) error {
	eventType := strings.TrimSpace(input.EventType)
	if !isConnectedAppTokenLifecycleNotificationEventType(eventType) {
		return fmt.Errorf("invalid connected app token lifecycle event type: %s", eventType)
	}
	if input.App.Id <= 0 || input.UserId <= 0 {
		return nil
	}
	if db == nil {
		db = model.DB
	}
	now := input.OccurredAt
	if now <= 0 {
		now = common.GetTimestamp()
	}
	tokenId := input.TokenId
	if tokenId == 0 {
		tokenId = input.NewTokenId
	}
	if tokenId == 0 {
		tokenId = input.PreviousTokenId
	}
	status := strings.TrimSpace(input.Status)
	if status == "" {
		switch eventType {
		case model.ConnectedAppNotificationEventTokenRotated:
			status = "rotated"
		case model.ConnectedAppNotificationEventTokenRevoked, model.ConnectedAppNotificationEventDeviceRevoked, model.ConnectedAppNotificationEventGrantRevoked:
			status = "revoked"
		default:
			status = eventType
		}
	}
	payload := ConnectedAppTokenLifecycleNotificationPayload{
		Version:           ConnectedAppWebhookPayloadVersion,
		AppId:             input.App.Id,
		AppSlug:           input.App.Slug,
		AppName:           input.App.Name,
		UserId:            input.UserId,
		GrantId:           input.GrantId,
		BindingId:         input.BindingId,
		TokenId:           tokenId,
		PreviousTokenId:   input.PreviousTokenId,
		NewTokenId:        input.NewTokenId,
		Status:            status,
		DeviceFingerprint: input.DeviceFingerprint,
		DeviceName:        input.DeviceName,
		Platform:          input.Platform,
		AppVersion:        input.AppVersion,
		OccurredAt:        now,
	}
	targetType, targetId := connectedAppTokenLifecycleNotificationTarget(input)
	if targetId <= 0 {
		return nil
	}
	auditId := connectedAppTokenLifecycleNotificationAuditId(input, tokenId, now)
	emailScope, emailEnabled, err := ConnectedAppNotificationEmailScopeWithDB(db, input.App.Id, eventType)
	if err != nil {
		return err
	}
	if emailEnabled {
		recipients, err := connectedAppDeviceNotificationRecipientUserIds(db, input.App.Id, input.UserId, emailScope)
		if err != nil {
			return err
		}
		emailByUserId, err := connectedAppNotificationRecipientEmails(db, recipients)
		if err != nil {
			return err
		}
		for _, recipientUserId := range recipients {
			if email := strings.TrimSpace(emailByUserId[recipientUserId]); email != "" {
				if _, err := EnqueueConnectedAppNotificationOutboxWithDB(db, ConnectedAppNotificationOutboxInput{
					EventKey:        connectedAppNotificationOutboxEventKey(eventType, targetType, targetId, auditId, model.ConnectedAppNotificationOutboxChannelEmail, "user:"+strconv.Itoa(recipientUserId)),
					EventType:       eventType,
					AppId:           input.App.Id,
					RecipientUserId: recipientUserId,
					RecipientEmail:  email,
					Channel:         model.ConnectedAppNotificationOutboxChannelEmail,
					TargetType:      targetType,
					TargetId:        targetId,
					Payload:         payload,
				}); err != nil {
					return err
				}
			}
		}
		if err := enqueueConnectedAppExplicitEmailRows(db, input.App.Id, eventType, targetType, targetId, auditId, payload, emailScope.ExplicitEmails); err != nil {
			return err
		}
	}
	return enqueueConnectedAppWebhookRows(db, input.App.Id, eventType, targetType, targetId, auditId, payload)
}

func isConnectedAppTokenLifecycleNotificationEventType(eventType string) bool {
	switch eventType {
	case model.ConnectedAppNotificationEventDeviceRevoked,
		model.ConnectedAppNotificationEventGrantRevoked,
		model.ConnectedAppNotificationEventTokenRevoked,
		model.ConnectedAppNotificationEventTokenRotated:
		return true
	default:
		return false
	}
}

func connectedAppTokenLifecycleNotificationTarget(input ConnectedAppTokenLifecycleNotificationInput) (string, int) {
	if input.EventType == model.ConnectedAppNotificationEventGrantRevoked {
		return "connected_app_grant", int(input.GrantId)
	}
	if input.BindingId > 0 {
		return "connected_app_token_binding", int(input.BindingId)
	}
	if input.TokenId > 0 {
		return "connected_app_token", input.TokenId
	}
	if input.NewTokenId > 0 {
		return "connected_app_token", input.NewTokenId
	}
	if input.PreviousTokenId > 0 {
		return "connected_app_token", input.PreviousTokenId
	}
	return "connected_app_user", input.UserId
}

func connectedAppTokenLifecycleNotificationAuditId(input ConnectedAppTokenLifecycleNotificationInput, tokenId int, now int64) int64 {
	switch input.EventType {
	case model.ConnectedAppNotificationEventTokenRotated:
		if input.PreviousTokenId > 0 {
			return int64(input.PreviousTokenId)
		}
		if input.NewTokenId > 0 {
			return int64(input.NewTokenId)
		}
	case model.ConnectedAppNotificationEventGrantRevoked:
		if tokenId > 0 {
			return int64(tokenId)
		}
		return now
	}
	if tokenId > 0 {
		return int64(tokenId)
	}
	return now
}

func connectedAppDeviceNotificationRecipientUserIds(db *gorm.DB, appId int, authorizingUserId int, scope ConnectedAppNotificationRecipientScope) ([]int, error) {
	seen := map[int]struct{}{}
	recipients := make([]int, 0, 2)
	add := func(userId int) {
		if userId <= 0 {
			return
		}
		if _, ok := seen[userId]; ok {
			return
		}
		seen[userId] = struct{}{}
		recipients = append(recipients, userId)
	}
	if scope.AuthorizingUser {
		add(authorizingUserId)
	}
	if scope.AppDevelopers && appId > 0 {
		var requests []model.ConnectedAppRequest
		if err := db.Select("applicant_user_id").Where("app_id = ? AND status = ?", appId, model.ConnectedAppRequestStatusApproved).Find(&requests).Error; err != nil {
			return nil, err
		}
		for _, request := range requests {
			add(request.ApplicantUserId)
		}
	}
	return recipients, nil
}

func connectedAppNotificationRecipientEmails(db *gorm.DB, userIds []int) (map[int]string, error) {
	result := map[int]string{}
	if len(userIds) == 0 {
		return result, nil
	}
	seen := map[int]struct{}{}
	ids := make([]int, 0, len(userIds))
	for _, userId := range userIds {
		if userId <= 0 {
			continue
		}
		if _, ok := seen[userId]; ok {
			continue
		}
		seen[userId] = struct{}{}
		ids = append(ids, userId)
	}
	if len(ids) == 0 {
		return result, nil
	}
	var users []model.User
	if err := db.Select("id, email").Where("id IN ? AND status = ?", ids, common.UserStatusEnabled).Find(&users).Error; err != nil {
		return nil, err
	}
	for _, user := range users {
		result[user.Id] = strings.TrimSpace(user.Email)
	}
	return result, nil
}

func enqueueConnectedAppExplicitEmailRows(db *gorm.DB, appId int, eventType string, targetType string, targetId int, auditId int64, payload any, emails []string) error {
	for _, email := range normalizeConnectedAppNotificationEmails(emails) {
		if _, err := EnqueueConnectedAppNotificationOutboxWithDB(db, ConnectedAppNotificationOutboxInput{
			EventKey:       connectedAppNotificationOutboxEventKey(eventType, targetType, targetId, auditId, model.ConnectedAppNotificationOutboxChannelEmail, "email:"+email),
			EventType:      strings.TrimSuffix(eventType, "."+connectedAppHealthStatusSuffix(eventType)),
			AppId:          appId,
			RecipientEmail: email,
			Channel:        model.ConnectedAppNotificationOutboxChannelEmail,
			TargetType:     targetType,
			TargetId:       targetId,
			Payload:        payload,
		}); err != nil {
			return err
		}
	}
	return nil
}

func enqueueConnectedAppWebhookRows(db *gorm.DB, appId int, eventType string, targetType string, targetId int, auditId int64, payload any) error {
	normalizedEventType := strings.TrimSuffix(eventType, "."+connectedAppHealthStatusSuffix(eventType))
	webhookEnabled, err := ConnectedAppNotificationChannelEnabledWithDB(db, appId, model.ConnectedAppNotificationOutboxChannelWebhook, normalizedEventType)
	if err != nil {
		return err
	}
	if !webhookEnabled {
		return nil
	}
	webhooks, err := ListEnabledConnectedAppWebhooksForEventWithDB(db, appId, normalizedEventType)
	if err != nil {
		return err
	}
	for _, webhook := range webhooks {
		if _, err := EnqueueConnectedAppNotificationOutboxWithDB(db, ConnectedAppNotificationOutboxInput{
			EventKey:       connectedAppNotificationOutboxEventKey(eventType, targetType, targetId, auditId, model.ConnectedAppNotificationOutboxChannelWebhook, "webhook:"+strconv.Itoa(webhook.Id)),
			EventType:      normalizedEventType,
			AppId:          appId,
			RecipientEmail: "webhook:" + strconv.Itoa(webhook.Id),
			Channel:        model.ConnectedAppNotificationOutboxChannelWebhook,
			TargetType:     targetType,
			TargetId:       targetId,
			Payload:        payload,
		}); err != nil {
			return err
		}
	}
	return nil
}

func connectedAppHealthStatusSuffix(eventType string) string {
	prefix := model.ConnectedAppNotificationEventHealthWarning + "."
	if strings.HasPrefix(eventType, prefix) {
		return strings.TrimPrefix(eventType, prefix)
	}
	return ""
}

func connectedAppNotificationOutboxEventKey(eventType string, targetType string, targetId int, auditId int64, channel string, recipient string) string {
	rawRecipient := strings.TrimSpace(recipient)
	sum := sha256.Sum256([]byte(rawRecipient))
	return fmt.Sprintf(
		"%s:%s:%d:%d:%s:%s",
		strings.TrimSpace(eventType),
		strings.TrimSpace(targetType),
		targetId,
		auditId,
		strings.TrimSpace(channel),
		hex.EncodeToString(sum[:8]),
	)
}
