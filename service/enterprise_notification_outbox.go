package service

import (
	"errors"
	"fmt"
	"html"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	EnterpriseNotificationOutboxWorkerBatchSize      = 100
	EnterpriseNotificationOutboxMaxRetryCount        = 5
	EnterpriseNotificationOutboxProcessingStaleAfter = 10 * time.Minute
)

var enterpriseNotificationOutboxSendEmail = common.SendEmail

var enterpriseNotificationOutboxMetricsMu sync.RWMutex

var enterpriseNotificationOutboxMetrics EnterpriseNotificationOutboxWorkerMetrics

const EnterpriseNotificationOutboxLastErrorMaxLength = 512

var enterpriseNotificationOutboxSensitiveQueryPattern = regexp.MustCompile(`(?i)(token|secret|key|signature|password)=([^&\s]+)`)

type EnterpriseNotificationOutboxInput struct {
	EventKey        string
	EventType       string
	EnterpriseId    int
	RecipientUserId int
	RecipientEmail  string
	Channel         string
	TargetType      string
	TargetId        int
	Payload         any
	NextRetryAt     int64
}

type EnterpriseNotificationOutboxListParams struct {
	EnterpriseId int
	Channel      string
	EventType    string
	Status       string
	TargetType   string
	TargetId     int
	WebhookId    int
	StartTime    int64
	EndTime      int64
	Offset       int
	Limit        int
}

type EnterpriseNotificationOutboxItem struct {
	Id              int64  `json:"id"`
	EventKey        string `json:"event_key"`
	EventType       string `json:"event_type"`
	EnterpriseId    int    `json:"enterprise_id"`
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

type EnterpriseNotificationOutboxBatchStats struct {
	Claimed         int   `json:"claimed"`
	Sent            int   `json:"sent"`
	Failed          int   `json:"failed"`
	PermanentFailed int   `json:"permanent_failed"`
	DurationMs      int64 `json:"duration_ms"`
	StartedAt       int64 `json:"started_at"`
	FinishedAt      int64 `json:"finished_at"`
}

type EnterpriseNotificationOutboxWorkerMetrics struct {
	LastRun              EnterpriseNotificationOutboxBatchStats `json:"last_run"`
	TotalRuns            int64                                  `json:"total_runs"`
	TotalClaimed         int64                                  `json:"total_claimed"`
	TotalSent            int64                                  `json:"total_sent"`
	TotalFailed          int64                                  `json:"total_failed"`
	TotalPermanentFailed int64                                  `json:"total_permanent_failed"`
}

func EnqueueEnterpriseNotificationOutbox(input EnterpriseNotificationOutboxInput) (bool, error) {
	return EnqueueEnterpriseNotificationOutboxWithDB(model.DB, input)
}

func ListEnterpriseNotificationOutbox(params EnterpriseNotificationOutboxListParams) ([]EnterpriseNotificationOutboxItem, int64, error) {
	if params.Limit <= 0 {
		params.Limit = 20
	}
	if params.Limit > 100 {
		params.Limit = 100
	}
	query := model.DB.Model(&model.EnterpriseNotificationOutbox{}).Where("enterprise_id = ?", params.EnterpriseId)
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
		query = query.Where("channel = ? AND recipient_email = ?", model.EnterpriseNotificationOutboxChannelWebhook, fmt.Sprintf("webhook:%d", params.WebhookId))
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
	var rows []model.EnterpriseNotificationOutbox
	if err := query.Order("created_at desc, id desc").Limit(params.Limit).Offset(params.Offset).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	items := make([]EnterpriseNotificationOutboxItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, EnterpriseNotificationOutboxToItem(row))
	}
	return items, total, nil
}

func EnterpriseNotificationOutboxToItem(row model.EnterpriseNotificationOutbox) EnterpriseNotificationOutboxItem {
	return EnterpriseNotificationOutboxItem{
		Id:              row.Id,
		EventKey:        row.EventKey,
		EventType:       row.EventType,
		EnterpriseId:    row.EnterpriseId,
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

func RetryEnterpriseNotificationOutbox(enterpriseId int, id int64) (model.EnterpriseNotificationOutbox, model.EnterpriseNotificationOutbox, error) {
	var before model.EnterpriseNotificationOutbox
	if err := model.DB.Where("id = ? AND enterprise_id = ?", id, enterpriseId).First(&before).Error; err != nil {
		return model.EnterpriseNotificationOutbox{}, model.EnterpriseNotificationOutbox{}, err
	}
	if before.Status != model.EnterpriseNotificationOutboxStatusFailed && before.Status != model.EnterpriseNotificationOutboxStatusPermanentFailed {
		return model.EnterpriseNotificationOutbox{}, model.EnterpriseNotificationOutbox{}, fmt.Errorf("notification outbox row %d is not retryable", id)
	}
	now := common.GetTimestamp()
	after := before
	after.Status = model.EnterpriseNotificationOutboxStatusPending
	after.RetryCount = 0
	after.NextRetryAt = 0
	after.LastError = ""
	after.UpdatedAt = now
	if err := model.DB.Model(&model.EnterpriseNotificationOutbox{}).
		Where("id = ? AND enterprise_id = ? AND status IN ?", id, enterpriseId, []string{model.EnterpriseNotificationOutboxStatusFailed, model.EnterpriseNotificationOutboxStatusPermanentFailed}).
		Updates(map[string]any{
			"status":        after.Status,
			"retry_count":   after.RetryCount,
			"next_retry_at": after.NextRetryAt,
			"last_error":    after.LastError,
			"updated_at":    after.UpdatedAt,
		}).Error; err != nil {
		return model.EnterpriseNotificationOutbox{}, model.EnterpriseNotificationOutbox{}, err
	}
	return before, after, nil
}

func GetEnterpriseNotificationOutboxWorkerMetrics() EnterpriseNotificationOutboxWorkerMetrics {
	enterpriseNotificationOutboxMetricsMu.RLock()
	defer enterpriseNotificationOutboxMetricsMu.RUnlock()
	return enterpriseNotificationOutboxMetrics
}

func recordEnterpriseNotificationOutboxWorkerMetrics(stats EnterpriseNotificationOutboxBatchStats) {
	enterpriseNotificationOutboxMetricsMu.Lock()
	defer enterpriseNotificationOutboxMetricsMu.Unlock()
	enterpriseNotificationOutboxMetrics.LastRun = stats
	enterpriseNotificationOutboxMetrics.TotalRuns++
	enterpriseNotificationOutboxMetrics.TotalClaimed += int64(stats.Claimed)
	enterpriseNotificationOutboxMetrics.TotalSent += int64(stats.Sent)
	enterpriseNotificationOutboxMetrics.TotalFailed += int64(stats.Failed)
	enterpriseNotificationOutboxMetrics.TotalPermanentFailed += int64(stats.PermanentFailed)
}

func maskEnterpriseNotificationRecipient(channel string, recipient string) string {
	recipient = strings.TrimSpace(recipient)
	if recipient == "" {
		return ""
	}
	if channel == model.EnterpriseNotificationOutboxChannelEmail {
		at := strings.Index(recipient, "@")
		if at <= 1 {
			return "***" + recipient[at:]
		}
		return recipient[:1] + "***" + recipient[at:]
	}
	return recipient
}

func sanitizeEnterpriseNotificationOutboxError(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	value = enterpriseNotificationOutboxSensitiveQueryPattern.ReplaceAllString(value, "$1=redacted")
	if len(value) > EnterpriseNotificationOutboxLastErrorMaxLength {
		return value[:EnterpriseNotificationOutboxLastErrorMaxLength] + "..."
	}
	return value
}

func EnqueueEnterpriseNotificationOutboxWithDB(db *gorm.DB, input EnterpriseNotificationOutboxInput) (bool, error) {
	row, err := buildEnterpriseNotificationOutbox(input)
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

func buildEnterpriseNotificationOutbox(input EnterpriseNotificationOutboxInput) (model.EnterpriseNotificationOutbox, error) {
	eventKey := strings.TrimSpace(input.EventKey)
	if eventKey == "" {
		return model.EnterpriseNotificationOutbox{}, errors.New("notification outbox event key is empty")
	}
	eventType := strings.TrimSpace(input.EventType)
	if eventType == "" {
		return model.EnterpriseNotificationOutbox{}, errors.New("notification outbox event type is empty")
	}
	targetType := strings.TrimSpace(input.TargetType)
	if targetType == "" {
		return model.EnterpriseNotificationOutbox{}, errors.New("notification outbox target type is empty")
	}
	payloadJson, err := enterpriseNotificationOutboxPayloadJson(input.Payload)
	if err != nil {
		return model.EnterpriseNotificationOutbox{}, err
	}
	channel := strings.TrimSpace(input.Channel)
	if channel == "" {
		channel = model.EnterpriseNotificationOutboxChannelInApp
	}
	return model.EnterpriseNotificationOutbox{
		EventKey:        eventKey,
		EventType:       eventType,
		EnterpriseId:    input.EnterpriseId,
		RecipientUserId: input.RecipientUserId,
		RecipientEmail:  strings.TrimSpace(input.RecipientEmail),
		Channel:         channel,
		TargetType:      targetType,
		TargetId:        input.TargetId,
		PayloadJson:     payloadJson,
		Status:          model.EnterpriseNotificationOutboxStatusPending,
		NextRetryAt:     input.NextRetryAt,
	}, nil
}

func enterpriseNotificationOutboxPayloadJson(value any) (string, error) {
	if value == nil {
		return "{}", nil
	}
	bytes, err := common.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func ClaimDueEnterpriseNotificationOutbox(batchSize int, now int64) ([]model.EnterpriseNotificationOutbox, error) {
	if batchSize <= 0 {
		batchSize = EnterpriseNotificationOutboxWorkerBatchSize
	}
	if now <= 0 {
		now = common.GetTimestamp()
	}
	processingStaleBefore := now - int64(EnterpriseNotificationOutboxProcessingStaleAfter/time.Second)
	var candidates []model.EnterpriseNotificationOutbox
	if err := model.DB.
		Where("(status IN ? AND next_retry_at <= ?) OR (status = ? AND updated_at <= ?)", []string{model.EnterpriseNotificationOutboxStatusPending, model.EnterpriseNotificationOutboxStatusFailed}, now, model.EnterpriseNotificationOutboxStatusProcessing, processingStaleBefore).
		Order("next_retry_at asc, id asc").
		Limit(batchSize).
		Find(&candidates).Error; err != nil {
		return nil, err
	}
	claimed := make([]model.EnterpriseNotificationOutbox, 0, len(candidates))
	for _, candidate := range candidates {
		result := model.DB.Model(&model.EnterpriseNotificationOutbox{}).
			Where("id = ? AND ((status IN ? AND next_retry_at <= ?) OR (status = ? AND updated_at <= ?))", candidate.Id, []string{model.EnterpriseNotificationOutboxStatusPending, model.EnterpriseNotificationOutboxStatusFailed}, now, model.EnterpriseNotificationOutboxStatusProcessing, processingStaleBefore).
			Updates(map[string]any{
				"status":     model.EnterpriseNotificationOutboxStatusProcessing,
				"updated_at": now,
			})
		if result.Error != nil {
			return claimed, result.Error
		}
		if result.RowsAffected == 0 {
			continue
		}
		candidate.Status = model.EnterpriseNotificationOutboxStatusProcessing
		candidate.UpdatedAt = now
		claimed = append(claimed, candidate)
	}
	return claimed, nil
}

func MarkEnterpriseNotificationOutboxSent(id int64, now int64) error {
	if now <= 0 {
		now = common.GetTimestamp()
	}
	return model.DB.Model(&model.EnterpriseNotificationOutbox{}).
		Where("id = ? AND status = ?", id, model.EnterpriseNotificationOutboxStatusProcessing).
		Updates(map[string]any{
			"status":        model.EnterpriseNotificationOutboxStatusSent,
			"last_error":    "",
			"next_retry_at": int64(0),
			"updated_at":    now,
		}).Error
}

func MarkEnterpriseNotificationOutboxFailed(id int64, err error, now int64) error {
	if now <= 0 {
		now = common.GetTimestamp()
	}
	errorMessage := "notification outbox delivery failed"
	if err != nil {
		errorMessage = err.Error()
	}
	var row model.EnterpriseNotificationOutbox
	if loadErr := model.DB.Where("id = ?", id).First(&row).Error; loadErr != nil {
		return loadErr
	}
	nextRetryCount := row.RetryCount + 1
	status := model.EnterpriseNotificationOutboxStatusFailed
	nextRetryAt := now + enterpriseNotificationOutboxRetryDelaySeconds(nextRetryCount)
	if nextRetryCount >= EnterpriseNotificationOutboxMaxRetryCount {
		status = model.EnterpriseNotificationOutboxStatusPermanentFailed
		nextRetryAt = 0
	}
	return model.DB.Model(&model.EnterpriseNotificationOutbox{}).
		Where("id = ? AND status = ?", id, model.EnterpriseNotificationOutboxStatusProcessing).
		Updates(map[string]any{
			"status":        status,
			"retry_count":   nextRetryCount,
			"next_retry_at": nextRetryAt,
			"last_error":    errorMessage,
			"updated_at":    now,
		}).Error
}

func enterpriseNotificationOutboxRetryDelaySeconds(retryCount int) int64 {
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

func DeliverEnterpriseNotificationOutbox(row model.EnterpriseNotificationOutbox) error {
	switch row.Channel {
	case model.EnterpriseNotificationOutboxChannelInApp:
		return nil
	case model.EnterpriseNotificationOutboxChannelEmail:
		return deliverEnterpriseNotificationOutboxEmail(row)
	case model.EnterpriseNotificationOutboxChannelWebhook:
		return DeliverEnterpriseWebhookOutbox(row)
	default:
		return fmt.Errorf("unsupported notification outbox channel: %s", row.Channel)
	}
}

func deliverEnterpriseNotificationOutboxEmail(row model.EnterpriseNotificationOutbox) error {
	receiver := strings.TrimSpace(row.RecipientEmail)
	if receiver == "" && row.RecipientUserId > 0 {
		var user model.User
		if err := model.DB.Select("id, email").Where("id = ?", row.RecipientUserId).First(&user).Error; err != nil {
			return err
		}
		receiver = strings.TrimSpace(user.Email)
	}
	if receiver == "" {
		return errors.New("email notification receiver is empty")
	}
	if strings.TrimSpace(common.SMTPServer) == "" && strings.TrimSpace(common.SMTPAccount) == "" {
		return errors.New("SMTP server is not configured")
	}
	subject, content := enterpriseNotificationOutboxEmailMessage(row)
	return enterpriseNotificationOutboxSendEmail(subject, receiver, content)
}

func enterpriseNotificationOutboxEmailMessage(row model.EnterpriseNotificationOutbox) (string, string) {
	subject := "Enterprise notification"
	switch row.EventType {
	case "quota_request.submit":
		subject = "Quota request pending"
	case "quota_request.approve":
		subject = "Quota request approved"
	case "quota_request.reject":
		subject = "Quota request rejected"
	case "quota_request.withdraw":
		subject = "Quota request withdrawn"
	case "quota_request.expire":
		subject = "Quota request expired"
	case "quota_request.expiring_soon":
		subject = "Quota request expiring soon"
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

func ProcessEnterpriseNotificationOutboxBatch(batchSize int) (int, int, error) {
	stats, err := ProcessEnterpriseNotificationOutboxBatchWithStats(batchSize)
	if err != nil {
		return stats.Sent, stats.Failed, err
	}
	return stats.Sent, stats.Failed, nil
}

func ProcessEnterpriseNotificationOutboxBatchWithStats(batchSize int) (stats EnterpriseNotificationOutboxBatchStats, err error) {
	started := time.Now()
	now := common.GetTimestamp()
	stats = EnterpriseNotificationOutboxBatchStats{StartedAt: now}
	defer func() {
		stats.DurationMs = time.Since(started).Milliseconds()
		stats.FinishedAt = common.GetTimestamp()
		recordEnterpriseNotificationOutboxWorkerMetrics(stats)
	}()
	rows, err := ClaimDueEnterpriseNotificationOutbox(batchSize, now)
	if err != nil {
		return stats, err
	}
	stats.Claimed = len(rows)
	for _, row := range rows {
		if err := DeliverEnterpriseNotificationOutbox(row); err != nil {
			stats.Failed++
			if markErr := MarkEnterpriseNotificationOutboxFailed(row.Id, err, common.GetTimestamp()); markErr != nil {
				return stats, markErr
			}
			var failedRow model.EnterpriseNotificationOutbox
			if loadErr := model.DB.Select("id, status").Where("id = ?", row.Id).First(&failedRow).Error; loadErr == nil && failedRow.Status == model.EnterpriseNotificationOutboxStatusPermanentFailed {
				stats.PermanentFailed++
			}
			continue
		}
		if err := MarkEnterpriseNotificationOutboxSent(row.Id, common.GetTimestamp()); err != nil {
			return stats, err
		}
		stats.Sent++
	}
	return stats, nil
}
