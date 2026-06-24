package dpagent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

var defaultMCPStdioSessions = newMCPStdioSessionCache()

type mcpStdioSessionCache struct {
	mu       sync.Mutex
	sessions map[string]*mcpStdioSession
}

func newMCPStdioSessionCache() *mcpStdioSessionCache {
	return &mcpStdioSessionCache{sessions: map[string]*mcpStdioSession{}}
}

func (c *mcpStdioSessionCache) GetOrStart(key string, server MCPServer, cfg Config) (*mcpStdioSession, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if session := c.sessions[key]; session != nil {
		if session.Alive() {
			return session, nil
		}
		delete(c.sessions, key)
	}
	session, err := startMCPStdioSession(key, server, cfg)
	if err != nil {
		return nil, err
	}
	c.sessions[key] = session
	return session, nil
}

func (c *mcpStdioSessionCache) Initialized(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	session := c.sessions[key]
	return session != nil && session.Alive() && session.Initialized()
}

func (c *mcpStdioSessionCache) MarkInitialized(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if session := c.sessions[key]; session != nil && session.Alive() {
		session.MarkInitialized()
	}
}

func (c *mcpStdioSessionCache) Forget(key string) {
	c.mu.Lock()
	session := c.sessions[key]
	delete(c.sessions, key)
	c.mu.Unlock()
	if session != nil {
		session.Close()
	}
}

type mcpStdioSession struct {
	key            string
	server         MCPServer
	command        string
	cmd            *exec.Cmd
	stdin          io.WriteCloser
	stdinWriter    *bufio.Writer
	stdout         *bufio.Reader
	stderr         *limitedBuffer
	maxResultBytes int64
	mu             sync.Mutex
	stateMu        sync.Mutex
	initialized    bool
	done           chan struct{}
	waitErr        error
	closeOnce      sync.Once
}

func startMCPStdioSession(key string, server MCPServer, cfg Config) (*mcpStdioSession, error) {
	command := strings.TrimSpace(server.Command)
	if command == "" {
		return nil, ToolError{Code: "MCP_PROXY_STDIO_NOT_CONFIGURED", Message: fmt.Sprintf("local MCP server %q has no command", server.Name)}
	}
	cmd := stdioShellCommand(command)
	if workspace := strings.TrimSpace(expandPath(cfg.Agent.Workspace)); workspace != "" {
		if info, err := os.Stat(workspace); err == nil && info.IsDir() {
			cmd.Dir = workspace
		}
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, ToolError{Code: "MCP_PROXY_STDIO_START_FAILED", Message: err.Error()}
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, ToolError{Code: "MCP_PROXY_STDIO_START_FAILED", Message: err.Error()}
	}
	stderr := &limitedBuffer{limit: 16 * 1024}
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		return nil, ToolError{Code: "MCP_PROXY_STDIO_START_FAILED", Message: err.Error()}
	}
	maxResultBytes := cfg.Runtime.MaxResultBytes
	if maxResultBytes <= 0 {
		maxResultBytes = DefaultMaxResultBytes
	}
	session := &mcpStdioSession{
		key:            key,
		server:         server,
		command:        command,
		cmd:            cmd,
		stdin:          stdin,
		stdinWriter:    bufio.NewWriter(stdin),
		stdout:         bufio.NewReader(stdout),
		stderr:         stderr,
		maxResultBytes: maxResultBytes,
		done:           make(chan struct{}),
	}
	go func() {
		session.waitErr = cmd.Wait()
		close(session.done)
	}()
	return session, nil
}

func stdioShellCommand(command string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.Command("cmd", "/C", command)
	}
	return exec.Command("/bin/sh", "-c", command)
}

func (s *mcpStdioSession) Alive() bool {
	select {
	case <-s.done:
		return false
	default:
		return true
	}
}

func (s *mcpStdioSession) Initialized() bool {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	return s.initialized
}

func (s *mcpStdioSession) MarkInitialized() {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	s.initialized = true
}

func (s *mcpStdioSession) Close() {
	s.closeOnce.Do(func() {
		_ = s.stdin.Close()
		if s.cmd != nil && s.cmd.Process != nil && s.Alive() {
			_ = s.cmd.Process.Kill()
		}
		select {
		case <-s.done:
		default:
		}
	})
}

func (c BridgeClient) mcpStdioRPC(ctx context.Context, endpoint mcpProxyEndpoint, bodyBytes []byte, notification bool) (mcpRPCResponse, error) {
	session, err := defaultMCPStdioSessions.GetOrStart(endpoint.Key, endpoint.StdioServer, c.Config)
	if err != nil {
		return mcpRPCResponse{}, err
	}
	return session.call(ctx, bodyBytes, notification)
}

func (s *mcpStdioSession) call(ctx context.Context, bodyBytes []byte, notification bool) (mcpRPCResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.Alive() {
		return mcpRPCResponse{}, ToolError{Code: "MCP_PROXY_STDIO_EXITED", Message: s.exitMessage()}
	}
	if err := writeMCPStdioFrame(s.stdinWriter, bodyBytes); err != nil {
		return mcpRPCResponse{}, ToolError{Code: "MCP_PROXY_STDIO_WRITE_FAILED", Message: err.Error()}
	}
	if notification {
		return mcpRPCResponse{Result: map[string]any{}, SessionID: s.key}, nil
	}

	type readResult struct {
		body []byte
		err  error
	}
	readCh := make(chan readResult, 1)
	go func() {
		body, err := readMCPStdioFrame(s.stdout, s.maxResultBytes)
		readCh <- readResult{body: body, err: err}
	}()

	select {
	case <-ctx.Done():
		s.Close()
		return mcpRPCResponse{}, ToolError{Code: "MCP_PROXY_TIMEOUT", Message: ctx.Err().Error()}
	case <-s.done:
		return mcpRPCResponse{}, ToolError{Code: "MCP_PROXY_STDIO_EXITED", Message: s.exitMessage()}
	case result := <-readCh:
		if result.err != nil {
			return mcpRPCResponse{}, ToolError{Code: "MCP_PROXY_STDIO_READ_FAILED", Message: result.err.Error()}
		}
		var object map[string]any
		if err := json.Unmarshal(result.body, &object); err != nil {
			return mcpRPCResponse{}, ToolError{Code: "MCP_PROXY_DECODE_FAILED", Message: err.Error()}
		}
		if errObject, ok := object["error"].(map[string]any); ok && len(errObject) > 0 {
			code := "MCP_PROXY_UPSTREAM_ERROR"
			if rawCode, ok := errObject["code"]; ok {
				code = "MCP_PROXY_UPSTREAM_" + sanitizeErrorCode(fmt.Sprint(rawCode))
			}
			return mcpRPCResponse{}, ToolError{Code: code, Message: stringFromMap(errObject, "message", "MCP upstream error")}
		}
		rawResult := object["result"]
		return mcpRPCResponse{Result: mapFromAny(rawResult), RawResult: rawResult, SessionID: s.key}, nil
	}
}

func (s *mcpStdioSession) exitMessage() string {
	message := "MCP stdio process exited"
	if s.waitErr != nil {
		message += ": " + s.waitErr.Error()
	}
	if stderr := strings.TrimSpace(s.stderr.String()); stderr != "" {
		message += ": " + truncateString(stderr, 512)
	}
	return message
}

func writeMCPStdioFrame(writer *bufio.Writer, body []byte) error {
	if _, err := fmt.Fprintf(writer, "Content-Length: %d\r\n\r\n", len(body)); err != nil {
		return err
	}
	if _, err := writer.Write(body); err != nil {
		return err
	}
	return writer.Flush()
}

func readMCPStdioFrame(reader *bufio.Reader, maxBytes int64) ([]byte, error) {
	for {
		contentLength := int64(-1)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return nil, err
			}
			line = strings.TrimRight(line, "\r\n")
			if line == "" {
				if contentLength >= 0 {
					if maxBytes > 0 && contentLength > maxBytes {
						return nil, fmt.Errorf("MCP stdio response exceeds max_result_bytes: %d > %d", contentLength, maxBytes)
					}
					body := make([]byte, contentLength)
					if _, err := io.ReadFull(reader, body); err != nil {
						return nil, err
					}
					return body, nil
				}
				continue
			}
			name, value, ok := strings.Cut(line, ":")
			if !ok {
				continue
			}
			if strings.EqualFold(strings.TrimSpace(name), "Content-Length") {
				parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
				if err != nil || parsed < 0 {
					return nil, fmt.Errorf("invalid MCP stdio Content-Length: %q", value)
				}
				contentLength = parsed
			}
		}
	}
}

type limitedBuffer struct {
	mu    sync.Mutex
	limit int
	data  []byte
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.data = append(b.data, p...)
	if b.limit > 0 && len(b.data) > b.limit {
		b.data = append([]byte(nil), b.data[len(b.data)-b.limit:]...)
	}
	return len(p), nil
}

func (b *limitedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(bytes.TrimSpace(b.data))
}
