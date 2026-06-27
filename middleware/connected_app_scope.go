package middleware

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	connectedAppScopeOpenAIModels              = "openai.models"
	connectedAppScopeOpenAIChat                = "openai.chat"
	connectedAppScopeOpenAIResponses           = "openai.responses"
	connectedAppScopeOpenAIAudioTranscriptions = "openai.audio.transcriptions"
	connectedAppScopeQuotaRead                 = "quota.read"
)

// ConnectedAppScopeAuth enforces app/grant scopes for native tokens issued by
// connected app device authorization. Regular user-created tokens are not scoped.
func ConnectedAppScopeAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenId := c.GetInt("token_id")
		if tokenId <= 0 {
			c.Next()
			return
		}

		binding, err := model.GetConnectedAppTokenBindingByTokenId(tokenId)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				c.Next()
				return
			}
			abortWithOpenAiMessage(c, http.StatusInternalServerError, "connected app scope check failed")
			return
		}

		requiredScope := connectedAppRequiredScope(c)
		if requiredScope == "" {
			abortWithOpenAiMessage(c, http.StatusForbidden, "connected app token is not allowed to access this endpoint", types.ErrorCodeAccessDenied)
			return
		}

		app, grant, statusCode, err := connectedAppScopeContext(binding, c.GetInt("id"))
		if err != nil {
			if statusCode == http.StatusForbidden {
				abortWithOpenAiMessage(c, statusCode, err.Error(), types.ErrorCodeAccessDenied)
			} else {
				abortWithOpenAiMessage(c, statusCode, err.Error())
			}
			return
		}
		if !connectedAppHasScope(app.ScopeList(), requiredScope) || !connectedAppHasScope(grant.ScopeList(), requiredScope) {
			abortWithOpenAiMessage(c, http.StatusForbidden, fmt.Sprintf("connected app token requires scope %s", requiredScope), types.ErrorCodeAccessDenied)
			return
		}

		if err := model.TouchConnectedAppUsage(binding.AppId, binding.UserId, binding.TokenId, common.GetTimestamp()); err != nil {
			common.SysLog("failed to touch connected app usage: " + err.Error())
		}
		c.Set("connected_app_id", binding.AppId)
		c.Set("connected_app_grant_id", binding.GrantId)
		c.Set("connected_app_required_scope", requiredScope)
		c.Next()
	}
}

func connectedAppRequiredScope(c *gin.Context) string {
	method := c.Request.Method
	path := c.FullPath()
	if path == "" && c.Request != nil && c.Request.URL != nil {
		path = c.Request.URL.Path
	}

	switch {
	case method == http.MethodGet && (path == "/v1/models" || path == "/v1/models/:model"):
		return connectedAppScopeOpenAIModels
	case method == http.MethodPost && path == "/v1/chat/completions":
		return connectedAppScopeOpenAIChat
	case method == http.MethodPost && (path == "/v1/responses" || path == "/v1/responses/compact"):
		return connectedAppScopeOpenAIResponses
	case method == http.MethodPost && path == "/v1/audio/transcriptions":
		return connectedAppScopeOpenAIAudioTranscriptions
	case method == http.MethodGet && (path == "/api/usage/token/" || path == "/api/usage/token"):
		return connectedAppScopeQuotaRead
	default:
		return ""
	}
}

func connectedAppScopeContext(binding *model.ConnectedAppTokenBinding, userId int) (*model.ConnectedApp, *model.ConnectedAppGrant, int, error) {
	if binding == nil || binding.Id <= 0 {
		return nil, nil, http.StatusForbidden, errors.New("connected app token binding is missing")
	}
	if binding.Status != model.ConnectedAppTokenBindingStatusActive {
		return nil, nil, http.StatusForbidden, errors.New("connected app token binding is not active")
	}
	if userId <= 0 || binding.UserId != userId {
		return nil, nil, http.StatusForbidden, errors.New("connected app token binding does not match user")
	}

	app, err := model.GetConnectedAppByID(binding.AppId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, http.StatusForbidden, errors.New("connected app is unavailable")
		}
		return nil, nil, http.StatusInternalServerError, errors.New("connected app scope check failed")
	}
	if app.Status != model.ConnectedAppStatusEnabled {
		return nil, nil, http.StatusForbidden, errors.New("connected app is disabled")
	}

	grant, err := model.GetConnectedAppGrant(binding.AppId, binding.UserId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, http.StatusForbidden, errors.New("connected app grant is unavailable")
		}
		return nil, nil, http.StatusInternalServerError, errors.New("connected app scope check failed")
	}
	if grant.Id != binding.GrantId || grant.Status != model.ConnectedAppGrantStatusAuthorized {
		return nil, nil, http.StatusForbidden, errors.New("connected app grant is not authorized")
	}
	return app, grant, http.StatusOK, nil
}

func connectedAppHasScope(scopes []string, requiredScope string) bool {
	for _, scope := range scopes {
		if scope == requiredScope {
			return true
		}
	}
	return false
}
