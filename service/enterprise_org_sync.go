package service

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

const (
	EnterpriseOrgSyncDefaultProvider = "manual"

	EnterpriseOrgSyncOperationOrgUnitCreate = "create"
	EnterpriseOrgSyncOperationOrgUnitUpdate = "update"
	EnterpriseOrgSyncOperationMemberAssign  = "assign"
	EnterpriseOrgSyncOperationMemberDisable = "disable"
	EnterpriseOrgSyncOperationTokenDisable  = "disable"
	EnterpriseOrgSyncOperationPolicyRemove  = "remove"

	EnterpriseOrgSyncMemberStatusEnabled  = common.UserStatusEnabled
	EnterpriseOrgSyncMemberStatusDisabled = common.UserStatusDisabled
)

type EnterpriseOrgSyncInput struct {
	Provider                 string                          `json:"provider"`
	SnapshotAt               int64                           `json:"snapshot_at"`
	OrgUnits                 []EnterpriseOrgSyncOrgUnitInput `json:"org_units"`
	Members                  []EnterpriseOrgSyncMemberInput  `json:"members"`
	AllowConflicts           bool                            `json:"allow_conflicts"`
	DisableMemberApiKeys     bool                            `json:"disable_member_api_keys"`
	RemoveMemberPolicyGroups bool                            `json:"remove_member_policy_groups"`
	ActorUserId              int                             `json:"-"`
}

type EnterpriseOrgSyncOrgUnitInput struct {
	ExternalId       string `json:"external_id"`
	ParentExternalId string `json:"parent_external_id"`
	Name             string `json:"name"`
	Slug             string `json:"slug"`
	Description      string `json:"description"`
	Sort             int    `json:"sort"`
	Status           int    `json:"status"`
}

type EnterpriseOrgSyncMemberInput struct {
	UserId            int    `json:"user_id"`
	Username          string `json:"username"`
	Email             string `json:"email"`
	ProviderUserId    string `json:"provider_user_id"`
	OrgUnitExternalId string `json:"org_unit_external_id"`
	OrgUnitSlug       string `json:"org_unit_slug"`
	Role              string `json:"role"`
	Status            int    `json:"status"`
}

type EnterpriseOrgSyncResult struct {
	Provider   string                       `json:"provider"`
	SnapshotAt int64                        `json:"snapshot_at"`
	DryRun     bool                         `json:"dry_run"`
	AppliedAt  int64                        `json:"applied_at,omitempty"`
	BatchId    string                       `json:"batch_id,omitempty"`
	RunId      int64                        `json:"run_id,omitempty"`
	Summary    EnterpriseOrgSyncSummary     `json:"summary"`
	Conflicts  []EnterpriseOrgSyncConflict  `json:"conflicts"`
	Operations []EnterpriseOrgSyncOperation `json:"operations"`
}

type EnterpriseOrgSyncSummary struct {
	OrgUnitsTotal            int `json:"org_units_total"`
	MembersTotal             int `json:"members_total"`
	CreateOrgUnits           int `json:"create_org_units"`
	UpdateOrgUnits           int `json:"update_org_units"`
	UnchangedOrgUnits        int `json:"unchanged_org_units"`
	AssignMembers            int `json:"assign_members"`
	DisableMembers           int `json:"disable_members"`
	DisableMemberTokens      int `json:"disable_member_tokens"`
	RemovePolicyGroupMembers int `json:"remove_policy_group_members"`
	UnchangedMembers         int `json:"unchanged_members"`
	Conflicts                int `json:"conflicts"`
}

type EnterpriseOrgSyncConflict struct {
	Type             string `json:"type"`
	ExternalId       string `json:"external_id,omitempty"`
	UserId           int    `json:"user_id,omitempty"`
	Username         string `json:"username,omitempty"`
	Email            string `json:"email,omitempty"`
	ProviderUserId   string `json:"provider_user_id,omitempty"`
	Field            string `json:"field,omitempty"`
	Message          string `json:"message"`
	CandidateUserIds []int  `json:"candidate_user_ids,omitempty"`
}

type EnterpriseOrgSyncOperation struct {
	Type       string         `json:"type"`
	Action     string         `json:"action"`
	ExternalId string         `json:"external_id,omitempty"`
	Slug       string         `json:"slug,omitempty"`
	UserId     int            `json:"user_id,omitempty"`
	TargetId   int            `json:"target_id,omitempty"`
	TargetName string         `json:"target_name,omitempty"`
	Before     map[string]any `json:"before,omitempty"`
	After      map[string]any `json:"after,omitempty"`
}

type EnterpriseOrgSyncRunItem struct {
	Id                 int64                            `json:"id"`
	EnterpriseId       int                              `json:"enterprise_id"`
	BatchId            string                           `json:"batch_id"`
	Provider           string                           `json:"provider"`
	SnapshotAt         int64                            `json:"snapshot_at"`
	Status             string                           `json:"status"`
	Summary            EnterpriseOrgSyncSummary         `json:"summary"`
	OperationsCount    int                              `json:"operations_count"`
	AppliedByUserId    int                              `json:"applied_by_user_id"`
	AppliedAt          int64                            `json:"applied_at"`
	RolledBackByUserId int                              `json:"rolled_back_by_user_id"`
	RolledBackAt       int64                            `json:"rolled_back_at"`
	RollbackSummary    EnterpriseOrgSyncRollbackSummary `json:"rollback_summary"`
	CreatedAt          int64                            `json:"created_at"`
	UpdatedAt          int64                            `json:"updated_at"`
}

type EnterpriseOrgSyncRollbackSummary struct {
	RestoredMembers            int `json:"restored_members"`
	DeletedMembers             int `json:"deleted_members"`
	RestoredTokens             int `json:"restored_tokens"`
	RestoredPolicyGroupMembers int `json:"restored_policy_group_members"`
	SkippedOperations          int `json:"skipped_operations"`
}

type EnterpriseOrgSyncRollbackResult struct {
	Run     EnterpriseOrgSyncRunItem         `json:"run"`
	Summary EnterpriseOrgSyncRollbackSummary `json:"summary"`
}

type enterpriseOrgSyncSnapshot struct {
	Provider                 string
	SnapshotAt               int64
	DisableMemberApiKeys     bool
	RemoveMemberPolicyGroups bool
	OrgUnits                 []enterpriseOrgSyncOrgUnit
	Members                  []enterpriseOrgSyncMember
}

type enterpriseOrgSyncOrgUnit struct {
	ExternalId       string
	ParentExternalId string
	Name             string
	Slug             string
	Description      string
	Sort             int
	Status           int
}

type enterpriseOrgSyncMember struct {
	UserId            int
	Username          string
	Email             string
	ProviderUserId    string
	OrgUnitExternalId string
	OrgUnitSlug       string
	Role              string
	Status            int
}

type enterpriseOrgSyncImporter interface {
	BuildSnapshot(input EnterpriseOrgSyncInput) (enterpriseOrgSyncSnapshot, error)
}

type enterpriseOrgSyncPayloadImporter struct{}

type enterpriseOrgSyncMemberIdentityConflictError struct {
	userIds []int
}

func (err enterpriseOrgSyncMemberIdentityConflictError) Error() string {
	return "同步成员匹配到多个不同用户"
}

type enterpriseOrgSyncPlanner struct {
	db                   *gorm.DB
	enterpriseId         int
	snapshot             enterpriseOrgSyncSnapshot
	result               EnterpriseOrgSyncResult
	existingBySlug       map[string]model.EnterpriseOrgUnit
	existingById         map[int]model.EnterpriseOrgUnit
	syncByExternalId     map[string]enterpriseOrgSyncOrgUnit
	slugByExternalId     map[string]string
	unitIdBySlug         map[string]int
	unitNameBySlug       map[string]string
	skipOrgExternal      map[string]struct{}
	skipMemberIndex      map[int]struct{}
	plannedDisabledUsers map[int]struct{}
}

func PreviewEnterpriseOrgSync(enterpriseId int, input EnterpriseOrgSyncInput) (EnterpriseOrgSyncResult, error) {
	return runEnterpriseOrgSync(model.DB, enterpriseId, input, false)
}

func ApplyEnterpriseOrgSync(enterpriseId int, input EnterpriseOrgSyncInput) (EnterpriseOrgSyncResult, error) {
	return runEnterpriseOrgSync(model.DB, enterpriseId, input, true)
}

func ListEnterpriseOrgSyncRuns(enterpriseId int, offset int, limit int) ([]EnterpriseOrgSyncRunItem, int64, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	query := model.DB.Model(&model.EnterpriseOrgSyncRun{}).Where("enterprise_id = ?", enterpriseId)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var runs []model.EnterpriseOrgSyncRun
	if err := query.Order("created_at desc, id desc").Limit(limit).Offset(offset).Find(&runs).Error; err != nil {
		return nil, 0, err
	}
	items := make([]EnterpriseOrgSyncRunItem, 0, len(runs))
	for _, run := range runs {
		items = append(items, EnterpriseOrgSyncRunToItem(run))
	}
	return items, total, nil
}

func RollbackEnterpriseOrgSyncRun(enterpriseId int, runId int64, actorUserId int) (EnterpriseOrgSyncRollbackResult, error) {
	var result EnterpriseOrgSyncRollbackResult
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		var run model.EnterpriseOrgSyncRun
		if err := tx.Where("enterprise_id = ? AND id = ?", enterpriseId, runId).First(&run).Error; err != nil {
			return err
		}
		if run.Status != model.EnterpriseOrgSyncRunStatusApplied {
			return errors.New("组织同步批次不是可回滚状态")
		}
		operations, err := enterpriseOrgSyncRunOperations(run)
		if err != nil {
			return err
		}
		summary, err := rollbackEnterpriseOrgSyncOperations(tx, enterpriseId, operations)
		if err != nil {
			return err
		}
		rollbackSummaryJson, err := common.Marshal(summary)
		if err != nil {
			return err
		}
		now := time.Now().Unix()
		if err := tx.Model(&run).Updates(map[string]any{
			"status":                 model.EnterpriseOrgSyncRunStatusRolledBack,
			"rolled_back_by_user_id": actorUserId,
			"rolled_back_at":         now,
			"rollback_summary_json":  string(rollbackSummaryJson),
		}).Error; err != nil {
			return err
		}
		run.Status = model.EnterpriseOrgSyncRunStatusRolledBack
		run.RolledBackByUserId = actorUserId
		run.RolledBackAt = now
		run.RollbackSummaryJson = string(rollbackSummaryJson)
		result = EnterpriseOrgSyncRollbackResult{
			Run:     EnterpriseOrgSyncRunToItem(run),
			Summary: summary,
		}
		return nil
	})
	return result, err
}

func runEnterpriseOrgSync(db *gorm.DB, enterpriseId int, input EnterpriseOrgSyncInput, apply bool) (EnterpriseOrgSyncResult, error) {
	snapshot, err := enterpriseOrgSyncPayloadImporter{}.BuildSnapshot(input)
	if err != nil {
		return EnterpriseOrgSyncResult{}, err
	}
	if len(snapshot.OrgUnits) == 0 && len(snapshot.Members) == 0 {
		return EnterpriseOrgSyncResult{}, errors.New("组织同步快照不能为空")
	}
	planner := &enterpriseOrgSyncPlanner{
		db:                   db,
		enterpriseId:         enterpriseId,
		snapshot:             snapshot,
		existingBySlug:       map[string]model.EnterpriseOrgUnit{},
		existingById:         map[int]model.EnterpriseOrgUnit{},
		syncByExternalId:     map[string]enterpriseOrgSyncOrgUnit{},
		slugByExternalId:     map[string]string{},
		unitIdBySlug:         map[string]int{},
		unitNameBySlug:       map[string]string{},
		skipOrgExternal:      map[string]struct{}{},
		skipMemberIndex:      map[int]struct{}{},
		plannedDisabledUsers: map[int]struct{}{},
		result: EnterpriseOrgSyncResult{
			Provider:   snapshot.Provider,
			SnapshotAt: snapshot.SnapshotAt,
			DryRun:     !apply,
			Summary: EnterpriseOrgSyncSummary{
				OrgUnitsTotal: len(snapshot.OrgUnits),
				MembersTotal:  len(snapshot.Members),
			},
		},
	}
	if err := planner.loadExistingOrgUnits(); err != nil {
		return EnterpriseOrgSyncResult{}, err
	}
	planner.validateOrgUnitSnapshot()
	planner.planOrgUnits()
	if err := planner.planMembers(); err != nil {
		return EnterpriseOrgSyncResult{}, err
	}
	planner.result.Summary.Conflicts = len(planner.result.Conflicts)
	if !apply {
		return planner.result, nil
	}
	if len(planner.result.Conflicts) > 0 && !input.AllowConflicts {
		return EnterpriseOrgSyncResult{}, errors.New("组织同步存在冲突，请先预览并处理冲突")
	}
	planner.result.AppliedAt = time.Now().Unix()
	planner.result.BatchId = newEnterpriseOrgSyncBatchId(enterpriseId)
	if err := planner.apply(input.ActorUserId); err != nil {
		return EnterpriseOrgSyncResult{}, err
	}
	return planner.result, nil
}

func (enterpriseOrgSyncPayloadImporter) BuildSnapshot(input EnterpriseOrgSyncInput) (enterpriseOrgSyncSnapshot, error) {
	provider := strings.TrimSpace(input.Provider)
	if provider == "" {
		provider = EnterpriseOrgSyncDefaultProvider
	}
	if len(provider) > 64 {
		return enterpriseOrgSyncSnapshot{}, errors.New("组织同步来源过长")
	}
	snapshotAt := input.SnapshotAt
	if snapshotAt == 0 {
		snapshotAt = time.Now().Unix()
	}
	orgUnits := make([]enterpriseOrgSyncOrgUnit, 0, len(input.OrgUnits))
	for _, unit := range input.OrgUnits {
		externalId := firstNonEmptyString(unit.ExternalId, unit.Slug)
		slug := firstNonEmptyString(unit.Slug, externalId)
		status := unit.Status
		if status == 0 {
			status = model.OrgUnitStatusEnabled
		}
		if status != model.OrgUnitStatusEnabled && status != model.OrgUnitStatusDisabled {
			return enterpriseOrgSyncSnapshot{}, errors.New("部门状态无效")
		}
		orgUnits = append(orgUnits, enterpriseOrgSyncOrgUnit{
			ExternalId:       externalId,
			ParentExternalId: strings.TrimSpace(unit.ParentExternalId),
			Name:             strings.TrimSpace(unit.Name),
			Slug:             slug,
			Description:      strings.TrimSpace(unit.Description),
			Sort:             unit.Sort,
			Status:           status,
		})
	}
	members := make([]enterpriseOrgSyncMember, 0, len(input.Members))
	for _, member := range input.Members {
		status := member.Status
		if status == 0 {
			status = EnterpriseOrgSyncMemberStatusEnabled
		}
		if status != EnterpriseOrgSyncMemberStatusEnabled && status != EnterpriseOrgSyncMemberStatusDisabled {
			return enterpriseOrgSyncSnapshot{}, errors.New("成员状态无效")
		}
		members = append(members, enterpriseOrgSyncMember{
			UserId:            member.UserId,
			Username:          strings.TrimSpace(member.Username),
			Email:             strings.TrimSpace(member.Email),
			ProviderUserId:    strings.TrimSpace(member.ProviderUserId),
			OrgUnitExternalId: strings.TrimSpace(member.OrgUnitExternalId),
			OrgUnitSlug:       strings.TrimSpace(member.OrgUnitSlug),
			Role:              strings.TrimSpace(member.Role),
			Status:            status,
		})
	}
	return enterpriseOrgSyncSnapshot{
		Provider:                 provider,
		SnapshotAt:               snapshotAt,
		DisableMemberApiKeys:     input.DisableMemberApiKeys,
		RemoveMemberPolicyGroups: input.RemoveMemberPolicyGroups,
		OrgUnits:                 orgUnits,
		Members:                  members,
	}, nil
}

func (p *enterpriseOrgSyncPlanner) loadExistingOrgUnits() error {
	var units []model.EnterpriseOrgUnit
	if err := p.db.Where("enterprise_id = ?", p.enterpriseId).Find(&units).Error; err != nil {
		return err
	}
	for _, unit := range units {
		p.existingBySlug[unit.Slug] = unit
		p.existingById[unit.Id] = unit
		p.unitIdBySlug[unit.Slug] = unit.Id
		p.unitNameBySlug[unit.Slug] = unit.Name
	}
	return nil
}

func (p *enterpriseOrgSyncPlanner) validateOrgUnitSnapshot() {
	seenExternalIds := map[string]struct{}{}
	seenSlugs := map[string]string{}
	for _, unit := range p.snapshot.OrgUnits {
		if unit.ExternalId == "" {
			p.addOrgConflict(unit, "external_id", "部门 external_id 或 slug 不能为空")
			continue
		}
		if unit.Name == "" {
			p.addOrgConflict(unit, "name", "部门名称不能为空")
		}
		if unit.Slug == "" {
			p.addOrgConflict(unit, "slug", "部门标识不能为空")
		}
		if _, ok := seenExternalIds[unit.ExternalId]; ok {
			p.addOrgConflict(unit, "external_id", "部门 external_id 重复")
		}
		seenExternalIds[unit.ExternalId] = struct{}{}
		if owner, ok := seenSlugs[unit.Slug]; ok && owner != unit.ExternalId {
			p.addOrgConflict(unit, "slug", "部门 slug 重复")
		}
		seenSlugs[unit.Slug] = unit.ExternalId
		p.syncByExternalId[unit.ExternalId] = unit
		p.slugByExternalId[unit.ExternalId] = unit.Slug
	}
	for _, unit := range p.snapshot.OrgUnits {
		if _, skip := p.skipOrgExternal[unit.ExternalId]; skip {
			continue
		}
		if unit.ParentExternalId == "" {
			continue
		}
		if _, ok := p.syncByExternalId[unit.ParentExternalId]; !ok {
			p.addOrgConflict(unit, "parent_external_id", "父部门不在同步快照中")
		}
	}
	p.detectOrgUnitCycles()
}

func (p *enterpriseOrgSyncPlanner) detectOrgUnitCycles() {
	visiting := map[string]bool{}
	visited := map[string]bool{}
	var visit func(string) bool
	visit = func(externalId string) bool {
		if visited[externalId] {
			return false
		}
		if visiting[externalId] {
			unit := p.syncByExternalId[externalId]
			p.addOrgConflict(unit, "parent_external_id", "部门父级存在循环引用")
			return true
		}
		unit, ok := p.syncByExternalId[externalId]
		if !ok || unit.ParentExternalId == "" {
			visited[externalId] = true
			return false
		}
		visiting[externalId] = true
		cycled := visit(unit.ParentExternalId)
		visiting[externalId] = false
		visited[externalId] = true
		if cycled {
			p.addOrgConflict(unit, "parent_external_id", "部门父级存在循环引用")
		}
		return cycled
	}
	for externalId := range p.syncByExternalId {
		visit(externalId)
	}
}

func (p *enterpriseOrgSyncPlanner) planOrgUnits() {
	for _, unit := range p.sortedOrgUnits() {
		if _, skip := p.skipOrgExternal[unit.ExternalId]; skip {
			continue
		}
		existing, exists := p.existingBySlug[unit.Slug]
		desired := p.orgUnitAuditValue(unit, 0)
		if parentSlug := p.slugByExternalId[unit.ParentExternalId]; parentSlug != "" {
			desired["parent_slug"] = parentSlug
		}
		if !exists {
			p.result.Summary.CreateOrgUnits++
			p.result.Operations = append(p.result.Operations, EnterpriseOrgSyncOperation{
				Type:       "org_unit",
				Action:     EnterpriseOrgSyncOperationOrgUnitCreate,
				ExternalId: unit.ExternalId,
				Slug:       unit.Slug,
				TargetName: unit.Name,
				After:      desired,
			})
			continue
		}
		before := orgUnitAuditValue(existing, p.existingParentSlug(existing))
		after := p.orgUnitAuditValue(unit, existing.Id)
		if parentSlug := p.slugByExternalId[unit.ParentExternalId]; parentSlug != "" {
			after["parent_slug"] = parentSlug
		}
		if orgUnitNeedsUpdate(before, after) {
			p.result.Summary.UpdateOrgUnits++
			p.result.Operations = append(p.result.Operations, EnterpriseOrgSyncOperation{
				Type:       "org_unit",
				Action:     EnterpriseOrgSyncOperationOrgUnitUpdate,
				ExternalId: unit.ExternalId,
				Slug:       unit.Slug,
				TargetId:   existing.Id,
				TargetName: unit.Name,
				Before:     before,
				After:      after,
			})
			continue
		}
		p.result.Summary.UnchangedOrgUnits++
	}
}

func (p *enterpriseOrgSyncPlanner) planMembers() error {
	existingMemberships, err := p.existingMembershipsByUserId()
	if err != nil {
		return err
	}
	for index, member := range p.snapshot.Members {
		user, ok, err := p.resolveSyncMemberUser(member)
		if err != nil {
			var identityConflict enterpriseOrgSyncMemberIdentityConflictError
			if errors.As(err, &identityConflict) {
				p.addMemberIdentityConflict(index, member, identityConflict.userIds)
				continue
			}
			return err
		}
		if !ok {
			p.addMemberConflict(index, member, "user", "同步成员未匹配到用户")
			continue
		}
		if member.Status == EnterpriseOrgSyncMemberStatusDisabled {
			changed, err := p.planDisabledMember(user, existingMemberships[user.Id])
			if err != nil {
				return err
			}
			if !changed {
				p.result.Summary.UnchangedMembers++
			}
			continue
		}
		targetSlug := member.OrgUnitSlug
		if targetSlug == "" {
			targetSlug = p.slugByExternalId[member.OrgUnitExternalId]
		}
		if targetSlug == "" {
			p.addMemberConflict(index, member, "org_unit", "同步成员未指定有效部门")
			continue
		}
		if _, ok := p.skipOrgExternal[member.OrgUnitExternalId]; ok && member.OrgUnitExternalId != "" {
			p.addMemberConflict(index, member, "org_unit", "同步成员目标部门存在冲突")
			continue
		}
		targetId := p.unitIdBySlug[targetSlug]
		targetName := p.unitNameBySlug[targetSlug]
		if targetName == "" {
			if unit, ok := p.syncUnitBySlug(targetSlug); ok {
				targetName = unit.Name
			}
		}
		if targetName == "" {
			p.addMemberConflict(index, member, "org_unit", "同步成员目标部门不存在")
			continue
		}
		before := membershipAuditValue(existingMemberships[user.Id], p.existingById)
		after := map[string]any{
			"user_id":       user.Id,
			"org_unit_id":   targetId,
			"org_unit_slug": targetSlug,
			"org_unit_name": targetName,
			"is_primary":    true,
		}
		if member.Role != "" {
			after["role"] = member.Role
		} else if existing := existingMemberships[user.Id]; existing.Id > 0 {
			after["role"] = existing.Role
		}
		if membershipNeedsUpdate(before, after) {
			p.result.Summary.AssignMembers++
			p.result.Operations = append(p.result.Operations, EnterpriseOrgSyncOperation{
				Type:       "member",
				Action:     EnterpriseOrgSyncOperationMemberAssign,
				ExternalId: member.OrgUnitExternalId,
				Slug:       targetSlug,
				UserId:     user.Id,
				TargetId:   targetId,
				TargetName: targetName,
				Before:     before,
				After:      after,
			})
			continue
		}
		p.result.Summary.UnchangedMembers++
	}
	return nil
}

func (p *enterpriseOrgSyncPlanner) planDisabledMember(user model.User, existing model.EnterpriseOrgMembership) (bool, error) {
	changed := false
	if existing.Id > 0 {
		before := membershipAuditValue(existing, p.existingById)
		after := map[string]any{
			"user_id":            user.Id,
			"status":             EnterpriseOrgSyncMemberStatusDisabled,
			"membership_removed": true,
		}
		p.result.Summary.DisableMembers++
		p.result.Operations = append(p.result.Operations, EnterpriseOrgSyncOperation{
			Type:       "member",
			Action:     EnterpriseOrgSyncOperationMemberDisable,
			Slug:       fmt.Sprint(before["org_unit_slug"]),
			UserId:     user.Id,
			TargetId:   existing.OrgUnitId,
			TargetName: fmt.Sprint(before["org_unit_name"]),
			Before:     before,
			After:      after,
		})
		changed = true
	}
	if _, seen := p.plannedDisabledUsers[user.Id]; seen {
		return changed, nil
	}
	p.plannedDisabledUsers[user.Id] = struct{}{}
	if p.snapshot.DisableMemberApiKeys {
		tokens, err := p.enabledTokensByUserId(user.Id)
		if err != nil {
			return false, err
		}
		if len(tokens) > 0 {
			tokenIds := tokenIdsForOrgSync(tokens)
			p.result.Summary.DisableMemberTokens += len(tokens)
			p.result.Operations = append(p.result.Operations, EnterpriseOrgSyncOperation{
				Type:       "token",
				Action:     EnterpriseOrgSyncOperationTokenDisable,
				UserId:     user.Id,
				TargetName: syncMemberTargetName(user),
				Before: map[string]any{
					"user_id":     user.Id,
					"token_ids":   tokenIds,
					"token_count": len(tokens),
					"status":      common.TokenStatusEnabled,
				},
				After: map[string]any{
					"user_id":     user.Id,
					"token_ids":   tokenIds,
					"token_count": len(tokens),
					"status":      common.TokenStatusDisabled,
				},
			})
			changed = true
		}
	}
	if p.snapshot.RemoveMemberPolicyGroups {
		memberships, err := p.policyGroupMembershipsByUserId(user.Id)
		if err != nil {
			return false, err
		}
		if len(memberships) > 0 {
			groupIds, groupNames, groupMembers := policyGroupValuesForOrgSync(memberships)
			p.result.Summary.RemovePolicyGroupMembers += len(memberships)
			p.result.Operations = append(p.result.Operations, EnterpriseOrgSyncOperation{
				Type:       "policy_group_member",
				Action:     EnterpriseOrgSyncOperationPolicyRemove,
				UserId:     user.Id,
				TargetName: syncMemberTargetName(user),
				Before: map[string]any{
					"user_id":              user.Id,
					"policy_group_ids":     groupIds,
					"policy_group_names":   groupNames,
					"policy_group_members": groupMembers,
					"member_count":         len(memberships),
				},
				After: map[string]any{
					"user_id":         user.Id,
					"member_count":    len(memberships),
					"members_removed": true,
				},
			})
			changed = true
		}
	}
	return changed, nil
}

func (p *enterpriseOrgSyncPlanner) apply(actorUserId int) error {
	return p.db.Transaction(func(tx *gorm.DB) error {
		p.db = tx
		if err := p.applyOrgUnits(); err != nil {
			return err
		}
		if err := p.applyMembers(); err != nil {
			return err
		}
		run, err := p.createRun(actorUserId)
		if err != nil {
			return err
		}
		p.result.RunId = run.Id
		return nil
	})
}

func (p *enterpriseOrgSyncPlanner) createRun(actorUserId int) (model.EnterpriseOrgSyncRun, error) {
	summaryJson, err := common.Marshal(p.result.Summary)
	if err != nil {
		return model.EnterpriseOrgSyncRun{}, err
	}
	operationsJson, err := common.Marshal(p.result.Operations)
	if err != nil {
		return model.EnterpriseOrgSyncRun{}, err
	}
	run := model.EnterpriseOrgSyncRun{
		EnterpriseId:    p.enterpriseId,
		BatchId:         p.result.BatchId,
		Provider:        p.result.Provider,
		SnapshotAt:      p.result.SnapshotAt,
		Status:          model.EnterpriseOrgSyncRunStatusApplied,
		SummaryJson:     string(summaryJson),
		OperationsJson:  string(operationsJson),
		AppliedByUserId: actorUserId,
		AppliedAt:       p.result.AppliedAt,
	}
	if err := p.db.Create(&run).Error; err != nil {
		return model.EnterpriseOrgSyncRun{}, err
	}
	return run, nil
}

func EnterpriseOrgSyncRunToItem(run model.EnterpriseOrgSyncRun) EnterpriseOrgSyncRunItem {
	var summary EnterpriseOrgSyncSummary
	if run.SummaryJson != "" {
		_ = common.Unmarshal([]byte(run.SummaryJson), &summary)
	}
	operations, _ := enterpriseOrgSyncRunOperations(run)
	var rollbackSummary EnterpriseOrgSyncRollbackSummary
	if run.RollbackSummaryJson != "" {
		_ = common.Unmarshal([]byte(run.RollbackSummaryJson), &rollbackSummary)
	}
	return EnterpriseOrgSyncRunItem{
		Id:                 run.Id,
		EnterpriseId:       run.EnterpriseId,
		BatchId:            run.BatchId,
		Provider:           run.Provider,
		SnapshotAt:         run.SnapshotAt,
		Status:             run.Status,
		Summary:            summary,
		OperationsCount:    len(operations),
		AppliedByUserId:    run.AppliedByUserId,
		AppliedAt:          run.AppliedAt,
		RolledBackByUserId: run.RolledBackByUserId,
		RolledBackAt:       run.RolledBackAt,
		RollbackSummary:    rollbackSummary,
		CreatedAt:          run.CreatedAt,
		UpdatedAt:          run.UpdatedAt,
	}
}

func enterpriseOrgSyncRunOperations(run model.EnterpriseOrgSyncRun) ([]EnterpriseOrgSyncOperation, error) {
	if run.OperationsJson == "" {
		return nil, nil
	}
	var operations []EnterpriseOrgSyncOperation
	if err := common.Unmarshal([]byte(run.OperationsJson), &operations); err != nil {
		return nil, err
	}
	return operations, nil
}

func rollbackEnterpriseOrgSyncOperations(tx *gorm.DB, enterpriseId int, operations []EnterpriseOrgSyncOperation) (EnterpriseOrgSyncRollbackSummary, error) {
	var summary EnterpriseOrgSyncRollbackSummary
	for index := len(operations) - 1; index >= 0; index-- {
		operation := operations[index]
		switch {
		case operation.Type == "member" && operation.Action == EnterpriseOrgSyncOperationMemberAssign:
			deleted, restored, err := rollbackEnterpriseOrgSyncMemberAssign(tx, enterpriseId, operation)
			if err != nil {
				return summary, err
			}
			if deleted {
				summary.DeletedMembers++
			}
			if restored {
				summary.RestoredMembers++
			}
		case operation.Type == "member" && operation.Action == EnterpriseOrgSyncOperationMemberDisable:
			restored, err := restoreEnterpriseOrgSyncMembership(tx, enterpriseId, operation.Before)
			if err != nil {
				return summary, err
			}
			if restored {
				summary.RestoredMembers++
			}
		case operation.Type == "token" && operation.Action == EnterpriseOrgSyncOperationTokenDisable:
			count, err := rollbackEnterpriseOrgSyncTokens(tx, operation)
			if err != nil {
				return summary, err
			}
			summary.RestoredTokens += count
		case operation.Type == "policy_group_member" && operation.Action == EnterpriseOrgSyncOperationPolicyRemove:
			count, err := rollbackEnterpriseOrgSyncPolicyGroupMembers(tx, enterpriseId, operation)
			if err != nil {
				return summary, err
			}
			if count == 0 {
				summary.SkippedOperations++
			}
			summary.RestoredPolicyGroupMembers += count
		default:
			summary.SkippedOperations++
		}
	}
	return summary, nil
}

func rollbackEnterpriseOrgSyncMemberAssign(tx *gorm.DB, enterpriseId int, operation EnterpriseOrgSyncOperation) (bool, bool, error) {
	if len(operation.Before) == 0 {
		userId := operation.UserId
		if userId == 0 {
			userId = orgSyncMapInt(operation.After, "user_id")
		}
		if userId == 0 {
			return false, false, nil
		}
		result := tx.Where("enterprise_id = ? AND user_id = ?", enterpriseId, userId).Delete(&model.EnterpriseOrgMembership{})
		return result.RowsAffected > 0, false, result.Error
	}
	restored, err := restoreEnterpriseOrgSyncMembership(tx, enterpriseId, operation.Before)
	return false, restored, err
}

func restoreEnterpriseOrgSyncMembership(tx *gorm.DB, enterpriseId int, value map[string]any) (bool, error) {
	userId := orgSyncMapInt(value, "user_id")
	orgUnitId := orgSyncMapInt(value, "org_unit_id")
	if userId == 0 || orgUnitId == 0 {
		return false, nil
	}
	role := orgSyncMapString(value, "role")
	isPrimary := orgSyncMapBool(value, "is_primary", true)
	var membership model.EnterpriseOrgMembership
	err := tx.Where("enterprise_id = ? AND user_id = ?", enterpriseId, userId).First(&membership).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return false, err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		membership = model.EnterpriseOrgMembership{
			EnterpriseId: enterpriseId,
			UserId:       userId,
			OrgUnitId:    orgUnitId,
			Role:         role,
			IsPrimary:    isPrimary,
		}
		return true, tx.Create(&membership).Error
	}
	membership.OrgUnitId = orgUnitId
	membership.Role = role
	membership.IsPrimary = isPrimary
	return true, tx.Save(&membership).Error
}

func rollbackEnterpriseOrgSyncTokens(tx *gorm.DB, operation EnterpriseOrgSyncOperation) (int, error) {
	tokenIds := orgSyncMapIntSlice(operation.Before, "token_ids")
	if len(tokenIds) == 0 {
		return 0, nil
	}
	status := orgSyncMapInt(operation.Before, "status")
	if status == 0 {
		status = common.TokenStatusEnabled
	}
	result := tx.Model(&model.Token{}).
		Where("id IN ? AND status = ?", tokenIds, common.TokenStatusDisabled).
		Update("status", status)
	return int(result.RowsAffected), result.Error
}

func rollbackEnterpriseOrgSyncPolicyGroupMembers(tx *gorm.DB, enterpriseId int, operation EnterpriseOrgSyncOperation) (int, error) {
	userId := operation.UserId
	if userId == 0 {
		userId = orgSyncMapInt(operation.Before, "user_id")
	}
	if userId == 0 {
		return 0, nil
	}
	values, ok := operation.Before["policy_group_members"].([]any)
	if !ok || len(values) == 0 {
		return 0, nil
	}
	restored := 0
	for _, value := range values {
		memberMap, ok := value.(map[string]any)
		if !ok {
			continue
		}
		groupId := orgSyncMapInt(memberMap, "policy_group_id")
		if groupId == 0 {
			continue
		}
		role := orgSyncMapString(memberMap, "role")
		if role == "" {
			role = model.PolicyGroupMemberRoleViewer
		}
		var member model.EnterprisePolicyGroupMember
		err := tx.Where("enterprise_id = ? AND policy_group_id = ? AND user_id = ?", enterpriseId, groupId, userId).First(&member).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return restored, err
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			member = model.EnterprisePolicyGroupMember{
				EnterpriseId:  enterpriseId,
				PolicyGroupId: groupId,
				UserId:        userId,
				Role:          role,
			}
			if err := tx.Create(&member).Error; err != nil {
				return restored, err
			}
			restored++
			continue
		}
		member.Role = role
		if err := tx.Save(&member).Error; err != nil {
			return restored, err
		}
		restored++
	}
	return restored, nil
}

func (p *enterpriseOrgSyncPlanner) applyOrgUnits() error {
	for _, unit := range p.sortedOrgUnits() {
		if _, skip := p.skipOrgExternal[unit.ExternalId]; skip {
			continue
		}
		parentId, parentPath, depth, err := p.resolveSyncParent(unit)
		if err != nil {
			return err
		}
		existing, exists := p.existingBySlug[unit.Slug]
		if !exists {
			created := model.EnterpriseOrgUnit{
				EnterpriseId: p.enterpriseId,
				ParentId:     parentId,
				Name:         unit.Name,
				Slug:         unit.Slug,
				Description:  unit.Description,
				Path:         "",
				Depth:        depth,
				Sort:         unit.Sort,
				Status:       unit.Status,
			}
			if err := p.db.Create(&created).Error; err != nil {
				return err
			}
			created.Path = fmt.Sprintf("%s%d/", parentPath, created.Id)
			if err := p.db.Save(&created).Error; err != nil {
				return err
			}
			p.rememberOrgUnit(created)
			p.fillOrgUnitOperationTarget(unit.ExternalId, created.Id)
			continue
		}
		if parentId == existing.Id {
			return errors.New("部门不能移动到自身下面")
		}
		if parentId > 0 {
			parent := p.existingById[parentId]
			if strings.Contains(parent.Path, fmt.Sprintf("/%d/", existing.Id)) {
				return errors.New("部门不能移动到自己的子部门下面")
			}
		}
		beforePath := existing.Path
		existing.ParentId = parentId
		existing.Name = unit.Name
		existing.Description = unit.Description
		existing.Depth = depth
		existing.Sort = unit.Sort
		existing.Status = unit.Status
		existing.Path = fmt.Sprintf("%s%d/", parentPath, existing.Id)
		if err := p.db.Save(&existing).Error; err != nil {
			return err
		}
		if beforePath != existing.Path {
			if err := p.updateOrgUnitChildrenPath(beforePath, existing.Path, existing.Depth-p.existingBySlug[unit.Slug].Depth); err != nil {
				return err
			}
		}
		p.rememberOrgUnit(existing)
		p.fillOrgUnitOperationTarget(unit.ExternalId, existing.Id)
	}
	return nil
}

func (p *enterpriseOrgSyncPlanner) applyMembers() error {
	appliedDisabledUsers := map[int]struct{}{}
	for index, member := range p.snapshot.Members {
		if _, skip := p.skipMemberIndex[index]; skip {
			continue
		}
		user, ok, err := p.resolveSyncMemberUser(member)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		if member.Status == EnterpriseOrgSyncMemberStatusDisabled {
			if _, seen := appliedDisabledUsers[user.Id]; seen {
				continue
			}
			appliedDisabledUsers[user.Id] = struct{}{}
			if err := p.applyDisabledMember(user.Id); err != nil {
				return err
			}
			continue
		}
		targetSlug := member.OrgUnitSlug
		if targetSlug == "" {
			targetSlug = p.slugByExternalId[member.OrgUnitExternalId]
		}
		targetId := p.unitIdBySlug[targetSlug]
		if targetId == 0 {
			continue
		}
		var membership model.EnterpriseOrgMembership
		err = p.db.Where("enterprise_id = ? AND user_id = ?", p.enterpriseId, user.Id).First(&membership).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			membership = model.EnterpriseOrgMembership{
				EnterpriseId: p.enterpriseId,
				UserId:       user.Id,
				OrgUnitId:    targetId,
				Role:         member.Role,
				IsPrimary:    true,
			}
			if err := p.db.Create(&membership).Error; err != nil {
				return err
			}
		} else {
			membership.OrgUnitId = targetId
			if member.Role != "" {
				membership.Role = member.Role
			}
			membership.IsPrimary = true
			if err := p.db.Save(&membership).Error; err != nil {
				return err
			}
		}
		p.fillMemberOperationTarget(user.Id, targetId, targetSlug)
	}
	return nil
}

func (p *enterpriseOrgSyncPlanner) applyDisabledMember(userId int) error {
	var membership model.EnterpriseOrgMembership
	err := p.db.Where("enterprise_id = ? AND user_id = ?", p.enterpriseId, userId).First(&membership).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if err == nil {
		if err := p.db.Delete(&membership).Error; err != nil {
			return err
		}
	}
	if p.snapshot.DisableMemberApiKeys {
		if err := p.db.Model(&model.Token{}).
			Where("user_id = ? AND status = ?", userId, common.TokenStatusEnabled).
			Update("status", common.TokenStatusDisabled).Error; err != nil {
			return err
		}
	}
	if p.snapshot.RemoveMemberPolicyGroups {
		if err := p.db.Where("enterprise_id = ? AND user_id = ?", p.enterpriseId, userId).
			Delete(&model.EnterprisePolicyGroupMember{}).Error; err != nil {
			return err
		}
	}
	return nil
}

func (p *enterpriseOrgSyncPlanner) sortedOrgUnits() []enterpriseOrgSyncOrgUnit {
	units := make([]enterpriseOrgSyncOrgUnit, 0, len(p.snapshot.OrgUnits))
	units = append(units, p.snapshot.OrgUnits...)
	sort.SliceStable(units, func(i, j int) bool {
		leftDepth := p.snapshotOrgUnitDepth(units[i].ExternalId, map[string]bool{})
		rightDepth := p.snapshotOrgUnitDepth(units[j].ExternalId, map[string]bool{})
		if leftDepth == rightDepth {
			return units[i].Slug < units[j].Slug
		}
		return leftDepth < rightDepth
	})
	return units
}

func (p *enterpriseOrgSyncPlanner) snapshotOrgUnitDepth(externalId string, seen map[string]bool) int {
	if seen[externalId] {
		return 0
	}
	seen[externalId] = true
	unit, ok := p.syncByExternalId[externalId]
	if !ok || unit.ParentExternalId == "" {
		return 1
	}
	return p.snapshotOrgUnitDepth(unit.ParentExternalId, seen) + 1
}

func (p *enterpriseOrgSyncPlanner) resolveSyncParent(unit enterpriseOrgSyncOrgUnit) (int, string, int, error) {
	if unit.ParentExternalId == "" {
		return 0, "/", 1, nil
	}
	parentSlug := p.slugByExternalId[unit.ParentExternalId]
	parentId := p.unitIdBySlug[parentSlug]
	if parentId == 0 {
		return 0, "", 0, errors.New("父部门不存在")
	}
	parent := p.existingById[parentId]
	return parent.Id, parent.Path, parent.Depth + 1, nil
}

func (p *enterpriseOrgSyncPlanner) updateOrgUnitChildrenPath(oldPath string, newPath string, depthDelta int) error {
	if oldPath == "" {
		return nil
	}
	var children []model.EnterpriseOrgUnit
	if err := p.db.Where("enterprise_id = ? AND path LIKE ? AND path <> ?", p.enterpriseId, oldPath+"%", oldPath).Find(&children).Error; err != nil {
		return err
	}
	for _, child := range children {
		child.Path = newPath + strings.TrimPrefix(child.Path, oldPath)
		child.Depth += depthDelta
		if err := p.db.Save(&child).Error; err != nil {
			return err
		}
		p.rememberOrgUnit(child)
	}
	return nil
}

func (p *enterpriseOrgSyncPlanner) existingMembershipsByUserId() (map[int]model.EnterpriseOrgMembership, error) {
	var memberships []model.EnterpriseOrgMembership
	if err := p.db.Where("enterprise_id = ?", p.enterpriseId).Find(&memberships).Error; err != nil {
		return nil, err
	}
	result := map[int]model.EnterpriseOrgMembership{}
	for _, membership := range memberships {
		result[membership.UserId] = membership
	}
	return result, nil
}

type enterpriseOrgSyncPolicyGroupMembership struct {
	PolicyGroupId   int
	PolicyGroupName string
	Role            string
}

func (p *enterpriseOrgSyncPlanner) enabledTokensByUserId(userId int) ([]model.Token, error) {
	var tokens []model.Token
	err := p.db.
		Select("id, user_id, name, status").
		Where("user_id = ? AND status = ?", userId, common.TokenStatusEnabled).
		Order("id asc").
		Find(&tokens).Error
	return tokens, err
}

func (p *enterpriseOrgSyncPlanner) policyGroupMembershipsByUserId(userId int) ([]enterpriseOrgSyncPolicyGroupMembership, error) {
	var memberships []enterpriseOrgSyncPolicyGroupMembership
	err := p.db.Table("enterprise_policy_group_members AS pgm").
		Select("pgm.policy_group_id, pg.name AS policy_group_name, pgm.role").
		Joins("LEFT JOIN enterprise_policy_groups AS pg ON pg.id = pgm.policy_group_id").
		Where("pgm.enterprise_id = ? AND pgm.user_id = ?", p.enterpriseId, userId).
		Order("pgm.policy_group_id asc").
		Find(&memberships).Error
	return memberships, err
}

func (p *enterpriseOrgSyncPlanner) resolveSyncMemberUser(member enterpriseOrgSyncMember) (model.User, bool, error) {
	if member.UserId > 0 {
		var user model.User
		err := p.db.Where("id = ?", member.UserId).First(&user).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.User{}, false, nil
		}
		if err != nil {
			return model.User{}, false, err
		}
		return user, true, nil
	}
	candidates := map[int]model.User{}
	match := func(query string, args ...any) error {
		var user model.User
		err := p.db.Where(query, args...).First(&user).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		candidates[user.Id] = user
		return nil
	}
	if member.ProviderUserId != "" {
		column, ok := providerUserIdColumn(p.snapshot.Provider)
		if ok {
			if err := match(column+" = ?", member.ProviderUserId); err != nil {
				return model.User{}, false, err
			}
		}
	}
	if member.Email != "" {
		if err := match("email = ?", member.Email); err != nil {
			return model.User{}, false, err
		}
	}
	if member.Username != "" {
		if err := match("username = ?", member.Username); err != nil {
			return model.User{}, false, err
		}
	}
	if len(candidates) == 0 {
		return model.User{}, false, nil
	}
	if len(candidates) > 1 {
		userIds := make([]int, 0, len(candidates))
		for userId := range candidates {
			userIds = append(userIds, userId)
		}
		sort.Ints(userIds)
		return model.User{}, false, enterpriseOrgSyncMemberIdentityConflictError{userIds: userIds}
	}
	for _, user := range candidates {
		return user, true, nil
	}
	return model.User{}, false, nil
}

func providerUserIdColumn(provider string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "github":
		return "github_id", true
	case "discord":
		return "discord_id", true
	case "oidc":
		return "oidc_id", true
	case "hstation":
		return "h_station_id", true
	case "wechat":
		return "wechat_id", true
	case "telegram":
		return "telegram_id", true
	case "linuxdo", "linux_do":
		return "linux_do_id", true
	default:
		return "", false
	}
}

func (p *enterpriseOrgSyncPlanner) syncUnitBySlug(slug string) (enterpriseOrgSyncOrgUnit, bool) {
	for _, unit := range p.snapshot.OrgUnits {
		if unit.Slug == slug {
			return unit, true
		}
	}
	return enterpriseOrgSyncOrgUnit{}, false
}

func (p *enterpriseOrgSyncPlanner) existingParentSlug(unit model.EnterpriseOrgUnit) string {
	if unit.ParentId == 0 {
		return ""
	}
	parent := p.existingById[unit.ParentId]
	return parent.Slug
}

func (p *enterpriseOrgSyncPlanner) orgUnitAuditValue(unit enterpriseOrgSyncOrgUnit, id int) map[string]any {
	value := map[string]any{
		"id":          id,
		"name":        unit.Name,
		"slug":        unit.Slug,
		"description": unit.Description,
		"sort":        unit.Sort,
		"status":      unit.Status,
	}
	if unit.ParentExternalId == "" {
		value["parent_slug"] = ""
	}
	return value
}

func orgUnitAuditValue(unit model.EnterpriseOrgUnit, parentSlug string) map[string]any {
	return map[string]any{
		"id":          unit.Id,
		"name":        unit.Name,
		"slug":        unit.Slug,
		"description": unit.Description,
		"sort":        unit.Sort,
		"status":      unit.Status,
		"parent_slug": parentSlug,
	}
}

func orgUnitNeedsUpdate(before map[string]any, after map[string]any) bool {
	for _, key := range []string{"name", "description", "sort", "status", "parent_slug"} {
		if fmt.Sprint(before[key]) != fmt.Sprint(after[key]) {
			return true
		}
	}
	return false
}

func membershipAuditValue(membership model.EnterpriseOrgMembership, unitsById map[int]model.EnterpriseOrgUnit) map[string]any {
	if membership.Id == 0 {
		return map[string]any{}
	}
	unit := unitsById[membership.OrgUnitId]
	return map[string]any{
		"user_id":       membership.UserId,
		"org_unit_id":   membership.OrgUnitId,
		"org_unit_slug": unit.Slug,
		"org_unit_name": unit.Name,
		"role":          membership.Role,
		"is_primary":    membership.IsPrimary,
	}
}

func membershipNeedsUpdate(before map[string]any, after map[string]any) bool {
	if len(before) == 0 {
		return true
	}
	for _, key := range []string{"org_unit_id", "org_unit_slug", "role", "is_primary"} {
		if fmt.Sprint(before[key]) != fmt.Sprint(after[key]) {
			return true
		}
	}
	return false
}

func syncMemberTargetName(user model.User) string {
	return firstNonEmptyString(user.DisplayName, user.Username, fmt.Sprintf("#%d", user.Id))
}

func newEnterpriseOrgSyncBatchId(enterpriseId int) string {
	return fmt.Sprintf("orgsync-%d-%d", enterpriseId, time.Now().UnixNano())
}

func tokenIdsForOrgSync(tokens []model.Token) []int {
	ids := make([]int, 0, len(tokens))
	for _, token := range tokens {
		ids = append(ids, token.Id)
	}
	return ids
}

func policyGroupValuesForOrgSync(memberships []enterpriseOrgSyncPolicyGroupMembership) ([]int, []string, []map[string]any) {
	ids := make([]int, 0, len(memberships))
	names := make([]string, 0, len(memberships))
	memberValues := make([]map[string]any, 0, len(memberships))
	for _, membership := range memberships {
		ids = append(ids, membership.PolicyGroupId)
		if membership.PolicyGroupName != "" {
			names = append(names, membership.PolicyGroupName)
		}
		memberValues = append(memberValues, map[string]any{
			"policy_group_id":   membership.PolicyGroupId,
			"policy_group_name": membership.PolicyGroupName,
			"role":              membership.Role,
		})
	}
	return ids, names, memberValues
}

func orgSyncMapInt(value map[string]any, key string) int {
	if value == nil {
		return 0
	}
	switch typed := value[key].(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case string:
		parsed, _ := strconv.Atoi(typed)
		return parsed
	default:
		return 0
	}
}

func orgSyncMapString(value map[string]any, key string) string {
	if value == nil {
		return ""
	}
	switch typed := value[key].(type) {
	case string:
		return typed
	default:
		return fmt.Sprint(typed)
	}
}

func orgSyncMapBool(value map[string]any, key string, fallback bool) bool {
	if value == nil {
		return fallback
	}
	switch typed := value[key].(type) {
	case bool:
		return typed
	default:
		return fallback
	}
}

func orgSyncMapIntSlice(value map[string]any, key string) []int {
	if value == nil {
		return nil
	}
	raw, ok := value[key].([]any)
	if !ok {
		if typed, ok := value[key].([]int); ok {
			return typed
		}
		return nil
	}
	result := make([]int, 0, len(raw))
	for _, item := range raw {
		switch typed := item.(type) {
		case int:
			result = append(result, typed)
		case int64:
			result = append(result, int(typed))
		case float64:
			result = append(result, int(typed))
		case string:
			if parsed, err := strconv.Atoi(typed); err == nil {
				result = append(result, parsed)
			}
		}
	}
	return result
}

func (p *enterpriseOrgSyncPlanner) rememberOrgUnit(unit model.EnterpriseOrgUnit) {
	p.existingBySlug[unit.Slug] = unit
	p.existingById[unit.Id] = unit
	p.unitIdBySlug[unit.Slug] = unit.Id
	p.unitNameBySlug[unit.Slug] = unit.Name
}

func (p *enterpriseOrgSyncPlanner) fillOrgUnitOperationTarget(externalId string, targetId int) {
	for index := range p.result.Operations {
		if p.result.Operations[index].Type == "org_unit" && p.result.Operations[index].ExternalId == externalId {
			p.result.Operations[index].TargetId = targetId
			if p.result.Operations[index].After != nil {
				p.result.Operations[index].After["id"] = targetId
			}
		}
	}
}

func (p *enterpriseOrgSyncPlanner) fillMemberOperationTarget(userId int, targetId int, targetSlug string) {
	targetName := p.unitNameBySlug[targetSlug]
	for index := range p.result.Operations {
		if p.result.Operations[index].Type == "member" && p.result.Operations[index].UserId == userId {
			p.result.Operations[index].TargetId = targetId
			p.result.Operations[index].TargetName = targetName
			if p.result.Operations[index].After != nil {
				p.result.Operations[index].After["org_unit_id"] = targetId
				p.result.Operations[index].After["org_unit_name"] = targetName
			}
		}
	}
}

func (p *enterpriseOrgSyncPlanner) addOrgConflict(unit enterpriseOrgSyncOrgUnit, field string, message string) {
	p.skipOrgExternal[unit.ExternalId] = struct{}{}
	p.result.Conflicts = append(p.result.Conflicts, EnterpriseOrgSyncConflict{
		Type:       "org_unit",
		ExternalId: unit.ExternalId,
		Field:      field,
		Message:    message,
	})
}

func (p *enterpriseOrgSyncPlanner) addMemberConflict(index int, member enterpriseOrgSyncMember, field string, message string) {
	p.skipMemberIndex[index] = struct{}{}
	p.result.Conflicts = append(p.result.Conflicts, EnterpriseOrgSyncConflict{
		Type:           "member",
		ExternalId:     member.OrgUnitExternalId,
		UserId:         member.UserId,
		Username:       member.Username,
		Email:          member.Email,
		ProviderUserId: member.ProviderUserId,
		Field:          field,
		Message:        message,
	})
}

func (p *enterpriseOrgSyncPlanner) addMemberIdentityConflict(index int, member enterpriseOrgSyncMember, candidateUserIds []int) {
	p.skipMemberIndex[index] = struct{}{}
	p.result.Conflicts = append(p.result.Conflicts, EnterpriseOrgSyncConflict{
		Type:             "member",
		ExternalId:       member.OrgUnitExternalId,
		UserId:           member.UserId,
		Username:         member.Username,
		Email:            member.Email,
		ProviderUserId:   member.ProviderUserId,
		Field:            "user",
		Message:          "同步成员的 provider ID、邮箱或用户名匹配到多个用户，请确认后指定 user_id",
		CandidateUserIds: candidateUserIds,
	})
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
