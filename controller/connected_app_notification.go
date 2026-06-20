package controller

import (
	"errors"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type connectedAppWebhookRequest struct {
	AppId      int      `json:"app_id"`
	Name       string   `json:"name"`
	Url        string   `json:"url"`
	Secret     *string  `json:"secret"`
	EventTypes []string `json:"event_types"`
	Status     int      `json:"status"`
}

type connectedAppNotificationPreferenceRequest struct {
	AppId          int                                            `json:"app_id"`
	Channel        string                                         `json:"channel"`
	EventType      string                                         `json:"event_type"`
	Enabled        bool                                           `json:"enabled"`
	RecipientScope service.ConnectedAppNotificationRecipientScope `json:"recipient_scope"`
}

func AdminListConnectedAppNotificationPreferences(c *gin.Context) {
	appId, _, err := connectedAppNotificationAppIdQuery(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	items, err := service.ListConnectedAppNotificationPreferences(appId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, items)
}

func AdminUpdateConnectedAppNotificationPreference(c *gin.Context) {
	var req connectedAppNotificationPreferenceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := ensureConnectedAppNotificationAppExists(req.AppId); err != nil {
		common.ApiError(c, err)
		return
	}
	before, after, err := service.UpsertConnectedAppNotificationPreference(service.ConnectedAppNotificationPreferenceUpsertInput{
		AppId:          req.AppId,
		Channel:        req.Channel,
		EventType:      req.EventType,
		Enabled:        req.Enabled,
		RecipientScope: req.RecipientScope,
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordConnectedAppAuditBestEffort(c, "connected_app_notification_preference.update", "connected_app_notification_preference", after.Id, sanitizeConnectedAppNotificationPreferenceAuditValue(before), sanitizeConnectedAppNotificationPreferenceAuditValue(after))
	item, err := service.ConnectedAppNotificationPreferenceToItem(after)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, item)
}

func DeveloperListConnectedAppNotificationPreferences(c *gin.Context) {
	app, _, err := connectedAppForDeveloper(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	items, err := service.ListConnectedAppNotificationPreferences(app.Id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, items)
}

func DeveloperUpdateConnectedAppNotificationPreference(c *gin.Context) {
	app, _, err := connectedAppForDeveloper(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var req connectedAppNotificationPreferenceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	before, after, err := service.UpsertConnectedAppNotificationPreference(service.ConnectedAppNotificationPreferenceUpsertInput{
		AppId:          app.Id,
		Channel:        req.Channel,
		EventType:      req.EventType,
		Enabled:        req.Enabled,
		RecipientScope: req.RecipientScope,
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordConnectedAppAuditBestEffort(c, "connected_app_notification_preference.update", "connected_app_notification_preference", after.Id, sanitizeConnectedAppNotificationPreferenceAuditValue(before), sanitizeConnectedAppNotificationPreferenceAuditValue(after))
	item, err := service.ConnectedAppNotificationPreferenceToItem(after)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, item)
}

func AdminListConnectedAppWebhooks(c *gin.Context) {
	appId, _, err := connectedAppNotificationAppIdQuery(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	items, err := service.ListConnectedAppWebhooks(appId, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, items)
}

func AdminCreateConnectedAppWebhook(c *gin.Context) {
	var req connectedAppWebhookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := ensureConnectedAppNotificationAppExists(req.AppId); err != nil {
		common.ApiError(c, err)
		return
	}
	webhook, err := service.CreateConnectedAppWebhook(connectedAppWebhookInputFromRequest(req))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordConnectedAppAuditBestEffort(c, "connected_app_webhook.create", "connected_app_webhook", webhook.Id, nil, sanitizeConnectedAppWebhookAuditValue(webhook))
	common.ApiSuccess(c, service.ConnectedAppWebhookToItem(webhook))
}

func AdminUpdateConnectedAppWebhook(c *gin.Context) {
	id, err := parsePathInt(c, "id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var before model.ConnectedAppWebhook
	if err := model.DB.Where("id = ?", id).First(&before).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	var req connectedAppWebhookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	req.AppId = before.AppId
	_, after, err := service.UpdateConnectedAppWebhook(before.AppId, id, connectedAppWebhookInputFromRequest(req))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordConnectedAppAuditBestEffort(c, "connected_app_webhook.update", "connected_app_webhook", id, sanitizeConnectedAppWebhookAuditValue(before), sanitizeConnectedAppWebhookAuditValue(after))
	common.ApiSuccess(c, service.ConnectedAppWebhookToItem(after))
}

func AdminDeleteConnectedAppWebhook(c *gin.Context) {
	id, err := parsePathInt(c, "id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var webhook model.ConnectedAppWebhook
	if err := model.DB.Where("id = ?", id).First(&webhook).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	before, after, err := service.DisableConnectedAppWebhook(webhook.AppId, id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordConnectedAppAuditBestEffort(c, "connected_app_webhook.disable", "connected_app_webhook", id, sanitizeConnectedAppWebhookAuditValue(before), sanitizeConnectedAppWebhookAuditValue(after))
	common.ApiSuccess(c, service.ConnectedAppWebhookToItem(after))
}

func AdminTestConnectedAppWebhook(c *gin.Context) {
	id, err := parsePathInt(c, "id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var webhook model.ConnectedAppWebhook
	if err := model.DB.Where("id = ?", id).First(&webhook).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	result, err := service.SendConnectedAppWebhookWithResult(webhook, service.BuildConnectedAppWebhookTestOutbox(webhook.AppId, webhook.Id))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordConnectedAppAuditBestEffort(c, "connected_app_webhook.test", "connected_app_webhook", id, sanitizeConnectedAppWebhookAuditValue(webhook), gin.H{
		"success":     result.Success,
		"status_code": result.StatusCode,
		"duration_ms": result.DurationMs,
		"error":       result.Error,
		"signed":      result.SignatureHeader != "",
	})
	common.ApiSuccess(c, result)
}

func DeveloperListConnectedAppWebhooks(c *gin.Context) {
	app, _, err := connectedAppForDeveloper(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	items, err := service.ListConnectedAppWebhooks(app.Id, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, items)
}

func DeveloperCreateConnectedAppWebhook(c *gin.Context) {
	app, _, err := connectedAppForDeveloper(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var req connectedAppWebhookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	req.AppId = app.Id
	webhook, err := service.CreateConnectedAppWebhook(connectedAppWebhookInputFromRequest(req))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordConnectedAppAuditBestEffort(c, "connected_app_webhook.create", "connected_app_webhook", webhook.Id, nil, sanitizeConnectedAppWebhookAuditValue(webhook))
	common.ApiSuccess(c, service.ConnectedAppWebhookToItem(webhook))
}

func DeveloperUpdateConnectedAppWebhook(c *gin.Context) {
	app, _, err := connectedAppForDeveloper(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	id, err := parsePathInt(c, "id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var req connectedAppWebhookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	req.AppId = app.Id
	before, after, err := service.UpdateConnectedAppWebhook(app.Id, id, connectedAppWebhookInputFromRequest(req))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordConnectedAppAuditBestEffort(c, "connected_app_webhook.update", "connected_app_webhook", id, sanitizeConnectedAppWebhookAuditValue(before), sanitizeConnectedAppWebhookAuditValue(after))
	common.ApiSuccess(c, service.ConnectedAppWebhookToItem(after))
}

func DeveloperDeleteConnectedAppWebhook(c *gin.Context) {
	app, _, err := connectedAppForDeveloper(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	id, err := parsePathInt(c, "id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	before, after, err := service.DisableConnectedAppWebhook(app.Id, id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordConnectedAppAuditBestEffort(c, "connected_app_webhook.disable", "connected_app_webhook", id, sanitizeConnectedAppWebhookAuditValue(before), sanitizeConnectedAppWebhookAuditValue(after))
	common.ApiSuccess(c, service.ConnectedAppWebhookToItem(after))
}

func DeveloperTestConnectedAppWebhook(c *gin.Context) {
	app, _, err := connectedAppForDeveloper(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	id, err := parsePathInt(c, "id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var webhook model.ConnectedAppWebhook
	if err := model.DB.Where("id = ? AND app_id = ?", id, app.Id).First(&webhook).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	result, err := service.SendConnectedAppWebhookWithResult(webhook, service.BuildConnectedAppWebhookTestOutbox(app.Id, webhook.Id))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordConnectedAppAuditBestEffort(c, "connected_app_webhook.test", "connected_app_webhook", id, sanitizeConnectedAppWebhookAuditValue(webhook), gin.H{
		"success":     result.Success,
		"status_code": result.StatusCode,
		"duration_ms": result.DurationMs,
		"error":       result.Error,
		"signed":      result.SignatureHeader != "",
	})
	common.ApiSuccess(c, result)
}

func AdminListConnectedAppNotificationOutbox(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	params, err := connectedAppNotificationOutboxListParamsFromQuery(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	params.Offset = pageInfo.GetStartIdx()
	params.Limit = pageInfo.GetPageSize()
	items, total, err := service.ListConnectedAppNotificationOutbox(params)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

func DeveloperListConnectedAppNotificationOutbox(c *gin.Context) {
	app, _, err := connectedAppForDeveloper(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo := common.GetPageQuery(c)
	params, err := connectedAppNotificationOutboxListParamsFromQuery(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	params.AppId = app.Id
	params.FilterAppId = true
	params.Offset = pageInfo.GetStartIdx()
	params.Limit = pageInfo.GetPageSize()
	items, total, err := service.ListConnectedAppNotificationOutbox(params)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

func AdminRetryConnectedAppNotificationOutbox(c *gin.Context) {
	id, err := parsePathInt64(c, "id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	before, after, err := service.RetryConnectedAppNotificationOutbox(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordConnectedAppAuditBestEffort(c, "connected_app_notification_outbox.retry", "connected_app_notification_outbox", int(id), service.ConnectedAppNotificationOutboxToItem(before), service.ConnectedAppNotificationOutboxToItem(after))
	common.ApiSuccess(c, service.ConnectedAppNotificationOutboxToItem(after))
}

func GetConnectedAppNotificationOutboxWorkerMetrics(c *gin.Context) {
	common.ApiSuccess(c, service.GetConnectedAppNotificationOutboxWorkerMetrics())
}

func connectedAppNotificationOutboxListParamsFromQuery(c *gin.Context) (service.ConnectedAppNotificationOutboxListParams, error) {
	appId, filterAppId, err := connectedAppNotificationAppIdQuery(c)
	if err != nil {
		return service.ConnectedAppNotificationOutboxListParams{}, err
	}
	targetId, err := parseOptionalIntQuery(c, "target_id")
	if err != nil {
		return service.ConnectedAppNotificationOutboxListParams{}, err
	}
	webhookId, err := parseOptionalIntQuery(c, "webhook_id")
	if err != nil {
		return service.ConnectedAppNotificationOutboxListParams{}, err
	}
	startTime, err := parseOptionalInt64Query(c, "start_time")
	if err != nil {
		return service.ConnectedAppNotificationOutboxListParams{}, err
	}
	endTime, err := parseOptionalInt64Query(c, "end_time")
	if err != nil {
		return service.ConnectedAppNotificationOutboxListParams{}, err
	}
	return service.ConnectedAppNotificationOutboxListParams{
		AppId:       appId,
		FilterAppId: filterAppId,
		Channel:     c.Query("channel"),
		EventType:   c.Query("event_type"),
		Status:      c.Query("status"),
		TargetType:  c.Query("target_type"),
		TargetId:    targetId,
		WebhookId:   webhookId,
		StartTime:   startTime,
		EndTime:     endTime,
	}, nil
}

func connectedAppNotificationAppIdQuery(c *gin.Context) (int, bool, error) {
	raw := strings.TrimSpace(c.Query("app_id"))
	if raw == "" {
		return 0, false, nil
	}
	appId, err := strconv.Atoi(raw)
	if err != nil || appId < 0 {
		return 0, false, errors.New("invalid connected app id")
	}
	return appId, true, nil
}

func ensureConnectedAppNotificationAppExists(appId int) error {
	if appId < 0 {
		return errors.New("invalid connected app id")
	}
	if appId == 0 {
		return nil
	}
	_, err := model.GetConnectedAppByID(appId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("connected app not found")
		}
		return err
	}
	return nil
}

func connectedAppWebhookInputFromRequest(req connectedAppWebhookRequest) service.ConnectedAppWebhookUpsertInput {
	return service.ConnectedAppWebhookUpsertInput{
		AppId:      req.AppId,
		Name:       req.Name,
		Url:        req.Url,
		Secret:     req.Secret,
		EventTypes: req.EventTypes,
		Status:     req.Status,
	}
}

func sanitizeConnectedAppWebhookAuditValue(webhook model.ConnectedAppWebhook) gin.H {
	return gin.H{
		"id":               webhook.Id,
		"app_id":           webhook.AppId,
		"name":             webhook.Name,
		"url":              redactEnterpriseWebhookURL(webhook.Url),
		"has_secret":       strings.TrimSpace(webhook.Secret) != "",
		"event_types_json": webhook.EventTypesJson,
		"status":           webhook.Status,
		"created_at":       webhook.CreatedAt,
		"updated_at":       webhook.UpdatedAt,
	}
}

func sanitizeConnectedAppNotificationPreferenceAuditValue(preference model.ConnectedAppNotificationPreference) gin.H {
	if preference.Id == 0 {
		return gin.H{}
	}
	scope, _ := service.ConnectedAppNotificationPreferenceToItem(preference)
	return gin.H{
		"id":                   preference.Id,
		"app_id":               preference.AppId,
		"channel":              preference.Channel,
		"event_type":           preference.EventType,
		"enabled":              preference.Enabled,
		"applicant":            scope.RecipientScope.Applicant,
		"authorizing_user":     scope.RecipientScope.AuthorizingUser,
		"app_developers":       scope.RecipientScope.AppDevelopers,
		"explicit_email_count": len(scope.RecipientScope.ExplicitEmails),
		"created_at":           preference.CreatedAt,
		"updated_at":           preference.UpdatedAt,
	}
}

func recordConnectedAppAuditBestEffort(c *gin.Context, action string, targetType string, targetId int, before any, after any) {
	if err := model.RecordConnectedAppAuditLog(model.ConnectedAppAuditInput{
		ActorUserId: c.GetInt("id"),
		Action:      action,
		TargetType:  targetType,
		TargetId:    targetId,
		Before:      before,
		After:       after,
		RequestId:   c.GetHeader(common.RequestIdKey),
	}); err != nil {
		common.SysLog("failed to record connected app audit: " + err.Error())
	}
}
