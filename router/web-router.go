package router

import (
	"embed"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/gin-contrib/gzip"
	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
)

// ThemeAssets holds the embedded new frontend assets.
type ThemeAssets struct {
	DefaultBuildFS   embed.FS
	DefaultIndexPage []byte
}

func SetWebRouter(router *gin.Engine, assets ThemeAssets) {
	defaultFS := common.EmbedFolder(assets.DefaultBuildFS, "web/default/dist")

	// Agent install scripts must not fall through to the SPA index.html shell.
	// Prefer the repo script when running from source; otherwise serve a thin
	// bootstrap that curls the canonical script from the public GitHub repo.
	registerAgentInstallRoutes(router)

	router.Use(gzip.Gzip(gzip.DefaultCompression))
	router.Use(middleware.GlobalWebRateLimit())
	router.Use(middleware.Cache())
	router.Use(static.Serve("/", defaultFS))
	router.NoRoute(func(c *gin.Context) {
		c.Set(middleware.RouteTagKey, "web")
		if strings.HasPrefix(c.Request.RequestURI, "/v1") || strings.HasPrefix(c.Request.RequestURI, "/api") || strings.HasPrefix(c.Request.RequestURI, "/assets") {
			controller.RelayNotFound(c)
			return
		}
		c.Header("Cache-Control", "no-cache")
		c.Data(http.StatusOK, "text/html; charset=utf-8", assets.DefaultIndexPage)
	})
}

func registerAgentInstallRoutes(router *gin.Engine) {
	handler := serveAgentInstallScript
	// Short and long names used in docs / console copy.
	router.GET("/agent/install.sh", handler)
	router.GET("/agent/install-data-proxy-agent.sh", handler)
}

func serveAgentInstallScript(c *gin.Context) {
	c.Set(middleware.RouteTagKey, "web")
	c.Header("Cache-Control", "no-cache")
	c.Header("Content-Type", "text/x-shellscript; charset=utf-8")
	c.Header("X-Content-Type-Options", "nosniff")

	// Prefer local script when present (dev / docker image with sources).
	candidates := []string{
		filepath.Join("scripts", "install-data-proxy-agent.sh"),
		filepath.Join(".", "scripts", "install-data-proxy-agent.sh"),
	}
	if wd, err := os.Getwd(); err == nil {
		candidates = append([]string{
			filepath.Join(wd, "scripts", "install-data-proxy-agent.sh"),
		}, candidates...)
	}
	for _, path := range candidates {
		if data, err := os.ReadFile(path); err == nil && len(data) > 0 {
			if !strings.HasPrefix(string(data), "#!") {
				continue
			}
			c.Data(http.StatusOK, "text/x-shellscript; charset=utf-8", data)
			return
		}
	}

	// Fallback bootstrap: fetch the published script from the public repo so
	// production containers that only embed web/dist still work.
	const bootstrap = `#!/usr/bin/env sh
set -eu
# Data Proxy agent install bootstrap (served when local scripts/ is unavailable).
REPO="${DATA_PROXY_AGENT_REPO:-normojs/data-proxy}"
BRANCH="${DATA_PROXY_AGENT_INSTALL_BRANCH:-main}"
URL="https://raw.githubusercontent.com/${REPO}/${BRANCH}/scripts/install-data-proxy-agent.sh"
if command -v curl >/dev/null 2>&1; then
  exec curl -fsSL "$URL" | sh -s -- "$@"
fi
if command -v wget >/dev/null 2>&1; then
  exec wget -qO- "$URL" | sh -s -- "$@"
fi
echo "missing required command: curl or wget" >&2
exit 1
`
	c.String(http.StatusOK, bootstrap)
}
