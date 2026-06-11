package openapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLocalBinaryObjectCleanupDeletesExpiredObjects(t *testing.T) {
	t.Setenv("OPENAPI_BINARY_OBJECT_PROVIDER", "local")
	t.Setenv("OPENAPI_BINARY_OBJECT_DIR", t.TempDir())

	oldObject, err := SaveBinaryObject([]byte("old"), "application/octet-stream", "old-sha")
	require.NoError(t, err)
	freshObject, err := SaveBinaryObject([]byte("fresh"), "application/octet-stream", "fresh-sha")
	require.NoError(t, err)
	rewriteLocalBinaryObjectCreatedAt(t, oldObject.Id, time.Now().Add(-2*time.Hour).Unix())

	cutoff := time.Now().Add(-time.Hour).Unix()
	preview, err := CleanupBinaryObjects(context.Background(), BinaryObjectCleanupOptions{
		CutoffUnix: cutoff,
		Limit:      10,
		DryRun:     true,
	})
	require.NoError(t, err)
	require.Equal(t, "local", preview.Provider)
	require.Equal(t, 2, preview.Scanned)
	require.Equal(t, 1, preview.Deleted)
	require.Equal(t, int64(oldObject.Size), preview.DeletedBytes)
	require.Equal(t, []string{oldObject.Id}, preview.DeletedObjectIds)

	_, oldContent, err := LoadBinaryObject(oldObject.Id)
	require.NoError(t, err)
	require.Equal(t, []byte("old"), oldContent)

	cleaned, err := CleanupBinaryObjects(context.Background(), BinaryObjectCleanupOptions{
		CutoffUnix: cutoff,
		Limit:      10,
	})
	require.NoError(t, err)
	require.Equal(t, 1, cleaned.Deleted)
	require.Equal(t, []string{oldObject.Id}, cleaned.DeletedObjectIds)
	_, _, err = LoadBinaryObject(oldObject.Id)
	require.Error(t, err)
	_, freshContent, err := LoadBinaryObject(freshObject.Id)
	require.NoError(t, err)
	require.Equal(t, []byte("fresh"), freshContent)
}

func TestS3BinaryObjectStoreSaveLoadCleanup(t *testing.T) {
	server, objects := newFakeS3Server(t)
	t.Setenv("OPENAPI_BINARY_OBJECT_PROVIDER", "s3")
	t.Setenv("OPENAPI_BINARY_OBJECT_S3_ENDPOINT", server.URL)
	t.Setenv("OPENAPI_BINARY_OBJECT_S3_BUCKET", "bucket")
	t.Setenv("OPENAPI_BINARY_OBJECT_S3_REGION", "us-east-1")
	t.Setenv("OPENAPI_BINARY_OBJECT_S3_ACCESS_KEY", "access-key")
	t.Setenv("OPENAPI_BINARY_OBJECT_S3_SECRET_KEY", "secret-key")
	t.Setenv("OPENAPI_BINARY_OBJECT_S3_KEY_PREFIX", "test-prefix")

	object, err := SaveBinaryObject([]byte("hello"), "application/octet-stream", "sha")
	require.NoError(t, err)
	require.Equal(t, "s3", object.Provider)
	require.Equal(t, "test-prefix/"+object.Id[:2]+"/"+object.Id, object.StorageKey)

	loaded, content, err := LoadBinaryObject(object.Id)
	require.NoError(t, err)
	require.Equal(t, object.Id, loaded.Id)
	require.Equal(t, []byte("hello"), content)

	preview, err := CleanupBinaryObjects(context.Background(), BinaryObjectCleanupOptions{
		CutoffUnix: time.Now().Add(time.Hour).Unix(),
		Limit:      10,
		DryRun:     true,
	})
	require.NoError(t, err)
	require.Equal(t, "s3", preview.Provider)
	require.Equal(t, 1, preview.Scanned)
	require.Equal(t, 1, preview.Deleted)
	require.Equal(t, []string{object.Id}, preview.DeletedObjectIds)
	require.True(t, objects.exists(object.StorageKey+"/body.bin"))

	cleaned, err := CleanupBinaryObjects(context.Background(), BinaryObjectCleanupOptions{
		CutoffUnix: time.Now().Add(time.Hour).Unix(),
		Limit:      10,
	})
	require.NoError(t, err)
	require.Equal(t, 1, cleaned.Deleted)
	require.Equal(t, []string{object.Id}, cleaned.DeletedObjectIds)
	require.False(t, objects.exists(object.StorageKey+"/body.bin"))
	require.False(t, objects.exists(object.StorageKey+"/metadata.json"))
}

func rewriteLocalBinaryObjectCreatedAt(t *testing.T, id string, createdAt int64) {
	t.Helper()
	object, _, err := LoadBinaryObject(id)
	require.NoError(t, err)
	object.CreatedAt = createdAt
	metaBytes, err := json.Marshal(object)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(localBinaryObjectMetaPath(id), metaBytes, 0600))
}

type fakeS3Objects struct {
	mu      sync.Mutex
	objects map[string][]byte
}

func newFakeS3Server(t *testing.T) (*httptest.Server, *fakeS3Objects) {
	t.Helper()
	state := &fakeS3Objects{objects: map[string][]byte{}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NotEmpty(t, r.Header.Get("Authorization"))
		bucket, key, ok := strings.Cut(strings.TrimPrefix(r.URL.Path, "/"), "/")
		if !ok {
			bucket = strings.TrimPrefix(r.URL.Path, "/")
		}
		require.Equal(t, "bucket", bucket)
		if r.Method == http.MethodGet && r.URL.Query().Get("list-type") == "2" {
			state.writeList(w, r.URL.Query().Get("prefix"))
			return
		}
		switch r.Method {
		case http.MethodPut:
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			state.set(key, body)
			w.WriteHeader(http.StatusOK)
		case http.MethodGet:
			body, ok := state.get(key)
			if !ok {
				http.NotFound(w, r)
				return
			}
			_, _ = w.Write(body)
		case http.MethodDelete:
			state.delete(key)
			w.WriteHeader(http.StatusNoContent)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}))
	t.Cleanup(server.Close)
	return server, state
}

func (f *fakeS3Objects) set(key string, content []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.objects[key] = append([]byte(nil), content...)
}

func (f *fakeS3Objects) get(key string) ([]byte, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	content, ok := f.objects[key]
	return append([]byte(nil), content...), ok
}

func (f *fakeS3Objects) delete(key string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.objects, key)
}

func (f *fakeS3Objects) exists(key string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.objects[key]
	return ok
}

func (f *fakeS3Objects) writeList(w http.ResponseWriter, prefix string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	keys := make([]string, 0, len(f.objects))
	for key := range f.objects {
		if strings.HasPrefix(key, prefix) {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	w.Header().Set("Content-Type", "application/xml")
	_, _ = fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?><ListBucketResult>`)
	for _, key := range keys {
		_, _ = fmt.Fprintf(w, "<Contents><Key>%s</Key><Size>%d</Size></Contents>", key, len(f.objects[key]))
	}
	_, _ = fmt.Fprint(w, `<IsTruncated>false</IsTruncated></ListBucketResult>`)
}
