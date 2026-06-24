package dpagent

import (
	"context"
	"fmt"
	"os"
	"os/exec"
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
	defaultRemoteInstallTimeoutMS  = 300000
	remoteHardInstallTimeoutMS     = 900000
)

type remoteInstallPackageCommand struct {
	Manager string
	Package string
	Name    string
	Args    []string
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
