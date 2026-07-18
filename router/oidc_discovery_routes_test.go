package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRootOIDCDiscoveryRoutesNotSPA(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/.well-known/openid-configuration", middleware.RouteTag("api"), controller.GetConnectedAppOpenIDConfiguration)
	r.GET("/oauth/jwks.json", middleware.RouteTag("api"), controller.GetConnectedAppJWKS)

	// discovery
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/.well-known/openid-configuration", nil)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	require.NotContains(t, body, "<!DOCTYPE html>")
	var cfg map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &cfg))
	require.Contains(t, cfg, "issuer")
	require.Contains(t, cfg, "jwks_uri")
	require.Contains(t, cfg, "authorization_endpoint")

	// jwks — may error if RSA key path not writable in test env; still must not be HTML SPA
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/oauth/jwks.json", nil)
	r.ServeHTTP(w2, req2)
	require.NotContains(t, w2.Body.String(), "<!DOCTYPE html>")
	require.NotContains(t, w2.Body.String(), `<div id="root"`)
	// Prefer JSON success when key can be created
	if w2.Code == http.StatusOK {
		var jwks map[string]any
		require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &jwks))
		require.Contains(t, jwks, "keys")
	}
}
