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
package service

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
)

const (
	connectedAppAuthCodeTTLSeconds = 600
	connectedAppOIDCKeyFile        = "oidc_rsa.pem"
	connectedAppOIDCKeyID          = "dp-oidc-1"
)

var (
	oidcKeyOnce sync.Once
	oidcKey     *rsa.PrivateKey
	oidcKeyErr  error
)

func HashConnectedAppOAuthValue(raw string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(raw)))
	return hex.EncodeToString(sum[:])
}

func VerifyPKCES256(codeVerifier, codeChallenge string) bool {
	codeVerifier = strings.TrimSpace(codeVerifier)
	codeChallenge = strings.TrimSpace(codeChallenge)
	if codeVerifier == "" || codeChallenge == "" {
		return false
	}
	sum := sha256.Sum256([]byte(codeVerifier))
	encoded := base64.RawURLEncoding.EncodeToString(sum[:])
	return encoded == codeChallenge
}

func ConnectedAppIssuer(c *gin.Context) string {
	base := strings.TrimRight(strings.TrimSpace(system_setting.ServerAddress), "/")
	if base == "" && c != nil && c.Request != nil {
		scheme := "http"
		if c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") {
			scheme = "https"
		}
		base = scheme + "://" + c.Request.Host
	}
	if base == "" {
		base = "http://localhost:3000"
	}
	return base
}

func EnsureConnectedAppOIDCKey() (*rsa.PrivateKey, error) {
	oidcKeyOnce.Do(func() {
		path := filepath.Join("data", connectedAppOIDCKeyFile)
		if key, err := loadRSAPrivateKey(path); err == nil {
			oidcKey = key
			return
		}
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			oidcKeyErr = err
			return
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			oidcKeyErr = err
			return
		}
		if err := saveRSAPrivateKey(path, key); err != nil {
			oidcKeyErr = err
			return
		}
		oidcKey = key
	})
	return oidcKey, oidcKeyErr
}

func loadRSAPrivateKey(path string) (*rsa.PrivateKey, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, errors.New("invalid pem")
	}
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err == nil {
		return key, nil
	}
	parsed, err2 := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err2 != nil {
		return nil, err
	}
	rsaKey, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("not rsa private key")
	}
	return rsaKey, nil
}

func saveRSAPrivateKey(path string, key *rsa.PrivateKey) error {
	block := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}
	return os.WriteFile(path, pem.EncodeToMemory(block), 0o600)
}

func MintConnectedAppIDToken(c *gin.Context, app *model.ConnectedApp, user *model.User, nonce string) (string, error) {
	if app == nil || user == nil {
		return "", errors.New("app and user required")
	}
	key, err := EnsureConnectedAppOIDCKey()
	if err != nil {
		return "", err
	}
	now := time.Now()
	claims := jwt.MapClaims{
		"iss": ConnectedAppIssuer(c),
		"sub": strconv.Itoa(user.Id),
		"aud": app.PublicClientID(),
		"iat": now.Unix(),
		"exp": now.Add(time.Hour).Unix(),
	}
	if name := strings.TrimSpace(user.DisplayName); name != "" {
		claims["name"] = name
	}
	if username := strings.TrimSpace(user.Username); username != "" {
		claims["preferred_username"] = username
	}
	if email := strings.TrimSpace(user.Email); email != "" {
		claims["email"] = email
	}
	if nonce = strings.TrimSpace(nonce); nonce != "" {
		claims["nonce"] = nonce
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = connectedAppOIDCKeyID
	return token.SignedString(key)
}

func ConnectedAppJWKS() (map[string]any, error) {
	key, err := EnsureConnectedAppOIDCKey()
	if err != nil {
		return nil, err
	}
	pub := key.PublicKey
	n := base64.RawURLEncoding.EncodeToString(pub.N.Bytes())
	e := base64.RawURLEncoding.EncodeToString(bigIntBytes(pub.E))
	return map[string]any{
		"keys": []map[string]string{
			{
				"kty": "RSA",
				"use": "sig",
				"alg": "RS256",
				"kid": connectedAppOIDCKeyID,
				"n":   n,
				"e":   e,
			},
		},
	}, nil
}

func bigIntBytes(v int) []byte {
	if v == 0 {
		return []byte{0}
	}
	var b []byte
	for v > 0 {
		b = append([]byte{byte(v & 0xff)}, b...)
		v >>= 8
	}
	return b
}

func IntersectConnectedAppScopes(allowed []string, requested []string) []string {
	if len(requested) == 0 {
		return model.NormalizeConnectedAppScopes(allowed)
	}
	allowedSet := map[string]struct{}{}
	for _, scope := range allowed {
		allowedSet[scope] = struct{}{}
	}
	out := make([]string, 0, len(requested))
	seen := map[string]struct{}{}
	for _, scope := range requested {
		scope = strings.TrimSpace(scope)
		if scope == "" {
			continue
		}
		if _, ok := allowedSet[scope]; !ok {
			continue
		}
		if _, ok := seen[scope]; ok {
			continue
		}
		seen[scope] = struct{}{}
		out = append(out, scope)
	}
	return out
}

// IssueConnectedAppAPIKey creates grant + platform token + binding and returns plaintext key once.
func IssueConnectedAppAPIKey(tx *gorm.DB, app *model.ConnectedApp, userId int, scopes []string, deviceName, platform, fingerprint string) (string, *model.Token, *model.ConnectedAppGrant, error) {
	if tx == nil {
		tx = model.DB
	}
	if app == nil || userId <= 0 {
		return "", nil, nil, errors.New("app and user required")
	}
	if fingerprint == "" {
		fingerprint = "oauth-web"
	}
	now := common.GetTimestamp()
	if len(scopes) == 0 {
		scopes = app.DefaultScopeList()
	}
	scopes = model.NormalizeConnectedAppScopes(scopes)
	grant, err := model.UpsertConnectedAppGrant(tx, *app, userId, scopes, now)
	if err != nil {
		return "", nil, nil, err
	}
	key, err := common.GenerateKey()
	if err != nil {
		return "", nil, nil, err
	}
	token := &model.Token{
		UserId:         userId,
		Name:           limitConnectedAppName(app.Name+" OAuth", 50),
		Key:            key,
		Status:         common.TokenStatusEnabled,
		CreatedTime:    now,
		AccessedTime:   now,
		ExpiredTime:    -1,
		UnlimitedQuota: true,
	}
	if err := tx.Create(token).Error; err != nil {
		return "", nil, nil, err
	}
	binding := model.ConnectedAppTokenBinding{
		AppId:             app.Id,
		GrantId:           grant.Id,
		UserId:            userId,
		TokenId:           token.Id,
		DeviceFingerprint: fingerprint,
		DeviceName:        deviceName,
		Platform:          platform,
		Status:            model.ConnectedAppTokenBindingStatusActive,
		LastUsedAt:        now,
	}
	storedBinding, err := model.UpsertConnectedAppTokenBinding(tx, binding, now)
	if err != nil {
		return "", nil, nil, err
	}
	_ = model.RecordConnectedAppTokenAttribution(tx, *storedBinding, now)
	return "sk-" + key, token, grant, nil
}

func limitConnectedAppName(name string, max int) string {
	name = strings.TrimSpace(name)
	if max <= 0 || len(name) <= max {
		return name
	}
	return name[:max]
}

func CreateConnectedAppAuthCodeRecord(tx *gorm.DB, app *model.ConnectedApp, userId int, grantId int64, redirectURI, scopes, state, nonce, challenge, method string) (string, error) {
	raw, err := common.GenerateRandomCharsKey(48)
	if err != nil {
		return "", err
	}
	now := common.GetTimestamp()
	code := &model.ConnectedAppAuthCode{
		CodeHash:            HashConnectedAppOAuthValue(raw),
		AppId:               app.Id,
		UserId:              userId,
		GrantId:             grantId,
		RedirectURI:         redirectURI,
		Scopes:              scopes,
		State:               state,
		Nonce:               nonce,
		CodeChallenge:       challenge,
		CodeChallengeMethod: method,
		Status:              model.ConnectedAppAuthCodeStatusPending,
		ExpiresAt:           now + connectedAppAuthCodeTTLSeconds,
	}
	if err := model.CreateConnectedAppAuthCode(tx, code); err != nil {
		return "", err
	}
	return raw, nil
}

func RedirectURIAllowed(app *model.ConnectedApp, redirectURI string) bool {
	redirectURI = strings.TrimSpace(redirectURI)
	if app == nil || redirectURI == "" {
		return false
	}
	for _, allowed := range app.RedirectURIList() {
		if allowed == redirectURI {
			return true
		}
	}
	return false
}

func ParseScopeQuery(raw string) []string {
	return model.NormalizeConnectedAppScopes(strings.Fields(raw))
}

func OpenIDConfiguration(c *gin.Context) map[string]any {
	issuer := ConnectedAppIssuer(c)
	return map[string]any{
		"issuer":                                issuer,
		"authorization_endpoint":                issuer + "/oauth/authorize",
		"token_endpoint":                        issuer + "/api/oauth/token",
		"userinfo_endpoint":                     issuer + "/api/oauth/userinfo",
		"jwks_uri":                              issuer + "/oauth/jwks.json",
		"response_types_supported":              []string{"code"},
		"subject_types_supported":               []string{"public"},
		"id_token_signing_alg_values_supported": []string{"RS256"},
		"scopes_supported": []string{
			"openid", "profile", "email",
			"openai.models", "openai.chat", "quota.read", "token.manage",
		},
		"token_endpoint_auth_methods_supported": []string{"none", "client_secret_post"},
		"code_challenge_methods_supported":      []string{"S256"},
		"grant_types_supported":                 []string{"authorization_code"},
	}
}

