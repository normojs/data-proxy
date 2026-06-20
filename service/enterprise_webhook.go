package service

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"

	"gorm.io/gorm"
)

type EnterpriseWebhookPayload struct {
	EventId      string `json:"event_id"`
	EventType    string `json:"event_type"`
	EnterpriseId int    `json:"enterprise_id"`
	TargetType   string `json:"target_type"`
	TargetId     int    `json:"target_id"`
	CreatedAt    int64  `json:"created_at"`
	PayloadJson  string `json:"payload_json"`
}

type EnterpriseWebhookItem struct {
	Id             int      `json:"id"`
	EnterpriseId   int      `json:"enterprise_id"`
	Name           string   `json:"name"`
	Url            string   `json:"url"`
	HasSecret      bool     `json:"has_secret"`
	EventTypes     []string `json:"event_types"`
	EventTypesJson string   `json:"event_types_json"`
	Status         int      `json:"status"`
	CreatedAt      int64    `json:"created_at"`
	UpdatedAt      int64    `json:"updated_at"`
}

type EnterpriseWebhookUpsertInput struct {
	Name       string
	Url        string
	Secret     *string
	EventTypes []string
	Status     int
}

type EnterpriseWebhookTestResult struct {
	Success         bool   `json:"success"`
	StatusCode      int    `json:"status_code"`
	DurationMs      int64  `json:"duration_ms"`
	Error           string `json:"error"`
	SignatureHeader string `json:"signature_header"`
}

var enterpriseWebhookHTTPClient = GetHttpClient

func ListEnabledEnterpriseWebhooksForEvent(enterpriseId int, eventType string) ([]model.EnterpriseWebhook, error) {
	return ListEnabledEnterpriseWebhooksForEventWithDB(model.DB, enterpriseId, eventType)
}

func ListEnterpriseWebhooks(enterpriseId int) ([]EnterpriseWebhookItem, error) {
	var webhooks []model.EnterpriseWebhook
	if err := model.DB.Where("enterprise_id = ?", enterpriseId).Order("id desc").Find(&webhooks).Error; err != nil {
		return nil, err
	}
	items := make([]EnterpriseWebhookItem, 0, len(webhooks))
	for _, webhook := range webhooks {
		items = append(items, EnterpriseWebhookToItem(webhook))
	}
	return items, nil
}

func CreateEnterpriseWebhook(enterpriseId int, input EnterpriseWebhookUpsertInput) (model.EnterpriseWebhook, error) {
	webhook, err := enterpriseWebhookFromInput(enterpriseId, model.EnterpriseWebhook{}, input, true)
	if err != nil {
		return model.EnterpriseWebhook{}, err
	}
	return webhook, model.DB.Create(&webhook).Error
}

func UpdateEnterpriseWebhook(enterpriseId int, id int, input EnterpriseWebhookUpsertInput) (model.EnterpriseWebhook, model.EnterpriseWebhook, error) {
	var before model.EnterpriseWebhook
	if err := model.DB.Where("id = ? AND enterprise_id = ?", id, enterpriseId).First(&before).Error; err != nil {
		return model.EnterpriseWebhook{}, model.EnterpriseWebhook{}, err
	}
	after, err := enterpriseWebhookFromInput(enterpriseId, before, input, false)
	if err != nil {
		return model.EnterpriseWebhook{}, model.EnterpriseWebhook{}, err
	}
	after.Id = before.Id
	after.CreatedAt = before.CreatedAt
	return before, after, model.DB.Save(&after).Error
}

func DisableEnterpriseWebhook(enterpriseId int, id int) (model.EnterpriseWebhook, model.EnterpriseWebhook, error) {
	var before model.EnterpriseWebhook
	if err := model.DB.Where("id = ? AND enterprise_id = ?", id, enterpriseId).First(&before).Error; err != nil {
		return model.EnterpriseWebhook{}, model.EnterpriseWebhook{}, err
	}
	after := before
	after.Status = model.EnterpriseWebhookStatusDisabled
	return before, after, model.DB.Save(&after).Error
}

func EnterpriseWebhookToItem(webhook model.EnterpriseWebhook) EnterpriseWebhookItem {
	events, _ := parseEnterpriseWebhookEventTypes(webhook.EventTypesJson)
	return EnterpriseWebhookItem{
		Id:             webhook.Id,
		EnterpriseId:   webhook.EnterpriseId,
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

func enterpriseWebhookFromInput(enterpriseId int, existing model.EnterpriseWebhook, input EnterpriseWebhookUpsertInput, create bool) (model.EnterpriseWebhook, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return model.EnterpriseWebhook{}, errors.New("webhook name is required")
	}
	webhookUrl := strings.TrimSpace(input.Url)
	if err := validateEnterpriseWebhookURL(webhookUrl); err != nil {
		return model.EnterpriseWebhook{}, err
	}
	events := normalizeEnterpriseWebhookEventTypes(input.EventTypes)
	eventsJson, err := common.Marshal(events)
	if err != nil {
		return model.EnterpriseWebhook{}, err
	}
	status := input.Status
	if status == 0 {
		if create {
			status = model.EnterpriseWebhookStatusEnabled
		} else {
			status = existing.Status
		}
	}
	if status != model.EnterpriseWebhookStatusEnabled && status != model.EnterpriseWebhookStatusDisabled {
		return model.EnterpriseWebhook{}, errors.New("invalid webhook status")
	}
	secret := existing.Secret
	if input.Secret != nil {
		secret = strings.TrimSpace(*input.Secret)
	}
	return model.EnterpriseWebhook{
		Id:             existing.Id,
		EnterpriseId:   enterpriseId,
		Name:           name,
		Url:            webhookUrl,
		Secret:         secret,
		EventTypesJson: string(eventsJson),
		Status:         status,
		CreatedAt:      existing.CreatedAt,
	}, nil
}

func validateEnterpriseWebhookURL(value string) error {
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

func normalizeEnterpriseWebhookEventTypes(values []string) []string {
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

func parseEnterpriseWebhookEventTypes(value string) ([]string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, nil
	}
	var events []string
	if err := common.Unmarshal([]byte(trimmed), &events); err != nil {
		return nil, err
	}
	return normalizeEnterpriseWebhookEventTypes(events), nil
}

func ListEnabledEnterpriseWebhooksForEventWithDB(db *gorm.DB, enterpriseId int, eventType string) ([]model.EnterpriseWebhook, error) {
	var webhooks []model.EnterpriseWebhook
	if err := db.Where("enterprise_id = ? AND status = ?", enterpriseId, model.EnterpriseWebhookStatusEnabled).Find(&webhooks).Error; err != nil {
		return nil, err
	}
	matched := make([]model.EnterpriseWebhook, 0, len(webhooks))
	for _, webhook := range webhooks {
		if enterpriseWebhookMatchesEvent(webhook, eventType) {
			matched = append(matched, webhook)
		}
	}
	return matched, nil
}

func enterpriseWebhookMatchesEvent(webhook model.EnterpriseWebhook, eventType string) bool {
	trimmed := strings.TrimSpace(webhook.EventTypesJson)
	if trimmed == "" || trimmed == "[]" {
		return true
	}
	events, err := parseEnterpriseWebhookEventTypes(trimmed)
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

func EnterpriseWebhookSignature(secret string, payload []byte) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(payload)
	return "sha256=" + hex.EncodeToString(h.Sum(nil))
}

func BuildEnterpriseWebhookPayload(row model.EnterpriseNotificationOutbox) EnterpriseWebhookPayload {
	return EnterpriseWebhookPayload{
		EventId:      row.EventKey,
		EventType:    row.EventType,
		EnterpriseId: row.EnterpriseId,
		TargetType:   row.TargetType,
		TargetId:     row.TargetId,
		CreatedAt:    row.CreatedAt,
		PayloadJson:  row.PayloadJson,
	}
}

func DeliverEnterpriseWebhookOutbox(row model.EnterpriseNotificationOutbox) error {
	webhookId, err := enterpriseWebhookIdFromRecipient(row.RecipientEmail)
	if err != nil {
		return err
	}
	var webhook model.EnterpriseWebhook
	if err := model.DB.Where("id = ? AND enterprise_id = ? AND status = ?", webhookId, row.EnterpriseId, model.EnterpriseWebhookStatusEnabled).First(&webhook).Error; err != nil {
		return err
	}
	if !enterpriseWebhookMatchesEvent(webhook, row.EventType) {
		return fmt.Errorf("webhook %d is not subscribed to %s", webhook.Id, row.EventType)
	}
	return SendEnterpriseWebhook(webhook, row)
}

func SendEnterpriseWebhook(webhook model.EnterpriseWebhook, row model.EnterpriseNotificationOutbox) error {
	result, err := SendEnterpriseWebhookWithResult(webhook, row)
	if err != nil {
		return err
	}
	if !result.Success {
		return errors.New(result.Error)
	}
	return nil
}

func SendEnterpriseWebhookWithResult(webhook model.EnterpriseWebhook, row model.EnterpriseNotificationOutbox) (EnterpriseWebhookTestResult, error) {
	url := strings.TrimSpace(webhook.Url)
	if url == "" {
		return EnterpriseWebhookTestResult{}, errors.New("enterprise webhook URL is empty")
	}
	payloadBytes, err := common.Marshal(BuildEnterpriseWebhookPayload(row))
	if err != nil {
		return EnterpriseWebhookTestResult{}, err
	}
	signature := ""
	if strings.TrimSpace(webhook.Secret) != "" {
		signature = EnterpriseWebhookSignature(webhook.Secret, payloadBytes)
	}
	startedAt := time.Now()
	result := EnterpriseWebhookTestResult{SignatureHeader: signature}
	if system_setting.EnableWorker() {
		workerReq := &WorkerRequest{
			URL:    url,
			Key:    system_setting.WorkerValidKey,
			Method: http.MethodPost,
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			Body: payloadBytes,
		}
		if signature != "" {
			workerReq.Headers["X-Enterprise-Webhook-Signature"] = signature
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
			result.Error = fmt.Sprintf("enterprise webhook request failed with status code: %d", resp.StatusCode)
			return result, nil
		}
		result.Success = true
		return result, nil
	}
	fetchSetting := system_setting.GetFetchSetting()
	if err := common.ValidateURLWithFetchSetting(url, fetchSetting.EnableSSRFProtection, fetchSetting.AllowPrivateIp, fetchSetting.DomainFilterMode, fetchSetting.IpFilterMode, fetchSetting.DomainList, fetchSetting.IpList, fetchSetting.AllowedPorts, fetchSetting.ApplyIPFilterForDomain); err != nil {
		return EnterpriseWebhookTestResult{}, err
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return EnterpriseWebhookTestResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	if signature != "" {
		req.Header.Set("X-Enterprise-Webhook-Signature", signature)
	}
	client := enterpriseWebhookHTTPClient()
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
		result.Error = enterpriseWebhookResponseError(resp)
		return result, nil
	}
	result.Success = true
	return result, nil
}

func enterpriseWebhookResponseError(resp *http.Response) string {
	message := fmt.Sprintf("enterprise webhook request failed with status code: %d", resp.StatusCode)
	if resp.Body == nil {
		return message
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 512))
	if err != nil || len(bytes.TrimSpace(body)) == 0 {
		return message
	}
	return message + ": " + string(bytes.TrimSpace(body))
}

func BuildEnterpriseWebhookTestOutbox(enterpriseId int, webhookId int) model.EnterpriseNotificationOutbox {
	now := common.GetTimestamp()
	return model.EnterpriseNotificationOutbox{
		EventKey:       fmt.Sprintf("enterprise.webhook.test:%d:%d", webhookId, now),
		EventType:      "enterprise.webhook.test",
		EnterpriseId:   enterpriseId,
		RecipientEmail: fmt.Sprintf("webhook:%d", webhookId),
		Channel:        model.EnterpriseNotificationOutboxChannelWebhook,
		TargetType:     "enterprise_webhook",
		TargetId:       webhookId,
		PayloadJson:    `{"message":"test"}`,
		Status:         model.EnterpriseNotificationOutboxStatusProcessing,
		NextRetryAt:    0,
		RetryCount:     0,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

func enterpriseWebhookIdFromRecipient(value string) (int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, errors.New("enterprise webhook recipient is empty")
	}
	var id int
	_, err := fmt.Sscanf(value, "webhook:%d", &id)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("invalid enterprise webhook recipient: %s", value)
	}
	return id, nil
}
