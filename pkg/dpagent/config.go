package dpagent

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	DefaultAgentCommandName = "dpa"
	LegacyAgentCommandName  = "data-proxy-agent"
	DefaultBridgePath       = "/bridge/ws"
	DefaultAgentVersion     = "0.1.0-dev"
	DefaultPingIntervalMS   = 30000
	DefaultHealthIntervalMS = 60000
	DefaultReconnectBaseMS  = 500
	DefaultReconnectMaxMS   = 15000
	DefaultMaxConcurrency   = 16
	DefaultHTTPTimeoutMS    = 30000
	DefaultConfigFileMode   = 0o600
	DefaultConfigFolderMode = 0o700
)

type Config struct {
	Server     ServerConfig  `json:"server" yaml:"server"`
	Agent      AgentConfig   `json:"agent" yaml:"agent"`
	Policy     PolicyConfig  `json:"policy" yaml:"policy"`
	MCPServers []MCPServer   `json:"mcp_servers,omitempty" yaml:"mcp_servers,omitempty"`
	HTTPRoutes []HTTPRoute   `json:"http_routes,omitempty" yaml:"http_routes,omitempty"`
	TCPRoutes  []TCPRoute    `json:"tcp_routes,omitempty" yaml:"tcp_routes,omitempty"`
	Logging    LoggingConfig `json:"logging" yaml:"logging"`
	Runtime    RuntimeConfig `json:"runtime" yaml:"runtime"`
}

type ServerConfig struct {
	BaseURL     string `json:"base_url,omitempty" yaml:"base_url,omitempty"`
	BridgeWSURL string `json:"bridge_ws_url,omitempty" yaml:"bridge_ws_url,omitempty"`
}

type AgentConfig struct {
	ClientID       string   `json:"client_id,omitempty" yaml:"client_id,omitempty"`
	Name           string   `json:"name,omitempty" yaml:"name,omitempty"`
	Version        string   `json:"version,omitempty" yaml:"version,omitempty"`
	VersionChannel string   `json:"version_channel,omitempty" yaml:"version_channel,omitempty"`
	Token          string   `json:"token,omitempty" yaml:"token,omitempty"`
	TokenEnv       string   `json:"token_env,omitempty" yaml:"token_env,omitempty"`
	TokenFile      string   `json:"token_file,omitempty" yaml:"token_file,omitempty"`
	TokenRef       string   `json:"token_ref,omitempty" yaml:"token_ref,omitempty"`
	Workspace      string   `json:"workspace,omitempty" yaml:"workspace,omitempty"`
	Capabilities   []string `json:"capabilities,omitempty" yaml:"capabilities,omitempty"`
}

type PolicyConfig struct {
	DefaultPermission    string     `json:"default_permission,omitempty" yaml:"default_permission,omitempty"`
	AllowWrite           bool       `json:"allow_write" yaml:"allow_write"`
	AllowNonLoopbackHTTP bool       `json:"allow_non_loopback_http" yaml:"allow_non_loopback_http"`
	AllowNonLoopbackMCP  bool       `json:"allow_non_loopback_mcp" yaml:"allow_non_loopback_mcp"`
	AllowNonLoopbackTCP  bool       `json:"allow_non_loopback_tcp" yaml:"allow_non_loopback_tcp"`
	AllowedWorkspaces    []string   `json:"allowed_workspaces,omitempty" yaml:"allowed_workspaces,omitempty"`
	DeniedPaths          []string   `json:"denied_paths,omitempty" yaml:"denied_paths,omitempty"`
	Exec                 ExecPolicy `json:"exec" yaml:"exec"`
}

type ExecPolicy struct {
	Enabled        bool     `json:"enabled" yaml:"enabled"`
	AllowArbitrary bool     `json:"allow_arbitrary" yaml:"allow_arbitrary"`
	SafeCommands   []string `json:"safe_commands,omitempty" yaml:"safe_commands,omitempty"`
}

type MCPServer struct {
	Name       string `json:"name" yaml:"name"`
	Transport  string `json:"transport" yaml:"transport"`
	Endpoint   string `json:"endpoint,omitempty" yaml:"endpoint,omitempty"`
	Command    string `json:"command,omitempty" yaml:"command,omitempty"`
	Permission string `json:"permission,omitempty" yaml:"permission,omitempty"`
}

type HTTPRoute struct {
	Name             string `json:"name" yaml:"name"`
	Target           string `json:"target" yaml:"target"`
	AllowWebSocket   bool   `json:"allow_websocket" yaml:"allow_websocket"`
	AllowSSE         bool   `json:"allow_sse" yaml:"allow_sse"`
	MaxRequestBytes  int64  `json:"max_request_bytes,omitempty" yaml:"max_request_bytes,omitempty"`
	MaxResponseBytes int64  `json:"max_response_bytes,omitempty" yaml:"max_response_bytes,omitempty"`
}

type TCPRoute struct {
	Name        string `json:"name" yaml:"name"`
	TargetHost  string `json:"target_host" yaml:"target_host"`
	TargetPort  int    `json:"target_port" yaml:"target_port"`
	AllowPublic bool   `json:"allow_public" yaml:"allow_public"`
}

type LoggingConfig struct {
	Level           string `json:"level,omitempty" yaml:"level,omitempty"`
	LocalAuditJSONL string `json:"local_audit_jsonl,omitempty" yaml:"local_audit_jsonl,omitempty"`
}

type RuntimeConfig struct {
	PingIntervalMS   int   `json:"ping_interval_ms,omitempty" yaml:"ping_interval_ms,omitempty"`
	HealthIntervalMS int   `json:"health_interval_ms,omitempty" yaml:"health_interval_ms,omitempty"`
	Reconnect        bool  `json:"reconnect" yaml:"reconnect"`
	ReconnectBaseMS  int   `json:"reconnect_base_ms,omitempty" yaml:"reconnect_base_ms,omitempty"`
	ReconnectMaxMS   int   `json:"reconnect_max_ms,omitempty" yaml:"reconnect_max_ms,omitempty"`
	MaxConcurrency   int   `json:"max_concurrency,omitempty" yaml:"max_concurrency,omitempty"`
	HTTPTimeoutMS    int   `json:"http_timeout_ms,omitempty" yaml:"http_timeout_ms,omitempty"`
	MaxResults       int   `json:"max_results,omitempty" yaml:"max_results,omitempty"`
	TreeDepth        int   `json:"tree_depth,omitempty" yaml:"tree_depth,omitempty"`
	WalkDepth        int   `json:"walk_depth,omitempty" yaml:"walk_depth,omitempty"`
	MaxResultBytes   int64 `json:"max_result_bytes,omitempty" yaml:"max_result_bytes,omitempty"`
	MaxScanFileBytes int64 `json:"max_scan_file_bytes,omitempty" yaml:"max_scan_file_bytes,omitempty"`
	MaxWriteBytes    int64 `json:"max_write_bytes,omitempty" yaml:"max_write_bytes,omitempty"`
}

type RuntimeOptions struct {
	ConfigPath  string
	BaseURL     string
	BridgeWSURL string
	Token       string
	ClientID    string
	Name        string
	Version     string
	Workspace   string
	Once        bool
	NoReconnect bool
}

type ValidationResult struct {
	Errors   []string `json:"errors"`
	Warnings []string `json:"warnings"`
}

func (r ValidationResult) OK() bool {
	return len(r.Errors) == 0
}

func (r ValidationResult) Error() error {
	if r.OK() {
		return nil
	}
	return errors.New(strings.Join(r.Errors, "; "))
}

func DefaultConfig() Config {
	hostname, _ := os.Hostname()
	workspace, _ := os.Getwd()
	clientID := sanitizeClientID(hostname)
	if clientID == "" {
		clientID = "local-agent"
	}
	name := hostname
	if name == "" {
		name = "Data Proxy Agent"
	}
	return Config{
		Server: ServerConfig{},
		Agent: AgentConfig{
			ClientID:       clientID,
			Name:           name,
			Version:        DefaultAgentVersion,
			VersionChannel: "stable",
			TokenEnv:       "DATA_PROXY_API_KEY",
			Workspace:      workspace,
			Capabilities:   []string{},
		},
		Policy: PolicyConfig{
			DefaultPermission: "read_only",
		},
		Logging: LoggingConfig{
			Level: "info",
		},
		Runtime: RuntimeConfig{
			PingIntervalMS:   DefaultPingIntervalMS,
			HealthIntervalMS: DefaultHealthIntervalMS,
			Reconnect:        true,
			ReconnectBaseMS:  DefaultReconnectBaseMS,
			ReconnectMaxMS:   DefaultReconnectMaxMS,
			MaxConcurrency:   DefaultMaxConcurrency,
			HTTPTimeoutMS:    DefaultHTTPTimeoutMS,
			MaxResults:       DefaultRemoteMaxResults,
			TreeDepth:        DefaultRemoteTreeDepth,
			WalkDepth:        DefaultRemoteWalkDepth,
			MaxResultBytes:   DefaultRemoteMaxResultBytes,
			MaxScanFileBytes: DefaultRemoteMaxScanFileBytes,
			MaxWriteBytes:    DefaultRemoteMaxWriteBytes,
		},
	}
}

func ConfigPath() (string, error) {
	if explicit := strings.TrimSpace(os.Getenv("DATA_PROXY_AGENT_CONFIG")); explicit != "" {
		return expandPath(explicit), nil
	}
	switch runtime.GOOS {
	case "windows":
		if base := strings.TrimSpace(os.Getenv("APPDATA")); base != "" {
			return filepath.Join(base, "DataProxyAgent", "config.yaml"), nil
		}
		if base := strings.TrimSpace(os.Getenv("ProgramData")); base != "" {
			return filepath.Join(base, "DataProxyAgent", "config.yaml"), nil
		}
	case "darwin":
		home, err := os.UserHomeDir()
		if err == nil && home != "" {
			return filepath.Join(home, "Library", "Application Support", "DataProxyAgent", "config.yaml"), nil
		}
	default:
		if base := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); base != "" {
			return filepath.Join(base, "data-proxy-agent", "config.yaml"), nil
		}
		home, err := os.UserHomeDir()
		if err == nil && home != "" {
			return filepath.Join(home, ".config", "data-proxy-agent", "config.yaml"), nil
		}
	}
	return "", errors.New("unable to determine config path")
}

func LoadConfig(path string) (Config, bool, error) {
	cfg := DefaultConfig()
	if strings.TrimSpace(path) == "" {
		resolved, err := ConfigPath()
		if err != nil {
			return cfg, false, err
		}
		path = resolved
	}
	bytes, err := os.ReadFile(expandPath(path))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			applyEnv(&cfg)
			return cfg, false, nil
		}
		return cfg, false, err
	}
	if err := yaml.Unmarshal(bytes, &cfg); err != nil {
		return cfg, true, err
	}
	fillConfigDefaults(&cfg)
	applyEnv(&cfg)
	return cfg, true, nil
}

func SaveConfig(path string, cfg Config) error {
	if strings.TrimSpace(path) == "" {
		resolved, err := ConfigPath()
		if err != nil {
			return err
		}
		path = resolved
	}
	path = expandPath(path)
	if err := os.MkdirAll(filepath.Dir(path), DefaultConfigFolderMode); err != nil {
		return err
	}
	bytes, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, bytes, DefaultConfigFileMode)
}

func ApplyRuntimeOptions(cfg Config, opts RuntimeOptions) Config {
	if opts.BaseURL != "" {
		cfg.Server.BaseURL = opts.BaseURL
	}
	if opts.BridgeWSURL != "" {
		cfg.Server.BridgeWSURL = opts.BridgeWSURL
	}
	if opts.Token != "" {
		cfg.Agent.Token = opts.Token
	}
	if opts.ClientID != "" {
		cfg.Agent.ClientID = opts.ClientID
	}
	if opts.Name != "" {
		cfg.Agent.Name = opts.Name
	}
	if opts.Version != "" {
		cfg.Agent.Version = opts.Version
	}
	if opts.Workspace != "" {
		cfg.Agent.Workspace = opts.Workspace
	}
	if opts.NoReconnect || opts.Once {
		cfg.Runtime.Reconnect = false
	}
	fillConfigDefaults(&cfg)
	return cfg
}

func ValidateConfig(cfg Config, requireToken bool) ValidationResult {
	var result ValidationResult
	if strings.TrimSpace(cfg.Server.BridgeWSURL) == "" {
		if strings.TrimSpace(cfg.Server.BaseURL) == "" {
			result.Errors = append(result.Errors, "server.bridge_ws_url or server.base_url is required")
		} else if _, err := BridgeURLFromBaseURL(cfg.Server.BaseURL); err != nil {
			result.Errors = append(result.Errors, "server.base_url is invalid: "+err.Error())
		}
	} else if err := validateBridgeWSURL(cfg.Server.BridgeWSURL); err != nil {
		result.Errors = append(result.Errors, "server.bridge_ws_url is invalid: "+err.Error())
	}
	if strings.TrimSpace(cfg.Agent.ClientID) == "" {
		result.Errors = append(result.Errors, "agent.client_id is required")
	}
	if len(cfg.Agent.ClientID) > 128 {
		result.Errors = append(result.Errors, "agent.client_id must be 128 characters or shorter")
	}
	if strings.TrimSpace(cfg.Agent.Version) == "" {
		result.Warnings = append(result.Warnings, "agent.version is empty; using "+DefaultAgentVersion)
	}
	if strings.TrimSpace(cfg.Agent.Workspace) == "" {
		result.Warnings = append(result.Warnings, "agent.workspace is empty")
	}
	if requireToken && strings.TrimSpace(ResolveToken(cfg)) == "" {
		result.Errors = append(result.Errors, "agent token is required; set agent.token, agent.token_file, agent.token_env, DATA_PROXY_API_KEY or --token")
	}
	for _, route := range cfg.HTTPRoutes {
		if strings.TrimSpace(route.Name) == "" {
			result.Errors = append(result.Errors, "http_routes[].name is required")
		}
		if strings.TrimSpace(route.Target) == "" {
			result.Errors = append(result.Errors, fmt.Sprintf("http route %q target is required", route.Name))
		} else if u, err := url.Parse(route.Target); err != nil || u.Scheme == "" || u.Host == "" {
			result.Errors = append(result.Errors, fmt.Sprintf("http route %q target is invalid", route.Name))
		}
	}
	for _, route := range cfg.TCPRoutes {
		if strings.TrimSpace(route.Name) == "" {
			result.Errors = append(result.Errors, "tcp_routes[].name is required")
		}
		if strings.TrimSpace(route.TargetHost) == "" {
			result.Errors = append(result.Errors, fmt.Sprintf("tcp route %q target_host is required", route.Name))
		}
		if route.TargetPort <= 0 || route.TargetPort > 65535 {
			result.Errors = append(result.Errors, fmt.Sprintf("tcp route %q target_port is invalid", route.Name))
		}
	}
	for _, server := range cfg.MCPServers {
		if strings.TrimSpace(server.Name) == "" {
			result.Errors = append(result.Errors, "mcp_servers[].name is required")
		}
		switch strings.TrimSpace(server.Transport) {
		case "streamable_http", "streamable-http", "http", "sse":
			if strings.TrimSpace(server.Endpoint) == "" {
				result.Errors = append(result.Errors, fmt.Sprintf("mcp server %q endpoint is required", server.Name))
			}
		case "stdio":
			if strings.TrimSpace(server.Command) == "" {
				result.Errors = append(result.Errors, fmt.Sprintf("mcp server %q command is required", server.Name))
			}
		case "":
			result.Errors = append(result.Errors, fmt.Sprintf("mcp server %q transport is required", server.Name))
		default:
			result.Errors = append(result.Errors, fmt.Sprintf("mcp server %q transport %q is not supported", server.Name, server.Transport))
		}
	}
	return result
}

func EffectiveBridgeWSURL(cfg Config) (string, error) {
	if strings.TrimSpace(cfg.Server.BridgeWSURL) != "" {
		return strings.TrimSpace(cfg.Server.BridgeWSURL), validateBridgeWSURL(cfg.Server.BridgeWSURL)
	}
	return BridgeURLFromBaseURL(cfg.Server.BaseURL)
}

func BridgeURLFromBaseURL(base string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(base))
	if err != nil {
		return "", err
	}
	if u.Scheme == "" || u.Host == "" {
		return "", errors.New("missing scheme or host")
	}
	switch u.Scheme {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	case "ws", "wss":
	default:
		return "", fmt.Errorf("unsupported scheme %q", u.Scheme)
	}
	u.Path = strings.TrimRight(u.Path, "/") + DefaultBridgePath
	u.RawQuery = ""
	u.Fragment = ""
	return u.String(), nil
}

func ResolveToken(cfg Config) string {
	token, _ := ResolveTokenWithSource(cfg)
	return token
}

func ResolveTokenWithSource(cfg Config) (string, string) {
	if token := strings.TrimSpace(cfg.Agent.Token); token != "" {
		return token, "agent.token"
	}
	envName := strings.TrimSpace(cfg.Agent.TokenEnv)
	if envName == "" {
		envName = "DATA_PROXY_API_KEY"
	}
	if token := strings.TrimSpace(os.Getenv(envName)); token != "" {
		return token, "env:" + envName
	}
	if token := strings.TrimSpace(os.Getenv("DATA_PROXY_AGENT_TOKEN")); token != "" {
		return token, "env:DATA_PROXY_AGENT_TOKEN"
	}
	if token := strings.TrimSpace(os.Getenv("BRIDGE_DAEMON_TOKEN")); token != "" {
		return token, "env:BRIDGE_DAEMON_TOKEN"
	}
	if ref := strings.TrimSpace(cfg.Agent.TokenRef); ref != "" {
		if token, err := ReadSecretRef(ref); err == nil && strings.TrimSpace(token) != "" {
			return strings.TrimSpace(token), "agent.token_ref"
		}
	}
	if path := strings.TrimSpace(cfg.Agent.TokenFile); path != "" {
		bytes, err := os.ReadFile(expandPath(path))
		if err == nil {
			return strings.TrimSpace(string(bytes)), "agent.token_file"
		}
	}
	return "", ""
}

func RedactedConfig(cfg Config) Config {
	redacted := cfg
	if redacted.Agent.Token != "" {
		redacted.Agent.Token = redactSecret(redacted.Agent.Token)
	}
	return redacted
}

func EffectiveCapabilities(cfg Config) []string {
	seen := map[string]bool{}
	var result []string
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			return
		}
		seen[value] = true
		result = append(result, value)
	}
	for _, capability := range cfg.Agent.Capabilities {
		if !capabilityAllowedByLocalPolicy(cfg, capability) {
			continue
		}
		add(capability)
	}
	if strings.TrimSpace(cfg.Agent.Workspace) != "" {
		add(BridgeToolRemoteRead)
		add(BridgeToolRemoteTree)
		add(BridgeToolRemoteGlob)
		add(BridgeToolRemoteGrep)
		add(BridgeToolRemoteEnvInfo)
		add(BridgeToolRemoteProjectInfo)
		add(BridgeToolRemoteGetRelatedFiles)
		add(BridgeToolRemoteGitStatus)
		add(BridgeToolRemoteGitDiff)
		add(BridgeToolRemoteGitLog)
		if cfg.Policy.AllowWrite {
			add(BridgeToolRemoteWrite)
			add(BridgeToolRemoteEdit)
		}
		if cfg.Policy.Exec.Enabled && len(normalizedSafeCommands(cfg.Policy.Exec.SafeCommands)) > 0 {
			add(BridgeToolRemoteRunTests)
		}
		if cfg.Policy.Exec.Enabled && cfg.Policy.Exec.AllowArbitrary {
			add(BridgeToolRemoteExec)
			add(BridgeToolRemoteShellOpen)
			add(BridgeToolRemoteShellEval)
			add(BridgeToolRemoteShellResize)
			if cfg.Policy.AllowWrite {
				add(BridgeToolRemoteInstallPackage)
			}
		}
	}
	if len(cfg.HTTPRoutes) > 0 {
		add(BridgeCapabilityHTTPTunnel)
	}
	if len(cfg.TCPRoutes) > 0 {
		add(BridgeCapabilityTCPTunnel)
	}
	if len(cfg.MCPServers) > 0 {
		add(BridgeCapabilityMCPProxy)
	}
	return result
}

func capabilityAllowedByLocalPolicy(cfg Config, capability string) bool {
	switch strings.TrimSpace(capability) {
	case BridgeToolRemoteWrite, BridgeToolRemoteEdit:
		return cfg.Policy.AllowWrite
	case BridgeToolRemoteRunTests:
		return cfg.Policy.Exec.Enabled && len(normalizedSafeCommands(cfg.Policy.Exec.SafeCommands)) > 0
	case BridgeToolRemoteExec:
		return cfg.Policy.Exec.Enabled && cfg.Policy.Exec.AllowArbitrary
	case BridgeToolRemoteShellOpen, BridgeToolRemoteShellEval, BridgeToolRemoteShellResize:
		return cfg.Policy.Exec.Enabled && cfg.Policy.Exec.AllowArbitrary
	case BridgeToolRemoteInstallPackage:
		return cfg.Policy.AllowWrite && cfg.Policy.Exec.Enabled && cfg.Policy.Exec.AllowArbitrary
	default:
		return true
	}
}

func fillConfigDefaults(cfg *Config) {
	if cfg.Agent.ClientID == "" {
		cfg.Agent.ClientID = DefaultConfig().Agent.ClientID
	}
	if cfg.Agent.Name == "" {
		cfg.Agent.Name = DefaultConfig().Agent.Name
	}
	if cfg.Agent.Version == "" {
		cfg.Agent.Version = DefaultAgentVersion
	}
	if cfg.Agent.TokenEnv == "" {
		cfg.Agent.TokenEnv = "DATA_PROXY_API_KEY"
	}
	if cfg.Agent.Workspace == "" {
		workspace, _ := os.Getwd()
		cfg.Agent.Workspace = workspace
	}
	if cfg.Policy.DefaultPermission == "" {
		cfg.Policy.DefaultPermission = "read_only"
	}
	if cfg.Logging.Level == "" {
		cfg.Logging.Level = "info"
	}
	if cfg.Runtime.PingIntervalMS <= 0 {
		cfg.Runtime.PingIntervalMS = DefaultPingIntervalMS
	}
	if cfg.Runtime.HealthIntervalMS == 0 {
		cfg.Runtime.HealthIntervalMS = DefaultHealthIntervalMS
	}
	if cfg.Runtime.ReconnectBaseMS <= 0 {
		cfg.Runtime.ReconnectBaseMS = DefaultReconnectBaseMS
	}
	if cfg.Runtime.ReconnectMaxMS <= 0 {
		cfg.Runtime.ReconnectMaxMS = DefaultReconnectMaxMS
	}
	if cfg.Runtime.MaxConcurrency <= 0 {
		cfg.Runtime.MaxConcurrency = DefaultMaxConcurrency
	}
	if cfg.Runtime.HTTPTimeoutMS <= 0 {
		cfg.Runtime.HTTPTimeoutMS = DefaultHTTPTimeoutMS
	}
	if cfg.Runtime.MaxResults <= 0 {
		cfg.Runtime.MaxResults = DefaultRemoteMaxResults
	}
	if cfg.Runtime.TreeDepth <= 0 {
		cfg.Runtime.TreeDepth = DefaultRemoteTreeDepth
	}
	if cfg.Runtime.WalkDepth <= 0 {
		cfg.Runtime.WalkDepth = DefaultRemoteWalkDepth
	}
	if cfg.Runtime.MaxResultBytes <= 0 {
		cfg.Runtime.MaxResultBytes = DefaultRemoteMaxResultBytes
	}
	if cfg.Runtime.MaxScanFileBytes <= 0 {
		cfg.Runtime.MaxScanFileBytes = DefaultRemoteMaxScanFileBytes
	}
	if cfg.Runtime.MaxWriteBytes <= 0 {
		cfg.Runtime.MaxWriteBytes = DefaultRemoteMaxWriteBytes
	}
}

func applyEnv(cfg *Config) {
	if base := strings.TrimSpace(os.Getenv("DATA_PROXY_BASE_URL")); base != "" {
		cfg.Server.BaseURL = base
	}
	if ws := strings.TrimSpace(os.Getenv("DATA_PROXY_BRIDGE_WS_URL")); ws != "" {
		cfg.Server.BridgeWSURL = ws
	}
	if ws := strings.TrimSpace(os.Getenv("BRIDGE_DAEMON_SERVER")); ws != "" {
		cfg.Server.BridgeWSURL = ws
	}
	if token := strings.TrimSpace(os.Getenv("DATA_PROXY_API_KEY")); token != "" {
		cfg.Agent.Token = token
	}
	if token := strings.TrimSpace(os.Getenv("DATA_PROXY_AGENT_TOKEN")); token != "" {
		cfg.Agent.Token = token
	}
	if clientID := strings.TrimSpace(os.Getenv("DATA_PROXY_BRIDGE_CLIENT_ID")); clientID != "" {
		cfg.Agent.ClientID = clientID
	}
	if clientID := strings.TrimSpace(os.Getenv("BRIDGE_DAEMON_CLIENT_ID")); clientID != "" {
		cfg.Agent.ClientID = clientID
	}
	if workspace := strings.TrimSpace(os.Getenv("BRIDGE_DAEMON_WORKSPACE")); workspace != "" {
		cfg.Agent.Workspace = workspace
	}
}

func validateBridgeWSURL(value string) error {
	u, err := url.Parse(strings.TrimSpace(value))
	if err != nil {
		return err
	}
	if u.Scheme != "ws" && u.Scheme != "wss" {
		return errors.New("scheme must be ws or wss")
	}
	if u.Host == "" {
		return errors.New("host is required")
	}
	return nil
}

func sanitizeClientID(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var builder strings.Builder
	for _, r := range value {
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
	return strings.Trim(builder.String(), "-_.")
}

func expandPath(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || value == "~" {
		home, err := os.UserHomeDir()
		if err == nil && home != "" {
			return home
		}
		return value
	}
	if strings.HasPrefix(value, "~/") || strings.HasPrefix(value, "~"+string(filepath.Separator)) {
		home, err := os.UserHomeDir()
		if err == nil && home != "" {
			return filepath.Join(home, value[2:])
		}
	}
	return os.ExpandEnv(value)
}

func redactSecret(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if len(value) <= 8 {
		return "***"
	}
	return value[:4] + "..." + value[len(value)-4:]
}
