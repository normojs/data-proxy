package model

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

func IsChannelEnabledForGroupModel(group string, modelName string, channelID int) bool {
	return IsChannelEnabledForGroupModelInSubsite(group, modelName, channelID, 0)
}

func IsChannelEnabledForGroupModelInSubsite(group string, modelName string, channelID int, subsiteId int64) bool {
	if group == "" || modelName == "" || channelID <= 0 {
		return false
	}
	if !common.MemoryCacheEnabled {
		return isChannelEnabledForGroupModelDB(group, modelName, channelID, subsiteId)
	}

	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()

	if group2model2channels == nil {
		return false
	}
	if channel, ok := channelsIDM[channelID]; !ok || channel.SubsiteId != subsiteId {
		return false
	}

	if isChannelIDInList(group2model2channels[group][modelName], channelID) {
		return true
	}
	normalized := ratio_setting.FormatMatchingModelName(modelName)
	if normalized != "" && normalized != modelName {
		return isChannelIDInList(group2model2channels[group][normalized], channelID)
	}
	return false
}

func IsChannelEnabledForAnyGroupModel(groups []string, modelName string, channelID int) bool {
	if len(groups) == 0 {
		return false
	}
	for _, g := range groups {
		if IsChannelEnabledForGroupModel(g, modelName, channelID) {
			return true
		}
	}
	return false
}

func isChannelEnabledForGroupModelDB(group string, modelName string, channelID int, subsiteId int64) bool {
	var count int64
	err := DB.Model(&Ability{}).
		Where(abilityGroupColumn()+" = ? and model = ? and channel_id = ? and enabled = ?", group, modelName, channelID, true).
		Where("channel_id IN (?)", scopedChannelIdsSubQuery(subsiteId)).
		Count(&count).Error
	if err == nil && count > 0 {
		return true
	}
	normalized := ratio_setting.FormatMatchingModelName(modelName)
	if normalized == "" || normalized == modelName {
		return false
	}
	count = 0
	err = DB.Model(&Ability{}).
		Where(abilityGroupColumn()+" = ? and model = ? and channel_id = ? and enabled = ?", group, normalized, channelID, true).
		Where("channel_id IN (?)", scopedChannelIdsSubQuery(subsiteId)).
		Count(&count).Error
	return err == nil && count > 0
}

func isChannelIDInList(list []int, channelID int) bool {
	for _, id := range list {
		if id == channelID {
			return true
		}
	}
	return false
}
