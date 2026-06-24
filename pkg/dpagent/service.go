package dpagent

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"html"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultServiceName      = "data-proxy-agent"
	DefaultLaunchdLabel     = "ltd.mbu.dataproxy.agent"
	DefaultServiceExecStart = "run"
)

type ServiceOptions struct {
	Command     string
	ConfigPath  string
	BinaryPath  string
	Name        string
	Scope       string
	Platform    string
	DryRun      bool
	Timeout     time.Duration
	StdoutPath  string
	StderrPath  string
	CommandExec commandRunner
	Out         io.Writer
}

type ServiceHealthOptions struct {
	ConfigPath  string
	BinaryPath  string
	Name        string
	Scope       string
	Platform    string
	Timeout     time.Duration
	CommandExec commandRunner
}

type ServiceDefinition struct {
	Platform    string
	Scope       string
	Name        string
	Label       string
	ConfigPath  string
	BinaryPath  string
	InstallPath string
	Content     string
}

type commandRunner func(ctx context.Context, name string, args ...string) ([]byte, error)

func (c CLI) runService(args []string) int {
	if len(args) == 0 || isHelpArg(args[0]) {
		c.printServiceHelp()
		if len(args) == 0 {
			return 2
		}
		return 0
	}
	subcommand := strings.ToLower(strings.TrimSpace(args[0]))
	switch subcommand {
	case "install", "uninstall", "start", "stop", "restart", "status", "print":
	default:
		fmt.Fprintf(c.Err, "unknown service subcommand: %s\n", subcommand)
		c.printServiceHelp()
		return 2
	}
	fs := flag.NewFlagSet("service "+subcommand, flag.ContinueOnError)
	fs.SetOutput(c.Err)
	configPath := fs.String("config", "", "config path")
	binaryPath := fs.String("binary", "", "agent binary path")
	name := fs.String("name", DefaultServiceName, "service name")
	scope := fs.String("scope", "", "service scope: user or system")
	platform := fs.String("platform", "", "target platform override for dry-run/print")
	dryRun := fs.Bool("dry-run", false, "print actions without changing the system")
	userScope := fs.Bool("user", false, "install as user service")
	systemScope := fs.Bool("system", false, "install as system service")
	stdoutPath := fs.String("stdout", "", "service stdout log path")
	stderrPath := fs.String("stderr", "", "service stderr log path")
	timeout := fs.Duration("timeout", 30*time.Second, "service command timeout")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	if *userScope && *systemScope {
		fmt.Fprintln(c.Err, "--user and --system cannot be used together")
		return 2
	}
	if *userScope {
		*scope = "user"
	}
	if *systemScope {
		*scope = "system"
	}
	opts := ServiceOptions{
		Command:    subcommand,
		ConfigPath: *configPath,
		BinaryPath: *binaryPath,
		Name:       *name,
		Scope:      *scope,
		Platform:   *platform,
		DryRun:     *dryRun || subcommand == "print",
		Timeout:    *timeout,
		StdoutPath: *stdoutPath,
		StderrPath: *stderrPath,
		Out:        c.Out,
	}
	if err := RunServiceCommand(context.Background(), opts); err != nil {
		fmt.Fprintln(c.Err, err)
		return 1
	}
	return 0
}

func (c CLI) printServiceHelp() {
	program := c.programName()
	fmt.Fprintf(c.Out, `Usage:
  %[1]s service install [--config <path>] [--user|--system] [--dry-run]
  %[1]s service uninstall [--config <path>] [--user|--system] [--dry-run]
  %[1]s service start|stop|restart|status [--config <path>] [--user|--system]
  %[1]s service print [--config <path>] [--platform linux|darwin|windows]

Service commands manage systemd, launchd, or Windows Service definitions for the local agent.
`, program)
}

func RunServiceCommand(ctx context.Context, opts ServiceOptions) error {
	def, err := BuildServiceDefinition(opts)
	if err != nil {
		return err
	}
	if opts.Out == nil {
		opts.Out = io.Discard
	}
	switch opts.Command {
	case "install":
		if opts.DryRun {
			printServiceDryRun(opts.Out, def, "install")
			return nil
		}
		return installService(ctx, opts, def)
	case "uninstall":
		if opts.DryRun {
			printServiceDryRun(opts.Out, def, "uninstall")
			return nil
		}
		return uninstallService(ctx, opts, def)
	case "start", "stop", "status":
		if opts.DryRun {
			printServiceDryRun(opts.Out, def, opts.Command)
			return nil
		}
		return runServiceAction(ctx, opts, def, opts.Command)
	case "restart":
		if opts.DryRun {
			printServiceDryRun(opts.Out, def, "restart")
			return nil
		}
		if err := runServiceAction(ctx, opts, def, "stop"); err != nil {
			return err
		}
		return runServiceAction(ctx, opts, def, "start")
	case "print":
		printServiceDryRun(opts.Out, def, "install")
		return nil
	default:
		return fmt.Errorf("unknown service command: %s", opts.Command)
	}
}

func CheckServiceStatus(ctx context.Context, opts ServiceHealthOptions) AgentHealthCheck {
	def, err := BuildServiceDefinition(ServiceOptions{
		ConfigPath: opts.ConfigPath,
		BinaryPath: opts.BinaryPath,
		Name:       opts.Name,
		Scope:      opts.Scope,
		Platform:   opts.Platform,
	})
	if err != nil {
		return AgentHealthCheck{Name: "service_status", Status: HealthStatusWarn, Detail: "service definition unavailable: " + err.Error()}
	}
	if def.Platform == "linux" || def.Platform == "darwin" {
		if _, err := os.Stat(def.InstallPath); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return AgentHealthCheck{Name: "service_status", Status: HealthStatusWarn, Detail: "not installed: " + def.InstallPath}
			}
			return AgentHealthCheck{Name: "service_status", Status: HealthStatusWarn, Detail: "cannot inspect service file: " + err.Error()}
		}
	}
	return queryServiceStatus(ctx, opts, def)
}

func BuildServiceDefinition(opts ServiceOptions) (ServiceDefinition, error) {
	platform := strings.ToLower(strings.TrimSpace(opts.Platform))
	if platform == "" {
		platform = runtime.GOOS
	}
	switch platform {
	case "linux", "darwin", "windows":
	default:
		return ServiceDefinition{}, fmt.Errorf("service command is not supported on %s", platform)
	}
	name := serviceName(opts.Name)
	scope := serviceScope(opts.Scope, platform)
	configPath, err := resolveServiceConfigPath(opts.ConfigPath)
	if err != nil {
		return ServiceDefinition{}, err
	}
	binaryPath, err := resolveServiceBinaryPath(opts.BinaryPath)
	if err != nil {
		return ServiceDefinition{}, err
	}
	def := ServiceDefinition{
		Platform:   platform,
		Scope:      scope,
		Name:       name,
		Label:      serviceLabel(name),
		ConfigPath: configPath,
		BinaryPath: binaryPath,
	}
	switch platform {
	case "linux":
		def.InstallPath, err = linuxServicePath(name, scope)
		if err == nil {
			def.Content = linuxServiceUnit(def)
		}
	case "darwin":
		def.InstallPath, err = darwinServicePath(def.Label, scope)
		if err == nil {
			def.Content = darwinLaunchdPlist(def, opts.StdoutPath, opts.StderrPath)
		}
	case "windows":
		def.InstallPath = `HKLM\SYSTEM\CurrentControlSet\Services\` + windowsServiceName(name)
		def.Content = windowsServiceCommandLine(def)
	}
	if err != nil {
		return ServiceDefinition{}, err
	}
	return def, nil
}

func installService(ctx context.Context, opts ServiceOptions, def ServiceDefinition) error {
	switch def.Platform {
	case "linux", "darwin":
		if err := os.MkdirAll(filepath.Dir(def.InstallPath), DefaultConfigFolderMode); err != nil {
			return err
		}
		if err := os.WriteFile(def.InstallPath, []byte(def.Content), 0o644); err != nil {
			return err
		}
	case "windows":
		if err := runPlatformCommand(ctx, opts, "sc.exe", append([]string{"create", windowsServiceName(def.Name)}, windowsCreateArgs(def)...)...); err != nil {
			return err
		}
		fmt.Fprintf(opts.Out, "service installed: %s\n", windowsServiceName(def.Name))
		return nil
	}
	if err := runServiceAction(ctx, opts, def, "enable"); err != nil {
		return err
	}
	fmt.Fprintf(opts.Out, "service installed: %s\n", def.InstallPath)
	return nil
}

func uninstallService(ctx context.Context, opts ServiceOptions, def ServiceDefinition) error {
	_ = runServiceAction(ctx, opts, def, "stop")
	_ = runServiceAction(ctx, opts, def, "disable")
	switch def.Platform {
	case "linux", "darwin":
		if err := os.Remove(def.InstallPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	case "windows":
		return runPlatformCommand(ctx, opts, "sc.exe", "delete", windowsServiceName(def.Name))
	}
	if def.Platform == "linux" {
		_ = runLinuxSystemctl(ctx, opts, def, "daemon-reload")
	}
	fmt.Fprintf(opts.Out, "service uninstalled: %s\n", def.Name)
	return nil
}

func runServiceAction(ctx context.Context, opts ServiceOptions, def ServiceDefinition, action string) error {
	switch def.Platform {
	case "linux":
		return runLinuxServiceAction(ctx, opts, def, action)
	case "darwin":
		return runDarwinServiceAction(ctx, opts, def, action)
	case "windows":
		return runWindowsServiceAction(ctx, opts, def, action)
	default:
		return fmt.Errorf("service command is not supported on %s", def.Platform)
	}
}

func queryServiceStatus(ctx context.Context, opts ServiceHealthOptions, def ServiceDefinition) AgentHealthCheck {
	name, args := serviceStatusCommand(def)
	output, err := runServiceStatusCommand(ctx, opts, name, args...)
	switch def.Platform {
	case "linux":
		return parseLinuxServiceStatus(def, output, err)
	case "darwin":
		return parseDarwinServiceStatus(def, output, err)
	case "windows":
		return parseWindowsServiceStatus(def, output, err)
	default:
		return AgentHealthCheck{Name: "service_status", Status: HealthStatusWarn, Detail: "unsupported platform: " + def.Platform}
	}
}

func serviceStatusCommand(def ServiceDefinition) (string, []string) {
	switch def.Platform {
	case "linux":
		args := []string{"is-active", filepath.Base(def.InstallPath)}
		if def.Scope == "user" {
			args = append([]string{"--user"}, args...)
		}
		return "systemctl", args
	case "darwin":
		return "launchctl", []string{"print", darwinLaunchdDomain(def) + "/" + def.Label}
	case "windows":
		return "sc.exe", []string{"query", windowsServiceName(def.Name)}
	default:
		return "", nil
	}
}

func runServiceStatusCommand(ctx context.Context, opts ServiceHealthOptions, name string, args ...string) ([]byte, error) {
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("missing service status command")
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	runner := opts.CommandExec
	if runner == nil {
		runner = defaultCommandRunner
	}
	return runner(cmdCtx, name, args...)
}

func parseLinuxServiceStatus(def ServiceDefinition, output []byte, err error) AgentHealthCheck {
	state := strings.TrimSpace(string(output))
	if err == nil && strings.EqualFold(state, "active") {
		return AgentHealthCheck{Name: "service_status", Status: HealthStatusOK, Detail: "active: " + filepath.Base(def.InstallPath)}
	}
	return AgentHealthCheck{Name: "service_status", Status: HealthStatusWarn, Detail: "not active: " + serviceStatusDetail(output, err)}
}

func parseDarwinServiceStatus(def ServiceDefinition, output []byte, err error) AgentHealthCheck {
	if err != nil {
		return AgentHealthCheck{Name: "service_status", Status: HealthStatusWarn, Detail: "not loaded or inaccessible: " + serviceStatusDetail(output, err)}
	}
	lower := strings.ToLower(string(output))
	if strings.Contains(lower, "state = running") || strings.Contains(lower, "\npid =") {
		return AgentHealthCheck{Name: "service_status", Status: HealthStatusOK, Detail: "running: " + def.Label}
	}
	return AgentHealthCheck{Name: "service_status", Status: HealthStatusWarn, Detail: "loaded but not running: " + def.Label}
}

func parseWindowsServiceStatus(def ServiceDefinition, output []byte, err error) AgentHealthCheck {
	upper := strings.ToUpper(string(output))
	if err == nil && strings.Contains(upper, "RUNNING") {
		return AgentHealthCheck{Name: "service_status", Status: HealthStatusOK, Detail: "running: " + windowsServiceName(def.Name)}
	}
	if strings.Contains(upper, "STOPPED") {
		return AgentHealthCheck{Name: "service_status", Status: HealthStatusWarn, Detail: "stopped: " + windowsServiceName(def.Name)}
	}
	return AgentHealthCheck{Name: "service_status", Status: HealthStatusWarn, Detail: "not running: " + serviceStatusDetail(output, err)}
}

func serviceStatusDetail(output []byte, err error) string {
	text := strings.TrimSpace(string(output))
	if text != "" {
		return firstServiceStatusLine(text)
	}
	if err != nil {
		return err.Error()
	}
	return "unknown"
}

func firstServiceStatusLine(text string) string {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return "unknown"
}

func runLinuxServiceAction(ctx context.Context, opts ServiceOptions, def ServiceDefinition, action string) error {
	unit := filepath.Base(def.InstallPath)
	switch action {
	case "enable":
		if err := runLinuxSystemctl(ctx, opts, def, "daemon-reload"); err != nil {
			return err
		}
		return runLinuxSystemctl(ctx, opts, def, "enable", unit)
	case "disable":
		return runLinuxSystemctl(ctx, opts, def, "disable", unit)
	case "start", "stop", "status":
		return runLinuxSystemctl(ctx, opts, def, action, unit)
	default:
		return fmt.Errorf("unsupported linux service action: %s", action)
	}
}

func runLinuxSystemctl(ctx context.Context, opts ServiceOptions, def ServiceDefinition, args ...string) error {
	fullArgs := append([]string{}, args...)
	if def.Scope == "user" {
		fullArgs = append([]string{"--user"}, fullArgs...)
	}
	return runPlatformCommand(ctx, opts, "systemctl", fullArgs...)
}

func runDarwinServiceAction(ctx context.Context, opts ServiceOptions, def ServiceDefinition, action string) error {
	switch action {
	case "enable":
		return runPlatformCommand(ctx, opts, "launchctl", "load", "-w", def.InstallPath)
	case "disable":
		return runPlatformCommand(ctx, opts, "launchctl", "unload", "-w", def.InstallPath)
	case "start":
		return runPlatformCommand(ctx, opts, "launchctl", "start", def.Label)
	case "stop":
		return runPlatformCommand(ctx, opts, "launchctl", "stop", def.Label)
	case "status":
		return runPlatformCommand(ctx, opts, "launchctl", "print", darwinLaunchdDomain(def)+"/"+def.Label)
	default:
		return fmt.Errorf("unsupported launchd service action: %s", action)
	}
}

func runWindowsServiceAction(ctx context.Context, opts ServiceOptions, def ServiceDefinition, action string) error {
	service := windowsServiceName(def.Name)
	switch action {
	case "enable":
		return nil
	case "disable":
		return runPlatformCommand(ctx, opts, "sc.exe", "config", service, "start=", "disabled")
	case "start":
		return runPlatformCommand(ctx, opts, "sc.exe", "start", service)
	case "stop":
		return runPlatformCommand(ctx, opts, "sc.exe", "stop", service)
	case "status":
		return runPlatformCommand(ctx, opts, "sc.exe", "query", service)
	default:
		return fmt.Errorf("unsupported windows service action: %s", action)
	}
}

func runPlatformCommand(ctx context.Context, opts ServiceOptions, name string, args ...string) error {
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	runner := opts.CommandExec
	if runner == nil {
		runner = defaultCommandRunner
	}
	output, err := runner(cmdCtx, name, args...)
	if opts.Out != nil && len(output) > 0 {
		_, _ = opts.Out.Write(output)
		if output[len(output)-1] != '\n' {
			fmt.Fprintln(opts.Out)
		}
	}
	if err != nil {
		return fmt.Errorf("%s %s failed: %w", name, strings.Join(args, " "), err)
	}
	return nil
}

func defaultCommandRunner(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	return cmd.CombinedOutput()
}

func printServiceDryRun(w io.Writer, def ServiceDefinition, action string) {
	fmt.Fprintf(w, "service_action: %s\n", action)
	fmt.Fprintf(w, "platform: %s\n", def.Platform)
	fmt.Fprintf(w, "scope: %s\n", def.Scope)
	fmt.Fprintf(w, "name: %s\n", def.Name)
	fmt.Fprintf(w, "label: %s\n", def.Label)
	fmt.Fprintf(w, "binary: %s\n", def.BinaryPath)
	fmt.Fprintf(w, "config: %s\n", def.ConfigPath)
	fmt.Fprintf(w, "install_path: %s\n", def.InstallPath)
	if strings.TrimSpace(def.Content) != "" {
		fmt.Fprintln(w, "content:")
		fmt.Fprintln(w, def.Content)
	}
}

func resolveServiceConfigPath(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		resolved, err := ConfigPath()
		if err != nil {
			return "", err
		}
		path = resolved
	}
	path = expandPath(path)
	if !filepath.IsAbs(path) {
		absolute, err := filepath.Abs(path)
		if err == nil {
			path = absolute
		}
	}
	if _, loaded, err := LoadConfig(path); err != nil {
		return "", err
	} else if !loaded {
		return "", fmt.Errorf("config file not found: %s; run enroll first or pass --config", path)
	}
	return path, nil
}

func resolveServiceBinaryPath(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		resolved, err := os.Executable()
		if err != nil {
			return "", err
		}
		path = resolved
	}
	path = expandPath(path)
	if !filepath.IsAbs(path) {
		absolute, err := filepath.Abs(path)
		if err == nil {
			path = absolute
		}
	}
	return path, nil
}

func serviceScope(scope string, platform string) string {
	scope = strings.ToLower(strings.TrimSpace(scope))
	if scope == "user" || scope == "system" {
		return scope
	}
	switch platform {
	case "linux":
		if os.Geteuid() == 0 {
			return "system"
		}
		return "user"
	case "darwin":
		return "user"
	default:
		return "system"
	}
}

func serviceName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		name = DefaultServiceName
	}
	var builder strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r == '-' || r == '_' || r == '.':
			builder.WriteRune(r)
		default:
			builder.WriteRune('-')
		}
	}
	result := strings.Trim(builder.String(), "-_.")
	if result == "" {
		return DefaultServiceName
	}
	return result
}

func serviceLabel(name string) string {
	if name == DefaultServiceName {
		return DefaultLaunchdLabel
	}
	return "ltd.mbu.dataproxy." + strings.ReplaceAll(serviceName(name), "_", "-")
}

func linuxServicePath(name string, scope string) (string, error) {
	unitName := serviceName(name) + ".service"
	if scope == "system" {
		return filepath.Join("/etc/systemd/system", unitName), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "systemd", "user", unitName), nil
}

func linuxServiceUnit(def ServiceDefinition) string {
	wantedBy := "multi-user.target"
	if def.Scope == "user" {
		wantedBy = "default.target"
	}
	return fmt.Sprintf(`[Unit]
Description=Data Proxy Agent
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=%s %s --config %s
Restart=always
RestartSec=5

[Install]
WantedBy=%s
`, quoteSystemdArg(def.BinaryPath), quoteSystemdArg(DefaultServiceExecStart), quoteSystemdArg(def.ConfigPath), wantedBy)
}

func darwinServicePath(label string, scope string) (string, error) {
	fileName := label + ".plist"
	if scope == "system" {
		return filepath.Join("/Library/LaunchDaemons", fileName), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents", fileName), nil
}

func darwinLaunchdPlist(def ServiceDefinition, stdoutPath string, stderrPath string) string {
	if strings.TrimSpace(stdoutPath) == "" {
		stdoutPath = filepath.Join(os.TempDir(), def.Name+".out.log")
	}
	if strings.TrimSpace(stderrPath) == "" {
		stderrPath = filepath.Join(os.TempDir(), def.Name+".err.log")
	}
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>%s</string>
  <key>ProgramArguments</key>
  <array>
    <string>%s</string>
    <string>%s</string>
    <string>--config</string>
    <string>%s</string>
  </array>
  <key>RunAtLoad</key>
  <true/>
  <key>KeepAlive</key>
  <true/>
  <key>StandardOutPath</key>
  <string>%s</string>
  <key>StandardErrorPath</key>
  <string>%s</string>
</dict>
</plist>
`, xmlEscape(def.Label), xmlEscape(def.BinaryPath), xmlEscape(DefaultServiceExecStart), xmlEscape(def.ConfigPath), xmlEscape(expandPath(stdoutPath)), xmlEscape(expandPath(stderrPath)))
}

func darwinLaunchdDomain(def ServiceDefinition) string {
	if def.Scope == "system" {
		return "system"
	}
	return "gui/" + strconv.Itoa(os.Getuid())
}

func windowsServiceCommandLine(def ServiceDefinition) string {
	return fmt.Sprintf("%s %s --config %s", quoteWindowsArg(def.BinaryPath), quoteWindowsArg(DefaultServiceExecStart), quoteWindowsArg(def.ConfigPath))
}

func windowsCreateArgs(def ServiceDefinition) []string {
	return []string{
		"binPath=", windowsServiceCommandLine(def),
		"start=", "auto",
		"DisplayName=", "Data Proxy Agent",
	}
}

func windowsServiceName(name string) string {
	if name == DefaultServiceName {
		return "DataProxyAgent"
	}
	var builder strings.Builder
	upperNext := true
	for _, r := range serviceName(name) {
		if r == '-' || r == '_' || r == '.' {
			upperNext = true
			continue
		}
		if upperNext && r >= 'a' && r <= 'z' {
			builder.WriteRune(r - 32)
		} else {
			builder.WriteRune(r)
		}
		upperNext = false
	}
	result := builder.String()
	if result == "" {
		return "DataProxyAgent"
	}
	return result
}

func quoteSystemdArg(value string) string {
	return strconv.Quote(value)
}

func quoteWindowsArg(value string) string {
	value = strings.ReplaceAll(value, `"`, `\"`)
	return `"` + value + `"`
}

func xmlEscape(value string) string {
	return html.EscapeString(value)
}
