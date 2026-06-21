package controller

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type enterpriseUpdateRequest struct {
	Name     string `json:"name"`
	Timezone string `json:"timezone"`
	Status   int    `json:"status"`
}

type enterpriseOrgUnitRequest struct {
	ParentId    int    `json:"parent_id"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
	Status      int    `json:"status"`
	Sort        int    `json:"sort"`
}

type enterpriseMemberOrgUnitRequest struct {
	OrgUnitId int `json:"org_unit_id"`
}

type enterprisePolicyGroupRequest struct {
	Name             string `json:"name"`
	Slug             string `json:"slug"`
	Description      string `json:"description"`
	OrgUnitId        *int   `json:"org_unit_id"`
	SharedOrgUnitIds []int  `json:"shared_org_unit_ids"`
	Status           int    `json:"status"`
}

type enterprisePolicyGroupMembersRequest struct {
	UserIds []int  `json:"user_ids"`
	Role    string `json:"role"`
}

type enterpriseProjectRequest struct {
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
	OwnerUserId int    `json:"owner_user_id"`
	OrgUnitIds  []int  `json:"org_unit_ids"`
	Status      int    `json:"status"`
}

type enterpriseProjectMemberRequest struct {
	UserId int    `json:"user_id"`
	Role   string `json:"role"`
}

type enterpriseQuotaPolicyRequest struct {
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	TargetType    string   `json:"target_type"`
	TargetId      int      `json:"target_id"`
	Metric        string   `json:"metric"`
	Period        string   `json:"period"`
	LimitValue    int64    `json:"limit_value"`
	Timezone      string   `json:"timezone"`
	ModelScope    string   `json:"model_scope"`
	Models        []string `json:"models"`
	ConditionMode string   `json:"condition_mode"`
	ConditionJson string   `json:"condition_json"`
	ConditionExpr string   `json:"condition_expr"`
	Action        string   `json:"action"`
	Priority      int      `json:"priority"`
	Status        int      `json:"status"`
	EffectiveAt   int64    `json:"effective_at"`
	ExpiresAt     int64    `json:"expires_at"`
}

type enterpriseQuotaRequestSubmitRequest struct {
	PolicyId   int    `json:"policy_id"`
	ProjectId  int    `json:"project_id"`
	LimitDelta int64  `json:"limit_delta"`
	Reason     string `json:"reason"`
	ExpiresAt  int64  `json:"expires_at"`
}

type enterpriseQuotaRequestDecisionRequest struct {
	DecisionReason string `json:"decision_reason"`
	ExpiresAt      int64  `json:"expires_at"`
}

type enterpriseWebhookRequest struct {
	Name       string   `json:"name"`
	Url        string   `json:"url"`
	Secret     *string  `json:"secret"`
	EventTypes []string `json:"event_types"`
	Status     int      `json:"status"`
}

type enterpriseNotificationPreferenceRequest struct {
	Channel        string                                       `json:"channel"`
	EventType      string                                       `json:"event_type"`
	Enabled        bool                                         `json:"enabled"`
	RecipientScope service.EnterpriseNotificationRecipientScope `json:"recipient_scope"`
}

type enterpriseQuotaCounterReconciliationRequest struct {
	Limit               int  `json:"limit"`
	Repair              bool `json:"repair"`
	IncludeRedisOrphans bool `json:"include_redis_orphans"`
}

type enterpriseOrgSyncRequest = service.EnterpriseOrgSyncInput

type enterpriseQuotaRequestItem struct {
	model.EnterpriseQuotaRequest
	PolicyName    string `json:"policy_name"`
	TargetName    string `json:"target_name"`
	ApplicantName string `json:"applicant_name"`
	ApproverName  string `json:"approver_name"`
}

type enterpriseMemberItem struct {
	UserId           int    `json:"user_id"`
	Username         string `json:"username"`
	DisplayName      string `json:"display_name"`
	Email            string `json:"email"`
	Status           int    `json:"status"`
	OrgUnitId        int    `json:"org_unit_id"`
	OrgUnitName      string `json:"org_unit_name"`
	Role             string `json:"role,omitempty"`
	PolicyGroupCount int64  `json:"policy_group_count"`
}

type enterprisePolicyGroupItem struct {
	model.EnterprisePolicyGroup
	SharedOrgUnitIds   []int    `json:"shared_org_unit_ids"`
	SharedOrgUnitNames []string `json:"shared_org_unit_names"`
	CanManage          bool     `json:"can_manage"`
	MemberCount        int64    `json:"member_count"`
	PolicyCount        int64    `json:"policy_count"`
}

type enterpriseProjectItem struct {
	model.EnterpriseProject
	OwnerName    string   `json:"owner_name"`
	OrgUnitIds   []int    `json:"org_unit_ids"`
	OrgUnitNames []string `json:"org_unit_names"`
	MemberRole   string   `json:"member_role,omitempty"`
	CanManage    bool     `json:"can_manage"`
	MemberCount  int64    `json:"member_count"`
	PolicyCount  int64    `json:"policy_count"`
}

type enterpriseQuotaPolicyItem struct {
	model.EnterpriseQuotaPolicy
	TargetName string `json:"target_name"`
	UsedValue  int64  `json:"used_value"`
}

type enterpriseUsageQuery struct {
	StartTime     int64
	EndTime       int64
	UserId        int
	OrgUnitId     int
	OrgUnitIds    []int
	ProjectId     int
	ProjectIds    []int
	PolicyGroupId int
	ChannelId     int
	TokenId       int
	ModelName     string
	Status        string
	Granularity   string
}

type enterpriseUsageRow struct {
	UserId             int
	TokenId            int
	OrgUnitId          int
	ProjectId          int
	PolicyGroupIdsJson string
	ModelName          string
	ChannelId          int
	PromptTokens       int
	CompletionTokens   int
	TotalTokens        int
	Quota              int
	Status             string
	CreatedAt          int64
}

type enterpriseUsageTotal struct {
	RequestCount     int64 `json:"request_count"`
	Quota            int64 `json:"quota"`
	PromptTokens     int64 `json:"prompt_tokens"`
	CompletionTokens int64 `json:"completion_tokens"`
	TotalTokens      int64 `json:"total_tokens"`
}

type enterpriseUsageSummary struct {
	StartTime int64                          `json:"start_time"`
	EndTime   int64                          `json:"end_time"`
	Total     enterpriseUsageTotal           `json:"total"`
	ByModel   []enterpriseUsageBreakdownItem `json:"by_model"`
	ByStatus  []enterpriseUsageBreakdownItem `json:"by_status"`
}

type enterpriseUsageBreakdownItem struct {
	Dimension        string `json:"dimension"`
	TargetId         int    `json:"target_id"`
	TargetName       string `json:"target_name"`
	ModelName        string `json:"model_name,omitempty"`
	Status           string `json:"status,omitempty"`
	TimeBucket       string `json:"time_bucket,omitempty"`
	RequestCount     int64  `json:"request_count"`
	Quota            int64  `json:"quota"`
	PromptTokens     int64  `json:"prompt_tokens"`
	CompletionTokens int64  `json:"completion_tokens"`
	TotalTokens      int64  `json:"total_tokens"`
}

func GetCurrentEnterprise(c *gin.Context) {
	enterprise, err := currentEnterprise()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, enterprise)
}

func UpdateCurrentEnterprise(c *gin.Context) {
	enterprise, err := currentEnterprise()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	before := *enterprise

	var req enterpriseUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		common.ApiErrorMsg(c, "企业名称不能为空")
		return
	}
	enterprise.Name = strings.TrimSpace(req.Name)
	enterprise.Timezone = normalizeEnterpriseTimezone(req.Timezone)
	if req.Status != 0 {
		enterprise.Status = req.Status
	}
	if err := model.DB.Save(enterprise).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	recordEnterpriseAudit(c, enterprise.Id, "enterprise.update", "enterprise", enterprise.Id, before, *enterprise)
	common.ApiSuccess(c, gin.H{"id": enterprise.Id})
}

func ListEnterpriseOrgUnits(c *gin.Context) {
	enterprise, err := currentEnterprise()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	access, err := enterpriseAccessForRequest(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	query := model.DB.Model(&model.EnterpriseOrgUnit{}).Where("enterprise_id = ?", enterprise.Id)
	query = applyDepartmentOrgUnitScope(query, access, "id")
	if parentId, err := parseOptionalIntQuery(c, "parent_id"); err != nil {
		common.ApiError(c, err)
		return
	} else if parentId >= 0 && c.Query("parent_id") != "" {
		query = query.Where("parent_id = ?", parentId)
	}
	if status, err := parseOptionalIntQuery(c, "status"); err != nil {
		common.ApiError(c, err)
		return
	} else if status > 0 {
		query = query.Where("status = ?", status)
	}
	if keyword := strings.TrimSpace(c.Query("keyword")); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("name LIKE ? OR slug LIKE ?", like, like)
	}
	var units []model.EnterpriseOrgUnit
	if err := query.Order("depth asc, sort asc, id asc").Find(&units).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, units)
}

func CreateEnterpriseOrgUnit(c *gin.Context) {
	enterprise, err := currentEnterprise()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var req enterpriseOrgUnitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := validateOrgUnitRequest(req); err != nil {
		common.ApiError(c, err)
		return
	}
	status := req.Status
	if status == 0 {
		status = model.OrgUnitStatusEnabled
	}
	var created model.EnterpriseOrgUnit
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		parentPath, depth, err := resolveOrgUnitParent(tx, enterprise.Id, req.ParentId)
		if err != nil {
			return err
		}
		created = model.EnterpriseOrgUnit{
			EnterpriseId: enterprise.Id,
			ParentId:     req.ParentId,
			Name:         strings.TrimSpace(req.Name),
			Slug:         strings.TrimSpace(req.Slug),
			Description:  req.Description,
			Path:         "",
			Depth:        depth,
			Sort:         req.Sort,
			Status:       status,
		}
		if err := tx.Create(&created).Error; err != nil {
			return err
		}
		created.Path = fmt.Sprintf("%s%d/", parentPath, created.Id)
		return tx.Save(&created).Error
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordEnterpriseAudit(c, enterprise.Id, "org_unit.create", "org_unit", created.Id, nil, created)
	common.ApiSuccess(c, gin.H{"id": created.Id})
}

func UpdateEnterpriseOrgUnit(c *gin.Context) {
	id, err := parsePathInt(c, "id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	enterprise, err := currentEnterprise()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var req enterpriseOrgUnitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := validateOrgUnitRequest(req); err != nil {
		common.ApiError(c, err)
		return
	}
	var before model.EnterpriseOrgUnit
	var after model.EnterpriseOrgUnit
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("id = ? AND enterprise_id = ?", id, enterprise.Id).First(&before).Error; err != nil {
			return err
		}
		parentPath, depth, err := resolveOrgUnitParent(tx, enterprise.Id, req.ParentId)
		if err != nil {
			return err
		}
		if req.ParentId == id {
			return errors.New("部门不能移动到自身下面")
		}
		if req.ParentId > 0 {
			var parent model.EnterpriseOrgUnit
			if err := tx.Where("id = ? AND enterprise_id = ?", req.ParentId, enterprise.Id).First(&parent).Error; err != nil {
				return err
			}
			if strings.Contains(parent.Path, fmt.Sprintf("/%d/", id)) {
				return errors.New("部门不能移动到自己的子部门下面")
			}
		}
		after = before
		after.ParentId = req.ParentId
		after.Name = strings.TrimSpace(req.Name)
		after.Slug = strings.TrimSpace(req.Slug)
		after.Description = req.Description
		after.Depth = depth
		after.Sort = req.Sort
		if req.Status != 0 {
			after.Status = req.Status
		}
		oldPath := before.Path
		after.Path = fmt.Sprintf("%s%d/", parentPath, after.Id)
		if err := tx.Save(&after).Error; err != nil {
			return err
		}
		if oldPath != after.Path {
			return updateOrgUnitChildrenPath(tx, enterprise.Id, oldPath, after.Path, after.Depth-before.Depth)
		}
		return nil
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordEnterpriseAudit(c, enterprise.Id, "org_unit.update", "org_unit", id, before, after)
	common.ApiSuccess(c, gin.H{"id": id})
}

func DeleteEnterpriseOrgUnit(c *gin.Context) {
	id, err := parsePathInt(c, "id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	enterprise, err := currentEnterprise()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var unit model.EnterpriseOrgUnit
	if err := model.DB.Where("id = ? AND enterprise_id = ?", id, enterprise.Id).First(&unit).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	var children int64
	if err := model.DB.Model(&model.EnterpriseOrgUnit{}).Where("enterprise_id = ? AND parent_id = ?", enterprise.Id, id).Count(&children).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	if children > 0 {
		common.ApiErrorMsg(c, "部门下仍有子部门，不能停用")
		return
	}
	var members int64
	if err := model.DB.Model(&model.EnterpriseOrgMembership{}).Where("enterprise_id = ? AND org_unit_id = ?", enterprise.Id, id).Count(&members).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	if members > 0 {
		common.ApiErrorMsg(c, "部门下仍有成员，不能停用")
		return
	}
	before := unit
	unit.Status = model.OrgUnitStatusDisabled
	if err := model.DB.Save(&unit).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	recordEnterpriseAudit(c, enterprise.Id, "org_unit.disable", "org_unit", id, before, unit)
	common.ApiSuccess(c, gin.H{"id": id})
}

func ListEnterpriseMembers(c *gin.Context) {
	enterprise, err := currentEnterprise()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	access, err := enterpriseAccessForRequest(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo := common.GetPageQuery(c)
	query := model.DB.Table("users").
		Select(`users.id AS user_id, users.username, users.display_name, users.email, users.status,
			COALESCE(m.org_unit_id, 0) AS org_unit_id, COALESCE(ou.name, '') AS org_unit_name`).
		Joins("LEFT JOIN enterprise_org_memberships m ON m.user_id = users.id AND m.enterprise_id = ?", enterprise.Id).
		Joins("LEFT JOIN enterprise_org_units ou ON ou.id = m.org_unit_id AND ou.enterprise_id = ?", enterprise.Id)
	query = applyDepartmentOrgUnitScope(query, access, "m.org_unit_id")
	if keyword := strings.TrimSpace(c.Query("keyword")); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("users.username LIKE ? OR users.display_name LIKE ? OR users.email LIKE ?", like, like, like)
	}
	if orgUnitId, err := parseOptionalIntQuery(c, "org_unit_id"); err != nil {
		common.ApiError(c, err)
		return
	} else if orgUnitId > 0 {
		query = query.Where("m.org_unit_id = ?", orgUnitId)
	}
	if c.Query("unassigned") == "true" {
		query = query.Where("m.id IS NULL OR m.org_unit_id = 0")
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	var items []enterpriseMemberItem
	if err := query.Order("users.id asc").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Scan(&items).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	if err := fillEnterpriseMemberPolicyGroupCounts(enterprise.Id, items); err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

func UpdateEnterpriseMemberOrgUnit(c *gin.Context) {
	userId, err := parsePathInt(c, "user_id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	enterprise, err := currentEnterprise()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var req enterpriseMemberOrgUnitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := ensureUserExists(userId); err != nil {
		common.ApiError(c, err)
		return
	}
	if req.OrgUnitId > 0 {
		if err := ensureOrgUnitExists(enterprise.Id, req.OrgUnitId); err != nil {
			common.ApiError(c, err)
			return
		}
	}
	access, err := enterpriseAccessForRequest(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if access.HasDepartmentScope() {
		if req.OrgUnitId <= 0 {
			common.ApiError(c, scopedEnterpriseError())
			return
		}
		if err := requireDepartmentOrgUnitInScope(access, req.OrgUnitId); err != nil {
			common.ApiError(c, err)
			return
		}
		if err := requireDepartmentUserInScope(enterprise.Id, access, userId); err != nil {
			common.ApiError(c, err)
			return
		}
	}
	var before *model.EnterpriseOrgMembership
	var membership model.EnterpriseOrgMembership
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		err := tx.Where("enterprise_id = ? AND user_id = ?", enterprise.Id, userId).First(&membership).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err == nil {
			beforeCopy := membership
			before = &beforeCopy
			membership.OrgUnitId = req.OrgUnitId
			membership.IsPrimary = true
			return tx.Save(&membership).Error
		}
		membership = model.EnterpriseOrgMembership{
			EnterpriseId: enterprise.Id,
			UserId:       userId,
			OrgUnitId:    req.OrgUnitId,
			IsPrimary:    true,
		}
		return tx.Create(&membership).Error
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordEnterpriseAudit(c, enterprise.Id, "member.update_org_unit", "user", userId, before, membership)
	common.ApiSuccess(c, gin.H{"user_id": userId})
}

func PreviewEnterpriseOrgSync(c *gin.Context) {
	enterprise, err := currentEnterprise()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var req enterpriseOrgSyncRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	result, err := service.PreviewEnterpriseOrgSync(enterprise.Id, service.EnterpriseOrgSyncInput(req))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

func ApplyEnterpriseOrgSync(c *gin.Context) {
	enterprise, err := currentEnterprise()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var req enterpriseOrgSyncRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	result, err := service.ApplyEnterpriseOrgSync(enterprise.Id, service.EnterpriseOrgSyncInput(req))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordEnterpriseAudit(c, enterprise.Id, "org_sync.apply", "org_sync", 0, nil, result)
	common.ApiSuccess(c, result)
}

func ListEnterprisePolicyGroups(c *gin.Context) {
	enterprise, err := currentEnterprise()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	access, err := enterpriseAccessForRequest(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo := common.GetPageQuery(c)
	query := model.DB.Model(&model.EnterprisePolicyGroup{}).Where("enterprise_id = ?", enterprise.Id)
	query = applyDepartmentPolicyGroupScope(query, access)
	if status, err := parseOptionalIntQuery(c, "status"); err != nil {
		common.ApiError(c, err)
		return
	} else if status > 0 {
		query = query.Where("status = ?", status)
	}
	if keyword := strings.TrimSpace(c.Query("keyword")); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("name LIKE ? OR slug LIKE ?", like, like)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	var groups []model.EnterprisePolicyGroup
	if err := query.Order("id desc").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Find(&groups).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	items, err := buildEnterprisePolicyGroupItems(enterprise.Id, groups, access)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

func CreateEnterprisePolicyGroup(c *gin.Context) {
	enterprise, err := currentEnterprise()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var req enterprisePolicyGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := validatePolicyGroupRequest(req); err != nil {
		common.ApiError(c, err)
		return
	}
	access, err := enterpriseAccessForRequest(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	orgUnitId, err := policyGroupOrgUnitFromRequest(enterprise.Id, access, req.OrgUnitId, 0)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	sharedOrgUnitIds, err := normalizeEnterprisePolicyGroupShareOrgUnitIds(enterprise.Id, access, req.SharedOrgUnitIds)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	status := req.Status
	if status == 0 {
		status = model.PolicyGroupStatusEnabled
	}
	group := model.EnterprisePolicyGroup{
		EnterpriseId: enterprise.Id,
		OrgUnitId:    orgUnitId,
		Name:         strings.TrimSpace(req.Name),
		Slug:         strings.TrimSpace(req.Slug),
		Description:  req.Description,
		Status:       status,
	}
	if err := model.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&group).Error; err != nil {
			return err
		}
		return replaceEnterprisePolicyGroupShares(tx, enterprise.Id, group.Id, sharedOrgUnitIds)
	}); err != nil {
		common.ApiError(c, err)
		return
	}
	recordEnterpriseAudit(c, enterprise.Id, "policy_group.create", "policy_group", group.Id, nil, gin.H{"group": group, "shared_org_unit_ids": sharedOrgUnitIds})
	common.ApiSuccess(c, gin.H{"id": group.Id})
}

func UpdateEnterprisePolicyGroup(c *gin.Context) {
	id, err := parsePathInt(c, "id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	enterprise, err := currentEnterprise()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var req enterprisePolicyGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := validatePolicyGroupRequest(req); err != nil {
		common.ApiError(c, err)
		return
	}
	var group model.EnterprisePolicyGroup
	if err := model.DB.Where("id = ? AND enterprise_id = ?", id, enterprise.Id).First(&group).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	access, err := enterpriseAccessForRequest(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := requireDepartmentPolicyGroupInScope(access, group); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := requireDepartmentPolicyGroupManageInScope(access, group); err != nil {
		common.ApiError(c, err)
		return
	}
	orgUnitId, err := policyGroupOrgUnitFromRequest(enterprise.Id, access, req.OrgUnitId, group.OrgUnitId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	sharedOrgUnitIds, err := normalizeEnterprisePolicyGroupShareOrgUnitIds(enterprise.Id, access, req.SharedOrgUnitIds)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	beforeSharedOrgUnitIds, err := enterprisePolicyGroupShareOrgUnitIds(enterprise.Id, id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	before := group
	group.OrgUnitId = orgUnitId
	group.Name = strings.TrimSpace(req.Name)
	group.Slug = strings.TrimSpace(req.Slug)
	group.Description = req.Description
	if req.Status != 0 {
		group.Status = req.Status
	}
	if err := model.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&group).Error; err != nil {
			return err
		}
		return replaceEnterprisePolicyGroupShares(tx, enterprise.Id, group.Id, sharedOrgUnitIds)
	}); err != nil {
		common.ApiError(c, err)
		return
	}
	recordEnterpriseAudit(c, enterprise.Id, "policy_group.update", "policy_group", id, gin.H{"group": before, "shared_org_unit_ids": beforeSharedOrgUnitIds}, gin.H{"group": group, "shared_org_unit_ids": sharedOrgUnitIds})
	common.ApiSuccess(c, gin.H{"id": id})
}

func DeleteEnterprisePolicyGroup(c *gin.Context) {
	id, err := parsePathInt(c, "id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	enterprise, err := currentEnterprise()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var group model.EnterprisePolicyGroup
	if err := model.DB.Where("id = ? AND enterprise_id = ?", id, enterprise.Id).First(&group).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	access, err := enterpriseAccessForRequest(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := requireDepartmentPolicyGroupInScope(access, group); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := requireDepartmentPolicyGroupManageInScope(access, group); err != nil {
		common.ApiError(c, err)
		return
	}
	if countEnterprisePoliciesForTarget(enterprise.Id, model.PolicyTargetPolicyGroup, id) > 0 {
		common.ApiErrorMsg(c, "策略分组仍被额度策略引用，不能停用")
		return
	}
	before := group
	group.Status = model.PolicyGroupStatusDisabled
	if err := model.DB.Save(&group).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	recordEnterpriseAudit(c, enterprise.Id, "policy_group.disable", "policy_group", id, before, group)
	common.ApiSuccess(c, gin.H{"id": id})
}

func ListEnterprisePolicyGroupMembers(c *gin.Context) {
	groupId, err := parsePathInt(c, "id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	enterprise, err := currentEnterprise()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	group, err := findEnterprisePolicyGroup(enterprise.Id, groupId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	access, err := enterpriseAccessForRequest(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := requireDepartmentPolicyGroupInScope(access, group); err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo := common.GetPageQuery(c)
	query := model.DB.Table("enterprise_policy_group_members pgm").
		Select("users.id AS user_id, users.username, users.display_name, users.email, users.status, CASE WHEN pgm.role = '' THEN ? ELSE pgm.role END AS role", model.PolicyGroupMemberRoleViewer).
		Joins("JOIN users ON users.id = pgm.user_id").
		Where("pgm.enterprise_id = ? AND pgm.policy_group_id = ?", enterprise.Id, groupId)
	if access.HasDepartmentScope() {
		scopedUserIds := model.DB.Model(&model.EnterpriseOrgMembership{}).
			Select("user_id").
			Where("enterprise_id = ? AND is_primary = ? AND org_unit_id IN ?", enterprise.Id, true, access.ScopedOrgUnitIds)
		query = query.Where("pgm.user_id IN (?)", scopedUserIds)
	}
	if keyword := strings.TrimSpace(c.Query("keyword")); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("users.username LIKE ? OR users.display_name LIKE ? OR users.email LIKE ?", like, like, like)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	var items []enterpriseMemberItem
	if err := query.Order("users.id asc").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Scan(&items).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

func AddEnterprisePolicyGroupMembers(c *gin.Context) {
	groupId, err := parsePathInt(c, "id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	enterprise, err := currentEnterprise()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	group, err := findEnterprisePolicyGroup(enterprise.Id, groupId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	access, err := enterpriseAccessForRequest(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := requireDepartmentPolicyGroupInScope(access, group); err != nil {
		common.ApiError(c, err)
		return
	}
	var req enterprisePolicyGroupMembersRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	if len(req.UserIds) == 0 {
		common.ApiErrorMsg(c, "用户列表不能为空")
		return
	}
	role, err := normalizeEnterprisePolicyGroupMemberRole(req.Role)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	seenUserIds := map[int]struct{}{}
	userIds := make([]int, 0, len(req.UserIds))
	for _, userId := range req.UserIds {
		if userId <= 0 {
			common.ApiErrorMsg(c, "用户 ID 无效")
			return
		}
		if _, ok := seenUserIds[userId]; ok {
			continue
		}
		if err := requireDepartmentUserInScope(enterprise.Id, access, userId); err != nil {
			common.ApiError(c, err)
			return
		}
		seenUserIds[userId] = struct{}{}
		userIds = append(userIds, userId)
	}
	added := make([]int, 0, len(req.UserIds))
	changes := make([]gin.H, 0, len(req.UserIds))
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		for _, userId := range userIds {
			var user model.User
			if err := tx.Where("id = ?", userId).First(&user).Error; err != nil {
				return err
			}
			member := model.EnterprisePolicyGroupMember{
				EnterpriseId:  enterprise.Id,
				PolicyGroupId: groupId,
				UserId:        userId,
				Role:          role,
			}
			var before model.EnterprisePolicyGroupMember
			_ = tx.Where("policy_group_id = ? AND user_id = ?", groupId, userId).First(&before).Error
			if err := tx.Where("policy_group_id = ? AND user_id = ?", groupId, userId).
				Assign(model.EnterprisePolicyGroupMember{Role: role}).
				FirstOrCreate(&member).Error; err != nil {
				return err
			}
			beforeRole := normalizeEnterprisePolicyGroupMemberRoleOrDefault(before.Role)
			added = append(added, userId)
			changes = append(changes, gin.H{"user_id": userId, "before_role": beforeRole, "role": role})
		}
		return nil
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordEnterpriseAudit(c, enterprise.Id, "policy_group.members.add", "policy_group", groupId, nil, gin.H{"user_ids": added, "role": role, "changes": changes})
	common.ApiSuccess(c, gin.H{"id": groupId, "user_ids": added, "role": role})
}

func DeleteEnterprisePolicyGroupMember(c *gin.Context) {
	groupId, err := parsePathInt(c, "id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	userId, err := parsePathInt(c, "user_id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	enterprise, err := currentEnterprise()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	group, err := findEnterprisePolicyGroup(enterprise.Id, groupId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	access, err := enterpriseAccessForRequest(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := requireDepartmentPolicyGroupInScope(access, group); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := requireDepartmentUserInScope(enterprise.Id, access, userId); err != nil {
		common.ApiError(c, err)
		return
	}
	result := model.DB.Where("enterprise_id = ? AND policy_group_id = ? AND user_id = ?", enterprise.Id, groupId, userId).
		Delete(&model.EnterprisePolicyGroupMember{})
	if result.Error != nil {
		common.ApiError(c, result.Error)
		return
	}
	recordEnterpriseAudit(c, enterprise.Id, "policy_group.members.delete", "policy_group", groupId, gin.H{"user_id": userId}, nil)
	common.ApiSuccess(c, gin.H{"id": groupId, "user_id": userId})
}

func ListEnterpriseProjects(c *gin.Context) {
	enterprise, err := currentEnterprise()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	access, err := enterpriseAccessForRequest(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo := common.GetPageQuery(c)
	query := model.DB.Model(&model.EnterpriseProject{}).Where("enterprise_id = ?", enterprise.Id)
	if access.HasDepartmentScope() {
		scopedProjectIds := model.DB.Model(&model.EnterpriseProjectOrgUnit{}).
			Select("project_id").
			Where("enterprise_id = ? AND org_unit_id IN ?", enterprise.Id, access.ScopedOrgUnitIds)
		query = query.Where("id IN (?)", scopedProjectIds)
	}
	if access.HasProjectScope() {
		if len(access.ScopedProjectIds) == 0 {
			query = query.Where("1 = 0")
		} else {
			query = query.Where("id IN ?", access.ScopedProjectIds)
		}
	}
	if status, err := parseOptionalIntQuery(c, "status"); err != nil {
		common.ApiError(c, err)
		return
	} else if status > 0 {
		query = query.Where("status = ?", status)
	}
	if ownerUserId, err := parseOptionalIntQuery(c, "owner_user_id"); err != nil {
		common.ApiError(c, err)
		return
	} else if ownerUserId > 0 {
		query = query.Where("owner_user_id = ?", ownerUserId)
	}
	if orgUnitId, err := parseOptionalIntQuery(c, "org_unit_id"); err != nil {
		common.ApiError(c, err)
		return
	} else if orgUnitId > 0 {
		query = query.Joins("JOIN enterprise_project_org_units epou ON epou.project_id = enterprise_projects.id AND epou.enterprise_id = enterprise_projects.enterprise_id").
			Where("epou.org_unit_id = ?", orgUnitId)
	}
	if keyword := strings.TrimSpace(c.Query("keyword")); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("enterprise_projects.name LIKE ? OR enterprise_projects.slug LIKE ?", like, like)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	var projects []model.EnterpriseProject
	if err := query.Order("enterprise_projects.id desc").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Find(&projects).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	items, err := buildEnterpriseProjectItems(enterprise.Id, projects, access)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

func CreateEnterpriseProject(c *gin.Context) {
	enterprise, err := currentEnterprise()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var req enterpriseProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	project, orgUnitIds, err := projectFromRequest(enterprise.Id, req)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	access, err := enterpriseAccessForRequest(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := requireProjectOwnerInScope(c, access, project); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&project).Error; err != nil {
			return err
		}
		return replaceEnterpriseProjectOrgUnits(tx, enterprise.Id, project.Id, orgUnitIds)
	}); err != nil {
		common.ApiError(c, err)
		return
	}
	recordEnterpriseAudit(c, enterprise.Id, "project.create", "project", project.Id, nil, gin.H{"project": project, "org_unit_ids": orgUnitIds})
	common.ApiSuccess(c, gin.H{"id": project.Id})
}

func UpdateEnterpriseProject(c *gin.Context) {
	id, err := parsePathInt(c, "id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	enterprise, err := currentEnterprise()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var req enterpriseProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	next, orgUnitIds, err := projectFromRequest(enterprise.Id, req)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var before model.EnterpriseProject
	beforeOrgUnitIds, err := enterpriseProjectOrgUnitIds(enterprise.Id, id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	access, err := enterpriseAccessForRequest(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("id = ? AND enterprise_id = ?", id, enterprise.Id).First(&before).Error; err != nil {
			return err
		}
		if err := requireProjectManageInScope(access, before.Id); err != nil {
			return err
		}
		if err := requireProjectOwnerUpdateInScope(c, access, before, next); err != nil {
			return err
		}
		next.Id = before.Id
		next.CreatedAt = before.CreatedAt
		if err := tx.Save(&next).Error; err != nil {
			return err
		}
		return replaceEnterpriseProjectOrgUnits(tx, enterprise.Id, id, orgUnitIds)
	}); err != nil {
		common.ApiError(c, err)
		return
	}
	recordEnterpriseAudit(c, enterprise.Id, "project.update", "project", id, gin.H{"project": before, "org_unit_ids": beforeOrgUnitIds}, gin.H{"project": next, "org_unit_ids": orgUnitIds})
	common.ApiSuccess(c, gin.H{"id": id})
}

func DeleteEnterpriseProject(c *gin.Context) {
	id, err := parsePathInt(c, "id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	enterprise, err := currentEnterprise()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if countEnterprisePoliciesForTarget(enterprise.Id, model.PolicyTargetProject, id) > 0 {
		common.ApiErrorMsg(c, "项目仍被额度策略引用，不能停用")
		return
	}
	var project model.EnterpriseProject
	if err := model.DB.Where("id = ? AND enterprise_id = ?", id, enterprise.Id).First(&project).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	access, err := enterpriseAccessForRequest(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := requireProjectManageInScope(access, project.Id); err != nil {
		common.ApiError(c, err)
		return
	}
	before := project
	project.Status = model.EnterpriseProjectStatusDisabled
	if err := model.DB.Save(&project).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	recordEnterpriseAudit(c, enterprise.Id, "project.disable", "project", id, before, project)
	common.ApiSuccess(c, gin.H{"id": id})
}

func ListEnterpriseProjectMembers(c *gin.Context) {
	projectId, err := parsePathInt(c, "id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	enterprise, err := currentEnterprise()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	access, err := enterpriseAccessForRequest(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := requireProjectInScope(access, projectId); err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo := common.GetPageQuery(c)
	query := model.DB.Table("enterprise_project_members epm").
		Select("epm.user_id, epm.role, users.username, users.display_name, users.email, users.status").
		Joins("JOIN users ON users.id = epm.user_id").
		Where("epm.enterprise_id = ? AND epm.project_id = ?", enterprise.Id, projectId)
	if keyword := strings.TrimSpace(c.Query("keyword")); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("users.username LIKE ? OR users.display_name LIKE ? OR users.email LIKE ?", like, like, like)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	var items []enterpriseMemberItem
	if err := query.Order("users.id asc").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Scan(&items).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

func UpsertEnterpriseProjectMember(c *gin.Context) {
	projectId, err := parsePathInt(c, "id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	enterprise, err := currentEnterprise()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	access, err := enterpriseAccessForRequest(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := requireProjectManageInScope(access, projectId); err != nil {
		common.ApiError(c, err)
		return
	}
	var req enterpriseProjectMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	role, err := normalizeEnterpriseProjectMemberRole(req.Role)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if req.UserId <= 0 {
		common.ApiErrorMsg(c, "用户 ID 无效")
		return
	}
	if err := ensureProjectExists(enterprise.Id, projectId); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := ensureUserExists(req.UserId); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := ensureEnterpriseMemberExists(enterprise.Id, req.UserId); err != nil {
		common.ApiError(c, err)
		return
	}
	member := model.EnterpriseProjectMember{
		EnterpriseId: enterprise.Id,
		ProjectId:    projectId,
		UserId:       req.UserId,
		Role:         role,
	}
	var before model.EnterpriseProjectMember
	_ = model.DB.Where("enterprise_id = ? AND project_id = ? AND user_id = ?", enterprise.Id, projectId, req.UserId).First(&before).Error
	if err := model.DB.Where("enterprise_id = ? AND project_id = ? AND user_id = ?", enterprise.Id, projectId, req.UserId).
		Assign(model.EnterpriseProjectMember{Role: role}).
		FirstOrCreate(&member).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	recordEnterpriseAudit(c, enterprise.Id, "project.members.upsert", "project", projectId, before, member)
	common.ApiSuccess(c, gin.H{"id": projectId, "user_id": req.UserId, "role": role})
}

func DeleteEnterpriseProjectMember(c *gin.Context) {
	projectId, err := parsePathInt(c, "id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	userId, err := parsePathInt(c, "user_id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	enterprise, err := currentEnterprise()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	access, err := enterpriseAccessForRequest(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := requireProjectManageInScope(access, projectId); err != nil {
		common.ApiError(c, err)
		return
	}
	var before model.EnterpriseProjectMember
	_ = model.DB.Where("enterprise_id = ? AND project_id = ? AND user_id = ?", enterprise.Id, projectId, userId).First(&before).Error
	result := model.DB.Where("enterprise_id = ? AND project_id = ? AND user_id = ?", enterprise.Id, projectId, userId).
		Delete(&model.EnterpriseProjectMember{})
	if result.Error != nil {
		common.ApiError(c, result.Error)
		return
	}
	recordEnterpriseAudit(c, enterprise.Id, "project.members.delete", "project", projectId, before, nil)
	common.ApiSuccess(c, gin.H{"id": projectId, "user_id": userId})
}

func ListEnterpriseQuotaPolicies(c *gin.Context) {
	enterprise, err := currentEnterprise()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	access, err := enterpriseAccessForRequest(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo := common.GetPageQuery(c)
	query := model.DB.Model(&model.EnterpriseQuotaPolicy{}).Where("enterprise_id = ?", enterprise.Id)
	query = applyDepartmentQuotaPolicyScope(query, enterprise.Id, access)
	if targetType := strings.TrimSpace(c.Query("target_type")); targetType != "" {
		query = query.Where("target_type = ?", targetType)
	}
	if metric := strings.TrimSpace(c.Query("metric")); metric != "" {
		query = query.Where("metric = ?", metric)
	}
	if status, err := parseOptionalIntQuery(c, "status"); err != nil {
		common.ApiError(c, err)
		return
	} else if status > 0 {
		query = query.Where("status = ?", status)
	}
	if keyword := strings.TrimSpace(c.Query("keyword")); keyword != "" {
		query = query.Where("name LIKE ?", "%"+keyword+"%")
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	var policies []model.EnterpriseQuotaPolicy
	if err := query.Order("priority desc, id desc").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Find(&policies).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	items := make([]enterpriseQuotaPolicyItem, 0, len(policies))
	for _, policy := range policies {
		items = append(items, enterpriseQuotaPolicyItem{
			EnterpriseQuotaPolicy: policy,
			TargetName:            resolveEnterprisePolicyTargetName(enterprise.Id, policy.TargetType, policy.TargetId),
			UsedValue:             sumEnterprisePolicyUsedValue(policy.Id),
		})
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

func CreateEnterpriseQuotaPolicy(c *gin.Context) {
	enterprise, err := currentEnterprise()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var req enterpriseQuotaPolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	policy, err := quotaPolicyFromRequest(enterprise.Id, req)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	access, err := enterpriseAccessForRequest(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := requireDepartmentQuotaPolicyInScope(enterprise.Id, access, policy); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.DB.Create(&policy).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	recordEnterpriseAudit(c, enterprise.Id, "quota_policy.create", "quota_policy", policy.Id, nil, policy)
	common.ApiSuccess(c, gin.H{"id": policy.Id})
}

func UpdateEnterpriseQuotaPolicy(c *gin.Context) {
	id, err := parsePathInt(c, "id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	enterprise, err := currentEnterprise()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var req enterpriseQuotaPolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	next, err := quotaPolicyFromRequest(enterprise.Id, req)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var policy model.EnterpriseQuotaPolicy
	if err := model.DB.Where("id = ? AND enterprise_id = ?", id, enterprise.Id).First(&policy).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	access, err := enterpriseAccessForRequest(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := requireDepartmentQuotaPolicyInScope(enterprise.Id, access, policy); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := requireDepartmentQuotaPolicyInScope(enterprise.Id, access, next); err != nil {
		common.ApiError(c, err)
		return
	}
	before := policy
	next.Id = policy.Id
	next.CreatedAt = policy.CreatedAt
	if err := model.DB.Save(&next).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	recordEnterpriseAudit(c, enterprise.Id, "quota_policy.update", "quota_policy", id, before, next)
	common.ApiSuccess(c, gin.H{"id": id})
}

func DeleteEnterpriseQuotaPolicy(c *gin.Context) {
	id, err := parsePathInt(c, "id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	enterprise, err := currentEnterprise()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var policy model.EnterpriseQuotaPolicy
	if err := model.DB.Where("id = ? AND enterprise_id = ?", id, enterprise.Id).First(&policy).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	access, err := enterpriseAccessForRequest(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := requireDepartmentQuotaPolicyInScope(enterprise.Id, access, policy); err != nil {
		common.ApiError(c, err)
		return
	}
	before := policy
	policy.Status = model.QuotaPolicyStatusDisabled
	if err := model.DB.Save(&policy).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	recordEnterpriseAudit(c, enterprise.Id, "quota_policy.disable", "quota_policy", id, before, policy)
	common.ApiSuccess(c, gin.H{"id": id})
}

func ReconcileEnterpriseQuotaCounters(c *gin.Context) {
	enterprise, err := currentEnterprise()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var req enterpriseQuotaCounterReconciliationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	result, err := service.ReconcileEnterpriseQuotaRedisCounters(service.EnterpriseQuotaCounterReconciliationParams{
		EnterpriseId:        enterprise.Id,
		Limit:               req.Limit,
		Repair:              req.Repair,
		IncludeRedisOrphans: req.IncludeRedisOrphans,
		ActorUserId:         c.GetInt("id"),
		RequestId:           c.GetHeader(common.RequestIdKey),
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

func ListEnterpriseQuotaRequests(c *gin.Context) {
	enterprise, err := currentEnterprise()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo := common.GetPageQuery(c)
	query := model.DB.Model(&model.EnterpriseQuotaRequest{}).Where("enterprise_id = ?", enterprise.Id)
	access, err := enterpriseAccessForRequest(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	canReview := access.HasCapability(service.EnterpriseCapabilityQuotaApprove)
	if !canReview {
		query = query.Where("applicant_user_id = ?", c.GetInt("id"))
	} else {
		query = applyDepartmentQuotaRequestScope(query, enterprise.Id, access)
	}
	if requestId, err := parseOptionalIntQuery(c, "id"); err != nil {
		common.ApiError(c, err)
		return
	} else if requestId > 0 {
		query = query.Where("id = ?", requestId)
	}
	if status := strings.TrimSpace(c.Query("status")); status != "" {
		query = query.Where("status = ?", status)
	}
	if policyId, err := parseOptionalIntQuery(c, "policy_id"); err != nil {
		common.ApiError(c, err)
		return
	} else if policyId > 0 {
		query = query.Where("policy_id = ?", policyId)
	}
	if projectId, err := parseOptionalIntQuery(c, "project_id"); err != nil {
		common.ApiError(c, err)
		return
	} else if projectId > 0 {
		query = query.Where(
			"(project_id = ? OR (project_id = ? AND target_type = ? AND target_id = ?))",
			projectId,
			0,
			model.PolicyTargetProject,
			projectId,
		)
	}
	if targetType := strings.TrimSpace(c.Query("target_type")); targetType != "" {
		query = query.Where("target_type = ?", targetType)
	}
	if targetId, err := parseOptionalIntQuery(c, "target_id"); err != nil {
		common.ApiError(c, err)
		return
	} else if targetId > 0 {
		query = query.Where("target_id = ?", targetId)
	}
	if applicantUserId, err := parseOptionalIntQuery(c, "applicant_user_id"); err != nil {
		common.ApiError(c, err)
		return
	} else if applicantUserId > 0 && canReview {
		query = query.Where("applicant_user_id = ?", applicantUserId)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	var requests []model.EnterpriseQuotaRequest
	if err := query.Order("id desc").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Find(&requests).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	items, err := buildEnterpriseQuotaRequestItems(enterprise.Id, requests)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

func ListEnterpriseQuotaRequestPolicies(c *gin.Context) {
	enterprise, err := currentEnterprise()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	projectId, err := parseOptionalIntQuery(c, "project_id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	ctx, err := service.ResolveEnterpriseContextWithProject(c.GetInt("id"), 0, 0, projectId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	policies, err := service.ListRequestableEnterpriseQuotaPolicies(ctx, time.Now())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	items := make([]enterpriseQuotaPolicyItem, 0, len(policies))
	for _, policy := range policies {
		if policy.EnterpriseId != enterprise.Id {
			continue
		}
		items = append(items, enterpriseQuotaPolicyItem{
			EnterpriseQuotaPolicy: policy,
			TargetName:            resolveEnterprisePolicyTargetName(enterprise.Id, policy.TargetType, policy.TargetId),
			UsedValue:             sumEnterprisePolicyUsedValue(policy.Id),
		})
	}
	common.ApiSuccess(c, items)
}

func SubmitEnterpriseQuotaRequest(c *gin.Context) {
	enterprise, err := currentEnterprise()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var req enterpriseQuotaRequestSubmitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	quotaRequest, err := quotaRequestFromSubmitRequest(enterprise.Id, c.GetInt("id"), req)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var quotaRequestId int
	if err := model.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&quotaRequest).Error; err != nil {
			return err
		}
		audit, err := recordEnterpriseAuditWithDB(tx, c, enterprise.Id, "quota_request.submit", "quota_request", quotaRequest.Id, nil, quotaRequest)
		if err != nil {
			return err
		}
		if err := service.EnqueueEnterpriseQuotaRequestOutboxWithDB(tx, quotaRequest, audit, "quota_request.submit"); err != nil {
			return err
		}
		quotaRequestId = quotaRequest.Id
		return nil
	}); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"id": quotaRequestId})
}

func ApproveEnterpriseQuotaRequest(c *gin.Context) {
	decideEnterpriseQuotaRequest(c, model.EnterpriseQuotaRequestStatusApproved)
}

func RejectEnterpriseQuotaRequest(c *gin.Context) {
	decideEnterpriseQuotaRequest(c, model.EnterpriseQuotaRequestStatusRejected)
}

func WithdrawEnterpriseQuotaRequest(c *gin.Context) {
	id, err := parsePathInt(c, "id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	enterprise, err := currentEnterprise()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var quotaRequest model.EnterpriseQuotaRequest
	if err := model.DB.Where("id = ? AND enterprise_id = ?", id, enterprise.Id).First(&quotaRequest).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	if quotaRequest.Status != model.EnterpriseQuotaRequestStatusPending {
		common.ApiError(c, errors.New("只能撤回待审批申请"))
		return
	}
	access, err := enterpriseAccessForRequest(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	canReview := access.HasCapability(service.EnterpriseCapabilityQuotaApprove)
	if canReview && access.HasDepartmentScope() && quotaRequest.ApplicantUserId != c.GetInt("id") {
		if err := requireDepartmentUserInScope(enterprise.Id, access, quotaRequest.ApplicantUserId); err != nil {
			common.ApiError(c, err)
			return
		}
	}
	if !canReview && quotaRequest.ApplicantUserId != c.GetInt("id") {
		common.ApiError(c, errors.New("只能撤回自己的申请"))
		return
	}
	before := quotaRequest
	quotaRequest.Status = model.EnterpriseQuotaRequestStatusWithdrawn
	if err := model.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&quotaRequest).Error; err != nil {
			return err
		}
		audit, err := recordEnterpriseAuditWithDB(tx, c, enterprise.Id, "quota_request.withdraw", "quota_request", quotaRequest.Id, before, quotaRequest)
		if err != nil {
			return err
		}
		return service.EnqueueEnterpriseQuotaRequestOutboxWithDB(tx, quotaRequest, audit, "quota_request.withdraw")
	}); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"id": quotaRequest.Id})
}

func GetEnterpriseUsageSummary(c *gin.Context) {
	enterprise, err := currentEnterprise()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	params, err := enterpriseUsageQueryFromRequest(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	access, err := enterpriseAccessForRequest(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := applyDepartmentUsageScope(&params, access); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := applyProjectUsageScope(&params, access); err != nil {
		common.ApiError(c, err)
		return
	}
	rows, err := loadEnterpriseUsageRows(enterprise.Id, params)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	summary := enterpriseUsageSummary{
		StartTime: params.StartTime,
		EndTime:   params.EndTime,
		Total:     aggregateEnterpriseUsageTotal(rows),
		ByModel:   aggregateEnterpriseUsageBreakdown(rows, "model"),
		ByStatus:  aggregateEnterpriseUsageBreakdown(rows, "status"),
	}
	common.ApiSuccess(c, summary)
}

func GetEnterpriseUsageBreakdown(c *gin.Context) {
	enterprise, err := currentEnterprise()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	params, err := enterpriseUsageQueryFromRequest(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	access, err := enterpriseAccessForRequest(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := applyDepartmentUsageScope(&params, access); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := applyProjectUsageScope(&params, access); err != nil {
		common.ApiError(c, err)
		return
	}
	dimension := strings.TrimSpace(c.Query("dimension"))
	if dimension == "" {
		dimension = "org_unit"
	}
	if !isSupportedEnterpriseUsageDimension(dimension) {
		common.ApiErrorMsg(c, "不支持的用量聚合维度")
		return
	}
	rows, err := loadEnterpriseUsageRows(enterprise.Id, params)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	items := aggregateEnterpriseUsageBreakdownWithGranularity(rows, dimension, params.Granularity)
	sortEnterpriseUsageBreakdown(items, c.Query("sort_by"), c.Query("sort_order"))
	if err := fillEnterpriseUsageBreakdownNames(enterprise.Id, dimension, items); err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo := common.GetPageQuery(c)
	pageInfo.SetTotal(len(items))
	start := pageInfo.GetStartIdx()
	if start > len(items) {
		start = len(items)
	}
	end := start + pageInfo.GetPageSize()
	if end > len(items) {
		end = len(items)
	}
	pageInfo.SetItems(items[start:end])
	common.ApiSuccess(c, pageInfo)
}

func ExportEnterpriseUsageBreakdown(c *gin.Context) {
	enterprise, err := currentEnterprise()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	params, err := enterpriseUsageQueryFromRequest(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	access, err := enterpriseAccessForRequest(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := applyDepartmentUsageScope(&params, access); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := applyProjectUsageScope(&params, access); err != nil {
		common.ApiError(c, err)
		return
	}
	dimension := strings.TrimSpace(c.Query("dimension"))
	if dimension == "" {
		dimension = "org_unit"
	}
	if !isSupportedEnterpriseUsageDimension(dimension) {
		common.ApiErrorMsg(c, "不支持的用量聚合维度")
		return
	}
	rows, err := loadEnterpriseUsageRows(enterprise.Id, params)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	items := aggregateEnterpriseUsageBreakdownWithGranularity(rows, dimension, params.Granularity)
	sortEnterpriseUsageBreakdown(items, c.Query("sort_by"), c.Query("sort_order"))
	if err := fillEnterpriseUsageBreakdownNames(enterprise.Id, dimension, items); err != nil {
		common.ApiError(c, err)
		return
	}
	payload, err := enterpriseUsageBreakdownCSV(items)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	filename := fmt.Sprintf("enterprise-usage-%s-%d-%d.csv", dimension, params.StartTime, params.EndTime)
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.Data(http.StatusOK, "text/csv; charset=utf-8", payload)
}

func ListEnterpriseAuditLogs(c *gin.Context) {
	enterprise, err := currentEnterprise()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo := common.GetPageQuery(c)
	query := model.DB.Model(&model.EnterpriseAuditLog{}).Where("enterprise_id = ?", enterprise.Id)
	access, err := enterpriseAccessForRequest(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	query = applyEnterpriseAuditScope(query, enterprise.Id, access)
	if action := strings.TrimSpace(c.Query("action")); action != "" {
		query = query.Where("action = ?", action)
	}
	if targetType := strings.TrimSpace(c.Query("target_type")); targetType != "" {
		query = query.Where("target_type = ?", targetType)
	}
	if targetId, err := parseOptionalIntQuery(c, "target_id"); err != nil {
		common.ApiError(c, err)
		return
	} else if targetId > 0 {
		query = query.Where("target_id = ?", targetId)
	}
	if actorUserId, err := parseOptionalIntQuery(c, "actor_user_id"); err != nil {
		common.ApiError(c, err)
		return
	} else if actorUserId > 0 {
		query = query.Where("actor_user_id = ?", actorUserId)
	}
	if requestId := strings.TrimSpace(c.Query("request_id")); requestId != "" {
		query = query.Where("request_id = ?", requestId)
	}
	if startTime, err := parseOptionalInt64Query(c, "start_time"); err != nil {
		common.ApiError(c, err)
		return
	} else if startTime > 0 {
		query = query.Where("created_at >= ?", startTime)
	}
	if endTime, err := parseOptionalInt64Query(c, "end_time"); err != nil {
		common.ApiError(c, err)
		return
	} else if endTime > 0 {
		query = query.Where("created_at <= ?", endTime)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	var logs []model.EnterpriseAuditLog
	if err := query.Order("created_at desc, id desc").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Find(&logs).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(logs)
	common.ApiSuccess(c, pageInfo)
}

func ListEnterpriseGovernanceQueueAdmissions(c *gin.Context) {
	enterprise, err := currentEnterprise()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo := common.GetPageQuery(c)
	query := model.DB.Model(&model.EnterpriseGovernanceQueueAdmission{}).Where("enterprise_id = ?", enterprise.Id)
	access, err := enterpriseAccessForRequest(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	query = applyEnterpriseQueueAdmissionScope(query, enterprise.Id, access)
	if status := strings.TrimSpace(c.Query("status")); status != "" {
		query = query.Where("status = ?", status)
	}
	if requestId := strings.TrimSpace(c.Query("request_id")); requestId != "" {
		query = query.Where("request_id = ?", requestId)
	}
	if modelName := strings.TrimSpace(c.Query("model_name")); modelName != "" {
		query = query.Where("model_name = ?", modelName)
	}
	if userId, err := parseOptionalIntQuery(c, "user_id"); err != nil {
		common.ApiError(c, err)
		return
	} else if userId > 0 {
		query = query.Where("user_id = ?", userId)
	}
	if tokenId, err := parseOptionalIntQuery(c, "token_id"); err != nil {
		common.ApiError(c, err)
		return
	} else if tokenId > 0 {
		query = query.Where("token_id = ?", tokenId)
	}
	if orgUnitId, err := parseOptionalIntQuery(c, "org_unit_id"); err != nil {
		common.ApiError(c, err)
		return
	} else if orgUnitId > 0 {
		query = query.Where("org_unit_id = ?", orgUnitId)
	}
	if projectId, err := parseOptionalIntQuery(c, "project_id"); err != nil {
		common.ApiError(c, err)
		return
	} else if projectId > 0 {
		query = query.Where("project_id = ?", projectId)
	}
	if policyId, err := parseOptionalIntQuery(c, "policy_id"); err != nil {
		common.ApiError(c, err)
		return
	} else if policyId > 0 {
		query = query.Where("policy_id = ?", policyId)
	}
	if channelId, err := parseOptionalIntQuery(c, "channel_id"); err != nil {
		common.ApiError(c, err)
		return
	} else if channelId > 0 {
		query = query.Where("channel_id = ?", channelId)
	}
	if startTime, err := parseOptionalInt64Query(c, "start_time"); err != nil {
		common.ApiError(c, err)
		return
	} else if startTime > 0 {
		query = query.Where("created_at >= ?", startTime)
	}
	if endTime, err := parseOptionalInt64Query(c, "end_time"); err != nil {
		common.ApiError(c, err)
		return
	} else if endTime > 0 {
		query = query.Where("created_at <= ?", endTime)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	var rows []model.EnterpriseGovernanceQueueAdmission
	if err := query.Order("created_at desc, id desc").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Find(&rows).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(rows)
	common.ApiSuccess(c, pageInfo)
}

func ListEnterpriseNotificationOutbox(c *gin.Context) {
	enterprise, err := currentEnterprise()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo := common.GetPageQuery(c)
	targetId, err := parseOptionalIntQuery(c, "target_id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	webhookId, err := parseOptionalIntQuery(c, "webhook_id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	startTime, err := parseOptionalInt64Query(c, "start_time")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	endTime, err := parseOptionalInt64Query(c, "end_time")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	items, total, err := service.ListEnterpriseNotificationOutbox(service.EnterpriseNotificationOutboxListParams{
		EnterpriseId: enterprise.Id,
		Channel:      c.Query("channel"),
		EventType:    c.Query("event_type"),
		Status:       c.Query("status"),
		TargetType:   c.Query("target_type"),
		TargetId:     targetId,
		WebhookId:    webhookId,
		StartTime:    startTime,
		EndTime:      endTime,
		Offset:       pageInfo.GetStartIdx(),
		Limit:        pageInfo.GetPageSize(),
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

func RetryEnterpriseNotificationOutbox(c *gin.Context) {
	id, err := parsePathInt64(c, "id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	enterprise, err := currentEnterprise()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	before, after, err := service.RetryEnterpriseNotificationOutbox(enterprise.Id, id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordEnterpriseAudit(c, enterprise.Id, "notification_outbox.retry", "notification_outbox", int(id), service.EnterpriseNotificationOutboxToItem(before), service.EnterpriseNotificationOutboxToItem(after))
	common.ApiSuccess(c, service.EnterpriseNotificationOutboxToItem(after))
}

func GetEnterpriseNotificationOutboxWorkerMetrics(c *gin.Context) {
	common.ApiSuccess(c, service.GetEnterpriseNotificationOutboxWorkerMetrics())
}

func ListEnterpriseNotificationPreferences(c *gin.Context) {
	enterprise, err := currentEnterprise()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	items, err := service.ListEnterpriseNotificationPreferences(enterprise.Id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, items)
}

func UpdateEnterpriseNotificationPreference(c *gin.Context) {
	enterprise, err := currentEnterprise()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var req enterpriseNotificationPreferenceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	before, after, err := service.UpsertEnterpriseNotificationPreference(enterprise.Id, service.EnterpriseNotificationPreferenceUpsertInput{
		Channel:        req.Channel,
		EventType:      req.EventType,
		Enabled:        req.Enabled,
		RecipientScope: req.RecipientScope,
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordEnterpriseAudit(c, enterprise.Id, "notification_preference.update", "notification_preference", after.Id, sanitizeEnterpriseNotificationPreferenceAuditValue(before), sanitizeEnterpriseNotificationPreferenceAuditValue(after))
	item, err := service.EnterpriseNotificationPreferenceToItem(after)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, item)
}

func ListEnterpriseWebhooks(c *gin.Context) {
	enterprise, err := currentEnterprise()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	items, err := service.ListEnterpriseWebhooks(enterprise.Id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, items)
}

func CreateEnterpriseWebhook(c *gin.Context) {
	enterprise, err := currentEnterprise()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var req enterpriseWebhookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	webhook, err := service.CreateEnterpriseWebhook(enterprise.Id, enterpriseWebhookInputFromRequest(req))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordEnterpriseAudit(c, enterprise.Id, "webhook.create", "enterprise_webhook", webhook.Id, nil, sanitizeEnterpriseWebhookAuditValue(webhook))
	common.ApiSuccess(c, service.EnterpriseWebhookToItem(webhook))
}

func UpdateEnterpriseWebhook(c *gin.Context) {
	id, err := parsePathInt(c, "id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	enterprise, err := currentEnterprise()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var req enterpriseWebhookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	before, after, err := service.UpdateEnterpriseWebhook(enterprise.Id, id, enterpriseWebhookInputFromRequest(req))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordEnterpriseAudit(c, enterprise.Id, "webhook.update", "enterprise_webhook", id, sanitizeEnterpriseWebhookAuditValue(before), sanitizeEnterpriseWebhookAuditValue(after))
	common.ApiSuccess(c, service.EnterpriseWebhookToItem(after))
}

func DeleteEnterpriseWebhook(c *gin.Context) {
	id, err := parsePathInt(c, "id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	enterprise, err := currentEnterprise()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	before, after, err := service.DisableEnterpriseWebhook(enterprise.Id, id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordEnterpriseAudit(c, enterprise.Id, "webhook.disable", "enterprise_webhook", id, sanitizeEnterpriseWebhookAuditValue(before), sanitizeEnterpriseWebhookAuditValue(after))
	common.ApiSuccess(c, service.EnterpriseWebhookToItem(after))
}

func TestEnterpriseWebhook(c *gin.Context) {
	id, err := parsePathInt(c, "id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	enterprise, err := currentEnterprise()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var webhook model.EnterpriseWebhook
	if err := model.DB.Where("id = ? AND enterprise_id = ?", id, enterprise.Id).First(&webhook).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	result, err := service.SendEnterpriseWebhookWithResult(webhook, service.BuildEnterpriseWebhookTestOutbox(enterprise.Id, webhook.Id))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordEnterpriseAudit(c, enterprise.Id, "webhook.test", "enterprise_webhook", id, sanitizeEnterpriseWebhookAuditValue(webhook), gin.H{
		"success":     result.Success,
		"status_code": result.StatusCode,
		"duration_ms": result.DurationMs,
		"error":       result.Error,
		"signed":      result.SignatureHeader != "",
	})
	common.ApiSuccess(c, result)
}

func currentEnterprise() (*model.Enterprise, error) {
	if err := model.EnsureDefaultEnterprise(); err != nil {
		return nil, err
	}
	return model.GetDefaultEnterprise()
}

func enterpriseAccessForRequest(c *gin.Context) (service.EnterpriseAccess, error) {
	return service.EnterpriseAccessForUser(c.GetInt("id"), c.GetInt("role"))
}

func scopedEnterpriseError() error {
	return errors.New("无权访问权限范围外的企业治理数据")
}

func applyDepartmentOrgUnitScope(query *gorm.DB, access service.EnterpriseAccess, column string) *gorm.DB {
	if !access.HasDepartmentScope() {
		return query
	}
	return query.Where(column+" IN ?", access.ScopedOrgUnitIds)
}

func applyDepartmentPolicyGroupScope(query *gorm.DB, access service.EnterpriseAccess) *gorm.DB {
	if !access.HasDepartmentScope() {
		return query
	}
	return query.Where("id IN (?)", departmentScopedPolicyGroupIds(access.EnterpriseId, access))
}

func requireDepartmentOrgUnitInScope(access service.EnterpriseAccess, orgUnitId int) error {
	if !access.HasDepartmentScope() {
		return nil
	}
	if !access.OrgUnitInScope(orgUnitId) {
		return scopedEnterpriseError()
	}
	return nil
}

func requireDepartmentUserInScope(enterpriseId int, access service.EnterpriseAccess, userId int) error {
	if !access.HasDepartmentScope() {
		return nil
	}
	ok, err := service.EnterpriseUserInOrgUnitScope(enterpriseId, userId, access.ScopedOrgUnitIds)
	if err != nil {
		return err
	}
	if !ok {
		return scopedEnterpriseError()
	}
	return nil
}

func requireDepartmentPolicyGroupInScope(access service.EnterpriseAccess, group model.EnterprisePolicyGroup) error {
	if !access.HasDepartmentScope() {
		return nil
	}
	if access.OrgUnitInScope(group.OrgUnitId) {
		return nil
	}
	shared := false
	if group.Id > 0 && len(access.ScopedOrgUnitIds) > 0 {
		var count int64
		_ = model.DB.Model(&model.EnterprisePolicyGroupShare{}).
			Where("policy_group_id = ? AND org_unit_id IN ?", group.Id, access.ScopedOrgUnitIds).
			Count(&count).Error
		shared = count > 0
	}
	if !shared {
		return scopedEnterpriseError()
	}
	return nil
}

func requireDepartmentPolicyGroupManageInScope(access service.EnterpriseAccess, group model.EnterprisePolicyGroup) error {
	if !access.HasDepartmentScope() {
		return nil
	}
	if !access.OrgUnitInScope(group.OrgUnitId) {
		return scopedEnterpriseError()
	}
	return nil
}

func departmentScopedPolicyGroupIds(enterpriseId int, access service.EnterpriseAccess) *gorm.DB {
	sharedPolicyGroupIds := model.DB.Model(&model.EnterprisePolicyGroupShare{}).
		Select("policy_group_id").
		Where("enterprise_id = ? AND org_unit_id IN ?", enterpriseId, access.ScopedOrgUnitIds)
	return model.DB.Model(&model.EnterprisePolicyGroup{}).
		Select("id").
		Where("enterprise_id = ? AND (org_unit_id IN ? OR id IN (?))", enterpriseId, access.ScopedOrgUnitIds, sharedPolicyGroupIds)
}

func requireProjectInScope(access service.EnterpriseAccess, projectId int) error {
	if !access.HasProjectScope() {
		return nil
	}
	if !access.ProjectInScope(projectId) {
		return scopedEnterpriseError()
	}
	return nil
}

func requireProjectManageInScope(access service.EnterpriseAccess, projectId int) error {
	if !access.HasProjectManageScope() {
		return nil
	}
	if !access.ProjectManageInScope(projectId) {
		return scopedEnterpriseError()
	}
	return nil
}

func requireProjectOwnerInScope(c *gin.Context, access service.EnterpriseAccess, project model.EnterpriseProject) error {
	if !access.HasProjectManageScope() {
		return nil
	}
	if project.OwnerUserId != c.GetInt("id") {
		return scopedEnterpriseError()
	}
	return nil
}

func requireProjectOwnerUpdateInScope(c *gin.Context, access service.EnterpriseAccess, before model.EnterpriseProject, after model.EnterpriseProject) error {
	if !access.HasProjectManageScope() {
		return nil
	}
	if before.OwnerUserId == after.OwnerUserId {
		return nil
	}
	if after.OwnerUserId != c.GetInt("id") {
		return scopedEnterpriseError()
	}
	return nil
}

func applyDepartmentQuotaPolicyScope(query *gorm.DB, enterpriseId int, access service.EnterpriseAccess) *gorm.DB {
	if !access.HasDepartmentScope() {
		return query
	}
	scopedUserIds := model.DB.Model(&model.EnterpriseOrgMembership{}).
		Select("user_id").
		Where("enterprise_id = ? AND is_primary = ? AND org_unit_id IN ?", enterpriseId, true, access.ScopedOrgUnitIds)
	scopedPolicyGroupIds := departmentScopedPolicyGroupIds(enterpriseId, access)
	return query.Where(
		"(target_type = ? AND target_id IN ?) OR (target_type = ? AND target_id IN (?)) OR (target_type = ? AND target_id IN (?))",
		model.PolicyTargetOrgUnit,
		access.ScopedOrgUnitIds,
		model.PolicyTargetUser,
		scopedUserIds,
		model.PolicyTargetPolicyGroup,
		scopedPolicyGroupIds,
	)
}

func requireDepartmentQuotaPolicyInScope(enterpriseId int, access service.EnterpriseAccess, policy model.EnterpriseQuotaPolicy) error {
	if !access.HasDepartmentScope() {
		return nil
	}
	switch policy.TargetType {
	case model.PolicyTargetOrgUnit:
		return requireDepartmentOrgUnitInScope(access, policy.TargetId)
	case model.PolicyTargetUser:
		return requireDepartmentUserInScope(enterpriseId, access, policy.TargetId)
	case model.PolicyTargetPolicyGroup:
		group, err := findEnterprisePolicyGroup(enterpriseId, policy.TargetId)
		if err != nil {
			return err
		}
		return requireDepartmentPolicyGroupInScope(access, group)
	default:
		return scopedEnterpriseError()
	}
}

func applyDepartmentQuotaRequestScope(query *gorm.DB, enterpriseId int, access service.EnterpriseAccess) *gorm.DB {
	if !access.HasDepartmentScope() {
		return query
	}
	scopedUserIds := model.DB.Model(&model.EnterpriseOrgMembership{}).
		Select("user_id").
		Where("enterprise_id = ? AND is_primary = ? AND org_unit_id IN ?", enterpriseId, true, access.ScopedOrgUnitIds)
	return query.Where("applicant_user_id IN (?)", scopedUserIds)
}

func applyDepartmentUsageScope(params *enterpriseUsageQuery, access service.EnterpriseAccess) error {
	if !access.HasDepartmentScope() {
		return nil
	}
	if params.OrgUnitId > 0 {
		return requireDepartmentOrgUnitInScope(access, params.OrgUnitId)
	}
	params.OrgUnitIds = access.ScopedOrgUnitIds
	return nil
}

func applyProjectUsageScope(params *enterpriseUsageQuery, access service.EnterpriseAccess) error {
	if !access.HasProjectScope() {
		return nil
	}
	if params.ProjectId > 0 {
		if !access.ProjectInScope(params.ProjectId) {
			return scopedEnterpriseError()
		}
		return nil
	}
	if len(access.ScopedProjectIds) == 0 {
		params.ProjectIds = []int{-1}
		return nil
	}
	params.ProjectIds = access.ScopedProjectIds
	return nil
}

func applyEnterpriseAuditScope(query *gorm.DB, enterpriseId int, access service.EnterpriseAccess) *gorm.DB {
	if access.HasDepartmentScope() {
		return applyDepartmentAuditScope(query, enterpriseId, access)
	}
	if access.HasProjectScope() {
		return applyProjectAuditScope(query, enterpriseId, access)
	}
	return query
}

func applyEnterpriseQueueAdmissionScope(query *gorm.DB, enterpriseId int, access service.EnterpriseAccess) *gorm.DB {
	if access.HasDepartmentScope() {
		return applyDepartmentQueueAdmissionScope(query, enterpriseId, access)
	}
	if access.HasProjectScope() {
		return applyProjectQueueAdmissionScope(query, enterpriseId, access)
	}
	return query
}

func applyDepartmentQueueAdmissionScope(query *gorm.DB, enterpriseId int, access service.EnterpriseAccess) *gorm.DB {
	scopedUserIds := model.DB.Model(&model.EnterpriseOrgMembership{}).
		Select("user_id").
		Where("enterprise_id = ? AND is_primary = ? AND org_unit_id IN ?", enterpriseId, true, access.ScopedOrgUnitIds)
	scopedProjectIds := model.DB.Model(&model.EnterpriseProjectOrgUnit{}).
		Select("project_id").
		Where("enterprise_id = ? AND org_unit_id IN ?", enterpriseId, access.ScopedOrgUnitIds)
	scopedPolicyGroupIds := departmentScopedPolicyGroupIds(enterpriseId, access)
	scopedQuotaPolicyIds := model.DB.Model(&model.EnterpriseQuotaPolicy{}).
		Select("id").
		Where(
			"enterprise_id = ? AND ((target_type = ? AND target_id IN (?)) OR (target_type = ? AND target_id IN (?)) OR (target_type = ? AND target_id IN (?)) OR (target_type = ? AND target_id IN (?)))",
			enterpriseId,
			model.PolicyTargetOrgUnit,
			access.ScopedOrgUnitIds,
			model.PolicyTargetUser,
			scopedUserIds,
			model.PolicyTargetPolicyGroup,
			scopedPolicyGroupIds,
			model.PolicyTargetProject,
			scopedProjectIds,
		)
	return query.Where(
		"org_unit_id IN ? OR user_id IN (?) OR project_id IN (?) OR policy_id IN (?)",
		access.ScopedOrgUnitIds,
		scopedUserIds,
		scopedProjectIds,
		scopedQuotaPolicyIds,
	)
}

func applyProjectQueueAdmissionScope(query *gorm.DB, enterpriseId int, access service.EnterpriseAccess) *gorm.DB {
	if len(access.ScopedProjectIds) == 0 {
		return query.Where("1 = 0")
	}
	scopedQuotaPolicyIds := model.DB.Model(&model.EnterpriseQuotaPolicy{}).
		Select("id").
		Where("enterprise_id = ? AND target_type = ? AND target_id IN (?)", enterpriseId, model.PolicyTargetProject, access.ScopedProjectIds)
	return query.Where("project_id IN ? OR policy_id IN (?)", access.ScopedProjectIds, scopedQuotaPolicyIds)
}

func applyDepartmentAuditScope(query *gorm.DB, enterpriseId int, access service.EnterpriseAccess) *gorm.DB {
	scopedUserIds := model.DB.Model(&model.EnterpriseOrgMembership{}).
		Select("user_id").
		Where("enterprise_id = ? AND is_primary = ? AND org_unit_id IN ?", enterpriseId, true, access.ScopedOrgUnitIds)
	scopedPolicyGroupIds := departmentScopedPolicyGroupIds(enterpriseId, access)
	scopedQuotaPolicyIds := model.DB.Model(&model.EnterpriseQuotaPolicy{}).
		Select("id").
		Where(
			"enterprise_id = ? AND ((target_type = ? AND target_id IN (?)) OR (target_type = ? AND target_id IN (?)) OR (target_type = ? AND target_id IN (?)))",
			enterpriseId,
			model.PolicyTargetOrgUnit,
			access.ScopedOrgUnitIds,
			model.PolicyTargetUser,
			scopedUserIds,
			model.PolicyTargetPolicyGroup,
			scopedPolicyGroupIds,
		)
	scopedQuotaRequestIds := model.DB.Model(&model.EnterpriseQuotaRequest{}).
		Select("id").
		Where("enterprise_id = ? AND applicant_user_id IN (?)", enterpriseId, scopedUserIds)
	scopedProjectIds := model.DB.Model(&model.EnterpriseProjectOrgUnit{}).
		Select("project_id").
		Where("enterprise_id = ? AND org_unit_id IN ?", enterpriseId, access.ScopedOrgUnitIds)
	scopedQuotaCounterIds := model.DB.Model(&model.EnterpriseQuotaCounter{}).
		Select("id").
		Where("enterprise_id = ? AND ((target_type = ? AND target_id IN (?)) OR (target_type = ? AND target_id IN (?)) OR (target_type = ? AND target_id IN (?)) OR policy_id IN (?))",
			enterpriseId,
			model.PolicyTargetOrgUnit,
			access.ScopedOrgUnitIds,
			model.PolicyTargetUser,
			scopedUserIds,
			model.PolicyTargetPolicyGroup,
			scopedPolicyGroupIds,
			scopedQuotaPolicyIds,
		)
	return query.Where(
		`scope_org_unit_id IN ? OR scope_user_id IN (?) OR scope_project_id IN (?) OR
		 (target_type = ? AND target_id IN ?) OR
		 (target_type = ? AND target_id IN (?)) OR
		 (target_type = ? AND target_id IN (?)) OR
		 (target_type = ? AND target_id IN (?)) OR
		 (target_type = ? AND target_id IN (?)) OR
		 (target_type = ? AND target_id IN (?)) OR
		 (target_type = ? AND target_id IN (?))`,
		access.ScopedOrgUnitIds,
		scopedUserIds,
		scopedProjectIds,
		"org_unit",
		access.ScopedOrgUnitIds,
		"user",
		scopedUserIds,
		"project",
		scopedProjectIds,
		"policy_group",
		scopedPolicyGroupIds,
		"quota_policy",
		scopedQuotaPolicyIds,
		"quota_request",
		scopedQuotaRequestIds,
		"quota_counter",
		scopedQuotaCounterIds,
	)
}

func applyProjectAuditScope(query *gorm.DB, enterpriseId int, access service.EnterpriseAccess) *gorm.DB {
	if len(access.ScopedProjectIds) == 0 {
		return query.Where("1 = 0")
	}
	scopedQuotaPolicyIds := model.DB.Model(&model.EnterpriseQuotaPolicy{}).
		Select("id").
		Where("enterprise_id = ? AND target_type = ? AND target_id IN (?)", enterpriseId, model.PolicyTargetProject, access.ScopedProjectIds)
	scopedQuotaRequestIds := model.DB.Model(&model.EnterpriseQuotaRequest{}).
		Select("id").
		Where("enterprise_id = ? AND target_type = ? AND target_id IN (?)", enterpriseId, model.PolicyTargetProject, access.ScopedProjectIds)
	scopedQuotaCounterIds := model.DB.Model(&model.EnterpriseQuotaCounter{}).
		Select("id").
		Where("enterprise_id = ? AND ((target_type = ? AND target_id IN (?)) OR policy_id IN (?))",
			enterpriseId,
			model.PolicyTargetProject,
			access.ScopedProjectIds,
			scopedQuotaPolicyIds,
		)
	return query.Where(
		`scope_project_id IN ? OR
		 (target_type = ? AND target_id IN ?) OR
		 (target_type = ? AND target_id IN (?)) OR
		 (target_type = ? AND target_id IN (?)) OR
		 (target_type = ? AND target_id IN (?))`,
		access.ScopedProjectIds,
		"project",
		access.ScopedProjectIds,
		"quota_policy",
		scopedQuotaPolicyIds,
		"quota_request",
		scopedQuotaRequestIds,
		"quota_counter",
		scopedQuotaCounterIds,
	)
}

func parsePathInt(c *gin.Context, name string) (int, error) {
	id, err := strconv.Atoi(c.Param(name))
	if err != nil || id <= 0 {
		return 0, errors.New("无效的 ID")
	}
	return id, nil
}

func parsePathInt64(c *gin.Context, name string) (int64, error) {
	id, err := strconv.ParseInt(c.Param(name), 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New("无效的 ID")
	}
	return id, nil
}

func enterpriseUsageQueryFromRequest(c *gin.Context) (enterpriseUsageQuery, error) {
	startTime, err := parseOptionalInt64Query(c, "start_time")
	if err != nil {
		return enterpriseUsageQuery{}, err
	}
	endTime, err := parseOptionalInt64Query(c, "end_time")
	if err != nil {
		return enterpriseUsageQuery{}, err
	}
	if endTime == 0 {
		endTime = time.Now().Unix()
	}
	if startTime == 0 {
		startTime = endTime - int64(30*24*time.Hour/time.Second)
	}
	if startTime < 0 || endTime < 0 || startTime > endTime {
		return enterpriseUsageQuery{}, errors.New("用量查询时间范围无效")
	}
	if endTime-startTime > int64(366*24*time.Hour/time.Second) {
		return enterpriseUsageQuery{}, errors.New("用量查询时间范围不能超过 366 天")
	}
	userId, err := parseOptionalIntQuery(c, "user_id")
	if err != nil {
		return enterpriseUsageQuery{}, err
	}
	orgUnitId, err := parseOptionalIntQuery(c, "org_unit_id")
	if err != nil {
		return enterpriseUsageQuery{}, err
	}
	policyGroupId, err := parseOptionalIntQuery(c, "policy_group_id")
	if err != nil {
		return enterpriseUsageQuery{}, err
	}
	projectId, err := parseOptionalIntQuery(c, "project_id")
	if err != nil {
		return enterpriseUsageQuery{}, err
	}
	channelId, err := parseOptionalIntQuery(c, "channel_id")
	if err != nil {
		return enterpriseUsageQuery{}, err
	}
	tokenId, err := parseOptionalIntQuery(c, "token_id")
	if err != nil {
		return enterpriseUsageQuery{}, err
	}
	granularity := strings.TrimSpace(c.Query("granularity"))
	if granularity == "" {
		granularity = "day"
	}
	if !isSupportedEnterpriseUsageGranularity(granularity) {
		return enterpriseUsageQuery{}, errors.New("不支持的用量时间粒度")
	}
	return enterpriseUsageQuery{
		StartTime:     startTime,
		EndTime:       endTime,
		UserId:        userId,
		OrgUnitId:     orgUnitId,
		ProjectId:     projectId,
		PolicyGroupId: policyGroupId,
		ChannelId:     channelId,
		TokenId:       tokenId,
		ModelName:     strings.TrimSpace(c.Query("model_name")),
		Status:        strings.TrimSpace(c.Query("status")),
		Granularity:   granularity,
	}, nil
}

func loadEnterpriseUsageRows(enterpriseId int, params enterpriseUsageQuery) ([]enterpriseUsageRow, error) {
	query := model.DB.Model(&model.EnterpriseUsageAttribution{}).
		Select("user_id, token_id, org_unit_id, project_id, policy_group_ids_json, model_name, channel_id, prompt_tokens, completion_tokens, total_tokens, quota, status, created_at").
		Where("enterprise_id = ? AND created_at >= ? AND created_at <= ?", enterpriseId, params.StartTime, params.EndTime)
	if params.UserId > 0 {
		query = query.Where("user_id = ?", params.UserId)
	}
	if params.OrgUnitId > 0 {
		query = query.Where("org_unit_id = ?", params.OrgUnitId)
	} else if len(params.OrgUnitIds) > 0 {
		query = query.Where("org_unit_id IN ?", params.OrgUnitIds)
	}
	if params.ProjectId > 0 {
		query = query.Where("project_id = ?", params.ProjectId)
	} else if len(params.ProjectIds) > 0 {
		query = query.Where("project_id IN ?", params.ProjectIds)
	}
	if params.ModelName != "" {
		query = query.Where("model_name = ?", params.ModelName)
	}
	if params.ChannelId > 0 {
		query = query.Where("channel_id = ?", params.ChannelId)
	}
	if params.TokenId > 0 {
		query = query.Where("token_id = ?", params.TokenId)
	}
	if params.Status != "" {
		query = query.Where("status = ?", params.Status)
	}
	var rows []enterpriseUsageRow
	if err := query.Find(&rows).Error; err != nil {
		return nil, err
	}
	if params.PolicyGroupId <= 0 {
		return rows, nil
	}
	filtered := rows[:0]
	for _, row := range rows {
		if enterprisePolicyGroupIdsContain(row.PolicyGroupIdsJson, params.PolicyGroupId) {
			filtered = append(filtered, row)
		}
	}
	return filtered, nil
}

func aggregateEnterpriseUsageTotal(rows []enterpriseUsageRow) enterpriseUsageTotal {
	var total enterpriseUsageTotal
	for _, row := range rows {
		total.RequestCount++
		total.Quota += int64(row.Quota)
		total.PromptTokens += int64(row.PromptTokens)
		total.CompletionTokens += int64(row.CompletionTokens)
		total.TotalTokens += int64(row.TotalTokens)
	}
	return total
}

func aggregateEnterpriseUsageBreakdown(rows []enterpriseUsageRow, dimension string) []enterpriseUsageBreakdownItem {
	return aggregateEnterpriseUsageBreakdownWithGranularity(rows, dimension, "day")
}

func aggregateEnterpriseUsageBreakdownWithGranularity(rows []enterpriseUsageRow, dimension string, granularity string) []enterpriseUsageBreakdownItem {
	items := map[string]*enterpriseUsageBreakdownItem{}
	for _, row := range rows {
		keys := enterpriseUsageDimensionKeys(row, dimension, granularity)
		for _, key := range keys {
			item := items[key.key]
			if item == nil {
				item = &enterpriseUsageBreakdownItem{
					Dimension:  dimension,
					TargetId:   key.id,
					TargetName: key.name,
					ModelName:  key.modelName,
					Status:     key.status,
					TimeBucket: key.timeBucket,
				}
				items[key.key] = item
			}
			item.RequestCount++
			item.Quota += int64(row.Quota)
			item.PromptTokens += int64(row.PromptTokens)
			item.CompletionTokens += int64(row.CompletionTokens)
			item.TotalTokens += int64(row.TotalTokens)
		}
	}
	result := make([]enterpriseUsageBreakdownItem, 0, len(items))
	for _, item := range items {
		result = append(result, *item)
	}
	sortEnterpriseUsageBreakdown(result, "quota", "desc")
	return result
}

type enterpriseUsageDimensionKey struct {
	key        string
	id         int
	name       string
	modelName  string
	status     string
	timeBucket string
}

func enterpriseUsageDimensionKeys(row enterpriseUsageRow, dimension string, granularity string) []enterpriseUsageDimensionKey {
	switch dimension {
	case "org_unit":
		return []enterpriseUsageDimensionKey{{key: strconv.Itoa(row.OrgUnitId), id: row.OrgUnitId}}
	case "project":
		return []enterpriseUsageDimensionKey{{key: strconv.Itoa(row.ProjectId), id: row.ProjectId}}
	case "policy_group":
		groupIds := parseEnterprisePolicyGroupIds(row.PolicyGroupIdsJson)
		if len(groupIds) == 0 {
			return nil
		}
		keys := make([]enterpriseUsageDimensionKey, 0, len(groupIds))
		for _, groupId := range groupIds {
			keys = append(keys, enterpriseUsageDimensionKey{key: strconv.Itoa(groupId), id: groupId})
		}
		return keys
	case "user":
		return []enterpriseUsageDimensionKey{{key: strconv.Itoa(row.UserId), id: row.UserId}}
	case "channel":
		return []enterpriseUsageDimensionKey{{key: strconv.Itoa(row.ChannelId), id: row.ChannelId}}
	case "api_key":
		return []enterpriseUsageDimensionKey{{key: strconv.Itoa(row.TokenId), id: row.TokenId}}
	case "model":
		name := row.ModelName
		if name == "" {
			name = "unknown"
		}
		return []enterpriseUsageDimensionKey{{key: name, name: name, modelName: name}}
	case "status":
		status := row.Status
		if status == "" {
			status = "unknown"
		}
		return []enterpriseUsageDimensionKey{{key: status, name: status, status: status}}
	case "time":
		bucket := enterpriseUsageTimeBucket(row.CreatedAt, granularity)
		return []enterpriseUsageDimensionKey{{key: bucket, name: bucket, timeBucket: bucket}}
	default:
		return nil
	}
}

func enterpriseUsageTimeBucket(timestamp int64, granularity string) string {
	if timestamp <= 0 {
		return "unknown"
	}
	t := time.Unix(timestamp, 0).UTC()
	if granularity == "month" {
		return t.Format("2006-01")
	}
	return t.Format("2006-01-02")
}

func sortEnterpriseUsageBreakdown(items []enterpriseUsageBreakdownItem, sortBy string, sortOrder string) {
	if sortBy == "" {
		sortBy = "quota"
	}
	desc := sortOrder != "asc"
	sort.SliceStable(items, func(i, j int) bool {
		left := enterpriseUsageSortValue(items[i], sortBy)
		right := enterpriseUsageSortValue(items[j], sortBy)
		if left == right {
			if items[i].TargetName == items[j].TargetName {
				return items[i].TargetId < items[j].TargetId
			}
			return items[i].TargetName < items[j].TargetName
		}
		if desc {
			return left > right
		}
		return left < right
	})
}

func enterpriseUsageSortValue(item enterpriseUsageBreakdownItem, sortBy string) int64 {
	switch sortBy {
	case "request_count":
		return item.RequestCount
	case "prompt_tokens":
		return item.PromptTokens
	case "completion_tokens":
		return item.CompletionTokens
	case "total_tokens", "tokens":
		return item.TotalTokens
	default:
		return item.Quota
	}
}

func fillEnterpriseUsageBreakdownNames(enterpriseId int, dimension string, items []enterpriseUsageBreakdownItem) error {
	switch dimension {
	case "org_unit":
		names, err := enterpriseOrgUnitNames(enterpriseId)
		if err != nil {
			return err
		}
		for i := range items {
			if items[i].TargetId == 0 {
				items[i].TargetName = "未分配部门"
				continue
			}
			items[i].TargetName = names[items[i].TargetId]
		}
	case "project":
		names, err := enterpriseProjectNames(enterpriseId)
		if err != nil {
			return err
		}
		for i := range items {
			if items[i].TargetId == 0 {
				items[i].TargetName = "未分配项目"
				continue
			}
			items[i].TargetName = names[items[i].TargetId]
		}
	case "policy_group":
		names, err := enterprisePolicyGroupNames(enterpriseId)
		if err != nil {
			return err
		}
		for i := range items {
			items[i].TargetName = names[items[i].TargetId]
		}
	case "user":
		names, err := enterpriseUserNames(items)
		if err != nil {
			return err
		}
		for i := range items {
			items[i].TargetName = names[items[i].TargetId]
		}
	case "channel":
		names, err := enterpriseChannelNames(items)
		if err != nil {
			return err
		}
		for i := range items {
			if items[i].TargetId == 0 {
				items[i].TargetName = "未分配渠道"
				continue
			}
			items[i].TargetName = names[items[i].TargetId]
		}
	case "api_key":
		names, err := enterpriseTokenNames(items)
		if err != nil {
			return err
		}
		for i := range items {
			if items[i].TargetId == 0 {
				items[i].TargetName = "未分配 API Key"
				continue
			}
			items[i].TargetName = names[items[i].TargetId]
		}
	}
	return nil
}

func enterpriseUsageBreakdownCSV(items []enterpriseUsageBreakdownItem) ([]byte, error) {
	var buffer bytes.Buffer
	buffer.Write([]byte{0xEF, 0xBB, 0xBF})
	writer := csv.NewWriter(&buffer)
	if err := writer.Write([]string{
		"dimension",
		"target_id",
		"target_name",
		"model_name",
		"status",
		"time_bucket",
		"request_count",
		"quota",
		"prompt_tokens",
		"completion_tokens",
		"total_tokens",
	}); err != nil {
		return nil, err
	}
	for _, item := range items {
		if err := writer.Write([]string{
			item.Dimension,
			strconv.Itoa(item.TargetId),
			item.TargetName,
			item.ModelName,
			item.Status,
			item.TimeBucket,
			strconv.FormatInt(item.RequestCount, 10),
			strconv.FormatInt(item.Quota, 10),
			strconv.FormatInt(item.PromptTokens, 10),
			strconv.FormatInt(item.CompletionTokens, 10),
			strconv.FormatInt(item.TotalTokens, 10),
		}); err != nil {
			return nil, err
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func enterpriseOrgUnitNames(enterpriseId int) (map[int]string, error) {
	var units []model.EnterpriseOrgUnit
	if err := model.DB.Where("enterprise_id = ?", enterpriseId).Find(&units).Error; err != nil {
		return nil, err
	}
	names := map[int]string{}
	for _, unit := range units {
		names[unit.Id] = unit.Name
	}
	return names, nil
}

func enterpriseProjectNames(enterpriseId int) (map[int]string, error) {
	var projects []model.EnterpriseProject
	if err := model.DB.Where("enterprise_id = ?", enterpriseId).Find(&projects).Error; err != nil {
		return nil, err
	}
	names := map[int]string{}
	for _, project := range projects {
		names[project.Id] = project.Name
	}
	return names, nil
}

func enterprisePolicyGroupNames(enterpriseId int) (map[int]string, error) {
	var groups []model.EnterprisePolicyGroup
	if err := model.DB.Where("enterprise_id = ?", enterpriseId).Find(&groups).Error; err != nil {
		return nil, err
	}
	names := map[int]string{}
	for _, group := range groups {
		names[group.Id] = group.Name
	}
	return names, nil
}

func enterpriseUserNames(items []enterpriseUsageBreakdownItem) (map[int]string, error) {
	ids := make([]int, 0, len(items))
	seen := map[int]struct{}{}
	for _, item := range items {
		if item.TargetId <= 0 {
			continue
		}
		if _, ok := seen[item.TargetId]; ok {
			continue
		}
		seen[item.TargetId] = struct{}{}
		ids = append(ids, item.TargetId)
	}
	if len(ids) == 0 {
		return map[int]string{}, nil
	}
	var users []model.User
	if err := model.DB.Select("id, username, display_name").Where("id IN ?", ids).Find(&users).Error; err != nil {
		return nil, err
	}
	names := map[int]string{}
	for _, user := range users {
		name := user.DisplayName
		if name == "" {
			name = user.Username
		}
		names[user.Id] = name
	}
	return names, nil
}

func enterpriseChannelNames(items []enterpriseUsageBreakdownItem) (map[int]string, error) {
	ids := enterpriseUsageTargetIds(items)
	if len(ids) == 0 {
		return map[int]string{}, nil
	}
	var channels []model.Channel
	if err := model.DB.Select("id, name").Where("id IN ?", ids).Find(&channels).Error; err != nil {
		return nil, err
	}
	names := map[int]string{}
	for _, channel := range channels {
		name := channel.Name
		if name == "" {
			name = fmt.Sprintf("Channel #%d", channel.Id)
		}
		names[channel.Id] = name
	}
	return names, nil
}

func enterpriseTokenNames(items []enterpriseUsageBreakdownItem) (map[int]string, error) {
	ids := enterpriseUsageTargetIds(items)
	if len(ids) == 0 {
		return map[int]string{}, nil
	}
	var tokens []model.Token
	if err := model.DB.Select("id, name").Where("id IN ?", ids).Find(&tokens).Error; err != nil {
		return nil, err
	}
	names := map[int]string{}
	for _, token := range tokens {
		name := token.Name
		if name == "" {
			name = fmt.Sprintf("API Key #%d", token.Id)
		}
		names[token.Id] = name
	}
	return names, nil
}

func enterpriseUsageTargetIds(items []enterpriseUsageBreakdownItem) []int {
	ids := make([]int, 0, len(items))
	seen := map[int]struct{}{}
	for _, item := range items {
		if item.TargetId <= 0 {
			continue
		}
		if _, ok := seen[item.TargetId]; ok {
			continue
		}
		seen[item.TargetId] = struct{}{}
		ids = append(ids, item.TargetId)
	}
	return ids
}

func isSupportedEnterpriseUsageDimension(dimension string) bool {
	switch dimension {
	case "org_unit", "project", "policy_group", "user", "model", "status", "channel", "api_key", "time":
		return true
	default:
		return false
	}
}

func isSupportedEnterpriseUsageGranularity(granularity string) bool {
	switch granularity {
	case "day", "month":
		return true
	default:
		return false
	}
}

func enterprisePolicyGroupIdsContain(data string, target int) bool {
	for _, id := range parseEnterprisePolicyGroupIds(data) {
		if id == target {
			return true
		}
	}
	return false
}

func parseEnterprisePolicyGroupIds(data string) []int {
	if strings.TrimSpace(data) == "" {
		return nil
	}
	var ids []int
	if err := common.UnmarshalJsonStr(data, &ids); err != nil {
		return nil
	}
	return ids
}

func normalizeEnterpriseTimezone(timezone string) string {
	timezone = strings.TrimSpace(timezone)
	if timezone == "" {
		return model.DefaultEnterpriseTimezone
	}
	return timezone
}

func validateOrgUnitRequest(req enterpriseOrgUnitRequest) error {
	if strings.TrimSpace(req.Name) == "" {
		return errors.New("部门名称不能为空")
	}
	if strings.TrimSpace(req.Slug) == "" {
		return errors.New("部门标识不能为空")
	}
	return nil
}

func validatePolicyGroupRequest(req enterprisePolicyGroupRequest) error {
	if strings.TrimSpace(req.Name) == "" {
		return errors.New("策略分组名称不能为空")
	}
	if strings.TrimSpace(req.Slug) == "" {
		return errors.New("策略分组标识不能为空")
	}
	return nil
}

func normalizeEnterprisePolicyGroupShareOrgUnitIds(enterpriseId int, access service.EnterpriseAccess, orgUnitIds []int) ([]int, error) {
	if len(orgUnitIds) == 0 {
		return []int{}, nil
	}
	seen := map[int]struct{}{}
	ids := make([]int, 0, len(orgUnitIds))
	for _, orgUnitId := range orgUnitIds {
		if orgUnitId <= 0 {
			return nil, errors.New("共享部门无效")
		}
		if _, ok := seen[orgUnitId]; ok {
			continue
		}
		if access.HasDepartmentScope() && !access.OrgUnitInScope(orgUnitId) {
			return nil, scopedEnterpriseError()
		}
		if err := ensureOrgUnitExists(enterpriseId, orgUnitId); err != nil {
			return nil, err
		}
		seen[orgUnitId] = struct{}{}
		ids = append(ids, orgUnitId)
	}
	sort.Ints(ids)
	return ids, nil
}

func policyGroupOrgUnitFromRequest(enterpriseId int, access service.EnterpriseAccess, orgUnitId *int, currentOrgUnitId int) (int, error) {
	if access.HasDepartmentScope() {
		value := currentOrgUnitId
		if orgUnitId != nil && *orgUnitId > 0 {
			value = *orgUnitId
		}
		if value <= 0 {
			value = access.OrgUnitId
		}
		if err := requireDepartmentOrgUnitInScope(access, value); err != nil {
			return 0, err
		}
		if err := ensureOrgUnitExists(enterpriseId, value); err != nil {
			return 0, err
		}
		return value, nil
	}
	if orgUnitId == nil {
		return currentOrgUnitId, nil
	}
	if *orgUnitId <= 0 {
		return 0, nil
	}
	if err := ensureOrgUnitExists(enterpriseId, *orgUnitId); err != nil {
		return 0, err
	}
	return *orgUnitId, nil
}

func replaceEnterprisePolicyGroupShares(tx *gorm.DB, enterpriseId int, groupId int, orgUnitIds []int) error {
	if err := tx.Where("enterprise_id = ? AND policy_group_id = ?", enterpriseId, groupId).Delete(&model.EnterprisePolicyGroupShare{}).Error; err != nil {
		return err
	}
	for _, orgUnitId := range orgUnitIds {
		share := model.EnterprisePolicyGroupShare{
			EnterpriseId:  enterpriseId,
			PolicyGroupId: groupId,
			OrgUnitId:     orgUnitId,
		}
		if err := tx.Create(&share).Error; err != nil {
			return err
		}
	}
	return nil
}

func enterprisePolicyGroupShareOrgUnitIds(enterpriseId int, groupId int) ([]int, error) {
	var shares []model.EnterprisePolicyGroupShare
	if err := model.DB.Where("enterprise_id = ? AND policy_group_id = ?", enterpriseId, groupId).Order("org_unit_id asc").Find(&shares).Error; err != nil {
		return nil, err
	}
	ids := make([]int, 0, len(shares))
	for _, share := range shares {
		ids = append(ids, share.OrgUnitId)
	}
	return ids, nil
}

func projectFromRequest(enterpriseId int, req enterpriseProjectRequest) (model.EnterpriseProject, []int, error) {
	if strings.TrimSpace(req.Name) == "" {
		return model.EnterpriseProject{}, nil, errors.New("项目名称不能为空")
	}
	if strings.TrimSpace(req.Slug) == "" {
		return model.EnterpriseProject{}, nil, errors.New("项目标识不能为空")
	}
	if req.OwnerUserId > 0 {
		if err := ensureUserExists(req.OwnerUserId); err != nil {
			return model.EnterpriseProject{}, nil, err
		}
	}
	orgUnitIds, err := normalizeEnterpriseProjectOrgUnitIds(enterpriseId, req.OrgUnitIds)
	if err != nil {
		return model.EnterpriseProject{}, nil, err
	}
	status := req.Status
	if status == 0 {
		status = model.EnterpriseProjectStatusEnabled
	}
	return model.EnterpriseProject{
		EnterpriseId: enterpriseId,
		Name:         strings.TrimSpace(req.Name),
		Slug:         strings.TrimSpace(req.Slug),
		Description:  req.Description,
		OwnerUserId:  req.OwnerUserId,
		Status:       status,
	}, orgUnitIds, nil
}

func normalizeEnterpriseProjectOrgUnitIds(enterpriseId int, values []int) ([]int, error) {
	result := make([]int, 0, len(values))
	seen := map[int]struct{}{}
	for _, orgUnitId := range values {
		if orgUnitId <= 0 {
			return nil, errors.New("项目部门 ID 无效")
		}
		if _, ok := seen[orgUnitId]; ok {
			continue
		}
		if err := ensureOrgUnitExists(enterpriseId, orgUnitId); err != nil {
			return nil, err
		}
		seen[orgUnitId] = struct{}{}
		result = append(result, orgUnitId)
	}
	return result, nil
}

func replaceEnterpriseProjectOrgUnits(tx *gorm.DB, enterpriseId int, projectId int, orgUnitIds []int) error {
	if err := tx.Where("enterprise_id = ? AND project_id = ?", enterpriseId, projectId).Delete(&model.EnterpriseProjectOrgUnit{}).Error; err != nil {
		return err
	}
	for _, orgUnitId := range orgUnitIds {
		binding := model.EnterpriseProjectOrgUnit{
			EnterpriseId: enterpriseId,
			ProjectId:    projectId,
			OrgUnitId:    orgUnitId,
		}
		if err := tx.Create(&binding).Error; err != nil {
			return err
		}
	}
	return nil
}

func enterpriseProjectOrgUnitIds(enterpriseId int, projectId int) ([]int, error) {
	var bindings []model.EnterpriseProjectOrgUnit
	if err := model.DB.Where("enterprise_id = ? AND project_id = ?", enterpriseId, projectId).Order("org_unit_id asc").Find(&bindings).Error; err != nil {
		return nil, err
	}
	ids := make([]int, 0, len(bindings))
	for _, binding := range bindings {
		ids = append(ids, binding.OrgUnitId)
	}
	return ids, nil
}

func buildEnterpriseProjectItems(enterpriseId int, projects []model.EnterpriseProject, access service.EnterpriseAccess) ([]enterpriseProjectItem, error) {
	ownerNames, err := enterpriseProjectOwnerNames(projects)
	if err != nil {
		return nil, err
	}
	orgUnitNames, err := enterpriseOrgUnitNames(enterpriseId)
	if err != nil {
		return nil, err
	}
	memberRoles, err := enterpriseProjectMemberRoles(enterpriseId, projects, access.UserId)
	if err != nil {
		return nil, err
	}
	items := make([]enterpriseProjectItem, 0, len(projects))
	for _, project := range projects {
		orgUnitIds, err := enterpriseProjectOrgUnitIds(enterpriseId, project.Id)
		if err != nil {
			return nil, err
		}
		names := make([]string, 0, len(orgUnitIds))
		for _, orgUnitId := range orgUnitIds {
			if name := orgUnitNames[orgUnitId]; name != "" {
				names = append(names, name)
			}
		}
		items = append(items, enterpriseProjectItem{
			EnterpriseProject: project,
			OwnerName:         ownerNames[project.OwnerUserId],
			OrgUnitIds:        orgUnitIds,
			OrgUnitNames:      names,
			MemberRole:        enterpriseProjectMemberRoleForAccess(access, project, memberRoles[project.Id]),
			CanManage:         enterpriseProjectCanManage(access, project.Id),
			MemberCount:       countEnterpriseProjectMembers(project.Id),
			PolicyCount:       countEnterprisePoliciesForTarget(enterpriseId, model.PolicyTargetProject, project.Id),
		})
	}
	return items, nil
}

func buildEnterprisePolicyGroupItems(enterpriseId int, groups []model.EnterprisePolicyGroup, access service.EnterpriseAccess) ([]enterprisePolicyGroupItem, error) {
	orgUnitNames, err := enterpriseOrgUnitNames(enterpriseId)
	if err != nil {
		return nil, err
	}
	items := make([]enterprisePolicyGroupItem, 0, len(groups))
	for _, group := range groups {
		sharedOrgUnitIds, err := enterprisePolicyGroupShareOrgUnitIds(enterpriseId, group.Id)
		if err != nil {
			return nil, err
		}
		sharedOrgUnitNames := make([]string, 0, len(sharedOrgUnitIds))
		for _, orgUnitId := range sharedOrgUnitIds {
			if name := orgUnitNames[orgUnitId]; name != "" {
				sharedOrgUnitNames = append(sharedOrgUnitNames, name)
			}
		}
		items = append(items, enterprisePolicyGroupItem{
			EnterprisePolicyGroup: group,
			SharedOrgUnitIds:      sharedOrgUnitIds,
			SharedOrgUnitNames:    sharedOrgUnitNames,
			CanManage:             enterprisePolicyGroupCanManage(access, group),
			MemberCount:           countEnterprisePolicyGroupMembers(group.Id),
			PolicyCount:           countEnterprisePoliciesForTarget(enterpriseId, model.PolicyTargetPolicyGroup, group.Id),
		})
	}
	return items, nil
}

func enterprisePolicyGroupCanManage(access service.EnterpriseAccess, group model.EnterprisePolicyGroup) bool {
	if access.SystemAdmin || access.Permissions.Manage {
		return true
	}
	if !access.HasDepartmentScope() {
		return false
	}
	return access.OrgUnitInScope(group.OrgUnitId)
}

func enterpriseProjectMemberRoles(enterpriseId int, projects []model.EnterpriseProject, userId int) (map[int]string, error) {
	roles := map[int]string{}
	if enterpriseId <= 0 || userId <= 0 || len(projects) == 0 {
		return roles, nil
	}
	projectIds := make([]int, 0, len(projects))
	for _, project := range projects {
		projectIds = append(projectIds, project.Id)
	}
	var members []model.EnterpriseProjectMember
	if err := model.DB.Select("project_id, role").Where("enterprise_id = ? AND user_id = ? AND project_id IN ?", enterpriseId, userId, projectIds).Find(&members).Error; err != nil {
		return nil, err
	}
	for _, member := range members {
		roles[member.ProjectId] = member.Role
	}
	return roles, nil
}

func enterpriseProjectMemberRoleForAccess(access service.EnterpriseAccess, project model.EnterpriseProject, explicitRole string) string {
	if explicitRole != "" {
		return explicitRole
	}
	if access.UserId > 0 && project.OwnerUserId == access.UserId {
		return model.EnterpriseProjectMemberRoleAdmin
	}
	return ""
}

func enterpriseProjectCanManage(access service.EnterpriseAccess, projectId int) bool {
	if access.SystemAdmin || access.Permissions.Manage {
		return true
	}
	if !access.HasProjectManageScope() {
		return false
	}
	return access.ProjectManageInScope(projectId)
}

func normalizeEnterpriseProjectMemberRole(role string) (string, error) {
	role = strings.ToLower(strings.TrimSpace(role))
	if role == "" {
		role = model.EnterpriseProjectMemberRoleMember
	}
	switch role {
	case model.EnterpriseProjectMemberRoleAdmin, model.EnterpriseProjectMemberRoleMember:
		return role, nil
	default:
		return "", errors.New("项目成员角色无效")
	}
}

func normalizeEnterprisePolicyGroupMemberRole(role string) (string, error) {
	role = normalizeEnterprisePolicyGroupMemberRoleOrDefault(role)
	switch role {
	case model.PolicyGroupMemberRoleEditor, model.PolicyGroupMemberRoleViewer:
		return role, nil
	default:
		return "", errors.New("策略组成员角色无效")
	}
}

func normalizeEnterprisePolicyGroupMemberRoleOrDefault(role string) string {
	role = strings.ToLower(strings.TrimSpace(role))
	if role == "" {
		return model.PolicyGroupMemberRoleViewer
	}
	return role
}

func enterpriseProjectOwnerNames(projects []model.EnterpriseProject) (map[int]string, error) {
	ids := make([]int, 0, len(projects))
	seen := map[int]struct{}{}
	for _, project := range projects {
		if project.OwnerUserId <= 0 {
			continue
		}
		if _, ok := seen[project.OwnerUserId]; ok {
			continue
		}
		seen[project.OwnerUserId] = struct{}{}
		ids = append(ids, project.OwnerUserId)
	}
	if len(ids) == 0 {
		return map[int]string{}, nil
	}
	var users []model.User
	if err := model.DB.Select("id, username, display_name").Where("id IN ?", ids).Find(&users).Error; err != nil {
		return nil, err
	}
	names := map[int]string{}
	for _, user := range users {
		name := user.DisplayName
		if name == "" {
			name = user.Username
		}
		names[user.Id] = name
	}
	return names, nil
}

func resolveOrgUnitParent(tx *gorm.DB, enterpriseId int, parentId int) (string, int, error) {
	if parentId == 0 {
		return "/", 1, nil
	}
	var parent model.EnterpriseOrgUnit
	if err := tx.Where("id = ? AND enterprise_id = ?", parentId, enterpriseId).First(&parent).Error; err != nil {
		return "", 0, err
	}
	return parent.Path, parent.Depth + 1, nil
}

func updateOrgUnitChildrenPath(tx *gorm.DB, enterpriseId int, oldPath string, newPath string, depthDelta int) error {
	if oldPath == "" {
		return nil
	}
	var children []model.EnterpriseOrgUnit
	if err := tx.Where("enterprise_id = ? AND path LIKE ? AND path <> ?", enterpriseId, oldPath+"%", oldPath).Find(&children).Error; err != nil {
		return err
	}
	for _, child := range children {
		child.Path = newPath + strings.TrimPrefix(child.Path, oldPath)
		child.Depth += depthDelta
		if err := tx.Save(&child).Error; err != nil {
			return err
		}
	}
	return nil
}

func ensureUserExists(userId int) error {
	var count int64
	if err := model.DB.Model(&model.User{}).Where("id = ?", userId).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return errors.New("用户不存在")
	}
	return nil
}

func ensureOrgUnitExists(enterpriseId int, orgUnitId int) error {
	var count int64
	if err := model.DB.Model(&model.EnterpriseOrgUnit{}).
		Where("enterprise_id = ? AND id = ? AND status = ?", enterpriseId, orgUnitId, model.OrgUnitStatusEnabled).
		Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return errors.New("部门不存在或已停用")
	}
	return nil
}

func ensureEnterpriseMemberExists(enterpriseId int, userId int) error {
	var count int64
	if err := model.DB.Model(&model.EnterpriseOrgMembership{}).
		Where("enterprise_id = ? AND user_id = ? AND is_primary = ?", enterpriseId, userId, true).
		Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return errors.New("企业成员不存在")
	}
	return nil
}

func ensurePolicyGroupExists(enterpriseId int, groupId int) error {
	var count int64
	if err := model.DB.Model(&model.EnterprisePolicyGroup{}).
		Where("enterprise_id = ? AND id = ? AND status = ?", enterpriseId, groupId, model.PolicyGroupStatusEnabled).
		Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return errors.New("策略分组不存在或已停用")
	}
	return nil
}

func findEnterprisePolicyGroup(enterpriseId int, groupId int) (model.EnterprisePolicyGroup, error) {
	var group model.EnterprisePolicyGroup
	if err := model.DB.Where("enterprise_id = ? AND id = ? AND status = ?", enterpriseId, groupId, model.PolicyGroupStatusEnabled).First(&group).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.EnterprisePolicyGroup{}, errors.New("策略分组不存在或已停用")
		}
		return model.EnterprisePolicyGroup{}, err
	}
	return group, nil
}

func ensureProjectExists(enterpriseId int, projectId int) error {
	var count int64
	if err := model.DB.Model(&model.EnterpriseProject{}).
		Where("enterprise_id = ? AND id = ? AND status = ?", enterpriseId, projectId, model.EnterpriseProjectStatusEnabled).
		Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return errors.New("项目不存在或已停用")
	}
	return nil
}

func fillEnterpriseMemberPolicyGroupCounts(enterpriseId int, items []enterpriseMemberItem) error {
	for i := range items {
		var count int64
		if err := model.DB.Model(&model.EnterprisePolicyGroupMember{}).
			Where("enterprise_id = ? AND user_id = ?", enterpriseId, items[i].UserId).
			Count(&count).Error; err != nil {
			return err
		}
		items[i].PolicyGroupCount = count
	}
	return nil
}

func countEnterprisePolicyGroupMembers(groupId int) int64 {
	var count int64
	_ = model.DB.Model(&model.EnterprisePolicyGroupMember{}).Where("policy_group_id = ?", groupId).Count(&count).Error
	return count
}

func countEnterpriseProjectMembers(projectId int) int64 {
	var count int64
	_ = model.DB.Model(&model.EnterpriseProjectMember{}).Where("project_id = ?", projectId).Count(&count).Error
	return count
}

func countEnterprisePoliciesForTarget(enterpriseId int, targetType string, targetId int) int64 {
	var count int64
	_ = model.DB.Model(&model.EnterpriseQuotaPolicy{}).
		Where("enterprise_id = ? AND target_type = ? AND target_id = ?", enterpriseId, targetType, targetId).
		Count(&count).Error
	return count
}

func quotaPolicyFromRequest(enterpriseId int, req enterpriseQuotaPolicyRequest) (model.EnterpriseQuotaPolicy, error) {
	if strings.TrimSpace(req.Name) == "" {
		return model.EnterpriseQuotaPolicy{}, errors.New("策略名称不能为空")
	}
	if req.LimitValue <= 0 {
		return model.EnterpriseQuotaPolicy{}, errors.New("额度上限必须大于 0")
	}
	if req.TargetType == model.PolicyTargetEnterprise && req.TargetId == 0 {
		req.TargetId = enterpriseId
	}
	if err := validateQuotaPolicyTarget(enterpriseId, req.TargetType, req.TargetId); err != nil {
		return model.EnterpriseQuotaPolicy{}, err
	}
	if req.Metric != model.PolicyMetricRequestCount && req.Metric != model.PolicyMetricQuota {
		return model.EnterpriseQuotaPolicy{}, errors.New("不支持的策略指标")
	}
	if req.Period != model.PolicyPeriodDay && req.Period != model.PolicyPeriodMonth {
		return model.EnterpriseQuotaPolicy{}, errors.New("不支持的策略周期")
	}
	if req.ModelScope == "" {
		req.ModelScope = model.PolicyModelScopeAll
	}
	if req.ModelScope != model.PolicyModelScopeAll && req.ModelScope != model.PolicyModelScopeSpecific {
		return model.EnterpriseQuotaPolicy{}, errors.New("不支持的模型范围")
	}
	models := normalizeStringList(req.Models)
	if req.ModelScope == model.PolicyModelScopeSpecific && len(models) == 0 {
		return model.EnterpriseQuotaPolicy{}, errors.New("指定模型范围不能为空")
	}
	req.Action = strings.TrimSpace(req.Action)
	if req.Action == "" {
		req.Action = model.PolicyActionReject
	}
	if !model.IsEnterpriseQuotaPolicyAction(req.Action) {
		return model.EnterpriseQuotaPolicy{}, errors.New("不支持的策略动作")
	}
	status := req.Status
	if status == 0 {
		status = model.QuotaPolicyStatusEnabled
	}
	modelsJson, err := common.Marshal(models)
	if err != nil {
		return model.EnterpriseQuotaPolicy{}, err
	}
	policy := model.EnterpriseQuotaPolicy{
		EnterpriseId:  enterpriseId,
		Name:          strings.TrimSpace(req.Name),
		Description:   req.Description,
		TargetType:    req.TargetType,
		TargetId:      req.TargetId,
		Metric:        req.Metric,
		Period:        req.Period,
		LimitValue:    req.LimitValue,
		Timezone:      normalizeEnterpriseTimezone(req.Timezone),
		ModelScope:    req.ModelScope,
		ModelsJson:    string(modelsJson),
		Action:        req.Action,
		Priority:      req.Priority,
		Status:        status,
		EffectiveAt:   req.EffectiveAt,
		ExpiresAt:     req.ExpiresAt,
		ConditionMode: strings.TrimSpace(req.ConditionMode),
		ConditionJson: strings.TrimSpace(req.ConditionJson),
		ConditionExpr: strings.TrimSpace(req.ConditionExpr),
	}
	if err := service.NormalizeEnterpriseQuotaPolicyCondition(&policy); err != nil {
		return model.EnterpriseQuotaPolicy{}, err
	}
	return policy, nil
}

func quotaRequestFromSubmitRequest(enterpriseId int, applicantUserId int, req enterpriseQuotaRequestSubmitRequest) (model.EnterpriseQuotaRequest, error) {
	if req.PolicyId <= 0 {
		return model.EnterpriseQuotaRequest{}, errors.New("额度策略不能为空")
	}
	if req.LimitDelta <= 0 {
		return model.EnterpriseQuotaRequest{}, errors.New("临时额度必须大于 0")
	}
	now := common.GetTimestamp()
	if req.ExpiresAt <= now {
		return model.EnterpriseQuotaRequest{}, errors.New("过期时间必须晚于当前时间")
	}
	ctx, err := service.ResolveEnterpriseContextWithProject(applicantUserId, 0, 0, req.ProjectId)
	if err != nil {
		return model.EnterpriseQuotaRequest{}, err
	}
	policy, ok, err := service.IsEnterpriseQuotaPolicyRequestable(ctx, req.PolicyId, time.Now())
	if err != nil {
		return model.EnterpriseQuotaRequest{}, err
	}
	if !ok || policy.EnterpriseId != enterpriseId {
		return model.EnterpriseQuotaRequest{}, errors.New("额度策略不存在、已停用或当前用户不可申请")
	}
	return model.EnterpriseQuotaRequest{
		EnterpriseId:    enterpriseId,
		ApplicantUserId: applicantUserId,
		PolicyId:        policy.Id,
		ProjectId:       req.ProjectId,
		TargetType:      policy.TargetType,
		TargetId:        policy.TargetId,
		Metric:          policy.Metric,
		Period:          policy.Period,
		LimitDelta:      req.LimitDelta,
		Reason:          strings.TrimSpace(req.Reason),
		Status:          model.EnterpriseQuotaRequestStatusPending,
		EffectiveAt:     now,
		ExpiresAt:       req.ExpiresAt,
	}, nil
}

func enterpriseWebhookInputFromRequest(req enterpriseWebhookRequest) service.EnterpriseWebhookUpsertInput {
	return service.EnterpriseWebhookUpsertInput{
		Name:       req.Name,
		Url:        req.Url,
		Secret:     req.Secret,
		EventTypes: req.EventTypes,
		Status:     req.Status,
	}
}

func sanitizeEnterpriseWebhookAuditValue(webhook model.EnterpriseWebhook) gin.H {
	return gin.H{
		"id":               webhook.Id,
		"enterprise_id":    webhook.EnterpriseId,
		"name":             webhook.Name,
		"url":              redactEnterpriseWebhookURL(webhook.Url),
		"has_secret":       strings.TrimSpace(webhook.Secret) != "",
		"event_types_json": webhook.EventTypesJson,
		"status":           webhook.Status,
		"created_at":       webhook.CreatedAt,
		"updated_at":       webhook.UpdatedAt,
	}
}

func sanitizeEnterpriseNotificationPreferenceAuditValue(preference model.EnterpriseNotificationPreference) gin.H {
	if preference.Id == 0 {
		return gin.H{}
	}
	scope, _ := service.EnterpriseNotificationPreferenceToItem(preference)
	return gin.H{
		"id":                   preference.Id,
		"enterprise_id":        preference.EnterpriseId,
		"channel":              preference.Channel,
		"event_type":           preference.EventType,
		"enabled":              preference.Enabled,
		"applicant":            scope.RecipientScope.Applicant,
		"enterprise_admins":    scope.RecipientScope.EnterpriseAdmins,
		"explicit_email_count": len(scope.RecipientScope.ExplicitEmails),
		"created_at":           preference.CreatedAt,
		"updated_at":           preference.UpdatedAt,
	}
}

func redactEnterpriseWebhookURL(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if strings.Contains(trimmed, "?") {
		return strings.SplitN(trimmed, "?", 2)[0] + "?redacted=true"
	}
	return trimmed
}

func decideEnterpriseQuotaRequest(c *gin.Context, status string) {
	id, err := parsePathInt(c, "id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	enterprise, err := currentEnterprise()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var req enterpriseQuotaRequestDecisionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	var quotaRequest model.EnterpriseQuotaRequest
	if err := model.DB.Where("id = ? AND enterprise_id = ?", id, enterprise.Id).First(&quotaRequest).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	access, err := enterpriseAccessForRequest(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := requireDepartmentUserInScope(enterprise.Id, access, quotaRequest.ApplicantUserId); err != nil {
		common.ApiError(c, err)
		return
	}
	if quotaRequest.Status != model.EnterpriseQuotaRequestStatusPending {
		common.ApiError(c, errors.New("只能处理待审批申请"))
		return
	}
	now := common.GetTimestamp()
	if quotaRequest.ExpiresAt <= now {
		before := quotaRequest
		quotaRequest.Status = model.EnterpriseQuotaRequestStatusExpired
		quotaRequest.DecidedAt = now
		if err := model.DB.Transaction(func(tx *gorm.DB) error {
			if err := tx.Save(&quotaRequest).Error; err != nil {
				return err
			}
			audit, err := recordEnterpriseAuditWithDB(tx, c, enterprise.Id, "quota_request.expire", "quota_request", quotaRequest.Id, before, quotaRequest)
			if err != nil {
				return err
			}
			return service.EnqueueEnterpriseQuotaRequestOutboxWithDB(tx, quotaRequest, audit, "quota_request.expire")
		}); err != nil {
			common.ApiError(c, err)
			return
		}
		common.ApiError(c, errors.New("申请已过期"))
		return
	}
	before := quotaRequest
	quotaRequest.Status = status
	quotaRequest.ApproverUserId = c.GetInt("id")
	quotaRequest.DecisionReason = strings.TrimSpace(req.DecisionReason)
	quotaRequest.DecidedAt = now
	if status == model.EnterpriseQuotaRequestStatusApproved {
		quotaRequest.EffectiveAt = now
		if req.ExpiresAt > now {
			quotaRequest.ExpiresAt = req.ExpiresAt
		}
	}
	action := "quota_request.reject"
	if status == model.EnterpriseQuotaRequestStatusApproved {
		action = "quota_request.approve"
	}
	if err := model.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&quotaRequest).Error; err != nil {
			return err
		}
		audit, err := recordEnterpriseAuditWithDB(tx, c, enterprise.Id, action, "quota_request", quotaRequest.Id, before, quotaRequest)
		if err != nil {
			return err
		}
		return service.EnqueueEnterpriseQuotaRequestOutboxWithDB(tx, quotaRequest, audit, action)
	}); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"id": quotaRequest.Id})
}

func buildEnterpriseQuotaRequestItems(enterpriseId int, requests []model.EnterpriseQuotaRequest) ([]enterpriseQuotaRequestItem, error) {
	policyNames, err := enterpriseQuotaRequestPolicyNames(enterpriseId, requests)
	if err != nil {
		return nil, err
	}
	userNames, err := enterpriseQuotaRequestUserNames(requests)
	if err != nil {
		return nil, err
	}
	items := make([]enterpriseQuotaRequestItem, 0, len(requests))
	for _, req := range requests {
		items = append(items, enterpriseQuotaRequestItem{
			EnterpriseQuotaRequest: req,
			PolicyName:             policyNames[req.PolicyId],
			TargetName:             resolveEnterprisePolicyTargetName(enterpriseId, req.TargetType, req.TargetId),
			ApplicantName:          userNames[req.ApplicantUserId],
			ApproverName:           userNames[req.ApproverUserId],
		})
	}
	return items, nil
}

func enterpriseQuotaRequestPolicyNames(enterpriseId int, requests []model.EnterpriseQuotaRequest) (map[int]string, error) {
	ids := make([]int, 0, len(requests))
	seen := map[int]struct{}{}
	for _, req := range requests {
		if req.PolicyId <= 0 {
			continue
		}
		if _, ok := seen[req.PolicyId]; ok {
			continue
		}
		seen[req.PolicyId] = struct{}{}
		ids = append(ids, req.PolicyId)
	}
	if len(ids) == 0 {
		return map[int]string{}, nil
	}
	var policies []model.EnterpriseQuotaPolicy
	if err := model.DB.Select("id, name").Where("enterprise_id = ? AND id IN ?", enterpriseId, ids).Find(&policies).Error; err != nil {
		return nil, err
	}
	names := map[int]string{}
	for _, policy := range policies {
		names[policy.Id] = policy.Name
	}
	return names, nil
}

func enterpriseQuotaRequestUserNames(requests []model.EnterpriseQuotaRequest) (map[int]string, error) {
	ids := make([]int, 0, len(requests)*2)
	seen := map[int]struct{}{}
	for _, req := range requests {
		for _, id := range []int{req.ApplicantUserId, req.ApproverUserId} {
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
	if len(ids) == 0 {
		return map[int]string{}, nil
	}
	var users []model.User
	if err := model.DB.Select("id, username, display_name").Where("id IN ?", ids).Find(&users).Error; err != nil {
		return nil, err
	}
	names := map[int]string{}
	for _, user := range users {
		name := user.DisplayName
		if name == "" {
			name = user.Username
		}
		names[user.Id] = name
	}
	return names, nil
}

func validateQuotaPolicyTarget(enterpriseId int, targetType string, targetId int) error {
	if targetId <= 0 {
		return errors.New("策略目标不能为空")
	}
	switch targetType {
	case model.PolicyTargetEnterprise:
		if targetId != enterpriseId {
			return errors.New("企业策略目标无效")
		}
	case model.PolicyTargetOrgUnit:
		return ensureOrgUnitExists(enterpriseId, targetId)
	case model.PolicyTargetProject:
		return ensureProjectExists(enterpriseId, targetId)
	case model.PolicyTargetPolicyGroup:
		return ensurePolicyGroupExists(enterpriseId, targetId)
	case model.PolicyTargetUser:
		return ensureUserExists(targetId)
	default:
		return errors.New("不支持的策略目标类型")
	}
	return nil
}

func normalizeStringList(values []string) []string {
	result := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func resolveEnterprisePolicyTargetName(enterpriseId int, targetType string, targetId int) string {
	switch targetType {
	case model.PolicyTargetEnterprise:
		enterprise, err := model.GetDefaultEnterprise()
		if err == nil && enterprise.Id == targetId {
			return enterprise.Name
		}
	case model.PolicyTargetOrgUnit:
		var orgUnit model.EnterpriseOrgUnit
		if err := model.DB.Where("enterprise_id = ? AND id = ?", enterpriseId, targetId).First(&orgUnit).Error; err == nil {
			return orgUnit.Name
		}
	case model.PolicyTargetProject:
		var project model.EnterpriseProject
		if err := model.DB.Where("enterprise_id = ? AND id = ?", enterpriseId, targetId).First(&project).Error; err == nil {
			return project.Name
		}
	case model.PolicyTargetPolicyGroup:
		var group model.EnterprisePolicyGroup
		if err := model.DB.Where("enterprise_id = ? AND id = ?", enterpriseId, targetId).First(&group).Error; err == nil {
			return group.Name
		}
	case model.PolicyTargetUser:
		var user model.User
		if err := model.DB.Where("id = ?", targetId).First(&user).Error; err == nil {
			return user.Username
		}
	}
	return ""
}

func sumEnterprisePolicyUsedValue(policyId int) int64 {
	var total int64
	_ = model.DB.Model(&model.EnterpriseQuotaCounter{}).
		Where("policy_id = ?", policyId).
		Select("COALESCE(SUM(used_value), 0)").
		Scan(&total).Error
	return total
}

func recordEnterpriseAudit(c *gin.Context, enterpriseId int, action string, targetType string, targetId int, before any, after any) {
	_ = model.RecordEnterpriseAuditLog(model.EnterpriseAuditInput{
		EnterpriseId: enterpriseId,
		ActorUserId:  c.GetInt("id"),
		Action:       action,
		TargetType:   targetType,
		TargetId:     targetId,
		Before:       before,
		After:        after,
		RequestId:    c.GetHeader(common.RequestIdKey),
	})
}

func recordEnterpriseAuditWithDB(tx *gorm.DB, c *gin.Context, enterpriseId int, action string, targetType string, targetId int, before any, after any) (model.EnterpriseAuditLog, error) {
	return model.RecordEnterpriseAuditLogWithDB(tx, model.EnterpriseAuditInput{
		EnterpriseId: enterpriseId,
		ActorUserId:  c.GetInt("id"),
		Action:       action,
		TargetType:   targetType,
		TargetId:     targetId,
		Before:       before,
		After:        after,
		RequestId:    c.GetHeader(common.RequestIdKey),
	})
}
