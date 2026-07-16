package service

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserFacingBillingErrorStableCodeAndStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	cases := []types.ErrorCode{
		types.ErrorCodeInsufficientUserQuota,
		types.ErrorCodeInsufficientModelTokenPackage,
		types.ErrorCodePreConsumeTokenQuotaFailed,
	}
	for _, code := range cases {
		apiErr := UserFacingBillingError(c, code)
		require.NotNil(t, apiErr)
		assert.Equal(t, code, apiErr.GetErrorCode())
		assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
		assert.NotEmpty(t, apiErr.Error())
		// Must not leak internal remain/need style strings
		assert.NotContains(t, apiErr.Error(), "剩余额度")
		assert.NotContains(t, apiErr.Error(), "token remain quota")
	}
}

func TestUserFacingSubscriptionQuotaErrorKeepsCode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	apiErr := UserFacingSubscriptionQuotaError(c)
	require.NotNil(t, apiErr)
	assert.Equal(t, types.ErrorCodeInsufficientUserQuota, apiErr.GetErrorCode())
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
}
