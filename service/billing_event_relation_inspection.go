package service

import (
	"context"
	"fmt"
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
	billingEventRelationInspectionTickInterval           = time.Minute
	defaultBillingEventRelationInspectionIntervalMinutes = 60
	defaultBillingEventRelationInspectionMaxAutoBackfill = 200
	defaultBillingEventRelationInspectionMaxAutoCleanup  = 100

	billingEventRelationInspectionOptionPrefix             = "BillingEventRelationInspection"
	billingEventRelationInspectionOptionEnabled            = billingEventRelationInspectionOptionPrefix + "Enabled"
	billingEventRelationInspectionOptionIntervalMinutes    = billingEventRelationInspectionOptionPrefix + "IntervalMinutes"
	billingEventRelationInspectionOptionLimit              = billingEventRelationInspectionOptionPrefix + "Limit"
	billingEventRelationInspectionOptionAutoBackfill       = billingEventRelationInspectionOptionPrefix + "AutoBackfill"
	billingEventRelationInspectionOptionAutoCleanupOrphans = billingEventRelationInspectionOptionPrefix + "AutoCleanupOrphans"
	billingEventRelationInspectionOptionMaxAutoBackfill    = billingEventRelationInspectionOptionPrefix + "MaxAutoBackfill"
	billingEventRelationInspectionOptionMaxAutoCleanup     = billingEventRelationInspectionOptionPrefix + "MaxAutoCleanupOrphans"
	billingEventRelationInspectionOptionCursor             = billingEventRelationInspectionOptionPrefix + "Cursor"
	billingEventRelationInspectionOptionLastRunAt          = billingEventRelationInspectionOptionPrefix + "LastRunAt"
	billingEventRelationInspectionOptionLastRunStatus      = billingEventRelationInspectionOptionPrefix + "LastRunStatus"
	billingEventRelationInspectionOptionLastRunMessage     = billingEventRelationInspectionOptionPrefix + "LastRunMessage"
	billingEventRelationInspectionOptionLastResult         = billingEventRelationInspectionOptionPrefix + "LastResult"
)

var (
	billingEventRelationInspectionOnce    sync.Once
	billingEventRelationInspectionRunning atomic.Bool
)

func GetBillingEventRelationInspectionStatus() dto.BillingEventRelationInspectionStatusResponse {
	settings := GetBillingEventRelationInspectionSettings()
	recentRuns, _ := ListBillingEventRelationInspectionRuns(5)
	status := dto.BillingEventRelationInspectionStatusResponse{
		Settings:       settings,
		Running:        billingEventRelationInspectionRunning.Load(),
		LastRunAt:      billingEventRelationInspectionOptionInt64(billingEventRelationInspectionOptionLastRunAt, 0),
		LastRunStatus:  billingEventRelationInspectionOptionString(billingEventRelationInspectionOptionLastRunStatus, ""),
		LastRunMessage: billingEventRelationInspectionOptionString(billingEventRelationInspectionOptionLastRunMessage, ""),
		RecentRuns:     recentRuns,
	}

	lastResult := billingEventRelationInspectionOptionString(billingEventRelationInspectionOptionLastResult, "")
	if lastResult == "" {
		return status
	}
	var run dto.BillingEventRelationInspectionRunResponse
	if err := common.UnmarshalJsonStr(lastResult, &run); err != nil {
		return status
	}
	if run.Health.CheckedAt > 0 {
		status.LastHealth = &run.Health
	}
	status.LastBackfill = run.Backfill
	status.LastCleanup = run.Cleanup
	return status
}

func GetBillingEventRelationInspectionSettings() dto.BillingEventRelationInspectionSettings {
	return dto.BillingEventRelationInspectionSettings{
		Enabled:               billingEventRelationInspectionOptionBool(billingEventRelationInspectionOptionEnabled, false),
		IntervalMinutes:       normalizeBillingEventRelationInspectionInterval(billingEventRelationInspectionOptionInt(billingEventRelationInspectionOptionIntervalMinutes, defaultBillingEventRelationInspectionIntervalMinutes)),
		Limit:                 normalizeBillingEventBackfillLimit(billingEventRelationInspectionOptionInt(billingEventRelationInspectionOptionLimit, defaultBillingEventBackfillLimit)),
		AutoBackfill:          billingEventRelationInspectionOptionBool(billingEventRelationInspectionOptionAutoBackfill, false),
		AutoCleanupOrphans:    billingEventRelationInspectionOptionBool(billingEventRelationInspectionOptionAutoCleanupOrphans, false),
		MaxAutoBackfill:       normalizeBillingEventRelationInspectionThreshold(billingEventRelationInspectionOptionInt(billingEventRelationInspectionOptionMaxAutoBackfill, defaultBillingEventRelationInspectionMaxAutoBackfill), defaultBillingEventRelationInspectionMaxAutoBackfill),
		MaxAutoCleanupOrphans: normalizeBillingEventRelationInspectionThreshold(billingEventRelationInspectionOptionInt(billingEventRelationInspectionOptionMaxAutoCleanup, defaultBillingEventRelationInspectionMaxAutoCleanup), defaultBillingEventRelationInspectionMaxAutoCleanup),
		Cursor:                normalizeBillingEventRelationInspectionCursor(billingEventRelationInspectionOptionInt64(billingEventRelationInspectionOptionCursor, 0)),
	}
}

func UpdateBillingEventRelationInspectionSettings(req dto.BillingEventRelationInspectionSettingsRequest) (dto.BillingEventRelationInspectionStatusResponse, error) {
	current := GetBillingEventRelationInspectionSettings()
	cursor := current.Cursor
	if req.Cursor != nil {
		cursor = normalizeBillingEventRelationInspectionCursor(*req.Cursor)
	}
	values := map[string]string{
		billingEventRelationInspectionOptionEnabled:            strconv.FormatBool(req.Enabled),
		billingEventRelationInspectionOptionIntervalMinutes:    strconv.Itoa(normalizeBillingEventRelationInspectionInterval(req.IntervalMinutes)),
		billingEventRelationInspectionOptionLimit:              strconv.Itoa(normalizeBillingEventBackfillLimit(req.Limit)),
		billingEventRelationInspectionOptionAutoBackfill:       strconv.FormatBool(req.AutoBackfill),
		billingEventRelationInspectionOptionAutoCleanupOrphans: strconv.FormatBool(req.AutoCleanupOrphans),
		billingEventRelationInspectionOptionMaxAutoBackfill:    strconv.Itoa(normalizeBillingEventRelationInspectionThreshold(req.MaxAutoBackfill, defaultBillingEventRelationInspectionMaxAutoBackfill)),
		billingEventRelationInspectionOptionMaxAutoCleanup:     strconv.Itoa(normalizeBillingEventRelationInspectionThreshold(req.MaxAutoCleanupOrphans, defaultBillingEventRelationInspectionMaxAutoCleanup)),
		billingEventRelationInspectionOptionCursor:             strconv.FormatInt(cursor, 10),
	}
	if err := updateBillingEventRelationInspectionOptions(values); err != nil {
		return dto.BillingEventRelationInspectionStatusResponse{}, err
	}
	return GetBillingEventRelationInspectionStatus(), nil
}

func RunBillingEventRelationInspectionOnce(manual bool) (dto.BillingEventRelationInspectionRunResponse, error) {
	settings := GetBillingEventRelationInspectionSettings()
	if !billingEventRelationInspectionRunning.CompareAndSwap(false, true) {
		return dto.BillingEventRelationInspectionRunResponse{
			Manual:   manual,
			Status:   "running",
			Message:  "billing event relation inspection is already running",
			Settings: settings,
		}, nil
	}
	defer billingEventRelationInspectionRunning.Store(false)
	trigger := model.BillingEventRelationInspectionTriggerScheduled
	if manual {
		trigger = model.BillingEventRelationInspectionTriggerManual
	}
	runRecord := newBillingEventRelationInspectionRunRecord(trigger, settings)
	if err := model.CreateBillingEventRelationInspectionRun(runRecord); err != nil {
		return dto.BillingEventRelationInspectionRunResponse{}, err
	}

	health, err := GetBillingEventRelationHealth(BillingEventRelationMaintenanceParams{
		Limit:  settings.Limit,
		Cursor: settings.Cursor,
	})
	if err != nil {
		_ = finishBillingEventRelationInspectionRun(runRecord, dto.BillingEventRelationInspectionRunResponse{}, model.BillingEventRelationInspectionStatusFailed, err.Error(), settings.Cursor)
		return dto.BillingEventRelationInspectionRunResponse{}, err
	}

	run := dto.BillingEventRelationInspectionRunResponse{
		Manual:   manual,
		Status:   "success",
		Settings: settings,
		Health:   health,
	}
	populateBillingEventRelationInspectionRunHealth(runRecord, health)
	if settings.AutoBackfill {
		preview, err := BackfillBillingEventRelations(BillingEventRelationMaintenanceParams{
			Limit:  settings.Limit,
			Cursor: settings.Cursor,
			DryRun: true,
		})
		if err != nil {
			run.Status = "failed"
			run.Message = err.Error()
			_ = finishBillingEventRelationInspectionRun(runRecord, run, model.BillingEventRelationInspectionStatusFailed, err.Error(), settings.Cursor)
			return dto.BillingEventRelationInspectionRunResponse{}, err
		}
		runRecord.BackfillWouldCreate = preview.WouldCreate
		runRecord.BackfillSkippedInvalid = preview.SkippedInvalid
		runRecord.BackfillErrorCount = preview.ErrorCount
		if settings.MaxAutoBackfill > 0 && preview.WouldCreate > settings.MaxAutoBackfill {
			runRecord.BackfillBlocked = true
			run.Status = model.BillingEventRelationInspectionStatusBlocked
			run.Message = fmt.Sprintf("auto backfill blocked: would_create=%d exceeds max_auto_backfill=%d", preview.WouldCreate, settings.MaxAutoBackfill)
		} else {
			backfill, err := BackfillBillingEventRelations(BillingEventRelationMaintenanceParams{
				Limit:  settings.Limit,
				Cursor: settings.Cursor,
			})
			if err != nil {
				run.Status = "failed"
				run.Message = err.Error()
				_ = finishBillingEventRelationInspectionRun(runRecord, run, model.BillingEventRelationInspectionStatusFailed, err.Error(), settings.Cursor)
				return dto.BillingEventRelationInspectionRunResponse{}, err
			}
			runRecord.BackfillCreated = backfill.Created
			runRecord.BackfillWouldCreate = backfill.WouldCreate
			runRecord.BackfillSkippedInvalid = backfill.SkippedInvalid
			runRecord.BackfillErrorCount = backfill.ErrorCount
			run.Backfill = &backfill
		}
	}
	if settings.AutoCleanupOrphans {
		preview, err := CleanupBillingEventRelationOrphans(BillingEventRelationMaintenanceParams{DryRun: true})
		if err != nil {
			run.Status = "failed"
			run.Message = err.Error()
			_ = finishBillingEventRelationInspectionRun(runRecord, run, model.BillingEventRelationInspectionStatusFailed, err.Error(), settings.Cursor)
			return dto.BillingEventRelationInspectionRunResponse{}, err
		}
		runRecord.CleanupWouldDelete = preview.WouldDelete
		if settings.MaxAutoCleanupOrphans > 0 && preview.WouldDelete > settings.MaxAutoCleanupOrphans {
			runRecord.CleanupBlocked = true
			run.Status = model.BillingEventRelationInspectionStatusBlocked
			message := fmt.Sprintf("auto cleanup blocked: would_delete=%d exceeds max_auto_cleanup_orphans=%d", preview.WouldDelete, settings.MaxAutoCleanupOrphans)
			if run.Message != "" {
				run.Message += "; " + message
			} else {
				run.Message = message
			}
		} else {
			cleanup, err := CleanupBillingEventRelationOrphans(BillingEventRelationMaintenanceParams{})
			if err != nil {
				run.Status = "failed"
				run.Message = err.Error()
				_ = finishBillingEventRelationInspectionRun(runRecord, run, model.BillingEventRelationInspectionStatusFailed, err.Error(), settings.Cursor)
				return dto.BillingEventRelationInspectionRunResponse{}, err
			}
			runRecord.CleanupDeleted = cleanup.Deleted
			runRecord.CleanupWouldDelete = cleanup.WouldDelete
			run.Cleanup = &cleanup
		}
	}

	nextCursor := int64(0)
	if health.HasMore {
		nextCursor = health.NextCursor
	}
	if run.Message == "" {
		run.Message = billingEventRelationInspectionSuccessMessage(run)
	}
	status := model.BillingEventRelationInspectionStatusSuccess
	if run.Status == model.BillingEventRelationInspectionStatusBlocked {
		status = model.BillingEventRelationInspectionStatusBlocked
		nextCursor = settings.Cursor
	}
	run.Settings.Cursor = nextCursor
	if err := finishBillingEventRelationInspectionRun(runRecord, run, status, run.Message, nextCursor); err != nil {
		return dto.BillingEventRelationInspectionRunResponse{}, err
	}
	runItem := billingEventRelationInspectionRunItem(*runRecord)
	run.Run = &runItem
	return run, nil
}

func StartBillingEventRelationInspectionTask() {
	billingEventRelationInspectionOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		gopool.Go(func() {
			logger.LogInfo(context.Background(), fmt.Sprintf("billing event relation inspection task started: tick=%s", billingEventRelationInspectionTickInterval))
			ticker := time.NewTicker(billingEventRelationInspectionTickInterval)
			defer ticker.Stop()

			runBillingEventRelationInspectionIfDue()
			for range ticker.C {
				runBillingEventRelationInspectionIfDue()
			}
		})
	})
}

func runBillingEventRelationInspectionIfDue() {
	settings := GetBillingEventRelationInspectionSettings()
	if !settings.Enabled {
		return
	}
	lastRunAt := billingEventRelationInspectionOptionInt64(billingEventRelationInspectionOptionLastRunAt, 0)
	if lastRunAt > 0 {
		nextRunAt := time.Unix(lastRunAt, 0).Add(time.Duration(settings.IntervalMinutes) * time.Minute)
		if time.Now().Before(nextRunAt) {
			return
		}
	}
	result, err := RunBillingEventRelationInspectionOnce(false)
	if err != nil {
		logger.LogWarn(context.Background(), fmt.Sprintf("billing event relation inspection failed: %v", err))
		return
	}
	if result.Status == "running" {
		return
	}
	logger.LogDebug(context.Background(), "billing event relation inspection: %s", result.Message)
}

func persistBillingEventRelationInspectionSuccess(nextCursor int64, run *dto.BillingEventRelationInspectionRunResponse) error {
	return persistBillingEventRelationInspectionResult("success", run.Message, nextCursor, run)
}

func persistBillingEventRelationInspectionFailure(message string, run *dto.BillingEventRelationInspectionRunResponse) error {
	values := map[string]string{
		billingEventRelationInspectionOptionLastRunAt:      strconv.FormatInt(common.GetTimestamp(), 10),
		billingEventRelationInspectionOptionLastRunStatus:  "failed",
		billingEventRelationInspectionOptionLastRunMessage: message,
		billingEventRelationInspectionOptionLastResult:     "",
	}
	if run != nil {
		encoded, err := common.Marshal(run)
		if err != nil {
			return err
		}
		values[billingEventRelationInspectionOptionLastResult] = string(encoded)
	}
	return updateBillingEventRelationInspectionOptions(values)
}

func persistBillingEventRelationInspectionResult(status string, message string, nextCursor int64, run *dto.BillingEventRelationInspectionRunResponse) error {
	encoded, err := common.Marshal(run)
	if err != nil {
		return err
	}
	values := map[string]string{
		billingEventRelationInspectionOptionCursor:         strconv.FormatInt(nextCursor, 10),
		billingEventRelationInspectionOptionLastRunAt:      strconv.FormatInt(common.GetTimestamp(), 10),
		billingEventRelationInspectionOptionLastRunStatus:  status,
		billingEventRelationInspectionOptionLastRunMessage: message,
		billingEventRelationInspectionOptionLastResult:     string(encoded),
	}
	return updateBillingEventRelationInspectionOptions(values)
}

func finishBillingEventRelationInspectionRun(record *model.BillingEventRelationInspectionRun, run dto.BillingEventRelationInspectionRunResponse, status string, message string, nextCursor int64) error {
	record.Status = status
	record.Message = message
	record.NextCursor = nextCursor
	record.FinishedAt = common.GetTimestamp()
	encoded, err := common.Marshal(run)
	if err != nil {
		return err
	}
	record.ResultJson = string(encoded)
	if err := model.UpdateBillingEventRelationInspectionRun(record); err != nil {
		return err
	}
	return persistBillingEventRelationInspectionResult(status, message, nextCursor, &run)
}

func newBillingEventRelationInspectionRunRecord(trigger string, settings dto.BillingEventRelationInspectionSettings) *model.BillingEventRelationInspectionRun {
	return &model.BillingEventRelationInspectionRun{
		Trigger:               trigger,
		Status:                model.BillingEventRelationInspectionStatusRunning,
		Limit:                 settings.Limit,
		Cursor:                settings.Cursor,
		AutoBackfill:          settings.AutoBackfill,
		AutoCleanupOrphans:    settings.AutoCleanupOrphans,
		MaxAutoBackfill:       settings.MaxAutoBackfill,
		MaxAutoCleanupOrphans: settings.MaxAutoCleanupOrphans,
	}
}

func populateBillingEventRelationInspectionRunHealth(record *model.BillingEventRelationInspectionRun, health dto.BillingEventRelationHealthResponse) {
	record.ScannedAuditEvents = health.ScannedAuditEvents
	record.MissingRelations = health.MissingRelations
	record.InvalidAuditEvents = health.InvalidAuditEvents
	record.OrphanSourceRelations = health.OrphanSourceRelations
	record.OrphanTargetRelations = health.OrphanTargetRelations
}

func ListBillingEventRelationInspectionRuns(limit int) ([]dto.BillingEventRelationInspectionRunItem, error) {
	runs, err := model.ListBillingEventRelationInspectionRuns(limit)
	if err != nil {
		return nil, err
	}
	items := make([]dto.BillingEventRelationInspectionRunItem, 0, len(runs))
	for _, run := range runs {
		items = append(items, billingEventRelationInspectionRunItem(run))
	}
	return items, nil
}

func ListBillingEventRelationInspectionRunsPage(offset int, limit int) ([]dto.BillingEventRelationInspectionRunItem, int64, error) {
	runs, total, err := model.ListBillingEventRelationInspectionRunsPage(offset, limit)
	if err != nil {
		return nil, 0, err
	}
	items := make([]dto.BillingEventRelationInspectionRunItem, 0, len(runs))
	for _, run := range runs {
		items = append(items, billingEventRelationInspectionRunItem(run))
	}
	return items, total, nil
}

func billingEventRelationInspectionRunItem(run model.BillingEventRelationInspectionRun) dto.BillingEventRelationInspectionRunItem {
	return dto.BillingEventRelationInspectionRunItem{
		Id:                     run.Id,
		Trigger:                run.Trigger,
		Status:                 run.Status,
		Message:                run.Message,
		Limit:                  run.Limit,
		Cursor:                 run.Cursor,
		NextCursor:             run.NextCursor,
		AutoBackfill:           run.AutoBackfill,
		AutoCleanupOrphans:     run.AutoCleanupOrphans,
		MaxAutoBackfill:        run.MaxAutoBackfill,
		MaxAutoCleanupOrphans:  run.MaxAutoCleanupOrphans,
		ScannedAuditEvents:     run.ScannedAuditEvents,
		MissingRelations:       run.MissingRelations,
		InvalidAuditEvents:     run.InvalidAuditEvents,
		OrphanSourceRelations:  run.OrphanSourceRelations,
		OrphanTargetRelations:  run.OrphanTargetRelations,
		BackfillCreated:        run.BackfillCreated,
		BackfillWouldCreate:    run.BackfillWouldCreate,
		BackfillSkippedInvalid: run.BackfillSkippedInvalid,
		BackfillErrorCount:     run.BackfillErrorCount,
		BackfillBlocked:        run.BackfillBlocked,
		CleanupDeleted:         run.CleanupDeleted,
		CleanupWouldDelete:     run.CleanupWouldDelete,
		CleanupBlocked:         run.CleanupBlocked,
		StartedAt:              run.StartedAt,
		FinishedAt:             run.FinishedAt,
		CreatedAt:              run.CreatedAt,
	}
}

func billingEventRelationInspectionSuccessMessage(run dto.BillingEventRelationInspectionRunResponse) string {
	orphans := run.Health.OrphanSourceRelations + run.Health.OrphanTargetRelations
	parts := []string{
		fmt.Sprintf("scanned=%d", run.Health.ScannedAuditEvents),
		fmt.Sprintf("missing=%d", run.Health.MissingRelations),
		fmt.Sprintf("invalid=%d", run.Health.InvalidAuditEvents),
		fmt.Sprintf("orphans=%d", orphans),
	}
	if run.Backfill != nil {
		parts = append(parts, fmt.Sprintf("backfilled=%d", run.Backfill.Created))
	}
	if run.Cleanup != nil {
		parts = append(parts, fmt.Sprintf("cleaned=%d", run.Cleanup.Deleted))
	}
	if run.Health.HasMore {
		parts = append(parts, fmt.Sprintf("next_cursor=%d", run.Health.NextCursor))
	}
	return strings.Join(parts, ", ")
}

func updateBillingEventRelationInspectionOptions(values map[string]string) error {
	ensureBillingEventRelationInspectionOptionMap()
	return model.UpdateOptionsBulk(values)
}

func billingEventRelationInspectionOptionString(key string, fallback string) string {
	ensureBillingEventRelationInspectionOptionMap()
	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()
	value, ok := common.OptionMap[key]
	if !ok {
		return fallback
	}
	return value
}

func billingEventRelationInspectionOptionBool(key string, fallback bool) bool {
	value := billingEventRelationInspectionOptionString(key, strconv.FormatBool(fallback))
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func billingEventRelationInspectionOptionInt(key string, fallback int) int {
	value := billingEventRelationInspectionOptionString(key, strconv.Itoa(fallback))
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func billingEventRelationInspectionOptionInt64(key string, fallback int64) int64 {
	value := billingEventRelationInspectionOptionString(key, strconv.FormatInt(fallback, 10))
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func ensureBillingEventRelationInspectionOptionMap() {
	common.OptionMapRWMutex.Lock()
	defer common.OptionMapRWMutex.Unlock()
	if common.OptionMap == nil {
		common.OptionMap = map[string]string{}
	}
}

func normalizeBillingEventRelationInspectionInterval(minutes int) int {
	if minutes <= 0 {
		return defaultBillingEventRelationInspectionIntervalMinutes
	}
	if minutes > 7*24*60 {
		return 7 * 24 * 60
	}
	return minutes
}

func normalizeBillingEventRelationInspectionCursor(cursor int64) int64 {
	if cursor < 0 {
		return 0
	}
	return cursor
}

func normalizeBillingEventRelationInspectionThreshold(limit int, fallback int) int {
	if limit <= 0 {
		return fallback
	}
	if limit > maxBillingEventBackfillLimit {
		return maxBillingEventBackfillLimit
	}
	return limit
}
