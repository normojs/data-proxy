package service

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/bytedance/gopkg/util/gopool"
)

const (
	requestCaptureFinalizerEnabledEnv          = "CAPTURE_FINALIZER_ENABLED"
	requestCaptureFinalizerIntervalSecondsEnv  = "CAPTURE_FINALIZER_INTERVAL_SECONDS"
	requestCaptureFinalizerLimitEnv            = "CAPTURE_FINALIZER_LIMIT"
	requestCaptureFinalizerRemoveOnSuccessEnv  = "CAPTURE_FINALIZER_REMOVE_ON_SUCCESS"
	requestCaptureFinalizerRetryBaseSecondsEnv = "CAPTURE_FINALIZER_RETRY_BASE_SECONDS"
	requestCaptureFinalizerRetryMaxSecondsEnv  = "CAPTURE_FINALIZER_RETRY_MAX_SECONDS"
	requestCaptureSpoolActiveStaleSecondsEnv   = "CAPTURE_SPOOL_ACTIVE_STALE_SECONDS"

	requestCaptureFinalizerDefaultIntervalSeconds  = 60
	requestCaptureFinalizerDefaultLimit            = 100
	requestCaptureFinalizerDefaultRetryBaseSeconds = 60
	requestCaptureFinalizerDefaultRetryMaxSeconds  = 3600
)

var (
	requestCaptureFinalizerOnce    sync.Once
	requestCaptureFinalizerRunning atomic.Bool
	requestCaptureFinalizePending  = FinalizePendingRequestCaptureSpool
)

func StartRequestCaptureFinalizerTask() {
	requestCaptureFinalizerOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		if !RequestCaptureFinalizerTaskEnabled() {
			return
		}
		intervalSeconds := RequestCaptureFinalizerIntervalSeconds()
		if intervalSeconds <= 0 {
			return
		}
		gopool.Go(func() {
			interval := time.Duration(intervalSeconds) * time.Second
			logger.LogInfo(context.Background(), fmt.Sprintf("request capture finalizer task started: tick=%s", interval))
			ticker := time.NewTicker(interval)
			defer ticker.Stop()

			runRequestCaptureSpoolRecoveryOnce()
			runRequestCaptureFinalizerOnce()
			for range ticker.C {
				runRequestCaptureFinalizerOnce()
			}
		})
	})
}

func RequestCaptureFinalizerTaskEnabled() bool {
	return requestCaptureEnvBool(requestCaptureFinalizerEnabledEnv, RequestCaptureObjectStorageEnabled())
}

func RequestCaptureFinalizerIntervalSeconds() int {
	return common.GetEnvOrDefault(requestCaptureFinalizerIntervalSecondsEnv, requestCaptureFinalizerDefaultIntervalSeconds)
}

func RequestCaptureFinalizerLimit() int {
	limit := common.GetEnvOrDefault(requestCaptureFinalizerLimitEnv, requestCaptureFinalizerDefaultLimit)
	if limit <= 0 {
		return requestCaptureFinalizerDefaultLimit
	}
	return limit
}

func RequestCaptureFinalizerRetryBaseSeconds() int {
	value := common.GetEnvOrDefault(requestCaptureFinalizerRetryBaseSecondsEnv, requestCaptureFinalizerDefaultRetryBaseSeconds)
	if value <= 0 {
		return requestCaptureFinalizerDefaultRetryBaseSeconds
	}
	return value
}

func RequestCaptureFinalizerRetryMaxSeconds() int {
	value := common.GetEnvOrDefault(requestCaptureFinalizerRetryMaxSecondsEnv, requestCaptureFinalizerDefaultRetryMaxSeconds)
	if value <= 0 {
		return requestCaptureFinalizerDefaultRetryMaxSeconds
	}
	return value
}

func RequestCaptureSpoolActiveStaleSeconds() int64 {
	return int64(common.GetEnvOrDefault(requestCaptureSpoolActiveStaleSecondsEnv, 0))
}

func RequestCaptureFinalizerWorkerOptionsFromEnv() RequestCaptureFinalizerWorkerOptions {
	return RequestCaptureFinalizerWorkerOptions{
		SpoolDir:         requestCaptureEnvString(requestCaptureSpoolDirEnv, requestCaptureDefaultSpoolDir),
		Limit:            RequestCaptureFinalizerLimit(),
		RemoveOnSuccess:  requestCaptureEnvBool(requestCaptureFinalizerRemoveOnSuccessEnv, true),
		RetryBaseSeconds: RequestCaptureFinalizerRetryBaseSeconds(),
		RetryMaxSeconds:  RequestCaptureFinalizerRetryMaxSeconds(),
	}
}

func RequestCaptureSpoolRecoveryOptionsFromEnv() RequestCaptureSpoolRecoveryOptions {
	return RequestCaptureSpoolRecoveryOptions{
		SpoolDir:           requestCaptureEnvString(requestCaptureSpoolDirEnv, requestCaptureDefaultSpoolDir),
		ActiveStaleSeconds: RequestCaptureSpoolActiveStaleSeconds(),
	}
}

func runRequestCaptureSpoolRecoveryOnce() {
	summary, err := RecoverStaleRequestCaptureSpool(context.Background(), RequestCaptureSpoolRecoveryOptionsFromEnv())
	if err != nil {
		logger.LogWarn(context.Background(), fmt.Sprintf("request capture spool recovery failed: %v", err))
		return
	}
	if summary.ActiveRecovered > 0 || summary.FinalizeSynced > 0 || summary.FailedSynced > 0 || len(summary.Errors) > 0 {
		logger.LogInfo(context.Background(), fmt.Sprintf("request capture spool recovery: active_recovered=%d finalize_synced=%d failed_synced=%d skipped=%d errors=%d", summary.ActiveRecovered, summary.FinalizeSynced, summary.FailedSynced, summary.Skipped, len(summary.Errors)))
	}
}

func runRequestCaptureFinalizerOnce() {
	if !requestCaptureFinalizerRunning.CompareAndSwap(false, true) {
		return
	}
	defer requestCaptureFinalizerRunning.Store(false)

	summary, err := requestCaptureFinalizePending(context.Background(), RequestCaptureFinalizerWorkerOptionsFromEnv())
	if err != nil {
		logger.LogWarn(context.Background(), fmt.Sprintf("request capture finalizer task failed: %v", err))
		return
	}
	if summary.Succeeded > 0 || summary.Failed > 0 || summary.Errors != nil {
		logger.LogInfo(context.Background(), fmt.Sprintf("request capture finalizer: scanned=%d succeeded=%d failed=%d skipped=%d errors=%d", summary.Scanned, summary.Succeeded, summary.Failed, summary.Skipped, len(summary.Errors)))
	}
}
