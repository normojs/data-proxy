package common

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type reusableBodyModelRequest struct {
	Model string `json:"model"`
	Group string `json:"group"`
}

func TestUnmarshalBodyReusableParsesSupportedContentTypes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name        string
		contentType string
		body        string
		wantModel   string
		wantGroup   string
	}{
		{
			name:        "json",
			contentType: "application/json",
			body:        `{"model":"json-model","group":"json-group"}`,
			wantModel:   "json-model",
			wantGroup:   "json-group",
		},
		{
			name:        "form urlencoded",
			contentType: gin.MIMEPOSTForm,
			body: url.Values{
				"model": []string{"form-model"},
				"group": []string{"form-group"},
			}.Encode(),
			wantModel: "form-model",
			wantGroup: "form-group",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newReusableBodyTestContext(t, tt.contentType, tt.body)

			var req reusableBodyModelRequest
			require.NoError(t, UnmarshalBodyReusable(c, &req))
			require.Equal(t, tt.wantModel, req.Model)
			require.Equal(t, tt.wantGroup, req.Group)

			bodyAfterFirstRead, err := io.ReadAll(c.Request.Body)
			require.NoError(t, err)
			require.Equal(t, tt.body, string(bodyAfterFirstRead))

			var second reusableBodyModelRequest
			require.NoError(t, UnmarshalBodyReusable(c, &second))
			require.Equal(t, req, second)
		})
	}
}

func TestUnmarshalBodyReusableParsesMultipartForm(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body, contentType := buildMultipartBody(t, map[string]string{
		"model": "multipart-model",
		"group": "multipart-group",
	})
	c := newReusableBodyTestContext(t, contentType, body)

	var req reusableBodyModelRequest
	require.NoError(t, UnmarshalBodyReusable(c, &req))
	require.Equal(t, "multipart-model", req.Model)
	require.Equal(t, "multipart-group", req.Group)

	bodyAfterFirstRead, err := io.ReadAll(c.Request.Body)
	require.NoError(t, err)
	require.Equal(t, body, string(bodyAfterFirstRead))
}

func TestUnmarshalBodyReusableIgnoresUnknownContentType(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := `{"model":"ignored-model","group":"ignored-group"}`
	c := newReusableBodyTestContext(t, "application/octet-stream", body)

	var req reusableBodyModelRequest
	require.NoError(t, UnmarshalBodyReusable(c, &req))
	require.Empty(t, req.Model)
	require.Empty(t, req.Group)

	bodyAfterFirstRead, err := io.ReadAll(c.Request.Body)
	require.NoError(t, err)
	require.Equal(t, body, string(bodyAfterFirstRead))
}

func newReusableBodyTestContext(t *testing.T, contentType string, body string) *gin.Context {
	t.Helper()

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", contentType)
	return c
}

func buildMultipartBody(t *testing.T, fields map[string]string) (string, string) {
	t.Helper()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	for key, value := range fields {
		require.NoError(t, writer.WriteField(key, value))
	}
	require.NoError(t, writer.Close())
	return buf.String(), writer.FormDataContentType()
}
