package service

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

const ConnectedAppRequestNotificationKind = "connected_app_request"

const (
	ConnectedAppRequestNotificationLimit    = 20
	ConnectedAppRequestNotificationMaxLimit = 100
	ConnectedAppRequestNotificationMaxScan  = 500
)

type ConnectedAppRequestNotificationListOptions struct {
	Page       int
	Limit      int
	UnreadOnly bool
}

type ConnectedAppRequestNotification struct {
	Key             string            `json:"key"`
	Kind            string            `json:"kind"`
	Title           string            `json:"title"`
	Content         string            `json:"content"`
	TitleKey        string            `json:"title_key"`
	ContentKey      string            `json:"content_key"`
	ContentParams   map[string]string `json:"content_params"`
	Status          string            `json:"status"`
	Read            bool              `json:"read"`
	RequestId       int               `json:"request_id"`
	AppId           int               `json:"app_id"`
	AuditLogId      int64             `json:"audit_log_id"`
	Slug            string            `json:"slug"`
	AppName         string            `json:"app_name"`
	ApplicantName   string            `json:"applicant_name"`
	ActorName       string            `json:"actor_name"`
	RequestedScopes []string          `json:"requested_scopes"`
	CreatedAt       int64             `json:"created_at"`
}

type ConnectedAppRequestNotificationList struct {
	Items       []ConnectedAppRequestNotification `json:"items"`
	UnreadCount int                               `json:"unread_count"`
	Page        int                               `json:"page"`
	PageSize    int                               `json:"page_size"`
	HasMore     bool                              `json:"has_more"`
}

type connectedAppRequestNotificationRow struct {
	Request        model.ConnectedAppRequest
	AuditLogId     int64
	Action         string
	ActorUserId    int
	AuditCreatedAt int64
}

func ListConnectedAppRequestNotifications(userId int, isAdmin bool, readKeys []string, options ConnectedAppRequestNotificationListOptions) (ConnectedAppRequestNotificationList, error) {
	options = normalizeConnectedAppRequestNotificationListOptions(options)
	readSet := make(map[string]struct{}, len(readKeys))
	for _, key := range readKeys {
		readSet[key] = struct{}{}
	}

	rows, err := listConnectedAppRequestNotificationRows(userId, isAdmin, ConnectedAppRequestNotificationMaxScan)
	if err != nil {
		return ConnectedAppRequestNotificationList{}, err
	}
	names, err := connectedAppRequestNotificationUserNames(rows)
	if err != nil {
		return ConnectedAppRequestNotificationList{}, err
	}
	allItems := make([]ConnectedAppRequestNotification, 0, len(rows))
	unreadCount := 0
	for _, row := range rows {
		item := connectedAppRequestNotificationFromRow(row, names, readSet)
		if !item.Read {
			unreadCount++
		}
		if options.UnreadOnly && item.Read {
			continue
		}
		allItems = append(allItems, item)
	}
	start := (options.Page - 1) * options.Limit
	if start > len(allItems) {
		start = len(allItems)
	}
	end := start + options.Limit
	if end > len(allItems) {
		end = len(allItems)
	}
	return ConnectedAppRequestNotificationList{
		Items:       allItems[start:end],
		UnreadCount: unreadCount,
		Page:        options.Page,
		PageSize:    options.Limit,
		HasMore:     end < len(allItems),
	}, nil
}

func normalizeConnectedAppRequestNotificationListOptions(options ConnectedAppRequestNotificationListOptions) ConnectedAppRequestNotificationListOptions {
	if options.Page <= 0 {
		options.Page = 1
	}
	if options.Limit <= 0 {
		options.Limit = ConnectedAppRequestNotificationLimit
	}
	if options.Limit > ConnectedAppRequestNotificationMaxLimit {
		options.Limit = ConnectedAppRequestNotificationMaxLimit
	}
	return options
}

func listConnectedAppRequestNotificationRows(userId int, isAdmin bool, limit int) ([]connectedAppRequestNotificationRow, error) {
	pendingRows, err := listPendingConnectedAppRequestNotificationRows(isAdmin, limit)
	if err != nil {
		return nil, err
	}
	decisionRows, err := listDecisionConnectedAppRequestNotificationRows(userId, limit)
	if err != nil {
		return nil, err
	}
	rows := append(pendingRows, decisionRows...)
	sort.SliceStable(rows, func(i, j int) bool {
		left := rows[i].AuditCreatedAt
		if left == 0 {
			left = rows[i].Request.CreatedAt
		}
		right := rows[j].AuditCreatedAt
		if right == 0 {
			right = rows[j].Request.CreatedAt
		}
		if left == right {
			return rows[i].Request.Id > rows[j].Request.Id
		}
		return left > right
	})
	if len(rows) > limit {
		rows = rows[:limit]
	}
	return rows, nil
}

func listPendingConnectedAppRequestNotificationRows(isAdmin bool, limit int) ([]connectedAppRequestNotificationRow, error) {
	if !isAdmin {
		return []connectedAppRequestNotificationRow{}, nil
	}
	var requests []model.ConnectedAppRequest
	if err := model.DB.
		Where("status = ?", model.ConnectedAppRequestStatusPending).
		Order("created_at desc, id desc").
		Limit(limit).
		Find(&requests).Error; err != nil {
		return nil, err
	}
	rows := make([]connectedAppRequestNotificationRow, 0, len(requests))
	for _, request := range requests {
		row := connectedAppRequestNotificationRow{
			Request:        request,
			Action:         "connected_app_request.pending",
			ActorUserId:    request.ApplicantUserId,
			AuditCreatedAt: request.CreatedAt,
		}
		var audit model.ConnectedAppAuditLog
		err := model.DB.
			Where("target_type = ? AND target_id = ? AND action = ?", model.ConnectedAppAuditTargetRequest, request.Id, model.ConnectedAppAuditActionSubmit).
			Order("created_at desc, id desc").
			First(&audit).Error
		if err == nil {
			row.AuditLogId = audit.Id
			row.ActorUserId = audit.ActorUserId
			row.AuditCreatedAt = audit.CreatedAt
		} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		} else {
			// Keep pending notifications visible even if the submit audit row is absent.
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func listDecisionConnectedAppRequestNotificationRows(userId int, limit int) ([]connectedAppRequestNotificationRow, error) {
	if userId <= 0 {
		return []connectedAppRequestNotificationRow{}, nil
	}
	actions := []string{
		model.ConnectedAppAuditActionApprove,
		model.ConnectedAppAuditActionReject,
	}
	query := model.DB.
		Model(&model.ConnectedAppAuditLog{}).
		Where("target_type = ? AND action IN ?", model.ConnectedAppAuditTargetRequest, actions).
		Where("target_id IN (?)",
			model.DB.Model(&model.ConnectedAppRequest{}).
				Select("id").
				Where("applicant_user_id = ?", userId),
		)
	var audits []model.ConnectedAppAuditLog
	if err := query.Order("created_at desc, id desc").Limit(limit).Find(&audits).Error; err != nil {
		return nil, err
	}
	if len(audits) == 0 {
		return []connectedAppRequestNotificationRow{}, nil
	}
	requestIds := make([]int, 0, len(audits))
	for _, audit := range audits {
		requestIds = append(requestIds, audit.TargetId)
	}
	var requests []model.ConnectedAppRequest
	if err := model.DB.Where("id IN ?", requestIds).Find(&requests).Error; err != nil {
		return nil, err
	}
	requestById := make(map[int]model.ConnectedAppRequest, len(requests))
	for _, request := range requests {
		requestById[request.Id] = request
	}
	rows := make([]connectedAppRequestNotificationRow, 0, len(audits))
	for _, audit := range audits {
		request, ok := requestById[audit.TargetId]
		if !ok {
			continue
		}
		rows = append(rows, connectedAppRequestNotificationRow{
			Request:        request,
			AuditLogId:     audit.Id,
			Action:         audit.Action,
			ActorUserId:    audit.ActorUserId,
			AuditCreatedAt: audit.CreatedAt,
		})
	}
	return rows, nil
}

func connectedAppRequestNotificationFromRow(row connectedAppRequestNotificationRow, names map[int]string, readSet map[string]struct{}) ConnectedAppRequestNotification {
	status := connectedAppRequestNotificationStatus(row)
	key := connectedAppRequestNotificationKey(status, row.Request.Id, row.AuditLogId)
	_, read := readSet[key]
	titleKey := "notification.connected_app_request.title." + status
	contentKey := "notification.connected_app_request.content." + status
	item := ConnectedAppRequestNotification{
		Key:        key,
		Kind:       ConnectedAppRequestNotificationKind,
		Title:      connectedAppRequestNotificationTitle(status),
		Content:    connectedAppRequestNotificationContent(status, row.Request.Name),
		TitleKey:   titleKey,
		ContentKey: contentKey,
		ContentParams: map[string]string{
			"appName":       row.Request.Name,
			"slug":          row.Request.Slug,
			"applicantName": names[row.Request.ApplicantUserId],
			"actorName":     names[row.ActorUserId],
		},
		Status:          status,
		Read:            read,
		RequestId:       row.Request.Id,
		AppId:           row.Request.AppId,
		AuditLogId:      row.AuditLogId,
		Slug:            row.Request.Slug,
		AppName:         row.Request.Name,
		ApplicantName:   names[row.Request.ApplicantUserId],
		ActorName:       names[row.ActorUserId],
		RequestedScopes: row.Request.ScopeList(),
		CreatedAt:       row.AuditCreatedAt,
	}
	if item.CreatedAt == 0 {
		item.CreatedAt = row.Request.CreatedAt
	}
	return item
}

func connectedAppRequestNotificationStatus(row connectedAppRequestNotificationRow) string {
	switch row.Action {
	case model.ConnectedAppAuditActionApprove:
		return model.ConnectedAppRequestStatusApproved
	case model.ConnectedAppAuditActionReject:
		return model.ConnectedAppRequestStatusRejected
	default:
		return model.ConnectedAppRequestStatusPending
	}
}

func connectedAppRequestNotificationKey(status string, requestId int, auditLogId int64) string {
	if auditLogId > 0 {
		return fmt.Sprintf("%s:%s:%d:%d", ConnectedAppRequestNotificationKind, status, requestId, auditLogId)
	}
	return fmt.Sprintf("%s:%s:%d", ConnectedAppRequestNotificationKind, status, requestId)
}

func connectedAppRequestNotificationTitle(status string) string {
	switch status {
	case model.ConnectedAppRequestStatusApproved:
		return "Connected app request approved"
	case model.ConnectedAppRequestStatusRejected:
		return "Connected app request rejected"
	default:
		return "Connected app request pending"
	}
}

func connectedAppRequestNotificationContent(status string, appName string) string {
	switch status {
	case model.ConnectedAppRequestStatusApproved:
		return "Your connected app request for " + appName + " was approved."
	case model.ConnectedAppRequestStatusRejected:
		return "Your connected app request for " + appName + " was rejected."
	default:
		return "A connected app request for " + appName + " is waiting for review."
	}
}

func connectedAppRequestNotificationUserNames(rows []connectedAppRequestNotificationRow) (map[int]string, error) {
	ids := make([]int, 0, len(rows)*2)
	seen := map[int]struct{}{}
	for _, row := range rows {
		for _, id := range []int{row.Request.ApplicantUserId, row.ActorUserId} {
			if id <= 0 {
				continue
			}
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			ids = append(ids, id)
		}
	}
	names := make(map[int]string, len(ids))
	if len(ids) == 0 {
		return names, nil
	}
	var users []model.User
	if err := model.DB.Select("id, username, display_name").Where("id IN ?", ids).Find(&users).Error; err != nil {
		return nil, err
	}
	for _, user := range users {
		name := strings.TrimSpace(user.DisplayName)
		if name == "" {
			name = strings.TrimSpace(user.Username)
		}
		if name == "" {
			name = "User " + strconv.Itoa(user.Id)
		}
		names[user.Id] = name
	}
	return names, nil
}
