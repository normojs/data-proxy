package oauth

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
)

func init() {
	Register("hstation", &HStationProvider{})
}

// HStationProvider implements OAuth for the dc.hhhl.cc bridge.
type HStationProvider struct{}

type hStationOAuthResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
	Error        string `json:"error"`
	ErrorDesc    string `json:"error_description"`
}

type hStationUser struct {
	Id                string `json:"id"`
	Sub               string `json:"sub"`
	Username          string `json:"username"`
	PreferredUsername string `json:"preferred_username"`
	Name              string `json:"name"`
	DisplayName       string `json:"displayName"`
	Email             string `json:"email"`
}

func (p *HStationProvider) GetName() string {
	return "H 站"
}

func (p *HStationProvider) IsEnabled() bool {
	return common.HStationOAuthEnabled
}

func (p *HStationProvider) ExchangeToken(ctx context.Context, code string, c *gin.Context) (*OAuthToken, error) {
	if code == "" {
		return nil, NewOAuthError(i18n.MsgOAuthInvalidCode, nil)
	}

	logger.LogDebug(ctx, "[OAuth-HStation] ExchangeToken: code=%s...", code[:min(len(code), 10)])

	redirectUri := fmt.Sprintf("%s/oauth/hstation", system_setting.ServerAddress)
	values := url.Values{}
	values.Set("client_id", common.HStationClientId)
	values.Set("client_secret", common.HStationClientSecret)
	values.Set("code", code)
	values.Set("grant_type", "authorization_code")
	values.Set("redirect_uri", redirectUri)

	req, err := http.NewRequestWithContext(ctx, "POST", common.HStationTokenEndpoint, strings.NewReader(values.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	logger.LogDebug(ctx, "[OAuth-HStation] ExchangeToken: token_endpoint=%s, redirect_uri=%s", common.HStationTokenEndpoint, redirectUri)

	client := http.Client{Timeout: 20 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		logger.LogError(ctx, fmt.Sprintf("[OAuth-HStation] ExchangeToken error: %s", err.Error()))
		return nil, NewOAuthErrorWithRaw(i18n.MsgOAuthConnectFailed, map[string]any{"Provider": p.GetName()}, err.Error())
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		logger.LogError(ctx, fmt.Sprintf("[OAuth-HStation] ExchangeToken read body error: %s", err.Error()))
		return nil, err
	}

	var tokenResponse hStationOAuthResponse
	if err := common.Unmarshal(body, &tokenResponse); err != nil {
		logger.LogError(ctx, fmt.Sprintf("[OAuth-HStation] ExchangeToken decode error: %s", err.Error()))
		return nil, err
	}

	if tokenResponse.Error != "" {
		logger.LogError(ctx, fmt.Sprintf("[OAuth-HStation] ExchangeToken OAuth error: %s - %s", tokenResponse.Error, tokenResponse.ErrorDesc))
		return nil, NewOAuthErrorWithRaw(i18n.MsgOAuthTokenFailed, map[string]any{"Provider": p.GetName()}, tokenResponse.ErrorDesc)
	}

	if tokenResponse.AccessToken == "" {
		logger.LogError(ctx, "[OAuth-HStation] ExchangeToken failed: empty access token")
		return nil, NewOAuthError(i18n.MsgOAuthTokenFailed, map[string]any{"Provider": p.GetName()})
	}

	logger.LogDebug(ctx, "[OAuth-HStation] ExchangeToken success: scope=%s", tokenResponse.Scope)

	return &OAuthToken{
		AccessToken:  tokenResponse.AccessToken,
		TokenType:    tokenResponse.TokenType,
		RefreshToken: tokenResponse.RefreshToken,
		ExpiresIn:    tokenResponse.ExpiresIn,
		Scope:        tokenResponse.Scope,
	}, nil
}

func (p *HStationProvider) GetUserInfo(ctx context.Context, token *OAuthToken) (*OAuthUser, error) {
	logger.LogDebug(ctx, "[OAuth-HStation] GetUserInfo: userinfo_endpoint=%s", common.HStationUserInfoEndpoint)

	req, err := http.NewRequestWithContext(ctx, "GET", common.HStationUserInfoEndpoint, nil)
	if err != nil {
		return nil, err
	}
	tokenType := normalizeAuthorizationTokenType(token.TokenType)
	req.Header.Set("Authorization", fmt.Sprintf("%s %s", tokenType, token.AccessToken))
	req.Header.Set("Accept", "application/json")

	client := http.Client{Timeout: 20 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		logger.LogError(ctx, fmt.Sprintf("[OAuth-HStation] GetUserInfo error: %s", err.Error()))
		return nil, NewOAuthErrorWithRaw(i18n.MsgOAuthConnectFailed, map[string]any{"Provider": p.GetName()}, err.Error())
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		logger.LogError(ctx, fmt.Sprintf("[OAuth-HStation] GetUserInfo failed: status=%d", res.StatusCode))
		return nil, NewOAuthErrorWithRaw(i18n.MsgOAuthGetUserErr, map[string]any{"Provider": p.GetName()}, fmt.Sprintf("status %d", res.StatusCode))
	}

	var user hStationUser
	if err := common.DecodeJson(res.Body, &user); err != nil {
		logger.LogError(ctx, fmt.Sprintf("[OAuth-HStation] GetUserInfo decode error: %s", err.Error()))
		return nil, err
	}

	providerUserID := firstNonEmpty(user.Sub, user.Id)
	username := firstNonEmpty(user.Username, user.PreferredUsername, user.Name, providerUserID)
	displayName := firstNonEmpty(user.Name, user.DisplayName, username)
	username = trimToMaxRunes(username, model.UserNameMaxLength)
	displayName = trimToMaxRunes(displayName, model.UserNameMaxLength)
	if providerUserID == "" {
		logger.LogError(ctx, "[OAuth-HStation] GetUserInfo failed: empty user id")
		return nil, NewOAuthError(i18n.MsgOAuthUserInfoEmpty, map[string]any{"Provider": p.GetName()})
	}

	logger.LogDebug(ctx, "[OAuth-HStation] GetUserInfo success: id=%s, username=%s, name=%s", providerUserID, username, displayName)

	return &OAuthUser{
		ProviderUserID: providerUserID,
		Username:       username,
		DisplayName:    displayName,
		Email:          user.Email,
	}, nil
}

func (p *HStationProvider) IsUserIDTaken(providerUserID string) bool {
	return model.IsHStationIdAlreadyTaken(providerUserID)
}

func (p *HStationProvider) FillUserByProviderID(user *model.User, providerUserID string) error {
	user.HStationId = providerUserID
	return user.FillUserByHStationId()
}

func (p *HStationProvider) SetProviderUserID(user *model.User, providerUserID string) {
	user.HStationId = providerUserID
}

func (p *HStationProvider) GetProviderPrefix() string {
	return "hstation_"
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func trimToMaxRunes(value string, maxLength int) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= maxLength {
		return string(runes)
	}
	return string(runes[:maxLength])
}
