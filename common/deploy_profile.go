package common

import (
	"os"
	"strings"
)

// Deploy profile names (DATA_PROXY_PROFILE). Empty means "infer from env".
const (
	DeployProfileLite     = "lite"
	DeployProfileStandard = "standard"
	DeployProfileHA       = "ha"
)

// DeployProfile returns the normalized DATA_PROXY_PROFILE value, or "".
// Aliases: self / self-use → lite; prod / production → standard.
func DeployProfile() string {
	p := strings.ToLower(strings.TrimSpace(os.Getenv("DATA_PROXY_PROFILE")))
	switch p {
	case "self", "self-use", "selfuse":
		return DeployProfileLite
	case "prod", "production":
		return DeployProfileStandard
	case DeployProfileLite, DeployProfileStandard, DeployProfileHA:
		return p
	default:
		return ""
	}
}

// ApplyCacheDefaults sets MemoryCacheEnabled for single-node / lite deployments.
// Call after InitDB (so UsingSQLite is known) and InitRedisClient.
//
// Rules:
//   - Redis on  → memory channel cache on (legacy compatibility)
//   - MEMORY_CACHE_ENABLED=false → leave off even on SQLite
//   - profile=lite → memory cache on when Redis is off
//   - no profile + SQLite + no Redis → memory cache on (self-use default)
//   - MEMORY_CACHE_ENABLED=true already handled in InitEnv
func ApplyCacheDefaults() {
	if RedisEnabled {
		MemoryCacheEnabled = true
		return
	}

	if strings.EqualFold(strings.TrimSpace(os.Getenv("MEMORY_CACHE_ENABLED")), "false") {
		MemoryCacheEnabled = false
		return
	}

	if MemoryCacheEnabled {
		return
	}

	profile := DeployProfile()
	autoLite := profile == DeployProfileLite || (profile == "" && UsingSQLite)
	if autoLite {
		MemoryCacheEnabled = true
		SysLog("cache backend=memory (single-node; Redis disabled)")
		if profile == DeployProfileLite {
			SysLog("DATA_PROXY_PROFILE=lite: SQLite-friendly process-local cache")
		}
	}
}
