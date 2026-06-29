package service

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

func PublicSubsiteFromModel(subsite *model.Subsite, now int64) dto.PublicSubsite {
	if now <= 0 {
		now = common.GetTimestamp()
	}
	decision := subsite.AccessDecision(now, false)
	return dto.PublicSubsite{
		Id:                 subsite.Id,
		Slug:               subsite.Slug,
		Name:               subsite.Name,
		Title:              subsite.Title,
		LogoURL:            subsite.LogoURL,
		FaviconURL:         subsite.FaviconURL,
		ThemeColor:         subsite.ThemeColor,
		Status:             subsite.Status,
		RuntimeStatus:      decision.Status,
		DisabledReason:     subsite.DisabledReason,
		AnnouncementIcon:   subsite.AnnouncementIcon,
		AnnouncementTitle:  subsite.AnnouncementTitle,
		AnnouncementBody:   subsite.AnnouncementBody,
		AnnouncementURL:    subsite.AnnouncementURL,
		ContactURL:         subsite.ContactURL,
		RegistrationPolicy: subsite.RegistrationPolicy,
		StartsAt:           subsite.StartsAt,
		EndsAt:             subsite.EndsAt,
		Access: dto.SubsiteAccessInfo{
			Allowed: decision.Allowed,
			Status:  decision.Status,
			Code:    decision.Code,
			Message: decision.Message,
		},
	}
}

func SubsiteMemberInfoFromModel(member *model.SubsiteMember) dto.SubsiteMemberInfo {
	if member == nil {
		return dto.SubsiteMemberInfo{}
	}
	return dto.SubsiteMemberInfo{
		SubsiteId: member.SubsiteId,
		UserId:    member.UserId,
		Role:      member.Role,
		Status:    member.Status,
		CanAccess: member.CanAccess(),
		CanManage: member.CanManage(),
	}
}

func SubsiteTokenInfoFromModel(token *model.Token, reveal bool) *dto.SubsiteTokenInfo {
	if token == nil {
		return nil
	}
	info := &dto.SubsiteTokenInfo{
		Id:             token.Id,
		Name:           token.Name,
		MaskedKey:      token.GetMaskedKey(),
		Status:         token.Status,
		CreatedTime:    token.CreatedTime,
		AccessedTime:   token.AccessedTime,
		ExpiredTime:    token.ExpiredTime,
		UnlimitedQuota: token.UnlimitedQuota,
	}
	if reveal {
		info.Key = token.GetFullKey()
	}
	return info
}

func GetSubsiteUserToken(subsiteId int64, userId int) (*model.Token, error) {
	var token model.Token
	if err := model.DB.
		Where("subsite_id = ? AND user_id = ?", subsiteId, userId).
		Order("id DESC").
		First(&token).Error; err != nil {
		return nil, err
	}
	return &token, nil
}

func EnsureSubsiteUserToken(subsite *model.Subsite, userId int, username string) (*model.Token, bool, error) {
	if subsite == nil {
		return nil, false, errors.New("subsite is required")
	}
	token, err := GetSubsiteUserToken(subsite.Id, userId)
	if err == nil {
		return token, false, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, err
	}
	key, err := common.GenerateKey()
	if err != nil {
		return nil, false, err
	}
	now := common.GetTimestamp()
	token = &model.Token{
		SubsiteId:          subsite.Id,
		UserId:             userId,
		Name:               subsiteTokenName(subsite, username),
		Key:                key,
		Status:             common.TokenStatusEnabled,
		CreatedTime:        now,
		AccessedTime:       now,
		ExpiredTime:        -1,
		UnlimitedQuota:     true,
		ModelLimitsEnabled: false,
	}
	if err := model.DB.Create(token).Error; err != nil {
		return nil, false, err
	}
	return token, true, nil
}

func RotateSubsiteUserToken(subsite *model.Subsite, userId int, username string) (*model.Token, bool, error) {
	token, created, err := EnsureSubsiteUserToken(subsite, userId, username)
	if err != nil || created {
		return token, created, err
	}
	oldKey := token.Key
	key, err := common.GenerateKey()
	if err != nil {
		return nil, false, err
	}
	token.Key = key
	token.AccessedTime = common.GetTimestamp()
	token.Status = common.TokenStatusEnabled
	if err := model.DB.Model(token).Select("key", "accessed_time", "status").Updates(token).Error; err != nil {
		return nil, false, err
	}
	if oldKey != "" {
		_ = model.InvalidateTokenCacheByKey(oldKey)
	}
	return token, false, nil
}

func BuildSubsiteDashboard(subsite *model.Subsite, member *model.SubsiteMember, baseURL string, now int64) (dto.SubsiteDashboard, error) {
	if now <= 0 {
		now = common.GetTimestamp()
	}
	userId := 0
	if member != nil {
		userId = member.UserId
	}
	token, err := GetSubsiteUserToken(subsite.Id, userId)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return dto.SubsiteDashboard{}, err
	}
	var tokenInfo *dto.SubsiteTokenInfo
	if token != nil {
		tokenInfo = SubsiteTokenInfoFromModel(token, false)
	}
	quota, err := getSubsiteQuotaSummary(subsite.Id, userId, now)
	if err != nil {
		return dto.SubsiteDashboard{}, err
	}
	stats, err := getSubsiteUsageStats(subsite.Id, userId, now)
	if err != nil {
		return dto.SubsiteDashboard{}, err
	}
	logs, err := getSubsiteRecentLogs(subsite.Id, userId)
	if err != nil {
		return dto.SubsiteDashboard{}, err
	}
	return dto.SubsiteDashboard{
		Subsite:    PublicSubsiteFromModel(subsite, now),
		Member:     SubsiteMemberInfoFromModel(member),
		BaseURL:    baseURL,
		Token:      tokenInfo,
		Quota:      quota,
		Stats24h:   stats,
		RecentLogs: logs,
	}, nil
}

func subsiteTokenName(subsite *model.Subsite, username string) string {
	name := strings.TrimSpace(subsite.Name)
	if name == "" {
		name = strings.TrimSpace(subsite.Slug)
	}
	if username == "" {
		return name + " API Key"
	}
	return fmt.Sprintf("%s API Key (%s)", name, username)
}

func getSubsiteQuotaSummary(subsiteId int64, userId int, now int64) (dto.SubsiteQuotaSummary, error) {
	var policy model.SubsiteQuotaPolicy
	err := model.DB.Where("subsite_id = ?", subsiteId).First(&policy).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return dto.SubsiteQuotaSummary{}, err
	}
	dailyStart, dailyEnd := dailyWindow(now)
	siteDaily, err := getSubsiteQuotaCounter(subsiteId, 0, model.SubsiteCounterScopeSite, model.SubsiteCounterWindowDaily, dailyStart, now)
	if err != nil {
		return dto.SubsiteQuotaSummary{}, err
	}
	userDaily, err := getSubsiteQuotaCounter(subsiteId, userId, model.SubsiteCounterScopeUser, model.SubsiteCounterWindowDaily, dailyStart, now)
	if err != nil {
		return dto.SubsiteQuotaSummary{}, err
	}
	siteRolling, err := getSubsiteQuotaCounter(subsiteId, 0, model.SubsiteCounterScopeSite, model.SubsiteCounterWindowRolling, 0, now)
	if err != nil {
		return dto.SubsiteQuotaSummary{}, err
	}
	userRolling, err := getSubsiteQuotaCounter(subsiteId, userId, model.SubsiteCounterScopeUser, model.SubsiteCounterWindowRolling, 0, now)
	if err != nil {
		return dto.SubsiteQuotaSummary{}, err
	}
	return dto.SubsiteQuotaSummary{
		SiteDailyQuota:     quotaMetric(policy.SiteDailyQuota, siteDaily.UsedQuota, dailyStart, dailyEnd, 0),
		SiteWindowQuota:    quotaMetric(policy.SiteWindowQuota, siteRolling.UsedQuota, siteRolling.WindowStart, siteRolling.WindowEnd, policy.SiteWindowSeconds),
		UserDailyQuota:     quotaMetric(policy.UserDailyQuota, userDaily.UsedQuota, dailyStart, dailyEnd, 0),
		UserWindowQuota:    quotaMetric(policy.UserWindowQuota, userRolling.UsedQuota, userRolling.WindowStart, userRolling.WindowEnd, policy.UserWindowSeconds),
		SiteDailyRequests:  quotaMetric(policy.SiteDailyRequestLimit, siteDaily.RequestCount, dailyStart, dailyEnd, 0),
		SiteWindowRequests: quotaMetric(policy.SiteWindowRequestLimit, siteRolling.RequestCount, siteRolling.WindowStart, siteRolling.WindowEnd, policy.SiteWindowSeconds),
		UserDailyRequests:  quotaMetric(policy.UserDailyRequestLimit, userDaily.RequestCount, dailyStart, dailyEnd, 0),
		UserWindowRequests: quotaMetric(policy.UserWindowRequestLimit, userRolling.RequestCount, userRolling.WindowStart, userRolling.WindowEnd, policy.UserWindowSeconds),
	}, nil
}

func getSubsiteQuotaCounter(subsiteId int64, userId int, scope string, windowType string, dailyStart int64, now int64) (model.SubsiteQuotaCounter, error) {
	var counter model.SubsiteQuotaCounter
	query := model.DB.Where("subsite_id = ? AND user_id = ? AND scope = ? AND window_type = ?", subsiteId, userId, scope, windowType)
	if windowType == model.SubsiteCounterWindowDaily {
		query = query.Where("window_start = ?", dailyStart)
	} else {
		query = query.Where("(window_end = 0 OR window_end >= ?)", now).Order("window_start DESC")
	}
	err := query.First(&counter).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.SubsiteQuotaCounter{}, nil
	}
	return counter, err
}

func quotaMetric(limit int, used int, windowStart int64, windowEnd int64, windowSeconds int64) dto.SubsiteQuotaMetric {
	remaining := 0
	if limit > 0 {
		remaining = int(math.Max(0, float64(limit-used)))
	}
	return dto.SubsiteQuotaMetric{
		Limit:         limit,
		Used:          used,
		Remaining:     remaining,
		WindowStart:   windowStart,
		WindowEnd:     windowEnd,
		NextResetTime: windowEnd,
		WindowSeconds: windowSeconds,
	}
}

func dailyWindow(now int64) (int64, int64) {
	t := time.Unix(now, 0).UTC()
	start := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	return start.Unix(), end.Unix()
}

func getSubsiteUsageStats(subsiteId int64, userId int, now int64) (dto.SubsiteUsageStats, error) {
	start := now - 24*60*60
	var stats struct {
		Calls            int64
		PromptTokens     int64
		CompletionTokens int64
		Quota            int64
		LastRequestAt    int64
	}
	err := model.LOG_DB.Model(&model.Log{}).
		Select("COUNT(*) AS calls, COALESCE(SUM(prompt_tokens), 0) AS prompt_tokens, COALESCE(SUM(completion_tokens), 0) AS completion_tokens, COALESCE(SUM(quota), 0) AS quota, COALESCE(MAX(created_at), 0) AS last_request_at").
		Where("subsite_id = ? AND user_id = ? AND created_at >= ? AND type IN ?", subsiteId, userId, start, []int{model.LogTypeConsume, model.LogTypeError}).
		Scan(&stats).Error
	if err != nil {
		return dto.SubsiteUsageStats{}, err
	}
	return dto.SubsiteUsageStats{
		WindowSeconds: 24 * 60 * 60,
		Calls:         stats.Calls,
		PromptTokens:  stats.PromptTokens,
		OutputTokens:  stats.CompletionTokens,
		TotalTokens:   stats.PromptTokens + stats.CompletionTokens,
		Quota:         stats.Quota,
		LastRequestAt: stats.LastRequestAt,
	}, nil
}

func getSubsiteRecentLogs(subsiteId int64, userId int) ([]dto.SubsiteRecentLog, error) {
	var logs []model.Log
	if err := model.LOG_DB.
		Where("subsite_id = ? AND user_id = ? AND type IN ?", subsiteId, userId, []int{model.LogTypeConsume, model.LogTypeError}).
		Order("created_at DESC, id DESC").
		Limit(10).
		Find(&logs).Error; err != nil {
		return nil, err
	}
	items := make([]dto.SubsiteRecentLog, 0, len(logs))
	for _, log := range logs {
		items = append(items, subsiteRecentLogFromModel(log))
	}
	return items, nil
}

func subsiteRecentLogFromModel(log model.Log) dto.SubsiteRecentLog {
	other, _ := common.StrToMap(log.Other)
	cacheTokens := intFromMap(other, "cache_tokens") +
		intFromMap(other, "cache_creation_tokens") +
		intFromMap(other, "cache_creation_tokens_5m") +
		intFromMap(other, "cache_creation_tokens_1h")
	reasoningTokens := intFromMap(other, "reasoning_tokens") +
		intFromMap(other, "reasoning_output_tokens")
	status := "success"
	if log.Type == model.LogTypeError {
		status = "error"
	}
	return dto.SubsiteRecentLog{
		Id:               log.Id,
		CreatedAt:        log.CreatedAt,
		Type:             log.Type,
		Username:         log.Username,
		ModelName:        log.ModelName,
		PromptTokens:     log.PromptTokens,
		CompletionTokens: log.CompletionTokens,
		CacheTokens:      cacheTokens,
		ReasoningTokens:  reasoningTokens,
		TotalTokens:      log.PromptTokens + log.CompletionTokens,
		Quota:            log.Quota,
		UseTime:          log.UseTime,
		Status:           status,
	}
}

func intFromMap(values map[string]interface{}, key string) int {
	if values == nil {
		return 0
	}
	switch value := values[key].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	case string:
		var parsed int
		if _, err := fmt.Sscanf(value, "%d", &parsed); err == nil {
			return parsed
		}
	}
	return 0
}
