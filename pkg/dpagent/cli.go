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
	Version string
	Out     io.Writer
	Err     io.Writer
}

func RunCLI(args []string, out io.Writer, errOut io.Writer, version string) int {
	cli := CLI{Version: version, Out: out, Err: errOut}
	return cli.Run(args)
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
	if len(args) == 0 {
		c.printHelp()
		return 0
	}
	switch args[0] {
	case "help", "-h", "--help":
		c.printHelp()
		return 0
	case "version", "--version":
		fmt.Fprintf(c.Out, "data-proxy-agent %s\n", c.Version)
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
	case "service":
		return c.runService(args[1:])
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
	fmt.Fprint(c.Out, `data-proxy-agent connects local MCP and HTTP services to Data Proxy.

Usage:
  data-proxy-agent version
  data-proxy-agent enroll --server <url> --access-token <token> --user-id <id>
  data-proxy-agent config path|show|validate|export [--config <path>]
  data-proxy-agent mcp list|add|test|remove [--config <path>]
  data-proxy-agent tunnel route list|add|remove [--config <path>]
  data-proxy-agent status [--config <path>]
  data-proxy-agent doctor [--config <path>]
  data-proxy-agent report [--config <path>] [--output <zip>]
  data-proxy-agent service install|uninstall|start|stop|restart|status [--config <path>]
  data-proxy-agent self-test
  data-proxy-agent run [--config <path>] [--bridge-ws-url <url>] [--token <token>] [--client-id <id>]

Environment:
  DATA_PROXY_AGENT_CONFIG      Config path override.
  DATA_PROXY_BASE_URL          Data Proxy base URL, for example https://dp.app.mbu.ltd.
  DATA_PROXY_BRIDGE_WS_URL     Bridge WebSocket URL.
  DATA_PROXY_API_KEY           Agent API key used for /bridge/ws.
  DATA_PROXY_ACCESS_TOKEN      Dashboard access token used by enroll.
  DATA_PROXY_USER_ID           Dashboard user id used by enroll.
  DATA_PROXY_BRIDGE_CLIENT_ID  Bridge client id.
`)
}

func (c CLI) runConfig(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(c.Err, "config subcommand is required")
		return 2
	}
	subcommand := args[0]
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
		return 2
	}
}

func (c CLI) runMCP(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(c.Err, "mcp subcommand is required")
		return 2
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
	if strings.TrimSpace(server.Endpoint) == "" {
		fmt.Fprintf(c.Err, "mcp server %s has no HTTP endpoint; stdio test is not implemented yet\n", name)
		return 1
	}
	result, err := (BridgeClient{Config: cfg}).handleMCPProxyTest(context.Background(), map[string]any{
		"target": server.Endpoint,
		"server": map[string]any{"name": server.Name},
	})
	if err != nil {
		fmt.Fprintln(c.Err, err)
		return 1
	}
	payload := mapFromAny(result.Metadata["result"])
	fmt.Fprintf(c.Out, "mcp server ok: %s (%s)\n", name, stringFromMap(payload, "server_name", server.Name))
	return 0
}

func (c CLI) runTunnel(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(c.Err, "tunnel subcommand is required")
		return 2
	}
	switch args[0] {
	case "list":
		return c.runTunnelRouteList(args[1:])
	case "route":
		return c.runTunnelRoute(args[1:])
	default:
		fmt.Fprintf(c.Err, "unknown tunnel subcommand: %s\n", args[0])
		return 2
	}
}

func (c CLI) runTunnelRoute(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(c.Err, "tunnel route subcommand is required")
		return 2
	}
	switch args[0] {
	case "list":
		return c.runTunnelRouteList(args[1:])
	case "add":
		return c.runTunnelRouteAdd(args[1:])
	case "remove", "rm":
		return c.runTunnelRouteRemove(args[1:])
	default:
		fmt.Fprintf(c.Err, "unknown tunnel route subcommand: %s\n", args[0])
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
		bytes, err := json.MarshalIndent(cfg.HTTPRoutes, "", "  ")
		if err != nil {
			fmt.Fprintln(c.Err, err)
			return 1
		}
		fmt.Fprintln(c.Out, string(bytes))
		return 0
	}
	if len(cfg.HTTPRoutes) == 0 {
		fmt.Fprintln(c.Out, "no tunnel routes configured")
		return 0
	}
	for _, route := range cfg.HTTPRoutes {
		fmt.Fprintf(c.Out, "%s\thttp\t%s\twebsocket=%t\tsse=%t\n", route.Name, route.Target, route.AllowWebSocket, route.AllowSSE)
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
	if routeType != "http" {
		fmt.Fprintf(c.Err, "unsupported tunnel route type: %s\n", routeType)
		return 2
	}
	fs := flag.NewFlagSet("tunnel route add", flag.ContinueOnError)
	fs.SetOutput(c.Err)
	configPath := fs.String("config", "", "config path")
	target := fs.String("url", "", "local HTTP target URL")
	targetAlias := fs.String("target", "", "local HTTP target URL")
	allowWebSocket := fs.Bool("allow-websocket", false, "allow WebSocket upgrade")
	allowSSE := fs.Bool("allow-sse", true, "allow SSE responses")
	maxRequestBytes := fs.Int64("max-request-bytes", 0, "max request bytes")
	maxResponseBytes := fs.Int64("max-response-bytes", 0, "max response bytes")
	if err := fs.Parse(args[2:]); err != nil {
		return 2
	}
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
	cfg, _, err := LoadConfig(*configPath)
	if err != nil {
		fmt.Fprintln(c.Err, err)
		return 1
	}
	cfg.HTTPRoutes = upsertHTTPRoute(cfg.HTTPRoutes, route)
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
	next, removed := removeHTTPRoute(cfg.HTTPRoutes, name)
	if !removed {
		fmt.Fprintf(c.Err, "tunnel route not found: %s\n", name)
		return 1
	}
	cfg.HTTPRoutes = next
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
	if err := fs.Parse(args); err != nil {
		return 2
	}
	cfg, loaded, err := LoadConfig(*configPath)
	if err != nil {
		fmt.Fprintln(c.Err, err)
		return 1
	}
	bridgeURL, _ := EffectiveBridgeWSURL(cfg)
	fmt.Fprintf(c.Out, "config_loaded: %t\n", loaded)
	fmt.Fprintf(c.Out, "client_id: %s\n", cfg.Agent.ClientID)
	fmt.Fprintf(c.Out, "name: %s\n", cfg.Agent.Name)
	fmt.Fprintf(c.Out, "version: %s\n", cfg.Agent.Version)
	fmt.Fprintf(c.Out, "workspace: %s\n", cfg.Agent.Workspace)
	fmt.Fprintf(c.Out, "bridge_ws_url: %s\n", bridgeURL)
	fmt.Fprintf(c.Out, "token_configured: %t\n", ResolveToken(cfg) != "")
	fmt.Fprintf(c.Out, "capabilities: %s\n", strings.Join(EffectiveCapabilities(cfg), ","))
	fmt.Fprintf(c.Out, "mcp_servers: %d\n", len(cfg.MCPServers))
	fmt.Fprintf(c.Out, "http_routes: %d\n", len(cfg.HTTPRoutes))
	return 0
}

func (c CLI) runDoctor(args []string) int {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(c.Err)
	configPath := fs.String("config", "", "config path")
	timeout := fs.Duration("timeout", 5*time.Second, "network check timeout")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	cfg, loaded, err := LoadConfig(*configPath)
	if err != nil {
		fmt.Fprintln(c.Err, err)
		return 1
	}
	fmt.Fprintf(c.Out, "config_loaded: %t\n", loaded)
	result := ValidateConfig(cfg, false)
	printValidation(c.Out, result)
	ok := result.OK()
	if bridgeURL, err := EffectiveBridgeWSURL(cfg); err == nil {
		if err := checkDNS(bridgeURL, *timeout); err != nil {
			fmt.Fprintf(c.Out, "dns: fail: %s\n", err)
			ok = false
		} else {
			fmt.Fprintln(c.Out, "dns: ok")
		}
	} else {
		fmt.Fprintf(c.Out, "bridge_url: fail: %s\n", err)
		ok = false
	}
	if cfg.Server.BaseURL != "" {
		if err := checkBaseURL(cfg.Server.BaseURL, *timeout); err != nil {
			fmt.Fprintf(c.Out, "base_url: warn: %s\n", err)
		} else {
			fmt.Fprintln(c.Out, "base_url: ok")
		}
	}
	if ResolveToken(cfg) == "" {
		fmt.Fprintln(c.Out, "token: missing")
	} else {
		fmt.Fprintln(c.Out, "token: configured")
	}
	if ok {
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
