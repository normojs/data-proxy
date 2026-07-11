package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRelayNotImplementedResponseShape(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/unsupported", nil)

	RelayNotImplemented(ctx)

	require.Equal(t, http.StatusNotImplemented, recorder.Code)

	var response struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Param   string `json:"param"`
			Code    string `json:"code"`
		} `json:"error"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, "API not implemented", response.Error.Message)
	require.Equal(t, "new_api_error", response.Error.Type)
	require.Equal(t, "", response.Error.Param)
	require.Equal(t, "api_not_implemented", response.Error.Code)
}
