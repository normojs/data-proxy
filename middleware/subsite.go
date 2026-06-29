package middleware

import (
	"errors"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	SubsiteContextKey        = "subsite"
	SubsiteIDContextKey      = "subsite_id"
	SubsiteSlugContextKey    = "subsite_slug"
	SubsiteStatusContextKey  = "subsite_status"
	SubsitePreviewContextKey = "subsite_preview"
)

func SubsiteContext(requireOpen bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		subsite, err := model.GetSubsiteBySlug(c.Param("slug"))
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				abortSubsiteOpenAI(c, http.StatusNotFound, "Subsite not found", model.SubsiteAccessCodeNotFound)
				return
			}
			if _, normalizeErr := model.NormalizeSubsiteSlug(c.Param("slug")); normalizeErr != nil {
				abortSubsiteOpenAI(c, http.StatusNotFound, "Subsite not found", model.SubsiteAccessCodeNotFound)
				return
			}
			abortSubsiteOpenAI(c, http.StatusInternalServerError, "Subsite lookup failed", string(types.ErrorCodeQueryDataError))
			return
		}

		decision := subsite.AccessDecision(common.GetTimestamp(), c.GetBool(SubsitePreviewContextKey))
		c.Set(SubsiteContextKey, subsite)
		c.Set(SubsiteIDContextKey, subsite.Id)
		c.Set(SubsiteSlugContextKey, subsite.Slug)
		c.Set(SubsiteStatusContextKey, decision.Status)
		if requireOpen && !decision.Allowed {
			abortSubsiteOpenAI(c, subsiteAccessHTTPStatus(decision.Code), decision.Message, decision.Code)
			return
		}
		c.Next()
	}
}

func SubsiteTokenScopeAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		subsiteId := c.GetInt64(SubsiteIDContextKey)
		tokenSubsiteId := c.GetInt64("token_subsite_id")
		if subsiteId <= 0 || tokenSubsiteId != subsiteId {
			abortSubsiteOpenAI(c, http.StatusForbidden, "API key is not scoped to this subsite", model.SubsiteAccessCodeTokenScope)
			return
		}
		c.Next()
	}
}

func MainSiteTokenScopeAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if tokenSubsiteId := c.GetInt64("token_subsite_id"); tokenSubsiteId > 0 {
			abortSubsiteOpenAI(c, http.StatusForbidden, "Subsite API key cannot access the main API", model.SubsiteAccessCodeTokenScope)
			return
		}
		c.Next()
	}
}

func abortSubsiteOpenAI(c *gin.Context, statusCode int, message string, code string) {
	c.JSON(statusCode, gin.H{
		"error": types.OpenAIError{
			Message: common.MessageWithRequestId(message, c.GetString(common.RequestIdKey)),
			Type:    "new_api_error",
			Code:    code,
		},
	})
	c.Abort()
}

func subsiteAccessHTTPStatus(code string) int {
	switch code {
	case model.SubsiteAccessCodeNotFound:
		return http.StatusNotFound
	case model.SubsiteAccessCodeDisabled, model.SubsiteAccessCodeDraft, model.SubsiteAccessCodeNotStarted, model.SubsiteAccessCodeExpired:
		return http.StatusForbidden
	default:
		return http.StatusForbidden
	}
}
