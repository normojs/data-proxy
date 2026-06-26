package dpagent

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type CLI struct {
	Program string
	Version string
	Out     io.Writer
	Err     io.Writer
}

func RunCLI(args []string, out io.Writer, errOut io.Writer, version string) int {
	return RunCLIWithProgram(DefaultAgentCommandName, args, out, errOut, version)
}

func RunCLIWithProgram(program string, args []string, out io.Writer, errOut io.Writer, version string) int {
	cli := CLI{Program: normalizeProgramName(program), Version: version, Out: out, Err: errOut}
	return cli.Run(args)
}

func normalizeProgramName(program string) string {
	program = strings.TrimSpace(program)
	if program == "" {
		return DefaultAgentCommandName
	}
	if strings.HasSuffix(strings.ToLower(program), ".exe") {
		program = program[:len(program)-len(".exe")]
	}
	return program
}

func (c CLI) programName() string {
	return normalizeProgramName(c.Program)
}

func (c CLI) Run(args []string) int {
	if c.Out == nil {
		c.Out = os.Stdout
	}
	if c.Err == nil {
		c.Err = os.Stderr
	}
	if c.Version == "" {
		c.Version = DefaultAgentVersion
	}
	if c.Program == "" {
		c.Program = DefaultAgentCommandName
	}
	if len(args) == 0 {
		c.printHelp()
		return 0
	}
	switch args[0] {
	case "help", "-h", "--help":
		c.printHelp()
		return 0
	case "version", "--version":
		fmt.Fprintf(c.Out, "%s %s\n", c.programName(), c.Version)
		return 0
	case "enroll":
		return c.runEnroll(args[1:])
	case "config":
		return c.runConfig(args[1:])
	case "mcp":
		return c.runMCP(args[1:])
	case "tunnel":
		return c.runTunnel(args[1:])
	case "status":
		return c.runStatus(args[1:])
	case "doctor":
		return c.runDoctor(args[1:])
	case "report":
		return c.runReport(args[1:])
	case "logs":
		return c.runLogs(args[1:])
	case "service":
		return c.runService(args[1:])
	case "update":
		return c.runUpdate(args[1:])
	case "self-test":
		return c.runSelfTest(args[1:])
	case "run":
		return c.runBridge(args[1:])
	default:
		fmt.Fprintf(c.Err, "unknown command: %s\n\n", args[0])
		c.printHelp()
		return 2
	}
}

func (c CLI) printHelp() {
	program := c.programName()
	fmt.Fprintf(c.Out, `%s connects local MCP and HTTP services to Data Proxy.
It runs like cloudflared: a local outbound daemon for HTTP tunnels, MCP bridge, and policy-guarded workspace tools.

Usage:
  %[1]s version
  %[1]s enroll --server <url> --setup-token <one-time-token>
  %[1]s enroll --server <url> --access-token <token> --user-id <id>
  %[1]s config path|show|validate|export [--config <path>]
  %[1]s config token status|store|migrate|delete [--config <path>]
  %[1]s mcp list|add|test|remove [--config <path>]
  %[1]s tunnel route list|add|test|remove [--config <path>]
  %[1]s status [--config <path>] [--json] [--health] [--timeout 5s]
  %[1]s doctor [--config <path>] [--json] [--check-update]
  %[1]s logs path|tail [--config <path>]
  %[1]s report [--config <path>] [--output <zip>] [--check-update]
  %[1]s service install|uninstall|start|stop|restart|status [--config <path>]
  %[1]s update [--version latest|vX.Y.Z] [--dry-run]
  %[1]s self-test
  %[1]s run [--config <path>] [--bridge-ws-url <url>] [--token <token>] [--client-id <id>]

Environment:
  DATA_PROXY_AGENT_CONFIG      Config path override.
  DATA_PROXY_BASE_URL          Data Proxy base URL, for example https://<data-proxy-host>.
  DATA_PROXY_BRIDGE_WS_URL     Bridge WebSocket URL.
  DATA_PROXY_API_KEY           Agent API key used for /bridge/ws.
  DATA_PROXY_ACCESS_TOKEN      Dashboard access token used by enroll.
  DATA_PROXY_USER_ID           Dashboard user id used by enroll.
  DATA_PROXY_BRIDGE_CLIENT_ID  Bridge client id.
`, program)
}

func (c CLI) printConfigHelp() {
	program := c.programName()
	fmt.Fprintf(c.Out, `Usage:
  %[1]s config path [--config <path>]
  %[1]s config show [--config <path>] [--json]
  %[1]s config validate [--config <path>]
  %[1]s config export [--config <path>] [--json]

Config commands print, validate, or export the local agent config. Secret values are redacted.
Token commands manage agent.token_ref in system keyring or a private secret-file.
`, program)
}

func (c CLI) printMCPHelp() {
	program := c.programName()
	fmt.Fprintf(c.Out, `Usage:
  %[1]s mcp list [--config <path>] [--json]
  %[1]s mcp add <name> --url <endpoint> [--transport streamable-http] [--config <path>]
  %[1]s mcp add <name> --transport stdio --command <command> [--config <path>]
  %[1]s mcp test <name> [--config <path>]
  %[1]s mcp remove <name> [--config <path>]

MCP servers are local targets. Stdio commands are read from local config only; the server cannot push arbitrary commands.
`, program)
}

func (c CLI) printTunnelHelp() {
	program := c.programName()
	fmt.Fprintf(c.Out, `Usage:
  %[1]s tunnel route list [--config <path>] [--json]
  %[1]s tunnel route add http <name> --url <local-url> [--allow-websocket] [--allow-sse] [--config <path>]
  %[1]s tunnel route add tcp <name> --host <local-host> --port <local-port> [--config <path>]
  %[1]s tunnel route test <name> [--config <path>] [--timeout 5s] [--json]
  %[1]s tunnel route remove <name> [--config <path>]

Tunnel routes expose local HTTP/SSE/WebSocket or TCP services through an approved Data Proxy tunnel connection.
`, program)
}

func (c CLI) printTunnelRouteHelp() {
	program := c.programName()
	fmt.Fprintf(c.Out, `Usage:
  %[1]s tunnel route list [--config <path>] [--json]
  %[1]s tunnel route add http <name> --url <local-url> [--allow-websocket] [--allow-sse]
  %[1]s tunnel route add tcp <name> --host <local-host> --port <local-port>
  %[1]s tunnel route test <name> [--config <path>] [--timeout 5s] [--json]
  %[1]s tunnel route remove <name> [--config <path>]
`, program)
}

func isHelpArg(value string) bool {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "help", "-h", "--help":
		return true
	default:
		return false
	}
}

func (c CLI) runConfig(args []string) int {
	if len(args) == 0 || isHelpArg(args[0]) {
		c.printConfigHelp()
		if len(args) == 0 {
			return 2
		}
		return 0
	}
	subcommand := args[0]
	if subcommand == "token" {
		return c.runConfigToken(args[1:])
	}
	fs := flag.NewFlagSet("config "+subcommand, flag.ContinueOnError)
	fs.SetOutput(c.Err)
	configPath := fs.String("config", "", "config path")
	jsonOutput := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	switch subcommand {
	case "path":
		path := *configPath
		if path == "" {
			resolved, err := ConfigPath()
			if err != nil {
				fmt.Fprintln(c.Err, err)
				return 1
			}
			path = resolved
		}
		fmt.Fprintln(c.Out, path)
		return 0
	case "show", "export":
		cfg, _, err := LoadConfig(*configPath)
		if err != nil {
			fmt.Fprintln(c.Err, err)
			return 1
		}
		cfg = RedactedConfig(cfg)
		return c.printConfig(cfg, *jsonOutput)
	case "validate":
		cfg, loaded, err := LoadConfig(*configPath)
		if err != nil {
			fmt.Fprintln(c.Err, err)
			return 1
		}
		if !loaded {
			fmt.Fprintln(c.Err, "warning: config file not found; validating defaults and environment")
		}
		result := ValidateConfig(cfg, false)
		printValidation(c.Out, result)
		if !result.OK() {
			return 1
		}
		return 0
	default:
		fmt.Fprintf(c.Err, "unknown config subcommand: %s\n", subcommand)
		c.printConfigHelp()
		return 2
	}
}

func (c CLI) runConfigToken(args []string) int {
	if len(args) == 0 || isHelpArg(args[0]) {
		program := c.programName()
		fmt.Fprintf(c.Out, `Usage:
  %[1]s config token status [--config <path>]
  %[1]s config token store --value <token> [--store auto|native|secret-file|config] [--config <path>]
  %[1]s config token store --value-env DATA_PROXY_API_KEY [--store auto|native|secret-file|config]
  %[1]s config token store --stdin [--store auto|native|secret-file|config]
  %[1]s config token migrate [--store auto|native|secret-file|config] [--config <path>]
  %[1]s config token delete [--config <path>]

Token store modes:
  auto        Try system keyring first, then fall back to a private secret-file.
  native      Store in the OS keyring/Keychain/Credential Manager/Secret Service.
  secret-file Store beside the config with 0600 file mode.
  config      Store in agent.token for compatibility.
`, program)
		if len(args) == 0 {
			return 2
		}
		return 0
	}
	switch args[0] {
	case "status":
		return c.runConfigTokenStatus(args[1:])
	case "store":
		return c.runConfigTokenStore(args[1:])
	case "migrate":
		return c.runConfigTokenMigrate(args[1:])
	case "delete", "remove", "rm":
		return c.runConfigTokenDelete(args[1:])
	default:
		fmt.Fprintf(c.Err, "unknown config token subcommand: %s\n", args[0])
		return 2
	}
}

func (c CLI) runConfigTokenStatus(args []string) int {
	fs := flag.NewFlagSet("config token status", flag.ContinueOnError)
	fs.SetOutput(c.Err)
	configPath := fs.String("config", "", "config path")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	cfg, loaded, err := LoadConfig(*configPath)
	if err != nil {
		fmt.Fprintln(c.Err, err)
		return 1
	}
	token, source := ResolveTokenWithSource(cfg)
	fmt.Fprintf(c.Out, "config_loaded: %t\n", loaded)
	fmt.Fprintf(c.Out, "token_configured: %t\n", token != "")
	if source != "" {
		fmt.Fprintf(c.Out, "token_source: %s\n", source)
	}
	if strings.TrimSpace(cfg.Agent.TokenRef) != "" {
		fmt.Fprintf(c.Out, "token_ref: %s\n", cfg.Agent.TokenRef)
	}
	return 0
}

func (c CLI) runConfigTokenStore(args []string) int {
	fs := flag.NewFlagSet("config token store", flag.ContinueOnError)
	fs.SetOutput(c.Err)
	configPath := fs.String("config", "", "config path")
	store := fs.String("store", TokenStoreAuto, "token store: auto, native, secret-file, config")
	value := fs.String("value", "", "agent token value")
	valueEnv := fs.String("value-env", "", "environment variable containing the token")
	readStdin := fs.Bool("stdin", false, "read token from stdin")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	token, err := tokenFromInput(*value, *valueEnv, *readStdin, os.Stdin)
	if err != nil {
		fmt.Fprintln(c.Err, err)
		return 2
	}
	return c.storeTokenAndSave(*configPath, *store, token)
}

func (c CLI) runConfigTokenMigrate(args []string) int {
	fs := flag.NewFlagSet("config token migrate", flag.ContinueOnError)
	fs.SetOutput(c.Err)
	configPath := fs.String("config", "", "config path")
	store := fs.String("store", TokenStoreAuto, "token store: auto, native, secret-file, config")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	cfg, _, err := LoadConfig(*configPath)
	if err != nil {
		fmt.Fprintln(c.Err, err)
		return 1
	}
	token := ResolveToken(cfg)
	if token == "" {
		fmt.Fprintln(c.Err, "agent token is not configured")
		return 1
	}
	return c.storeTokenAndSave(*configPath, *store, token)
}

func (c CLI) runConfigTokenDelete(args []string) int {
	fs := flag.NewFlagSet("config token delete", flag.ContinueOnError)
	fs.SetOutput(c.Err)
	configPath := fs.String("config", "", "config path")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	cfg, _, err := LoadConfig(*configPath)
	if err != nil {
		fmt.Fprintln(c.Err, err)
		return 1
	}
	if strings.TrimSpace(cfg.Agent.TokenRef) != "" {
		if err := DeleteSecretRef(cfg.Agent.TokenRef); err != nil {
			fmt.Fprintf(c.Err, "warning: failed to delete token_ref secret: %s\n", err)
		}
	}
	cfg.Agent.Token = ""
	cfg.Agent.TokenRef = ""
	cfg.Agent.TokenFile = ""
	if err := SaveConfig(*configPath, cfg); err != nil {
		fmt.Fprintln(c.Err, err)
		return 1
	}
	fmt.Fprintln(c.Out, "agent token deleted")
	return 0
}

func (c CLI) storeTokenAndSave(configPath string, store string, token string) int {
	cfg, _, err := LoadConfig(configPath)
	if err != nil {
		fmt.Fprintln(c.Err, err)
		return 1
	}
	resolvedConfigPath := configPath
	if strings.TrimSpace(resolvedConfigPath) == "" {
		resolvedConfigPath, err = ConfigPath()
		if err != nil {
			fmt.Fprintln(c.Err, err)
			return 1
		}
	}
	ref, err := StoreAgentToken(resolvedConfigPath, &cfg, token, store)
	if err != nil {
		fmt.Fprintln(c.Err, err)
		return 1
	}
	if err := SaveConfig(resolvedConfigPath, cfg); err != nil {
		fmt.Fprintln(c.Err, err)
		return 1
	}
	fmt.Fprintf(c.Out, "agent token stored: %s\n", ref)
	return 0
}

func (c CLI) runMCP(args []string) int {
	if len(args) == 0 || isHelpArg(args[0]) {
		c.printMCPHelp()
		if len(args) == 0 {
			return 2
		}
		return 0
	}
	switch args[0] {
	case "list":
		return c.runMCPList(args[1:])
	case "add":
		return c.runMCPAdd(args[1:])
	case "test":
		return c.runMCPTest(args[1:])
	case "remove", "rm":
		return c.runMCPRemove(args[1:])
	default:
		fmt.Fprintf(c.Err, "unknown mcp subcommand: %s\n", args[0])
		c.printMCPHelp()
		return 2
	}
}

func (c CLI) runMCPList(args []string) int {
	fs := flag.NewFlagSet("mcp list", flag.ContinueOnError)
	fs.SetOutput(c.Err)
	configPath := fs.String("config", "", "config path")
	jsonOutput := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	cfg, _, err := LoadConfig(*configPath)
	if err != nil {
		fmt.Fprintln(c.Err, err)
		return 1
	}
	if *jsonOutput {
		bytes, err := json.MarshalIndent(cfg.MCPServers, "", "  ")
		if err != nil {
			fmt.Fprintln(c.Err, err)
			return 1
		}
		fmt.Fprintln(c.Out, string(bytes))
		return 0
	}
	if len(cfg.MCPServers) == 0 {
		fmt.Fprintln(c.Out, "no mcp servers configured")
		return 0
	}
	for _, server := range cfg.MCPServers {
		target := server.Endpoint
		if strings.TrimSpace(target) == "" {
			target = server.Command
		}
		fmt.Fprintf(c.Out, "%s\t%s\t%s\n", server.Name, server.Transport, target)
	}
	return 0
}

func (c CLI) runMCPAdd(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(c.Err, "mcp add requires a name")
		return 2
	}
	name := strings.TrimSpace(args[0])
	fs := flag.NewFlagSet("mcp add", flag.ContinueOnError)
	fs.SetOutput(c.Err)
	configPath := fs.String("config", "", "config path")
	transport := fs.String("transport", "", "transport: streamable-http, http, sse, or stdio")
	endpoint := fs.String("url", "", "MCP HTTP endpoint")
	endpointAlias := fs.String("endpoint", "", "MCP HTTP endpoint")
	command := fs.String("command", "", "stdio command")
	permission := fs.String("permission", "read_only", "permission label")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	if *endpoint == "" {
		*endpoint = *endpointAlias
	}
	if name == "" {
		fmt.Fprintln(c.Err, "mcp add name is required")
		return 2
	}
	selectedTransport := normalizeMCPTransport(*transport, *endpoint, *command)
	server := MCPServer{
		Name:       name,
		Transport:  selectedTransport,
		Endpoint:   strings.TrimSpace(*endpoint),
		Command:    strings.TrimSpace(*command),
		Permission: strings.TrimSpace(*permission),
	}
	cfg, _, err := LoadConfig(*configPath)
	if err != nil {
		fmt.Fprintln(c.Err, err)
		return 1
	}
	cfg.MCPServers = upsertMCPServer(cfg.MCPServers, server)
	if result := ValidateConfig(cfg, false); !result.OK() {
		printValidation(c.Err, result)
		return 1
	}
	if err := SaveConfig(*configPath, cfg); err != nil {
		fmt.Fprintln(c.Err, err)
		return 1
	}
	fmt.Fprintf(c.Out, "mcp server saved: %s\n", name)
	return 0
}

func (c CLI) runMCPRemove(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(c.Err, "mcp remove requires a name")
		return 2
	}
	name := strings.TrimSpace(args[0])
	fs := flag.NewFlagSet("mcp remove", flag.ContinueOnError)
	fs.SetOutput(c.Err)
	configPath := fs.String("config", "", "config path")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	cfg, _, err := LoadConfig(*configPath)
	if err != nil {
		fmt.Fprintln(c.Err, err)
		return 1
	}
	next, removed := removeMCPServer(cfg.MCPServers, name)
	if !removed {
		fmt.Fprintf(c.Err, "mcp server not found: %s\n", name)
		return 1
	}
	cfg.MCPServers = next
	if err := SaveConfig(*configPath, cfg); err != nil {
		fmt.Fprintln(c.Err, err)
		return 1
	}
	fmt.Fprintf(c.Out, "mcp server removed: %s\n", name)
	return 0
}

func (c CLI) runMCPTest(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(c.Err, "mcp test requires a name")
		return 2
	}
	name := strings.TrimSpace(args[0])
	fs := flag.NewFlagSet("mcp test", flag.ContinueOnError)
	fs.SetOutput(c.Err)
	configPath := fs.String("config", "", "config path")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	cfg, _, err := LoadConfig(*configPath)
	if err != nil {
		fmt.Fprintln(c.Err, err)
		return 1
	}
	server, ok := findMCPServer(cfg.MCPServers, name)
	if !ok {
		fmt.Fprintf(c.Err, "mcp server not found: %s\n", name)
		return 1
	}
	bridgeArgs := map[string]any{
		"transport": server.Transport,
		"server":    map[string]any{"name": server.Name},
	}
	if strings.TrimSpace(server.Endpoint) != "" {
		bridgeArgs["target"] = server.Endpoint
	}
	result, err := (BridgeClient{Config: cfg}).handleMCPProxyTest(context.Background(), bridgeArgs)
	if err != nil {
		fmt.Fprintln(c.Err, err)
		return 1
	}
	payload := mapFromAny(result.Metadata["result"])
	fmt.Fprintf(c.Out, "mcp server ok: %s (%s)\n", name, stringFromMap(payload, "server_name", server.Name))
	return 0
}

func (c CLI) runTunnel(args []string) int {
	if len(args) == 0 || isHelpArg(args[0]) {
		c.printTunnelHelp()
		if len(args) == 0 {
			return 2
		}
		return 0
	}
	switch args[0] {
	case "list":
		return c.runTunnelRouteList(args[1:])
	case "route":
		return c.runTunnelRoute(args[1:])
	default:
		fmt.Fprintf(c.Err, "unknown tunnel subcommand: %s\n", args[0])
		c.printTunnelHelp()
		return 2
	}
}

func (c CLI) runTunnelRoute(args []string) int {
	if len(args) == 0 || isHelpArg(args[0]) {
		c.printTunnelRouteHelp()
		if len(args) == 0 {
			return 2
		}
		return 0
	}
	switch args[0] {
	case "list":
		return c.runTunnelRouteList(args[1:])
	case "add":
		return c.runTunnelRouteAdd(args[1:])
	case "test":
		return c.runTunnelRouteTest(args[1:])
	case "remove", "rm":
		return c.runTunnelRouteRemove(args[1:])
	default:
		fmt.Fprintf(c.Err, "unknown tunnel route subcommand: %s\n", args[0])
		c.printTunnelRouteHelp()
		return 2
	}
}

func (c CLI) runTunnelRouteList(args []string) int {
	fs := flag.NewFlagSet("tunnel route list", flag.ContinueOnError)
	fs.SetOutput(c.Err)
	configPath := fs.String("config", "", "config path")
	jsonOutput := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	cfg, _, err := LoadConfig(*configPath)
	if err != nil {
		fmt.Fprintln(c.Err, err)
		return 1
	}
	if *jsonOutput {
		bytes, err := json.MarshalIndent(map[string]any{
			"http_routes": cfg.HTTPRoutes,
			"tcp_routes":  cfg.TCPRoutes,
		}, "", "  ")
		if err != nil {
			fmt.Fprintln(c.Err, err)
			return 1
		}
		fmt.Fprintln(c.Out, string(bytes))
		return 0
	}
	if len(cfg.HTTPRoutes) == 0 && len(cfg.TCPRoutes) == 0 {
		fmt.Fprintln(c.Out, "no tunnel routes configured")
		return 0
	}
	for _, route := range cfg.HTTPRoutes {
		fmt.Fprintf(c.Out, "%s\thttp\t%s\twebsocket=%t\tsse=%t\n", route.Name, route.Target, route.AllowWebSocket, route.AllowSSE)
	}
	for _, route := range cfg.TCPRoutes {
		fmt.Fprintf(c.Out, "%s\ttcp\t%s:%d\tallow_public=%t\n", route.Name, route.TargetHost, route.TargetPort, route.AllowPublic)
	}
	return 0
}

func (c CLI) runTunnelRouteAdd(args []string) int {
	if len(args) < 2 {
		fmt.Fprintln(c.Err, "tunnel route add requires type and name, for example: tunnel route add http local --url http://127.0.0.1:3000")
		return 2
	}
	routeType := strings.TrimSpace(strings.ToLower(args[0]))
	name := strings.TrimSpace(args[1])
	if routeType != "http" && routeType != "tcp" {
		fmt.Fprintf(c.Err, "unsupported tunnel route type: %s\n", routeType)
		return 2
	}
	fs := flag.NewFlagSet("tunnel route add", flag.ContinueOnError)
	fs.SetOutput(c.Err)
	configPath := fs.String("config", "", "config path")
	target := fs.String("url", "", "local HTTP target URL")
	targetAlias := fs.String("target", "", "local HTTP target URL")
	host := fs.String("host", "127.0.0.1", "local TCP target host")
	port := fs.Int("port", 0, "local TCP target port")
	allowPublic := fs.Bool("allow-public", false, "document that this TCP route may be exposed after server-side approval")
	allowWebSocket := fs.Bool("allow-websocket", false, "allow WebSocket upgrade")
	allowSSE := fs.Bool("allow-sse", true, "allow SSE responses")
	maxRequestBytes := fs.Int64("max-request-bytes", 0, "max request bytes")
	maxResponseBytes := fs.Int64("max-response-bytes", 0, "max response bytes")
	if err := fs.Parse(args[2:]); err != nil {
		return 2
	}
	cfg, _, err := LoadConfig(*configPath)
	if err != nil {
		fmt.Fprintln(c.Err, err)
		return 1
	}
	if routeType == "tcp" {
		route := TCPRoute{
			Name:        name,
			TargetHost:  strings.TrimSpace(*host),
			TargetPort:  *port,
			AllowPublic: *allowPublic,
		}
		cfg.TCPRoutes = upsertTCPRoute(cfg.TCPRoutes, route)
	} else {
		if *target == "" {
			*target = *targetAlias
		}
		route := HTTPRoute{
			Name:             name,
			Target:           strings.TrimSpace(*target),
			AllowWebSocket:   *allowWebSocket,
			AllowSSE:         *allowSSE,
			MaxRequestBytes:  *maxRequestBytes,
			MaxResponseBytes: *maxResponseBytes,
		}
		cfg.HTTPRoutes = upsertHTTPRoute(cfg.HTTPRoutes, route)
	}
	if result := ValidateConfig(cfg, false); !result.OK() {
		printValidation(c.Err, result)
		return 1
	}
	if err := SaveConfig(*configPath, cfg); err != nil {
		fmt.Fprintln(c.Err, err)
		return 1
	}
	fmt.Fprintf(c.Out, "tunnel route saved: %s\n", name)
	return 0
}

func (c CLI) runTunnelRouteTest(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(c.Err, "tunnel route test requires a name")
		return 2
	}
	name := strings.TrimSpace(args[0])
	fs := flag.NewFlagSet("tunnel route test", flag.ContinueOnError)
	fs.SetOutput(c.Err)
	configPath := fs.String("config", "", "config path")
	timeout := fs.Duration("timeout", 5*time.Second, "route probe timeout")
	jsonOutput := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	cfg, _, err := LoadConfig(*configPath)
	if err != nil {
		fmt.Fprintln(c.Err, err)
		return 1
	}
	routeType := ""
	var check AgentHealthCheck
	if route, ok := findHTTPRoute(cfg.HTTPRoutes, name); ok {
		routeType = "http"
		check = checkHTTPRoute(cfg, route, *timeout)
	} else if route, ok := findTCPRoute(cfg.TCPRoutes, name); ok {
		routeType = "tcp"
		check = checkTCPRoute(cfg, route, *timeout)
	} else {
		fmt.Fprintf(c.Err, "tunnel route not found: %s\n", name)
		return 1
	}
	if *jsonOutput {
		bytes, err := json.MarshalIndent(map[string]any{
			"name":    name,
			"type":    routeType,
			"status":  check.Status,
			"message": check.Detail,
		}, "", "  ")
		if err != nil {
			fmt.Fprintln(c.Err, err)
			return 1
		}
		fmt.Fprintln(c.Out, string(bytes))
	} else {
		printAgentHealthCheck(c.Out, check)
	}
	if normalizeAgentHealthStatus(check.Status) == HealthStatusFail {
		return 1
	}
	return 0
}

func (c CLI) runTunnelRouteRemove(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(c.Err, "tunnel route remove requires a name")
		return 2
	}
	name := strings.TrimSpace(args[0])
	fs := flag.NewFlagSet("tunnel route remove", flag.ContinueOnError)
	fs.SetOutput(c.Err)
	configPath := fs.String("config", "", "config path")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	cfg, _, err := LoadConfig(*configPath)
	if err != nil {
		fmt.Fprintln(c.Err, err)
		return 1
	}
	nextHTTP, removedHTTP := removeHTTPRoute(cfg.HTTPRoutes, name)
	nextTCP, removedTCP := removeTCPRoute(cfg.TCPRoutes, name)
	removed := removedHTTP || removedTCP
	if !removed {
		fmt.Fprintf(c.Err, "tunnel route not found: %s\n", name)
		return 1
	}
	cfg.HTTPRoutes = nextHTTP
	cfg.TCPRoutes = nextTCP
	if err := SaveConfig(*configPath, cfg); err != nil {
		fmt.Fprintln(c.Err, err)
		return 1
	}
	fmt.Fprintf(c.Out, "tunnel route removed: %s\n", name)
	return 0
}

func (c CLI) runStatus(args []string) int {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.SetOutput(c.Err)
	configPath := fs.String("config", "", "config path")
	jsonOutput := fs.Bool("json", false, "print JSON")
	includeHealth := fs.Bool("health", false, "include local route health checks")
	timeout := fs.Duration("timeout", 5*time.Second, "local health probe timeout")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	cfg, loaded, err := LoadConfig(*configPath)
	if err != nil {
		fmt.Fprintln(c.Err, err)
		return 1
	}
	summary := BuildAgentStatusSummaryWithHealth(cfg, loaded, *includeHealth, *timeout)
	if *jsonOutput {
		bytes, err := json.MarshalIndent(summary, "", "  ")
		if err != nil {
			fmt.Fprintln(c.Err, err)
			return 1
		}
		fmt.Fprintln(c.Out, string(bytes))
		return 0
	}
	fmt.Fprint(c.Out, RenderAgentStatusText(summary))
	return 0
}

func (c CLI) runDoctor(args []string) int {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(c.Err)
	configPath := fs.String("config", "", "config path")
	timeout := fs.Duration("timeout", 5*time.Second, "network check timeout")
	jsonOutput := fs.Bool("json", false, "print JSON")
	checkUpdate := fs.Bool("check-update", false, "check latest dpa release metadata")
	updateManifestURL := fs.String("manifest-url", "", "custom update manifest URL for --check-update")
	updateRepo := fs.String("repo", DefaultAgentUpdateRepo, "GitHub repo in owner/name form for --check-update")
	updateGitHubAPI := fs.String("github-api", DefaultAgentUpdateGitHubAPI, "GitHub API base URL for --check-update")
	allowPrerelease := fs.Bool("allow-prerelease", false, "allow prerelease versions for --check-update")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	cfg, loaded, err := LoadConfig(*configPath)
	if err != nil {
		fmt.Fprintln(c.Err, err)
		return 1
	}
	result := ValidateConfig(cfg, false)
	report := AgentDoctorReport{
		OK:           result.OK(),
		Version:      c.Version,
		ConfigLoaded: loaded,
		Validation:   result,
	}
	if !*jsonOutput {
		fmt.Fprintf(c.Out, "config_loaded: %t\n", loaded)
		printValidation(c.Out, result)
	}
	emitCheck := func(name string, status string, message string) {
		report.addCheck(name, status, message)
		if *jsonOutput {
			return
		}
		if strings.TrimSpace(message) == "" {
			fmt.Fprintf(c.Out, "%s: %s\n", name, status)
			return
		}
		fmt.Fprintf(c.Out, "%s: %s: %s\n", name, status, message)
	}
	emitHealthCheck := func(check AgentHealthCheck) {
		report.addHealthCheck(check)
		if !*jsonOutput {
			printAgentHealthCheck(c.Out, check)
		}
	}
	bridgeURL := ""
	bridgeURLValid := false
	if resolvedBridgeURL, err := EffectiveBridgeWSURL(cfg); err == nil {
		bridgeURL = resolvedBridgeURL
		bridgeURLValid = true
		if err := checkDNS(bridgeURL, *timeout); err != nil {
			emitCheck("dns", HealthStatusFail, err.Error())
		} else {
			emitCheck("dns", HealthStatusOK, "")
		}
	} else {
		emitCheck("bridge_url", HealthStatusFail, err.Error())
	}
	if cfg.Server.BaseURL != "" {
		if err := checkBaseURL(cfg.Server.BaseURL, *timeout); err != nil {
			emitCheck("base_url", HealthStatusWarn, err.Error())
		} else {
			emitCheck("base_url", HealthStatusOK, "")
		}
	}
	token := ResolveToken(cfg)
	if token == "" {
		emitCheck("token", "missing", "")
		emitCheck("bridge_auth", "skipped", "token missing")
	} else {
		emitCheck("token", "configured", "")
		if bridgeURLValid {
			if err := checkBridgeWebSocketAuth(bridgeURL, token, *timeout); err != nil {
				emitCheck("bridge_auth", HealthStatusFail, err.Error())
			} else {
				emitCheck("bridge_auth", HealthStatusOK, "")
			}
		}
	}
	for _, check := range AgentLocalHealthChecks(cfg, *timeout) {
		emitHealthCheck(check)
	}
	serviceCheck := CheckServiceStatus(context.Background(), ServiceHealthOptions{
		ConfigPath: *configPath,
		Timeout:    *timeout,
	})
	emitHealthCheck(serviceCheck)
	if *checkUpdate {
		updateCheck, err := CheckAgentUpdate(context.Background(), AgentUpdateCheckOptions{
			CurrentVersion:  c.Version,
			Version:         "latest",
			Repo:            *updateRepo,
			ManifestURL:     *updateManifestURL,
			GitHubAPIBase:   *updateGitHubAPI,
			AllowPrerelease: *allowPrerelease,
			Timeout:         *timeout,
		})
		if err != nil {
			emitCheck("update", HealthStatusWarn, err.Error())
		} else if updateCheck.UpdateAvailable {
			emitCheck("update", HealthStatusWarn, fmt.Sprintf("%s available (current %s, asset %s)", updateCheck.LatestVersion, updateCheck.CurrentVersion, updateCheck.AssetName))
		} else {
			emitCheck("update", HealthStatusOK, updateCheck.CurrentVersion+" is current")
		}
	} else {
		emitCheck("update", "skipped", "pass --check-update to query release metadata")
	}
	if *jsonOutput {
		bytes, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fmt.Fprintln(c.Err, err)
			return 1
		}
		fmt.Fprintln(c.Out, string(bytes))
		if report.OK {
			return 0
		}
		return 1
	}
	if report.OK {
		fmt.Fprintln(c.Out, "doctor: ok")
		return 0
	}
	fmt.Fprintln(c.Out, "doctor: failed")
	return 1
}

func (c CLI) runSelfTest(args []string) int {
	fs := flag.NewFlagSet("self-test", flag.ContinueOnError)
	fs.SetOutput(c.Err)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	cfg := DefaultConfig()
	cfg.Server.BaseURL = "https://dp.example.test"
	if _, err := EffectiveBridgeWSURL(cfg); err != nil {
		fmt.Fprintf(c.Err, "self-test bridge url failed: %s\n", err)
		return 1
	}
	cfg.HTTPRoutes = []HTTPRoute{{Name: "local", Target: "http://127.0.0.1:3000"}}
	if result := ValidateConfig(cfg, false); !result.OK() {
		fmt.Fprintf(c.Err, "self-test validation failed: %s\n", result.Error())
		return 1
	}
	fmt.Fprintln(c.Out, "self-test: ok")
	return 0
}

func (c CLI) runBridge(args []string) int {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(c.Err)
	opts := RuntimeOptions{}
	fs.StringVar(&opts.ConfigPath, "config", "", "config path")
	fs.StringVar(&opts.BaseURL, "base-url", "", "Data Proxy base URL")
	fs.StringVar(&opts.BridgeWSURL, "bridge-ws-url", "", "bridge WebSocket URL")
	fs.StringVar(&opts.BridgeWSURL, "server", "", "bridge WebSocket URL, or use --base-url")
	fs.StringVar(&opts.Token, "token", "", "agent API key")
	fs.StringVar(&opts.ClientID, "client-id", "", "bridge client id")
	fs.StringVar(&opts.Name, "name", "", "client display name")
	fs.StringVar(&opts.Version, "agent-version", c.Version, "agent version")
	fs.StringVar(&opts.Workspace, "workspace", "", "workspace path")
	fs.BoolVar(&opts.Once, "once", false, "disable reconnect")
	fs.BoolVar(&opts.NoReconnect, "no-reconnect", false, "disable reconnect")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	cfg, _, err := LoadConfig(opts.ConfigPath)
	if err != nil {
		fmt.Fprintln(c.Err, err)
		return 1
	}
	if opts.BridgeWSURL != "" && strings.HasPrefix(opts.BridgeWSURL, "http") {
		opts.BaseURL = opts.BridgeWSURL
		opts.BridgeWSURL = ""
	}
	cfg = ApplyRuntimeOptions(cfg, opts)
	if result := ValidateConfig(cfg, true); !result.OK() {
		printValidation(c.Err, result)
		return 1
	}
	client := BridgeClient{Config: cfg, Out: c.Out, Err: c.Err}
	if err := client.Run(context.Background()); err != nil {
		fmt.Fprintln(c.Err, err)
		return 1
	}
	return 0
}

func (c CLI) printConfig(cfg Config, jsonOutput bool) int {
	var (
		bytes []byte
		err   error
	)
	if jsonOutput {
		bytes, err = json.MarshalIndent(cfg, "", "  ")
	} else {
		bytes, err = yaml.Marshal(cfg)
	}
	if err != nil {
		fmt.Fprintln(c.Err, err)
		return 1
	}
	fmt.Fprintln(c.Out, string(bytes))
	return 0
}

func printValidation(w io.Writer, result ValidationResult) {
	if result.OK() {
		fmt.Fprintln(w, "validation: ok")
	} else {
		fmt.Fprintln(w, "validation: failed")
	}
	for _, warning := range result.Warnings {
		fmt.Fprintf(w, "warning: %s\n", warning)
	}
	for _, item := range result.Errors {
		fmt.Fprintf(w, "error: %s\n", item)
	}
}

func printAgentHealthCheck(w io.Writer, check AgentHealthCheck) {
	if strings.TrimSpace(check.Detail) == "" {
		fmt.Fprintf(w, "%s: %s\n", check.Name, check.Status)
		return
	}
	fmt.Fprintf(w, "%s: %s: %s\n", check.Name, check.Status, check.Detail)
}

func checkDNS(rawURL string, timeout time.Duration) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("missing host")
	}
	resolver := net.Resolver{}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	_, err = resolver.LookupHost(ctx, host)
	return err
}

func checkBaseURL(rawURL string, timeout time.Duration) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	if u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("missing scheme or host")
	}
	check := *u
	check.Path = "/api/status"
	check.RawQuery = ""
	client := http.Client{Timeout: timeout}
	resp, err := client.Get(check.String())
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return nil
}

func normalizeMCPTransport(transport string, endpoint string, command string) string {
	value := strings.ToLower(strings.TrimSpace(transport))
	switch value {
	case "streamable-http", "streamable_http":
		return "streamable_http"
	case "http", "sse", "stdio":
		return value
	}
	if strings.TrimSpace(command) != "" {
		return "stdio"
	}
	if strings.TrimSpace(endpoint) != "" {
		return "streamable_http"
	}
	return value
}

func upsertMCPServer(items []MCPServer, server MCPServer) []MCPServer {
	for index, item := range items {
		if item.Name == server.Name {
			next := append([]MCPServer(nil), items...)
			next[index] = server
			return next
		}
	}
	return append(append([]MCPServer(nil), items...), server)
}

func removeMCPServer(items []MCPServer, name string) ([]MCPServer, bool) {
	next := make([]MCPServer, 0, len(items))
	removed := false
	for _, item := range items {
		if item.Name == name {
			removed = true
			continue
		}
		next = append(next, item)
	}
	return next, removed
}

func findMCPServer(items []MCPServer, name string) (MCPServer, bool) {
	for _, item := range items {
		if item.Name == name {
			return item, true
		}
	}
	return MCPServer{}, false
}

func upsertHTTPRoute(items []HTTPRoute, route HTTPRoute) []HTTPRoute {
	for index, item := range items {
		if item.Name == route.Name {
			next := append([]HTTPRoute(nil), items...)
			next[index] = route
			return next
		}
	}
	return append(append([]HTTPRoute(nil), items...), route)
}

func upsertTCPRoute(items []TCPRoute, route TCPRoute) []TCPRoute {
	for index, item := range items {
		if item.Name == route.Name {
			next := append([]TCPRoute(nil), items...)
			next[index] = route
			return next
		}
	}
	return append(append([]TCPRoute(nil), items...), route)
}

func findHTTPRoute(items []HTTPRoute, name string) (HTTPRoute, bool) {
	for _, item := range items {
		if item.Name == name {
			return item, true
		}
	}
	return HTTPRoute{}, false
}

func findTCPRoute(items []TCPRoute, name string) (TCPRoute, bool) {
	for _, item := range items {
		if item.Name == name {
			return item, true
		}
	}
	return TCPRoute{}, false
}

func removeHTTPRoute(items []HTTPRoute, name string) ([]HTTPRoute, bool) {
	next := make([]HTTPRoute, 0, len(items))
	removed := false
	for _, item := range items {
		if item.Name == name {
			removed = true
			continue
		}
		next = append(next, item)
	}
	return next, removed
}

func removeTCPRoute(items []TCPRoute, name string) ([]TCPRoute, bool) {
	next := make([]TCPRoute, 0, len(items))
	removed := false
	for _, item := range items {
		if item.Name == name {
			removed = true
			continue
		}
		next = append(next, item)
	}
	return next, removed
}
