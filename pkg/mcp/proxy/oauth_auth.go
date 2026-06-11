package proxy

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/mcp/secretref"
)

const oauthRefreshSkewSeconds int64 = 60

type oauthSecretConfig struct {
	AccessToken  string   `json:"access_token"`
	TokenType    string   `json:"token_type"`
	RefreshToken string   `json:"refresh_token"`
	TokenURL     string   `json:"token_url"`
	ClientID     string   `json:"client_id"`
	ClientSecret string   `json:"client_secret"`
	Scope        string   `json:"scope"`
	Scopes       []string `json:"scopes"`
	ExpiresAt    any      `json:"expires_at"`
}

type oauthCachedToken struct {
	AccessToken  string
	TokenType    string
	RefreshToken string
	ExpiresAt    int64
}

type oauthTokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
	ExpiresAt    any    `json:"expires_at"`
}

func applyHTTPAuth(req *http.Request, server model.MCPProxyServer) error {
	return applyHTTPAuthWithClient(req.Context(), http.DefaultClient, nil, req, server)
}

func (c *HTTPClient) applyHTTPAuth(ctx context.Context, req *http.Request, server model.MCPProxyServer) error {
	if c == nil {
		return applyHTTPAuthWithClient(ctx, http.DefaultClient, nil, req, server)
	}
	return applyHTTPAuthWithClient(ctx, c.Client, c, req, server)
}

func applyHTTPAuthWithClient(ctx context.Context, client *http.Client, cache *HTTPClient, req *http.Request, server model.MCPProxyServer) error {
	authRef := strings.TrimSpace(server.AuthRef)
	if authRef == "" || server.AuthType == model.MCPProxyAuthTypeNone {
		return nil
	}
	secret, err := secretref.ResolveEnv(authRef, "mcp proxy auth")
	if err != nil {
		return err
	}
	switch server.AuthType {
	case model.MCPProxyAuthTypeBearer:
		req.Header.Set("Authorization", "Bearer "+secret)
	case model.MCPProxyAuthTypeHeader:
		name, value, ok := strings.Cut(secret, "=")
		if !ok || strings.TrimSpace(name) == "" {
			return errors.New("header auth secret must use Header-Name=value")
		}
		req.Header.Set(strings.TrimSpace(name), strings.TrimSpace(value))
	case model.MCPProxyAuthTypeBasic:
		username, password, ok := strings.Cut(secret, ":")
		if !ok {
			return errors.New("basic auth secret must use username:password")
		}
		req.SetBasicAuth(username, password)
	case model.MCPProxyAuthTypeOAuth:
		token, err := resolveOAuthAccessToken(ctx, client, cache, authRef, secret)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", token.authorizationHeader())
	default:
		return fmt.Errorf("unsupported mcp proxy auth_type: %s", server.AuthType)
	}
	return nil
}

func resolveOAuthAccessToken(ctx context.Context, client *http.Client, cache *HTTPClient, authRef string, rawSecret string) (oauthCachedToken, error) {
	config, err := parseOAuthSecretConfig(rawSecret)
	if err != nil {
		return oauthCachedToken{}, err
	}
	cacheKey := oauthCacheKey(authRef, rawSecret)
	now := time.Now().Unix()
	if cache != nil {
		cache.oauthMu.Lock()
		cached := cache.oauthTokens[cacheKey]
		if cached.valid(now) {
			cache.oauthMu.Unlock()
			return cached, nil
		}
		cache.oauthMu.Unlock()
	}

	token := oauthCachedToken{
		AccessToken:  strings.TrimSpace(config.AccessToken),
		TokenType:    normalizeOAuthTokenType(config.TokenType),
		RefreshToken: strings.TrimSpace(config.RefreshToken),
		ExpiresAt:    parseOAuthExpiresAt(config.ExpiresAt, now),
	}
	if token.valid(now) || (token.AccessToken != "" && token.ExpiresAt == 0) {
		storeOAuthToken(cache, cacheKey, token)
		return token, nil
	}
	if token.RefreshToken == "" || strings.TrimSpace(config.TokenURL) == "" {
		return oauthCachedToken{}, errors.New("oauth auth access token is expired and refresh credentials are incomplete")
	}
	refreshed, err := refreshOAuthAccessToken(ctx, client, config, token.RefreshToken)
	if err != nil {
		return oauthCachedToken{}, err
	}
	if refreshed.RefreshToken == "" {
		refreshed.RefreshToken = token.RefreshToken
	}
	storeOAuthToken(cache, cacheKey, refreshed)
	return refreshed, nil
}

func parseOAuthSecretConfig(rawSecret string) (oauthSecretConfig, error) {
	var config oauthSecretConfig
	if err := common.UnmarshalJsonStr(strings.TrimSpace(rawSecret), &config); err != nil {
		return oauthSecretConfig{}, errors.New("oauth auth secret must be a JSON object")
	}
	config.AccessToken = strings.TrimSpace(config.AccessToken)
	config.TokenType = normalizeOAuthTokenType(config.TokenType)
	config.RefreshToken = strings.TrimSpace(config.RefreshToken)
	config.TokenURL = strings.TrimSpace(config.TokenURL)
	config.ClientID = strings.TrimSpace(config.ClientID)
	config.ClientSecret = strings.TrimSpace(config.ClientSecret)
	config.Scope = strings.TrimSpace(config.Scope)
	return config, nil
}

func refreshOAuthAccessToken(ctx context.Context, client *http.Client, config oauthSecretConfig, refreshToken string) (oauthCachedToken, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if client == nil {
		client = http.DefaultClient
	}
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refreshToken)
	if config.ClientID != "" {
		form.Set("client_id", config.ClientID)
	}
	if config.ClientSecret != "" {
		form.Set("client_secret", config.ClientSecret)
	}
	if scope := oauthScope(config); scope != "" {
		form.Set("scope", scope)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, config.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return oauthCachedToken{}, errors.New("oauth auth token_url is invalid")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := client.Do(req)
	if err != nil {
		return oauthCachedToken{}, errors.New("oauth auth token refresh request failed")
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return oauthCachedToken{}, errors.New("oauth auth token refresh response could not be read")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return oauthCachedToken{}, fmt.Errorf("oauth auth token refresh failed with status %d", resp.StatusCode)
	}
	var tokenResponse oauthTokenResponse
	if err := common.Unmarshal(body, &tokenResponse); err != nil {
		return oauthCachedToken{}, errors.New("oauth auth token refresh response is invalid JSON")
	}
	accessToken := strings.TrimSpace(tokenResponse.AccessToken)
	if accessToken == "" {
		return oauthCachedToken{}, errors.New("oauth auth token refresh response missing access_token")
	}
	now := time.Now().Unix()
	expiresAt := parseOAuthExpiresAt(tokenResponse.ExpiresAt, now)
	if expiresAt == 0 && tokenResponse.ExpiresIn > 0 {
		expiresAt = now + tokenResponse.ExpiresIn
	}
	return oauthCachedToken{
		AccessToken:  accessToken,
		TokenType:    normalizeOAuthTokenType(tokenResponse.TokenType),
		RefreshToken: strings.TrimSpace(tokenResponse.RefreshToken),
		ExpiresAt:    expiresAt,
	}, nil
}

func storeOAuthToken(cache *HTTPClient, cacheKey string, token oauthCachedToken) {
	if cache == nil || cacheKey == "" {
		return
	}
	cache.oauthMu.Lock()
	if cache.oauthTokens == nil {
		cache.oauthTokens = map[string]oauthCachedToken{}
	}
	cache.oauthTokens[cacheKey] = token
	cache.oauthMu.Unlock()
}

func (token oauthCachedToken) valid(now int64) bool {
	if strings.TrimSpace(token.AccessToken) == "" {
		return false
	}
	return token.ExpiresAt == 0 || token.ExpiresAt > now+oauthRefreshSkewSeconds
}

func (token oauthCachedToken) authorizationHeader() string {
	tokenType := normalizeOAuthTokenType(token.TokenType)
	return tokenType + " " + strings.TrimSpace(token.AccessToken)
}

func normalizeOAuthTokenType(tokenType string) string {
	tokenType = strings.TrimSpace(tokenType)
	if tokenType == "" {
		return "Bearer"
	}
	return tokenType
}

func oauthScope(config oauthSecretConfig) string {
	if config.Scope != "" {
		return config.Scope
	}
	if len(config.Scopes) == 0 {
		return ""
	}
	parts := make([]string, 0, len(config.Scopes))
	for _, scope := range config.Scopes {
		scope = strings.TrimSpace(scope)
		if scope != "" {
			parts = append(parts, scope)
		}
	}
	return strings.Join(parts, " ")
}

func oauthCacheKey(authRef string, rawSecret string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(authRef) + "\x00" + rawSecret))
	return hex.EncodeToString(sum[:])
}

func parseOAuthExpiresAt(value any, now int64) int64 {
	switch typed := value.(type) {
	case nil:
		return 0
	case float64:
		if typed <= 0 {
			return 0
		}
		return int64(typed)
	case int64:
		if typed <= 0 {
			return 0
		}
		return typed
	case string:
		typed = strings.TrimSpace(typed)
		if typed == "" {
			return 0
		}
		if parsed, err := time.Parse(time.RFC3339, typed); err == nil {
			return parsed.Unix()
		}
		if seconds := common.String2Int(typed); seconds > 0 {
			return int64(seconds)
		}
	}
	return now
}
