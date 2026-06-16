package common

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

const runtimeConfigEnv = "DATA_PROXY_RUNTIME_CONFIG"

type RuntimeConfig struct {
	SQLDSN          string `json:"sql_dsn,omitempty"`
	LogSQLDSN       string `json:"log_sql_dsn,omitempty"`
	RedisConnString string `json:"redis_conn_string,omitempty"`
	SQLitePath      string `json:"sqlite_path,omitempty"`
	UpdatedAt       int64  `json:"updated_at"`
}

var (
	RuntimeConfigLoaded          bool
	RuntimeConfigPath            string
	RuntimeConfigRestartRequired bool
	runtimeConfigAppliedKeys     = map[string]bool{}
)

func RuntimeConfigFilePath() string {
	if path := os.Getenv(runtimeConfigEnv); path != "" {
		return path
	}
	if info, err := os.Stat("/data"); err == nil && info.IsDir() {
		return "/data/runtime-config.json"
	}
	return filepath.Join("data", "runtime-config.json")
}

func LoadRuntimeConfigIntoEnv() error {
	RuntimeConfigPath = RuntimeConfigFilePath()
	data, err := os.ReadFile(RuntimeConfigPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}

	var cfg RuntimeConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return err
	}

	RuntimeConfigLoaded = true
	applyRuntimeConfigEnv("SQL_DSN", cfg.SQLDSN)
	applyRuntimeConfigEnv("LOG_SQL_DSN", cfg.LogSQLDSN)
	applyRuntimeConfigEnv("REDIS_CONN_STRING", cfg.RedisConnString)
	applyRuntimeConfigEnv("SQLITE_PATH", cfg.SQLitePath)
	return nil
}

func applyRuntimeConfigEnv(key string, value string) {
	if value == "" || os.Getenv(key) != "" {
		return
	}
	_ = os.Setenv(key, value)
	runtimeConfigAppliedKeys[key] = true
}

func RuntimeConfigValueSource(key string) string {
	if os.Getenv(key) == "" {
		return ""
	}
	if runtimeConfigAppliedKeys[key] {
		return "runtime-config"
	}
	return "env"
}

func SaveRuntimeConfig(cfg RuntimeConfig) error {
	RuntimeConfigPath = RuntimeConfigFilePath()
	cfg.UpdatedAt = time.Now().Unix()

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(RuntimeConfigPath), 0700); err != nil {
		return err
	}
	if err := os.WriteFile(RuntimeConfigPath, data, 0600); err != nil {
		return err
	}

	RuntimeConfigLoaded = true
	RuntimeConfigRestartRequired = true
	return nil
}
