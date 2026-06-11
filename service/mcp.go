package service

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/bridge"
	mcpbilling "github.com/QuantumNous/new-api/pkg/mcp/billing"
	"github.com/QuantumNous/new-api/pkg/mcp/catalog"
	mcpexecutor "github.com/QuantumNous/new-api/pkg/mcp/executor"
	mcpproxy "github.com/QuantumNous/new-api/pkg/mcp/proxy"
)

var defaultMCPExecutorRegistry = newDefaultMCPExecutorRegistry(mcpproxy.NewDefaultClient(nil))

func newDefaultMCPExecutorRegistry(proxyClient mcpproxy.Client) *mcpexecutor.Registry {
	return mcpexecutor.NewRegistry(
		mcpexecutor.NewBuiltinExecutor(),
		mcpexecutor.NewRemoteBridgeExecutor(nil),
		mcpexecutor.NewOpenAPIExecutor(nil),
		mcpexecutor.NewMCPProxyExecutor(proxyClient),
	)
}

func ListMCPTools() ([]dto.MCPTool, error) {
	tools, err := model.ListEnabledMCPTools()
	if err != nil {
		return nil, err
	}
	if len(tools) == 0 {
		return builtinMCPTools(), nil
	}
	result := make([]dto.MCPTool, 0, len(tools))
	for _, tool := range tools {
		result = append(result, mcpModelToDTO(tool))
	}
	return result, nil
}

func HasMCPTool(name string) (bool, error) {
	if name == "" {
		return false, nil
	}
	exists, err := model.EnabledMCPToolExists(name)
	if err != nil {
		return false, err
	}
	if exists {
		return true, nil
	}
	for _, tool := range catalog.BuiltinTools() {
		if tool.Name == name {
			return true, nil
		}
	}
	return false, nil
}

type MCPToolCallRequest struct {
	Context        context.Context
	UserId         int
	TokenId        int
	TokenKey       string
	TokenUnlimited bool
	TokenQuota     int
	UsingGroup     string
	RequestId      string
	RequestIP      string
	Params         dto.MCPToolCallParams
}

type MCPToolCallResponse struct {
	Result       *dto.MCPToolCallResult
	ErrorCode    int
	ErrorMessage string
	ErrorData    any
}

func setMCPExecutorRegistryForTest(registry *mcpexecutor.Registry) func() {
	previous := defaultMCPExecutorRegistry
	if registry == nil {
		registry = mcpexecutor.NewRegistry()
	}
	defaultMCPExecutorRegistry = registry
	return func() {
		defaultMCPExecutorRegistry = previous
	}
}

func CallMCPTool(req MCPToolCallRequest) (*MCPToolCallResponse, error) {
	if req.Params.Name == "" {
		return &MCPToolCallResponse{
			ErrorCode:    dto.MCPErrorCodeInvalidParams,
			ErrorMessage: "Invalid params",
			ErrorData:    "tool name is required",
		}, nil
	}

	tool, err := model.GetEnabledMCPToolByName(req.Params.Name)
	if err != nil {
		return &MCPToolCallResponse{
			ErrorCode:    dto.MCPErrorCodeUnknownTool,
			ErrorMessage: "Unknown tool",
			ErrorData: dto.MCPToolCallErrorData{
				ToolName: req.Params.Name,
				Reason:   err.Error(),
			},
		}, nil
	}
	if err := validateMCPToolArguments(tool.InputSchema, req.Params.Arguments); err != nil {
		return &MCPToolCallResponse{
			ErrorCode:    dto.MCPErrorCodeInvalidParams,
			ErrorMessage: "Invalid params",
			ErrorData: dto.MCPToolCallErrorData{
				ToolName: tool.Name,
				Reason:   err.Error(),
			},
		}, nil
	}

	startedAt := time.Now()
	requestParams := ""
	if req.Params.Arguments != nil {
		if bytes, err := common.Marshal(req.Params.Arguments); err == nil {
			requestParams = string(bytes)
		}
	}
	idempotencyKey, replayResp, err := acquireMCPToolCallIdempotency(req, *tool, requestParams)
	if err != nil {
		return nil, err
	}
	if replayResp != nil {
		return replayResp, nil
	}

	call := &model.MCPToolCall{
		UserId:        req.UserId,
		TokenId:       req.TokenId,
		ToolId:        tool.Id,
		ToolName:      tool.Name,
		RequestId:     req.RequestId,
		RequestParams: requestParams,
		RequestIP:     req.RequestIP,
		Status:        model.MCPToolCallStatusPending,
	}
	if err := model.CreateMCPToolCall(call); err != nil {
		if idempotencyKey != nil {
			_ = model.DeleteMCPToolCallIdempotencyKey(idempotencyKey.Id)
		}
		return nil, err
	}
	if idempotencyKey != nil {
		if err := model.AttachMCPToolCallIdempotencyKey(idempotencyKey.Id, call.Id); err != nil {
			return nil, err
		}
	}
	freeQuotaActive, err := model.ReserveMCPToolCallFreeQuota(call.Id, req.UserId, tool.Id, tool.FreeQuota)
	if err != nil {
		return nil, err
	}
	call.FreeUsed = freeQuotaActive

	billingResult, billingErr := mcpbilling.Precheck(mcpbilling.PrecheckInput{
		UserId:          req.UserId,
		TokenId:         req.TokenId,
		TokenKey:        req.TokenKey,
		TokenUnlimited:  req.TokenUnlimited,
		TokenQuota:      req.TokenQuota,
		UsingGroup:      req.UsingGroup,
		PricePerCall:    tool.PricePerCall,
		PriceUnit:       tool.PriceUnit,
		FreeQuotaActive: freeQuotaActive,
	})
	if billingErr != nil {
		errorCode := "BILLING_PRECHECK_FAILED"
		if precheckErr, ok := billingErr.(*mcpbilling.PrecheckError); ok {
			errorCode = precheckErr.Code
		}
		durationMS := int(time.Since(startedAt).Milliseconds())
		if err := model.UpdateMCPToolCallStatus(call.Id, model.MCPToolCallStatusError, map[string]any{
			"error_code":    errorCode,
			"error_message": billingErr.Error(),
			"duration_ms":   durationMS,
			"cost":          billingResult.Cost,
			"quota":         billingResult.Quota,
			"free_used":     false,
		}); err != nil {
			return nil, err
		}
		return &MCPToolCallResponse{
			ErrorCode:    dto.MCPErrorCodeBillingFailed,
			ErrorMessage: "Billing precheck failed",
			ErrorData: dto.MCPToolCallErrorData{
				ToolName: tool.Name,
				CallId:   call.Id,
				Reason:   billingErr.Error(),
				Quota:    billingResult.Quota,
				Cost:     billingResult.Cost,
			},
		}, nil
	}

	ctx := req.Context
	if ctx == nil {
		ctx = context.Background()
	}
	settleResult, settleErr := mcpbilling.Settle(mcpbilling.SettleInput{
		CallId:         call.Id,
		UserId:         req.UserId,
		TokenId:        req.TokenId,
		TokenUnlimited: req.TokenUnlimited,
		PriceUnit:      billingResult.PriceUnit,
		Result:         billingResult,
	})
	if settleErr != nil {
		freeUsed := false
		if billingResult.FreeUsed {
			if _, releaseErr := model.ReleaseMCPToolCallFreeQuota(call.Id); releaseErr != nil {
				settleErr = errors.Join(settleErr, releaseErr)
			} else {
				billingResult.FreeUsed = false
			}
			freeUsed = billingResult.FreeUsed
		}
		durationMS := int(time.Since(startedAt).Milliseconds())
		if err := model.UpdateMCPToolCallStatus(call.Id, model.MCPToolCallStatusError, map[string]any{
			"error_code":    "BILLING_SETTLEMENT_FAILED",
			"error_message": truncateMCPString(settleErr.Error(), 512),
			"duration_ms":   durationMS,
			"cost":          billingResult.Cost,
			"quota":         settleResult.Quota,
			"free_used":     freeUsed,
		}); err != nil {
			return nil, err
		}
		return &MCPToolCallResponse{
			ErrorCode:    dto.MCPErrorCodeBillingFailed,
			ErrorMessage: "Billing settlement failed",
			ErrorData: dto.MCPToolCallErrorData{
				ToolName: call.ToolName,
				CallId:   call.Id,
				Reason:   settleErr.Error(),
				Quota:    settleResult.Quota,
				Cost:     billingResult.Cost,
			},
		}, nil
	}
	call.Quota = settleResult.Quota
	call.SettledAt = common.GetTimestamp()
	if settleResult.Settled {
		if err := model.UpdateMCPToolCallStatus(call.Id, model.MCPToolCallStatusPending, map[string]any{
			"cost":      billingResult.Cost,
			"quota":     settleResult.Quota,
			"free_used": billingResult.FreeUsed,
		}); err != nil {
			return nil, err
		}
	}

	result, execErr := defaultMCPExecutorRegistry.Resolve(*tool).Execute(ctx, mcpexecutor.Request{
		CallId:    call.Id,
		UserId:    req.UserId,
		TokenId:   req.TokenId,
		RequestId: req.RequestId,
		RequestIP: req.RequestIP,
		Tool:      *tool,
		Arguments: req.Params.Arguments,
	})
	if execErr != nil {
		return updateMCPToolCallWithExecutorError(call, *tool, result, execErr, billingResult, req, startedAt)
	}

	return updateMCPToolCallWithExecutorResult(call, result, billingResult, req, startedAt)
}

func acquireMCPToolCallIdempotency(req MCPToolCallRequest, tool model.MCPTool, requestParams string) (*model.MCPToolCallIdempotencyKey, *MCPToolCallResponse, error) {
	requestId := strings.TrimSpace(req.RequestId)
	if requestId == "" {
		return nil, nil, nil
	}
	paramsHash := mcpToolCallRequestParamsHash(requestParams)
	record, created, err := model.CreateMCPToolCallIdempotencyKey(&model.MCPToolCallIdempotencyKey{
		IdempotencyKey:    mcpToolCallIdempotencyKey(req.UserId, req.TokenId, requestId),
		UserId:            req.UserId,
		TokenId:           req.TokenId,
		RequestId:         requestId,
		ToolName:          tool.Name,
		RequestParamsHash: paramsHash,
	})
	if err != nil {
		return nil, nil, err
	}
	if created {
		return record, nil, nil
	}
	replayResp, err := mcpToolCallReplayResponse(record, tool, paramsHash)
	return nil, replayResp, err
}

func mcpToolCallIdempotencyKey(userId int, tokenId int, requestId string) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("mcp_tool_call:v1:%d:%d:%s", userId, tokenId, requestId)))
	return fmt.Sprintf("%x", sum)
}

func mcpToolCallRequestParamsHash(requestParams string) string {
	sum := sha256.Sum256([]byte(requestParams))
	return fmt.Sprintf("%x", sum)
}

func mcpToolCallReplayResponse(record *model.MCPToolCallIdempotencyKey, tool model.MCPTool, paramsHash string) (*MCPToolCallResponse, error) {
	reason := "request is already reserved"
	callId := int64(0)
	if record != nil {
		callId = record.CallId
		if record.ToolName != tool.Name || record.RequestParamsHash != paramsHash {
			reason = "request id was already used for a different tool call"
			return duplicateMCPToolCallResponse(tool.Name, callId, reason), nil
		}
	}
	if callId <= 0 {
		return duplicateMCPToolCallResponse(tool.Name, callId, reason), nil
	}
	call, found, err := model.GetMCPToolCallById(nil, callId)
	if err != nil {
		return nil, err
	}
	if !found || call.Status == model.MCPToolCallStatusPending {
		return duplicateMCPToolCallResponse(tool.Name, callId, "request is already in progress"), nil
	}
	return duplicateMCPToolCallResponse(tool.Name, callId, "request has already been processed"), nil
}

func duplicateMCPToolCallResponse(toolName string, callId int64, reason string) *MCPToolCallResponse {
	return &MCPToolCallResponse{
		ErrorCode:    dto.MCPErrorCodeDuplicateRequest,
		ErrorMessage: "Duplicate MCP tool call request",
		ErrorData: dto.MCPToolCallErrorData{
			ToolName: toolName,
			CallId:   callId,
			Reason:   reason,
		},
	}
}

func updateMCPToolCallWithExecutorResult(call *model.MCPToolCall, result mcpexecutor.Result, billingResult mcpbilling.PrecheckResult, req MCPToolCallRequest, startedAt time.Time) (*MCPToolCallResponse, error) {
	durationMS := result.DurationMS
	if durationMS <= 0 {
		durationMS = int(time.Since(startedAt).Milliseconds())
	}
	mcpResult := &dto.MCPToolCallResult{
		Content:  result.Content,
		Metadata: result.Metadata,
	}
	if mcpResult.Content == nil {
		mcpResult.Content = []dto.MCPContentBlock{}
	}
	resultSize := result.ResultSize
	if resultSize <= 0 {
		if bytes, err := common.Marshal(mcpResult); err == nil {
			resultSize = len(bytes)
		}
	}
	summary := result.Summary
	if summary == "" {
		summary = summarizeMCPContent(result.Content)
	}
	if err := model.UpdateMCPToolCallStatus(call.Id, model.MCPToolCallStatusSuccess, map[string]any{
		"result_summary":    truncateMCPString(summary, 512),
		"metadata":          marshalMCPMetadata(result.Metadata),
		"duration_ms":       durationMS,
		"result_size":       resultSize,
		"bridge_session_id": result.BridgeSessionId,
		"target_client":     result.TargetClient,
		"cost":              billingResult.Cost,
		"quota":             billingResult.Quota,
		"free_used":         billingResult.FreeUsed,
	}); err != nil {
		return nil, err
	}
	return &MCPToolCallResponse{Result: mcpResult}, nil
}

func updateMCPToolCallWithExecutorError(call *model.MCPToolCall, tool model.MCPTool, result mcpexecutor.Result, execErr error, billingResult mcpbilling.PrecheckResult, req MCPToolCallRequest, startedAt time.Time) (*MCPToolCallResponse, error) {
	durationMS := result.DurationMS
	if durationMS <= 0 {
		durationMS = int(time.Since(startedAt).Milliseconds())
	}
	errorCode := mcpexecutor.ErrorCode(execErr)
	errorMessage := execErr.Error()
	status := model.MCPToolCallStatusError
	if errorCode == mcpexecutor.ErrorCodeTimeout {
		status = model.MCPToolCallStatusTimeout
	}
	refundReason := string(errorCode)
	if refundReason == "" {
		refundReason = "executor_failed"
	}
	refunded, refundErr := model.RefundMCPToolCallQuota(call.Id, req.UserId, req.TokenId, billingResult.Quota, req.TokenUnlimited, refundReason)
	if refundErr != nil {
		errorCode = "BILLING_REFUND_FAILED"
		errorMessage = execErr.Error() + "; refund failed: " + refundErr.Error()
		status = model.MCPToolCallStatusError
	}
	freeReleased := false
	if billingResult.FreeUsed {
		var releaseErr error
		freeReleased, releaseErr = model.ReleaseMCPToolCallFreeQuota(call.Id)
		if releaseErr != nil {
			errorCode = "FREE_QUOTA_RELEASE_FAILED"
			errorMessage = execErr.Error() + "; free quota release failed: " + releaseErr.Error()
			status = model.MCPToolCallStatusError
		}
	}
	finalCost := billingResult.Cost
	finalQuota := billingResult.Quota
	if refunded {
		finalCost = 0
		finalQuota = 0
	}
	finalFreeUsed := billingResult.FreeUsed && !freeReleased
	if err := model.UpdateMCPToolCallStatus(call.Id, status, map[string]any{
		"error_code":        errorCode,
		"error_message":     truncateMCPString(errorMessage, 512),
		"metadata":          marshalMCPMetadata(result.Metadata),
		"duration_ms":       durationMS,
		"result_size":       result.ResultSize,
		"bridge_session_id": result.BridgeSessionId,
		"target_client":     result.TargetClient,
		"cost":              finalCost,
		"quota":             finalQuota,
		"free_used":         finalFreeUsed,
	}); err != nil {
		return nil, err
	}
	return &MCPToolCallResponse{
		ErrorCode:    executorMCPErrorCode(errorCode),
		ErrorMessage: errorMessage,
		ErrorData: dto.MCPToolCallErrorData{
			ToolName: tool.Name,
			CallId:   call.Id,
			Reason:   errorMessage,
		},
	}, nil
}

func marshalMCPMetadata(metadata map[string]any) string {
	if len(metadata) == 0 {
		return "{}"
	}
	bytes, err := common.Marshal(metadata)
	if err != nil {
		return "{}"
	}
	return string(bytes)
}

func executorMCPErrorCode(errorCode string) int {
	switch errorCode {
	case mcpexecutor.ErrorCodeNotImplemented:
		return dto.MCPErrorCodeNotImplemented
	case mcpexecutor.ErrorCodeTimeout:
		return dto.MCPErrorCodeExecutorTimeout
	case "BRIDGE_CLIENT_NOT_FOUND":
		return dto.MCPErrorCodeBridgeUnavailable
	default:
		return dto.MCPErrorCodeExecutorFailed
	}
}

func summarizeMCPContent(content []dto.MCPContentBlock) string {
	for _, block := range content {
		if block.Text != "" {
			return truncateMCPString(block.Text, 512)
		}
		if block.Type != "" {
			return truncateMCPString(block.Type, 512)
		}
	}
	return ""
}

func truncateMCPString(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	if limit <= 3 {
		return string(runes[:limit])
	}
	return string(runes[:limit-3]) + "..."
}

func builtinMCPTools() []dto.MCPTool {
	defs := catalog.BuiltinTools()
	tools := make([]dto.MCPTool, 0, len(defs))
	for _, def := range defs {
		tools = append(tools, dto.MCPTool{
			Name:        def.Name,
			Description: def.Description,
			InputSchema: def.InputSchema,
		})
	}
	return tools
}

func mcpModelToDTO(tool model.MCPTool) dto.MCPTool {
	inputSchema := map[string]any{}
	if tool.InputSchema != "" {
		if err := common.UnmarshalJsonStr(tool.InputSchema, &inputSchema); err != nil {
			inputSchema = map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			}
		}
	}
	return dto.MCPTool{
		Name:        tool.Name,
		Description: tool.Description,
		InputSchema: inputSchema,
	}
}

type MCPToolListParams struct {
	Category string
	Source   string
	Status   *int
	Keyword  string
	Offset   int
	Limit    int
}

type MCPToolCallListParams struct {
	UserId          int
	TokenId         int
	ToolName        string
	Status          string
	RequestId       string
	BridgeSessionId string
	TargetClient    string
	StartTime       int64
	EndTime         int64
	Keyword         string
	Offset          int
	Limit           int
}

type MCPSummaryParams struct {
	UserId        int
	WindowSeconds int64
}

func ListMCPToolsForAdmin(params MCPToolListParams) ([]dto.MCPToolAdminItem, int64, error) {
	tools, total, err := model.ListMCPTools(model.MCPToolFilter{
		Category: params.Category,
		Source:   params.Source,
		Status:   params.Status,
		Keyword:  params.Keyword,
	}, params.Offset, params.Limit)
	if err != nil {
		return nil, 0, err
	}

	items := make([]dto.MCPToolAdminItem, 0, len(tools))
	for _, tool := range tools {
		items = append(items, mcpModelToAdminItem(tool))
	}
	return items, total, nil
}

func ListMCPToolsForUser(params MCPToolListParams) ([]dto.MCPToolAdminItem, int64, error) {
	enabled := model.MCPToolStatusEnabled
	params.Status = &enabled
	return ListMCPToolsForAdmin(params)
}

func ListMCPToolCallsForAdmin(params MCPToolCallListParams) ([]dto.MCPToolCallAdminItem, int64, error) {
	calls, total, err := model.ListMCPToolCalls(model.MCPToolCallFilter{
		UserId:          params.UserId,
		TokenId:         params.TokenId,
		ToolName:        params.ToolName,
		Status:          params.Status,
		RequestId:       params.RequestId,
		BridgeSessionId: params.BridgeSessionId,
		TargetClient:    params.TargetClient,
		StartTime:       params.StartTime,
		EndTime:         params.EndTime,
		Keyword:         params.Keyword,
	}, params.Offset, params.Limit)
	if err != nil {
		return nil, 0, err
	}
	items := make([]dto.MCPToolCallAdminItem, 0, len(calls))
	for _, call := range calls {
		items = append(items, mcpToolCallToAdminItem(call))
	}
	return items, total, nil
}

func GetMCPSummaryForAdmin(params MCPSummaryParams) (*dto.MCPSummary, error) {
	windowSeconds := normalizeMCPSummaryWindow(params.WindowSeconds)
	now := common.GetTimestamp()
	startTime := now - windowSeconds
	callFilter := model.MCPToolCallFilter{
		UserId:    params.UserId,
		StartTime: startTime,
	}
	auditFilter := model.BridgeAuditLogFilter{
		UserId:    params.UserId,
		StartTime: startTime,
	}

	toolStats, err := model.GetMCPToolStats()
	if err != nil {
		return nil, err
	}
	bridgeStats, err := model.GetBridgeClientStats(params.UserId, startTime)
	if err != nil {
		return nil, err
	}
	callStats, err := model.GetMCPToolCallStats(callFilter)
	if err != nil {
		return nil, err
	}
	auditStats, err := model.GetBridgeAuditLogStats(auditFilter)
	if err != nil {
		return nil, err
	}
	topToolStats, err := model.ListMCPTopToolStats(callFilter, 5)
	if err != nil {
		return nil, err
	}
	callErrors, err := model.ListRecentMCPToolCallErrors(callFilter, 3)
	if err != nil {
		return nil, err
	}
	auditErrors, err := model.ListRecentBridgeAuditErrors(auditFilter, 3)
	if err != nil {
		return nil, err
	}

	onlineSessions := bridge.DefaultHub.List()
	onlineClients := int64(0)
	for _, session := range onlineSessions {
		if params.UserId > 0 && session.UserId != params.UserId {
			continue
		}
		onlineClients++
	}

	summary := &dto.MCPSummary{
		WindowSeconds: windowSeconds,
		GeneratedAt:   now,
		Tools: dto.MCPSummaryToolStats{
			Total:    toolStats.TotalTools,
			Enabled:  toolStats.EnabledTools,
			Disabled: toolStats.DisabledTools,
			Remote:   toolStats.RemoteTools,
		},
		Bridge: dto.MCPSummaryBridgeStats{
			TotalClients:   bridgeStats.TotalClients,
			OnlineClients:  onlineClients,
			OfflineClients: maxInt64(bridgeStats.TotalClients-onlineClients, 0),
			ActiveClients:  bridgeStats.ActiveClients,
			OnlineSessions: onlineClients,
		},
		Calls: dto.MCPSummaryCallStats{
			TotalCalls:    callStats.TotalCalls,
			SuccessCalls:  callStats.SuccessCalls,
			ErrorCalls:    callStats.ErrorCalls,
			TimeoutCalls:  callStats.TimeoutCalls,
			PendingCalls:  callStats.PendingCalls,
			SettledCalls:  callStats.SettledCalls,
			Unsettled:     callStats.Unsettled,
			FreeCalls:     callStats.FreeCalls,
			Quota:         callStats.Quota,
			Cost:          callStats.Cost,
			ResultSize:    callStats.ResultSize,
			AvgDurationMS: callStats.AvgDurationMS,
			SuccessRate:   ratioPercent(callStats.SuccessCalls, callStats.TotalCalls),
		},
		Audit: dto.MCPSummaryAuditStats{
			TotalRequests: auditStats.TotalRequests,
			Success:       auditStats.Success,
			Error:         auditStats.Error,
			Timeout:       auditStats.Timeout,
			Pending:       auditStats.Pending,
			ResultSize:    auditStats.ResultSize,
			AvgDurationMS: auditStats.AvgDurationMS,
			SuccessRate:   ratioPercent(auditStats.Success, auditStats.TotalRequests),
		},
		TopTools:     make([]dto.MCPSummaryTopTool, 0, len(topToolStats)),
		RecentErrors: make([]dto.MCPSummaryRecentError, 0, len(callErrors)+len(auditErrors)),
	}

	for _, item := range topToolStats {
		summary.TopTools = append(summary.TopTools, dto.MCPSummaryTopTool{
			ToolName:      item.ToolName,
			Calls:         item.Calls,
			SuccessCalls:  item.SuccessCalls,
			Quota:         item.Quota,
			Cost:          item.Cost,
			AvgDurationMS: item.AvgDurationMS,
			SuccessRate:   ratioPercent(item.SuccessCalls, item.Calls),
		})
	}
	for _, call := range callErrors {
		summary.RecentErrors = append(summary.RecentErrors, dto.MCPSummaryRecentError{
			Source:       "tool_call",
			RequestId:    call.RequestId,
			ToolName:     call.ToolName,
			ClientId:     call.TargetClient,
			SessionId:    call.BridgeSessionId,
			ErrorCode:    call.ErrorCode,
			ErrorMessage: call.ErrorMessage,
			CreatedAt:    call.CreatedAt,
		})
	}
	for _, log := range auditErrors {
		summary.RecentErrors = append(summary.RecentErrors, dto.MCPSummaryRecentError{
			Source:       "bridge_audit",
			RequestId:    log.RequestId,
			ToolName:     log.ToolName,
			ClientId:     log.ClientId,
			SessionId:    log.SessionId,
			ErrorCode:    log.ErrorCode,
			ErrorMessage: log.ErrorMessage,
			CreatedAt:    log.CreatedAt,
		})
	}

	// The review queue aggregates global operations signals (proxy servers,
	// bridge clients, background tasks, high-error tools). It is only relevant
	// for the admin-wide view (UserId == 0); per-user summaries omit it.
	if params.UserId == 0 {
		reviewQueue, err := BuildMCPReviewQueue(MCPReviewQueueParams{
			WindowSeconds: windowSeconds,
			StartTime:     startTime,
		})
		if err != nil {
			return nil, err
		}
		summary.ReviewQueue = reviewQueue
	}

	return summary, nil
}

func GetMCPToolForAdmin(id int) (*dto.MCPToolAdminItem, error) {
	if id <= 0 {
		return nil, errors.New("invalid MCP tool id")
	}
	tool, err := model.GetMCPToolById(id)
	if err != nil {
		return nil, err
	}
	item := mcpModelToAdminItem(*tool)
	return &item, nil
}

func GetMCPToolForUser(id int) (*dto.MCPToolAdminItem, error) {
	if id <= 0 {
		return nil, errors.New("invalid MCP tool id")
	}
	tool, err := model.GetEnabledMCPToolById(id)
	if err != nil {
		return nil, errors.New("MCP tool not found")
	}
	item := mcpModelToAdminItem(*tool)
	return &item, nil
}

func CreateMCPToolForAdmin(req dto.MCPToolCreateRequest) (*dto.MCPToolAdminItem, error) {
	name := strings.TrimSpace(req.Name)
	if err := validateMCPToolName(name); err != nil {
		return nil, err
	}
	exists, err := model.MCPToolNameExistsUnscoped(name)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, errors.New("MCP tool name already exists")
	}

	inputSchema, err := normalizeMCPToolInputSchema(req.InputSchema)
	if err != nil {
		return nil, err
	}

	displayName := strings.TrimSpace(req.DisplayName)
	if displayName == "" {
		displayName = name
	}
	category := strings.TrimSpace(req.Category)
	if category == "" {
		category = "custom"
	}

	pricePerCall := 0.0
	if req.PricePerCall != nil {
		if *req.PricePerCall < 0 {
			return nil, errors.New("price_per_call must be greater than or equal to 0")
		}
		pricePerCall = *req.PricePerCall
	}
	priceUnit := strings.TrimSpace(req.PriceUnit)
	if priceUnit == "" {
		priceUnit = model.MCPToolPriceUnitPerCall
	}
	if err := validateMCPToolPriceUnit(priceUnit); err != nil {
		return nil, err
	}

	freeQuota := 0
	if req.FreeQuota != nil {
		if *req.FreeQuota < 0 {
			return nil, errors.New("free_quota must be greater than or equal to 0")
		}
		freeQuota = *req.FreeQuota
	}
	status := model.MCPToolStatusEnabled
	if req.Status != nil {
		if err := validateMCPToolStatus(*req.Status); err != nil {
			return nil, err
		}
		status = *req.Status
	}
	sortOrder := 0
	if req.SortOrder != nil {
		sortOrder = *req.SortOrder
	}

	tool := &model.MCPTool{
		Name:         name,
		DisplayName:  displayName,
		Description:  strings.TrimSpace(req.Description),
		Category:     category,
		Source:       model.MCPToolSourceCustom,
		InputSchema:  inputSchema,
		PricePerCall: pricePerCall,
		PriceUnit:    priceUnit,
		FreeQuota:    freeQuota,
		IsRemote:     false,
		Status:       status,
		SortOrder:    sortOrder,
	}
	if err := model.CreateMCPTool(tool); err != nil {
		return nil, err
	}
	item := mcpModelToAdminItem(*tool)
	return &item, nil
}

func UpdateMCPToolForAdmin(id int, req dto.MCPToolUpdateRequest) (*dto.MCPToolAdminItem, error) {
	if id <= 0 {
		return nil, errors.New("invalid MCP tool id")
	}
	updates := map[string]any{}
	if req.DisplayName != nil {
		updates["display_name"] = strings.TrimSpace(*req.DisplayName)
	}
	if req.Description != nil {
		updates["description"] = strings.TrimSpace(*req.Description)
	}
	if req.Category != nil {
		updates["category"] = strings.TrimSpace(*req.Category)
	}
	if req.PricePerCall != nil {
		if *req.PricePerCall < 0 {
			return nil, errors.New("price_per_call must be greater than or equal to 0")
		}
		updates["price_per_call"] = *req.PricePerCall
	}
	if req.PriceUnit != nil {
		priceUnit := strings.TrimSpace(*req.PriceUnit)
		if err := validateMCPToolPriceUnit(priceUnit); err != nil {
			return nil, err
		}
		updates["price_unit"] = priceUnit
	}
	if req.FreeQuota != nil {
		if *req.FreeQuota < 0 {
			return nil, errors.New("free_quota must be greater than or equal to 0")
		}
		updates["free_quota"] = *req.FreeQuota
	}
	if req.Status != nil {
		if err := validateMCPToolStatus(*req.Status); err != nil {
			return nil, err
		}
		updates["status"] = *req.Status
	}
	if req.SortOrder != nil {
		updates["sort_order"] = *req.SortOrder
	}
	if len(updates) == 0 {
		return GetMCPToolForAdmin(id)
	}
	tool, err := model.UpdateMCPToolFields(id, updates)
	if err != nil {
		return nil, err
	}
	item := mcpModelToAdminItem(*tool)
	return &item, nil
}

func ArchiveMCPToolForAdmin(id int) (*dto.MCPToolAdminItem, error) {
	tool, err := requireCustomMCPTool(id)
	if err != nil {
		return nil, err
	}
	if tool.Status == model.MCPToolStatusDisabled {
		item := mcpModelToAdminItem(*tool)
		return &item, nil
	}
	archived, err := model.ArchiveMCPTool(id)
	if err != nil {
		return nil, err
	}
	item := mcpModelToAdminItem(*archived)
	return &item, nil
}

func DeleteMCPToolForAdmin(id int) (*dto.MCPToolAdminItem, error) {
	tool, err := requireCustomMCPTool(id)
	if err != nil {
		return nil, err
	}
	if tool.Status != model.MCPToolStatusDisabled {
		tool, err = model.ArchiveMCPTool(id)
		if err != nil {
			return nil, err
		}
	}
	item := mcpModelToAdminItem(*tool)
	if err := model.DeleteMCPTool(id); err != nil {
		return nil, err
	}
	return &item, nil
}

func SeedBuiltinMCPToolsForAdmin() error {
	return model.SeedBuiltinMCPTools()
}

func requireCustomMCPTool(id int) (*model.MCPTool, error) {
	if id <= 0 {
		return nil, errors.New("invalid MCP tool id")
	}
	tool, err := model.GetMCPToolById(id)
	if err != nil {
		return nil, err
	}
	if tool.Source != model.MCPToolSourceCustom {
		return nil, errors.New("only custom MCP tools can use this operation")
	}
	return tool, nil
}

func validateMCPToolStatus(status int) error {
	if status != model.MCPToolStatusDisabled && status != model.MCPToolStatusEnabled {
		return errors.New("invalid status")
	}
	return nil
}

func validateMCPToolPriceUnit(priceUnit string) error {
	switch strings.TrimSpace(priceUnit) {
	case model.MCPToolPriceUnitPerCall:
		return nil
	default:
		return errors.New("unsupported price_unit; only per_call is currently supported")
	}
}

func validateMCPToolName(name string) error {
	if name == "" {
		return errors.New("name is required")
	}
	if len(name) < 3 || len(name) > 128 {
		return errors.New("name must be 3-128 chars")
	}
	parts := strings.Split(name, ".")
	if len(parts) > 2 {
		return errors.New("name can contain at most one namespace separator")
	}
	for _, part := range parts {
		if !isValidMCPToolNamePart(part) {
			return errors.New("name must contain only lowercase letters, numbers, underscores, hyphens, or one dot namespace separator")
		}
	}
	return nil
}

func isValidMCPToolNamePart(part string) bool {
	if len(part) == 0 {
		return false
	}
	for _, r := range part {
		if r >= 'a' && r <= 'z' {
			continue
		}
		if r >= '0' && r <= '9' {
			continue
		}
		if r == '_' || r == '-' {
			continue
		}
		return false
	}
	return true
}

func normalizeMCPToolInputSchema(value any) (string, error) {
	if value == nil {
		value = defaultMCPToolInputSchema()
	}
	if text, ok := value.(string); ok {
		text = strings.TrimSpace(text)
		if text == "" {
			value = defaultMCPToolInputSchema()
		} else {
			var parsed any
			if err := common.UnmarshalJsonStr(text, &parsed); err != nil {
				return "", errors.New("input_schema must be a valid JSON object")
			}
			value = parsed
		}
	}
	bytes, err := common.Marshal(value)
	if err != nil {
		return "", err
	}
	var object map[string]any
	if err := common.Unmarshal(bytes, &object); err != nil || object == nil {
		return "", errors.New("input_schema must be a valid JSON object")
	}
	return string(bytes), nil
}

func validateMCPToolArguments(rawSchema string, arguments map[string]any) error {
	rawSchema = strings.TrimSpace(rawSchema)
	if rawSchema == "" {
		return nil
	}
	var schema map[string]any
	if err := common.UnmarshalJsonStr(rawSchema, &schema); err != nil {
		return fmt.Errorf("invalid tool input_schema: %w", err)
	}
	value := any(arguments)
	if arguments == nil {
		value = map[string]any{}
	}
	return validateMCPValueAgainstSchema("$", schema, value)
}

func validateMCPValueAgainstSchema(path string, schema map[string]any, value any) error {
	if len(schema) == 0 {
		return nil
	}
	if value == nil {
		return nil
	}
	schemaType := mcpSchemaType(schema)
	if schemaType == "" && len(mcpSchemaProperties(schema)) > 0 {
		schemaType = "object"
	}
	switch schemaType {
	case "", "any":
	case "object":
		object, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("%s must be an object", path)
		}
		for _, key := range mcpSchemaRequired(schema) {
			if object[key] == nil {
				return fmt.Errorf("%s.%s is required", path, key)
			}
		}
		for key, childSchema := range mcpSchemaProperties(schema) {
			childValue, ok := object[key]
			if !ok || childValue == nil {
				continue
			}
			if err := validateMCPValueAgainstSchema(path+"."+key, childSchema, childValue); err != nil {
				return err
			}
		}
	case "array":
		items, ok := value.([]any)
		if !ok {
			return fmt.Errorf("%s must be an array", path)
		}
		itemSchema := mcpSchemaMap(schema["items"])
		for index, item := range items {
			if err := validateMCPValueAgainstSchema(fmt.Sprintf("%s[%d]", path, index), itemSchema, item); err != nil {
				return err
			}
		}
	case "string":
		if _, ok := value.(string); !ok {
			return fmt.Errorf("%s must be a string", path)
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("%s must be a boolean", path)
		}
	case "number":
		if !mcpSchemaNumber(value) {
			return fmt.Errorf("%s must be a number", path)
		}
	case "integer":
		if !mcpSchemaInteger(value) {
			return fmt.Errorf("%s must be an integer", path)
		}
	default:
		return nil
	}
	return nil
}

func mcpSchemaType(schema map[string]any) string {
	switch typed := schema["type"].(type) {
	case string:
		return strings.ToLower(strings.TrimSpace(typed))
	case []any:
		for _, item := range typed {
			if text, ok := item.(string); ok && strings.ToLower(strings.TrimSpace(text)) != "null" {
				return strings.ToLower(strings.TrimSpace(text))
			}
		}
	}
	return ""
}

func mcpSchemaProperties(schema map[string]any) map[string]map[string]any {
	raw := mcpSchemaMap(schema["properties"])
	if len(raw) == 0 {
		return nil
	}
	result := make(map[string]map[string]any, len(raw))
	for key, value := range raw {
		result[key] = mcpSchemaMap(value)
	}
	return result
}

func mcpSchemaRequired(schema map[string]any) []string {
	switch typed := schema["required"].(type) {
	case []string:
		return typed
	case []any:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			if text, ok := item.(string); ok && strings.TrimSpace(text) != "" {
				result = append(result, strings.TrimSpace(text))
			}
		}
		return result
	default:
		return nil
	}
}

func mcpSchemaMap(value any) map[string]any {
	if typed, ok := value.(map[string]any); ok {
		return typed
	}
	return nil
}

func mcpSchemaNumber(value any) bool {
	switch value.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return true
	default:
		return false
	}
}

func mcpSchemaInteger(value any) bool {
	switch typed := value.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return true
	case float32:
		return math.Trunc(float64(typed)) == float64(typed)
	case float64:
		return math.Trunc(typed) == typed
	default:
		return false
	}
}

func defaultMCPToolInputSchema() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}
}

func mcpToolCallToAdminItem(call model.MCPToolCall) dto.MCPToolCallAdminItem {
	return dto.MCPToolCallAdminItem{
		Id:              call.Id,
		UserId:          call.UserId,
		TokenId:         call.TokenId,
		ToolId:          call.ToolId,
		ToolName:        call.ToolName,
		RequestId:       call.RequestId,
		RequestParams:   call.RequestParams,
		RequestIP:       call.RequestIP,
		Status:          call.Status,
		ResultSummary:   call.ResultSummary,
		ErrorCode:       call.ErrorCode,
		ErrorMessage:    call.ErrorMessage,
		Metadata:        call.Metadata,
		DurationMS:      call.DurationMS,
		ResultSize:      call.ResultSize,
		BridgeSessionId: call.BridgeSessionId,
		TargetClient:    call.TargetClient,
		Cost:            call.Cost,
		Quota:           call.Quota,
		FreeUsed:        call.FreeUsed,
		SettledAt:       call.SettledAt,
		CreatedAt:       call.CreatedAt,
	}
}

func mcpModelToAdminItem(tool model.MCPTool) dto.MCPToolAdminItem {
	var inputSchema any = map[string]any{}
	if tool.InputSchema != "" {
		var schema map[string]any
		if err := common.UnmarshalJsonStr(tool.InputSchema, &schema); err == nil {
			inputSchema = schema
		} else {
			inputSchema = tool.InputSchema
		}
	}
	return dto.MCPToolAdminItem{
		Id:           tool.Id,
		Name:         tool.Name,
		DisplayName:  tool.DisplayName,
		Description:  tool.Description,
		Category:     tool.Category,
		Source:       tool.Source,
		PluginId:     tool.PluginId,
		OpenAPIUrl:   tool.OpenAPIUrl,
		InputSchema:  inputSchema,
		PricePerCall: tool.PricePerCall,
		PriceUnit:    tool.PriceUnit,
		FreeQuota:    tool.FreeQuota,
		IsRemote:     tool.IsRemote,
		Status:       tool.Status,
		SortOrder:    tool.SortOrder,
		CreatedAt:    tool.CreatedAt,
		UpdatedAt:    tool.UpdatedAt,
	}
}

func normalizeMCPSummaryWindow(windowSeconds int64) int64 {
	if windowSeconds <= 0 {
		return 24 * 60 * 60
	}
	if windowSeconds < 60 {
		return 60
	}
	if windowSeconds > 30*24*60*60 {
		return 30 * 24 * 60 * 60
	}
	return windowSeconds
}

func ratioPercent(numerator int64, denominator int64) float64 {
	if denominator <= 0 {
		return 0
	}
	return float64(numerator) / float64(denominator) * 100
}

func maxInt64(a int64, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
