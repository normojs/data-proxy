package service

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestCaptureSessionWritesArtifactsAndFinalizes(t *testing.T) {
	spoolDir := t.TempDir()
	session, err := NewRequestCaptureSession(RequestCaptureSessionOptions{
		RequestId:     "req-spool-123",
		CaptureLevel:  model.RequestCaptureLevelSanitizedBundle,
		SpoolDir:      spoolDir,
		ModelName:     "deepseek-ai/DeepSeek-V4-Flash",
		RequestPath:   "/v1/responses",
		ProtocolChain: "responses->chat",
		IsStream:      true,
		Now: func() time.Time {
			return time.Date(2026, 6, 23, 13, 0, 0, 0, time.UTC)
		},
	})
	require.NoError(t, err)

	activeDir := session.Dir()
	assert.Contains(t, activeDir, filepath.Join(spoolDir, "active"))
	require.NoError(t, session.WriteArtifact("client_request.json", "application/json", []byte(`{"model":"test"}`)))
	require.NoError(t, session.AppendArtifact("upstream_response.sse", "text/event-stream", []byte("data: hello\n\n")))
	require.NoError(t, session.AppendArtifact("upstream_response.sse", "text/event-stream", []byte("data: done\n\n")))

	require.NoError(t, session.Finish())
	finalizeDir := session.Dir()
	assert.Contains(t, finalizeDir, filepath.Join(spoolDir, "finalize"))
	assert.NoDirExists(t, activeDir)

	manifest := readRequestCaptureSpoolManifestForTest(t, finalizeDir)
	assert.Equal(t, "req-spool-123", manifest.RequestId)
	assert.Equal(t, requestCaptureSpoolStatusFinalize, manifest.Status)
	assert.Equal(t, model.RequestCaptureLevelSanitizedBundle, manifest.CaptureLevel)
	assert.Equal(t, "deepseek-ai/DeepSeek-V4-Flash", manifest.ModelName)
	assert.Equal(t, "/v1/responses", manifest.RequestPath)
	assert.Equal(t, "responses->chat", manifest.ProtocolChain)
	assert.True(t, manifest.IsStream)
	require.Len(t, manifest.Artifacts, 2)
	assert.Equal(t, "client_request.json", manifest.Artifacts[0].Name)
	assert.Equal(t, int64(len(`{"model":"test"}`)), manifest.Artifacts[0].Bytes)
	assert.Len(t, manifest.Artifacts[0].SHA256, 64)
	assert.Equal(t, "upstream_response.sse", manifest.Artifacts[1].Name)
	assert.Equal(t, int64(len("data: hello\n\ndata: done\n\n")), manifest.Artifacts[1].Bytes)
	assert.Len(t, manifest.Artifacts[1].SHA256, 64)

	upstreamBody, err := os.ReadFile(filepath.Join(finalizeDir, manifest.Artifacts[1].Path))
	require.NoError(t, err)
	assert.Equal(t, "data: hello\n\ndata: done\n\n", string(upstreamBody))

	err = session.AppendArtifact("upstream_response.sse", "text/event-stream", []byte("late"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "finalized")
}

func TestRequestCaptureSessionMovesFailedSession(t *testing.T) {
	spoolDir := t.TempDir()
	session, err := NewRequestCaptureSession(RequestCaptureSessionOptions{
		RequestId: "req-spool-failed",
		SpoolDir:  spoolDir,
	})
	require.NoError(t, err)

	activeDir := session.Dir()
	require.NoError(t, session.WriteArtifact("trace.json", "application/json", []byte(`{"step":"start"}`)))
	require.NoError(t, session.Fail(errors.New("upstream stream closed early")))

	failedDir := session.Dir()
	assert.Contains(t, failedDir, filepath.Join(spoolDir, "failed"))
	assert.NoDirExists(t, activeDir)

	manifest := readRequestCaptureSpoolManifestForTest(t, failedDir)
	assert.Equal(t, requestCaptureSpoolStatusFailed, manifest.Status)
	assert.Equal(t, "upstream stream closed early", manifest.Error)
	require.Len(t, manifest.Artifacts, 1)
	assert.Equal(t, "trace.json", manifest.Artifacts[0].Name)
	assert.Len(t, manifest.Artifacts[0].SHA256, 64)
}

func TestRequestCaptureSessionTruncatesWrittenArtifact(t *testing.T) {
	session, err := NewRequestCaptureSession(RequestCaptureSessionOptions{
		RequestId:        "req-spool-write-truncated",
		SpoolDir:         t.TempDir(),
		MaxArtifactBytes: 5,
	})
	require.NoError(t, err)

	require.NoError(t, session.WriteArtifact("client_request.json", "application/json", []byte("hello world")))

	manifest := readRequestCaptureSpoolManifestForTest(t, session.Dir())
	require.Len(t, manifest.Artifacts, 1)
	assert.Equal(t, int64(5), manifest.MaxArtifactBytes)
	assert.Equal(t, int64(5), manifest.Artifacts[0].Bytes)
	assert.True(t, manifest.Artifacts[0].Truncated)
	body, err := os.ReadFile(filepath.Join(session.Dir(), manifest.Artifacts[0].Path))
	require.NoError(t, err)
	assert.Equal(t, "hello", string(body))
}

func TestRequestCaptureSessionTruncatesAppendedArtifact(t *testing.T) {
	session, err := NewRequestCaptureSession(RequestCaptureSessionOptions{
		RequestId:        "req-spool-append-truncated",
		SpoolDir:         t.TempDir(),
		MaxArtifactBytes: 10,
	})
	require.NoError(t, err)

	require.NoError(t, session.AppendArtifact("upstream_response.sse", "text/event-stream", []byte("hello ")))
	require.NoError(t, session.AppendArtifact("upstream_response.sse", "text/event-stream", []byte("world")))
	require.NoError(t, session.AppendArtifact("upstream_response.sse", "text/event-stream", []byte("ignored")))

	manifest := readRequestCaptureSpoolManifestForTest(t, session.Dir())
	require.Len(t, manifest.Artifacts, 1)
	assert.Equal(t, int64(10), manifest.Artifacts[0].Bytes)
	assert.True(t, manifest.Artifacts[0].Truncated)
	body, err := os.ReadFile(filepath.Join(session.Dir(), manifest.Artifacts[0].Path))
	require.NoError(t, err)
	assert.Equal(t, "hello worl", string(body))

	require.NoError(t, session.Finish())
	finalManifest := readRequestCaptureSpoolManifestForTest(t, session.Dir())
	require.Len(t, finalManifest.Artifacts, 1)
	assert.Equal(t, int64(10), finalManifest.Artifacts[0].Bytes)
	assert.True(t, finalManifest.Artifacts[0].Truncated)
	assert.Len(t, finalManifest.Artifacts[0].SHA256, 64)
}

func readRequestCaptureSpoolManifestForTest(t *testing.T, dir string) RequestCaptureSpoolManifest {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(dir, requestCaptureSpoolManifestName))
	require.NoError(t, err)
	var manifest RequestCaptureSpoolManifest
	require.NoError(t, json.Unmarshal(body, &manifest))
	return manifest
}
