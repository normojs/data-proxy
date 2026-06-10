package service

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"

	"github.com/bytedance/gopkg/util/gopool"
)

const (
	mcpProxyHealthCheckTickInterval           = time.Minute
	mcpProxyHeartbeatTickInterval             = 30 * time.Second
	defaultMCPProxyHealthCheckIntervalMinutes = 30
	defaultMCPProxyHealthCheckLimit           = 20
	defaultMCPProxyHealthCheckStaleSeconds    = 15 * 60
	defaultMCPProxyHealthCheckMaxDiscover     = 200
	defaultMCPProxyHealthCheckLeaseSeconds    = 10 * 60
	defaultMCPProxyHeartbeatIntervalSeconds   = 5 * 60
	defaultMCPProxyHeartbeatLimit             = 50
	defaultMCPProxyHeartbeatActiveWindow      = 30 * 60
	defaultMCPProxyHeartbeatTimeoutSeconds    = 10
	defaultMCPProxyHeartbeatLeaseSeconds      = 2 * 60

	mcpProxyHealthCheckOptionPrefix          = "MCPProxyHealthCheck"
	mcpProxyHealthCheckOptionEnabled         = mcpProxyHealthCheckOptionPrefix + "Enabled"
	mcpProxyHealthCheckOptionIntervalMinutes = mcpProxyHealthCheckOptionPrefix + "IntervalMinutes"
	mcpProxyHealthCheckOptionLimit           = mcpProxyHealthCheckOptionPrefix + "Limit"
	mcpProxyHealthCheckOptionStaleSeconds    = mcpProxyHealthCheckOptionPrefix + "StaleSeconds"
	mcpProxyHealthCheckOptionDiscover        = mcpProxyHealthCheckOptionPrefix + "Discover"
	mcpProxyHealthCheckOptionMaxDiscover     = mcpProxyHealthCheckOptionPrefix + "MaxDiscoverTools"
	mcpProxyHealthCheckOptionLastRunAt       = mcpProxyHealthCheckOptionPrefix + "LastRunAt"
	mcpProxyHealthCheckOptionLastRunStatus   = mcpProxyHealthCheckOptionPrefix + "LastRunStatus"
	mcpProxyHealthCheckOptionLastRunMessage  = mcpProxyHealthCheckOptionPrefix + "LastRunMessage"
	mcpProxyHealthCheckOptionLastRunResult   = mcpProxyHealthCheckOptionPrefix + "LastRunResult"
	mcpProxyHealthCheckOptionLeaseOwner      = mcpProxyHealthCheckOptionPrefix + "LeaseOwner"
	mcpProxyHealthCheckOptionLeaseUntil      = mcpProxyHealthCheckOptionPrefix + "LeaseUntil"

	mcpProxyHeartbeatOptionPrefix              = "MCPProxyHeartbeat"
	mcpProxyHeartbeatOptionEnabled             = mcpProxyHeartbeatOptionPrefix + "Enabled"
	mcpProxyHeartbeatOptionIntervalSeconds     = mcpProxyHeartbeatOptionPrefix + "IntervalSeconds"
	mcpProxyHeartbeatOptionLimit               = mcpProxyHeartbeatOptionPrefix + "Limit"
	mcpProxyHeartbeatOptionActiveWindowSeconds = mcpProxyHeartbeatOptionPrefix + "ActiveWindowSeconds"
	mcpProxyHeartbeatOptionTimeoutSeconds      = mcpProxyHeartbeatOptionPrefix + "TimeoutSeconds"
	mcpProxyHeartbeatOptionLastRunAt           = mcpProxyHeartbeatOptionPrefix + "LastRunAt"
	mcpProxyHeartbeatOptionLastRunStatus       = mcpProxyHeartbeatOptionPrefix + "LastRunStatus"
	mcpProxyHeartbeatOptionLastRunMessage      = mcpProxyHeartbeatOptionPrefix + "LastRunMessage"
	mcpProxyHeartbeatOptionLastRunResult       = mcpProxyHeartbeatOptionPrefix + "LastRunResult"
	mcpProxyHeartbeatOptionLeaseOwner          = mcpProxyHeartbeatOptionPrefix + "LeaseOwner"
	mcpProxyHeartbeatOptionLeaseUntil          = mcpProxyHeartbeatOptionPrefix + "LeaseUntil"
)

var (
	mcpProxyHealthCheckOnce    sync.Once
	mcpProxyHealthCheckRunning atomic.Bool
	mcpProxyHeartbeatOnce      sync.Once
	mcpProxyHeartbeatRunning   atomic.Bool
)

func GetMCPProxyHealthCheckStatus() dto.MCPProxyHealthCheckStatusResponse {
	status := dto.MCPProxyHealthCheckStatusResponse{
		Settings:       GetMCPProxyHealthCheckSettings(),
		Running:        mcpProxyHealthCheckRunning.Load(),
		LastRunAt:      mcpProxyHealthCheckOptionInt64(mcpProxyHealthCheckOptionLastRunAt, 0),
		LastRunStatus:  mcpProxyHealthCheckOptionString(mcpProxyHealthCheckOptionLastRunStatus, ""),
		LastRunMessage: mcpProxyHealthCheckOptionString(mcpProxyHealthCheckOptionLastRunMessage, ""),
	}
	lastRun := mcpProxyHealthCheckOptionString(mcpProxyHealthCheckOptionLastRunResult, "")
	if lastRun == "" {
		return status
	}
	var run dto.MCPProxyHealthCheckRunResponse
	if err := common.UnmarshalJsonStr(lastRun, &run); err != nil {
		return status
	}
	status.LastRun = &run
	return status
}

func GetMCPProxyHealthCheckSettings() dto.MCPProxyHealthCheckSettings {
	return dto.MCPProxyHealthCheckSettings{
		Enabled:         mcpProxyHealthCheckOptionBool(mcpProxyHealthCheckOptionEnabled, false),
		IntervalMinutes: normalizeMCPProxyHealthCheckInterval(mcpProxyHealthCheckOptionInt(mcpProxyHealthCheckOptionIntervalMinutes, defaultMCPProxyHealthCheckIntervalMinutes)),
		Limit:           normalizeMCPProxyHealthCheckLimit(mcpProxyHealthCheckOptionInt(mcpProxyHealthCheckOptionLimit, defaultMCPProxyHealthCheckLimit)),
		StaleSeconds:    normalizeMCPProxyHealthCheckStaleSeconds(mcpProxyHealthCheckOptionInt64(mcpProxyHealthCheckOptionStaleSeconds, defaultMCPProxyHealthCheckStaleSeconds)),
		Discover:        mcpProxyHealthCheckOptionBool(mcpProxyHealthCheckOptionDiscover, false),
		MaxDiscoverTools: normalizeMCPProxyHealthCheckMaxDiscover(
			mcpProxyHealthCheckOptionInt(mcpProxyHealthCheckOptionMaxDiscover, defaultMCPProxyHealthCheckMaxDiscover),
		),
	}
}

func UpdateMCPProxyHealthCheckSettings(req dto.MCPProxyHealthCheckSettingsRequest) (dto.MCPProxyHealthCheckStatusResponse, error) {
	values := map[string]string{
		mcpProxyHealthCheckOptionEnabled:         strconv.FormatBool(req.Enabled),
		mcpProxyHealthCheckOptionIntervalMinutes: strconv.Itoa(normalizeMCPProxyHealthCheckInterval(req.IntervalMinutes)),
		mcpProxyHealthCheckOptionLimit:           strconv.Itoa(normalizeMCPProxyHealthCheckLimit(req.Limit)),
		mcpProxyHealthCheckOptionStaleSeconds:    strconv.FormatInt(normalizeMCPProxyHealthCheckStaleSeconds(req.StaleSeconds), 10),
		mcpProxyHealthCheckOptionDiscover:        strconv.FormatBool(req.Discover),
		mcpProxyHealthCheckOptionMaxDiscover:     strconv.Itoa(normalizeMCPProxyHealthCheckMaxDiscover(req.MaxDiscoverTools)),
	}
	if err := updateMCPProxyHealthCheckOptions(values); err != nil {
		return dto.MCPProxyHealthCheckStatusResponse{}, err
	}
	return GetMCPProxyHealthCheckStatus(), nil
}

func GetMCPProxyHeartbeatStatus() dto.MCPProxyHeartbeatStatusResponse {
	status := dto.MCPProxyHeartbeatStatusResponse{
		Settings:       GetMCPProxyHeartbeatSettings(),
		Running:        mcpProxyHeartbeatRunning.Load(),
		LastRunAt:      mcpProxyHealthCheckOptionInt64(mcpProxyHeartbeatOptionLastRunAt, 0),
		LastRunStatus:  mcpProxyHealthCheckOptionString(mcpProxyHeartbeatOptionLastRunStatus, ""),
		LastRunMessage: mcpProxyHealthCheckOptionString(mcpProxyHeartbeatOptionLastRunMessage, ""),
	}
	lastRun := mcpProxyHealthCheckOptionString(mcpProxyHeartbeatOptionLastRunResult, "")
	if lastRun == "" {
		return status
	}
	var run dto.MCPProxyHeartbeatRunResponse
	if err := common.UnmarshalJsonStr(lastRun, &run); err != nil {
		return status
	}
	status.LastRun = &run
	return status
}

func GetMCPProxyHeartbeatSettings() dto.MCPProxyHeartbeatSettings {
	return dto.MCPProxyHeartbeatSettings{
		Enabled:             mcpProxyHealthCheckOptionBool(mcpProxyHeartbeatOptionEnabled, false),
		IntervalSeconds:     normalizeMCPProxyHeartbeatInterval(mcpProxyHealthCheckOptionInt64(mcpProxyHeartbeatOptionIntervalSeconds, defaultMCPProxyHeartbeatIntervalSeconds)),
		Limit:               normalizeMCPProxyHeartbeatLimit(mcpProxyHealthCheckOptionInt(mcpProxyHeartbeatOptionLimit, defaultMCPProxyHeartbeatLimit)),
		ActiveWindowSeconds: normalizeMCPProxyHeartbeatActiveWindow(mcpProxyHealthCheckOptionInt64(mcpProxyHeartbeatOptionActiveWindowSeconds, defaultMCPProxyHeartbeatActiveWindow)),
		TimeoutSeconds:      normalizeMCPProxyHeartbeatTimeout(mcpProxyHealthCheckOptionInt64(mcpProxyHeartbeatOptionTimeoutSeconds, defaultMCPProxyHeartbeatTimeoutSeconds)),
	}
}

func UpdateMCPProxyHeartbeatSettings(req dto.MCPProxyHeartbeatSettingsRequest) (dto.MCPProxyHeartbeatStatusResponse, error) {
	values := map[string]string{
		mcpProxyHeartbeatOptionEnabled:             strconv.FormatBool(req.Enabled),
		mcpProxyHeartbeatOptionIntervalSeconds:     strconv.FormatInt(normalizeMCPProxyHeartbeatInterval(req.IntervalSeconds), 10),
		mcpProxyHeartbeatOptionLimit:               strconv.Itoa(normalizeMCPProxyHeartbeatLimit(req.Limit)),
		mcpProxyHeartbeatOptionActiveWindowSeconds: strconv.FormatInt(normalizeMCPProxyHeartbeatActiveWindow(req.ActiveWindowSeconds), 10),
		mcpProxyHeartbeatOptionTimeoutSeconds:      strconv.FormatInt(normalizeMCPProxyHeartbeatTimeout(req.TimeoutSeconds), 10),
	}
	if err := updateMCPProxyHealthCheckOptions(values); err != nil {
		return dto.MCPProxyHeartbeatStatusResponse{}, err
	}
	return GetMCPProxyHeartbeatStatus(), nil
}

func RunMCPProxyHeartbeatOnce(ctx context.Context, manual bool) (dto.MCPProxyHeartbeatRunResponse, error) {
	settings := GetMCPProxyHeartbeatSettings()
	if !mcpProxyHeartbeatRunning.CompareAndSwap(false, true) {
		return dto.MCPProxyHeartbeatRunResponse{
			Manual:   manual,
			Status:   "running",
			Message:  "MCP proxy heartbeat is already running",
			Settings: settings,
			Items:    []dto.MCPProxyHeartbeatRunItem{},
		}, nil
	}
	defer mcpProxyHeartbeatRunning.Store(false)

	leaseOwner, acquired, err := acquireMCPProxyHeartbeatLease()
	if err != nil {
		return dto.MCPProxyHeartbeatRunResponse{}, err
	}
	if !acquired {
		return dto.MCPProxyHeartbeatRunResponse{
			Manual:   manual,
			Status:   "running",
			Message:  "MCP proxy heartbeat is running on another instance",
			Settings: settings,
			Items:    []dto.MCPProxyHeartbeatRunItem{},
		}, nil
	}
	defer releaseMCPProxyHeartbeatLease(leaseOwner)

	run := dto.MCPProxyHeartbeatRunResponse{
		Manual:    manual,
		Status:    "success",
		Settings:  settings,
		CheckedAt: common.GetTimestamp(),
		Items:     []dto.MCPProxyHeartbeatRunItem{},
	}
	servers, err := model.ListMCPProxyServersForHeartbeat(settings.Limit)
	if err != nil {
		run.Status = "failed"
		run.Message = err.Error()
		_ = persistMCPProxyHeartbeatRun(&run)
		return dto.MCPProxyHeartbeatRunResponse{}, err
	}
	run.ScannedCount = len(servers)
	for _, server := range servers {
		item := dto.MCPProxyHeartbeatRunItem{
			ProxyServerId: server.Id,
			Name:          server.Name,
			Namespace:     server.Namespace,
			Transport:     server.Transport,
		}
		transport := mcpProxyTransportHealth(server)
		item.LastActivityAt = transport.LastActivityAt
		if !transport.Observable {
			item.Action = "skip"
			item.SkippedReason = "transport_not_observable"
			run.SkippedCount++
			run.Items = append(run.Items, item)
			continue
		}
		if !transport.HasSession {
			item.Action = "skip"
			item.SkippedReason = "no_active_session"
			run.SkippedCount++
			run.Items = append(run.Items, item)
			continue
		}
		if settings.ActiveWindowSeconds > 0 && item.LastActivityAt > 0 && run.CheckedAt-item.LastActivityAt > settings.ActiveWindowSeconds {
			item.Action = "skip"
			item.SkippedReason = "session_outside_active_window"
			run.SkippedCount++
			run.Items = append(run.Items, item)
			continue
		}

		item.Action = "heartbeat"
		pingCtx, cancel := context.WithTimeout(ctxOrBackground(ctx), time.Duration(settings.TimeoutSeconds)*time.Second)
		_, err := defaultMCPProxyClient.Test(pingCtx, server)
		cancel()
		run.PingedCount++
		if err != nil {
			item.Error = err.Error()
			run.ErrorCount++
			_, _ = model.UpdateMCPProxyServerFields(server.Id, map[string]any{
				"status":     model.MCPProxyServerStatusError,
				"last_error": truncateMCPString(err.Error(), 512),
				"updated_at": common.GetTimestamp(),
			})
		} else {
			run.SuccessCount++
			_, _ = markMCPProxyServerHealthy(server)
		}
		run.Items = append(run.Items, item)
	}
	run.Status = mcpProxyHeartbeatRunStatus(run)
	run.Message = mcpProxyHeartbeatRunMessage(run)
	if err := persistMCPProxyHeartbeatRun(&run); err != nil {
		return dto.MCPProxyHeartbeatRunResponse{}, err
	}
	return run, nil
}

func RunMCPProxyHealthCheckOnce(ctx context.Context, manual bool, req dto.MCPProxyHealthCheckRunRequest) (dto.MCPProxyHealthCheckRunResponse, error) {
	settings := mcpProxyHealthCheckSettingsWithOverrides(GetMCPProxyHealthCheckSettings(), req)
	if !mcpProxyHealthCheckRunning.CompareAndSwap(false, true) {
		return dto.MCPProxyHealthCheckRunResponse{
			Manual:   manual,
			DryRun:   req.DryRun,
			Status:   "running",
			Message:  "MCP proxy health check is already running",
			Settings: settings,
			Items:    []dto.MCPProxyHealthCheckRunItem{},
		}, nil
	}
	defer mcpProxyHealthCheckRunning.Store(false)
	leaseOwner, acquired, err := acquireMCPProxyHealthCheckLease()
	if err != nil {
		return dto.MCPProxyHealthCheckRunResponse{}, err
	}
	if !acquired {
		return dto.MCPProxyHealthCheckRunResponse{
			Manual:   manual,
			DryRun:   req.DryRun,
			Status:   "running",
			Message:  "MCP proxy health check is running on another instance",
			Settings: settings,
			Items:    []dto.MCPProxyHealthCheckRunItem{},
		}, nil
	}
	defer releaseMCPProxyHealthCheckLease(leaseOwner)

	now := common.GetTimestamp()
	run := dto.MCPProxyHealthCheckRunResponse{
		Manual:          manual,
		DryRun:          req.DryRun,
		Status:          "success",
		Settings:        settings,
		CheckedAt:       now,
		Items:           []dto.MCPProxyHealthCheckRunItem{},
		LatestEventById: map[int]dto.MCPProxyDiscoveryEventItem{},
	}
	servers, err := model.ListMCPProxyServersForHealthCheck(settings.Limit)
	if err != nil {
		run.Status = "failed"
		run.Message = err.Error()
		_ = persistMCPProxyHealthCheckRun(&run)
		return dto.MCPProxyHealthCheckRunResponse{}, err
	}
	run.ScannedCount = len(servers)
	latestChecks, err := model.ListLatestMCPProxyDiscoveryEventsByServerIds(mcpProxyServerIds(servers))
	if err != nil {
		run.Status = "failed"
		run.Message = err.Error()
		_ = persistMCPProxyHealthCheckRun(&run)
		return dto.MCPProxyHealthCheckRunResponse{}, err
	}

	for _, server := range servers {
		item := dto.MCPProxyHealthCheckRunItem{
			ProxyServerId: server.Id,
			Name:          server.Name,
			Namespace:     server.Namespace,
			Status:        server.Status,
		}
		if latest, ok := latestChecks[server.Id]; ok {
			converted := mcpProxyDiscoveryEventToDTO(latest)
			item.PreviousCheck = &converted
		}
		if !req.Force && !mcpProxyHealthCheckDue(item.PreviousCheck, settings.StaleSeconds, now) {
			item.Action = "skip"
			item.SkippedReason = "latest_check_is_fresh"
			run.SkippedCount++
			run.Items = append(run.Items, item)
			continue
		}
		if req.DryRun {
			if settings.Discover {
				item.Action = "discover"
			} else {
				item.Action = "test"
			}
			item.SkippedReason = "dry_run"
			run.SkippedCount++
			run.Items = append(run.Items, item)
			continue
		}

		if settings.Discover {
			item.Action = "discover"
			discovery, blocked, err := runMCPProxyHealthCheckDiscover(ctx, server, settings.MaxDiscoverTools)
			if err != nil {
				item.Error = err.Error()
				run.ErrorCount++
			} else if blocked {
				item.Error = "discover blocked by max_discover_tools"
				run.BlockedCount++
			} else {
				item.Discovered = discovery.DiscoveredCount
				item.SchemaChanged = discovery.SchemaChanged
				run.SuccessCount++
				run.DiscoverCount++
			}
		} else {
			item.Action = "test"
			if _, err := TestMCPProxyServerForAdmin(ctx, server.Id); err != nil {
				item.Error = err.Error()
				run.ErrorCount++
			} else {
				run.SuccessCount++
			}
		}
		run.CheckedCount++
		if latest, err := latestMCPProxyDiscoveryEventItem(server.Id); err == nil && latest != nil {
			item.LatestCheck = latest
			run.LatestEventById[server.Id] = *latest
		}
		run.Items = append(run.Items, item)
	}
	run.Status = mcpProxyHealthCheckRunStatus(run)
	run.Message = mcpProxyHealthCheckRunMessage(run)
	if err := persistMCPProxyHealthCheckRun(&run); err != nil {
		return dto.MCPProxyHealthCheckRunResponse{}, err
	}
	return run, nil
}

func StartMCPProxyHealthCheckTask() {
	mcpProxyHealthCheckOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		gopool.Go(func() {
			logger.LogInfo(context.Background(), fmt.Sprintf("MCP proxy health check task started: tick=%s", mcpProxyHealthCheckTickInterval))
			ticker := time.NewTicker(mcpProxyHealthCheckTickInterval)
			defer ticker.Stop()

			runMCPProxyHealthCheckIfDue()
			for range ticker.C {
				runMCPProxyHealthCheckIfDue()
			}
		})
	})
	StartMCPProxyHeartbeatTask()
}

func StartMCPProxyHeartbeatTask() {
	mcpProxyHeartbeatOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		gopool.Go(func() {
			logger.LogInfo(context.Background(), fmt.Sprintf("MCP proxy heartbeat task started: tick=%s", mcpProxyHeartbeatTickInterval))
			ticker := time.NewTicker(mcpProxyHeartbeatTickInterval)
			defer ticker.Stop()

			runMCPProxyHeartbeatIfDue()
			for range ticker.C {
				runMCPProxyHeartbeatIfDue()
			}
		})
	})
}

func runMCPProxyHealthCheckIfDue() {
	settings := GetMCPProxyHealthCheckSettings()
	if !settings.Enabled {
		return
	}
	lastRunAt := mcpProxyHealthCheckOptionInt64(mcpProxyHealthCheckOptionLastRunAt, 0)
	if lastRunAt > 0 {
		nextRunAt := time.Unix(lastRunAt, 0).Add(time.Duration(settings.IntervalMinutes) * time.Minute)
		if time.Now().Before(nextRunAt) {
			return
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(settings.Limit)*time.Minute)
	defer cancel()
	run, err := RunMCPProxyHealthCheckOnce(ctx, false, dto.MCPProxyHealthCheckRunRequest{})
	if err != nil {
		logger.LogWarn(context.Background(), fmt.Sprintf("MCP proxy health check failed: %v", err))
		return
	}
	if run.Status == "running" {
		return
	}
	logger.LogDebug(context.Background(), "MCP proxy health check: %s", run.Message)
}

func runMCPProxyHeartbeatIfDue() {
	settings := GetMCPProxyHeartbeatSettings()
	if !settings.Enabled {
		return
	}
	lastRunAt := mcpProxyHealthCheckOptionInt64(mcpProxyHeartbeatOptionLastRunAt, 0)
	if lastRunAt > 0 {
		nextRunAt := time.Unix(lastRunAt, 0).Add(time.Duration(settings.IntervalSeconds) * time.Second)
		if time.Now().Before(nextRunAt) {
			return
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(settings.TimeoutSeconds)*time.Second*time.Duration(settings.Limit+1))
	defer cancel()
	run, err := RunMCPProxyHeartbeatOnce(ctx, false)
	if err != nil {
		logger.LogWarn(context.Background(), fmt.Sprintf("MCP proxy heartbeat failed: %v", err))
		return
	}
	if run.Status == "running" {
		return
	}
	closeMCPProxyIdleSessions()
	logger.LogDebug(context.Background(), "MCP proxy heartbeat: %s", run.Message)
}

func runMCPProxyHealthCheckDiscover(ctx context.Context, server model.MCPProxyServer, maxDiscoverTools int) (dto.MCPProxyDiscoveryResult, bool, error) {
	startedAt := common.GetTimestamp()
	startedClock := time.Now()
	tools, err := defaultMCPProxyClient.ListTools(ctx, server)
	if err != nil {
		_, _ = model.UpdateMCPProxyServerFields(server.Id, map[string]any{
			"status":     model.MCPProxyServerStatusError,
			"last_error": truncateMCPString(err.Error(), 512),
			"updated_at": common.GetTimestamp(),
		})
		recordMCPProxyDiscoveryEvent(model.MCPProxyDiscoveryEvent{
			ProxyServerId: server.Id,
			EventType:     model.MCPProxyDiscoveryEventTypeDiscover,
			Status:        model.MCPProxyDiscoveryEventStatusError,
			Message:       truncateMCPString(err.Error(), 1024),
			DurationMS:    durationMSSince(startedClock),
			StartedAt:     startedAt,
		})
		return dto.MCPProxyDiscoveryResult{}, false, err
	}
	if maxDiscoverTools > 0 && len(tools) > maxDiscoverTools {
		message := fmt.Sprintf("active discover blocked: discovered=%d exceeds max_discover_tools=%d", len(tools), maxDiscoverTools)
		recordMCPProxyDiscoveryEvent(model.MCPProxyDiscoveryEvent{
			ProxyServerId:   server.Id,
			EventType:       model.MCPProxyDiscoveryEventTypeDiscover,
			Status:          model.MCPProxyDiscoveryEventStatusError,
			Message:         message,
			DiscoveredCount: len(tools),
			DurationMS:      durationMSSince(startedClock),
			StartedAt:       startedAt,
		})
		return dto.MCPProxyDiscoveryResult{}, true, nil
	}
	result, err := syncMCPProxyDiscoveredTools(server, tools)
	if err != nil {
		recordMCPProxyDiscoveryEvent(model.MCPProxyDiscoveryEvent{
			ProxyServerId: server.Id,
			EventType:     model.MCPProxyDiscoveryEventTypeDiscover,
			Status:        model.MCPProxyDiscoveryEventStatusError,
			Message:       truncateMCPString(err.Error(), 1024),
			DurationMS:    durationMSSince(startedClock),
			StartedAt:     startedAt,
		})
		return result, false, err
	}
	recordMCPProxyDiscoveryEvent(mcpProxyDiscoverEvent(server.Id, startedAt, durationMSSince(startedClock), result))
	return result, false, nil
}

func mcpProxyHealthCheckSettingsWithOverrides(settings dto.MCPProxyHealthCheckSettings, req dto.MCPProxyHealthCheckRunRequest) dto.MCPProxyHealthCheckSettings {
	if req.Limit > 0 {
		settings.Limit = normalizeMCPProxyHealthCheckLimit(req.Limit)
	}
	if req.StaleSeconds > 0 {
		settings.StaleSeconds = normalizeMCPProxyHealthCheckStaleSeconds(req.StaleSeconds)
	}
	if req.Discover != nil {
		settings.Discover = *req.Discover
	}
	if req.MaxDiscoverTools > 0 {
		settings.MaxDiscoverTools = normalizeMCPProxyHealthCheckMaxDiscover(req.MaxDiscoverTools)
	}
	return settings
}

func mcpProxyHealthCheckDue(previous *dto.MCPProxyDiscoveryEventItem, staleSeconds int64, now int64) bool {
	if previous == nil {
		return true
	}
	checkedAt := previous.FinishedAt
	if checkedAt == 0 {
		checkedAt = previous.StartedAt
	}
	if checkedAt == 0 {
		checkedAt = previous.CreatedAt
	}
	return now-checkedAt >= staleSeconds
}

func mcpProxyHealthCheckRunStatus(run dto.MCPProxyHealthCheckRunResponse) string {
	if run.ErrorCount > 0 {
		return "failed"
	}
	if run.BlockedCount > 0 {
		return "blocked"
	}
	return "success"
}

func mcpProxyHealthCheckRunMessage(run dto.MCPProxyHealthCheckRunResponse) string {
	return fmt.Sprintf(
		"scanned=%d checked=%d skipped=%d success=%d errors=%d blocked=%d",
		run.ScannedCount,
		run.CheckedCount,
		run.SkippedCount,
		run.SuccessCount,
		run.ErrorCount,
		run.BlockedCount,
	)
}

func mcpProxyHeartbeatRunStatus(run dto.MCPProxyHeartbeatRunResponse) string {
	if run.ErrorCount > 0 {
		return "failed"
	}
	return "success"
}

func mcpProxyHeartbeatRunMessage(run dto.MCPProxyHeartbeatRunResponse) string {
	return fmt.Sprintf(
		"scanned=%d pinged=%d skipped=%d success=%d errors=%d",
		run.ScannedCount,
		run.PingedCount,
		run.SkippedCount,
		run.SuccessCount,
		run.ErrorCount,
	)
}

func ctxOrBackground(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func closeMCPProxyIdleSessions() {
	type idleSessionCloser interface {
		CloseIdleSessions(idleTimeout time.Duration) int
	}
	closer, ok := defaultMCPProxyClient.(idleSessionCloser)
	if !ok || closer == nil {
		return
	}
	closed := closer.CloseIdleSessions(0)
	if closed > 0 {
		logger.LogDebug(context.Background(), "MCP proxy heartbeat closed idle sessions: %d", closed)
	}
}

func mcpProxyServerIds(servers []model.MCPProxyServer) []int {
	ids := make([]int, 0, len(servers))
	for _, server := range servers {
		if server.Id > 0 {
			ids = append(ids, server.Id)
		}
	}
	return ids
}

func persistMCPProxyHealthCheckRun(run *dto.MCPProxyHealthCheckRunResponse) error {
	encoded, err := common.Marshal(run)
	if err != nil {
		return err
	}
	return updateMCPProxyHealthCheckOptions(map[string]string{
		mcpProxyHealthCheckOptionLastRunAt:      strconv.FormatInt(common.GetTimestamp(), 10),
		mcpProxyHealthCheckOptionLastRunStatus:  run.Status,
		mcpProxyHealthCheckOptionLastRunMessage: run.Message,
		mcpProxyHealthCheckOptionLastRunResult:  string(encoded),
	})
}

func persistMCPProxyHeartbeatRun(run *dto.MCPProxyHeartbeatRunResponse) error {
	encoded, err := common.Marshal(run)
	if err != nil {
		return err
	}
	return updateMCPProxyHealthCheckOptions(map[string]string{
		mcpProxyHeartbeatOptionLastRunAt:      strconv.FormatInt(common.GetTimestamp(), 10),
		mcpProxyHeartbeatOptionLastRunStatus:  run.Status,
		mcpProxyHeartbeatOptionLastRunMessage: run.Message,
		mcpProxyHeartbeatOptionLastRunResult:  string(encoded),
	})
}

func updateMCPProxyHealthCheckOptions(values map[string]string) error {
	ensureMCPProxyHealthCheckOptionMap()
	return model.UpdateOptionsBulk(values)
}

func acquireMCPProxyHealthCheckLease() (string, bool, error) {
	owner := mcpProxyHealthCheckLeaseOwner()
	now := common.GetTimestamp()
	currentOwner := strings.TrimSpace(mcpProxyHealthCheckOptionString(mcpProxyHealthCheckOptionLeaseOwner, ""))
	currentUntil := mcpProxyHealthCheckOptionInt64(mcpProxyHealthCheckOptionLeaseUntil, 0)
	if currentOwner != "" && currentOwner != owner && currentUntil > now {
		return owner, false, nil
	}
	until := now + defaultMCPProxyHealthCheckLeaseSeconds
	if err := updateMCPProxyHealthCheckOptions(map[string]string{
		mcpProxyHealthCheckOptionLeaseOwner: owner,
		mcpProxyHealthCheckOptionLeaseUntil: strconv.FormatInt(until, 10),
	}); err != nil {
		return owner, false, err
	}
	return owner, true, nil
}

func acquireMCPProxyHeartbeatLease() (string, bool, error) {
	owner := mcpProxyHealthCheckLeaseOwner()
	now := common.GetTimestamp()
	currentOwner := strings.TrimSpace(mcpProxyHealthCheckOptionString(mcpProxyHeartbeatOptionLeaseOwner, ""))
	currentUntil := mcpProxyHealthCheckOptionInt64(mcpProxyHeartbeatOptionLeaseUntil, 0)
	if currentOwner != "" && currentOwner != owner && currentUntil > now {
		return owner, false, nil
	}
	until := now + defaultMCPProxyHeartbeatLeaseSeconds
	if err := updateMCPProxyHealthCheckOptions(map[string]string{
		mcpProxyHeartbeatOptionLeaseOwner: owner,
		mcpProxyHeartbeatOptionLeaseUntil: strconv.FormatInt(until, 10),
	}); err != nil {
		return owner, false, err
	}
	return owner, true, nil
}

func releaseMCPProxyHealthCheckLease(owner string) {
	owner = strings.TrimSpace(owner)
	if owner == "" {
		return
	}
	currentOwner := strings.TrimSpace(mcpProxyHealthCheckOptionString(mcpProxyHealthCheckOptionLeaseOwner, ""))
	if currentOwner != owner {
		return
	}
	_ = updateMCPProxyHealthCheckOptions(map[string]string{
		mcpProxyHealthCheckOptionLeaseOwner: "",
		mcpProxyHealthCheckOptionLeaseUntil: "0",
	})
}

func releaseMCPProxyHeartbeatLease(owner string) {
	owner = strings.TrimSpace(owner)
	if owner == "" {
		return
	}
	currentOwner := strings.TrimSpace(mcpProxyHealthCheckOptionString(mcpProxyHeartbeatOptionLeaseOwner, ""))
	if currentOwner != owner {
		return
	}
	_ = updateMCPProxyHealthCheckOptions(map[string]string{
		mcpProxyHeartbeatOptionLeaseOwner: "",
		mcpProxyHeartbeatOptionLeaseUntil: "0",
	})
}

func mcpProxyHealthCheckLeaseOwner() string {
	hostname, _ := os.Hostname()
	hostname = strings.TrimSpace(hostname)
	if hostname == "" {
		hostname = "unknown-host"
	}
	return fmt.Sprintf("%s:%d", hostname, os.Getpid())
}

func mcpProxyHealthCheckOptionString(key string, fallback string) string {
	ensureMCPProxyHealthCheckOptionMap()
	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()
	value, ok := common.OptionMap[key]
	if !ok {
		return fallback
	}
	return value
}

func mcpProxyHealthCheckOptionBool(key string, fallback bool) bool {
	value := mcpProxyHealthCheckOptionString(key, strconv.FormatBool(fallback))
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func mcpProxyHealthCheckOptionInt(key string, fallback int) int {
	value := mcpProxyHealthCheckOptionString(key, strconv.Itoa(fallback))
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func mcpProxyHealthCheckOptionInt64(key string, fallback int64) int64 {
	value := mcpProxyHealthCheckOptionString(key, strconv.FormatInt(fallback, 10))
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func ensureMCPProxyHealthCheckOptionMap() {
	common.OptionMapRWMutex.Lock()
	defer common.OptionMapRWMutex.Unlock()
	if common.OptionMap == nil {
		common.OptionMap = map[string]string{}
	}
}

func normalizeMCPProxyHealthCheckInterval(minutes int) int {
	if minutes <= 0 {
		return defaultMCPProxyHealthCheckIntervalMinutes
	}
	if minutes > 7*24*60 {
		return 7 * 24 * 60
	}
	return minutes
}

func normalizeMCPProxyHealthCheckLimit(limit int) int {
	if limit <= 0 {
		return defaultMCPProxyHealthCheckLimit
	}
	if limit > 100 {
		return 100
	}
	return limit
}

func normalizeMCPProxyHealthCheckStaleSeconds(seconds int64) int64 {
	if seconds <= 0 {
		return defaultMCPProxyHealthCheckStaleSeconds
	}
	if seconds > 7*24*60*60 {
		return 7 * 24 * 60 * 60
	}
	return seconds
}

func normalizeMCPProxyHealthCheckMaxDiscover(limit int) int {
	if limit <= 0 {
		return defaultMCPProxyHealthCheckMaxDiscover
	}
	if limit > 1000 {
		return 1000
	}
	return limit
}

func normalizeMCPProxyHeartbeatInterval(seconds int64) int64 {
	if seconds <= 0 {
		return defaultMCPProxyHeartbeatIntervalSeconds
	}
	if seconds < 30 {
		return 30
	}
	if seconds > 24*60*60 {
		return 24 * 60 * 60
	}
	return seconds
}

func normalizeMCPProxyHeartbeatLimit(limit int) int {
	if limit <= 0 {
		return defaultMCPProxyHeartbeatLimit
	}
	if limit > 200 {
		return 200
	}
	return limit
}

func normalizeMCPProxyHeartbeatActiveWindow(seconds int64) int64 {
	if seconds < 0 {
		return defaultMCPProxyHeartbeatActiveWindow
	}
	if seconds > 7*24*60*60 {
		return 7 * 24 * 60 * 60
	}
	return seconds
}

func normalizeMCPProxyHeartbeatTimeout(seconds int64) int64 {
	if seconds <= 0 {
		return defaultMCPProxyHeartbeatTimeoutSeconds
	}
	if seconds < 2 {
		return 2
	}
	if seconds > 120 {
		return 120
	}
	return seconds
}
