package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/bridge"
	mcpproxy "github.com/QuantumNous/new-api/pkg/mcp/proxy"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestMCPProxyServerAdminCRUD(t *testing.T) {
	setupMCPProxyServiceTestDB(t)

	created, err := CreateMCPProxyServerForAdmin(dto.MCPProxyServerCreateRequest{
		Name:          "  GitHub MCP  ",
		Namespace:     "github",
		Transport:     model.MCPProxyTransportStreamableHTTP,
		Endpoint:      "  https://mcp.example.com/mcp  ",
		AuthType:      model.MCPProxyAuthTypeBearer,
		AuthRef:       "  ENV: MCP_PROXY_GITHUB_TOKEN  ",
		AllowedGroups: []string{"default", "vip", "default", ""},
	})
	require.NoError(t, err)
	require.Positive(t, created.Id)
	require.Equal(t, "GitHub MCP", created.Name)
	require.Equal(t, model.MCPProxyTransportStreamableHTTP, created.Transport)
	require.Equal(t, "configured", created.AuthRef)
	require.Equal(t, []string{"default", "vip"}, created.AllowedGroups)
	require.Equal(t, model.MCPProxyServerStatusDisabled, created.Status)
	var createdStored model.MCPProxyServer
	require.NoError(t, model.DB.Where("id = ?", created.Id).First(&createdStored).Error)
	require.Equal(t, "env:MCP_PROXY_GITHUB_TOKEN", createdStored.AuthRef)

	items, total, err := ListMCPProxyServersForAdmin(MCPProxyServerListParams{
		Transport: model.MCPProxyTransportStreamableHTTP,
		Keyword:   "github",
		Limit:     20,
	})
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, items, 1)
	require.Equal(t, created.Id, items[0].Id)

	enabled := model.MCPProxyServerStatusEnabled
	timeout := 45000
	updated, err := UpdateMCPProxyServerForAdmin(created.Id, dto.MCPProxyServerUpdateRequest{
		Status:    &enabled,
		TimeoutMS: &timeout,
	})
	require.NoError(t, err)
	require.Equal(t, model.MCPProxyServerStatusEnabled, updated.Status)
	require.Equal(t, 45000, updated.TimeoutMS)

	noAuth := model.MCPProxyAuthTypeNone
	updated, err = UpdateMCPProxyServerForAdmin(created.Id, dto.MCPProxyServerUpdateRequest{
		AuthType: &noAuth,
	})
	require.NoError(t, err)
	require.Equal(t, model.MCPProxyAuthTypeNone, updated.AuthType)
	require.Empty(t, updated.AuthRef)
	var stored model.MCPProxyServer
	require.NoError(t, model.DB.Where("id = ?", created.Id).First(&stored).Error)
	require.Empty(t, stored.AuthRef)

	archived, err := ArchiveMCPProxyServerForAdmin(created.Id)
	require.NoError(t, err)
	require.True(t, archived.Archived)
	require.Equal(t, model.MCPProxyServerStatusArchived, archived.Status)

	items, total, err = ListMCPProxyServersForAdmin(MCPProxyServerListParams{Limit: 20})
	require.NoError(t, err)
	require.EqualValues(t, 0, total)
	require.Empty(t, items)
}

func TestMCPProxyServerAdminValidation(t *testing.T) {
	setupMCPProxyServiceTestDB(t)

	_, err := CreateMCPProxyServerForAdmin(dto.MCPProxyServerCreateRequest{
		Name:      "Bad Namespace",
		Namespace: "Bad.Namespace",
		Transport: model.MCPProxyTransportHTTP,
		Endpoint:  "https://mcp.example.com/mcp",
	})
	require.Error(t, err)

	bridgeServer, err := CreateMCPProxyServerForAdmin(dto.MCPProxyServerCreateRequest{
		Name:      "Qidian MCP",
		Namespace: "local_mcp",
		Transport: model.MCPProxyTransportQidianBrowser,
		Endpoint:  "bridge://local",
	})
	require.NoError(t, err)
	require.Equal(t, model.MCPProxyTransportQidianBrowser, bridgeServer.Transport)

	_, err = CreateMCPProxyServerForAdmin(dto.MCPProxyServerCreateRequest{
		Name:      "Server Localhost MCP",
		Namespace: "host_local",
		Transport: model.MCPProxyTransportLocalhost,
		Endpoint:  "http://127.0.0.1:8765/mcp",
	})
	require.Error(t, err)

	_, err = CreateMCPProxyServerForAdmin(dto.MCPProxyServerCreateRequest{
		Name:      "Missing Endpoint",
		Namespace: "missing_endpoint",
		Transport: model.MCPProxyTransportHTTP,
	})
	require.Error(t, err)

	_, err = CreateMCPProxyServerForAdmin(dto.MCPProxyServerCreateRequest{
		Name:      "Raw Secret",
		Namespace: "raw_secret",
		Transport: model.MCPProxyTransportHTTP,
		Endpoint:  "https://mcp.example.com/mcp",
		AuthType:  model.MCPProxyAuthTypeBearer,
		AuthRef:   "raw:test-token",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "env:NAME")

	_, err = CreateMCPProxyServerForAdmin(dto.MCPProxyServerCreateRequest{
		Name:      "Empty Secret Ref",
		Namespace: "empty_secret_ref",
		Transport: model.MCPProxyTransportHTTP,
		Endpoint:  "https://mcp.example.com/mcp",
		AuthType:  model.MCPProxyAuthTypeBearer,
		AuthRef:   "env:",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "env:NAME")

	_, err = CreateMCPProxyServerForAdmin(dto.MCPProxyServerCreateRequest{
		Name:      "Malformed Secret Ref",
		Namespace: "malformed_secret_ref",
		Transport: model.MCPProxyTransportHTTP,
		Endpoint:  "https://mcp.example.com/mcp",
		AuthType:  model.MCPProxyAuthTypeBearer,
		AuthRef:   "env:MCP-PROXY-TOKEN",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "env:NAME")

	created, err := CreateMCPProxyServerForAdmin(dto.MCPProxyServerCreateRequest{
		Name:      "Valid MCP",
		Namespace: "valid_mcp",
		Transport: model.MCPProxyTransportHTTP,
		Endpoint:  "https://mcp.example.com/mcp",
	})
	require.NoError(t, err)

	emptyEndpoint := ""
	_, err = UpdateMCPProxyServerForAdmin(created.Id, dto.MCPProxyServerUpdateRequest{
		Endpoint: &emptyEndpoint,
	})
	require.Error(t, err)
}

func TestMCPProxyServerTestAndDiscover(t *testing.T) {
	setupMCPProxyServiceTestDB(t)

	created, err := CreateMCPProxyServerForAdmin(dto.MCPProxyServerCreateRequest{
		Name:      "GitHub MCP",
		Namespace: "github",
		Transport: model.MCPProxyTransportHTTP,
		Endpoint:  "https://mcp.example.com/mcp",
	})
	require.NoError(t, err)

	fake := &fakeMCPProxyClient{
		testResult: mcpproxy.TestResult{
			ProtocolVersion: "2025-06-18",
			ServerName:      "fake-github",
			Capabilities:    map[string]any{"tools": true},
		},
		tools: []mcpproxy.ToolDefinition{
			{
				Name:        "search_repos",
				Title:       "Search Repos",
				Description: "Search repositories",
				InputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"query": map[string]any{"type": "string"},
					},
				},
			},
			{
				Name:        "list_issues",
				Description: "List issues",
				InputSchema: map[string]any{"type": "object"},
			},
		},
	}
	restore := setMCPProxyClientForTest(fake)
	defer restore()

	testResult, err := TestMCPProxyServerForAdmin(context.Background(), created.Id)
	require.NoError(t, err)
	require.Equal(t, "2025-06-18", testResult.ProtocolVersion)
	require.Equal(t, "fake-github", testResult.ServerName)

	discovered, err := DiscoverMCPProxyServerToolsForAdmin(context.Background(), created.Id)
	require.NoError(t, err)
	require.Equal(t, created.Id, discovered.ProxyServerId)
	require.Equal(t, 2, discovered.DiscoveredCount)
	require.Equal(t, 2, discovered.CreatedCount)
	require.Len(t, discovered.Tools, 2)

	var exposed model.MCPTool
	require.NoError(t, model.DB.Where("name = ?", "github.search_repos").First(&exposed).Error)
	require.Equal(t, model.MCPToolSourceMCPProxy, exposed.Source)
	require.Equal(t, model.MCPToolStatusDisabled, exposed.Status)
	require.Contains(t, exposed.InputSchema, "query")

	var proxyTool model.MCPProxyTool
	require.NoError(t, model.DB.Where("exposed_tool_name = ?", "github.search_repos").First(&proxyTool).Error)
	require.Equal(t, "search_repos", proxyTool.DownstreamToolName)
	require.Equal(t, model.MCPProxyToolStatusPending, proxyTool.Status)
	require.NotEmpty(t, proxyTool.SchemaHash)

	fake.tools = []mcpproxy.ToolDefinition{
		{
			Name:        "search_repos",
			Title:       "Search Repos",
			Description: "Search repositories with language",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query":    map[string]any{"type": "string"},
					"language": map[string]any{"type": "string"},
				},
			},
		},
	}

	rediscovered, err := DiscoverMCPProxyServerToolsForAdmin(context.Background(), created.Id)
	require.NoError(t, err)
	require.Equal(t, 1, rediscovered.DiscoveredCount)
	require.Equal(t, 0, rediscovered.CreatedCount)
	require.Equal(t, 1, rediscovered.SchemaChanged)
	require.Equal(t, 1, rediscovered.DisabledCount)

	require.NoError(t, model.DB.Where("exposed_tool_name = ?", "github.search_repos").First(&proxyTool).Error)
	require.Equal(t, model.MCPProxyToolStatusSchemaChanged, proxyTool.Status)

	var disabledTool model.MCPProxyTool
	require.NoError(t, model.DB.Where("exposed_tool_name = ?", "github.list_issues").First(&disabledTool).Error)
	require.Equal(t, model.MCPProxyToolStatusDisabled, disabledTool.Status)
}

func TestMCPProxyToolAdminListAndUpdate(t *testing.T) {
	setupMCPProxyServiceTestDB(t)

	created, err := CreateMCPProxyServerForAdmin(dto.MCPProxyServerCreateRequest{
		Name:      "GitHub MCP",
		Namespace: "github",
		Transport: model.MCPProxyTransportHTTP,
		Endpoint:  "https://mcp.example.com/mcp",
	})
	require.NoError(t, err)

	fake := &fakeMCPProxyClient{
		tools: []mcpproxy.ToolDefinition{
			{
				Name:        "search_repos",
				Description: "Search repositories",
				InputSchema: map[string]any{"type": "object"},
			},
		},
	}
	restore := setMCPProxyClientForTest(fake)
	defer restore()

	discovered, err := DiscoverMCPProxyServerToolsForAdmin(context.Background(), created.Id)
	require.NoError(t, err)
	require.Len(t, discovered.Tools, 1)
	proxyTool := discovered.Tools[0]

	usagePriceUnit := model.MCPToolPriceUnitPer1K
	_, err = UpdateMCPProxyToolForAdmin(proxyTool.Id, dto.MCPProxyToolUpdateRequest{
		PriceUnit: &usagePriceUnit,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "only per_call")

	serverTools, err := ListMCPProxyServerToolsForAdmin(created.Id)
	require.NoError(t, err)
	require.Len(t, serverTools, 1)
	require.Equal(t, proxyTool.Id, serverTools[0].Id)

	items, total, err := ListMCPProxyToolsForAdmin(MCPProxyToolListParams{
		ProxyServerId: created.Id,
		Status:        model.MCPProxyToolStatusPending,
		Keyword:       "search",
		Limit:         20,
	})
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, items, 1)
	require.Equal(t, model.MCPToolPriceUnitPerCall, items[0].PriceUnit)

	enabled := model.MCPProxyToolStatusEnabled
	description := "Confirmed search repositories"
	price := 0.002
	freeQuota := 3
	sortOrder := 99
	updated, err := UpdateMCPProxyToolForAdmin(proxyTool.Id, dto.MCPProxyToolUpdateRequest{
		Status:             &enabled,
		ExposedDescription: &description,
		PricePerCall:       &price,
		FreeQuota:          &freeQuota,
		SortOrder:          &sortOrder,
	})
	require.NoError(t, err)
	require.Equal(t, model.MCPProxyToolStatusEnabled, updated.Status)
	require.Equal(t, description, updated.ExposedDescription)
	require.Equal(t, price, updated.PricePerCall)
	require.Equal(t, freeQuota, updated.FreeQuota)
	require.Equal(t, sortOrder, updated.SortOrder)

	var mcpTool model.MCPTool
	require.NoError(t, model.DB.Where("id = ?", proxyTool.MCPToolId).First(&mcpTool).Error)
	require.Equal(t, model.MCPToolStatusEnabled, mcpTool.Status)
	require.Equal(t, description, mcpTool.Description)
	require.Equal(t, price, mcpTool.PricePerCall)
	require.Equal(t, freeQuota, mcpTool.FreeQuota)
	require.Equal(t, sortOrder, mcpTool.SortOrder)

	fake.tools = []mcpproxy.ToolDefinition{
		{
			Name:        "search_repos",
			Description: "Search repositories with language",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"language": map[string]any{"type": "string"},
				},
			},
		},
	}
	rediscovered, err := DiscoverMCPProxyServerToolsForAdmin(context.Background(), created.Id)
	require.NoError(t, err)
	require.Equal(t, 1, rediscovered.SchemaChanged)

	require.NoError(t, model.DB.Where("id = ?", proxyTool.MCPToolId).First(&mcpTool).Error)
	require.Equal(t, model.MCPToolStatusDisabled, mcpTool.Status)

	updated, err = UpdateMCPProxyToolForAdmin(proxyTool.Id, dto.MCPProxyToolUpdateRequest{Status: &enabled})
	require.NoError(t, err)
	require.Equal(t, model.MCPProxyToolStatusEnabled, updated.Status)

	require.NoError(t, model.DB.Where("id = ?", proxyTool.MCPToolId).First(&mcpTool).Error)
	require.Equal(t, model.MCPToolStatusEnabled, mcpTool.Status)
}

func TestMCPProxyToolCallSuccessAndError(t *testing.T) {
	setupMCPProxyServiceTestDB(t)
	user, token := seedMCPBillingUserAndToken(t, 100000, 100000, false)

	created, err := CreateMCPProxyServerForAdmin(dto.MCPProxyServerCreateRequest{
		Name:      "GitHub MCP",
		Namespace: "github",
		Transport: model.MCPProxyTransportHTTP,
		Endpoint:  "https://mcp.example.com/mcp",
		Status:    model.MCPProxyServerStatusEnabled,
	})
	require.NoError(t, err)

	fake := &fakeMCPProxyClient{
		tools: []mcpproxy.ToolDefinition{
			{
				Name:        "search_repos",
				Title:       "Search Repos",
				Description: "Search repositories",
				InputSchema: map[string]any{"type": "object"},
			},
		},
		callResult: mcpproxy.CallResult{
			Content: []dto.MCPContentBlock{{Type: "text", Text: "proxy-ok"}},
			Metadata: map[string]any{
				"downstream_request_id": "downstream-1",
			},
			Summary:    "proxy-ok",
			DurationMS: 12,
		},
	}
	restore := setMCPProxyClientForTest(fake)
	defer restore()

	discovered, err := DiscoverMCPProxyServerToolsForAdmin(context.Background(), created.Id)
	require.NoError(t, err)
	require.Len(t, discovered.Tools, 1)
	require.Equal(t, "github.search_repos", discovered.Tools[0].ExposedToolName)

	_, err = model.UpdateMCPToolFields(discovered.Tools[0].MCPToolId, map[string]any{
		"status":         model.MCPToolStatusEnabled,
		"price_per_call": 0.001,
	})
	require.NoError(t, err)
	_, err = model.UpdateMCPProxyToolFields(discovered.Tools[0].Id, map[string]any{
		"status": model.MCPProxyToolStatusEnabled,
	})
	require.NoError(t, err)

	resp, err := CallMCPTool(MCPToolCallRequest{
		Context:        context.Background(),
		UserId:         user.Id,
		TokenId:        token.Id,
		TokenKey:       token.Key,
		TokenQuota:     token.RemainQuota,
		TokenUnlimited: token.UnlimitedQuota,
		UsingGroup:     "default",
		RequestId:      "mcp-proxy-call-success",
		Params: dto.MCPToolCallParams{
			Name:      "github.search_repos",
			Arguments: map[string]any{"query": "data-proxy"},
		},
	})
	require.NoError(t, err)
	require.NotNil(t, resp.Result)
	require.Equal(t, "proxy-ok", resp.Result.Content[0].Text)
	require.Equal(t, "search_repos", fake.lastCall.ToolName)
	require.Equal(t, "data-proxy", fake.lastCall.Arguments["query"])
	require.Equal(t, float64(created.Id), numericMetadataValue(resp.Result.Metadata["proxy_server_id"]))
	require.Equal(t, "search_repos", resp.Result.Metadata["downstream_tool_name"])

	var successCall model.MCPToolCall
	require.NoError(t, model.DB.Where("request_id = ?", "mcp-proxy-call-success").First(&successCall).Error)
	require.Equal(t, model.MCPToolCallStatusSuccess, successCall.Status)
	successMetadata := decodeMCPToolCallMetadata(t, successCall)
	require.Equal(t, "downstream-1", successMetadata["downstream_request_id"])
	require.Equal(t, "search_repos", successMetadata["downstream_tool_name"])
	require.Equal(t, "github.search_repos", successMetadata["exposed_tool_name"])
	require.Equal(t, "github", successMetadata["proxy_server_namespace"])
	assertMCPQuotaSettled(t, user.Id, token.Id, 100000, 100000, 500, false)
	assertMCPBillingEvent(t, "mcp-proxy-call-success", successCall.Id, user.Id, token.Id, 500)

	fake.callErr = errors.New("downstream failed")
	resp, err = CallMCPTool(MCPToolCallRequest{
		Context:        context.Background(),
		UserId:         user.Id,
		TokenId:        token.Id,
		TokenKey:       token.Key,
		TokenQuota:     99500,
		TokenUnlimited: token.UnlimitedQuota,
		UsingGroup:     "default",
		RequestId:      "mcp-proxy-call-error",
		Params: dto.MCPToolCallParams{
			Name:      "github.search_repos",
			Arguments: map[string]any{"query": "fail"},
		},
	})
	require.NoError(t, err)
	require.Nil(t, resp.Result)
	require.Equal(t, dto.MCPErrorCodeExecutorFailed, resp.ErrorCode)

	var errorCall model.MCPToolCall
	require.NoError(t, model.DB.Where("request_id = ?", "mcp-proxy-call-error").First(&errorCall).Error)
	require.Equal(t, model.MCPToolCallStatusError, errorCall.Status)
	require.Equal(t, 0, errorCall.Quota)
	errorMetadata := decodeMCPToolCallMetadata(t, errorCall)
	require.Equal(t, "search_repos", errorMetadata["downstream_tool_name"])
	require.Equal(t, "github.search_repos", errorMetadata["exposed_tool_name"])
	assertMCPQuotaState(t, user.Id, token.Id, 100000, 100000, 500, 1, false)
}

func TestMCPProxyServerHealthForAdmin(t *testing.T) {
	setupMCPProxyServiceTestDB(t)
	user, token := seedMCPBillingUserAndToken(t, 100000, 100000, false)

	created, err := CreateMCPProxyServerForAdmin(dto.MCPProxyServerCreateRequest{
		Name:      "GitHub MCP",
		Namespace: "github",
		Transport: model.MCPProxyTransportHTTP,
		Endpoint:  "https://mcp.example.com/mcp",
		Status:    model.MCPProxyServerStatusEnabled,
	})
	require.NoError(t, err)

	fake := &fakeMCPProxyClient{
		tools: []mcpproxy.ToolDefinition{
			{
				Name:        "search_repos",
				Description: "Search repositories",
				InputSchema: map[string]any{"type": "object"},
			},
			{
				Name:        "get_issue",
				Description: "Get issue detail",
				InputSchema: map[string]any{"type": "object"},
			},
		},
	}
	restore := setMCPProxyClientForTest(fake)
	defer restore()

	discovered, err := DiscoverMCPProxyServerToolsForAdmin(context.Background(), created.Id)
	require.NoError(t, err)
	require.Len(t, discovered.Tools, 2)
	searchTool := discovered.Tools[0]
	issueTool := discovered.Tools[1]
	_, err = model.UpdateMCPProxyToolFields(searchTool.Id, map[string]any{
		"status": model.MCPProxyToolStatusEnabled,
	})
	require.NoError(t, err)
	_, err = model.UpdateMCPProxyToolFields(issueTool.Id, map[string]any{
		"status": model.MCPProxyToolStatusEnabled,
	})
	require.NoError(t, err)

	now := common.GetTimestamp()
	require.NoError(t, model.CreateMCPToolCall(&model.MCPToolCall{
		UserId:        user.Id,
		TokenId:       token.Id,
		ToolId:        searchTool.MCPToolId,
		ToolName:      searchTool.ExposedToolName,
		RequestId:     "proxy-health-success",
		Status:        model.MCPToolCallStatusSuccess,
		ResultSummary: "ok",
		DurationMS:    20,
		ResultSize:    120,
		Cost:          0.001,
		Quota:         500,
		PriceUnit:     model.MCPToolPriceUnitPerCall,
		SettledAt:     now,
	}))
	require.NoError(t, model.CreateMCPToolCall(&model.MCPToolCall{
		UserId:       user.Id,
		TokenId:      token.Id,
		ToolId:       searchTool.MCPToolId,
		ToolName:     searchTool.ExposedToolName,
		RequestId:    "proxy-health-error",
		Status:       model.MCPToolCallStatusError,
		ErrorCode:    "executor_failed",
		ErrorMessage: "downstream failed",
		DurationMS:   7,
		PriceUnit:    model.MCPToolPriceUnitPerCall,
	}))
	require.NoError(t, model.CreateMCPToolCall(&model.MCPToolCall{
		UserId:       user.Id,
		TokenId:      token.Id,
		ToolId:       issueTool.MCPToolId,
		ToolName:     issueTool.ExposedToolName,
		RequestId:    "proxy-health-timeout",
		Status:       model.MCPToolCallStatusTimeout,
		ErrorCode:    "executor_timeout",
		ErrorMessage: "downstream timed out",
		DurationMS:   30,
		Quota:        200,
		PriceUnit:    model.MCPToolPriceUnitPerCall,
	}))
	otherTool := &model.MCPTool{
		Name:        "other.proxy_tool",
		DisplayName: "Other",
		Description: "Other",
		Category:    "third_party",
		Source:      model.MCPToolSourceMCPProxy,
		InputSchema: "{}",
		Status:      model.MCPToolStatusEnabled,
	}
	require.NoError(t, model.CreateMCPTool(otherTool))
	require.NoError(t, model.CreateMCPToolCall(&model.MCPToolCall{
		UserId:    user.Id,
		TokenId:   token.Id,
		ToolId:    otherTool.Id,
		ToolName:  otherTool.Name,
		RequestId: "other-proxy-call",
		Status:    model.MCPToolCallStatusSuccess,
		Quota:     500,
		PriceUnit: model.MCPToolPriceUnitPerCall,
	}))

	health, err := GetMCPProxyServerHealthForAdmin(created.Id, MCPProxyServerHealthParams{WindowSeconds: 3600})
	require.NoError(t, err)
	require.Equal(t, created.Id, health.ProxyServerId)
	require.EqualValues(t, 3600, health.WindowSeconds)
	require.EqualValues(t, 3, health.Calls.TotalCalls)
	require.EqualValues(t, 1, health.Calls.SuccessCalls)
	require.EqualValues(t, 1, health.Calls.ErrorCalls)
	require.EqualValues(t, 1, health.Calls.TimeoutCalls)
	require.EqualValues(t, 700, health.Calls.Quota)
	require.InDelta(t, 33.33, health.Calls.SuccessRate, 0.01)
	require.Equal(t, 2, health.Discovery.TotalTools)
	require.Equal(t, 2, health.Discovery.EnabledTools)
	require.Equal(t, model.MCPProxyTransportHTTP, health.Transport.Transport)
	require.False(t, health.Transport.Observable)
	require.True(t, health.NeedsReview)
	require.Contains(t, health.ReviewReasons, "recent_call_errors")
	require.NotNil(t, health.LatestCheck)
	require.Equal(t, model.MCPProxyDiscoveryEventTypeDiscover, health.LatestCheck.EventType)
	require.Equal(t, model.MCPProxyDiscoveryEventStatusSuccess, health.LatestCheck.Status)
	require.Len(t, health.TopTools, 2)
	require.Equal(t, searchTool.ExposedToolName, health.TopTools[0].ExposedToolName)
	require.Equal(t, "search_repos", health.TopTools[0].DownstreamToolName)
	require.EqualValues(t, 2, health.TopTools[0].Calls)
	require.EqualValues(t, 1, health.TopTools[0].SuccessCalls)
	require.EqualValues(t, 1, health.TopTools[0].ErrorCalls)
	require.EqualValues(t, 500, health.TopTools[0].Quota)
	require.Equal(t, 50.0, health.TopTools[0].SuccessRate)
	require.Equal(t, issueTool.ExposedToolName, health.TopTools[1].ExposedToolName)
	require.Equal(t, "get_issue", health.TopTools[1].DownstreamToolName)
	require.EqualValues(t, 1, health.TopTools[1].TimeoutCalls)
	require.EqualValues(t, 200, health.TopTools[1].Quota)
	require.Len(t, health.RecentErrors, 2)
	require.Equal(t, "proxy-health-timeout", health.RecentErrors[0].RequestId)
	require.Equal(t, "proxy-health-error", health.RecentErrors[1].RequestId)

	servers, total, err := ListMCPProxyServersForAdmin(MCPProxyServerListParams{Limit: 20})
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, servers, 1)
	require.NotNil(t, servers[0].Health)
	require.EqualValues(t, 24*60*60, servers[0].Health.WindowSeconds)
	require.EqualValues(t, 3, servers[0].Health.Calls.TotalCalls)
	require.EqualValues(t, 1, servers[0].Health.Calls.SuccessCalls)
	require.EqualValues(t, 1, servers[0].Health.Calls.ErrorCalls)
	require.EqualValues(t, 1, servers[0].Health.Calls.TimeoutCalls)
	require.EqualValues(t, 700, servers[0].Health.Calls.Quota)
	require.True(t, servers[0].Health.NeedsReview)
	require.Contains(t, servers[0].Health.ReviewReasons, "recent_call_errors")
	require.NotNil(t, servers[0].Health.LatestCheck)
	require.NotNil(t, servers[0].Health.TopTool)
	require.Equal(t, searchTool.ExposedToolName, servers[0].Health.TopTool.ExposedToolName)
	require.NotNil(t, servers[0].Health.LatestError)
	require.Equal(t, "proxy-health-timeout", servers[0].Health.LatestError.RequestId)
}

func TestMCPProxyTrendsForAdmin(t *testing.T) {
	setupMCPProxyServiceTestDB(t)
	user, token := seedMCPBillingUserAndToken(t, 100000, 100000, false)

	created, err := CreateMCPProxyServerForAdmin(dto.MCPProxyServerCreateRequest{
		Name:      "GitHub MCP",
		Namespace: "github",
		Transport: model.MCPProxyTransportHTTP,
		Endpoint:  "https://mcp.example.com/mcp",
		Status:    model.MCPProxyServerStatusEnabled,
	})
	require.NoError(t, err)
	other, err := CreateMCPProxyServerForAdmin(dto.MCPProxyServerCreateRequest{
		Name:      "Linear MCP",
		Namespace: "linear",
		Transport: model.MCPProxyTransportHTTP,
		Endpoint:  "https://linear.example.com/mcp",
		Status:    model.MCPProxyServerStatusEnabled,
	})
	require.NoError(t, err)

	fake := &fakeMCPProxyClient{
		tools: []mcpproxy.ToolDefinition{
			{Name: "search_repos", Description: "Search repositories", InputSchema: map[string]any{"type": "object"}},
		},
	}
	restore := setMCPProxyClientForTest(fake)
	defer restore()
	githubTools, err := DiscoverMCPProxyServerToolsForAdmin(context.Background(), created.Id)
	require.NoError(t, err)
	linearTools, err := DiscoverMCPProxyServerToolsForAdmin(context.Background(), other.Id)
	require.NoError(t, err)
	require.Len(t, githubTools.Tools, 1)
	require.Len(t, linearTools.Tools, 1)
	githubTool := githubTools.Tools[0]
	linearTool := linearTools.Tools[0]

	now := common.GetTimestamp()
	dayOne := now - 2*24*60*60
	dayTwo := now - 24*60*60
	createTrendCall := func(requestId string, tool dto.MCPProxyToolAdminItem, status string, createdAt int64, quota int) {
		t.Helper()
		require.NoError(t, model.CreateMCPToolCall(&model.MCPToolCall{
			UserId:     user.Id,
			TokenId:    token.Id,
			ToolId:     tool.MCPToolId,
			ToolName:   tool.ExposedToolName,
			RequestId:  requestId,
			Status:     status,
			DurationMS: 20,
			ResultSize: 120,
			Cost:       float64(quota) / 1000,
			Quota:      quota,
			PriceUnit:  model.MCPToolPriceUnitPerCall,
			SettledAt:  createdAt,
		}))
		require.NoError(t, model.DB.Model(&model.MCPToolCall{}).
			Where("request_id = ?", requestId).
			Update("created_at", createdAt).Error)
	}
	createTrendCall("proxy-trend-github-success", githubTool, model.MCPToolCallStatusSuccess, dayOne, 100)
	createTrendCall("proxy-trend-github-error", githubTool, model.MCPToolCallStatusError, dayTwo, 0)
	createTrendCall("proxy-trend-linear-success", linearTool, model.MCPToolCallStatusSuccess, dayTwo, 300)

	all, err := GetMCPProxyTrendsForAdmin(MCPProxyTrendParams{
		StartTime:     dayOne - 60,
		EndTime:       now + 60,
		BucketSeconds: 24 * 60 * 60,
	})
	require.NoError(t, err)
	require.EqualValues(t, 3, all.Totals.TotalCalls)
	require.EqualValues(t, 2, all.Totals.SuccessCalls)
	require.EqualValues(t, 1, all.Totals.ErrorCalls)
	require.EqualValues(t, 400, all.Totals.Quota)
	require.InDelta(t, 66.67, all.Totals.SuccessRate, 0.01)
	require.Len(t, all.Servers, 2)
	require.Equal(t, "github", all.Servers[0].Namespace)
	require.EqualValues(t, 2, all.Servers[0].TotalCalls)
	require.NotEmpty(t, all.Buckets)

	filtered, err := GetMCPProxyTrendsForAdmin(MCPProxyTrendParams{
		ProxyServerId: created.Id,
		StartTime:     dayOne - 60,
		EndTime:       now + 60,
		BucketSeconds: 24 * 60 * 60,
	})
	require.NoError(t, err)
	require.EqualValues(t, 2, filtered.Totals.TotalCalls)
	require.EqualValues(t, 1, filtered.Totals.SuccessCalls)
	require.EqualValues(t, 1, filtered.Totals.ErrorCalls)
	require.Len(t, filtered.Servers, 1)
	require.Equal(t, "github", filtered.Servers[0].Namespace)
	require.Len(t, filtered.Tools, 1)
	require.Equal(t, githubTool.Id, filtered.Tools[0].ProxyToolId)
}

func TestMCPProxyToolDetailAndHealthForAdmin(t *testing.T) {
	setupMCPProxyServiceTestDB(t)
	user, token := seedMCPBillingUserAndToken(t, 100000, 100000, false)

	created, err := CreateMCPProxyServerForAdmin(dto.MCPProxyServerCreateRequest{
		Name:      "GitHub MCP",
		Namespace: "github",
		Transport: model.MCPProxyTransportHTTP,
		Endpoint:  "https://mcp.example.com/mcp",
		Status:    model.MCPProxyServerStatusEnabled,
	})
	require.NoError(t, err)

	fake := &fakeMCPProxyClient{
		tools: []mcpproxy.ToolDefinition{
			{
				Name:        "search_repos",
				Description: "Search repositories",
				InputSchema: map[string]any{"type": "object"},
			},
			{
				Name:        "get_issue",
				Description: "Get issue detail",
				InputSchema: map[string]any{"type": "object"},
			},
		},
	}
	restore := setMCPProxyClientForTest(fake)
	defer restore()

	discovered, err := DiscoverMCPProxyServerToolsForAdmin(context.Background(), created.Id)
	require.NoError(t, err)
	require.Len(t, discovered.Tools, 2)
	searchTool := discovered.Tools[0]
	issueTool := discovered.Tools[1]

	now := common.GetTimestamp()
	require.NoError(t, model.CreateMCPToolCall(&model.MCPToolCall{
		UserId:        user.Id,
		TokenId:       token.Id,
		ToolId:        searchTool.MCPToolId,
		ToolName:      searchTool.ExposedToolName,
		RequestId:     "proxy-tool-health-success",
		Status:        model.MCPToolCallStatusSuccess,
		ResultSummary: "ok",
		DurationMS:    25,
		ResultSize:    100,
		Cost:          0.001,
		Quota:         300,
		PriceUnit:     model.MCPToolPriceUnitPerCall,
		SettledAt:     now,
	}))
	require.NoError(t, model.CreateMCPToolCall(&model.MCPToolCall{
		UserId:       user.Id,
		TokenId:      token.Id,
		ToolId:       searchTool.MCPToolId,
		ToolName:     searchTool.ExposedToolName,
		RequestId:    "proxy-tool-health-error",
		Status:       model.MCPToolCallStatusError,
		ErrorCode:    "executor_failed",
		ErrorMessage: "downstream failed",
		DurationMS:   10,
		PriceUnit:    model.MCPToolPriceUnitPerCall,
	}))
	require.NoError(t, model.CreateMCPToolCall(&model.MCPToolCall{
		UserId:    user.Id,
		TokenId:   token.Id,
		ToolId:    issueTool.MCPToolId,
		ToolName:  issueTool.ExposedToolName,
		RequestId: "proxy-tool-health-other-tool",
		Status:    model.MCPToolCallStatusSuccess,
		Quota:     900,
		PriceUnit: model.MCPToolPriceUnitPerCall,
	}))

	detail, err := GetMCPProxyToolForAdmin(searchTool.Id)
	require.NoError(t, err)
	require.Equal(t, searchTool.Id, detail.Id)
	require.Equal(t, searchTool.ExposedToolName, detail.ExposedToolName)

	health, err := GetMCPProxyToolHealthForAdmin(searchTool.Id, MCPProxyServerHealthParams{WindowSeconds: 3600})
	require.NoError(t, err)
	require.Equal(t, searchTool.Id, health.ProxyToolId)
	require.Equal(t, created.Id, health.ProxyServerId)
	require.Equal(t, searchTool.MCPToolId, health.MCPToolId)
	require.EqualValues(t, 2, health.Calls.TotalCalls)
	require.EqualValues(t, 1, health.Calls.SuccessCalls)
	require.EqualValues(t, 1, health.Calls.ErrorCalls)
	require.EqualValues(t, 0, health.Calls.TimeoutCalls)
	require.EqualValues(t, 300, health.Calls.Quota)
	require.Equal(t, 50.0, health.Calls.SuccessRate)
	require.Equal(t, searchTool.ExposedToolName, health.Tool.ExposedToolName)
	require.Equal(t, "search_repos", health.Tool.DownstreamToolName)
	require.EqualValues(t, 2, health.Tool.Calls)
	require.Len(t, health.RecentErrors, 1)
	require.Equal(t, "proxy-tool-health-error", health.RecentErrors[0].RequestId)
}

func TestMCPProxyDiscoveryEventsForAdmin(t *testing.T) {
	setupMCPProxyServiceTestDB(t)

	created, err := CreateMCPProxyServerForAdmin(dto.MCPProxyServerCreateRequest{
		Name:      "GitHub MCP",
		Namespace: "github",
		Transport: model.MCPProxyTransportHTTP,
		Endpoint:  "https://mcp.example.com/mcp",
		Status:    model.MCPProxyServerStatusEnabled,
	})
	require.NoError(t, err)
	other, err := CreateMCPProxyServerForAdmin(dto.MCPProxyServerCreateRequest{
		Name:      "Other MCP",
		Namespace: "other",
		Transport: model.MCPProxyTransportHTTP,
		Endpoint:  "https://other.example.com/mcp",
		Status:    model.MCPProxyServerStatusEnabled,
	})
	require.NoError(t, err)

	fake := &fakeMCPProxyClient{
		testResult: mcpproxy.TestResult{
			ProtocolVersion: "2025-06-18",
			ServerName:      "github",
			Capabilities:    map[string]any{"tools": map[string]any{"listChanged": true}},
		},
		tools: []mcpproxy.ToolDefinition{
			{
				Name:        "search_repos",
				Description: "Search repositories",
				InputSchema: map[string]any{"type": "object"},
			},
		},
	}
	restore := setMCPProxyClientForTest(fake)
	defer restore()

	_, err = TestMCPProxyServerForAdmin(context.Background(), created.Id)
	require.NoError(t, err)
	_, err = DiscoverMCPProxyServerToolsForAdmin(context.Background(), created.Id)
	require.NoError(t, err)
	require.NoError(t, model.CreateMCPProxyDiscoveryEvent(&model.MCPProxyDiscoveryEvent{
		ProxyServerId:   other.Id,
		EventType:       model.MCPProxyDiscoveryEventTypeDiscover,
		Status:          model.MCPProxyDiscoveryEventStatusSuccess,
		DiscoveredCount: 99,
	}))

	events, total, err := ListMCPProxyDiscoveryEventsForAdmin(created.Id, 0, 10)
	require.NoError(t, err)
	require.EqualValues(t, 2, total)
	require.Len(t, events, 2)
	require.Equal(t, created.Id, events[0].ProxyServerId)
	require.Equal(t, model.MCPProxyDiscoveryEventTypeDiscover, events[0].EventType)
	require.Equal(t, model.MCPProxyDiscoveryEventStatusSuccess, events[0].Status)
	require.Equal(t, 1, events[0].DiscoveredCount)
	require.Equal(t, model.MCPProxyDiscoveryEventTypeTest, events[1].EventType)
	require.Equal(t, "github", events[1].ServerName)
	require.Contains(t, events[1].Capabilities, "tools")
}

func TestMCPProxyDiscoveryEventsRecordFailure(t *testing.T) {
	setupMCPProxyServiceTestDB(t)

	created, err := CreateMCPProxyServerForAdmin(dto.MCPProxyServerCreateRequest{
		Name:      "GitHub MCP",
		Namespace: "github",
		Transport: model.MCPProxyTransportHTTP,
		Endpoint:  "https://mcp.example.com/mcp",
		Status:    model.MCPProxyServerStatusEnabled,
	})
	require.NoError(t, err)

	fake := &fakeMCPProxyClient{listErr: errors.New("downstream unavailable")}
	restore := setMCPProxyClientForTest(fake)
	defer restore()

	_, err = DiscoverMCPProxyServerToolsForAdmin(context.Background(), created.Id)
	require.Error(t, err)

	events, total, err := ListMCPProxyDiscoveryEventsForAdmin(created.Id, 0, 10)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, events, 1)
	require.Equal(t, model.MCPProxyDiscoveryEventTypeDiscover, events[0].EventType)
	require.Equal(t, model.MCPProxyDiscoveryEventStatusError, events[0].Status)
	require.Contains(t, events[0].Message, "downstream unavailable")
}

func TestMCPProxyHealthCheckSuccessClearsErrorWithoutEnablingDisabled(t *testing.T) {
	setupMCPProxyServiceTestDB(t)

	created, err := CreateMCPProxyServerForAdmin(dto.MCPProxyServerCreateRequest{
		Name:      "GitHub MCP",
		Namespace: "github",
		Transport: model.MCPProxyTransportHTTP,
		Endpoint:  "https://mcp.example.com/mcp",
		Status:    model.MCPProxyServerStatusEnabled,
	})
	require.NoError(t, err)

	fake := &fakeMCPProxyClient{listErr: errors.New("downstream unavailable")}
	restore := setMCPProxyClientForTest(fake)
	defer restore()

	_, err = DiscoverMCPProxyServerToolsForAdmin(context.Background(), created.Id)
	require.Error(t, err)
	var stored model.MCPProxyServer
	require.NoError(t, model.DB.Where("id = ?", created.Id).First(&stored).Error)
	require.Equal(t, model.MCPProxyServerStatusError, stored.Status)
	require.Contains(t, stored.LastError, "downstream unavailable")

	fake.listErr = nil
	fake.tools = []mcpproxy.ToolDefinition{
		{
			Name:        "search_repos",
			Description: "Search repositories",
			InputSchema: map[string]any{"type": "object"},
		},
	}
	_, err = DiscoverMCPProxyServerToolsForAdmin(context.Background(), created.Id)
	require.NoError(t, err)
	require.NoError(t, model.DB.Where("id = ?", created.Id).First(&stored).Error)
	require.Equal(t, model.MCPProxyServerStatusEnabled, stored.Status)
	require.Empty(t, stored.LastError)

	disabled, err := CreateMCPProxyServerForAdmin(dto.MCPProxyServerCreateRequest{
		Name:      "Disabled MCP",
		Namespace: "disabled",
		Transport: model.MCPProxyTransportHTTP,
		Endpoint:  "https://disabled.example.com/mcp",
		Status:    model.MCPProxyServerStatusDisabled,
	})
	require.NoError(t, err)
	_, err = model.UpdateMCPProxyServerFields(disabled.Id, map[string]any{
		"last_error": "previous error",
	})
	require.NoError(t, err)
	fake.testResult = mcpproxy.TestResult{
		ProtocolVersion: "2025-06-18",
		ServerName:      "disabled",
		Capabilities:    map[string]any{"tools": true},
	}
	_, err = TestMCPProxyServerForAdmin(context.Background(), disabled.Id)
	require.NoError(t, err)
	stored = model.MCPProxyServer{}
	require.NoError(t, model.DB.Where("id = ?", disabled.Id).First(&stored).Error)
	require.Equal(t, model.MCPProxyServerStatusDisabled, stored.Status)
	require.Empty(t, stored.LastError)
}

func TestMCPProxyActiveHealthCheckRunsAndSkipsFreshChecks(t *testing.T) {
	setupMCPProxyServiceTestDB(t)

	created, err := CreateMCPProxyServerForAdmin(dto.MCPProxyServerCreateRequest{
		Name:      "GitHub MCP",
		Namespace: "github",
		Transport: model.MCPProxyTransportHTTP,
		Endpoint:  "https://mcp.example.com/mcp",
		Status:    model.MCPProxyServerStatusEnabled,
	})
	require.NoError(t, err)
	_, err = CreateMCPProxyServerForAdmin(dto.MCPProxyServerCreateRequest{
		Name:      "Disabled MCP",
		Namespace: "disabled",
		Transport: model.MCPProxyTransportHTTP,
		Endpoint:  "https://disabled.example.com/mcp",
		Status:    model.MCPProxyServerStatusDisabled,
	})
	require.NoError(t, err)

	fake := &fakeMCPProxyClient{
		testResult: mcpproxy.TestResult{
			ProtocolVersion: "2025-06-18",
			ServerName:      "github",
			Capabilities:    map[string]any{"tools": true},
		},
	}
	restore := setMCPProxyClientForTest(fake)
	defer restore()

	run, err := RunMCPProxyHealthCheckOnce(context.Background(), true, dto.MCPProxyHealthCheckRunRequest{
		Force: true,
		Limit: 20,
	})
	require.NoError(t, err)
	require.Equal(t, "success", run.Status)
	require.Equal(t, 1, run.ScannedCount)
	require.Equal(t, 1, run.CheckedCount)
	require.Equal(t, 1, run.SuccessCount)
	require.Equal(t, 1, fake.testCalls)
	require.Equal(t, created.Id, run.Items[0].ProxyServerId)
	require.Equal(t, "test", run.Items[0].Action)
	require.NotNil(t, run.Items[0].LatestCheck)

	second, err := RunMCPProxyHealthCheckOnce(context.Background(), true, dto.MCPProxyHealthCheckRunRequest{
		Limit:        20,
		StaleSeconds: 3600,
	})
	require.NoError(t, err)
	require.Equal(t, "success", second.Status)
	require.Equal(t, 1, second.ScannedCount)
	require.Equal(t, 0, second.CheckedCount)
	require.Equal(t, 1, second.SkippedCount)
	require.Equal(t, "latest_check_is_fresh", second.Items[0].SkippedReason)
	require.Equal(t, 1, fake.testCalls)
}

func TestMCPProxyActiveHealthCheckDiscoverThresholdBlocksSync(t *testing.T) {
	setupMCPProxyServiceTestDB(t)

	created, err := CreateMCPProxyServerForAdmin(dto.MCPProxyServerCreateRequest{
		Name:      "GitHub MCP",
		Namespace: "github",
		Transport: model.MCPProxyTransportHTTP,
		Endpoint:  "https://mcp.example.com/mcp",
		Status:    model.MCPProxyServerStatusEnabled,
	})
	require.NoError(t, err)

	discover := true
	fake := &fakeMCPProxyClient{
		tools: []mcpproxy.ToolDefinition{
			{Name: "search_repos", Description: "Search repositories", InputSchema: map[string]any{"type": "object"}},
			{Name: "list_issues", Description: "List issues", InputSchema: map[string]any{"type": "object"}},
		},
	}
	restore := setMCPProxyClientForTest(fake)
	defer restore()

	run, err := RunMCPProxyHealthCheckOnce(context.Background(), true, dto.MCPProxyHealthCheckRunRequest{
		Force:            true,
		Discover:         &discover,
		Limit:            20,
		StaleSeconds:     1,
		MaxDiscoverTools: 1,
	})
	require.NoError(t, err)
	require.Equal(t, "blocked", run.Status)
	require.Equal(t, 1, run.CheckedCount)
	require.Equal(t, 1, run.BlockedCount)
	require.Equal(t, "discover", run.Items[0].Action)
	require.Equal(t, 1, fake.listCalls)

	var proxyToolCount int64
	require.NoError(t, model.DB.Model(&model.MCPProxyTool{}).Where("proxy_server_id = ?", created.Id).Count(&proxyToolCount).Error)
	require.Equal(t, int64(0), proxyToolCount)

	events, total, err := ListMCPProxyDiscoveryEventsForAdmin(created.Id, 0, 10)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Contains(t, events[0].Message, "max_discover_tools")
	require.Equal(t, model.MCPProxyDiscoveryEventStatusError, events[0].Status)
}

func TestMCPProxyHealthCheckSettingsPersist(t *testing.T) {
	setupMCPProxyServiceTestDB(t)

	status, err := UpdateMCPProxyHealthCheckSettings(dto.MCPProxyHealthCheckSettingsRequest{
		Enabled:          true,
		IntervalMinutes:  5,
		Limit:            3,
		StaleSeconds:     30,
		Discover:         true,
		MaxDiscoverTools: 7,
	})
	require.NoError(t, err)
	require.True(t, status.Settings.Enabled)
	require.Equal(t, 5, status.Settings.IntervalMinutes)
	require.Equal(t, 3, status.Settings.Limit)
	require.EqualValues(t, 30, status.Settings.StaleSeconds)
	require.True(t, status.Settings.Discover)
	require.Equal(t, 7, status.Settings.MaxDiscoverTools)

	loaded := GetMCPProxyHealthCheckStatus()
	require.True(t, loaded.Settings.Enabled)
	require.Equal(t, 5, loaded.Settings.IntervalMinutes)
	require.Equal(t, 7, loaded.Settings.MaxDiscoverTools)
}

func TestMCPProxyActiveHealthCheckHonorsCrossProcessLease(t *testing.T) {
	setupMCPProxyServiceTestDB(t)

	now := common.GetTimestamp()
	require.NoError(t, updateMCPProxyHealthCheckOptions(map[string]string{
		mcpProxyHealthCheckOptionLeaseOwner: "other-instance",
		mcpProxyHealthCheckOptionLeaseUntil: fmt.Sprintf("%d", now+300),
	}))
	run, err := RunMCPProxyHealthCheckOnce(context.Background(), false, dto.MCPProxyHealthCheckRunRequest{})
	require.NoError(t, err)
	require.Equal(t, "running", run.Status)
	require.Contains(t, run.Message, "another instance")

	require.NoError(t, updateMCPProxyHealthCheckOptions(map[string]string{
		mcpProxyHealthCheckOptionLeaseOwner: "expired-instance",
		mcpProxyHealthCheckOptionLeaseUntil: fmt.Sprintf("%d", now-1),
	}))
	run, err = RunMCPProxyHealthCheckOnce(context.Background(), false, dto.MCPProxyHealthCheckRunRequest{})
	require.NoError(t, err)
	require.Equal(t, "success", run.Status)
	require.Equal(t, "", mcpProxyHealthCheckOptionString(mcpProxyHealthCheckOptionLeaseOwner, ""))
	require.EqualValues(t, 0, mcpProxyHealthCheckOptionInt64(mcpProxyHealthCheckOptionLeaseUntil, -1))
}

func TestMCPProxyHeartbeatPingsActiveSessions(t *testing.T) {
	setupMCPProxyServiceTestDB(t)

	active, err := CreateMCPProxyServerForAdmin(dto.MCPProxyServerCreateRequest{
		Name:      "Streamable MCP",
		Namespace: "streamable",
		Transport: model.MCPProxyTransportStreamableHTTP,
		Endpoint:  "https://mcp.example.com/mcp",
		Status:    model.MCPProxyServerStatusEnabled,
	})
	require.NoError(t, err)
	inactive, err := CreateMCPProxyServerForAdmin(dto.MCPProxyServerCreateRequest{
		Name:      "SSE MCP",
		Namespace: "sse",
		Transport: model.MCPProxyTransportSSE,
		Endpoint:  "https://sse.example.com/mcp",
		Status:    model.MCPProxyServerStatusEnabled,
	})
	require.NoError(t, err)

	now := common.GetTimestamp()
	fake := &fakeMCPProxyHeartbeatClient{
		fakeMCPProxyClient: &fakeMCPProxyClient{
			testResult: mcpproxy.TestResult{
				ProtocolVersion: "2025-06-18",
				ServerName:      "streamable",
				Capabilities:    map[string]any{"ping": true},
			},
		},
		snapshots: map[int]mcpproxy.SessionSnapshot{
			active.Id: {
				Transport:      model.MCPProxyTransportStreamableHTTP,
				HasSession:     true,
				Initialized:    true,
				LastActivityAt: now,
			},
			inactive.Id: {
				Transport: model.MCPProxyTransportSSE,
			},
		},
	}
	restore := setMCPProxyClientForTest(fake)
	defer restore()

	run, err := RunMCPProxyHeartbeatOnce(context.Background(), true)
	require.NoError(t, err)
	require.Equal(t, "success", run.Status)
	require.EqualValues(t, 2, run.ScannedCount)
	require.EqualValues(t, 1, run.PingedCount)
	require.EqualValues(t, 1, run.SuccessCount)
	require.EqualValues(t, 1, run.SkippedCount)
	require.EqualValues(t, 1, fake.testCalls)
	require.Equal(t, "heartbeat", run.Items[0].Action)
	require.Equal(t, "skip", run.Items[1].Action)
	require.Equal(t, "no_active_session", run.Items[1].SkippedReason)

	status := GetMCPProxyHeartbeatStatus()
	require.Equal(t, "success", status.LastRunStatus)
	require.NotNil(t, status.LastRun)
	require.EqualValues(t, 1, status.LastRun.PingedCount)
}

func TestMCPProxyHeartbeatHonorsCrossProcessLease(t *testing.T) {
	setupMCPProxyServiceTestDB(t)

	now := common.GetTimestamp()
	require.NoError(t, updateMCPProxyHealthCheckOptions(map[string]string{
		mcpProxyHeartbeatOptionLeaseOwner: "other-host:999",
		mcpProxyHeartbeatOptionLeaseUntil: fmt.Sprintf("%d", now+300),
	}))
	fake := &fakeMCPProxyHeartbeatClient{
		fakeMCPProxyClient: &fakeMCPProxyClient{},
		snapshots:          map[int]mcpproxy.SessionSnapshot{},
	}
	restore := setMCPProxyClientForTest(fake)
	defer restore()

	run, err := RunMCPProxyHeartbeatOnce(context.Background(), false)
	require.NoError(t, err)
	require.Equal(t, "running", run.Status)
	require.Equal(t, "MCP proxy heartbeat is running on another instance", run.Message)
	require.EqualValues(t, 0, fake.testCalls)
}

func TestMCPProxyDefaultHTTPClientDiscoverAndCall(t *testing.T) {
	setupMCPProxyServiceTestDB(t)
	user, token := seedMCPBillingUserAndToken(t, 100000, 100000, false)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		switch req.Method {
		case dto.MCPMethodToolsList:
			writeMCPProxyServiceJSONRPCResult(t, w, req.ID, map[string]any{
				"tools": []map[string]any{
					{
						"name":        "search_repos",
						"description": "Search repositories",
						"inputSchema": map[string]any{"type": "object"},
					},
				},
			})
		case dto.MCPMethodToolsCall:
			writeMCPProxyServiceJSONRPCResult(t, w, req.ID, map[string]any{
				"content": []map[string]any{{"type": "text", "text": "http-ok"}},
				"metadata": map[string]any{
					"downstream_request_id": "http-1",
				},
			})
		default:
			t.Fatalf("unexpected method %s", req.Method)
		}
	}))
	defer upstream.Close()

	created, err := CreateMCPProxyServerForAdmin(dto.MCPProxyServerCreateRequest{
		Name:      "HTTP MCP",
		Namespace: "httpmcp",
		Transport: model.MCPProxyTransportHTTP,
		Endpoint:  upstream.URL,
		Status:    model.MCPProxyServerStatusEnabled,
	})
	require.NoError(t, err)

	discovered, err := DiscoverMCPProxyServerToolsForAdmin(context.Background(), created.Id)
	require.NoError(t, err)
	require.Len(t, discovered.Tools, 1)

	_, err = model.UpdateMCPToolFields(discovered.Tools[0].MCPToolId, map[string]any{
		"status":         model.MCPToolStatusEnabled,
		"price_per_call": 0.001,
	})
	require.NoError(t, err)
	_, err = model.UpdateMCPProxyToolFields(discovered.Tools[0].Id, map[string]any{
		"status": model.MCPProxyToolStatusEnabled,
	})
	require.NoError(t, err)

	resp, err := CallMCPTool(MCPToolCallRequest{
		Context:        context.Background(),
		UserId:         user.Id,
		TokenId:        token.Id,
		TokenKey:       token.Key,
		TokenQuota:     token.RemainQuota,
		TokenUnlimited: token.UnlimitedQuota,
		UsingGroup:     "default",
		RequestId:      "mcp-proxy-http-call",
		Params: dto.MCPToolCallParams{
			Name:      "httpmcp.search_repos",
			Arguments: map[string]any{"query": "data-proxy"},
		},
	})
	require.NoError(t, err)
	require.NotNil(t, resp.Result)
	require.Equal(t, "http-ok", resp.Result.Content[0].Text)
	require.Equal(t, "http-1", resp.Result.Metadata["downstream_request_id"])
	require.Equal(t, "search_repos", resp.Result.Metadata["downstream_tool_name"])
}

func TestMCPProxyQidianBridgeTransportDiscoverAndCall(t *testing.T) {
	setupMCPProxyServiceTestDB(t)
	user, token := seedMCPBillingUserAndToken(t, 100000, 100000, false)

	hub := bridge.NewHub()
	outbound := make(chan bridge.OutboundMessage, 2)
	hub.Register(bridge.Session{
		SessionId:    "qidian-proxy-session",
		ClientId:     "qidian-proxy-client",
		UserId:       user.Id,
		TokenId:      token.Id,
		Capabilities: []string{mcpproxy.BridgeCapabilityMCPProxy},
		Send:         outbound,
	})
	restore := setMCPProxyClientForTest(mcpproxy.NewBridgeClient(hub))
	defer restore()

	created, err := CreateMCPProxyServerForAdmin(dto.MCPProxyServerCreateRequest{
		Name:      "Qidian Local MCP",
		Namespace: "qidian",
		Transport: model.MCPProxyTransportQidianBrowser,
		Endpoint:  "bridge://qidian-proxy-client?target=http%3A%2F%2F127.0.0.1%3A8765%2Fmcp",
		Status:    model.MCPProxyServerStatusEnabled,
	})
	require.NoError(t, err)

	discoverDone := make(chan struct{})
	go func() {
		defer close(discoverDone)
		msg := <-outbound
		require.Equal(t, bridge.MessageTypeToolCall, msg.Type)
		req := msg.Data.(dto.BridgeToolCallRequest)
		require.Equal(t, mcpproxy.BridgeToolMCPProxyListTools, req.ToolName)
		require.Equal(t, "http://127.0.0.1:8765/mcp", req.Arguments["target"])
		require.True(t, hub.CompleteToolCall(msg.Id, dto.BridgeToolCallResult{
			Metadata: map[string]any{
				"tools": []map[string]any{
					{
						"name":        "qidian_status",
						"title":       "Qidian Status",
						"description": "Return Qidian MCP status",
						"inputSchema": map[string]any{"type": "object"},
					},
				},
			},
		}))
	}()

	discovered, err := DiscoverMCPProxyServerToolsForAdmin(context.Background(), created.Id)
	require.NoError(t, err)
	<-discoverDone
	require.Len(t, discovered.Tools, 1)
	require.Equal(t, "qidian.qidian_status", discovered.Tools[0].ExposedToolName)

	_, err = model.UpdateMCPToolFields(discovered.Tools[0].MCPToolId, map[string]any{
		"status":         model.MCPToolStatusEnabled,
		"price_per_call": 0.001,
	})
	require.NoError(t, err)
	_, err = model.UpdateMCPProxyToolFields(discovered.Tools[0].Id, map[string]any{
		"status": model.MCPProxyToolStatusEnabled,
	})
	require.NoError(t, err)

	callDone := make(chan struct{})
	go func() {
		defer close(callDone)
		msg := <-outbound
		require.Equal(t, bridge.MessageTypeToolCall, msg.Type)
		req := msg.Data.(dto.BridgeToolCallRequest)
		require.Equal(t, mcpproxy.BridgeToolMCPProxyCallTool, req.ToolName)
		require.Equal(t, "qidian_status", req.Arguments["name"])
		require.True(t, hub.CompleteToolCall(msg.Id, dto.BridgeToolCallResult{
			Content: []dto.MCPContentBlock{{Type: "text", Text: "qidian-ok"}},
			Metadata: map[string]any{
				"downstream_request_id": "bridge-proxy-1",
			},
			Summary:    "qidian-ok",
			DurationMS: 11,
			ResultSize: 42,
		}))
	}()

	resp, err := CallMCPTool(MCPToolCallRequest{
		Context:        context.Background(),
		UserId:         user.Id,
		TokenId:        token.Id,
		TokenKey:       token.Key,
		TokenQuota:     token.RemainQuota,
		TokenUnlimited: token.UnlimitedQuota,
		UsingGroup:     "default",
		RequestId:      "mcp-proxy-qidian-bridge-call",
		Params: dto.MCPToolCallParams{
			Name:      "qidian.qidian_status",
			Arguments: map[string]any{},
		},
	})
	require.NoError(t, err)
	<-callDone
	require.NotNil(t, resp.Result)
	require.Equal(t, "qidian-ok", resp.Result.Content[0].Text)
	require.Equal(t, "bridge-proxy-1", resp.Result.Metadata["downstream_request_id"])
	require.Equal(t, model.MCPProxyTransportQidianBrowser, resp.Result.Metadata["transport"])

	var call model.MCPToolCall
	require.NoError(t, model.DB.Where("request_id = ?", "mcp-proxy-qidian-bridge-call").First(&call).Error)
	require.Equal(t, model.MCPToolCallStatusSuccess, call.Status)
	require.Equal(t, "qidian-proxy-session", call.BridgeSessionId)
	require.Equal(t, "qidian-proxy-client", call.TargetClient)
	callMetadata := decodeMCPToolCallMetadata(t, call)
	require.Equal(t, "qidian_status", callMetadata["downstream_tool_name"])
	require.Equal(t, model.MCPProxyTransportQidianBrowser, callMetadata["transport"])

	var audit model.BridgeAuditLog
	require.NoError(t, model.DB.Where("request_id = ?", "mcp-proxy-qidian-bridge-call").First(&audit).Error)
	require.Equal(t, model.BridgeAuditStatusSuccess, audit.Status)
	require.Equal(t, mcpproxy.BridgeToolMCPProxyCallTool, audit.ToolName)
	require.Equal(t, "qidian-proxy-session", audit.SessionId)
	require.Equal(t, "qidian-proxy-client", audit.ClientId)
}

func TestMCPProxyQidianBridgeToolErrorRefundsBilling(t *testing.T) {
	setupMCPProxyServiceTestDB(t)
	user, token := seedMCPBillingUserAndToken(t, 100000, 100000, false)

	hub := bridge.NewHub()
	outbound := make(chan bridge.OutboundMessage, 1)
	hub.Register(bridge.Session{
		SessionId:    "qidian-proxy-error-session",
		ClientId:     "qidian-proxy-error-client",
		UserId:       user.Id,
		TokenId:      token.Id,
		Capabilities: []string{mcpproxy.BridgeCapabilityMCPProxy},
		Send:         outbound,
	})
	restore := setMCPProxyClientForTest(mcpproxy.NewBridgeClient(hub))
	defer restore()

	created, err := CreateMCPProxyServerForAdmin(dto.MCPProxyServerCreateRequest{
		Name:      "Qidian Local MCP",
		Namespace: "qidian",
		Transport: model.MCPProxyTransportQidianBrowser,
		Endpoint:  "bridge://qidian-proxy-error-client",
		Status:    model.MCPProxyServerStatusEnabled,
	})
	require.NoError(t, err)
	mcpTool := &model.MCPTool{
		Name:         "qidian.qidian_status",
		DisplayName:  "Qidian Status",
		Description:  "Qidian Status",
		Category:     "third_party",
		Source:       model.MCPToolSourceMCPProxy,
		InputSchema:  `{"type":"object"}`,
		PricePerCall: 0.001,
		PriceUnit:    model.MCPToolPriceUnitPerCall,
		Status:       model.MCPToolStatusEnabled,
	}
	require.NoError(t, model.CreateMCPTool(mcpTool))
	proxyTool := &model.MCPProxyTool{
		ProxyServerId:         created.Id,
		MCPToolId:             mcpTool.Id,
		DownstreamToolName:    "qidian_status",
		DownstreamTitle:       "Qidian Status",
		DownstreamDescription: "Qidian Status",
		DownstreamInputSchema: `{"type":"object"}`,
		ExposedToolName:       "qidian.qidian_status",
		ExposedDescription:    "Qidian Status",
		SchemaHash:            "sha256:test",
		Status:                model.MCPProxyToolStatusEnabled,
	}
	require.NoError(t, model.CreateMCPProxyTool(proxyTool))

	callDone := make(chan struct{})
	go func() {
		defer close(callDone)
		msg := <-outbound
		require.Equal(t, bridge.MessageTypeToolCall, msg.Type)
		require.True(t, hub.FailToolCall(msg.Id, "QIDIAN_PERMISSION_DENIED", "browser denied proxy call"))
	}()

	resp, err := CallMCPTool(MCPToolCallRequest{
		Context:        context.Background(),
		UserId:         user.Id,
		TokenId:        token.Id,
		TokenKey:       token.Key,
		TokenQuota:     token.RemainQuota,
		TokenUnlimited: token.UnlimitedQuota,
		UsingGroup:     "default",
		RequestId:      "mcp-proxy-qidian-bridge-error",
		Params: dto.MCPToolCallParams{
			Name:      "qidian.qidian_status",
			Arguments: map[string]any{},
		},
	})
	require.NoError(t, err)
	<-callDone
	require.Nil(t, resp.Result)
	require.Equal(t, dto.MCPErrorCodeExecutorFailed, resp.ErrorCode)

	var call model.MCPToolCall
	require.NoError(t, model.DB.Where("request_id = ?", "mcp-proxy-qidian-bridge-error").First(&call).Error)
	require.Equal(t, model.MCPToolCallStatusError, call.Status)
	require.Equal(t, "QIDIAN_PERMISSION_DENIED", call.ErrorCode)
	require.Equal(t, "qidian-proxy-error-session", call.BridgeSessionId)
	require.Equal(t, "qidian-proxy-error-client", call.TargetClient)
	require.Equal(t, 0, call.Quota)
	assertMCPQuotaState(t, user.Id, token.Id, 100000, 100000, 0, 0, false)
	assertMCPBillingDebitAndRefund(t, "mcp-proxy-qidian-bridge-error", call.Id, 500, "QIDIAN_PERMISSION_DENIED")

	var audit model.BridgeAuditLog
	require.NoError(t, model.DB.Where("request_id = ?", "mcp-proxy-qidian-bridge-error").First(&audit).Error)
	require.Equal(t, model.BridgeAuditStatusError, audit.Status)
	require.Equal(t, "QIDIAN_PERMISSION_DENIED", audit.ErrorCode)
}

func setupMCPProxyServiceTestDB(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.User{},
		&model.Token{},
		&model.BillingEvent{},
		&model.MCPTool{},
		&model.MCPToolCall{},
		&model.MCPUserDailyQuota{},
		&model.MCPOpenAPITool{},
		&model.MCPOpenAPIBinaryObject{},
		&model.MCPProxyServer{},
		&model.MCPProxyTool{},
		&model.MCPProxyDiscoveryEvent{},
		&model.Option{},
		&model.BridgeClient{},
		&model.BridgeSession{},
		&model.BridgeAuditLog{},
	))
	originalDB := model.DB
	model.DB = db
	common.OptionMapRWMutex.Lock()
	originalOptionMap := common.OptionMap
	common.OptionMap = map[string]string{}
	common.OptionMapRWMutex.Unlock()
	common.QuotaPerUnit = 500000
	t.Cleanup(func() {
		model.DB = originalDB
		common.OptionMapRWMutex.Lock()
		common.OptionMap = originalOptionMap
		common.OptionMapRWMutex.Unlock()
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
}

func writeMCPProxyServiceJSONRPCResult(t *testing.T, w http.ResponseWriter, id json.RawMessage, result any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  result,
	}))
}

type fakeMCPProxyClient struct {
	testResult mcpproxy.TestResult
	tools      []mcpproxy.ToolDefinition
	callResult mcpproxy.CallResult
	lastCall   mcpproxy.CallRequest
	testErr    error
	listErr    error
	callErr    error
	testCalls  int
	listCalls  int
}

type fakeMCPProxyHeartbeatClient struct {
	*fakeMCPProxyClient
	snapshots map[int]mcpproxy.SessionSnapshot
}

func (client *fakeMCPProxyHeartbeatClient) SessionSnapshot(server model.MCPProxyServer) mcpproxy.SessionSnapshot {
	if client == nil {
		return mcpproxy.SessionSnapshot{}
	}
	snapshot := client.snapshots[server.Id]
	if snapshot.Transport == "" {
		snapshot.Transport = server.Transport
	}
	return snapshot
}

func (client *fakeMCPProxyClient) Test(ctx context.Context, server model.MCPProxyServer) (mcpproxy.TestResult, error) {
	client.testCalls++
	if client.testErr != nil {
		return mcpproxy.TestResult{}, client.testErr
	}
	return client.testResult, nil
}

func (client *fakeMCPProxyClient) ListTools(ctx context.Context, server model.MCPProxyServer) ([]mcpproxy.ToolDefinition, error) {
	client.listCalls++
	if client.listErr != nil {
		return nil, client.listErr
	}
	return client.tools, nil
}

func (client *fakeMCPProxyClient) CallTool(ctx context.Context, server model.MCPProxyServer, req mcpproxy.CallRequest) (mcpproxy.CallResult, error) {
	client.lastCall = req
	if client.callErr != nil {
		return mcpproxy.CallResult{DurationMS: 7}, client.callErr
	}
	return client.callResult, nil
}

func numericMetadataValue(value any) float64 {
	switch typed := value.(type) {
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case float64:
		return typed
	default:
		return 0
	}
}

func decodeMCPToolCallMetadata(t *testing.T, call model.MCPToolCall) map[string]any {
	t.Helper()
	metadata := map[string]any{}
	require.NoError(t, common.UnmarshalJsonStr(call.Metadata, &metadata))
	return metadata
}
