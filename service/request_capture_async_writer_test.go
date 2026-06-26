package service

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestCaptureAsyncArtifactWriterAppendAfterCloseIsSafe(t *testing.T) {
	session, err := NewRequestCaptureSession(RequestCaptureSessionOptions{
		RequestId: "req-async-writer-close",
		SpoolDir:  t.TempDir(),
	})
	require.NoError(t, err)

	writer := newRequestCaptureAsyncArtifactWriter(session, "downstream_response.sse", "text/event-stream")
	writer.Append([]byte("hello"))
	require.NoError(t, writer.Close())

	assert.NotPanics(t, func() {
		writer.Append([]byte("ignored"))
	})

	manifest := readRequestCaptureSpoolManifestForTest(t, session.Dir())
	require.Len(t, manifest.Artifacts, 1)
	body, err := os.ReadFile(filepath.Join(session.Dir(), manifest.Artifacts[0].Path))
	require.NoError(t, err)
	assert.Equal(t, "hello", string(body))
	assert.False(t, manifest.Artifacts[0].Truncated)
}

func TestRequestCaptureAsyncArtifactWriterTruncatesOversizedPendingChunk(t *testing.T) {
	session, err := NewRequestCaptureSession(RequestCaptureSessionOptions{
		RequestId: "req-async-writer-pending-cap",
		SpoolDir:  t.TempDir(),
	})
	require.NoError(t, err)

	writer := newRequestCaptureAsyncArtifactWriter(session, "downstream_response.sse", "text/event-stream")
	writer.Append(bytes.Repeat([]byte("a"), requestCaptureAsyncWriterMaxPendingBytes+1))
	require.NoError(t, writer.Close())

	manifest := readRequestCaptureSpoolManifestForTest(t, session.Dir())
	require.Len(t, manifest.Artifacts, 1)
	assert.Equal(t, "downstream_response.sse", manifest.Artifacts[0].Name)
	assert.True(t, manifest.Artifacts[0].Truncated)
	assert.Equal(t, int64(0), manifest.Artifacts[0].Bytes)
}

func TestRequestCaptureAsyncArtifactWriterCloseAndAppendDoNotDeadlock(t *testing.T) {
	session, err := NewRequestCaptureSession(RequestCaptureSessionOptions{
		RequestId: "req-async-writer-close-race",
		SpoolDir:  t.TempDir(),
	})
	require.NoError(t, err)

	writer := newRequestCaptureAsyncArtifactWriter(session, "downstream_response.sse", "text/event-stream")
	appendDone := make(chan struct{})
	go func() {
		defer close(appendDone)
		writer.Append(bytes.Repeat([]byte("z"), 1024))
	}()

	require.NoError(t, writer.Close())

	select {
	case <-appendDone:
	case <-time.After(2 * time.Second):
		t.Fatal("append goroutine did not finish after close")
	}

	manifest := readRequestCaptureSpoolManifestForTest(t, session.Dir())
	if len(manifest.Artifacts) == 0 {
		return
	}
	require.Len(t, manifest.Artifacts, 1)
	body, err := os.ReadFile(filepath.Join(session.Dir(), manifest.Artifacts[0].Path))
	require.NoError(t, err)
	assert.NotEmpty(t, body)
}
