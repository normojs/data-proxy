package dto

type BillingEventItem struct {
	Id int64 `json:"id"`

	EventId string `json:"event_id"`
	UserId  int    `json:"user_id"`
	TokenId int    `json:"token_id"`

	Source    string `json:"source"`
	SourceId  string `json:"source_id"`
	EventType string `json:"event_type"`
	Status    string `json:"status"`

	RequestId string `json:"request_id"`
	Group     string `json:"group"`

	BillingSource string `json:"billing_source"`
	PriceUnit     string `json:"price_unit"`
	Currency      string `json:"currency"`

	AmountQuota int     `json:"amount_quota"`
	QuotaDelta  int     `json:"quota_delta"`
	Cost        float64 `json:"cost"`

	Metadata  string `json:"metadata"`
	CreatedAt int64  `json:"created_at"`

	RelatedAuditEvents []BillingEventAuditLink      `json:"related_audit_events,omitempty"`
	RelatedTargetEvent *BillingEventTargetLink      `json:"related_target_event,omitempty"`
	RelatedMCPToolCall *BillingEventMCPToolCallLink `json:"related_mcp_tool_call,omitempty"`
}

type BillingEventAuditLink struct {
	Id        int64  `json:"id"`
	EventId   string `json:"event_id"`
	SourceId  string `json:"source_id"`
	PriceUnit string `json:"price_unit"`
	Reason    string `json:"reason"`
	Label     string `json:"label"`
	AdminId   int    `json:"admin_id"`
	CreatedAt int64  `json:"created_at"`
}

type BillingEventTargetLink struct {
	Id          int64  `json:"id"`
	EventId     string `json:"event_id"`
	UserId      int    `json:"user_id"`
	Source      string `json:"source"`
	SourceId    string `json:"source_id"`
	EventType   string `json:"event_type"`
	Status      string `json:"status"`
	PriceUnit   string `json:"price_unit"`
	AmountQuota int    `json:"amount_quota"`
	QuotaDelta  int    `json:"quota_delta"`
	CreatedAt   int64  `json:"created_at"`
}

type BillingEventMCPToolCallLink struct {
	Id              int64  `json:"id"`
	ToolId          int    `json:"tool_id"`
	ToolName        string `json:"tool_name"`
	RequestId       string `json:"request_id"`
	Status          string `json:"status"`
	ErrorCode       string `json:"error_code"`
	ErrorMessage    string `json:"error_message"`
	Metadata        string `json:"metadata"`
	BridgeSessionId string `json:"bridge_session_id"`
	TargetClient    string `json:"target_client"`
	DurationMS      int    `json:"duration_ms"`
	ResultSize      int    `json:"result_size"`
	CreatedAt       int64  `json:"created_at"`
}

type BillingEventBackfillRequest struct {
	Sources []string `json:"sources"`
	Limit   int      `json:"limit"`
	DryRun  bool     `json:"dry_run"`
}

type BillingEventBackfillSourceResult struct {
	Source          string   `json:"source"`
	Scanned         int      `json:"scanned"`
	Created         int      `json:"created"`
	WouldCreate     int      `json:"would_create"`
	SkippedExisting int      `json:"skipped_existing"`
	SkippedInvalid  int      `json:"skipped_invalid"`
	ErrorCount      int      `json:"error_count"`
	Errors          []string `json:"errors"`
}

type BillingEventBackfillResponse struct {
	DryRun  bool     `json:"dry_run"`
	Limit   int      `json:"limit"`
	Sources []string `json:"sources"`

	Results []BillingEventBackfillSourceResult `json:"results"`

	TotalScanned         int `json:"total_scanned"`
	TotalCreated         int `json:"total_created"`
	TotalWouldCreate     int `json:"total_would_create"`
	TotalSkippedExisting int `json:"total_skipped_existing"`
	TotalSkippedInvalid  int `json:"total_skipped_invalid"`
	TotalErrorCount      int `json:"total_error_count"`
}

type BillingEventSourceCapabilityItem struct {
	Source      string `json:"source"`
	EventSource string `json:"event_source"`
	Label       string `json:"label"`
	Status      string `json:"status"`

	BackfillSources []string `json:"backfill_sources"`

	SupportsRecording        bool `json:"supports_recording"`
	SupportsBackfill         bool `json:"supports_backfill"`
	SupportsReconciliation   bool `json:"supports_reconciliation"`
	SupportsMissingBackfill  bool `json:"supports_missing_backfill"`
	SupportsRefundOrDelta    bool `json:"supports_refund_or_delta"`
	SupportsRepairAudit      bool `json:"supports_repair_audit"`
	SupportsAuditRelation    bool `json:"supports_audit_relation"`
	RequiresDurableSourceLog bool `json:"requires_durable_source_log"`

	Notes []string `json:"notes"`
}

type BillingEventSourceMatrixResponse struct {
	CheckedAt int64 `json:"checked_at"`

	Items []BillingEventSourceCapabilityItem `json:"items"`

	TotalSources      int `json:"total_sources"`
	ReadySources      int `json:"ready_sources"`
	RecordOnlySources int `json:"record_only_sources"`
	PlannedSources    int `json:"planned_sources"`
	AuditOnlySources  int `json:"audit_only_sources"`
}

type BillingEventHealthResponse struct {
	Limit     int      `json:"limit"`
	Sources   []string `json:"sources"`
	CheckedAt int64    `json:"checked_at"`

	NeedsReview bool `json:"needs_review"`

	TotalWouldCreate int `json:"total_would_create"`
	TotalMissing     int `json:"total_missing"`
	TotalMismatched  int `json:"total_mismatched"`
	TotalInvalid     int `json:"total_invalid"`
	TotalErrorCount  int `json:"total_error_count"`

	Backfill       BillingEventBackfillResponse       `json:"backfill"`
	Reconciliation BillingEventReconciliationResponse `json:"reconciliation"`
}

type BillingEventSummaryAggregate struct {
	TotalEvents      int64   `json:"total_events"`
	CreditEvents     int64   `json:"credit_events"`
	DebitEvents      int64   `json:"debit_events"`
	AuditEvents      int64   `json:"audit_events"`
	AmountQuota      int64   `json:"amount_quota"`
	NetQuotaDelta    int64   `json:"net_quota_delta"`
	CreditQuotaDelta int64   `json:"credit_quota_delta"`
	DebitQuotaDelta  int64   `json:"debit_quota_delta"`
	TotalCost        float64 `json:"total_cost"`
}

type BillingEventSummaryDimension struct {
	Key string `json:"key"`
	BillingEventSummaryAggregate
}

type BillingEventSummaryBucket struct {
	BucketStart int64 `json:"bucket_start"`
	BillingEventSummaryAggregate
}

type BillingEventSummaryResponse struct {
	StartTime     int64 `json:"start_time"`
	EndTime       int64 `json:"end_time"`
	BucketSeconds int64 `json:"bucket_seconds"`
	CheckedAt     int64 `json:"checked_at"`

	Totals     BillingEventSummaryAggregate   `json:"totals"`
	BySource   []BillingEventSummaryDimension `json:"by_source"`
	ByType     []BillingEventSummaryDimension `json:"by_type"`
	DailyTrend []BillingEventSummaryBucket    `json:"daily_trend"`
}

type BillingEventRelationHealthResponse struct {
	Limit     int   `json:"limit"`
	Cursor    int64 `json:"cursor"`
	CheckedAt int64 `json:"checked_at"`

	TotalAuditEvents       int64                                 `json:"total_audit_events"`
	TotalRelations         int64                                 `json:"total_relations"`
	ScannedAuditEvents     int                                   `json:"scanned_audit_events"`
	MissingRelations       int                                   `json:"missing_relations"`
	InvalidAuditEvents     int                                   `json:"invalid_audit_events"`
	OrphanSourceRelations  int                                   `json:"orphan_source_relations"`
	OrphanTargetRelations  int                                   `json:"orphan_target_relations"`
	NeedsReview            bool                                  `json:"needs_review"`
	HasMore                bool                                  `json:"has_more"`
	ScanComplete           bool                                  `json:"scan_complete"`
	NextCursor             int64                                 `json:"next_cursor"`
	SampleMissingRelations []BillingEventRelationMaintenanceItem `json:"sample_missing_relations"`
	SampleInvalidAudits    []BillingEventRelationMaintenanceItem `json:"sample_invalid_audits"`
}

type BillingEventRelationBackfillRequest struct {
	Limit  int   `json:"limit"`
	Cursor int64 `json:"cursor"`
	DryRun bool  `json:"dry_run"`
}

type BillingEventRelationBackfillResponse struct {
	DryRun bool  `json:"dry_run"`
	Limit  int   `json:"limit"`
	Cursor int64 `json:"cursor"`

	ScannedAuditEvents int   `json:"scanned_audit_events"`
	Created            int   `json:"created"`
	WouldCreate        int   `json:"would_create"`
	SkippedExisting    int   `json:"skipped_existing"`
	SkippedInvalid     int   `json:"skipped_invalid"`
	ErrorCount         int   `json:"error_count"`
	HasMore            bool  `json:"has_more"`
	ScanComplete       bool  `json:"scan_complete"`
	NextCursor         int64 `json:"next_cursor"`

	Items  []BillingEventRelationMaintenanceItem `json:"items"`
	Errors []string                              `json:"errors"`
}

type BillingEventRelationOrphanCleanupRequest struct {
	DryRun bool `json:"dry_run"`
}

type BillingEventRelationOrphanCleanupResponse struct {
	DryRun        bool `json:"dry_run"`
	SourceOrphans int  `json:"source_orphans"`
	TargetOrphans int  `json:"target_orphans"`
	WouldDelete   int  `json:"would_delete"`
	Deleted       int  `json:"deleted"`
}

type BillingEventRelationInspectionSettings struct {
	Enabled               bool  `json:"enabled"`
	IntervalMinutes       int   `json:"interval_minutes"`
	Limit                 int   `json:"limit"`
	AutoBackfill          bool  `json:"auto_backfill"`
	AutoCleanupOrphans    bool  `json:"auto_cleanup_orphans"`
	MaxAutoBackfill       int   `json:"max_auto_backfill"`
	MaxAutoCleanupOrphans int   `json:"max_auto_cleanup_orphans"`
	Cursor                int64 `json:"cursor"`
}

type BillingEventRelationInspectionSettingsRequest struct {
	Enabled               bool   `json:"enabled"`
	IntervalMinutes       int    `json:"interval_minutes"`
	Limit                 int    `json:"limit"`
	AutoBackfill          bool   `json:"auto_backfill"`
	AutoCleanupOrphans    bool   `json:"auto_cleanup_orphans"`
	MaxAutoBackfill       int    `json:"max_auto_backfill"`
	MaxAutoCleanupOrphans int    `json:"max_auto_cleanup_orphans"`
	Cursor                *int64 `json:"cursor,omitempty"`
}

type BillingEventRelationInspectionStatusResponse struct {
	Settings       BillingEventRelationInspectionSettings     `json:"settings"`
	Running        bool                                       `json:"running"`
	LastRunAt      int64                                      `json:"last_run_at"`
	LastRunStatus  string                                     `json:"last_run_status"`
	LastRunMessage string                                     `json:"last_run_message"`
	LastHealth     *BillingEventRelationHealthResponse        `json:"last_health,omitempty"`
	LastBackfill   *BillingEventRelationBackfillResponse      `json:"last_backfill,omitempty"`
	LastCleanup    *BillingEventRelationOrphanCleanupResponse `json:"last_cleanup,omitempty"`
	RecentRuns     []BillingEventRelationInspectionRunItem    `json:"recent_runs"`
}

type BillingEventRelationInspectionRunResponse struct {
	Manual   bool                                       `json:"manual"`
	Status   string                                     `json:"status"`
	Message  string                                     `json:"message"`
	Settings BillingEventRelationInspectionSettings     `json:"settings"`
	Health   BillingEventRelationHealthResponse         `json:"health"`
	Backfill *BillingEventRelationBackfillResponse      `json:"backfill,omitempty"`
	Cleanup  *BillingEventRelationOrphanCleanupResponse `json:"cleanup,omitempty"`
	Run      *BillingEventRelationInspectionRunItem     `json:"run,omitempty"`
}

type BillingEventRelationInspectionRunItem struct {
	Id      int64  `json:"id"`
	Trigger string `json:"trigger"`
	Status  string `json:"status"`
	Message string `json:"message"`

	Limit              int   `json:"limit"`
	Cursor             int64 `json:"cursor"`
	NextCursor         int64 `json:"next_cursor"`
	AutoBackfill       bool  `json:"auto_backfill"`
	AutoCleanupOrphans bool  `json:"auto_cleanup_orphans"`

	MaxAutoBackfill       int `json:"max_auto_backfill"`
	MaxAutoCleanupOrphans int `json:"max_auto_cleanup_orphans"`

	ScannedAuditEvents    int `json:"scanned_audit_events"`
	MissingRelations      int `json:"missing_relations"`
	InvalidAuditEvents    int `json:"invalid_audit_events"`
	OrphanSourceRelations int `json:"orphan_source_relations"`
	OrphanTargetRelations int `json:"orphan_target_relations"`

	BackfillCreated        int  `json:"backfill_created"`
	BackfillWouldCreate    int  `json:"backfill_would_create"`
	BackfillSkippedInvalid int  `json:"backfill_skipped_invalid"`
	BackfillErrorCount     int  `json:"backfill_error_count"`
	BackfillBlocked        bool `json:"backfill_blocked"`

	CleanupDeleted     int  `json:"cleanup_deleted"`
	CleanupWouldDelete int  `json:"cleanup_would_delete"`
	CleanupBlocked     bool `json:"cleanup_blocked"`

	StartedAt  int64 `json:"started_at"`
	FinishedAt int64 `json:"finished_at"`
	CreatedAt  int64 `json:"created_at"`
}

type BillingEventRelationMaintenanceItem struct {
	AuditEventId  int64  `json:"audit_event_id"`
	AuditEvent    string `json:"audit_event"`
	TargetEventId int64  `json:"target_event_id"`
	RelationType  string `json:"relation_type"`
	Reason        string `json:"reason"`
	Label         string `json:"label"`
	AdminId       int    `json:"admin_id"`
	Error         string `json:"error,omitempty"`
}

type BillingEventReconciliationRequest struct {
	Sources []string `json:"sources"`
	Limit   int      `json:"limit"`
}

type BillingEventReconciliationMismatchRequest struct {
	Sources     []string `json:"sources"`
	Limit       int      `json:"limit"`
	DetailLimit int      `json:"detail_limit"`
}

type BillingEventReconciliationMissingRequest struct {
	Sources     []string `json:"sources"`
	Limit       int      `json:"limit"`
	DetailLimit int      `json:"detail_limit"`
}

type BillingEventReconciliationExpectedEvent struct {
	Label         string `json:"label"`
	Source        string `json:"source"`
	SourceId      string `json:"source_id"`
	Phase         string `json:"phase"`
	UserId        int    `json:"user_id"`
	TokenId       int    `json:"token_id"`
	EventType     string `json:"event_type"`
	Status        string `json:"status"`
	AmountQuota   int    `json:"amount_quota"`
	QuotaDelta    int    `json:"quota_delta"`
	RequestId     string `json:"request_id"`
	Group         string `json:"group"`
	BillingSource string `json:"billing_source"`
	PriceUnit     string `json:"price_unit"`
	Currency      string `json:"currency"`
}

type BillingEventReconciliationDiff struct {
	Field    string `json:"field"`
	Expected string `json:"expected"`
	Actual   string `json:"actual"`
}

type BillingEventReconciliationMismatchItem struct {
	Source   string                                  `json:"source"`
	Label    string                                  `json:"label"`
	Expected BillingEventReconciliationExpectedEvent `json:"expected"`
	Actual   *BillingEventItem                       `json:"actual"`
	Diffs    []BillingEventReconciliationDiff        `json:"diffs"`
}

type BillingEventReconciliationMissingItem struct {
	Source   string                                  `json:"source"`
	Label    string                                  `json:"label"`
	Expected BillingEventReconciliationExpectedEvent `json:"expected"`
}

type BillingEventReconciliationMismatchResponse struct {
	Limit       int      `json:"limit"`
	DetailLimit int      `json:"detail_limit"`
	Sources     []string `json:"sources"`

	Items []BillingEventReconciliationMismatchItem `json:"items"`

	TotalScanned    int  `json:"total_scanned"`
	TotalExpected   int  `json:"total_expected"`
	TotalMismatched int  `json:"total_mismatched"`
	TotalMissing    int  `json:"total_missing"`
	TotalInvalid    int  `json:"total_invalid"`
	TotalErrorCount int  `json:"total_error_count"`
	HasMore         bool `json:"has_more"`
	ScanComplete    bool `json:"scan_complete"`
}

type BillingEventReconciliationMissingResponse struct {
	Limit       int      `json:"limit"`
	DetailLimit int      `json:"detail_limit"`
	Sources     []string `json:"sources"`

	Items []BillingEventReconciliationMissingItem `json:"items"`

	TotalScanned    int  `json:"total_scanned"`
	TotalExpected   int  `json:"total_expected"`
	TotalMissing    int  `json:"total_missing"`
	TotalMismatched int  `json:"total_mismatched"`
	TotalInvalid    int  `json:"total_invalid"`
	TotalErrorCount int  `json:"total_error_count"`
	HasMore         bool `json:"has_more"`
	ScanComplete    bool `json:"scan_complete"`
}

type BillingEventReconciliationRepairRequest struct {
	Source   string                                   `json:"source"`
	Label    string                                   `json:"label"`
	Limit    int                                      `json:"limit"`
	Reason   string                                   `json:"reason"`
	ActualId int64                                    `json:"actual_id"`
	Expected *BillingEventReconciliationExpectedEvent `json:"expected"`
}

type BillingEventReconciliationBackfillMissingRequest struct {
	Source   string                                   `json:"source"`
	Label    string                                   `json:"label"`
	Limit    int                                      `json:"limit"`
	Reason   string                                   `json:"reason"`
	Expected *BillingEventReconciliationExpectedEvent `json:"expected"`
}

type BillingEventReconciliationRepairResponse struct {
	Repaired bool `json:"repaired"`

	Label    string                                  `json:"label"`
	Source   string                                  `json:"source"`
	Expected BillingEventReconciliationExpectedEvent `json:"expected"`
	Diffs    []BillingEventReconciliationDiff        `json:"diffs"`

	Before     *BillingEventItem `json:"before"`
	After      *BillingEventItem `json:"after"`
	AuditEvent *BillingEventItem `json:"audit_event"`
}

type BillingEventReconciliationBackfillMissingResponse struct {
	Backfilled bool `json:"backfilled"`

	Label    string                                  `json:"label"`
	Source   string                                  `json:"source"`
	Expected BillingEventReconciliationExpectedEvent `json:"expected"`

	Event      *BillingEventItem `json:"event"`
	AuditEvent *BillingEventItem `json:"audit_event"`
}

type BillingEventReconciliationSourceResult struct {
	Source string `json:"source"`

	Scanned    int `json:"scanned"`
	Expected   int `json:"expected"`
	Ledgered   int `json:"ledgered"`
	Missing    int `json:"missing"`
	Mismatched int `json:"mismatched"`
	Invalid    int `json:"invalid"`

	HasMore      bool `json:"has_more"`
	ScanComplete bool `json:"scan_complete"`

	ErrorCount       int      `json:"error_count"`
	SampleMissing    []string `json:"sample_missing"`
	SampleMismatched []string `json:"sample_mismatched"`
	SampleInvalid    []string `json:"sample_invalid"`
	Errors           []string `json:"errors"`
}

type BillingEventReconciliationResponse struct {
	Limit   int      `json:"limit"`
	Sources []string `json:"sources"`

	Results []BillingEventReconciliationSourceResult `json:"results"`

	TotalScanned    int  `json:"total_scanned"`
	TotalExpected   int  `json:"total_expected"`
	TotalLedgered   int  `json:"total_ledgered"`
	TotalMissing    int  `json:"total_missing"`
	TotalMismatched int  `json:"total_mismatched"`
	TotalInvalid    int  `json:"total_invalid"`
	TotalErrorCount int  `json:"total_error_count"`
	HasMore         bool `json:"has_more"`
	ScanComplete    bool `json:"scan_complete"`
}
