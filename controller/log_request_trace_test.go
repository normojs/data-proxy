package controller

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupLogTraceTestDB(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Log{}))

	originalLogDB := model.LOG_DB
	model.LOG_DB = db
	t.Cleanup(func() {
		model.LOG_DB = originalLogDB
	})
}

func TestGetRequestLogTraceAggregatesDiagnostics(t *testing.T) {
	setupLogTraceTestDB(t)
	require.NoError(t, model.LOG_DB.Create(&[]model.Log{
		{
			UserId:            7,
			Username:          "alice",
			CreatedAt:         1000,
			Type:              model.LogTypeConsume,
			Content:           "success",
			TokenId:           11,
			TokenName:         "desktop",
			ModelName:         "deepseek-ai/DeepSeek-V4-Flash",
			Quota:             12,
			PromptTokens:      3,
			CompletionTokens:  9,
			UseTime:           4,
			IsStream:          true,
			RequestId:         "req-trace-1",
			UpstreamRequestId: "up-trace-1",
			Other:             `{"request_path":"/v1/responses","request_conversion":["OpenAI Responses","OpenAI Compatible"],"request_conversion_meta":{"responses_terminal_status":"completed","hosted_tools_filtered":["web_search"]},"stream_status":{"status":"ok","end_reason":"done"},"admin_info":{"use_channel":["42"]}}`,
		},
		{
			UserId:    8,
			Username:  "bob",
			CreatedAt: 1001,
			Type:      model.LogTypeError,
			Content:   "upstream failed",
			RequestId: "req-trace-1",
			Other:     `{"error_type":"upstream_error","status_code":502}`,
		},
	}).Error)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/log/request/:request_id", GetRequestLogTrace)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/log/request/req-trace-1", nil))

	require.Equal(t, http.StatusOK, recorder.Code)
	var body struct {
		Success bool `json:"success"`
		Data    struct {
			Total       int                    `json:"total"`
			RequestIds  []string               `json:"request_ids"`
			Diagnostics map[string]interface{} `json:"diagnostics"`
			Summary     struct {
				Status           string         `json:"status"`
				TypeCounts       map[string]int `json:"type_counts"`
				Quota            int            `json:"quota"`
				PromptTokens     int            `json:"prompt_tokens"`
				CompletionTokens int            `json:"completion_tokens"`
				IsStream         bool           `json:"is_stream"`
			} `json:"summary"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.True(t, body.Success)
	require.Equal(t, 2, body.Data.Total)
	require.Contains(t, body.Data.RequestIds, "req-trace-1")
	require.Equal(t, "error", body.Data.Summary.Status)
	require.Equal(t, 1, body.Data.Summary.TypeCounts["consume"])
	require.Equal(t, 1, body.Data.Summary.TypeCounts["error"])
	require.Equal(t, 12, body.Data.Summary.Quota)
	require.True(t, body.Data.Summary.IsStream)
	require.Equal(t, "/v1/responses", body.Data.Diagnostics["request_path"])
	require.Contains(t, body.Data.Diagnostics, "admin_info")
	require.Contains(t, body.Data.Diagnostics, "stream_status")

	meta, ok := body.Data.Diagnostics["request_conversion_meta"].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, "completed", meta["responses_terminal_status"])
	require.Contains(t, meta, "hosted_tools_filtered")
}

func TestGetSelfRequestLogTraceScopesAndRedactsDiagnostics(t *testing.T) {
	setupLogTraceTestDB(t)
	require.NoError(t, model.LOG_DB.Create(&[]model.Log{
		{
			UserId:    7,
			Username:  "alice",
			CreatedAt: 1000,
			Type:      model.LogTypeConsume,
			RequestId: "req-self-1",
			Other:     `{"request_path":"/v1/responses","stream_status":{"status":"ok"},"admin_info":{"use_channel":["42"]}}`,
		},
		{
			UserId:    8,
			Username:  "bob",
			CreatedAt: 1001,
			Type:      model.LogTypeConsume,
			RequestId: "req-self-1",
			Other:     `{"request_path":"/v1/chat/completions","admin_info":{"use_channel":["99"]}}`,
		},
	}).Error)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/log/self/request/:request_id", func(c *gin.Context) {
		c.Set("id", 7)
		GetSelfRequestLogTrace(c)
	})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/log/self/request/req-self-1", nil))

	require.Equal(t, http.StatusOK, recorder.Code)
	var body struct {
		Success bool `json:"success"`
		Data    struct {
			Total       int                    `json:"total"`
			Diagnostics map[string]interface{} `json:"diagnostics"`
			Logs        []struct {
				UserId int                    `json:"user_id"`
				Other  map[string]interface{} `json:"other"`
			} `json:"logs"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.True(t, body.Success)
	require.Equal(t, 1, body.Data.Total)
	require.Len(t, body.Data.Logs, 1)
	require.Equal(t, 7, body.Data.Logs[0].UserId)
	require.NotContains(t, body.Data.Diagnostics, "admin_info")
	require.NotContains(t, body.Data.Diagnostics, "stream_status")
	require.NotContains(t, body.Data.Logs[0].Other, "admin_info")
	require.NotContains(t, body.Data.Logs[0].Other, "stream_status")
}

func TestGenerateAndGetRequestDiagnosticReport(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.Log{},
		&model.RequestCaptureRecord{},
		&model.RequestCaptureArtifact{},
		&model.RequestDiagnosticReport{},
	))

	originalLogDB := model.LOG_DB
	originalDB := model.DB
	model.LOG_DB = db
	model.DB = db
	t.Cleanup(func() {
		model.LOG_DB = originalLogDB
		model.DB = originalDB
	})

	require.NoError(t, db.Create(&model.Log{
		UserId:    7,
		Username:  "alice",
		CreatedAt: 1000,
		Type:      model.LogTypeError,
		Content:   "empty stream response",
		RequestId: "req-diagnostic-1",
		Other:     `{"request_conversion_meta":{"responses_terminal_status":"incomplete","hosted_tools_filtered":["web_search"]},"admin_info":{"channel_failover":[{"event":"selected","channel_id":42,"channel_name":"primary","retry_index":0,"remaining_retries":1},{"event":"failed","channel_id":42,"channel_name":"primary","status_code":502,"error_code":"bad_response","retry_planned":true,"remaining_retries":1,"health_action":"record_transient","reason":"upstream returned 502"}]}}`,
	}).Error)
	require.NoError(t, db.Create(&model.RequestCaptureRecord{
		RequestId:           "req-diagnostic-1",
		ModelName:           "deepseek-ai/DeepSeek-V4-Flash",
		RequestPath:         "/v1/responses",
		CaptureLevel:        model.RequestCaptureLevelFullBundle,
		CaptureStatus:       model.RequestCaptureStatusFailed,
		IsStream:            true,
		HasError:            true,
		LastError:           "capture writer failed",
		DownstreamBodyBytes: 0,
	}).Error)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/log/request/:request_id/diagnostic", GenerateRequestDiagnosticReport)
	router.GET("/api/log/request/:request_id/diagnostic", GetRequestDiagnosticReport)

	postRecorder := httptest.NewRecorder()
	router.ServeHTTP(postRecorder, httptest.NewRequest(http.MethodPost, "/api/log/request/req-diagnostic-1/diagnostic", nil))
	require.Equal(t, http.StatusOK, postRecorder.Code)

	var postBody struct {
		Success bool `json:"success"`
		Data    struct {
			Status   string `json:"status"`
			Severity string `json:"severity"`
			Summary  string `json:"summary"`
			Report   struct {
				Findings []struct {
					Code string `json:"code"`
				} `json:"findings"`
				Capture *struct {
					CaptureStatus string `json:"capture_status"`
				} `json:"capture"`
			} `json:"report"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(postRecorder.Body.Bytes(), &postBody))
	require.True(t, postBody.Success)
	require.Equal(t, model.RequestDiagnosticStatusCompleted, postBody.Data.Status)
	require.Equal(t, "error", postBody.Data.Severity)
	require.NotEmpty(t, postBody.Data.Summary)
	require.NotNil(t, postBody.Data.Report.Capture)
	require.Equal(t, model.RequestCaptureStatusFailed, postBody.Data.Report.Capture.CaptureStatus)

	codes := make([]string, 0, len(postBody.Data.Report.Findings))
	for _, finding := range postBody.Data.Report.Findings {
		codes = append(codes, finding.Code)
	}
	require.Contains(t, codes, "log_error")
	require.Contains(t, codes, "capture_failed")
	require.Contains(t, codes, "hosted_tools_filtered")
	require.Contains(t, codes, "channel_failover_failed")

	var saved model.RequestDiagnosticReport
	require.NoError(t, db.Where("request_id = ?", "req-diagnostic-1").First(&saved).Error)
	require.Equal(t, "error", saved.Severity)

	getRecorder := httptest.NewRecorder()
	router.ServeHTTP(getRecorder, httptest.NewRequest(http.MethodGet, "/api/log/request/req-diagnostic-1/diagnostic", nil))
	require.Equal(t, http.StatusOK, getRecorder.Code)
	require.Contains(t, getRecorder.Body.String(), `"request_id":"req-diagnostic-1"`)
}

func TestGenerateRequestDiagnosticReportIncludesCaptureTruncatedFinding(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.Log{},
		&model.RequestCaptureRecord{},
		&model.RequestCaptureArtifact{},
		&model.RequestDiagnosticReport{},
	))

	originalLogDB := model.LOG_DB
	originalDB := model.DB
	model.LOG_DB = db
	model.DB = db
	t.Cleanup(func() {
		model.LOG_DB = originalLogDB
		model.DB = originalDB
	})

	require.NoError(t, db.Create(&model.Log{
		UserId:    7,
		Username:  "alice",
		CreatedAt: 1000,
		Type:      model.LogTypeConsume,
		Content:   "ok",
		RequestId: "req-diagnostic-truncated",
	}).Error)
	require.NoError(t, db.Create(&model.RequestCaptureRecord{
		RequestId:           "req-diagnostic-truncated",
		ModelName:           "qwen-plus",
		RequestPath:         "/v1/responses",
		CaptureLevel:        model.RequestCaptureLevelFullBundle,
		CaptureStatus:       model.RequestCaptureStatusUploaded,
		IsStream:            true,
		DownstreamBodyBytes: 256,
		MetadataJson:        `{"capture_truncated":true,"capture_truncated_artifacts":["downstream_response.sse"],"capture_truncation_reasons":{"downstream_response.sse":"artifact_size_limit"}}`,
	}).Error)
	require.NoError(t, db.Create(&model.RequestCaptureArtifact{
		RequestId:   "req-diagnostic-truncated",
		Kind:        model.RequestCaptureArtifactKindRawBundle,
		Status:      model.RequestCaptureArtifactStatusAvailable,
		ContentType: "application/x-data-proxy-capture-bundle",
		SizeBytes:   128,
	}).Error)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/log/request/:request_id/diagnostic", GenerateRequestDiagnosticReport)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/log/request/req-diagnostic-truncated/diagnostic", nil))

	require.Equal(t, http.StatusOK, recorder.Code)
	var body struct {
		Success bool `json:"success"`
		Data    struct {
			Severity string `json:"severity"`
			Report   struct {
				Findings []struct {
					Code   string `json:"code"`
					Detail string `json:"detail"`
				} `json:"findings"`
				Capture *struct {
					Metadata map[string]interface{} `json:"metadata"`
				} `json:"capture"`
			} `json:"report"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.True(t, body.Success)
	require.Equal(t, "warning", body.Data.Severity)
	require.NotNil(t, body.Data.Report.Capture)
	require.Equal(t, true, body.Data.Report.Capture.Metadata["capture_truncated"])

	found := false
	for _, finding := range body.Data.Report.Findings {
		if finding.Code == "capture_truncated" {
			found = true
			require.Contains(t, finding.Detail, "downstream_response.sse")
		}
	}
	require.True(t, found, "expected capture_truncated finding")
}

func TestListRequestDiagnosticCandidates(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.Log{},
		&model.RequestCaptureRecord{},
		&model.RequestDiagnosticReport{},
	))

	originalLogDB := model.LOG_DB
	originalDB := model.DB
	model.LOG_DB = db
	model.DB = db
	t.Cleanup(func() {
		model.LOG_DB = originalLogDB
		model.DB = originalDB
	})

	require.NoError(t, db.Create(&model.Log{
		UserId:    9,
		Username:  "charlie",
		CreatedAt: 2000,
		Type:      model.LogTypeError,
		Content:   "upstream stream ended without data",
		RequestId: "req-candidate-1",
		ModelName: "qwen-plus",
	}).Error)
	require.NoError(t, db.Create(&model.RequestCaptureRecord{
		RequestId:           "req-candidate-1",
		ModelName:           "qwen-plus",
		CaptureStatus:       model.RequestCaptureStatusUploaded,
		IsStream:            true,
		DownstreamBodyBytes: 0,
		CreatedAt:           2001,
	}).Error)
	require.NoError(t, db.Create(&model.RequestDiagnosticReport{
		RequestId:   "req-candidate-1",
		Status:      model.RequestDiagnosticStatusCompleted,
		Severity:    "error",
		GeneratedAt: 2002,
	}).Error)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/log/request-diagnostic-candidates", ListRequestDiagnosticCandidates)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/log/request-diagnostic-candidates?limit=10", nil))

	require.Equal(t, http.StatusOK, recorder.Code)
	var body struct {
		Success bool `json:"success"`
		Data    struct {
			Total int `json:"total"`
			Items []struct {
				RequestId      string `json:"request_id"`
				Severity       string `json:"severity"`
				Source         string `json:"source"`
				ErrorCount     int    `json:"error_count"`
				ReportStatus   string `json:"report_status"`
				ReportSeverity string `json:"report_severity"`
			} `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.True(t, body.Success)
	require.Equal(t, 1, body.Data.Total)
	require.Len(t, body.Data.Items, 1)
	require.Equal(t, "req-candidate-1", body.Data.Items[0].RequestId)
	require.Equal(t, "error", body.Data.Items[0].Severity)
	require.Contains(t, body.Data.Items[0].Source, "log_error")
	require.Contains(t, body.Data.Items[0].Source, "capture")
	require.Equal(t, 1, body.Data.Items[0].ErrorCount)
	require.Equal(t, model.RequestDiagnosticStatusCompleted, body.Data.Items[0].ReportStatus)
	require.Equal(t, "error", body.Data.Items[0].ReportSeverity)
}

func TestListRequestDiagnosticCandidatesIncludesTraceMetaAnomaly(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.Log{},
		&model.RequestCaptureRecord{},
		&model.RequestDiagnosticReport{},
	))

	originalLogDB := model.LOG_DB
	originalDB := model.DB
	model.LOG_DB = db
	model.DB = db
	t.Cleanup(func() {
		model.LOG_DB = originalLogDB
		model.DB = originalDB
	})

	require.NoError(t, db.Create(&[]model.Log{
		{
			UserId:    9,
			Username:  "charlie",
			CreatedAt: 3000,
			Type:      model.LogTypeConsume,
			Content:   "consume without explicit error log",
			RequestId: "req-candidate-trace-meta",
			ModelName: "deepseek-ai/DeepSeek-V4-Flash",
			Other:     `{"request_conversion_meta":{"responses_terminal_status":"failed","hosted_web_search_executor_error":"max_iterations_exceeded"}}`,
		},
		{
			UserId:    9,
			Username:  "charlie",
			CreatedAt: 3001,
			Type:      model.LogTypeConsume,
			Content:   "normal converted request",
			RequestId: "req-candidate-normal",
			ModelName: "deepseek-ai/DeepSeek-V4-Flash",
			Other:     `{"request_conversion_meta":{"responses_terminal_status":"completed"}}`,
		},
	}).Error)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/log/request-diagnostic-candidates", ListRequestDiagnosticCandidates)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/log/request-diagnostic-candidates?limit=10", nil))

	require.Equal(t, http.StatusOK, recorder.Code)
	var body struct {
		Success bool `json:"success"`
		Data    struct {
			Total int `json:"total"`
			Items []struct {
				RequestId    string `json:"request_id"`
				Severity     string `json:"severity"`
				Source       string `json:"source"`
				Summary      string `json:"summary"`
				ConsumeCount int    `json:"consume_count"`
				ModelName    string `json:"model_name"`
			} `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.True(t, body.Success)
	require.Equal(t, 1, body.Data.Total)
	require.Len(t, body.Data.Items, 1)
	item := body.Data.Items[0]
	require.Equal(t, "req-candidate-trace-meta", item.RequestId)
	require.Equal(t, "error", item.Severity)
	require.Equal(t, "trace_meta", item.Source)
	require.Equal(t, 1, item.ConsumeCount)
	require.Equal(t, "deepseek-ai/DeepSeek-V4-Flash", item.ModelName)
	require.Contains(t, item.Summary, "max_iterations_exceeded")
}

func TestListRequestDiagnosticCandidatesIncludesChannelFailoverAnomaly(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.Log{},
		&model.RequestCaptureRecord{},
		&model.RequestDiagnosticReport{},
	))

	originalLogDB := model.LOG_DB
	originalDB := model.DB
	model.LOG_DB = db
	model.DB = db
	t.Cleanup(func() {
		model.LOG_DB = originalLogDB
		model.DB = originalDB
	})

	require.NoError(t, db.Create(&[]model.Log{
		{
			UserId:    9,
			Username:  "charlie",
			CreatedAt: 4000,
			Type:      model.LogTypeConsume,
			Content:   "consume succeeded after retry",
			RequestId: "req-candidate-channel-failover",
			ModelName: "deepseek-ai/DeepSeek-V4-Flash",
			ChannelId: 13,
			Other:     `{"request_conversion_meta":{"responses_terminal_status":"completed"},"admin_info":{"channel_failover":[{"event":"selected","channel_id":12,"channel_name":"primary","retry_index":0,"remaining_retries":1},{"event":"failed","channel_id":12,"channel_name":"primary","status_code":502,"error_code":"bad_response","retry_planned":true,"remaining_retries":1,"health_action":"record_transient","reason":"upstream returned 502"},{"event":"selected","channel_id":13,"channel_name":"backup","retry_index":1,"remaining_retries":0}]}}`,
		},
		{
			UserId:    9,
			Username:  "charlie",
			CreatedAt: 4001,
			Type:      model.LogTypeConsume,
			Content:   "normal request",
			RequestId: "req-candidate-normal",
			ModelName: "deepseek-ai/DeepSeek-V4-Flash",
			Other:     `{"request_conversion_meta":{"responses_terminal_status":"completed"},"admin_info":{"channel_failover":[{"event":"selected","channel_id":13,"channel_name":"backup","retry_index":0,"remaining_retries":1}]}}`,
		},
	}).Error)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/log/request-diagnostic-candidates", ListRequestDiagnosticCandidates)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/log/request-diagnostic-candidates?limit=10", nil))

	require.Equal(t, http.StatusOK, recorder.Code)
	var body struct {
		Success bool `json:"success"`
		Data    struct {
			Total int `json:"total"`
			Items []struct {
				RequestId    string `json:"request_id"`
				Severity     string `json:"severity"`
				Source       string `json:"source"`
				Summary      string `json:"summary"`
				ConsumeCount int    `json:"consume_count"`
				ModelName    string `json:"model_name"`
				ChannelId    int    `json:"channel_id"`
			} `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.True(t, body.Success)
	require.Equal(t, 1, body.Data.Total)
	require.Len(t, body.Data.Items, 1)
	item := body.Data.Items[0]
	require.Equal(t, "req-candidate-channel-failover", item.RequestId)
	require.Equal(t, "warning", item.Severity)
	require.Equal(t, "channel_failover", item.Source)
	require.Equal(t, 1, item.ConsumeCount)
	require.Equal(t, "deepseek-ai/DeepSeek-V4-Flash", item.ModelName)
	require.Equal(t, 13, item.ChannelId)
	require.Contains(t, item.Summary, "已尝试切换")
	require.Contains(t, item.Summary, "status=502")
}

func TestListRequestDiagnosticCandidatesIncludesHostedToolsDirectAnswer(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.Log{},
		&model.RequestCaptureRecord{},
		&model.RequestDiagnosticReport{},
	))

	originalLogDB := model.LOG_DB
	originalDB := model.DB
	model.LOG_DB = db
	model.DB = db
	t.Cleanup(func() {
		model.LOG_DB = originalLogDB
		model.DB = originalDB
	})

	require.NoError(t, db.Create(&[]model.Log{
		{
			UserId:    9,
			Username:  "charlie",
			CreatedAt: 5000,
			Type:      model.LogTypeConsume,
			Content:   "completed with direct-answer fallback",
			RequestId: "req-candidate-hosted-tools",
			ModelName: "deepseek-ai/DeepSeek-V4-Flash",
			Other:     `{"request_conversion_meta":{"responses_terminal_status":"completed","hosted_tools_filtered":["web_search"],"hosted_tools_policy_effect":"direct_answer","hosted_tools_direct_answer_hint":true}}`,
		},
		{
			UserId:    9,
			Username:  "charlie",
			CreatedAt: 5001,
			Type:      model.LogTypeConsume,
			Content:   "native executor bridge request",
			RequestId: "req-candidate-executor-ready",
			ModelName: "deepseek-ai/DeepSeek-V4-Flash",
			Other:     `{"request_conversion_meta":{"responses_terminal_status":"completed","hosted_tools_filtered":["web_search"],"hosted_tools_policy_effect":"executor_bridge_ready"}}`,
		},
	}).Error)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/log/request-diagnostic-candidates", ListRequestDiagnosticCandidates)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/log/request-diagnostic-candidates?limit=10", nil))

	require.Equal(t, http.StatusOK, recorder.Code)
	var body struct {
		Success bool `json:"success"`
		Data    struct {
			Total int `json:"total"`
			Items []struct {
				RequestId    string `json:"request_id"`
				Severity     string `json:"severity"`
				Source       string `json:"source"`
				Summary      string `json:"summary"`
				ConsumeCount int    `json:"consume_count"`
			} `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.True(t, body.Success)
	require.Equal(t, 1, body.Data.Total)
	require.Len(t, body.Data.Items, 1)
	item := body.Data.Items[0]
	require.Equal(t, "req-candidate-hosted-tools", item.RequestId)
	require.Equal(t, "warning", item.Severity)
	require.Equal(t, "trace_meta", item.Source)
	require.Equal(t, 1, item.ConsumeCount)
	require.Contains(t, item.Summary, "web_search")
	require.Contains(t, item.Summary, "direct_answer")
}

func TestDownloadRequestDiagnosticBundleIncludesDecodedRawCapture(t *testing.T) {
	var mu sync.Mutex
	objects := map[string][]byte{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			mu.Lock()
			objects[r.URL.Path] = body
			mu.Unlock()
			w.WriteHeader(http.StatusOK)
		case http.MethodGet:
			mu.Lock()
			body, ok := objects[r.URL.Path]
			mu.Unlock()
			if !ok {
				http.NotFound(w, r)
				return
			}
			_, _ = w.Write(body)
		default:
			http.Error(w, "unsupported method", http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.Log{},
		&model.RequestCaptureRecord{},
		&model.RequestCaptureArtifact{},
		&model.RequestDiagnosticReport{},
	))

	originalLogDB := model.LOG_DB
	originalDB := model.DB
	model.LOG_DB = db
	model.DB = db
	t.Cleanup(func() {
		model.LOG_DB = originalLogDB
		model.DB = originalDB
	})

	key := bytes.Repeat([]byte{8}, 32)
	t.Setenv("CAPTURE_ENABLED", "true")
	t.Setenv("CAPTURE_OBJECT_BACKEND", "s3")
	t.Setenv("CAPTURE_S3_ENDPOINT", server.URL)
	t.Setenv("CAPTURE_S3_BUCKET", "data-proxy-captures")
	t.Setenv("CAPTURE_S3_REGION", "us-east-1")
	t.Setenv("CAPTURE_S3_ACCESS_KEY", "capture-access-key")
	t.Setenv("CAPTURE_S3_SECRET_KEY", "capture-secret-key")
	t.Setenv("CAPTURE_S3_KEY_PREFIX", "raw")
	t.Setenv("CAPTURE_BUNDLE_MASTER_KEY", "base64:"+base64.StdEncoding.EncodeToString(key))

	requestId := "req-diagnostic-bundle"
	require.NoError(t, db.Create(&model.Log{
		UserId:    3,
		Username:  "diagnostic-user",
		CreatedAt: 1000,
		Type:      model.LogTypeConsume,
		Content:   "ok",
		RequestId: requestId,
		ModelName: "qwen-plus",
	}).Error)
	require.NoError(t, db.Create(&model.RequestCaptureRecord{
		RequestId:     requestId,
		CaptureLevel:  model.RequestCaptureLevelFullBundle,
		CaptureStatus: model.RequestCaptureStatusFinalizing,
		ModelName:     "qwen-plus",
		RequestPath:   "/v1/chat/completions",
	}).Error)

	spoolDir := t.TempDir()
	session, err := service.NewRequestCaptureSession(service.RequestCaptureSessionOptions{
		RequestId:    requestId,
		CaptureLevel: model.RequestCaptureLevelFullBundle,
		SpoolDir:     spoolDir,
		ModelName:    "qwen-plus",
		RequestPath:  "/v1/chat/completions",
	})
	require.NoError(t, err)
	require.NoError(t, session.WriteArtifact("client_request.json", "application/json", []byte(`{"model":"qwen-plus"}`)))
	require.NoError(t, session.AppendArtifact("downstream_response.sse", "text/event-stream", []byte("data: hello\n\n")))
	require.NoError(t, session.Finish())
	_, err = service.FinalizeAndPersistRequestCaptureSpoolSession(context.Background(), service.RequestCaptureFinalizeOptions{SessionDir: session.Dir()})
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/log/request/:request_id/diagnostic/bundle", DownloadRequestDiagnosticBundle)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/log/request/"+requestId+"/diagnostic/bundle", nil))

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "application/zip", recorder.Header().Get("Content-Type"))
	files := readZipFilesForTest(t, recorder.Body.Bytes())
	require.Contains(t, files, "diagnostic/report.json")
	require.Contains(t, files, "capture/raw/manifest.json")
	require.Equal(t, `{"model":"qwen-plus"}`, string(files["capture/raw/artifacts/client_request.json"]))
	require.Equal(t, "data: hello\n\n", string(files["capture/raw/artifacts/downstream_response.sse"]))
}

func TestDownloadRequestDiagnosticBundleSkipsOversizedRawCapture(t *testing.T) {
	var mu sync.Mutex
	objects := map[string][]byte{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			mu.Lock()
			objects[r.URL.Path] = body
			mu.Unlock()
			w.WriteHeader(http.StatusOK)
		case http.MethodGet:
			mu.Lock()
			body, ok := objects[r.URL.Path]
			mu.Unlock()
			if !ok {
				http.NotFound(w, r)
				return
			}
			_, _ = w.Write(body)
		default:
			http.Error(w, "unsupported method", http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.Log{},
		&model.RequestCaptureRecord{},
		&model.RequestCaptureArtifact{},
		&model.RequestDiagnosticReport{},
	))

	originalLogDB := model.LOG_DB
	originalDB := model.DB
	model.LOG_DB = db
	model.DB = db
	t.Cleanup(func() {
		model.LOG_DB = originalLogDB
		model.DB = originalDB
	})

	key := bytes.Repeat([]byte{6}, 32)
	t.Setenv("CAPTURE_ENABLED", "true")
	t.Setenv("CAPTURE_OBJECT_BACKEND", "s3")
	t.Setenv("CAPTURE_S3_ENDPOINT", server.URL)
	t.Setenv("CAPTURE_S3_BUCKET", "data-proxy-captures")
	t.Setenv("CAPTURE_S3_REGION", "us-east-1")
	t.Setenv("CAPTURE_S3_ACCESS_KEY", "capture-access-key")
	t.Setenv("CAPTURE_S3_SECRET_KEY", "capture-secret-key")
	t.Setenv("CAPTURE_S3_KEY_PREFIX", "raw")
	t.Setenv("CAPTURE_BUNDLE_MASTER_KEY", "base64:"+base64.StdEncoding.EncodeToString(key))
	t.Setenv(requestDiagnosticBundleMaxRawTarBytesEnv, "32")

	requestId := "req-diagnostic-bundle-oversized"
	require.NoError(t, db.Create(&model.Log{
		UserId:    3,
		Username:  "diagnostic-user",
		CreatedAt: 1000,
		Type:      model.LogTypeConsume,
		Content:   "ok",
		RequestId: requestId,
		ModelName: "qwen-plus",
	}).Error)
	require.NoError(t, db.Create(&model.RequestCaptureRecord{
		RequestId:     requestId,
		CaptureLevel:  model.RequestCaptureLevelFullBundle,
		CaptureStatus: model.RequestCaptureStatusFinalizing,
		ModelName:     "qwen-plus",
		RequestPath:   "/v1/chat/completions",
	}).Error)

	spoolDir := t.TempDir()
	session, err := service.NewRequestCaptureSession(service.RequestCaptureSessionOptions{
		RequestId:    requestId,
		CaptureLevel: model.RequestCaptureLevelFullBundle,
		SpoolDir:     spoolDir,
		ModelName:    "qwen-plus",
		RequestPath:  "/v1/chat/completions",
	})
	require.NoError(t, err)
	require.NoError(t, session.WriteArtifact("client_request.json", "application/json", bytes.Repeat([]byte("x"), 2048)))
	require.NoError(t, session.Finish())
	_, err = service.FinalizeAndPersistRequestCaptureSpoolSession(context.Background(), service.RequestCaptureFinalizeOptions{SessionDir: session.Dir()})
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/log/request/:request_id/diagnostic/bundle", DownloadRequestDiagnosticBundle)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/log/request/"+requestId+"/diagnostic/bundle", nil))

	require.Equal(t, http.StatusOK, recorder.Code)
	files := readZipFilesForTest(t, recorder.Body.Bytes())
	require.Contains(t, files, "diagnostic/report.json")
	require.Contains(t, files, "capture/raw_bundle_skipped.txt")
	require.Contains(t, string(files["capture/raw_bundle_skipped.txt"]), "DIAGNOSTIC_BUNDLE_MAX_RAW_TAR_BYTES=32")
	require.NotContains(t, files, "capture/raw/artifacts/client_request.json")
}

func TestRequestDiagnosticBundleMaxRawTarBytesFromEnv(t *testing.T) {
	t.Setenv(requestDiagnosticBundleMaxRawTarBytesEnv, "0")
	require.Equal(t, int64(0), requestDiagnosticBundleMaxRawTarBytes())

	t.Setenv(requestDiagnosticBundleMaxRawTarBytesEnv, "1024")
	require.Equal(t, int64(1024), requestDiagnosticBundleMaxRawTarBytes())

	t.Setenv(requestDiagnosticBundleMaxRawTarBytesEnv, "-1")
	require.Equal(t, int64(requestDiagnosticBundleDefaultMaxRawTarBytes), requestDiagnosticBundleMaxRawTarBytes())

	t.Setenv(requestDiagnosticBundleMaxRawTarBytesEnv, "not-a-number")
	require.Equal(t, int64(requestDiagnosticBundleDefaultMaxRawTarBytes), requestDiagnosticBundleMaxRawTarBytes())
}

func readZipFilesForTest(t *testing.T, body []byte) map[string][]byte {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	require.NoError(t, err)
	files := map[string][]byte{}
	for _, file := range reader.File {
		rc, err := file.Open()
		require.NoError(t, err)
		content, err := io.ReadAll(rc)
		require.NoError(t, err)
		require.NoError(t, rc.Close())
		files[file.Name] = content
	}
	return files
}
