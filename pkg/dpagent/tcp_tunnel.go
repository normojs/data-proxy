package dpagent

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/dto"
)

const (
	tcpTunnelChunkBytes = 32 * 1024
	tcpTunnelFrameData  = "tcp_data"
)

type tcpTunnelArgs struct {
	Target           string `json:"target"`
	TargetHost       string `json:"target_host"`
	TargetPort       int    `json:"target_port"`
	MaxRequestBytes  int64  `json:"max_request_bytes"`
	MaxResponseBytes int64  `json:"max_response_bytes"`
}

func (c BridgeClient) handleTCPTunnelStream(ctx context.Context, args map[string]any, emit bridgeStreamChunkEmitter, inputQueue *bridgeStreamInputQueue) (dto.BridgeToolCallResult, error) {
	if emit == nil {
		return dto.BridgeToolCallResult{}, ToolError{Code: "TCP_TUNNEL_STREAM_EMITTER_MISSING", Message: "TCP tunnel stream emitter is required"}
	}
	if inputQueue == nil {
		return dto.BridgeToolCallResult{}, ToolError{Code: "TCP_TUNNEL_STREAM_INPUT_MISSING", Message: "TCP tunnel requires stream input"}
	}
	var input tcpTunnelArgs
	if err := decodeBridgeData(args, &input); err != nil {
		return dto.BridgeToolCallResult{}, ToolError{Code: "TCP_TUNNEL_INVALID_ARGUMENTS", Message: err.Error()}
	}
	streamCtx, cancelStream := context.WithCancel(ctx)
	defer cancelStream()
	defer inputQueue.Close()
	target, err := allowedTCPTarget(c.Config, input)
	if err != nil {
		return dto.BridgeToolCallResult{}, err
	}
	timeout := time.Duration(c.Config.Runtime.HTTPTimeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = DefaultHTTPTimeoutMS * time.Millisecond
	}
	dialer := net.Dialer{Timeout: timeout}
	startedAt := time.Now()
	conn, err := dialer.DialContext(ctx, "tcp", target)
	if err != nil {
		return dto.BridgeToolCallResult{}, ToolError{Code: "TCP_TUNNEL_CONNECT_FAILED", Message: err.Error()}
	}
	defer conn.Close()

	if err := emit(dto.BridgeToolStreamChunk{
		FrameType: tcpTunnelFrameData,
		Metadata: map[string]any{
			"target":    target,
			"connected": true,
		},
	}); err != nil {
		return dto.BridgeToolCallResult{}, err
	}

	var bytesIn int64
	var bytesOut int64
	errCh := make(chan error, 2)
	go c.pipeTCPRemoteToBridge(streamCtx, conn, input.MaxResponseBytes, emit, &bytesOut, errCh)
	go c.pipeTCPBridgeToRemote(streamCtx, conn, inputQueue, input.MaxRequestBytes, &bytesIn, errCh)

	err = <-errCh
	cancelStream()
	inputQueue.Close()
	_ = conn.Close()
	if err != nil {
		return dto.BridgeToolCallResult{}, err
	}
	payload := map[string]any{
		"target":    target,
		"bytes_in":  atomic.LoadInt64(&bytesIn),
		"bytes_out": atomic.LoadInt64(&bytesOut),
	}
	payloadBytes, _ := json.Marshal(payload)
	return dto.BridgeToolCallResult{
		Content:    []dto.MCPContentBlock{{Type: "text", Text: string(payloadBytes)}},
		Summary:    fmt.Sprintf("TCP %s closed (%d in / %d out)", target, atomic.LoadInt64(&bytesIn), atomic.LoadInt64(&bytesOut)),
		DurationMS: int(time.Since(startedAt).Milliseconds()),
		ResultSize: int(atomic.LoadInt64(&bytesOut)),
		Metadata: map[string]any{
			"tcp_response": payload,
			"target":       target,
			"bytes_in":     atomic.LoadInt64(&bytesIn),
			"bytes_out":    atomic.LoadInt64(&bytesOut),
		},
	}, nil
}

func (c BridgeClient) pipeTCPRemoteToBridge(ctx context.Context, conn net.Conn, maxResponseBytes int64, emit bridgeStreamChunkEmitter, bytesOut *int64, errCh chan<- error) {
	buffer := make([]byte, tcpTunnelChunkBytes)
	for {
		n, readErr := conn.Read(buffer)
		if n > 0 {
			chunk := buffer[:n]
			if maxResponseBytes > 0 && atomic.LoadInt64(bytesOut)+int64(len(chunk)) > maxResponseBytes {
				errCh <- ToolError{Code: "TCP_TUNNEL_RESPONSE_TOO_LARGE", Message: fmt.Sprintf("TCP tunnel response exceeded %d bytes", maxResponseBytes)}
				return
			}
			atomic.AddInt64(bytesOut, int64(len(chunk)))
			if err := emit(dto.BridgeToolStreamChunk{
				FrameType:  tcpTunnelFrameData,
				BodyBase64: base64.StdEncoding.EncodeToString(chunk),
				Bytes:      len(chunk),
			}); err != nil {
				errCh <- err
				return
			}
		}
		if readErr == io.EOF {
			_ = emit(dto.BridgeToolStreamChunk{FrameType: tunnelWebSocketFrameClose, Done: true, Bytes: int(atomic.LoadInt64(bytesOut))})
			errCh <- nil
			return
		}
		if readErr != nil {
			select {
			case <-ctx.Done():
				errCh <- nil
			default:
				errCh <- ToolError{Code: "TCP_TUNNEL_READ_FAILED", Message: readErr.Error()}
			}
			return
		}
	}
}

func (c BridgeClient) pipeTCPBridgeToRemote(ctx context.Context, conn net.Conn, inputQueue *bridgeStreamInputQueue, maxRequestBytes int64, bytesIn *int64, errCh chan<- error) {
	for {
		input, ok := inputQueue.Next(ctx)
		if !ok {
			errCh <- nil
			return
		}
		if input.ErrorCode != "" || input.ErrorMessage != "" {
			message := input.ErrorMessage
			if message == "" {
				message = input.ErrorCode
			}
			errCh <- ToolError{Code: normalizeErrorCode(input.ErrorCode, "TCP_TUNNEL_INPUT_FAILED"), Message: message}
			return
		}
		if input.Done {
			errCh <- nil
			return
		}
		frameType := strings.TrimSpace(input.FrameType)
		if frameType != "" && frameType != tcpTunnelFrameData && frameType != tunnelWebSocketFrameBinary && frameType != tunnelWebSocketFrameText {
			continue
		}
		if strings.TrimSpace(input.BodyBase64) == "" {
			continue
		}
		body, err := base64.StdEncoding.DecodeString(strings.TrimSpace(input.BodyBase64))
		if err != nil {
			errCh <- ToolError{Code: "TCP_TUNNEL_INVALID_STREAM_BODY", Message: err.Error()}
			return
		}
		if len(body) == 0 {
			continue
		}
		nextBytesIn := atomic.AddInt64(bytesIn, int64(len(body)))
		if maxRequestBytes > 0 && nextBytesIn > maxRequestBytes {
			errCh <- ToolError{Code: "TCP_TUNNEL_REQUEST_TOO_LARGE", Message: fmt.Sprintf("TCP tunnel request exceeded %d bytes", maxRequestBytes)}
			return
		}
		if _, err := conn.Write(body); err != nil {
			errCh <- ToolError{Code: "TCP_TUNNEL_WRITE_FAILED", Message: err.Error()}
			return
		}
	}
}

func allowedTCPTarget(cfg Config, input tcpTunnelArgs) (string, error) {
	host := strings.TrimSpace(input.TargetHost)
	port := input.TargetPort
	if strings.TrimSpace(input.Target) != "" {
		nextHost, nextPort, err := splitTCPTarget(input.Target)
		if err != nil {
			return "", err
		}
		host = nextHost
		port = nextPort
	}
	if host == "" || port <= 0 || port > 65535 {
		return "", ToolError{Code: "TCP_TUNNEL_INVALID_TARGET", Message: "TCP tunnel target_host and target_port are required"}
	}
	if !tcpTargetConfigured(cfg, host, port) {
		return "", ToolError{Code: "TCP_TUNNEL_TARGET_NOT_CONFIGURED", Message: "TCP tunnel target is not listed in local tcp_routes"}
	}
	if cfg.Policy.AllowNonLoopbackTCP || isLoopbackHost(host) {
		return net.JoinHostPort(host, strconv.Itoa(port)), nil
	}
	return "", ToolError{Code: "TCP_TUNNEL_FORBIDDEN_TARGET", Message: "TCP tunnel target must be loopback unless allow_non_loopback_tcp is enabled"}
}

func tcpTargetConfigured(cfg Config, host string, port int) bool {
	if len(cfg.TCPRoutes) == 0 {
		return false
	}
	for _, route := range cfg.TCPRoutes {
		if route.TargetPort != port {
			continue
		}
		if tcpHostMatches(route.TargetHost, host) {
			return true
		}
	}
	return false
}

func tcpHostMatches(configured string, requested string) bool {
	configured = normalizeTCPHost(configured)
	requested = normalizeTCPHost(requested)
	if configured == "" || requested == "" {
		return false
	}
	if configured == requested {
		return true
	}
	return isLoopbackHost(configured) && isLoopbackHost(requested)
}

func normalizeTCPHost(value string) string {
	return strings.ToLower(strings.Trim(strings.TrimSpace(value), "[]"))
}

func splitTCPTarget(value string) (string, int, error) {
	host, portText, err := net.SplitHostPort(strings.TrimSpace(value))
	if err != nil {
		return "", 0, ToolError{Code: "TCP_TUNNEL_INVALID_TARGET", Message: err.Error()}
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port <= 0 || port > 65535 {
		if err == nil {
			err = fmt.Errorf("invalid port %s", portText)
		}
		return "", 0, ToolError{Code: "TCP_TUNNEL_INVALID_TARGET", Message: err.Error()}
	}
	return host, port, nil
}
