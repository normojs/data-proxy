package controller

import (
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

func GetMCPTools(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	var status *int
	if statusQuery := c.Query("status"); statusQuery != "" {
		statusValue, err := strconv.Atoi(statusQuery)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		status = &statusValue
	}

	params := service.MCPToolListParams{
		Category: c.Query("category"),
		Source:   c.Query("source"),
		Status:   status,
		Keyword:  c.Query("keyword"),
		Offset:   pageInfo.GetStartIdx(),
		Limit:    pageInfo.GetPageSize(),
	}
	var items []dto.MCPToolAdminItem
	var total int64
	var err error
	if model.IsAdmin(c.GetInt("id")) {
		items, total, err = service.ListMCPToolsForAdmin(params)
	} else {
		items, total, err = service.ListMCPToolsForUser(params)
	}
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

func GetMCPTool(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var item *dto.MCPToolAdminItem
	if model.IsAdmin(c.GetInt("id")) {
		item, err = service.GetMCPToolForAdmin(id)
	} else {
		item, err = service.GetMCPToolForUser(id)
	}
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, item)
}

func CreateMCPTool(c *gin.Context) {
	var req dto.MCPToolCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	item, err := service.CreateMCPToolForAdmin(req)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, item)
}

func GetMCPSummary(c *gin.Context) {
	windowSeconds, err := parseOptionalInt64Query(c, "window_seconds")
	if err != nil {
		common.ApiError(c, err)
		return
	}

	userId := c.GetInt("id")
	if model.IsAdmin(userId) && c.Query("scope") == "all" {
		userId = 0
	}
	summary, err := service.GetMCPSummaryForAdmin(service.MCPSummaryParams{
		UserId:        userId,
		WindowSeconds: windowSeconds,
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, summary)
}

func GetMCPToolCalls(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	tokenId, err := parseOptionalIntQuery(c, "token_id")
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

	userId := c.GetInt("id")
	if model.IsAdmin(userId) && c.Query("scope") == "all" {
		userId = 0
	}
	items, total, err := service.ListMCPToolCallsForAdmin(service.MCPToolCallListParams{
		UserId:          userId,
		TokenId:         tokenId,
		ToolName:        c.Query("tool_name"),
		Status:          c.Query("status"),
		RequestId:       c.Query("request_id"),
		BridgeSessionId: c.Query("bridge_session_id"),
		TargetClient:    c.Query("target_client"),
		StartTime:       startTime,
		EndTime:         endTime,
		Keyword:         c.Query("keyword"),
		Offset:          pageInfo.GetStartIdx(),
		Limit:           pageInfo.GetPageSize(),
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

func UpdateMCPTool(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var req dto.MCPToolUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	item, err := service.UpdateMCPToolForAdmin(id, req)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, item)
}

func ArchiveMCPTool(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	item, err := service.ArchiveMCPToolForAdmin(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, item)
}

func DeleteMCPTool(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	item, err := service.DeleteMCPToolForAdmin(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, item)
}

func SeedMCPTools(c *gin.Context) {
	if err := service.SeedBuiltinMCPToolsForAdmin(); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"seeded": true})
}
