package service

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/mcp/executor"
)

func TestMCPToolFreeQuotaAppliesOncePerUserToolPerDay(t *testing.T) {
	withServiceTestQuotaPerUnit(t, 100)
	restoreExecutorRegistry := setMCPExecutorRegistryForTest(executor.NewRegistry(successMCPExecutor{}))
	t.Cleanup(restoreExecutorRegistry)

	user, token := seedMCPBillingUserAndToken(t, 1000, 1000, false)
	tool, err := model.GetEnabledMCPToolByName("remote_read")
	if err != nil {
		t.Fatalf("get MCP tool failed: %v", err)
	}
	originalPrice := tool.PricePerCall
	originalFreeQuota := tool.FreeQuota
	updated, err := model.UpdateMCPToolFields(tool.Id, map[string]any{
		"price_per_call": 1.0,
		"free_quota":     1,
	})
	if err != nil {
		t.Fatalf("update MCP tool billing failed: %v", err)
	}
	t.Cleanup(func() {
		_, _ = model.UpdateMCPToolFields(tool.Id, map[string]any{
			"price_per_call": originalPrice,
			"free_quota":     originalFreeQuota,
		})
		_ = model.DB.Where("request_id IN ?", []string{"mcp-free-quota-yesterday", "mcp-free-quota-1", "mcp-free-quota-2"}).Delete(&model.MCPToolCall{}).Error
		_ = model.DB.Where("request_id IN ?", []string{"mcp-free-quota-yesterday", "mcp-free-quota-1", "mcp-free-quota-2"}).Delete(&model.BillingEvent{}).Error
		_ = model.DB.Where("user_id = ? AND tool_id = ?", user.Id, tool.Id).Delete(&model.MCPUserDailyQuota{}).Error
	})

	yesterday := time.Now().Add(-24 * time.Hour)
	yesterdayUnix := yesterday.Unix()
	historicalCall := &model.MCPToolCall{
		UserId:        user.Id,
		TokenId:       token.Id,
		ToolId:        tool.Id,
		ToolName:      tool.Name,
		RequestId:     "mcp-free-quota-yesterday",
		Status:        model.MCPToolCallStatusSuccess,
		FreeUsed:      true,
		PriceUnit:     model.MCPToolPriceUnitPerCall,
		ResultSummary: "historical free call",
		SettledAt:     yesterdayUnix,
	}
	if err := model.DB.Create(historicalCall).Error; err != nil {
		t.Fatalf("create historical free call failed: %v", err)
	}
	if err := model.DB.Model(&model.MCPToolCall{}).
		Where("id = ?", historicalCall.Id).
		Update("created_at", yesterdayUnix).Error; err != nil {
		t.Fatalf("move historical free call to yesterday failed: %v", err)
	}
	if err := model.DB.Create(&model.MCPUserDailyQuota{
		UserId:    user.Id,
		ToolId:    tool.Id,
		QuotaDate: yesterday.Local().Format("2006-01-02"),
		UsedCount: 1,
		FreeLimit: 1,
		CreatedAt: yesterdayUnix,
		UpdatedAt: yesterdayUnix,
	}).Error; err != nil {
		t.Fatalf("create historical daily quota failed: %v", err)
	}

	first := callFreeQuotaTestTool(t, user.Id, token, "mcp-free-quota-1")
	if first.FreeUsed != true || first.Quota != 0 || first.SettledAt <= 0 {
		t.Fatalf("first call should consume free quota without charge: %#v", first)
	}
	assertNoBillingEventForRequest(t, "mcp-free-quota-1")

	second := callFreeQuotaTestTool(t, user.Id, token, "mcp-free-quota-2")
	if second.FreeUsed || second.Quota != 100 || second.SettledAt <= 0 {
		t.Fatalf("second call should be charged after free quota is used: %#v", second)
	}
	assertMCPQuotaSettled(t, user.Id, token.Id, 1000, 1000, 100, false)
	assertMCPBillingEvent(t, "mcp-free-quota-2", second.Id, user.Id, token.Id, 100)

	if updated.FreeQuota != 1 {
		t.Fatalf("test setup free quota mismatch, got %d", updated.FreeQuota)
	}
}

func TestMCPToolFreeQuotaReleasedWhenExecutorFails(t *testing.T) {
	withServiceTestQuotaPerUnit(t, 100)
	restoreExecutorRegistry := setMCPExecutorRegistryForTest(executor.NewRegistry(failingMCPExecutor{}))
	t.Cleanup(restoreExecutorRegistry)

	user, token := seedMCPBillingUserAndToken(t, 1000, 1000, false)
	tool := setMCPToolPriceAndFreeQuotaForTest(t, "remote_read", 1.0, 1)
	t.Cleanup(func() {
		_ = model.DB.Where("request_id IN ?", []string{"mcp-free-quota-failed", "mcp-free-quota-after-failed"}).Delete(&model.MCPToolCall{}).Error
		_ = model.DB.Where("request_id IN ?", []string{"mcp-free-quota-failed", "mcp-free-quota-after-failed"}).Delete(&model.BillingEvent{}).Error
		_ = model.DB.Where("user_id = ? AND tool_id = ?", user.Id, tool.Id).Delete(&model.MCPUserDailyQuota{}).Error
	})

	resp, err := CallMCPTool(MCPToolCallRequest{
		UserId:         user.Id,
		TokenId:        token.Id,
		TokenKey:       token.Key,
		TokenUnlimited: token.UnlimitedQuota,
		TokenQuota:     token.RemainQuota,
		UsingGroup:     "default",
		RequestId:      "mcp-free-quota-failed",
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
	if resp == nil || resp.ErrorCode != dto.MCPErrorCodeExecutorFailed {
		t.Fatalf("expected executor failure, got %#v", resp)
	}

	var failedCall model.MCPToolCall
	if err := model.DB.Where("request_id = ?", "mcp-free-quota-failed").First(&failedCall).Error; err != nil {
		t.Fatalf("load failed MCP tool call failed: %v", err)
	}
	if failedCall.FreeUsed || failedCall.Quota != 0 {
		t.Fatalf("failed call should release free quota and stay uncharged: %#v", failedCall)
	}
	var dailyQuota model.MCPUserDailyQuota
	if err := model.DB.Where("user_id = ? AND tool_id = ?", user.Id, tool.Id).First(&dailyQuota).Error; err != nil {
		t.Fatalf("load daily quota failed: %v", err)
	}
	if dailyQuota.UsedCount != 0 || dailyQuota.FreeLimit != 1 {
		t.Fatalf("daily quota should be released after executor failure: %#v", dailyQuota)
	}

	restoreExecutorRegistry()
	restoreExecutorRegistry = setMCPExecutorRegistryForTest(executor.NewRegistry(successMCPExecutor{}))
	t.Cleanup(restoreExecutorRegistry)

	next := callFreeQuotaTestTool(t, user.Id, token, "mcp-free-quota-after-failed")
	if !next.FreeUsed || next.Quota != 0 || next.SettledAt <= 0 {
		t.Fatalf("next call should still use free quota after failed release: %#v", next)
	}
	assertNoBillingEventForRequest(t, "mcp-free-quota-after-failed")
	assertMCPQuotaState(t, user.Id, token.Id, 1000, 1000, 0, 0, false)
}

func callFreeQuotaTestTool(t *testing.T, userId int, token *model.Token, requestId string) model.MCPToolCall {
	t.Helper()
	resp, err := CallMCPTool(MCPToolCallRequest{
		UserId:         userId,
		TokenId:        token.Id,
		TokenKey:       token.Key,
		TokenUnlimited: token.UnlimitedQuota,
		TokenQuota:     token.RemainQuota,
		UsingGroup:     "default",
		RequestId:      requestId,
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
		t.Fatalf("expected MCP result, got %#v", resp)
	}

	var call model.MCPToolCall
	if err := model.DB.Where("request_id = ?", requestId).First(&call).Error; err != nil {
		t.Fatalf("load MCP tool call failed: %v", err)
	}
	if call.Status != model.MCPToolCallStatusSuccess {
		t.Fatalf("call status mismatch, got %s", call.Status)
	}
	return call
}

func assertNoBillingEventForRequest(t *testing.T, requestId string) {
	t.Helper()
	var count int64
	if err := model.DB.Model(&model.BillingEvent{}).Where("request_id = ?", requestId).Count(&count).Error; err != nil {
		t.Fatalf("count billing events failed: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected no billing event for %s, got %d", requestId, count)
	}
}
