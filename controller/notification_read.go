package controller

import (
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

type MarkNotificationsReadRequest struct {
	AnnouncementKeys           []string `json:"announcement_keys"`
	EnterpriseNotificationKeys []string `json:"enterprise_notification_keys"`
	EnterpriseQuotaRequestKeys []string `json:"enterprise_quota_request_keys"`
	ConnectedAppRequestKeys    []string `json:"connected_app_request_keys"`
}

func GetNotificationReadState(c *gin.Context) {
	keys, err := model.ListUserAnnouncementReadKeys(c.GetInt("id"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	enterpriseKeys, err := model.ListEnterpriseNotificationReadKeys(c.GetInt("id"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"announcement_keys":             keys,
			"enterprise_notification_keys":  enterpriseKeys,
			"enterprise_quota_request_keys": enterpriseKeys,
			"connected_app_request_keys":    enterpriseKeys,
		},
	})
}

func MarkNotificationsRead(c *gin.Context) {
	var req MarkNotificationsReadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的参数",
		})
		return
	}
	keys, err := model.MarkUserAnnouncementsRead(c.GetInt("id"), req.AnnouncementKeys)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	enterpriseInputKeys := append(req.EnterpriseNotificationKeys, req.EnterpriseQuotaRequestKeys...)
	enterpriseInputKeys = append(enterpriseInputKeys, req.ConnectedAppRequestKeys...)
	enterpriseKeys, err := model.MarkEnterpriseNotificationsRead(c.GetInt("id"), enterpriseInputKeys)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"announcement_keys":             keys,
			"enterprise_notification_keys":  enterpriseKeys,
			"enterprise_quota_request_keys": enterpriseKeys,
			"connected_app_request_keys":    enterpriseKeys,
		},
	})
}

func ListEnterpriseQuotaRequestNotifications(c *gin.Context) {
	if !common.EnterpriseGovernanceEnabled {
		common.ApiSuccess(c, gin.H{
			"items":        []service.EnterpriseQuotaRequestNotification{},
			"unread_count": 0,
		})
		return
	}
	if err := model.EnsureDefaultEnterprise(); err != nil {
		common.ApiError(c, err)
		return
	}
	enterprise, err := model.GetDefaultEnterprise()
	if err != nil {
		common.ApiError(c, err)
		return
	}

	readKeys, err := model.ListEnterpriseNotificationReadKeys(c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	options := service.EnterpriseQuotaRequestNotificationListOptions{
		Page:       parsePositiveNotificationQuery(c.Query("page")),
		Limit:      parsePositiveNotificationLimit(c),
		UnreadOnly: c.Query("unread_only") == "true" || c.Query("unread_only") == "1",
	}
	access, err := service.EnterpriseAccessForUser(c.GetInt("id"), c.GetInt("role"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	canReview := access.HasCapability(service.EnterpriseCapabilityQuotaApprove)
	if access.HasDepartmentScope() {
		options.ReviewOrgUnitIds = access.ScopedOrgUnitIds
	}
	result, err := service.ListEnterpriseQuotaRequestNotifications(enterprise.Id, c.GetInt("id"), canReview, readKeys, options)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"items":        result.Items,
		"unread_count": result.UnreadCount,
		"page":         result.Page,
		"page_size":    result.PageSize,
		"has_more":     result.HasMore,
	})
}

func parsePositiveNotificationQuery(value string) int {
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return 0
	}
	return parsed
}

func parsePositiveNotificationLimit(c *gin.Context) int {
	if value := c.Query("page_size"); value != "" {
		return parsePositiveNotificationQuery(value)
	}
	return parsePositiveNotificationQuery(c.Query("limit"))
}

func MarkEnterpriseQuotaRequestNotificationsRead(c *gin.Context) {
	var req MarkNotificationsReadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的参数",
		})
		return
	}
	inputKeys := append(req.EnterpriseNotificationKeys, req.EnterpriseQuotaRequestKeys...)
	keys, err := model.MarkEnterpriseNotificationsRead(c.GetInt("id"), inputKeys)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"enterprise_notification_keys":  keys,
		"enterprise_quota_request_keys": keys,
	})
}
