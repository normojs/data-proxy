package controller

import (
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type requestDiagnosticReportView struct {
	Id          int64                          `json:"id,omitempty"`
	RequestId   string                         `json:"request_id"`
	Status      string                         `json:"status"`
	Severity    string                         `json:"severity"`
	Summary     string                         `json:"summary"`
	GeneratedAt int64                          `json:"generated_at,omitempty"`
	Report      requestDiagnosticReportPayload `json:"report"`
}

type requestDiagnosticReportPayload struct {
	Trace    requestLogTraceResponse          `json:"trace"`
	Capture  *requestDiagnosticCaptureSummary `json:"capture,omitempty"`
	Findings []requestDiagnosticFinding       `json:"findings"`
}

type requestDiagnosticFinding struct {
	Level   string `json:"level"`
	Code    string `json:"code"`
	Message string `json:"message"`
	Detail  string `json:"detail,omitempty"`
}

type requestDiagnosticCaptureSummary struct {
	Id                   int64                              `json:"id"`
	RequestId            string                             `json:"request_id"`
	UpstreamRequestId    string                             `json:"upstream_request_id,omitempty"`
	UserId               int                                `json:"user_id"`
	TokenId              int                                `json:"token_id"`
	ChannelId            int                                `json:"channel_id"`
	ConnectedAppId       int64                              `json:"connected_app_id"`
	Group                string                             `json:"group,omitempty"`
	ModelName            string                             `json:"model_name"`
	RequestPath          string                             `json:"request_path"`
	ProtocolChain        string                             `json:"protocol_chain,omitempty"`
	CaptureLevel         string                             `json:"capture_level"`
	CaptureStatus        string                             `json:"capture_status"`
	IsStream             bool                               `json:"is_stream"`
	HasError             bool                               `json:"has_error"`
	LastError            string                             `json:"last_error,omitempty"`
	RequestBytes         int64                              `json:"request_bytes"`
	UpstreamRequestBytes int64                              `json:"upstream_request_bytes"`
	UpstreamBodyBytes    int64                              `json:"upstream_body_bytes"`
	DownstreamBodyBytes  int64                              `json:"downstream_body_bytes"`
	TotalBytes           int64                              `json:"total_bytes"`
	SpoolDir             string                             `json:"spool_dir,omitempty"`
	StartedAt            int64                              `json:"started_at,omitempty"`
	FinishedAt           int64                              `json:"finished_at,omitempty"`
	FinalizedAt          int64                              `json:"finalized_at,omitempty"`
	Artifacts            []requestDiagnosticArtifactSummary `json:"artifacts,omitempty"`
}

type requestDiagnosticArtifactSummary struct {
	Id                  int64  `json:"id"`
	Kind                string `json:"kind"`
	Status              string `json:"status"`
	Provider            string `json:"provider"`
	Bucket              string `json:"bucket,omitempty"`
	StorageKey          string `json:"storage_key,omitempty"`
	ContentType         string `json:"content_type"`
	Compression         string `json:"compression,omitempty"`
	EncryptionAlgorithm string `json:"encryption_algorithm,omitempty"`
	EncryptionKeyId     string `json:"encryption_key_id,omitempty"`
	SHA256              string `json:"sha256,omitempty"`
	SizeBytes           int64  `json:"size_bytes"`
	LastError           string `json:"last_error,omitempty"`
	UploadedAt          int64  `json:"uploaded_at,omitempty"`
}

type requestDiagnosticCandidate struct {
	RequestId      string `json:"request_id"`
	Severity       string `json:"severity"`
	Source         string `json:"source"`
	Summary        string `json:"summary"`
	LastSeenAt     int64  `json:"last_seen_at"`
	ErrorCount     int    `json:"error_count"`
	ConsumeCount   int    `json:"consume_count"`
	UserId         int    `json:"user_id,omitempty"`
	Username       string `json:"username,omitempty"`
	TokenId        int    `json:"token_id,omitempty"`
	TokenName      string `json:"token_name,omitempty"`
	ModelName      string `json:"model_name,omitempty"`
	ChannelId      int    `json:"channel_id,omitempty"`
	Group          string `json:"group,omitempty"`
	ReportStatus   string `json:"report_status,omitempty"`
	ReportSeverity string `json:"report_severity,omitempty"`
}

func GetRequestDiagnosticReport(c *gin.Context) {
	requestId := requestDiagnosticQuery(c)
	if requestId == "" {
		common.ApiErrorMsg(c, "request_id is required")
		return
	}
	if model.DB == nil {
		common.ApiErrorMsg(c, "database is not initialized")
		return
	}
	var report model.RequestDiagnosticReport
	err := model.DB.Where("request_id = ?", requestId).Order("id desc").First(&report).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		common.ApiSuccess(c, requestDiagnosticReportView{
			RequestId: requestId,
			Status:    "not_found",
			Severity:  "info",
			Summary:   "未生成诊断报告",
			Report: requestDiagnosticReportPayload{
				Findings: []requestDiagnosticFinding{},
			},
		})
		return
	}
	if err != nil {
		common.ApiError(c, err)
		return
	}
	view, err := requestDiagnosticReportViewFromModel(report)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, view)
}

func ListRequestDiagnosticCandidates(c *gin.Context) {
	limit := requestDiagnosticCandidateLimit(c)
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	candidates := map[string]*requestDiagnosticCandidate{}
	if err := collectRequestDiagnosticLogCandidates(candidates, limit, startTimestamp, endTimestamp); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := collectRequestDiagnosticCaptureCandidates(candidates, limit, startTimestamp, endTimestamp); err != nil {
		common.ApiError(c, err)
		return
	}
	items := make([]requestDiagnosticCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		items = append(items, *candidate)
	}
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].LastSeenAt > items[j].LastSeenAt
	})
	if len(items) > limit {
		items = items[:limit]
	}
	hydrateRequestDiagnosticCandidateReports(items)
	common.ApiSuccess(c, gin.H{
		"total": len(items),
		"items": items,
	})
}

func GenerateRequestDiagnosticReport(c *gin.Context) {
	requestId := requestDiagnosticQuery(c)
	if requestId == "" {
		common.ApiErrorMsg(c, "request_id is required")
		return
	}
	if model.DB == nil {
		common.ApiErrorMsg(c, "database is not initialized")
		return
	}
	payload, severity, summary, captureId, artifactId, err := buildRequestDiagnosticReportPayload(requestId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	reportBody, err := json.Marshal(payload)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	now := common.GetTimestamp()
	report := model.RequestDiagnosticReport{
		RequestId:   requestId,
		CaptureId:   captureId,
		ArtifactId:  artifactId,
		ReportType:  "request",
		Status:      model.RequestDiagnosticStatusCompleted,
		Severity:    severity,
		Summary:     summary,
		ReportJson:  string(reportBody),
		GeneratedBy: "admin",
		GeneratedAt: now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := model.DB.Create(&report).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, requestDiagnosticReportView{
		Id:          report.Id,
		RequestId:   report.RequestId,
		Status:      report.Status,
		Severity:    report.Severity,
		Summary:     report.Summary,
		GeneratedAt: report.GeneratedAt,
		Report:      payload,
	})
}

func requestDiagnosticCandidateLimit(c *gin.Context) int {
	limit, _ := strconv.Atoi(c.Query("limit"))
	if limit <= 0 {
		return 50
	}
	if limit > 200 {
		return 200
	}
	return limit
}

func collectRequestDiagnosticLogCandidates(candidates map[string]*requestDiagnosticCandidate, limit int, startTimestamp int64, endTimestamp int64) error {
	if model.LOG_DB == nil {
		return errors.New("log database is not initialized")
	}
	var logs []model.Log
	tx := model.LOG_DB.Model(&model.Log{}).Where("(request_id <> '' OR upstream_request_id <> '')").Where("type = ?", model.LogTypeError)
	if startTimestamp > 0 {
		tx = tx.Where("created_at >= ?", startTimestamp)
	}
	if endTimestamp > 0 {
		tx = tx.Where("created_at <= ?", endTimestamp)
	}
	if err := tx.Order("created_at desc, id desc").Limit(limit * 2).Find(&logs).Error; err != nil {
		return err
	}
	for _, log := range logs {
		requestId := strings.TrimSpace(log.RequestId)
		if requestId == "" {
			requestId = strings.TrimSpace(log.UpstreamRequestId)
		}
		if requestId == "" {
			continue
		}
		candidate := ensureRequestDiagnosticCandidate(candidates, requestId)
		candidate.Severity = requestDiagnosticMaxSeverity(candidate.Severity, "error")
		candidate.Source = requestDiagnosticAppendSource(candidate.Source, "log_error")
		if candidate.Summary == "" {
			candidate.Summary = strings.TrimSpace(log.Content)
		}
		if candidate.Summary == "" {
			candidate.Summary = "请求包含错误日志"
		}
		if log.CreatedAt > candidate.LastSeenAt {
			candidate.LastSeenAt = log.CreatedAt
		}
		candidate.ErrorCount++
		if log.UserId != 0 {
			candidate.UserId = log.UserId
		}
		if log.Username != "" {
			candidate.Username = log.Username
		}
		if log.TokenId != 0 {
			candidate.TokenId = log.TokenId
		}
		if log.TokenName != "" {
			candidate.TokenName = log.TokenName
		}
		if log.ModelName != "" {
			candidate.ModelName = log.ModelName
		}
		if log.ChannelId != 0 {
			candidate.ChannelId = log.ChannelId
		}
		if log.Group != "" {
			candidate.Group = log.Group
		}
	}
	return nil
}

func collectRequestDiagnosticCaptureCandidates(candidates map[string]*requestDiagnosticCandidate, limit int, startTimestamp int64, endTimestamp int64) error {
	if model.DB == nil {
		return nil
	}
	var captures []model.RequestCaptureRecord
	tx := model.DB.Model(&model.RequestCaptureRecord{}).
		Where("request_id <> ''").
		Where("(has_error = ? OR capture_status = ? OR (is_stream = ? AND downstream_body_bytes = 0 AND capture_status IN ?))",
			true,
			model.RequestCaptureStatusFailed,
			true,
			[]string{
				model.RequestCaptureStatusSpooling,
				model.RequestCaptureStatusFinalizing,
				model.RequestCaptureStatusUploaded,
			},
		)
	if startTimestamp > 0 {
		tx = tx.Where("created_at >= ? OR started_at >= ?", startTimestamp, startTimestamp)
	}
	if endTimestamp > 0 {
		tx = tx.Where("created_at <= ? OR started_at <= ?", endTimestamp, endTimestamp)
	}
	if err := tx.Order("created_at desc, id desc").Limit(limit * 2).Find(&captures).Error; err != nil {
		return err
	}
	for _, capture := range captures {
		requestId := strings.TrimSpace(capture.RequestId)
		if requestId == "" {
			continue
		}
		candidate := ensureRequestDiagnosticCandidate(candidates, requestId)
		severity := "warning"
		if capture.HasError || capture.CaptureStatus == model.RequestCaptureStatusFailed {
			severity = "error"
		}
		candidate.Severity = requestDiagnosticMaxSeverity(candidate.Severity, severity)
		candidate.Source = requestDiagnosticAppendSource(candidate.Source, "capture")
		if candidate.Summary == "" {
			candidate.Summary = requestDiagnosticCaptureCandidateSummary(capture)
		}
		if seenAt := requestDiagnosticCaptureSeenAt(capture); seenAt > candidate.LastSeenAt {
			candidate.LastSeenAt = seenAt
		}
		if capture.UserId != 0 {
			candidate.UserId = capture.UserId
		}
		if capture.TokenId != 0 {
			candidate.TokenId = capture.TokenId
		}
		if capture.ModelName != "" {
			candidate.ModelName = capture.ModelName
		}
		if capture.ChannelId != 0 {
			candidate.ChannelId = capture.ChannelId
		}
		if capture.Group != "" {
			candidate.Group = capture.Group
		}
	}
	return nil
}

func ensureRequestDiagnosticCandidate(candidates map[string]*requestDiagnosticCandidate, requestId string) *requestDiagnosticCandidate {
	if candidate, ok := candidates[requestId]; ok {
		return candidate
	}
	candidate := &requestDiagnosticCandidate{
		RequestId: requestId,
		Severity:  "warning",
	}
	candidates[requestId] = candidate
	return candidate
}

func requestDiagnosticAppendSource(existing string, source string) string {
	if existing == "" {
		return source
	}
	for _, item := range strings.Split(existing, ",") {
		if strings.TrimSpace(item) == source {
			return existing
		}
	}
	return existing + "," + source
}

func requestDiagnosticMaxSeverity(current string, incoming string) string {
	rank := map[string]int{"info": 0, "ok": 0, "warning": 1, "error": 2}
	if rank[incoming] > rank[current] {
		return incoming
	}
	if current == "" {
		return incoming
	}
	return current
}

func requestDiagnosticCaptureCandidateSummary(capture model.RequestCaptureRecord) string {
	if capture.LastError != "" {
		return capture.LastError
	}
	if capture.CaptureStatus == model.RequestCaptureStatusFailed {
		return "请求捕获失败"
	}
	if capture.IsStream && capture.DownstreamBodyBytes == 0 {
		return "流式请求未捕获到下游响应体"
	}
	return "请求捕获状态可疑"
}

func requestDiagnosticCaptureSeenAt(capture model.RequestCaptureRecord) int64 {
	if capture.FinishedAt > 0 {
		return capture.FinishedAt
	}
	if capture.StartedAt > 0 {
		return capture.StartedAt
	}
	return capture.CreatedAt
}

func hydrateRequestDiagnosticCandidateReports(items []requestDiagnosticCandidate) {
	if model.DB == nil || len(items) == 0 {
		return
	}
	requestIds := make([]string, 0, len(items))
	for _, item := range items {
		requestIds = append(requestIds, item.RequestId)
	}
	var reports []model.RequestDiagnosticReport
	if err := model.DB.Where("request_id IN ?", requestIds).Order("generated_at desc, id desc").Find(&reports).Error; err != nil {
		return
	}
	latest := map[string]model.RequestDiagnosticReport{}
	for _, report := range reports {
		if _, ok := latest[report.RequestId]; !ok {
			latest[report.RequestId] = report
		}
	}
	for i := range items {
		report, ok := latest[items[i].RequestId]
		if !ok {
			continue
		}
		items[i].ReportStatus = report.Status
		items[i].ReportSeverity = report.Severity
	}
}

func requestDiagnosticQuery(c *gin.Context) string {
	requestId := strings.TrimSpace(c.Param("request_id"))
	if requestId == "" {
		requestId = strings.TrimSpace(c.Query("request_id"))
	}
	return requestId
}

func buildRequestDiagnosticReportPayload(requestId string) (requestDiagnosticReportPayload, string, string, int64, int64, error) {
	logs, err := model.GetLogsByRequestId(requestId, 0, false)
	if err != nil {
		return requestDiagnosticReportPayload{}, "", "", 0, 0, err
	}
	trace := buildRequestLogTraceResponse(requestId, "admin", logs, true)
	capture, captureId, artifactId, err := loadRequestDiagnosticCaptureSummary(requestId)
	if err != nil {
		return requestDiagnosticReportPayload{}, "", "", 0, 0, err
	}
	findings := buildRequestDiagnosticFindings(trace, capture)
	severity := requestDiagnosticSeverity(findings)
	summary := requestDiagnosticSummary(severity, findings)
	payload := requestDiagnosticReportPayload{
		Trace:    trace,
		Capture:  capture,
		Findings: findings,
	}
	return payload, severity, summary, captureId, artifactId, nil
}

func loadRequestDiagnosticCaptureSummary(requestId string) (*requestDiagnosticCaptureSummary, int64, int64, error) {
	var record model.RequestCaptureRecord
	err := model.DB.Where("request_id = ?", requestId).Order("id desc").First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, 0, 0, nil
	}
	if err != nil {
		return nil, 0, 0, err
	}
	var artifacts []model.RequestCaptureArtifact
	if err := model.DB.Where("request_id = ?", requestId).Order("id desc").Find(&artifacts).Error; err != nil {
		return nil, record.Id, 0, err
	}
	summary := &requestDiagnosticCaptureSummary{
		Id:                   record.Id,
		RequestId:            record.RequestId,
		UpstreamRequestId:    record.UpstreamRequestId,
		UserId:               record.UserId,
		TokenId:              record.TokenId,
		ChannelId:            record.ChannelId,
		ConnectedAppId:       record.ConnectedAppId,
		Group:                record.Group,
		ModelName:            record.ModelName,
		RequestPath:          record.RequestPath,
		ProtocolChain:        record.ProtocolChain,
		CaptureLevel:         record.CaptureLevel,
		CaptureStatus:        record.CaptureStatus,
		IsStream:             record.IsStream,
		HasError:             record.HasError,
		LastError:            record.LastError,
		RequestBytes:         record.RequestBytes,
		UpstreamRequestBytes: record.UpstreamRequestBytes,
		UpstreamBodyBytes:    record.UpstreamBodyBytes,
		DownstreamBodyBytes:  record.DownstreamBodyBytes,
		TotalBytes:           record.TotalBytes,
		SpoolDir:             record.SpoolDir,
		StartedAt:            record.StartedAt,
		FinishedAt:           record.FinishedAt,
		FinalizedAt:          record.FinalizedAt,
		Artifacts:            make([]requestDiagnosticArtifactSummary, 0, len(artifacts)),
	}
	var artifactId int64
	for _, artifact := range artifacts {
		if artifactId == 0 {
			artifactId = artifact.Id
		}
		summary.Artifacts = append(summary.Artifacts, requestDiagnosticArtifactSummary{
			Id:                  artifact.Id,
			Kind:                artifact.Kind,
			Status:              artifact.Status,
			Provider:            artifact.Provider,
			Bucket:              artifact.Bucket,
			StorageKey:          artifact.StorageKey,
			ContentType:         artifact.ContentType,
			Compression:         artifact.Compression,
			EncryptionAlgorithm: artifact.EncryptionAlgorithm,
			EncryptionKeyId:     artifact.EncryptionKeyId,
			SHA256:              artifact.SHA256,
			SizeBytes:           artifact.SizeBytes,
			LastError:           artifact.LastError,
			UploadedAt:          artifact.UploadedAt,
		})
	}
	return summary, record.Id, artifactId, nil
}

func buildRequestDiagnosticFindings(trace requestLogTraceResponse, capture *requestDiagnosticCaptureSummary) []requestDiagnosticFinding {
	findings := make([]requestDiagnosticFinding, 0)
	if trace.Total == 0 {
		findings = append(findings, requestDiagnosticFinding{
			Level:   "warning",
			Code:    "logs_missing",
			Message: "没有找到匹配的请求日志",
		})
	}
	if trace.Summary.Status == "error" {
		findings = append(findings, requestDiagnosticFinding{
			Level:   "error",
			Code:    "log_error",
			Message: "请求链路包含错误日志",
			Detail:  requestDiagnosticFirstError(trace),
		})
	}
	if capture == nil {
		findings = append(findings, requestDiagnosticFinding{
			Level:   "info",
			Code:    "capture_missing",
			Message: "没有找到请求捕获记录",
		})
	} else {
		if capture.HasError || capture.CaptureStatus == model.RequestCaptureStatusFailed {
			findings = append(findings, requestDiagnosticFinding{
				Level:   "error",
				Code:    "capture_failed",
				Message: "请求捕获链路失败",
				Detail:  capture.LastError,
			})
		}
		if capture.CaptureStatus == model.RequestCaptureStatusSpooling || capture.CaptureStatus == model.RequestCaptureStatusFinalizing {
			findings = append(findings, requestDiagnosticFinding{
				Level:   "warning",
				Code:    "capture_pending",
				Message: "请求捕获数据包仍在处理",
				Detail:  capture.CaptureStatus,
			})
		}
		if capture.IsStream && capture.DownstreamBodyBytes == 0 && trace.Summary.Status != "error" {
			findings = append(findings, requestDiagnosticFinding{
				Level:   "warning",
				Code:    "empty_downstream_stream",
				Message: "流式请求没有记录到下游响应体",
			})
		}
		if len(capture.Artifacts) == 0 && capture.CaptureStatus == model.RequestCaptureStatusUploaded {
			findings = append(findings, requestDiagnosticFinding{
				Level:   "warning",
				Code:    "capture_artifacts_missing",
				Message: "捕获记录已完成但没有对象存储 artifact",
			})
		}
	}
	meta := requestDiagnosticTraceMeta(trace)
	if len(requestDiagnosticStringSlice(meta["hosted_tools_filtered"])) > 0 {
		findings = append(findings, requestDiagnosticFinding{
			Level:   "warning",
			Code:    "hosted_tools_filtered",
			Message: "协议转换过滤了 hosted tool",
			Detail:  strings.Join(requestDiagnosticStringSlice(meta["hosted_tools_filtered"]), ", "),
		})
	}
	if status, ok := meta["responses_terminal_status"].(string); ok && status != "" && status != "completed" {
		findings = append(findings, requestDiagnosticFinding{
			Level:   "warning",
			Code:    "responses_terminal_status",
			Message: "Responses 流没有正常 completed",
			Detail:  status,
		})
	}
	return findings
}

func requestDiagnosticTraceMeta(trace requestLogTraceResponse) map[string]interface{} {
	meta, ok := trace.Diagnostics["request_conversion_meta"].(map[string]interface{})
	if !ok || meta == nil {
		return map[string]interface{}{}
	}
	return meta
}

func requestDiagnosticStringSlice(value interface{}) []string {
	switch current := value.(type) {
	case []string:
		return current
	case []interface{}:
		result := make([]string, 0, len(current))
		for _, item := range current {
			if str, ok := item.(string); ok && str != "" {
				result = append(result, str)
			}
		}
		return result
	default:
		return nil
	}
}

func requestDiagnosticFirstError(trace requestLogTraceResponse) string {
	errorsValue, ok := trace.Diagnostics["errors"].([]string)
	if ok && len(errorsValue) > 0 {
		return errorsValue[0]
	}
	errorsAny, ok := trace.Diagnostics["errors"].([]interface{})
	if ok && len(errorsAny) > 0 {
		if str, ok := errorsAny[0].(string); ok {
			return str
		}
	}
	return ""
}

func requestDiagnosticSeverity(findings []requestDiagnosticFinding) string {
	severity := "ok"
	for _, finding := range findings {
		if finding.Level == "error" {
			return "error"
		}
		if finding.Level == "warning" {
			severity = "warning"
		}
	}
	return severity
}

func requestDiagnosticSummary(severity string, findings []requestDiagnosticFinding) string {
	switch severity {
	case "error":
		return "发现错误，需要优先排查"
	case "warning":
		return "发现可疑项，建议进一步检查"
	default:
		if len(findings) == 0 {
			return "未发现明显异常"
		}
		return "已生成诊断报告"
	}
}

func requestDiagnosticReportViewFromModel(report model.RequestDiagnosticReport) (requestDiagnosticReportView, error) {
	var payload requestDiagnosticReportPayload
	if strings.TrimSpace(report.ReportJson) != "" {
		if err := json.Unmarshal([]byte(report.ReportJson), &payload); err != nil {
			return requestDiagnosticReportView{}, err
		}
	}
	return requestDiagnosticReportView{
		Id:          report.Id,
		RequestId:   report.RequestId,
		Status:      report.Status,
		Severity:    report.Severity,
		Summary:     report.Summary,
		GeneratedAt: report.GeneratedAt,
		Report:      payload,
	}, nil
}
