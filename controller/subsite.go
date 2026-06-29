package controller

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type SubsiteRegisterRequest struct {
	Username         string `json:"username"`
	Password         string `json:"password"`
	Email            string `json:"email"`
	VerificationCode string `json:"verification_code"`
	AffCode          string `json:"aff_code"`
	InviteCode       string `json:"invite_code"`
}

func ListManagedSubsites(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	items, total, err := service.ListManagedSubsites(c.GetInt("id"), c.GetInt("role"), pageInfo.GetStartIdx(), pageInfo.GetPageSize(), common.GetTimestamp())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

func CreateManagedSubsite(c *gin.Context) {
	if c.GetInt("role") < common.RoleAdminUser {
		common.ApiErrorMsg(c, "admin permission required")
		return
	}
	var req dto.ManagedSubsiteCreateRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	subsite, err := service.CreateManagedSubsite(req, c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordSubsiteManageLog(c, subsite.Id, "created subsite")
	if req.QuotaPolicy != nil {
		recordSubsiteManageLog(c, subsite.Id, "updated subsite quota policy")
	}
	item, err := service.GetManagedSubsite(subsite.Id, c.GetInt("id"), c.GetInt("role"), common.GetTimestamp())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, item)
}

func GetManagedSubsite(c *gin.Context) {
	subsiteId, ok := managedSubsiteIdParam(c)
	if !ok {
		return
	}
	item, err := service.GetManagedSubsite(subsiteId, c.GetInt("id"), c.GetInt("role"), common.GetTimestamp())
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			common.ApiErrorMsg(c, "subsite not found")
			return
		}
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, item)
}

func GetManagedSubsiteActivity(c *gin.Context) {
	subsiteId, ok := managedSubsiteIdParam(c)
	if !ok {
		return
	}
	if !requireManagedSubsiteAccess(c, subsiteId) {
		return
	}
	activity, err := service.GetManagedSubsiteActivity(subsiteId, common.GetTimestamp())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, activity)
}

func ListManagedSubsiteChannels(c *gin.Context) {
	subsiteId, ok := managedSubsiteIdParam(c)
	if !ok {
		return
	}
	if !requireManagedSubsiteAccess(c, subsiteId) {
		return
	}
	items, err := service.ListManagedSubsiteChannels(subsiteId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, items)
}

func CreateManagedSubsiteChannel(c *gin.Context) {
	subsiteId, ok := managedSubsiteIdParam(c)
	if !ok {
		return
	}
	if !requireManagedSubsiteAccess(c, subsiteId) {
		return
	}
	var req dto.ManagedSubsiteChannelUpsertRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	item, err := service.CreateManagedSubsiteChannel(subsiteId, req)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordSubsiteManageLog(c, subsiteId, "created subsite channel")
	common.ApiSuccess(c, item)
}

func UpdateManagedSubsiteChannel(c *gin.Context) {
	subsiteId, ok := managedSubsiteIdParam(c)
	if !ok {
		return
	}
	if !requireManagedSubsiteAccess(c, subsiteId) {
		return
	}
	channelId, ok := managedSubsiteChannelIdParam(c)
	if !ok {
		return
	}
	var req dto.ManagedSubsiteChannelUpsertRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	item, err := service.UpdateManagedSubsiteChannel(subsiteId, channelId, req)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordSubsiteManageLog(c, subsiteId, "updated subsite channel")
	common.ApiSuccess(c, item)
}

func DeleteManagedSubsiteChannel(c *gin.Context) {
	subsiteId, ok := managedSubsiteIdParam(c)
	if !ok {
		return
	}
	if !requireManagedSubsiteAccess(c, subsiteId) {
		return
	}
	channelId, ok := managedSubsiteChannelIdParam(c)
	if !ok {
		return
	}
	if err := service.DeleteManagedSubsiteChannel(subsiteId, channelId); err != nil {
		common.ApiError(c, err)
		return
	}
	recordSubsiteManageLog(c, subsiteId, "removed subsite channel")
	common.ApiSuccess(c, gin.H{"channel_id": channelId})
}

func TestManagedSubsiteChannel(c *gin.Context) {
	subsiteId, channel, ok := managedSubsiteChannelForAction(c)
	if !ok {
		return
	}
	testUserId, err := resolveChannelTestUserID(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	tik := time.Now()
	result := testChannel(channel, testUserId, c.Query("model"), c.Query("endpoint_type"), queryBool(c, "stream"))
	if result.localErr != nil {
		resp := gin.H{
			"success": false,
			"message": result.localErr.Error(),
			"time":    0.0,
		}
		if result.newAPIError != nil {
			resp["error_code"] = result.newAPIError.GetErrorCode()
		}
		c.JSON(http.StatusOK, resp)
		return
	}

	milliseconds := time.Since(tik).Milliseconds()
	go channel.UpdateResponseTime(milliseconds)
	consumedTime := float64(milliseconds) / 1000.0
	if result.newAPIError != nil {
		c.JSON(http.StatusOK, gin.H{
			"success":    false,
			"message":    result.newAPIError.Error(),
			"time":       consumedTime,
			"error_code": result.newAPIError.GetErrorCode(),
		})
		return
	}
	recordSubsiteManageLog(c, subsiteId, "tested subsite channel")
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"time":    consumedTime,
	})
}

func UpdateManagedSubsiteChannelBalance(c *gin.Context) {
	subsiteId, channel, ok := managedSubsiteChannelForAction(c)
	if !ok {
		return
	}
	if channel.ChannelInfo.IsMultiKey {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "multi-key channels do not support balance query",
		})
		return
	}
	balance, err := updateChannelBalance(channel)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordSubsiteManageLog(c, subsiteId, "updated subsite channel balance")
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"balance": balance,
		},
	})
}

func GetManagedSubsiteChannelUpstreamModels(c *gin.Context) {
	_, channel, ok := managedSubsiteChannelForAction(c)
	if !ok {
		return
	}
	models, err := fetchChannelUpstreamModelIDs(channel)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "failed to fetch upstream models: " + err.Error(),
		})
		return
	}
	common.ApiSuccess(c, gin.H{"models": models})
}

func SyncManagedSubsiteChannelModels(c *gin.Context) {
	subsiteId, channel, ok := managedSubsiteChannelForAction(c)
	if !ok {
		return
	}
	models, err := fetchChannelUpstreamModelIDs(channel)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "failed to fetch upstream models: " + err.Error(),
		})
		return
	}
	if len(models) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "upstream returned no models",
		})
		return
	}
	channel.Models = strings.Join(models, ",")
	service.FilterSubsiteModelDisplayNames(channel)
	if err := channel.Update(); err != nil {
		common.ApiError(c, err)
		return
	}
	service.ResetProxyClientCache()
	model.InitChannelCache()
	recordSubsiteManageLog(c, subsiteId, "synced subsite channel models")
	common.ApiSuccess(c, service.ManagedSubsiteChannelInfoFromModel(*channel))
}

func UpdateManagedSubsite(c *gin.Context) {
	subsiteId, ok := managedSubsiteIdParam(c)
	if !ok {
		return
	}
	if !requireManagedSubsiteAccess(c, subsiteId) {
		return
	}
	var req dto.ManagedSubsiteUpdateRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	subsite, disabled, err := service.UpdateManagedSubsite(subsiteId, req)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordSubsiteManageLog(c, subsite.Id, "updated subsite settings")
	if disabled {
		recordSubsiteManageLog(c, subsite.Id, "disabled subsite")
	}
	if req.QuotaPolicy != nil {
		if _, err := service.UpsertManagedSubsiteQuotaPolicy(subsite.Id, *req.QuotaPolicy); err != nil {
			common.ApiError(c, err)
			return
		}
		recordSubsiteManageLog(c, subsite.Id, "updated subsite quota policy")
	}
	item, err := service.GetManagedSubsite(subsite.Id, c.GetInt("id"), c.GetInt("role"), common.GetTimestamp())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, item)
}

func UpsertManagedSubsiteQuotaPolicy(c *gin.Context) {
	subsiteId, ok := managedSubsiteIdParam(c)
	if !ok {
		return
	}
	if !requireManagedSubsiteAccess(c, subsiteId) {
		return
	}
	var req dto.SubsiteQuotaPolicyInfo
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	policy, err := service.UpsertManagedSubsiteQuotaPolicy(subsiteId, req)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordSubsiteManageLog(c, subsiteId, "updated subsite quota policy")
	common.ApiSuccess(c, service.SubsiteQuotaPolicyInfoFromModel(policy))
}

func ListManagedSubsiteMembers(c *gin.Context) {
	subsiteId, ok := managedSubsiteIdParam(c)
	if !ok {
		return
	}
	if !requireManagedSubsiteAccess(c, subsiteId) {
		return
	}
	items, err := service.ListManagedSubsiteMembers(subsiteId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, items)
}

func UpsertManagedSubsiteMember(c *gin.Context) {
	subsiteId, ok := managedSubsiteIdParam(c)
	if !ok {
		return
	}
	if !requireManagedSubsiteAccess(c, subsiteId) {
		return
	}
	var req dto.ManagedSubsiteMemberUpsertRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	item, err := service.UpsertManagedSubsiteMember(subsiteId, req)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if model.NormalizeSubsiteMemberStatus(req.Status) == model.SubsiteMemberStatusDisabled {
		recordSubsiteManageLog(c, subsiteId, "disabled subsite member")
	} else {
		recordSubsiteManageLog(c, subsiteId, "updated subsite member")
	}
	common.ApiSuccess(c, item)
}

func DeleteManagedSubsiteMember(c *gin.Context) {
	subsiteId, ok := managedSubsiteIdParam(c)
	if !ok {
		return
	}
	if !requireManagedSubsiteAccess(c, subsiteId) {
		return
	}
	userId, ok := managedSubsiteMemberUserIdParam(c)
	if !ok {
		return
	}
	if err := service.DeleteManagedSubsiteMember(subsiteId, userId); err != nil {
		common.ApiError(c, err)
		return
	}
	recordSubsiteManageLog(c, subsiteId, "removed subsite member")
	common.ApiSuccess(c, gin.H{"user_id": userId})
}

func GetPublicSubsite(c *gin.Context) {
	subsite, err := getSubsiteFromRequest(c)
	if err != nil {
		handleSubsiteLookupError(c, err)
		return
	}
	common.ApiSuccess(c, service.PublicSubsiteFromModel(subsite, common.GetTimestamp()))
}

func RegisterSubsiteUser(c *gin.Context) {
	subsite, err := getSubsiteFromRequest(c)
	if err != nil {
		handleSubsiteLookupError(c, err)
		return
	}
	decision := subsite.AccessDecision(common.GetTimestamp(), false)
	if !decision.Allowed {
		subsiteOpenAIError(c, subsiteHTTPStatus(decision.Code), decision.Message, decision.Code)
		return
	}
	if !common.RegisterEnabled {
		common.ApiErrorI18n(c, i18n.MsgUserRegisterDisabled)
		return
	}
	if !common.PasswordRegisterEnabled {
		common.ApiErrorI18n(c, i18n.MsgUserPasswordRegisterDisabled)
		return
	}

	var req SubsiteRegisterRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	user := model.User{
		Username:         req.Username,
		Password:         req.Password,
		Email:            req.Email,
		VerificationCode: req.VerificationCode,
		AffCode:          req.AffCode,
	}
	if err := common.Validate.Struct(&user); err != nil {
		common.ApiErrorI18n(c, i18n.MsgUserInputInvalid, map[string]any{"Error": err.Error()})
		return
	}
	if err := subsite.RegistrationAllowed(user.Email, req.InviteCode); err != nil {
		common.ApiError(c, err)
		return
	}
	if common.EmailVerificationEnabled {
		if user.Email == "" || user.VerificationCode == "" {
			common.ApiErrorI18n(c, i18n.MsgUserEmailVerificationRequired)
			return
		}
		if !common.VerifyCodeWithKey(user.Email, user.VerificationCode, common.EmailVerificationPurpose) {
			common.ApiErrorI18n(c, i18n.MsgUserVerificationCodeError)
			return
		}
	}

	exist, err := model.CheckUserExistOrDeleted(user.Username, user.Email)
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgDatabaseError)
		common.SysLog("CheckUserExistOrDeleted error: " + err.Error())
		return
	}
	if exist {
		common.ApiErrorI18n(c, i18n.MsgUserExists)
		return
	}

	inviterId, _ := model.GetUserIdByAffCode(user.AffCode)
	cleanUser := model.User{
		Username:    user.Username,
		Password:    user.Password,
		DisplayName: user.Username,
		InviterId:   inviterId,
		Role:        common.RoleCommonUser,
		Email:       user.Email,
	}
	if err := createSubsiteUserAndMember(subsite, &cleanUser, inviterId); err != nil {
		common.ApiError(c, err)
		return
	}
	cleanUser.FinalizeOAuthUserCreation(inviterId)

	common.ApiSuccess(c, gin.H{
		"subsite_id": subsite.Id,
		"user_id":    cleanUser.Id,
	})
}

func GetSubsiteSelfMember(c *gin.Context) {
	subsite, err := getSubsiteFromRequest(c)
	if err != nil {
		handleSubsiteLookupError(c, err)
		return
	}
	decision := subsite.AccessDecision(common.GetTimestamp(), false)
	if !decision.Allowed {
		subsiteOpenAIError(c, subsiteHTTPStatus(decision.Code), decision.Message, decision.Code)
		return
	}

	member, err := model.GetSubsiteMember(subsite.Id, c.GetInt("id"))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			common.ApiErrorMsg(c, "user is not a member of this subsite")
			return
		}
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, dto.SubsiteMemberInfo{
		SubsiteId: member.SubsiteId,
		UserId:    member.UserId,
		Role:      member.Role,
		Status:    member.Status,
		CanAccess: member.CanAccess(),
		CanManage: member.CanManage(),
	})
}

func GetSubsiteDashboard(c *gin.Context) {
	subsite, member, ok := resolveActiveSubsiteMember(c)
	if !ok {
		return
	}
	dashboard, err := service.BuildSubsiteDashboard(subsite, member, subsiteAPIBaseURL(c, subsite), common.GetTimestamp())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, dashboard)
}

func EnsureSubsiteToken(c *gin.Context) {
	subsite, _, ok := resolveActiveSubsiteMember(c)
	if !ok {
		return
	}
	token, created, err := service.EnsureSubsiteUserToken(subsite, c.GetInt("id"), c.GetString("username"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if created {
		recordSubsiteManageLog(c, subsite.Id, "created subsite API key")
	}
	common.ApiSuccess(c, gin.H{
		"created": created,
		"token":   service.SubsiteTokenInfoFromModel(token, created),
	})
}

func GetSubsiteTokenKey(c *gin.Context) {
	subsite, _, ok := resolveActiveSubsiteMember(c)
	if !ok {
		return
	}
	token, err := service.GetSubsiteUserToken(subsite.Id, c.GetInt("id"))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			common.ApiErrorMsg(c, "subsite token not found")
			return
		}
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, service.SubsiteTokenInfoFromModel(token, true))
}

func RotateSubsiteToken(c *gin.Context) {
	subsite, _, ok := resolveActiveSubsiteMember(c)
	if !ok {
		return
	}
	token, created, err := service.RotateSubsiteUserToken(subsite, c.GetInt("id"), c.GetString("username"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordSubsiteManageLog(c, subsite.Id, "rotated subsite API key")
	common.ApiSuccess(c, gin.H{
		"created": created,
		"token":   service.SubsiteTokenInfoFromModel(token, true),
	})
}

func SubsiteRelayNotReady(c *gin.Context) {
	subsiteOpenAIError(c, http.StatusNotImplemented, "Subsite API is not ready yet", model.SubsiteAccessCodeAPINotReady)
}

func managedSubsiteIdParam(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "invalid subsite id")
		return 0, false
	}
	return id, true
}

func managedSubsiteMemberUserIdParam(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(strings.TrimSpace(c.Param("user_id")))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "invalid user id")
		return 0, false
	}
	return id, true
}

func managedSubsiteChannelIdParam(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(strings.TrimSpace(c.Param("channel_id")))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "invalid channel id")
		return 0, false
	}
	return id, true
}

func managedSubsiteChannelForAction(c *gin.Context) (int64, *model.Channel, bool) {
	subsiteId, ok := managedSubsiteIdParam(c)
	if !ok {
		return 0, nil, false
	}
	if !requireManagedSubsiteAccess(c, subsiteId) {
		return 0, nil, false
	}
	channelId, ok := managedSubsiteChannelIdParam(c)
	if !ok {
		return 0, nil, false
	}
	channel, err := model.GetChannelById(channelId, true)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			common.ApiErrorMsg(c, "subsite channel not found")
			return 0, nil, false
		}
		common.ApiError(c, err)
		return 0, nil, false
	}
	if channel.SubsiteId != subsiteId {
		common.ApiErrorMsg(c, "subsite channel not found")
		return 0, nil, false
	}
	return subsiteId, channel, true
}

func queryBool(c *gin.Context, key string) bool {
	value, _ := strconv.ParseBool(strings.TrimSpace(c.Query(key)))
	return value
}

func requireManagedSubsiteAccess(c *gin.Context, subsiteId int64) bool {
	if c.GetInt("role") >= common.RoleAdminUser {
		return true
	}
	member, err := model.GetSubsiteMember(subsiteId, c.GetInt("id"))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			common.ApiErrorMsg(c, "subsite manage permission required")
			return false
		}
		common.ApiError(c, err)
		return false
	}
	if !member.CanManage() {
		common.ApiErrorMsg(c, "subsite manage permission required")
		return false
	}
	return true
}

func getSubsiteFromRequest(c *gin.Context) (*model.Subsite, error) {
	return model.GetSubsiteBySlug(c.Param("slug"))
}

func handleSubsiteLookupError(c *gin.Context, err error) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		subsiteOpenAIError(c, http.StatusNotFound, "Subsite not found", model.SubsiteAccessCodeNotFound)
		return
	}
	if _, normalizeErr := model.NormalizeSubsiteSlug(c.Param("slug")); normalizeErr != nil {
		subsiteOpenAIError(c, http.StatusNotFound, "Subsite not found", model.SubsiteAccessCodeNotFound)
		return
	}
	common.ApiError(c, err)
}

func resolveActiveSubsiteMember(c *gin.Context) (*model.Subsite, *model.SubsiteMember, bool) {
	subsite, err := getSubsiteFromRequest(c)
	if err != nil {
		handleSubsiteLookupError(c, err)
		return nil, nil, false
	}
	decision := subsite.AccessDecision(common.GetTimestamp(), false)
	if !decision.Allowed {
		subsiteOpenAIError(c, subsiteHTTPStatus(decision.Code), decision.Message, decision.Code)
		return nil, nil, false
	}
	member, err := model.GetSubsiteMember(subsite.Id, c.GetInt("id"))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			common.ApiErrorMsg(c, "user is not a member of this subsite")
			return nil, nil, false
		}
		common.ApiError(c, err)
		return nil, nil, false
	}
	if !member.CanAccess() {
		common.ApiErrorMsg(c, "subsite member is disabled")
		return nil, nil, false
	}
	return subsite, member, true
}

func subsiteAPIBaseURL(c *gin.Context, subsite *model.Subsite) string {
	proto := strings.TrimSpace(c.GetHeader("X-Forwarded-Proto"))
	if proto == "" {
		proto = strings.TrimSpace(c.GetHeader("X-Scheme"))
	}
	if proto == "" {
		if c.Request.TLS != nil {
			proto = "https"
		} else {
			proto = "http"
		}
	}
	host := c.Request.Host
	if forwardedHost := strings.TrimSpace(c.GetHeader("X-Forwarded-Host")); forwardedHost != "" {
		host = forwardedHost
	}
	return strings.TrimRight(proto, ":/") + "://" + host + "/s/" + subsite.Slug + "/v1"
}

func recordSubsiteManageLog(c *gin.Context, subsiteId int64, content string) {
	userId := c.GetInt("id")
	log := &model.Log{
		SubsiteId: subsiteId,
		UserId:    userId,
		Username:  c.GetString("username"),
		CreatedAt: common.GetTimestamp(),
		Type:      model.LogTypeManage,
		Content:   content,
	}
	if err := model.LOG_DB.Create(log).Error; err != nil {
		common.SysLog("failed to record subsite manage log: " + err.Error())
	}
}

func createSubsiteUserAndMember(subsite *model.Subsite, user *model.User, inviterId int) error {
	return model.DB.Transaction(func(tx *gorm.DB) error {
		if err := user.InsertWithTx(tx, inviterId); err != nil {
			return err
		}
		member := model.SubsiteMember{
			SubsiteId: subsite.Id,
			UserId:    user.Id,
			Role:      model.SubsiteMemberRoleMember,
			Status:    model.SubsiteMemberStatusActive,
		}
		if err := tx.Create(&member).Error; err != nil {
			return err
		}
		if !constant.GenerateDefaultToken {
			return nil
		}
		key, err := common.GenerateKey()
		if err != nil {
			return err
		}
		token := model.Token{
			SubsiteId:          subsite.Id,
			UserId:             user.Id,
			Name:               subsite.Name + " API Key (" + user.Username + ")",
			Key:                key,
			CreatedTime:        common.GetTimestamp(),
			AccessedTime:       common.GetTimestamp(),
			ExpiredTime:        -1,
			RemainQuota:        500000,
			UnlimitedQuota:     true,
			ModelLimitsEnabled: false,
		}
		if setting.DefaultUseAutoGroup {
			token.Group = "auto"
		}
		return tx.Create(&token).Error
	})
}

func subsiteOpenAIError(c *gin.Context, statusCode int, message string, code string) {
	c.JSON(statusCode, gin.H{
		"error": types.OpenAIError{
			Message: common.MessageWithRequestId(message, c.GetString(common.RequestIdKey)),
			Type:    "new_api_error",
			Code:    code,
		},
	})
	c.Abort()
}

func subsiteHTTPStatus(code string) int {
	switch code {
	case model.SubsiteAccessCodeNotFound:
		return http.StatusNotFound
	case model.SubsiteAccessCodeDisabled, model.SubsiteAccessCodeDraft, model.SubsiteAccessCodeNotStarted, model.SubsiteAccessCodeExpired:
		return http.StatusForbidden
	default:
		return http.StatusForbidden
	}
}
