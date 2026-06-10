package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	mcpopenapi "github.com/QuantumNous/new-api/pkg/mcp/openapi"
	"github.com/QuantumNous/new-api/pkg/mcp/secretref"

	"gorm.io/gorm"
)

const maxOpenAPIDocumentSize = 2 * 1024 * 1024

var defaultOpenAPIHTTPClient = &http.Client{Timeout: 30 * time.Second}

func PreviewMCPOpenAPIForAdmin(ctx context.Context, req dto.MCPOpenAPIPreviewRequest) (*dto.MCPOpenAPIPreviewResponse, error) {
	spec, namespace, category, err := loadMCPOpenAPISpec(ctx, req.OpenAPIUrl, req.Document, req.Namespace, req.Category)
	if err != nil {
		return nil, err
	}
	return mcpOpenAPISpecToPreview(spec, namespace, category), nil
}

func ImportMCPOpenAPIForAdmin(ctx context.Context, req dto.MCPOpenAPIImportRequest) (*dto.MCPOpenAPIImportResponse, error) {
	return processMCPOpenAPIImportForAdmin(ctx, req, true)
}

func DiffMCPOpenAPIForAdmin(ctx context.Context, req dto.MCPOpenAPIImportRequest) (*dto.MCPOpenAPIImportResponse, error) {
	return processMCPOpenAPIImportForAdmin(ctx, req, false)
}

func processMCPOpenAPIImportForAdmin(ctx context.Context, req dto.MCPOpenAPIImportRequest, apply bool) (*dto.MCPOpenAPIImportResponse, error) {
	spec, namespace, category, err := loadMCPOpenAPISpec(ctx, req.OpenAPIUrl, req.Document, req.Namespace, req.Category)
	if err != nil {
		return nil, err
	}
	pricePerCall := 0.0
	if req.PricePerCall != nil {
		if *req.PricePerCall < 0 {
			return nil, errors.New("price_per_call must be greater than or equal to 0")
		}
		pricePerCall = *req.PricePerCall
	}
	priceUnit := strings.TrimSpace(req.PriceUnit)
	if priceUnit == "" {
		priceUnit = model.MCPToolPriceUnitPerCall
	}
	if err := validateMCPToolPriceUnit(priceUnit); err != nil {
		return nil, err
	}
	freeQuota := 0
	if req.FreeQuota != nil {
		if *req.FreeQuota < 0 {
			return nil, errors.New("free_quota must be greater than or equal to 0")
		}
		freeQuota = *req.FreeQuota
	}
	status := model.MCPToolStatusDisabled
	if req.Status != nil {
		if err := validateMCPToolStatus(*req.Status); err != nil {
			return nil, err
		}
		status = *req.Status
	}
	sortOrder := 0
	if req.SortOrder != nil {
		sortOrder = *req.SortOrder
	}
	authType, authRef, authHeaderName, err := normalizeMCPOpenAPIAuth(req.AuthType, req.AuthRef, req.AuthHeaderName)
	if err != nil {
		return nil, err
	}

	selected := selectedOpenAPIOperations(req.SelectedOperations)
	response := &dto.MCPOpenAPIImportResponse{
		OpenAPIUrl: spec.OpenAPIUrl,
		Imported:   []dto.MCPOpenAPIImportItem{},
		Updated:    []dto.MCPOpenAPIImportItem{},
		Skipped:    []string{},
	}
	for _, operation := range spec.Operations {
		if len(selected) > 0 && !selected[operation.Key] {
			continue
		}
		toolName := mcpopenapi.ToolName(namespace, operation)
		if err := validateMCPToolName(toolName); err != nil {
			response.Skipped = append(response.Skipped, fmt.Sprintf("%s: %v", operation.Key, err))
			continue
		}
		inputSchemaBytes, err := common.Marshal(operation.InputSchema)
		if err != nil {
			return nil, err
		}
		parametersBytes, err := common.Marshal(operation.Parameters)
		if err != nil {
			return nil, err
		}
		requestBodySchemaBytes, err := common.Marshal(operation.RequestBodySchema)
		if err != nil {
			return nil, err
		}
		tool := &model.MCPTool{
			Name:         toolName,
			DisplayName:  openAPIToolDisplayName(operation),
			Description:  openAPIToolDescription(operation),
			Category:     category,
			Source:       model.MCPToolSourceOpenAPI,
			OpenAPIUrl:   spec.OpenAPIUrl,
			InputSchema:  string(inputSchemaBytes),
			PricePerCall: pricePerCall,
			PriceUnit:    priceUnit,
			FreeQuota:    freeQuota,
			IsRemote:     false,
			Status:       status,
			SortOrder:    sortOrder,
		}
		openapiTool := &model.MCPOpenAPITool{
			OpenAPIUrl:          spec.OpenAPIUrl,
			ServerURL:           operation.ServerURL,
			OperationId:         operation.OperationId,
			OperationKey:        operation.Key,
			Method:              operation.Method,
			Path:                operation.Path,
			RequestContentType:  operation.RequestContentType,
			ResponseContentType: operation.ResponseContentType,
			AuthType:            authType,
			AuthRef:             authRef,
			AuthHeaderName:      authHeaderName,
			Parameters:          string(parametersBytes),
			RequestBodySchema:   string(requestBodySchemaBytes),
		}

		existing, err := model.GetMCPToolByName(toolName)
		if err == nil {
			if !req.UpdateExisting {
				response.Skipped = append(response.Skipped, fmt.Sprintf("%s: tool name %s already exists", operation.Key, toolName))
				continue
			}
			if existing.Source != model.MCPToolSourceOpenAPI {
				response.Skipped = append(response.Skipped, fmt.Sprintf("%s: tool name %s belongs to %s", operation.Key, toolName, existing.Source))
				continue
			}
			existingMapping, mappingErr := model.GetMCPOpenAPIToolByMCPToolId(existing.Id)
			if mappingErr != nil && !errors.Is(mappingErr, gorm.ErrRecordNotFound) {
				return nil, mappingErr
			}
			toolUpdates := map[string]any{
				"display_name": openAPIToolDisplayName(operation),
				"description":  openAPIToolDescription(operation),
				"source":       model.MCPToolSourceOpenAPI,
				"openapi_url":  spec.OpenAPIUrl,
				"input_schema": string(inputSchemaBytes),
				"is_remote":    false,
				"updated_at":   common.GetTimestamp(),
			}
			changes := openAPIImportChanges(*existing, existingMapping, toolUpdates, openapiTool)
			updatedTool := previewUpdatedOpenAPITool(*existing, toolUpdates)
			if apply {
				updatedTool, err = model.UpdateMCPToolWithOpenAPI(existing.Id, toolUpdates, openapiTool)
				if err != nil {
					return nil, err
				}
			}
			response.Updated = append(response.Updated, dto.MCPOpenAPIImportItem{
				OperationKey: operation.Key,
				Tool:         mcpModelToAdminItem(*updatedTool),
				Changes:      changes,
			})
			continue
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		exists, err := model.MCPToolNameExistsUnscoped(toolName)
		if err != nil {
			return nil, err
		}
		if exists {
			response.Skipped = append(response.Skipped, fmt.Sprintf("%s: tool name %s already exists", operation.Key, toolName))
			continue
		}
		if apply {
			if err := model.CreateMCPToolWithOpenAPI(tool, openapiTool); err != nil {
				return nil, err
			}
		}
		response.Imported = append(response.Imported, dto.MCPOpenAPIImportItem{
			OperationKey: operation.Key,
			Tool:         mcpModelToAdminItem(*tool),
		})
	}
	response.ImportedCount = len(response.Imported)
	response.UpdatedCount = len(response.Updated)
	response.SkippedCount = len(response.Skipped)
	return response, nil
}

func DisableMCPOpenAPIForAdmin(req dto.MCPOpenAPILifecycleRequest) (*dto.MCPOpenAPILifecycleResponse, error) {
	tools, openAPIURL, err := requireMCPOpenAPILifecycleTools(req)
	if err != nil {
		return nil, err
	}
	ids := mcpToolIds(tools)
	disabled, err := model.DisableMCPOpenAPITools(ids)
	if err != nil {
		return nil, err
	}
	return mcpOpenAPILifecycleResponse(openAPIURL, disabled), nil
}

func DeleteMCPOpenAPIForAdmin(req dto.MCPOpenAPILifecycleRequest) (*dto.MCPOpenAPILifecycleResponse, error) {
	tools, openAPIURL, err := requireMCPOpenAPILifecycleTools(req)
	if err != nil {
		return nil, err
	}
	if err := model.DeleteMCPOpenAPITools(mcpToolIds(tools)); err != nil {
		return nil, err
	}
	return mcpOpenAPILifecycleResponse(openAPIURL, tools), nil
}

func requireMCPOpenAPILifecycleTools(req dto.MCPOpenAPILifecycleRequest) ([]model.MCPTool, string, error) {
	openAPIURL := strings.TrimSpace(req.OpenAPIUrl)
	if openAPIURL == "" && len(req.ToolIds) == 0 {
		return nil, "", errors.New("openapi_url or tool_ids is required")
	}
	tools, err := model.ListMCPToolsForOpenAPILifecycle(openAPIURL, req.ToolIds)
	if err != nil {
		return nil, "", err
	}
	if len(tools) == 0 {
		return nil, openAPIURL, errors.New("no OpenAPI MCP tools matched")
	}
	if openAPIURL == "" {
		openAPIURL = tools[0].OpenAPIUrl
	}
	return tools, openAPIURL, nil
}

func mcpToolIds(tools []model.MCPTool) []int {
	ids := make([]int, 0, len(tools))
	for _, tool := range tools {
		ids = append(ids, tool.Id)
	}
	return ids
}

func mcpOpenAPILifecycleResponse(openAPIURL string, tools []model.MCPTool) *dto.MCPOpenAPILifecycleResponse {
	items := make([]dto.MCPToolAdminItem, 0, len(tools))
	for _, tool := range tools {
		items = append(items, mcpModelToAdminItem(tool))
	}
	return &dto.MCPOpenAPILifecycleResponse{
		OpenAPIUrl:    openAPIURL,
		AffectedCount: len(items),
		Tools:         items,
	}
}

func previewUpdatedOpenAPITool(existing model.MCPTool, toolUpdates map[string]any) *model.MCPTool {
	updated := existing
	if value, ok := toolUpdates["display_name"].(string); ok {
		updated.DisplayName = value
	}
	if value, ok := toolUpdates["description"].(string); ok {
		updated.Description = value
	}
	if value, ok := toolUpdates["source"].(string); ok {
		updated.Source = value
	}
	if value, ok := toolUpdates["openapi_url"].(string); ok {
		updated.OpenAPIUrl = value
	}
	if value, ok := toolUpdates["input_schema"].(string); ok {
		updated.InputSchema = value
	}
	if value, ok := toolUpdates["is_remote"].(bool); ok {
		updated.IsRemote = value
	}
	if value, ok := toolUpdates["updated_at"].(int64); ok {
		updated.UpdatedAt = value
	}
	return &updated
}

func openAPIImportChanges(existing model.MCPTool, existingMapping *model.MCPOpenAPITool, toolUpdates map[string]any, next *model.MCPOpenAPITool) []dto.MCPOpenAPIImportChange {
	changes := []dto.MCPOpenAPIImportChange{}
	changes = appendOpenAPIImportChange(changes, "display_name", existing.DisplayName, openAPIImportString(toolUpdates["display_name"]))
	changes = appendOpenAPIImportChange(changes, "description", existing.Description, openAPIImportString(toolUpdates["description"]))
	changes = appendOpenAPIImportChange(changes, "openapi_url", existing.OpenAPIUrl, openAPIImportString(toolUpdates["openapi_url"]))
	changes = appendOpenAPIImportHashChange(changes, "input_schema", existing.InputSchema, openAPIImportString(toolUpdates["input_schema"]))

	if existingMapping == nil {
		return appendOpenAPIImportChange(changes, "mapping", "", "created")
	}
	changes = appendOpenAPIImportChange(changes, "server_url", existingMapping.ServerURL, next.ServerURL)
	changes = appendOpenAPIImportChange(changes, "operation_id", existingMapping.OperationId, next.OperationId)
	changes = appendOpenAPIImportChange(changes, "operation_key", existingMapping.OperationKey, next.OperationKey)
	changes = appendOpenAPIImportChange(changes, "method", existingMapping.Method, next.Method)
	changes = appendOpenAPIImportChange(changes, "path", existingMapping.Path, next.Path)
	changes = appendOpenAPIImportChange(changes, "request_content_type", existingMapping.RequestContentType, next.RequestContentType)
	changes = appendOpenAPIImportChange(changes, "response_content_type", existingMapping.ResponseContentType, next.ResponseContentType)
	changes = appendOpenAPIImportChange(changes, "auth_type", existingMapping.AuthType, next.AuthType)
	changes = appendOpenAPIImportChange(changes, "auth_header_name", existingMapping.AuthHeaderName, next.AuthHeaderName)
	changes = appendOpenAPIImportHashChange(changes, "parameters", existingMapping.Parameters, next.Parameters)
	changes = appendOpenAPIImportHashChange(changes, "request_body_schema", existingMapping.RequestBodySchema, next.RequestBodySchema)
	return changes
}

func appendOpenAPIImportChange(changes []dto.MCPOpenAPIImportChange, field string, previous string, current string) []dto.MCPOpenAPIImportChange {
	previous = strings.TrimSpace(previous)
	current = strings.TrimSpace(current)
	if previous == current {
		return changes
	}
	return append(changes, dto.MCPOpenAPIImportChange{
		Field:    field,
		Previous: truncateOpenAPIImportChangeValue(previous),
		Current:  truncateOpenAPIImportChangeValue(current),
	})
}

func appendOpenAPIImportHashChange(changes []dto.MCPOpenAPIImportChange, field string, previous string, current string) []dto.MCPOpenAPIImportChange {
	previous = strings.TrimSpace(previous)
	current = strings.TrimSpace(current)
	if previous == current {
		return changes
	}
	return append(changes, dto.MCPOpenAPIImportChange{
		Field:    field,
		Previous: openAPIImportHashValue(previous),
		Current:  openAPIImportHashValue(current),
	})
}

func openAPIImportString(value any) string {
	text, _ := value.(string)
	return text
}

func truncateOpenAPIImportChangeValue(value string) string {
	if len(value) <= 160 {
		return value
	}
	return value[:157] + "..."
}

func openAPIImportHashValue(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(value))
	return "sha256:" + hex.EncodeToString(sum[:8])
}

func normalizeMCPOpenAPIAuth(authType string, authRef string, authHeaderName string) (string, string, string, error) {
	authType = strings.ToLower(strings.TrimSpace(authType))
	if authType == "" {
		authType = model.MCPProxyAuthTypeNone
	}
	authRef = strings.TrimSpace(authRef)
	authHeaderName = strings.TrimSpace(authHeaderName)

	switch authType {
	case model.MCPProxyAuthTypeNone:
		return model.MCPProxyAuthTypeNone, "", "", nil
	case model.MCPProxyAuthTypeBearer, model.MCPProxyAuthTypeBasic, model.MCPProxyAuthTypeHeader:
	default:
		return "", "", "", fmt.Errorf("unsupported openapi auth_type: %s", authType)
	}
	if authRef == "" {
		return "", "", "", errors.New("auth_ref is required when auth_type is not none")
	}
	normalizedAuthRef, err := secretref.Normalize(authRef)
	if err != nil {
		return "", "", "", err
	}
	if authType == model.MCPProxyAuthTypeHeader && authHeaderName == "" {
		return "", "", "", errors.New("auth_header_name is required when auth_type is header")
	}
	if authType != model.MCPProxyAuthTypeHeader {
		authHeaderName = ""
	}
	return authType, normalizedAuthRef, authHeaderName, nil
}

func loadMCPOpenAPISpec(ctx context.Context, openAPIURL string, document any, namespace string, category string) (*mcpopenapi.Spec, string, string, error) {
	openAPIURL = strings.TrimSpace(openAPIURL)
	bytes, err := openAPIDocumentBytes(ctx, openAPIURL, document)
	if err != nil {
		return nil, "", "", err
	}
	spec, err := mcpopenapi.ParseSpec(bytes, openAPIURL)
	if err != nil {
		return nil, "", "", err
	}
	namespace = strings.TrimSpace(namespace)
	if namespace == "" {
		namespace = spec.Title
	}
	if namespace == "" {
		namespace = "openapi"
	}
	category = strings.TrimSpace(category)
	if category == "" {
		category = "openapi"
	}
	return spec, namespace, category, nil
}

func openAPIDocumentBytes(ctx context.Context, openAPIURL string, document any) ([]byte, error) {
	if document != nil {
		if text, ok := document.(string); ok {
			text = strings.TrimSpace(text)
			if text == "" {
				return nil, errors.New("document is empty")
			}
			if len(text) > maxOpenAPIDocumentSize {
				return nil, errors.New("document exceeds max size")
			}
			return []byte(text), nil
		}
		bytes, err := common.Marshal(document)
		if err != nil {
			return nil, err
		}
		if len(bytes) > maxOpenAPIDocumentSize {
			return nil, errors.New("document exceeds max size")
		}
		return bytes, nil
	}
	if openAPIURL == "" {
		return nil, errors.New("openapi_url or document is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, openAPIURL, nil)
	if err != nil {
		return nil, err
	}
	response, err := defaultOpenAPIHTTPClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("fetch openapi document failed with status %d", response.StatusCode)
	}
	reader := io.LimitReader(response.Body, maxOpenAPIDocumentSize+1)
	bytes, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	if len(bytes) > maxOpenAPIDocumentSize {
		return nil, errors.New("openapi document exceeds max size")
	}
	return bytes, nil
}

func mcpOpenAPISpecToPreview(spec *mcpopenapi.Spec, namespace string, category string) *dto.MCPOpenAPIPreviewResponse {
	response := &dto.MCPOpenAPIPreviewResponse{
		OpenAPIUrl: spec.OpenAPIUrl,
		Title:      spec.Title,
		Version:    spec.Version,
		ServerURL:  spec.ServerURL,
		Namespace:  namespace,
		Category:   category,
		Operations: make([]dto.MCPOpenAPIPreviewOperation, 0, len(spec.Operations)),
	}
	for _, operation := range spec.Operations {
		response.Operations = append(response.Operations, dto.MCPOpenAPIPreviewOperation{
			Key:            operation.Key,
			OperationId:    operation.OperationId,
			Method:         operation.Method,
			Path:           operation.Path,
			Summary:        operation.Summary,
			Description:    operation.Description,
			ToolName:       mcpopenapi.ToolName(namespace, operation),
			InputSchema:    operation.InputSchema,
			HasRequestBody: len(operation.RequestBodySchema) > 0,
		})
	}
	return response
}

func selectedOpenAPIOperations(operations []string) map[string]bool {
	if len(operations) == 0 {
		return nil
	}
	selected := map[string]bool{}
	for _, operation := range operations {
		operation = strings.TrimSpace(operation)
		if operation == "" {
			continue
		}
		selected[operation] = true
	}
	return selected
}

func openAPIToolDisplayName(operation mcpopenapi.Operation) string {
	if strings.TrimSpace(operation.Summary) != "" {
		return strings.TrimSpace(operation.Summary)
	}
	if strings.TrimSpace(operation.OperationId) != "" {
		return strings.TrimSpace(operation.OperationId)
	}
	return operation.Method + " " + operation.Path
}

func openAPIToolDescription(operation mcpopenapi.Operation) string {
	if strings.TrimSpace(operation.Description) != "" {
		return strings.TrimSpace(operation.Description)
	}
	return strings.TrimSpace(operation.Summary)
}
