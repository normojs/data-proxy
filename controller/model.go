package controller

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay"
	"github.com/QuantumNous/new-api/relay/channel/ai360"
	"github.com/QuantumNous/new-api/relay/channel/lingyiwanwu"
	"github.com/QuantumNous/new-api/relay/channel/minimax"
	"github.com/QuantumNous/new-api/relay/channel/moonshot"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
)

// https://platform.openai.com/docs/api-reference/models/list

var openAIModels []dto.OpenAIModels
var openAIModelsMap map[string]dto.OpenAIModels
var channelId2Models map[int][]string

func init() {
	// https://platform.openai.com/docs/models/model-endpoint-compatibility
	for i := 0; i < constant.APITypeDummy; i++ {
		if i == constant.APITypeAIProxyLibrary {
			continue
		}
		adaptor := relay.GetAdaptor(i)
		channelName := adaptor.GetChannelName()
		modelNames := adaptor.GetModelList()
		for _, modelName := range modelNames {
			openAIModels = append(openAIModels, dto.OpenAIModels{
				Id:      modelName,
				Object:  "model",
				Created: 1626777600,
				OwnedBy: channelName,
			})
		}
	}
	for _, modelName := range ai360.ModelList {
		openAIModels = append(openAIModels, dto.OpenAIModels{
			Id:      modelName,
			Object:  "model",
			Created: 1626777600,
			OwnedBy: ai360.ChannelName,
		})
	}
	for _, modelName := range moonshot.ModelList {
		openAIModels = append(openAIModels, dto.OpenAIModels{
			Id:      modelName,
			Object:  "model",
			Created: 1626777600,
			OwnedBy: moonshot.ChannelName,
		})
	}
	for _, modelName := range lingyiwanwu.ModelList {
		openAIModels = append(openAIModels, dto.OpenAIModels{
			Id:      modelName,
			Object:  "model",
			Created: 1626777600,
			OwnedBy: lingyiwanwu.ChannelName,
		})
	}
	for _, modelName := range minimax.ModelList {
		openAIModels = append(openAIModels, dto.OpenAIModels{
			Id:      modelName,
			Object:  "model",
			Created: 1626777600,
			OwnedBy: minimax.ChannelName,
		})
	}
	for modelName, _ := range constant.MidjourneyModel2Action {
		openAIModels = append(openAIModels, dto.OpenAIModels{
			Id:      modelName,
			Object:  "model",
			Created: 1626777600,
			OwnedBy: "midjourney",
		})
	}
	openAIModelsMap = make(map[string]dto.OpenAIModels)
	for _, aiModel := range openAIModels {
		openAIModelsMap[aiModel.Id] = aiModel
	}
	channelId2Models = make(map[int][]string)
	for i := 1; i <= constant.ChannelTypeDummy; i++ {
		apiType, success := common.ChannelType2APIType(i)
		if !success || apiType == constant.APITypeAIProxyLibrary {
			continue
		}
		meta := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType: i,
		}}
		adaptor := relay.GetAdaptor(apiType)
		adaptor.Init(meta)
		channelId2Models[i] = adaptor.GetModelList()
	}
	openAIModels = lo.UniqBy(openAIModels, func(m dto.OpenAIModels) string {
		return m.Id
	})
}

func channelOwnerName(channelType int) string {
	apiType, success := common.ChannelType2APIType(channelType)
	if !success {
		return strings.ToLower(constant.GetChannelTypeName(channelType))
	}
	adaptor := relay.GetAdaptor(apiType)
	if adaptor == nil {
		return strings.ToLower(constant.GetChannelTypeName(channelType))
	}
	adaptor.Init(&relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{
		ChannelType: channelType,
	}})
	if name := strings.TrimSpace(adaptor.GetChannelName()); name != "" {
		return name
	}
	return strings.ToLower(constant.GetChannelTypeName(channelType))
}

func getPreferredModelOwners(modelNames []string, groups []string, subsiteId int64) map[string]string {
	channelTypes, err := model.GetPreferredModelOwnerChannelTypesForSubsite(modelNames, groups, subsiteId)
	if err != nil {
		common.SysLog(fmt.Sprintf("GetPreferredModelOwnerChannelTypes error: %v", err))
		return map[string]string{}
	}

	ownerByChannelType := make(map[int]string)
	owners := make(map[string]string, len(channelTypes))
	for modelName, channelType := range channelTypes {
		owner, ok := ownerByChannelType[channelType]
		if !ok {
			owner = channelOwnerName(channelType)
			ownerByChannelType[channelType] = owner
		}
		if owner != "" {
			owners[modelName] = owner
		}
	}
	return owners
}

func getPreferredModelDisplayNames(modelNames []string, groups []string, subsiteId int64) map[string]string {
	displayNames, err := model.GetPreferredModelDisplayNamesForSubsite(modelNames, groups, subsiteId)
	if err != nil {
		common.SysLog(fmt.Sprintf("GetPreferredModelDisplayNames error: %v", err))
		return map[string]string{}
	}
	return displayNames
}

func getEnabledModelsForModelListGroups(groups modelListGroups, subsiteId int64) []string {
	if groups.tokenGroup == "auto" {
		return model.GetGroupsEnabledModelsForSubsite(groups.ownerGroups, subsiteId)
	}
	return model.GetGroupEnabledModelsForSubsite(groups.ownerGroups[0], subsiteId)
}

func subsiteModelAvailabilitySet(groups modelListGroups, subsiteId int64) map[string]struct{} {
	available := map[string]struct{}{}
	if subsiteId <= 0 || len(groups.ownerGroups) == 0 {
		return available
	}
	for _, modelName := range model.GetGroupsEnabledModelsForSubsite(groups.ownerGroups, subsiteId) {
		available[modelName] = struct{}{}
	}
	return available
}

func buildOpenAIModel(modelName string, ownerByModel map[string]string, displayNameByModel map[string]string) dto.OpenAIModels {
	var oaiModel dto.OpenAIModels
	if staticModel, ok := openAIModelsMap[modelName]; ok {
		oaiModel = staticModel
	} else {
		oaiModel = dto.OpenAIModels{
			Id:      modelName,
			Object:  "model",
			Created: 1626777600,
			OwnedBy: "custom",
		}
	}
	if owner, ok := ownerByModel[modelName]; ok && owner != "" {
		oaiModel.OwnedBy = owner
	}
	if displayName, ok := displayNameByModel[modelName]; ok && displayName != "" {
		oaiModel.DisplayName = displayName
	}
	oaiModel.SupportedEndpointTypes = model.GetModelSupportEndpointTypes(modelName)
	return oaiModel
}

type modelListGroups struct {
	userGroup   string
	tokenGroup  string
	ownerGroups []string
}

func getModelListGroups(c *gin.Context) (modelListGroups, error) {
	tokenGroup := common.GetContextKeyString(c, constant.ContextKeyTokenGroup)
	userGroup := common.GetContextKeyString(c, constant.ContextKeyUserGroup)
	boundGroups := common.GetContextKeyStringSlice(c, constant.ContextKeyUserTokenGroups)
	boundGroups = model.NormalizeTokenGroups(boundGroups)
	if userGroup == "" && (tokenGroup == "" || tokenGroup == "auto") {
		var err error
		if userCache, cacheErr := model.GetUserCache(c.GetInt("id")); cacheErr == nil && userCache != nil {
			userGroup = userCache.Group
			boundGroups = model.NormalizeTokenGroups(userCache.GetTokenGroups())
		} else {
			userGroup, err = model.GetUserGroup(c.GetInt("id"), false)
		}
		if err != nil {
			return modelListGroups{}, err
		}
	}

	if tokenGroup == "auto" {
		return modelListGroups{
			userGroup:   userGroup,
			tokenGroup:  tokenGroup,
			ownerGroups: service.GetUserAutoGroupWithBindings(userGroup, boundGroups),
		}, nil
	}

	if tokenGroup == "" && len(boundGroups) > 0 {
		usableGroups := service.GetUserUsableGroupsWithBindings(userGroup, boundGroups)
		ownerGroups := make([]string, 0, len(usableGroups))
		for _, group := range boundGroups {
			if _, ok := usableGroups[group]; ok {
				ownerGroups = append(ownerGroups, group)
			}
		}
		return modelListGroups{
			userGroup:   userGroup,
			tokenGroup:  tokenGroup,
			ownerGroups: ownerGroups,
		}, nil
	}

	group := userGroup
	if tokenGroup != "" {
		group = tokenGroup
	}
	if !model.TokenGroupAllowedByBindings(boundGroups, group) {
		return modelListGroups{
			userGroup:   userGroup,
			tokenGroup:  tokenGroup,
			ownerGroups: []string{},
		}, nil
	}
	return modelListGroups{
		userGroup:   userGroup,
		tokenGroup:  tokenGroup,
		ownerGroups: []string{group},
	}, nil
}

func ListModels(c *gin.Context, modelType int) {
	acceptUnsetRatioModel := operation_setting.SelfUseModeEnabled
	if !acceptUnsetRatioModel {
		userId := c.GetInt("id")
		if userId > 0 {
			userSettings, _ := model.GetUserSetting(userId, false)
			if userSettings.AcceptUnsetRatioModel {
				acceptUnsetRatioModel = true
			}
		}
	}

	userModelNames := make([]string, 0)
	groups, err := getModelListGroups(c)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "get user group failed",
		})
		return
	}
	ownerGroups := groups.ownerGroups
	if len(ownerGroups) == 0 {
		respondEmptyModelList(c, modelType)
		return
	}
	subsiteId := c.GetInt64("subsite_id")
	modelLimitEnable := common.GetContextKeyBool(c, constant.ContextKeyTokenModelLimitEnabled)
	if modelLimitEnable {
		s, ok := common.GetContextKey(c, constant.ContextKeyTokenModelLimit)
		var tokenModelLimit map[string]bool
		if ok {
			tokenModelLimit = s.(map[string]bool)
		} else {
			tokenModelLimit = map[string]bool{}
		}
		availableInSubsite := subsiteModelAvailabilitySet(groups, subsiteId)
		for allowModel, _ := range tokenModelLimit {
			if subsiteId > 0 {
				if _, ok := availableInSubsite[allowModel]; !ok {
					continue
				}
			}
			if !acceptUnsetRatioModel {
				if !helper.HasModelBillingConfig(allowModel) {
					continue
				}
			}
			userModelNames = append(userModelNames, allowModel)
		}
	} else {
		models := getEnabledModelsForModelListGroups(groups, subsiteId)
		for _, modelName := range models {
			if !acceptUnsetRatioModel {
				if !helper.HasModelBillingConfig(modelName) {
					continue
				}
			}
			userModelNames = append(userModelNames, modelName)
		}
	}

	ownerByModel := map[string]string{}
	if len(ownerGroups) > 0 {
		ownerByModel = getPreferredModelOwners(userModelNames, ownerGroups, subsiteId)
	}
	displayNameByModel := map[string]string{}
	if subsiteId > 0 && len(ownerGroups) > 0 {
		displayNameByModel = getPreferredModelDisplayNames(userModelNames, ownerGroups, subsiteId)
	}
	userOpenAiModels := make([]dto.OpenAIModels, 0, len(userModelNames))
	for _, modelName := range userModelNames {
		userOpenAiModels = append(userOpenAiModels, buildOpenAIModel(modelName, ownerByModel, displayNameByModel))
	}
	if len(userOpenAiModels) == 0 {
		respondEmptyModelList(c, modelType)
		return
	}

	switch modelType {
	case constant.ChannelTypeAnthropic:
		useranthropicModels := make([]dto.AnthropicModel, len(userOpenAiModels))
		for i, model := range userOpenAiModels {
			useranthropicModels[i] = dto.AnthropicModel{
				ID:          model.Id,
				CreatedAt:   time.Unix(int64(model.Created), 0).UTC().Format(time.RFC3339),
				DisplayName: modelDisplayName(model),
				Type:        "model",
			}
		}
		c.JSON(200, gin.H{
			"data":     useranthropicModels,
			"first_id": useranthropicModels[0].ID,
			"has_more": false,
			"last_id":  useranthropicModels[len(useranthropicModels)-1].ID,
		})
	case constant.ChannelTypeGemini:
		userGeminiModels := make([]dto.GeminiModel, len(userOpenAiModels))
		for i, model := range userOpenAiModels {
			userGeminiModels[i] = dto.GeminiModel{
				Name:        model.Id,
				DisplayName: modelDisplayName(model),
			}
		}
		c.JSON(200, gin.H{
			"models":        userGeminiModels,
			"nextPageToken": nil,
		})
	default:
		c.JSON(200, gin.H{
			"success": true,
			"data":    userOpenAiModels,
			"object":  "list",
		})
	}
}

func respondEmptyModelList(c *gin.Context, modelType int) {
	switch modelType {
	case constant.ChannelTypeAnthropic:
		c.JSON(200, gin.H{
			"data":     []dto.AnthropicModel{},
			"first_id": "",
			"has_more": false,
			"last_id":  "",
		})
	case constant.ChannelTypeGemini:
		c.JSON(200, gin.H{
			"models":        []dto.GeminiModel{},
			"nextPageToken": nil,
		})
	default:
		c.JSON(200, gin.H{
			"success": true,
			"data":    []dto.OpenAIModels{},
			"object":  "list",
		})
	}
}

func ChannelListModels(c *gin.Context) {
	c.JSON(200, gin.H{
		"success": true,
		"data":    openAIModels,
	})
}

func DashboardListModels(c *gin.Context) {
	c.JSON(200, gin.H{
		"success": true,
		"data":    channelId2Models,
	})
}

func EnabledListModels(c *gin.Context) {
	c.JSON(200, gin.H{
		"success": true,
		"data":    model.GetEnabledModels(),
	})
}

func RetrieveModel(c *gin.Context, modelType int) {
	modelId := c.Param("model")
	if c.GetInt64("subsite_id") > 0 && !isModelAvailableForCurrentSubsite(c, modelId) {
		respondModelNotFound(c, modelId)
		return
	}
	if aiModel, ok := openAIModelsMap[modelId]; ok {
		if displayName := getDisplayNameForCurrentSubsite(c, modelId); displayName != "" {
			aiModel.DisplayName = displayName
		}
		switch modelType {
		case constant.ChannelTypeAnthropic:
			c.JSON(200, dto.AnthropicModel{
				ID:          aiModel.Id,
				CreatedAt:   time.Unix(int64(aiModel.Created), 0).UTC().Format(time.RFC3339),
				DisplayName: modelDisplayName(aiModel),
				Type:        "model",
			})
		default:
			c.JSON(200, aiModel)
		}
	} else {
		respondModelNotFound(c, modelId)
	}
}

func modelDisplayName(model dto.OpenAIModels) string {
	if strings.TrimSpace(model.DisplayName) != "" {
		return model.DisplayName
	}
	return model.Id
}

func getDisplayNameForCurrentSubsite(c *gin.Context, modelId string) string {
	subsiteId := c.GetInt64("subsite_id")
	if subsiteId <= 0 || strings.TrimSpace(modelId) == "" {
		return ""
	}
	groups, err := getModelListGroups(c)
	if err != nil || len(groups.ownerGroups) == 0 {
		return ""
	}
	displayNames := getPreferredModelDisplayNames([]string{modelId}, groups.ownerGroups, subsiteId)
	return displayNames[modelId]
}

func isModelAvailableForCurrentSubsite(c *gin.Context, modelId string) bool {
	groups, err := getModelListGroups(c)
	if err != nil || len(groups.ownerGroups) == 0 {
		return false
	}
	if common.GetContextKeyBool(c, constant.ContextKeyTokenModelLimitEnabled) {
		value, ok := common.GetContextKey(c, constant.ContextKeyTokenModelLimit)
		if !ok {
			return false
		}
		tokenModelLimit, ok := value.(map[string]bool)
		if !ok || !tokenModelLimit[modelId] {
			return false
		}
	}
	return model.IsModelEnabledForGroupsInSubsite(modelId, groups.ownerGroups, c.GetInt64("subsite_id"))
}

func respondModelNotFound(c *gin.Context, modelId string) {
	openAIError := types.OpenAIError{
		Message: fmt.Sprintf("The model '%s' does not exist", modelId),
		Type:    "invalid_request_error",
		Param:   "model",
		Code:    "model_not_found",
	}
	c.JSON(200, gin.H{
		"error": openAIError,
	})
}
