package controller

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	requestDiagnosticBundleMaxRawTarBytesEnv     = "DIAGNOSTIC_BUNDLE_MAX_RAW_TAR_BYTES"
	requestDiagnosticBundleDefaultMaxRawTarBytes = 256 * 1024 * 1024
)

type requestDiagnosticReportView struct {
	Id          int64                          `json:"id,omitempty"`
	RequestId   string                         `json:"request_id"`
	SubsiteId   int64                          `json:"subsite_id,omitempty"`
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
	SubsiteId            int64                              `json:"subsite_id,omitempty"`
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
	Metadata             map[string]interface{}             `json:"metadata,omitempty"`
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
	SubsiteId      int64  `json:"subsite_id,omitempty"`
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

type requestDiagnosticCandidateFilters struct {
	Severity     string
	Source       string
	ModelName    string
	ChannelId    int
	Group        string
	ReportStatus string
	UserId       int
	TokenId      int
	SubsiteId    *int64
}

func (filters requestDiagnosticCandidateFilters) HasFilter() bool {
	return filters.Severity != "" ||
		filters.Source != "" ||
		filters.ModelName != "" ||
		filters.ChannelId > 0 ||
		filters.Group != "" ||
		filters.ReportStatus != "" ||
		filters.UserId > 0 ||
		filters.TokenId > 0 ||
		filters.SubsiteId != nil
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
	subsiteId, ok := optionalSubsiteIdQuery(c)
	if !ok {
		return
	}
	var report model.RequestDiagnosticReport
	tx := model.DB.Where("request_id = ?", requestId)
	if subsiteId != nil {
		tx = tx.Where("subsite_id = ?", *subsiteId)
	}
	err := tx.Order("id desc").First(&report).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		view := requestDiagnosticReportView{
			RequestId: requestId,
			Status:    "not_found",
			Severity:  "info",
			Summary:   "未生成诊断报告",
			Report: requestDiagnosticReportPayload{
				Findings: []requestDiagnosticFinding{},
			},
		}
		if subsiteId != nil {
			view.SubsiteId = *subsiteId
		}
		common.ApiSuccess(c, view)
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
	filters, ok := requestDiagnosticCandidateFiltersFromQuery(c)
	if !ok {
		return
	}
	scanLimit := requestDiagnosticCandidateScanLimit(limit, filters)
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	candidates := map[string]*requestDiagnosticCandidate{}
	if err := collectRequestDiagnosticLogCandidates(candidates, scanLimit, startTimestamp, endTimestamp, filters.SubsiteId); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := collectRequestDiagnosticTraceMetaCandidates(candidates, scanLimit, startTimestamp, endTimestamp, filters.SubsiteId); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := collectRequestDiagnosticCaptureCandidates(candidates, scanLimit, startTimestamp, endTimestamp, filters.SubsiteId); err != nil {
		common.ApiError(c, err)
		return
	}
	items := make([]requestDiagnosticCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		items = append(items, *candidate)
	}
	hydrateRequestDiagnosticCandidateReports(items, filters.SubsiteId)
	if filters.HasFilter() {
		filtered := items[:0]
		for _, candidate := range items {
			if requestDiagnosticCandidateMatchesFilters(candidate, filters) {
				filtered = append(filtered, candidate)
			}
		}
		items = filtered
	}
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].LastSeenAt > items[j].LastSeenAt
	})
	if len(items) > limit {
		items = items[:limit]
	}
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
	subsiteId, ok := optionalSubsiteIdQuery(c)
	if !ok {
		return
	}
	payload, severity, summary, captureId, artifactId, err := buildRequestDiagnosticReportPayload(requestId, subsiteId)
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
		SubsiteId:   requestDiagnosticPayloadSubsiteId(payload, subsiteId),
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
		SubsiteId:   report.SubsiteId,
		Status:      report.Status,
		Severity:    report.Severity,
		Summary:     report.Summary,
		GeneratedAt: report.GeneratedAt,
		Report:      payload,
	})
}

func DownloadRequestDiagnosticBundle(c *gin.Context) {
	requestId := requestDiagnosticQuery(c)
	if requestId == "" {
		common.ApiErrorMsg(c, "request_id is required")
		return
	}
	if model.DB == nil {
		common.ApiErrorMsg(c, "database is not initialized")
		return
	}
	subsiteId, ok := optionalSubsiteIdQuery(c)
	if !ok {
		return
	}
	payload, severity, summary, _, artifactId, err := buildRequestDiagnosticReportPayload(requestId, subsiteId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	now := common.GetTimestamp()
	report := requestDiagnosticReportView{
		RequestId:   requestId,
		SubsiteId:   requestDiagnosticPayloadSubsiteId(payload, subsiteId),
		Status:      model.RequestDiagnosticStatusCompleted,
		Severity:    severity,
		Summary:     summary,
		GeneratedAt: now,
		Report:      payload,
	}
	zipBody, err := buildRequestDiagnosticBundleZip(c.Request.Context(), requestId, report, artifactId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	filename := "data-proxy-diagnostic-" + sanitizeDiagnosticBundleFileName(requestId) + ".zip"
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.Header("X-Data-Proxy-Request-Id", requestId)
	c.Data(http.StatusOK, "application/zip", zipBody)
}

func CleanupRequestCaptureData(c *gin.Context) {
	options := service.RequestCaptureCleanupOptionsFromEnv()
	if value, ok := requestCaptureCleanupIntQuery(c, "retention_days"); ok {
		options.RetentionDays = value
	}
	if value, ok := requestCaptureCleanupIntQuery(c, "spool_retention_days"); ok {
		options.SpoolRetentionDays = value
	}
	if value, ok := requestCaptureCleanupIntQuery(c, "limit"); ok {
		options.Limit = value
	}
	options.DryRun = requestCaptureCleanupBoolQuery(c, "dry_run")
	summary, err := service.CleanupExpiredRequestCaptureData(c.Request.Context(), options)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, summary)
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

func requestDiagnosticCandidateFiltersFromQuery(c *gin.Context) (requestDiagnosticCandidateFilters, bool) {
	if c == nil {
		return requestDiagnosticCandidateFilters{}, true
	}
	severity := strings.ToLower(strings.TrimSpace(c.Query("severity")))
	if severity == "" {
		severity = strings.ToLower(strings.TrimSpace(c.Query("status")))
	}
	channelId, _ := strconv.Atoi(strings.TrimSpace(c.Query("channel_id")))
	if channelId <= 0 {
		channelId, _ = strconv.Atoi(strings.TrimSpace(c.Query("channel")))
	}
	userId, _ := strconv.Atoi(strings.TrimSpace(c.Query("user_id")))
	tokenId, _ := strconv.Atoi(strings.TrimSpace(c.Query("token_id")))
	subsiteId, ok := optionalSubsiteIdQuery(c)
	if !ok {
		return requestDiagnosticCandidateFilters{}, false
	}
	return requestDiagnosticCandidateFilters{
		Severity:     severity,
		Source:       normalizeRequestDiagnosticCandidateSource(c.Query("source")),
		ModelName:    strings.TrimSpace(c.Query("model_name")),
		ChannelId:    channelId,
		Group:        strings.TrimSpace(c.Query("group")),
		ReportStatus: strings.ToLower(strings.TrimSpace(c.Query("report_status"))),
		UserId:       userId,
		TokenId:      tokenId,
		SubsiteId:    subsiteId,
	}, true
}

func requestDiagnosticCandidateScanLimit(limit int, filters requestDiagnosticCandidateFilters) int {
	if limit <= 0 {
		limit = 50
	}
	if !filters.HasFilter() {
		return limit
	}
	scanLimit := limit * 10
	if scanLimit < 100 {
		scanLimit = 100
	}
	if scanLimit > 1000 {
		scanLimit = 1000
	}
	return scanLimit
}

func requestCaptureCleanupIntQuery(c *gin.Context, name string) (int, bool) {
	raw := strings.TrimSpace(c.Query(name))
	if raw == "" {
		return 0, false
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, false
	}
	return value, true
}

func requestCaptureCleanupBoolQuery(c *gin.Context, name string) bool {
	switch strings.ToLower(strings.TrimSpace(c.Query(name))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func buildRequestDiagnosticBundleZip(ctx context.Context, requestId string, report requestDiagnosticReportView, artifactId int64) ([]byte, error) {
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	if err := writeDiagnosticZipJSON(writer, "diagnostic/report.json", report); err != nil {
		_ = writer.Close()
		return nil, err
	}
	if err := writeDiagnosticZipJSON(writer, "diagnostic/trace.json", report.Report.Trace); err != nil {
		_ = writer.Close()
		return nil, err
	}
	if report.Report.Capture != nil {
		if err := writeDiagnosticZipJSON(writer, "diagnostic/capture.json", report.Report.Capture); err != nil {
			_ = writer.Close()
			return nil, err
		}
	}
	if err := writeDiagnosticZipJSON(writer, "diagnostic/findings.json", report.Report.Findings); err != nil {
		_ = writer.Close()
		return nil, err
	}
	artifact, err := loadRequestDiagnosticRawBundleArtifact(requestId, artifactId)
	if err != nil {
		_ = writer.Close()
		return nil, err
	}
	if artifact == nil {
		if err := writeDiagnosticZipText(writer, "capture/raw_bundle_unavailable.txt", "no available raw capture bundle artifact for request_id "+requestId+"\n"); err != nil {
			_ = writer.Close()
			return nil, err
		}
	} else {
		maxRawBytes := requestDiagnosticBundleMaxRawTarBytes()
		tarBody, err := service.LoadDecodedRequestCaptureArtifactBundleWithLimit(ctx, *artifact, maxRawBytes)
		if err != nil {
			message := err.Error() + "\n"
			target := "capture/raw_bundle_error.txt"
			if errors.Is(err, service.ErrRequestCaptureBundleDecodedTooLarge) {
				target = "capture/raw_bundle_skipped.txt"
				message = fmt.Sprintf("raw capture bundle is larger than DIAGNOSTIC_BUNDLE_MAX_RAW_TAR_BYTES=%d; increase the limit or inspect the object storage artifact directly\n", maxRawBytes)
			}
			if writeErr := writeDiagnosticZipText(writer, target, message); writeErr != nil {
				_ = writer.Close()
				return nil, writeErr
			}
		} else if err := addDiagnosticTarToZip(writer, "capture/raw/", tarBody); err != nil {
			_ = writer.Close()
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func requestDiagnosticBundleMaxRawTarBytes() int64 {
	value := common.GetEnvOrDefault(requestDiagnosticBundleMaxRawTarBytesEnv, requestDiagnosticBundleDefaultMaxRawTarBytes)
	if value < 0 {
		return requestDiagnosticBundleDefaultMaxRawTarBytes
	}
	return int64(value)
}

func loadRequestDiagnosticRawBundleArtifact(requestId string, preferredArtifactId int64) (*model.RequestCaptureArtifact, error) {
	if model.DB == nil {
		return nil, nil
	}
	var artifact model.RequestCaptureArtifact
	tx := model.DB.Where("request_id = ? AND kind = ? AND status = ?", requestId, model.RequestCaptureArtifactKindRawBundle, model.RequestCaptureArtifactStatusAvailable)
	if preferredArtifactId > 0 {
		err := tx.Where("id = ?", preferredArtifactId).First(&artifact).Error
		if err == nil {
			return &artifact, nil
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}
	err := model.DB.Where("request_id = ? AND kind = ? AND status = ?", requestId, model.RequestCaptureArtifactKindRawBundle, model.RequestCaptureArtifactStatusAvailable).
		Order("uploaded_at desc, id desc").
		First(&artifact).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &artifact, nil
}

func writeDiagnosticZipJSON(writer *zip.Writer, name string, value interface{}) error {
	body, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	return writeDiagnosticZipFile(writer, name, body)
}

func writeDiagnosticZipText(writer *zip.Writer, name string, value string) error {
	return writeDiagnosticZipFile(writer, name, []byte(value))
}

func writeDiagnosticZipFile(writer *zip.Writer, name string, body []byte) error {
	file, err := writer.Create(name)
	if err != nil {
		return err
	}
	_, err = file.Write(body)
	return err
}

func addDiagnosticTarToZip(writer *zip.Writer, prefix string, tarBody []byte) error {
	reader := tar.NewReader(bytes.NewReader(tarBody))
	added := 0
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		if header == nil || header.Typeflag != tar.TypeReg {
			continue
		}
		name, ok := cleanDiagnosticTarEntryName(header.Name)
		if !ok {
			continue
		}
		file, err := writer.Create(prefix + name)
		if err != nil {
			return err
		}
		if _, err := io.Copy(file, reader); err != nil {
			return err
		}
		added++
	}
	if added == 0 {
		return writeDiagnosticZipText(writer, prefix+"empty.txt", "raw capture bundle did not contain regular files\n")
	}
	return nil
}

func cleanDiagnosticTarEntryName(name string) (string, bool) {
	name = strings.TrimSpace(strings.ReplaceAll(name, "\\", "/"))
	if name == "" {
		return "", false
	}
	name = strings.TrimPrefix(name, "/")
	name = path.Clean(name)
	if name == "." || strings.HasPrefix(name, "../") || strings.Contains(name, "/../") {
		return "", false
	}
	return name, true
}

func sanitizeDiagnosticBundleFileName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "request"
	}
	var builder strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			builder.WriteRune(r)
			continue
		}
		builder.WriteByte('_')
	}
	result := strings.Trim(builder.String(), "._-")
	if result == "" {
		return "request"
	}
	return result
}

func collectRequestDiagnosticLogCandidates(candidates map[string]*requestDiagnosticCandidate, limit int, startTimestamp int64, endTimestamp int64, subsiteId *int64) error {
	if model.LOG_DB == nil {
		return errors.New("log database is not initialized")
	}
	var logs []model.Log
	tx := model.LOG_DB.Model(&model.Log{}).Where("(request_id <> '' OR upstream_request_id <> '')").Where("type = ?", model.LogTypeError)
	if subsiteId != nil {
		tx = tx.Where("subsite_id = ?", *subsiteId)
	}
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
		if log.SubsiteId != 0 || candidate.SubsiteId == 0 {
			candidate.SubsiteId = log.SubsiteId
		}
	}
	return nil
}

func collectRequestDiagnosticTraceMetaCandidates(candidates map[string]*requestDiagnosticCandidate, limit int, startTimestamp int64, endTimestamp int64, subsiteId *int64) error {
	if model.LOG_DB == nil {
		return errors.New("log database is not initialized")
	}
	var logs []model.Log
	tx := model.LOG_DB.Model(&model.Log{}).
		Where("(request_id <> '' OR upstream_request_id <> '')").
		Where("type = ?", model.LogTypeConsume).
		Where("other <> ''")
	if subsiteId != nil {
		tx = tx.Where("subsite_id = ?", *subsiteId)
	}
	if startTimestamp > 0 {
		tx = tx.Where("created_at >= ?", startTimestamp)
	}
	if endTimestamp > 0 {
		tx = tx.Where("created_at <= ?", endTimestamp)
	}
	if err := tx.Order("created_at desc, id desc").Limit(limit * 4).Find(&logs).Error; err != nil {
		return err
	}
	for _, log := range logs {
		other := parseRequestLogOther(log.Other, true)
		meta, _ := other["request_conversion_meta"].(map[string]interface{})
		traceSeverity, traceSummary, hasTraceAnomaly := requestDiagnosticTraceMetaCandidate(meta)
		failoverSeverity, failoverSummary, hasFailoverAnomaly := requestDiagnosticChannelFailoverCandidate(other)
		if !hasTraceAnomaly && !hasFailoverAnomaly {
			continue
		}
		requestId := strings.TrimSpace(log.RequestId)
		if requestId == "" {
			requestId = strings.TrimSpace(log.UpstreamRequestId)
		}
		if requestId == "" {
			continue
		}
		candidate := ensureRequestDiagnosticCandidate(candidates, requestId)
		if hasTraceAnomaly {
			candidate.Severity = requestDiagnosticMaxSeverity(candidate.Severity, traceSeverity)
			candidate.Source = requestDiagnosticAppendSource(candidate.Source, "trace_meta")
			if candidate.Summary == "" {
				candidate.Summary = traceSummary
			}
		}
		if hasFailoverAnomaly {
			candidate.Severity = requestDiagnosticMaxSeverity(candidate.Severity, failoverSeverity)
			candidate.Source = requestDiagnosticAppendSource(candidate.Source, "channel_failover")
			if candidate.Summary == "" {
				candidate.Summary = failoverSummary
			}
		}
		if log.CreatedAt > candidate.LastSeenAt {
			candidate.LastSeenAt = log.CreatedAt
		}
		candidate.ConsumeCount++
		hydrateRequestDiagnosticCandidateFromLog(candidate, log)
	}
	return nil
}

func requestDiagnosticTraceMetaCandidate(meta map[string]interface{}) (string, string, bool) {
	if len(meta) == 0 {
		return "", "", false
	}
	if rawError := strings.TrimSpace(common.Interface2String(meta["hosted_web_search_executor_error"])); rawError != "" {
		return "error", "web_search executor bridge error: " + rawError, true
	}
	if rejected, ok := meta["hosted_tools_rejected"].(bool); ok && rejected {
		return "error", "hosted tools rejected by channel policy", true
	}
	if status := strings.TrimSpace(common.Interface2String(meta["responses_terminal_status"])); status != "" && status != "completed" {
		severity := "warning"
		if status == "failed" {
			severity = "error"
		}
		return severity, "Responses terminal status: " + status, true
	}
	if filteredTools := requestDiagnosticStringSlice(meta["hosted_tools_filtered"]); len(filteredTools) > 0 {
		effect := strings.TrimSpace(common.Interface2String(meta["hosted_tools_policy_effect"]))
		directAnswerHint := requestDiagnosticBool(meta["hosted_tools_direct_answer_hint"])
		if effect == "direct_answer" || effect == "executor_bridge_fallback" || directAnswerHint {
			summary := "Hosted tools filtered: " + strings.Join(filteredTools, ", ")
			if effect != "" {
				summary += " (" + effect + ")"
			}
			return "warning", summary, true
		}
	}
	return "", "", false
}

func requestDiagnosticChannelFailoverCandidate(other map[string]interface{}) (string, string, bool) {
	if len(other) == 0 {
		return "", "", false
	}
	adminInfo, ok := other["admin_info"].(map[string]interface{})
	if !ok || adminInfo == nil {
		return "", "", false
	}
	events := requestDiagnosticChannelFailoverEvents(adminInfo)
	if len(events) == 0 {
		return "", "", false
	}
	failedCount := 0
	retryPlanned := false
	retryBlocked := false
	details := make([]string, 0, 1)
	for _, event := range events {
		if strings.TrimSpace(common.Interface2String(event["event"])) != "failed" {
			continue
		}
		failedCount++
		if planned, ok := event["retry_planned"].(bool); ok {
			if planned {
				retryPlanned = true
			} else {
				retryBlocked = true
			}
		}
		if len(details) == 0 {
			if detail := requestDiagnosticChannelFailoverDetail(event); detail != "" {
				details = append(details, detail)
			}
		}
	}
	if failedCount == 0 {
		return "", "", false
	}
	severity := "warning"
	summary := "请求发生渠道失败或切换"
	if retryPlanned {
		summary = "请求发生渠道失败并已尝试切换"
	}
	if retryBlocked && !retryPlanned {
		severity = "error"
		summary = "请求发生渠道失败且未计划重试"
	}
	if failedCount > 1 {
		summary += "（" + strconv.Itoa(failedCount) + " 次）"
	}
	if len(details) > 0 {
		summary += ": " + details[0]
	}
	return severity, summary, true
}

func hydrateRequestDiagnosticCandidateFromLog(candidate *requestDiagnosticCandidate, log model.Log) {
	if candidate == nil {
		return
	}
	if log.SubsiteId != 0 || candidate.SubsiteId == 0 {
		candidate.SubsiteId = log.SubsiteId
	}
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

func collectRequestDiagnosticCaptureCandidates(candidates map[string]*requestDiagnosticCandidate, limit int, startTimestamp int64, endTimestamp int64, subsiteId *int64) error {
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
	if subsiteId != nil {
		tx = tx.Where("subsite_id = ?", *subsiteId)
	}
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
		if capture.SubsiteId != 0 || candidate.SubsiteId == 0 {
			candidate.SubsiteId = capture.SubsiteId
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

func normalizeRequestDiagnosticCandidateSource(source string) string {
	source = strings.ToLower(strings.TrimSpace(source))
	switch source {
	case "error", "log", "logs":
		return "log_error"
	case "trace", "conversion", "conversion_trace":
		return "trace_meta"
	case "failover":
		return "channel_failover"
	default:
		return source
	}
}

func requestDiagnosticCandidateSourceContains(source string, expected string) bool {
	expected = normalizeRequestDiagnosticCandidateSource(expected)
	if expected == "" {
		return true
	}
	for _, item := range strings.Split(source, ",") {
		if normalizeRequestDiagnosticCandidateSource(item) == expected {
			return true
		}
	}
	return false
}

func requestDiagnosticCandidateMatchesFilters(candidate requestDiagnosticCandidate, filters requestDiagnosticCandidateFilters) bool {
	if filters.Severity != "" && strings.ToLower(candidate.Severity) != filters.Severity {
		return false
	}
	if filters.Source != "" && !requestDiagnosticCandidateSourceContains(candidate.Source, filters.Source) {
		return false
	}
	if filters.ModelName != "" && !strings.Contains(strings.ToLower(candidate.ModelName), strings.ToLower(filters.ModelName)) {
		return false
	}
	if filters.ChannelId > 0 && candidate.ChannelId != filters.ChannelId {
		return false
	}
	if filters.Group != "" && !strings.EqualFold(candidate.Group, filters.Group) {
		return false
	}
	if filters.ReportStatus != "" && strings.ToLower(candidate.ReportStatus) != filters.ReportStatus {
		return false
	}
	if filters.UserId > 0 && candidate.UserId != filters.UserId {
		return false
	}
	if filters.TokenId > 0 && candidate.TokenId != filters.TokenId {
		return false
	}
	if filters.SubsiteId != nil && candidate.SubsiteId != *filters.SubsiteId {
		return false
	}
	return true
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

func hydrateRequestDiagnosticCandidateReports(items []requestDiagnosticCandidate, subsiteId *int64) {
	if model.DB == nil || len(items) == 0 {
		return
	}
	requestIds := make([]string, 0, len(items))
	for _, item := range items {
		requestIds = append(requestIds, item.RequestId)
	}
	var reports []model.RequestDiagnosticReport
	tx := model.DB.Where("request_id IN ?", requestIds)
	if subsiteId != nil {
		tx = tx.Where("subsite_id = ?", *subsiteId)
	}
	if err := tx.Order("generated_at desc, id desc").Find(&reports).Error; err != nil {
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

func buildRequestDiagnosticReportPayload(requestId string, subsiteId *int64) (requestDiagnosticReportPayload, string, string, int64, int64, error) {
	logs, err := model.GetLogsByRequestId(requestId, 0, false, subsiteId)
	if err != nil {
		return requestDiagnosticReportPayload{}, "", "", 0, 0, err
	}
	trace := buildRequestLogTraceResponse(requestId, "admin", logs, true)
	capture, captureId, artifactId, err := loadRequestDiagnosticCaptureSummary(requestId, subsiteId)
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

func requestDiagnosticPayloadSubsiteId(payload requestDiagnosticReportPayload, requested *int64) int64 {
	if requested != nil {
		return *requested
	}
	if payload.Capture != nil && payload.Capture.SubsiteId != 0 {
		return payload.Capture.SubsiteId
	}
	if len(payload.Trace.SubsiteIds) == 1 {
		return payload.Trace.SubsiteIds[0]
	}
	return 0
}

func loadRequestDiagnosticCaptureSummary(requestId string, subsiteId *int64) (*requestDiagnosticCaptureSummary, int64, int64, error) {
	var record model.RequestCaptureRecord
	tx := model.DB.Where("request_id = ?", requestId)
	if subsiteId != nil {
		tx = tx.Where("subsite_id = ?", *subsiteId)
	}
	err := tx.Order("id desc").First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, 0, 0, nil
	}
	if err != nil {
		return nil, 0, 0, err
	}
	var artifacts []model.RequestCaptureArtifact
	artifactQuery := model.DB.Where("request_id = ?", requestId)
	if record.Id > 0 {
		artifactQuery = artifactQuery.Where("(capture_id = ? OR capture_id = 0)", record.Id)
	}
	if err := artifactQuery.Order("id desc").Find(&artifacts).Error; err != nil {
		return nil, record.Id, 0, err
	}
	summary := &requestDiagnosticCaptureSummary{
		Id:                   record.Id,
		RequestId:            record.RequestId,
		UpstreamRequestId:    record.UpstreamRequestId,
		SubsiteId:            record.SubsiteId,
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
		Metadata:             requestDiagnosticCaptureMetadata(record.MetadataJson),
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
		if requestDiagnosticBool(capture.Metadata["capture_truncated"]) {
			findings = append(findings, requestDiagnosticFinding{
				Level:   "warning",
				Code:    "capture_truncated",
				Message: "请求捕获数据被截断",
				Detail:  strings.Join(requestDiagnosticStringSlice(capture.Metadata["capture_truncated_artifacts"]), ", "),
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
	findings = append(findings, requestDiagnosticChannelFailoverFindings(trace)...)
	return findings
}

func requestDiagnosticTraceMeta(trace requestLogTraceResponse) map[string]interface{} {
	meta, ok := trace.Diagnostics["request_conversion_meta"].(map[string]interface{})
	if !ok || meta == nil {
		return map[string]interface{}{}
	}
	return meta
}

func requestDiagnosticChannelFailoverFindings(trace requestLogTraceResponse) []requestDiagnosticFinding {
	adminInfo, ok := trace.Diagnostics["admin_info"].(map[string]interface{})
	if !ok || adminInfo == nil {
		return nil
	}
	events := requestDiagnosticChannelFailoverEvents(adminInfo)
	if len(events) == 0 {
		return nil
	}

	failedDetails := make([]string, 0)
	for _, event := range events {
		if strings.TrimSpace(common.Interface2String(event["event"])) != "failed" {
			continue
		}
		detail := requestDiagnosticChannelFailoverDetail(event)
		if detail != "" {
			failedDetails = append(failedDetails, detail)
		}
	}
	if len(failedDetails) == 0 {
		return nil
	}

	return []requestDiagnosticFinding{
		{
			Level:   "warning",
			Code:    "channel_failover_failed",
			Message: "请求链路发生渠道失败或切换",
			Detail:  strings.Join(failedDetails, "\n"),
		},
	}
}

func requestDiagnosticChannelFailoverEvents(adminInfo map[string]interface{}) []map[string]interface{} {
	if len(adminInfo) == 0 {
		return nil
	}
	switch rawEvents := adminInfo["channel_failover"].(type) {
	case []interface{}:
		events := make([]map[string]interface{}, 0, len(rawEvents))
		for _, rawEvent := range rawEvents {
			event, ok := rawEvent.(map[string]interface{})
			if !ok {
				continue
			}
			events = append(events, event)
		}
		return events
	case []map[string]interface{}:
		return rawEvents
	default:
		return nil
	}
}

func requestDiagnosticChannelFailoverDetail(event map[string]interface{}) string {
	parts := make([]string, 0)
	channelId := common.Interface2String(event["channel_id"])
	channelName := strings.TrimSpace(common.Interface2String(event["channel_name"]))
	if channelId != "" {
		channel := "#" + channelId
		if channelName != "" {
			channel += " " + channelName
		}
		parts = append(parts, "channel="+channel)
	}
	if statusCode := common.Interface2String(event["status_code"]); statusCode != "" {
		parts = append(parts, "status="+statusCode)
	}
	if errorCode := common.Interface2String(event["error_code"]); errorCode != "" {
		parts = append(parts, "error_code="+errorCode)
	}
	if retryPlanned, ok := event["retry_planned"].(bool); ok {
		parts = append(parts, fmt.Sprintf("retry_planned=%t", retryPlanned))
	}
	if remaining := common.Interface2String(event["remaining_retries"]); remaining != "" {
		parts = append(parts, "remaining_retries="+remaining)
	}
	if healthAction := common.Interface2String(event["health_action"]); healthAction != "" {
		parts = append(parts, "health_action="+healthAction)
	}
	if runtimeStatus := common.Interface2String(event["runtime_status"]); runtimeStatus != "" {
		parts = append(parts, "runtime_status="+runtimeStatus)
	}
	if cooldownUntil := common.Interface2String(event["cooldown_until"]); cooldownUntil != "" {
		parts = append(parts, "cooldown_until="+cooldownUntil)
	}
	if reason := strings.TrimSpace(common.Interface2String(event["reason"])); reason != "" {
		parts = append(parts, "reason="+reason)
	}
	return strings.Join(parts, " ")
}

func requestDiagnosticCaptureMetadata(value string) map[string]interface{} {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	var metadata map[string]interface{}
	if err := json.Unmarshal([]byte(value), &metadata); err != nil {
		return nil
	}
	if len(metadata) == 0 {
		return nil
	}
	return metadata
}

func requestDiagnosticBool(value interface{}) bool {
	switch current := value.(type) {
	case bool:
		return current
	case string:
		switch strings.ToLower(strings.TrimSpace(current)) {
		case "1", "true", "yes", "on":
			return true
		default:
			return false
		}
	default:
		return false
	}
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
		SubsiteId:   report.SubsiteId,
		Status:      report.Status,
		Severity:    report.Severity,
		Summary:     report.Summary,
		GeneratedAt: report.GeneratedAt,
		Report:      payload,
	}, nil
}
