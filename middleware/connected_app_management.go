package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	ContextConnectedApp             = "connected_app"
	ContextConnectedAppGrant        = "connected_app_grant"
	ContextConnectedAppAccessToken  = "connected_app_access_token"
	ContextConnectedAppDevice       = "connected_app_device_fingerprint"
	ContextConnectedAppManagementID = "connected_app_management_token_id"
)

func ConnectedAppManagementAuth(requiredScopes ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		rawToken := bearerToken(c.GetHeader("Authorization"))
		if rawToken == "" || !strings.HasPrefix(rawToken, "cdpat_") {
			abortConnectedAppManagement(c, http.StatusUnauthorized, "invalid_management_token", "management token is required")
			return
		}

		accessToken, err := model.GetConnectedAppAccessTokenByHash(connectedAppManagementTokenHash(rawToken))
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				abortConnectedAppManagement(c, http.StatusUnauthorized, "invalid_management_token", "management token is invalid")
				return
			}
			abortConnectedAppManagement(c, http.StatusInternalServerError, "internal_error", "failed to validate management token")
			return
		}
		now := common.GetTimestamp()
		if accessToken.Status != model.ConnectedAppAccessTokenStatusActive || accessToken.ExpiresAt <= now {
			abortConnectedAppManagement(c, http.StatusUnauthorized, "invalid_management_token", "management token is expired or revoked")
			return
		}

		app, err := model.GetConnectedAppByID(accessToken.AppId)
		if err != nil {
			abortConnectedAppManagement(c, http.StatusForbidden, "app_not_authorized", "connected app is unavailable")
			return
		}
		if app.Status != model.ConnectedAppStatusEnabled {
			abortConnectedAppManagement(c, http.StatusForbidden, "app_not_authorized", "connected app is disabled")
			return
		}
		if slug := strings.TrimSpace(c.Param("slug")); slug != "" && slug != app.Slug {
			abortConnectedAppManagement(c, http.StatusForbidden, "app_not_authorized", "management token does not belong to this connected app")
			return
		}

		grant, err := model.GetConnectedAppGrant(accessToken.AppId, accessToken.UserId)
		if err != nil || grant.Id != accessToken.GrantId || grant.Status != model.ConnectedAppGrantStatusAuthorized {
			abortConnectedAppManagement(c, http.StatusForbidden, "app_not_authorized", "connected app grant is not authorized")
			return
		}
		for _, requiredScope := range requiredScopes {
			if !model.ConnectedAppHasScope(accessToken.ScopeList(), requiredScope) || !model.ConnectedAppHasScope(grant.ScopeList(), requiredScope) {
				abortConnectedAppManagement(c, http.StatusForbidden, "insufficient_scope", "management token scope is insufficient")
				return
			}
		}

		_ = model.TouchConnectedAppAccessToken(accessToken.Id, now)
		c.Set(ContextConnectedApp, app)
		c.Set(ContextConnectedAppGrant, grant)
		c.Set(ContextConnectedAppAccessToken, accessToken)
		c.Set(ContextConnectedAppDevice, accessToken.DeviceFingerprint)
		c.Set(ContextConnectedAppManagementID, accessToken.Id)
		c.Set("id", accessToken.UserId)
		c.Next()
	}
}

func GetConnectedAppManagementContext(c *gin.Context) (*model.ConnectedApp, *model.ConnectedAppGrant, *model.ConnectedAppAccessToken, bool) {
	app, appOK := c.Get(ContextConnectedApp)
	grant, grantOK := c.Get(ContextConnectedAppGrant)
	accessToken, tokenOK := c.Get(ContextConnectedAppAccessToken)
	if !appOK || !grantOK || !tokenOK {
		return nil, nil, nil, false
	}
	typedApp, appOK := app.(*model.ConnectedApp)
	typedGrant, grantOK := grant.(*model.ConnectedAppGrant)
	typedAccessToken, tokenOK := accessToken.(*model.ConnectedAppAccessToken)
	return typedApp, typedGrant, typedAccessToken, appOK && grantOK && tokenOK
}

func ConnectedAppManagementTokenHash(token string) string {
	return connectedAppManagementTokenHash(token)
}

func connectedAppManagementTokenHash(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(sum[:])
}

func bearerToken(header string) string {
	parts := strings.Fields(strings.TrimSpace(header))
	if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
		return strings.TrimSpace(parts[1])
	}
	return ""
}

func abortConnectedAppManagement(c *gin.Context, status int, code string, message string) {
	c.JSON(status, gin.H{
		"success": false,
		"message": message,
		"error": gin.H{
			"code":       code,
			"message":    message,
			"request_id": c.GetHeader(common.RequestIdKey),
		},
	})
	c.Abort()
}
