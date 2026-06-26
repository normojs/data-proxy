package service

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestStartAndFinishRelayRequestCapture(t *testing.T) {
	previousDB := model.DB
	t.Cleanup(func() {
		model.DB = previousDB
	})
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	require.NoError(t, db.AutoMigrate(&model.RequestCaptureRecord{}))

	spoolDir := t.TempDir()
	t.Setenv("CAPTURE_ENABLED", "true")
	t.Setenv("CAPTURE_OBJECT_BACKEND", "s3")
	t.Setenv("CAPTURE_LEVEL", model.RequestCaptureLevelFullBundle)
	t.Setenv("CAPTURE_SPOOL_DIR", spoolDir)
	t.Setenv("CAPTURE_PATH_PREFIXES", "/v1/responses")

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	body := `{"model":"deepseek-ai/DeepSeek-V4-Flash","input":"hello"}`
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(common.RequestIdKey, "req-relay-capture")
	c.Set(string(constant.ContextKeyChannelId), 300)
	c.Set("connected_app_id", 400)

	info := &relaycommon.RelayInfo{
		RequestId:       "req-relay-capture",
		UserId:          100,
		TokenId:         200,
		UsingGroup:      "default",
		OriginModelName: "deepseek-ai/DeepSeek-V4-Flash",
		RequestURLPath:  "/v1/responses",
		RelayFormat:     types.RelayFormatOpenAIResponses,
		IsStream:        true,
	}
	info.SetEstimatePromptTokens(12)
	info.SetEstimateCompletionTokens(34)

	capture := StartRelayRequestCapture(c, info)
	require.NotNil(t, capture)
	require.NotNil(t, capture.Session)
	assert.Equal(t, model.RequestCaptureLevelFullBundle, capture.Decision.Level)

	var record model.RequestCaptureRecord
	require.NoError(t, db.Where("request_id = ?", "req-relay-capture").First(&record).Error)
	assert.Equal(t, model.RequestCaptureStatusSpooling, record.CaptureStatus)
	assert.Equal(t, int64(400), record.ConnectedAppId)
	assert.Equal(t, 300, record.ChannelId)
	assert.Equal(t, "openai_responses", record.ProtocolChain)
	assert.Equal(t, 12, record.PromptTokens)
	assert.Equal(t, 34, record.CompletionTokens)

	sessionDir := capture.Session.Dir()
	manifest := readRequestCaptureSpoolManifestForTest(t, sessionDir)
	require.Len(t, manifest.Artifacts, 1)
	assert.Equal(t, "client_request.json", manifest.Artifacts[0].Name)
	capturedBody, err := os.ReadFile(filepath.Join(sessionDir, manifest.Artifacts[0].Path))
	require.NoError(t, err)
	assert.Equal(t, body, string(capturedBody))

	upstreamResponse := &http.Response{
		Header: http.Header{"Content-Type": []string{"application/json"}},
		Body:   io.NopCloser(strings.NewReader(`{"id":"upstream-response"}`)),
	}
	c.Set(common.UpstreamRequestIdKey, "up-relay-capture")
	wrappedUpstreamResponse := WrapUpstreamResponseForCapture(c, upstreamResponse)
	upstreamBody, err := io.ReadAll(wrappedUpstreamResponse.Body)
	require.NoError(t, err)
	assert.Equal(t, `{"id":"upstream-response"}`, string(upstreamBody))

	capture.AppendDownstreamResponse("text/event-stream", []byte("data: hello\n\n"))
	FinishRelayRequestCapture(capture, nil)

	require.NoError(t, db.First(&record, record.Id).Error)
	assert.Equal(t, model.RequestCaptureStatusFinalizing, record.CaptureStatus)
	assert.NotZero(t, record.FinishedAt)
	assert.Contains(t, record.SpoolDir, filepath.Join(spoolDir, "finalize"))
	assert.Equal(t, "up-relay-capture", record.UpstreamRequestId)
	assert.Equal(t, int64(len(`{"id":"upstream-response"}`)), record.UpstreamBodyBytes)
	assert.Equal(t, int64(len("data: hello\n\n")), record.DownstreamBodyBytes)

	finalManifest := readRequestCaptureSpoolManifestForTest(t, record.SpoolDir)
	require.Len(t, finalManifest.Artifacts, 3)
	assert.Equal(t, "upstream_response.sse", finalManifest.Artifacts[1].Name)
	storedUpstreamBody, err := os.ReadFile(filepath.Join(record.SpoolDir, finalManifest.Artifacts[1].Path))
	require.NoError(t, err)
	assert.Equal(t, `{"id":"upstream-response"}`, string(storedUpstreamBody))
	assert.Equal(t, "downstream_response.sse", finalManifest.Artifacts[2].Name)
	downstreamBody, err := os.ReadFile(filepath.Join(record.SpoolDir, finalManifest.Artifacts[2].Path))
	require.NoError(t, err)
	assert.Equal(t, "data: hello\n\n", string(downstreamBody))
}

func TestRelayRequestCaptureMetadataOnlyDoesNotCreateSpoolSession(t *testing.T) {
	previousDB := model.DB
	t.Cleanup(func() {
		model.DB = previousDB
	})
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	require.NoError(t, db.AutoMigrate(&model.RequestCaptureRecord{}))

	t.Setenv("CAPTURE_ENABLED", "true")
	t.Setenv("CAPTURE_OBJECT_BACKEND", "s3")
	t.Setenv("CAPTURE_LEVEL", model.RequestCaptureLevelMetadata)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"qwen-plus"}`))

	info := &relaycommon.RelayInfo{
		RequestId:       "req-relay-metadata",
		UserId:          100,
		TokenId:         200,
		OriginModelName: "qwen-plus",
		RequestURLPath:  "/v1/chat/completions",
		RelayFormat:     types.RelayFormatOpenAI,
	}
	capture := StartRelayRequestCapture(c, info)
	require.NotNil(t, capture)
	assert.Nil(t, capture.Session)

	FinishRelayRequestCapture(capture, nil)

	var record model.RequestCaptureRecord
	require.NoError(t, db.Where("request_id = ?", "req-relay-metadata").First(&record).Error)
	assert.Equal(t, model.RequestCaptureStatusUploaded, record.CaptureStatus)
	assert.Empty(t, record.SpoolDir)
}

func TestRelayRequestCaptureFailOpenWhenDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"qwen-plus"}`))

	capture := StartRelayRequestCapture(c, &relaycommon.RelayInfo{RequestId: "req-disabled"})
	assert.Nil(t, capture)
}

func TestRelayRequestCaptureResponseWriterCapturesWrites(t *testing.T) {
	session, err := NewRequestCaptureSession(RequestCaptureSessionOptions{
		RequestId:    "req-response-writer",
		CaptureLevel: model.RequestCaptureLevelFullBundle,
		SpoolDir:     t.TempDir(),
	})
	require.NoError(t, err)
	capture := &RelayRequestCapture{
		Session:  session,
		IsStream: false,
		Decision: RequestCaptureDecision{
			Enabled: true,
			Level:   model.RequestCaptureLevelFullBundle,
		},
	}

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Writer = NewRelayRequestCaptureResponseWriter(c.Writer, capture)
	c.Header("Content-Type", "application/json")
	_, err = c.Writer.WriteString(`{"ok":true}`)
	require.NoError(t, err)

	manifest := readRequestCaptureSpoolManifestForTest(t, session.Dir())
	require.Len(t, manifest.Artifacts, 1)
	assert.Equal(t, "downstream_response.json", manifest.Artifacts[0].Name)
	body, err := os.ReadFile(filepath.Join(session.Dir(), manifest.Artifacts[0].Path))
	require.NoError(t, err)
	assert.Equal(t, `{"ok":true}`, string(body))
	assert.Equal(t, int64(len(`{"ok":true}`)), capture.DownstreamBodyBytes)
}

func TestRelayRequestCaptureResponseWriterFailOpenWhenCaptureAppendFails(t *testing.T) {
	session, err := NewRequestCaptureSession(RequestCaptureSessionOptions{
		RequestId:    "req-response-writer-fail-open",
		CaptureLevel: model.RequestCaptureLevelFullBundle,
		SpoolDir:     t.TempDir(),
	})
	require.NoError(t, err)
	require.NoError(t, os.RemoveAll(session.Dir()))
	capture := &RelayRequestCapture{
		Session:  session,
		IsStream: false,
		Decision: RequestCaptureDecision{
			Enabled: true,
			Level:   model.RequestCaptureLevelFullBundle,
		},
	}

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Writer = NewRelayRequestCaptureResponseWriter(c.Writer, capture)
	c.Header("Content-Type", "application/json")
	_, err = c.Writer.WriteString(`{"ok":true}`)
	require.NoError(t, err)

	assert.Equal(t, `{"ok":true}`, recorder.Body.String())
	assert.Equal(t, int64(len(`{"ok":true}`)), capture.DownstreamBodyBytes)
}

func TestRelayRequestCaptureStreamWriterFailureMarksRecordFailed(t *testing.T) {
	previousDB := model.DB
	t.Cleanup(func() {
		model.DB = previousDB
	})
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	require.NoError(t, db.AutoMigrate(&model.RequestCaptureRecord{}))

	spoolDir := t.TempDir()
	t.Setenv("CAPTURE_ENABLED", "true")
	t.Setenv("CAPTURE_OBJECT_BACKEND", "s3")
	t.Setenv("CAPTURE_LEVEL", model.RequestCaptureLevelFullBundle)
	t.Setenv("CAPTURE_SPOOL_DIR", spoolDir)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"qwen-plus"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(common.RequestIdKey, "req-stream-capture-fail-open")

	info := &relaycommon.RelayInfo{
		RequestId:       "req-stream-capture-fail-open",
		UserId:          100,
		TokenId:         200,
		OriginModelName: "qwen-plus",
		RequestURLPath:  "/v1/responses",
		RelayFormat:     types.RelayFormatOpenAIResponses,
		IsStream:        true,
	}

	capture := StartRelayRequestCapture(c, info)
	require.NotNil(t, capture)
	require.NotNil(t, capture.Session)
	require.NoError(t, os.RemoveAll(capture.Session.Dir()))

	c.Writer = NewRelayRequestCaptureResponseWriter(c.Writer, capture)
	c.Header("Content-Type", "text/event-stream")
	_, err = c.Writer.WriteString("data: hello\n\n")
	require.NoError(t, err)
	FinishRelayRequestCapture(capture, nil)

	assert.Equal(t, "data: hello\n\n", recorder.Body.String())

	var record model.RequestCaptureRecord
	require.NoError(t, db.Where("request_id = ?", "req-stream-capture-fail-open").First(&record).Error)
	assert.Equal(t, model.RequestCaptureStatusFailed, record.CaptureStatus)
	assert.True(t, record.HasError)
	assert.Equal(t, requestCaptureRelayFailureErrorCode, record.ErrorCode)
	assert.Contains(t, record.LastError, "no such file")
	assert.Equal(t, int64(len("data: hello\n\n")), record.DownstreamBodyBytes)
}

func TestRelayRequestCaptureUpstreamResponseWrapperCapturesJSON(t *testing.T) {
	session, err := NewRequestCaptureSession(RequestCaptureSessionOptions{
		RequestId:    "req-upstream-wrapper",
		CaptureLevel: model.RequestCaptureLevelFullBundle,
		SpoolDir:     t.TempDir(),
	})
	require.NoError(t, err)
	capture := &RelayRequestCapture{
		Session:  session,
		IsStream: false,
		Decision: RequestCaptureDecision{
			Enabled: true,
			Level:   model.RequestCaptureLevelFullBundle,
		},
	}

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set(relayRequestCaptureContextKey, capture)

	resp := &http.Response{
		Header: http.Header{"Content-Type": []string{"application/json"}},
		Body:   io.NopCloser(strings.NewReader(`{"ok":true}`)),
	}
	wrapped := WrapUpstreamResponseForCapture(c, resp)
	body, err := io.ReadAll(wrapped.Body)
	require.NoError(t, err)
	assert.Equal(t, `{"ok":true}`, string(body))

	manifest := readRequestCaptureSpoolManifestForTest(t, session.Dir())
	require.Len(t, manifest.Artifacts, 1)
	assert.Equal(t, "upstream_response.json", manifest.Artifacts[0].Name)
	storedBody, err := os.ReadFile(filepath.Join(session.Dir(), manifest.Artifacts[0].Path))
	require.NoError(t, err)
	assert.Equal(t, `{"ok":true}`, string(storedBody))
	assert.Equal(t, int64(len(`{"ok":true}`)), capture.UpstreamBodyBytes)
}

func TestRelayRequestCaptureUpstreamWrapperFailOpenWhenCaptureAppendFails(t *testing.T) {
	session, err := NewRequestCaptureSession(RequestCaptureSessionOptions{
		RequestId:    "req-upstream-wrapper-fail-open",
		CaptureLevel: model.RequestCaptureLevelFullBundle,
		SpoolDir:     t.TempDir(),
	})
	require.NoError(t, err)
	require.NoError(t, os.RemoveAll(session.Dir()))
	capture := &RelayRequestCapture{
		Session:  session,
		IsStream: false,
		Decision: RequestCaptureDecision{
			Enabled: true,
			Level:   model.RequestCaptureLevelFullBundle,
		},
	}

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set(relayRequestCaptureContextKey, capture)

	resp := &http.Response{
		Header: http.Header{"Content-Type": []string{"application/json"}},
		Body:   io.NopCloser(strings.NewReader(`{"ok":true}`)),
	}
	wrapped := WrapUpstreamResponseForCapture(c, resp)
	body, err := io.ReadAll(wrapped.Body)
	require.NoError(t, err)
	assert.Equal(t, `{"ok":true}`, string(body))
	assert.Equal(t, int64(len(`{"ok":true}`)), capture.UpstreamBodyBytes)
}

func TestRelayRequestCaptureFlushesAsyncStreamWriters(t *testing.T) {
	session, err := NewRequestCaptureSession(RequestCaptureSessionOptions{
		RequestId:    "req-async-stream-writer",
		CaptureLevel: model.RequestCaptureLevelFullBundle,
		SpoolDir:     t.TempDir(),
	})
	require.NoError(t, err)
	capture := &RelayRequestCapture{
		Session:  session,
		IsStream: true,
		Decision: RequestCaptureDecision{
			Enabled: true,
			Level:   model.RequestCaptureLevelFullBundle,
		},
	}

	capture.AppendDownstreamResponse("text/event-stream", []byte("data: one\n\n"))
	capture.AppendDownstreamResponse("text/event-stream", []byte("data: two\n\n"))
	require.NoError(t, capture.closeWriters())

	manifest := readRequestCaptureSpoolManifestForTest(t, session.Dir())
	require.Len(t, manifest.Artifacts, 1)
	assert.Equal(t, "downstream_response.sse", manifest.Artifacts[0].Name)
	body, err := os.ReadFile(filepath.Join(session.Dir(), manifest.Artifacts[0].Path))
	require.NoError(t, err)
	assert.Equal(t, "data: one\n\ndata: two\n\n", string(body))
	assert.Equal(t, int64(len("data: one\n\ndata: two\n\n")), capture.DownstreamBodyBytes)
}

func TestRelayRequestCaptureStreamArtifactLimitDoesNotLimitResponseByteCount(t *testing.T) {
	session, err := NewRequestCaptureSession(RequestCaptureSessionOptions{
		RequestId:        "req-async-stream-truncated",
		CaptureLevel:     model.RequestCaptureLevelFullBundle,
		SpoolDir:         t.TempDir(),
		MaxArtifactBytes: 7,
	})
	require.NoError(t, err)
	capture := &RelayRequestCapture{
		Session:  session,
		IsStream: true,
		Decision: RequestCaptureDecision{
			Enabled: true,
			Level:   model.RequestCaptureLevelFullBundle,
		},
	}

	capture.AppendDownstreamResponse("text/event-stream", []byte("data: one\n\n"))
	capture.AppendDownstreamResponse("text/event-stream", []byte("data: two\n\n"))
	require.NoError(t, capture.closeWriters())

	manifest := readRequestCaptureSpoolManifestForTest(t, session.Dir())
	require.Len(t, manifest.Artifacts, 1)
	assert.Equal(t, int64(7), manifest.Artifacts[0].Bytes)
	assert.True(t, manifest.Artifacts[0].Truncated)
	body, err := os.ReadFile(filepath.Join(session.Dir(), manifest.Artifacts[0].Path))
	require.NoError(t, err)
	assert.Equal(t, "data: o", string(body))
	assert.Equal(t, int64(len("data: one\n\ndata: two\n\n")), capture.DownstreamBodyBytes)
}

func TestRelayRequestCaptureStreamTruncationUpdatesRecordMetadata(t *testing.T) {
	previousDB := model.DB
	t.Cleanup(func() {
		model.DB = previousDB
	})
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	require.NoError(t, db.AutoMigrate(&model.RequestCaptureRecord{}))

	spoolDir := t.TempDir()
	t.Setenv("CAPTURE_ENABLED", "true")
	t.Setenv("CAPTURE_OBJECT_BACKEND", "s3")
	t.Setenv("CAPTURE_LEVEL", model.RequestCaptureLevelFullBundle)
	t.Setenv("CAPTURE_SPOOL_DIR", spoolDir)
	t.Setenv("CAPTURE_MAX_ARTIFACT_BYTES", "7")

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"qwen-plus"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	capture := StartRelayRequestCapture(c, &relaycommon.RelayInfo{
		RequestId:       "req-stream-truncated-metadata",
		UserId:          100,
		TokenId:         200,
		OriginModelName: "qwen-plus",
		RequestURLPath:  "/v1/responses",
		RelayFormat:     types.RelayFormatOpenAIResponses,
		IsStream:        true,
	})
	require.NotNil(t, capture)
	require.NotNil(t, capture.Session)

	capture.AppendDownstreamResponse("text/event-stream", []byte("data: one\n\n"))
	capture.AppendDownstreamResponse("text/event-stream", []byte("data: two\n\n"))
	FinishRelayRequestCapture(capture, nil)

	var record model.RequestCaptureRecord
	require.NoError(t, db.Where("request_id = ?", "req-stream-truncated-metadata").First(&record).Error)
	require.Equal(t, model.RequestCaptureStatusFinalizing, record.CaptureStatus)
	require.False(t, record.HasError)
	require.Equal(t, int64(len("data: one\n\ndata: two\n\n")), record.DownstreamBodyBytes)

	var metadata map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(record.MetadataJson), &metadata))
	require.Equal(t, true, metadata["capture_truncated"])
	require.Contains(t, metadata["capture_truncated_artifacts"], "downstream_response.sse")
	reasons, ok := metadata["capture_truncation_reasons"].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, requestCaptureRelayTruncationArtifactSizeLimit, reasons["downstream_response.sse"])
}
