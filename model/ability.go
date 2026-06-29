package model

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"

	"github.com/samber/lo"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Ability struct {
	Group     string  `json:"group" gorm:"type:varchar(64);primaryKey;autoIncrement:false"`
	Model     string  `json:"model" gorm:"type:varchar(255);primaryKey;autoIncrement:false"`
	ChannelId int     `json:"channel_id" gorm:"primaryKey;autoIncrement:false;index"`
	Enabled   bool    `json:"enabled"`
	Priority  *int64  `json:"priority" gorm:"bigint;default:0;index"`
	Weight    uint    `json:"weight" gorm:"default:0;index"`
	Tag       *string `json:"tag" gorm:"index"`
}

type AbilityWithChannel struct {
	Ability
	ChannelType int `json:"channel_type"`
}

func GetAllEnableAbilityWithChannels() ([]AbilityWithChannel, error) {
	var abilities []AbilityWithChannel
	err := DB.Table("abilities").
		Select("abilities.*, channels.type as channel_type").
		Joins("left join channels on abilities.channel_id = channels.id").
		Where("abilities.enabled = ?", true).
		Scan(&abilities).Error
	return abilities, err
}

func GetGroupEnabledModels(group string) []string {
	return GetGroupEnabledModelsForSubsite(group, 0)
}

func GetGroupEnabledModelsForSubsite(group string, subsiteId int64) []string {
	var models []string
	query := DB.Table("abilities").
		Joins("LEFT JOIN channels ON abilities.channel_id = channels.id").
		Where("abilities."+abilityGroupColumn()+" = ? and abilities.enabled = ?", group, true)
	if subsiteId > 0 {
		query = query.Where("channels.subsite_id = ?", subsiteId)
	} else {
		query = query.Where("(channels.id IS NULL OR channels.subsite_id = 0)")
	}
	query.Distinct("abilities.model").Pluck("abilities.model", &models)
	return models
}

func GetGroupsEnabledModelsForSubsite(groups []string, subsiteId int64) []string {
	seen := map[string]struct{}{}
	models := make([]string, 0)
	for _, group := range normalizeLookupValues(groups) {
		for _, modelName := range GetGroupEnabledModelsForSubsite(group, subsiteId) {
			if _, ok := seen[modelName]; ok {
				continue
			}
			seen[modelName] = struct{}{}
			models = append(models, modelName)
		}
	}
	return models
}

func IsModelEnabledForGroupsInSubsite(modelName string, groups []string, subsiteId int64) bool {
	if modelName == "" {
		return false
	}
	for _, enabledModel := range GetGroupsEnabledModelsForSubsite(groups, subsiteId) {
		if enabledModel == modelName {
			return true
		}
	}
	return false
}

func GetEnabledModels() []string {
	var models []string
	// Find distinct models
	DB.Table("abilities").Where("enabled = ?", true).Distinct("model").Pluck("model", &models)
	return models
}

func GetAllEnableAbilities() []Ability {
	var abilities []Ability
	DB.Find(&abilities, "enabled = ?", true)
	return abilities
}

func abilityGroupColumn() string {
	if commonGroupCol != "" {
		return commonGroupCol
	}
	if common.UsingPostgreSQL {
		return `"group"`
	}
	return "`group`"
}

func getPriority(group string, model string, retry int) (int, error) {
	return getPriorityForSubsite(group, model, retry, 0)
}

func getPriorityForSubsite(group string, model string, retry int, subsiteId int64) (int, error) {

	var priorities []int
	err := DB.Model(&Ability{}).
		Select("DISTINCT(priority)").
		Where(abilityGroupColumn()+" = ? and model = ? and enabled = ?", group, model, true).
		Where("channel_id IN (?)", scopedChannelIdsSubQuery(subsiteId)).
		Order("priority DESC").              // 按优先级降序排序
		Pluck("priority", &priorities).Error // Pluck用于将查询的结果直接扫描到一个切片中

	if err != nil {
		// 处理错误
		return 0, err
	}

	if len(priorities) == 0 {
		// 如果没有查询到优先级，则返回错误
		return 0, errors.New("数据库一致性被破坏")
	}

	// 确定要使用的优先级
	var priorityToUse int
	if retry >= len(priorities) {
		// 如果重试次数大于优先级数，则使用最小的优先级
		priorityToUse = priorities[len(priorities)-1]
	} else {
		priorityToUse = priorities[retry]
	}
	return priorityToUse, nil
}

func getChannelQuery(group string, model string, retry int) (*gorm.DB, error) {
	return getChannelQueryForSubsite(group, model, retry, 0)
}

func getChannelQueryForSubsite(group string, model string, retry int, subsiteId int64) (*gorm.DB, error) {
	channelIds := scopedChannelIdsSubQuery(subsiteId)
	maxPrioritySubQuery := DB.Model(&Ability{}).
		Select("MAX(priority)").
		Where(abilityGroupColumn()+" = ? and model = ? and enabled = ?", group, model, true).
		Where("channel_id IN (?)", channelIds)
	channelQuery := DB.Where(abilityGroupColumn()+" = ? and model = ? and enabled = ? and priority = (?)", group, model, true, maxPrioritySubQuery)
	channelQuery = channelQuery.Where("channel_id IN (?)", channelIds)
	if retry != 0 {
		priority, err := getPriorityForSubsite(group, model, retry, subsiteId)
		if err != nil {
			return nil, err
		} else {
			channelQuery = DB.Where(abilityGroupColumn()+" = ? and model = ? and enabled = ? and priority = ?", group, model, true, priority)
			channelQuery = channelQuery.Where("channel_id IN (?)", channelIds)
		}
	}

	return channelQuery, nil
}

func GetChannel(group string, model string, retry int) (*Channel, error) {
	return GetChannelExcluding(group, model, retry, nil)
}

func GetChannelExcluding(group string, model string, retry int, excludeChannelIds map[int]bool) (*Channel, error) {
	return GetChannelExcludingForSubsite(group, model, retry, excludeChannelIds, 0)
}

func GetChannelExcludingForSubsite(group string, model string, retry int, excludeChannelIds map[int]bool, subsiteId int64) (*Channel, error) {
	var abilities []Ability

	var err error = nil
	if len(excludeChannelIds) == 0 {
		channelQuery, err := getChannelQueryForSubsite(group, model, retry, subsiteId)
		if err != nil {
			return nil, err
		}
		err = channelQuery.Order("weight DESC").Find(&abilities).Error
	} else {
		err = DB.Where(abilityGroupColumn()+" = ? and model = ? and enabled = ?", group, model, true).
			Where("channel_id IN (?)", scopedChannelIdsSubQuery(subsiteId)).
			Order("priority DESC").
			Order("weight DESC").
			Find(&abilities).Error
	}
	if err != nil {
		return nil, err
	}
	abilities = filterExcludedAbilities(abilities, excludeChannelIds)
	if len(excludeChannelIds) > 0 {
		abilities = selectAbilitiesForRetryPriority(abilities, retry)
	}
	channel := Channel{}
	if len(abilities) > 0 {
		channel.Id = weightedRandomAbilityChannelId(abilities)
	} else {
		return nil, nil
	}
	err = DB.First(&channel, "id = ? AND subsite_id = ?", channel.Id, subsiteId).Error
	return &channel, err
}

func scopedChannelIdsSubQuery(subsiteId int64) *gorm.DB {
	return DB.Model(&Channel{}).Select("id").Where("subsite_id = ?", subsiteId)
}

func filterExcludedAbilities(abilities []Ability, excludeChannelIds map[int]bool) []Ability {
	if len(excludeChannelIds) == 0 || len(abilities) == 0 {
		return abilities
	}
	filtered := abilities[:0]
	for _, ability := range abilities {
		if excludeChannelIds[ability.ChannelId] {
			continue
		}
		filtered = append(filtered, ability)
	}
	return filtered
}

func selectAbilitiesForRetryPriority(abilities []Ability, retry int) []Ability {
	if len(abilities) <= 1 {
		return abilities
	}
	priorities := make([]int64, 0)
	seen := map[int64]bool{}
	for _, ability := range abilities {
		priority := int64(0)
		if ability.Priority != nil {
			priority = *ability.Priority
		}
		if seen[priority] {
			continue
		}
		seen[priority] = true
		priorities = append(priorities, priority)
	}
	if len(priorities) == 0 {
		return nil
	}
	if retry >= len(priorities) {
		retry = len(priorities) - 1
	}
	if retry < 0 {
		retry = 0
	}
	targetPriority := priorities[retry]
	filtered := abilities[:0]
	for _, ability := range abilities {
		priority := int64(0)
		if ability.Priority != nil {
			priority = *ability.Priority
		}
		if priority == targetPriority {
			filtered = append(filtered, ability)
		}
	}
	return filtered
}

func weightedRandomAbilityChannelId(abilities []Ability) int {
	weightSum := uint(0)
	for _, ability := range abilities {
		weightSum += ability.Weight + 10
	}
	weight := common.GetRandomInt(int(weightSum))
	for _, ability := range abilities {
		weight -= int(ability.Weight) + 10
		if weight <= 0 {
			return ability.ChannelId
		}
	}
	return abilities[len(abilities)-1].ChannelId
}

func (channel *Channel) AddAbilities(tx *gorm.DB) error {
	models_ := strings.Split(channel.Models, ",")
	groups_ := strings.Split(channel.Group, ",")
	abilitySet := make(map[string]struct{})
	abilities := make([]Ability, 0, len(models_))
	for _, model := range models_ {
		for _, group := range groups_ {
			key := group + "|" + model
			if _, exists := abilitySet[key]; exists {
				continue
			}
			abilitySet[key] = struct{}{}
			ability := Ability{
				Group:     group,
				Model:     model,
				ChannelId: channel.Id,
				Enabled:   channel.Status == common.ChannelStatusEnabled,
				Priority:  channel.Priority,
				Weight:    uint(channel.GetWeight()),
				Tag:       channel.Tag,
			}
			abilities = append(abilities, ability)
		}
	}
	if len(abilities) == 0 {
		return nil
	}
	// choose DB or provided tx
	useDB := DB
	if tx != nil {
		useDB = tx
	}
	for _, chunk := range lo.Chunk(abilities, 50) {
		err := useDB.Clauses(clause.OnConflict{DoNothing: true}).Create(&chunk).Error
		if err != nil {
			return err
		}
	}
	return nil
}

func (channel *Channel) DeleteAbilities() error {
	return DB.Where("channel_id = ?", channel.Id).Delete(&Ability{}).Error
}

// UpdateAbilities updates abilities of this channel.
// Make sure the channel is completed before calling this function.
func (channel *Channel) UpdateAbilities(tx *gorm.DB) error {
	isNewTx := false
	// 如果没有传入事务，创建新的事务
	if tx == nil {
		tx = DB.Begin()
		if tx.Error != nil {
			return tx.Error
		}
		isNewTx = true
		defer func() {
			if r := recover(); r != nil {
				tx.Rollback()
			}
		}()
	}

	// First delete all abilities of this channel
	err := tx.Where("channel_id = ?", channel.Id).Delete(&Ability{}).Error
	if err != nil {
		if isNewTx {
			tx.Rollback()
		}
		return err
	}

	// Then add new abilities
	models_ := strings.Split(channel.Models, ",")
	groups_ := strings.Split(channel.Group, ",")
	abilitySet := make(map[string]struct{})
	abilities := make([]Ability, 0, len(models_))
	for _, model := range models_ {
		for _, group := range groups_ {
			key := group + "|" + model
			if _, exists := abilitySet[key]; exists {
				continue
			}
			abilitySet[key] = struct{}{}
			ability := Ability{
				Group:     group,
				Model:     model,
				ChannelId: channel.Id,
				Enabled:   channel.Status == common.ChannelStatusEnabled,
				Priority:  channel.Priority,
				Weight:    uint(channel.GetWeight()),
				Tag:       channel.Tag,
			}
			abilities = append(abilities, ability)
		}
	}

	if len(abilities) > 0 {
		for _, chunk := range lo.Chunk(abilities, 50) {
			err = tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&chunk).Error
			if err != nil {
				if isNewTx {
					tx.Rollback()
				}
				return err
			}
		}
	}

	// 如果是新创建的事务，需要提交
	if isNewTx {
		return tx.Commit().Error
	}

	return nil
}

func UpdateAbilityStatus(channelId int, status bool) error {
	return DB.Model(&Ability{}).Where("channel_id = ?", channelId).Select("enabled").Update("enabled", status).Error
}

func UpdateAbilityStatusByTag(tag string, status bool) error {
	return DB.Model(&Ability{}).Where("tag = ?", tag).Select("enabled").Update("enabled", status).Error
}

func UpdateAbilityByTag(tag string, newTag *string, priority *int64, weight *uint) error {
	ability := Ability{}
	if newTag != nil {
		ability.Tag = newTag
	}
	if priority != nil {
		ability.Priority = priority
	}
	if weight != nil {
		ability.Weight = *weight
	}
	return DB.Model(&Ability{}).Where("tag = ?", tag).Updates(ability).Error
}

var fixLock = sync.Mutex{}

func FixAbility() (int, int, error) {
	lock := fixLock.TryLock()
	if !lock {
		return 0, 0, errors.New("已经有一个修复任务在运行中，请稍后再试")
	}
	defer fixLock.Unlock()

	// truncate abilities table
	if common.UsingSQLite {
		err := DB.Exec("DELETE FROM abilities").Error
		if err != nil {
			common.SysLog(fmt.Sprintf("Delete abilities failed: %s", err.Error()))
			return 0, 0, err
		}
	} else {
		err := DB.Exec("TRUNCATE TABLE abilities").Error
		if err != nil {
			common.SysLog(fmt.Sprintf("Truncate abilities failed: %s", err.Error()))
			return 0, 0, err
		}
	}
	var channels []*Channel
	// Find all channels
	err := DB.Model(&Channel{}).Find(&channels).Error
	if err != nil {
		return 0, 0, err
	}
	if len(channels) == 0 {
		return 0, 0, nil
	}
	successCount := 0
	failCount := 0
	for _, chunk := range lo.Chunk(channels, 50) {
		ids := lo.Map(chunk, func(c *Channel, _ int) int { return c.Id })
		// Delete all abilities of this channel
		err = DB.Where("channel_id IN ?", ids).Delete(&Ability{}).Error
		if err != nil {
			common.SysLog(fmt.Sprintf("Delete abilities failed: %s", err.Error()))
			failCount += len(chunk)
			continue
		}
		// Then add new abilities
		for _, channel := range chunk {
			err = channel.AddAbilities(nil)
			if err != nil {
				common.SysLog(fmt.Sprintf("Add abilities for channel %d failed: %s", channel.Id, err.Error()))
				failCount++
			} else {
				successCount++
			}
		}
	}
	InitChannelCache()
	return successCount, failCount, nil
}
