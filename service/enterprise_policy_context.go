package service

import (
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

type EnterpriseContext struct {
	Enabled          bool
	DryRun           bool
	UserId           int
	TokenId          int
	EnterpriseId     int
	PrimaryOrgUnitId int
	OrgUnitIds       []int
	ProjectId        int
	PolicyGroupIds   []int
	RuntimeGroup     string
	Role             string
}

type EnterpriseProjectContextError struct {
	Message string
}

func (err EnterpriseProjectContextError) Error() string {
	return err.Message
}

func ResolveEnterpriseContext(userId int, tokenId int) (*EnterpriseContext, error) {
	return ResolveEnterpriseContextWithProject(userId, tokenId, 0, 0)
}

func ResolveEnterpriseContextWithProject(userId int, tokenId int, tokenDefaultProjectId int, requestedProjectId int) (*EnterpriseContext, error) {
	ctx := &EnterpriseContext{
		Enabled: common.EnterpriseGovernanceEnabled,
		DryRun:  common.EnterpriseGovernanceDryRunEnabled,
		UserId:  userId,
		TokenId: tokenId,
	}
	if !ctx.Enabled {
		return ctx, nil
	}
	enterprise, err := model.GetDefaultEnterprise()
	if err != nil {
		if err := model.EnsureDefaultEnterprise(); err != nil {
			return nil, err
		}
		enterprise, err = model.GetDefaultEnterprise()
		if err != nil {
			return nil, err
		}
	}
	ctx.EnterpriseId = enterprise.Id

	var user model.User
	if err := model.DB.Select("id, role, `group`").Where("id = ?", userId).First(&user).Error; err != nil {
		return nil, err
	}
	ctx.RuntimeGroup = user.Group
	ctx.Role = enterpriseRoleName(user.Role)

	var membership model.EnterpriseOrgMembership
	if err := model.DB.Where("enterprise_id = ? AND user_id = ? AND is_primary = ?", enterprise.Id, userId, true).
		First(&membership).Error; err == nil && membership.OrgUnitId > 0 {
		ctx.PrimaryOrgUnitId = membership.OrgUnitId
		ctx.OrgUnitIds = []int{membership.OrgUnitId}
		var orgUnit model.EnterpriseOrgUnit
		if err := model.DB.Where("enterprise_id = ? AND id = ? AND status = ?", enterprise.Id, membership.OrgUnitId, model.OrgUnitStatusEnabled).
			First(&orgUnit).Error; err == nil {
			ctx.OrgUnitIds = parseEnterpriseOrgUnitPath(orgUnit.Path)
		}
	}
	if ctx.OrgUnitIds == nil {
		ctx.OrgUnitIds = []int{}
	}

	projectId := tokenDefaultProjectId
	if requestedProjectId > 0 {
		projectId = requestedProjectId
	}
	if projectId > 0 {
		if err := validateEnterpriseProjectForContext(enterprise.Id, projectId, ctx.OrgUnitIds); err != nil {
			return nil, err
		}
		ctx.ProjectId = projectId
	}

	var groupIds []int
	if err := model.DB.Table("enterprise_policy_group_members AS pgm").
		Select("pgm.policy_group_id").
		Joins("JOIN enterprise_policy_groups pg ON pg.id = pgm.policy_group_id AND pg.enterprise_id = pgm.enterprise_id").
		Where("pgm.enterprise_id = ? AND pgm.user_id = ? AND pg.status = ?", enterprise.Id, userId, model.PolicyGroupStatusEnabled).
		Order("pgm.policy_group_id asc").
		Pluck("pgm.policy_group_id", &groupIds).Error; err != nil {
		return nil, err
	}
	ctx.PolicyGroupIds = groupIds
	return ctx, nil
}

func validateEnterpriseProjectForContext(enterpriseId int, projectId int, orgUnitIds []int) error {
	var project model.EnterpriseProject
	if err := model.DB.Where("enterprise_id = ? AND id = ? AND status = ?", enterpriseId, projectId, model.EnterpriseProjectStatusEnabled).
		First(&project).Error; err != nil {
		return EnterpriseProjectContextError{Message: "请求项目不存在或已停用"}
	}
	if len(orgUnitIds) == 0 {
		return EnterpriseProjectContextError{Message: "当前用户没有可用部门，不能使用项目"}
	}
	var count int64
	if err := model.DB.Model(&model.EnterpriseProjectOrgUnit{}).
		Where("enterprise_id = ? AND project_id = ? AND org_unit_id IN ?", enterpriseId, projectId, orgUnitIds).
		Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return EnterpriseProjectContextError{Message: "当前用户不能使用请求项目"}
	}
	return nil
}

func parseEnterpriseOrgUnitPath(path string) []int {
	parts := strings.Split(path, "/")
	ids := make([]int, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}
		id, err := strconv.Atoi(part)
		if err == nil && id > 0 {
			ids = append(ids, id)
		}
	}
	return ids
}

func enterpriseRoleName(role int) string {
	switch role {
	case common.RoleRootUser:
		return "root"
	case common.RoleAdminUser:
		return "admin"
	default:
		return "user"
	}
}
