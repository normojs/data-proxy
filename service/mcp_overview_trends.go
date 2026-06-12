package service

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
)

const (
	defaultMCPSummaryTrendBucketSeconds = 60 * 60
	maxMCPSummaryTrendBuckets           = 48
	mcpSummaryProxyErrorTopLimit        = 5
)

type MCPSummaryOperationsTrendParams struct {
	UserId        int
	StartTime     int64
	EndTime       int64
	BucketSeconds int64
}

func BuildMCPSummaryOperationsTrends(params MCPSummaryOperationsTrendParams) (*dto.MCPSummaryOperationsTrends, error) {
	now := common.GetTimestamp()
	startTime := params.StartTime
	endTime := params.EndTime
	if endTime <= 0 {
		endTime = now
	}
	if startTime <= 0 || startTime > endTime {
		startTime = endTime - normalizeMCPSummaryWindow(0)
	}
	bucketSeconds := normalizeMCPSummaryTrendBucket(params.BucketSeconds, startTime, endTime)
	bucketStarts := mcpSummaryTrendBucketStarts(startTime, endTime, bucketSeconds)

	bridgeOnline, err := buildMCPSummaryBridgeOnlineTrend(params.UserId, startTime, endTime, bucketSeconds, bucketStarts)
	if err != nil {
		return nil, err
	}
	openAPIStorage, err := buildMCPSummaryOpenAPIStorageTrend(params.UserId, startTime, endTime, now, bucketSeconds, bucketStarts)
	if err != nil {
		return nil, err
	}
	proxyErrorTopN, err := buildMCPSummaryProxyErrorTopN(params.UserId, startTime, endTime)
	if err != nil {
		return nil, err
	}
	billingAnomalies, err := buildMCPSummaryBillingAnomalies(params.UserId, startTime, endTime, bucketSeconds)
	if err != nil {
		return nil, err
	}

	return &dto.MCPSummaryOperationsTrends{
		StartTime:        startTime,
		EndTime:          endTime,
		BucketSeconds:    bucketSeconds,
		CheckedAt:        now,
		BridgeOnline:     bridgeOnline,
		OpenAPIStorage:   openAPIStorage,
		ProxyErrorTopN:   proxyErrorTopN,
		BillingAnomalies: billingAnomalies,
	}, nil
}

func buildMCPSummaryBridgeOnlineTrend(userId int, startTime int64, endTime int64, bucketSeconds int64, bucketStarts []int64) ([]dto.MCPSummaryBridgeTrendBucket, error) {
	buckets := make([]dto.MCPSummaryBridgeTrendBucket, 0, len(bucketStarts))
	for _, bucketStart := range bucketStarts {
		bucketEnd := bucketStart + bucketSeconds - 1
		if bucketEnd > endTime {
			bucketEnd = endTime
		}
		stats, err := model.GetBridgeSessionTrendBucketStats(userId, bucketStart, bucketEnd)
		if err != nil {
			return nil, err
		}
		buckets = append(buckets, dto.MCPSummaryBridgeTrendBucket{
			BucketStart:     bucketStart,
			OnlineClients:   stats.OnlineClients,
			StartedSessions: stats.StartedSessions,
			ClosedSessions:  stats.ClosedSessions,
		})
	}
	return buckets, nil
}

func buildMCPSummaryOpenAPIStorageTrend(userId int, startTime int64, endTime int64, now int64, bucketSeconds int64, bucketStarts []int64) ([]dto.MCPSummaryOpenAPIStorageBucket, error) {
	rawBuckets, err := model.ListMCPOpenAPIBinaryObjectTrend(model.MCPOpenAPIBinaryObjectFilter{
		UserId:    userId,
		StartTime: startTime,
		EndTime:   endTime,
		Now:       now,
	}, bucketSeconds)
	if err != nil {
		return nil, err
	}
	byStart := make(map[int64]model.MCPOpenAPIBinaryObjectTrendBucket, len(rawBuckets))
	for _, bucket := range rawBuckets {
		byStart[bucket.BucketStart] = bucket
	}
	buckets := make([]dto.MCPSummaryOpenAPIStorageBucket, 0, len(bucketStarts))
	for _, bucketStart := range bucketStarts {
		raw := byStart[bucketStart]
		buckets = append(buckets, dto.MCPSummaryOpenAPIStorageBucket{
			BucketStart:   bucketStart,
			ObjectCount:   raw.ObjectCount,
			TotalBytes:    raw.TotalBytes,
			ExpiredCount:  raw.ExpiredCount,
			DownloadCount: raw.DownloadCount,
		})
	}
	return buckets, nil
}

func buildMCPSummaryProxyErrorTopN(userId int, startTime int64, endTime int64) ([]dto.MCPSummaryProxyErrorTool, error) {
	stats, err := model.ListMCPProxyErrorToolStats(model.MCPProxyTrendFilter{
		UserId:    userId,
		StartTime: startTime,
		EndTime:   endTime,
	}, mcpSummaryProxyErrorTopLimit)
	if err != nil {
		return nil, err
	}
	items := make([]dto.MCPSummaryProxyErrorTool, 0, len(stats))
	for _, stat := range stats {
		items = append(items, dto.MCPSummaryProxyErrorTool{
			ProxyServerId:      stat.ProxyServerId,
			ProxyToolId:        stat.ProxyToolId,
			ToolId:             stat.ToolId,
			ToolName:           stat.ExposedToolName,
			DownstreamToolName: stat.DownstreamToolName,
			TotalCalls:         stat.TotalCalls,
			SuccessCalls:       stat.SuccessCalls,
			ErrorCalls:         stat.ErrorCalls,
			TimeoutCalls:       stat.TimeoutCalls,
			SuccessRate:        ratioPercent(stat.SuccessCalls, stat.TotalCalls),
			AvgDurationMS:      stat.AvgDurationMS,
		})
	}
	return items, nil
}

func buildMCPSummaryBillingAnomalies(userId int, startTime int64, endTime int64, bucketSeconds int64) (dto.MCPSummaryBillingAnomalies, error) {
	callFilter := model.MCPToolCallFilter{
		UserId:    userId,
		StartTime: startTime,
		EndTime:   endTime,
	}
	callStats, err := model.GetMCPBillingAnomalyStats(callFilter)
	if err != nil {
		return dto.MCPSummaryBillingAnomalies{}, err
	}
	billingSummary, err := model.SummarizeBillingEvents(model.BillingEventFilter{
		UserId:    userId,
		Source:    model.BillingEventSourceMCPToolCall,
		StartTime: startTime,
		EndTime:   endTime,
	}, bucketSeconds)
	if err != nil {
		return dto.MCPSummaryBillingAnomalies{}, err
	}
	return dto.MCPSummaryBillingAnomalies{
		UnsettledSuccessCalls: callStats.UnsettledSuccessCalls,
		FailedChargedCalls:    callStats.FailedChargedCalls,
		MissingDebitEvents:    maxInt64(callStats.SettledChargedCalls-billingSummary.Totals.DebitEvents, 0),
		RefundEvents:          billingSummary.Totals.CreditEvents,
		RefundQuota:           billingSummary.Totals.CreditQuotaDelta,
		NetMCPQuotaDelta:      billingSummary.Totals.NetQuotaDelta,
	}, nil
}

func normalizeMCPSummaryTrendBucket(bucketSeconds int64, startTime int64, endTime int64) int64 {
	if bucketSeconds <= 0 {
		bucketSeconds = defaultMCPSummaryTrendBucketSeconds
	}
	if bucketSeconds < 60 {
		bucketSeconds = 60
	}
	window := endTime - startTime
	if window <= 0 {
		return bucketSeconds
	}
	for window/bucketSeconds > maxMCPSummaryTrendBuckets {
		bucketSeconds *= 2
	}
	return bucketSeconds
}

func mcpSummaryTrendBucketStarts(startTime int64, endTime int64, bucketSeconds int64) []int64 {
	if bucketSeconds <= 0 {
		bucketSeconds = defaultMCPSummaryTrendBucketSeconds
	}
	first := alignMCPSummaryTrendBucket(startTime, bucketSeconds)
	starts := []int64{}
	for bucketStart := first; bucketStart <= endTime; bucketStart += bucketSeconds {
		starts = append(starts, bucketStart)
	}
	if len(starts) == 0 {
		starts = append(starts, first)
	}
	return starts
}

func alignMCPSummaryTrendBucket(timestamp int64, bucketSeconds int64) int64 {
	if bucketSeconds <= 0 {
		return timestamp
	}
	return timestamp - (timestamp % bucketSeconds)
}
