package model

import (
	"strings"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	BillingEventSourceMCPToolCall  = "mcp_tool_call"
	BillingEventSourceTunnelMCP    = "tunnel_mcp"
	BillingEventSourceTunnelHTTP   = "tunnel_http"
	BillingEventSourceTunnelTCP    = "tunnel_tcp"
	BillingEventSourceModelRequest = "model_request"
	BillingEventSourceAsyncTask    = "async_task"
	BillingEventSourceViolationFee = "violation_fee"
	BillingEventSourceWalletTopUp  = "wallet_topup"
	BillingEventSourceWalletAdjust = "wallet_adjust"
	BillingEventSourceSubscription = "subscription"
	BillingEventSourceLedgerRepair = "billing_event_repair"

	BillingEventTypeDebit  = "debit"
	BillingEventTypeCredit = "credit"
	BillingEventTypeAudit  = "audit"

	BillingEventStatusSettled = "settled"

	BillingEventUsageKindText       = "text"
	BillingEventUsageKindAudio      = "audio"
	BillingEventUsageKindRealtime   = "realtime"
	BillingEventUsageKindMidjourney = "midjourney"
	BillingEventUsageKindTunnel     = "tunnel"
)

type BillingEvent struct {
	Id int64 `json:"id"`

	EventId   string `json:"event_id" gorm:"type:varchar(128);not null;uniqueIndex"`
	SubsiteId int64  `json:"subsite_id" gorm:"not null;default:0;index"`
	UserId    int    `json:"user_id" gorm:"not null;index"`
	TokenId   int    `json:"token_id" gorm:"index"`

	Source    string `json:"source" gorm:"type:varchar(64);not null;index:idx_billing_events_source_ref,priority:1"`
	SourceId  string `json:"source_id" gorm:"type:varchar(128);not null;index:idx_billing_events_source_ref,priority:2"`
	EventType string `json:"event_type" gorm:"type:varchar(32);not null;index"`
	Status    string `json:"status" gorm:"type:varchar(32);not null;default:'settled';index"`

	RequestId string `json:"request_id" gorm:"type:varchar(128);default:'';index"`
	Group     string `json:"group" gorm:"type:varchar(64);default:'';index"`

	BillingSource string `json:"billing_source" gorm:"type:varchar(64);default:'';index"`
	PriceUnit     string `json:"price_unit" gorm:"type:varchar(32);default:''"`
	Currency      string `json:"currency" gorm:"type:varchar(16);default:'quota'"`

	AmountQuota int     `json:"amount_quota" gorm:"not null;default:0"`
	QuotaDelta  int     `json:"quota_delta" gorm:"not null;default:0"`
	Cost        float64 `json:"cost" gorm:"type:decimal(18,8);not null;default:0"`

	Metadata  string `json:"metadata" gorm:"type:text"`
	CreatedAt int64  `json:"created_at" gorm:"bigint;index"`
}

func (BillingEvent) TableName() string {
	return "billing_events"
}

func (event *BillingEvent) BeforeCreate(tx *gorm.DB) error {
	if event.CreatedAt == 0 {
		event.CreatedAt = common.GetTimestamp()
	}
	if event.Status == "" {
		event.Status = BillingEventStatusSettled
	}
	if event.Currency == "" {
		event.Currency = "quota"
	}
	return nil
}

type BillingEventFilter struct {
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
}

type BillingEventAggregate struct {
	TotalEvents      int64   `gorm:"column:total_events"`
	CreditEvents     int64   `gorm:"column:credit_events"`
	DebitEvents      int64   `gorm:"column:debit_events"`
	AuditEvents      int64   `gorm:"column:audit_events"`
	AmountQuota      int64   `gorm:"column:amount_quota"`
	NetQuotaDelta    int64   `gorm:"column:net_quota_delta"`
	CreditQuotaDelta int64   `gorm:"column:credit_quota_delta"`
	DebitQuotaDelta  int64   `gorm:"column:debit_quota_delta"`
	TotalCost        float64 `gorm:"column:total_cost"`
}

type BillingEventDimensionAggregate struct {
	Key string `gorm:"column:dimension_key"`
	BillingEventAggregate
}

type BillingEventTrendAggregate struct {
	BucketStart int64 `gorm:"column:bucket_start"`
	BillingEventAggregate
}

type BillingEventSummary struct {
	Totals     BillingEventAggregate
	BySource   []BillingEventDimensionAggregate
	ByType     []BillingEventDimensionAggregate
	DailyTrend []BillingEventTrendAggregate
}

func CreateBillingEventIfNotExists(tx *gorm.DB, event *BillingEvent) (bool, error) {
	if tx == nil {
		tx = DB
	}
	result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(event)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func ListBillingEvents(filter BillingEventFilter, offset int, limit int) ([]BillingEvent, int64, error) {
	query := DB.Model(&BillingEvent{})
	query = applyBillingEventFilter(query, filter)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var events []BillingEvent
	err := query.Order("created_at desc, id desc").
		Limit(limit).
		Offset(offset).
		Find(&events).Error
	return events, total, err
}

func SummarizeBillingEvents(filter BillingEventFilter, bucketSeconds int64) (BillingEventSummary, error) {
	if bucketSeconds <= 0 {
		bucketSeconds = 86400
	}
	summary := BillingEventSummary{}
	if err := billingEventSummaryQuery(filter).
		Select(billingEventAggregateSelect(), billingEventAggregateArgs()...).
		Scan(&summary.Totals).Error; err != nil {
		return summary, err
	}
	if err := billingEventSummaryQuery(filter).
		Select("source AS dimension_key, "+billingEventAggregateSelect(), billingEventAggregateArgs()...).
		Group("source").
		Order("total_events DESC, dimension_key ASC").
		Limit(12).
		Scan(&summary.BySource).Error; err != nil {
		return summary, err
	}
	if err := billingEventSummaryQuery(filter).
		Select("event_type AS dimension_key, "+billingEventAggregateSelect(), billingEventAggregateArgs()...).
		Group("event_type").
		Order("total_events DESC, dimension_key ASC").
		Scan(&summary.ByType).Error; err != nil {
		return summary, err
	}
	bucketExpression := "created_at - (created_at % ?)"
	args := append([]any{bucketSeconds}, billingEventAggregateArgs()...)
	if err := billingEventSummaryQuery(filter).
		Select(bucketExpression+" AS bucket_start, "+billingEventAggregateSelect(), args...).
		Group("bucket_start").
		Order("bucket_start ASC").
		Scan(&summary.DailyTrend).Error; err != nil {
		return summary, err
	}
	return summary, nil
}

func billingEventSummaryQuery(filter BillingEventFilter) *gorm.DB {
	return applyBillingEventFilter(DB.Model(&BillingEvent{}), filter)
}

func billingEventAggregateSelect() string {
	return strings.Join([]string{
		"COUNT(*) AS total_events",
		"COALESCE(SUM(CASE WHEN event_type = ? THEN 1 ELSE 0 END), 0) AS credit_events",
		"COALESCE(SUM(CASE WHEN event_type = ? THEN 1 ELSE 0 END), 0) AS debit_events",
		"COALESCE(SUM(CASE WHEN event_type = ? THEN 1 ELSE 0 END), 0) AS audit_events",
		"COALESCE(SUM(amount_quota), 0) AS amount_quota",
		"COALESCE(SUM(quota_delta), 0) AS net_quota_delta",
		"COALESCE(SUM(CASE WHEN event_type = ? THEN quota_delta ELSE 0 END), 0) AS credit_quota_delta",
		"COALESCE(SUM(CASE WHEN event_type = ? THEN quota_delta ELSE 0 END), 0) AS debit_quota_delta",
		"COALESCE(SUM(cost), 0) AS total_cost",
	}, ", ")
}

func billingEventAggregateArgs() []any {
	return []any{
		BillingEventTypeCredit,
		BillingEventTypeDebit,
		BillingEventTypeAudit,
		BillingEventTypeCredit,
		BillingEventTypeDebit,
	}
}

func applyBillingEventFilter(query *gorm.DB, filter BillingEventFilter) *gorm.DB {
	if filter.UserId > 0 {
		query = query.Where("user_id = ?", filter.UserId)
	}
	if filter.TokenId > 0 {
		query = query.Where("token_id = ?", filter.TokenId)
	}
	if strings.TrimSpace(filter.Source) != "" {
		query = query.Where("source = ?", strings.TrimSpace(filter.Source))
	}
	if strings.TrimSpace(filter.SourceId) != "" {
		query = query.Where("source_id = ?", strings.TrimSpace(filter.SourceId))
	}
	if strings.TrimSpace(filter.EventType) != "" {
		query = query.Where("event_type = ?", strings.TrimSpace(filter.EventType))
	}
	if strings.TrimSpace(filter.Status) != "" {
		query = query.Where("status = ?", strings.TrimSpace(filter.Status))
	}
	if strings.TrimSpace(filter.RequestId) != "" {
		query = query.Where("request_id = ?", strings.TrimSpace(filter.RequestId))
	}
	if strings.TrimSpace(filter.BillingSource) != "" {
		query = query.Where("billing_source = ?", strings.TrimSpace(filter.BillingSource))
	}
	if strings.TrimSpace(filter.UsageKind) != "" {
		usageKind := normalizeBillingEventUsageKindFilter(filter.UsageKind)
		if usageKind == "" {
			query = query.Where("1 = 0")
		} else {
			query = query.Where("metadata LIKE ?", "%\"usage_kind\":\""+usageKind+"\"%")
		}
	}
	if filter.StartTime > 0 {
		query = query.Where("created_at >= ?", filter.StartTime)
	}
	if filter.EndTime > 0 {
		query = query.Where("created_at <= ?", filter.EndTime)
	}
	if strings.TrimSpace(filter.Keyword) != "" {
		keyword := "%" + strings.TrimSpace(filter.Keyword) + "%"
		query = query.Where("event_id LIKE ? OR source_id LIKE ? OR request_id LIKE ? OR metadata LIKE ?",
			keyword, keyword, keyword, keyword)
	}
	return query
}

func normalizeBillingEventUsageKindFilter(usageKind string) string {
	switch strings.ToLower(strings.TrimSpace(usageKind)) {
	case BillingEventUsageKindText:
		return BillingEventUsageKindText
	case BillingEventUsageKindAudio:
		return BillingEventUsageKindAudio
	case BillingEventUsageKindRealtime:
		return BillingEventUsageKindRealtime
	case BillingEventUsageKindMidjourney:
		return BillingEventUsageKindMidjourney
	case BillingEventUsageKindTunnel:
		return BillingEventUsageKindTunnel
	default:
		return ""
	}
}
