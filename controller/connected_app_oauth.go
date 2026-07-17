/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
package controller

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type connectedAppOAuthConsentRequest struct {
	ClientID            string `json:"client_id"`
	RedirectURI         string `json:"redirect_uri"`
	Scope               string `json:"scope"`
	State               string `json:"state"`
	Nonce               string `json:"nonce"`
	CodeChallenge       string `json:"code_challenge"`
	CodeChallengeMethod string `json:"code_challenge_method"`
	Approve             *bool  `json:"approve"`
}

func GetConnectedAppOpenIDConfiguration(c *gin.Context) {
	c.JSON(http.StatusOK, service.OpenIDConfiguration(c))
}

func GetConnectedAppJWKS(c *gin.Context) {
	jwks, err := service.ConnectedAppJWKS()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, jwks)
}

func ValidateConnectedAppOAuthAuthorize(c *gin.Context) {
	clientID := strings.TrimSpace(c.Query("client_id"))
	redirectURI := strings.TrimSpace(c.Query("redirect_uri"))
	responseType := strings.TrimSpace(c.Query("response_type"))
	scope := strings.TrimSpace(c.Query("scope"))
	state := strings.TrimSpace(c.Query("state"))
	nonce := strings.TrimSpace(c.Query("nonce"))
	challenge := strings.TrimSpace(c.Query("code_challenge"))
	method := strings.TrimSpace(c.Query("code_challenge_method"))
	if method == "" {
		method = "S256"
	}

	app, err := model.GetConnectedAppByClientID(clientID)
	if err != nil {
		common.ApiErrorMsg(c, "invalid client_id")
		return
	}
	if app.Status != model.ConnectedAppStatusEnabled {
		common.ApiErrorMsg(c, "client is disabled")
		return
	}
	if !app.SupportsAuthorizationCode() {
		common.ApiErrorMsg(c, "client does not support authorization_code")
		return
	}
	if responseType != "code" {
		common.ApiErrorMsg(c, "unsupported response_type")
		return
	}
	if !service.RedirectURIAllowed(app, redirectURI) {
		common.ApiErrorMsg(c, "redirect_uri is not registered")
		return
	}
	if challenge == "" || !strings.EqualFold(method, "S256") {
		common.ApiErrorMsg(c, "PKCE S256 code_challenge is required")
		return
	}
	requested := service.ParseScopeQuery(scope)
	// openid is identity-only; API scopes come from app defaults if omitted
	apiScopes := make([]string, 0, len(requested))
	for _, s := range requested {
		if s == "openid" || s == "profile" || s == "email" {
			continue
		}
		apiScopes = append(apiScopes, s)
	}
	granted := service.IntersectConnectedAppScopes(app.ScopeList(), apiScopes)
	if len(granted) == 0 {
		granted = app.DefaultScopeList()
	}
	common.ApiSuccess(c, gin.H{
		"client_id":             app.PublicClientID(),
		"app":                   gin.H{"id": app.Id, "slug": app.Slug, "name": app.Name, "description": app.Description, "trusted": app.Trusted},
		"redirect_uri":          redirectURI,
		"scope":                 strings.Join(granted, " "),
		"scopes":                granted,
		"state":                 state,
		"nonce":                 nonce,
		"code_challenge":        challenge,
		"code_challenge_method": method,
	})
}

func ConsentConnectedAppOAuth(c *gin.Context) {
	var req connectedAppOAuthConsentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	userId := c.GetInt("id")
	if userId <= 0 {
		common.ApiErrorMsg(c, "login required")
		return
	}
	approve := req.Approve == nil || *req.Approve
	app, err := model.GetConnectedAppByClientID(req.ClientID)
	if err != nil {
		common.ApiErrorMsg(c, "invalid client_id")
		return
	}
	if !service.RedirectURIAllowed(app, req.RedirectURI) {
		common.ApiErrorMsg(c, "redirect_uri is not registered")
		return
	}
	if !approve {
		u, _ := url.Parse(req.RedirectURI)
		q := u.Query()
		q.Set("error", "access_denied")
		if req.State != "" {
			q.Set("state", req.State)
		}
		u.RawQuery = q.Encode()
		common.ApiSuccess(c, gin.H{"redirect_to": u.String()})
		return
	}
	if strings.TrimSpace(req.CodeChallenge) == "" || !strings.EqualFold(strings.TrimSpace(req.CodeChallengeMethod), "S256") {
		common.ApiErrorMsg(c, "PKCE S256 code_challenge is required")
		return
	}
	requested := service.ParseScopeQuery(req.Scope)
	apiScopes := make([]string, 0, len(requested))
	for _, s := range requested {
		if s == "openid" || s == "profile" || s == "email" {
			continue
		}
		apiScopes = append(apiScopes, s)
	}
	granted := service.IntersectConnectedAppScopes(app.ScopeList(), apiScopes)
	if len(granted) == 0 {
		granted = app.DefaultScopeList()
	}
	now := common.GetTimestamp()
	grant, err := model.UpsertConnectedAppGrant(model.DB, *app, userId, granted, now)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	code, err := service.CreateConnectedAppAuthCodeRecord(
		model.DB,
		app,
		userId,
		grant.Id,
		req.RedirectURI,
		strings.Join(granted, " "),
		req.State,
		req.Nonce,
		req.CodeChallenge,
		"S256",
	)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	u, err := url.Parse(req.RedirectURI)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	q := u.Query()
	q.Set("code", code)
	if req.State != "" {
		q.Set("state", req.State)
	}
	u.RawQuery = q.Encode()
	common.ApiSuccess(c, gin.H{"redirect_to": u.String(), "code_issued": true})
}

func ExchangeConnectedAppOAuthToken(c *gin.Context) {
	grantType := firstNonEmptyForm(c, "grant_type")
	if grantType != "authorization_code" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported_grant_type"})
		return
	}
	code := firstNonEmptyForm(c, "code")
	redirectURI := firstNonEmptyForm(c, "redirect_uri")
	clientID := firstNonEmptyForm(c, "client_id")
	clientSecret := firstNonEmptyForm(c, "client_secret")
	codeVerifier := firstNonEmptyForm(c, "code_verifier")
	if code == "" || redirectURI == "" || clientID == "" || codeVerifier == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
		return
	}
	app, err := model.GetConnectedAppByClientID(clientID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_client"})
		return
	}
	if app.ClientSecretHash != "" {
		if service.HashConnectedAppOAuthValue(clientSecret) != app.ClientSecretHash {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_client"})
			return
		}
	}
	if !service.RedirectURIAllowed(app, redirectURI) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_grant"})
		return
	}
	now := common.GetTimestamp()
	var (
		apiKey string
		grant  *model.ConnectedAppGrant
		user   *model.User
		nonce  string
	)
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		authCode, err := model.ConsumeConnectedAppAuthCode(tx, service.HashConnectedAppOAuthValue(code), now)
		if err != nil {
			return err
		}
		if authCode.AppId != app.Id || authCode.RedirectURI != redirectURI {
			return errors.New("invalid_grant")
		}
		if !service.VerifyPKCES256(codeVerifier, authCode.CodeChallenge) {
			return errors.New("invalid_grant")
		}
		nonce = authCode.Nonce
		scopes := authCode.ScopeList()
		key, _, g, err := service.IssueConnectedAppAPIKey(tx, app, authCode.UserId, scopes, "OAuth Web", "web", "oauth-web")
		if err != nil {
			return err
		}
		apiKey = key
		grant = g
		u, err := model.GetUserById(authCode.UserId, false)
		if err != nil {
			return err
		}
		user = u
		return nil
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_grant", "error_description": err.Error()})
		return
	}
	scope := ""
	if grant != nil {
		scope = strings.Join(grant.ScopeList(), " ")
	}
	resp := gin.H{
		"access_token":  apiKey,
		"token_type":    "Bearer",
		"scope":         scope,
		"api_key":       apiKey,
		"api_key_once":  true,
		"expires_in":    nil,
	}
	if idToken, err := service.MintConnectedAppIDToken(c, app, user, nonce); err == nil {
		resp["id_token"] = idToken
	}
	c.JSON(http.StatusOK, resp)
}

func GetConnectedAppOAuthUserInfo(c *gin.Context) {
	// Prefer session user if present; otherwise resolve by Bearer sk- token.
	userId := c.GetInt("id")
	if userId <= 0 {
		auth := strings.TrimSpace(c.GetHeader("Authorization"))
		raw := strings.TrimPrefix(auth, "Bearer ")
		raw = strings.TrimPrefix(raw, "sk-")
		if raw == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_token"})
			return
		}
		token, err := model.GetTokenByKey(raw, false)
		if err != nil || token == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_token"})
			return
		}
		userId = token.UserId
	}
	user, err := model.GetUserById(userId, false)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_token"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"sub":                user.Id,
		"name":               user.DisplayName,
		"preferred_username": user.Username,
		"email":              user.Email,
	})
}

func firstNonEmptyForm(c *gin.Context, key string) string {
	if v := strings.TrimSpace(c.PostForm(key)); v != "" {
		return v
	}
	if v := strings.TrimSpace(c.Query(key)); v != "" {
		return v
	}
	return ""
}
