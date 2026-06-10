package controller

import (
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

func GetMCPProxyServers(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	items, total, err := service.ListMCPProxyServersForAdmin(service.MCPProxyServerListParams{
		Transport: c.Query("transport"),
		Status:    c.Query("status"),
		Keyword:   c.Query("keyword"),
		Offset:    pageInfo.GetStartIdx(),
		Limit:     pageInfo.GetPageSize(),
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

func GetMCPProxyTrends(c *gin.Context) {
	proxyServerId, err := parseOptionalIntQuery(c, "proxy_server_id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	proxyToolId, err := parseOptionalInt64Query(c, "proxy_tool_id")
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
	bucketSeconds, err := parseOptionalInt64Query(c, "bucket_seconds")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	item, err := service.GetMCPProxyTrendsForAdmin(service.MCPProxyTrendParams{
		ProxyServerId: proxyServerId,
		ProxyToolId:   proxyToolId,
		Status:        c.Query("status"),
		StartTime:     startTime,
		EndTime:       endTime,
		BucketSeconds: bucketSeconds,
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, item)
}

func CreateMCPProxyServer(c *gin.Context) {
	var req dto.MCPProxyServerCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	item, err := service.CreateMCPProxyServerForAdmin(req)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, item)
}

func GetMCPProxyServer(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	item, err := service.GetMCPProxyServerForAdmin(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, item)
}

func UpdateMCPProxyServer(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var req dto.MCPProxyServerUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	item, err := service.UpdateMCPProxyServerForAdmin(id, req)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, item)
}

func DeleteMCPProxyServer(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	item, err := service.ArchiveMCPProxyServerForAdmin(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, item)
}

func TestMCPProxyServer(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	item, err := service.TestMCPProxyServerForAdmin(c.Request.Context(), id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, item)
}

func DiscoverMCPProxyServerTools(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	item, err := service.DiscoverMCPProxyServerToolsForAdmin(c.Request.Context(), id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, item)
}

func GetMCPProxyServerTools(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	items, err := service.ListMCPProxyServerToolsForAdmin(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, items)
}

func GetMCPProxyServerHealth(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	windowSeconds, err := parseOptionalInt64Query(c, "window_seconds")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	item, err := service.GetMCPProxyServerHealthForAdmin(id, service.MCPProxyServerHealthParams{
		WindowSeconds: windowSeconds,
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, item)
}

func GetMCPProxyServerDiscoveryEvents(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo := common.GetPageQuery(c)
	items, total, err := service.ListMCPProxyDiscoveryEventsForAdmin(id, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

func GetMCPProxyHealthCheck(c *gin.Context) {
	common.ApiSuccess(c, service.GetMCPProxyHealthCheckStatus())
}

func UpdateMCPProxyHealthCheck(c *gin.Context) {
	var req dto.MCPProxyHealthCheckSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	item, err := service.UpdateMCPProxyHealthCheckSettings(req)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, item)
}

func RunMCPProxyHealthCheck(c *gin.Context) {
	var req dto.MCPProxyHealthCheckRunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	item, err := service.RunMCPProxyHealthCheckOnce(c.Request.Context(), true, req)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, item)
}

func GetMCPProxyHeartbeat(c *gin.Context) {
	common.ApiSuccess(c, service.GetMCPProxyHeartbeatStatus())
}

func UpdateMCPProxyHeartbeat(c *gin.Context) {
	var req dto.MCPProxyHeartbeatSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	item, err := service.UpdateMCPProxyHeartbeatSettings(req)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, item)
}

func RunMCPProxyHeartbeat(c *gin.Context) {
	item, err := service.RunMCPProxyHeartbeatOnce(c.Request.Context(), true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, item)
}

func GetMCPProxyTools(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	proxyServerId, err := parseOptionalIntQuery(c, "proxy_server_id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	items, total, err := service.ListMCPProxyToolsForAdmin(service.MCPProxyToolListParams{
		ProxyServerId: proxyServerId,
		Status:        c.Query("status"),
		SchemaHash:    c.Query("schema_hash"),
		Keyword:       c.Query("keyword"),
		Offset:        pageInfo.GetStartIdx(),
		Limit:         pageInfo.GetPageSize(),
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

func GetMCPProxyTool(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	item, err := service.GetMCPProxyToolForAdmin(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, item)
}

func GetMCPProxyToolHealth(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	windowSeconds, err := parseOptionalInt64Query(c, "window_seconds")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	item, err := service.GetMCPProxyToolHealthForAdmin(id, service.MCPProxyServerHealthParams{
		WindowSeconds: windowSeconds,
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, item)
}

func UpdateMCPProxyTool(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var req dto.MCPProxyToolUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	item, err := service.UpdateMCPProxyToolForAdmin(id, req)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, item)
}
