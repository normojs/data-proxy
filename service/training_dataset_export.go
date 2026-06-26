package service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/QuantumNous/new-api/model"
	"github.com/klauspost/compress/zstd"
)

var ErrTrainingDatasetNoApprovedSamples = errors.New("training dataset has no approved samples")

type TrainingDatasetApprovedExportResult struct {
	Body            []byte `json:"-"`
	SHA256          string `json:"sha256"`
	SourceSamples   int64  `json:"source_samples"`
	ApprovedSamples int64  `json:"approved_samples"`
	ExportedSamples int64  `json:"exported_samples"`
}

func BuildApprovedTrainingDatasetExport(ctx context.Context, dataset model.TrainingDatasetVersion) (TrainingDatasetApprovedExportResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if model.DB == nil {
		return TrainingDatasetApprovedExportResult{}, errors.New("database is not initialized")
	}
	if dataset.Id <= 0 {
		return TrainingDatasetApprovedExportResult{}, errors.New("training dataset id is required")
	}
	storageKey := strings.TrimSpace(dataset.StorageKey)
	if storageKey == "" {
		return TrainingDatasetApprovedExportResult{}, errors.New("training dataset storage key is empty")
	}
	var samples []model.TrainingSample
	if err := model.DB.WithContext(ctx).
		Where("dataset_version_id = ? AND review_status = ?", dataset.Id, model.TrainingSampleReviewStatusApproved).
		Find(&samples).Error; err != nil {
		return TrainingDatasetApprovedExportResult{}, err
	}
	if len(samples) == 0 {
		return TrainingDatasetApprovedExportResult{SourceSamples: dataset.SampleCount}, ErrTrainingDatasetNoApprovedSamples
	}
	approved := map[string]bool{}
	for _, sample := range samples {
		sourceHash := strings.TrimSpace(sample.SourceHash)
		if sourceHash != "" {
			approved[sourceHash] = true
		}
	}
	if len(approved) == 0 {
		return TrainingDatasetApprovedExportResult{SourceSamples: dataset.SampleCount}, ErrTrainingDatasetNoApprovedSamples
	}
	body, err := LoadRequestCaptureObject(ctx, storageKey)
	if err != nil {
		return TrainingDatasetApprovedExportResult{}, err
	}
	filtered, exported, err := filterTrainingDatasetJSONLZstdBySourceHash(body, approved)
	if err != nil {
		return TrainingDatasetApprovedExportResult{}, err
	}
	if exported == 0 {
		return TrainingDatasetApprovedExportResult{SourceSamples: dataset.SampleCount, ApprovedSamples: int64(len(samples))}, ErrTrainingDatasetNoApprovedSamples
	}
	return TrainingDatasetApprovedExportResult{
		Body:            filtered,
		SHA256:          requestCaptureObjectSHA256(filtered),
		SourceSamples:   dataset.SampleCount,
		ApprovedSamples: int64(len(samples)),
		ExportedSamples: exported,
	}, nil
}

func filterTrainingDatasetJSONLZstdBySourceHash(body []byte, approved map[string]bool) ([]byte, int64, error) {
	reader, err := zstd.NewReader(bytes.NewReader(body))
	if err != nil {
		return nil, 0, err
	}
	defer reader.Close()
	var out bytes.Buffer
	lineReader := bufio.NewReader(reader)
	var exported int64
	for {
		line, err := lineReader.ReadBytes('\n')
		if len(line) > 0 {
			trimmed := bytes.TrimSpace(line)
			if len(trimmed) > 0 {
				include, err := trainingDatasetExportLineApproved(trimmed, approved)
				if err != nil {
					return nil, exported, err
				}
				if include {
					out.Write(trimmed)
					out.WriteByte('\n')
					exported++
				}
			}
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, exported, err
		}
	}
	compressed, err := compressTrainingCorpusJSONLZstd(out.Bytes())
	if err != nil {
		return nil, exported, err
	}
	return compressed, exported, nil
}

func trainingDatasetExportLineApproved(line []byte, approved map[string]bool) (bool, error) {
	var payload map[string]interface{}
	if err := json.Unmarshal(line, &payload); err != nil {
		return false, fmt.Errorf("decode training dataset jsonl line: %w", err)
	}
	sourceHash := strings.TrimSpace(fmt.Sprint(payload["source_hash"]))
	if sourceHash == "" {
		return false, nil
	}
	return approved[sourceHash], nil
}
