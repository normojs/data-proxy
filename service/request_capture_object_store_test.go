package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadRequestCaptureStorageConfigFromEnv(t *testing.T) {
	t.Setenv("CAPTURE_ENABLED", "true")
	t.Setenv("CAPTURE_OBJECT_BACKEND", "s3")
	t.Setenv("CAPTURE_SPOOL_DIR", "/capture/spool")
	t.Setenv("CAPTURE_TMP_DIR", "/capture/tmp")

	config := LoadRequestCaptureStorageConfigFromEnv()

	assert.True(t, config.Enabled)
	assert.Equal(t, "s3", config.Backend)
	assert.Equal(t, "/capture/spool", config.SpoolDir)
	assert.Equal(t, "/capture/tmp", config.TmpDir)
}

func TestRequestCaptureObjectStorageDisabledByDefault(t *testing.T) {
	config := LoadRequestCaptureStorageConfigFromEnv()

	assert.False(t, config.Enabled)
	assert.Empty(t, config.Backend)
	assert.Equal(t, requestCaptureDefaultSpoolDir, config.SpoolDir)
	assert.Equal(t, requestCaptureDefaultTmpDir, config.TmpDir)
}

func TestRequestCaptureS3ObjectStoreSaveLoadDelete(t *testing.T) {
	var mu sync.Mutex
	objects := map[string][]byte{}
	var putPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			http.Error(w, "missing authorization", http.StatusForbidden)
			return
		}
		if r.Header.Get("X-Amz-Content-Sha256") == "" {
			http.Error(w, "missing payload hash", http.StatusForbidden)
			return
		}
		switch r.Method {
		case http.MethodPut:
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			mu.Lock()
			objects[r.URL.Path] = body
			putPath = r.URL.Path
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
		case http.MethodDelete:
			mu.Lock()
			delete(objects, r.URL.Path)
			mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
		default:
			http.Error(w, "unsupported method", http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()

	t.Setenv("CAPTURE_ENABLED", "true")
	t.Setenv("CAPTURE_OBJECT_BACKEND", "s3")
	t.Setenv("CAPTURE_S3_ENDPOINT", server.URL)
	t.Setenv("CAPTURE_S3_BUCKET", "data-proxy-captures")
	t.Setenv("CAPTURE_S3_REGION", "us-east-1")
	t.Setenv("CAPTURE_S3_ACCESS_KEY", "capture-access-key")
	t.Setenv("CAPTURE_S3_SECRET_KEY", "capture-secret-key")
	t.Setenv("CAPTURE_S3_KEY_PREFIX", "raw")

	body := []byte("encrypted-bundle")
	createdAt := time.Date(2026, 6, 23, 13, 0, 0, 0, time.UTC).Unix()
	object, err := SaveRequestCaptureObject(context.Background(), RequestCaptureObject{
		RequestId:   "req-123",
		ContentType: "application/x-data-proxy-capture-bundle",
		CreatedAt:   createdAt,
	}, body)
	require.NoError(t, err)

	assert.Equal(t, "s3", object.Provider)
	assert.Equal(t, "data-proxy-captures", object.Bucket)
	assert.Equal(t, "raw/2026/06/23/13/re/q-/req-123.bundle.tar.zst.enc", object.StorageKey)
	assert.Equal(t, int64(len(body)), object.BodyBytes)
	assert.Len(t, object.SHA256, 64)
	assert.Equal(t, "/data-proxy-captures/"+object.StorageKey, putPath)

	loaded, err := LoadRequestCaptureObject(context.Background(), object.StorageKey)
	require.NoError(t, err)
	assert.Equal(t, body, loaded)

	require.NoError(t, DeleteRequestCaptureObject(context.Background(), object.StorageKey))
	_, err = LoadRequestCaptureObject(context.Background(), object.StorageKey)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status 404")
}

func TestRequestCaptureS3ObjectStoreReportsMissingConfig(t *testing.T) {
	t.Setenv("CAPTURE_ENABLED", "true")
	t.Setenv("CAPTURE_OBJECT_BACKEND", "s3")
	t.Setenv("CAPTURE_S3_ACCESS_KEY", "capture-access-key")
	t.Setenv("CAPTURE_S3_SECRET_KEY", "capture-secret-key")

	_, err := SaveRequestCaptureObject(context.Background(), RequestCaptureObject{RequestId: "req-missing"}, []byte("body"))
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "CAPTURE_S3_ENDPOINT"))
}
