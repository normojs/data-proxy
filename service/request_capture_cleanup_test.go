package service

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestCleanupExpiredRequestCaptureDataExpiresRecordsAndDeletesArtifacts(t *testing.T) {
	var deleteCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.Error(w, "unsupported method", http.StatusMethodNotAllowed)
			return
		}
		deleteCount.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	previousDB := model.DB
	t.Cleanup(func() {
		model.DB = previousDB
	})
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	require.NoError(t, db.AutoMigrate(&model.RequestCaptureRecord{}, &model.RequestCaptureArtifact{}))

	t.Setenv("CAPTURE_OBJECT_BACKEND", "s3")
	t.Setenv("CAPTURE_S3_ENDPOINT", server.URL)
	t.Setenv("CAPTURE_S3_BUCKET", "data-proxy-captures")
	t.Setenv("CAPTURE_S3_REGION", "us-east-1")
	t.Setenv("CAPTURE_S3_ACCESS_KEY", "capture-access-key")
	t.Setenv("CAPTURE_S3_SECRET_KEY", "capture-secret-key")

	now := int64(100 * 86400)
	old := now - int64(40*86400)
	fresh := now - int64(2*86400)
	require.NoError(t, db.Create(&model.RequestCaptureRecord{
		RequestId:     "req-cleanup-old",
		CaptureStatus: model.RequestCaptureStatusUploaded,
		CaptureLevel:  model.RequestCaptureLevelFullBundle,
		CreatedAt:     old,
		UpdatedAt:     old,
	}).Error)
	require.NoError(t, db.Create(&model.RequestCaptureRecord{
		RequestId:     "req-cleanup-fresh",
		CaptureStatus: model.RequestCaptureStatusUploaded,
		CaptureLevel:  model.RequestCaptureLevelFullBundle,
		CreatedAt:     fresh,
		UpdatedAt:     fresh,
	}).Error)
	require.NoError(t, db.Create(&model.RequestCaptureArtifact{
		RequestId:  "req-cleanup-old",
		Kind:       model.RequestCaptureArtifactKindRawBundle,
		Status:     model.RequestCaptureArtifactStatusAvailable,
		Provider:   "s3",
		Bucket:     "data-proxy-captures",
		StorageKey: "raw/old.bundle.tar.zst.enc",
		SizeBytes:  123,
		UploadedAt: old,
		CreatedAt:  old,
		UpdatedAt:  old,
	}).Error)
	require.NoError(t, db.Create(&model.RequestCaptureArtifact{
		RequestId:  "req-cleanup-fresh",
		Kind:       model.RequestCaptureArtifactKindRawBundle,
		Status:     model.RequestCaptureArtifactStatusAvailable,
		Provider:   "s3",
		Bucket:     "data-proxy-captures",
		StorageKey: "raw/fresh.bundle.tar.zst.enc",
		SizeBytes:  456,
		UploadedAt: fresh,
		CreatedAt:  fresh,
		UpdatedAt:  fresh,
	}).Error)

	summary, err := CleanupExpiredRequestCaptureData(context.Background(), RequestCaptureCleanupOptions{
		RetentionDays:      30,
		SpoolRetentionDays: 0,
		Limit:              10,
		Now:                func() int64 { return now },
	})
	require.NoError(t, err)
	assert.Equal(t, 1, summary.ExpiredRecords)
	assert.Equal(t, 1, summary.DeletedArtifacts)
	assert.Equal(t, 1, summary.DeletedObjects)
	assert.Equal(t, int64(123), summary.DeletedObjectBytes)
	assert.EqualValues(t, 1, deleteCount.Load())

	var oldRecord model.RequestCaptureRecord
	require.NoError(t, db.Where("request_id = ?", "req-cleanup-old").First(&oldRecord).Error)
	assert.Equal(t, model.RequestCaptureStatusExpired, oldRecord.CaptureStatus)
	assert.Equal(t, now, oldRecord.ExpiresAt)

	var freshRecord model.RequestCaptureRecord
	require.NoError(t, db.Where("request_id = ?", "req-cleanup-fresh").First(&freshRecord).Error)
	assert.Equal(t, model.RequestCaptureStatusUploaded, freshRecord.CaptureStatus)

	var oldArtifact model.RequestCaptureArtifact
	require.NoError(t, db.Where("request_id = ?", "req-cleanup-old").First(&oldArtifact).Error)
	assert.Equal(t, model.RequestCaptureArtifactStatusDeleted, oldArtifact.Status)
	assert.Equal(t, now, oldArtifact.DeletedAt)

	var freshArtifact model.RequestCaptureArtifact
	require.NoError(t, db.Where("request_id = ?", "req-cleanup-fresh").First(&freshArtifact).Error)
	assert.Equal(t, model.RequestCaptureArtifactStatusAvailable, freshArtifact.Status)
}

func TestCleanupExpiredRequestCaptureDataRetriesFailedArtifactDeletion(t *testing.T) {
	var deleteCount atomic.Int32
	failDeletes := atomic.Bool{}
	failDeletes.Store(true)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.Error(w, "unsupported method", http.StatusMethodNotAllowed)
			return
		}
		deleteCount.Add(1)
		if failDeletes.Load() {
			http.Error(w, "temporary object storage outage", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	previousDB := model.DB
	t.Cleanup(func() {
		model.DB = previousDB
	})
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	require.NoError(t, db.AutoMigrate(&model.RequestCaptureRecord{}, &model.RequestCaptureArtifact{}))

	t.Setenv("CAPTURE_OBJECT_BACKEND", "s3")
	t.Setenv("CAPTURE_S3_ENDPOINT", server.URL)
	t.Setenv("CAPTURE_S3_BUCKET", "data-proxy-captures")
	t.Setenv("CAPTURE_S3_REGION", "us-east-1")
	t.Setenv("CAPTURE_S3_ACCESS_KEY", "capture-access-key")
	t.Setenv("CAPTURE_S3_SECRET_KEY", "capture-secret-key")

	now := int64(100 * 86400)
	old := now - int64(40*86400)
	require.NoError(t, db.Create(&model.RequestCaptureArtifact{
		RequestId:  "req-cleanup-retry",
		Kind:       model.RequestCaptureArtifactKindRawBundle,
		Status:     model.RequestCaptureArtifactStatusAvailable,
		Provider:   "s3",
		Bucket:     "data-proxy-captures",
		StorageKey: "raw/retry.bundle.tar.zst.enc",
		SizeBytes:  321,
		UploadedAt: old,
		CreatedAt:  old,
		UpdatedAt:  old,
	}).Error)

	first, err := CleanupExpiredRequestCaptureData(context.Background(), RequestCaptureCleanupOptions{
		RetentionDays:      30,
		SpoolRetentionDays: 0,
		Limit:              10,
		Now:                func() int64 { return now },
	})
	require.NoError(t, err)
	require.Equal(t, 1, first.ScannedArtifacts)
	require.Equal(t, 1, first.Failed)
	require.Equal(t, 0, first.DeletedArtifacts)
	require.EqualValues(t, 1, deleteCount.Load())

	var artifact model.RequestCaptureArtifact
	require.NoError(t, db.Where("request_id = ?", "req-cleanup-retry").First(&artifact).Error)
	require.Equal(t, model.RequestCaptureArtifactStatusFailed, artifact.Status)
	require.Contains(t, artifact.LastError, "status 503")
	require.Zero(t, artifact.DeletedAt)

	failDeletes.Store(false)
	second, err := CleanupExpiredRequestCaptureData(context.Background(), RequestCaptureCleanupOptions{
		RetentionDays:      30,
		SpoolRetentionDays: 0,
		Limit:              10,
		Now:                func() int64 { return now + 60 },
	})
	require.NoError(t, err)
	require.Equal(t, 1, second.ScannedArtifacts)
	require.Equal(t, 0, second.Failed)
	require.Equal(t, 1, second.DeletedArtifacts)
	require.Equal(t, 1, second.DeletedObjects)
	require.EqualValues(t, 2, deleteCount.Load())

	require.NoError(t, db.First(&artifact, artifact.Id).Error)
	require.Equal(t, model.RequestCaptureArtifactStatusDeleted, artifact.Status)
	require.Empty(t, artifact.LastError)
	require.Equal(t, now+60, artifact.DeletedAt)
}

func TestCleanupExpiredRequestCaptureDataCleansSpoolSafely(t *testing.T) {
	previousDB := model.DB
	t.Cleanup(func() {
		model.DB = previousDB
	})
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	require.NoError(t, db.AutoMigrate(&model.RequestCaptureRecord{}, &model.RequestCaptureArtifact{}))

	spoolDir := t.TempDir()
	now := int64(1000000)
	old := now - int64(8*86400)
	fresh := now - int64(60)

	oldFailedDir := createRequestCaptureFailedSpoolForCleanupTest(t, spoolDir, "req-spool-old-failed", old)
	freshFailedDir := createRequestCaptureFailedSpoolForCleanupTest(t, spoolDir, "req-spool-fresh-failed", fresh)
	oldUploadedFinalizeDir := createRequestCaptureFinalizeSpoolForCleanupTest(t, db, spoolDir, "req-spool-old-uploaded", model.RequestCaptureStatusUploaded, old)
	oldPendingFinalizeDir := createRequestCaptureFinalizeSpoolForCleanupTest(t, db, spoolDir, "req-spool-old-finalizing", model.RequestCaptureStatusFinalizing, old)

	summary, err := CleanupExpiredRequestCaptureData(context.Background(), RequestCaptureCleanupOptions{
		SpoolDir:           spoolDir,
		RetentionDays:      0,
		SpoolRetentionDays: 7,
		Limit:              10,
		Now:                func() int64 { return now },
	})
	require.NoError(t, err)
	assert.Equal(t, 2, summary.DeletedSpoolDirs)
	assert.NoFileExists(t, filepath.Join(oldFailedDir, requestCaptureSpoolManifestName))
	assert.NoFileExists(t, filepath.Join(oldUploadedFinalizeDir, requestCaptureSpoolManifestName))
	assert.FileExists(t, filepath.Join(freshFailedDir, requestCaptureSpoolManifestName))
	assert.FileExists(t, filepath.Join(oldPendingFinalizeDir, requestCaptureSpoolManifestName))
}

func TestCleanupExpiredRequestCaptureDataReportsSpoolWarning(t *testing.T) {
	previousDB := model.DB
	t.Cleanup(func() {
		model.DB = previousDB
	})
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	require.NoError(t, db.AutoMigrate(&model.RequestCaptureRecord{}, &model.RequestCaptureArtifact{}))

	spoolDir := t.TempDir()
	session, err := NewRequestCaptureSession(RequestCaptureSessionOptions{
		RequestId: "req-spool-warning",
		SpoolDir:  spoolDir,
		Now:       func() time.Time { return time.Unix(1000, 0) },
	})
	require.NoError(t, err)
	require.NoError(t, session.WriteArtifact("client_request.json", "application/json", bytes.Repeat([]byte("x"), 64)))

	summary, err := CleanupExpiredRequestCaptureData(context.Background(), RequestCaptureCleanupOptions{
		SpoolDir:           spoolDir,
		RetentionDays:      0,
		SpoolRetentionDays: 0,
		SpoolWarnBytes:     1,
		Limit:              10,
		Now:                func() int64 { return 1000 },
	})
	require.NoError(t, err)
	assert.True(t, summary.SpoolWarning)
	assert.Greater(t, summary.SpoolBytes, int64(0))
	assert.Equal(t, int64(1), summary.SpoolWarnBytes)
}

func TestRequestCaptureCleanupOptionsFromEnv(t *testing.T) {
	t.Setenv("CAPTURE_CLEANUP_INTERVAL_SECONDS", "12")
	t.Setenv("CAPTURE_CLEANUP_LIMIT", "34")
	t.Setenv("CAPTURE_RETENTION_DAYS", "45")
	t.Setenv("CAPTURE_SPOOL_RETENTION_DAYS", "6")
	t.Setenv("CAPTURE_SPOOL_WARN_BYTES", "1024")
	t.Setenv("CAPTURE_SPOOL_DIR", "/capture/spool")

	assert.Equal(t, 12, RequestCaptureCleanupIntervalSeconds())
	assert.Equal(t, 34, RequestCaptureCleanupLimit())
	assert.Equal(t, 45, RequestCaptureRetentionDays())
	assert.Equal(t, 6, RequestCaptureSpoolRetentionDays())

	options := RequestCaptureCleanupOptionsFromEnv()
	assert.Equal(t, "/capture/spool", options.SpoolDir)
	assert.Equal(t, 45, options.RetentionDays)
	assert.Equal(t, 6, options.SpoolRetentionDays)
	assert.Equal(t, int64(1024), options.SpoolWarnBytes)
	assert.Equal(t, 34, options.Limit)
}

func createRequestCaptureFailedSpoolForCleanupTest(t *testing.T, spoolDir string, requestId string, timestamp int64) string {
	t.Helper()
	session, err := NewRequestCaptureSession(RequestCaptureSessionOptions{
		RequestId: requestId,
		SpoolDir:  spoolDir,
		Now:       func() time.Time { return time.Unix(timestamp, 0) },
	})
	require.NoError(t, err)
	require.NoError(t, session.WriteArtifact("client_request.json", "application/json", []byte(`{"model":"qwen"}`)))
	require.NoError(t, session.Fail(assert.AnError))
	dir := session.Dir()
	setRequestCaptureSpoolManifestTimeForCleanupTest(t, dir, timestamp)
	return dir
}

func createRequestCaptureFinalizeSpoolForCleanupTest(t *testing.T, db *gorm.DB, spoolDir string, requestId string, status string, timestamp int64) string {
	t.Helper()
	require.NoError(t, db.Create(&model.RequestCaptureRecord{
		RequestId:     requestId,
		CaptureLevel:  model.RequestCaptureLevelFullBundle,
		CaptureStatus: status,
		CreatedAt:     timestamp,
		UpdatedAt:     timestamp,
	}).Error)
	session, err := NewRequestCaptureSession(RequestCaptureSessionOptions{
		RequestId:    requestId,
		CaptureLevel: model.RequestCaptureLevelFullBundle,
		SpoolDir:     spoolDir,
		Now:          func() time.Time { return time.Unix(timestamp, 0) },
	})
	require.NoError(t, err)
	require.NoError(t, session.WriteArtifact("client_request.json", "application/json", []byte(`{"model":"qwen"}`)))
	require.NoError(t, session.Finish())
	dir := session.Dir()
	setRequestCaptureSpoolManifestTimeForCleanupTest(t, dir, timestamp)
	return dir
}

func setRequestCaptureSpoolManifestTimeForCleanupTest(t *testing.T, dir string, timestamp int64) {
	t.Helper()
	manifest, err := readRequestCaptureSpoolManifest(dir)
	require.NoError(t, err)
	manifest.CreatedAt = timestamp
	manifest.UpdatedAt = timestamp
	manifest.FinishedAt = timestamp
	require.NoError(t, writeRequestCaptureSpoolManifest(dir, manifest))
	require.NoError(t, os.Chtimes(filepath.Join(dir, requestCaptureSpoolManifestName), time.Unix(timestamp, 0), time.Unix(timestamp, 0)))
}
