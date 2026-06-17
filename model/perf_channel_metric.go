package model

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// PerfChannelMetric stores relay performance metrics with channel dimension.
// It intentionally lives beside PerfMetric instead of changing that table's
// unique index, so existing model performance dashboards remain compatible.
type PerfChannelMetric struct {
	Id             int    `json:"id" gorm:"primaryKey"`
	ChannelId      int    `json:"channel_id" gorm:"uniqueIndex:idx_perf_channel_model_group_bucket,priority:1;index:idx_perf_channel_id"`
	ModelName      string `json:"model_name" gorm:"size:128;uniqueIndex:idx_perf_channel_model_group_bucket,priority:2"`
	Group          string `json:"group" gorm:"column:group;size:64;uniqueIndex:idx_perf_channel_model_group_bucket,priority:3"`
	BucketTs       int64  `json:"bucket_ts" gorm:"uniqueIndex:idx_perf_channel_model_group_bucket,priority:4;index:idx_perf_channel_bucket_ts"`
	RequestCount   int64  `json:"-" gorm:"default:0"`
	SuccessCount   int64  `json:"-" gorm:"default:0"`
	TotalLatencyMs int64  `json:"-" gorm:"default:0"`
	TtftSumMs      int64  `json:"-" gorm:"default:0"`
	TtftCount      int64  `json:"-" gorm:"default:0"`
	OutputTokens   int64  `json:"-" gorm:"default:0"`
	GenerationMs   int64  `json:"-" gorm:"default:0"`
}

func (PerfChannelMetric) TableName() string {
	return "perf_channel_metrics"
}

func UpsertPerfChannelMetric(metric *PerfChannelMetric) error {
	if metric == nil || metric.ChannelId <= 0 || metric.RequestCount == 0 {
		return nil
	}
	return DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "channel_id"},
			{Name: "model_name"},
			{Name: "group"},
			{Name: "bucket_ts"},
		},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"request_count":    gorm.Expr("perf_channel_metrics.request_count + ?", metric.RequestCount),
			"success_count":    gorm.Expr("perf_channel_metrics.success_count + ?", metric.SuccessCount),
			"total_latency_ms": gorm.Expr("perf_channel_metrics.total_latency_ms + ?", metric.TotalLatencyMs),
			"ttft_sum_ms":      gorm.Expr("perf_channel_metrics.ttft_sum_ms + ?", metric.TtftSumMs),
			"ttft_count":       gorm.Expr("perf_channel_metrics.ttft_count + ?", metric.TtftCount),
			"output_tokens":    gorm.Expr("perf_channel_metrics.output_tokens + ?", metric.OutputTokens),
			"generation_ms":    gorm.Expr("perf_channel_metrics.generation_ms + ?", metric.GenerationMs),
		}),
	}).Create(metric).Error
}

func GetPerfChannelMetrics(startTs int64, endTs int64) ([]PerfChannelMetric, error) {
	var metrics []PerfChannelMetric
	err := DB.Model(&PerfChannelMetric{}).
		Where("bucket_ts >= ? AND bucket_ts <= ?", startTs, endTs).
		Order("bucket_ts ASC").
		Find(&metrics).Error
	return metrics, err
}

func DeletePerfChannelMetricsBefore(cutoffTs int64) error {
	if cutoffTs <= 0 {
		return nil
	}
	return DB.Where("bucket_ts < ?", cutoffTs).Delete(&PerfChannelMetric{}).Error
}
