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
	case "config":
		return c.runConfig(args[1:])
	case "status":
		return c.runStatus(args[1:])
	case "doctor":
		return c.runDoctor(args[1:])
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
  data-proxy-agent config path|show|validate|export [--config <path>]
  data-proxy-agent status [--config <path>]
  data-proxy-agent doctor [--config <path>]
  data-proxy-agent self-test
  data-proxy-agent run [--config <path>] [--bridge-ws-url <url>] [--token <token>] [--client-id <id>]

Environment:
  DATA_PROXY_AGENT_CONFIG      Config path override.
  DATA_PROXY_BASE_URL          Data Proxy base URL, for example https://dp.app.mbu.ltd.
  DATA_PROXY_BRIDGE_WS_URL     Bridge WebSocket URL.
  DATA_PROXY_API_KEY           Agent API key used for /bridge/ws.
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
