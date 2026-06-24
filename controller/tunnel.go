package controller

import (
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
)

func ListTunnelApps(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	items, total, err := service.ListTunnelApps(service.TunnelAppListParams{
		UserId:  c.GetInt("id"),
		AppType: c.Query("app_type"),
		Status:  c.Query("status"),
		Keyword: c.Query("keyword"),
		Offset:  pageInfo.GetStartIdx(),
		Limit:   pageInfo.GetPageSize(),
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

func AdminListTunnelApps(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	userId, _ := strconv.Atoi(c.Query("user_id"))
	items, total, err := service.ListTunnelApps(service.TunnelAppListParams{
		UserId:  userId,
		AppType: c.Query("app_type"),
		Status:  c.Query("status"),
		Keyword: c.Query("keyword"),
		Offset:  pageInfo.GetStartIdx(),
		Limit:   pageInfo.GetPageSize(),
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

func CreateTunnelApp(c *gin.Context) {
	var req dto.TunnelAppCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	item, err := service.CreateTunnelAppForUser(c.GetInt("id"), req)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, item)
}

func GetTunnelApp(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	item, err := service.GetTunnelAppForUser(id, c.GetInt("id"), false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, item)
}

func ListTunnelConnections(c *gin.Context) {
	appId, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo := common.GetPageQuery(c)
	items, total, err := service.ListTunnelConnections(service.TunnelConnectionListParams{
		UserId:  c.GetInt("id"),
		AppId:   appId,
		Status:  c.Query("status"),
		Keyword: c.Query("keyword"),
		Offset:  pageInfo.GetStartIdx(),
		Limit:   pageInfo.GetPageSize(),
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

func ListTunnelSessions(c *gin.Context) {
	appId, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	connectionId, err := parseOptionalInt64Query(c, "connection_id")
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
	pageInfo := common.GetPageQuery(c)
	items, total, err := service.ListTunnelSessions(service.TunnelSessionListParams{
		UserId:       c.GetInt("id"),
		AppId:        appId,
		ConnectionId: connectionId,
		Status:       c.Query("status"),
		SessionId:    c.Query("session_id"),
		Keyword:      c.Query("keyword"),
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

func ListTunnelAuditLogs(c *gin.Context) {
	appId, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	connectionId, err := parseOptionalInt64Query(c, "connection_id")
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
	pageInfo := common.GetPageQuery(c)
	items, total, err := service.ListTunnelAuditLogs(service.TunnelAuditLogListParams{
		UserId:       c.GetInt("id"),
		AppId:        appId,
		ConnectionId: connectionId,
		Action:       c.Query("action"),
		Decision:     c.Query("decision"),
		RequestId:    c.Query("request_id"),
		ToolName:     c.Query("tool_name"),
		SessionId:    c.Query("session_id"),
		Keyword:      c.Query("keyword"),
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

func CreateTunnelConnection(c *gin.Context) {
	appId, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var req dto.TunnelConnectionCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	item, err := service.CreateTunnelConnectionForUser(appId, c.GetInt("id"), req)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, item)
}

func UpdateTunnelConnection(c *gin.Context) {
	appId, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	connectionId, err := strconv.ParseInt(c.Param("connection_id"), 10, 64)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var req dto.TunnelConnectionUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	item, err := service.UpdateTunnelConnectionForUser(appId, connectionId, c.GetInt("id"), req)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, item)
}

func RevokeTunnelConnection(c *gin.Context) {
	appId, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	connectionId, err := strconv.ParseInt(c.Param("connection_id"), 10, 64)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	item, err := service.RevokeTunnelConnectionForUser(appId, connectionId, c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, item)
}

func EnsureTunnelAgentSetup(c *gin.Context) {
	appId, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var req dto.TunnelAgentSetupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	item, err := service.EnsureTunnelAgentSetup(service.TunnelAgentSetupParams{
		UserId:  c.GetInt("id"),
		AppId:   appId,
		BaseURL: tunnelServerBaseURL(c),
		Request: req,
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, item)
}

func AdminGetTunnelApp(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	item, err := service.GetTunnelAppForUser(id, c.GetInt("id"), true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, item)
}

func tunnelServerBaseURL(c *gin.Context) string {
	base := strings.TrimRight(strings.TrimSpace(system_setting.ServerAddress), "/")
	if base == "" && c != nil && c.Request != nil {
		scheme := "http"
		if c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") {
			scheme = "https"
		}
		if c.Request.Host != "" {
			base = scheme + "://" + c.Request.Host
		}
	}
	if base == "" {
		base = "http://localhost:3000"
	}
	return strings.TrimRight(base, "/")
}

func AdminUpdateTunnelApp(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var req dto.TunnelAppAdminUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	item, err := service.UpdateTunnelAppForAdmin(id, c.GetInt("id"), req)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, item)
}
