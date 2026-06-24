package dpagent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/dto"
)

const DefaultEnrollTimeout = 20 * time.Second

type EnrollOptions struct {
	ConfigPath  string
	BaseURL     string
	AccessToken string
	SetupToken  string
	UserID      int
	ClientID    string
	Name        string
	Workspace   string
	Version     string
	TokenStore  string
	Rotate      bool
	DryRun      bool
	Timeout     time.Duration
	HTTPClient  *http.Client
}

type EnrollResult struct {
	ConfigPath string                       `json:"config_path"`
	Saved      bool                         `json:"saved"`
	Setup      dto.BridgeAgentSetupResponse `json:"setup"`
	Config     Config                       `json:"config"`
}

type apiEnvelope[T any] struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    T      `json:"data"`
}

func (c CLI) runEnroll(args []string) int {
	fs := flag.NewFlagSet("enroll", flag.ContinueOnError)
	fs.SetOutput(c.Err)
	configPath := fs.String("config", "", "config path")
	server := fs.String("server", "", "Data Proxy base URL")
	baseURL := fs.String("base-url", "", "Data Proxy base URL")
	accessToken := fs.String("access-token", "", "dashboard access token")
	setupToken := fs.String("setup-token", "", "one-time setup token generated in Data Proxy")
	userID := fs.Int("user-id", 0, "dashboard user id")
	clientID := fs.String("client-id", "", "bridge client id")
	name := fs.String("name", "", "client display name")
	workspace := fs.String("workspace", "", "workspace path")
	tokenStore := fs.String("token-store", TokenStoreAuto, "token store: auto, native, secret-file, or config")
	rotate := fs.Bool("rotate", false, "rotate agent token")
	dryRun := fs.Bool("dry-run", false, "do not write config")
	jsonOutput := fs.Bool("json", false, "print JSON")
	timeout := fs.Duration("timeout", DefaultEnrollTimeout, "enroll request timeout")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *baseURL == "" {
		*baseURL = *server
	}
	if *accessToken == "" {
		*accessToken = strings.TrimSpace(os.Getenv("DATA_PROXY_ACCESS_TOKEN"))
	}
	if *userID == 0 {
		*userID = envInt("DATA_PROXY_USER_ID")
	}
	opts := EnrollOptions{
		ConfigPath:  *configPath,
		BaseURL:     *baseURL,
		AccessToken: *accessToken,
		SetupToken:  *setupToken,
		UserID:      *userID,
		ClientID:    *clientID,
		Name:        *name,
		Workspace:   *workspace,
		Version:     c.Version,
		TokenStore:  *tokenStore,
		Rotate:      *rotate,
		DryRun:      *dryRun,
		Timeout:     *timeout,
	}
	result, err := EnrollBridgeAgent(context.Background(), opts)
	if err != nil {
		fmt.Fprintln(c.Err, err)
		return 1
	}
	if *jsonOutput {
		redacted := result
		redacted.Setup.APIKey = redactSecret(redacted.Setup.APIKey)
		redacted.Setup.Headers = redactStringMap(redacted.Setup.Headers)
		redacted.Setup.Environment = redactStringMap(redacted.Setup.Environment)
		redacted.Setup.Config = redactAnyMap(redacted.Setup.Config)
		redacted.Config = RedactedConfig(redacted.Config)
		bytes, err := json.MarshalIndent(redacted, "", "  ")
		if err != nil {
			fmt.Fprintln(c.Err, err)
			return 1
		}
		fmt.Fprintln(c.Out, string(bytes))
		return 0
	}
	if result.Saved {
		fmt.Fprintf(c.Out, "enrolled bridge client: %s\n", result.Setup.ClientId)
		fmt.Fprintf(c.Out, "config saved: %s\n", result.ConfigPath)
	} else {
		fmt.Fprintf(c.Out, "enroll dry-run ok: %s\n", result.Setup.ClientId)
		fmt.Fprintf(c.Out, "config path: %s\n", result.ConfigPath)
	}
	fmt.Fprintf(c.Out, "bridge_ws_url: %s\n", result.Setup.BridgeWSURL)
	fmt.Fprintf(c.Out, "token: %s\n", result.Setup.TokenMaskedKey)
	return 0
}

func EnrollBridgeAgent(ctx context.Context, opts EnrollOptions) (EnrollResult, error) {
	cfg, _, err := LoadConfig(opts.ConfigPath)
	if err != nil {
		return EnrollResult{}, err
	}
	baseURL := strings.TrimRight(strings.TrimSpace(opts.BaseURL), "/")
	if baseURL == "" {
		baseURL = strings.TrimRight(strings.TrimSpace(cfg.Server.BaseURL), "/")
	}
	if baseURL == "" {
		baseURL = strings.TrimRight(strings.TrimSpace(os.Getenv("DATA_PROXY_BASE_URL")), "/")
	}
	if baseURL == "" {
		return EnrollResult{}, errors.New("server URL is required; pass --server or set DATA_PROXY_BASE_URL")
	}
	if _, err := url.ParseRequestURI(baseURL); err != nil {
		return EnrollResult{}, fmt.Errorf("server URL is invalid: %w", err)
	}
	accessToken := strings.TrimSpace(opts.AccessToken)
	setupToken := strings.TrimSpace(opts.SetupToken)
	if setupToken == "" {
		if accessToken == "" {
			return EnrollResult{}, errors.New("dashboard access token is required; pass --access-token or set DATA_PROXY_ACCESS_TOKEN, or pass --setup-token")
		}
		if opts.UserID <= 0 {
			return EnrollResult{}, errors.New("dashboard user id is required; pass --user-id or set DATA_PROXY_USER_ID, or pass --setup-token")
		}
	}
	if strings.TrimSpace(opts.Version) == "" {
		opts.Version = DefaultAgentVersion
	}
	if strings.TrimSpace(opts.Name) == "" {
		opts.Name = cfg.Agent.Name
	}
	if strings.TrimSpace(opts.Workspace) == "" {
		opts.Workspace = cfg.Agent.Workspace
	}
	request := dto.BridgeAgentSetupRequest{
		ClientId:   strings.TrimSpace(opts.ClientID),
		Rotate:     opts.Rotate,
		ClientName: strings.TrimSpace(opts.Name),
		Version:    strings.TrimSpace(opts.Version),
		Platform:   agentPlatform(),
		Workspace:  strings.TrimSpace(opts.Workspace),
	}
	var setup dto.BridgeAgentSetupResponse
	if setupToken != "" {
		setup, err = requestBridgeAgentSetupTokenConsume(ctx, baseURL, setupToken, request, opts)
	} else {
		setup, err = requestBridgeAgentSetup(ctx, baseURL, accessToken, opts.UserID, request, opts)
	}
	if err != nil {
		return EnrollResult{}, err
	}
	if strings.TrimSpace(setup.APIKey) == "" && strings.TrimSpace(ResolveToken(cfg)) == "" {
		return EnrollResult{}, errors.New("server did not return a new agent token; rerun enroll with --rotate or create a fresh bridge client")
	}
	cfg.Server.BaseURL = setup.BaseURL
	cfg.Server.BridgeWSURL = setup.BridgeWSURL
	cfg.Agent.ClientID = setup.ClientId
	if strings.TrimSpace(setup.Client.Name) != "" {
		cfg.Agent.Name = setup.Client.Name
	}
	cfg.Agent.Version = strings.TrimSpace(opts.Version)
	if strings.TrimSpace(setup.Client.Workspace) != "" {
		cfg.Agent.Workspace = setup.Client.Workspace
	}
	if strings.TrimSpace(setup.APIKey) != "" {
		cfg.Agent.Token = setup.APIKey
	}
	fillConfigDefaults(&cfg)

	configPath := opts.ConfigPath
	if strings.TrimSpace(configPath) == "" {
		configPath, err = ConfigPath()
		if err != nil {
			return EnrollResult{}, err
		}
	}
	result := EnrollResult{
		ConfigPath: configPath,
		Saved:      !opts.DryRun,
		Setup:      setup,
		Config:     cfg,
	}
	if opts.DryRun {
		return result, nil
	}
	if strings.TrimSpace(setup.APIKey) != "" {
		storeMode := opts.TokenStore
		if strings.TrimSpace(storeMode) == "" {
			storeMode = TokenStoreAuto
		}
		if _, err := StoreAgentToken(configPath, &cfg, setup.APIKey, storeMode); err != nil {
			return EnrollResult{}, fmt.Errorf("failed to store agent token: %w", err)
		}
		result.Config = cfg
	}
	if err := SaveConfig(configPath, cfg); err != nil {
		return EnrollResult{}, err
	}
	return result, nil
}

func requestBridgeAgentSetup(ctx context.Context, baseURL string, accessToken string, userID int, payload dto.BridgeAgentSetupRequest, opts EnrollOptions) (dto.BridgeAgentSetupResponse, error) {
	bytesPayload, err := json.Marshal(payload)
	if err != nil {
		return dto.BridgeAgentSetupResponse{}, err
	}
	endpoint := strings.TrimRight(baseURL, "/") + "/api/bridge/agent-setup"
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = DefaultEnrollTimeout
	}
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, endpoint, bytes.NewReader(bytesPayload))
	if err != nil {
		return dto.BridgeAgentSetupResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", accessToken)
	req.Header.Set("New-Api-User", strconv.Itoa(userID))
	req.Header.Set("User-Agent", "data-proxy-agent/"+strings.TrimSpace(opts.Version))

	client := opts.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: timeout}
	}
	resp, err := client.Do(req)
	if err != nil {
		return dto.BridgeAgentSetupResponse{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return dto.BridgeAgentSetupResponse{}, err
	}
	var envelope apiEnvelope[dto.BridgeAgentSetupResponse]
	if err := json.Unmarshal(body, &envelope); err != nil {
		return dto.BridgeAgentSetupResponse{}, fmt.Errorf("failed to decode enroll response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if envelope.Message != "" {
			return dto.BridgeAgentSetupResponse{}, fmt.Errorf("enroll failed: HTTP %d: %s", resp.StatusCode, envelope.Message)
		}
		return dto.BridgeAgentSetupResponse{}, fmt.Errorf("enroll failed: HTTP %d", resp.StatusCode)
	}
	if !envelope.Success {
		if envelope.Message == "" {
			envelope.Message = "server returned success=false"
		}
		return dto.BridgeAgentSetupResponse{}, errors.New("enroll failed: " + envelope.Message)
	}
	if strings.TrimSpace(envelope.Data.ClientId) == "" || strings.TrimSpace(envelope.Data.BridgeWSURL) == "" {
		return dto.BridgeAgentSetupResponse{}, errors.New("enroll response is missing client_id or bridge_ws_url")
	}
	return envelope.Data, nil
}

func requestBridgeAgentSetupTokenConsume(ctx context.Context, baseURL string, setupToken string, payload dto.BridgeAgentSetupRequest, opts EnrollOptions) (dto.BridgeAgentSetupResponse, error) {
	consumePayload := dto.BridgeAgentSetupTokenConsumeRequest{
		SetupToken: strings.TrimSpace(setupToken),
		ClientId:   strings.TrimSpace(payload.ClientId),
		Rotate:     payload.Rotate,
		ClientName: strings.TrimSpace(payload.ClientName),
		Version:    strings.TrimSpace(payload.Version),
		Platform:   strings.TrimSpace(payload.Platform),
		Workspace:  strings.TrimSpace(payload.Workspace),
	}
	bytesPayload, err := json.Marshal(consumePayload)
	if err != nil {
		return dto.BridgeAgentSetupResponse{}, err
	}
	endpoint := strings.TrimRight(baseURL, "/") + "/api/bridge/agent-setup/consume"
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = DefaultEnrollTimeout
	}
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, endpoint, bytes.NewReader(bytesPayload))
	if err != nil {
		return dto.BridgeAgentSetupResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "data-proxy-agent/"+strings.TrimSpace(opts.Version))

	client := opts.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: timeout}
	}
	resp, err := client.Do(req)
	if err != nil {
		return dto.BridgeAgentSetupResponse{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return dto.BridgeAgentSetupResponse{}, err
	}
	var envelope apiEnvelope[dto.BridgeAgentSetupResponse]
	if err := json.Unmarshal(body, &envelope); err != nil {
		return dto.BridgeAgentSetupResponse{}, fmt.Errorf("failed to decode enroll response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if envelope.Message != "" {
			return dto.BridgeAgentSetupResponse{}, fmt.Errorf("enroll failed: HTTP %d: %s", resp.StatusCode, envelope.Message)
		}
		return dto.BridgeAgentSetupResponse{}, fmt.Errorf("enroll failed: HTTP %d", resp.StatusCode)
	}
	if !envelope.Success {
		if envelope.Message == "" {
			envelope.Message = "server returned success=false"
		}
		return dto.BridgeAgentSetupResponse{}, errors.New("enroll failed: " + envelope.Message)
	}
	if strings.TrimSpace(envelope.Data.ClientId) == "" || strings.TrimSpace(envelope.Data.BridgeWSURL) == "" {
		return dto.BridgeAgentSetupResponse{}, errors.New("enroll response is missing client_id or bridge_ws_url")
	}
	return envelope.Data, nil
}

func envInt(name string) int {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return 0
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return parsed
}

func redactStringMap(values map[string]string) map[string]string {
	if values == nil {
		return nil
	}
	redacted := make(map[string]string, len(values))
	for key, value := range values {
		if isSensitiveKey(key) {
			redacted[key] = redactSecret(value)
			continue
		}
		redacted[key] = value
	}
	return redacted
}

func redactAnyMap(values map[string]any) map[string]any {
	if values == nil {
		return nil
	}
	redacted := make(map[string]any, len(values))
	for key, value := range values {
		if isSensitiveKey(key) {
			if text, ok := value.(string); ok {
				redacted[key] = redactSecret(text)
			} else {
				redacted[key] = "***"
			}
			continue
		}
		switch nested := value.(type) {
		case map[string]any:
			redacted[key] = redactAnyMap(nested)
		case map[string]string:
			redacted[key] = redactStringMap(nested)
		default:
			redacted[key] = value
		}
	}
	return redacted
}

func isSensitiveKey(key string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	return strings.Contains(key, "token") ||
		strings.Contains(key, "key") ||
		strings.Contains(key, "secret") ||
		strings.Contains(key, "authorization")
}
