package service

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	mcpopenapi "github.com/QuantumNous/new-api/pkg/mcp/openapi"
)

const (
	MCPOpenAPIBinaryExpiryActive   = "active"
	MCPOpenAPIBinaryExpiryExpired  = "expired"
	MCPOpenAPIBinaryExpiryNoExpiry = "no_expiry"
)

type MCPOpenAPIBinaryObjectListParams struct {
	Provider      string
	ContentFamily string
	ExpiryStatus  string
	Keyword       string
	UserId        int
	MCPToolId     int
	StartTime     int64
	EndTime       int64
	Offset        int
	Limit         int
}

func ListMCPOpenAPIBinaryObjectsForAdmin(params MCPOpenAPIBinaryObjectListParams) ([]dto.MCPOpenAPIBinaryObjectListItem, int64, error) {
	now := common.GetTimestamp()
	objects, total, err := model.ListMCPOpenAPIBinaryObjects(openAPIBinaryObjectFilter(params, now), params.Offset, params.Limit)
	if err != nil {
		return nil, 0, err
	}
	items := make([]dto.MCPOpenAPIBinaryObjectListItem, 0, len(objects))
	for _, object := range objects {
		items = append(items, openAPIBinaryObjectToDTO(object, now))
	}
	return items, total, nil
}

func GetMCPOpenAPIBinaryObjectSummaryForAdmin(params MCPOpenAPIBinaryObjectListParams) (dto.MCPOpenAPIBinaryObjectSummary, error) {
	now := common.GetTimestamp()
	summary, err := model.SummarizeMCPOpenAPIBinaryObjects(openAPIBinaryObjectFilter(params, now))
	if err != nil {
		return dto.MCPOpenAPIBinaryObjectSummary{}, err
	}
	return dto.MCPOpenAPIBinaryObjectSummary{
		TotalCount:          summary.TotalCount,
		TotalBytes:          summary.TotalBytes,
		ActiveCount:         summary.ActiveCount,
		ExpiredCount:        summary.ExpiredCount,
		NoExpiryCount:       summary.NoExpiryCount,
		DownloadedCount:     summary.DownloadedCount,
		DownloadCount:       summary.DownloadCount,
		DefaultTTLSeconds:   mcpopenapi.BinaryObjectTTLSeconds(),
		DefaultCleanupLimit: mcpopenapi.BinaryObjectCleanupLimit(),
		CheckedAt:           now,
	}, nil
}

func openAPIBinaryObjectFilter(params MCPOpenAPIBinaryObjectListParams, now int64) model.MCPOpenAPIBinaryObjectFilter {
	return model.MCPOpenAPIBinaryObjectFilter{
		Provider:      strings.TrimSpace(params.Provider),
		ContentFamily: strings.TrimSpace(params.ContentFamily),
		ExpiryStatus:  strings.TrimSpace(params.ExpiryStatus),
		Keyword:       strings.TrimSpace(params.Keyword),
		UserId:        params.UserId,
		MCPToolId:     params.MCPToolId,
		StartTime:     params.StartTime,
		EndTime:       params.EndTime,
		Now:           now,
	}
}

func openAPIBinaryObjectToDTO(object model.MCPOpenAPIBinaryObject, now int64) dto.MCPOpenAPIBinaryObjectListItem {
	return dto.MCPOpenAPIBinaryObjectListItem{
		Id:               object.Id,
		ObjectId:         object.ObjectId,
		Provider:         object.Provider,
		ContentType:      object.ContentType,
		ContentFamily:    object.ContentFamily,
		SHA256:           object.SHA256,
		Size:             object.Size,
		Filename:         object.Filename,
		Disposition:      object.Disposition,
		MCPToolCallId:    object.MCPToolCallId,
		MCPToolId:        object.MCPToolId,
		OpenAPIToolId:    object.OpenAPIToolId,
		UserId:           object.UserId,
		TokenId:          object.TokenId,
		RequestId:        object.RequestId,
		OperationKey:     object.OperationKey,
		ExpiresAt:        object.ExpiresAt,
		ExpiryStatus:     openAPIBinaryObjectExpiryStatus(object, now),
		DownloadCount:    object.DownloadCount,
		LastDownloadedAt: object.LastDownloadedAt,
		DownloadURL:      mcpopenapi.BinaryObjectDownloadURL(object.ObjectId),
		CreatedAt:        object.CreatedAt,
		UpdatedAt:        object.UpdatedAt,
	}
}

func openAPIBinaryObjectExpiryStatus(object model.MCPOpenAPIBinaryObject, now int64) string {
	if object.ExpiresAt <= 0 {
		return MCPOpenAPIBinaryExpiryNoExpiry
	}
	if object.ExpiresAt < now {
		return MCPOpenAPIBinaryExpiryExpired
	}
	return MCPOpenAPIBinaryExpiryActive
}
