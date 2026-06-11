package service

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
)

const maxBillingEventRelationMaintenanceSamples = 20
const maxBillingEventRelationSelectedRepairItems = 100

type BillingEventRelationMaintenanceParams struct {
	Limit  int
	Cursor int64
	DryRun bool
}

type BillingEventRelationSelectedRepairParams struct {
	DryRun bool
	Items  []dto.BillingEventRelationMaintenanceItem
}

type billingEventRelationCandidate struct {
	Audit         model.BillingEvent
	Metadata      billingEventAuditMetadata
	TargetEventId int64
	RelationType  string
	Error         string
}

type billingEventRelationKey struct {
	SourceEventId int64
	TargetEventId int64
	RelationType  string
}

func GetBillingEventRelationHealth(params BillingEventRelationMaintenanceParams) (dto.BillingEventRelationHealthResponse, error) {
	limit := normalizeBillingEventBackfillLimit(params.Limit)
	audits, hasMore, nextCursor, totalAudits, err := listBillingEventRelationAuditEvents(limit, params.Cursor)
	if err != nil {
		return dto.BillingEventRelationHealthResponse{}, err
	}
	totalRelations, err := model.CountBillingEventRelations()
	if err != nil {
		return dto.BillingEventRelationHealthResponse{}, err
	}
	orphanSources, orphanTargets, err := countBillingEventRelationOrphans()
	if err != nil {
		return dto.BillingEventRelationHealthResponse{}, err
	}
	candidates := billingEventRelationCandidates(audits)
	existingRelations, existingTargets, err := loadBillingEventRelationState(candidates)
	if err != nil {
		return dto.BillingEventRelationHealthResponse{}, err
	}

	response := dto.BillingEventRelationHealthResponse{
		Limit:                  limit,
		Cursor:                 params.Cursor,
		CheckedAt:              common.GetTimestamp(),
		TotalAuditEvents:       totalAudits,
		TotalRelations:         totalRelations,
		ScannedAuditEvents:     len(audits),
		OrphanSourceRelations:  orphanSources,
		OrphanTargetRelations:  orphanTargets,
		HasMore:                hasMore,
		ScanComplete:           !hasMore,
		NextCursor:             nextCursor,
		SampleMissingRelations: []dto.BillingEventRelationMaintenanceItem{},
		SampleInvalidAudits:    []dto.BillingEventRelationMaintenanceItem{},
	}

	for _, candidate := range candidates {
		if candidate.Error != "" {
			response.InvalidAuditEvents++
			appendBillingEventRelationMaintenanceSample(&response.SampleInvalidAudits, billingEventRelationMaintenanceItem(candidate))
			continue
		}
		if !existingTargets[candidate.TargetEventId] {
			candidate.Error = "target event not found"
			response.InvalidAuditEvents++
			appendBillingEventRelationMaintenanceSample(&response.SampleInvalidAudits, billingEventRelationMaintenanceItem(candidate))
			continue
		}
		if _, ok := existingRelations[billingEventRelationCandidateKey(candidate)]; ok {
			continue
		}
		response.MissingRelations++
		appendBillingEventRelationMaintenanceSample(&response.SampleMissingRelations, billingEventRelationMaintenanceItem(candidate))
	}

	response.NeedsReview = response.MissingRelations > 0 ||
		response.InvalidAuditEvents > 0 ||
		response.OrphanSourceRelations > 0 ||
		response.OrphanTargetRelations > 0
	return response, nil
}

func BackfillBillingEventRelations(params BillingEventRelationMaintenanceParams) (dto.BillingEventRelationBackfillResponse, error) {
	limit := normalizeBillingEventBackfillLimit(params.Limit)
	audits, hasMore, nextCursor, _, err := listBillingEventRelationAuditEvents(limit, params.Cursor)
	if err != nil {
		return dto.BillingEventRelationBackfillResponse{}, err
	}
	candidates := billingEventRelationCandidates(audits)
	existingRelations, existingTargets, err := loadBillingEventRelationState(candidates)
	if err != nil {
		return dto.BillingEventRelationBackfillResponse{}, err
	}

	response := dto.BillingEventRelationBackfillResponse{
		DryRun:             params.DryRun,
		Limit:              limit,
		Cursor:             params.Cursor,
		ScannedAuditEvents: len(audits),
		HasMore:            hasMore,
		ScanComplete:       !hasMore,
		NextCursor:         nextCursor,
		Items:              []dto.BillingEventRelationMaintenanceItem{},
		Errors:             []string{},
	}

	for _, candidate := range candidates {
		if candidate.Error != "" {
			response.SkippedInvalid++
			appendBillingEventRelationBackfillError(&response, candidate.Error, candidate.Audit.EventId)
			continue
		}
		if !existingTargets[candidate.TargetEventId] {
			response.SkippedInvalid++
			appendBillingEventRelationBackfillError(&response, "target event not found", candidate.Audit.EventId)
			continue
		}
		key := billingEventRelationCandidateKey(candidate)
		if _, ok := existingRelations[key]; ok {
			response.SkippedExisting++
			continue
		}
		item := billingEventRelationMaintenanceItem(candidate)
		if params.DryRun {
			response.WouldCreate++
			appendBillingEventRelationMaintenanceSample(&response.Items, item)
			continue
		}

		created, err := model.CreateBillingEventRelationIfNotExists(nil, &model.BillingEventRelation{
			SourceEventId: candidate.Audit.Id,
			TargetEventId: candidate.TargetEventId,
			RelationType:  candidate.RelationType,
			Reason:        billingEventRelationReason(candidate.Metadata.Reason),
			Label:         billingEventRelationLabel(candidate.Metadata.Label),
			AdminId:       candidate.Metadata.AdminId,
		})
		if err != nil {
			response.ErrorCount++
			appendBillingEventRelationBackfillError(&response, err.Error(), candidate.Audit.EventId)
			continue
		}
		if !created {
			response.SkippedExisting++
			existingRelations[key] = struct{}{}
			continue
		}
		response.Created++
		existingRelations[key] = struct{}{}
		appendBillingEventRelationMaintenanceSample(&response.Items, item)
	}

	return response, nil
}

func RepairSelectedBillingEventRelations(params BillingEventRelationSelectedRepairParams) (dto.BillingEventRelationRepairResponse, error) {
	if len(params.Items) > maxBillingEventRelationSelectedRepairItems {
		return dto.BillingEventRelationRepairResponse{}, fmt.Errorf("too many selected relation items: max %d", maxBillingEventRelationSelectedRepairItems)
	}
	response := dto.BillingEventRelationRepairResponse{
		DryRun:   params.DryRun,
		Selected: len(params.Items),
		Items:    []dto.BillingEventRelationMaintenanceItem{},
		Errors:   []string{},
	}
	if len(params.Items) == 0 {
		return response, nil
	}

	for _, item := range params.Items {
		candidate, err := billingEventRelationRepairCandidate(item)
		if err != nil {
			response.SkippedInvalid++
			appendBillingEventRelationRepairError(&response, err.Error(), item.AuditEvent)
			continue
		}
		if candidate.Error != "" {
			response.SkippedInvalid++
			appendBillingEventRelationRepairError(&response, candidate.Error, candidate.Audit.EventId)
			continue
		}

		existingRelations, existingTargets, err := loadBillingEventRelationState([]billingEventRelationCandidate{candidate})
		if err != nil {
			response.ErrorCount++
			appendBillingEventRelationRepairError(&response, err.Error(), candidate.Audit.EventId)
			continue
		}
		if !existingTargets[candidate.TargetEventId] {
			response.SkippedInvalid++
			appendBillingEventRelationRepairError(&response, "target event not found", candidate.Audit.EventId)
			continue
		}
		if _, ok := existingRelations[billingEventRelationCandidateKey(candidate)]; ok {
			response.SkippedExisting++
			continue
		}

		repairedItem := billingEventRelationMaintenanceItem(candidate)
		if params.DryRun {
			response.WouldCreate++
			appendBillingEventRelationMaintenanceSample(&response.Items, repairedItem)
			continue
		}
		created, err := model.CreateBillingEventRelationIfNotExists(nil, &model.BillingEventRelation{
			SourceEventId: candidate.Audit.Id,
			TargetEventId: candidate.TargetEventId,
			RelationType:  candidate.RelationType,
			Reason:        billingEventRelationReason(candidate.Metadata.Reason),
			Label:         billingEventRelationLabel(candidate.Metadata.Label),
			AdminId:       candidate.Metadata.AdminId,
		})
		if err != nil {
			response.ErrorCount++
			appendBillingEventRelationRepairError(&response, err.Error(), candidate.Audit.EventId)
			continue
		}
		if !created {
			response.SkippedExisting++
			continue
		}
		response.Created++
		appendBillingEventRelationMaintenanceSample(&response.Items, repairedItem)
	}

	return response, nil
}

func CleanupBillingEventRelationOrphans(params BillingEventRelationMaintenanceParams) (dto.BillingEventRelationOrphanCleanupResponse, error) {
	orphanIds, sourceOrphans, targetOrphans, err := listBillingEventRelationOrphanIds()
	if err != nil {
		return dto.BillingEventRelationOrphanCleanupResponse{}, err
	}
	response := dto.BillingEventRelationOrphanCleanupResponse{
		DryRun:        params.DryRun,
		SourceOrphans: sourceOrphans,
		TargetOrphans: targetOrphans,
		WouldDelete:   len(orphanIds),
	}
	if params.DryRun || len(orphanIds) == 0 {
		return response, nil
	}
	deleted, err := model.DeleteBillingEventRelationsByIds(orphanIds)
	if err != nil {
		return dto.BillingEventRelationOrphanCleanupResponse{}, err
	}
	response.Deleted = int(deleted)
	return response, nil
}

func billingEventRelationRepairCandidate(item dto.BillingEventRelationMaintenanceItem) (billingEventRelationCandidate, error) {
	if item.AuditEventId <= 0 {
		return billingEventRelationCandidate{}, fmt.Errorf("audit_event_id is required")
	}
	var audit model.BillingEvent
	if err := model.DB.Where("id = ?", item.AuditEventId).First(&audit).Error; err != nil {
		return billingEventRelationCandidate{}, err
	}
	if audit.Source != model.BillingEventSourceLedgerRepair || audit.EventType != model.BillingEventTypeAudit {
		return billingEventRelationCandidate{}, fmt.Errorf("audit event is not a billing repair audit")
	}
	candidates := billingEventRelationCandidates([]model.BillingEvent{audit})
	if len(candidates) != 1 {
		return billingEventRelationCandidate{}, fmt.Errorf("audit event could not be parsed")
	}
	candidate := candidates[0]
	if err := validateBillingEventRelationRepairItem(item, candidate); err != nil {
		return billingEventRelationCandidate{}, err
	}
	return candidate, nil
}

func validateBillingEventRelationRepairItem(item dto.BillingEventRelationMaintenanceItem, candidate billingEventRelationCandidate) error {
	if strings.TrimSpace(item.Error) != "" {
		return fmt.Errorf("selected relation item is invalid: %s", item.Error)
	}
	if strings.TrimSpace(item.AuditEvent) != "" && item.AuditEvent != candidate.Audit.EventId {
		return fmt.Errorf("audit event mismatch")
	}
	if item.TargetEventId > 0 && item.TargetEventId != candidate.TargetEventId {
		return fmt.Errorf("target event mismatch")
	}
	if strings.TrimSpace(item.RelationType) != "" && item.RelationType != candidate.RelationType {
		return fmt.Errorf("relation type mismatch")
	}
	return nil
}

func listBillingEventRelationAuditEvents(limit int, cursor int64) ([]model.BillingEvent, bool, int64, int64, error) {
	baseQuery := model.DB.Model(&model.BillingEvent{}).
		Where("source = ? AND event_type = ?", model.BillingEventSourceLedgerRepair, model.BillingEventTypeAudit)
	var total int64
	if err := baseQuery.Count(&total).Error; err != nil {
		return nil, false, 0, 0, err
	}
	query := baseQuery
	if cursor > 0 {
		query = query.Where("id < ?", cursor)
	}
	var audits []model.BillingEvent
	if err := query.Order("id desc").Limit(limit + 1).Find(&audits).Error; err != nil {
		return nil, false, 0, 0, err
	}
	hasMore := len(audits) > limit
	if hasMore {
		audits = audits[:limit]
	}
	nextCursor := int64(0)
	if hasMore && len(audits) > 0 {
		nextCursor = audits[len(audits)-1].Id
	}
	return audits, hasMore, nextCursor, total, nil
}

func billingEventRelationCandidates(audits []model.BillingEvent) []billingEventRelationCandidate {
	candidates := make([]billingEventRelationCandidate, 0, len(audits))
	for _, audit := range audits {
		metadata := billingEventAuditMetadata{}
		candidate := billingEventRelationCandidate{Audit: audit, Metadata: metadata}
		if err := common.UnmarshalJsonStr(audit.Metadata, &metadata); err != nil {
			candidate.Error = "invalid audit metadata"
			candidates = append(candidates, candidate)
			continue
		}
		candidate.Metadata = metadata
		candidate.TargetEventId = billingEventAuditTargetId(metadata)
		candidate.RelationType = billingEventRelationTypeForAudit(audit, metadata)
		if candidate.TargetEventId <= 0 {
			candidate.Error = "missing target event id"
		} else if candidate.RelationType == "" {
			candidate.Error = "missing relation type"
		}
		candidates = append(candidates, candidate)
	}
	return candidates
}

func billingEventRelationTypeForAudit(audit model.BillingEvent, metadata billingEventAuditMetadata) string {
	switch audit.PriceUnit {
	case model.BillingEventRelationTypeReconciliationRepair, model.BillingEventRelationTypeReconciliationBackfillMissing:
		return audit.PriceUnit
	}
	if metadata.TargetEventPk > 0 {
		return model.BillingEventRelationTypeReconciliationRepair
	}
	if metadata.CreatedEventPk > 0 || metadata.CreatedEvent != nil {
		return model.BillingEventRelationTypeReconciliationBackfillMissing
	}
	return ""
}

func loadBillingEventRelationState(candidates []billingEventRelationCandidate) (map[billingEventRelationKey]struct{}, map[int64]bool, error) {
	sourceIds := make([]int64, 0, len(candidates))
	targetIds := make([]int64, 0, len(candidates))
	seenSources := map[int64]struct{}{}
	seenTargets := map[int64]struct{}{}
	for _, candidate := range candidates {
		if candidate.Audit.Id > 0 {
			if _, ok := seenSources[candidate.Audit.Id]; !ok {
				seenSources[candidate.Audit.Id] = struct{}{}
				sourceIds = append(sourceIds, candidate.Audit.Id)
			}
		}
		if candidate.TargetEventId > 0 {
			if _, ok := seenTargets[candidate.TargetEventId]; !ok {
				seenTargets[candidate.TargetEventId] = struct{}{}
				targetIds = append(targetIds, candidate.TargetEventId)
			}
		}
	}

	relationSet := map[billingEventRelationKey]struct{}{}
	relations, err := model.ListBillingEventRelationsBySourceIds(sourceIds)
	if err != nil {
		return nil, nil, err
	}
	for _, relation := range relations {
		relationSet[billingEventRelationKey{
			SourceEventId: relation.SourceEventId,
			TargetEventId: relation.TargetEventId,
			RelationType:  relation.RelationType,
		}] = struct{}{}
	}

	targetSet := map[int64]bool{}
	if len(targetIds) > 0 {
		var existingTargetIds []int64
		if err := model.DB.Model(&model.BillingEvent{}).Where("id IN ?", targetIds).Pluck("id", &existingTargetIds).Error; err != nil {
			return nil, nil, err
		}
		for _, id := range existingTargetIds {
			targetSet[id] = true
		}
	}
	return relationSet, targetSet, nil
}

func countBillingEventRelationOrphans() (int, int, error) {
	var sourceCount int64
	if err := model.DB.Table("billing_event_relations AS r").
		Joins("LEFT JOIN billing_events AS e ON e.id = r.source_event_id").
		Where("e.id IS NULL").
		Count(&sourceCount).Error; err != nil {
		return 0, 0, err
	}
	var targetCount int64
	if err := model.DB.Table("billing_event_relations AS r").
		Joins("LEFT JOIN billing_events AS e ON e.id = r.target_event_id").
		Where("e.id IS NULL").
		Count(&targetCount).Error; err != nil {
		return 0, 0, err
	}
	return int(sourceCount), int(targetCount), nil
}

func listBillingEventRelationOrphanIds() ([]int64, int, int, error) {
	var sourceIds []int64
	if err := model.DB.Table("billing_event_relations AS r").
		Select("r.id").
		Joins("LEFT JOIN billing_events AS e ON e.id = r.source_event_id").
		Where("e.id IS NULL").
		Pluck("r.id", &sourceIds).Error; err != nil {
		return nil, 0, 0, err
	}
	var targetIds []int64
	if err := model.DB.Table("billing_event_relations AS r").
		Select("r.id").
		Joins("LEFT JOIN billing_events AS e ON e.id = r.target_event_id").
		Where("e.id IS NULL").
		Pluck("r.id", &targetIds).Error; err != nil {
		return nil, 0, 0, err
	}
	seen := make(map[int64]struct{}, len(sourceIds)+len(targetIds))
	ids := make([]int64, 0, len(sourceIds)+len(targetIds))
	for _, id := range append(sourceIds, targetIds...) {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids, len(sourceIds), len(targetIds), nil
}

func billingEventRelationCandidateKey(candidate billingEventRelationCandidate) billingEventRelationKey {
	return billingEventRelationKey{
		SourceEventId: candidate.Audit.Id,
		TargetEventId: candidate.TargetEventId,
		RelationType:  candidate.RelationType,
	}
}

func billingEventRelationMaintenanceItem(candidate billingEventRelationCandidate) dto.BillingEventRelationMaintenanceItem {
	return dto.BillingEventRelationMaintenanceItem{
		AuditEventId:  candidate.Audit.Id,
		AuditEvent:    candidate.Audit.EventId,
		TargetEventId: candidate.TargetEventId,
		RelationType:  candidate.RelationType,
		Reason:        candidate.Metadata.Reason,
		Label:         candidate.Metadata.Label,
		AdminId:       candidate.Metadata.AdminId,
		Error:         candidate.Error,
	}
}

func appendBillingEventRelationMaintenanceSample(items *[]dto.BillingEventRelationMaintenanceItem, item dto.BillingEventRelationMaintenanceItem) {
	if len(*items) >= maxBillingEventRelationMaintenanceSamples {
		return
	}
	*items = append(*items, item)
}

func appendBillingEventRelationBackfillError(response *dto.BillingEventRelationBackfillResponse, message string, eventId string) {
	if len(response.Errors) >= maxBillingEventBackfillErrors {
		return
	}
	if eventId != "" {
		message = fmt.Sprintf("%s: %s", eventId, message)
	}
	response.Errors = append(response.Errors, message)
}

func appendBillingEventRelationRepairError(response *dto.BillingEventRelationRepairResponse, message string, eventId string) {
	if len(response.Errors) >= maxBillingEventBackfillErrors {
		return
	}
	if eventId != "" {
		message = fmt.Sprintf("%s: %s", eventId, message)
	}
	response.Errors = append(response.Errors, message)
}
