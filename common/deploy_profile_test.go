package common

import (
	"os"
	"testing"
)

func TestDeployProfileAliases(t *testing.T) {
	t.Setenv("DATA_PROXY_PROFILE", "self-use")
	if got := DeployProfile(); got != DeployProfileLite {
		t.Fatalf("self-use: got %q", got)
	}
	t.Setenv("DATA_PROXY_PROFILE", "production")
	if got := DeployProfile(); got != DeployProfileStandard {
		t.Fatalf("production: got %q", got)
	}
	t.Setenv("DATA_PROXY_PROFILE", "ha")
	if got := DeployProfile(); got != DeployProfileHA {
		t.Fatalf("ha: got %q", got)
	}
	t.Setenv("DATA_PROXY_PROFILE", "")
	if got := DeployProfile(); got != "" {
		t.Fatalf("empty: got %q", got)
	}
}

func TestApplyCacheDefaults(t *testing.T) {
	origRedis := RedisEnabled
	origMem := MemoryCacheEnabled
	origSQLite := UsingSQLite
	t.Cleanup(func() {
		RedisEnabled = origRedis
		MemoryCacheEnabled = origMem
		UsingSQLite = origSQLite
		_ = os.Unsetenv("MEMORY_CACHE_ENABLED")
		_ = os.Unsetenv("DATA_PROXY_PROFILE")
	})

	// Redis forces memory cache
	RedisEnabled = true
	MemoryCacheEnabled = false
	UsingSQLite = false
	t.Setenv("MEMORY_CACHE_ENABLED", "")
	t.Setenv("DATA_PROXY_PROFILE", "")
	ApplyCacheDefaults()
	if !MemoryCacheEnabled {
		t.Fatal("expected memory cache on when Redis enabled")
	}

	// SQLite without Redis auto-enables
	RedisEnabled = false
	MemoryCacheEnabled = false
	UsingSQLite = true
	t.Setenv("MEMORY_CACHE_ENABLED", "")
	t.Setenv("DATA_PROXY_PROFILE", "")
	ApplyCacheDefaults()
	if !MemoryCacheEnabled {
		t.Fatal("expected memory cache on for SQLite without Redis")
	}

	// explicit false wins
	RedisEnabled = false
	MemoryCacheEnabled = false
	UsingSQLite = true
	t.Setenv("MEMORY_CACHE_ENABLED", "false")
	ApplyCacheDefaults()
	if MemoryCacheEnabled {
		t.Fatal("expected memory cache off when MEMORY_CACHE_ENABLED=false")
	}

	// profile=lite without sqlite still enables when no redis
	RedisEnabled = false
	MemoryCacheEnabled = false
	UsingSQLite = false
	t.Setenv("MEMORY_CACHE_ENABLED", "")
	t.Setenv("DATA_PROXY_PROFILE", "lite")
	ApplyCacheDefaults()
	if !MemoryCacheEnabled {
		t.Fatal("expected memory cache on for DATA_PROXY_PROFILE=lite")
	}

	// MySQL without redis and without profile stays off
	RedisEnabled = false
	MemoryCacheEnabled = false
	UsingSQLite = false
	t.Setenv("MEMORY_CACHE_ENABLED", "")
	t.Setenv("DATA_PROXY_PROFILE", "")
	ApplyCacheDefaults()
	if MemoryCacheEnabled {
		t.Fatal("expected memory cache off for non-SQLite without profile")
	}
}
