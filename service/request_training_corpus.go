package service

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/klauspost/compress/zstd"
	"gorm.io/gorm"
)

const (
	trainingCorpusDefaultName                 = "request-captures"
	trainingCorpusDefaultLimit                = 100
	trainingCorpusDefaultMaxDecodedBundleSize = 64 * 1024 * 1024
	trainingCorpusSchemaVersion               = "data_proxy.training.sample.v1"
	trainingCorpusOutputFormat                = "jsonl.zst"
	trainingCorpusRedactionVersion            = "basic-json-key-redaction-v1"
	trainingCorpusContentType                 = "application/x-ndjson"
)

type TrainingCorpusBuildOptions struct {
	Name                  string
	Version               string
	StartTimestamp        int64
	EndTimestamp          int64
	Limit                 int
	MaxDecodedBundleBytes int64
	IncludeErrored        bool
	OutputStorageKey      string
	Now                   func() int64
}

type TrainingCorpusBuildResult struct {
	Dataset model.TrainingDatasetVersion `json:"dataset"`
	Object  RequestCaptureObject         `json:"object"`
	Samples []model.TrainingSample       `json:"samples"`
	Skipped int                          `json:"skipped"`
	Errors  []string                     `json:"errors,omitempty"`
}

type trainingCorpusSampleLine struct {
	Schema           string                 `json:"schema"`
	SampleType       string                 `json:"sample_type"`
	RequestId        string                 `json:"request_id"`
	CaptureId        int64                  `json:"capture_id"`
	ArtifactId       int64                  `json:"artifact_id"`
	ModelName        string                 `json:"model_name,omitempty"`
	RequestPath      string                 `json:"request_path,omitempty"`
	ProtocolChain    string                 `json:"protocol_chain,omitempty"`
	IsStream         bool                   `json:"is_stream"`
	Input            any                    `json:"input,omitempty"`
	Output           map[string]any         `json:"output,omitempty"`
	Source           map[string]any         `json:"source"`
	RedactionStatus  string                 `json:"redaction_status"`
	RedactionVersion string                 `json:"redaction_version"`
	QualityScore     int                    `json:"quality_score"`
	Tags             []string               `json:"tags,omitempty"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
	SourceHash       string                 `json:"source_hash"`
	CreatedAt        int64                  `json:"created_at"`
}

type trainingCorpusBuiltSample struct {
	line   trainingCorpusSampleLine
	sample model.TrainingSample
}

func BuildTrainingCorpusDataset(ctx context.Context, options TrainingCorpusBuildOptions) (TrainingCorpusBuildResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if model.DB == nil {
		return TrainingCorpusBuildResult{}, errors.New("database is not initialized")
	}
	name := strings.TrimSpace(options.Name)
	if name == "" {
		name = trainingCorpusDefaultName
	}
	now := trainingCorpusNow(options)
	version := strings.TrimSpace(options.Version)
	if version == "" {
		version = time.Unix(now, 0).UTC().Format("20060102T150405Z")
	}
	limit := options.Limit
	if limit <= 0 {
		limit = trainingCorpusDefaultLimit
	}
	maxDecodedBytes := options.MaxDecodedBundleBytes
	if maxDecodedBytes <= 0 {
		maxDecodedBytes = trainingCorpusDefaultMaxDecodedBundleSize
	}
	scope := trainingCorpusSourceScope(options, limit, maxDecodedBytes)
	scopeJSON, _ := common.Marshal(scope)
	redactionJSON, _ := common.Marshal(map[string]any{
		"version": trainingCorpusRedactionVersion,
		"mode":    "basic_json_key_redaction",
	})
	dataset := model.TrainingDatasetVersion{
		Name:                name,
		Version:             version,
		Status:              model.TrainingDatasetStatusBuilding,
		OutputFormat:        trainingCorpusOutputFormat,
		SourceScopeJson:     string(scopeJSON),
		RedactionPolicyJson: string(redactionJSON),
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	if err := model.DB.WithContext(ctx).Create(&dataset).Error; err != nil {
		return TrainingCorpusBuildResult{}, err
	}

	result := TrainingCorpusBuildResult{Dataset: dataset}
	fail := func(err error) (TrainingCorpusBuildResult, error) {
		message := truncateRequestCaptureError(err.Error())
		_ = model.DB.WithContext(ctx).Model(&model.TrainingDatasetVersion{}).Where("id = ?", dataset.Id).Updates(map[string]interface{}{
			"status":     model.TrainingDatasetStatusFailed,
			"last_error": message,
			"updated_at": common.GetTimestamp(),
		}).Error
		dataset.Status = model.TrainingDatasetStatusFailed
		dataset.LastError = message
		result.Dataset = dataset
		return result, err
	}

	artifacts, err := listTrainingCorpusSourceArtifacts(ctx, options, limit)
	if err != nil {
		return fail(err)
	}
	built, skipped, buildErrors := buildTrainingCorpusSamples(ctx, artifacts, options, maxDecodedBytes, now)
	result.Skipped = skipped
	result.Errors = buildErrors
	jsonl, err := encodeTrainingCorpusJSONL(built)
	if err != nil {
		return fail(err)
	}
	compressed, err := compressTrainingCorpusJSONLZstd(jsonl)
	if err != nil {
		return fail(err)
	}
	storageKey := strings.Trim(strings.TrimSpace(options.OutputStorageKey), "/")
	if storageKey == "" {
		storageKey = trainingCorpusStorageKey(name, version, now)
	}
	object, err := SaveRequestCaptureObject(ctx, RequestCaptureObject{
		RequestId:   name + "-" + version,
		Kind:        model.RequestCaptureArtifactKindTrainingDataset,
		ContentType: trainingCorpusContentType,
		StorageKey:  storageKey,
		CreatedAt:   now,
	}, compressed)
	if err != nil {
		return fail(err)
	}
	manifestJSON, _ := common.Marshal(map[string]any{
		"schema":            trainingCorpusSchemaVersion,
		"sample_count":      len(built),
		"skipped":           skipped,
		"errors":            buildErrors,
		"source_artifacts":  len(artifacts),
		"jsonl_bytes":       len(jsonl),
		"compressed_bytes":  len(compressed),
		"redaction_version": trainingCorpusRedactionVersion,
		"storage_key":       object.StorageKey,
	})
	if err := model.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for i := range built {
			built[i].sample.DatasetVersionId = dataset.Id
			if err := tx.Create(&built[i].sample).Error; err != nil {
				return err
			}
		}
		updates := map[string]interface{}{
			"status":              model.TrainingDatasetStatusCompleted,
			"provider":            object.Provider,
			"bucket":              object.Bucket,
			"storage_key":         object.StorageKey,
			"sha256":              object.SHA256,
			"size_bytes":          object.BodyBytes,
			"sample_count":        int64(len(built)),
			"build_manifest_json": string(manifestJSON),
			"last_error":          "",
			"built_at":            now,
			"updated_at":          now,
		}
		return tx.Model(&model.TrainingDatasetVersion{}).Where("id = ?", dataset.Id).Updates(updates).Error
	}); err != nil {
		return fail(err)
	}
	if err := model.DB.WithContext(ctx).First(&dataset, dataset.Id).Error; err != nil {
		return fail(err)
	}
	result.Dataset = dataset
	result.Object = object
	result.Samples = make([]model.TrainingSample, 0, len(built))
	for _, item := range built {
		result.Samples = append(result.Samples, item.sample)
	}
	return result, nil
}

func listTrainingCorpusSourceArtifacts(ctx context.Context, options TrainingCorpusBuildOptions, limit int) ([]model.RequestCaptureArtifact, error) {
	tx := model.DB.WithContext(ctx).Model(&model.RequestCaptureArtifact{}).
		Where("kind = ? AND status = ?", model.RequestCaptureArtifactKindRawBundle, model.RequestCaptureArtifactStatusAvailable)
	if options.StartTimestamp > 0 {
		tx = tx.Where("uploaded_at >= ? OR created_at >= ?", options.StartTimestamp, options.StartTimestamp)
	}
	if options.EndTimestamp > 0 {
		tx = tx.Where("uploaded_at <= ? OR created_at <= ?", options.EndTimestamp, options.EndTimestamp)
	}
	var artifacts []model.RequestCaptureArtifact
	if err := tx.Order("uploaded_at asc, id asc").Limit(limit).Find(&artifacts).Error; err != nil {
		return nil, err
	}
	return artifacts, nil
}

func buildTrainingCorpusSamples(ctx context.Context, artifacts []model.RequestCaptureArtifact, options TrainingCorpusBuildOptions, maxDecodedBytes int64, now int64) ([]trainingCorpusBuiltSample, int, []string) {
	built := make([]trainingCorpusBuiltSample, 0, len(artifacts))
	seen := map[string]bool{}
	skipped := 0
	buildErrors := make([]string, 0)
	for _, artifact := range artifacts {
		if err := ctx.Err(); err != nil {
			buildErrors = append(buildErrors, err.Error())
			break
		}
		record, ok, err := loadTrainingCorpusCaptureRecord(ctx, artifact)
		if err != nil {
			skipped++
			buildErrors = append(buildErrors, fmt.Sprintf("%s: %s", artifact.RequestId, err.Error()))
			continue
		}
		if !ok {
			skipped++
			continue
		}
		if record.HasError && !options.IncludeErrored {
			skipped++
			continue
		}
		item, err := buildTrainingCorpusSample(ctx, record, artifact, maxDecodedBytes, now)
		if err != nil {
			skipped++
			buildErrors = append(buildErrors, fmt.Sprintf("%s: %s", artifact.RequestId, err.Error()))
			continue
		}
		if item.line.SourceHash == "" || seen[item.line.SourceHash] {
			skipped++
			continue
		}
		seen[item.line.SourceHash] = true
		built = append(built, item)
	}
	return built, skipped, buildErrors
}

func loadTrainingCorpusCaptureRecord(ctx context.Context, artifact model.RequestCaptureArtifact) (model.RequestCaptureRecord, bool, error) {
	var record model.RequestCaptureRecord
	tx := model.DB.WithContext(ctx)
	if artifact.CaptureId > 0 {
		err := tx.Where("id = ?", artifact.CaptureId).First(&record).Error
		if err == nil {
			return record, true, nil
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return model.RequestCaptureRecord{}, false, err
		}
	}
	requestId := strings.TrimSpace(artifact.RequestId)
	if requestId == "" {
		return model.RequestCaptureRecord{}, false, nil
	}
	err := tx.Where("request_id = ?", requestId).Order("id desc").First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.RequestCaptureRecord{}, false, nil
	}
	if err != nil {
		return model.RequestCaptureRecord{}, false, err
	}
	return record, true, nil
}

func buildTrainingCorpusSample(ctx context.Context, record model.RequestCaptureRecord, artifact model.RequestCaptureArtifact, maxDecodedBytes int64, now int64) (trainingCorpusBuiltSample, error) {
	body, err := LoadDecodedRequestCaptureArtifactBundleWithLimit(ctx, artifact, maxDecodedBytes)
	if err != nil {
		return trainingCorpusBuiltSample{}, err
	}
	files, manifest, err := readTrainingCorpusBundleFiles(body)
	if err != nil {
		return trainingCorpusBuiltSample{}, err
	}
	input, inputArtifact := trainingCorpusInput(files)
	output, outputArtifact := trainingCorpusOutput(files)
	if input == nil && output == nil {
		return trainingCorpusBuiltSample{}, errors.New("capture bundle has no usable input or output artifacts")
	}
	truncated := trainingCorpusManifestHasTruncatedArtifacts(manifest)
	sampleType := trainingCorpusSampleType(record, output)
	qualityScore := trainingCorpusQualityScore(record, output, truncated)
	sourceHash := trainingCorpusSourceHash(input, output)
	tags := trainingCorpusTags(record, manifest, outputArtifact)
	metadata := map[string]interface{}{
		"capture_level":  record.CaptureLevel,
		"capture_status": record.CaptureStatus,
		"has_error":      record.HasError,
		"truncated":      truncated,
	}
	line := trainingCorpusSampleLine{
		Schema:           trainingCorpusSchemaVersion,
		SampleType:       sampleType,
		RequestId:        record.RequestId,
		CaptureId:        record.Id,
		ArtifactId:       artifact.Id,
		ModelName:        record.ModelName,
		RequestPath:      record.RequestPath,
		ProtocolChain:    record.ProtocolChain,
		IsStream:         record.IsStream || strings.HasSuffix(outputArtifact, ".sse"),
		Input:            input,
		Output:           output,
		Source:           trainingCorpusSource(record, artifact, inputArtifact, outputArtifact),
		RedactionStatus:  "basic",
		RedactionVersion: trainingCorpusRedactionVersion,
		QualityScore:     qualityScore,
		Tags:             tags,
		Metadata:         metadata,
		SourceHash:       sourceHash,
		CreatedAt:        now,
	}
	tagsJSON, _ := common.Marshal(tags)
	metadataJSON, _ := common.Marshal(metadata)
	return trainingCorpusBuiltSample{
		line: line,
		sample: model.TrainingSample{
			RequestId:       record.RequestId,
			CaptureId:       record.Id,
			ArtifactId:      artifact.Id,
			ModelName:       record.ModelName,
			SourceHash:      sourceHash,
			RedactionStatus: trainingCorpusRedactionVersion,
			QualityScore:    qualityScore,
			ReviewStatus:    model.TrainingSampleReviewStatusPending,
			TagsJson:        string(tagsJSON),
			MetadataJson:    string(metadataJSON),
			CreatedAt:       now,
			UpdatedAt:       now,
		},
	}, nil
}

func readTrainingCorpusBundleFiles(body []byte) (map[string][]byte, RequestCaptureSpoolManifest, error) {
	reader := tar.NewReader(bytes.NewReader(body))
	files := map[string][]byte{}
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, RequestCaptureSpoolManifest{}, err
		}
		if header == nil || header.Typeflag != tar.TypeReg {
			continue
		}
		name := path.Clean(strings.TrimPrefix(strings.ReplaceAll(header.Name, "\\", "/"), "/"))
		if name == "." || strings.HasPrefix(name, "../") || strings.Contains(name, "/../") {
			continue
		}
		body, err := io.ReadAll(reader)
		if err != nil {
			return nil, RequestCaptureSpoolManifest{}, err
		}
		files[name] = body
	}
	var manifest RequestCaptureSpoolManifest
	if raw := files[requestCaptureSpoolManifestName]; len(raw) > 0 {
		_ = json.Unmarshal(raw, &manifest)
	}
	return files, manifest, nil
}

func trainingCorpusInput(files map[string][]byte) (any, string) {
	for _, name := range []string{"artifacts/client_request.json", "client_request.json"} {
		if body, ok := files[name]; ok {
			return trainingCorpusDecodePayload(body), name
		}
	}
	return nil, ""
}

func trainingCorpusOutput(files map[string][]byte) (map[string]any, string) {
	candidates := []string{
		"artifacts/downstream_response.json",
		"artifacts/downstream_response.sse",
		"artifacts/upstream_response.json",
		"artifacts/upstream_response.sse",
		"downstream_response.json",
		"downstream_response.sse",
		"upstream_response.json",
		"upstream_response.sse",
	}
	for _, name := range candidates {
		body, ok := files[name]
		if !ok {
			continue
		}
		output := trainingCorpusDecodeOutput(body, name)
		if len(output) == 0 {
			continue
		}
		return output, name
	}
	return nil, ""
}

func trainingCorpusDecodePayload(body []byte) any {
	var value any
	if json.Unmarshal(body, &value) == nil {
		return trainingCorpusRedactValue(value)
	}
	return map[string]any{
		"raw_text": strings.TrimSpace(string(body)),
	}
}

func trainingCorpusDecodeOutput(body []byte, artifactName string) map[string]any {
	var value any
	if json.Unmarshal(body, &value) == nil {
		redacted := trainingCorpusRedactValue(value)
		return map[string]any{
			"format":        "json",
			"text":          trainingCorpusOutputText(redacted),
			"reasoning":     trainingCorpusOutputReasoning(redacted),
			"payload":       redacted,
			"artifact_name": artifactName,
		}
	}
	if strings.HasSuffix(artifactName, ".sse") {
		text, reasoning, eventTypes := trainingCorpusSSEText(body)
		return map[string]any{
			"format":        "sse",
			"text":          text,
			"reasoning":     reasoning,
			"event_types":   eventTypes,
			"artifact_name": artifactName,
		}
	}
	return map[string]any{
		"format":        "text",
		"text":          strings.TrimSpace(string(body)),
		"artifact_name": artifactName,
	}
}

func trainingCorpusSSEText(body []byte) (string, string, []string) {
	blocks := strings.Split(strings.ReplaceAll(string(body), "\r\n", "\n"), "\n\n")
	var text strings.Builder
	var reasoning strings.Builder
	eventTypes := map[string]bool{}
	for _, block := range blocks {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}
		dataLines := make([]string, 0)
		for _, line := range strings.Split(block, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "event:") {
				eventType := strings.TrimSpace(strings.TrimPrefix(line, "event:"))
				if eventType != "" {
					eventTypes[eventType] = true
				}
				continue
			}
			if strings.HasPrefix(line, "data:") {
				dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
			}
		}
		data := strings.TrimSpace(strings.Join(dataLines, "\n"))
		if data == "" || data == "[DONE]" {
			continue
		}
		var value any
		if json.Unmarshal([]byte(data), &value) != nil {
			continue
		}
		if eventType := trainingCorpusEventType(value); eventType != "" {
			eventTypes[eventType] = true
		}
		text.WriteString(trainingCorpusOutputText(value))
		reasoning.WriteString(trainingCorpusOutputReasoning(value))
	}
	eventList := make([]string, 0, len(eventTypes))
	for eventType := range eventTypes {
		eventList = append(eventList, eventType)
	}
	sort.Strings(eventList)
	return text.String(), reasoning.String(), eventList
}

func trainingCorpusOutputText(value any) string {
	var builder strings.Builder
	trainingCorpusCollectOutputText(value, &builder)
	return builder.String()
}

func trainingCorpusCollectOutputText(value any, builder *strings.Builder) {
	switch current := value.(type) {
	case map[string]any:
		if delta := common.Interface2String(current["delta"]); delta != "" {
			if t := common.Interface2String(current["type"]); strings.Contains(t, "output_text") || strings.Contains(t, "text.delta") {
				builder.WriteString(delta)
			}
		}
		if text := common.Interface2String(current["output_text"]); text != "" {
			builder.WriteString(text)
		}
		if text := common.Interface2String(current["text"]); text != "" {
			if t := common.Interface2String(current["type"]); t == "output_text" || t == "text" || strings.Contains(t, "output_text") {
				builder.WriteString(text)
			}
		}
		if content := common.Interface2String(current["content"]); content != "" {
			if _, hasRole := current["role"]; hasRole {
				builder.WriteString(content)
			}
		}
		if choices, ok := current["choices"].([]any); ok {
			for _, choice := range choices {
				trainingCorpusCollectOutputText(choice, builder)
			}
			return
		}
		if deltaMap, ok := current["delta"].(map[string]any); ok {
			if content := common.Interface2String(deltaMap["content"]); content != "" {
				builder.WriteString(content)
			}
		}
		if message, ok := current["message"].(map[string]any); ok {
			if content := common.Interface2String(message["content"]); content != "" {
				builder.WriteString(content)
			}
		}
		if output, ok := current["output"].([]any); ok {
			for _, item := range output {
				trainingCorpusCollectOutputText(item, builder)
			}
		}
		if content, ok := current["content"].([]any); ok {
			for _, item := range content {
				trainingCorpusCollectOutputText(item, builder)
			}
		}
	case []any:
		for _, item := range current {
			trainingCorpusCollectOutputText(item, builder)
		}
	}
}

func trainingCorpusOutputReasoning(value any) string {
	var builder strings.Builder
	trainingCorpusCollectOutputReasoning(value, &builder)
	return builder.String()
}

func trainingCorpusCollectOutputReasoning(value any, builder *strings.Builder) {
	switch current := value.(type) {
	case map[string]any:
		for _, key := range []string{"reasoning", "reasoning_content", "reasoning_text"} {
			if text := common.Interface2String(current[key]); text != "" {
				builder.WriteString(text)
			}
		}
		for _, childKey := range []string{"choices", "delta", "message", "output", "content", "reasoning_details"} {
			switch child := current[childKey].(type) {
			case map[string]any:
				trainingCorpusCollectOutputReasoning(child, builder)
			case []any:
				for _, item := range child {
					trainingCorpusCollectOutputReasoning(item, builder)
				}
			}
		}
	case []any:
		for _, item := range current {
			trainingCorpusCollectOutputReasoning(item, builder)
		}
	}
}

func trainingCorpusEventType(value any) string {
	m, ok := value.(map[string]any)
	if !ok {
		return ""
	}
	return common.Interface2String(m["type"])
}

func trainingCorpusRedactValue(value any) any {
	switch current := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(current))
		for key, child := range current {
			if trainingCorpusSensitiveKey(key) {
				out[key] = "[REDACTED]"
				continue
			}
			out[key] = trainingCorpusRedactValue(child)
		}
		return out
	case []any:
		out := make([]any, len(current))
		for i, child := range current {
			out[i] = trainingCorpusRedactValue(child)
		}
		return out
	default:
		return current
	}
}

func trainingCorpusSensitiveKey(key string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	if key == "" {
		return false
	}
	for _, token := range []string{"authorization", "api_key", "apikey", "access_token", "refresh_token", "secret", "password", "passwd", "cookie", "session"} {
		if key == token || strings.Contains(key, token) {
			return true
		}
	}
	return false
}

func trainingCorpusManifestHasTruncatedArtifacts(manifest RequestCaptureSpoolManifest) bool {
	for _, artifact := range manifest.Artifacts {
		if artifact.Truncated {
			return true
		}
	}
	return false
}

func trainingCorpusSampleType(record model.RequestCaptureRecord, output map[string]any) string {
	if record.HasError {
		return "failed_negative"
	}
	if trainingCorpusOutputContainsToolCall(output) {
		return "tool_call"
	}
	if strings.Contains(record.RequestPath, "/responses") || strings.Contains(record.ProtocolChain, "responses") {
		return "responses_sft"
	}
	return "chat_sft"
}

func trainingCorpusOutputContainsToolCall(value any) bool {
	switch current := value.(type) {
	case map[string]any:
		for key, child := range current {
			key = strings.ToLower(key)
			if key == "tool_calls" || key == "function_call" || key == "function_call_output" {
				return true
			}
			if trainingCorpusOutputContainsToolCall(child) {
				return true
			}
		}
	case []any:
		for _, item := range current {
			if trainingCorpusOutputContainsToolCall(item) {
				return true
			}
		}
	}
	return false
}

func trainingCorpusQualityScore(record model.RequestCaptureRecord, output map[string]any, truncated bool) int {
	if record.HasError {
		return 30
	}
	text := strings.TrimSpace(common.Interface2String(output["text"]))
	if text == "" && strings.TrimSpace(common.Interface2String(output["reasoning"])) == "" {
		return 40
	}
	if truncated {
		return 60
	}
	return 80
}

func trainingCorpusTags(record model.RequestCaptureRecord, manifest RequestCaptureSpoolManifest, outputArtifact string) []string {
	tags := []string{}
	if record.IsStream || manifest.IsStream || strings.HasSuffix(outputArtifact, ".sse") {
		tags = append(tags, "stream")
	}
	if strings.Contains(record.ProtocolChain, "responses") {
		tags = append(tags, "responses")
	}
	if strings.Contains(record.ProtocolChain, "chat") {
		tags = append(tags, "chat")
	}
	if record.Group != "" {
		tags = append(tags, "group:"+record.Group)
	}
	return tags
}

func trainingCorpusSource(record model.RequestCaptureRecord, artifact model.RequestCaptureArtifact, inputArtifact string, outputArtifact string) map[string]any {
	return map[string]any{
		"request_id":      record.RequestId,
		"capture_id":      record.Id,
		"artifact_id":     artifact.Id,
		"artifact_sha256": artifact.SHA256,
		"storage_key":     artifact.StorageKey,
		"input_artifact":  inputArtifact,
		"output_artifact": outputArtifact,
	}
}

func trainingCorpusSourceHash(input any, output map[string]any) string {
	return trainingCorpusAnyHash(map[string]any{
		"input":  input,
		"output": output,
	})
}

func trainingCorpusAnyHash(value any) string {
	body, _ := json.Marshal(value)
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func encodeTrainingCorpusJSONL(samples []trainingCorpusBuiltSample) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	for _, sample := range samples {
		if err := encoder.Encode(sample.line); err != nil {
			return nil, err
		}
	}
	return buffer.Bytes(), nil
}

func compressTrainingCorpusJSONLZstd(body []byte) ([]byte, error) {
	var buffer bytes.Buffer
	writer, err := zstd.NewWriter(&buffer)
	if err != nil {
		return nil, err
	}
	if _, err := writer.Write(body); err != nil {
		_ = writer.Close()
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func trainingCorpusStorageKey(name string, version string, now int64) string {
	t := time.Unix(now, 0).UTC()
	return requestCaptureJoinS3Key(
		"training",
		"dataset="+requestCaptureSanitizeKeySegment(name),
		"version="+requestCaptureSanitizeKeySegment(version),
		"date="+t.Format("2006-01-02"),
		"part-0001.jsonl.zst",
	)
}

func trainingCorpusSourceScope(options TrainingCorpusBuildOptions, limit int, maxDecodedBytes int64) map[string]any {
	return map[string]any{
		"start_timestamp":          options.StartTimestamp,
		"end_timestamp":            options.EndTimestamp,
		"limit":                    limit,
		"max_decoded_bundle_bytes": maxDecodedBytes,
		"include_errored":          options.IncludeErrored,
	}
}

func trainingCorpusNow(options TrainingCorpusBuildOptions) int64 {
	if options.Now != nil {
		return options.Now()
	}
	return common.GetTimestamp()
}
