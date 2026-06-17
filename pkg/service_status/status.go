package servicestatus

import (
	"math"
	"sort"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	perfmetrics "github.com/QuantumNous/new-api/pkg/perf_metrics"
	"github.com/QuantumNous/new-api/setting/perf_metrics_setting"
)

const (
	StatusNormal        = "normal"
	StatusDegraded      = "degraded"
	StatusOutage        = "outage"
	StatusNoTraffic     = "no_traffic"
	StatusOnlyProbe     = "only_probe"
	StatusLowConfidence = "low_confidence"
	StatusUnknown       = "unknown"

	SignalObserved      = "observed"
	SignalNotObserved   = "not_observed"
	SignalNotConfigured = "not_configured"

	ConfidenceNone = "none"
	ConfidenceLow  = "low"
	ConfidenceHigh = "high"
)

const (
	defaultWindowHours    = 24
	maxWindowHours        = 24 * 7
	minReliableSamples    = int64(10)
	minBucketSamples      = int64(3)
	outageSuccessRate     = 95.0
	degradedSuccessRate   = 99.0
	degradedLatencyMs     = int64(8000)
	degradedTTFTMs        = int64(3000)
	maxConfiguredModels   = 8
	maxObservedModelNames = 8
)

type QueryOptions struct {
	Hours         int
	IncludeAlerts bool
}

type Result struct {
	Summary       Summary         `json:"summary"`
	Channels      []ChannelStatus `json:"channels"`
	Alerts        []Alert         `json:"alerts,omitempty"`
	WindowHours   int             `json:"window_hours"`
	BucketSeconds int64           `json:"bucket_seconds"`
	UpdatedAt     int64           `json:"updated_at"`
}

type Summary struct {
	TotalChannels int `json:"total_channels"`
	Normal        int `json:"normal"`
	Degraded      int `json:"degraded"`
	Outage        int `json:"outage"`
	NoTraffic     int `json:"no_traffic"`
	OnlyProbe     int `json:"only_probe"`
	LowConfidence int `json:"low_confidence"`
	Unknown       int `json:"unknown"`
	ActiveAlerts  int `json:"active_alerts"`
}

type ChannelStatus struct {
	ChannelId        int           `json:"channel_id"`
	ChannelName      string        `json:"channel_name"`
	ChannelType      int           `json:"channel_type"`
	Group            string        `json:"group"`
	ConfiguredStatus int           `json:"configured_status"`
	Status           string        `json:"status"`
	Confidence       string        `json:"confidence"`
	RequestCount     int64         `json:"request_count"`
	SuccessRate      float64       `json:"success_rate"`
	AvgLatencyMs     int64         `json:"avg_latency_ms"`
	AvgTtftMs        int64         `json:"avg_ttft_ms"`
	LastObservedAt   int64         `json:"last_observed_at"`
	ConfiguredModels []string      `json:"configured_models"`
	ObservedModels   []string      `json:"observed_models"`
	Signals          Signals       `json:"signals"`
	Series           []SeriesPoint `json:"series"`
}

type Signals struct {
	RealTraffic  string `json:"real_traffic"`
	Probe        string `json:"probe"`
	Connectivity string `json:"connectivity"`
}

type SeriesPoint struct {
	Ts           int64   `json:"ts"`
	Status       string  `json:"status"`
	RequestCount int64   `json:"request_count"`
	SuccessRate  float64 `json:"success_rate"`
	AvgLatencyMs int64   `json:"avg_latency_ms"`
	AvgTtftMs    int64   `json:"avg_ttft_ms"`
}

type Alert struct {
	Severity       string  `json:"severity"`
	ChannelId      int     `json:"channel_id"`
	ChannelName    string  `json:"channel_name"`
	Status         string  `json:"status"`
	RequestCount   int64   `json:"request_count"`
	SuccessRate    float64 `json:"success_rate"`
	AvgLatencyMs   int64   `json:"avg_latency_ms"`
	LastObservedAt int64   `json:"last_observed_at"`
}

type aggregate struct {
	requestCount   int64
	successCount   int64
	totalLatencyMs int64
	ttftSumMs      int64
	ttftCount      int64
	lastObservedAt int64
	models         map[string]struct{}
}

func Query(options QueryOptions) (Result, error) {
	hours := normalizeHours(options.Hours)
	now := time.Now().Unix()
	startTs := now - int64(hours)*3600

	channels, err := model.GetChannelsForServiceStatus()
	if err != nil {
		return Result{}, err
	}

	rows, err := model.GetPerfChannelMetrics(startTs, now)
	if err != nil {
		return Result{}, err
	}

	totals := make(map[int]*aggregate)
	series := make(map[int]map[int64]*aggregate)
	for _, row := range rows {
		mergeMetric(totals, series, row.ChannelId, row.ModelName, row.BucketTs, aggregate{
			requestCount:   row.RequestCount,
			successCount:   row.SuccessCount,
			totalLatencyMs: row.TotalLatencyMs,
			ttftSumMs:      row.TtftSumMs,
			ttftCount:      row.TtftCount,
		})
	}
	for _, hot := range perfmetrics.ChannelHotBucketSnapshots(startTs, now) {
		mergeMetric(totals, series, hot.ChannelId, hot.ModelName, hot.BucketTs, aggregate{
			requestCount:   hot.RequestCount,
			successCount:   hot.SuccessCount,
			totalLatencyMs: hot.TotalLatencyMs,
			ttftSumMs:      hot.TtftSumMs,
			ttftCount:      hot.TtftCount,
		})
	}

	result := Result{
		Channels:      make([]ChannelStatus, 0, len(channels)),
		WindowHours:   hours,
		BucketSeconds: perf_metrics_setting.GetBucketSeconds(),
		UpdatedAt:     now,
	}

	for _, channel := range channels {
		status := buildChannelStatus(channel, totals[channel.Id], series[channel.Id])
		result.Summary.TotalChannels++
		addSummaryStatus(&result.Summary, status.Status)
		if options.IncludeAlerts && channel.Status == common.ChannelStatusEnabled && isAlertStatus(status.Status) {
			alert := Alert{
				Severity:       alertSeverity(status.Status),
				ChannelId:      status.ChannelId,
				ChannelName:    status.ChannelName,
				Status:         status.Status,
				RequestCount:   status.RequestCount,
				SuccessRate:    status.SuccessRate,
				AvgLatencyMs:   status.AvgLatencyMs,
				LastObservedAt: status.LastObservedAt,
			}
			result.Alerts = append(result.Alerts, alert)
		}
		result.Channels = append(result.Channels, status)
	}

	sort.SliceStable(result.Channels, func(i, j int) bool {
		left := statusRank(result.Channels[i].Status)
		right := statusRank(result.Channels[j].Status)
		if left != right {
			return left < right
		}
		if result.Channels[i].RequestCount != result.Channels[j].RequestCount {
			return result.Channels[i].RequestCount > result.Channels[j].RequestCount
		}
		return result.Channels[i].ChannelId > result.Channels[j].ChannelId
	})
	sort.SliceStable(result.Alerts, func(i, j int) bool {
		if result.Alerts[i].Severity != result.Alerts[j].Severity {
			return result.Alerts[i].Severity == "critical"
		}
		return result.Alerts[i].RequestCount > result.Alerts[j].RequestCount
	})
	result.Summary.ActiveAlerts = len(result.Alerts)

	return result, nil
}

func normalizeHours(hours int) int {
	if hours <= 0 {
		return defaultWindowHours
	}
	if hours > maxWindowHours {
		return maxWindowHours
	}
	return hours
}

func mergeMetric(totals map[int]*aggregate, series map[int]map[int64]*aggregate, channelId int, modelName string, bucketTs int64, value aggregate) {
	if channelId <= 0 || value.requestCount == 0 {
		return
	}
	total := ensureAggregate(totals, channelId)
	addAggregate(total, value, bucketTs, modelName)

	if _, ok := series[channelId]; !ok {
		series[channelId] = make(map[int64]*aggregate)
	}
	point := ensureAggregate(series[channelId], bucketTs)
	addAggregate(point, value, bucketTs, modelName)
}

func ensureAggregate[T comparable](target map[T]*aggregate, key T) *aggregate {
	if target[key] == nil {
		target[key] = &aggregate{models: make(map[string]struct{})}
	}
	return target[key]
}

func addAggregate(target *aggregate, value aggregate, bucketTs int64, modelName string) {
	target.requestCount += value.requestCount
	target.successCount += value.successCount
	target.totalLatencyMs += value.totalLatencyMs
	target.ttftSumMs += value.ttftSumMs
	target.ttftCount += value.ttftCount
	if bucketTs > target.lastObservedAt {
		target.lastObservedAt = bucketTs
	}
	modelName = strings.TrimSpace(modelName)
	if modelName != "" {
		target.models[modelName] = struct{}{}
	}
}

func buildChannelStatus(channel *model.Channel, total *aggregate, points map[int64]*aggregate) ChannelStatus {
	if total == nil {
		total = &aggregate{models: make(map[string]struct{})}
	}
	status := classify(total, minReliableSamples)
	return ChannelStatus{
		ChannelId:        channel.Id,
		ChannelName:      channel.Name,
		ChannelType:      channel.Type,
		Group:            channel.Group,
		ConfiguredStatus: channel.Status,
		Status:           status,
		Confidence:       confidence(total),
		RequestCount:     total.requestCount,
		SuccessRate:      roundPercent(successRate(total)),
		AvgLatencyMs:     avg(total.totalLatencyMs, total.requestCount),
		AvgTtftMs:        avg(total.ttftSumMs, total.ttftCount),
		LastObservedAt:   total.lastObservedAt,
		ConfiguredModels: splitModels(channel.Models, maxConfiguredModels),
		ObservedModels:   aggregateModelNames(total, maxObservedModelNames),
		Signals: Signals{
			RealTraffic:  realTrafficSignal(total),
			Probe:        SignalNotConfigured,
			Connectivity: SignalNotConfigured,
		},
		Series: buildSeries(points),
	}
}

func buildSeries(points map[int64]*aggregate) []SeriesPoint {
	if len(points) == 0 {
		return []SeriesPoint{}
	}
	timestamps := make([]int64, 0, len(points))
	for ts := range points {
		timestamps = append(timestamps, ts)
	}
	sort.Slice(timestamps, func(i, j int) bool { return timestamps[i] < timestamps[j] })

	series := make([]SeriesPoint, 0, len(timestamps))
	for _, ts := range timestamps {
		point := points[ts]
		series = append(series, SeriesPoint{
			Ts:           ts,
			Status:       classify(point, minBucketSamples),
			RequestCount: point.requestCount,
			SuccessRate:  roundPercent(successRate(point)),
			AvgLatencyMs: avg(point.totalLatencyMs, point.requestCount),
			AvgTtftMs:    avg(point.ttftSumMs, point.ttftCount),
		})
	}
	return series
}

func classify(value *aggregate, minSamples int64) string {
	if value == nil || value.requestCount == 0 {
		return StatusNoTraffic
	}
	if value.requestCount < minSamples {
		return StatusLowConfidence
	}
	rate := successRate(value)
	avgLatency := avg(value.totalLatencyMs, value.requestCount)
	avgTtft := avg(value.ttftSumMs, value.ttftCount)
	if rate < outageSuccessRate {
		return StatusOutage
	}
	if rate < degradedSuccessRate || avgLatency > degradedLatencyMs || avgTtft > degradedTTFTMs {
		return StatusDegraded
	}
	return StatusNormal
}

func confidence(value *aggregate) string {
	if value == nil || value.requestCount == 0 {
		return ConfidenceNone
	}
	if value.requestCount < minReliableSamples {
		return ConfidenceLow
	}
	return ConfidenceHigh
}

func successRate(value *aggregate) float64 {
	if value == nil || value.requestCount == 0 {
		return 0
	}
	return float64(value.successCount) / float64(value.requestCount) * 100
}

func avg(sum int64, count int64) int64 {
	if count <= 0 {
		return 0
	}
	return sum / count
}

func roundPercent(value float64) float64 {
	return math.Round(value*100) / 100
}

func realTrafficSignal(value *aggregate) string {
	if value == nil || value.requestCount == 0 {
		return SignalNotObserved
	}
	return SignalObserved
}

func isAlertStatus(status string) bool {
	return status == StatusOutage || status == StatusDegraded
}

func alertSeverity(status string) string {
	if status == StatusOutage {
		return "critical"
	}
	return "warning"
}

func addSummaryStatus(summary *Summary, status string) {
	switch status {
	case StatusNormal:
		summary.Normal++
	case StatusDegraded:
		summary.Degraded++
	case StatusOutage:
		summary.Outage++
	case StatusNoTraffic:
		summary.NoTraffic++
	case StatusOnlyProbe:
		summary.OnlyProbe++
	case StatusLowConfidence:
		summary.LowConfidence++
	default:
		summary.Unknown++
	}
}

func statusRank(status string) int {
	switch status {
	case StatusOutage:
		return 0
	case StatusDegraded:
		return 1
	case StatusLowConfidence:
		return 2
	case StatusNoTraffic:
		return 3
	case StatusOnlyProbe:
		return 4
	case StatusUnknown:
		return 5
	case StatusNormal:
		return 6
	default:
		return 7
	}
}

func splitModels(raw string, limit int) []string {
	items := strings.Split(raw, ",")
	models := make([]string, 0, len(items))
	seen := make(map[string]struct{})
	for _, item := range items {
		name := strings.TrimSpace(item)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		models = append(models, name)
		if limit > 0 && len(models) >= limit {
			break
		}
	}
	return models
}

func aggregateModelNames(value *aggregate, limit int) []string {
	if value == nil || len(value.models) == 0 {
		return []string{}
	}
	models := make([]string, 0, len(value.models))
	for name := range value.models {
		models = append(models, name)
	}
	sort.Strings(models)
	if limit > 0 && len(models) > limit {
		return models[:limit]
	}
	return models
}
