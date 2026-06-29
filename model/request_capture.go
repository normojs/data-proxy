package model

const (
	RequestCaptureLevelOff             = "off"
	RequestCaptureLevelMetadata        = "metadata"
	RequestCaptureLevelSanitizedBundle = "sanitized_bundle"
	RequestCaptureLevelFullBundle      = "full_bundle"

	RequestCaptureStatusPending    = "pending"
	RequestCaptureStatusSpooling   = "spooling"
	RequestCaptureStatusFinalizing = "finalizing"
	RequestCaptureStatusUploaded   = "uploaded"
	RequestCaptureStatusFailed     = "failed"
	RequestCaptureStatusExpired    = "expired"
	RequestCaptureStatusDeleted    = "deleted"

	RequestCaptureArtifactKindRawBundle        = "raw_bundle"
	RequestCaptureArtifactKindSanitizedBundle  = "sanitized_bundle"
	RequestCaptureArtifactKindDiagnosticBundle = "diagnostic_bundle"
	RequestCaptureArtifactKindTrainingDataset  = "training_dataset"

	RequestCaptureArtifactStatusPending   = "pending"
	RequestCaptureArtifactStatusAvailable = "available"
	RequestCaptureArtifactStatusFailed    = "failed"
	RequestCaptureArtifactStatusDeleted   = "deleted"

	RequestDiagnosticStatusPending   = "pending"
	RequestDiagnosticStatusRunning   = "running"
	RequestDiagnosticStatusCompleted = "completed"
	RequestDiagnosticStatusFailed    = "failed"

	TrainingDatasetStatusPending   = "pending"
	TrainingDatasetStatusBuilding  = "building"
	TrainingDatasetStatusCompleted = "completed"
	TrainingDatasetStatusFailed    = "failed"
	TrainingDatasetStatusDeleted   = "deleted"

	TrainingSampleReviewStatusPending  = "pending"
	TrainingSampleReviewStatusApproved = "approved"
	TrainingSampleReviewStatusRejected = "rejected"
)

type RequestCaptureRecord struct {
	Id                   int64  `json:"id" gorm:"primaryKey"`
	RequestId            string `json:"request_id" gorm:"type:varchar(128);not null;default:'';index"`
	UpstreamRequestId    string `json:"upstream_request_id" gorm:"type:varchar(128);not null;default:'';index"`
	SubsiteId            int64  `json:"subsite_id" gorm:"not null;default:0;index"`
	UserId               int    `json:"user_id" gorm:"not null;default:0;index"`
	TokenId              int    `json:"token_id" gorm:"not null;default:0;index"`
	ChannelId            int    `json:"channel_id" gorm:"not null;default:0;index"`
	ConnectedAppId       int64  `json:"connected_app_id" gorm:"not null;default:0;index"`
	Group                string `json:"group" gorm:"type:varchar(64);not null;default:'';index"`
	ModelName            string `json:"model_name" gorm:"type:varchar(255);not null;default:'';index"`
	Method               string `json:"method" gorm:"type:varchar(16);not null;default:''"`
	RequestPath          string `json:"request_path" gorm:"type:varchar(255);not null;default:'';index"`
	ProtocolChain        string `json:"protocol_chain" gorm:"type:varchar(128);not null;default:'';index"`
	CaptureLevel         string `json:"capture_level" gorm:"type:varchar(32);not null;default:'metadata';index"`
	CaptureStatus        string `json:"capture_status" gorm:"type:varchar(32);not null;default:'pending';index"`
	CapturePolicyId      string `json:"capture_policy_id" gorm:"type:varchar(128);not null;default:'';index"`
	IsStream             bool   `json:"is_stream" gorm:"not null;default:false;index"`
	HasError             bool   `json:"has_error" gorm:"not null;default:false;index"`
	ErrorCode            string `json:"error_code" gorm:"type:varchar(128);not null;default:'';index"`
	LastError            string `json:"last_error" gorm:"type:varchar(1024);not null;default:''"`
	RequestBytes         int64  `json:"request_bytes" gorm:"not null;default:0"`
	UpstreamRequestBytes int64  `json:"upstream_request_bytes" gorm:"not null;default:0"`
	UpstreamBodyBytes    int64  `json:"upstream_body_bytes" gorm:"not null;default:0"`
	DownstreamBodyBytes  int64  `json:"downstream_body_bytes" gorm:"not null;default:0"`
	TotalBytes           int64  `json:"total_bytes" gorm:"not null;default:0"`
	PromptTokens         int    `json:"prompt_tokens" gorm:"not null;default:0"`
	CompletionTokens     int    `json:"completion_tokens" gorm:"not null;default:0"`
	Quota                int    `json:"quota" gorm:"not null;default:0"`
	SpoolDir             string `json:"spool_dir" gorm:"type:varchar(512);not null;default:''"`
	MetadataJson         string `json:"metadata_json" gorm:"type:text"`
	ConversionJson       string `json:"conversion_json" gorm:"type:text"`
	RedactionJson        string `json:"redaction_json" gorm:"type:text"`
	StartedAt            int64  `json:"started_at" gorm:"not null;default:0;index"`
	FinishedAt           int64  `json:"finished_at" gorm:"not null;default:0;index"`
	FinalizedAt          int64  `json:"finalized_at" gorm:"not null;default:0;index"`
	FinalizeAttempts     int    `json:"finalize_attempts" gorm:"not null;default:0"`
	NextFinalizeAt       int64  `json:"next_finalize_at" gorm:"not null;default:0;index"`
	ExpiresAt            int64  `json:"expires_at" gorm:"not null;default:0;index"`
	CreatedAt            int64  `json:"created_at" gorm:"autoCreateTime;index"`
	UpdatedAt            int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (RequestCaptureRecord) TableName() string {
	return "request_capture_records"
}

type RequestCaptureArtifact struct {
	Id                  int64  `json:"id" gorm:"primaryKey"`
	CaptureId           int64  `json:"capture_id" gorm:"not null;default:0;index"`
	RequestId           string `json:"request_id" gorm:"type:varchar(128);not null;default:'';index"`
	Kind                string `json:"kind" gorm:"type:varchar(64);not null;default:'';index"`
	Status              string `json:"status" gorm:"type:varchar(32);not null;default:'pending';index"`
	Provider            string `json:"provider" gorm:"type:varchar(32);not null;default:'';index"`
	Bucket              string `json:"bucket" gorm:"type:varchar(128);not null;default:''"`
	StorageKey          string `json:"storage_key" gorm:"type:varchar(1024);not null;default:''"`
	ObjectVersion       string `json:"object_version" gorm:"type:varchar(255);not null;default:''"`
	ContentType         string `json:"content_type" gorm:"type:varchar(255);not null;default:'application/octet-stream'"`
	Compression         string `json:"compression" gorm:"type:varchar(64);not null;default:''"`
	EncryptionAlgorithm string `json:"encryption_algorithm" gorm:"type:varchar(64);not null;default:''"`
	EncryptionKeyId     string `json:"encryption_key_id" gorm:"type:varchar(255);not null;default:''"`
	SHA256              string `json:"sha256" gorm:"type:varchar(64);not null;default:'';index"`
	SizeBytes           int64  `json:"size_bytes" gorm:"not null;default:0"`
	ManifestJson        string `json:"manifest_json" gorm:"type:text"`
	LastError           string `json:"last_error" gorm:"type:varchar(1024);not null;default:''"`
	UploadedAt          int64  `json:"uploaded_at" gorm:"not null;default:0;index"`
	DeletedAt           int64  `json:"deleted_at" gorm:"not null;default:0;index"`
	ExpiresAt           int64  `json:"expires_at" gorm:"not null;default:0;index"`
	CreatedAt           int64  `json:"created_at" gorm:"autoCreateTime;index"`
	UpdatedAt           int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (RequestCaptureArtifact) TableName() string {
	return "request_capture_artifacts"
}

type RequestDiagnosticReport struct {
	Id          int64  `json:"id" gorm:"primaryKey"`
	RequestId   string `json:"request_id" gorm:"type:varchar(128);not null;default:'';index"`
	SubsiteId   int64  `json:"subsite_id" gorm:"not null;default:0;index"`
	CaptureId   int64  `json:"capture_id" gorm:"not null;default:0;index"`
	ArtifactId  int64  `json:"artifact_id" gorm:"not null;default:0;index"`
	ReportType  string `json:"report_type" gorm:"type:varchar(64);not null;default:'request';index"`
	Status      string `json:"status" gorm:"type:varchar(32);not null;default:'pending';index"`
	Severity    string `json:"severity" gorm:"type:varchar(32);not null;default:'';index"`
	Summary     string `json:"summary" gorm:"type:varchar(1024);not null;default:''"`
	ReportJson  string `json:"report_json" gorm:"type:text"`
	GeneratedBy string `json:"generated_by" gorm:"type:varchar(64);not null;default:'';index"`
	LastError   string `json:"last_error" gorm:"type:varchar(1024);not null;default:''"`
	GeneratedAt int64  `json:"generated_at" gorm:"not null;default:0;index"`
	ExpiresAt   int64  `json:"expires_at" gorm:"not null;default:0;index"`
	CreatedAt   int64  `json:"created_at" gorm:"autoCreateTime;index"`
	UpdatedAt   int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (RequestDiagnosticReport) TableName() string {
	return "request_diagnostic_reports"
}

type TrainingDatasetVersion struct {
	Id                  int64  `json:"id" gorm:"primaryKey"`
	Name                string `json:"name" gorm:"type:varchar(128);not null;default:'';uniqueIndex:idx_training_dataset_name_version,priority:1"`
	Version             string `json:"version" gorm:"type:varchar(128);not null;default:'';uniqueIndex:idx_training_dataset_name_version,priority:2"`
	Status              string `json:"status" gorm:"type:varchar(32);not null;default:'pending';index"`
	OutputFormat        string `json:"output_format" gorm:"type:varchar(32);not null;default:'jsonl';index"`
	Provider            string `json:"provider" gorm:"type:varchar(32);not null;default:'';index"`
	Bucket              string `json:"bucket" gorm:"type:varchar(128);not null;default:''"`
	StorageKey          string `json:"storage_key" gorm:"type:varchar(1024);not null;default:''"`
	SHA256              string `json:"sha256" gorm:"type:varchar(64);not null;default:'';index"`
	SizeBytes           int64  `json:"size_bytes" gorm:"not null;default:0"`
	SampleCount         int64  `json:"sample_count" gorm:"not null;default:0"`
	SourceScopeJson     string `json:"source_scope_json" gorm:"type:text"`
	RedactionPolicyJson string `json:"redaction_policy_json" gorm:"type:text"`
	BuildManifestJson   string `json:"build_manifest_json" gorm:"type:text"`
	LastError           string `json:"last_error" gorm:"type:varchar(1024);not null;default:''"`
	BuiltAt             int64  `json:"built_at" gorm:"not null;default:0;index"`
	ExpiresAt           int64  `json:"expires_at" gorm:"not null;default:0;index"`
	CreatedAt           int64  `json:"created_at" gorm:"autoCreateTime;index"`
	UpdatedAt           int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (TrainingDatasetVersion) TableName() string {
	return "training_dataset_versions"
}

type TrainingSample struct {
	Id               int64  `json:"id" gorm:"primaryKey"`
	DatasetVersionId int64  `json:"dataset_version_id" gorm:"not null;default:0;index"`
	RequestId        string `json:"request_id" gorm:"type:varchar(128);not null;default:'';index"`
	CaptureId        int64  `json:"capture_id" gorm:"not null;default:0;index"`
	ArtifactId       int64  `json:"artifact_id" gorm:"not null;default:0;index"`
	ModelName        string `json:"model_name" gorm:"type:varchar(255);not null;default:'';index"`
	SourceHash       string `json:"source_hash" gorm:"type:varchar(64);not null;default:'';index"`
	RedactionStatus  string `json:"redaction_status" gorm:"type:varchar(32);not null;default:'';index"`
	QualityScore     int    `json:"quality_score" gorm:"not null;default:0;index"`
	ReviewStatus     string `json:"review_status" gorm:"type:varchar(32);not null;default:'pending';index"`
	ReviewComment    string `json:"review_comment" gorm:"type:varchar(1024);not null;default:''"`
	ReviewedBy       int    `json:"reviewed_by" gorm:"not null;default:0;index"`
	ReviewedAt       int64  `json:"reviewed_at" gorm:"not null;default:0;index"`
	TagsJson         string `json:"tags_json" gorm:"type:text"`
	MetadataJson     string `json:"metadata_json" gorm:"type:text"`
	CreatedAt        int64  `json:"created_at" gorm:"autoCreateTime;index"`
	UpdatedAt        int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (TrainingSample) TableName() string {
	return "training_samples"
}
