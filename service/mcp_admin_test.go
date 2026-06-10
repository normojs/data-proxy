package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/bridge"
	mcpexecutor "github.com/QuantumNous/new-api/pkg/mcp/executor"
	"github.com/stretchr/testify/require"
)

func TestMCPToolAdminServiceSmoke(t *testing.T) {
	if os.Getenv("MCP_MIGRATION_TEST") != "1" {
		t.Skip("set MCP_MIGRATION_TEST=1 to run the MCP admin service smoke test")
	}
	if os.Getenv("SQL_DSN") == "" {
		t.Fatal("SQL_DSN is required")
	}

	common.InitEnv()
	logger.SetupLogger()
	if err := model.InitDB(); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	if err := model.InitLogDB(); err != nil {
		t.Fatalf("InitLogDB failed: %v", err)
	}
	t.Cleanup(func() {
		_ = model.CloseDB()
	})

	items, total, err := ListMCPToolsForAdmin(MCPToolListParams{Limit: 100})
	if err != nil {
		t.Fatalf("ListMCPToolsForAdmin failed: %v", err)
	}
	if total < 1 || len(items) < 1 {
		t.Fatalf("expected seeded MCP tools, total=%d len=%d", total, len(items))
	}

	first := items[0]
	detail, err := GetMCPToolForAdmin(first.Id)
	if err != nil {
		t.Fatalf("GetMCPToolForAdmin failed: %v", err)
	}
	if detail.Name != first.Name {
		t.Fatalf("detail name mismatch, got %s want %s", detail.Name, first.Name)
	}

	newFreeQuota := detail.FreeQuota + 1
	updated, err := UpdateMCPToolForAdmin(detail.Id, dto.MCPToolUpdateRequest{FreeQuota: &newFreeQuota})
	if err != nil {
		t.Fatalf("UpdateMCPToolForAdmin failed: %v", err)
	}
	if updated.FreeQuota != newFreeQuota {
		t.Fatalf("free quota mismatch, got %d want %d", updated.FreeQuota, newFreeQuota)
	}

	originalFreeQuota := detail.FreeQuota
	if _, err := UpdateMCPToolForAdmin(detail.Id, dto.MCPToolUpdateRequest{FreeQuota: &originalFreeQuota}); err != nil {
		t.Fatalf("restore free quota failed: %v", err)
	}

	customName := fmt.Sprintf("custom_tool_%d", time.Now().UnixNano())
	_ = model.DB.Unscoped().Where("name = ?", customName).Delete(&model.MCPTool{}).Error
	t.Cleanup(func() {
		_ = model.DB.Unscoped().Where("name = ?", customName).Delete(&model.MCPTool{}).Error
	})
	price := 0.0025
	freeQuota := 3
	status := model.MCPToolStatusEnabled
	created, err := CreateMCPToolForAdmin(dto.MCPToolCreateRequest{
		Name:         customName,
		DisplayName:  "Custom Tool Smoke",
		Description:  "custom tool smoke",
		Category:     "custom",
		InputSchema:  map[string]any{"type": "object", "properties": map[string]any{}},
		PricePerCall: &price,
		PriceUnit:    model.MCPToolPriceUnitPerCall,
		FreeQuota:    &freeQuota,
		Status:       &status,
	})
	if err != nil {
		t.Fatalf("CreateMCPToolForAdmin failed: %v", err)
	}
	if created.Source != model.MCPToolSourceCustom || created.Name != customName {
		t.Fatalf("created custom tool mismatch: %#v", created)
	}
	if _, err := CreateMCPToolForAdmin(dto.MCPToolCreateRequest{Name: customName}); err == nil {
		t.Fatal("expected duplicate custom tool name to fail")
	}
	archived, err := ArchiveMCPToolForAdmin(created.Id)
	if err != nil {
		t.Fatalf("ArchiveMCPToolForAdmin failed: %v", err)
	}
	if archived.Status != model.MCPToolStatusDisabled {
		t.Fatalf("archived custom tool status mismatch, got %d", archived.Status)
	}
	if _, err := ArchiveMCPToolForAdmin(detail.Id); err == nil {
		t.Fatal("expected archiving built-in tool to fail")
	}
	deleted, err := DeleteMCPToolForAdmin(created.Id)
	if err != nil {
		t.Fatalf("DeleteMCPToolForAdmin failed: %v", err)
	}
	if deleted.Source != model.MCPToolSourceCustom {
		t.Fatalf("deleted custom tool mismatch: %#v", deleted)
	}
	if _, err := GetMCPToolForAdmin(created.Id); err == nil {
		t.Fatal("expected deleted custom tool to be hidden from normal queries")
	}
}

func TestMCPToolAdminValidation(t *testing.T) {
	validNames := []string{"custom_read", "custom-read", "custom1", "namespace.tool_1"}
	for _, name := range validNames {
		if err := validateMCPToolName(name); err != nil {
			t.Fatalf("expected valid MCP tool name %q: %v", name, err)
		}
	}

	invalidNames := []string{"", "ab", "Custom", "custom.tool.extra", "custom/tool", "custom tool"}
	for _, name := range invalidNames {
		if err := validateMCPToolName(name); err == nil {
			t.Fatalf("expected invalid MCP tool name %q", name)
		}
	}

	if _, err := normalizeMCPToolInputSchema(`{"type":"object","properties":{}}`); err != nil {
		t.Fatalf("expected valid input schema string: %v", err)
	}
	if _, err := normalizeMCPToolInputSchema(`[]`); err == nil {
		t.Fatal("expected non-object input schema to fail")
	}
	if err := validateMCPToolPriceUnit(model.MCPToolPriceUnitPerCall); err != nil {
		t.Fatalf("expected valid price unit: %v", err)
	}
	if err := validateMCPToolPriceUnit(model.MCPToolPriceUnitPer1K); err == nil {
		t.Fatal("expected per_1k_tokens to fail until usage-based MCP billing is implemented")
	}
	if err := validateMCPToolPriceUnit(model.MCPToolPriceUnitPerMB); err == nil {
		t.Fatal("expected per_mb to fail until usage-based MCP billing is implemented")
	}
	if err := validateMCPToolPriceUnit("bad_unit"); err == nil {
		t.Fatal("expected invalid price unit to fail")
	}
}

func TestMCPToolUserListOnlyReturnsEnabledTools(t *testing.T) {
	setupMCPProxyServiceTestDB(t)

	enabledTool := &model.MCPTool{
		Name:         "user_visible_enabled",
		DisplayName:  "User Visible Enabled",
		Description:  "visible to normal users",
		Category:     "audit",
		Source:       model.MCPToolSourceCustom,
		InputSchema:  `{"type":"object"}`,
		PriceUnit:    model.MCPToolPriceUnitPerCall,
		Status:       model.MCPToolStatusEnabled,
		PricePerCall: 0,
	}
	disabledTool := &model.MCPTool{
		Name:         "user_visible_disabled",
		DisplayName:  "User Visible Disabled",
		Description:  "hidden from normal users",
		Category:     "audit",
		Source:       model.MCPToolSourceCustom,
		InputSchema:  `{"type":"object"}`,
		PriceUnit:    model.MCPToolPriceUnitPerCall,
		Status:       model.MCPToolStatusDisabled,
		PricePerCall: 0,
	}
	require.NoError(t, model.CreateMCPTool(enabledTool))
	require.NoError(t, model.CreateMCPTool(disabledTool))
	disabledTool, err := model.UpdateMCPToolFields(disabledTool.Id, map[string]any{"status": model.MCPToolStatusDisabled})
	require.NoError(t, err)

	items, total, err := ListMCPToolsForUser(MCPToolListParams{
		Keyword: enabledTool.Name,
		Limit:   20,
	})
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, items, 1)
	require.Equal(t, enabledTool.Id, items[0].Id)

	detail, err := GetMCPToolForUser(enabledTool.Id)
	require.NoError(t, err)
	require.Equal(t, enabledTool.Id, detail.Id)
	_, err = GetMCPToolForUser(disabledTool.Id)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

func TestMCPToolCallRejectsArgumentsBeforePersistence(t *testing.T) {
	setupMCPProxyServiceTestDB(t)
	user, token := seedMCPBillingUserAndToken(t, 100000, 100000, false)

	tool := &model.MCPTool{
		Name:         "custom_required_args",
		DisplayName:  "Custom Required Args",
		Description:  "validates arguments before billing",
		Category:     "audit",
		Source:       model.MCPToolSourceCustom,
		InputSchema:  `{"type":"object","required":["name"],"properties":{"name":{"type":"string"},"count":{"type":"integer"}}}`,
		PriceUnit:    model.MCPToolPriceUnitPerCall,
		PricePerCall: 0.001,
		Status:       model.MCPToolStatusEnabled,
	}
	require.NoError(t, model.CreateMCPTool(tool))

	resp, err := CallMCPTool(MCPToolCallRequest{
		UserId:         user.Id,
		TokenId:        token.Id,
		TokenKey:       token.Key,
		TokenUnlimited: token.UnlimitedQuota,
		TokenQuota:     token.RemainQuota,
		UsingGroup:     "default",
		RequestId:      "mcp-invalid-args",
		Params: dto.MCPToolCallParams{
			Name:      tool.Name,
			Arguments: map[string]any{"count": 1},
		},
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, dto.MCPErrorCodeInvalidParams, resp.ErrorCode)
	errorData, ok := resp.ErrorData.(dto.MCPToolCallErrorData)
	require.True(t, ok)
	require.Contains(t, errorData.Reason, "$.name is required")

	var calls int64
	require.NoError(t, model.DB.Model(&model.MCPToolCall{}).Where("request_id = ?", "mcp-invalid-args").Count(&calls).Error)
	require.Zero(t, calls)

	var events int64
	require.NoError(t, model.DB.Model(&model.BillingEvent{}).Where("request_id = ?", "mcp-invalid-args").Count(&events).Error)
	require.Zero(t, events)
}

func TestMCPToolCallPersistenceSmoke(t *testing.T) {
	if os.Getenv("MCP_MIGRATION_TEST") != "1" {
		t.Skip("set MCP_MIGRATION_TEST=1 to run the MCP tool call persistence smoke test")
	}
	if os.Getenv("SQL_DSN") == "" {
		t.Fatal("SQL_DSN is required")
	}

	common.InitEnv()
	logger.SetupLogger()
	if err := model.InitDB(); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	if err := model.InitLogDB(); err != nil {
		t.Fatalf("InitLogDB failed: %v", err)
	}
	t.Cleanup(func() {
		_ = model.DB.Where("request_id = ?", "mcp-call-smoke").Delete(&model.MCPToolCall{}).Error
		_ = model.DB.Where("request_id = ?", "mcp-call-smoke").Delete(&model.BillingEvent{}).Error
		_ = model.DB.Where("request_id = ?", "mcp-call-smoke").Delete(&model.BridgeAuditLog{}).Error
		_ = model.CloseDB()
	})

	hub := bridge.NewHub()
	restoreExecutorRegistry := setMCPExecutorRegistryForTest(mcpexecutor.NewRegistry(&mcpexecutor.RemoteBridgeExecutor{
		Hub:     hub,
		Timeout: time.Second,
	}))
	t.Cleanup(restoreExecutorRegistry)

	user, token := seedMCPBillingUserAndToken(t, 100000, 100000, false)
	setMCPToolPriceForTest(t, "remote_read", 0.001)

	resp, err := CallMCPTool(MCPToolCallRequest{
		UserId:         user.Id,
		TokenId:        token.Id,
		TokenKey:       token.Key,
		TokenUnlimited: token.UnlimitedQuota,
		TokenQuota:     token.RemainQuota,
		UsingGroup:     "default",
		RequestId:      "mcp-call-smoke",
		RequestIP:      "127.0.0.1",
		Params: dto.MCPToolCallParams{
			Name: "remote_read",
			Arguments: map[string]any{
				"file_path": "README.md",
			},
		},
	})
	if err != nil {
		t.Fatalf("CallMCPTool failed: %v", err)
	}
	if resp == nil || resp.ErrorCode != dto.MCPErrorCodeBridgeUnavailable {
		t.Fatalf("expected bridge unavailable response, got %#v", resp)
	}

	var call model.MCPToolCall
	if err := model.DB.Where("request_id = ?", "mcp-call-smoke").First(&call).Error; err != nil {
		t.Fatalf("MCP tool call was not persisted: %v", err)
	}
	if call.Status != model.MCPToolCallStatusError {
		t.Fatalf("call status mismatch, got %s", call.Status)
	}
	if call.ErrorCode != "BRIDGE_CLIENT_NOT_FOUND" {
		t.Fatalf("call error code mismatch, got %s", call.ErrorCode)
	}
	if call.ToolName != "remote_read" {
		t.Fatalf("call tool name mismatch, got %s", call.ToolName)
	}
	if call.Cost != 0 || call.Quota != 0 {
		t.Fatalf("refunded bridge-unavailable call should have net zero billing, got quota=%d cost=%f", call.Quota, call.Cost)
	}
	if call.SettledAt <= 0 {
		t.Fatalf("expected settlement timestamp before bridge failure refund, got %d", call.SettledAt)
	}
	if call.BridgeSessionId != "" || call.TargetClient != "" {
		t.Fatalf("client-not-found call should not have bridge session/client, got %#v", call)
	}
	assertMCPQuotaState(t, user.Id, token.Id, 100000, 100000, 0, 0, false)
	assertMCPBillingDebitAndRefund(t, "mcp-call-smoke", call.Id, 500, "BRIDGE_CLIENT_NOT_FOUND")

	var auditCount int64
	if err := model.DB.Model(&model.BridgeAuditLog{}).Where("request_id = ?", "mcp-call-smoke").Count(&auditCount).Error; err != nil {
		t.Fatalf("count bridge audit logs failed: %v", err)
	}
	if auditCount != 0 {
		t.Fatalf("client-not-found should not create bridge audit log, got %d", auditCount)
	}
}

func TestMCPToolCallExecutorSuccessSmoke(t *testing.T) {
	if os.Getenv("MCP_MIGRATION_TEST") != "1" {
		t.Skip("set MCP_MIGRATION_TEST=1 to run the MCP tool call executor smoke test")
	}
	if os.Getenv("SQL_DSN") == "" {
		t.Fatal("SQL_DSN is required")
	}

	common.InitEnv()
	logger.SetupLogger()
	if err := model.InitDB(); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	if err := model.InitLogDB(); err != nil {
		t.Fatalf("InitLogDB failed: %v", err)
	}
	t.Cleanup(func() {
		_ = model.DB.Where("request_id = ?", "mcp-call-success-smoke").Delete(&model.MCPToolCall{}).Error
		_ = model.DB.Where("request_id = ?", "mcp-call-success-smoke").Delete(&model.BillingEvent{}).Error
		_ = model.CloseDB()
	})

	restoreExecutorRegistry := setMCPExecutorRegistryForTest(mcpexecutor.NewRegistry(successMCPExecutor{}))
	t.Cleanup(restoreExecutorRegistry)

	user, token := seedMCPBillingUserAndToken(t, 100000, 100000, false)
	tool := setMCPToolPriceForTest(t, "remote_read", 0.001)

	resp, err := CallMCPTool(MCPToolCallRequest{
		UserId:         user.Id,
		TokenId:        token.Id,
		TokenKey:       token.Key,
		TokenUnlimited: token.UnlimitedQuota,
		TokenQuota:     token.RemainQuota,
		UsingGroup:     "default",
		RequestId:      "mcp-call-success-smoke",
		RequestIP:      "127.0.0.1",
		Params: dto.MCPToolCallParams{
			Name: "remote_read",
			Arguments: map[string]any{
				"file_path": "README.md",
			},
		},
	})
	if err != nil {
		t.Fatalf("CallMCPTool failed: %v", err)
	}
	if resp == nil || resp.Result == nil {
		t.Fatalf("expected successful result, got %#v", resp)
	}
	if len(resp.Result.Content) != 1 || resp.Result.Content[0].Text != "executor-ok" {
		t.Fatalf("result content mismatch, got %#v", resp.Result.Content)
	}

	var call model.MCPToolCall
	if err := model.DB.Where("request_id = ?", "mcp-call-success-smoke").First(&call).Error; err != nil {
		t.Fatalf("MCP tool call was not persisted: %v", err)
	}
	if call.Status != model.MCPToolCallStatusSuccess {
		t.Fatalf("call status mismatch, got %s", call.Status)
	}
	if call.ResultSummary != "executor-ok" {
		t.Fatalf("call result summary mismatch, got %s", call.ResultSummary)
	}
	if call.ErrorCode != "" {
		t.Fatalf("call error code should be empty, got %s", call.ErrorCode)
	}
	if tool.PricePerCall > 0 && call.Cost != tool.PricePerCall {
		t.Fatalf("call cost mismatch, got %f want %f", call.Cost, tool.PricePerCall)
	}
	if call.Quota != 500 {
		t.Fatalf("call quota mismatch, got %d want 500", call.Quota)
	}
	if call.SettledAt <= 0 {
		t.Fatalf("expected settled_at to be set, got %d", call.SettledAt)
	}
	assertMCPQuotaSettled(t, user.Id, token.Id, 100000, 100000, 500, false)
	assertMCPBillingEvent(t, "mcp-call-success-smoke", call.Id, user.Id, token.Id, 500)

	items, total, err := ListMCPToolCallsForAdmin(MCPToolCallListParams{
		UserId:    user.Id,
		RequestId: "mcp-call-success-smoke",
		Status:    model.MCPToolCallStatusSuccess,
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("ListMCPToolCallsForAdmin failed: %v", err)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("expected one MCP tool call, total=%d len=%d", total, len(items))
	}
	if items[0].RequestId != "mcp-call-success-smoke" || items[0].Quota != 500 || items[0].SettledAt <= 0 {
		t.Fatalf("MCP tool call list item mismatch: %#v", items[0])
	}
}

func TestMCPToolCallRefundsPreSettlementOnExecutorError(t *testing.T) {
	truncate(t)
	withServiceTestQuotaPerUnit(t, 500000)
	restoreExecutorRegistry := setMCPExecutorRegistryForTest(mcpexecutor.NewRegistry(failingMCPExecutor{}))
	t.Cleanup(restoreExecutorRegistry)

	user, token := seedMCPBillingUserAndToken(t, 100000, 100000, false)
	setMCPToolPriceForTest(t, "remote_read", 0.001)

	resp, err := CallMCPTool(MCPToolCallRequest{
		UserId:         user.Id,
		TokenId:        token.Id,
		TokenKey:       token.Key,
		TokenUnlimited: token.UnlimitedQuota,
		TokenQuota:     token.RemainQuota,
		UsingGroup:     "default",
		RequestId:      "mcp-refund-on-executor-error",
		RequestIP:      "127.0.0.1",
		Params: dto.MCPToolCallParams{
			Name:      "remote_read",
			Arguments: map[string]any{"file_path": "README.md"},
		},
	})
	if err != nil {
		t.Fatalf("CallMCPTool failed: %v", err)
	}
	if resp == nil || resp.ErrorCode != dto.MCPErrorCodeExecutorFailed {
		t.Fatalf("expected executor failed response, got %#v", resp)
	}

	var call model.MCPToolCall
	if err := model.DB.Where("request_id = ?", "mcp-refund-on-executor-error").First(&call).Error; err != nil {
		t.Fatalf("MCP tool call was not persisted: %v", err)
	}
	if call.Status != model.MCPToolCallStatusError {
		t.Fatalf("call status mismatch, got %s", call.Status)
	}
	if call.SettledAt <= 0 {
		t.Fatalf("expected pre-settlement timestamp before executor, got %d", call.SettledAt)
	}
	if call.Quota != 0 || call.Cost != 0 {
		t.Fatalf("refunded failed call should have net zero billing, got quota=%d cost=%f", call.Quota, call.Cost)
	}
	assertMCPQuotaState(t, user.Id, token.Id, 100000, 100000, 0, 0, false)

	var events []model.BillingEvent
	if err := model.DB.Where("source = ? AND request_id = ?", model.BillingEventSourceMCPToolCall, "mcp-refund-on-executor-error").
		Order("id asc").
		Find(&events).Error; err != nil {
		t.Fatalf("list MCP billing events failed: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected settlement and refund events, got %d: %#v", len(events), events)
	}
	if events[0].EventType != model.BillingEventTypeDebit || events[0].QuotaDelta != -500 {
		t.Fatalf("settlement event mismatch: %#v", events[0])
	}
	if events[1].EventType != model.BillingEventTypeCredit || events[1].QuotaDelta != 500 {
		t.Fatalf("refund event mismatch: %#v", events[1])
	}
}

func TestMCPBuiltinJSONPrettyCall(t *testing.T) {
	truncate(t)
	withServiceTestQuotaPerUnit(t, 500000)
	user, token := seedMCPBillingUserAndToken(t, 100000, 100000, false)

	resp, err := CallMCPTool(MCPToolCallRequest{
		UserId:         user.Id,
		TokenId:        token.Id,
		TokenKey:       token.Key,
		TokenUnlimited: token.UnlimitedQuota,
		TokenQuota:     token.RemainQuota,
		UsingGroup:     "default",
		RequestId:      "mcp-builtin-json-pretty",
		RequestIP:      "127.0.0.1",
		Params: dto.MCPToolCallParams{
			Name: "json_pretty",
			Arguments: map[string]any{
				"json":   `{"b":2,"a":1}`,
				"indent": 2,
			},
		},
	})
	if err != nil {
		t.Fatalf("CallMCPTool failed: %v", err)
	}
	if resp == nil || resp.Result == nil || len(resp.Result.Content) != 1 {
		t.Fatalf("expected MCP result, got %#v", resp)
	}
	if resp.Result.Content[0].Text != "{\n  \"a\": 1,\n  \"b\": 2\n}" {
		t.Fatalf("json_pretty result mismatch: %s", resp.Result.Content[0].Text)
	}

	var call model.MCPToolCall
	if err := model.DB.Where("request_id = ?", "mcp-builtin-json-pretty").First(&call).Error; err != nil {
		t.Fatalf("load MCP tool call failed: %v", err)
	}
	if call.Status != model.MCPToolCallStatusSuccess {
		t.Fatalf("call status mismatch, got %s", call.Status)
	}
	if call.Quota != 0 || call.Cost != 0 {
		t.Fatalf("free built-in call should not charge quota: %#v", call)
	}
	if call.ResultSummary != "formatted JSON (22 bytes)" {
		t.Fatalf("summary mismatch, got %s", call.ResultSummary)
	}
	assertNoBillingEventForRequest(t, "mcp-builtin-json-pretty")
}

func TestMCPToolCallSettlementPersistsCost(t *testing.T) {
	truncate(t)
	withServiceTestQuotaPerUnit(t, 500000)

	user, token := seedMCPBillingUserAndToken(t, 100000, 100000, false)
	tool := setMCPToolPriceForTest(t, "remote_read", 0.001)
	call := &model.MCPToolCall{
		UserId:    user.Id,
		TokenId:   token.Id,
		ToolId:    tool.Id,
		ToolName:  tool.Name,
		RequestId: "mcp-settlement-persists-cost",
		Status:    model.MCPToolCallStatusPending,
	}
	if err := model.CreateMCPToolCall(call); err != nil {
		t.Fatalf("create MCP tool call failed: %v", err)
	}

	settled, err := model.SettleMCPToolCallQuota(call.Id, user.Id, token.Id, 500, tool.PricePerCall, false, tool.PriceUnit)
	if err != nil {
		t.Fatalf("settlement failed: %v", err)
	}
	if !settled {
		t.Fatal("expected settlement to apply")
	}

	var persisted model.MCPToolCall
	if err := model.DB.Where("id = ?", call.Id).First(&persisted).Error; err != nil {
		t.Fatalf("load settled MCP tool call failed: %v", err)
	}
	if persisted.Cost != tool.PricePerCall {
		t.Fatalf("persisted call cost mismatch, got %f want %f", persisted.Cost, tool.PricePerCall)
	}
	if persisted.Quota != 500 || persisted.SettledAt <= 0 {
		t.Fatalf("persisted settlement fields mismatch: %#v", persisted)
	}
	assertMCPQuotaSettled(t, user.Id, token.Id, 100000, 100000, 500, false)
	assertMCPBillingEvent(t, "mcp-settlement-persists-cost", call.Id, user.Id, token.Id, 500)
}

func TestMCPToolCallFreeQuotaAppliesOnce(t *testing.T) {
	truncate(t)
	withServiceTestQuotaPerUnit(t, 500000)
	restoreExecutorRegistry := setMCPExecutorRegistryForTest(mcpexecutor.NewRegistry(successMCPExecutor{}))
	t.Cleanup(restoreExecutorRegistry)

	user, token := seedMCPBillingUserAndToken(t, 100000, 100000, false)
	tool := setMCPToolPriceAndFreeQuotaForTest(t, "remote_read", 0.001, 1)

	firstResp, err := CallMCPTool(MCPToolCallRequest{
		UserId:         user.Id,
		TokenId:        token.Id,
		TokenKey:       token.Key,
		TokenUnlimited: token.UnlimitedQuota,
		TokenQuota:     token.RemainQuota,
		UsingGroup:     "default",
		RequestId:      "mcp-free-quota-first",
		RequestIP:      "127.0.0.1",
		Params: dto.MCPToolCallParams{
			Name:      tool.Name,
			Arguments: map[string]any{"file_path": "README.md"},
		},
	})
	if err != nil {
		t.Fatalf("first CallMCPTool failed: %v", err)
	}
	if firstResp == nil || firstResp.Result == nil {
		t.Fatalf("expected first successful result, got %#v", firstResp)
	}

	var firstCall model.MCPToolCall
	if err := model.DB.Where("request_id = ?", "mcp-free-quota-first").First(&firstCall).Error; err != nil {
		t.Fatalf("load first MCP tool call failed: %v", err)
	}
	if !firstCall.FreeUsed || firstCall.Quota != 0 || firstCall.Cost != 0 || firstCall.SettledAt <= 0 {
		t.Fatalf("first free call mismatch: %#v", firstCall)
	}
	assertMCPQuotaState(t, user.Id, token.Id, 100000, 100000, 0, 0, false)

	secondResp, err := CallMCPTool(MCPToolCallRequest{
		UserId:         user.Id,
		TokenId:        token.Id,
		TokenKey:       token.Key,
		TokenUnlimited: token.UnlimitedQuota,
		TokenQuota:     token.RemainQuota,
		UsingGroup:     "default",
		RequestId:      "mcp-free-quota-second",
		RequestIP:      "127.0.0.1",
		Params: dto.MCPToolCallParams{
			Name:      tool.Name,
			Arguments: map[string]any{"file_path": "README.md"},
		},
	})
	if err != nil {
		t.Fatalf("second CallMCPTool failed: %v", err)
	}
	if secondResp == nil || secondResp.Result == nil {
		t.Fatalf("expected second successful result, got %#v", secondResp)
	}

	var secondCall model.MCPToolCall
	if err := model.DB.Where("request_id = ?", "mcp-free-quota-second").First(&secondCall).Error; err != nil {
		t.Fatalf("load second MCP tool call failed: %v", err)
	}
	if secondCall.FreeUsed || secondCall.Quota != 500 || secondCall.SettledAt <= 0 {
		t.Fatalf("second paid call mismatch: %#v", secondCall)
	}
	assertMCPQuotaState(t, user.Id, token.Id, 100000, 100000, 500, 1, false)
	assertMCPBillingEvent(t, "mcp-free-quota-second", secondCall.Id, user.Id, token.Id, 500)

	var freeEventCount int64
	if err := model.DB.Model(&model.BillingEvent{}).Where("request_id = ?", "mcp-free-quota-first").Count(&freeEventCount).Error; err != nil {
		t.Fatalf("count first free billing events failed: %v", err)
	}
	if freeEventCount != 0 {
		t.Fatalf("free MCP call should not create debit billing event, got %d", freeEventCount)
	}
}

func TestMCPToolCallRemoteBridgeExecutorSmoke(t *testing.T) {
	if os.Getenv("MCP_MIGRATION_TEST") != "1" {
		t.Skip("set MCP_MIGRATION_TEST=1 to run the MCP remote bridge executor smoke test")
	}
	if os.Getenv("SQL_DSN") == "" {
		t.Fatal("SQL_DSN is required")
	}

	common.InitEnv()
	logger.SetupLogger()
	if err := model.InitDB(); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	if err := model.InitLogDB(); err != nil {
		t.Fatalf("InitLogDB failed: %v", err)
	}
	t.Cleanup(func() {
		_, _ = bridge.DefaultHub.Unregister("mcp-bridge-session-smoke")
		_ = model.DB.Where("request_id = ?", "mcp-bridge-call-smoke").Delete(&model.MCPToolCall{}).Error
		_ = model.DB.Where("request_id = ?", "mcp-bridge-call-smoke").Delete(&model.BridgeAuditLog{}).Error
		_ = model.DB.Where("request_id = ?", "mcp-bridge-call-smoke").Delete(&model.BillingEvent{}).Error
		_ = model.CloseDB()
	})

	user, token := seedMCPBillingUserAndToken(t, 100000, 100000, false)
	tool := setMCPToolPriceForTest(t, "remote_read", 0.001)
	outbound := make(chan bridge.OutboundMessage, 1)
	bridge.DefaultHub.Register(bridge.Session{
		SessionId:    "mcp-bridge-session-smoke",
		ClientId:     "mcp-bridge-client-smoke",
		UserId:       user.Id,
		TokenId:      token.Id,
		Capabilities: []string{"remote_read"},
		Send:         outbound,
	})

	done := make(chan struct{})
	go func() {
		defer close(done)
		msg := <-outbound
		if msg.Type != bridge.MessageTypeToolCall {
			return
		}
		bridge.DefaultHub.CompleteToolCall(msg.Id, dto.BridgeToolCallResult{
			Content: []dto.MCPContentBlock{{Type: "text", Text: "bridge-ok"}},
			Summary: "bridge-ok",
		})
	}()

	resp, err := CallMCPTool(MCPToolCallRequest{
		UserId:         user.Id,
		TokenId:        token.Id,
		TokenKey:       token.Key,
		TokenUnlimited: token.UnlimitedQuota,
		TokenQuota:     token.RemainQuota,
		UsingGroup:     "default",
		RequestId:      "mcp-bridge-call-smoke",
		RequestIP:      "127.0.0.1",
		Params: dto.MCPToolCallParams{
			Name: "remote_read",
			Arguments: map[string]any{
				"file_path": "README.md",
			},
		},
	})
	<-done
	if err != nil {
		t.Fatalf("CallMCPTool failed: %v", err)
	}
	if resp == nil || resp.Result == nil {
		t.Fatalf("expected bridge result, got %#v", resp)
	}
	if len(resp.Result.Content) != 1 || resp.Result.Content[0].Text != "bridge-ok" {
		t.Fatalf("bridge result mismatch, got %#v", resp.Result.Content)
	}

	var call model.MCPToolCall
	if err := model.DB.Where("request_id = ?", "mcp-bridge-call-smoke").First(&call).Error; err != nil {
		t.Fatalf("MCP tool call was not persisted: %v", err)
	}
	if call.Status != model.MCPToolCallStatusSuccess {
		t.Fatalf("call status mismatch, got %s", call.Status)
	}
	if call.BridgeSessionId != "mcp-bridge-session-smoke" {
		t.Fatalf("bridge session mismatch, got %s", call.BridgeSessionId)
	}
	if call.TargetClient != "mcp-bridge-client-smoke" {
		t.Fatalf("target client mismatch, got %s", call.TargetClient)
	}
	if tool.PricePerCall > 0 && call.Cost != tool.PricePerCall {
		t.Fatalf("call cost mismatch, got %f want %f", call.Cost, tool.PricePerCall)
	}
	if call.Quota != 500 {
		t.Fatalf("call quota mismatch, got %d want 500", call.Quota)
	}
	if call.SettledAt <= 0 {
		t.Fatalf("expected settled_at to be set, got %d", call.SettledAt)
	}
	assertMCPQuotaSettled(t, user.Id, token.Id, 100000, 100000, 500, false)
	assertMCPBillingEvent(t, "mcp-bridge-call-smoke", call.Id, user.Id, token.Id, 500)

	var audit model.BridgeAuditLog
	if err := model.DB.Where("request_id = ?", "mcp-bridge-call-smoke").First(&audit).Error; err != nil {
		t.Fatalf("bridge audit log was not persisted: %v", err)
	}
	if audit.Status != model.BridgeAuditStatusSuccess {
		t.Fatalf("audit status mismatch, got %s", audit.Status)
	}

	callItems, callTotal, err := ListMCPToolCallsForAdmin(MCPToolCallListParams{
		UserId:          user.Id,
		RequestId:       "mcp-bridge-call-smoke",
		BridgeSessionId: "mcp-bridge-session-smoke",
		TargetClient:    "mcp-bridge-client-smoke",
		Limit:           10,
	})
	if err != nil {
		t.Fatalf("ListMCPToolCallsForAdmin bridge query failed: %v", err)
	}
	if callTotal != 1 || len(callItems) != 1 {
		t.Fatalf("expected one bridge MCP tool call, total=%d len=%d", callTotal, len(callItems))
	}
	if callItems[0].BridgeSessionId != "mcp-bridge-session-smoke" || callItems[0].TargetClient != "mcp-bridge-client-smoke" {
		t.Fatalf("bridge MCP tool call list item mismatch: %#v", callItems[0])
	}
}

func TestMCPToolCallRemoteBridgeToolErrorRefundsBilling(t *testing.T) {
	truncate(t)
	withServiceTestQuotaPerUnit(t, 500000)

	hub := bridge.NewHub()
	restoreExecutorRegistry := setMCPExecutorRegistryForTest(mcpexecutor.NewRegistry(&mcpexecutor.RemoteBridgeExecutor{
		Hub:     hub,
		Timeout: time.Second,
	}))
	t.Cleanup(restoreExecutorRegistry)

	user, token := seedMCPBillingUserAndToken(t, 100000, 100000, false)
	setMCPToolPriceForTest(t, "remote_read", 0.001)
	outbound := make(chan bridge.OutboundMessage, 1)
	hub.Register(bridge.Session{
		SessionId:    "mcp-bridge-session-tool-error",
		ClientId:     "mcp-bridge-client-tool-error",
		UserId:       user.Id,
		TokenId:      token.Id,
		Capabilities: []string{"remote_read"},
		Send:         outbound,
	})

	done := make(chan struct{})
	go func() {
		defer close(done)
		msg := <-outbound
		if msg.Type != bridge.MessageTypeToolCall {
			return
		}
		hub.FailToolCall(msg.Id, "REMOTE_PERMISSION_DENIED", "mock bridge denied remote_read")
	}()

	resp, err := CallMCPTool(MCPToolCallRequest{
		UserId:         user.Id,
		TokenId:        token.Id,
		TokenKey:       token.Key,
		TokenUnlimited: token.UnlimitedQuota,
		TokenQuota:     token.RemainQuota,
		UsingGroup:     "default",
		RequestId:      "mcp-bridge-tool-error",
		RequestIP:      "127.0.0.1",
		Params: dto.MCPToolCallParams{
			Name:      "remote_read",
			Arguments: map[string]any{"file_path": "README.md"},
		},
	})
	<-done
	if err != nil {
		t.Fatalf("CallMCPTool failed: %v", err)
	}
	if resp == nil || resp.ErrorCode != dto.MCPErrorCodeExecutorFailed {
		t.Fatalf("expected executor failed response, got %#v", resp)
	}

	var call model.MCPToolCall
	if err := model.DB.Where("request_id = ?", "mcp-bridge-tool-error").First(&call).Error; err != nil {
		t.Fatalf("MCP tool call was not persisted: %v", err)
	}
	if call.Status != model.MCPToolCallStatusError || call.ErrorCode != "REMOTE_PERMISSION_DENIED" {
		t.Fatalf("call error state mismatch: %#v", call)
	}
	if call.BridgeSessionId != "mcp-bridge-session-tool-error" || call.TargetClient != "mcp-bridge-client-tool-error" {
		t.Fatalf("bridge call linkage mismatch: %#v", call)
	}
	if call.Quota != 0 || call.Cost != 0 || call.SettledAt <= 0 {
		t.Fatalf("tool_error should be refunded but retain settlement timestamp: %#v", call)
	}
	assertMCPQuotaState(t, user.Id, token.Id, 100000, 100000, 0, 0, false)
	assertMCPBillingDebitAndRefund(t, "mcp-bridge-tool-error", call.Id, 500, "REMOTE_PERMISSION_DENIED")

	var audit model.BridgeAuditLog
	if err := model.DB.Where("request_id = ?", "mcp-bridge-tool-error").First(&audit).Error; err != nil {
		t.Fatalf("bridge audit log was not persisted: %v", err)
	}
	if audit.Status != model.BridgeAuditStatusError || audit.ErrorCode != "REMOTE_PERMISSION_DENIED" {
		t.Fatalf("bridge audit error mismatch: %#v", audit)
	}
	if audit.SessionId != call.BridgeSessionId || audit.ClientId != call.TargetClient {
		t.Fatalf("bridge audit linkage mismatch: %#v", audit)
	}
}

func TestMCPToolCallRemoteBridgeTimeoutRefundsBilling(t *testing.T) {
	truncate(t)
	withServiceTestQuotaPerUnit(t, 500000)

	hub := bridge.NewHub()
	restoreExecutorRegistry := setMCPExecutorRegistryForTest(mcpexecutor.NewRegistry(&mcpexecutor.RemoteBridgeExecutor{
		Hub:     hub,
		Timeout: 20 * time.Millisecond,
	}))
	t.Cleanup(restoreExecutorRegistry)

	user, token := seedMCPBillingUserAndToken(t, 100000, 100000, false)
	setMCPToolPriceForTest(t, "remote_read", 0.001)
	outbound := make(chan bridge.OutboundMessage, 1)
	hub.Register(bridge.Session{
		SessionId:    "mcp-bridge-session-timeout",
		ClientId:     "mcp-bridge-client-timeout",
		UserId:       user.Id,
		TokenId:      token.Id,
		Capabilities: []string{"remote_read"},
		Send:         outbound,
	})

	resp, err := CallMCPTool(MCPToolCallRequest{
		UserId:         user.Id,
		TokenId:        token.Id,
		TokenKey:       token.Key,
		TokenUnlimited: token.UnlimitedQuota,
		TokenQuota:     token.RemainQuota,
		UsingGroup:     "default",
		RequestId:      "mcp-bridge-timeout",
		RequestIP:      "127.0.0.1",
		Params: dto.MCPToolCallParams{
			Name:      "remote_read",
			Arguments: map[string]any{"file_path": "README.md"},
		},
	})
	if err != nil {
		t.Fatalf("CallMCPTool failed: %v", err)
	}
	if resp == nil || resp.ErrorCode != dto.MCPErrorCodeExecutorTimeout {
		t.Fatalf("expected executor timeout response, got %#v", resp)
	}
	select {
	case msg := <-outbound:
		if msg.Type != bridge.MessageTypeToolCall || msg.Id != "mcp-bridge-timeout" {
			t.Fatalf("unexpected outbound timeout message: %#v", msg)
		}
	default:
		t.Fatal("expected outbound bridge tool_call before timeout")
	}

	var call model.MCPToolCall
	if err := model.DB.Where("request_id = ?", "mcp-bridge-timeout").First(&call).Error; err != nil {
		t.Fatalf("MCP tool call was not persisted: %v", err)
	}
	if call.Status != model.MCPToolCallStatusTimeout || call.ErrorCode != mcpexecutor.ErrorCodeTimeout {
		t.Fatalf("call timeout state mismatch: %#v", call)
	}
	if call.BridgeSessionId != "mcp-bridge-session-timeout" || call.TargetClient != "mcp-bridge-client-timeout" {
		t.Fatalf("bridge timeout linkage mismatch: %#v", call)
	}
	if call.Quota != 0 || call.Cost != 0 || call.SettledAt <= 0 {
		t.Fatalf("timeout should be refunded but retain settlement timestamp: %#v", call)
	}
	assertMCPQuotaState(t, user.Id, token.Id, 100000, 100000, 0, 0, false)
	assertMCPBillingDebitAndRefund(t, "mcp-bridge-timeout", call.Id, 500, mcpexecutor.ErrorCodeTimeout)

	var audit model.BridgeAuditLog
	if err := model.DB.Where("request_id = ?", "mcp-bridge-timeout").First(&audit).Error; err != nil {
		t.Fatalf("bridge audit log was not persisted: %v", err)
	}
	if audit.Status != model.BridgeAuditStatusTimeout || audit.ErrorCode != mcpexecutor.ErrorCodeTimeout {
		t.Fatalf("bridge audit timeout mismatch: %#v", audit)
	}
	if audit.SessionId != call.BridgeSessionId || audit.ClientId != call.TargetClient {
		t.Fatalf("bridge audit timeout linkage mismatch: %#v", audit)
	}
}

func TestMCPToolBillingPrecheckInsufficientUserQuotaSmoke(t *testing.T) {
	if os.Getenv("MCP_MIGRATION_TEST") != "1" {
		t.Skip("set MCP_MIGRATION_TEST=1 to run the MCP billing precheck smoke test")
	}
	if os.Getenv("SQL_DSN") == "" {
		t.Fatal("SQL_DSN is required")
	}

	common.InitEnv()
	logger.SetupLogger()
	if err := model.InitDB(); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	if err := model.InitLogDB(); err != nil {
		t.Fatalf("InitLogDB failed: %v", err)
	}
	t.Cleanup(func() {
		_ = model.DB.Where("request_id = ?", "mcp-billing-smoke").Delete(&model.MCPToolCall{}).Error
		_ = model.CloseDB()
	})

	user, token := seedMCPBillingUserAndToken(t, 0, 100000, false)
	setMCPToolPriceForTest(t, "remote_read", 0.001)

	resp, err := CallMCPTool(MCPToolCallRequest{
		UserId:         user.Id,
		TokenId:        token.Id,
		TokenKey:       token.Key,
		TokenUnlimited: token.UnlimitedQuota,
		TokenQuota:     token.RemainQuota,
		UsingGroup:     "default",
		RequestId:      "mcp-billing-smoke",
		RequestIP:      "127.0.0.1",
		Params: dto.MCPToolCallParams{
			Name: "remote_read",
			Arguments: map[string]any{
				"file_path": "README.md",
			},
		},
	})
	if err != nil {
		t.Fatalf("CallMCPTool failed: %v", err)
	}
	if resp == nil || resp.ErrorCode != dto.MCPErrorCodeBillingFailed {
		t.Fatalf("expected billing failed response, got %#v", resp)
	}

	var call model.MCPToolCall
	if err := model.DB.Where("request_id = ?", "mcp-billing-smoke").First(&call).Error; err != nil {
		t.Fatalf("MCP tool call was not persisted: %v", err)
	}
	if call.ErrorCode != "INSUFFICIENT_USER_QUOTA" {
		t.Fatalf("call error code mismatch, got %s", call.ErrorCode)
	}
	if call.Cost <= 0 {
		t.Fatalf("expected positive cost, got %f", call.Cost)
	}
	if call.Quota != 500 {
		t.Fatalf("call quota mismatch, got %d want 500", call.Quota)
	}
	if call.SettledAt != 0 {
		t.Fatalf("failed billing precheck should not be settled, got %d", call.SettledAt)
	}
}

func TestMCPToolBillingSettlementIdempotentSmoke(t *testing.T) {
	if os.Getenv("MCP_MIGRATION_TEST") != "1" {
		t.Skip("set MCP_MIGRATION_TEST=1 to run the MCP billing settlement idempotency smoke test")
	}
	if os.Getenv("SQL_DSN") == "" {
		t.Fatal("SQL_DSN is required")
	}

	common.InitEnv()
	logger.SetupLogger()
	if err := model.InitDB(); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	if err := model.InitLogDB(); err != nil {
		t.Fatalf("InitLogDB failed: %v", err)
	}
	t.Cleanup(func() {
		_ = model.DB.Where("request_id = ?", "mcp-settlement-idempotent-smoke").Delete(&model.MCPToolCall{}).Error
		_ = model.DB.Where("request_id = ?", "mcp-settlement-idempotent-smoke").Delete(&model.BillingEvent{}).Error
		_ = model.CloseDB()
	})

	user, token := seedMCPBillingUserAndToken(t, 100000, 100000, false)
	tool := setMCPToolPriceForTest(t, "remote_read", 0.001)
	call := &model.MCPToolCall{
		UserId:    user.Id,
		TokenId:   token.Id,
		ToolId:    tool.Id,
		ToolName:  tool.Name,
		RequestId: "mcp-settlement-idempotent-smoke",
		Status:    model.MCPToolCallStatusSuccess,
		Cost:      tool.PricePerCall,
		Quota:     500,
		Metadata:  `{"proxy_server_namespace":"github","downstream_tool_name":"search_repos","transport":"http"}`,
	}
	if err := model.CreateMCPToolCall(call); err != nil {
		t.Fatalf("create MCP tool call failed: %v", err)
	}

	settled, err := model.SettleMCPToolCallQuota(call.Id, user.Id, token.Id, 500, tool.PricePerCall, false, tool.PriceUnit)
	if err != nil {
		t.Fatalf("first settlement failed: %v", err)
	}
	if !settled {
		t.Fatal("expected first settlement to apply")
	}
	settled, err = model.SettleMCPToolCallQuota(call.Id, user.Id, token.Id, 500, tool.PricePerCall, false, tool.PriceUnit)
	if err != nil {
		t.Fatalf("second settlement failed: %v", err)
	}
	if settled {
		t.Fatal("expected second settlement to be ignored")
	}
	assertMCPQuotaSettled(t, user.Id, token.Id, 100000, 100000, 500, false)
	assertMCPBillingEvent(t, "mcp-settlement-idempotent-smoke", call.Id, user.Id, token.Id, 500)

	events, total, err := ListBillingEvents(BillingEventListParams{
		UserId:    user.Id,
		RequestId: "mcp-settlement-idempotent-smoke",
		Source:    model.BillingEventSourceMCPToolCall,
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("ListBillingEvents failed: %v", err)
	}
	if total != 1 || len(events) != 1 {
		t.Fatalf("expected one billing event, total=%d len=%d", total, len(events))
	}
	if events[0].QuotaDelta != -500 || events[0].AmountQuota != 500 {
		t.Fatalf("billing event quota mismatch: %#v", events[0])
	}
	if events[0].RelatedMCPToolCall == nil {
		t.Fatal("expected related MCP tool call link")
	}
	if events[0].RelatedMCPToolCall.Id != call.Id || events[0].RelatedMCPToolCall.ToolName != tool.Name {
		t.Fatalf("related MCP tool call mismatch: %#v", events[0].RelatedMCPToolCall)
	}
	if events[0].RelatedMCPToolCall.Metadata != call.Metadata {
		t.Fatalf("related MCP tool call metadata mismatch: %#v", events[0].RelatedMCPToolCall)
	}
	events, total, err = ListBillingEvents(BillingEventListParams{
		UserId:   user.Id,
		Source:   model.BillingEventSourceMCPToolCall,
		SourceId: fmt.Sprintf("%d", call.Id),
		Limit:    10,
	})
	if err != nil {
		t.Fatalf("ListBillingEvents by source_id failed: %v", err)
	}
	if total != 1 || len(events) != 1 {
		t.Fatalf("expected one billing event by source_id, total=%d len=%d", total, len(events))
	}
	if events[0].SourceId != fmt.Sprintf("%d", call.Id) {
		t.Fatalf("billing event source_id mismatch: %#v", events[0])
	}
}

func TestMCPSummarySmoke(t *testing.T) {
	if os.Getenv("MCP_MIGRATION_TEST") != "1" {
		t.Skip("set MCP_MIGRATION_TEST=1 to run the MCP summary smoke test")
	}
	if os.Getenv("SQL_DSN") == "" {
		t.Fatal("SQL_DSN is required")
	}

	common.InitEnv()
	logger.SetupLogger()
	if err := model.InitDB(); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	if err := model.InitLogDB(); err != nil {
		t.Fatalf("InitLogDB failed: %v", err)
	}
	t.Cleanup(func() {
		_ = model.DB.Where("request_id IN ?", []string{"mcp-summary-success", "mcp-summary-error"}).Delete(&model.MCPToolCall{}).Error
		_ = model.DB.Where("request_id = ?", "mcp-summary-audit-error").Delete(&model.BridgeAuditLog{}).Error
		_ = model.DB.Where("request_id IN ?", []string{"mcp-summary-success", "mcp-summary-error"}).Delete(&model.BillingEvent{}).Error
		_ = model.CloseDB()
	})

	user, token := seedMCPBillingUserAndToken(t, 100000, 100000, false)
	tool := setMCPToolPriceForTest(t, "remote_read", 0.001)
	successCall := &model.MCPToolCall{
		UserId:    user.Id,
		TokenId:   token.Id,
		ToolId:    tool.Id,
		ToolName:  tool.Name,
		RequestId: "mcp-summary-success",
		Status:    model.MCPToolCallStatusSuccess,
		Cost:      tool.PricePerCall,
		Quota:     500,
	}
	if err := model.CreateMCPToolCall(successCall); err != nil {
		t.Fatalf("create summary success call failed: %v", err)
	}
	if _, err := model.SettleMCPToolCallQuota(successCall.Id, user.Id, token.Id, 500, tool.PricePerCall, false, tool.PriceUnit); err != nil {
		t.Fatalf("settle summary success call failed: %v", err)
	}
	errorCall := &model.MCPToolCall{
		UserId:       user.Id,
		TokenId:      token.Id,
		ToolId:       tool.Id,
		ToolName:     tool.Name,
		RequestId:    "mcp-summary-error",
		Status:       model.MCPToolCallStatusError,
		ErrorCode:    "SUMMARY_SMOKE",
		ErrorMessage: "summary smoke error",
		Quota:        500,
		Cost:         tool.PricePerCall,
	}
	if err := model.CreateMCPToolCall(errorCall); err != nil {
		t.Fatalf("create summary error call failed: %v", err)
	}
	audit := &model.BridgeAuditLog{
		RequestId:    "mcp-summary-audit-error",
		SessionId:    "mcp-summary-session",
		ClientId:     "mcp-summary-client",
		UserId:       user.Id,
		TokenId:      token.Id,
		ToolName:     tool.Name,
		Status:       model.BridgeAuditStatusError,
		ErrorCode:    "SUMMARY_AUDIT",
		ErrorMessage: "summary audit error",
		DurationMS:   9,
		ResultSize:   128,
	}
	if err := model.CreateBridgeAuditLog(audit); err != nil {
		t.Fatalf("create summary audit log failed: %v", err)
	}

	summary, err := GetMCPSummaryForAdmin(MCPSummaryParams{
		UserId:        user.Id,
		WindowSeconds: 24 * 60 * 60,
	})
	if err != nil {
		t.Fatalf("GetMCPSummaryForAdmin failed: %v", err)
	}
	if summary.Calls.TotalCalls < 2 {
		t.Fatalf("expected at least two summary calls, got %#v", summary.Calls)
	}
	if summary.Calls.Quota < 1000 {
		t.Fatalf("expected summary quota >= 1000, got %#v", summary.Calls)
	}
	if len(summary.TopTools) == 0 {
		t.Fatalf("expected top tools in summary, got %#v", summary.TopTools)
	}
	if len(summary.RecentErrors) == 0 {
		t.Fatalf("expected recent errors in summary, got %#v", summary.RecentErrors)
	}
}

type successMCPExecutor struct{}

func (successMCPExecutor) Supports(tool model.MCPTool) bool {
	return tool.Name == "remote_read"
}

func (successMCPExecutor) Execute(ctx context.Context, req mcpexecutor.Request) (mcpexecutor.Result, error) {
	return mcpexecutor.Result{
		Content: []dto.MCPContentBlock{
			{Type: "text", Text: "executor-ok"},
		},
		Metadata: map[string]any{
			"request_id": req.RequestId,
		},
		Summary:    "executor-ok",
		DurationMS: 7,
	}, nil
}

type failingMCPExecutor struct{}

func (failingMCPExecutor) Supports(tool model.MCPTool) bool {
	return tool.Name == "remote_read"
}

func (failingMCPExecutor) Execute(ctx context.Context, req mcpexecutor.Request) (mcpexecutor.Result, error) {
	return mcpexecutor.Result{}, errors.New("executor failed after billing reservation")
}

func seedMCPBillingUserAndToken(t *testing.T, userQuota int, tokenQuota int, tokenUnlimited bool) (*model.User, *model.Token) {
	t.Helper()
	suffix := common.GetRandomString(10)
	user := &model.User{
		Username:    "mcp-test-" + suffix,
		Password:    "test-password",
		DisplayName: "MCP Test",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		Quota:       userQuota,
		Group:       "default",
		AffCode:     common.GetRandomString(8),
	}
	if err := model.DB.Create(user).Error; err != nil {
		t.Fatalf("create test user failed: %v", err)
	}
	token := &model.Token{
		UserId:         user.Id,
		Key:            "mcp-test-token-" + suffix,
		Status:         common.TokenStatusEnabled,
		Name:           "MCP Test Token",
		RemainQuota:    tokenQuota,
		UnlimitedQuota: tokenUnlimited,
		ExpiredTime:    -1,
	}
	if err := model.DB.Create(token).Error; err != nil {
		t.Fatalf("create test token failed: %v", err)
	}
	t.Cleanup(func() {
		_ = model.DB.Where("user_id = ?", user.Id).Delete(&model.MCPUserDailyQuota{}).Error
		_ = model.DB.Unscoped().Delete(token).Error
		_ = model.DB.Unscoped().Delete(user).Error
	})
	return user, token
}

func assertMCPBillingEvent(t *testing.T, requestId string, callId int64, userId int, tokenId int, quota int) {
	t.Helper()
	var events []model.BillingEvent
	if err := model.DB.Where("request_id = ?", requestId).Find(&events).Error; err != nil {
		t.Fatalf("query billing events failed: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected exactly one billing event for %s, got %d", requestId, len(events))
	}
	event := events[0]
	if event.EventId != "mcp_tool_call:"+fmt.Sprint(callId)+":settlement" {
		t.Fatalf("billing event id mismatch, got %s", event.EventId)
	}
	if event.Source != model.BillingEventSourceMCPToolCall {
		t.Fatalf("billing event source mismatch, got %s", event.Source)
	}
	if event.UserId != userId || event.TokenId != tokenId {
		t.Fatalf("billing event actor mismatch: %#v", event)
	}
	if event.EventType != model.BillingEventTypeDebit || event.Status != model.BillingEventStatusSettled {
		t.Fatalf("billing event state mismatch: %#v", event)
	}
	if event.AmountQuota != quota || event.QuotaDelta != -quota {
		t.Fatalf("billing event quota mismatch: %#v", event)
	}
}

func assertMCPBillingDebitAndRefund(t *testing.T, requestId string, callId int64, quota int, refundReason string) {
	t.Helper()
	var events []model.BillingEvent
	if err := model.DB.Where("source = ? AND request_id = ?", model.BillingEventSourceMCPToolCall, requestId).
		Order("id asc").
		Find(&events).Error; err != nil {
		t.Fatalf("query billing events failed: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected settlement and refund events for %s, got %d: %#v", requestId, len(events), events)
	}
	settlement := events[0]
	refund := events[1]
	if settlement.EventId != "mcp_tool_call:"+fmt.Sprint(callId)+":settlement" {
		t.Fatalf("settlement event id mismatch: %#v", settlement)
	}
	if settlement.EventType != model.BillingEventTypeDebit || settlement.AmountQuota != quota || settlement.QuotaDelta != -quota {
		t.Fatalf("settlement event quota mismatch: %#v", settlement)
	}
	if refund.EventId != "mcp_tool_call:"+fmt.Sprint(callId)+":refund" {
		t.Fatalf("refund event id mismatch: %#v", refund)
	}
	if refund.EventType != model.BillingEventTypeCredit || refund.AmountQuota != quota || refund.QuotaDelta != quota {
		t.Fatalf("refund event quota mismatch: %#v", refund)
	}
	if refund.SourceId != fmt.Sprint(callId) || settlement.SourceId != fmt.Sprint(callId) {
		t.Fatalf("billing event source id mismatch: settlement=%#v refund=%#v", settlement, refund)
	}
	if refundReason != "" && !strings.Contains(refund.Metadata, `"reason":"`+refundReason+`"`) {
		t.Fatalf("refund reason mismatch, want %s metadata=%s", refundReason, refund.Metadata)
	}
}

func setMCPToolPriceForTest(t *testing.T, toolName string, price float64) *model.MCPTool {
	t.Helper()
	tool, err := model.GetEnabledMCPToolByName(toolName)
	if err != nil {
		t.Fatalf("get MCP tool failed: %v", err)
	}
	originalPrice := tool.PricePerCall
	updated, err := model.UpdateMCPToolFields(tool.Id, map[string]any{"price_per_call": price})
	if err != nil {
		t.Fatalf("update MCP tool price failed: %v", err)
	}
	t.Cleanup(func() {
		_, _ = model.UpdateMCPToolFields(tool.Id, map[string]any{"price_per_call": originalPrice})
	})
	return updated
}

func setMCPToolPriceAndFreeQuotaForTest(t *testing.T, toolName string, price float64, freeQuota int) *model.MCPTool {
	t.Helper()
	tool, err := model.GetEnabledMCPToolByName(toolName)
	if err != nil {
		t.Fatalf("get MCP tool failed: %v", err)
	}
	originalPrice := tool.PricePerCall
	originalFreeQuota := tool.FreeQuota
	updated, err := model.UpdateMCPToolFields(tool.Id, map[string]any{
		"price_per_call": price,
		"free_quota":     freeQuota,
	})
	if err != nil {
		t.Fatalf("update MCP tool price/free quota failed: %v", err)
	}
	t.Cleanup(func() {
		_, _ = model.UpdateMCPToolFields(tool.Id, map[string]any{
			"price_per_call": originalPrice,
			"free_quota":     originalFreeQuota,
		})
	})
	return updated
}

func assertMCPQuotaSettled(t *testing.T, userId int, tokenId int, originalUserQuota int, originalTokenQuota int, chargedQuota int, tokenUnlimited bool) {
	assertMCPQuotaState(t, userId, tokenId, originalUserQuota, originalTokenQuota, chargedQuota, 1, tokenUnlimited)
}

func assertMCPQuotaState(t *testing.T, userId int, tokenId int, originalUserQuota int, originalTokenQuota int, chargedQuota int, requestCount int, tokenUnlimited bool) {
	t.Helper()
	var user model.User
	if err := model.DB.Select("quota", "used_quota", "request_count").Where("id = ?", userId).First(&user).Error; err != nil {
		t.Fatalf("load settled user failed: %v", err)
	}
	if user.Quota != originalUserQuota-chargedQuota {
		t.Fatalf("user quota mismatch, got %d want %d", user.Quota, originalUserQuota-chargedQuota)
	}
	if user.UsedQuota != chargedQuota {
		t.Fatalf("user used quota mismatch, got %d want %d", user.UsedQuota, chargedQuota)
	}
	if user.RequestCount != requestCount {
		t.Fatalf("user request count mismatch, got %d want %d", user.RequestCount, requestCount)
	}

	var token model.Token
	if err := model.DB.Select("remain_quota", "used_quota").Where("id = ?", tokenId).First(&token).Error; err != nil {
		t.Fatalf("load settled token failed: %v", err)
	}
	expectedRemainQuota := originalTokenQuota
	expectedUsedQuota := 0
	if !tokenUnlimited {
		expectedRemainQuota -= chargedQuota
		expectedUsedQuota = chargedQuota
	}
	if token.RemainQuota != expectedRemainQuota {
		t.Fatalf("token remain quota mismatch, got %d want %d", token.RemainQuota, expectedRemainQuota)
	}
	if token.UsedQuota != expectedUsedQuota {
		t.Fatalf("token used quota mismatch, got %d want %d", token.UsedQuota, expectedUsedQuota)
	}
}
