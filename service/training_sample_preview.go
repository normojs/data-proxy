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

var ErrTrainingSamplePreviewNotFound = errors.New("training sample preview not found")

type TrainingSamplePreviewResult struct {
	Sample  model.TrainingSample         `json:"sample"`
	Dataset model.TrainingDatasetVersion `json:"dataset"`
	Line    map[string]interface{}       `json:"line"`
}

func LoadTrainingSamplePreview(ctx context.Context, sampleId int64) (TrainingSamplePreviewResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if model.DB == nil {
		return TrainingSamplePreviewResult{}, errors.New("database is not initialized")
	}
	if sampleId <= 0 {
		return TrainingSamplePreviewResult{}, errors.New("training sample id is required")
	}
	var sample model.TrainingSample
	if err := model.DB.WithContext(ctx).First(&sample, sampleId).Error; err != nil {
		return TrainingSamplePreviewResult{}, err
	}
	var dataset model.TrainingDatasetVersion
	if err := model.DB.WithContext(ctx).First(&dataset, sample.DatasetVersionId).Error; err != nil {
		return TrainingSamplePreviewResult{}, err
	}
	if dataset.Status != model.TrainingDatasetStatusCompleted || strings.TrimSpace(dataset.StorageKey) == "" {
		return TrainingSamplePreviewResult{}, errors.New("training dataset export is not available")
	}
	sourceHash := strings.TrimSpace(sample.SourceHash)
	if sourceHash == "" {
		return TrainingSamplePreviewResult{}, ErrTrainingSamplePreviewNotFound
	}
	body, err := LoadRequestCaptureObject(ctx, dataset.StorageKey)
	if err != nil {
		return TrainingSamplePreviewResult{}, err
	}
	line, err := findTrainingDatasetJSONLLineBySourceHash(body, sourceHash)
	if err != nil {
		return TrainingSamplePreviewResult{}, err
	}
	return TrainingSamplePreviewResult{
		Sample:  sample,
		Dataset: dataset,
		Line:    line,
	}, nil
}

func findTrainingDatasetJSONLLineBySourceHash(body []byte, sourceHash string) (map[string]interface{}, error) {
	reader, err := zstd.NewReader(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	lineReader := bufio.NewReader(reader)
	for {
		line, err := lineReader.ReadBytes('\n')
		if len(line) > 0 {
			trimmed := bytes.TrimSpace(line)
			if len(trimmed) > 0 {
				var payload map[string]interface{}
				if decodeErr := json.Unmarshal(trimmed, &payload); decodeErr != nil {
					return nil, fmt.Errorf("decode training dataset jsonl line: %w", decodeErr)
				}
				if strings.TrimSpace(fmt.Sprint(payload["source_hash"])) == sourceHash {
					return payload, nil
				}
			}
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
	}
	return nil, ErrTrainingSamplePreviewNotFound
}
