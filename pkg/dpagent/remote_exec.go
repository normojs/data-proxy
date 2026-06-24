package dpagent

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/dto"
)

const (
	defaultRemoteRunTestsTimeoutMS = 120000
	remoteHardRunTestsTimeoutMS    = 600000
	defaultRemoteExecTimeoutMS     = 30000
	remoteHardExecTimeoutMS        = 600000
	defaultRemoteShellEvalWaitMS   = 1000
	remoteHardShellEvalWaitMS      = 30000
	defaultRemoteShellMaxSessions  = 8
	defaultRemoteInstallTimeoutMS  = 300000
	remoteHardInstallTimeoutMS     = 900000
)

var defaultRemoteShellSessions = newRemoteShellSessionRegistry(defaultRemoteShellMaxSessions)

type remoteInstallPackageCommand struct {
	Manager string
	Package string
	Name    string
	Args    []string
}

type remoteShellSpec struct {
	Name    string
	Args    []string
	Display string
}

type remoteShellSessionRegistry struct {
	mu       sync.Mutex
	max      int
	sessions map[string]*remoteShellSession
}

type remoteShellSession struct {
	id      string
	shell   string
	workdir string
	rel     string
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	tty     *os.File
	output  *remoteShellOutputBuffer
	cancel  context.CancelFunc

	mu        sync.Mutex
	closed    bool
	exitError error
	pty       bool
	createdAt time.Time
	lastUsed  time.Time
}

type remoteShellOutputBuffer struct {
	mu        sync.Mutex
	limit     int64
	data      []byte
	truncated bool
}

func (c BridgeClient) handleRemoteExec(ctx context.Context, args map[string]any) (dto.BridgeToolCallResult, error) {
	startedAt := time.Now()
	command, err := allowedRemoteExecCommand(c.Config, stringFromMap(args, "command", ""))
	if err != nil {
		return dto.BridgeToolCallResult{}, err
	}
	info, err := resolveExistingRemotePath(c.Config, stringFromMap(args, "workdir", ""), ".", "REMOTE_EXEC")
	if err != nil {
		return dto.BridgeToolCallResult{}, err
	}
	stat, err := os.Stat(info.Path)
	if err != nil {
		return dto.BridgeToolCallResult{}, ToolError{Code: "REMOTE_EXEC_NOT_FOUND", Message: err.Error()}
	}
	if !stat.IsDir() {
		return dto.BridgeToolCallResult{}, ToolError{Code: "REMOTE_EXEC_NOT_DIRECTORY", Message: "workdir is not a directory: " + info.Rel}
	}

	timeoutMS := remotePositiveInt(args["timeout_ms"], defaultRemoteExecTimeoutMS, remoteHardExecTimeoutMS)
	runCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMS)*time.Millisecond)
	defer cancel()
	cmd := remoteShellCommand(runCtx, command)
	cmd.Dir = info.Path
	output := &remoteExecOutputBuffer{limit: remoteLimitsFromConfig(c.Config, args).MaxResultBytes}
	cmd.Stdout = output
	cmd.Stderr = output
	err = cmd.Run()
	timedOut := runCtx.Err() == context.DeadlineExceeded
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else if timedOut {
			exitCode = -1
		} else {
			return dto.BridgeToolCallResult{}, ToolError{Code: "REMOTE_EXEC_FAILED", Message: err.Error()}
		}
	}
	text := output.String()
	if text == "" {
		text = fmt.Sprintf("command completed with exit code %d", exitCode)
	}
	if output.Truncated() {
		text += remoteTruncatedMarker
	}
	summary := fmt.Sprintf("remote_exec exited %d", exitCode)
	if timedOut {
		summary = fmt.Sprintf("remote_exec timed out after %dms", timeoutMS)
	}
	return dto.BridgeToolCallResult{
		Content:    []dto.MCPContentBlock{{Type: "text", Text: text}},
		Summary:    summary,
		DurationMS: int(time.Since(startedAt).Milliseconds()),
		ResultSize: len([]byte(text)),
		Metadata: map[string]any{
			"command":   command,
			"workdir":   info.Rel,
			"exit_code": exitCode,
			"timed_out": timedOut,
			"truncated": output.Truncated(),
		},
	}, nil
}

func allowedRemoteExecCommand(cfg Config, requested string) (string, error) {
	if !cfg.Policy.Exec.Enabled {
		return "", ToolError{Code: "REMOTE_EXEC_DISABLED", Message: "remote_exec requires policy.exec.enabled=true in data-proxy-agent config"}
	}
	if !cfg.Policy.Exec.AllowArbitrary {
		return "", ToolError{Code: "REMOTE_EXEC_DISABLED", Message: "remote_exec requires policy.exec.allow_arbitrary=true in data-proxy-agent config"}
	}
	command := strings.TrimSpace(requested)
	if command == "" {
		return "", ToolError{Code: "REMOTE_EXEC_INVALID_ARGUMENTS", Message: "command is required"}
	}
	return command, nil
}

func (c BridgeClient) handleRemoteShellOpen(ctx context.Context, args map[string]any) (dto.BridgeToolCallResult, error) {
	startedAt := time.Now()
	if err := requireRemoteTrustedExec(c.Config, "remote_shell_open"); err != nil {
		return dto.BridgeToolCallResult{}, err
	}
	spec, err := remoteShellSpecFromArg(stringFromMap(args, "shell", ""))
	if err != nil {
		return dto.BridgeToolCallResult{}, err
	}
	info, err := resolveExistingRemotePath(c.Config, stringFromMap(args, "workdir", ""), ".", "REMOTE_SHELL_OPEN")
	if err != nil {
		return dto.BridgeToolCallResult{}, err
	}
	stat, err := os.Stat(info.Path)
	if err != nil {
		return dto.BridgeToolCallResult{}, ToolError{Code: "REMOTE_SHELL_OPEN_NOT_FOUND", Message: err.Error()}
	}
	if !stat.IsDir() {
		return dto.BridgeToolCallResult{}, ToolError{Code: "REMOTE_SHELL_OPEN_NOT_DIRECTORY", Message: "workdir is not a directory: " + info.Rel}
	}
	usePTY := boolFromMap(args, "pty") || boolFromMap(args, "use_pty")
	cols := remotePositiveInt(args["cols"], 120, 500)
	rows := remotePositiveInt(args["rows"], 30, 200)
	session, err := defaultRemoteShellSessions.Open(ctx, spec, info, remoteLimitsFromConfig(c.Config, args).MaxResultBytes, usePTY, cols, rows)
	if err != nil {
		return dto.BridgeToolCallResult{}, err
	}
	time.Sleep(50 * time.Millisecond)
	initial, initialTruncated := session.output.Drain()
	payload := map[string]any{
		"session_id":     session.id,
		"shell":          session.shell,
		"workdir":        session.rel,
		"pty":            session.pty,
		"initial_output": initial,
	}
	text, truncated, err := encodeLimitedRemoteJSON(payload, remoteLimitsFromConfig(c.Config, args).MaxResultBytes)
	if err != nil {
		return dto.BridgeToolCallResult{}, err
	}
	return dto.BridgeToolCallResult{
		Content:    []dto.MCPContentBlock{{Type: "text", Text: text}},
		Summary:    "remote shell opened " + session.id,
		DurationMS: int(time.Since(startedAt).Milliseconds()),
		ResultSize: len([]byte(text)),
		Metadata: map[string]any{
			"session_id": session.id,
			"shell":      session.shell,
			"workdir":    session.rel,
			"pty":        session.pty,
			"truncated":  truncated || initialTruncated,
		},
	}, nil
}

func (c BridgeClient) handleRemoteShellEval(ctx context.Context, args map[string]any) (dto.BridgeToolCallResult, error) {
	startedAt := time.Now()
	if err := requireRemoteTrustedExec(c.Config, "remote_shell_eval"); err != nil {
		return dto.BridgeToolCallResult{}, err
	}
	sessionID := stringFromMap(args, "session_id", "")
	input, hasInput := remoteStringArg(args, "input")
	if sessionID == "" || !hasInput || input == "" {
		return dto.BridgeToolCallResult{}, ToolError{Code: "REMOTE_SHELL_EVAL_INVALID_ARGUMENTS", Message: "session_id and input are required"}
	}
	limits := remoteLimitsFromConfig(c.Config, args)
	if int64(len([]byte(input))) > limits.MaxWriteBytes {
		return dto.BridgeToolCallResult{}, ToolError{Code: "REMOTE_SHELL_EVAL_TOO_LARGE", Message: fmt.Sprintf("input exceeds max_write_bytes %d", limits.MaxWriteBytes)}
	}
	session, ok := defaultRemoteShellSessions.Get(sessionID)
	if !ok {
		return dto.BridgeToolCallResult{}, ToolError{Code: "REMOTE_SHELL_EVAL_NOT_FOUND", Message: "shell session not found: " + sessionID}
	}
	if err := session.Write(input); err != nil {
		return dto.BridgeToolCallResult{}, err
	}
	waitMS := remotePositiveInt(args["timeout_ms"], defaultRemoteShellEvalWaitMS, remoteHardShellEvalWaitMS)
	select {
	case <-time.After(time.Duration(waitMS) * time.Millisecond):
	case <-ctx.Done():
		return dto.BridgeToolCallResult{}, ctx.Err()
	}
	text, truncated := session.output.Drain()
	closed, exitMessage := session.Status()
	if text == "" {
		if closed && exitMessage != "" {
			text = exitMessage
		} else {
			text = "no output"
		}
	}
	if truncated {
		text += remoteTruncatedMarker
	}
	return dto.BridgeToolCallResult{
		Content:    []dto.MCPContentBlock{{Type: "text", Text: text}},
		Summary:    "remote shell eval " + session.id,
		DurationMS: int(time.Since(startedAt).Milliseconds()),
		ResultSize: len([]byte(text)),
		Metadata: map[string]any{
			"session_id": session.id,
			"shell":      session.shell,
			"workdir":    session.rel,
			"pty":        session.pty,
			"closed":     closed,
			"truncated":  truncated,
		},
	}, nil
}

func (c BridgeClient) handleRemoteShellResize(ctx context.Context, args map[string]any) (dto.BridgeToolCallResult, error) {
	startedAt := time.Now()
	if err := requireRemoteTrustedExec(c.Config, "remote_shell_resize"); err != nil {
		return dto.BridgeToolCallResult{}, err
	}
	sessionID := stringFromMap(args, "session_id", "")
	if sessionID == "" {
		return dto.BridgeToolCallResult{}, ToolError{Code: "REMOTE_SHELL_RESIZE_INVALID_ARGUMENTS", Message: "session_id is required"}
	}
	cols := remotePositiveInt(args["cols"], 120, 500)
	rows := remotePositiveInt(args["rows"], 30, 200)
	session, ok := defaultRemoteShellSessions.Get(sessionID)
	if !ok {
		return dto.BridgeToolCallResult{}, ToolError{Code: "REMOTE_SHELL_RESIZE_NOT_FOUND", Message: "shell session not found: " + sessionID}
	}
	if !session.pty || session.tty == nil {
		return dto.BridgeToolCallResult{}, ToolError{Code: "REMOTE_SHELL_RESIZE_NOT_PTY", Message: "shell session was not opened with pty=true"}
	}
	if err := resizeRemoteShellPTY(session.tty, cols, rows); err != nil {
		return dto.BridgeToolCallResult{}, ToolError{Code: "REMOTE_SHELL_RESIZE_FAILED", Message: err.Error()}
	}
	text := fmt.Sprintf("shell session %s resized to %dx%d", session.id, cols, rows)
	return dto.BridgeToolCallResult{
		Content:    []dto.MCPContentBlock{{Type: "text", Text: text}},
		Summary:    "remote shell resized " + session.id,
		DurationMS: int(time.Since(startedAt).Milliseconds()),
		ResultSize: len([]byte(text)),
		Metadata: map[string]any{
			"session_id": session.id,
			"cols":       cols,
			"rows":       rows,
			"pty":        true,
		},
	}, nil
}

func requireRemoteTrustedExec(cfg Config, toolName string) error {
	if !cfg.Policy.Exec.Enabled {
		return ToolError{Code: "REMOTE_TRUSTED_EXEC_DISABLED", Message: toolName + " requires policy.exec.enabled=true in data-proxy-agent config"}
	}
	if !cfg.Policy.Exec.AllowArbitrary {
		return ToolError{Code: "REMOTE_TRUSTED_EXEC_DISABLED", Message: toolName + " requires policy.exec.allow_arbitrary=true in data-proxy-agent config"}
	}
	return nil
}

func newRemoteShellSessionRegistry(max int) *remoteShellSessionRegistry {
	if max <= 0 {
		max = defaultRemoteShellMaxSessions
	}
	return &remoteShellSessionRegistry{max: max, sessions: map[string]*remoteShellSession{}}
}

func (r *remoteShellSessionRegistry) Open(ctx context.Context, spec remoteShellSpec, info remotePathInfo, maxOutputBytes int64, usePTY bool, cols int, rows int) (*remoteShellSession, error) {
	runCtx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(runCtx, spec.Name, spec.Args...)
	cmd.Dir = info.Path
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	session := &remoteShellSession{
		id:        newRemoteShellSessionID(),
		shell:     spec.Display,
		workdir:   info.Path,
		rel:       info.Rel,
		cmd:       cmd,
		output:    &remoteShellOutputBuffer{limit: maxOutputBytes},
		cancel:    cancel,
		pty:       false,
		createdAt: time.Now(),
		lastUsed:  time.Now(),
	}
	if usePTY {
		tty, err := startRemoteShellPTY(cmd, cols, rows)
		if err != nil {
			cancel()
			return nil, ToolError{Code: "REMOTE_SHELL_OPEN_PTY_FAILED", Message: err.Error()}
		}
		session.stdin = tty
		session.tty = tty
		session.pty = true
		go func() {
			_, _ = io.Copy(session.output, tty)
		}()
	} else {
		stdin, err := cmd.StdinPipe()
		if err != nil {
			cancel()
			return nil, ToolError{Code: "REMOTE_SHELL_OPEN_FAILED", Message: err.Error()}
		}
		session.stdin = stdin
		cmd.Stdout = session.output
		cmd.Stderr = session.output
		if err := cmd.Start(); err != nil {
			cancel()
			return nil, ToolError{Code: "REMOTE_SHELL_OPEN_FAILED", Message: err.Error()}
		}
	}
	r.mu.Lock()
	for len(r.sessions) >= r.max {
		r.closeOldestLocked()
	}
	r.sessions[session.id] = session
	r.mu.Unlock()
	go func() {
		err := cmd.Wait()
		session.MarkClosed(err)
		r.mu.Lock()
		delete(r.sessions, session.id)
		r.mu.Unlock()
	}()
	return session, nil
}

func (r *remoteShellSessionRegistry) Get(id string) (*remoteShellSession, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	session, ok := r.sessions[strings.TrimSpace(id)]
	return session, ok
}

func (r *remoteShellSessionRegistry) CloseAll() {
	r.mu.Lock()
	sessions := make([]*remoteShellSession, 0, len(r.sessions))
	for _, session := range r.sessions {
		sessions = append(sessions, session)
	}
	r.sessions = map[string]*remoteShellSession{}
	r.mu.Unlock()
	for _, session := range sessions {
		session.Close()
	}
}

func (r *remoteShellSessionRegistry) closeOldestLocked() {
	var oldest *remoteShellSession
	for _, session := range r.sessions {
		if oldest == nil || session.lastUsed.Before(oldest.lastUsed) {
			oldest = session
		}
	}
	if oldest != nil {
		oldest.Close()
		delete(r.sessions, oldest.id)
	}
}

func (s *remoteShellSession) Write(input string) error {
	s.mu.Lock()
	if s.closed {
		exitMessage := s.exitMessageLocked()
		s.mu.Unlock()
		return ToolError{Code: "REMOTE_SHELL_EVAL_CLOSED", Message: exitMessage}
	}
	s.lastUsed = time.Now()
	s.mu.Unlock()
	if !strings.HasSuffix(input, "\n") && !strings.HasSuffix(input, "\r") {
		input += "\n"
	}
	if _, err := io.WriteString(s.stdin, input); err != nil {
		return ToolError{Code: "REMOTE_SHELL_EVAL_FAILED", Message: err.Error()}
	}
	return nil
}

func (s *remoteShellSession) MarkClosed(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	s.exitError = err
}

func (s *remoteShellSession) Status() (bool, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.closed {
		return false, ""
	}
	return true, s.exitMessageLocked()
}

func (s *remoteShellSession) Close() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	s.mu.Unlock()
	_ = s.stdin.Close()
	if s.tty != nil {
		_ = s.tty.Close()
	}
	s.cancel()
}

func (s *remoteShellSession) exitMessageLocked() string {
	if s.exitError == nil {
		return "shell session closed"
	}
	return "shell session closed: " + s.exitError.Error()
}

func (b *remoteShellOutputBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.limit <= 0 {
		b.data = append(b.data, p...)
		return len(p), nil
	}
	remaining := b.limit - int64(len(b.data))
	if remaining <= 0 {
		b.truncated = true
		return len(p), nil
	}
	if int64(len(p)) <= remaining {
		b.data = append(b.data, p...)
		return len(p), nil
	}
	b.data = append(b.data, p[:remaining]...)
	b.truncated = true
	return len(p), nil
}

func (b *remoteShellOutputBuffer) Drain() (string, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	text := strings.TrimRight(string(b.data), "\r\n")
	truncated := b.truncated
	b.data = nil
	b.truncated = false
	return text, truncated
}

func remoteShellSpecFromArg(shell string) (remoteShellSpec, error) {
	shell = strings.TrimSpace(shell)
	if shell == "" {
		if runtime.GOOS == "windows" {
			return remoteShellSpec{Name: "cmd", Display: "cmd"}, nil
		}
		shell = strings.TrimSpace(os.Getenv("SHELL"))
		if shell == "" {
			shell = "/bin/sh"
		}
	}
	if strings.ContainsAny(shell, "\x00\r\n\t ") {
		return remoteShellSpec{}, ToolError{Code: "REMOTE_SHELL_OPEN_INVALID_SHELL", Message: "shell must be a single executable path or name"}
	}
	base := strings.ToLower(filepath.Base(shell))
	switch runtime.GOOS {
	case "windows":
		switch base {
		case "cmd", "cmd.exe", "powershell", "powershell.exe", "pwsh", "pwsh.exe":
			return remoteShellSpec{Name: shell, Display: shell}, nil
		default:
			return remoteShellSpec{}, ToolError{Code: "REMOTE_SHELL_OPEN_INVALID_SHELL", Message: "unsupported shell: " + shell}
		}
	default:
		switch base {
		case "sh", "bash", "zsh", "fish", "dash", "ksh":
			return remoteShellSpec{Name: shell, Display: shell}, nil
		default:
			return remoteShellSpec{}, ToolError{Code: "REMOTE_SHELL_OPEN_INVALID_SHELL", Message: "unsupported shell: " + shell}
		}
	}
}

func newRemoteShellSessionID() string {
	var bytes [8]byte
	if _, err := rand.Read(bytes[:]); err == nil {
		return "sh_" + hex.EncodeToString(bytes[:])
	}
	return fmt.Sprintf("sh_%d", time.Now().UnixNano())
}

func (c BridgeClient) handleRemoteInstallPackage(ctx context.Context, args map[string]any) (dto.BridgeToolCallResult, error) {
	startedAt := time.Now()
	install, err := allowedRemoteInstallPackageCommand(c.Config, args)
	if err != nil {
		return dto.BridgeToolCallResult{}, err
	}
	info, err := resolveExistingRemotePath(c.Config, stringFromMap(args, "workdir", ""), ".", "REMOTE_INSTALL_PACKAGE")
	if err != nil {
		return dto.BridgeToolCallResult{}, err
	}
	stat, err := os.Stat(info.Path)
	if err != nil {
		return dto.BridgeToolCallResult{}, ToolError{Code: "REMOTE_INSTALL_PACKAGE_NOT_FOUND", Message: err.Error()}
	}
	if !stat.IsDir() {
		return dto.BridgeToolCallResult{}, ToolError{Code: "REMOTE_INSTALL_PACKAGE_NOT_DIRECTORY", Message: "workdir is not a directory: " + info.Rel}
	}

	timeoutMS := remotePositiveInt(args["timeout_ms"], defaultRemoteInstallTimeoutMS, remoteHardInstallTimeoutMS)
	runCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMS)*time.Millisecond)
	defer cancel()
	cmd := exec.CommandContext(runCtx, install.Name, install.Args...)
	cmd.Dir = info.Path
	cmd.Env = append(os.Environ(),
		"CI=1",
		"GIT_TERMINAL_PROMPT=0",
		"NPM_CONFIG_AUDIT=false",
		"NPM_CONFIG_FUND=false",
		"PIP_DISABLE_PIP_VERSION_CHECK=1",
	)
	output := &remoteExecOutputBuffer{limit: remoteLimitsFromConfig(c.Config, args).MaxResultBytes}
	cmd.Stdout = output
	cmd.Stderr = output
	err = cmd.Run()
	timedOut := runCtx.Err() == context.DeadlineExceeded
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else if timedOut {
			exitCode = -1
		} else {
			return dto.BridgeToolCallResult{}, ToolError{Code: "REMOTE_INSTALL_PACKAGE_FAILED", Message: err.Error()}
		}
	}
	text := output.String()
	if text == "" {
		text = fmt.Sprintf("install command completed with exit code %d", exitCode)
	}
	if output.Truncated() {
		text += remoteTruncatedMarker
	}
	summary := fmt.Sprintf("%s install %s exited %d", install.Manager, install.Package, exitCode)
	if timedOut {
		summary = fmt.Sprintf("%s install %s timed out after %dms", install.Manager, install.Package, timeoutMS)
	}
	return dto.BridgeToolCallResult{
		Content:    []dto.MCPContentBlock{{Type: "text", Text: text}},
		Summary:    summary,
		DurationMS: int(time.Since(startedAt).Milliseconds()),
		ResultSize: len([]byte(text)),
		Metadata: map[string]any{
			"manager":   install.Manager,
			"package":   install.Package,
			"workdir":   info.Rel,
			"exit_code": exitCode,
			"timed_out": timedOut,
			"truncated": output.Truncated(),
		},
	}, nil
}

func allowedRemoteInstallPackageCommand(cfg Config, args map[string]any) (remoteInstallPackageCommand, error) {
	if !cfg.Policy.AllowWrite {
		return remoteInstallPackageCommand{}, ToolError{Code: "REMOTE_INSTALL_PACKAGE_DISABLED", Message: "remote_install_package requires policy.allow_write=true in data-proxy-agent config"}
	}
	if !cfg.Policy.Exec.Enabled {
		return remoteInstallPackageCommand{}, ToolError{Code: "REMOTE_INSTALL_PACKAGE_DISABLED", Message: "remote_install_package requires policy.exec.enabled=true in data-proxy-agent config"}
	}
	if !cfg.Policy.Exec.AllowArbitrary {
		return remoteInstallPackageCommand{}, ToolError{Code: "REMOTE_INSTALL_PACKAGE_DISABLED", Message: "remote_install_package requires policy.exec.allow_arbitrary=true in data-proxy-agent config"}
	}
	manager := strings.ToLower(strings.TrimSpace(stringFromMap(args, "manager", "")))
	pkg := strings.TrimSpace(firstString(args, "package", "name"))
	if manager == "" || pkg == "" {
		return remoteInstallPackageCommand{}, ToolError{Code: "REMOTE_INSTALL_PACKAGE_INVALID_ARGUMENTS", Message: "manager and package are required"}
	}
	if err := validateRemotePackageSpec(pkg); err != nil {
		return remoteInstallPackageCommand{}, err
	}
	name, installArgs, err := remotePackageManagerCommand(manager, pkg)
	if err != nil {
		return remoteInstallPackageCommand{}, err
	}
	return remoteInstallPackageCommand{
		Manager: manager,
		Package: pkg,
		Name:    name,
		Args:    installArgs,
	}, nil
}

func validateRemotePackageSpec(pkg string) error {
	if len(pkg) > 512 {
		return ToolError{Code: "REMOTE_INSTALL_PACKAGE_INVALID_ARGUMENTS", Message: "package is too long"}
	}
	if strings.HasPrefix(pkg, "-") {
		return ToolError{Code: "REMOTE_INSTALL_PACKAGE_INVALID_ARGUMENTS", Message: "package must not start with '-'"}
	}
	for _, r := range pkg {
		if r == 0 || r == '\n' || r == '\r' || r == '\t' || r == ' ' {
			return ToolError{Code: "REMOTE_INSTALL_PACKAGE_INVALID_ARGUMENTS", Message: "package must be a single package spec without whitespace"}
		}
	}
	return nil
}

func remotePackageManagerCommand(manager string, pkg string) (string, []string, error) {
	switch manager {
	case "npm":
		return "npm", []string{"install", pkg}, nil
	case "pnpm":
		return "pnpm", []string{"add", pkg}, nil
	case "yarn":
		return "yarn", []string{"add", pkg}, nil
	case "bun":
		return "bun", []string{"add", pkg}, nil
	case "go":
		return "go", []string{"get", pkg}, nil
	case "cargo":
		return "cargo", []string{"add", pkg}, nil
	case "pip", "pip3":
		if runtime.GOOS == "windows" {
			return "python", []string{"-m", "pip", "install", pkg}, nil
		}
		return "python3", []string{"-m", "pip", "install", pkg}, nil
	case "uv":
		return "uv", []string{"add", pkg}, nil
	case "composer":
		return "composer", []string{"require", pkg}, nil
	default:
		return "", nil, ToolError{Code: "REMOTE_INSTALL_PACKAGE_UNSUPPORTED_MANAGER", Message: "unsupported package manager: " + manager}
	}
}

func (c BridgeClient) handleRemoteRunTests(ctx context.Context, args map[string]any) (dto.BridgeToolCallResult, error) {
	startedAt := time.Now()
	command, err := allowedRemoteRunTestsCommand(c.Config, stringFromMap(args, "command", ""))
	if err != nil {
		return dto.BridgeToolCallResult{}, err
	}
	info, err := resolveExistingRemotePath(c.Config, stringFromMap(args, "workdir", ""), ".", "REMOTE_RUN_TESTS")
	if err != nil {
		return dto.BridgeToolCallResult{}, err
	}
	stat, err := os.Stat(info.Path)
	if err != nil {
		return dto.BridgeToolCallResult{}, ToolError{Code: "REMOTE_RUN_TESTS_NOT_FOUND", Message: err.Error()}
	}
	if !stat.IsDir() {
		return dto.BridgeToolCallResult{}, ToolError{Code: "REMOTE_RUN_TESTS_NOT_DIRECTORY", Message: "workdir is not a directory: " + info.Rel}
	}

	timeoutMS := remotePositiveInt(args["timeout_ms"], defaultRemoteRunTestsTimeoutMS, remoteHardRunTestsTimeoutMS)
	runCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMS)*time.Millisecond)
	defer cancel()
	cmd := remoteShellCommand(runCtx, command)
	cmd.Dir = info.Path
	output := &remoteExecOutputBuffer{limit: remoteLimitsFromConfig(c.Config, args).MaxResultBytes}
	cmd.Stdout = output
	cmd.Stderr = output
	err = cmd.Run()
	timedOut := runCtx.Err() == context.DeadlineExceeded
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else if timedOut {
			exitCode = -1
		} else {
			return dto.BridgeToolCallResult{}, ToolError{Code: "REMOTE_RUN_TESTS_FAILED", Message: err.Error()}
		}
	}
	text := output.String()
	if text == "" {
		text = fmt.Sprintf("command completed with exit code %d", exitCode)
	}
	if output.Truncated() {
		text += remoteTruncatedMarker
	}
	summary := fmt.Sprintf("%s exited %d", command, exitCode)
	if timedOut {
		summary = fmt.Sprintf("%s timed out after %dms", command, timeoutMS)
	}
	return dto.BridgeToolCallResult{
		Content:    []dto.MCPContentBlock{{Type: "text", Text: text}},
		Summary:    summary,
		DurationMS: int(time.Since(startedAt).Milliseconds()),
		ResultSize: len([]byte(text)),
		Metadata: map[string]any{
			"command":   command,
			"workdir":   info.Rel,
			"exit_code": exitCode,
			"timed_out": timedOut,
			"truncated": output.Truncated(),
		},
	}, nil
}

func allowedRemoteRunTestsCommand(cfg Config, requested string) (string, error) {
	if !cfg.Policy.Exec.Enabled {
		return "", ToolError{Code: "REMOTE_RUN_TESTS_DISABLED", Message: "remote_run_tests requires policy.exec.enabled=true in data-proxy-agent config"}
	}
	allowed := normalizedSafeCommands(cfg.Policy.Exec.SafeCommands)
	if len(allowed) == 0 {
		return "", ToolError{Code: "REMOTE_RUN_TESTS_DISABLED", Message: "remote_run_tests requires policy.exec.safe_commands in data-proxy-agent config"}
	}
	command := strings.TrimSpace(requested)
	if command == "" {
		command = allowed[0]
	}
	for _, item := range allowed {
		if command == item {
			return command, nil
		}
	}
	return "", ToolError{Code: "REMOTE_RUN_TESTS_FORBIDDEN", Message: "command is not in policy.exec.safe_commands"}
}

func normalizedSafeCommands(commands []string) []string {
	result := make([]string, 0, len(commands))
	seen := map[string]bool{}
	for _, command := range commands {
		command = strings.TrimSpace(command)
		if command == "" || seen[command] {
			continue
		}
		seen[command] = true
		result = append(result, command)
	}
	return result
}

func remoteShellCommand(ctx context.Context, command string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.CommandContext(ctx, "cmd", "/C", command)
	}
	return exec.CommandContext(ctx, "/bin/sh", "-c", command)
}

type remoteExecOutputBuffer struct {
	mu        sync.Mutex
	limit     int64
	data      []byte
	total     int64
	truncated bool
}

func (b *remoteExecOutputBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.total += int64(len(p))
	if b.limit <= 0 || int64(len(b.data)) >= b.limit {
		b.truncated = b.truncated || b.limit > 0
		return len(p), nil
	}
	remaining := b.limit - int64(len(b.data))
	if int64(len(p)) <= remaining {
		b.data = append(b.data, p...)
		return len(p), nil
	}
	b.data = append(b.data, p[:remaining]...)
	b.truncated = true
	return len(p), nil
}

func (b *remoteExecOutputBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return strings.TrimRight(string(b.data), "\r\n")
}

func (b *remoteExecOutputBuffer) Truncated() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.truncated || (b.limit > 0 && b.total > b.limit)
}
