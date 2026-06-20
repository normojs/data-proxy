package controller

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type connectedAppAccessRequestPayload struct {
	Slug              string   `json:"slug"`
	Name              string   `json:"name"`
	Description       string   `json:"description"`
	RequestedScopes   []string `json:"requested_scopes"`
	DefaultScopes     []string `json:"default_scopes"`
	AuthorizationFlow string   `json:"authorization_flow"`
	HomepageURL       string   `json:"homepage_url"`
	CallbackURL       string   `json:"callback_url"`
	Reason            string   `json:"reason"`
}

type connectedAppReviewPayload struct {
	Decision          string   `json:"decision"`
	ReviewNote        string   `json:"review_note"`
	Name              *string  `json:"name"`
	Description       *string  `json:"description"`
	AllowedScopes     []string `json:"allowed_scopes"`
	DefaultScopes     []string `json:"default_scopes"`
	AuthorizationFlow *string  `json:"authorization_flow"`
	HomepageURL       *string  `json:"homepage_url"`
	CallbackURL       *string  `json:"callback_url"`
}

type connectedAppAccessRequestResponse struct {
	ID                int      `json:"id"`
	ApplicantUserID   int      `json:"applicant_user_id"`
	ApplicantName     string   `json:"applicant_name"`
	AppID             int      `json:"app_id"`
	Slug              string   `json:"slug"`
	Name              string   `json:"name"`
	Description       string   `json:"description"`
	RequestedScopes   []string `json:"requested_scopes"`
	DefaultScopes     []string `json:"default_scopes"`
	AuthorizationFlow string   `json:"authorization_flow"`
	HomepageURL       string   `json:"homepage_url"`
	CallbackURL       string   `json:"callback_url"`
	Reason            string   `json:"reason"`
	Status            string   `json:"status"`
	ReviewerUserID    int      `json:"reviewer_user_id"`
	ReviewerName      string   `json:"reviewer_name"`
	ReviewNote        string   `json:"review_note"`
	ReviewedAt        int64    `json:"reviewed_at"`
	CreatedAt         int64    `json:"created_at"`
	UpdatedAt         int64    `json:"updated_at"`
}

type connectedAppAuditLogResponse struct {
	ID          int64  `json:"id"`
	ActorUserID int    `json:"actor_user_id"`
	ActorName   string `json:"actor_name"`
	Action      string `json:"action"`
	TargetType  string `json:"target_type"`
	TargetID    int    `json:"target_id"`
	BeforeJSON  string `json:"before_json"`
	AfterJSON   string `json:"after_json"`
	RequestID   string `json:"request_id"`
	CreatedAt   int64  `json:"created_at"`
}

func SubmitConnectedAppRequest(c *gin.Context) {
	var req connectedAppAccessRequestPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "Invalid request body")
		return
	}

	request, err := buildConnectedAppAccessRequestForSubmit(req, c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := ensureConnectedAppRequestSlugAvailable(request.Slug); err != nil {
		common.ApiError(c, err)
		return
	}

	var audit model.ConnectedAppAuditLog
	if err := model.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&request).Error; err != nil {
			return err
		}
		createdAudit, err := model.RecordConnectedAppAuditLogWithDB(tx, model.ConnectedAppAuditInput{
			ActorUserId: c.GetInt("id"),
			Action:      model.ConnectedAppAuditActionSubmit,
			TargetType:  model.ConnectedAppAuditTargetRequest,
			TargetId:    request.Id,
			After:       request,
			RequestId:   c.GetHeader(common.RequestIdKey),
		})
		if err != nil {
			return err
		}
		audit = createdAudit
		return nil
	}); err != nil {
		common.ApiError(c, err)
		return
	}

	resp, err := buildConnectedAppAccessRequestResponse(request, connectedAppUserNameMap(request.ApplicantUserId, request.ReviewerUserId))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"request": resp,
		"audit":   buildConnectedAppAuditLogResponse(audit, connectedAppUserNameMap(audit.ActorUserId)),
	})
}

func ListSelfConnectedAppRequests(c *gin.Context) {
	listConnectedAppRequests(c, c.GetInt("id"))
}

func AdminListConnectedAppRequests(c *gin.Context) {
	listConnectedAppRequests(c, 0)
}

func listConnectedAppRequests(c *gin.Context, applicantUserId int) {
	pageInfo := common.GetPageQuery(c)
	requests, total, err := model.ListConnectedAppRequests(c.Query("status"), applicantUserId, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	items, err := buildConnectedAppAccessRequestResponses(requests)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"items":     items,
		"total":     total,
		"page":      pageInfo.GetPage(),
		"page_size": pageInfo.GetPageSize(),
	})
}

func ReviewConnectedAppRequest(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "Invalid connected app request id")
		return
	}

	var req connectedAppReviewPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "Invalid request body")
		return
	}

	var reviewed model.ConnectedAppRequest
	var app model.ConnectedApp
	var audit model.ConnectedAppAuditLog
	if err := model.DB.Transaction(func(tx *gorm.DB) error {
		var current model.ConnectedAppRequest
		if err := tx.Where("id = ?", id).First(&current).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("connected app request not found")
			}
			return err
		}
		if current.Status != model.ConnectedAppRequestStatusPending {
			return fmt.Errorf("connected app request has already been reviewed")
		}
		before := current
		decision := strings.ToLower(strings.TrimSpace(req.Decision))
		switch decision {
		case model.ConnectedAppRequestStatusApproved:
			approvedApp, err := buildConnectedAppFromReviewPayload(current, req)
			if err != nil {
				return err
			}
			var existing model.ConnectedApp
			if err := tx.Where("slug = ?", approvedApp.Slug).First(&existing).Error; err == nil && existing.Id > 0 {
				return fmt.Errorf("connected app slug already exists")
			} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			if err := tx.Create(&approvedApp).Error; err != nil {
				return err
			}
			app = approvedApp
			current.AppId = approvedApp.Id
			current.Status = model.ConnectedAppRequestStatusApproved
			current.ReviewerUserId = c.GetInt("id")
			current.ReviewNote = strings.TrimSpace(req.ReviewNote)
			current.ReviewedAt = common.GetTimestamp()
		case model.ConnectedAppRequestStatusRejected:
			current.Status = model.ConnectedAppRequestStatusRejected
			current.ReviewerUserId = c.GetInt("id")
			current.ReviewNote = strings.TrimSpace(req.ReviewNote)
			current.ReviewedAt = common.GetTimestamp()
		default:
			return fmt.Errorf("decision must be approved or rejected")
		}
		if len(current.ReviewNote) > 1024 {
			return fmt.Errorf("review_note is too long")
		}
		if err := model.UpdateConnectedAppRequest(tx, &current); err != nil {
			return err
		}

		action := model.ConnectedAppAuditActionReject
		if current.Status == model.ConnectedAppRequestStatusApproved {
			action = model.ConnectedAppAuditActionApprove
		}
		after := map[string]any{"request": current}
		if app.Id > 0 {
			after["app"] = app
		}
		createdAudit, err := model.RecordConnectedAppAuditLogWithDB(tx, model.ConnectedAppAuditInput{
			ActorUserId: c.GetInt("id"),
			Action:      action,
			TargetType:  model.ConnectedAppAuditTargetRequest,
			TargetId:    current.Id,
			Before:      before,
			After:       after,
			RequestId:   c.GetHeader(common.RequestIdKey),
		})
		if err != nil {
			return err
		}
		reviewed = current
		audit = createdAudit
		return nil
	}); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := service.EnqueueConnectedAppRequestReviewOutboxWithDB(model.DB, reviewed, app, audit); err != nil {
		common.SysLog("failed to enqueue connected app review notification outbox: " + err.Error())
	}

	names := connectedAppUserNameMap(reviewed.ApplicantUserId, reviewed.ReviewerUserId, audit.ActorUserId)
	requestResponse, err := buildConnectedAppAccessRequestResponse(reviewed, names)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	response := gin.H{
		"request": requestResponse,
		"audit":   buildConnectedAppAuditLogResponse(audit, names),
	}
	if app.Id > 0 {
		appResponse, err := buildConnectedAppResponse(app)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		response["app"] = appResponse
	}
	common.ApiSuccess(c, response)
}

func ListConnectedAppAuditLogs(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	targetID := parsePositiveNotificationQuery(c.Query("target_id"))
	actorUserID := parsePositiveNotificationQuery(c.Query("actor_user_id"))
	logs, total, err := model.ListConnectedAppAuditLogs(
		c.Query("action"),
		c.Query("target_type"),
		targetID,
		actorUserID,
		c.Query("request_id"),
		pageInfo.GetStartIdx(),
		pageInfo.GetPageSize(),
	)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	userIDs := make([]int, 0, len(logs))
	for _, log := range logs {
		userIDs = append(userIDs, log.ActorUserId)
	}
	names := connectedAppUserNameMap(userIDs...)
	items := make([]connectedAppAuditLogResponse, 0, len(logs))
	for _, log := range logs {
		items = append(items, buildConnectedAppAuditLogResponse(log, names))
	}
	common.ApiSuccess(c, gin.H{
		"items":     items,
		"total":     total,
		"page":      pageInfo.GetPage(),
		"page_size": pageInfo.GetPageSize(),
	})
}

func ListConnectedAppRequestNotifications(c *gin.Context) {
	readKeys, err := model.ListEnterpriseNotificationReadKeys(c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	options := service.ConnectedAppRequestNotificationListOptions{
		Page:       parsePositiveNotificationQuery(c.Query("page")),
		Limit:      parsePositiveNotificationLimit(c),
		UnreadOnly: c.Query("unread_only") == "true" || c.Query("unread_only") == "1",
	}
	result, err := service.ListConnectedAppRequestNotifications(c.GetInt("id"), c.GetInt("role") >= common.RoleAdminUser, readKeys, options)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"items":        result.Items,
		"unread_count": result.UnreadCount,
		"page":         result.Page,
		"page_size":    result.PageSize,
		"has_more":     result.HasMore,
	})
}

func MarkConnectedAppRequestNotificationsRead(c *gin.Context) {
	var req MarkNotificationsReadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "无效的参数")
		return
	}
	inputKeys := append(req.EnterpriseNotificationKeys, req.ConnectedAppRequestKeys...)
	keys, err := model.MarkEnterpriseNotificationsRead(c.GetInt("id"), inputKeys)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"enterprise_notification_keys":     keys,
		"connected_app_request_keys":       keys,
		"connected_app_request_event_keys": keys,
	})
}

func buildConnectedAppAccessRequestForSubmit(req connectedAppAccessRequestPayload, userID int) (model.ConnectedAppRequest, error) {
	slug, err := normalizeConnectedAppSlug(req.Slug)
	if err != nil {
		return model.ConnectedAppRequest{}, err
	}
	name, description, requestedScopes, defaultScopes, err := normalizeConnectedAppFields(connectedAppRequest{
		Name:          req.Name,
		Description:   req.Description,
		AllowedScopes: req.RequestedScopes,
		DefaultScopes: req.DefaultScopes,
	})
	if err != nil {
		return model.ConnectedAppRequest{}, err
	}
	authorizationFlow, err := normalizeConnectedAppAuthorizationFlow(req.AuthorizationFlow)
	if err != nil {
		return model.ConnectedAppRequest{}, err
	}
	homepageURL, err := normalizeConnectedAppOptionalURL(req.HomepageURL, "homepage_url")
	if err != nil {
		return model.ConnectedAppRequest{}, err
	}
	callbackURL, err := normalizeConnectedAppOptionalURL(req.CallbackURL, "callback_url")
	if err != nil {
		return model.ConnectedAppRequest{}, err
	}
	reason := strings.TrimSpace(req.Reason)
	if len(reason) > 2048 {
		return model.ConnectedAppRequest{}, fmt.Errorf("reason is too long")
	}
	return model.ConnectedAppRequest{
		ApplicantUserId:   userID,
		Slug:              slug,
		Name:              name,
		Description:       description,
		RequestedScopes:   strings.Join(requestedScopes, " "),
		DefaultScopes:     strings.Join(defaultScopes, " "),
		AuthorizationFlow: authorizationFlow,
		HomepageURL:       homepageURL,
		CallbackURL:       callbackURL,
		Reason:            reason,
		Status:            model.ConnectedAppRequestStatusPending,
	}, nil
}

func buildConnectedAppFromReviewPayload(request model.ConnectedAppRequest, req connectedAppReviewPayload) (model.ConnectedApp, error) {
	name := request.Name
	if req.Name != nil {
		name = *req.Name
	}
	description := request.Description
	if req.Description != nil {
		description = *req.Description
	}
	allowedScopes := request.ScopeList()
	if len(req.AllowedScopes) > 0 {
		allowedScopes = req.AllowedScopes
	}
	defaultScopes := request.DefaultScopeList()
	if len(req.DefaultScopes) > 0 {
		defaultScopes = req.DefaultScopes
	}
	normalizedName, normalizedDescription, normalizedAllowedScopes, normalizedDefaultScopes, err := normalizeConnectedAppFields(connectedAppRequest{
		Name:          name,
		Description:   description,
		AllowedScopes: allowedScopes,
		DefaultScopes: defaultScopes,
	})
	if err != nil {
		return model.ConnectedApp{}, err
	}
	authorizationFlow := request.AuthorizationFlow
	if req.AuthorizationFlow != nil {
		authorizationFlow = *req.AuthorizationFlow
	}
	normalizedFlow, err := normalizeConnectedAppAuthorizationFlow(authorizationFlow)
	if err != nil {
		return model.ConnectedApp{}, err
	}
	if req.HomepageURL != nil {
		if _, err := normalizeConnectedAppOptionalURL(*req.HomepageURL, "homepage_url"); err != nil {
			return model.ConnectedApp{}, err
		}
	}
	if req.CallbackURL != nil {
		if _, err := normalizeConnectedAppOptionalURL(*req.CallbackURL, "callback_url"); err != nil {
			return model.ConnectedApp{}, err
		}
	}
	return model.ConnectedApp{
		Slug:              request.Slug,
		Name:              normalizedName,
		Description:       normalizedDescription,
		AllowedScopes:     strings.Join(normalizedAllowedScopes, " "),
		DefaultScopes:     strings.Join(normalizedDefaultScopes, " "),
		AuthorizationFlow: normalizedFlow,
		Trusted:           true,
		Status:            model.ConnectedAppStatusEnabled,
	}, nil
}

func ensureConnectedAppRequestSlugAvailable(slug string) error {
	if existing, err := model.GetConnectedAppBySlug(slug); err == nil && existing.Id > 0 {
		return fmt.Errorf("connected app slug already exists")
	} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	var pendingCount int64
	if err := model.DB.Model(&model.ConnectedAppRequest{}).
		Where("slug = ? AND status = ?", slug, model.ConnectedAppRequestStatusPending).
		Count(&pendingCount).Error; err != nil {
		return err
	}
	if pendingCount > 0 {
		return fmt.Errorf("connected app request for this slug is already pending")
	}
	return nil
}

func normalizeConnectedAppOptionalURL(raw string, fieldName string) (string, error) {
	normalized := strings.TrimSpace(raw)
	if normalized == "" {
		return "", nil
	}
	if len(normalized) > 512 {
		return "", fmt.Errorf("%s is too long", fieldName)
	}
	parsed, err := url.Parse(normalized)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return "", fmt.Errorf("%s must be an absolute http or https URL", fieldName)
	}
	return normalized, nil
}

func buildConnectedAppAccessRequestResponses(requests []model.ConnectedAppRequest) ([]connectedAppAccessRequestResponse, error) {
	userIDs := make([]int, 0, len(requests)*2)
	for _, request := range requests {
		userIDs = append(userIDs, request.ApplicantUserId, request.ReviewerUserId)
	}
	names := connectedAppUserNameMap(userIDs...)
	items := make([]connectedAppAccessRequestResponse, 0, len(requests))
	for _, request := range requests {
		item, err := buildConnectedAppAccessRequestResponse(request, names)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func buildConnectedAppAccessRequestResponse(request model.ConnectedAppRequest, names map[int]string) (connectedAppAccessRequestResponse, error) {
	return connectedAppAccessRequestResponse{
		ID:                request.Id,
		ApplicantUserID:   request.ApplicantUserId,
		ApplicantName:     names[request.ApplicantUserId],
		AppID:             request.AppId,
		Slug:              request.Slug,
		Name:              request.Name,
		Description:       request.Description,
		RequestedScopes:   request.ScopeList(),
		DefaultScopes:     request.DefaultScopeList(),
		AuthorizationFlow: connectedAppRequestAuthorizationFlow(request),
		HomepageURL:       request.HomepageURL,
		CallbackURL:       request.CallbackURL,
		Reason:            request.Reason,
		Status:            request.Status,
		ReviewerUserID:    request.ReviewerUserId,
		ReviewerName:      names[request.ReviewerUserId],
		ReviewNote:        request.ReviewNote,
		ReviewedAt:        request.ReviewedAt,
		CreatedAt:         request.CreatedAt,
		UpdatedAt:         request.UpdatedAt,
	}, nil
}

func connectedAppRequestAuthorizationFlow(request model.ConnectedAppRequest) string {
	if strings.TrimSpace(request.AuthorizationFlow) == "" {
		return model.ConnectedAppAuthorizationFlowDeviceCode
	}
	return request.AuthorizationFlow
}

func buildConnectedAppAuditLogResponse(log model.ConnectedAppAuditLog, names map[int]string) connectedAppAuditLogResponse {
	return connectedAppAuditLogResponse{
		ID:          log.Id,
		ActorUserID: log.ActorUserId,
		ActorName:   names[log.ActorUserId],
		Action:      log.Action,
		TargetType:  log.TargetType,
		TargetID:    log.TargetId,
		BeforeJSON:  log.BeforeJson,
		AfterJSON:   log.AfterJson,
		RequestID:   log.RequestId,
		CreatedAt:   log.CreatedAt,
	}
}

func connectedAppUserNameMap(userIDs ...int) map[int]string {
	seen := make(map[int]struct{}, len(userIDs))
	ids := make([]int, 0, len(userIDs))
	for _, id := range userIDs {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	names := make(map[int]string, len(ids))
	if len(ids) == 0 {
		return names
	}
	var users []model.User
	if err := model.DB.Select("id, username, display_name").Where("id IN ?", ids).Find(&users).Error; err != nil {
		return names
	}
	for _, user := range users {
		name := strings.TrimSpace(user.DisplayName)
		if name == "" {
			name = strings.TrimSpace(user.Username)
		}
		names[user.Id] = name
	}
	return names
}
