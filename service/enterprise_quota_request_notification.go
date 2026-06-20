package service

import (
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"gorm.io/gorm"
)

const EnterpriseQuotaRequestNotificationKind = "enterprise_quota_request"

const (
	EnterpriseQuotaRequestExpiringSoonWindow   = 24 * time.Hour
	EnterpriseQuotaRequestExpiryBatchSize      = 200
	EnterpriseQuotaRequestNotificationLimit    = 20
	EnterpriseQuotaRequestNotificationMaxLimit = 100
	EnterpriseQuotaRequestNotificationMaxScan  = 500
)

type EnterpriseQuotaRequestNotificationListOptions struct {
	Page             int
	Limit            int
	UnreadOnly       bool
	ReviewOrgUnitIds []int
}

type EnterpriseQuotaRequestNotification struct {
	Key            string            `json:"key"`
	Kind           string            `json:"kind"`
	Title          string            `json:"title"`
	Content        string            `json:"content"`
	TitleKey       string            `json:"title_key"`
	ContentKey     string            `json:"content_key"`
	ContentParams  map[string]string `json:"content_params"`
	Status         string            `json:"status"`
	Read           bool              `json:"read"`
	QuotaRequestId int               `json:"quota_request_id"`
	AuditLogId     int64             `json:"audit_log_id"`
	PolicyName     string            `json:"policy_name"`
	ApplicantName  string            `json:"applicant_name"`
	ActorName      string            `json:"actor_name"`
	LimitDelta     int64             `json:"limit_delta"`
	ExpiresAt      int64             `json:"expires_at"`
	CreatedAt      int64             `json:"created_at"`
}

type EnterpriseQuotaRequestNotificationList struct {
	Items       []EnterpriseQuotaRequestNotification `json:"items"`
	UnreadCount int                                  `json:"unread_count"`
	Page        int                                  `json:"page"`
	PageSize    int                                  `json:"page_size"`
	HasMore     bool                                 `json:"has_more"`
}

type quotaRequestNotificationRow struct {
	RequestId        int
	ApplicantUserId  int
	PolicyId         int
	LimitDelta       int64
	ExpiresAt        int64
	RequestCreatedAt int64
	RequestStatus    string
	PolicyName       string
	ApplicantName    string
	AuditLogId       int64
	Action           string
	ActorUserId      int
	AuditCreatedAt   int64
	ActorName        string
}

func ListEnterpriseQuotaRequestNotifications(enterpriseId int, userId int, isAdmin bool, readKeys []string, options EnterpriseQuotaRequestNotificationListOptions) (EnterpriseQuotaRequestNotificationList, error) {
	options = normalizeEnterpriseQuotaRequestNotificationListOptions(options)
	readSet := make(map[string]struct{}, len(readKeys))
	for _, key := range readKeys {
		readSet[key] = struct{}{}
	}

	rowLimit := EnterpriseQuotaRequestNotificationMaxScan
	rows, err := listQuotaRequestNotificationRows(enterpriseId, userId, isAdmin, options.ReviewOrgUnitIds, rowLimit)
	if err != nil {
		return EnterpriseQuotaRequestNotificationList{}, err
	}
	allItems := make([]EnterpriseQuotaRequestNotification, 0, len(rows))
	unreadCount := 0
	for _, row := range rows {
		item := quotaRequestNotificationFromRow(row, readSet)
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
	items := allItems[start:end]
	return EnterpriseQuotaRequestNotificationList{
		Items:       items,
		UnreadCount: unreadCount,
		Page:        options.Page,
		PageSize:    options.Limit,
		HasMore:     end < len(allItems),
	}, nil
}

func normalizeEnterpriseQuotaRequestNotificationListOptions(options EnterpriseQuotaRequestNotificationListOptions) EnterpriseQuotaRequestNotificationListOptions {
	if options.Page <= 0 {
		options.Page = 1
	}
	if options.Limit <= 0 {
		options.Limit = EnterpriseQuotaRequestNotificationLimit
	}
	if options.Limit > EnterpriseQuotaRequestNotificationMaxLimit {
		options.Limit = EnterpriseQuotaRequestNotificationMaxLimit
	}
	return options
}

func listQuotaRequestNotificationRows(enterpriseId int, userId int, isAdmin bool, reviewOrgUnitIds []int, limit int) ([]quotaRequestNotificationRow, error) {
	pendingRows, err := listPendingQuotaRequestNotificationRows(enterpriseId, isAdmin, reviewOrgUnitIds, limit)
	if err != nil {
		return nil, err
	}
	decisionRows, err := listDecisionQuotaRequestNotificationRows(enterpriseId, userId, limit)
	if err != nil {
		return nil, err
	}
	expiringRows, err := listExpiringSoonQuotaRequestNotificationRows(enterpriseId, userId, isAdmin, reviewOrgUnitIds, limit)
	if err != nil {
		return nil, err
	}
	rows := append(pendingRows, decisionRows...)
	rows = append(rows, expiringRows...)
	sortQuotaRequestNotificationRows(rows)
	if len(rows) > limit {
		rows = rows[:limit]
	}
	return rows, nil
}

func listPendingQuotaRequestNotificationRows(enterpriseId int, isAdmin bool, reviewOrgUnitIds []int, limit int) ([]quotaRequestNotificationRow, error) {
	if !isAdmin {
		return []quotaRequestNotificationRow{}, nil
	}
	var requests []model.EnterpriseQuotaRequest
	query := model.DB.
		Where("enterprise_id = ? AND status = ?", enterpriseId, model.EnterpriseQuotaRequestStatusPending).
		Order("created_at desc, id desc").
		Limit(limit)
	query = applyQuotaRequestNotificationReviewScope(query, enterpriseId, reviewOrgUnitIds)
	if err := query.Find(&requests).Error; err != nil {
		return nil, err
	}
	rows := make([]quotaRequestNotificationRow, 0, len(requests))
	for _, request := range requests {
		row := quotaRequestNotificationRow{
			RequestId:        request.Id,
			ApplicantUserId:  request.ApplicantUserId,
			PolicyId:         request.PolicyId,
			LimitDelta:       request.LimitDelta,
			ExpiresAt:        request.ExpiresAt,
			RequestCreatedAt: request.CreatedAt,
			RequestStatus:    request.Status,
			Action:           "quota_request.pending",
			AuditCreatedAt:   request.CreatedAt,
		}
		var audit model.EnterpriseAuditLog
		err := model.DB.
			Where("enterprise_id = ? AND target_type = ? AND target_id = ? AND action = ?", enterpriseId, "quota_request", request.Id, "quota_request.submit").
			Order("created_at desc, id desc").
			First(&audit).Error
		if err == nil {
			row.AuditLogId = audit.Id
			row.ActorUserId = audit.ActorUserId
			row.AuditCreatedAt = audit.CreatedAt
		} else if err != nil && err != gorm.ErrRecordNotFound {
			return nil, err
		}
		rows = append(rows, row)
	}
	fillQuotaRequestNotificationNames(enterpriseId, rows)
	return rows, nil
}

func listDecisionQuotaRequestNotificationRows(enterpriseId int, userId int, limit int) ([]quotaRequestNotificationRow, error) {
	actions := []string{
		"quota_request.approve",
		"quota_request.reject",
		"quota_request.withdraw",
		"quota_request.expire",
	}
	query := model.DB.
		Model(&model.EnterpriseAuditLog{}).
		Where("enterprise_id = ? AND target_type = ? AND action IN ?", enterpriseId, "quota_request", actions)
	query = query.Where("target_id IN (?)",
		model.DB.Model(&model.EnterpriseQuotaRequest{}).
			Select("id").
			Where("enterprise_id = ? AND applicant_user_id = ?", enterpriseId, userId),
	)
	var audits []model.EnterpriseAuditLog
	if err := query.Order("created_at desc, id desc").Limit(limit).Find(&audits).Error; err != nil {
		return nil, err
	}
	if len(audits) == 0 {
		return []quotaRequestNotificationRow{}, nil
	}
	requestIds := make([]int, 0, len(audits))
	for _, audit := range audits {
		requestIds = append(requestIds, audit.TargetId)
	}
	var requests []model.EnterpriseQuotaRequest
	if err := model.DB.Where("enterprise_id = ? AND id IN ?", enterpriseId, requestIds).Find(&requests).Error; err != nil {
		return nil, err
	}
	requestById := make(map[int]model.EnterpriseQuotaRequest, len(requests))
	for _, request := range requests {
		requestById[request.Id] = request
	}
	rows := make([]quotaRequestNotificationRow, 0, len(audits))
	for _, audit := range audits {
		request, ok := requestById[audit.TargetId]
		if !ok {
			continue
		}
		rows = append(rows, quotaRequestNotificationRow{
			RequestId:        request.Id,
			ApplicantUserId:  request.ApplicantUserId,
			PolicyId:         request.PolicyId,
			LimitDelta:       request.LimitDelta,
			ExpiresAt:        request.ExpiresAt,
			RequestCreatedAt: request.CreatedAt,
			RequestStatus:    request.Status,
			AuditLogId:       audit.Id,
			Action:           audit.Action,
			ActorUserId:      audit.ActorUserId,
			AuditCreatedAt:   audit.CreatedAt,
		})
	}
	fillQuotaRequestNotificationNames(enterpriseId, rows)
	return rows, nil
}

func listExpiringSoonQuotaRequestNotificationRows(enterpriseId int, userId int, isAdmin bool, reviewOrgUnitIds []int, limit int) ([]quotaRequestNotificationRow, error) {
	now := common.GetTimestamp()
	windowEnd := now + int64(EnterpriseQuotaRequestExpiringSoonWindow/time.Second)
	query := model.DB.
		Where("enterprise_id = ? AND status = ?", enterpriseId, model.EnterpriseQuotaRequestStatusApproved).
		Where("effective_at <= ? AND expires_at > ? AND expires_at <= ?", now, now, windowEnd)
	if !isAdmin {
		query = query.Where("applicant_user_id = ?", userId)
	} else {
		query = applyQuotaRequestNotificationReviewScope(query, enterpriseId, reviewOrgUnitIds)
	}
	var requests []model.EnterpriseQuotaRequest
	if err := query.Order("expires_at asc, id desc").Limit(limit).Find(&requests).Error; err != nil {
		return nil, err
	}
	rows := make([]quotaRequestNotificationRow, 0, len(requests))
	for _, request := range requests {
		rows = append(rows, quotaRequestNotificationRow{
			RequestId:        request.Id,
			ApplicantUserId:  request.ApplicantUserId,
			PolicyId:         request.PolicyId,
			LimitDelta:       request.LimitDelta,
			ExpiresAt:        request.ExpiresAt,
			RequestCreatedAt: request.CreatedAt,
			RequestStatus:    request.Status,
			Action:           "quota_request.expiring_soon",
			ActorUserId:      request.ApproverUserId,
			AuditCreatedAt:   request.ExpiresAt,
		})
	}
	fillQuotaRequestNotificationNames(enterpriseId, rows)
	return rows, nil
}

func applyQuotaRequestNotificationReviewScope(query *gorm.DB, enterpriseId int, reviewOrgUnitIds []int) *gorm.DB {
	if len(reviewOrgUnitIds) == 0 {
		return query
	}
	scopedUserIds := model.DB.Model(&model.EnterpriseOrgMembership{}).
		Select("user_id").
		Where("enterprise_id = ? AND is_primary = ? AND org_unit_id IN ?", enterpriseId, true, reviewOrgUnitIds)
	return query.Where("applicant_user_id IN (?)", scopedUserIds)
}

func fillQuotaRequestNotificationNames(enterpriseId int, rows []quotaRequestNotificationRow) {
	if len(rows) == 0 {
		return
	}
	policyIds := make([]int, 0, len(rows))
	userIds := make([]int, 0, len(rows)*2)
	seenPolicyIds := map[int]struct{}{}
	seenUserIds := map[int]struct{}{}
	for _, row := range rows {
		if row.PolicyId > 0 {
			if _, ok := seenPolicyIds[row.PolicyId]; !ok {
				seenPolicyIds[row.PolicyId] = struct{}{}
				policyIds = append(policyIds, row.PolicyId)
			}
		}
		for _, userId := range []int{row.ApplicantUserId, row.ActorUserId} {
			if userId <= 0 {
				continue
			}
			if _, ok := seenUserIds[userId]; ok {
				continue
			}
			seenUserIds[userId] = struct{}{}
			userIds = append(userIds, userId)
		}
	}
	policyNames := map[int]string{}
	if len(policyIds) > 0 {
		var policies []model.EnterpriseQuotaPolicy
		if err := model.DB.Where("enterprise_id = ? AND id IN ?", enterpriseId, policyIds).Find(&policies).Error; err == nil {
			for _, policy := range policies {
				policyNames[policy.Id] = policy.Name
			}
		}
	}
	userNames := map[int]string{}
	if len(userIds) > 0 {
		var users []model.User
		if err := model.DB.Select("id, username, display_name").Where("id IN ?", userIds).Find(&users).Error; err == nil {
			for _, user := range users {
				name := strings.TrimSpace(user.DisplayName)
				if name == "" {
					name = user.Username
				}
				userNames[user.Id] = name
			}
		}
	}
	for index := range rows {
		rows[index].PolicyName = policyNames[rows[index].PolicyId]
		rows[index].ApplicantName = userNames[rows[index].ApplicantUserId]
		rows[index].ActorName = userNames[rows[index].ActorUserId]
	}
}

func quotaRequestNotificationFromRow(row quotaRequestNotificationRow, readSet map[string]struct{}) EnterpriseQuotaRequestNotification {
	key := quotaRequestNotificationKey(row)
	status := quotaRequestNotificationStatus(row)
	title := quotaRequestNotificationTitle(status)
	actorName := row.ActorName
	if actorName == "" && row.ActorUserId > 0 {
		actorName = "User #" + strconv.Itoa(row.ActorUserId)
	}
	applicantName := row.ApplicantName
	if applicantName == "" && row.ApplicantUserId > 0 {
		applicantName = "User #" + strconv.Itoa(row.ApplicantUserId)
	}
	policyName := row.PolicyName
	if policyName == "" && row.PolicyId > 0 {
		policyName = "Policy #" + strconv.Itoa(row.PolicyId)
	}
	content := quotaRequestNotificationContent(status, applicantName, actorName, policyName)
	titleKey := quotaRequestNotificationTitleKey(status)
	contentKey := quotaRequestNotificationContentKey(status, actorName)
	contentParams := map[string]string{
		"applicantName": applicantName,
		"actorName":     actorName,
		"policyName":    policyName,
	}
	_, read := readSet[key]
	return EnterpriseQuotaRequestNotification{
		Key:            key,
		Kind:           EnterpriseQuotaRequestNotificationKind,
		Title:          title,
		Content:        content,
		TitleKey:       titleKey,
		ContentKey:     contentKey,
		ContentParams:  contentParams,
		Status:         status,
		Read:           read,
		QuotaRequestId: row.RequestId,
		AuditLogId:     row.AuditLogId,
		PolicyName:     policyName,
		ApplicantName:  applicantName,
		ActorName:      actorName,
		LimitDelta:     row.LimitDelta,
		ExpiresAt:      row.ExpiresAt,
		CreatedAt:      row.AuditCreatedAt,
	}
}

func quotaRequestNotificationKey(row quotaRequestNotificationRow) string {
	if row.Action == "quota_request.pending" {
		return "quota_request:pending:" + strconv.Itoa(row.RequestId)
	}
	if row.Action == "quota_request.expiring_soon" {
		return "quota_request:expiring_soon:" + strconv.Itoa(row.RequestId) + ":" + strconv.FormatInt(row.ExpiresAt, 10)
	}
	if row.AuditLogId > 0 {
		return "quota_request:audit:" + strconv.FormatInt(row.AuditLogId, 10)
	}
	return "quota_request:" + row.Action + ":" + strconv.Itoa(row.RequestId)
}

func quotaRequestNotificationStatus(row quotaRequestNotificationRow) string {
	switch row.Action {
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
	case "quota_request.pending":
		return model.EnterpriseQuotaRequestStatusPending
	default:
		return row.RequestStatus
	}
}

func quotaRequestNotificationTitle(status string) string {
	switch status {
	case model.EnterpriseQuotaRequestStatusPending:
		return "Quota request pending"
	case model.EnterpriseQuotaRequestStatusApproved:
		return "Quota request approved"
	case model.EnterpriseQuotaRequestStatusRejected:
		return "Quota request rejected"
	case model.EnterpriseQuotaRequestStatusWithdrawn:
		return "Quota request withdrawn"
	case model.EnterpriseQuotaRequestStatusExpired:
		return "Quota request expired"
	case "expiring_soon":
		return "Quota request expiring soon"
	default:
		return "Quota request updated"
	}
}

func quotaRequestNotificationTitleKey(status string) string {
	switch status {
	case model.EnterpriseQuotaRequestStatusPending:
		return "notification.enterprise_quota_request.title.pending"
	case model.EnterpriseQuotaRequestStatusApproved:
		return "notification.enterprise_quota_request.title.approved"
	case model.EnterpriseQuotaRequestStatusRejected:
		return "notification.enterprise_quota_request.title.rejected"
	case model.EnterpriseQuotaRequestStatusWithdrawn:
		return "notification.enterprise_quota_request.title.withdrawn"
	case model.EnterpriseQuotaRequestStatusExpired:
		return "notification.enterprise_quota_request.title.expired"
	case "expiring_soon":
		return "notification.enterprise_quota_request.title.expiring_soon"
	default:
		return "notification.enterprise_quota_request.title.updated"
	}
}

func quotaRequestNotificationContentKey(status string, actorName string) string {
	switch status {
	case model.EnterpriseQuotaRequestStatusPending:
		return "notification.enterprise_quota_request.content.pending"
	case model.EnterpriseQuotaRequestStatusApproved:
		return "notification.enterprise_quota_request.content.approved"
	case model.EnterpriseQuotaRequestStatusRejected:
		return "notification.enterprise_quota_request.content.rejected"
	case model.EnterpriseQuotaRequestStatusWithdrawn:
		if actorName != "" {
			return "notification.enterprise_quota_request.content.withdrawn_by_actor"
		}
		return "notification.enterprise_quota_request.content.withdrawn"
	case model.EnterpriseQuotaRequestStatusExpired:
		return "notification.enterprise_quota_request.content.expired"
	case "expiring_soon":
		return "notification.enterprise_quota_request.content.expiring_soon"
	default:
		return "notification.enterprise_quota_request.content.updated"
	}
}

func quotaRequestNotificationContent(status string, applicantName string, actorName string, policyName string) string {
	switch status {
	case model.EnterpriseQuotaRequestStatusPending:
		return strings.TrimSpace(applicantName + " submitted a temporary quota request for " + policyName + ".")
	case model.EnterpriseQuotaRequestStatusApproved:
		return strings.TrimSpace(actorName + " approved your temporary quota request for " + policyName + ".")
	case model.EnterpriseQuotaRequestStatusRejected:
		return strings.TrimSpace(actorName + " rejected your temporary quota request for " + policyName + ".")
	case model.EnterpriseQuotaRequestStatusWithdrawn:
		if actorName != "" {
			return strings.TrimSpace(actorName + " withdrew a temporary quota request for " + policyName + ".")
		}
		return strings.TrimSpace("A temporary quota request for " + policyName + " was withdrawn.")
	case model.EnterpriseQuotaRequestStatusExpired:
		return strings.TrimSpace("A temporary quota request for " + policyName + " expired before approval.")
	case "expiring_soon":
		return strings.TrimSpace("A temporary quota approval for " + policyName + " is expiring soon.")
	default:
		return strings.TrimSpace("A temporary quota request for " + policyName + " was updated.")
	}
}

func ExpireDueEnterpriseQuotaRequests(now int64, batchSize int) (int, error) {
	if now <= 0 {
		now = common.GetTimestamp()
	}
	if batchSize <= 0 {
		batchSize = EnterpriseQuotaRequestExpiryBatchSize
	}
	var requests []model.EnterpriseQuotaRequest
	if err := model.DB.
		Where("status IN ? AND expires_at > 0 AND expires_at <= ?", []string{model.EnterpriseQuotaRequestStatusPending, model.EnterpriseQuotaRequestStatusApproved}, now).
		Order("expires_at asc, id asc").
		Limit(batchSize).
		Find(&requests).Error; err != nil {
		return 0, err
	}
	expired := 0
	for _, request := range requests {
		updated, err := expireEnterpriseQuotaRequest(request, now)
		if err != nil {
			return expired, err
		}
		if updated {
			expired++
		}
	}
	return expired, nil
}

func expireEnterpriseQuotaRequest(request model.EnterpriseQuotaRequest, now int64) (bool, error) {
	before := request
	updated := false
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&model.EnterpriseQuotaRequest{}).
			Where("id = ? AND status IN ? AND expires_at > 0 AND expires_at <= ?", request.Id, []string{model.EnterpriseQuotaRequestStatusPending, model.EnterpriseQuotaRequestStatusApproved}, now).
			Updates(map[string]any{
				"status":     model.EnterpriseQuotaRequestStatusExpired,
				"decided_at": now,
				"updated_at": now,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return nil
		}
		request.Status = model.EnterpriseQuotaRequestStatusExpired
		request.DecidedAt = now
		request.UpdatedAt = now
		audit, err := recordEnterpriseQuotaRequestExpiryAudit(tx, before, request)
		if err != nil {
			return err
		}
		if err := EnqueueEnterpriseQuotaRequestOutboxWithDB(tx, request, audit, "quota_request.expire"); err != nil {
			return err
		}
		updated = true
		return nil
	})
	return updated, err
}

func recordEnterpriseQuotaRequestExpiryAudit(tx *gorm.DB, before model.EnterpriseQuotaRequest, after model.EnterpriseQuotaRequest) (model.EnterpriseAuditLog, error) {
	return model.RecordEnterpriseAuditLogWithDB(tx, model.EnterpriseAuditInput{
		EnterpriseId: before.EnterpriseId,
		ActorUserId:  0,
		Action:       "quota_request.expire",
		TargetType:   "quota_request",
		TargetId:     before.Id,
		Before:       before,
		After:        after,
	})
}

func sortQuotaRequestNotificationRows(rows []quotaRequestNotificationRow) {
	for i := 0; i < len(rows)-1; i++ {
		for j := i + 1; j < len(rows); j++ {
			if rows[j].AuditCreatedAt > rows[i].AuditCreatedAt ||
				(rows[j].AuditCreatedAt == rows[i].AuditCreatedAt && rows[j].AuditLogId > rows[i].AuditLogId) {
				rows[i], rows[j] = rows[j], rows[i]
			}
		}
	}
}
