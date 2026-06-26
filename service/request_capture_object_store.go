package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/google/uuid"
)

const (
	requestCaptureEnabledEnv        = "CAPTURE_ENABLED"
	requestCaptureObjectBackendEnv  = "CAPTURE_OBJECT_BACKEND"
	requestCaptureObjectProviderEnv = "CAPTURE_OBJECT_PROVIDER"
	requestCaptureSpoolDirEnv       = "CAPTURE_SPOOL_DIR"
	requestCaptureTmpDirEnv         = "CAPTURE_TMP_DIR"

	requestCaptureS3EndpointEnv     = "CAPTURE_S3_ENDPOINT"
	requestCaptureS3BucketEnv       = "CAPTURE_S3_BUCKET"
	requestCaptureS3RegionEnv       = "CAPTURE_S3_REGION"
	requestCaptureS3AccessKeyEnv    = "CAPTURE_S3_ACCESS_KEY"
	requestCaptureS3SecretKeyEnv    = "CAPTURE_S3_SECRET_KEY"
	requestCaptureS3SessionTokenEnv = "CAPTURE_S3_SESSION_TOKEN"
	requestCaptureS3KeyPrefixEnv    = "CAPTURE_S3_KEY_PREFIX"
	requestCaptureS3PathStyleEnv    = "CAPTURE_S3_PATH_STYLE"

	requestCaptureDefaultSpoolDir   = "/data/dataproxy-capture/spool"
	requestCaptureDefaultTmpDir     = "/data/dataproxy-capture/tmp"
	requestCaptureDefaultS3Region   = "us-east-1"
	requestCaptureDefaultS3Prefix   = "raw"
	requestCaptureDefaultBucket     = "data-proxy-captures"
	requestCaptureObjectS3Service   = "s3"
	requestCaptureObjectEmptySHA256 = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
)

type RequestCaptureStorageConfig struct {
	Enabled  bool   `json:"enabled"`
	Backend  string `json:"backend"`
	SpoolDir string `json:"spool_dir"`
	TmpDir   string `json:"tmp_dir"`
}

type RequestCaptureObject struct {
	Id            string `json:"id"`
	RequestId     string `json:"request_id"`
	Kind          string `json:"kind"`
	Provider      string `json:"provider"`
	Bucket        string `json:"bucket"`
	StorageKey    string `json:"storage_key"`
	ContentType   string `json:"content_type"`
	ContentLength int64  `json:"content_length"`
	BodyBytes     int64  `json:"body_bytes"`
	SHA256        string `json:"sha256"`
	CreatedAt     int64  `json:"created_at"`
}

type requestCaptureObjectStore interface {
	Provider() string
	Save(ctx context.Context, object RequestCaptureObject, body []byte) (RequestCaptureObject, error)
	Load(ctx context.Context, storageKey string) ([]byte, error)
	Delete(ctx context.Context, storageKey string) error
}

func LoadRequestCaptureStorageConfigFromEnv() RequestCaptureStorageConfig {
	backend := requestCaptureObjectBackendFromEnv()
	return RequestCaptureStorageConfig{
		Enabled:  requestCaptureEnvBool(requestCaptureEnabledEnv, false) && backend != "",
		Backend:  backend,
		SpoolDir: requestCaptureEnvString(requestCaptureSpoolDirEnv, requestCaptureDefaultSpoolDir),
		TmpDir:   requestCaptureEnvString(requestCaptureTmpDirEnv, requestCaptureDefaultTmpDir),
	}
}

func RequestCaptureObjectStorageEnabled() bool {
	return LoadRequestCaptureStorageConfigFromEnv().Enabled
}

func SaveRequestCaptureObject(ctx context.Context, object RequestCaptureObject, body []byte) (RequestCaptureObject, error) {
	if len(body) == 0 {
		return RequestCaptureObject{}, errors.New("request capture object body is empty")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if object.Id == "" {
		object.Id = uuid.NewString()
	}
	object.RequestId = strings.TrimSpace(object.RequestId)
	object.Kind = strings.TrimSpace(object.Kind)
	if object.Kind == "" {
		object.Kind = model.RequestCaptureArtifactKindRawBundle
	}
	object.ContentType = strings.TrimSpace(object.ContentType)
	if object.ContentType == "" {
		object.ContentType = "application/octet-stream"
	}
	object.BodyBytes = int64(len(body))
	if object.ContentLength <= 0 {
		object.ContentLength = object.BodyBytes
	}
	if object.SHA256 == "" {
		object.SHA256 = requestCaptureObjectSHA256(body)
	}
	if object.CreatedAt == 0 {
		object.CreatedAt = common.GetTimestamp()
	}
	return requestCaptureObjectStoreFromEnv().Save(ctx, object, body)
}

func LoadRequestCaptureObject(ctx context.Context, storageKey string) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	return requestCaptureObjectStoreFromEnv().Load(ctx, storageKey)
}

func DeleteRequestCaptureObject(ctx context.Context, storageKey string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	storageKey = strings.TrimSpace(storageKey)
	if storageKey == "" {
		return nil
	}
	return requestCaptureObjectStoreFromEnv().Delete(ctx, storageKey)
}

func requestCaptureObjectBackendFromEnv() string {
	backend := strings.TrimSpace(os.Getenv(requestCaptureObjectBackendEnv))
	if backend == "" {
		backend = strings.TrimSpace(os.Getenv(requestCaptureObjectProviderEnv))
	}
	return strings.ToLower(backend)
}

func requestCaptureObjectStoreFromEnv() requestCaptureObjectStore {
	return requestCaptureObjectStoreFromProvider(requestCaptureObjectBackendFromEnv())
}

func requestCaptureObjectStoreFromProvider(provider string) requestCaptureObjectStore {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "s3", "s3-compatible", "s3_compatible", "seaweedfs":
		return newRequestCaptureS3ObjectStoreFromEnv()
	default:
		return requestCaptureUnsupportedObjectStore{provider: provider}
	}
}

type requestCaptureUnsupportedObjectStore struct {
	provider string
}

func (s requestCaptureUnsupportedObjectStore) Provider() string {
	return strings.TrimSpace(s.provider)
}

func (s requestCaptureUnsupportedObjectStore) Save(context.Context, RequestCaptureObject, []byte) (RequestCaptureObject, error) {
	return RequestCaptureObject{}, s.err()
}

func (s requestCaptureUnsupportedObjectStore) Load(context.Context, string) ([]byte, error) {
	return nil, s.err()
}

func (s requestCaptureUnsupportedObjectStore) Delete(context.Context, string) error {
	return s.err()
}

func (s requestCaptureUnsupportedObjectStore) err() error {
	provider := strings.TrimSpace(s.provider)
	if provider == "" {
		return errors.New("request capture object backend is disabled")
	}
	return fmt.Errorf("unsupported request capture object backend %q", provider)
}

type requestCaptureS3ObjectStore struct {
	endpoint      string
	bucket        string
	region        string
	keyPrefix     string
	pathStyle     bool
	credentials   aws.Credentials
	client        *http.Client
	configuration error
}

func newRequestCaptureS3ObjectStoreFromEnv() requestCaptureObjectStore {
	store := &requestCaptureS3ObjectStore{
		endpoint:  strings.TrimSpace(os.Getenv(requestCaptureS3EndpointEnv)),
		bucket:    strings.TrimSpace(os.Getenv(requestCaptureS3BucketEnv)),
		region:    strings.TrimSpace(os.Getenv(requestCaptureS3RegionEnv)),
		keyPrefix: strings.Trim(strings.TrimSpace(os.Getenv(requestCaptureS3KeyPrefixEnv)), "/"),
		pathStyle: requestCaptureEnvBool(requestCaptureS3PathStyleEnv, true),
		credentials: aws.Credentials{
			AccessKeyID:     strings.TrimSpace(os.Getenv(requestCaptureS3AccessKeyEnv)),
			SecretAccessKey: strings.TrimSpace(os.Getenv(requestCaptureS3SecretKeyEnv)),
			SessionToken:    strings.TrimSpace(os.Getenv(requestCaptureS3SessionTokenEnv)),
			Source:          "request-capture-object-env",
		},
		client: http.DefaultClient,
	}
	if store.region == "" {
		store.region = requestCaptureDefaultS3Region
	}
	if store.bucket == "" {
		store.bucket = requestCaptureDefaultBucket
	}
	if store.keyPrefix == "" {
		store.keyPrefix = requestCaptureDefaultS3Prefix
	}
	store.configuration = store.validate()
	return store
}

func (s *requestCaptureS3ObjectStore) Provider() string {
	return "s3"
}

func (s *requestCaptureS3ObjectStore) Save(ctx context.Context, object RequestCaptureObject, body []byte) (RequestCaptureObject, error) {
	if err := s.configuration; err != nil {
		return RequestCaptureObject{}, err
	}
	if err := ctx.Err(); err != nil {
		return RequestCaptureObject{}, err
	}
	object.Provider = s.Provider()
	object.Bucket = s.bucket
	object.StorageKey = strings.Trim(strings.TrimSpace(object.StorageKey), "/")
	if object.StorageKey == "" {
		object.StorageKey = s.objectKey(object)
	}
	if err := s.putObject(ctx, object.StorageKey, body, object.ContentType); err != nil {
		return RequestCaptureObject{}, err
	}
	return object, nil
}

func (s *requestCaptureS3ObjectStore) Load(ctx context.Context, storageKey string) ([]byte, error) {
	if err := s.configuration; err != nil {
		return nil, err
	}
	storageKey = strings.Trim(strings.TrimSpace(storageKey), "/")
	if storageKey == "" {
		return nil, errors.New("request capture object storage key is empty")
	}
	return s.getObject(ctx, storageKey)
}

func (s *requestCaptureS3ObjectStore) Delete(ctx context.Context, storageKey string) error {
	if err := s.configuration; err != nil {
		return err
	}
	storageKey = strings.Trim(strings.TrimSpace(storageKey), "/")
	if storageKey == "" {
		return nil
	}
	return s.deleteObject(ctx, storageKey)
}

func (s *requestCaptureS3ObjectStore) validate() error {
	if s.endpoint == "" {
		return errors.New("CAPTURE_S3_ENDPOINT is required when CAPTURE_OBJECT_BACKEND=s3")
	}
	if _, err := url.ParseRequestURI(s.endpoint); err != nil {
		return fmt.Errorf("invalid CAPTURE_S3_ENDPOINT: %w", err)
	}
	if s.bucket == "" {
		return errors.New("CAPTURE_S3_BUCKET is required when CAPTURE_OBJECT_BACKEND=s3")
	}
	if s.credentials.AccessKeyID == "" || s.credentials.SecretAccessKey == "" {
		return errors.New("CAPTURE_S3_ACCESS_KEY and CAPTURE_S3_SECRET_KEY are required when CAPTURE_OBJECT_BACKEND=s3")
	}
	return nil
}

func (s *requestCaptureS3ObjectStore) objectKey(object RequestCaptureObject) string {
	createdAt := object.CreatedAt
	if createdAt == 0 {
		createdAt = common.GetTimestamp()
	}
	t := time.Unix(createdAt, 0).UTC()
	requestId := requestCaptureObjectKeyName(object)
	return requestCaptureJoinS3Key(
		s.keyPrefix,
		t.Format("2006"),
		t.Format("01"),
		t.Format("02"),
		t.Format("15"),
		requestCapturePrefixSegment(requestId, 0, 2),
		requestCapturePrefixSegment(requestId, 2, 4),
		requestId+".bundle.tar.zst.enc",
	)
}

func (s *requestCaptureS3ObjectStore) putObject(ctx context.Context, key string, content []byte, contentType string) error {
	req, err := s.newObjectRequest(ctx, http.MethodPut, key, bytes.NewReader(content))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", contentType)
	req.ContentLength = int64(len(content))
	payloadHash := requestCaptureObjectSHA256(content)
	if err := s.sign(ctx, req, payloadHash); err != nil {
		return err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("s3 put request capture object %s failed with status %d: %s", key, resp.StatusCode, requestCaptureReadS3ErrorBody(resp.Body))
	}
	return nil
}

func (s *requestCaptureS3ObjectStore) getObject(ctx context.Context, key string) ([]byte, error) {
	req, err := s.newObjectRequest(ctx, http.MethodGet, key, nil)
	if err != nil {
		return nil, err
	}
	if err := s.sign(ctx, req, requestCaptureObjectEmptySHA256); err != nil {
		return nil, err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("s3 get request capture object %s failed with status %d: %s", key, resp.StatusCode, requestCaptureReadS3ErrorBody(resp.Body))
	}
	return io.ReadAll(resp.Body)
}

func (s *requestCaptureS3ObjectStore) deleteObject(ctx context.Context, key string) error {
	req, err := s.newObjectRequest(ctx, http.MethodDelete, key, nil)
	if err != nil {
		return err
	}
	if err := s.sign(ctx, req, requestCaptureObjectEmptySHA256); err != nil {
		return err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("s3 delete request capture object %s failed with status %d: %s", key, resp.StatusCode, requestCaptureReadS3ErrorBody(resp.Body))
	}
	return nil
}

func (s *requestCaptureS3ObjectStore) newObjectRequest(ctx context.Context, method string, key string, body io.Reader) (*http.Request, error) {
	requestURL, err := s.objectURL(key)
	if err != nil {
		return nil, err
	}
	return http.NewRequestWithContext(ctx, method, requestURL, body)
}

func (s *requestCaptureS3ObjectStore) objectURL(key string) (string, error) {
	base, err := url.Parse(s.endpoint)
	if err != nil {
		return "", err
	}
	if s.pathStyle {
		base.Path = requestCaptureJoinURLPath(base.Path, s.bucket, key)
		return base.String(), nil
	}
	base.Host = s.bucket + "." + base.Host
	base.Path = requestCaptureJoinURLPath(base.Path, key)
	return base.String(), nil
}

func (s *requestCaptureS3ObjectStore) sign(ctx context.Context, req *http.Request, payloadHash string) error {
	req.Header.Set("X-Amz-Content-Sha256", payloadHash)
	return v4.NewSigner().SignHTTP(ctx, s.credentials, req, payloadHash, requestCaptureObjectS3Service, s.region, time.Now())
}

func requestCaptureObjectKeyName(object RequestCaptureObject) string {
	name := strings.TrimSpace(object.RequestId)
	if name == "" {
		name = strings.TrimSpace(object.Id)
	}
	if name == "" {
		name = uuid.NewString()
	}
	return requestCaptureSanitizeKeySegment(name)
}

func requestCaptureSanitizeKeySegment(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return uuid.NewString()
	}
	var builder strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			builder.WriteRune(r)
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r == '-', r == '_', r == '.':
			builder.WriteRune(r)
		default:
			builder.WriteByte('_')
		}
	}
	if builder.Len() == 0 {
		return uuid.NewString()
	}
	return builder.String()
}

func requestCapturePrefixSegment(value string, start int, end int) string {
	if len(value) >= end {
		return value[start:end]
	}
	if len(value) > start {
		return value[start:]
	}
	return "00"
}

func requestCaptureObjectSHA256(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func requestCaptureJoinS3Key(parts ...string) string {
	cleaned := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.Trim(part, "/")
		if part != "" {
			cleaned = append(cleaned, part)
		}
	}
	return strings.Join(cleaned, "/")
}

func requestCaptureJoinURLPath(base string, parts ...string) string {
	segments := []string{}
	for _, segment := range strings.Split(strings.Trim(base, "/"), "/") {
		if segment != "" {
			segments = append(segments, url.PathEscape(segment))
		}
	}
	for _, part := range parts {
		for _, segment := range strings.Split(strings.Trim(part, "/"), "/") {
			if segment != "" {
				segments = append(segments, url.PathEscape(segment))
			}
		}
	}
	if len(segments) == 0 {
		return "/"
	}
	return "/" + strings.Join(segments, "/")
}

func requestCaptureReadS3ErrorBody(reader io.Reader) string {
	body, err := io.ReadAll(io.LimitReader(reader, 4096))
	if err != nil {
		return err.Error()
	}
	value := strings.TrimSpace(string(body))
	if len(value) <= 512 {
		return value
	}
	return value[:509] + "..."
}

func requestCaptureEnvString(name string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	return value
}

func requestCaptureEnvBool(name string, fallback bool) bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(name)))
	if value == "" {
		return fallback
	}
	switch value {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}
