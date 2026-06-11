package controller

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

func GetBillingEvents(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	params, err := billingEventParamsFromQuery(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	params.Offset = pageInfo.GetStartIdx()
	params.Limit = pageInfo.GetPageSize()

	items, total, err := service.ListBillingEvents(params)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

func GetBillingEventSummary(c *gin.Context) {
	params, err := billingEventParamsFromQuery(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	result, err := service.GetBillingEventSummary(params)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

func billingEventParamsFromQuery(c *gin.Context) (service.BillingEventListParams, error) {
	tokenId, err := parseOptionalIntQuery(c, "token_id")
	if err != nil {
		return service.BillingEventListParams{}, err
	}
	startTime, err := parseOptionalInt64Query(c, "start_time")
	if err != nil {
		return service.BillingEventListParams{}, err
	}
	endTime, err := parseOptionalInt64Query(c, "end_time")
	if err != nil {
		return service.BillingEventListParams{}, err
	}

	userId := c.GetInt("id")
	if model.IsAdmin(userId) && c.Query("scope") == "all" {
		userId = 0
	}
	return service.BillingEventListParams{
		UserId:        userId,
		TokenId:       tokenId,
		Source:        c.Query("source"),
		SourceId:      c.Query("source_id"),
		EventType:     c.Query("event_type"),
		Status:        c.Query("status"),
		RequestId:     c.Query("request_id"),
		BillingSource: c.Query("billing_source"),
		UsageKind:     c.Query("usage_kind"),
		StartTime:     startTime,
		EndTime:       endTime,
		Keyword:       c.Query("keyword"),
	}, nil
}

func BackfillBillingEvents(c *gin.Context) {
	var req dto.BillingEventBackfillRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	result, err := service.BackfillBillingEvents(service.BillingEventBackfillParams{
		Sources: req.Sources,
		Limit:   req.Limit,
		DryRun:  req.DryRun,
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

func GetBillingEventHealth(c *gin.Context) {
	limit, err := parseOptionalIntQuery(c, "limit")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	result, err := service.GetBillingEventHealth(service.BillingEventReconciliationParams{
		Sources: c.QueryArray("sources"),
		Limit:   limit,
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

func GetBillingEventSourceMatrix(c *gin.Context) {
	common.ApiSuccess(c, service.GetBillingEventSourceMatrix())
}

func GetBillingEventRelationHealth(c *gin.Context) {
	limit, err := parseOptionalIntQuery(c, "limit")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	cursor, err := parseOptionalInt64Query(c, "cursor")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	result, err := service.GetBillingEventRelationHealth(service.BillingEventRelationMaintenanceParams{
		Limit:  limit,
		Cursor: cursor,
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

func BackfillBillingEventRelations(c *gin.Context) {
	var req dto.BillingEventRelationBackfillRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	result, err := service.BackfillBillingEventRelations(service.BillingEventRelationMaintenanceParams{
		Limit:  req.Limit,
		Cursor: req.Cursor,
		DryRun: req.DryRun,
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

func RepairBillingEventRelations(c *gin.Context) {
	var req dto.BillingEventRelationRepairRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	result, err := service.RepairSelectedBillingEventRelations(service.BillingEventRelationSelectedRepairParams{
		DryRun: req.DryRun,
		Items:  req.Items,
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

func CleanupBillingEventRelationOrphans(c *gin.Context) {
	var req dto.BillingEventRelationOrphanCleanupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	result, err := service.CleanupBillingEventRelationOrphans(service.BillingEventRelationMaintenanceParams{
		DryRun: req.DryRun,
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

func GetBillingEventRelationInspection(c *gin.Context) {
	common.ApiSuccess(c, service.GetBillingEventRelationInspectionStatus())
}

func GetBillingEventRelationInspectionRuns(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	items, total, err := service.ListBillingEventRelationInspectionRunsPage(pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

func UpdateBillingEventRelationInspection(c *gin.Context) {
	var req dto.BillingEventRelationInspectionSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	result, err := service.UpdateBillingEventRelationInspectionSettings(req)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

func RunBillingEventRelationInspection(c *gin.Context) {
	result, err := service.RunBillingEventRelationInspectionOnce(true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

func ReconcileBillingEvents(c *gin.Context) {
	var req dto.BillingEventReconciliationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	result, err := service.ReconcileBillingEvents(service.BillingEventReconciliationParams{
		Sources: req.Sources,
		Limit:   req.Limit,
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

func GetBillingEventReconciliationMismatches(c *gin.Context) {
	var req dto.BillingEventReconciliationMismatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	result, err := service.ListBillingEventReconciliationMismatches(service.BillingEventReconciliationMismatchParams{
		Sources:     req.Sources,
		Limit:       req.Limit,
		DetailLimit: req.DetailLimit,
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

func GetBillingEventReconciliationMissing(c *gin.Context) {
	var req dto.BillingEventReconciliationMissingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	result, err := service.ListBillingEventReconciliationMissing(service.BillingEventReconciliationMissingParams{
		Sources:     req.Sources,
		Limit:       req.Limit,
		DetailLimit: req.DetailLimit,
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

func RepairBillingEventReconciliationMismatch(c *gin.Context) {
	var req dto.BillingEventReconciliationRepairRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	result, err := service.RepairBillingEventReconciliationMismatch(service.BillingEventReconciliationRepairParams{
		Source:   req.Source,
		Label:    req.Label,
		Limit:    req.Limit,
		Reason:   req.Reason,
		AdminId:  c.GetInt("id"),
		ActualId: req.ActualId,
		Expected: req.Expected,
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

func BackfillBillingEventReconciliationMissing(c *gin.Context) {
	var req dto.BillingEventReconciliationBackfillMissingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	result, err := service.BackfillBillingEventReconciliationMissing(service.BillingEventReconciliationBackfillMissingParams{
		Source:   req.Source,
		Label:    req.Label,
		Limit:    req.Limit,
		Reason:   req.Reason,
		AdminId:  c.GetInt("id"),
		Expected: req.Expected,
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}
