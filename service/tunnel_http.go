package service

import (
	"context"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"path"
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
	BridgeCapabilityHTTPTunnel  = "http_tunnel"
	BridgeToolHTTPTunnelRequest = "http_tunnel.request"

	DefaultTunnelHTTPMaxRequestBytes  = 8 * 1024 * 1024
	DefaultTunnelHTTPMaxResponseBytes = 2 * 1024 * 1024
	tunnelHTTPStreamRequestChunkBytes = 64 * 1024
)

const (
	TunnelWebSocketFrameText   = "text"
	TunnelWebSocketFrameBinary = "binary"
	TunnelWebSocketFrameClose  = "close"
	TunnelWebSocketFramePing   = "ping"
	TunnelWebSocketFramePong   = "pong"
)

const (
	tunnelWebSocketClosePolicyViolation = 1008
	tunnelWebSocketCloseMessageTooLarge = 1009

	tunnelHTTPReasonClientWriteFailed = "client_write_failed"
)

var (
	ErrTunnelHTTPAuthRequired       = errors.New("tunnel http auth token is required")
	ErrTunnelHTTPAuthForbidden      = errors.New("tunnel http auth token is invalid")
	ErrTunnelHTTPRouteForbidden     = errors.New("tunnel http route is forbidden")
	ErrTunnelHTTPRouteConfigInvalid = errors.New("tunnel http route config is invalid")
	ErrTunnelHTTPRequestTooLarge    = errors.New("tunnel http request body is too large")
	ErrTunnelHTTPResponseTooLarge   = errors.New("tunnel http response body is too large")
)

type TunnelHTTPForwardRequest struct {
	Context       context.Context
	ConnectionKey string
	Slug          string
	Method        string
	Host          string
	ProxyPath     string
	RawQuery      string
	Headers       http.Header
	Body          []byte
	BodyReader    io.Reader
	ContentLength int64
	RequestId     string
	ClientIP      string
}

type TunnelHTTPForwardResponse struct {
	StatusCode      int
	Headers         http.Header
	Body            []byte
	DurationMS      int
	RequestId       string
	BridgeSessionId string
	TargetClient    string
	TargetURL       string
}

type TunnelHTTPStreamEvent struct {
	StatusCode      int
	Headers         http.Header
	Body            []byte
	RequestId       string
	BridgeSessionId string
	TargetClient    string
	Flush           bool
}

type TunnelHTTPStreamWriter func(TunnelHTTPStreamEvent) error

type TunnelHTTPWebSocketFrame struct {
	FrameType   string
	Data        []byte
	CloseCode   int
	CloseReason string
}

type TunnelHTTPWebSocketPeer interface {
	ReadFrame() (TunnelHTTPWebSocketFrame, error)
	WriteFrame(TunnelHTTPWebSocketFrame) error
	Close() error
}

type tunnelHTTPForwardPrepared struct {
	Request     TunnelHTTPForwardRequest
	App         model.TunnelApp
	Connection  model.TunnelConnection
	TargetURL   string
	RouteConfig tunnelHTTPRouteConfig
	Policy      bridgepolicy.Policy
	Session     bridge.SessionSnapshot
	Arguments   map[string]any
	StartedAt   time.Time
	BytesIn     int64
}

func ForwardTunnelHTTPRequest(req TunnelHTTPForwardRequest) (TunnelHTTPForwardResponse, error) {
	prepared, err := prepareTunnelHTTPForward(req)
	if err != nil {
		return TunnelHTTPForwardResponse{}, err
	}
	audit := createTunnelHTTPBridgeAudit(prepared.Request.RequestId, prepared.Session, prepared.Arguments, prepared.App.UserId, prepared.Connection.AgentTokenId)
	response, err := bridge.DefaultHub.ForwardToolCall(tunnelHTTPContext(prepared.Request.Context), prepared.Session.SessionId, bridge.ToolCallRequest{
		Id:        prepared.Request.RequestId,
		ToolName:  BridgeToolHTTPTunnelRequest,
		Arguments: bridgepolicy.ApplyArgumentLimits(prepared.Policy, BridgeToolHTTPTunnelRequest, prepared.Arguments),
	})
	durationMS := int(time.Since(prepared.StartedAt).Milliseconds())
	if err != nil {
		updateTunnelHTTPBridgeAuditError(audit, err, durationMS)
		_ = createTunnelHTTPAuditLog(prepared.App, prepared.Connection, prepared.Request, prepared.TargetURL, mcpgateway.DecisionDeny, prepared.BytesIn, 0, durationMS, map[string]any{
			"bridge_session_id": prepared.Session.SessionId,
			"target_client":     prepared.Session.ClientId,
			"error":             err.Error(),
		})
		return TunnelHTTPForwardResponse{}, err
	}
	result, err := tunnelHTTPResponseFromBridge(response.Result)
	if err != nil {
		updateTunnelHTTPBridgeAuditError(audit, err, durationMS)
		_ = createTunnelHTTPAuditLog(prepared.App, prepared.Connection, prepared.Request, prepared.TargetURL, mcpgateway.DecisionDeny, prepared.BytesIn, 0, durationMS, map[string]any{
			"bridge_session_id": prepared.Session.SessionId,
			"target_client":     prepared.Session.ClientId,
			"error":             err.Error(),
		})
		return TunnelHTTPForwardResponse{}, err
	}
	maxResponseBytes := tunnelHTTPMaxResponseBytes(prepared.Policy, prepared.RouteConfig)
	if maxResponseBytes > 0 && int64(len(result.Body)) > maxResponseBytes {
		err := fmt.Errorf("%w: %d > %d", ErrTunnelHTTPResponseTooLarge, len(result.Body), maxResponseBytes)
		updateTunnelHTTPBridgeAuditError(audit, err, durationMS)
		_ = createTunnelHTTPAuditLog(prepared.App, prepared.Connection, prepared.Request, prepared.TargetURL, mcpgateway.DecisionDeny, prepared.BytesIn, int64(len(result.Body)), durationMS, map[string]any{
			"bridge_session_id":  prepared.Session.SessionId,
			"target_client":      prepared.Session.ClientId,
			"reason":             "response_too_large",
			"error":              err.Error(),
			"max_response_bytes": maxResponseBytes,
		})
		return TunnelHTTPForwardResponse{}, err
	}
	if err := checkTunnelResponseRateLimit(prepared.App, prepared.Connection, int64(len(result.Body))); err != nil {
		updateTunnelHTTPBridgeAuditError(audit, err, durationMS)
		metadata := tunnelHTTPRateLimitMetadata(err)
		metadata["phase"] = "response"
		metadata["bridge_session_id"] = prepared.Session.SessionId
		metadata["target_client"] = prepared.Session.ClientId
		metadata["status_code"] = result.StatusCode
		_ = createTunnelHTTPAuditLogWithAction(model.TunnelAuditActionRateLimit, prepared.App, prepared.Connection, prepared.Request, prepared.TargetURL, mcpgateway.DecisionDeny, prepared.BytesIn, int64(len(result.Body)), durationMS, metadata)
		return TunnelHTTPForwardResponse{}, err
	}
	result.DurationMS = durationMS
	result.RequestId = prepared.Request.RequestId
	result.BridgeSessionId = response.Session.SessionId
	result.TargetClient = response.Session.ClientId
	result.TargetURL = prepared.TargetURL
	updateTunnelHTTPBridgeAuditSuccess(audit, durationMS, len(result.Body))
	_ = createTunnelHTTPAuditLog(prepared.App, prepared.Connection, prepared.Request, prepared.TargetURL, mcpgateway.DecisionAllow, prepared.BytesIn, int64(len(result.Body)), durationMS, map[string]any{
		"bridge_session_id": result.BridgeSessionId,
		"target_client":     result.TargetClient,
		"status_code":       result.StatusCode,
	})
	return result, nil
}

func ForwardTunnelHTTPStream(req TunnelHTTPForwardRequest, writer TunnelHTTPStreamWriter) (TunnelHTTPForwardResponse, error) {
	if writer == nil {
		return TunnelHTTPForwardResponse{}, errors.New("tunnel http stream writer is required")
	}
	prepared, err := prepareTunnelHTTPForward(req)
	if err != nil {
		return TunnelHTTPForwardResponse{}, err
	}
	arguments := copyTunnelHTTPBridgeArguments(prepared.Arguments)
	arguments["stream_response"] = true
	streamRequestBody := tunnelHTTPShouldStreamRequestBody(prepared.Request)
	if streamRequestBody {
		arguments["stream_request"] = true
		delete(arguments, "body_base64")
	}
	audit := createTunnelHTTPBridgeAudit(prepared.Request.RequestId, prepared.Session, arguments, prepared.App.UserId, prepared.Connection.AgentTokenId)
	stream, err := bridge.DefaultHub.ForwardToolStream(tunnelHTTPContext(prepared.Request.Context), prepared.Session.SessionId, bridge.ToolCallRequest{
		Id:        prepared.Request.RequestId,
		ToolName:  BridgeToolHTTPTunnelRequest,
		Arguments: bridgepolicy.ApplyArgumentLimits(prepared.Policy, BridgeToolHTTPTunnelRequest, arguments),
	})
	if err != nil {
		durationMS := int(time.Since(prepared.StartedAt).Milliseconds())
		updateTunnelHTTPBridgeAuditError(audit, err, durationMS)
		_ = createTunnelHTTPAuditLog(prepared.App, prepared.Connection, prepared.Request, prepared.TargetURL, mcpgateway.DecisionDeny, prepared.BytesIn, 0, durationMS, map[string]any{
			"bridge_session_id": prepared.Session.SessionId,
			"target_client":     prepared.Session.ClientId,
			"error":             err.Error(),
		})
		return TunnelHTTPForwardResponse{}, err
	}
	cancelStreamOnError := func() {
		stream.Cancel()
	}

	ctx := tunnelHTTPContext(prepared.Request.Context)
	var bytesIn int64 = prepared.BytesIn
	currentBytesIn := func() int64 {
		return atomic.LoadInt64(&bytesIn)
	}
	var uploadDone <-chan error
	if streamRequestBody {
		done := make(chan error, 1)
		uploadDone = done
		go func() {
			done <- forwardTunnelHTTPRequestBodyStream(ctx, stream, prepared, &bytesIn)
		}()
	}
	var response TunnelHTTPForwardResponse
	var bytesOut int64
	wroteHeader := false
	maxResponseBytes := tunnelHTTPMaxResponseBytes(prepared.Policy, prepared.RouteConfig)
readChunks:
	for {
		var chunk dto.BridgeToolStreamChunk
		var ok bool
		select {
		case chunk, ok = <-stream.Chunks:
			if !ok {
				break readChunks
			}
		case <-ctx.Done():
			err := ctx.Err()
			stream.Cancel()
			durationMS := int(time.Since(prepared.StartedAt).Milliseconds())
			updateTunnelHTTPBridgeAuditError(audit, err, durationMS)
			_ = createTunnelHTTPAuditLog(prepared.App, prepared.Connection, prepared.Request, prepared.TargetURL, mcpgateway.DecisionDeny, currentBytesIn(), bytesOut, durationMS, map[string]any{
				"bridge_session_id": stream.Session.SessionId,
				"target_client":     stream.Session.ClientId,
				"error":             err.Error(),
			})
			return TunnelHTTPForwardResponse{}, err
		case uploadErr := <-uploadDone:
			uploadDone = nil
			if uploadErr != nil {
				stream.Cancel()
				durationMS := int(time.Since(prepared.StartedAt).Milliseconds())
				updateTunnelHTTPBridgeAuditError(audit, uploadErr, durationMS)
				_ = createTunnelHTTPAuditLog(prepared.App, prepared.Connection, prepared.Request, prepared.TargetURL, mcpgateway.DecisionDeny, currentBytesIn(), bytesOut, durationMS, map[string]any{
					"bridge_session_id": stream.Session.SessionId,
					"target_client":     stream.Session.ClientId,
					"error":             uploadErr.Error(),
					"stream_request":    true,
				})
				return TunnelHTTPForwardResponse{}, uploadErr
			}
		}
		if chunk.ErrorCode != "" || chunk.ErrorMessage != "" {
			err := &bridge.ClientError{Code: chunk.ErrorCode, Message: chunk.ErrorMessage}
			durationMS := int(time.Since(prepared.StartedAt).Milliseconds())
			cancelStreamOnError()
			updateTunnelHTTPBridgeAuditError(audit, err, durationMS)
			_ = createTunnelHTTPAuditLog(prepared.App, prepared.Connection, prepared.Request, prepared.TargetURL, mcpgateway.DecisionDeny, currentBytesIn(), bytesOut, durationMS, map[string]any{
				"bridge_session_id": prepared.Session.SessionId,
				"target_client":     prepared.Session.ClientId,
				"error":             err.Error(),
			})
			return TunnelHTTPForwardResponse{}, err
		}
		if !wroteHeader {
			response.StatusCode = chunk.StatusCode
			if response.StatusCode <= 0 {
				response.StatusCode = http.StatusOK
			}
			response.Headers = tunnelHTTPHeaderFromAny(chunk.Headers)
			if chunk.Truncated {
				response.Headers.Set("X-Data-Proxy-Tunnel-Truncated", "true")
			}
			response.RequestId = prepared.Request.RequestId
			response.BridgeSessionId = stream.Session.SessionId
			response.TargetClient = stream.Session.ClientId
			response.TargetURL = prepared.TargetURL
			if err := writer(TunnelHTTPStreamEvent{
				StatusCode:      response.StatusCode,
				Headers:         response.Headers,
				RequestId:       response.RequestId,
				BridgeSessionId: response.BridgeSessionId,
				TargetClient:    response.TargetClient,
				Flush:           true,
			}); err != nil {
				durationMS := int(time.Since(prepared.StartedAt).Milliseconds())
				cancelStreamOnError()
				updateTunnelHTTPBridgeAuditError(audit, err, durationMS)
				_ = createTunnelHTTPAuditLog(prepared.App, prepared.Connection, prepared.Request, prepared.TargetURL, mcpgateway.DecisionDeny, currentBytesIn(), bytesOut, durationMS, map[string]any{
					"bridge_session_id": stream.Session.SessionId,
					"target_client":     stream.Session.ClientId,
					"reason":            tunnelHTTPReasonClientWriteFailed,
					"error":             err.Error(),
				})
				return TunnelHTTPForwardResponse{}, err
			}
			wroteHeader = true
		}
		if strings.TrimSpace(chunk.BodyBase64) != "" {
			body, err := base64.StdEncoding.DecodeString(strings.TrimSpace(chunk.BodyBase64))
			if err != nil {
				durationMS := int(time.Since(prepared.StartedAt).Milliseconds())
				cancelStreamOnError()
				updateTunnelHTTPBridgeAuditError(audit, err, durationMS)
				_ = createTunnelHTTPAuditLog(prepared.App, prepared.Connection, prepared.Request, prepared.TargetURL, mcpgateway.DecisionDeny, currentBytesIn(), bytesOut, durationMS, map[string]any{
					"bridge_session_id": stream.Session.SessionId,
					"target_client":     stream.Session.ClientId,
					"error":             err.Error(),
				})
				return TunnelHTTPForwardResponse{}, err
			}
			if len(body) > 0 {
				if maxResponseBytes > 0 && bytesOut+int64(len(body)) > maxResponseBytes {
					err := fmt.Errorf("%w: streamed response exceeded %d", ErrTunnelHTTPResponseTooLarge, maxResponseBytes)
					durationMS := int(time.Since(prepared.StartedAt).Milliseconds())
					cancelStreamOnError()
					updateTunnelHTTPBridgeAuditError(audit, err, durationMS)
					_ = createTunnelHTTPAuditLog(prepared.App, prepared.Connection, prepared.Request, prepared.TargetURL, mcpgateway.DecisionDeny, currentBytesIn(), bytesOut, durationMS, map[string]any{
						"bridge_session_id":  stream.Session.SessionId,
						"target_client":      stream.Session.ClientId,
						"reason":             "response_too_large",
						"error":              err.Error(),
						"max_response_bytes": maxResponseBytes,
					})
					return TunnelHTTPForwardResponse{}, err
				}
				if err := checkTunnelResponseRateLimit(prepared.App, prepared.Connection, int64(len(body))); err != nil {
					durationMS := int(time.Since(prepared.StartedAt).Milliseconds())
					cancelStreamOnError()
					updateTunnelHTTPBridgeAuditError(audit, err, durationMS)
					metadata := tunnelHTTPRateLimitMetadata(err)
					metadata["phase"] = "response"
					metadata["bridge_session_id"] = stream.Session.SessionId
					metadata["target_client"] = stream.Session.ClientId
					metadata["status_code"] = response.StatusCode
					_ = createTunnelHTTPAuditLogWithAction(model.TunnelAuditActionRateLimit, prepared.App, prepared.Connection, prepared.Request, prepared.TargetURL, mcpgateway.DecisionDeny, currentBytesIn(), bytesOut, durationMS, metadata)
					return TunnelHTTPForwardResponse{}, err
				}
				bytesOut += int64(len(body))
				if err := writer(TunnelHTTPStreamEvent{Body: body, Flush: true}); err != nil {
					durationMS := int(time.Since(prepared.StartedAt).Milliseconds())
					cancelStreamOnError()
					updateTunnelHTTPBridgeAuditError(audit, err, durationMS)
					_ = createTunnelHTTPAuditLog(prepared.App, prepared.Connection, prepared.Request, prepared.TargetURL, mcpgateway.DecisionDeny, currentBytesIn(), bytesOut, durationMS, map[string]any{
						"bridge_session_id": stream.Session.SessionId,
						"target_client":     stream.Session.ClientId,
						"reason":            tunnelHTTPReasonClientWriteFailed,
						"error":             err.Error(),
					})
					return TunnelHTTPForwardResponse{}, err
				}
			}
		}
		if chunk.Done {
			break readChunks
		}
	}

	bridgeResponse, err := stream.WaitContext(ctx)
	durationMS := int(time.Since(prepared.StartedAt).Milliseconds())
	if err != nil {
		updateTunnelHTTPBridgeAuditError(audit, err, durationMS)
		_ = createTunnelHTTPAuditLog(prepared.App, prepared.Connection, prepared.Request, prepared.TargetURL, mcpgateway.DecisionDeny, currentBytesIn(), bytesOut, durationMS, map[string]any{
			"bridge_session_id": stream.Session.SessionId,
			"target_client":     stream.Session.ClientId,
			"error":             err.Error(),
		})
		return TunnelHTTPForwardResponse{}, err
	}
	if uploadDone != nil {
		select {
		case uploadErr := <-uploadDone:
			uploadDone = nil
			if uploadErr != nil {
				durationMS = int(time.Since(prepared.StartedAt).Milliseconds())
				updateTunnelHTTPBridgeAuditError(audit, uploadErr, durationMS)
				_ = createTunnelHTTPAuditLog(prepared.App, prepared.Connection, prepared.Request, prepared.TargetURL, mcpgateway.DecisionDeny, currentBytesIn(), bytesOut, durationMS, map[string]any{
					"bridge_session_id": stream.Session.SessionId,
					"target_client":     stream.Session.ClientId,
					"error":             uploadErr.Error(),
					"stream_request":    true,
				})
				return TunnelHTTPForwardResponse{}, uploadErr
			}
		case <-ctx.Done():
			err := ctx.Err()
			stream.Cancel()
			durationMS = int(time.Since(prepared.StartedAt).Milliseconds())
			updateTunnelHTTPBridgeAuditError(audit, err, durationMS)
			_ = createTunnelHTTPAuditLog(prepared.App, prepared.Connection, prepared.Request, prepared.TargetURL, mcpgateway.DecisionDeny, currentBytesIn(), bytesOut, durationMS, map[string]any{
				"bridge_session_id": stream.Session.SessionId,
				"target_client":     stream.Session.ClientId,
				"error":             err.Error(),
				"stream_request":    true,
			})
			return TunnelHTTPForwardResponse{}, err
		}
		durationMS = int(time.Since(prepared.StartedAt).Milliseconds())
	}
	if !wroteHeader {
		result, err := tunnelHTTPResponseFromBridge(bridgeResponse.Result)
		if err != nil {
			updateTunnelHTTPBridgeAuditError(audit, err, durationMS)
			_ = createTunnelHTTPAuditLog(prepared.App, prepared.Connection, prepared.Request, prepared.TargetURL, mcpgateway.DecisionDeny, currentBytesIn(), 0, durationMS, map[string]any{
				"bridge_session_id": stream.Session.SessionId,
				"target_client":     stream.Session.ClientId,
				"error":             err.Error(),
			})
			return TunnelHTTPForwardResponse{}, err
		}
		if maxResponseBytes > 0 && int64(len(result.Body)) > maxResponseBytes {
			err := fmt.Errorf("%w: %d > %d", ErrTunnelHTTPResponseTooLarge, len(result.Body), maxResponseBytes)
			updateTunnelHTTPBridgeAuditError(audit, err, durationMS)
			_ = createTunnelHTTPAuditLog(prepared.App, prepared.Connection, prepared.Request, prepared.TargetURL, mcpgateway.DecisionDeny, currentBytesIn(), int64(len(result.Body)), durationMS, map[string]any{
				"bridge_session_id":  stream.Session.SessionId,
				"target_client":      stream.Session.ClientId,
				"reason":             "response_too_large",
				"error":              err.Error(),
				"max_response_bytes": maxResponseBytes,
			})
			return TunnelHTTPForwardResponse{}, err
		}
		if err := checkTunnelResponseRateLimit(prepared.App, prepared.Connection, int64(len(result.Body))); err != nil {
			updateTunnelHTTPBridgeAuditError(audit, err, durationMS)
			metadata := tunnelHTTPRateLimitMetadata(err)
			metadata["phase"] = "response"
			metadata["bridge_session_id"] = stream.Session.SessionId
			metadata["target_client"] = stream.Session.ClientId
			metadata["status_code"] = result.StatusCode
			_ = createTunnelHTTPAuditLogWithAction(model.TunnelAuditActionRateLimit, prepared.App, prepared.Connection, prepared.Request, prepared.TargetURL, mcpgateway.DecisionDeny, currentBytesIn(), int64(len(result.Body)), durationMS, metadata)
			return TunnelHTTPForwardResponse{}, err
		}
		if err := writer(TunnelHTTPStreamEvent{
			StatusCode:      result.StatusCode,
			Headers:         result.Headers,
			Body:            result.Body,
			RequestId:       prepared.Request.RequestId,
			BridgeSessionId: stream.Session.SessionId,
			TargetClient:    stream.Session.ClientId,
			Flush:           true,
		}); err != nil {
			updateTunnelHTTPBridgeAuditError(audit, err, durationMS)
			_ = createTunnelHTTPAuditLog(prepared.App, prepared.Connection, prepared.Request, prepared.TargetURL, mcpgateway.DecisionDeny, currentBytesIn(), int64(len(result.Body)), durationMS, map[string]any{
				"bridge_session_id": stream.Session.SessionId,
				"target_client":     stream.Session.ClientId,
				"reason":            tunnelHTTPReasonClientWriteFailed,
				"error":             err.Error(),
			})
			return TunnelHTTPForwardResponse{}, err
		}
		result.DurationMS = durationMS
		result.RequestId = prepared.Request.RequestId
		result.BridgeSessionId = stream.Session.SessionId
		result.TargetClient = stream.Session.ClientId
		result.TargetURL = prepared.TargetURL
		updateTunnelHTTPBridgeAuditSuccess(audit, durationMS, len(result.Body))
		_ = createTunnelHTTPAuditLog(prepared.App, prepared.Connection, prepared.Request, prepared.TargetURL, mcpgateway.DecisionAllow, currentBytesIn(), int64(len(result.Body)), durationMS, map[string]any{
			"bridge_session_id": result.BridgeSessionId,
			"target_client":     result.TargetClient,
			"status_code":       result.StatusCode,
			"stream_fallback":   true,
			"stream_request":    streamRequestBody,
		})
		return result, nil
	}

	response.DurationMS = durationMS
	updateTunnelHTTPBridgeAuditSuccess(audit, durationMS, int(bytesOut))
	_ = createTunnelHTTPAuditLog(prepared.App, prepared.Connection, prepared.Request, prepared.TargetURL, mcpgateway.DecisionAllow, currentBytesIn(), bytesOut, durationMS, map[string]any{
		"bridge_session_id": response.BridgeSessionId,
		"target_client":     response.TargetClient,
		"status_code":       response.StatusCode,
		"streamed":          true,
		"stream_request":    streamRequestBody,
	})
	return response, nil
}

func forwardTunnelHTTPRequestBodyStream(ctx context.Context, stream bridge.ToolCallStream, prepared tunnelHTTPForwardPrepared, bytesIn *int64) error {
	reader := prepared.Request.BodyReader
	if reader == nil {
		return nil
	}
	maxRequestBytes := tunnelHTTPMaxRequestBytes(prepared.RouteConfig)
	buffer := make([]byte, tunnelHTTPStreamRequestChunkBytes)
	for {
		n, readErr := reader.Read(buffer)
		if n > 0 {
			chunk := buffer[:n]
			totalBytes := atomic.AddInt64(bytesIn, int64(n))
			if maxRequestBytes > 0 && totalBytes > maxRequestBytes {
				err := fmt.Errorf("%w: streamed request exceeded %d", ErrTunnelHTTPRequestTooLarge, maxRequestBytes)
				_ = sendTunnelHTTPRequestBodyError(ctx, stream, "HTTP_TUNNEL_REQUEST_TOO_LARGE", err)
				return err
			}
			if err := checkTunnelRequestBytesRateLimit(prepared.App, prepared.Connection, int64(n)); err != nil {
				_ = sendTunnelHTTPRequestBodyError(ctx, stream, "HTTP_TUNNEL_RATE_LIMITED", err)
				return err
			}
			if err := stream.SendInput(ctx, dto.BridgeToolStreamInput{
				FrameType:  "http_request_body",
				BodyBase64: base64.StdEncoding.EncodeToString(chunk),
				Metadata: map[string]any{
					"bytes": n,
				},
			}); err != nil {
				return err
			}
		}
		if errors.Is(readErr, io.EOF) {
			return stream.SendInput(ctx, dto.BridgeToolStreamInput{
				FrameType: "http_request_body",
				Done:      true,
			})
		}
		if readErr != nil {
			_ = sendTunnelHTTPRequestBodyError(ctx, stream, "HTTP_TUNNEL_REQUEST_READ_FAILED", readErr)
			return readErr
		}
	}
}

func sendTunnelHTTPRequestBodyError(ctx context.Context, stream bridge.ToolCallStream, code string, err error) error {
	message := ""
	if err != nil {
		message = err.Error()
	}
	return stream.SendInput(ctx, dto.BridgeToolStreamInput{
		FrameType:    "http_request_body",
		Done:         true,
		ErrorCode:    code,
		ErrorMessage: message,
	})
}

func ForwardTunnelHTTPWebSocket(req TunnelHTTPForwardRequest, peer TunnelHTTPWebSocketPeer) (TunnelHTTPForwardResponse, error) {
	if peer == nil {
		return TunnelHTTPForwardResponse{}, errors.New("tunnel http websocket peer is required")
	}
	prepared, err := prepareTunnelHTTPForward(req)
	if err != nil {
		_ = peer.Close()
		return TunnelHTTPForwardResponse{}, err
	}
	arguments := copyTunnelHTTPBridgeArguments(prepared.Arguments)
	arguments["stream_response"] = true
	arguments["websocket"] = true
	audit := createTunnelHTTPBridgeAudit(prepared.Request.RequestId, prepared.Session, arguments, prepared.App.UserId, prepared.Connection.AgentTokenId)
	ctx := tunnelHTTPContext(prepared.Request.Context)
	stream, err := bridge.DefaultHub.ForwardToolStream(ctx, prepared.Session.SessionId, bridge.ToolCallRequest{
		Id:        prepared.Request.RequestId,
		ToolName:  BridgeToolHTTPTunnelRequest,
		Arguments: bridgepolicy.ApplyArgumentLimits(prepared.Policy, BridgeToolHTTPTunnelRequest, arguments),
	})
	if err != nil {
		durationMS := int(time.Since(prepared.StartedAt).Milliseconds())
		updateTunnelHTTPBridgeAuditError(audit, err, durationMS)
		_ = createTunnelHTTPAuditLog(prepared.App, prepared.Connection, prepared.Request, prepared.TargetURL, mcpgateway.DecisionDeny, prepared.BytesIn, 0, durationMS, map[string]any{
			"bridge_session_id": prepared.Session.SessionId,
			"target_client":     prepared.Session.ClientId,
			"error":             err.Error(),
			"websocket":         true,
		})
		_ = peer.Close()
		return TunnelHTTPForwardResponse{}, err
	}
	defer peer.Close()

	var bytesIn int64 = prepared.BytesIn
	maxRequestBytes := tunnelHTTPMaxRequestBytes(prepared.RouteConfig)
	maxResponseBytes := tunnelHTTPMaxResponseBytes(prepared.Policy, prepared.RouteConfig)
	clientDone := make(chan error, 1)
	go func() {
		for {
			frame, readErr := peer.ReadFrame()
			if readErr != nil {
				if frame.FrameType == TunnelWebSocketFrameClose {
					_ = stream.SendInput(context.Background(), dto.BridgeToolStreamInput{
						FrameType:   TunnelWebSocketFrameClose,
						Done:        true,
						CloseCode:   frame.CloseCode,
						CloseReason: frame.CloseReason,
					})
				} else {
					_ = stream.SendInput(context.Background(), dto.BridgeToolStreamInput{
						FrameType: TunnelWebSocketFrameClose,
						Done:      true,
					})
				}
				clientDone <- readErr
				return
			}
			if len(frame.Data) > 0 {
				totalBytesIn := atomic.AddInt64(&bytesIn, int64(len(frame.Data)))
				if maxRequestBytes > 0 && totalBytesIn > maxRequestBytes {
					err := fmt.Errorf("%w: websocket request exceeded %d", ErrTunnelHTTPRequestTooLarge, maxRequestBytes)
					_ = stream.SendInput(context.Background(), dto.BridgeToolStreamInput{
						FrameType:    TunnelWebSocketFrameClose,
						Done:         true,
						CloseCode:    tunnelWebSocketCloseMessageTooLarge,
						CloseReason:  "request too large",
						ErrorCode:    "HTTP_TUNNEL_REQUEST_TOO_LARGE",
						ErrorMessage: err.Error(),
					})
					clientDone <- err
					return
				}
				if err := checkTunnelRequestBytesRateLimit(prepared.App, prepared.Connection, int64(len(frame.Data))); err != nil {
					_ = stream.SendInput(context.Background(), dto.BridgeToolStreamInput{
						FrameType:    TunnelWebSocketFrameClose,
						Done:         true,
						CloseCode:    tunnelWebSocketClosePolicyViolation,
						CloseReason:  "rate limited",
						ErrorCode:    "HTTP_TUNNEL_RATE_LIMITED",
						ErrorMessage: err.Error(),
					})
					clientDone <- err
					return
				}
			}
			input := dto.BridgeToolStreamInput{
				FrameType: frame.FrameType,
			}
			if len(frame.Data) > 0 {
				input.BodyBase64 = base64.StdEncoding.EncodeToString(frame.Data)
			}
			if frame.FrameType == TunnelWebSocketFrameClose {
				input.Done = true
				input.CloseCode = frame.CloseCode
				input.CloseReason = frame.CloseReason
			}
			if err := stream.SendInput(ctx, input); err != nil {
				clientDone <- err
				return
			}
			if frame.FrameType == TunnelWebSocketFrameClose {
				clientDone <- nil
				return
			}
		}
	}()

	response := TunnelHTTPForwardResponse{
		StatusCode:      http.StatusSwitchingProtocols,
		Headers:         http.Header{},
		RequestId:       prepared.Request.RequestId,
		BridgeSessionId: stream.Session.SessionId,
		TargetClient:    stream.Session.ClientId,
		TargetURL:       prepared.TargetURL,
	}
	var bytesOut int64
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
				_ = createTunnelHTTPAuditLog(prepared.App, prepared.Connection, prepared.Request, prepared.TargetURL, mcpgateway.DecisionDeny, atomic.LoadInt64(&bytesIn), bytesOut, durationMS, map[string]any{
					"bridge_session_id": stream.Session.SessionId,
					"target_client":     stream.Session.ClientId,
					"error":             err.Error(),
					"websocket":         true,
				})
				return TunnelHTTPForwardResponse{}, err
			}
			if chunk.StatusCode > 0 {
				response.StatusCode = chunk.StatusCode
				response.Headers = tunnelHTTPHeaderFromAny(chunk.Headers)
			}
			if strings.TrimSpace(chunk.BodyBase64) != "" {
				body, err := base64.StdEncoding.DecodeString(strings.TrimSpace(chunk.BodyBase64))
				if err != nil {
					stream.Cancel()
					durationMS := int(time.Since(prepared.StartedAt).Milliseconds())
					updateTunnelHTTPBridgeAuditError(audit, err, durationMS)
					_ = createTunnelHTTPAuditLog(prepared.App, prepared.Connection, prepared.Request, prepared.TargetURL, mcpgateway.DecisionDeny, atomic.LoadInt64(&bytesIn), bytesOut, durationMS, map[string]any{
						"bridge_session_id": stream.Session.SessionId,
						"target_client":     stream.Session.ClientId,
						"error":             err.Error(),
						"websocket":         true,
					})
					return TunnelHTTPForwardResponse{}, err
				}
				if maxResponseBytes > 0 && bytesOut+int64(len(body)) > maxResponseBytes {
					err := fmt.Errorf("%w: websocket response exceeded %d", ErrTunnelHTTPResponseTooLarge, maxResponseBytes)
					stream.Cancel()
					durationMS := int(time.Since(prepared.StartedAt).Milliseconds())
					updateTunnelHTTPBridgeAuditError(audit, err, durationMS)
					_ = peer.WriteFrame(TunnelHTTPWebSocketFrame{
						FrameType:   TunnelWebSocketFrameClose,
						CloseCode:   tunnelWebSocketCloseMessageTooLarge,
						CloseReason: "response too large",
					})
					_ = createTunnelHTTPAuditLog(prepared.App, prepared.Connection, prepared.Request, prepared.TargetURL, mcpgateway.DecisionDeny, atomic.LoadInt64(&bytesIn), bytesOut, durationMS, map[string]any{
						"bridge_session_id":  stream.Session.SessionId,
						"target_client":      stream.Session.ClientId,
						"reason":             "response_too_large",
						"error":              err.Error(),
						"max_response_bytes": maxResponseBytes,
						"websocket":          true,
					})
					return TunnelHTTPForwardResponse{}, err
				}
				if err := checkTunnelResponseRateLimit(prepared.App, prepared.Connection, int64(len(body))); err != nil {
					stream.Cancel()
					durationMS := int(time.Since(prepared.StartedAt).Milliseconds())
					updateTunnelHTTPBridgeAuditError(audit, err, durationMS)
					metadata := tunnelHTTPRateLimitMetadata(err)
					metadata["phase"] = "response"
					metadata["bridge_session_id"] = stream.Session.SessionId
					metadata["target_client"] = stream.Session.ClientId
					metadata["status_code"] = response.StatusCode
					metadata["websocket"] = true
					_ = peer.WriteFrame(TunnelHTTPWebSocketFrame{
						FrameType:   TunnelWebSocketFrameClose,
						CloseCode:   tunnelWebSocketClosePolicyViolation,
						CloseReason: "rate limited",
					})
					_ = createTunnelHTTPAuditLogWithAction(model.TunnelAuditActionRateLimit, prepared.App, prepared.Connection, prepared.Request, prepared.TargetURL, mcpgateway.DecisionDeny, atomic.LoadInt64(&bytesIn), bytesOut, durationMS, metadata)
					return TunnelHTTPForwardResponse{}, err
				}
				bytesOut += int64(len(body))
				if err := peer.WriteFrame(TunnelHTTPWebSocketFrame{
					FrameType: tunnelWebSocketFrameType(chunk.FrameType),
					Data:      body,
				}); err != nil {
					stream.Cancel()
					durationMS := int(time.Since(prepared.StartedAt).Milliseconds())
					updateTunnelHTTPBridgeAuditError(audit, err, durationMS)
					_ = createTunnelHTTPAuditLog(prepared.App, prepared.Connection, prepared.Request, prepared.TargetURL, mcpgateway.DecisionDeny, atomic.LoadInt64(&bytesIn), bytesOut, durationMS, map[string]any{
						"bridge_session_id": stream.Session.SessionId,
						"target_client":     stream.Session.ClientId,
						"reason":            tunnelHTTPReasonClientWriteFailed,
						"error":             err.Error(),
						"websocket":         true,
					})
					return TunnelHTTPForwardResponse{}, err
				}
			} else if strings.TrimSpace(chunk.FrameType) != "" && tunnelWebSocketFrameType(chunk.FrameType) != TunnelWebSocketFrameClose {
				if err := peer.WriteFrame(TunnelHTTPWebSocketFrame{
					FrameType: tunnelWebSocketFrameType(chunk.FrameType),
				}); err != nil {
					stream.Cancel()
					durationMS := int(time.Since(prepared.StartedAt).Milliseconds())
					updateTunnelHTTPBridgeAuditError(audit, err, durationMS)
					_ = createTunnelHTTPAuditLog(prepared.App, prepared.Connection, prepared.Request, prepared.TargetURL, mcpgateway.DecisionDeny, atomic.LoadInt64(&bytesIn), bytesOut, durationMS, map[string]any{
						"bridge_session_id": stream.Session.SessionId,
						"target_client":     stream.Session.ClientId,
						"reason":            tunnelHTTPReasonClientWriteFailed,
						"error":             err.Error(),
						"websocket":         true,
					})
					return TunnelHTTPForwardResponse{}, err
				}
			}
			if tunnelWebSocketFrameType(chunk.FrameType) == TunnelWebSocketFrameClose || chunk.Done {
				if tunnelWebSocketFrameType(chunk.FrameType) == TunnelWebSocketFrameClose {
					_ = peer.WriteFrame(TunnelHTTPWebSocketFrame{
						FrameType:   TunnelWebSocketFrameClose,
						CloseCode:   chunk.CloseCode,
						CloseReason: chunk.CloseReason,
					})
				}
				remoteDone = true
			}
		case err := <-clientDone:
			if err != nil {
				if errors.Is(err, ErrTunnelHTTPRequestTooLarge) {
					stream.Cancel()
					durationMS := int(time.Since(prepared.StartedAt).Milliseconds())
					updateTunnelHTTPBridgeAuditError(audit, err, durationMS)
					_ = peer.WriteFrame(TunnelHTTPWebSocketFrame{
						FrameType:   TunnelWebSocketFrameClose,
						CloseCode:   tunnelWebSocketCloseMessageTooLarge,
						CloseReason: "request too large",
					})
					_ = createTunnelHTTPAuditLog(prepared.App, prepared.Connection, prepared.Request, prepared.TargetURL, mcpgateway.DecisionDeny, atomic.LoadInt64(&bytesIn), bytesOut, durationMS, map[string]any{
						"bridge_session_id": stream.Session.SessionId,
						"target_client":     stream.Session.ClientId,
						"reason":            "request_too_large",
						"error":             err.Error(),
						"max_request_bytes": maxRequestBytes,
						"websocket":         true,
					})
					return TunnelHTTPForwardResponse{}, err
				}
				if errors.Is(err, ErrTunnelRateLimited) {
					stream.Cancel()
					durationMS := int(time.Since(prepared.StartedAt).Milliseconds())
					updateTunnelHTTPBridgeAuditError(audit, err, durationMS)
					metadata := tunnelHTTPRateLimitMetadata(err)
					metadata["phase"] = "request"
					metadata["bridge_session_id"] = stream.Session.SessionId
					metadata["target_client"] = stream.Session.ClientId
					metadata["websocket"] = true
					_ = peer.WriteFrame(TunnelHTTPWebSocketFrame{
						FrameType:   TunnelWebSocketFrameClose,
						CloseCode:   tunnelWebSocketClosePolicyViolation,
						CloseReason: "rate limited",
					})
					_ = createTunnelHTTPAuditLogWithAction(model.TunnelAuditActionRateLimit, prepared.App, prepared.Connection, prepared.Request, prepared.TargetURL, mcpgateway.DecisionDeny, atomic.LoadInt64(&bytesIn), bytesOut, durationMS, metadata)
					return TunnelHTTPForwardResponse{}, err
				}
				remoteDone = true
			}
		case <-ctx.Done():
			err := ctx.Err()
			stream.Cancel()
			durationMS := int(time.Since(prepared.StartedAt).Milliseconds())
			updateTunnelHTTPBridgeAuditError(audit, err, durationMS)
			_ = createTunnelHTTPAuditLog(prepared.App, prepared.Connection, prepared.Request, prepared.TargetURL, mcpgateway.DecisionDeny, atomic.LoadInt64(&bytesIn), bytesOut, durationMS, map[string]any{
				"bridge_session_id": stream.Session.SessionId,
				"target_client":     stream.Session.ClientId,
				"error":             err.Error(),
				"websocket":         true,
			})
			return TunnelHTTPForwardResponse{}, err
		}
	}

	bridgeResponse, err := stream.WaitContext(ctx)
	durationMS := int(time.Since(prepared.StartedAt).Milliseconds())
	if err != nil && !errors.Is(err, bridge.ErrRequestNotFound) {
		updateTunnelHTTPBridgeAuditError(audit, err, durationMS)
		_ = createTunnelHTTPAuditLog(prepared.App, prepared.Connection, prepared.Request, prepared.TargetURL, mcpgateway.DecisionDeny, atomic.LoadInt64(&bytesIn), bytesOut, durationMS, map[string]any{
			"bridge_session_id": stream.Session.SessionId,
			"target_client":     stream.Session.ClientId,
			"error":             err.Error(),
			"websocket":         true,
		})
		return TunnelHTTPForwardResponse{}, err
	}
	if bridgeResponse.Result.ResultSize > 0 && bytesOut == 0 {
		bytesOut = int64(bridgeResponse.Result.ResultSize)
	}
	response.DurationMS = durationMS
	updateTunnelHTTPBridgeAuditSuccess(audit, durationMS, int(bytesOut))
	_ = createTunnelHTTPAuditLog(prepared.App, prepared.Connection, prepared.Request, prepared.TargetURL, mcpgateway.DecisionAllow, atomic.LoadInt64(&bytesIn), bytesOut, durationMS, map[string]any{
		"bridge_session_id": response.BridgeSessionId,
		"target_client":     response.TargetClient,
		"status_code":       response.StatusCode,
		"websocket":         true,
	})
	return response, nil
}

func prepareTunnelHTTPForward(req TunnelHTTPForwardRequest) (tunnelHTTPForwardPrepared, error) {
	req.RequestId = tunnelHTTPRequestId(req.RequestId)
	app, connection, err := getAuthorizedTunnelHTTPApp(req.Slug, req.ConnectionKey)
	if err != nil {
		return tunnelHTTPForwardPrepared{}, err
	}
	_ = model.TouchTunnelConnectionUsage(connection.Id, req.RequestId)
	targetURL, err := tunnelHTTPTargetURL(*app, req.ProxyPath, req.RawQuery)
	if err != nil {
		return tunnelHTTPForwardPrepared{}, err
	}
	streamRequestBody := tunnelHTTPShouldStreamRequestBody(req)
	bytesIn := int64(len(req.Body))
	if streamRequestBody {
		bytesIn = 0
	}
	startedAt := time.Now()
	routeConfig, err := tunnelHTTPRouteConfigFromApp(*app)
	if err != nil {
		durationMS := int(time.Since(startedAt).Milliseconds())
		_ = createTunnelHTTPAuditLog(*app, *connection, req, targetURL, mcpgateway.DecisionDeny, bytesIn, 0, durationMS, tunnelHTTPRouteAuditMetadata(routeConfig, "route_config_invalid", err, req))
		return tunnelHTTPForwardPrepared{}, err
	}
	if err := validateTunnelHTTPRouteAccess(req, routeConfig); err != nil {
		durationMS := int(time.Since(startedAt).Milliseconds())
		_ = createTunnelHTTPAuditLog(*app, *connection, req, targetURL, mcpgateway.DecisionDeny, bytesIn, 0, durationMS, tunnelHTTPRouteAuditMetadata(routeConfig, tunnelHTTPRouteErrorReason(err), err, req))
		return tunnelHTTPForwardPrepared{}, err
	}
	maxRequestBytes := tunnelHTTPMaxRequestBytes(routeConfig)
	if streamRequestBody && req.ContentLength > maxRequestBytes {
		durationMS := int(time.Since(startedAt).Milliseconds())
		err := fmt.Errorf("%w: %d > %d", ErrTunnelHTTPRequestTooLarge, req.ContentLength, maxRequestBytes)
		_ = createTunnelHTTPAuditLog(*app, *connection, req, targetURL, mcpgateway.DecisionDeny, req.ContentLength, 0, durationMS, tunnelHTTPRouteAuditMetadata(routeConfig, "request_too_large", err, req))
		return tunnelHTTPForwardPrepared{}, err
	}
	if maxRequestBytes > 0 && bytesIn > maxRequestBytes {
		durationMS := int(time.Since(startedAt).Milliseconds())
		err := fmt.Errorf("%w: %d > %d", ErrTunnelHTTPRequestTooLarge, bytesIn, maxRequestBytes)
		_ = createTunnelHTTPAuditLog(*app, *connection, req, targetURL, mcpgateway.DecisionDeny, bytesIn, 0, durationMS, tunnelHTTPRouteAuditMetadata(routeConfig, "request_too_large", err, req))
		return tunnelHTTPForwardPrepared{}, err
	}
	if err := checkTunnelHTTPBillingBalance(*app, *connection, req, targetURL, bytesIn, startedAt); err != nil {
		return tunnelHTTPForwardPrepared{}, err
	}
	if streamRequestBody {
		if err := checkTunnelRequestCountRateLimit(*app, *connection); err != nil {
			durationMS := int(time.Since(startedAt).Milliseconds())
			metadata := tunnelHTTPRateLimitMetadata(err)
			metadata["phase"] = "request"
			_ = createTunnelHTTPAuditLogWithAction(model.TunnelAuditActionRateLimit, *app, *connection, req, targetURL, mcpgateway.DecisionDeny, bytesIn, 0, durationMS, metadata)
			return tunnelHTTPForwardPrepared{}, err
		}
	} else {
		if err := checkTunnelRequestRateLimit(*app, *connection, bytesIn); err != nil {
			durationMS := int(time.Since(startedAt).Milliseconds())
			metadata := tunnelHTTPRateLimitMetadata(err)
			metadata["phase"] = "request"
			_ = createTunnelHTTPAuditLogWithAction(model.TunnelAuditActionRateLimit, *app, *connection, req, targetURL, mcpgateway.DecisionDeny, bytesIn, 0, durationMS, metadata)
			return tunnelHTTPForwardPrepared{}, err
		}
	}
	policy, err := model.GetBridgeClientPolicyByClientId(app.BridgeClientId)
	if err != nil {
		return tunnelHTTPForwardPrepared{}, err
	}
	if err := bridgepolicy.ValidateTool(policy, BridgeToolHTTPTunnelRequest); err != nil {
		durationMS := int(time.Since(startedAt).Milliseconds())
		_ = createTunnelHTTPAuditLog(*app, *connection, req, targetURL, mcpgateway.DecisionDeny, bytesIn, 0, durationMS, map[string]any{
			"reason": bridgepolicy.ErrorCode(err),
			"error":  err.Error(),
		})
		return tunnelHTTPForwardPrepared{}, err
	}
	if err := bridgepolicy.ValidateHTTPTarget(policy, targetURL); err != nil {
		durationMS := int(time.Since(startedAt).Milliseconds())
		_ = createTunnelHTTPAuditLog(*app, *connection, req, targetURL, mcpgateway.DecisionDeny, bytesIn, 0, durationMS, map[string]any{
			"reason": bridgepolicy.ErrorCode(err),
			"error":  err.Error(),
		})
		return tunnelHTTPForwardPrepared{}, err
	}
	session, ok := bridge.DefaultHub.SelectSession(app.UserId, app.BridgeClientId, BridgeCapabilityHTTPTunnel)
	if !ok {
		durationMS := int(time.Since(startedAt).Milliseconds())
		_ = createTunnelHTTPAuditLog(*app, *connection, req, targetURL, mcpgateway.DecisionDeny, bytesIn, 0, durationMS, map[string]any{
			"reason": "bridge_client_not_online",
		})
		return tunnelHTTPForwardPrepared{}, bridge.ErrClientNotFound
	}
	arguments := tunnelHTTPBridgeArguments(req, *app, targetURL, policy, routeConfig)
	return tunnelHTTPForwardPrepared{
		Request:     req,
		App:         *app,
		Connection:  *connection,
		TargetURL:   targetURL,
		RouteConfig: routeConfig,
		Policy:      policy,
		Session:     session,
		Arguments:   arguments,
		StartedAt:   startedAt,
		BytesIn:     bytesIn,
	}, nil
}

func getAuthorizedTunnelHTTPApp(slug string, connectionKey string) (*model.TunnelApp, *model.TunnelConnection, error) {
	if strings.TrimSpace(connectionKey) == "" {
		return nil, nil, errors.New("tunnel http connection key is required")
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
		return nil, nil, errors.New("tunnel http app not found")
	}
	if app.AppType != model.TunnelAppTypeHTTP {
		return nil, nil, errors.New("tunnel app is not an HTTP tunnel")
	}
	if app.Status != model.TunnelAppStatusApproved {
		return nil, nil, errors.New("tunnel http app is not approved")
	}
	if strings.TrimSpace(app.BridgeClientId) == "" {
		return nil, nil, errors.New("tunnel http app has no bridge client")
	}
	if connection.Status != model.TunnelConnectionStatusActive {
		return nil, nil, errors.New("tunnel http connection is not active")
	}
	if connection.ExpiresAt > 0 && connection.ExpiresAt <= common.GetTimestamp() {
		return nil, nil, errors.New("tunnel http connection is expired")
	}
	return app, connection, nil
}

type tunnelHTTPRouteConfig struct {
	AuthMode         string
	AuthToken        string
	Host             string
	PathPrefix       string
	MaxRequestBytes  int64
	MaxResponseBytes int64
}

func tunnelHTTPRouteConfigFromApp(app model.TunnelApp) (tunnelHTTPRouteConfig, error) {
	return tunnelHTTPRouteConfigFromJSON(app.RouteJson)
}

func tunnelHTTPRouteConfigFromJSON(raw string) (tunnelHTTPRouteConfig, error) {
	config := tunnelHTTPRouteConfig{
		AuthMode:   model.TunnelRouteAuthPrivate,
		PathPrefix: "/",
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return config, nil
	}
	route := map[string]any{}
	if err := json.Unmarshal([]byte(raw), &route); err != nil {
		return config, fmt.Errorf("%w: %v", ErrTunnelHTTPRouteConfigInvalid, err)
	}
	authMode := strings.ToLower(stringFromAny(route["auth_mode"]))
	if authMode == "" {
		authMode = model.TunnelRouteAuthPrivate
	}
	switch authMode {
	case model.TunnelRouteAuthPrivate, model.TunnelRouteAuthPublic:
		config.AuthMode = authMode
	case model.TunnelRouteAuthToken:
		config.AuthMode = authMode
		config.AuthToken = stringFromAny(route["auth_token"])
		if config.AuthToken == "" {
			config.AuthToken = stringFromAny(route["token"])
		}
		if config.AuthToken == "" {
			config.AuthToken = stringFromAny(route["bearer_token"])
		}
		if config.AuthToken == "" {
			return config, fmt.Errorf("%w: token auth mode requires auth_token", ErrTunnelHTTPRouteConfigInvalid)
		}
	default:
		return config, fmt.Errorf("%w: unsupported auth_mode %q", ErrTunnelHTTPRouteConfigInvalid, authMode)
	}
	config.Host = tunnelHTTPNormalizeHost(stringFromAny(route["host"]))
	config.PathPrefix = tunnelHTTPNormalizePathPrefix(stringFromAny(route["path_prefix"]))
	config.MaxRequestBytes = int64FromAny(route["max_request_bytes"])
	config.MaxResponseBytes = int64FromAny(route["max_response_bytes"])
	if config.MaxRequestBytes < 0 {
		return config, fmt.Errorf("%w: max_request_bytes cannot be negative", ErrTunnelHTTPRouteConfigInvalid)
	}
	if config.MaxResponseBytes < 0 {
		return config, fmt.Errorf("%w: max_response_bytes cannot be negative", ErrTunnelHTTPRouteConfigInvalid)
	}
	return config, nil
}

func validateTunnelHTTPRouteConfigJSON(appType string, raw string) error {
	if appType != model.TunnelAppTypeHTTP {
		return nil
	}
	_, err := tunnelHTTPRouteConfigFromJSON(raw)
	return err
}

func validateTunnelHTTPRouteAccess(req TunnelHTTPForwardRequest, route tunnelHTTPRouteConfig) error {
	if route.Host != "" && !tunnelHTTPHostMatches(tunnelHTTPRequestHost(req), route.Host) {
		return fmt.Errorf("%w: host mismatch", ErrTunnelHTTPRouteForbidden)
	}
	if !tunnelHTTPPathMatches(req.ProxyPath, route.PathPrefix) {
		return fmt.Errorf("%w: path prefix mismatch", ErrTunnelHTTPRouteForbidden)
	}
	if route.AuthMode == model.TunnelRouteAuthToken {
		token := tunnelHTTPRequestAuthToken(req.Headers)
		if token == "" {
			return ErrTunnelHTTPAuthRequired
		}
		if subtle.ConstantTimeCompare([]byte(token), []byte(route.AuthToken)) != 1 {
			return ErrTunnelHTTPAuthForbidden
		}
	}
	return nil
}

func tunnelHTTPRouteAuditMetadata(route tunnelHTTPRouteConfig, reason string, err error, req TunnelHTTPForwardRequest) map[string]any {
	metadata := map[string]any{
		"reason":            reason,
		"auth_mode":         route.AuthMode,
		"route_host":        route.Host,
		"route_path_prefix": route.PathPrefix,
		"request_host":      tunnelHTTPRequestHost(req),
	}
	if route.MaxRequestBytes > 0 {
		metadata["max_request_bytes"] = route.MaxRequestBytes
	}
	if route.MaxResponseBytes > 0 {
		metadata["max_response_bytes"] = route.MaxResponseBytes
	}
	if err != nil {
		metadata["error"] = err.Error()
	}
	return metadata
}

func tunnelHTTPRouteErrorReason(err error) string {
	switch {
	case errors.Is(err, ErrTunnelHTTPAuthRequired):
		return "auth_token_required"
	case errors.Is(err, ErrTunnelHTTPAuthForbidden):
		return "auth_token_invalid"
	case errors.Is(err, ErrTunnelHTTPRouteConfigInvalid):
		return "route_config_invalid"
	case errors.Is(err, ErrTunnelHTTPRouteForbidden):
		return "route_forbidden"
	default:
		return "route_denied"
	}
}

func tunnelHTTPRequestAuthToken(headers http.Header) string {
	if headers == nil {
		return ""
	}
	for _, value := range headers.Values("Authorization") {
		value = strings.TrimSpace(value)
		if strings.EqualFold(value, "bearer") {
			continue
		}
		if strings.HasPrefix(strings.ToLower(value), "bearer ") {
			return strings.TrimSpace(value[len("bearer "):])
		}
	}
	return strings.TrimSpace(headers.Get("X-Tunnel-Token"))
}

func tunnelHTTPRequestHost(req TunnelHTTPForwardRequest) string {
	if req.Host != "" {
		return tunnelHTTPNormalizeHost(req.Host)
	}
	return tunnelHTTPNormalizeHost(req.Headers.Get("Host"))
}

func tunnelHTTPHostMatches(requestHost string, routeHost string) bool {
	requestHost = tunnelHTTPNormalizeHost(requestHost)
	routeHost = tunnelHTTPNormalizeHost(routeHost)
	if routeHost == "" || requestHost == routeHost {
		return true
	}
	if strings.Contains(routeHost, ":") {
		return false
	}
	return tunnelHTTPHostWithoutPort(requestHost) == routeHost
}

func tunnelHTTPNormalizeHost(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return ""
	}
	if strings.Contains(value, "://") {
		parsed, err := url.Parse(value)
		if err == nil && parsed.Host != "" {
			value = parsed.Host
		}
	}
	value = strings.TrimSuffix(value, ".")
	return value
}

func tunnelHTTPHostWithoutPort(value string) string {
	value = tunnelHTTPNormalizeHost(value)
	if value == "" {
		return ""
	}
	host, _, err := net.SplitHostPort(value)
	if err == nil {
		return strings.Trim(strings.TrimSpace(strings.ToLower(host)), "[]")
	}
	if strings.Count(value, ":") == 1 {
		host, _, ok := strings.Cut(value, ":")
		if ok {
			return strings.TrimSpace(host)
		}
	}
	return strings.Trim(value, "[]")
}

func tunnelHTTPNormalizePathPrefix(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || value == "/" {
		return "/"
	}
	if !strings.HasPrefix(value, "/") {
		value = "/" + value
	}
	return path.Clean(value)
}

func tunnelHTTPPathMatches(proxyPath string, pathPrefix string) bool {
	pathPrefix = tunnelHTTPNormalizePathPrefix(pathPrefix)
	if pathPrefix == "/" {
		return true
	}
	proxyPath = tunnelHTTPNormalizePathPrefix(proxyPath)
	return proxyPath == pathPrefix || strings.HasPrefix(proxyPath, strings.TrimRight(pathPrefix, "/")+"/")
}

func tunnelHTTPTargetURL(app model.TunnelApp, proxyPath string, rawQuery string) (string, error) {
	base := strings.TrimSpace(app.TargetPath)
	if strings.HasPrefix(base, "http://") || strings.HasPrefix(base, "https://") {
		parsed, err := url.Parse(base)
		if err != nil {
			return "", err
		}
		joinTunnelHTTPURLPath(parsed, proxyPath)
		if rawQuery != "" {
			parsed.RawQuery = rawQuery
		}
		return parsed.String(), nil
	}
	host := strings.TrimSpace(app.TargetHost)
	if host == "" {
		host = "127.0.0.1"
	}
	if app.TargetPort <= 0 || app.TargetPort > 65535 {
		return "", errors.New("invalid tunnel http target port")
	}
	if base == "" {
		base = "/"
	}
	if !strings.HasPrefix(base, "/") {
		base = "/" + base
	}
	parsed := &url.URL{
		Scheme: "http",
		Host:   fmt.Sprintf("%s:%d", host, app.TargetPort),
		Path:   base,
	}
	joinTunnelHTTPURLPath(parsed, proxyPath)
	if rawQuery != "" {
		parsed.RawQuery = rawQuery
	}
	return parsed.String(), nil
}

func joinTunnelHTTPURLPath(parsed *url.URL, proxyPath string) {
	if parsed == nil {
		return
	}
	basePath := parsed.EscapedPath()
	if basePath == "" {
		basePath = "/"
	}
	proxyPath = strings.TrimSpace(proxyPath)
	if proxyPath == "" || proxyPath == "/" {
		parsed.Path = path.Clean("/" + strings.TrimPrefix(basePath, "/"))
		if strings.HasSuffix(basePath, "/") && !strings.HasSuffix(parsed.Path, "/") {
			parsed.Path += "/"
		}
		return
	}
	joined := path.Join("/", strings.TrimPrefix(basePath, "/"), strings.TrimPrefix(proxyPath, "/"))
	if strings.HasSuffix(proxyPath, "/") && !strings.HasSuffix(joined, "/") {
		joined += "/"
	}
	parsed.Path = joined
}

func tunnelHTTPBridgeArguments(req TunnelHTTPForwardRequest, app model.TunnelApp, targetURL string, policy bridgepolicy.Policy, route tunnelHTTPRouteConfig) map[string]any {
	headers := tunnelHTTPHeaderMap(req.Headers, true)
	maxResponseBytes := tunnelHTTPMaxResponseBytes(policy, route)
	return map[string]any{
		"target":             targetURL,
		"method":             strings.ToUpper(strings.TrimSpace(req.Method)),
		"headers":            headers,
		"body_base64":        base64.StdEncoding.EncodeToString(req.Body),
		"max_response_bytes": maxResponseBytes,
		"client_ip":          req.ClientIP,
		"server": map[string]any{
			"id":        app.Id,
			"name":      app.Name,
			"namespace": app.PublicSlug,
		},
	}
}

func copyTunnelHTTPBridgeArguments(args map[string]any) map[string]any {
	next := make(map[string]any, len(args)+1)
	for key, value := range args {
		next[key] = value
	}
	return next
}

func tunnelHTTPMaxResponseBytes(policy bridgepolicy.Policy, route tunnelHTTPRouteConfig) int64 {
	maxResponseBytes := int64(policy.MaxResultBytes)
	if maxResponseBytes <= 0 {
		maxResponseBytes = DefaultTunnelHTTPMaxResponseBytes
	}
	if route.MaxResponseBytes > 0 && route.MaxResponseBytes < maxResponseBytes {
		maxResponseBytes = route.MaxResponseBytes
	}
	return maxResponseBytes
}

func tunnelHTTPMaxRequestBytes(route tunnelHTTPRouteConfig) int64 {
	if route.MaxRequestBytes > 0 {
		return route.MaxRequestBytes
	}
	return DefaultTunnelHTTPMaxRequestBytes
}

func tunnelHTTPShouldStreamRequestBody(req TunnelHTTPForwardRequest) bool {
	if req.BodyReader == nil || len(req.Body) > 0 || !tunnelHTTPMethodMayHaveBody(req.Method) {
		return false
	}
	return req.ContentLength != 0
}

func tunnelHTTPMethodMayHaveBody(method string) bool {
	switch strings.ToUpper(strings.TrimSpace(method)) {
	case http.MethodGet, http.MethodHead:
		return false
	default:
		return true
	}
}

func tunnelHTTPResponseFromBridge(result dto.BridgeToolCallResult) (TunnelHTTPForwardResponse, error) {
	payload := mapFromBridgeMetadata(result.Metadata, "http_response")
	if len(payload) == 0 {
		payload = mapFromBridgeMetadata(result.Metadata, "result")
	}
	if len(payload) == 0 {
		for _, block := range result.Content {
			if strings.TrimSpace(block.Text) == "" {
				continue
			}
			if err := common.Unmarshal(common.StringToByteSlice(block.Text), &payload); err == nil && len(payload) > 0 {
				break
			}
		}
	}
	if len(payload) == 0 {
		return TunnelHTTPForwardResponse{}, errors.New("http tunnel bridge response is empty")
	}
	statusCode := intFromAny(payload["status_code"])
	if statusCode <= 0 {
		statusCode = intFromAny(payload["status"])
	}
	if statusCode <= 0 {
		statusCode = http.StatusOK
	}
	body, err := base64.StdEncoding.DecodeString(strings.TrimSpace(stringFromAny(payload["body_base64"])))
	if err != nil {
		return TunnelHTTPForwardResponse{}, err
	}
	return TunnelHTTPForwardResponse{
		StatusCode: statusCode,
		Headers:    tunnelHTTPHeaderFromAny(payload["headers"]),
		Body:       body,
	}, nil
}

func createTunnelHTTPAuditLog(app model.TunnelApp, connection model.TunnelConnection, req TunnelHTTPForwardRequest, targetURL string, decision string, bytesIn int64, bytesOut int64, durationMS int, metadata map[string]any) error {
	return createTunnelHTTPAuditLogWithAction(model.TunnelAuditActionProxyRequest, app, connection, req, targetURL, decision, bytesIn, bytesOut, durationMS, metadata)
}

func createTunnelHTTPAuditLogWithAction(action string, app model.TunnelApp, connection model.TunnelConnection, req TunnelHTTPForwardRequest, targetURL string, decision string, bytesIn int64, bytesOut int64, durationMS int, metadata map[string]any) error {
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadata["connection_id"] = connection.Id
	metadata["connection_key_prefix"] = connection.KeyPrefix
	metadata["target_url"] = targetURL
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
		Method:              strings.ToUpper(strings.TrimSpace(req.Method)),
		Path:                limitTunnelString(req.ProxyPath, 512),
		BytesIn:             bytesIn,
		BytesOut:            bytesOut,
		DurationMS:          durationMS,
		SessionId:           stringFromAny(metadata["bridge_session_id"]),
		MetadataJson:        tunnelAuditMetadataJSON(metadata),
	}
	if err := model.CreateTunnelAuditLog(log); err != nil {
		return err
	}
	return recordTunnelAuditBillingEvent(model.BillingEventSourceTunnelHTTP, app, connection, *log)
}

func checkTunnelHTTPBillingBalance(app model.TunnelApp, connection model.TunnelConnection, req TunnelHTTPForwardRequest, targetURL string, bytesIn int64, startedAt time.Time) error {
	estimatedMinQuota := tunnelBillingPreflightQuota(app, model.TunnelAuditActionProxyRequest, bytesIn)
	policy, quota, err := checkTunnelBillingBalance(app, estimatedMinQuota)
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrTunnelBillingInsufficient) {
		durationMS := int(time.Since(startedAt).Milliseconds())
		metadata := tunnelBillingDenyMetadata(policy, quota, err)
		_ = createTunnelHTTPAuditLogWithAction(model.TunnelAuditActionBillingDeny, app, connection, req, targetURL, mcpgateway.DecisionDeny, bytesIn, 0, durationMS, metadata)
	}
	return err
}

func tunnelHTTPRateLimitMetadata(err error) map[string]any {
	var limitErr TunnelRateLimitError
	if errors.As(err, &limitErr) {
		metadata := limitErr.Metadata()
		metadata["error"] = err.Error()
		return metadata
	}
	return map[string]any{
		"reason": "rate_limited",
		"error":  err.Error(),
	}
}

func createTunnelHTTPBridgeAudit(requestId string, session bridge.SessionSnapshot, args map[string]any, userId int, tokenId int) *model.BridgeAuditLog {
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
		ToolName:    BridgeToolHTTPTunnelRequest,
		RequestBody: body,
		Status:      model.BridgeAuditStatusPending,
	}
	if err := model.CreateBridgeAuditLog(audit); err != nil {
		return nil
	}
	return audit
}

func updateTunnelHTTPBridgeAuditSuccess(audit *model.BridgeAuditLog, durationMS int, resultSize int) {
	if audit == nil {
		return
	}
	_ = model.UpdateBridgeAuditLogStatus(audit.Id, model.BridgeAuditStatusSuccess, map[string]any{
		"duration_ms": durationMS,
		"result_size": resultSize,
	})
}

func updateTunnelHTTPBridgeAuditError(audit *model.BridgeAuditLog, err error, durationMS int) {
	if audit == nil {
		return
	}
	code := bridgepolicy.ErrorCode(err)
	var bridgeErr *bridge.ClientError
	if errors.As(err, &bridgeErr) && strings.TrimSpace(bridgeErr.Code) != "" {
		code = strings.TrimSpace(bridgeErr.Code)
	}
	if code == "" {
		code = "HTTP_TUNNEL_FAILED"
	}
	_ = model.UpdateBridgeAuditLogStatus(audit.Id, model.BridgeAuditStatusError, map[string]any{
		"error_code":    code,
		"error_message": limitTunnelString(err.Error(), 512),
		"duration_ms":   durationMS,
	})
}

func tunnelHTTPHeaderMap(headers http.Header, request bool) map[string][]string {
	result := map[string][]string{}
	for key, values := range headers {
		name := http.CanonicalHeaderKey(strings.TrimSpace(key))
		if name == "" || tunnelHTTPDropHeader(name, request) {
			continue
		}
		for _, value := range values {
			result[name] = append(result[name], value)
		}
	}
	return result
}

func tunnelHTTPHeaderFromAny(value any) http.Header {
	result := http.Header{}
	object := mapFromAny(value)
	for key, raw := range object {
		name := http.CanonicalHeaderKey(strings.TrimSpace(key))
		if name == "" || tunnelHTTPDropHeader(name, false) {
			continue
		}
		for _, item := range sliceStringFromAny(raw) {
			result.Add(name, item)
		}
	}
	return result
}

func tunnelHTTPDropHeader(name string, request bool) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "connection", "proxy-connection", "keep-alive", "proxy-authenticate", "proxy-authorization",
		"te", "trailer", "transfer-encoding", "upgrade", "content-length":
		return true
	case "host":
		return request
	case "content-encoding":
		return !request
	default:
		return false
	}
}

func tunnelHTTPRequestId(requestId string) string {
	requestId = strings.TrimSpace(requestId)
	if requestId != "" {
		return requestId
	}
	return "tunnel-http-" + common.GetRandomString(24)
}

func tunnelWebSocketFrameType(frameType string) string {
	switch strings.ToLower(strings.TrimSpace(frameType)) {
	case TunnelWebSocketFrameText:
		return TunnelWebSocketFrameText
	case TunnelWebSocketFramePing:
		return TunnelWebSocketFramePing
	case TunnelWebSocketFramePong:
		return TunnelWebSocketFramePong
	case TunnelWebSocketFrameClose:
		return TunnelWebSocketFrameClose
	case TunnelWebSocketFrameBinary:
		return TunnelWebSocketFrameBinary
	default:
		return TunnelWebSocketFrameBinary
	}
}

func tunnelHTTPContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func mapFromBridgeMetadata(metadata map[string]any, key string) map[string]any {
	if metadata == nil {
		return nil
	}
	return mapFromAny(metadata[key])
}

func mapFromAny(value any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	if object, ok := value.(map[string]any); ok {
		return object
	}
	body, err := common.Marshal(value)
	if err != nil {
		return map[string]any{}
	}
	var object map[string]any
	if err := common.Unmarshal(body, &object); err != nil {
		return map[string]any{}
	}
	return object
}

func sliceStringFromAny(value any) []string {
	switch typed := value.(type) {
	case []string:
		return typed
	case []any:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			if text := stringFromAny(item); text != "" {
				result = append(result, text)
			}
		}
		return result
	case string:
		return []string{typed}
	default:
		text := stringFromAny(value)
		if text == "" {
			return nil
		}
		return []string{text}
	}
}

func stringFromAny(value any) string {
	if value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case fmt.Stringer:
		return strings.TrimSpace(typed.String())
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", value))
	}
}

func intFromAny(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case json.Number:
		parsed, _ := typed.Int64()
		return int(parsed)
	default:
		return 0
	}
}

func int64FromAny(value any) int64 {
	switch typed := value.(type) {
	case int:
		return int64(typed)
	case int64:
		return typed
	case float64:
		return int64(typed)
	case json.Number:
		parsed, _ := typed.Int64()
		return parsed
	default:
		return 0
	}
}
