package oauth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/stretchr/testify/require"
)

func withHStationOAuthSettings(t *testing.T, serverURL string) {
	t.Helper()

	oldEnabled := common.HStationOAuthEnabled
	oldClientID := common.HStationClientId
	oldClientSecret := common.HStationClientSecret
	oldAuthorizationEndpoint := common.HStationAuthorizationEndpoint
	oldTokenEndpoint := common.HStationTokenEndpoint
	oldUserInfoEndpoint := common.HStationUserInfoEndpoint
	oldScopes := common.HStationScopes
	oldServerAddress := system_setting.ServerAddress

	common.HStationOAuthEnabled = true
	common.HStationClientId = "hstation-client"
	common.HStationClientSecret = "hstation-secret"
	common.HStationAuthorizationEndpoint = serverURL + "/authorize"
	common.HStationTokenEndpoint = serverURL + "/token"
	common.HStationUserInfoEndpoint = serverURL + "/userinfo"
	common.HStationScopes = "read:profile email"
	system_setting.ServerAddress = "https://data-proxy.example"

	t.Cleanup(func() {
		common.HStationOAuthEnabled = oldEnabled
		common.HStationClientId = oldClientID
		common.HStationClientSecret = oldClientSecret
		common.HStationAuthorizationEndpoint = oldAuthorizationEndpoint
		common.HStationTokenEndpoint = oldTokenEndpoint
		common.HStationUserInfoEndpoint = oldUserInfoEndpoint
		common.HStationScopes = oldScopes
		system_setting.ServerAddress = oldServerAddress
	})
}

func TestHStationExchangeTokenPostsExpectedOAuthForm(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/token", r.URL.Path)
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))
		require.NoError(t, r.ParseForm())
		require.Equal(t, "hstation-client", r.Form.Get("client_id"))
		require.Equal(t, "hstation-secret", r.Form.Get("client_secret"))
		require.Equal(t, "authorization-code", r.Form.Get("code"))
		require.Equal(t, "authorization_code", r.Form.Get("grant_type"))
		require.Equal(t, "https://data-proxy.example/oauth/hstation", r.Form.Get("redirect_uri"))

		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "access-token",
			"token_type":    "Bearer",
			"refresh_token": "refresh-token",
			"expires_in":    3600,
			"scope":         "read:profile email",
		}))
	}))
	defer server.Close()
	withHStationOAuthSettings(t, server.URL)

	token, err := (&HStationProvider{}).ExchangeToken(context.Background(), "authorization-code", nil)
	require.NoError(t, err)
	require.Equal(t, "access-token", token.AccessToken)
	require.Equal(t, "Bearer", token.TokenType)
	require.Equal(t, "refresh-token", token.RefreshToken)
	require.Equal(t, 3600, token.ExpiresIn)
	require.Equal(t, "read:profile email", token.Scope)
}

func TestHStationGetUserInfoMapsProviderUser(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/userinfo", r.URL.Path)
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "Bearer access-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"sub":                "hstation-user-1",
			"preferred_username": "station-user",
			"displayName":        "Station User",
			"email":              "station@example.com",
		}))
	}))
	defer server.Close()
	withHStationOAuthSettings(t, server.URL)

	user, err := (&HStationProvider{}).GetUserInfo(context.Background(), &OAuthToken{
		AccessToken: "access-token",
	})
	require.NoError(t, err)
	require.Equal(t, "hstation-user-1", user.ProviderUserID)
	require.Equal(t, "station-user", user.Username)
	require.Equal(t, "Station User", user.DisplayName)
	require.Equal(t, "station@example.com", user.Email)
}

func TestHStationGetUserInfoRequiresProviderUserID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"username": "station-user",
		}))
	}))
	defer server.Close()
	withHStationOAuthSettings(t, server.URL)

	user, err := (&HStationProvider{}).GetUserInfo(context.Background(), &OAuthToken{
		AccessToken: "access-token",
	})
	require.Nil(t, user)
	require.Error(t, err)
}
