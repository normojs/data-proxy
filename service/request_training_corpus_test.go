package service

import (
	"archive/tar"
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/klauspost/compress/zstd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestBuildTrainingCorpusDatasetFromRawCapture(t *testing.T) {
	var mu sync.Mutex
	objects := map[string][]byte{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			mu.Lock()
			objects[r.URL.Path] = body
			mu.Unlock()
			w.WriteHeader(http.StatusOK)
		case http.MethodGet:
			mu.Lock()
			body, ok := objects[r.URL.Path]
			mu.Unlock()
			if !ok {
				http.NotFound(w, r)
				return
			}
			_, _ = w.Write(body)
		default:
			http.Error(w, "unsupported method", http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()

	previousDB := model.DB
	t.Cleanup(func() {
		model.DB = previousDB
	})
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	require.NoError(t, db.AutoMigrate(
		&model.RequestCaptureRecord{},
		&model.RequestCaptureArtifact{},
		&model.TrainingDatasetVersion{},
		&model.TrainingSample{},
	))

	key := bytes.Repeat([]byte{4}, 32)
	t.Setenv("CAPTURE_ENABLED", "true")
	t.Setenv("CAPTURE_OBJECT_BACKEND", "s3")
	t.Setenv("CAPTURE_S3_ENDPOINT", server.URL)
	t.Setenv("CAPTURE_S3_BUCKET", "data-proxy-captures")
	t.Setenv("CAPTURE_S3_REGION", "us-east-1")
	t.Setenv("CAPTURE_S3_ACCESS_KEY", "capture-access-key")
	t.Setenv("CAPTURE_S3_SECRET_KEY", "capture-secret-key")
	t.Setenv("CAPTURE_S3_KEY_PREFIX", "raw")
	t.Setenv("CAPTURE_BUNDLE_MASTER_KEY", "base64:"+base64.StdEncoding.EncodeToString(key))

	requestId := "req-training-corpus"
	record := model.RequestCaptureRecord{
		RequestId:      requestId,
		CaptureLevel:   model.RequestCaptureLevelFullBundle,
		CaptureStatus:  model.RequestCaptureStatusFinalizing,
		ModelName:      "qwen-plus",
		RequestPath:    "/v1/chat/completions",
		ProtocolChain:  "chat",
		IsStream:       false,
		StartedAt:      1000,
		FinishedAt:     1001,
		CreatedAt:      1000,
		UpdatedAt:      1001,
		MetadataJson:   `{"source":"test"}`,
		ConversionJson: `{"chain":["chat"]}`,
	}
	require.NoError(t, db.Create(&record).Error)

	spoolDir := t.TempDir()
	session, err := NewRequestCaptureSession(RequestCaptureSessionOptions{
		RequestId:     requestId,
		CaptureLevel:  model.RequestCaptureLevelFullBundle,
		SpoolDir:      spoolDir,
		ModelName:     "qwen-plus",
		RequestPath:   "/v1/chat/completions",
		ProtocolChain: "chat",
	})
	require.NoError(t, err)
	require.NoError(t, session.WriteArtifact("client_request.json", "application/json", []byte(`{"model":"qwen-plus","api_key":"secret-value","messages":[{"role":"user","content":"hello from alice@example.com, call 13800138000, key sk-testsecretvalue1234567890"}]}`)))
	require.NoError(t, session.WriteArtifact("downstream_response.json", "application/json", []byte(`{"choices":[{"message":{"role":"assistant","content":"你好"}}],"headers":{"authorization":"Bearer outputsecret1234567890"}}`)))
	require.NoError(t, session.Finish())
	_, err = FinalizeAndPersistRequestCaptureSpoolSession(context.Background(), RequestCaptureFinalizeOptions{SessionDir: session.Dir()})
	require.NoError(t, err)

	result, err := BuildTrainingCorpusDataset(context.Background(), TrainingCorpusBuildOptions{
		Name:    "domestic-chat",
		Version: "v1",
		Limit:   10,
		Now: func() int64 {
			return 1780000000
		},
	})
	require.NoError(t, err)
	require.Equal(t, model.TrainingDatasetStatusCompleted, result.Dataset.Status)
	require.EqualValues(t, 1, result.Dataset.SampleCount)
	require.Equal(t, "training/dataset=domestic-chat/version=v1/date=2026-05-28/part-0001.jsonl.zst", result.Dataset.StorageKey)
	require.Len(t, result.Samples, 1)
	require.Equal(t, requestId, result.Samples[0].RequestId)
	require.Equal(t, record.ModelName, result.Samples[0].ModelName)
	require.Equal(t, trainingCorpusRedactionVersion, result.Samples[0].RedactionStatus)
	require.Equal(t, model.TrainingSampleReviewStatusPending, result.Samples[0].ReviewStatus)

	compressed, err := LoadRequestCaptureObject(context.Background(), result.Dataset.StorageKey)
	require.NoError(t, err)
	lines := decodeTrainingCorpusJSONLForTest(t, compressed)
	require.Len(t, lines, 1)
	assert.Equal(t, trainingCorpusSchemaVersion, lines[0]["schema"])
	assert.Equal(t, "chat_sft", lines[0]["sample_type"])
	assert.Equal(t, requestId, lines[0]["request_id"])
	assert.Equal(t, "你好", lines[0]["output"].(map[string]any)["text"])
	body, _ := json.Marshal(lines[0])
	assert.NotContains(t, string(body), "secret-value")
	assert.NotContains(t, string(body), "alice@example.com")
	assert.NotContains(t, string(body), "13800138000")
	assert.NotContains(t, string(body), "sk-testsecretvalue1234567890")
	assert.NotContains(t, string(body), "outputsecret1234567890")
	assert.Contains(t, string(body), "[REDACTED]")
	assert.Contains(t, string(body), "[REDACTED_EMAIL]")
	assert.Contains(t, string(body), "[REDACTED_PHONE]")
	assert.Contains(t, string(body), "[REDACTED_API_KEY]")
}

func TestBuildTrainingCorpusDatasetExtractsResponsesSSEText(t *testing.T) {
	files := map[string][]byte{
		requestCaptureSpoolManifestName: []byte(`{"request_id":"req-sse","is_stream":true,"artifacts":[{"name":"downstream_response.sse","path":"artifacts/downstream_response.sse","content_type":"text/event-stream"}]}`),
		"artifacts/client_request.json": []byte(`{"model":"deepseek-ai/DeepSeek-V4-Flash","input":"查一下汇率"}`),
		"artifacts/downstream_response.sse": []byte(strings.Join([]string{
			`event: response.output_text.delta`,
			`data: {"type":"response.output_text.delta","delta":"当前"}`,
			``,
			`event: response.output_text.delta`,
			`data: {"type":"response.output_text.delta","delta":"汇率"}`,
			``,
			`data: [DONE]`,
			``,
		}, "\n")),
	}
	tarBody := tarFilesForTrainingCorpusTest(t, files)
	parsedFiles, manifest, err := readTrainingCorpusBundleFiles(tarBody)
	require.NoError(t, err)

	input, _ := trainingCorpusInput(parsedFiles)
	output, artifactName := trainingCorpusOutput(parsedFiles)
	require.NotNil(t, input)
	require.Equal(t, "artifacts/downstream_response.sse", artifactName)
	require.Equal(t, "当前汇率", output["text"])
	require.True(t, manifest.IsStream)
}

func TestTrainingCorpusRedactValueMasksStrings(t *testing.T) {
	value := map[string]any{
		"content": "email alice@example.com bearer Bearer abcdefghijklmnop phone +1 415-555-0100 key sk-testsecretvalue1234567890",
		"nested": []any{
			map[string]any{"note": "cn phone 13800138000"},
			"send to bob@example.org",
		},
	}

	body, err := json.Marshal(trainingCorpusRedactValue(value))
	require.NoError(t, err)
	text := string(body)

	assert.NotContains(t, text, "alice@example.com")
	assert.NotContains(t, text, "bob@example.org")
	assert.NotContains(t, text, "Bearer abcdefghijklmnop")
	assert.NotContains(t, text, "+1 415-555-0100")
	assert.NotContains(t, text, "13800138000")
	assert.NotContains(t, text, "sk-testsecretvalue1234567890")
	assert.Contains(t, text, "[REDACTED_EMAIL]")
	assert.Contains(t, text, "[REDACTED_BEARER]")
	assert.Contains(t, text, "[REDACTED_PHONE]")
	assert.Contains(t, text, "[REDACTED_API_KEY]")
}

func decodeTrainingCorpusJSONLForTest(t *testing.T, compressed []byte) []map[string]any {
	t.Helper()
	reader, err := zstd.NewReader(bytes.NewReader(compressed))
	require.NoError(t, err)
	defer reader.Close()
	body, err := io.ReadAll(reader)
	require.NoError(t, err)

	result := make([]map[string]any, 0)
	scanner := bufio.NewScanner(bytes.NewReader(body))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var item map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &item))
		result = append(result, item)
	}
	require.NoError(t, scanner.Err())
	return result
}

func tarFilesForTrainingCorpusTest(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := tar.NewWriter(&buffer)
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		body := files[name]
		require.NoError(t, writer.WriteHeader(&tar.Header{
			Name: name,
			Mode: 0600,
			Size: int64(len(body)),
		}))
		_, err := writer.Write(body)
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())
	return buffer.Bytes()
}
