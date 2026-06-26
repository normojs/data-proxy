package service

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/bridge"
	"github.com/QuantumNous/new-api/pkg/bridgepolicy"
	"github.com/QuantumNous/new-api/pkg/mcpgateway"
)

const (
	BridgeCapabilityTCPTunnel  = "tcp_tunnel"
	BridgeToolTCPTunnelConnect = "tcp_tunnel.connect"

	tunnelTCPFrameData = "tcp_data"
)

type TunnelTCPForwardRequest struct {
	Context       context.Context
	ConnectionKey string
	Slug          string
	RequestId     string
	ClientIP      string
}

type TunnelTCPForwardResponse struct {
	RequestId       string
	BridgeSessionId string
	TargetClient    string
	Target          string
	DurationMS      int
	BytesIn         int64
	BytesOut        int64
}

type tunnelTCPForwardPrepared struct {
	Request     TunnelTCPForwardRequest
	App         model.TunnelApp
	Connection  model.TunnelConnection
	Target      string
	RouteConfig tunnelHTTPRouteConfig
	Policy      bridgepolicy.Policy
	Session     bridge.SessionSnapshot
	Arguments   map[string]any
	StartedAt   time.Time
}

func ForwardTunnelTCPWebSocket(req TunnelTCPForwardRequest, peer TunnelHTTPWebSocketPeer) (TunnelTCPForwardResponse, error) {
	if peer == nil {
		return TunnelTCPForwardResponse{}, errors.New("tunnel tcp websocket peer is required")
	}
	prepared, err := prepareTunnelTCPForward(req)
	if err != nil {
		_ = peer.Close()
		return TunnelTCPForwardResponse{}, err
	}
	audit := createTunnelTCPBridgeAudit(prepared.Request.RequestId, prepared.Session, prepared.Arguments, prepared.App.UserId, prepared.Connection.AgentTokenId)
	ctx := tunnelHTTPContext(prepared.Request.Context)
	stream, err := bridge.DefaultHub.ForwardToolStream(ctx, prepared.Session.SessionId, bridge.ToolCallRequest{
		Id:        prepared.Request.RequestId,
		ToolName:  BridgeToolTCPTunnelConnect,
		Arguments: bridgepolicy.ApplyArgumentLimits(prepared.Policy, BridgeToolTCPTunnelConnect, prepared.Arguments),
	})
	if err != nil {
		durationMS := int(time.Since(prepared.StartedAt).Milliseconds())
		updateTunnelHTTPBridgeAuditError(audit, err, durationMS)
		_ = createTunnelTCPAuditLog(prepared.App, prepared.Connection, prepared.Request, prepared.Target, mcpgateway.DecisionDeny, 0, 0, durationMS, map[string]any{
			"bridge_session_id": prepared.Session.SessionId,
			"target_client":     prepared.Session.ClientId,
			"error":             err.Error(),
		})
		_ = peer.Close()
		return TunnelTCPForwardResponse{}, err
	}
	defer peer.Close()

	var bytesIn int64
	var bytesOut int64
	maxRequestBytes := tunnelHTTPMaxRequestBytes(prepared.RouteConfig)
	maxResponseBytes := tunnelHTTPMaxResponseBytes(prepared.Policy, prepared.RouteConfig)
	clientDone := make(chan error, 1)
	go forwardTunnelTCPClientFrames(ctx, peer, stream, prepared, maxRequestBytes, &bytesIn, clientDone)

	response := TunnelTCPForwardResponse{
		RequestId:       prepared.Request.RequestId,
		BridgeSessionId: stream.Session.SessionId,
		TargetClient:    stream.Session.ClientId,
		Target:          prepared.Target,
	}
	remoteDone := false
	for !remoteDone {
		select {
		case chunk, ok := <-stream.Chunks:
			if !ok {
				remoteDone = true
				break
			}
			if chunk.ErrorCode != "" || chunk.ErrorMessage != "" {
				err := &bridge.ClientError{Code: chunk.ErrorCode, Message: chunk.ErrorMessage}
				stream.Cancel()
				durationMS := int(time.Since(prepared.StartedAt).Milliseconds())
				updateTunnelHTTPBridgeAuditError(audit, err, durationMS)
				_ = createTunnelTCPAuditLog(prepared.App, prepared.Connection, prepared.Request, prepared.Target, mcpgateway.DecisionDeny, atomic.LoadInt64(&bytesIn), atomic.LoadInt64(&bytesOut), durationMS, map[string]any{
					"bridge_session_id": stream.Session.SessionId,
					"target_client":     stream.Session.ClientId,
					"error":             err.Error(),
				})
				return TunnelTCPForwardResponse{}, err
			}
			if strings.TrimSpace(chunk.BodyBase64) != "" {
				body, err := base64.StdEncoding.DecodeString(strings.TrimSpace(chunk.BodyBase64))
				if err != nil {
					stream.Cancel()
					durationMS := int(time.Since(prepared.StartedAt).Milliseconds())
					updateTunnelHTTPBridgeAuditError(audit, err, durationMS)
					_ = createTunnelTCPAuditLog(prepared.App, prepared.Connection, prepared.Request, prepared.Target, mcpgateway.DecisionDeny, atomic.LoadInt64(&bytesIn), atomic.LoadInt64(&bytesOut), durationMS, map[string]any{
						"bridge_session_id": stream.Session.SessionId,
						"target_client":     stream.Session.ClientId,
						"error":             err.Error(),
					})
					return TunnelTCPForwardResponse{}, err
				}
				if maxResponseBytes > 0 && atomic.LoadInt64(&bytesOut)+int64(len(body)) > maxResponseBytes {
					err := fmt.Errorf("%w: tcp response exceeded %d", ErrTunnelHTTPResponseTooLarge, maxResponseBytes)
					stream.Cancel()
					durationMS := int(time.Since(prepared.StartedAt).Milliseconds())
					updateTunnelHTTPBridgeAuditError(audit, err, durationMS)
					_ = peer.WriteFrame(TunnelHTTPWebSocketFrame{FrameType: TunnelWebSocketFrameClose, CloseCode: tunnelWebSocketCloseMessageTooLarge, CloseReason: "response too large"})
					_ = createTunnelTCPAuditLog(prepared.App, prepared.Connection, prepared.Request, prepared.Target, mcpgateway.DecisionDeny, atomic.LoadInt64(&bytesIn), atomic.LoadInt64(&bytesOut), durationMS, map[string]any{
						"bridge_session_id":  stream.Session.SessionId,
						"target_client":      stream.Session.ClientId,
						"reason":             "response_too_large",
						"error":              err.Error(),
						"max_response_bytes": maxResponseBytes,
					})
					return TunnelTCPForwardResponse{}, err
				}
				if err := checkTunnelResponseRateLimit(prepared.App, prepared.Connection, int64(len(body))); err != nil {
					stream.Cancel()
					durationMS := int(time.Since(prepared.StartedAt).Milliseconds())
					updateTunnelHTTPBridgeAuditError(audit, err, durationMS)
					metadata := tunnelHTTPRateLimitMetadata(err)
					metadata["phase"] = "response"
					metadata["bridge_session_id"] = stream.Session.SessionId
					metadata["target_client"] = stream.Session.ClientId
					metadata["tcp"] = true
					_ = peer.WriteFrame(TunnelHTTPWebSocketFrame{FrameType: TunnelWebSocketFrameClose, CloseCode: tunnelWebSocketClosePolicyViolation, CloseReason: "rate limited"})
					_ = createTunnelTCPAuditLogWithAction(model.TunnelAuditActionRateLimit, prepared.App, prepared.Connection, prepared.Request, prepared.Target, mcpgateway.DecisionDeny, atomic.LoadInt64(&bytesIn), atomic.LoadInt64(&bytesOut), durationMS, metadata)
					return TunnelTCPForwardResponse{}, err
				}
				atomic.AddInt64(&bytesOut, int64(len(body)))
				if err := peer.WriteFrame(TunnelHTTPWebSocketFrame{FrameType: TunnelWebSocketFrameBinary, Data: body}); err != nil {
					stream.Cancel()
					durationMS := int(time.Since(prepared.StartedAt).Milliseconds())
					updateTunnelHTTPBridgeAuditError(audit, err, durationMS)
					_ = createTunnelTCPAuditLog(prepared.App, prepared.Connection, prepared.Request, prepared.Target, mcpgateway.DecisionDeny, atomic.LoadInt64(&bytesIn), atomic.LoadInt64(&bytesOut), durationMS, map[string]any{
						"bridge_session_id": stream.Session.SessionId,
						"target_client":     stream.Session.ClientId,
						"reason":            tunnelHTTPReasonClientWriteFailed,
						"error":             err.Error(),
					})
					return TunnelTCPForwardResponse{}, err
				}
			}
			if chunk.Done || tunnelWebSocketFrameType(chunk.FrameType) == TunnelWebSocketFrameClose {
				_ = peer.WriteFrame(TunnelHTTPWebSocketFrame{
					FrameType:   TunnelWebSocketFrameClose,
					CloseCode:   chunk.CloseCode,
					CloseReason: chunk.CloseReason,
				})
				remoteDone = true
			}
		case err := <-clientDone:
			if err != nil {
				if errors.Is(err, ErrTunnelHTTPRequestTooLarge) {
					stream.Cancel()
					durationMS := int(time.Since(prepared.StartedAt).Milliseconds())
					updateTunnelHTTPBridgeAuditError(audit, err, durationMS)
					_ = peer.WriteFrame(TunnelHTTPWebSocketFrame{FrameType: TunnelWebSocketFrameClose, CloseCode: tunnelWebSocketCloseMessageTooLarge, CloseReason: "request too large"})
					_ = createTunnelTCPAuditLog(prepared.App, prepared.Connection, prepared.Request, prepared.Target, mcpgateway.DecisionDeny, atomic.LoadInt64(&bytesIn), atomic.LoadInt64(&bytesOut), durationMS, map[string]any{
						"bridge_session_id": stream.Session.SessionId,
						"target_client":     stream.Session.ClientId,
						"reason":            "request_too_large",
						"error":             err.Error(),
						"max_request_bytes": maxRequestBytes,
					})
					return TunnelTCPForwardResponse{}, err
				}
				if errors.Is(err, ErrTunnelRateLimited) {
					stream.Cancel()
					durationMS := int(time.Since(prepared.StartedAt).Milliseconds())
					updateTunnelHTTPBridgeAuditError(audit, err, durationMS)
					metadata := tunnelHTTPRateLimitMetadata(err)
					metadata["phase"] = "request"
					metadata["bridge_session_id"] = stream.Session.SessionId
					metadata["target_client"] = stream.Session.ClientId
					metadata["tcp"] = true
					_ = peer.WriteFrame(TunnelHTTPWebSocketFrame{FrameType: TunnelWebSocketFrameClose, CloseCode: tunnelWebSocketClosePolicyViolation, CloseReason: "rate limited"})
					_ = createTunnelTCPAuditLogWithAction(model.TunnelAuditActionRateLimit, prepared.App, prepared.Connection, prepared.Request, prepared.Target, mcpgateway.DecisionDeny, atomic.LoadInt64(&bytesIn), atomic.LoadInt64(&bytesOut), durationMS, metadata)
					return TunnelTCPForwardResponse{}, err
				}
			}
			remoteDone = true
		case <-ctx.Done():
			err := ctx.Err()
			stream.Cancel()
			durationMS := int(time.Since(prepared.StartedAt).Milliseconds())
			updateTunnelHTTPBridgeAuditError(audit, err, durationMS)
			_ = createTunnelTCPAuditLog(prepared.App, prepared.Connection, prepared.Request, prepared.Target, mcpgateway.DecisionDeny, atomic.LoadInt64(&bytesIn), atomic.LoadInt64(&bytesOut), durationMS, map[string]any{
				"bridge_session_id": stream.Session.SessionId,
				"target_client":     stream.Session.ClientId,
				"error":             err.Error(),
			})
			return TunnelTCPForwardResponse{}, err
		}
	}

	bridgeResponse, err := stream.WaitContext(ctx)
	durationMS := int(time.Since(prepared.StartedAt).Milliseconds())
	if err != nil && !errors.Is(err, bridge.ErrRequestNotFound) {
		updateTunnelHTTPBridgeAuditError(audit, err, durationMS)
		_ = createTunnelTCPAuditLog(prepared.App, prepared.Connection, prepared.Request, prepared.Target, mcpgateway.DecisionDeny, atomic.LoadInt64(&bytesIn), atomic.LoadInt64(&bytesOut), durationMS, map[string]any{
			"bridge_session_id": stream.Session.SessionId,
			"target_client":     stream.Session.ClientId,
			"error":             err.Error(),
		})
		return TunnelTCPForwardResponse{}, err
	}
	if bridgeResponse.Result.ResultSize > 0 && atomic.LoadInt64(&bytesOut) == 0 {
		atomic.StoreInt64(&bytesOut, int64(bridgeResponse.Result.ResultSize))
	}
	response.DurationMS = durationMS
	response.BytesIn = atomic.LoadInt64(&bytesIn)
	response.BytesOut = atomic.LoadInt64(&bytesOut)
	updateTunnelHTTPBridgeAuditSuccess(audit, durationMS, int(response.BytesOut))
	_ = createTunnelTCPAuditLog(prepared.App, prepared.Connection, prepared.Request, prepared.Target, mcpgateway.DecisionAllow, response.BytesIn, response.BytesOut, durationMS, map[string]any{
		"bridge_session_id": response.BridgeSessionId,
		"target_client":     response.TargetClient,
	})
	return response, nil
}

func forwardTunnelTCPClientFrames(ctx context.Context, peer TunnelHTTPWebSocketPeer, stream bridge.ToolCallStream, prepared tunnelTCPForwardPrepared, maxRequestBytes int64, bytesIn *int64, done chan<- error) {
	for {
		frame, readErr := peer.ReadFrame()
		if readErr != nil {
			_ = stream.SendInput(context.Background(), dto.BridgeToolStreamInput{
				FrameType:   TunnelWebSocketFrameClose,
				Done:        true,
				CloseCode:   frame.CloseCode,
				CloseReason: frame.CloseReason,
			})
			done <- readErr
			return
		}
		switch tunnelWebSocketFrameType(frame.FrameType) {
		case TunnelWebSocketFramePing:
			_ = peer.WriteFrame(TunnelHTTPWebSocketFrame{FrameType: TunnelWebSocketFramePong, Data: frame.Data})
			continue
		case TunnelWebSocketFramePong:
			continue
		}
		if len(frame.Data) > 0 {
			totalBytesIn := atomic.AddInt64(bytesIn, int64(len(frame.Data)))
			if maxRequestBytes > 0 && totalBytesIn > maxRequestBytes {
				err := fmt.Errorf("%w: tcp request exceeded %d", ErrTunnelHTTPRequestTooLarge, maxRequestBytes)
				_ = stream.SendInput(context.Background(), dto.BridgeToolStreamInput{
					FrameType:    TunnelWebSocketFrameClose,
					Done:         true,
					CloseCode:    tunnelWebSocketCloseMessageTooLarge,
					CloseReason:  "request too large",
					ErrorCode:    "TCP_TUNNEL_REQUEST_TOO_LARGE",
					ErrorMessage: err.Error(),
				})
				done <- err
				return
			}
			if err := checkTunnelRequestBytesRateLimit(prepared.App, prepared.Connection, int64(len(frame.Data))); err != nil {
				_ = stream.SendInput(context.Background(), dto.BridgeToolStreamInput{
					FrameType:    TunnelWebSocketFrameClose,
					Done:         true,
					CloseCode:    tunnelWebSocketClosePolicyViolation,
					CloseReason:  "rate limited",
					ErrorCode:    "TCP_TUNNEL_RATE_LIMITED",
					ErrorMessage: err.Error(),
				})
				done <- err
				return
			}
		}
		input := dto.BridgeToolStreamInput{FrameType: tunnelTCPFrameData}
		if len(frame.Data) > 0 {
			input.BodyBase64 = base64.StdEncoding.EncodeToString(frame.Data)
		}
		if frame.FrameType == TunnelWebSocketFrameClose {
			input.FrameType = TunnelWebSocketFrameClose
			input.Done = true
			input.CloseCode = frame.CloseCode
			input.CloseReason = frame.CloseReason
		}
		if err := stream.SendInput(ctx, input); err != nil {
			done <- err
			return
		}
		if frame.FrameType == TunnelWebSocketFrameClose {
			done <- nil
			return
		}
	}
}

func prepareTunnelTCPForward(req TunnelTCPForwardRequest) (tunnelTCPForwardPrepared, error) {
	req.RequestId = tunnelHTTPRequestId(req.RequestId)
	app, connection, err := getAuthorizedTunnelTCPApp(req.Slug, req.ConnectionKey)
	if err != nil {
		return tunnelTCPForwardPrepared{}, err
	}
	_ = model.TouchTunnelConnectionUsage(connection.Id, req.RequestId)
	target, err := tunnelTCPTarget(*app)
	if err != nil {
		return tunnelTCPForwardPrepared{}, err
	}
	startedAt := time.Now()
	routeConfig, err := tunnelHTTPRouteConfigFromApp(*app)
	if err != nil {
		durationMS := int(time.Since(startedAt).Milliseconds())
		_ = createTunnelTCPAuditLog(*app, *connection, req, target, mcpgateway.DecisionDeny, 0, 0, durationMS, tunnelHTTPRouteAuditMetadata(routeConfig, "route_config_invalid", err, TunnelHTTPForwardRequest{RequestId: req.RequestId}))
		return tunnelTCPForwardPrepared{}, err
	}
	if err := checkTunnelTCPBillingBalance(*app, *connection, req, target, startedAt); err != nil {
		return tunnelTCPForwardPrepared{}, err
	}
	if err := checkTunnelRequestCountRateLimit(*app, *connection); err != nil {
		durationMS := int(time.Since(startedAt).Milliseconds())
		metadata := tunnelHTTPRateLimitMetadata(err)
		metadata["phase"] = "request"
		metadata["tcp"] = true
		_ = createTunnelTCPAuditLogWithAction(model.TunnelAuditActionRateLimit, *app, *connection, req, target, mcpgateway.DecisionDeny, 0, 0, durationMS, metadata)
		return tunnelTCPForwardPrepared{}, err
	}
	policy, err := model.GetBridgeClientPolicyByClientId(app.BridgeClientId)
	if err != nil {
		return tunnelTCPForwardPrepared{}, err
	}
	if err := bridgepolicy.ValidateTool(policy, BridgeToolTCPTunnelConnect); err != nil {
		durationMS := int(time.Since(startedAt).Milliseconds())
		_ = createTunnelTCPAuditLog(*app, *connection, req, target, mcpgateway.DecisionDeny, 0, 0, durationMS, map[string]any{
			"reason": bridgepolicy.ErrorCode(err),
			"error":  err.Error(),
		})
		return tunnelTCPForwardPrepared{}, err
	}
	session, ok := bridge.DefaultHub.SelectSession(app.UserId, app.BridgeClientId, BridgeCapabilityTCPTunnel)
	if !ok {
		durationMS := int(time.Since(startedAt).Milliseconds())
		_ = createTunnelTCPAuditLog(*app, *connection, req, target, mcpgateway.DecisionDeny, 0, 0, durationMS, map[string]any{
			"reason": "bridge_client_not_online",
		})
		return tunnelTCPForwardPrepared{}, bridge.ErrClientNotFound
	}
	arguments := map[string]any{
		"target":      target,
		"target_host": app.TargetHost,
		"target_port": app.TargetPort,
	}
	if routeConfig.MaxRequestBytes > 0 {
		arguments["max_request_bytes"] = routeConfig.MaxRequestBytes
	}
	if routeConfig.MaxResponseBytes > 0 {
		arguments["max_response_bytes"] = routeConfig.MaxResponseBytes
	}
	return tunnelTCPForwardPrepared{
		Request:     req,
		App:         *app,
		Connection:  *connection,
		Target:      target,
		RouteConfig: routeConfig,
		Policy:      policy,
		Session:     session,
		Arguments:   arguments,
		StartedAt:   startedAt,
	}, nil
}

func getAuthorizedTunnelTCPApp(slug string, connectionKey string) (*model.TunnelApp, *model.TunnelConnection, error) {
	if strings.TrimSpace(connectionKey) == "" {
		return nil, nil, errors.New("tunnel tcp connection key is required")
	}
	connection, err := model.GetTunnelConnectionByKeyHash(tunnelConnectionKeyHash(connectionKey))
	if err != nil {
		return nil, nil, err
	}
	app, err := model.GetTunnelAppByPublicSlug(slug)
	if err != nil {
		return nil, nil, err
	}
	if connection.AppId != app.Id || connection.UserId != app.UserId {
		return nil, nil, errors.New("tunnel tcp app not found")
	}
	if app.AppType != model.TunnelAppTypeTCP {
		return nil, nil, errors.New("tunnel app is not a TCP tunnel")
	}
	if app.Status != model.TunnelAppStatusApproved {
		return nil, nil, errors.New("tunnel tcp app is not approved")
	}
	if strings.TrimSpace(app.BridgeClientId) == "" {
		return nil, nil, errors.New("tunnel tcp app has no bridge client")
	}
	if connection.Status != model.TunnelConnectionStatusActive {
		return nil, nil, errors.New("tunnel tcp connection is not active")
	}
	if connection.ExpiresAt > 0 && connection.ExpiresAt <= common.GetTimestamp() {
		return nil, nil, errors.New("tunnel tcp connection is expired")
	}
	return app, connection, nil
}

func tunnelTCPTarget(app model.TunnelApp) (string, error) {
	host := strings.TrimSpace(app.TargetHost)
	if host == "" {
		host = "127.0.0.1"
	}
	if app.TargetPort <= 0 || app.TargetPort > 65535 {
		return "", errors.New("invalid tunnel tcp target port")
	}
	return net.JoinHostPort(host, strconv.Itoa(app.TargetPort)), nil
}

func checkTunnelTCPBillingBalance(app model.TunnelApp, connection model.TunnelConnection, req TunnelTCPForwardRequest, target string, startedAt time.Time) error {
	estimatedMinQuota := tunnelBillingPreflightQuota(app, model.TunnelAuditActionProxyRequest, 0)
	policy, quota, err := checkTunnelBillingBalance(app, estimatedMinQuota)
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrTunnelBillingInsufficient) {
		durationMS := int(time.Since(startedAt).Milliseconds())
		metadata := tunnelBillingDenyMetadata(policy, quota, err)
		_ = createTunnelTCPAuditLogWithAction(model.TunnelAuditActionBillingDeny, app, connection, req, target, mcpgateway.DecisionDeny, 0, 0, durationMS, metadata)
	}
	return err
}

func createTunnelTCPAuditLog(app model.TunnelApp, connection model.TunnelConnection, req TunnelTCPForwardRequest, target string, decision string, bytesIn int64, bytesOut int64, durationMS int, metadata map[string]any) error {
	return createTunnelTCPAuditLogWithAction(model.TunnelAuditActionProxyRequest, app, connection, req, target, decision, bytesIn, bytesOut, durationMS, metadata)
}

func createTunnelTCPAuditLogWithAction(action string, app model.TunnelApp, connection model.TunnelConnection, req TunnelTCPForwardRequest, target string, decision string, bytesIn int64, bytesOut int64, durationMS int, metadata map[string]any) error {
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadata["connection_id"] = connection.Id
	metadata["connection_key_prefix"] = connection.KeyPrefix
	metadata["target"] = target
	metadata["tcp"] = true
	log := &model.TunnelAuditLog{
		AppId:               app.Id,
		ConnectionId:        connection.Id,
		ConnectionKeyPrefix: connection.KeyPrefix,
		UserId:              app.UserId,
		ActorUserId:         app.UserId,
		Action:              action,
		Decision:            decision,
		Reason:              stringFromAny(metadata["reason"]),
		RequestId:           req.RequestId,
		Method:              "TCP",
		Path:                limitTunnelString(target, 512),
		BytesIn:             bytesIn,
		BytesOut:            bytesOut,
		DurationMS:          durationMS,
		SessionId:           stringFromAny(metadata["bridge_session_id"]),
		MetadataJson:        tunnelAuditMetadataJSON(metadata),
	}
	if err := model.CreateTunnelAuditLog(log); err != nil {
		return err
	}
	return recordTunnelAuditBillingEvent(model.BillingEventSourceTunnelTCP, app, connection, *log)
}

func createTunnelTCPBridgeAudit(requestId string, session bridge.SessionSnapshot, args map[string]any, userId int, tokenId int) *model.BridgeAuditLog {
	if model.DB == nil {
		return nil
	}
	body := ""
	if raw, err := common.Marshal(args); err == nil {
		body = string(raw)
	}
	audit := &model.BridgeAuditLog{
		RequestId:   tunnelHTTPRequestId(requestId),
		SessionId:   session.SessionId,
		ClientId:    session.ClientId,
		UserId:      userId,
		TokenId:     tokenId,
		ToolName:    BridgeToolTCPTunnelConnect,
		RequestBody: body,
		Status:      model.BridgeAuditStatusPending,
	}
	if err := model.CreateBridgeAuditLog(audit); err != nil {
		return nil
	}
	return audit
}
