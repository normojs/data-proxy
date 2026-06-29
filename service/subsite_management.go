package service

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

type managedSubsiteUsageRow struct {
	SubsiteId int64
	Calls     int64
	Quota     int64
}

type managedSubsiteActivityStatsRow struct {
	Calls            int64
	ErrorCalls       int64
	PromptTokens     int64
	CompletionTokens int64
	Quota            int64
	LastRequestAt    int64
}

type managedSubsiteOwnerRow struct {
	SubsiteId int64
	UserId    int
	Username  string
}

type managedSubsiteMemberCountRow struct {
	SubsiteId int64
	Count     int64
}

type managedSubsiteMemberRow struct {
	Id          int64
	SubsiteId   int64
	UserId      int
	Username    string
	DisplayName string
	Email       string
	UserStatus  int
	Role        string
	Status      string
	JoinedAt    int64
	CreatedAt   int64
	UpdatedAt   int64
}

func ListManagedSubsites(userId int, role int, startIdx int, limit int, now int64) ([]dto.ManagedSubsite, int64, error) {
	if now <= 0 {
		now = common.GetTimestamp()
	}
	query := model.DB.Model(&model.Subsite{})
	if role < common.RoleAdminUser {
		query = query.
			Joins("JOIN subsite_members sm ON sm.subsite_id = subsites.id").
			Where("sm.user_id = ? AND sm.status = ? AND sm.role IN ?", userId, model.SubsiteMemberStatusActive, []string{model.SubsiteMemberRoleOwner, model.SubsiteMemberRoleAdmin})
	}

	var total int64
	if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var subsites []model.Subsite
	if err := query.
		Order("subsites.updated_at DESC, subsites.id DESC").
		Limit(limit).
		Offset(startIdx).
		Find(&subsites).Error; err != nil {
		return nil, 0, err
	}
	items, err := BuildManagedSubsiteItems(subsites, userId, role, now)
	return items, total, err
}

func GetManagedSubsite(subsiteId int64, userId int, role int, now int64) (dto.ManagedSubsite, error) {
	var subsite model.Subsite
	if err := model.DB.First(&subsite, "id = ?", subsiteId).Error; err != nil {
		return dto.ManagedSubsite{}, err
	}
	if role < common.RoleAdminUser {
		member, err := model.GetSubsiteMember(subsiteId, userId)
		if err != nil {
			return dto.ManagedSubsite{}, err
		}
		if !member.CanManage() {
			return dto.ManagedSubsite{}, gorm.ErrRecordNotFound
		}
	}
	items, err := BuildManagedSubsiteItems([]model.Subsite{subsite}, userId, role, now)
	if err != nil {
		return dto.ManagedSubsite{}, err
	}
	if len(items) == 0 {
		return dto.ManagedSubsite{}, gorm.ErrRecordNotFound
	}
	return items[0], nil
}

func GetManagedSubsiteActivity(subsiteId int64, now int64) (dto.ManagedSubsiteActivity, error) {
	if now <= 0 {
		now = common.GetTimestamp()
	}
	if model.LOG_DB == nil {
		return dto.ManagedSubsiteActivity{}, nil
	}
	start := now - 24*60*60
	var stats managedSubsiteActivityStatsRow
	if err := model.LOG_DB.Model(&model.Log{}).
		Select("COUNT(*) AS calls, COALESCE(SUM(CASE WHEN type = ? THEN 1 ELSE 0 END), 0) AS error_calls, COALESCE(SUM(prompt_tokens), 0) AS prompt_tokens, COALESCE(SUM(completion_tokens), 0) AS completion_tokens, COALESCE(SUM(quota), 0) AS quota, COALESCE(MAX(created_at), 0) AS last_request_at", model.LogTypeError).
		Where("subsite_id = ? AND created_at >= ? AND type IN ?", subsiteId, start, []int{model.LogTypeConsume, model.LogTypeError}).
		Scan(&stats).Error; err != nil {
		return dto.ManagedSubsiteActivity{}, err
	}
	var logs []model.Log
	if err := model.LOG_DB.
		Where("subsite_id = ? AND type IN ?", subsiteId, []int{model.LogTypeConsume, model.LogTypeError}).
		Order("created_at DESC, id DESC").
		Limit(20).
		Find(&logs).Error; err != nil {
		return dto.ManagedSubsiteActivity{}, err
	}
	recentLogs := make([]dto.SubsiteRecentLog, 0, len(logs))
	for _, log := range logs {
		recentLogs = append(recentLogs, subsiteRecentLogFromModel(log))
	}
	return dto.ManagedSubsiteActivity{
		Stats24h: dto.SubsiteUsageStats{
			WindowSeconds: 24 * 60 * 60,
			Calls:         stats.Calls,
			PromptTokens:  stats.PromptTokens,
			OutputTokens:  stats.CompletionTokens,
			TotalTokens:   stats.PromptTokens + stats.CompletionTokens,
			Quota:         stats.Quota,
			LastRequestAt: stats.LastRequestAt,
		},
		ErrorCalls24h: stats.ErrorCalls,
		RecentLogs:    recentLogs,
	}, nil
}

func ListManagedSubsiteChannels(subsiteId int64) ([]dto.ManagedSubsiteChannelInfo, error) {
	channels := make([]model.Channel, 0)
	if err := model.DB.
		Where("subsite_id = ?", subsiteId).
		Order("priority DESC, id DESC").
		Find(&channels).Error; err != nil {
		return nil, err
	}
	items := make([]dto.ManagedSubsiteChannelInfo, 0, len(channels))
	for _, channel := range channels {
		items = append(items, managedSubsiteChannelInfoFromModel(channel))
	}
	return items, nil
}

func CreateManagedSubsiteChannel(subsiteId int64, input dto.ManagedSubsiteChannelUpsertRequest) (dto.ManagedSubsiteChannelInfo, error) {
	channel, err := managedSubsiteChannelFromInput(subsiteId, input, nil)
	if err != nil {
		return dto.ManagedSubsiteChannelInfo{}, err
	}
	if channel.Key == "" {
		return dto.ManagedSubsiteChannelInfo{}, errors.New("channel key is required")
	}
	channel.CreatedTime = common.GetTimestamp()
	if err := channel.Insert(); err != nil {
		return dto.ManagedSubsiteChannelInfo{}, err
	}
	ResetProxyClientCache()
	model.InitChannelCache()
	return managedSubsiteChannelInfoFromModel(*channel), nil
}

func UpdateManagedSubsiteChannel(subsiteId int64, channelId int, input dto.ManagedSubsiteChannelUpsertRequest) (dto.ManagedSubsiteChannelInfo, error) {
	if channelId <= 0 {
		return dto.ManagedSubsiteChannelInfo{}, errors.New("channel id is required")
	}
	existing, err := model.GetChannelById(channelId, true)
	if err != nil {
		return dto.ManagedSubsiteChannelInfo{}, err
	}
	if existing.SubsiteId != subsiteId {
		return dto.ManagedSubsiteChannelInfo{}, gorm.ErrRecordNotFound
	}
	channel, err := managedSubsiteChannelFromInput(subsiteId, input, existing)
	if err != nil {
		return dto.ManagedSubsiteChannelInfo{}, err
	}
	channel.Id = channelId
	channel.CreatedTime = existing.CreatedTime
	channel.TestTime = existing.TestTime
	channel.ResponseTime = existing.ResponseTime
	channel.UsedQuota = existing.UsedQuota
	channel.Balance = existing.Balance
	channel.BalanceUpdatedTime = existing.BalanceUpdatedTime
	channel.ChannelInfo = existing.ChannelInfo
	if strings.TrimSpace(input.Key) == "" {
		channel.Key = ""
	}
	if err := channel.Update(); err != nil {
		return dto.ManagedSubsiteChannelInfo{}, err
	}
	ResetProxyClientCache()
	model.InitChannelCache()
	updated, err := model.GetChannelById(channelId, true)
	if err != nil {
		return dto.ManagedSubsiteChannelInfo{}, err
	}
	return managedSubsiteChannelInfoFromModel(*updated), nil
}

func DeleteManagedSubsiteChannel(subsiteId int64, channelId int) error {
	if channelId <= 0 {
		return errors.New("channel id is required")
	}
	channel, err := model.GetChannelById(channelId, true)
	if err != nil {
		return err
	}
	if channel.SubsiteId != subsiteId {
		return gorm.ErrRecordNotFound
	}
	if err := channel.Delete(); err != nil {
		return err
	}
	ResetProxyClientCache()
	model.InitChannelCache()
	return nil
}

func managedSubsiteChannelFromInput(subsiteId int64, input dto.ManagedSubsiteChannelUpsertRequest, existing *model.Channel) (*model.Channel, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, errors.New("channel name is required")
	}
	if input.Type <= 0 {
		return nil, errors.New("channel type is required")
	}
	models, err := normalizeSubsiteChannelCSV(input.Models, true)
	if err != nil {
		return nil, err
	}
	group, err := normalizeSubsiteChannelCSV(input.Group, false)
	if err != nil {
		return nil, err
	}
	status := input.Status
	if status == 0 {
		status = common.ChannelStatusEnabled
	}
	if status != common.ChannelStatusEnabled && status != common.ChannelStatusManuallyDisabled {
		return nil, errors.New("unsupported channel status")
	}
	priority := input.Priority
	weight := input.Weight
	baseURL := strings.TrimSpace(input.BaseURL)
	remark := strings.TrimSpace(input.Remark)
	channel := &model.Channel{
		SubsiteId: subsiteId,
		Type:      input.Type,
		Key:       strings.TrimSpace(input.Key),
		Status:    status,
		Name:      name,
		Models:    models,
		Group:     group,
		Priority:  &priority,
		Weight:    &weight,
	}
	if existing != nil {
		channel.OpenAIOrganization = existing.OpenAIOrganization
		channel.TestModel = existing.TestModel
		channel.Other = existing.Other
		channel.ModelMapping = existing.ModelMapping
		channel.StatusCodeMapping = existing.StatusCodeMapping
		channel.AutoBan = existing.AutoBan
		channel.Tag = existing.Tag
		channel.Setting = existing.Setting
		channel.ParamOverride = existing.ParamOverride
		channel.HeaderOverride = existing.HeaderOverride
		channel.OtherSettings = existing.OtherSettings
	}
	settings := channel.GetOtherSettings()
	settings.SubsiteModelDisplayNames = normalizeSubsiteModelDisplayNames(input.ModelDisplayNames, models)
	channel.SetOtherSettings(settings)
	if baseURL != "" {
		channel.BaseURL = &baseURL
	}
	if remark != "" {
		channel.Remark = &remark
	}
	return channel, nil
}

func normalizeSubsiteChannelCSV(value string, requireValue bool) (string, error) {
	parts := strings.Split(value, ",")
	seen := map[string]struct{}{}
	cleaned := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item == "" {
			continue
		}
		if len(item) > 255 {
			return "", errors.New("model or group name is too long")
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		cleaned = append(cleaned, item)
	}
	if len(cleaned) == 0 {
		if requireValue {
			return "", errors.New("models are required")
		}
		return "default", nil
	}
	return strings.Join(cleaned, ","), nil
}

func normalizeSubsiteModelDisplayNames(values map[string]string, models string) map[string]string {
	allowed := map[string]struct{}{}
	for _, modelName := range strings.Split(models, ",") {
		modelName = strings.TrimSpace(modelName)
		if modelName != "" {
			allowed[modelName] = struct{}{}
		}
	}
	if len(allowed) == 0 || len(values) == 0 {
		return nil
	}
	cleaned := make(map[string]string, len(values))
	for modelName, displayName := range values {
		modelName = strings.TrimSpace(modelName)
		displayName = strings.TrimSpace(displayName)
		if modelName == "" || displayName == "" {
			continue
		}
		if len(modelName) > 255 || len(displayName) > 255 {
			continue
		}
		if _, ok := allowed[modelName]; !ok {
			continue
		}
		cleaned[modelName] = displayName
	}
	if len(cleaned) == 0 {
		return nil
	}
	return cleaned
}

func FilterSubsiteModelDisplayNames(channel *model.Channel) {
	if channel == nil {
		return
	}
	settings := channel.GetOtherSettings()
	settings.SubsiteModelDisplayNames = normalizeSubsiteModelDisplayNames(settings.SubsiteModelDisplayNames, channel.Models)
	channel.SetOtherSettings(settings)
}

func managedSubsiteChannelInfoFromModel(channel model.Channel) dto.ManagedSubsiteChannelInfo {
	baseURL := ""
	if channel.BaseURL != nil {
		baseURL = *channel.BaseURL
	}
	priority := int64(0)
	if channel.Priority != nil {
		priority = *channel.Priority
	}
	weight := uint(0)
	if channel.Weight != nil {
		weight = *channel.Weight
	}
	remark := ""
	if channel.Remark != nil {
		remark = *channel.Remark
	}
	settings := channel.GetOtherSettings()
	displayNames := normalizeSubsiteModelDisplayNames(settings.SubsiteModelDisplayNames, channel.Models)
	if displayNames == nil {
		displayNames = map[string]string{}
	}
	return dto.ManagedSubsiteChannelInfo{
		Id:                channel.Id,
		SubsiteId:         channel.SubsiteId,
		Name:              channel.Name,
		Type:              channel.Type,
		Status:            channel.Status,
		Models:            channel.Models,
		Group:             channel.Group,
		BaseURL:           baseURL,
		Priority:          priority,
		Weight:            weight,
		CreatedTime:       channel.CreatedTime,
		TestTime:          channel.TestTime,
		ResponseTime:      channel.ResponseTime,
		UsedQuota:         channel.UsedQuota,
		Balance:           channel.Balance,
		Remark:            remark,
		HasKey:            strings.TrimSpace(channel.Key) != "",
		ModelDisplayNames: displayNames,
	}
}

func ManagedSubsiteChannelInfoFromModel(channel model.Channel) dto.ManagedSubsiteChannelInfo {
	return managedSubsiteChannelInfoFromModel(channel)
}

func BuildManagedSubsiteItems(subsites []model.Subsite, userId int, role int, now int64) ([]dto.ManagedSubsite, error) {
	if now <= 0 {
		now = common.GetTimestamp()
	}
	if len(subsites) == 0 {
		return []dto.ManagedSubsite{}, nil
	}
	subsiteIds := make([]int64, 0, len(subsites))
	for _, subsite := range subsites {
		subsiteIds = append(subsiteIds, subsite.Id)
	}

	ownerUserIdsBySubsite, ownerUsernamesBySubsite, err := managedSubsiteOwners(subsiteIds)
	if err != nil {
		return nil, err
	}
	memberCountBySubsite, err := managedSubsiteMemberCounts(subsiteIds)
	if err != nil {
		return nil, err
	}
	usageBySubsite, err := managedSubsiteTodayUsage(subsiteIds, now)
	if err != nil {
		return nil, err
	}
	roleBySubsite, err := managedSubsiteRolesForUser(subsiteIds, userId, role)
	if err != nil {
		return nil, err
	}
	policyBySubsite, err := managedSubsiteQuotaPolicies(subsiteIds)
	if err != nil {
		return nil, err
	}

	items := make([]dto.ManagedSubsite, 0, len(subsites))
	for i := range subsites {
		subsite := subsites[i]
		item := dto.ManagedSubsite{
			Subsite:        PublicSubsiteFromModel(&subsite, now),
			Role:           roleBySubsite[subsite.Id],
			CanManage:      role >= common.RoleAdminUser || model.SubsiteRoleCanManage(roleBySubsite[subsite.Id]),
			OwnerUserIds:   ownerUserIdsBySubsite[subsite.Id],
			OwnerUsernames: ownerUsernamesBySubsite[subsite.Id],
			MemberCount:    memberCountBySubsite[subsite.Id],
			TodayCalls:     usageBySubsite[subsite.Id].Calls,
			TodayQuota:     usageBySubsite[subsite.Id].Quota,
		}
		if policy, ok := policyBySubsite[subsite.Id]; ok {
			info := SubsiteQuotaPolicyInfoFromModel(policy)
			item.QuotaPolicy = &info
		}
		items = append(items, item)
	}
	return items, nil
}

func CreateManagedSubsite(input dto.ManagedSubsiteCreateRequest, actorUserId int) (*model.Subsite, error) {
	ownerUserId := input.OwnerUserId
	if ownerUserId <= 0 {
		ownerUserId = actorUserId
	}
	var created model.Subsite
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		if ownerUserId > 0 {
			var owner model.User
			if err := tx.Select("id").First(&owner, "id = ?", ownerUserId).Error; err != nil {
				return err
			}
		}
		created = model.Subsite{
			Slug:                 input.Slug,
			Name:                 input.Name,
			Title:                input.Title,
			LogoURL:              input.LogoURL,
			FaviconURL:           input.FaviconURL,
			ThemeColor:           input.ThemeColor,
			Status:               input.Status,
			DisabledReason:       input.DisabledReason,
			AnnouncementIcon:     input.AnnouncementIcon,
			AnnouncementTitle:    input.AnnouncementTitle,
			AnnouncementBody:     input.AnnouncementBody,
			AnnouncementURL:      input.AnnouncementURL,
			ContactURL:           input.ContactURL,
			RegistrationPolicy:   input.RegistrationPolicy,
			InviteCode:           input.InviteCode,
			EmailDomainWhitelist: input.EmailDomainWhitelist,
			StartsAt:             input.StartsAt,
			EndsAt:               input.EndsAt,
			CreatedBy:            actorUserId,
		}
		if err := tx.Create(&created).Error; err != nil {
			return err
		}
		if ownerUserId > 0 {
			member := model.SubsiteMember{
				SubsiteId: created.Id,
				UserId:    ownerUserId,
				Role:      model.SubsiteMemberRoleOwner,
				Status:    model.SubsiteMemberStatusActive,
			}
			if err := tx.Create(&member).Error; err != nil {
				return err
			}
		}
		if input.QuotaPolicy != nil {
			policy := SubsiteQuotaPolicyFromInfo(created.Id, *input.QuotaPolicy)
			if err := tx.Create(&policy).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &created, nil
}

func UpdateManagedSubsite(subsiteId int64, input dto.ManagedSubsiteUpdateRequest) (*model.Subsite, bool, error) {
	var subsite model.Subsite
	if err := model.DB.First(&subsite, "id = ?", subsiteId).Error; err != nil {
		return nil, false, err
	}
	wasEnabled := subsite.RuntimeStatus(common.GetTimestamp()) == model.SubsiteRuntimeStatusEnabled

	if input.Slug != nil {
		subsite.Slug = *input.Slug
	}
	if input.Name != nil {
		subsite.Name = *input.Name
	}
	if input.Title != nil {
		subsite.Title = *input.Title
	}
	if input.LogoURL != nil {
		subsite.LogoURL = *input.LogoURL
	}
	if input.FaviconURL != nil {
		subsite.FaviconURL = *input.FaviconURL
	}
	if input.ThemeColor != nil {
		subsite.ThemeColor = *input.ThemeColor
	}
	if input.Status != nil {
		subsite.Status = *input.Status
	}
	if input.DisabledReason != nil {
		subsite.DisabledReason = *input.DisabledReason
	}
	if input.AnnouncementIcon != nil {
		subsite.AnnouncementIcon = *input.AnnouncementIcon
	}
	if input.AnnouncementTitle != nil {
		subsite.AnnouncementTitle = *input.AnnouncementTitle
	}
	if input.AnnouncementBody != nil {
		subsite.AnnouncementBody = *input.AnnouncementBody
	}
	if input.AnnouncementURL != nil {
		subsite.AnnouncementURL = *input.AnnouncementURL
	}
	if input.ContactURL != nil {
		subsite.ContactURL = *input.ContactURL
	}
	if input.RegistrationPolicy != nil {
		subsite.RegistrationPolicy = *input.RegistrationPolicy
	}
	if input.InviteCode != nil {
		subsite.InviteCode = *input.InviteCode
	}
	if input.EmailDomainWhitelist != nil {
		subsite.EmailDomainWhitelist = *input.EmailDomainWhitelist
	}
	if input.StartsAt != nil {
		subsite.StartsAt = *input.StartsAt
	}
	if input.EndsAt != nil {
		subsite.EndsAt = *input.EndsAt
	}

	if err := model.DB.Save(&subsite).Error; err != nil {
		return nil, false, err
	}
	disabledNow := wasEnabled && subsite.RuntimeStatus(common.GetTimestamp()) == model.SubsiteRuntimeStatusDisabled
	return &subsite, disabledNow, nil
}

func UpsertManagedSubsiteQuotaPolicy(subsiteId int64, input dto.SubsiteQuotaPolicyInfo) (model.SubsiteQuotaPolicy, error) {
	var policy model.SubsiteQuotaPolicy
	err := model.DB.Where("subsite_id = ?", subsiteId).First(&policy).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return model.SubsiteQuotaPolicy{}, err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		policy.SubsiteId = subsiteId
	}
	applySubsiteQuotaPolicyInfo(&policy, input)
	if policy.Id == 0 {
		err = model.DB.Create(&policy).Error
	} else {
		err = model.DB.Save(&policy).Error
	}
	return policy, err
}

func ListManagedSubsiteMembers(subsiteId int64) ([]dto.ManagedSubsiteMemberInfo, error) {
	rows := make([]managedSubsiteMemberRow, 0)
	if err := model.DB.Table("subsite_members").
		Select("subsite_members.id, subsite_members.subsite_id, subsite_members.user_id, users.username, users.display_name, users.email, users.status AS user_status, subsite_members.role, subsite_members.status, subsite_members.joined_at, subsite_members.created_at, subsite_members.updated_at").
		Joins("LEFT JOIN users ON users.id = subsite_members.user_id").
		Where("subsite_members.subsite_id = ?", subsiteId).
		Order("CASE subsite_members.role WHEN 'owner' THEN 0 WHEN 'admin' THEN 1 ELSE 2 END, subsite_members.id ASC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]dto.ManagedSubsiteMemberInfo, 0, len(rows))
	for _, row := range rows {
		items = append(items, managedSubsiteMemberInfoFromRow(row))
	}
	return items, nil
}

func UpsertManagedSubsiteMember(subsiteId int64, input dto.ManagedSubsiteMemberUpsertRequest) (dto.ManagedSubsiteMemberInfo, error) {
	if input.UserId <= 0 {
		return dto.ManagedSubsiteMemberInfo{}, errors.New("user id is required")
	}
	nextRole := model.NormalizeSubsiteMemberRole(input.Role)
	nextStatus := model.NormalizeSubsiteMemberStatus(input.Status)

	err := model.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Select("id").First(&model.Subsite{}, "id = ?", subsiteId).Error; err != nil {
			return err
		}
		if err := tx.Select("id").First(&model.User{}, "id = ?", input.UserId).Error; err != nil {
			return err
		}

		var member model.SubsiteMember
		err := tx.Where("subsite_id = ? AND user_id = ?", subsiteId, input.UserId).First(&member).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			member = model.SubsiteMember{
				SubsiteId: subsiteId,
				UserId:    input.UserId,
				Role:      nextRole,
				Status:    nextStatus,
			}
			return tx.Create(&member).Error
		}

		if err := ensureSubsiteKeepsActiveOwner(tx, member, nextRole, nextStatus); err != nil {
			return err
		}
		member.Role = nextRole
		member.Status = nextStatus
		return tx.Save(&member).Error
	})
	if err != nil {
		return dto.ManagedSubsiteMemberInfo{}, err
	}
	return GetManagedSubsiteMember(subsiteId, input.UserId)
}

func DeleteManagedSubsiteMember(subsiteId int64, userId int) error {
	if userId <= 0 {
		return errors.New("user id is required")
	}
	return model.DB.Transaction(func(tx *gorm.DB) error {
		var member model.SubsiteMember
		if err := tx.Where("subsite_id = ? AND user_id = ?", subsiteId, userId).First(&member).Error; err != nil {
			return err
		}
		if err := ensureSubsiteKeepsActiveOwner(tx, member, model.SubsiteMemberRoleMember, model.SubsiteMemberStatusDisabled); err != nil {
			return err
		}
		return tx.Delete(&member).Error
	})
}

func GetManagedSubsiteMember(subsiteId int64, userId int) (dto.ManagedSubsiteMemberInfo, error) {
	var row managedSubsiteMemberRow
	if err := model.DB.Table("subsite_members").
		Select("subsite_members.id, subsite_members.subsite_id, subsite_members.user_id, users.username, users.display_name, users.email, users.status AS user_status, subsite_members.role, subsite_members.status, subsite_members.joined_at, subsite_members.created_at, subsite_members.updated_at").
		Joins("LEFT JOIN users ON users.id = subsite_members.user_id").
		Where("subsite_members.subsite_id = ? AND subsite_members.user_id = ?", subsiteId, userId).
		Scan(&row).Error; err != nil {
		return dto.ManagedSubsiteMemberInfo{}, err
	}
	if row.Id == 0 {
		return dto.ManagedSubsiteMemberInfo{}, gorm.ErrRecordNotFound
	}
	return managedSubsiteMemberInfoFromRow(row), nil
}

func managedSubsiteMemberInfoFromRow(row managedSubsiteMemberRow) dto.ManagedSubsiteMemberInfo {
	member := model.SubsiteMember{
		Id:        row.Id,
		SubsiteId: row.SubsiteId,
		UserId:    row.UserId,
		Role:      row.Role,
		Status:    row.Status,
		JoinedAt:  row.JoinedAt,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}
	return dto.ManagedSubsiteMemberInfo{
		Id:          row.Id,
		SubsiteId:   row.SubsiteId,
		UserId:      row.UserId,
		Username:    row.Username,
		DisplayName: row.DisplayName,
		Email:       row.Email,
		UserStatus:  row.UserStatus,
		Role:        model.NormalizeSubsiteMemberRole(row.Role),
		Status:      model.NormalizeSubsiteMemberStatus(row.Status),
		CanAccess:   member.CanAccess(),
		CanManage:   member.CanManage(),
		JoinedAt:    row.JoinedAt,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}
}

func ensureSubsiteKeepsActiveOwner(tx *gorm.DB, current model.SubsiteMember, nextRole string, nextStatus string) error {
	currentIsActiveOwner := model.NormalizeSubsiteMemberRole(current.Role) == model.SubsiteMemberRoleOwner &&
		model.NormalizeSubsiteMemberStatus(current.Status) == model.SubsiteMemberStatusActive
	nextIsActiveOwner := model.NormalizeSubsiteMemberRole(nextRole) == model.SubsiteMemberRoleOwner &&
		model.NormalizeSubsiteMemberStatus(nextStatus) == model.SubsiteMemberStatusActive
	if !currentIsActiveOwner || nextIsActiveOwner {
		return nil
	}
	var activeOwners int64
	if err := tx.Model(&model.SubsiteMember{}).
		Where("subsite_id = ? AND role = ? AND status = ?", current.SubsiteId, model.SubsiteMemberRoleOwner, model.SubsiteMemberStatusActive).
		Count(&activeOwners).Error; err != nil {
		return err
	}
	if activeOwners <= 1 {
		return errors.New("subsite must keep at least one active owner")
	}
	return nil
}

func SubsiteQuotaPolicyInfoFromModel(policy model.SubsiteQuotaPolicy) dto.SubsiteQuotaPolicyInfo {
	return dto.SubsiteQuotaPolicyInfo{
		SiteDailyQuota:         policy.SiteDailyQuota,
		SiteWindowQuota:        policy.SiteWindowQuota,
		UserDailyQuota:         policy.UserDailyQuota,
		UserWindowQuota:        policy.UserWindowQuota,
		SiteDailyRequestLimit:  policy.SiteDailyRequestLimit,
		SiteWindowRequestLimit: policy.SiteWindowRequestLimit,
		UserDailyRequestLimit:  policy.UserDailyRequestLimit,
		UserWindowRequestLimit: policy.UserWindowRequestLimit,
		SiteWindowSeconds:      policy.SiteWindowSeconds,
		UserWindowSeconds:      policy.UserWindowSeconds,
	}
}

func SubsiteQuotaPolicyFromInfo(subsiteId int64, info dto.SubsiteQuotaPolicyInfo) model.SubsiteQuotaPolicy {
	policy := model.SubsiteQuotaPolicy{SubsiteId: subsiteId}
	applySubsiteQuotaPolicyInfo(&policy, info)
	return policy
}

func applySubsiteQuotaPolicyInfo(policy *model.SubsiteQuotaPolicy, info dto.SubsiteQuotaPolicyInfo) {
	policy.SiteDailyQuota = info.SiteDailyQuota
	policy.SiteWindowQuota = info.SiteWindowQuota
	policy.UserDailyQuota = info.UserDailyQuota
	policy.UserWindowQuota = info.UserWindowQuota
	policy.SiteDailyRequestLimit = info.SiteDailyRequestLimit
	policy.SiteWindowRequestLimit = info.SiteWindowRequestLimit
	policy.UserDailyRequestLimit = info.UserDailyRequestLimit
	policy.UserWindowRequestLimit = info.UserWindowRequestLimit
	policy.SiteWindowSeconds = info.SiteWindowSeconds
	policy.UserWindowSeconds = info.UserWindowSeconds
}

func managedSubsiteOwners(subsiteIds []int64) (map[int64][]int, map[int64][]string, error) {
	rows := make([]managedSubsiteOwnerRow, 0)
	if err := model.DB.Table("subsite_members").
		Select("subsite_members.subsite_id, subsite_members.user_id, users.username").
		Joins("LEFT JOIN users ON users.id = subsite_members.user_id").
		Where("subsite_members.subsite_id IN ? AND subsite_members.role = ?", subsiteIds, model.SubsiteMemberRoleOwner).
		Order("subsite_members.id ASC").
		Scan(&rows).Error; err != nil {
		return nil, nil, err
	}
	ids := map[int64][]int{}
	usernames := map[int64][]string{}
	for _, row := range rows {
		ids[row.SubsiteId] = append(ids[row.SubsiteId], row.UserId)
		if row.Username != "" {
			usernames[row.SubsiteId] = append(usernames[row.SubsiteId], row.Username)
		}
	}
	return ids, usernames, nil
}

func managedSubsiteMemberCounts(subsiteIds []int64) (map[int64]int64, error) {
	rows := make([]managedSubsiteMemberCountRow, 0)
	if err := model.DB.Model(&model.SubsiteMember{}).
		Select("subsite_id, COUNT(*) AS count").
		Where("subsite_id IN ?", subsiteIds).
		Group("subsite_id").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	counts := map[int64]int64{}
	for _, row := range rows {
		counts[row.SubsiteId] = row.Count
	}
	return counts, nil
}

func managedSubsiteTodayUsage(subsiteIds []int64, now int64) (map[int64]managedSubsiteUsageRow, error) {
	usage := map[int64]managedSubsiteUsageRow{}
	if model.LOG_DB == nil {
		return usage, nil
	}
	start, _ := dailyWindow(now)
	rows := make([]managedSubsiteUsageRow, 0)
	if err := model.LOG_DB.Model(&model.Log{}).
		Select("subsite_id, COUNT(*) AS calls, COALESCE(SUM(quota), 0) AS quota").
		Where("subsite_id IN ? AND created_at >= ? AND type IN ?", subsiteIds, start, []int{model.LogTypeConsume, model.LogTypeError}).
		Group("subsite_id").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		usage[row.SubsiteId] = row
	}
	return usage, nil
}

func managedSubsiteRolesForUser(subsiteIds []int64, userId int, role int) (map[int64]string, error) {
	roles := map[int64]string{}
	if role >= common.RoleAdminUser {
		for _, subsiteId := range subsiteIds {
			roles[subsiteId] = "admin"
		}
		return roles, nil
	}
	var members []model.SubsiteMember
	if err := model.DB.
		Where("subsite_id IN ? AND user_id = ?", subsiteIds, userId).
		Find(&members).Error; err != nil {
		return nil, err
	}
	for _, member := range members {
		roles[member.SubsiteId] = member.Role
	}
	return roles, nil
}

func managedSubsiteQuotaPolicies(subsiteIds []int64) (map[int64]model.SubsiteQuotaPolicy, error) {
	var policies []model.SubsiteQuotaPolicy
	if err := model.DB.Where("subsite_id IN ?", subsiteIds).Find(&policies).Error; err != nil {
		return nil, err
	}
	result := map[int64]model.SubsiteQuotaPolicy{}
	for _, policy := range policies {
		result[policy.SubsiteId] = policy
	}
	return result, nil
}
