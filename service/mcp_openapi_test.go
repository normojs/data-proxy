package service

import (
	"context"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	mcpopenapi "github.com/QuantumNous/new-api/pkg/mcp/openapi"
	"github.com/stretchr/testify/require"
)

const openAPITestDocument = `{
  "openapi": "3.0.3",
  "info": {
    "title": "Pet API",
    "version": "1.0.0"
  },
  "servers": [
    { "url": "https://api.example.test" }
  ],
  "paths": {
    "/pets/{petId}": {
      "get": {
        "operationId": "getPet",
        "summary": "Get Pet",
        "parameters": [
          {
            "name": "petId",
            "in": "path",
            "required": true,
            "schema": { "type": "string" }
          },
          {
            "name": "includeOwner",
            "in": "query",
            "schema": { "type": "boolean" }
          }
        ],
        "responses": {
          "200": {
            "description": "OK",
            "content": {
              "application/json": {
                "schema": { "type": "object" }
              }
            }
          }
        }
      }
    },
    "/pets": {
      "post": {
        "operationId": "createPet",
        "summary": "Create Pet",
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "type": "object",
                "properties": {
                  "name": { "type": "string" }
                }
              }
            }
          }
        },
        "responses": {
          "201": {
            "description": "Created",
            "content": {
              "application/json": {
                "schema": { "type": "object" }
              }
            }
          }
        }
      }
    }
  }
}`

func TestPreviewMCPOpenAPIForAdmin(t *testing.T) {
	preview, err := PreviewMCPOpenAPIForAdmin(context.Background(), dto.MCPOpenAPIPreviewRequest{
		OpenAPIUrl: "https://docs.example.test/openapi.json",
		Document:   openAPITestDocument,
		Namespace:  "pet_api",
	})
	require.NoError(t, err)
	require.Equal(t, "Pet API", preview.Title)
	require.Equal(t, "https://api.example.test", preview.ServerURL)
	require.Len(t, preview.Operations, 2)
	byKey := map[string]dto.MCPOpenAPIPreviewOperation{}
	for _, operation := range preview.Operations {
		byKey[operation.Key] = operation
	}
	require.Equal(t, "pet_api.getpet", byKey["GET /pets/{petId}"].ToolName)
	properties, ok := byKey["GET /pets/{petId}"].InputSchema["properties"].(map[string]any)
	require.True(t, ok)
	require.Contains(t, properties, "petId")
	require.True(t, byKey["POST /pets"].HasRequestBody)
}

func TestImportMCPOpenAPIForAdmin(t *testing.T) {
	setupMCPProxyServiceTestDB(t)

	status := model.MCPToolStatusEnabled
	price := 0.001
	imported, err := ImportMCPOpenAPIForAdmin(context.Background(), dto.MCPOpenAPIImportRequest{
		OpenAPIUrl:         "https://docs.example.test/openapi.json",
		Document:           openAPITestDocument,
		Namespace:          "pet_api",
		Category:           "pets",
		SelectedOperations: []string{"GET /pets/{petId}"},
		AuthType:           model.MCPProxyAuthTypeBearer,
		AuthRef:            "  ENV: PET_API_TOKEN  ",
		PricePerCall:       &price,
		Status:             &status,
	})
	require.NoError(t, err)
	require.Equal(t, 1, imported.ImportedCount)
	require.Equal(t, 0, imported.SkippedCount)
	require.Equal(t, "pet_api.getpet", imported.Imported[0].Tool.Name)

	var tool model.MCPTool
	require.NoError(t, model.DB.Where("name = ?", "pet_api.getpet").First(&tool).Error)
	require.Equal(t, model.MCPToolSourceOpenAPI, tool.Source)
	require.Equal(t, "pets", tool.Category)
	require.Equal(t, model.MCPToolStatusEnabled, tool.Status)

	var mapping model.MCPOpenAPITool
	require.NoError(t, model.DB.Where("mcp_tool_id = ?", tool.Id).First(&mapping).Error)
	require.Equal(t, "GET /pets/{petId}", mapping.OperationKey)
	require.Equal(t, "GET", mapping.Method)
	require.Equal(t, "/pets/{petId}", mapping.Path)
	require.Equal(t, model.MCPProxyAuthTypeBearer, mapping.AuthType)
	require.Equal(t, "env:PET_API_TOKEN", mapping.AuthRef)
	require.Contains(t, mapping.Parameters, "petId")

	duplicate, err := ImportMCPOpenAPIForAdmin(context.Background(), dto.MCPOpenAPIImportRequest{
		OpenAPIUrl:         "https://docs.example.test/openapi.json",
		Document:           openAPITestDocument,
		Namespace:          "pet_api",
		SelectedOperations: []string{"GET /pets/{petId}"},
	})
	require.NoError(t, err)
	require.Equal(t, 0, duplicate.ImportedCount)
	require.Equal(t, 1, duplicate.SkippedCount)

	updatedDocument := strings.Replace(
		openAPITestDocument,
		`"summary": "Get Pet"`,
		`"summary": "Fetch Pet"`,
		1,
	)
	updatedDocument = strings.Replace(
		updatedDocument,
		`"name": "includeOwner"`,
		`"name": "expand"`,
		1,
	)
	updated, err := ImportMCPOpenAPIForAdmin(context.Background(), dto.MCPOpenAPIImportRequest{
		OpenAPIUrl:         "https://docs.example.test/openapi-v2.json",
		Document:           updatedDocument,
		Namespace:          "pet_api",
		SelectedOperations: []string{"GET /pets/{petId}"},
		UpdateExisting:     true,
		AuthType:           model.MCPProxyAuthTypeHeader,
		AuthRef:            "env:PET_API_KEY",
		AuthHeaderName:     "X-API-Key",
	})
	require.NoError(t, err)
	require.Equal(t, 0, updated.ImportedCount)
	require.Equal(t, 1, updated.UpdatedCount)
	require.Equal(t, tool.Id, updated.Updated[0].Tool.Id)
	require.Equal(t, "Fetch Pet", updated.Updated[0].Tool.DisplayName)
	require.Equal(t, "pets", updated.Updated[0].Tool.Category)
	require.Equal(t, model.MCPToolStatusEnabled, updated.Updated[0].Tool.Status)
	requireOpenAPIImportChange(t, updated.Updated[0].Changes, "display_name", "Get Pet", "Fetch Pet")
	requireOpenAPIImportChange(t, updated.Updated[0].Changes, "openapi_url", "https://docs.example.test/openapi.json", "https://docs.example.test/openapi-v2.json")
	requireOpenAPIImportHashChange(t, updated.Updated[0].Changes, "input_schema")
	requireOpenAPIImportHashChange(t, updated.Updated[0].Changes, "parameters")
	requireOpenAPIImportChange(t, updated.Updated[0].Changes, "auth_type", model.MCPProxyAuthTypeBearer, model.MCPProxyAuthTypeHeader)
	requireOpenAPIImportChange(t, updated.Updated[0].Changes, "auth_header_name", "", "X-API-Key")

	var refreshedTool model.MCPTool
	require.NoError(t, model.DB.Where("name = ?", "pet_api.getpet").First(&refreshedTool).Error)
	require.Equal(t, tool.Id, refreshedTool.Id)
	require.Equal(t, "Fetch Pet", refreshedTool.DisplayName)
	require.Equal(t, "pets", refreshedTool.Category)
	require.Equal(t, model.MCPToolStatusEnabled, refreshedTool.Status)
	require.Contains(t, refreshedTool.InputSchema, "expand")

	var refreshedMapping model.MCPOpenAPITool
	require.NoError(t, model.DB.Where("mcp_tool_id = ?", tool.Id).First(&refreshedMapping).Error)
	require.Equal(t, "https://docs.example.test/openapi-v2.json", refreshedMapping.OpenAPIUrl)
	require.Equal(t, model.MCPProxyAuthTypeHeader, refreshedMapping.AuthType)
	require.Equal(t, "env:PET_API_KEY", refreshedMapping.AuthRef)
	require.Equal(t, "X-API-Key", refreshedMapping.AuthHeaderName)
}

func TestDiffMCPOpenAPIForAdminDoesNotPersist(t *testing.T) {
	setupMCPProxyServiceTestDB(t)

	status := model.MCPToolStatusEnabled
	_, err := ImportMCPOpenAPIForAdmin(context.Background(), dto.MCPOpenAPIImportRequest{
		OpenAPIUrl:         "https://docs.example.test/openapi.json",
		Document:           openAPITestDocument,
		Namespace:          "pet_api",
		SelectedOperations: []string{"GET /pets/{petId}"},
		Status:             &status,
	})
	require.NoError(t, err)

	updatedDocument := strings.Replace(
		openAPITestDocument,
		`"summary": "Get Pet"`,
		`"summary": "Fetch Pet"`,
		1,
	)
	diff, err := DiffMCPOpenAPIForAdmin(context.Background(), dto.MCPOpenAPIImportRequest{
		OpenAPIUrl:         "https://docs.example.test/openapi-v2.json",
		Document:           updatedDocument,
		Namespace:          "pet_api",
		SelectedOperations: []string{"GET /pets/{petId}", "POST /pets"},
		UpdateExisting:     true,
	})
	require.NoError(t, err)
	require.Equal(t, 1, diff.ImportedCount)
	require.Equal(t, 1, diff.UpdatedCount)
	requireOpenAPIImportChange(t, diff.Updated[0].Changes, "display_name", "Get Pet", "Fetch Pet")
	require.Equal(t, 0, diff.Imported[0].Tool.Id)

	var stored model.MCPTool
	require.NoError(t, model.DB.Where("name = ?", "pet_api.getpet").First(&stored).Error)
	require.Equal(t, "Get Pet", stored.DisplayName)
	require.Equal(t, "https://docs.example.test/openapi.json", stored.OpenAPIUrl)

	var createdCount int64
	require.NoError(t, model.DB.Model(&model.MCPTool{}).Where("name = ?", "pet_api.createpet").Count(&createdCount).Error)
	require.Equal(t, int64(0), createdCount)
}

func TestMCPOpenAPILifecycleForAdmin(t *testing.T) {
	setupMCPProxyServiceTestDB(t)

	status := model.MCPToolStatusEnabled
	imported, err := ImportMCPOpenAPIForAdmin(context.Background(), dto.MCPOpenAPIImportRequest{
		OpenAPIUrl: "https://docs.example.test/openapi.json",
		Document:   openAPITestDocument,
		Namespace:  "pet_api",
		Status:     &status,
	})
	require.NoError(t, err)
	require.Equal(t, 2, imported.ImportedCount)

	disabled, err := DisableMCPOpenAPIForAdmin(dto.MCPOpenAPILifecycleRequest{
		OpenAPIUrl: "https://docs.example.test/openapi.json",
	})
	require.NoError(t, err)
	require.Equal(t, 2, disabled.AffectedCount)
	for _, tool := range disabled.Tools {
		require.Equal(t, model.MCPToolStatusDisabled, tool.Status)
	}

	deleted, err := DeleteMCPOpenAPIForAdmin(dto.MCPOpenAPILifecycleRequest{
		OpenAPIUrl: "https://docs.example.test/openapi.json",
	})
	require.NoError(t, err)
	require.Equal(t, 2, deleted.AffectedCount)

	var activeTools int64
	require.NoError(t, model.DB.Model(&model.MCPTool{}).
		Where("source = ? AND openapi_url = ?", model.MCPToolSourceOpenAPI, "https://docs.example.test/openapi.json").
		Count(&activeTools).Error)
	require.Equal(t, int64(0), activeTools)

	var activeMappings int64
	require.NoError(t, model.DB.Model(&model.MCPOpenAPITool{}).
		Where("openapi_url = ?", "https://docs.example.test/openapi.json").
		Count(&activeMappings).Error)
	require.Equal(t, int64(0), activeMappings)
}

func TestImportMCPOpenAPIRejectsInvalidAuthRef(t *testing.T) {
	setupMCPProxyServiceTestDB(t)

	for _, authRef := range []string{"raw:secret-token", "env:", "env:PET-API-TOKEN"} {
		t.Run(authRef, func(t *testing.T) {
			_, err := ImportMCPOpenAPIForAdmin(context.Background(), dto.MCPOpenAPIImportRequest{
				OpenAPIUrl:         "https://docs.example.test/openapi.json",
				Document:           openAPITestDocument,
				Namespace:          "pet_api_raw_auth",
				SelectedOperations: []string{"GET /pets/{petId}"},
				AuthType:           model.MCPProxyAuthTypeBearer,
				AuthRef:            authRef,
			})
			require.Error(t, err)
			require.Contains(t, err.Error(), "env:NAME")
		})
	}
}

func TestDownloadMCPOpenAPIBinaryObjectEnforcesOwner(t *testing.T) {
	setupMCPProxyServiceTestDB(t)
	t.Setenv("OPENAPI_BINARY_OBJECT_PROVIDER", "local")
	t.Setenv("OPENAPI_BINARY_OBJECT_DIR", t.TempDir())

	object, err := mcpopenapi.SaveBinaryObject([]byte("owner-only"), "application/octet-stream", "owner-only")
	require.NoError(t, err)
	require.NoError(t, model.CreateMCPOpenAPIBinaryObject(&model.MCPOpenAPIBinaryObject{
		ObjectId:      object.Id,
		Provider:      object.Provider,
		StorageKey:    object.StorageKey,
		ContentType:   object.ContentType,
		ContentFamily: "binary",
		SHA256:        object.SHA256,
		Size:          object.Size,
		Filename:      object.Filename,
		Disposition:   "attachment",
		MCPToolCallId: 11,
		MCPToolId:     22,
		OpenAPIToolId: 33,
		UserId:        101,
		TokenId:       201,
		RequestId:     "openapi-binary-owner",
		OperationKey:  "GET /download",
	}))

	_, err = DownloadMCPOpenAPIBinaryObject(context.Background(), MCPOpenAPIBinaryDownloadParams{
		ObjectId: object.Id,
		UserId:   102,
	})
	require.ErrorIs(t, err, ErrMCPOpenAPIBinaryObjectNotFound)

	download, err := DownloadMCPOpenAPIBinaryObject(context.Background(), MCPOpenAPIBinaryDownloadParams{
		ObjectId: object.Id,
		UserId:   101,
	})
	require.NoError(t, err)
	require.Equal(t, []byte("owner-only"), download.Content)
	require.Equal(t, object.ContentType, download.Object.ContentType)

	adminDownload, err := DownloadMCPOpenAPIBinaryObject(context.Background(), MCPOpenAPIBinaryDownloadParams{
		ObjectId: object.Id,
		UserId:   999,
		IsAdmin:  true,
	})
	require.NoError(t, err)
	require.Equal(t, []byte("owner-only"), adminDownload.Content)

	var refreshed model.MCPOpenAPIBinaryObject
	require.NoError(t, model.DB.Where("object_id = ?", object.Id).First(&refreshed).Error)
	require.Equal(t, 2, refreshed.DownloadCount)
	require.NotZero(t, refreshed.LastDownloadedAt)

	expiredObject, err := mcpopenapi.SaveBinaryObject([]byte("expired"), "application/octet-stream", "expired")
	require.NoError(t, err)
	require.NoError(t, model.CreateMCPOpenAPIBinaryObject(&model.MCPOpenAPIBinaryObject{
		ObjectId:      expiredObject.Id,
		Provider:      expiredObject.Provider,
		StorageKey:    expiredObject.StorageKey,
		ContentType:   expiredObject.ContentType,
		ContentFamily: "binary",
		SHA256:        expiredObject.SHA256,
		Size:          expiredObject.Size,
		Filename:      expiredObject.Filename,
		Disposition:   "attachment",
		MCPToolCallId: 12,
		MCPToolId:     22,
		OpenAPIToolId: 33,
		UserId:        101,
		TokenId:       201,
		RequestId:     "openapi-binary-expired",
		OperationKey:  "GET /download",
		ExpiresAt:     common.GetTimestamp() - 60,
	}))
	_, err = DownloadMCPOpenAPIBinaryObject(context.Background(), MCPOpenAPIBinaryDownloadParams{
		ObjectId: expiredObject.Id,
		UserId:   101,
	})
	require.ErrorIs(t, err, ErrMCPOpenAPIBinaryObjectNotFound)
	_, err = DownloadMCPOpenAPIBinaryObject(context.Background(), MCPOpenAPIBinaryDownloadParams{
		ObjectId: expiredObject.Id,
		UserId:   999,
		IsAdmin:  true,
	})
	require.ErrorIs(t, err, ErrMCPOpenAPIBinaryObjectNotFound)
}

func requireOpenAPIImportChange(t *testing.T, changes []dto.MCPOpenAPIImportChange, field string, previous string, current string) {
	t.Helper()
	for _, change := range changes {
		if change.Field != field {
			continue
		}
		require.Equal(t, previous, change.Previous)
		require.Equal(t, current, change.Current)
		return
	}
	t.Fatalf("expected OpenAPI import change for %s in %#v", field, changes)
}

func requireOpenAPIImportHashChange(t *testing.T, changes []dto.MCPOpenAPIImportChange, field string) {
	t.Helper()
	for _, change := range changes {
		if change.Field != field {
			continue
		}
		require.NotEmpty(t, change.Previous)
		require.NotEmpty(t, change.Current)
		require.NotEqual(t, change.Previous, change.Current)
		require.True(t, strings.HasPrefix(change.Previous, "sha256:"), change.Previous)
		require.True(t, strings.HasPrefix(change.Current, "sha256:"), change.Current)
		return
	}
	t.Fatalf("expected OpenAPI import hash change for %s in %#v", field, changes)
}
