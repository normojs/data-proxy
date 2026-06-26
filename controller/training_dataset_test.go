package controller

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/klauspost/compress/zstd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestDownloadTrainingDatasetExport(t *testing.T) {
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

	previousDB := model.DB
	t.Cleanup(func() {
		model.DB = previousDB
	})
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	require.NoError(t, db.AutoMigrate(&model.TrainingDatasetVersion{}, &model.TrainingSample{}))

	t.Setenv("CAPTURE_OBJECT_BACKEND", "s3")
	t.Setenv("CAPTURE_S3_ENDPOINT", server.URL)
	t.Setenv("CAPTURE_S3_BUCKET", "data-proxy-captures")
	t.Setenv("CAPTURE_S3_REGION", "us-east-1")
	t.Setenv("CAPTURE_S3_ACCESS_KEY", "capture-access-key")
	t.Setenv("CAPTURE_S3_SECRET_KEY", "capture-secret-key")

	sourceJSONL := []byte(strings.Join([]string{
		`{"schema":"data_proxy.training.sample.v1","source_hash":"approved-hash","text":"keep"}`,
		`{"schema":"data_proxy.training.sample.v1","source_hash":"rejected-hash","text":"drop"}`,
		``,
	}, "\n"))
	body := compressZstdForTrainingDatasetControllerTest(t, sourceJSONL)
	object, err := service.SaveRequestCaptureObject(context.Background(), service.RequestCaptureObject{
		RequestId:   "dataset-v1",
		Kind:        model.RequestCaptureArtifactKindTrainingDataset,
		ContentType: "application/x-ndjson",
		StorageKey:  "training/dataset=test/version=v1/date=2026-06-25/part-0001.jsonl.zst",
	}, body)
	require.NoError(t, err)

	dataset := model.TrainingDatasetVersion{
		Name:         "test",
		Version:      "v1",
		Status:       model.TrainingDatasetStatusCompleted,
		OutputFormat: "jsonl.zst",
		Provider:     object.Provider,
		Bucket:       object.Bucket,
		StorageKey:   object.StorageKey,
		SHA256:       object.SHA256,
		SizeBytes:    object.BodyBytes,
		SampleCount:  2,
	}
	require.NoError(t, db.Create(&dataset).Error)
	require.NoError(t, db.Create(&model.TrainingSample{
		DatasetVersionId: dataset.Id,
		RequestId:        "req-approved",
		SourceHash:       "approved-hash",
		ReviewStatus:     model.TrainingSampleReviewStatusApproved,
	}).Error)
	require.NoError(t, db.Create(&model.TrainingSample{
		DatasetVersionId: dataset.Id,
		RequestId:        "req-rejected",
		SourceHash:       "rejected-hash",
		ReviewStatus:     model.TrainingSampleReviewStatusRejected,
	}).Error)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/training/datasets/:id/export", DownloadTrainingDatasetExport)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/training/datasets/1/export", nil))

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "application/zstd", recorder.Header().Get("Content-Type"))
	assert.Contains(t, recorder.Header().Get("Content-Disposition"), "data-proxy-training-test-v1-approved.jsonl.zst")
	assert.NotEqual(t, object.SHA256, recorder.Header().Get("X-Data-Proxy-Training-Dataset-SHA256"))
	assert.Equal(t, object.SHA256, recorder.Header().Get("X-Data-Proxy-Training-Dataset-Source-SHA256"))
	assert.Equal(t, "2", recorder.Header().Get("X-Data-Proxy-Training-Dataset-Source-Samples"))
	assert.Equal(t, "1", recorder.Header().Get("X-Data-Proxy-Training-Dataset-Approved-Samples"))
	assert.Equal(t, "1", recorder.Header().Get("X-Data-Proxy-Training-Dataset-Exported-Samples"))
	exported := decompressZstdForTrainingDatasetControllerTest(t, recorder.Body.Bytes())
	assert.Contains(t, string(exported), "keep")
	assert.NotContains(t, string(exported), "drop")
}

func TestDownloadTrainingDatasetExportRejectsUnavailableDataset(t *testing.T) {
	previousDB := model.DB
	t.Cleanup(func() {
		model.DB = previousDB
	})
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	require.NoError(t, db.AutoMigrate(&model.TrainingDatasetVersion{}))
	require.NoError(t, db.Create(&model.TrainingDatasetVersion{
		Name:    "test",
		Version: "building",
		Status:  model.TrainingDatasetStatusBuilding,
	}).Error)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/training/datasets/:id/export", DownloadTrainingDatasetExport)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/training/datasets/1/export", nil))

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, strings.ToLower(recorder.Body.String()), "not available")
}

func TestGetTrainingSamplePreview(t *testing.T) {
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

	previousDB := model.DB
	t.Cleanup(func() {
		model.DB = previousDB
	})
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	require.NoError(t, db.AutoMigrate(&model.TrainingDatasetVersion{}, &model.TrainingSample{}))

	t.Setenv("CAPTURE_OBJECT_BACKEND", "s3")
	t.Setenv("CAPTURE_S3_ENDPOINT", server.URL)
	t.Setenv("CAPTURE_S3_BUCKET", "data-proxy-captures")
	t.Setenv("CAPTURE_S3_REGION", "us-east-1")
	t.Setenv("CAPTURE_S3_ACCESS_KEY", "capture-access-key")
	t.Setenv("CAPTURE_S3_SECRET_KEY", "capture-secret-key")

	body := compressZstdForTrainingDatasetControllerTest(t, []byte(strings.Join([]string{
		`{"source_hash":"preview-hash","text":"visible sample"}`,
		`{"source_hash":"other-hash","text":"hidden sample"}`,
		``,
	}, "\n")))
	object, err := service.SaveRequestCaptureObject(context.Background(), service.RequestCaptureObject{
		RequestId:   "dataset-preview",
		Kind:        model.RequestCaptureArtifactKindTrainingDataset,
		ContentType: "application/x-ndjson",
		StorageKey:  "training/dataset=test/version=preview/date=2026-06-25/part-0001.jsonl.zst",
	}, body)
	require.NoError(t, err)
	dataset := model.TrainingDatasetVersion{
		Name:         "test",
		Version:      "preview",
		Status:       model.TrainingDatasetStatusCompleted,
		OutputFormat: "jsonl.zst",
		Provider:     object.Provider,
		Bucket:       object.Bucket,
		StorageKey:   object.StorageKey,
		SHA256:       object.SHA256,
		SizeBytes:    object.BodyBytes,
		SampleCount:  2,
	}
	require.NoError(t, db.Create(&dataset).Error)
	require.NoError(t, db.Create(&model.TrainingSample{
		DatasetVersionId: dataset.Id,
		RequestId:        "req-preview",
		SourceHash:       "preview-hash",
		ReviewStatus:     model.TrainingSampleReviewStatusPending,
	}).Error)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/training/samples/:id/preview", GetTrainingSamplePreview)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/training/samples/1/preview", nil))

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "visible sample")
	assert.NotContains(t, recorder.Body.String(), "hidden sample")
}

func TestListTrainingSamplesFiltersByDatasetVersion(t *testing.T) {
	previousDB := model.DB
	t.Cleanup(func() {
		model.DB = previousDB
	})
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	require.NoError(t, db.AutoMigrate(&model.TrainingSample{}))
	require.NoError(t, db.Create(&model.TrainingSample{
		DatasetVersionId: 10,
		RequestId:        "req-keep",
		ModelName:        "qwen-plus",
		QualityScore:     80,
		ReviewStatus:     model.TrainingSampleReviewStatusPending,
	}).Error)
	require.NoError(t, db.Create(&model.TrainingSample{
		DatasetVersionId: 11,
		RequestId:        "req-skip",
		ModelName:        "deepseek-chat",
		QualityScore:     80,
		ReviewStatus:     model.TrainingSampleReviewStatusApproved,
	}).Error)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/training/samples", ListTrainingSamples)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/training/samples?dataset_version_id=10&page_size=10", nil))

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "req-keep")
	assert.NotContains(t, recorder.Body.String(), "req-skip")

	statusRecorder := httptest.NewRecorder()
	router.ServeHTTP(statusRecorder, httptest.NewRequest(http.MethodGet, "/api/training/samples?review_status=approved&page_size=10", nil))
	require.Equal(t, http.StatusOK, statusRecorder.Code)
	assert.Contains(t, statusRecorder.Body.String(), "req-skip")
	assert.NotContains(t, statusRecorder.Body.String(), "req-keep")
}

func TestReviewTrainingSampleApproveAndReject(t *testing.T) {
	previousDB := model.DB
	t.Cleanup(func() {
		model.DB = previousDB
	})
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	require.NoError(t, db.AutoMigrate(&model.TrainingSample{}))
	require.NoError(t, db.Create(&model.TrainingSample{
		DatasetVersionId: 12,
		RequestId:        "req-review",
		ModelName:        "qwen-plus",
		QualityScore:     80,
		ReviewStatus:     model.TrainingSampleReviewStatusPending,
	}).Error)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/training/samples/:id/approve", ApproveTrainingSample)
	router.POST("/api/training/samples/:id/reject", RejectTrainingSample)

	approveRecorder := httptest.NewRecorder()
	approveReq := httptest.NewRequest(http.MethodPost, "/api/training/samples/1/approve", strings.NewReader(`{"comment":"looks good"}`))
	approveReq.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(approveRecorder, approveReq)
	require.Equal(t, http.StatusOK, approveRecorder.Code)
	assert.Contains(t, approveRecorder.Body.String(), model.TrainingSampleReviewStatusApproved)

	var sample model.TrainingSample
	require.NoError(t, db.First(&sample, 1).Error)
	assert.Equal(t, model.TrainingSampleReviewStatusApproved, sample.ReviewStatus)
	assert.Equal(t, "looks good", sample.ReviewComment)
	assert.Greater(t, sample.ReviewedAt, int64(0))

	rejectRecorder := httptest.NewRecorder()
	rejectReq := httptest.NewRequest(http.MethodPost, "/api/training/samples/1/reject", strings.NewReader(`{"comment":"contains noisy output"}`))
	rejectReq.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rejectRecorder, rejectReq)
	require.Equal(t, http.StatusOK, rejectRecorder.Code)

	require.NoError(t, db.First(&sample, 1).Error)
	assert.Equal(t, model.TrainingSampleReviewStatusRejected, sample.ReviewStatus)
	assert.Equal(t, "contains noisy output", sample.ReviewComment)
}

func compressZstdForTrainingDatasetControllerTest(t *testing.T, body []byte) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer, err := zstd.NewWriter(&buffer)
	require.NoError(t, err)
	_, err = writer.Write(body)
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	return buffer.Bytes()
}

func decompressZstdForTrainingDatasetControllerTest(t *testing.T, body []byte) []byte {
	t.Helper()
	reader, err := zstd.NewReader(bytes.NewReader(body))
	require.NoError(t, err)
	defer reader.Close()
	decoded, err := io.ReadAll(reader)
	require.NoError(t, err)
	return decoded
}
