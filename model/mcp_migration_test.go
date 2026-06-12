package model

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/pkg/mcp/catalog"
)

func TestMCPMigrationSmoke(t *testing.T) {
	if os.Getenv("MCP_MIGRATION_TEST") != "1" {
		t.Skip("set MCP_MIGRATION_TEST=1 to run the MCP migration smoke test")
	}
	if os.Getenv("SQL_DSN") == "" {
		t.Setenv("SQLITE_PATH", "file:"+filepath.Join(t.TempDir(), "mcp-migration-smoke.db")+"?_pragma=busy_timeout(30000)&_pragma=journal_mode(WAL)")
	}

	common.InitEnv()
	logger.SetupLogger()

	if err := InitDB(); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	if err := InitLogDB(); err != nil {
		t.Fatalf("InitLogDB failed: %v", err)
	}
	assertMCPMigrationSmokeState(t)
	assertNoSQLiteDecimalDDL(t)
	if err := CloseDB(); err != nil {
		t.Fatalf("CloseDB after first migration failed: %v", err)
	}

	if err := InitDB(); err != nil {
		t.Fatalf("second InitDB failed: %v", err)
	}
	if err := InitLogDB(); err != nil {
		t.Fatalf("second InitLogDB failed: %v", err)
	}
	t.Cleanup(func() {
		_ = CloseDB()
	})
	assertMCPMigrationSmokeState(t)
	assertNoSQLiteDecimalDDL(t)
}

func assertMCPMigrationSmokeState(t *testing.T) {
	t.Helper()
	if !DB.Migrator().HasTable(&MCPTool{}) {
		t.Fatal("mcp_tools table was not migrated")
	}
	if !DB.Migrator().HasTable(&MCPToolCall{}) {
		t.Fatal("mcp_tool_calls table was not migrated")
	}
	if !DB.Migrator().HasTable(&MCPToolCallIdempotencyKey{}) {
		t.Fatal("mcp_tool_call_idempotency_keys table was not migrated")
	}
	if !DB.Migrator().HasTable(&MCPUserDailyQuota{}) {
		t.Fatal("mcp_user_daily_quota table was not migrated")
	}
	if !DB.Migrator().HasTable(&MCPProxyServer{}) {
		t.Fatal("mcp_proxy_servers table was not migrated")
	}
	if !DB.Migrator().HasTable(&MCPProxyTool{}) {
		t.Fatal("mcp_proxy_tools table was not migrated")
	}
	if !DB.Migrator().HasTable(&MCPProxyDiscoveryEvent{}) {
		t.Fatal("mcp_proxy_discovery_events table was not migrated")
	}
	if !DB.Migrator().HasTable(&MCPOpenAPITool{}) {
		t.Fatal("mcp_openapi_tools table was not migrated")
	}
	if !DB.Migrator().HasTable(&MCPOpenAPIBinaryObject{}) {
		t.Fatal("mcp_openapi_binary_objects table was not migrated")
	}
	if !DB.Migrator().HasTable(&BridgeClient{}) {
		t.Fatal("bridge_clients table was not migrated")
	}
	if !DB.Migrator().HasColumn(&BridgeClient{}, "policy") {
		t.Fatal("bridge_clients.policy column was not migrated")
	}
	if !DB.Migrator().HasTable(&BridgeSession{}) {
		t.Fatal("bridge_sessions table was not migrated")
	}
	if !DB.Migrator().HasTable(&BridgeAuditLog{}) {
		t.Fatal("bridge_audit_logs table was not migrated")
	}
	if !DB.Migrator().HasTable(&BillingEvent{}) {
		t.Fatal("billing_events table was not migrated")
	}
	if !DB.Migrator().HasTable(&BillingEventRelation{}) {
		t.Fatal("billing_event_relations table was not migrated")
	}
	if !DB.Migrator().HasTable(&BillingEventRelationInspectionRun{}) {
		t.Fatal("billing_event_relation_inspection_runs table was not migrated")
	}

	var toolCount int64
	if err := DB.Model(&MCPTool{}).Where("source = ?", MCPToolSourceBuiltin).Count(&toolCount).Error; err != nil {
		t.Fatalf("count builtin MCP tools failed: %v", err)
	}
	if toolCount != int64(len(catalog.BuiltinTools())) {
		t.Fatalf("builtin MCP tool count mismatch, got %d want %d", toolCount, len(catalog.BuiltinTools()))
	}
}

func assertNoSQLiteDecimalDDL(t *testing.T) {
	t.Helper()
	if !common.UsingSQLite {
		return
	}
	var count int64
	if err := DB.Raw(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND LOWER(sql) LIKE '%decimal%'`).Scan(&count).Error; err != nil {
		t.Fatalf("inspect sqlite decimal DDL failed: %v", err)
	}
	if count != 0 {
		t.Fatalf("sqlite schema still contains decimal DDL entries: %d", count)
	}
}
