package service

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/klauspost/compress/zstd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestFinalizeRequestCaptureSpoolSessionUploadsEncryptedBundle(t *testing.T) {
	var mu sync.Mutex
	objects := map[string][]byte{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			http.Error(w, "missing authorization", http.StatusForbidden)
			return
		}
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

	key := bytes.Repeat([]byte{7}, 32)
	t.Setenv("CAPTURE_ENABLED", "true")
	t.Setenv("CAPTURE_OBJECT_BACKEND", "s3")
	t.Setenv("CAPTURE_S3_ENDPOINT", server.URL)
	t.Setenv("CAPTURE_S3_BUCKET", "data-proxy-captures")
	t.Setenv("CAPTURE_S3_REGION", "us-east-1")
	t.Setenv("CAPTURE_S3_ACCESS_KEY", "capture-access-key")
	t.Setenv("CAPTURE_S3_SECRET_KEY", "capture-secret-key")
	t.Setenv("CAPTURE_S3_KEY_PREFIX", "raw")
	t.Setenv("CAPTURE_BUNDLE_MASTER_KEY", "base64:"+base64.StdEncoding.EncodeToString(key))
	t.Setenv("CAPTURE_BUNDLE_KEY_ID", "test-key-1")

	spoolDir := t.TempDir()
	session, err := NewRequestCaptureSession(RequestCaptureSessionOptions{
		RequestId:     "req-finalizer-123",
		CaptureLevel:  model.RequestCaptureLevelFullBundle,
		SpoolDir:      spoolDir,
		ModelName:     "moonshot-v1",
		RequestPath:   "/v1/chat/completions",
		ProtocolChain: "chat",
		Now: func() time.Time {
			return time.Date(2026, 6, 23, 13, 0, 0, 0, time.UTC)
		},
	})
	require.NoError(t, err)
	require.NoError(t, session.WriteArtifact("client_request.json", "application/json", []byte(`{"model":"moonshot-v1"}`)))
	require.NoError(t, session.AppendArtifact("downstream_response.sse", "text/event-stream", []byte("data: hello\n\n")))
	require.NoError(t, session.Finish())

	result, err := FinalizeRequestCaptureSpoolSession(context.Background(), RequestCaptureFinalizeOptions{SessionDir: session.Dir()})
	require.NoError(t, err)

	assert.Equal(t, "req-finalizer-123", result.Manifest.RequestId)
	assert.Equal(t, model.RequestCaptureArtifactKindRawBundle, result.Artifact.Kind)
	assert.Equal(t, model.RequestCaptureArtifactStatusAvailable, result.Artifact.Status)
	assert.Equal(t, requestCaptureBundleCompression, result.Artifact.Compression)
	assert.Equal(t, requestCaptureBundleEncryption, result.Artifact.EncryptionAlgorithm)
	assert.Equal(t, "test-key-1", result.Artifact.EncryptionKeyId)
	assert.Equal(t, result.Object.StorageKey, result.Artifact.StorageKey)
	assert.Equal(t, int64(len(requestCaptureBundleMagic))+12+result.CompressedBytes+16, result.EncryptedBytes)
	assert.Greater(t, result.TarBytes, int64(0))
	assert.Greater(t, result.CompressedBytes, int64(0))

	encrypted, err := LoadRequestCaptureObject(context.Background(), result.Object.StorageKey)
	require.NoError(t, err)
	assert.True(t, bytes.HasPrefix(encrypted, []byte(requestCaptureBundleMagic)))

	tarBody := decryptAndDecompressRequestCaptureBundleForTest(t, encrypted, key)
	files := readTarFilesForTest(t, tarBody)
	assert.Contains(t, files, requestCaptureSpoolManifestName)
	assert.Equal(t, `{"model":"moonshot-v1"}`, string(files["artifacts/client_request.json"]))
	assert.Equal(t, "data: hello\n\n", string(files["artifacts/downstream_response.sse"]))
}

func TestFinalizeRequestCaptureSpoolSessionRequiresFinalizeStatus(t *testing.T) {
	session, err := NewRequestCaptureSession(RequestCaptureSessionOptions{
		RequestId: "req-not-finalized",
		SpoolDir:  t.TempDir(),
	})
	require.NoError(t, err)

	_, err = FinalizeRequestCaptureSpoolSession(context.Background(), RequestCaptureFinalizeOptions{SessionDir: session.Dir()})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected finalize status")
}

func TestDecodeRequestCaptureBundleWithLimit(t *testing.T) {
	key := bytes.Repeat([]byte{3}, 32)
	t.Setenv("CAPTURE_BUNDLE_MASTER_KEY", "base64:"+base64.StdEncoding.EncodeToString(key))

	compressed, err := compressRequestCaptureBundleZstd([]byte("hello world"))
	require.NoError(t, err)
	encrypted, err := encryptRequestCaptureBundle(compressed, key)
	require.NoError(t, err)

	decoded, err := DecodeRequestCaptureBundleWithLimit(encrypted, 11)
	require.NoError(t, err)
	assert.Equal(t, "hello world", string(decoded))

	_, err = DecodeRequestCaptureBundleWithLimit(encrypted, 5)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrRequestCaptureBundleDecodedTooLarge)
}

func TestPersistRequestCaptureFinalizeResultUpdatesCaptureRecord(t *testing.T) {
	previousDB := model.DB
	t.Cleanup(func() {
		model.DB = previousDB
	})
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	require.NoError(t, db.AutoMigrate(&model.RequestCaptureRecord{}, &model.RequestCaptureArtifact{}))

	record := model.RequestCaptureRecord{
		RequestId:      "req-persist-finalizer",
		CaptureLevel:   model.RequestCaptureLevelFullBundle,
		CaptureStatus:  model.RequestCaptureStatusFinalizing,
		ModelName:      "qwen-plus",
		RequestPath:    "/v1/responses",
		ProtocolChain:  "responses->chat",
		SpoolDir:       "/spool/finalize/req-persist-finalizer",
		MetadataJson:   `{"source":"test"}`,
		ConversionJson: `{"mode":"responses_to_chat"}`,
	}
	require.NoError(t, db.Create(&record).Error)

	artifact, err := PersistRequestCaptureFinalizeResult(context.Background(), RequestCaptureFinalizeResult{
		Manifest: RequestCaptureSpoolManifest{
			RequestId: "req-persist-finalizer",
		},
		Artifact: model.RequestCaptureArtifact{
			RequestId:           "req-persist-finalizer",
			Kind:                model.RequestCaptureArtifactKindRawBundle,
			Status:              model.RequestCaptureArtifactStatusAvailable,
			Provider:            "s3",
			Bucket:              "data-proxy-captures",
			StorageKey:          "raw/2026/06/23/13/re/q-/req-persist-finalizer.bundle.tar.zst.enc",
			Compression:         requestCaptureBundleCompression,
			EncryptionAlgorithm: requestCaptureBundleEncryption,
			EncryptionKeyId:     "test-key",
			SHA256:              "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			SizeBytes:           1234,
			UploadedAt:          100,
		},
	})
	require.NoError(t, err)
	assert.NotZero(t, artifact.Id)
	assert.Equal(t, record.Id, artifact.CaptureId)

	var updatedRecord model.RequestCaptureRecord
	require.NoError(t, db.First(&updatedRecord, record.Id).Error)
	assert.Equal(t, model.RequestCaptureStatusUploaded, updatedRecord.CaptureStatus)
	assert.Equal(t, int64(100), updatedRecord.FinalizedAt)
	assert.Equal(t, int64(1234), updatedRecord.TotalBytes)

	var count int64
	require.NoError(t, db.Model(&model.RequestCaptureArtifact{}).Where("request_id = ?", "req-persist-finalizer").Count(&count).Error)
	assert.EqualValues(t, 1, count)
}

func TestPersistRequestCaptureFinalizeResultClearsPreviousFailureState(t *testing.T) {
	previousDB := model.DB
	t.Cleanup(func() {
		model.DB = previousDB
	})
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	require.NoError(t, db.AutoMigrate(&model.RequestCaptureRecord{}, &model.RequestCaptureArtifact{}))

	record := model.RequestCaptureRecord{
		RequestId:        "req-persist-finalizer-clears-error",
		CaptureLevel:     model.RequestCaptureLevelFullBundle,
		CaptureStatus:    model.RequestCaptureStatusFinalizing,
		HasError:         true,
		ErrorCode:        "request_capture_finalize_failed",
		LastError:        "previous object storage outage",
		FinalizeAttempts: 3,
		NextFinalizeAt:   12345,
	}
	require.NoError(t, db.Create(&record).Error)

	_, err = PersistRequestCaptureFinalizeResult(context.Background(), RequestCaptureFinalizeResult{
		Manifest: RequestCaptureSpoolManifest{
			RequestId: "req-persist-finalizer-clears-error",
		},
		Artifact: model.RequestCaptureArtifact{
			RequestId:  "req-persist-finalizer-clears-error",
			Kind:       model.RequestCaptureArtifactKindRawBundle,
			Status:     model.RequestCaptureArtifactStatusAvailable,
			Provider:   "s3",
			Bucket:     "data-proxy-captures",
			StorageKey: "raw/req-persist-finalizer-clears-error.bundle.tar.zst.enc",
			SHA256:     "abcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcd",
			SizeBytes:  456,
			UploadedAt: 200,
		},
	})
	require.NoError(t, err)

	var updated model.RequestCaptureRecord
	require.NoError(t, db.First(&updated, record.Id).Error)
	require.Equal(t, model.RequestCaptureStatusUploaded, updated.CaptureStatus)
	require.False(t, updated.HasError)
	require.Empty(t, updated.ErrorCode)
	require.Empty(t, updated.LastError)
	require.Equal(t, 0, updated.FinalizeAttempts)
	require.Equal(t, int64(0), updated.NextFinalizeAt)
}

func TestFinalizePendingRequestCaptureSpoolProcessesFinalizeDirs(t *testing.T) {
	var mu sync.Mutex
	objects := map[string][]byte{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			http.Error(w, "unsupported method", http.StatusMethodNotAllowed)
			return
		}
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		mu.Lock()
		objects[r.URL.Path] = body
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
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

	key := bytes.Repeat([]byte{9}, 32)
	t.Setenv("CAPTURE_ENABLED", "true")
	t.Setenv("CAPTURE_OBJECT_BACKEND", "s3")
	t.Setenv("CAPTURE_S3_ENDPOINT", server.URL)
	t.Setenv("CAPTURE_S3_BUCKET", "data-proxy-captures")
	t.Setenv("CAPTURE_S3_REGION", "us-east-1")
	t.Setenv("CAPTURE_S3_ACCESS_KEY", "capture-access-key")
	t.Setenv("CAPTURE_S3_SECRET_KEY", "capture-secret-key")
	t.Setenv("CAPTURE_S3_KEY_PREFIX", "raw")
	t.Setenv("CAPTURE_BUNDLE_MASTER_KEY", "base64:"+base64.StdEncoding.EncodeToString(key))

	spoolDir := t.TempDir()
	for _, requestId := range []string{"req-worker-1", "req-worker-2"} {
		record := model.RequestCaptureRecord{
			RequestId:     requestId,
			CaptureLevel:  model.RequestCaptureLevelFullBundle,
			CaptureStatus: model.RequestCaptureStatusFinalizing,
			ModelName:     "qwen-plus",
			RequestPath:   "/v1/chat/completions",
		}
		require.NoError(t, db.Create(&record).Error)
		session, err := NewRequestCaptureSession(RequestCaptureSessionOptions{
			RequestId:    requestId,
			CaptureLevel: model.RequestCaptureLevelFullBundle,
			SpoolDir:     spoolDir,
			ModelName:    "qwen-plus",
			RequestPath:  "/v1/chat/completions",
		})
		require.NoError(t, err)
		require.NoError(t, session.WriteArtifact("client_request.json", "application/json", []byte(`{"model":"qwen-plus"}`)))
		require.NoError(t, session.Finish())
	}

	summary, err := FinalizePendingRequestCaptureSpool(context.Background(), RequestCaptureFinalizerWorkerOptions{
		SpoolDir:        spoolDir,
		Limit:           10,
		RemoveOnSuccess: true,
	})
	require.NoError(t, err)
	assert.Equal(t, 2, summary.Scanned)
	assert.Equal(t, 2, summary.Succeeded)
	assert.Equal(t, 0, summary.Failed)

	var artifactCount int64
	require.NoError(t, db.Model(&model.RequestCaptureArtifact{}).Count(&artifactCount).Error)
	assert.EqualValues(t, 2, artifactCount)

	var uploadedCount int64
	require.NoError(t, db.Model(&model.RequestCaptureRecord{}).
		Where("capture_status = ?", model.RequestCaptureStatusUploaded).
		Count(&uploadedCount).Error)
	assert.EqualValues(t, 2, uploadedCount)

	mu.Lock()
	assert.Len(t, objects, 2)
	mu.Unlock()

	entries, err := os.ReadDir(filepath.Join(spoolDir, requestCaptureSpoolStatusFinalize))
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestFinalizePendingRequestCaptureSpoolBacksOffAfterFailure(t *testing.T) {
	previousDB := model.DB
	t.Cleanup(func() {
		model.DB = previousDB
	})
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	require.NoError(t, db.AutoMigrate(&model.RequestCaptureRecord{}, &model.RequestCaptureArtifact{}))

	key := bytes.Repeat([]byte{5}, 32)
	t.Setenv("CAPTURE_BUNDLE_MASTER_KEY", "base64:"+base64.StdEncoding.EncodeToString(key))
	t.Setenv("CAPTURE_OBJECT_BACKEND", "")

	spoolDir := t.TempDir()
	requestId := "req-finalizer-backoff"
	require.NoError(t, db.Create(&model.RequestCaptureRecord{
		RequestId:        requestId,
		CaptureLevel:     model.RequestCaptureLevelFullBundle,
		CaptureStatus:    model.RequestCaptureStatusFinalizing,
		ModelName:        "deepseek-chat",
		RequestPath:      "/v1/responses",
		ProtocolChain:    "responses->chat",
		FinalizeAttempts: 0,
	}).Error)
	session, err := NewRequestCaptureSession(RequestCaptureSessionOptions{
		RequestId:    requestId,
		CaptureLevel: model.RequestCaptureLevelFullBundle,
		SpoolDir:     spoolDir,
		ModelName:    "deepseek-chat",
		RequestPath:  "/v1/responses",
	})
	require.NoError(t, err)
	require.NoError(t, session.WriteArtifact("client_request.json", "application/json", []byte(`{"model":"deepseek-chat"}`)))
	require.NoError(t, session.Finish())

	first, err := FinalizePendingRequestCaptureSpool(context.Background(), RequestCaptureFinalizerWorkerOptions{
		SpoolDir:         spoolDir,
		Limit:            10,
		RetryBaseSeconds: 10,
		RetryMaxSeconds:  100,
		Now:              func() int64 { return 1000 },
		RemoveOnSuccess:  true,
	})
	require.NoError(t, err)
	require.Equal(t, 1, first.Scanned)
	require.Equal(t, 1, first.Failed)

	var afterFailure model.RequestCaptureRecord
	require.NoError(t, db.Where("request_id = ?", requestId).First(&afterFailure).Error)
	require.Equal(t, 1, afterFailure.FinalizeAttempts)
	require.Equal(t, int64(1010), afterFailure.NextFinalizeAt)
	require.Equal(t, model.RequestCaptureStatusFinalizing, afterFailure.CaptureStatus)
	require.True(t, afterFailure.HasError)
	require.Contains(t, afterFailure.LastError, "request capture object backend is disabled")

	second, err := FinalizePendingRequestCaptureSpool(context.Background(), RequestCaptureFinalizerWorkerOptions{
		SpoolDir:         spoolDir,
		Limit:            10,
		RetryBaseSeconds: 10,
		RetryMaxSeconds:  100,
		Now:              func() int64 { return 1005 },
		RemoveOnSuccess:  true,
	})
	require.NoError(t, err)
	require.Equal(t, 1, second.Scanned)
	require.Equal(t, 1, second.Skipped)
	require.Equal(t, 0, second.Failed)
}

func TestFinalizePendingRequestCaptureSpoolQuarantinesUnreadableManifest(t *testing.T) {
	previousDB := model.DB
	t.Cleanup(func() {
		model.DB = previousDB
	})
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	require.NoError(t, db.AutoMigrate(&model.RequestCaptureRecord{}, &model.RequestCaptureArtifact{}))

	spoolDir := t.TempDir()
	requestId := "req-finalizer-corrupt-manifest"
	sessionDir := filepath.Join(spoolDir, requestCaptureSpoolStatusFinalize, requestId)
	require.NoError(t, os.MkdirAll(sessionDir, 0700))
	require.NoError(t, os.WriteFile(filepath.Join(sessionDir, requestCaptureSpoolManifestName), []byte("{broken"), 0600))
	require.NoError(t, db.Create(&model.RequestCaptureRecord{
		RequestId:     requestId,
		CaptureLevel:  model.RequestCaptureLevelFullBundle,
		CaptureStatus: model.RequestCaptureStatusFinalizing,
		SpoolDir:      sessionDir,
	}).Error)

	summary, err := FinalizePendingRequestCaptureSpool(context.Background(), RequestCaptureFinalizerWorkerOptions{
		SpoolDir: spoolDir,
		Limit:    10,
		Now:      func() int64 { return 1234 },
	})
	require.NoError(t, err)
	assert.Equal(t, 1, summary.Scanned)
	assert.Equal(t, 1, summary.Failed)
	require.NotEmpty(t, summary.Errors)
	assert.Contains(t, summary.Errors[0], "moved to failed spool")

	_, err = os.Stat(sessionDir)
	assert.True(t, os.IsNotExist(err))
	failedDir := filepath.Join(spoolDir, requestCaptureSpoolStatusFailed, requestId)
	manifest, err := readRequestCaptureSpoolManifest(failedDir)
	require.NoError(t, err)
	assert.Equal(t, requestCaptureSpoolStatusFailed, manifest.Status)
	assert.Contains(t, manifest.Error, "manifest unreadable")

	var record model.RequestCaptureRecord
	require.NoError(t, db.Where("request_id = ?", requestId).First(&record).Error)
	assert.Equal(t, model.RequestCaptureStatusFailed, record.CaptureStatus)
	assert.True(t, record.HasError)
	assert.Contains(t, record.LastError, "manifest unreadable")
	assert.Equal(t, failedDir, record.SpoolDir)
}

func TestRecoverStaleRequestCaptureSpoolMovesActiveToFailed(t *testing.T) {
	previousDB := model.DB
	t.Cleanup(func() {
		model.DB = previousDB
	})
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	require.NoError(t, db.AutoMigrate(&model.RequestCaptureRecord{}, &model.RequestCaptureArtifact{}))

	spoolDir := t.TempDir()
	requestId := "req-recover-active"
	require.NoError(t, db.Create(&model.RequestCaptureRecord{
		RequestId:     requestId,
		CaptureLevel:  model.RequestCaptureLevelFullBundle,
		CaptureStatus: model.RequestCaptureStatusSpooling,
		ModelName:     "qwen-plus",
		RequestPath:   "/v1/chat/completions",
	}).Error)
	session, err := NewRequestCaptureSession(RequestCaptureSessionOptions{
		RequestId:    requestId,
		CaptureLevel: model.RequestCaptureLevelFullBundle,
		SpoolDir:     spoolDir,
		ModelName:    "qwen-plus",
		RequestPath:  "/v1/chat/completions",
		Now: func() time.Time {
			return time.Unix(900, 0)
		},
	})
	require.NoError(t, err)
	require.NoError(t, session.WriteArtifact("client_request.json", "application/json", []byte(`{"model":"qwen-plus"}`)))

	summary, err := RecoverStaleRequestCaptureSpool(context.Background(), RequestCaptureSpoolRecoveryOptions{
		SpoolDir:           spoolDir,
		ActiveStaleSeconds: 0,
		Now:                func() int64 { return 1000 },
	})
	require.NoError(t, err)
	require.Equal(t, 1, summary.ActiveRecovered)
	require.Empty(t, summary.Errors)

	activeEntries, err := os.ReadDir(filepath.Join(spoolDir, requestCaptureSpoolStatusActive))
	require.NoError(t, err)
	require.Empty(t, activeEntries)
	failedEntries, err := os.ReadDir(filepath.Join(spoolDir, requestCaptureSpoolStatusFailed))
	require.NoError(t, err)
	require.Len(t, failedEntries, 1)

	var record model.RequestCaptureRecord
	require.NoError(t, db.Where("request_id = ?", requestId).First(&record).Error)
	require.Equal(t, model.RequestCaptureStatusFailed, record.CaptureStatus)
	require.True(t, record.HasError)
	require.Contains(t, record.LastError, "recovered stale active")
	require.Contains(t, record.SpoolDir, requestCaptureSpoolStatusFailed)
}

func TestRecoverStaleRequestCaptureSpoolQuarantinesUnreadableActiveManifest(t *testing.T) {
	previousDB := model.DB
	t.Cleanup(func() {
		model.DB = previousDB
	})
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	require.NoError(t, db.AutoMigrate(&model.RequestCaptureRecord{}, &model.RequestCaptureArtifact{}))

	spoolDir := t.TempDir()
	requestId := "req-recover-corrupt-active"
	sessionDir := filepath.Join(spoolDir, requestCaptureSpoolStatusActive, requestId)
	require.NoError(t, os.MkdirAll(sessionDir, 0700))
	require.NoError(t, os.WriteFile(filepath.Join(sessionDir, requestCaptureSpoolManifestName), []byte("{broken"), 0600))
	require.NoError(t, db.Create(&model.RequestCaptureRecord{
		RequestId:     requestId,
		CaptureLevel:  model.RequestCaptureLevelFullBundle,
		CaptureStatus: model.RequestCaptureStatusSpooling,
		SpoolDir:      sessionDir,
	}).Error)

	summary, err := RecoverStaleRequestCaptureSpool(context.Background(), RequestCaptureSpoolRecoveryOptions{
		SpoolDir: spoolDir,
		Now:      func() int64 { return 2000 },
	})
	require.NoError(t, err)
	assert.Equal(t, 1, summary.ActiveRecovered)
	require.NotEmpty(t, summary.Errors)
	assert.Contains(t, summary.Errors[0], "moved to failed spool")

	_, err = os.Stat(sessionDir)
	assert.True(t, os.IsNotExist(err))
	failedDir := filepath.Join(spoolDir, requestCaptureSpoolStatusFailed, requestId)
	manifest, err := readRequestCaptureSpoolManifest(failedDir)
	require.NoError(t, err)
	assert.Equal(t, requestCaptureSpoolStatusFailed, manifest.Status)
	assert.Contains(t, manifest.Error, "manifest unreadable")

	var record model.RequestCaptureRecord
	require.NoError(t, db.Where("request_id = ?", requestId).First(&record).Error)
	assert.Equal(t, model.RequestCaptureStatusFailed, record.CaptureStatus)
	assert.True(t, record.HasError)
	assert.Contains(t, record.LastError, "manifest unreadable")
	assert.Equal(t, failedDir, record.SpoolDir)
}

func TestRecoverStaleRequestCaptureSpoolKeepsFinalizeBackoffError(t *testing.T) {
	previousDB := model.DB
	t.Cleanup(func() {
		model.DB = previousDB
	})
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	require.NoError(t, db.AutoMigrate(&model.RequestCaptureRecord{}, &model.RequestCaptureArtifact{}))

	spoolDir := t.TempDir()
	requestId := "req-recover-finalize-backoff"
	require.NoError(t, db.Create(&model.RequestCaptureRecord{
		RequestId:        requestId,
		CaptureLevel:     model.RequestCaptureLevelFullBundle,
		CaptureStatus:    model.RequestCaptureStatusFinalizing,
		ModelName:        "qwen-plus",
		RequestPath:      "/v1/chat/completions",
		HasError:         true,
		ErrorCode:        "request_capture_finalize_failed",
		LastError:        "previous object storage outage",
		FinalizeAttempts: 2,
		NextFinalizeAt:   2000,
	}).Error)
	session, err := NewRequestCaptureSession(RequestCaptureSessionOptions{
		RequestId:    requestId,
		CaptureLevel: model.RequestCaptureLevelFullBundle,
		SpoolDir:     spoolDir,
		ModelName:    "qwen-plus",
		RequestPath:  "/v1/chat/completions",
		Now: func() time.Time {
			return time.Unix(900, 0)
		},
	})
	require.NoError(t, err)
	require.NoError(t, session.WriteArtifact("client_request.json", "application/json", []byte(`{"model":"qwen-plus"}`)))
	require.NoError(t, session.Finish())

	summary, err := RecoverStaleRequestCaptureSpool(context.Background(), RequestCaptureSpoolRecoveryOptions{
		SpoolDir: spoolDir,
		Now:      func() int64 { return 1000 },
	})
	require.NoError(t, err)
	require.Equal(t, 1, summary.FinalizeSynced)
	require.Empty(t, summary.Errors)

	var record model.RequestCaptureRecord
	require.NoError(t, db.Where("request_id = ?", requestId).First(&record).Error)
	require.Equal(t, model.RequestCaptureStatusFinalizing, record.CaptureStatus)
	require.True(t, record.HasError)
	require.Equal(t, "request_capture_finalize_failed", record.ErrorCode)
	require.Equal(t, "previous object storage outage", record.LastError)
	require.Equal(t, 2, record.FinalizeAttempts)
	require.Equal(t, int64(2000), record.NextFinalizeAt)
	require.Contains(t, record.SpoolDir, requestCaptureSpoolStatusFinalize)
}

func decryptAndDecompressRequestCaptureBundleForTest(t *testing.T, encrypted []byte, key []byte) []byte {
	t.Helper()
	require.True(t, bytes.HasPrefix(encrypted, []byte(requestCaptureBundleMagic)))
	encrypted = encrypted[len(requestCaptureBundleMagic):]
	block, err := aes.NewCipher(key)
	require.NoError(t, err)
	gcm, err := cipher.NewGCM(block)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(encrypted), gcm.NonceSize())
	nonce := encrypted[:gcm.NonceSize()]
	ciphertext := encrypted[gcm.NonceSize():]
	compressed, err := gcm.Open(nil, nonce, ciphertext, nil)
	require.NoError(t, err)
	decoder, err := zstd.NewReader(bytes.NewReader(compressed))
	require.NoError(t, err)
	defer decoder.Close()
	body, err := io.ReadAll(decoder)
	require.NoError(t, err)
	return body
}

func readTarFilesForTest(t *testing.T, body []byte) map[string][]byte {
	t.Helper()
	files := map[string][]byte{}
	reader := tar.NewReader(bytes.NewReader(body))
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		content, err := io.ReadAll(reader)
		require.NoError(t, err)
		files[header.Name] = content
	}
	return files
}
