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
	requestCaptureFinalizerEnabledEnv         = "CAPTURE_FINALIZER_ENABLED"
	requestCaptureFinalizerIntervalSecondsEnv = "CAPTURE_FINALIZER_INTERVAL_SECONDS"
	requestCaptureFinalizerLimitEnv           = "CAPTURE_FINALIZER_LIMIT"
	requestCaptureFinalizerRemoveOnSuccessEnv = "CAPTURE_FINALIZER_REMOVE_ON_SUCCESS"

	requestCaptureFinalizerDefaultIntervalSeconds = 60
	requestCaptureFinalizerDefaultLimit           = 100
)

var (
	requestCaptureFinalizerOnce    sync.Once
	requestCaptureFinalizerRunning atomic.Bool
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

func RequestCaptureFinalizerWorkerOptionsFromEnv() RequestCaptureFinalizerWorkerOptions {
	return RequestCaptureFinalizerWorkerOptions{
		SpoolDir:        requestCaptureEnvString(requestCaptureSpoolDirEnv, requestCaptureDefaultSpoolDir),
		Limit:           RequestCaptureFinalizerLimit(),
		RemoveOnSuccess: requestCaptureEnvBool(requestCaptureFinalizerRemoveOnSuccessEnv, true),
	}
}

func runRequestCaptureFinalizerOnce() {
	if !requestCaptureFinalizerRunning.CompareAndSwap(false, true) {
		return
	}
	defer requestCaptureFinalizerRunning.Store(false)

	summary, err := FinalizePendingRequestCaptureSpool(context.Background(), RequestCaptureFinalizerWorkerOptionsFromEnv())
	if err != nil {
		logger.LogWarn(context.Background(), fmt.Sprintf("request capture finalizer task failed: %v", err))
		return
	}
	if summary.Succeeded > 0 || summary.Failed > 0 || summary.Errors != nil {
		logger.LogInfo(context.Background(), fmt.Sprintf("request capture finalizer: scanned=%d succeeded=%d failed=%d skipped=%d errors=%d", summary.Scanned, summary.Succeeded, summary.Failed, summary.Skipped, len(summary.Errors)))
	}
}
