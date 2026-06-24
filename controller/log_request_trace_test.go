package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/model"
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
		Other:     `{"request_conversion_meta":{"responses_terminal_status":"incomplete","hosted_tools_filtered":["web_search"]}}`,
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

	var saved model.RequestDiagnosticReport
	require.NoError(t, db.Where("request_id = ?", "req-diagnostic-1").First(&saved).Error)
	require.Equal(t, "error", saved.Severity)

	getRecorder := httptest.NewRecorder()
	router.ServeHTTP(getRecorder, httptest.NewRequest(http.MethodGet, "/api/log/request/req-diagnostic-1/diagnostic", nil))
	require.Equal(t, http.StatusOK, getRecorder.Code)
	require.Contains(t, getRecorder.Body.String(), `"request_id":"req-diagnostic-1"`)
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
