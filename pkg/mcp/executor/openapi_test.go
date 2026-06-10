package executor

import (
	"context"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/QuantumNous/new-api/model"
	mcpopenapi "github.com/QuantumNous/new-api/pkg/mcp/openapi"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestOpenAPIExecutorCallsHTTPUpstream(t *testing.T) {
	var seenPath string
	var seenQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.Path
		seenQuery = r.URL.Query().Get("includeOwner")
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"id":            "pet-1",
			"include_owner": seenQuery,
		}))
	}))
	defer server.Close()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.MCPTool{}, &model.MCPOpenAPITool{}, &model.MCPOpenAPIBinaryObject{}))
	originalDB := model.DB
	model.DB = db
	t.Cleanup(func() {
		model.DB = originalDB
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	tool := &model.MCPTool{
		Name:        "pet_api.getpet",
		DisplayName: "Get Pet",
		Category:    "pets",
		Source:      model.MCPToolSourceOpenAPI,
		InputSchema: `{"type":"object"}`,
		Status:      model.MCPToolStatusEnabled,
	}
	parameters, err := json.Marshal([]mcpopenapi.Parameter{
		{Name: "petId", In: "path", Required: true, Schema: map[string]any{"type": "string"}},
		{Name: "includeOwner", In: "query", Schema: map[string]any{"type": "boolean"}},
	})
	require.NoError(t, err)
	require.NoError(t, model.CreateMCPToolWithOpenAPI(tool, &model.MCPOpenAPITool{
		OpenAPIUrl:   server.URL + "/openapi.json",
		ServerURL:    server.URL,
		OperationId:  "getPet",
		OperationKey: "GET /pets/{petId}",
		Method:       "GET",
		Path:         "/pets/{petId}",
		Parameters:   string(parameters),
	}))

	executor := NewOpenAPIExecutor(server.Client())
	result, err := executor.Execute(context.Background(), Request{
		Tool: *tool,
		Arguments: map[string]any{
			"petId":        "pet-1",
			"includeOwner": true,
		},
	})
	require.NoError(t, err)
	require.Equal(t, "/pets/pet-1", seenPath)
	require.Equal(t, "true", seenQuery)
	require.Len(t, result.Content, 1)
	require.Contains(t, result.Content[0].Text, "pet-1")
	require.Equal(t, "openapi", result.Metadata["executor"])
	require.Equal(t, "getPet", result.Metadata["operation_id"])
	require.Equal(t, http.StatusOK, result.Metadata["status_code"])
}

func TestOpenAPIExecutorSendsJSONBody(t *testing.T) {
	var seenBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&seenBody))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.MCPTool{}, &model.MCPOpenAPITool{}, &model.MCPOpenAPIBinaryObject{}))
	originalDB := model.DB
	model.DB = db
	t.Cleanup(func() {
		model.DB = originalDB
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	tool := &model.MCPTool{
		Name:        "pet_api.createpet",
		DisplayName: "Create Pet",
		Category:    "pets",
		Source:      model.MCPToolSourceOpenAPI,
		InputSchema: `{"type":"object"}`,
		Status:      model.MCPToolStatusEnabled,
	}
	require.NoError(t, model.CreateMCPToolWithOpenAPI(tool, &model.MCPOpenAPITool{
		OpenAPIUrl:         server.URL + "/openapi.json",
		ServerURL:          server.URL,
		OperationId:        "createPet",
		OperationKey:       "POST /pets",
		Method:             "POST",
		Path:               "/pets",
		RequestContentType: "application/json",
	}))

	result, err := NewOpenAPIExecutor(server.Client()).Execute(context.Background(), Request{
		Tool:      *tool,
		Arguments: map[string]any{"body": map[string]any{"name": "Milo"}},
	})
	require.NoError(t, err)
	require.Equal(t, "Milo", seenBody["name"])
	require.Contains(t, result.Content[0].Text, "ok")
}

func TestOpenAPIExecutorSummarizesBinaryResponse(t *testing.T) {
	t.Setenv("OPENAPI_BINARY_OBJECT_DIR", t.TempDir())
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write([]byte("hello"))
	}))
	defer server.Close()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.MCPTool{}, &model.MCPOpenAPITool{}, &model.MCPOpenAPIBinaryObject{}))
	originalDB := model.DB
	model.DB = db
	t.Cleanup(func() {
		model.DB = originalDB
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	tool := &model.MCPTool{
		Name:        "file_api.download",
		DisplayName: "Download",
		Category:    "files",
		Source:      model.MCPToolSourceOpenAPI,
		InputSchema: `{"type":"object"}`,
		Status:      model.MCPToolStatusEnabled,
	}
	require.NoError(t, model.CreateMCPToolWithOpenAPI(tool, &model.MCPOpenAPITool{
		OpenAPIUrl:          server.URL + "/openapi.json",
		ServerURL:           server.URL,
		OperationId:         "downloadFile",
		OperationKey:        "GET /download",
		Method:              "GET",
		Path:                "/download",
		ResponseContentType: "application/octet-stream",
	}))

	result, err := NewOpenAPIExecutor(server.Client()).Execute(context.Background(), Request{
		CallId:    123,
		UserId:    456,
		TokenId:   789,
		RequestId: "openapi-binary-test",
		Tool:      *tool,
		Arguments: map[string]any{},
	})
	require.NoError(t, err)
	require.Len(t, result.Content, 1)
	require.Contains(t, result.Content[0].Text, "Binary OpenAPI response omitted")
	require.Contains(t, result.Content[0].Text, "bytes=5")
	require.Equal(t, true, result.Metadata["response_binary"])
	require.Equal(t, "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824", result.Metadata["response_body_sha256"])
	objectId, ok := result.Metadata["response_object_id"].(string)
	require.True(t, ok)
	require.NotEmpty(t, objectId)
	require.Equal(t, "/api/mcp/openapi/binary/"+objectId+"/download", result.Metadata["response_download_url"])

	object, content, err := mcpopenapi.LoadBinaryObject(objectId)
	require.NoError(t, err)
	require.Equal(t, objectId, object.Id)
	require.Equal(t, "application/octet-stream", object.ContentType)
	require.Equal(t, []byte("hello"), content)

	registryObject, err := model.GetMCPOpenAPIBinaryObjectByObjectId(objectId)
	require.NoError(t, err)
	require.Equal(t, int64(123), registryObject.MCPToolCallId)
	require.Equal(t, 456, registryObject.UserId)
	require.Equal(t, 789, registryObject.TokenId)
	require.Equal(t, "openapi-binary-test", registryObject.RequestId)
}

func TestBuildOpenAPIHTTPRequestsSendsURLEncodedFormBody(t *testing.T) {
	req, err := buildOpenAPIHTTPRequest(context.Background(), model.MCPOpenAPITool{
		ServerURL:           "https://api.example.test",
		OperationId:         "createToken",
		OperationKey:        "POST /oauth/token",
		Method:              "POST",
		Path:                "/oauth/token",
		RequestContentType:  "application/x-www-form-urlencoded",
		RequestBodySchema:   `{"type":"object"}`,
		ResponseContentType: "application/json",
	}, nil, map[string]any{
		"body": map[string]any{
			"grant_type": "client_credentials",
			"scope":      []any{"read", "write"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, "application/x-www-form-urlencoded", req.Header.Get("Content-Type"))

	body, err := io.ReadAll(req.Body)
	require.NoError(t, err)
	values, err := url.ParseQuery(string(body))
	require.NoError(t, err)
	require.Equal(t, "client_credentials", values.Get("grant_type"))
	require.ElementsMatch(t, []string{"read", "write"}, values["scope"])
}

func TestBuildOpenAPIHTTPRequestsSendsRawTextBody(t *testing.T) {
	req, err := buildOpenAPIHTTPRequest(context.Background(), model.MCPOpenAPITool{
		ServerURL:          "https://api.example.test",
		OperationId:        "createNote",
		OperationKey:       "POST /notes",
		Method:             "POST",
		Path:               "/notes",
		RequestContentType: "text/plain",
		RequestBodySchema:  `{"type":"string"}`,
	}, nil, map[string]any{
		"body": "plain text",
	})
	require.NoError(t, err)
	require.Equal(t, "text/plain", req.Header.Get("Content-Type"))

	body, err := io.ReadAll(req.Body)
	require.NoError(t, err)
	require.Equal(t, "plain text", string(body))
}

func TestBuildOpenAPIHTTPRequestsSendsMultipartTextFields(t *testing.T) {
	req, err := buildOpenAPIHTTPRequest(context.Background(), model.MCPOpenAPITool{
		ServerURL:          "https://api.example.test",
		OperationId:        "createTicket",
		OperationKey:       "POST /tickets",
		Method:             "POST",
		Path:               "/tickets",
		RequestContentType: "multipart/form-data",
		RequestBodySchema:  `{"type":"object","properties":{"title":{"type":"string"},"priority":{"type":"integer"}}}`,
	}, nil, map[string]any{
		"body": map[string]any{
			"title":    "Broken window",
			"priority": 2,
		},
	})
	require.NoError(t, err)
	contentType := req.Header.Get("Content-Type")
	mediaType, params, err := mime.ParseMediaType(contentType)
	require.NoError(t, err)
	require.Equal(t, "multipart/form-data", mediaType)
	require.NotEmpty(t, params["boundary"])

	reader := multipart.NewReader(req.Body, params["boundary"])
	fields := map[string]string{}
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		value, err := io.ReadAll(part)
		require.NoError(t, err)
		require.Empty(t, part.FileName())
		fields[part.FormName()] = string(value)
	}
	require.Equal(t, "Broken window", fields["title"])
	require.Equal(t, "2", fields["priority"])
}

func TestBuildOpenAPIHTTPRequestsSendsMultipartFileFields(t *testing.T) {
	schemaBytes, err := json.Marshal(map[string]any{
		"type": "object",
		"properties": map[string]any{
			"title": map[string]any{"type": "string"},
			"file":  map[string]any{"type": "string", "format": "binary"},
		},
	})
	require.NoError(t, err)

	req, err := buildOpenAPIHTTPRequest(context.Background(), model.MCPOpenAPITool{
		ServerURL:          "https://api.example.test",
		OperationId:        "uploadAsset",
		OperationKey:       "POST /uploads",
		Method:             "POST",
		Path:               "/uploads",
		RequestContentType: "multipart/form-data",
		RequestBodySchema:  string(schemaBytes),
	}, nil, map[string]any{
		"body": map[string]any{
			"title": "Avatar",
			"file": map[string]any{
				"filename":       "avatar.txt",
				"content_type":   "text/plain",
				"content_base64": "aGVsbG8=",
			},
		},
	})
	require.NoError(t, err)

	contentType := req.Header.Get("Content-Type")
	mediaType, params, err := mime.ParseMediaType(contentType)
	require.NoError(t, err)
	require.Equal(t, "multipart/form-data", mediaType)

	reader := multipart.NewReader(req.Body, params["boundary"])
	parts := map[string]struct {
		filename    string
		contentType string
		body        string
	}{}
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		value, err := io.ReadAll(part)
		require.NoError(t, err)
		parts[part.FormName()] = struct {
			filename    string
			contentType string
			body        string
		}{
			filename:    part.FileName(),
			contentType: part.Header.Get("Content-Type"),
			body:        string(value),
		}
	}
	require.Equal(t, "Avatar", parts["title"].body)
	require.Empty(t, parts["title"].filename)
	require.Equal(t, "avatar.txt", parts["file"].filename)
	require.Equal(t, "text/plain", parts["file"].contentType)
	require.Equal(t, "hello", parts["file"].body)
}

func TestBuildOpenAPIHTTPRequestsSendsBinaryBody(t *testing.T) {
	req, err := buildOpenAPIHTTPRequest(context.Background(), model.MCPOpenAPITool{
		ServerURL:          "https://api.example.test",
		OperationId:        "uploadRaw",
		OperationKey:       "POST /raw",
		Method:             "POST",
		Path:               "/raw",
		RequestContentType: "application/octet-stream",
		RequestBodySchema:  `{"type":"string","format":"binary"}`,
	}, nil, map[string]any{
		"body": map[string]any{
			"content_type":   "image/png",
			"content_base64": "aGVsbG8=",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "image/png", req.Header.Get("Content-Type"))

	body, err := io.ReadAll(req.Body)
	require.NoError(t, err)
	require.Equal(t, []byte("hello"), body)
}

func TestBuildOpenAPIHTTPRequestsRejectsInvalidBinaryBody(t *testing.T) {
	_, err := buildOpenAPIHTTPRequest(context.Background(), model.MCPOpenAPITool{
		ServerURL:          "https://api.example.test",
		OperationId:        "uploadRaw",
		OperationKey:       "POST /raw",
		Method:             "POST",
		Path:               "/raw",
		RequestContentType: "application/octet-stream",
		RequestBodySchema:  `{"type":"string","format":"binary"}`,
	}, nil, map[string]any{
		"body": map[string]any{
			"content_base64": "not base64!",
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "openapi binary body has invalid content_base64")
}

func TestBuildOpenAPIHTTPRequestsRejectsInvalidMultipartFileFields(t *testing.T) {
	schemaBytes, err := json.Marshal(map[string]any{
		"type": "object",
		"properties": map[string]any{
			"title": map[string]any{"type": "string"},
			"file":  map[string]any{"type": "string", "format": "binary"},
		},
	})
	require.NoError(t, err)

	_, err = buildOpenAPIHTTPRequest(context.Background(), model.MCPOpenAPITool{
		ServerURL:          "https://api.example.test",
		OperationId:        "uploadAsset",
		OperationKey:       "POST /uploads",
		Method:             "POST",
		Path:               "/uploads",
		RequestContentType: "multipart/form-data",
		RequestBodySchema:  string(schemaBytes),
	}, nil, map[string]any{
		"body": map[string]any{
			"title": "Avatar",
			"file":  "avatar.png",
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "openapi multipart file field file must be an object with content_base64")
}

func TestOpenAPIExecutorInjectsBearerAuthFromEnvRef(t *testing.T) {
	t.Setenv("PET_API_TOKEN", "test-token")

	var seenAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.MCPTool{}, &model.MCPOpenAPITool{}, &model.MCPOpenAPIBinaryObject{}))
	originalDB := model.DB
	model.DB = db
	t.Cleanup(func() {
		model.DB = originalDB
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	tool := &model.MCPTool{
		Name:        "pet_api.authed",
		DisplayName: "Authed Pet",
		Category:    "pets",
		Source:      model.MCPToolSourceOpenAPI,
		InputSchema: `{"type":"object"}`,
		Status:      model.MCPToolStatusEnabled,
	}
	require.NoError(t, model.CreateMCPToolWithOpenAPI(tool, &model.MCPOpenAPITool{
		OpenAPIUrl:   server.URL + "/openapi.json",
		ServerURL:    server.URL,
		OperationId:  "authedPet",
		OperationKey: "GET /authed",
		Method:       "GET",
		Path:         "/authed",
		AuthType:     model.MCPProxyAuthTypeBearer,
		AuthRef:      "env:PET_API_TOKEN",
	}))

	result, err := NewOpenAPIExecutor(server.Client()).Execute(context.Background(), Request{
		Tool:      *tool,
		Arguments: map[string]any{},
	})
	require.NoError(t, err)
	require.Equal(t, "Bearer test-token", seenAuth)
	require.Equal(t, model.MCPProxyAuthTypeBearer, result.Metadata["auth_type"])
	require.NotContains(t, result.Metadata, "auth_ref")
}
