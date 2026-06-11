package executor

import (
	"context"
	"errors"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/bridge"
	"github.com/QuantumNous/new-api/pkg/bridgepolicy"
	mcpproxy "github.com/QuantumNous/new-api/pkg/mcp/proxy"
)

const defaultMCPProxyTimeout = 30 * time.Second

type MCPProxyExecutor struct {
	Client  mcpproxy.Client
	Timeout time.Duration
}

func NewMCPProxyExecutor(client mcpproxy.Client) *MCPProxyExecutor {
	if client == nil {
		client = mcpproxy.UnconfiguredClient{}
	}
	return &MCPProxyExecutor{
		Client:  client,
		Timeout: defaultMCPProxyTimeout,
	}
}

func (e *MCPProxyExecutor) Supports(tool model.MCPTool) bool {
	return tool.Source == model.MCPToolSourceMCPProxy
}

func (e *MCPProxyExecutor) Execute(ctx context.Context, req Request) (Result, error) {
	if e == nil || e.Client == nil {
		return Result{}, &ExecutionError{
			Code:    ErrorCodeFailed,
			Message: "mcp proxy executor is not configured",
			Err:     mcpproxy.ErrClientNotConfigured,
		}
	}
	proxyTool, err := model.GetMCPProxyToolByMCPToolId(req.Tool.Id)
	if err != nil {
		return Result{}, &ExecutionError{
			Code:    ErrorCodeFailed,
			Message: "mcp proxy tool mapping not found",
			Err:     err,
		}
	}
	if proxyTool.Status != model.MCPProxyToolStatusEnabled {
		return Result{}, &ExecutionError{
			Code:    ErrorCodeFailed,
			Message: "mcp proxy tool is not enabled",
		}
	}
	server, err := model.GetMCPProxyServerById(proxyTool.ProxyServerId)
	if err != nil {
		return Result{}, &ExecutionError{
			Code:    ErrorCodeFailed,
			Message: "mcp proxy server not found",
			Err:     err,
		}
	}
	if server.Status != model.MCPProxyServerStatusEnabled {
		return Result{}, &ExecutionError{
			Code:    ErrorCodeFailed,
			Message: "mcp proxy server is not enabled",
		}
	}

	if ctx == nil {
		ctx = context.Background()
	}
	timeout := e.Timeout
	if timeout <= 0 {
		timeout = defaultMCPProxyTimeout
	}
	if server.TimeoutMS > 0 {
		timeout = time.Duration(server.TimeoutMS) * time.Millisecond
	}
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	startedAt := time.Now()
	callResult, err := e.Client.CallTool(callCtx, *server, mcpproxy.CallRequest{
		ToolName:  proxyTool.DownstreamToolName,
		Arguments: req.Arguments,
		RequestId: req.RequestId,
		UserId:    req.UserId,
		TokenId:   req.TokenId,
	})
	durationMS := callResult.DurationMS
	if durationMS <= 0 {
		durationMS = int(time.Since(startedAt).Milliseconds())
	}
	resultSize := callResult.ResultSize
	if resultSize <= 0 {
		if bytes, marshalErr := common.Marshal(callResult); marshalErr == nil {
			resultSize = len(bytes)
		}
	}
	metadata := mcpProxyExecutionMetadata(callResult.Metadata, *server, *proxyTool)
	if err != nil {
		code := ErrorCodeFailed
		var clientErr *bridge.ClientError
		if policyCode := bridgepolicy.ErrorCode(err); policyCode != "" {
			code = policyCode
		} else if errors.As(err, &clientErr) && clientErr.Code != "" {
			code = clientErr.Code
		} else if errors.Is(err, context.DeadlineExceeded) || errors.Is(callCtx.Err(), context.DeadlineExceeded) {
			code = ErrorCodeTimeout
		} else if errors.Is(err, bridge.ErrClientDisconnected) {
			code = "BRIDGE_CLIENT_DISCONNECTED"
		} else if errors.Is(err, bridge.ErrClientNotFound) || errors.Is(err, bridge.ErrClientUnavailable) {
			code = "BRIDGE_CLIENT_NOT_FOUND"
		}
		return Result{
				Metadata:        metadata,
				DurationMS:      durationMS,
				ResultSize:      resultSize,
				BridgeSessionId: callResult.BridgeSessionId,
				TargetClient:    callResult.TargetClient,
			}, &ExecutionError{
				Code:    code,
				Message: err.Error(),
				Err:     err,
			}
	}
	if server.MaxResultSize > 0 && resultSize > server.MaxResultSize {
		return Result{
				Metadata:        metadata,
				DurationMS:      durationMS,
				ResultSize:      resultSize,
				BridgeSessionId: callResult.BridgeSessionId,
				TargetClient:    callResult.TargetClient,
			}, &ExecutionError{
				Code:    ErrorCodeFailed,
				Message: "mcp proxy result exceeds max_result_size",
			}
	}
	return Result{
		Content:         callResult.Content,
		Metadata:        metadata,
		Summary:         callResult.Summary,
		DurationMS:      durationMS,
		ResultSize:      resultSize,
		BridgeSessionId: callResult.BridgeSessionId,
		TargetClient:    callResult.TargetClient,
	}, nil
}

func mcpProxyExecutionMetadata(metadata map[string]any, server model.MCPProxyServer, proxyTool model.MCPProxyTool) map[string]any {
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadata["proxy_server_id"] = server.Id
	metadata["proxy_server_namespace"] = server.Namespace
	metadata["proxy_server_name"] = server.Name
	metadata["proxy_tool_id"] = proxyTool.Id
	metadata["downstream_tool_name"] = proxyTool.DownstreamToolName
	metadata["exposed_tool_name"] = proxyTool.ExposedToolName
	metadata["transport"] = server.Transport
	return metadata
}
