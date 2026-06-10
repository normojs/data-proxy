package model

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestMCPProxyModelsPersistDefaultsAndConstraints(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&MCPTool{}, &MCPProxyServer{}, &MCPProxyTool{}, &MCPProxyDiscoveryEvent{}))

	originalDB := DB
	DB = db
	t.Cleanup(func() {
		DB = originalDB
	})

	server := &MCPProxyServer{
		Name:      "  GitHub MCP  ",
		Namespace: "  github  ",
		Endpoint:  "  https://mcp.example.com/mcp  ",
		AuthRef:   "  secret:mcp/github-token  ",
	}
	require.NoError(t, CreateMCPProxyServer(server))
	require.Positive(t, server.Id)
	require.Equal(t, "GitHub MCP", server.Name)
	require.Equal(t, "github", server.Namespace)
	require.Equal(t, MCPProxyTransportHTTP, server.Transport)
	require.Equal(t, MCPProxyAuthTypeNone, server.AuthType)
	require.Equal(t, MCPProxyVisibilityAdmin, server.Visibility)
	require.Equal(t, MCPProxyServerStatusDisabled, server.Status)
	require.Equal(t, 30000, server.TimeoutMS)
	require.Equal(t, 1048576, server.MaxResultSize)
	require.Equal(t, 65536, server.MaxMetadataSize)
	require.NotZero(t, server.CreatedAt)
	require.NotZero(t, server.UpdatedAt)

	loadedServer, err := GetMCPProxyServerByNamespace("github")
	require.NoError(t, err)
	require.Equal(t, server.Id, loadedServer.Id)

	duplicateServer := &MCPProxyServer{Name: "GitHub Duplicate", Namespace: "github"}
	require.Error(t, CreateMCPProxyServer(duplicateServer))

	exposedTool := &MCPTool{
		Name:        "github.search_repos",
		DisplayName: "GitHub Search Repos",
		Description: "Search repositories through downstream MCP",
		Category:    "third_party",
		Source:      MCPToolSourceMCPProxy,
		InputSchema: `{"type":"object"}`,
		Status:      MCPToolStatusDisabled,
	}
	require.NoError(t, db.Create(exposedTool).Error)

	proxyTool := &MCPProxyTool{
		ProxyServerId:         server.Id,
		MCPToolId:             exposedTool.Id,
		DownstreamToolName:    "  search_repos  ",
		DownstreamTitle:       "  Search Repos  ",
		DownstreamDescription: "Search repositories",
		DownstreamInputSchema: `{"type":"object"}`,
		ExposedToolName:       "  github.search_repos  ",
		SchemaHash:            "  sha256:test  ",
	}
	require.NoError(t, CreateMCPProxyTool(proxyTool))
	require.Positive(t, proxyTool.Id)
	require.Equal(t, "search_repos", proxyTool.DownstreamToolName)
	require.Equal(t, "Search Repos", proxyTool.DownstreamTitle)
	require.Equal(t, "github.search_repos", proxyTool.ExposedToolName)
	require.Equal(t, "sha256:test", proxyTool.SchemaHash)
	require.Equal(t, MCPProxyToolStatusPending, proxyTool.Status)

	loadedTool, err := GetMCPProxyToolByExposedName("github.search_repos")
	require.NoError(t, err)
	require.Equal(t, proxyTool.Id, loadedTool.Id)

	duplicateDownstreamTool := &MCPProxyTool{
		ProxyServerId:         server.Id,
		DownstreamToolName:    "search_repos",
		DownstreamInputSchema: `{"type":"object"}`,
		ExposedToolName:       "github.search_repos_duplicate",
		SchemaHash:            "sha256:test",
	}
	require.Error(t, CreateMCPProxyTool(duplicateDownstreamTool))

	duplicateExposedTool := &MCPProxyTool{
		ProxyServerId:         server.Id,
		DownstreamToolName:    "list_issues",
		DownstreamInputSchema: `{"type":"object"}`,
		ExposedToolName:       "github.search_repos",
		SchemaHash:            "sha256:other",
	}
	require.Error(t, CreateMCPProxyTool(duplicateExposedTool))

	event := &MCPProxyDiscoveryEvent{
		ProxyServerId:   server.Id,
		EventType:       MCPProxyDiscoveryEventTypeDiscover,
		Status:          MCPProxyDiscoveryEventStatusSuccess,
		DiscoveredCount: 1,
	}
	require.NoError(t, CreateMCPProxyDiscoveryEvent(event))
	require.Positive(t, event.Id)
	require.NotZero(t, event.StartedAt)
	require.NotZero(t, event.FinishedAt)

	events, total, err := ListMCPProxyDiscoveryEventsByServerId(server.Id, 0, 10)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, events, 1)
	require.Equal(t, MCPProxyDiscoveryEventTypeDiscover, events[0].EventType)
}
