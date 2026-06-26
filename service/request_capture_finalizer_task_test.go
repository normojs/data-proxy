package service

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestRequestCaptureFinalizerTaskOptionsFromEnv(t *testing.T) {
	t.Setenv("CAPTURE_FINALIZER_INTERVAL_SECONDS", "12")
	t.Setenv("CAPTURE_FINALIZER_LIMIT", "34")
	t.Setenv("CAPTURE_FINALIZER_REMOVE_ON_SUCCESS", "false")
	t.Setenv("CAPTURE_FINALIZER_RETRY_BASE_SECONDS", "45")
	t.Setenv("CAPTURE_FINALIZER_RETRY_MAX_SECONDS", "600")
	t.Setenv("CAPTURE_SPOOL_ACTIVE_STALE_SECONDS", "120")
	t.Setenv("CAPTURE_SPOOL_DIR", "/capture/spool")

	assert.Equal(t, 12, RequestCaptureFinalizerIntervalSeconds())
	assert.Equal(t, 34, RequestCaptureFinalizerLimit())
	assert.Equal(t, 45, RequestCaptureFinalizerRetryBaseSeconds())
	assert.Equal(t, 600, RequestCaptureFinalizerRetryMaxSeconds())
	assert.Equal(t, int64(120), RequestCaptureSpoolActiveStaleSeconds())

	options := RequestCaptureFinalizerWorkerOptionsFromEnv()
	assert.Equal(t, "/capture/spool", options.SpoolDir)
	assert.Equal(t, 34, options.Limit)
	assert.False(t, options.RemoveOnSuccess)
	assert.Equal(t, 45, options.RetryBaseSeconds)
	assert.Equal(t, 600, options.RetryMaxSeconds)

	recoveryOptions := RequestCaptureSpoolRecoveryOptionsFromEnv()
	assert.Equal(t, "/capture/spool", recoveryOptions.SpoolDir)
	assert.Equal(t, int64(120), recoveryOptions.ActiveStaleSeconds)
}

func TestRequestCaptureFinalizerTaskEnabledFollowsCaptureStorage(t *testing.T) {
	assert.False(t, RequestCaptureFinalizerTaskEnabled())

	t.Setenv("CAPTURE_ENABLED", "true")
	t.Setenv("CAPTURE_OBJECT_BACKEND", "s3")
	assert.True(t, RequestCaptureFinalizerTaskEnabled())

	t.Setenv("CAPTURE_FINALIZER_ENABLED", "false")
	assert.False(t, RequestCaptureFinalizerTaskEnabled())
}

func TestRunRequestCaptureFinalizerOnceSkipsConcurrentRun(t *testing.T) {
	previousFinalizePending := requestCaptureFinalizePending
	requestCaptureFinalizerRunning.Store(false)
	t.Cleanup(func() {
		requestCaptureFinalizePending = previousFinalizePending
		requestCaptureFinalizerRunning.Store(false)
	})

	entered := make(chan struct{})
	release := make(chan struct{})
	done := make(chan struct{})
	var calls atomic.Int32

	requestCaptureFinalizePending = func(ctx context.Context, options RequestCaptureFinalizerWorkerOptions) (RequestCaptureFinalizerWorkerSummary, error) {
		if calls.Add(1) == 1 {
			close(entered)
			<-release
		}
		return RequestCaptureFinalizerWorkerSummary{}, nil
	}

	go func() {
		defer close(done)
		runRequestCaptureFinalizerOnce()
	}()

	select {
	case <-entered:
	case <-done:
		t.Fatal("finalizer returned before entering the worker")
	}

	runRequestCaptureFinalizerOnce()
	assert.EqualValues(t, 1, calls.Load())

	close(release)
	<-done

	runRequestCaptureFinalizerOnce()
	assert.EqualValues(t, 2, calls.Load())
}

func TestStartRequestCaptureFinalizerTaskRecoversStaleActiveSpoolOnStartup(t *testing.T) {
	previousMasterNode := common.IsMasterNode
	previousFinalizePending := requestCaptureFinalizePending
	previousFinalizerOnce := requestCaptureFinalizerOnce
	requestCaptureFinalizerRunning.Store(false)
	requestCaptureFinalizerOnce = sync.Once{}
	t.Cleanup(func() {
		common.IsMasterNode = previousMasterNode
		requestCaptureFinalizePending = previousFinalizePending
		requestCaptureFinalizerRunning.Store(false)
		requestCaptureFinalizerOnce = previousFinalizerOnce
	})

	previousDB := model.DB
	t.Cleanup(func() {
		model.DB = previousDB
	})
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	require.NoError(t, db.AutoMigrate(&model.RequestCaptureRecord{}, &model.RequestCaptureArtifact{}))

	spoolDir := t.TempDir()
	oldTime := time.Now().Add(-2 * time.Hour)
	t.Setenv("CAPTURE_ENABLED", "true")
	t.Setenv("CAPTURE_OBJECT_BACKEND", "s3")
	t.Setenv("CAPTURE_FINALIZER_INTERVAL_SECONDS", "3600")
	t.Setenv("CAPTURE_SPOOL_ACTIVE_STALE_SECONDS", "1")
	t.Setenv("CAPTURE_SPOOL_DIR", spoolDir)
	common.IsMasterNode = true

	requestId := "req-finalizer-startup-recover"
	require.NoError(t, db.Create(&model.RequestCaptureRecord{
		RequestId:     requestId,
		CaptureLevel:  model.RequestCaptureLevelFullBundle,
		CaptureStatus: model.RequestCaptureStatusSpooling,
		SpoolDir:      filepath.Join(spoolDir, requestCaptureSpoolStatusActive, requestId),
	}).Error)
	session, err := NewRequestCaptureSession(RequestCaptureSessionOptions{
		RequestId:    requestId,
		CaptureLevel: model.RequestCaptureLevelFullBundle,
		SpoolDir:     spoolDir,
		Now: func() time.Time {
			return oldTime
		},
	})
	require.NoError(t, err)
	_ = session

	var finalizerCalls atomic.Int32
	requestCaptureFinalizePending = func(ctx context.Context, options RequestCaptureFinalizerWorkerOptions) (RequestCaptureFinalizerWorkerSummary, error) {
		finalizerCalls.Add(1)
		return RequestCaptureFinalizerWorkerSummary{}, nil
	}

	StartRequestCaptureFinalizerTask()

	require.Eventually(t, func() bool {
		_, err := os.Stat(filepath.Join(spoolDir, requestCaptureSpoolStatusActive, requestId))
		return os.IsNotExist(err)
	}, 3*time.Second, 20*time.Millisecond)
	require.Eventually(t, func() bool {
		return finalizerCalls.Load() == 1
	}, 3*time.Second, 20*time.Millisecond)

	manifest := readRequestCaptureSpoolManifestForTest(t, filepath.Join(spoolDir, requestCaptureSpoolStatusFailed, requestId))
	assert.Equal(t, requestCaptureSpoolStatusFailed, manifest.Status)
	assert.Contains(t, manifest.Error, "recovered stale active request capture session")

	var record model.RequestCaptureRecord
	require.NoError(t, db.Where("request_id = ?", requestId).First(&record).Error)
	assert.Equal(t, model.RequestCaptureStatusFailed, record.CaptureStatus)
	assert.Equal(t, filepath.Join(spoolDir, requestCaptureSpoolStatusFailed, requestId), record.SpoolDir)
}

func TestRequestCaptureFinalizeRetryDelaySecondsCapsAtMax(t *testing.T) {
	require.EqualValues(t, 10, requestCaptureFinalizeRetryDelaySeconds(1, 10, 35))
	require.EqualValues(t, 20, requestCaptureFinalizeRetryDelaySeconds(2, 10, 35))
	require.EqualValues(t, 35, requestCaptureFinalizeRetryDelaySeconds(4, 10, 35))
}
