package controller

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRelayOpenAIManagementRequiresModelOrSpecificChannel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v1/files", nil)

	RelayOpenAIManagement(ctx)

	require.Equal(t, http.StatusBadRequest, recorder.Code)

	var response struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    string `json:"code"`
		} `json:"error"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.Contains(t, response.Error.Message, "requires a model-bearing request")
	require.Equal(t, "new_api_error", response.Error.Type)
	require.Equal(t, string(types.ErrorCodeInvalidRequest), response.Error.Code)
}

func TestNormalizeOpenAIManagementRequestPathStripsSubsitePrefix(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Params = gin.Params{{Key: "slug", Value: "team-a"}}
	ctx.Request = httptest.NewRequest(http.MethodGet, "/s/team-a/v1/files/file-123/content?alt=media", nil)

	require.Equal(t, "/v1/files/file-123/content?alt=media", normalizeOpenAIManagementRequestPath(ctx))
}

func TestOpenAIManagementModelNameSkipsMultipartBodyParsing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "sample.jsonl")
	require.NoError(t, err)
	_, err = part.Write([]byte(`{"prompt":"hello"}`))
	require.NoError(t, err)
	require.NoError(t, writer.WriteField("purpose", "fine-tune"))
	require.NoError(t, writer.Close())

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/files", &body)
	ctx.Request.Header.Set("Content-Type", writer.FormDataContentType())

	modelName, err := openAIManagementModelName(ctx)

	require.NoError(t, err)
	require.Empty(t, modelName)
}

func TestOpenAIManagementModelNameReadsJSONModel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/fine-tunes", strings.NewReader(`{"model":"gpt-4o-mini"}`))
	ctx.Request.Header.Set("Content-Type", "application/json; charset=utf-8")

	modelName, err := openAIManagementModelName(ctx)

	require.NoError(t, err)
	require.Equal(t, "gpt-4o-mini", modelName)
}

func TestOpenAIManagementModelNameDefaultsImageVariations(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/images/variations", nil)

	modelName, err := openAIManagementModelName(ctx)

	require.NoError(t, err)
	require.Equal(t, "dall-e-2", modelName)
}

func TestDoOpenAIManagementProxyRequestForwardsRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var gotMethod, gotPath, gotQuery, gotAuth, gotLocalHeader, gotBody string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		gotAuth = r.Header.Get("Authorization")
		gotLocalHeader = r.Header.Get(managementModelHeader)
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		gotBody = string(body)
		w.Header().Set("X-Upstream", "ok")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/files?purpose=fine-tune", strings.NewReader("payload"))
	ctx.Request.Header.Set("Content-Type", "application/octet-stream")
	ctx.Request.Header.Set(managementModelHeader, "gpt-4o-mini")

	resp, err := doOpenAIManagementProxyRequest(ctx, &relaycommon.RelayInfo{
		RequestURLPath: "/v1/files?purpose=fine-tune",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl: upstream.URL,
			ChannelType:    constant.ChannelTypeOpenAI,
			ApiKey:         "upstream-key",
		},
	})
	require.NoError(t, err)
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	require.Equal(t, http.MethodPost, gotMethod)
	require.Equal(t, "/v1/files", gotPath)
	require.Equal(t, "purpose=fine-tune", gotQuery)
	require.Equal(t, "Bearer upstream-key", gotAuth)
	require.Empty(t, gotLocalHeader)
	require.Equal(t, "payload", gotBody)
	require.Equal(t, "ok", resp.Header.Get("X-Upstream"))
	require.JSONEq(t, `{"ok":true}`, string(responseBody))
}
