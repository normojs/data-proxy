package service

import (
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
)

type BillingEventListParams struct {
	UserId        int
	TokenId       int
	Source        string
	SourceId      string
	EventType     string
	Status        string
	RequestId     string
	BillingSource string
	UsageKind     string
	StartTime     int64
	EndTime       int64
	Keyword       string
	Offset        int
	Limit         int
}

const (
	defaultBillingEventSummaryWindowSeconds int64 = 30 * 24 * 60 * 60
	defaultBillingEventSummaryBucketSeconds int64 = 24 * 60 * 60
)

func ListBillingEvents(params BillingEventListParams) ([]dto.BillingEventItem, int64, error) {
	events, total, err := model.ListBillingEvents(model.BillingEventFilter{
		UserId:        params.UserId,
		TokenId:       params.TokenId,
		Source:        params.Source,
		SourceId:      params.SourceId,
		EventType:     params.EventType,
		Status:        params.Status,
		RequestId:     params.RequestId,
		BillingSource: params.BillingSource,
		UsageKind:     params.UsageKind,
		StartTime:     params.StartTime,
		EndTime:       params.EndTime,
		Keyword:       params.Keyword,
	}, params.Offset, params.Limit)
	if err != nil {
		return nil, 0, err
	}
	items := make([]dto.BillingEventItem, 0, len(events))
	for _, event := range events {
		items = append(items, billingEventToDTO(event))
	}
	if err := attachBillingEventAuditLinks(items); err != nil {
		return nil, 0, err
	}
	if err := attachBillingEventTargetLinks(items); err != nil {
		return nil, 0, err
	}
	if err := attachBillingEventMCPToolCallLinks(items); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func GetBillingEventSummary(params BillingEventListParams) (dto.BillingEventSummaryResponse, error) {
	startTime, endTime := normalizeBillingEventSummaryWindow(params.StartTime, params.EndTime)
	params.StartTime = startTime
	params.EndTime = endTime
	summary, err := model.SummarizeBillingEvents(billingEventFilterFromParams(params), defaultBillingEventSummaryBucketSeconds)
	if err != nil {
		return dto.BillingEventSummaryResponse{}, err
	}
	return dto.BillingEventSummaryResponse{
		StartTime:     startTime,
		EndTime:       endTime,
		BucketSeconds: defaultBillingEventSummaryBucketSeconds,
		CheckedAt:     common.GetTimestamp(),
		Totals:        billingEventAggregateDTO(summary.Totals),
		BySource:      billingEventDimensionDTOs(summary.BySource),
		ByType:        billingEventDimensionDTOs(summary.ByType),
		DailyTrend:    billingEventTrendDTOs(summary.DailyTrend),
	}, nil
}

func billingEventFilterFromParams(params BillingEventListParams) model.BillingEventFilter {
	return model.BillingEventFilter{
		UserId:        params.UserId,
		TokenId:       params.TokenId,
		Source:        params.Source,
		SourceId:      params.SourceId,
		EventType:     params.EventType,
		Status:        params.Status,
		RequestId:     params.RequestId,
		BillingSource: params.BillingSource,
		UsageKind:     params.UsageKind,
		StartTime:     params.StartTime,
		EndTime:       params.EndTime,
		Keyword:       params.Keyword,
	}
}

func normalizeBillingEventSummaryWindow(startTime int64, endTime int64) (int64, int64) {
	now := common.GetTimestamp()
	if endTime <= 0 {
		endTime = now
	}
	if startTime <= 0 {
		startTime = endTime - defaultBillingEventSummaryWindowSeconds
	}
	if startTime > endTime {
		startTime, endTime = endTime, startTime
	}
	return startTime, endTime
}

func billingEventAggregateDTO(value model.BillingEventAggregate) dto.BillingEventSummaryAggregate {
	return dto.BillingEventSummaryAggregate{
		TotalEvents:      value.TotalEvents,
		CreditEvents:     value.CreditEvents,
		DebitEvents:      value.DebitEvents,
		AuditEvents:      value.AuditEvents,
		AmountQuota:      value.AmountQuota,
		NetQuotaDelta:    value.NetQuotaDelta,
		CreditQuotaDelta: value.CreditQuotaDelta,
		DebitQuotaDelta:  value.DebitQuotaDelta,
		TotalCost:        value.TotalCost,
	}
}

func billingEventDimensionDTOs(items []model.BillingEventDimensionAggregate) []dto.BillingEventSummaryDimension {
	result := make([]dto.BillingEventSummaryDimension, 0, len(items))
	for _, item := range items {
		result = append(result, dto.BillingEventSummaryDimension{
			Key:                          item.Key,
			BillingEventSummaryAggregate: billingEventAggregateDTO(item.BillingEventAggregate),
		})
	}
	return result
}

func billingEventTrendDTOs(items []model.BillingEventTrendAggregate) []dto.BillingEventSummaryBucket {
	result := make([]dto.BillingEventSummaryBucket, 0, len(items))
	for _, item := range items {
		result = append(result, dto.BillingEventSummaryBucket{
			BucketStart:                  item.BucketStart,
			BillingEventSummaryAggregate: billingEventAggregateDTO(item.BillingEventAggregate),
		})
	}
	return result
}

type billingEventAuditMetadata struct {
	AdminId        int                  `json:"admin_id"`
	Reason         string               `json:"reason"`
	Label          string               `json:"label"`
	TargetEventPk  int64                `json:"target_event_pk"`
	CreatedEventPk int64                `json:"created_event_pk"`
	CreatedEvent   *billingEventAuditId `json:"created_event"`
}

type billingEventAuditId struct {
	Id int64 `json:"id"`
}

func attachBillingEventAuditLinks(items []dto.BillingEventItem) error {
	if len(items) == 0 {
		return nil
	}
	targetIds := make(map[int64]int, len(items))
	targetIdList := make([]int64, 0, len(items))
	for index := range items {
		targetIds[items[index].Id] = index
		targetIdList = append(targetIdList, items[index].Id)
	}

	linkedAuditIds := make(map[int64]map[int64]struct{}, len(items))
	relations, err := model.ListBillingEventRelationsByTargetIds(targetIdList)
	if err != nil {
		return err
	}
	if len(relations) > 0 {
		auditIds := make([]int64, 0, len(relations))
		seenAuditIds := make(map[int64]struct{}, len(relations))
		for _, relation := range relations {
			if _, ok := targetIds[relation.TargetEventId]; !ok {
				continue
			}
			if _, ok := seenAuditIds[relation.SourceEventId]; ok {
				continue
			}
			seenAuditIds[relation.SourceEventId] = struct{}{}
			auditIds = append(auditIds, relation.SourceEventId)
		}

		if len(auditIds) > 0 {
			var audits []model.BillingEvent
			if err := model.DB.Where("id IN ? AND source = ? AND event_type = ?", auditIds, model.BillingEventSourceLedgerRepair, model.BillingEventTypeAudit).
				Find(&audits).Error; err != nil {
				return err
			}
			auditById := make(map[int64]model.BillingEvent, len(audits))
			for _, audit := range audits {
				auditById[audit.Id] = audit
			}
			for _, relation := range relations {
				index, ok := targetIds[relation.TargetEventId]
				if !ok {
					continue
				}
				audit, ok := auditById[relation.SourceEventId]
				if !ok || !markBillingEventAuditLinked(linkedAuditIds, relation.TargetEventId, audit.Id) {
					continue
				}
				items[index].RelatedAuditEvents = append(items[index].RelatedAuditEvents, billingEventAuditLinkFromRelation(audit, relation))
			}
		}
	}

	return attachBillingEventAuditLinksFromMetadata(items, items, targetIds, linkedAuditIds)
}

func attachBillingEventAuditLinksFromMetadata(items []dto.BillingEventItem, candidates []dto.BillingEventItem, targetIds map[int64]int, linkedAuditIds map[int64]map[int64]struct{}) error {
	if len(candidates) == 0 {
		return nil
	}
	var audits []model.BillingEvent
	query := model.DB.Where("source = ? AND event_type = ?", model.BillingEventSourceLedgerRepair, model.BillingEventTypeAudit)
	candidateQuery, candidateArgs := buildBillingEventAuditLinkCandidateQuery(candidates)
	query = query.Where(candidateQuery, candidateArgs...)
	if err := query.
		Order("created_at desc, id desc").
		Find(&audits).Error; err != nil {
		return err
	}
	for _, audit := range audits {
		metadata := billingEventAuditMetadata{}
		if err := common.UnmarshalJsonStr(audit.Metadata, &metadata); err != nil {
			continue
		}
		targetId := billingEventAuditTargetId(metadata)
		index, ok := targetIds[targetId]
		if !ok || !markBillingEventAuditLinked(linkedAuditIds, targetId, audit.Id) {
			continue
		}
		items[index].RelatedAuditEvents = append(items[index].RelatedAuditEvents, billingEventAuditLinkFromMetadata(audit, metadata))
	}
	return nil
}

func buildBillingEventAuditLinkCandidateQuery(items []dto.BillingEventItem) (string, []any) {
	conditions := make([]string, 0, len(items)*2)
	args := make([]any, 0, len(items)*2)
	for _, item := range items {
		id := strconv.FormatInt(item.Id, 10)
		targetPattern := "%\"target_event_pk\":" + id + "%"
		createdPkPattern := "%\"created_event_pk\":" + id + "%"
		createdPattern := "%\"created_event\":{\"id\":" + id + "%"
		conditions = append(conditions, "metadata LIKE ?", "metadata LIKE ?", "metadata LIKE ?")
		args = append(args, targetPattern, createdPkPattern, createdPattern)
	}
	return "(" + strings.Join(conditions, " OR ") + ")", args
}

func attachBillingEventTargetLinks(items []dto.BillingEventItem) error {
	auditIds := make([]int64, 0, len(items))
	auditIndexes := make(map[int64][]int, len(items))
	for index := range items {
		item := items[index]
		if item.Source != model.BillingEventSourceLedgerRepair || item.EventType != model.BillingEventTypeAudit {
			continue
		}
		auditIds = append(auditIds, item.Id)
		auditIndexes[item.Id] = append(auditIndexes[item.Id], index)
	}
	if len(auditIds) == 0 {
		return nil
	}

	relations, err := model.ListBillingEventRelationsBySourceIds(auditIds)
	if err != nil {
		return err
	}
	if len(relations) > 0 {
		targetIds := make([]int64, 0, len(relations))
		targetIdSet := make(map[int64]struct{}, len(relations))
		targetByAuditId := make(map[int64]int64, len(relations))
		for _, relation := range relations {
			if _, ok := auditIndexes[relation.SourceEventId]; !ok {
				continue
			}
			if _, ok := targetByAuditId[relation.SourceEventId]; ok {
				continue
			}
			targetByAuditId[relation.SourceEventId] = relation.TargetEventId
			if _, ok := targetIdSet[relation.TargetEventId]; ok {
				continue
			}
			targetIdSet[relation.TargetEventId] = struct{}{}
			targetIds = append(targetIds, relation.TargetEventId)
		}

		if len(targetIds) > 0 {
			var targets []model.BillingEvent
			if err := model.DB.Where("id IN ?", targetIds).Find(&targets).Error; err != nil {
				return err
			}
			targetsById := make(map[int64]dto.BillingEventTargetLink, len(targets))
			for _, target := range targets {
				targetsById[target.Id] = billingEventTargetLink(target)
			}
			for auditId, targetId := range targetByAuditId {
				link, ok := targetsById[targetId]
				if !ok {
					continue
				}
				for _, index := range auditIndexes[auditId] {
					targetCopy := link
					items[index].RelatedTargetEvent = &targetCopy
				}
			}
		}
	}
	return attachBillingEventTargetLinksFromMetadata(items)
}

func attachBillingEventMCPToolCallLinks(items []dto.BillingEventItem) error {
	callIds := make([]int64, 0, len(items))
	indexesByCallId := make(map[int64][]int, len(items))
	for index := range items {
		item := items[index]
		if item.Source != model.BillingEventSourceMCPToolCall {
			continue
		}
		callId, err := strconv.ParseInt(strings.TrimSpace(item.SourceId), 10, 64)
		if err != nil || callId <= 0 {
			continue
		}
		if len(indexesByCallId[callId]) == 0 {
			callIds = append(callIds, callId)
		}
		indexesByCallId[callId] = append(indexesByCallId[callId], index)
	}
	if len(callIds) == 0 {
		return nil
	}
	callsById, err := model.ListMCPToolCallsByIds(callIds)
	if err != nil {
		return err
	}
	for callId, indexes := range indexesByCallId {
		call, ok := callsById[callId]
		if !ok {
			continue
		}
		link := billingEventMCPToolCallLink(call)
		for _, index := range indexes {
			linkCopy := link
			items[index].RelatedMCPToolCall = &linkCopy
		}
	}
	return nil
}

func attachBillingEventTargetLinksFromMetadata(items []dto.BillingEventItem) error {
	targetIds := make(map[int64][]int)
	for index := range items {
		item := items[index]
		if item.Source != model.BillingEventSourceLedgerRepair || item.EventType != model.BillingEventTypeAudit {
			continue
		}
		if item.RelatedTargetEvent != nil {
			continue
		}
		metadata := billingEventAuditMetadata{}
		if err := common.UnmarshalJsonStr(item.Metadata, &metadata); err != nil {
			continue
		}
		targetId := billingEventAuditTargetId(metadata)
		if targetId <= 0 {
			continue
		}
		targetIds[targetId] = append(targetIds[targetId], index)
	}
	if len(targetIds) == 0 {
		return nil
	}

	ids := make([]int64, 0, len(targetIds))
	for id := range targetIds {
		ids = append(ids, id)
	}
	var targets []model.BillingEvent
	if err := model.DB.Where("id IN ?", ids).Find(&targets).Error; err != nil {
		return err
	}
	for _, target := range targets {
		link := billingEventTargetLink(target)
		for _, index := range targetIds[target.Id] {
			targetCopy := link
			items[index].RelatedTargetEvent = &targetCopy
		}
	}
	return nil
}

func billingEventAuditTargetId(metadata billingEventAuditMetadata) int64 {
	if metadata.TargetEventPk > 0 {
		return metadata.TargetEventPk
	}
	if metadata.CreatedEventPk > 0 {
		return metadata.CreatedEventPk
	}
	if metadata.CreatedEvent != nil {
		return metadata.CreatedEvent.Id
	}
	return 0
}

func markBillingEventAuditLinked(linkedAuditIds map[int64]map[int64]struct{}, targetId int64, auditId int64) bool {
	if targetId <= 0 || auditId <= 0 {
		return false
	}
	if linkedAuditIds[targetId] == nil {
		linkedAuditIds[targetId] = make(map[int64]struct{})
	}
	if _, ok := linkedAuditIds[targetId][auditId]; ok {
		return false
	}
	linkedAuditIds[targetId][auditId] = struct{}{}
	return true
}

func billingEventAuditLinkFromRelation(audit model.BillingEvent, relation model.BillingEventRelation) dto.BillingEventAuditLink {
	metadata := billingEventAuditMetadata{}
	if relation.Reason == "" || relation.Label == "" || relation.AdminId == 0 {
		_ = common.UnmarshalJsonStr(audit.Metadata, &metadata)
	}
	reason := relation.Reason
	if reason == "" {
		reason = metadata.Reason
	}
	label := relation.Label
	if label == "" {
		label = metadata.Label
	}
	adminId := relation.AdminId
	if adminId == 0 {
		adminId = metadata.AdminId
	}
	return dto.BillingEventAuditLink{
		Id:        audit.Id,
		EventId:   audit.EventId,
		SourceId:  audit.SourceId,
		PriceUnit: audit.PriceUnit,
		Reason:    reason,
		Label:     label,
		AdminId:   adminId,
		CreatedAt: audit.CreatedAt,
	}
}

func billingEventAuditLinkFromMetadata(audit model.BillingEvent, metadata billingEventAuditMetadata) dto.BillingEventAuditLink {
	return dto.BillingEventAuditLink{
		Id:        audit.Id,
		EventId:   audit.EventId,
		SourceId:  audit.SourceId,
		PriceUnit: audit.PriceUnit,
		Reason:    metadata.Reason,
		Label:     metadata.Label,
		AdminId:   metadata.AdminId,
		CreatedAt: audit.CreatedAt,
	}
}

func billingEventTargetLink(event model.BillingEvent) dto.BillingEventTargetLink {
	return dto.BillingEventTargetLink{
		Id:          event.Id,
		EventId:     event.EventId,
		UserId:      event.UserId,
		Source:      event.Source,
		SourceId:    event.SourceId,
		EventType:   event.EventType,
		Status:      event.Status,
		PriceUnit:   event.PriceUnit,
		AmountQuota: event.AmountQuota,
		QuotaDelta:  event.QuotaDelta,
		CreatedAt:   event.CreatedAt,
	}
}

func billingEventMCPToolCallLink(call model.MCPToolCall) dto.BillingEventMCPToolCallLink {
	return dto.BillingEventMCPToolCallLink{
		Id:              call.Id,
		ToolId:          call.ToolId,
		ToolName:        call.ToolName,
		RequestId:       call.RequestId,
		Status:          call.Status,
		ErrorCode:       call.ErrorCode,
		ErrorMessage:    call.ErrorMessage,
		Metadata:        call.Metadata,
		BridgeSessionId: call.BridgeSessionId,
		TargetClient:    call.TargetClient,
		DurationMS:      call.DurationMS,
		ResultSize:      call.ResultSize,
		CreatedAt:       call.CreatedAt,
	}
}

func GetBillingEventHealth(params BillingEventReconciliationParams) (dto.BillingEventHealthResponse, error) {
	limit := normalizeBillingEventBackfillLimit(params.Limit)
	sources, err := normalizeBillingEventBackfillSources(params.Sources)
	if err != nil {
		return dto.BillingEventHealthResponse{}, err
	}

	backfill, err := BackfillBillingEvents(BillingEventBackfillParams{
		Sources: sources,
		Limit:   limit,
		DryRun:  true,
	})
	if err != nil {
		return dto.BillingEventHealthResponse{}, err
	}
	reconciliation, err := ReconcileBillingEvents(BillingEventReconciliationParams{
		Sources: sources,
		Limit:   limit,
	})
	if err != nil {
		return dto.BillingEventHealthResponse{}, err
	}

	response := dto.BillingEventHealthResponse{
		Limit:            limit,
		Sources:          sources,
		CheckedAt:        common.GetTimestamp(),
		TotalWouldCreate: backfill.TotalWouldCreate,
		TotalMissing:     reconciliation.TotalMissing,
		TotalMismatched:  reconciliation.TotalMismatched,
		TotalInvalid:     reconciliation.TotalInvalid,
		TotalErrorCount:  backfill.TotalErrorCount + reconciliation.TotalErrorCount,
		Backfill:         backfill,
		Reconciliation:   reconciliation,
	}
	response.NeedsReview = response.TotalWouldCreate > 0 ||
		response.TotalMissing > 0 ||
		response.TotalMismatched > 0 ||
		response.TotalInvalid > 0 ||
		response.TotalErrorCount > 0
	return response, nil
}

func billingEventToDTO(event model.BillingEvent) dto.BillingEventItem {
	return dto.BillingEventItem{
		Id:            event.Id,
		EventId:       event.EventId,
		UserId:        event.UserId,
		TokenId:       event.TokenId,
		Source:        event.Source,
		SourceId:      event.SourceId,
		EventType:     event.EventType,
		Status:        event.Status,
		RequestId:     event.RequestId,
		Group:         event.Group,
		BillingSource: event.BillingSource,
		PriceUnit:     event.PriceUnit,
		Currency:      event.Currency,
		AmountQuota:   event.AmountQuota,
		QuotaDelta:    event.QuotaDelta,
		Cost:          event.Cost,
		Metadata:      event.Metadata,
		CreatedAt:     event.CreatedAt,
	}
}
