package openapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	openAPIBinaryObjectProviderEnv        = "OPENAPI_BINARY_OBJECT_PROVIDER"
	openAPIBinaryObjectDirEnv             = "OPENAPI_BINARY_OBJECT_DIR"
	openAPIBinaryObjectTTLEnv             = "OPENAPI_BINARY_OBJECT_TTL_SECONDS"
	openAPIBinaryObjectCleanupIntervalEnv = "OPENAPI_BINARY_OBJECT_CLEANUP_INTERVAL_SECONDS"
	openAPIBinaryObjectCleanupLimitEnv    = "OPENAPI_BINARY_OBJECT_CLEANUP_LIMIT"
	openAPIBinaryStoreDirName             = "data-proxy-openapi-binaries"
	defaultBinaryObjectCleanupInterval    = int64(60 * 60)
	defaultBinaryObjectCleanupLimit       = 500
)

var errBinaryObjectCleanupLimitReached = errors.New("openapi binary object cleanup limit reached")

type BinaryObject struct {
	Id          string `json:"id"`
	Provider    string `json:"provider,omitempty"`
	StorageKey  string `json:"storage_key,omitempty"`
	ContentType string `json:"content_type"`
	SHA256      string `json:"sha256"`
	Size        int    `json:"size"`
	CreatedAt   int64  `json:"created_at"`
	Filename    string `json:"filename"`
}

type BinaryObjectCleanupOptions struct {
	CutoffUnix int64 `json:"cutoff_unix"`
	Limit      int   `json:"limit"`
	DryRun     bool  `json:"dry_run"`
}

type BinaryObjectCleanupResult struct {
	Provider         string   `json:"provider"`
	CutoffUnix       int64    `json:"cutoff_unix"`
	DryRun           bool     `json:"dry_run"`
	Scanned          int      `json:"scanned"`
	Deleted          int      `json:"deleted"`
	DeletedBytes     int64    `json:"deleted_bytes"`
	DeletedObjectIds []string `json:"deleted_object_ids,omitempty"`
	Errors           []string `json:"errors,omitempty"`
}

type BinaryObjectStore interface {
	Provider() string
	Save(ctx context.Context, object BinaryObject, content []byte) (BinaryObject, error)
	Load(ctx context.Context, id string) (BinaryObject, []byte, error)
	Cleanup(ctx context.Context, options BinaryObjectCleanupOptions) (BinaryObjectCleanupResult, error)
}

func SaveBinaryObject(content []byte, contentType string, sha256Hex string) (BinaryObject, error) {
	return SaveBinaryObjectWithContext(context.Background(), content, contentType, sha256Hex)
}

func SaveBinaryObjectWithContext(ctx context.Context, content []byte, contentType string, sha256Hex string) (BinaryObject, error) {
	if len(content) == 0 {
		return BinaryObject{}, errors.New("openapi binary object content is empty")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	object := BinaryObject{
		Id:          uuid.NewString(),
		ContentType: strings.TrimSpace(contentType),
		SHA256:      strings.TrimSpace(sha256Hex),
		Size:        len(content),
		CreatedAt:   time.Now().Unix(),
	}
	if object.ContentType == "" {
		object.ContentType = "application/octet-stream"
	}
	object.Filename = binaryObjectFilename(object)
	return binaryObjectStoreFromEnv().Save(ctx, object, content)
}

func LoadBinaryObject(id string) (BinaryObject, []byte, error) {
	return LoadBinaryObjectWithContext(context.Background(), id)
}

func LoadBinaryObjectWithContext(ctx context.Context, id string) (BinaryObject, []byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	store := binaryObjectStoreFromEnv()
	object, content, err := store.Load(ctx, id)
	if err == nil || store.Provider() == "local" {
		return object, content, err
	}
	localObject, localContent, localErr := localBinaryObjectStore{}.Load(ctx, id)
	if localErr == nil {
		return localObject, localContent, nil
	}
	return object, content, err
}

func CleanupBinaryObjects(ctx context.Context, options BinaryObjectCleanupOptions) (BinaryObjectCleanupResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if options.Limit < 0 {
		options.Limit = 0
	}
	store := binaryObjectStoreFromEnv()
	return store.Cleanup(ctx, options)
}

func BinaryObjectDownloadURL(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return ""
	}
	return "/api/mcp/openapi/binary/" + id + "/download"
}

func BinaryObjectTTLSeconds() int64 {
	return envInt64(openAPIBinaryObjectTTLEnv, 0)
}

func BinaryObjectCleanupIntervalSeconds() int64 {
	return envInt64(openAPIBinaryObjectCleanupIntervalEnv, defaultBinaryObjectCleanupInterval)
}

func BinaryObjectCleanupLimit() int {
	value := envInt64(openAPIBinaryObjectCleanupLimitEnv, int64(defaultBinaryObjectCleanupLimit))
	if value <= 0 {
		return defaultBinaryObjectCleanupLimit
	}
	if value > 10000 {
		return 10000
	}
	return int(value)
}

func binaryObjectStoreFromEnv() BinaryObjectStore {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(openAPIBinaryObjectProviderEnv))) {
	case "", "local", "disk", "filesystem", "fs":
		return localBinaryObjectStore{}
	case "s3", "s3-compatible", "s3_compatible":
		return newS3BinaryObjectStoreFromEnv()
	default:
		return localBinaryObjectStore{}
	}
}

type localBinaryObjectStore struct{}

func (localBinaryObjectStore) Provider() string {
	return "local"
}

func (localBinaryObjectStore) Save(ctx context.Context, object BinaryObject, content []byte) (BinaryObject, error) {
	if err := ctx.Err(); err != nil {
		return BinaryObject{}, err
	}
	object.Provider = "local"
	object.StorageKey = object.Id
	dir, err := localBinaryObjectDir(object.Id)
	if err != nil {
		return BinaryObject{}, err
	}
	if err := os.MkdirAll(dir, 0750); err != nil {
		return BinaryObject{}, fmt.Errorf("failed to create openapi binary object directory: %w", err)
	}
	dataPath := localBinaryObjectDataPath(object.Id)
	metaPath := localBinaryObjectMetaPath(object.Id)
	if err := os.WriteFile(dataPath, content, 0600); err != nil {
		return BinaryObject{}, fmt.Errorf("failed to write openapi binary object: %w", err)
	}
	metaBytes, err := json.Marshal(object)
	if err != nil {
		_ = os.Remove(dataPath)
		return BinaryObject{}, err
	}
	if err := os.WriteFile(metaPath, metaBytes, 0600); err != nil {
		_ = os.Remove(dataPath)
		return BinaryObject{}, fmt.Errorf("failed to write openapi binary object metadata: %w", err)
	}
	return object, nil
}

func (localBinaryObjectStore) Load(ctx context.Context, id string) (BinaryObject, []byte, error) {
	if err := ctx.Err(); err != nil {
		return BinaryObject{}, nil, err
	}
	id = strings.TrimSpace(id)
	if !isValidBinaryObjectId(id) {
		return BinaryObject{}, nil, errors.New("invalid openapi binary object id")
	}
	metaBytes, err := os.ReadFile(localBinaryObjectMetaPath(id))
	if err != nil {
		return BinaryObject{}, nil, err
	}
	var object BinaryObject
	if err := json.Unmarshal(metaBytes, &object); err != nil {
		return BinaryObject{}, nil, err
	}
	if object.Id != id {
		return BinaryObject{}, nil, errors.New("openapi binary object metadata mismatch")
	}
	content, err := os.ReadFile(localBinaryObjectDataPath(id))
	if err != nil {
		return BinaryObject{}, nil, err
	}
	return object, content, nil
}

func (localBinaryObjectStore) Cleanup(ctx context.Context, options BinaryObjectCleanupOptions) (BinaryObjectCleanupResult, error) {
	result := BinaryObjectCleanupResult{
		Provider:   "local",
		CutoffUnix: options.CutoffUnix,
		DryRun:     options.DryRun,
		Errors:     []string{},
	}
	if options.CutoffUnix <= 0 {
		return result, errors.New("cutoff_unix is required")
	}
	base := localBinaryObjectBaseDir()
	if _, err := os.Stat(base); err != nil {
		if os.IsNotExist(err) {
			return result, nil
		}
		return result, err
	}
	err := filepath.WalkDir(base, func(path string, entry os.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if walkErr != nil {
			result.Errors = append(result.Errors, walkErr.Error())
			return nil
		}
		if entry == nil || entry.IsDir() || entry.Name() != "metadata.json" {
			return nil
		}
		if options.Limit > 0 && result.Deleted >= options.Limit {
			return errBinaryObjectCleanupLimitReached
		}
		metaBytes, err := os.ReadFile(path)
		if err != nil {
			result.Errors = append(result.Errors, err.Error())
			return nil
		}
		var object BinaryObject
		if err := json.Unmarshal(metaBytes, &object); err != nil {
			result.Errors = append(result.Errors, err.Error())
			return nil
		}
		result.Scanned++
		if object.CreatedAt <= 0 || object.CreatedAt >= options.CutoffUnix {
			return nil
		}
		if options.DryRun {
			result.Deleted++
			result.DeletedBytes += int64(object.Size)
			result.DeletedObjectIds = append(result.DeletedObjectIds, object.Id)
			return nil
		}
		if err := os.RemoveAll(filepath.Dir(path)); err != nil {
			result.Errors = append(result.Errors, err.Error())
			return nil
		}
		result.Deleted++
		result.DeletedBytes += int64(object.Size)
		result.DeletedObjectIds = append(result.DeletedObjectIds, object.Id)
		return nil
	})
	if errors.Is(err, errBinaryObjectCleanupLimitReached) {
		err = nil
	}
	if len(result.Errors) == 0 {
		result.Errors = nil
	}
	return result, err
}

func binaryObjectFilename(object BinaryObject) string {
	extension := ".bin"
	if extensions, err := mime.ExtensionsByType(object.ContentType); err == nil && len(extensions) > 0 {
		extension = extensions[0]
	}
	prefix := object.SHA256
	if len(prefix) > 12 {
		prefix = prefix[:12]
	}
	if prefix == "" && len(object.Id) >= 12 {
		prefix = object.Id[:12]
	}
	if prefix == "" {
		prefix = "object"
	}
	return "openapi-response-" + prefix + extension
}

func localBinaryObjectBaseDir() string {
	base := strings.TrimSpace(os.Getenv(openAPIBinaryObjectDirEnv))
	if base == "" {
		base = filepath.Join(os.TempDir(), openAPIBinaryStoreDirName)
	}
	return base
}

func localBinaryObjectDir(id string) (string, error) {
	if !isValidBinaryObjectId(id) {
		return "", errors.New("invalid openapi binary object id")
	}
	return filepath.Join(localBinaryObjectBaseDir(), id[:2], id), nil
}

func localBinaryObjectDataPath(id string) string {
	dir, _ := localBinaryObjectDir(id)
	return filepath.Join(dir, "body.bin")
}

func localBinaryObjectMetaPath(id string) string {
	dir, _ := localBinaryObjectDir(id)
	return filepath.Join(dir, "metadata.json")
}

func isValidBinaryObjectId(id string) bool {
	if len(id) != 36 {
		return false
	}
	_, err := uuid.Parse(id)
	return err == nil
}

func envInt64(name string, fallback int64) int64 {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fallback
	}
	return parsed
}
