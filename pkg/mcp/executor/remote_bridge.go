package executor

import (
	"context"
	"errors"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/bridge"
)

const defaultRemoteBridgeTimeout = 60 * time.Second
const remoteBridgeTimeoutEnv = "MCP_REMOTE_BRIDGE_TIMEOUT_MS"

type RemoteBridgeExecutor struct {
	Hub     *bridge.Hub
	Timeout time.Duration
}

func NewRemoteBridgeExecutor(hub *bridge.Hub) *RemoteBridgeExecutor {
	if hub == nil {
		hub = bridge.DefaultHub
	}
	return &RemoteBridgeExecutor{
		Hub:     hub,
		Timeout: configuredRemoteBridgeTimeout(),
	}
}

func configuredRemoteBridgeTimeout() time.Duration {
	timeoutMs := common.GetEnvOrDefault(remoteBridgeTimeoutEnv, int(defaultRemoteBridgeTimeout/time.Millisecond))
	if timeoutMs <= 0 {
		return defaultRemoteBridgeTimeout
	}
	return time.Duration(timeoutMs) * time.Millisecond
}

func (e *RemoteBridgeExecutor) Supports(tool model.MCPTool) bool {
	return tool.IsRemote
}

func (e *RemoteBridgeExecutor) Execute(ctx context.Context, req Request) (Result, error) {
	if e == nil || e.Hub == nil {
		return Result{}, &ExecutionError{
			Code:    ErrorCodeFailed,
			Message: "remote bridge executor is not configured",
			Err:     bridge.ErrClientUnavailable,
		}
	}
	session, ok := e.Hub.SelectSession(req.UserId, "", req.Tool.Name)
	if !ok {
		return Result{}, &ExecutionError{
			Code:    "BRIDGE_CLIENT_NOT_FOUND",
			Message: "No online bridge client supports this tool",
			Err:     bridge.ErrClientNotFound,
		}
	}
	if ctx == nil {
		ctx = context.Background()
	}
	timeout := e.Timeout
	if timeout <= 0 {
		timeout = defaultRemoteBridgeTimeout
	}
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	startedAt := time.Now()
	requestBody := ""
	if bytes, err := common.Marshal(req.Arguments); err == nil {
		requestBody = string(bytes)
	}
	audit := &model.BridgeAuditLog{
		RequestId:   req.RequestId,
		SessionId:   session.SessionId,
		ClientId:    session.ClientId,
		UserId:      req.UserId,
		TokenId:     req.TokenId,
		ToolName:    req.Tool.Name,
		RequestBody: requestBody,
		Status:      model.BridgeAuditStatusPending,
	}
	if err := model.CreateBridgeAuditLog(audit); err != nil {
		return Result{}, err
	}

	response, err := e.Hub.ForwardToolCall(callCtx, session.SessionId, bridge.ToolCallRequest{
		Id:        req.RequestId,
		ToolName:  req.Tool.Name,
		Arguments: req.Arguments,
	})
	durationMS := int(time.Since(startedAt).Milliseconds())
	if err != nil {
		status := model.BridgeAuditStatusError
		errorCode := ErrorCodeFailed
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, bridge.ErrClientDisconnected) {
			status = model.BridgeAuditStatusTimeout
			errorCode = ErrorCodeTimeout
		}
		if clientErr, ok := err.(*bridge.ClientError); ok && clientErr.Code != "" {
			errorCode = clientErr.Code
		}
		_ = model.UpdateBridgeAuditLogStatus(audit.Id, status, map[string]any{
			"error_code":    errorCode,
			"error_message": err.Error(),
			"duration_ms":   durationMS,
		})
		return Result{
				DurationMS:      durationMS,
				BridgeSessionId: session.SessionId,
				TargetClient:    session.ClientId,
			}, &ExecutionError{
				Code:    errorCode,
				Message: err.Error(),
				Err:     err,
			}
	}

	resultSize := response.Result.ResultSize
	if resultSize <= 0 {
		if bytes, err := common.Marshal(response.Result); err == nil {
			resultSize = len(bytes)
		}
	}
	_ = model.UpdateBridgeAuditLogStatus(audit.Id, model.BridgeAuditStatusSuccess, map[string]any{
		"duration_ms": durationMS,
		"result_size": resultSize,
	})
	resultDurationMS := response.Result.DurationMS
	if resultDurationMS <= 0 {
		resultDurationMS = durationMS
	}
	return Result{
		Content:         response.Result.Content,
		Metadata:        response.Result.Metadata,
		Summary:         response.Result.Summary,
		DurationMS:      resultDurationMS,
		ResultSize:      resultSize,
		BridgeSessionId: response.Session.SessionId,
		TargetClient:    response.Session.ClientId,
	}, nil
}
