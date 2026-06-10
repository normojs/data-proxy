package model

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
)

type MCPOpenAPITool struct {
	Id                  int64          `json:"id"`
	MCPToolId           int            `json:"mcp_tool_id" gorm:"not null;uniqueIndex"`
	OpenAPIUrl          string         `json:"openapi_url" gorm:"column:openapi_url;type:varchar(512);not null;index"`
	ServerURL           string         `json:"server_url" gorm:"type:varchar(512);not null"`
	OperationId         string         `json:"operation_id" gorm:"type:varchar(128);default:'';index"`
	OperationKey        string         `json:"operation_key" gorm:"type:varchar(256);not null;index"`
	Method              string         `json:"method" gorm:"type:varchar(16);not null"`
	Path                string         `json:"path" gorm:"type:varchar(512);not null"`
	RequestContentType  string         `json:"request_content_type" gorm:"type:varchar(128);default:'application/json'"`
	ResponseContentType string         `json:"response_content_type" gorm:"type:varchar(128);default:''"`
	AuthType            string         `json:"auth_type" gorm:"type:varchar(32);not null;default:'none'"`
	AuthRef             string         `json:"auth_ref" gorm:"type:varchar(256);not null;default:''"`
	AuthHeaderName      string         `json:"auth_header_name" gorm:"type:varchar(128);not null;default:''"`
	Parameters          string         `json:"parameters" gorm:"type:text"`
	RequestBodySchema   string         `json:"request_body_schema" gorm:"type:text"`
	CreatedAt           int64          `json:"created_at" gorm:"bigint"`
	UpdatedAt           int64          `json:"updated_at" gorm:"bigint"`
	DeletedAt           gorm.DeletedAt `json:"-" gorm:"index"`
}

func (MCPOpenAPITool) TableName() string {
	return "mcp_openapi_tools"
}

type MCPOpenAPIBinaryObject struct {
	Id               int64          `json:"id"`
	ObjectId         string         `json:"object_id" gorm:"type:varchar(64);not null;uniqueIndex"`
	Provider         string         `json:"provider" gorm:"type:varchar(32);not null;default:'';index"`
	StorageKey       string         `json:"storage_key" gorm:"type:varchar(512);not null;default:''"`
	ContentType      string         `json:"content_type" gorm:"type:varchar(128);not null;default:'application/octet-stream';index"`
	ContentFamily    string         `json:"content_family" gorm:"type:varchar(32);not null;default:'binary';index"`
	SHA256           string         `json:"sha256" gorm:"type:varchar(128);not null;default:'';index"`
	Size             int            `json:"size" gorm:"not null;default:0"`
	Filename         string         `json:"filename" gorm:"type:varchar(255);not null;default:''"`
	Disposition      string         `json:"disposition" gorm:"type:varchar(32);not null;default:'attachment'"`
	MCPToolCallId    int64          `json:"mcp_tool_call_id" gorm:"not null;default:0;index"`
	MCPToolId        int            `json:"mcp_tool_id" gorm:"not null;default:0;index"`
	OpenAPIToolId    int64          `json:"openapi_tool_id" gorm:"not null;default:0;index"`
	UserId           int            `json:"user_id" gorm:"not null;default:0;index"`
	TokenId          int            `json:"token_id" gorm:"not null;default:0;index"`
	RequestId        string         `json:"request_id" gorm:"type:varchar(128);not null;default:'';index"`
	OperationKey     string         `json:"operation_key" gorm:"type:varchar(256);not null;default:'';index"`
	ExpiresAt        int64          `json:"expires_at" gorm:"bigint;not null;default:0;index"`
	DownloadCount    int            `json:"download_count" gorm:"not null;default:0"`
	LastDownloadedAt int64          `json:"last_downloaded_at" gorm:"bigint;not null;default:0"`
	CreatedAt        int64          `json:"created_at" gorm:"bigint;index"`
	UpdatedAt        int64          `json:"updated_at" gorm:"bigint"`
	DeletedAt        gorm.DeletedAt `json:"-" gorm:"index"`
}

func (MCPOpenAPIBinaryObject) TableName() string {
	return "mcp_openapi_binary_objects"
}

func (object *MCPOpenAPIBinaryObject) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	object.normalize()
	object.CreatedAt = now
	object.UpdatedAt = now
	return nil
}

func (object *MCPOpenAPIBinaryObject) BeforeUpdate(tx *gorm.DB) error {
	object.normalize()
	object.UpdatedAt = common.GetTimestamp()
	return nil
}

func (object *MCPOpenAPIBinaryObject) normalize() {
	if object == nil {
		return
	}
	object.ObjectId = strings.TrimSpace(object.ObjectId)
	object.Provider = strings.TrimSpace(object.Provider)
	object.StorageKey = strings.TrimSpace(object.StorageKey)
	object.ContentType = strings.TrimSpace(object.ContentType)
	if object.ContentType == "" {
		object.ContentType = "application/octet-stream"
	}
	object.ContentFamily = strings.TrimSpace(object.ContentFamily)
	if object.ContentFamily == "" {
		object.ContentFamily = "binary"
	}
	object.SHA256 = strings.TrimSpace(object.SHA256)
	object.Filename = strings.TrimSpace(object.Filename)
	object.Disposition = strings.TrimSpace(object.Disposition)
	if object.Disposition == "" {
		object.Disposition = "attachment"
	}
	object.RequestId = strings.TrimSpace(object.RequestId)
	object.OperationKey = strings.TrimSpace(object.OperationKey)
}

func (tool *MCPOpenAPITool) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	tool.normalize()
	tool.CreatedAt = now
	tool.UpdatedAt = now
	return nil
}

func (tool *MCPOpenAPITool) BeforeUpdate(tx *gorm.DB) error {
	tool.normalize()
	tool.UpdatedAt = common.GetTimestamp()
	return nil
}

func (tool *MCPOpenAPITool) normalize() {
	if tool == nil {
		return
	}
	tool.OpenAPIUrl = strings.TrimSpace(tool.OpenAPIUrl)
	tool.ServerURL = strings.TrimSpace(tool.ServerURL)
	tool.OperationId = strings.TrimSpace(tool.OperationId)
	tool.OperationKey = strings.TrimSpace(tool.OperationKey)
	tool.Method = strings.ToUpper(strings.TrimSpace(tool.Method))
	tool.Path = strings.TrimSpace(tool.Path)
	tool.RequestContentType = strings.TrimSpace(tool.RequestContentType)
	if tool.RequestContentType == "" {
		tool.RequestContentType = "application/json"
	}
	tool.ResponseContentType = strings.TrimSpace(tool.ResponseContentType)
	tool.AuthType = strings.ToLower(strings.TrimSpace(tool.AuthType))
	if tool.AuthType == "" {
		tool.AuthType = MCPProxyAuthTypeNone
	}
	tool.AuthRef = strings.TrimSpace(tool.AuthRef)
	tool.AuthHeaderName = strings.TrimSpace(tool.AuthHeaderName)
}

func CreateMCPToolWithOpenAPI(tool *MCPTool, openapiTool *MCPOpenAPITool) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Select("*").Create(tool).Error; err != nil {
			return err
		}
		openapiTool.MCPToolId = tool.Id
		return tx.Select("*").Create(openapiTool).Error
	})
}

func UpdateMCPToolWithOpenAPI(mcpToolId int, toolUpdates map[string]any, openapiTool *MCPOpenAPITool) (*MCPTool, error) {
	err := DB.Transaction(func(tx *gorm.DB) error {
		if len(toolUpdates) > 0 {
			if err := tx.Model(&MCPTool{}).Where("id = ?", mcpToolId).Updates(toolUpdates).Error; err != nil {
				return err
			}
		}

		openapiTool.MCPToolId = mcpToolId
		var existing MCPOpenAPITool
		err := tx.Where("mcp_tool_id = ?", mcpToolId).First(&existing).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return tx.Select("*").Create(openapiTool).Error
		}
		if err != nil {
			return err
		}
		return tx.Model(&MCPOpenAPITool{}).
			Where("id = ?", existing.Id).
			Updates(mcpOpenAPIToolUpdateFields(openapiTool)).Error
	})
	if err != nil {
		return nil, err
	}
	return GetMCPToolById(mcpToolId)
}

func mcpOpenAPIToolUpdateFields(tool *MCPOpenAPITool) map[string]any {
	tool.normalize()
	return map[string]any{
		"openapi_url":           tool.OpenAPIUrl,
		"server_url":            tool.ServerURL,
		"operation_id":          tool.OperationId,
		"operation_key":         tool.OperationKey,
		"method":                tool.Method,
		"path":                  tool.Path,
		"request_content_type":  tool.RequestContentType,
		"response_content_type": tool.ResponseContentType,
		"auth_type":             tool.AuthType,
		"auth_ref":              tool.AuthRef,
		"auth_header_name":      tool.AuthHeaderName,
		"parameters":            tool.Parameters,
		"request_body_schema":   tool.RequestBodySchema,
		"updated_at":            common.GetTimestamp(),
	}
}

func GetMCPOpenAPIToolByMCPToolId(mcpToolId int) (*MCPOpenAPITool, error) {
	var tool MCPOpenAPITool
	err := DB.Where("mcp_tool_id = ?", mcpToolId).First(&tool).Error
	return &tool, err
}

func CreateMCPOpenAPIBinaryObject(object *MCPOpenAPIBinaryObject) error {
	if object == nil || strings.TrimSpace(object.ObjectId) == "" {
		return errors.New("openapi binary object id is required")
	}
	return DB.Select("*").Create(object).Error
}

func GetMCPOpenAPIBinaryObjectByObjectId(objectId string) (*MCPOpenAPIBinaryObject, error) {
	var object MCPOpenAPIBinaryObject
	err := DB.Where("object_id = ?", strings.TrimSpace(objectId)).First(&object).Error
	return &object, err
}

func TouchMCPOpenAPIBinaryObjectDownload(objectId string) error {
	objectId = strings.TrimSpace(objectId)
	if objectId == "" {
		return nil
	}
	now := common.GetTimestamp()
	return DB.Model(&MCPOpenAPIBinaryObject{}).
		Where("object_id = ?", objectId).
		Updates(map[string]any{
			"download_count":     gorm.Expr("download_count + ?", 1),
			"last_downloaded_at": now,
			"updated_at":         now,
		}).Error
}

func ListMCPToolsForOpenAPILifecycle(openAPIURL string, toolIds []int) ([]MCPTool, error) {
	query := DB.Where("source = ?", MCPToolSourceOpenAPI)
	openAPIURL = strings.TrimSpace(openAPIURL)
	if openAPIURL != "" {
		query = query.Where("openapi_url = ?", openAPIURL)
	} else {
		query = query.Where("id IN ?", compactPositiveIds(toolIds))
	}

	var tools []MCPTool
	err := query.Order("id asc").Find(&tools).Error
	return tools, err
}

func DisableMCPOpenAPITools(ids []int) ([]MCPTool, error) {
	ids = compactPositiveIds(ids)
	if len(ids) == 0 {
		return []MCPTool{}, nil
	}
	if err := DB.Model(&MCPTool{}).
		Where("source = ? AND id IN ?", MCPToolSourceOpenAPI, ids).
		Update("status", MCPToolStatusDisabled).Error; err != nil {
		return nil, err
	}
	var tools []MCPTool
	err := DB.Where("source = ? AND id IN ?", MCPToolSourceOpenAPI, ids).
		Order("id asc").
		Find(&tools).Error
	return tools, err
}

func DeleteMCPOpenAPITools(ids []int) error {
	ids = compactPositiveIds(ids)
	if len(ids) == 0 {
		return nil
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("mcp_tool_id IN ?", ids).Delete(&MCPOpenAPITool{}).Error; err != nil {
			return err
		}
		return tx.Where("source = ? AND id IN ?", MCPToolSourceOpenAPI, ids).Delete(&MCPTool{}).Error
	})
}

func compactPositiveIds(ids []int) []int {
	if len(ids) == 0 {
		return []int{}
	}
	result := make([]int, 0, len(ids))
	seen := map[int]bool{}
	for _, id := range ids {
		if id <= 0 || seen[id] {
			continue
		}
		seen[id] = true
		result = append(result, id)
	}
	return result
}
