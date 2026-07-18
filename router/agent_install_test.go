package router

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAgentInstallScriptRoutesServeShellNotSPA(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	registerAgentInstallRoutes(r)

	// Ensure local script is discoverable: tests run with package dir as cwd.
	// serveAgentInstallScript also checks Getwd()/scripts — create a temp layout if needed.
	wd, err := os.Getwd()
	require.NoError(t, err)
	repoScript := filepath.Clean(filepath.Join(wd, "..", "scripts", "install-data-proxy-agent.sh"))
	if _, err := os.Stat(repoScript); err != nil {
		// Still OK: bootstrap fallback must not be HTML.
		t.Logf("repo script not found at %s; asserting bootstrap fallback", repoScript)
	} else {
		// Point cwd-relative lookup at a copy under ./scripts for the handler.
		_ = os.MkdirAll("scripts", 0o755)
		data, err := os.ReadFile(repoScript)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join("scripts", "install-data-proxy-agent.sh"), data, 0o644))
		t.Cleanup(func() {
			_ = os.Remove(filepath.Join("scripts", "install-data-proxy-agent.sh"))
			_ = os.Remove("scripts")
		})
	}

	for _, path := range []string{"/agent/install.sh", "/agent/install-data-proxy-agent.sh"} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code, path)
		body := w.Body.String()
		require.True(t, strings.HasPrefix(body, "#!"), "%s: expected shell shebang, body starts %q", path, body[:min(40, len(body))])
		require.NotContains(t, body, "<!DOCTYPE html>", path)
		require.NotContains(t, body, "<div id=\"root\"", path)
		require.Contains(t, w.Header().Get("Content-Type"), "shellscript", path)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
