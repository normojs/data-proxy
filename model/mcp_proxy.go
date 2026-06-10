package model

import (
	"strings"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
)

const (
	MCPProxyTransportHTTP           = "http"
	MCPProxyTransportSSE            = "sse"
	MCPProxyTransportStreamableHTTP = "streamable_http"
	MCPProxyTransportStdio          = "stdio"
	MCPProxyTransportLocalhost      = "localhost"
	MCPProxyTransportBridge         = "bridge"
	MCPProxyTransportQidianBrowser  = "qidian_browser"
)

const (
	MCPProxyAuthTypeNone   = "none"
	MCPProxyAuthTypeBearer = "bearer"
	MCPProxyAuthTypeBasic  = "basic"
	MCPProxyAuthTypeHeader = "header"
	MCPProxyAuthTypeOAuth  = "oauth"
)

const (
	MCPProxyServerStatusEnabled  = "enabled"
	MCPProxyServerStatusDisabled = "disabled"
	MCPProxyServerStatusError    = "error"
	MCPProxyServerStatusArchived = "archived"
)

const (
	MCPProxyVisibilityAdmin  = "admin"
	MCPProxyVisibilityGroup  = "group"
	MCPProxyVisibilityPublic = "public"
)

const (
	MCPProxyToolStatusPending       = "pending"
	MCPProxyToolStatusEnabled       = "enabled"
	MCPProxyToolStatusDisabled      = "disabled"
	MCPProxyToolStatusSchemaChanged = "schema_changed"
	MCPProxyToolStatusError         = "error"
)

const (
	MCPProxyDiscoveryEventTypeTest     = "test"
	MCPProxyDiscoveryEventTypeDiscover = "discover"

	MCPProxyDiscoveryEventStatusSuccess = "success"
	MCPProxyDiscoveryEventStatusError   = "error"
)

type MCPProxyServer struct {
	Id        int    `json:"id"`
	Name      string `json:"name" gorm:"type:varchar(128);not null"`
	Namespace string `json:"namespace" gorm:"type:varchar(64);not null;uniqueIndex"`

	Transport string `json:"transport" gorm:"type:varchar(32);not null;index"`
	Endpoint  string `json:"endpoint" gorm:"type:varchar(512);not null;default:''"`
	Command   string `json:"command" gorm:"type:text"`

	AuthType        string `json:"auth_type" gorm:"type:varchar(32);not null;default:'none'"`
	AuthRef         string `json:"auth_ref" gorm:"type:varchar(256);not null;default:''"`
	TimeoutMS       int    `json:"timeout_ms" gorm:"not null;default:30000"`
	MaxResultSize   int    `json:"max_result_size" gorm:"not null;default:1048576"`
	MaxMetadataSize int    `json:"max_metadata_size" gorm:"not null;default:65536"`

	Visibility    string `json:"visibility" gorm:"type:varchar(32);not null;default:'admin'"`
	AllowedGroups string `json:"allowed_groups" gorm:"type:text"`

	Status           string         `json:"status" gorm:"type:varchar(32);not null;default:'disabled';index"`
	LastError        string         `json:"last_error" gorm:"type:varchar(512);not null;default:''"`
	LastDiscoveredAt int64          `json:"last_discovered_at" gorm:"bigint;index"`
	CreatedAt        int64          `json:"created_at" gorm:"bigint"`
	UpdatedAt        int64          `json:"updated_at" gorm:"bigint"`
	DeletedAt        gorm.DeletedAt `json:"-" gorm:"index"`
}

func (MCPProxyServer) TableName() string {
	return "mcp_proxy_servers"
}

func (server *MCPProxyServer) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	server.CreatedAt = now
	server.UpdatedAt = now
	normalizeMCPProxyServer(server)
	return nil
}

func (server *MCPProxyServer) BeforeUpdate(tx *gorm.DB) error {
	server.UpdatedAt = common.GetTimestamp()
	normalizeMCPProxyServer(server)
	return nil
}

type MCPProxyTool struct {
	Id            int64 `json:"id"`
	ProxyServerId int   `json:"proxy_server_id" gorm:"not null;uniqueIndex:idx_mcp_proxy_tools_server_downstream,priority:1;index"`
	MCPToolId     int   `json:"mcp_tool_id" gorm:"index"`

	DownstreamToolName    string         `json:"downstream_tool_name" gorm:"type:varchar(128);not null;uniqueIndex:idx_mcp_proxy_tools_server_downstream,priority:2"`
	DownstreamTitle       string         `json:"downstream_title" gorm:"type:varchar(256);not null;default:''"`
	DownstreamDescription string         `json:"downstream_description" gorm:"type:text"`
	DownstreamInputSchema string         `json:"downstream_input_schema" gorm:"type:text;not null"`
	ExposedToolName       string         `json:"exposed_tool_name" gorm:"type:varchar(160);not null;uniqueIndex"`
	ExposedDescription    string         `json:"exposed_description" gorm:"type:text"`
	SchemaHash            string         `json:"schema_hash" gorm:"type:varchar(128);not null;index"`
	Status                string         `json:"status" gorm:"type:varchar(32);not null;default:'pending';index"`
	LastError             string         `json:"last_error" gorm:"type:varchar(512);not null;default:''"`
	LastDiscoveredAt      int64          `json:"last_discovered_at" gorm:"bigint;index"`
	CreatedAt             int64          `json:"created_at" gorm:"bigint"`
	UpdatedAt             int64          `json:"updated_at" gorm:"bigint"`
	DeletedAt             gorm.DeletedAt `json:"-" gorm:"index"`
}

func (MCPProxyTool) TableName() string {
	return "mcp_proxy_tools"
}

func (tool *MCPProxyTool) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	tool.CreatedAt = now
	tool.UpdatedAt = now
	normalizeMCPProxyTool(tool)
	return nil
}

func (tool *MCPProxyTool) BeforeUpdate(tx *gorm.DB) error {
	tool.UpdatedAt = common.GetTimestamp()
	normalizeMCPProxyTool(tool)
	return nil
}

type MCPProxyDiscoveryEvent struct {
	Id            int64  `json:"id"`
	ProxyServerId int    `json:"proxy_server_id" gorm:"not null;index"`
	EventType     string `json:"event_type" gorm:"type:varchar(32);not null;index"`
	Status        string `json:"status" gorm:"type:varchar(32);not null;index"`
	Message       string `json:"message" gorm:"type:varchar(1024);not null;default:''"`

	ProtocolVersion string `json:"protocol_version" gorm:"type:varchar(64);not null;default:''"`
	ServerName      string `json:"server_name" gorm:"type:varchar(128);not null;default:''"`
	Capabilities    string `json:"capabilities" gorm:"type:text"`

	DiscoveredCount int `json:"discovered_count" gorm:"not null;default:0"`
	CreatedCount    int `json:"created_count" gorm:"not null;default:0"`
	UpdatedCount    int `json:"updated_count" gorm:"not null;default:0"`
	DisabledCount   int `json:"disabled_count" gorm:"not null;default:0"`
	SchemaChanged   int `json:"schema_changed" gorm:"not null;default:0"`

	DurationMS int   `json:"duration_ms" gorm:"not null;default:0"`
	StartedAt  int64 `json:"started_at" gorm:"bigint;index"`
	FinishedAt int64 `json:"finished_at" gorm:"bigint;index"`
	CreatedAt  int64 `json:"created_at" gorm:"bigint;index"`
}

func (MCPProxyDiscoveryEvent) TableName() string {
	return "mcp_proxy_discovery_events"
}

func (event *MCPProxyDiscoveryEvent) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	if event.CreatedAt == 0 {
		event.CreatedAt = now
	}
	if event.StartedAt == 0 {
		event.StartedAt = now
	}
	if event.FinishedAt == 0 {
		event.FinishedAt = event.StartedAt
	}
	normalizeMCPProxyDiscoveryEvent(event)
	return nil
}

func CreateMCPProxyServer(server *MCPProxyServer) error {
	return DB.Create(server).Error
}

type MCPProxyServerFilter struct {
	Transport string
	Status    string
	Keyword   string
}

func ListMCPProxyServers(filter MCPProxyServerFilter, offset int, limit int) ([]MCPProxyServer, int64, error) {
	query := DB.Model(&MCPProxyServer{})
	query = applyMCPProxyServerFilter(query, filter)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	items := make([]MCPProxyServer, 0, limit)
	err := query.Order("id desc").
		Limit(limit).
		Offset(offset).
		Find(&items).Error
	return items, total, err
}

func ListMCPProxyServersForHealthCheck(limit int) ([]MCPProxyServer, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	items := make([]MCPProxyServer, 0, limit)
	err := DB.Where("status IN ?", []string{MCPProxyServerStatusEnabled, MCPProxyServerStatusError}).
		Order("id asc").
		Limit(limit).
		Find(&items).Error
	return items, err
}

func ListMCPProxyServersForHeartbeat(limit int) ([]MCPProxyServer, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	items := make([]MCPProxyServer, 0, limit)
	err := DB.Where("status IN ?", []string{MCPProxyServerStatusEnabled, MCPProxyServerStatusError}).
		Order("id asc").
		Limit(limit).
		Find(&items).Error
	return items, err
}

func GetMCPProxyServerById(id int) (*MCPProxyServer, error) {
	var server MCPProxyServer
	err := DB.First(&server, "id = ?", id).Error
	return &server, err
}

func GetMCPProxyServerByNamespace(namespace string) (*MCPProxyServer, error) {
	var server MCPProxyServer
	err := DB.Where("namespace = ?", strings.TrimSpace(namespace)).First(&server).Error
	return &server, err
}

func UpdateMCPProxyServerFields(id int, updates map[string]any) (*MCPProxyServer, error) {
	if err := DB.Model(&MCPProxyServer{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return nil, err
	}
	return GetMCPProxyServerById(id)
}

func ArchiveMCPProxyServer(id int) (*MCPProxyServer, error) {
	if err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&MCPProxyServer{}).
			Where("id = ?", id).
			Update("status", MCPProxyServerStatusArchived).Error; err != nil {
			return err
		}
		if err := tx.Delete(&MCPProxyServer{}, id).Error; err != nil {
			return err
		}
		if err := tx.Model(&MCPProxyTool{}).
			Where("proxy_server_id = ?", id).
			Update("status", MCPProxyToolStatusDisabled).Error; err != nil {
			return err
		}
		if err := tx.Where("proxy_server_id = ?", id).Delete(&MCPProxyTool{}).Error; err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}
	var server MCPProxyServer
	err := DB.Unscoped().First(&server, "id = ?", id).Error
	return &server, err
}

func CreateMCPProxyTool(tool *MCPProxyTool) error {
	return DB.Create(tool).Error
}

type MCPProxyToolFilter struct {
	ProxyServerId int
	Status        string
	SchemaHash    string
	Keyword       string
}

func ListMCPProxyTools(filter MCPProxyToolFilter, offset int, limit int) ([]MCPProxyTool, int64, error) {
	query := DB.Model(&MCPProxyTool{})
	query = applyMCPProxyToolFilter(query, filter)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	items := make([]MCPProxyTool, 0, limit)
	err := query.Order("id desc").
		Limit(limit).
		Offset(offset).
		Find(&items).Error
	return items, total, err
}

func ListMCPProxyToolsByServerId(proxyServerId int) ([]MCPProxyTool, error) {
	items := make([]MCPProxyTool, 0)
	err := DB.Where("proxy_server_id = ?", proxyServerId).
		Order("id asc").
		Find(&items).Error
	return items, err
}

func ListMCPProxyToolsByServerIds(proxyServerIds []int) ([]MCPProxyTool, error) {
	ids := uniquePositiveInts(proxyServerIds)
	if len(ids) == 0 {
		return []MCPProxyTool{}, nil
	}
	items := make([]MCPProxyTool, 0)
	err := DB.Where("proxy_server_id IN ?", ids).
		Order("proxy_server_id asc, id asc").
		Find(&items).Error
	return items, err
}

func GetMCPProxyToolByServerAndDownstreamName(proxyServerId int, downstreamToolName string) (*MCPProxyTool, error) {
	var tool MCPProxyTool
	err := DB.Where("proxy_server_id = ? AND downstream_tool_name = ?", proxyServerId, strings.TrimSpace(downstreamToolName)).
		First(&tool).Error
	return &tool, err
}

func GetMCPProxyToolByExposedName(name string) (*MCPProxyTool, error) {
	var tool MCPProxyTool
	err := DB.Where("exposed_tool_name = ?", strings.TrimSpace(name)).First(&tool).Error
	return &tool, err
}

func GetMCPProxyToolByMCPToolId(mcpToolId int) (*MCPProxyTool, error) {
	var tool MCPProxyTool
	err := DB.Where("mcp_tool_id = ?", mcpToolId).First(&tool).Error
	return &tool, err
}

func GetMCPProxyToolById(id int64) (*MCPProxyTool, error) {
	var tool MCPProxyTool
	err := DB.First(&tool, "id = ?", id).Error
	return &tool, err
}

func UpdateMCPProxyToolFields(id int64, updates map[string]any) (*MCPProxyTool, error) {
	if err := DB.Model(&MCPProxyTool{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return nil, err
	}
	var tool MCPProxyTool
	err := DB.First(&tool, "id = ?", id).Error
	return &tool, err
}

func CreateMCPProxyDiscoveryEvent(event *MCPProxyDiscoveryEvent) error {
	return DB.Create(event).Error
}

func ListMCPProxyDiscoveryEventsByServerId(proxyServerId int, offset int, limit int) ([]MCPProxyDiscoveryEvent, int64, error) {
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}

	query := DB.Model(&MCPProxyDiscoveryEvent{}).Where("proxy_server_id = ?", proxyServerId)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	items := make([]MCPProxyDiscoveryEvent, 0, limit)
	err := query.Order("started_at desc, id desc").Offset(offset).Limit(limit).Find(&items).Error
	return items, total, err
}

func ListLatestMCPProxyDiscoveryEventsByServerIds(proxyServerIds []int) (map[int]MCPProxyDiscoveryEvent, error) {
	ids := uniquePositiveInts(proxyServerIds)
	result := make(map[int]MCPProxyDiscoveryEvent, len(ids))
	for _, id := range ids {
		var event MCPProxyDiscoveryEvent
		err := DB.Where("proxy_server_id = ?", id).
			Order("started_at desc, id desc").
			First(&event).Error
		if err == gorm.ErrRecordNotFound {
			continue
		}
		if err != nil {
			return nil, err
		}
		result[id] = event
	}
	return result, nil
}

func applyMCPProxyServerFilter(query *gorm.DB, filter MCPProxyServerFilter) *gorm.DB {
	if strings.TrimSpace(filter.Transport) != "" {
		query = query.Where("transport = ?", strings.TrimSpace(filter.Transport))
	}
	if strings.TrimSpace(filter.Status) != "" {
		query = query.Where("status = ?", strings.TrimSpace(filter.Status))
	}
	if strings.TrimSpace(filter.Keyword) != "" {
		keyword := "%" + strings.TrimSpace(filter.Keyword) + "%"
		query = query.Where("name LIKE ? OR namespace LIKE ? OR endpoint LIKE ?", keyword, keyword, keyword)
	}
	return query
}

func applyMCPProxyToolFilter(query *gorm.DB, filter MCPProxyToolFilter) *gorm.DB {
	if filter.ProxyServerId > 0 {
		query = query.Where("proxy_server_id = ?", filter.ProxyServerId)
	}
	if strings.TrimSpace(filter.Status) != "" {
		query = query.Where("status = ?", strings.TrimSpace(filter.Status))
	}
	if strings.TrimSpace(filter.SchemaHash) != "" {
		query = query.Where("schema_hash = ?", strings.TrimSpace(filter.SchemaHash))
	}
	if strings.TrimSpace(filter.Keyword) != "" {
		keyword := "%" + strings.TrimSpace(filter.Keyword) + "%"
		query = query.Where("downstream_tool_name LIKE ? OR downstream_title LIKE ? OR downstream_description LIKE ? OR exposed_tool_name LIKE ? OR exposed_description LIKE ?",
			keyword, keyword, keyword, keyword, keyword)
	}
	return query
}

func normalizeMCPProxyServer(server *MCPProxyServer) {
	if server == nil {
		return
	}
	server.Name = strings.TrimSpace(server.Name)
	server.Namespace = strings.TrimSpace(server.Namespace)
	server.Transport = strings.TrimSpace(server.Transport)
	server.Endpoint = strings.TrimSpace(server.Endpoint)
	server.AuthType = strings.TrimSpace(server.AuthType)
	server.AuthRef = strings.TrimSpace(server.AuthRef)
	server.Visibility = strings.TrimSpace(server.Visibility)
	server.Status = strings.TrimSpace(server.Status)
	if server.Transport == "" {
		server.Transport = MCPProxyTransportHTTP
	}
	if server.AuthType == "" {
		server.AuthType = MCPProxyAuthTypeNone
	}
	if server.Visibility == "" {
		server.Visibility = MCPProxyVisibilityAdmin
	}
	if server.Status == "" {
		server.Status = MCPProxyServerStatusDisabled
	}
	if server.TimeoutMS <= 0 {
		server.TimeoutMS = 30000
	}
	if server.MaxResultSize <= 0 {
		server.MaxResultSize = 1048576
	}
	if server.MaxMetadataSize <= 0 {
		server.MaxMetadataSize = 65536
	}
}

func normalizeMCPProxyTool(tool *MCPProxyTool) {
	if tool == nil {
		return
	}
	tool.DownstreamToolName = strings.TrimSpace(tool.DownstreamToolName)
	tool.DownstreamTitle = strings.TrimSpace(tool.DownstreamTitle)
	tool.ExposedToolName = strings.TrimSpace(tool.ExposedToolName)
	tool.SchemaHash = strings.TrimSpace(tool.SchemaHash)
	tool.Status = strings.TrimSpace(tool.Status)
	if tool.Status == "" {
		tool.Status = MCPProxyToolStatusPending
	}
}

func normalizeMCPProxyDiscoveryEvent(event *MCPProxyDiscoveryEvent) {
	if event == nil {
		return
	}
	event.EventType = strings.TrimSpace(event.EventType)
	event.Status = strings.TrimSpace(event.Status)
	event.Message = strings.TrimSpace(event.Message)
	event.ProtocolVersion = strings.TrimSpace(event.ProtocolVersion)
	event.ServerName = strings.TrimSpace(event.ServerName)
	if event.EventType == "" {
		event.EventType = MCPProxyDiscoveryEventTypeDiscover
	}
	if event.Status == "" {
		event.Status = MCPProxyDiscoveryEventStatusSuccess
	}
}
