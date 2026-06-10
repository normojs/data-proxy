package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	mcpopenapi "github.com/QuantumNous/new-api/pkg/mcp/openapi"

	"github.com/bytedance/gopkg/util/gopool"
)

var (
	mcpOpenAPIBinaryCleanupOnce    sync.Once
	mcpOpenAPIBinaryCleanupRunning atomic.Bool
)

func CleanupMCPOpenAPIBinaryObjectsForAdmin(req dto.MCPOpenAPIBinaryCleanupRequest) (dto.MCPOpenAPIBinaryCleanupResponse, error) {
	ttlSeconds := req.TTLSeconds
	if ttlSeconds <= 0 {
		ttlSeconds = mcpopenapi.BinaryObjectTTLSeconds()
	}
	if ttlSeconds <= 0 {
		return dto.MCPOpenAPIBinaryCleanupResponse{}, errors.New("ttl_seconds is required when OPENAPI_BINARY_OBJECT_TTL_SECONDS is not configured")
	}
	limit := req.Limit
	if limit <= 0 {
		limit = mcpopenapi.BinaryObjectCleanupLimit()
	}
	return cleanupMCPOpenAPIBinaryObjects(context.Background(), ttlSeconds, limit, req.DryRun)
}

func StartMCPOpenAPIBinaryObjectCleanupTask() {
	mcpOpenAPIBinaryCleanupOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		ttlSeconds := mcpopenapi.BinaryObjectTTLSeconds()
		if ttlSeconds <= 0 {
			return
		}
		intervalSeconds := mcpopenapi.BinaryObjectCleanupIntervalSeconds()
		if intervalSeconds <= 0 {
			return
		}
		gopool.Go(func() {
			interval := time.Duration(intervalSeconds) * time.Second
			logger.LogInfo(context.Background(), fmt.Sprintf("OpenAPI binary object cleanup task started: ttl=%ds tick=%s", ttlSeconds, interval))
			ticker := time.NewTicker(interval)
			defer ticker.Stop()

			runMCPOpenAPIBinaryCleanupOnce()
			for range ticker.C {
				runMCPOpenAPIBinaryCleanupOnce()
			}
		})
	})
}

func runMCPOpenAPIBinaryCleanupOnce() {
	if !mcpOpenAPIBinaryCleanupRunning.CompareAndSwap(false, true) {
		return
	}
	defer mcpOpenAPIBinaryCleanupRunning.Store(false)

	ttlSeconds := mcpopenapi.BinaryObjectTTLSeconds()
	if ttlSeconds <= 0 {
		return
	}
	result, err := cleanupMCPOpenAPIBinaryObjects(context.Background(), ttlSeconds, mcpopenapi.BinaryObjectCleanupLimit(), false)
	if err != nil {
		logger.LogWarn(context.Background(), fmt.Sprintf("OpenAPI binary object cleanup failed: %v", err))
		return
	}
	if result.Deleted > 0 || len(result.Errors) > 0 {
		logger.LogInfo(context.Background(), fmt.Sprintf("OpenAPI binary object cleanup: provider=%s scanned=%d deleted=%d bytes=%d errors=%d", result.Provider, result.Scanned, result.Deleted, result.DeletedBytes, len(result.Errors)))
	}
}

func cleanupMCPOpenAPIBinaryObjects(ctx context.Context, ttlSeconds int64, limit int, dryRun bool) (dto.MCPOpenAPIBinaryCleanupResponse, error) {
	if ttlSeconds <= 0 {
		return dto.MCPOpenAPIBinaryCleanupResponse{}, errors.New("ttl_seconds must be greater than 0")
	}
	cutoff := common.GetTimestamp() - ttlSeconds
	result, err := mcpopenapi.CleanupBinaryObjects(ctx, mcpopenapi.BinaryObjectCleanupOptions{
		CutoffUnix: cutoff,
		Limit:      limit,
		DryRun:     dryRun,
	})
	if err != nil {
		return dto.MCPOpenAPIBinaryCleanupResponse{}, err
	}
	return dto.MCPOpenAPIBinaryCleanupResponse{
		Provider:     result.Provider,
		TTLSeconds:   ttlSeconds,
		CutoffTime:   result.CutoffUnix,
		DryRun:       result.DryRun,
		Scanned:      result.Scanned,
		Deleted:      result.Deleted,
		DeletedBytes: result.DeletedBytes,
		Errors:       result.Errors,
	}, nil
}
