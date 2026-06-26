package dpagent

import (
	"context"
	"encoding/base64"
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
)

func TestHandleTCPTunnelStreamEcho(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()

	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		buffer := make([]byte, 16)
		n, err := conn.Read(buffer)
		if err != nil {
			return
		}
		_, _ = conn.Write([]byte("echo:" + string(buffer[:n])))
	}()

	queue := newBridgeStreamInputQueue()
	var mu sync.Mutex
	var chunks []dto.BridgeToolStreamChunk
	client := BridgeClient{Config: Config{
		TCPRoutes: []TCPRoute{{
			Name:       "echo",
			TargetHost: "127.0.0.1",
			TargetPort: listener.Addr().(*net.TCPAddr).Port,
		}},
	}}
	resultCh := make(chan dto.BridgeToolCallResult, 1)
	errCh := make(chan error, 1)
	go func() {
		result, err := client.handleTCPTunnelStream(context.Background(), map[string]any{
			"target": listener.Addr().String(),
		}, func(chunk dto.BridgeToolStreamChunk) error {
			mu.Lock()
			defer mu.Unlock()
			chunks = append(chunks, chunk)
			return nil
		}, queue)
		if err != nil {
			errCh <- err
			return
		}
		resultCh <- result
	}()

	require.True(t, queue.Push(dto.BridgeToolStreamInput{
		FrameType:  tcpTunnelFrameData,
		BodyBase64: base64.StdEncoding.EncodeToString([]byte("ping")),
	}))

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case result := <-resultCh:
		require.Contains(t, result.Summary, "TCP")
		require.Equal(t, len("echo:ping"), result.ResultSize)
	case <-time.After(3 * time.Second):
		t.Fatal("tcp tunnel handler timed out")
	}
	<-serverDone

	mu.Lock()
	defer mu.Unlock()
	var sawConnected bool
	var sawBody bool
	var sawDone bool
	for _, chunk := range chunks {
		if chunk.Metadata != nil && chunk.Metadata["connected"] == true {
			sawConnected = true
		}
		if chunk.BodyBase64 == base64.StdEncoding.EncodeToString([]byte("echo:ping")) {
			sawBody = true
		}
		if chunk.Done {
			sawDone = true
		}
	}
	require.True(t, sawConnected)
	require.True(t, sawBody)
	require.True(t, sawDone)
}

func TestHandleTCPTunnelStreamEmitterFailureReturns(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()

	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_, _ = conn.Write([]byte("server-push"))
	}()

	queue := newBridgeStreamInputQueue()
	client := BridgeClient{Config: Config{
		TCPRoutes: []TCPRoute{{
			Name:       "echo",
			TargetHost: "127.0.0.1",
			TargetPort: listener.Addr().(*net.TCPAddr).Port,
		}},
	}}
	emitErr := errors.New("tcp bridge stream emit failed")
	var emitCalls int
	_, err = client.handleTCPTunnelStream(context.Background(), map[string]any{
		"target": listener.Addr().String(),
	}, func(chunk dto.BridgeToolStreamChunk) error {
		emitCalls++
		if chunk.BodyBase64 != "" {
			return emitErr
		}
		return nil
	}, queue)
	require.ErrorIs(t, err, emitErr)
	require.GreaterOrEqual(t, emitCalls, 2)

	select {
	case <-serverDone:
	case <-time.After(3 * time.Second):
		t.Fatal("tcp test server did not finish")
	}
}

func TestHandleTCPTunnelStreamInputErrorFrame(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()

	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_, _ = conn.Read(make([]byte, 1))
	}()

	queue := newBridgeStreamInputQueue()
	require.True(t, queue.Push(dto.BridgeToolStreamInput{
		FrameType:    tcpTunnelFrameData,
		ErrorCode:    "CLIENT_ABORTED",
		ErrorMessage: "client tcp stream aborted",
	}))

	var mu sync.Mutex
	var chunks []dto.BridgeToolStreamChunk
	client := BridgeClient{Config: Config{
		TCPRoutes: []TCPRoute{{
			Name:       "echo",
			TargetHost: "127.0.0.1",
			TargetPort: listener.Addr().(*net.TCPAddr).Port,
		}},
	}}
	_, err = client.handleTCPTunnelStream(context.Background(), map[string]any{
		"target": listener.Addr().String(),
	}, func(chunk dto.BridgeToolStreamChunk) error {
		mu.Lock()
		defer mu.Unlock()
		chunks = append(chunks, chunk)
		return nil
	}, queue)
	var toolErr ToolError
	require.ErrorAs(t, err, &toolErr)
	require.Equal(t, "CLIENT_ABORTED", toolErr.Code)
	require.Contains(t, toolErr.Message, "client tcp stream aborted")

	select {
	case <-serverDone:
	case <-time.After(3 * time.Second):
		t.Fatal("tcp test server did not observe connection close")
	}

	mu.Lock()
	defer mu.Unlock()
	require.NotEmpty(t, chunks)
	require.Equal(t, true, chunks[0].Metadata["connected"])
}

func TestAllowedTCPTargetRequiresConfiguredRoute(t *testing.T) {
	target, err := allowedTCPTarget(Config{
		TCPRoutes: []TCPRoute{{Name: "ssh", TargetHost: "127.0.0.1", TargetPort: 22}},
	}, tcpTunnelArgs{
		Target: "localhost:22",
	})
	require.NoError(t, err)
	require.Contains(t, target, ":22")

	_, err = allowedTCPTarget(Config{
		TCPRoutes: []TCPRoute{{Name: "ssh", TargetHost: "127.0.0.1", TargetPort: 22}},
	}, tcpTunnelArgs{
		Target: "127.0.0.1:3306",
	})
	var toolErr ToolError
	require.ErrorAs(t, err, &toolErr)
	require.Equal(t, "TCP_TUNNEL_TARGET_NOT_CONFIGURED", toolErr.Code)
}

func TestEffectiveCapabilitiesIncludesTCPTunnelWhenConfigured(t *testing.T) {
	cfg := DefaultConfig()
	cfg.TCPRoutes = []TCPRoute{{Name: "ssh", TargetHost: "127.0.0.1", TargetPort: 22}}
	require.Contains(t, EffectiveCapabilities(cfg), BridgeCapabilityTCPTunnel)
}
