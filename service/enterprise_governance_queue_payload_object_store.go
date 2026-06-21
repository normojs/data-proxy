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
	"path/filepath"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/google/uuid"
)

const (
	enterpriseQueuePayloadObjectProviderEnv       = "ENTERPRISE_QUEUE_PAYLOAD_OBJECT_PROVIDER"
	enterpriseQueuePayloadObjectDirEnv            = "ENTERPRISE_QUEUE_PAYLOAD_OBJECT_DIR"
	enterpriseQueuePayloadObjectS3EndpointEnv     = "ENTERPRISE_QUEUE_PAYLOAD_OBJECT_S3_ENDPOINT"
	enterpriseQueuePayloadObjectS3BucketEnv       = "ENTERPRISE_QUEUE_PAYLOAD_OBJECT_S3_BUCKET"
	enterpriseQueuePayloadObjectS3RegionEnv       = "ENTERPRISE_QUEUE_PAYLOAD_OBJECT_S3_REGION"
	enterpriseQueuePayloadObjectS3AccessKeyEnv    = "ENTERPRISE_QUEUE_PAYLOAD_OBJECT_S3_ACCESS_KEY"
	enterpriseQueuePayloadObjectS3SecretKeyEnv    = "ENTERPRISE_QUEUE_PAYLOAD_OBJECT_S3_SECRET_KEY"
	enterpriseQueuePayloadObjectS3SessionTokenEnv = "ENTERPRISE_QUEUE_PAYLOAD_OBJECT_S3_SESSION_TOKEN"
	enterpriseQueuePayloadObjectS3KeyPrefixEnv    = "ENTERPRISE_QUEUE_PAYLOAD_OBJECT_S3_KEY_PREFIX"
	enterpriseQueuePayloadObjectS3PathStyleEnv    = "ENTERPRISE_QUEUE_PAYLOAD_OBJECT_S3_PATH_STYLE"
	enterpriseQueuePayloadObjectS3DefaultRegion   = "us-east-1"
	enterpriseQueuePayloadObjectS3DefaultPrefix   = "enterprise-queue-payloads"
	enterpriseQueuePayloadObjectStoreDirName      = "data-proxy-enterprise-queue-payloads"
	enterpriseQueuePayloadObjectS3Service         = "s3"
	enterpriseQueuePayloadObjectEmptySHA256       = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
)

type EnterpriseGovernanceQueuePayloadObject struct {
	Id            string `json:"id"`
	Provider      string `json:"provider"`
	StorageKey    string `json:"storage_key"`
	AdmissionId   int64  `json:"admission_id"`
	RequestId     string `json:"request_id"`
	EnterpriseId  int    `json:"enterprise_id"`
	UserId        int    `json:"user_id"`
	TokenId       int    `json:"token_id"`
	ContentType   string `json:"content_type"`
	ContentLength int64  `json:"content_length"`
	BodyBytes     int64  `json:"body_bytes"`
	SHA256        string `json:"sha256"`
	CreatedAt     int64  `json:"created_at"`
}

type enterpriseGovernanceQueuePayloadObjectStore interface {
	Provider() string
	Save(ctx context.Context, object EnterpriseGovernanceQueuePayloadObject, body []byte) (EnterpriseGovernanceQueuePayloadObject, error)
	Load(ctx context.Context, id string) (EnterpriseGovernanceQueuePayloadObject, []byte, error)
	Delete(ctx context.Context, id string) error
}

func EnterpriseGovernanceQueuePayloadObjectStorageEnabled() bool {
	return strings.TrimSpace(os.Getenv(enterpriseQueuePayloadObjectProviderEnv)) != ""
}

func SaveEnterpriseGovernanceQueuePayloadObject(ctx context.Context, row model.EnterpriseGovernanceQueuePayload, body []byte) (EnterpriseGovernanceQueuePayloadObject, error) {
	if len(body) == 0 {
		return EnterpriseGovernanceQueuePayloadObject{}, errors.New("enterprise queue payload object body is empty")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	object := EnterpriseGovernanceQueuePayloadObject{
		Id:            uuid.NewString(),
		AdmissionId:   row.AdmissionId,
		RequestId:     strings.TrimSpace(row.RequestId),
		EnterpriseId:  row.EnterpriseId,
		UserId:        row.UserId,
		TokenId:       row.TokenId,
		ContentType:   strings.TrimSpace(row.ContentType),
		ContentLength: row.ContentLength,
		BodyBytes:     int64(len(body)),
		SHA256:        strings.TrimSpace(row.SHA256),
		CreatedAt:     common.GetTimestamp(),
	}
	if object.ContentType == "" {
		object.ContentType = "application/octet-stream"
	}
	if object.SHA256 == "" {
		object.SHA256 = enterpriseGovernanceQueuePayloadObjectSHA256(body)
	}
	return enterpriseGovernanceQueuePayloadObjectStoreFromEnv().Save(ctx, object, body)
}

func LoadEnterpriseGovernanceQueuePayloadObject(ctx context.Context, id string) (EnterpriseGovernanceQueuePayloadObject, []byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	return enterpriseGovernanceQueuePayloadObjectStoreFromEnv().Load(ctx, id)
}

func LoadEnterpriseGovernanceQueuePayloadObjectFromRegistry(ctx context.Context, provider string, id string) (EnterpriseGovernanceQueuePayloadObject, []byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	return enterpriseGovernanceQueuePayloadObjectStoreFromRegistry(provider).Load(ctx, id)
}

func DeleteEnterpriseGovernanceQueuePayloadObject(ctx context.Context, id string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return nil
	}
	return enterpriseGovernanceQueuePayloadObjectStoreFromProvider(os.Getenv(enterpriseQueuePayloadObjectProviderEnv)).Delete(ctx, id)
}

func DeleteEnterpriseGovernanceQueuePayloadObjectFromRegistry(ctx context.Context, provider string, id string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return nil
	}
	return enterpriseGovernanceQueuePayloadObjectStoreFromRegistry(provider).Delete(ctx, id)
}

func enterpriseGovernanceQueuePayloadObjectStoreFromEnv() enterpriseGovernanceQueuePayloadObjectStore {
	return enterpriseGovernanceQueuePayloadObjectStoreFromProvider(os.Getenv(enterpriseQueuePayloadObjectProviderEnv))
}

func enterpriseGovernanceQueuePayloadObjectStoreFromRegistry(provider string) enterpriseGovernanceQueuePayloadObjectStore {
	if strings.TrimSpace(provider) == "" {
		return enterpriseGovernanceQueuePayloadObjectStoreFromEnv()
	}
	return enterpriseGovernanceQueuePayloadObjectStoreFromProvider(provider)
}

func enterpriseGovernanceQueuePayloadObjectStoreFromProvider(provider string) enterpriseGovernanceQueuePayloadObjectStore {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "":
		return enterpriseGovernanceQueuePayloadLocalObjectStore{}
	case "local", "disk", "filesystem", "fs":
		return enterpriseGovernanceQueuePayloadLocalObjectStore{}
	case "s3", "s3-compatible", "s3_compatible":
		return newEnterpriseGovernanceQueuePayloadS3ObjectStoreFromEnv()
	default:
		return enterpriseGovernanceQueuePayloadUnsupportedObjectStore{provider: provider}
	}
}

type enterpriseGovernanceQueuePayloadUnsupportedObjectStore struct {
	provider string
}

func (s enterpriseGovernanceQueuePayloadUnsupportedObjectStore) Provider() string {
	return strings.TrimSpace(s.provider)
}

func (s enterpriseGovernanceQueuePayloadUnsupportedObjectStore) Save(context.Context, EnterpriseGovernanceQueuePayloadObject, []byte) (EnterpriseGovernanceQueuePayloadObject, error) {
	return EnterpriseGovernanceQueuePayloadObject{}, s.err()
}

func (s enterpriseGovernanceQueuePayloadUnsupportedObjectStore) Load(context.Context, string) (EnterpriseGovernanceQueuePayloadObject, []byte, error) {
	return EnterpriseGovernanceQueuePayloadObject{}, nil, s.err()
}

func (s enterpriseGovernanceQueuePayloadUnsupportedObjectStore) Delete(context.Context, string) error {
	return s.err()
}

func (s enterpriseGovernanceQueuePayloadUnsupportedObjectStore) err() error {
	provider := strings.TrimSpace(s.provider)
	if provider == "" {
		provider = "<empty>"
	}
	return fmt.Errorf("unsupported enterprise queue payload object provider %q", provider)
}

type enterpriseGovernanceQueuePayloadLocalObjectStore struct{}

func (enterpriseGovernanceQueuePayloadLocalObjectStore) Provider() string {
	return "local"
}

func (enterpriseGovernanceQueuePayloadLocalObjectStore) Save(ctx context.Context, object EnterpriseGovernanceQueuePayloadObject, body []byte) (EnterpriseGovernanceQueuePayloadObject, error) {
	if err := ctx.Err(); err != nil {
		return EnterpriseGovernanceQueuePayloadObject{}, err
	}
	object.Provider = "local"
	object.StorageKey = object.Id
	dir, err := enterpriseGovernanceQueuePayloadLocalObjectDir(object.Id)
	if err != nil {
		return EnterpriseGovernanceQueuePayloadObject{}, err
	}
	if err := os.MkdirAll(dir, 0750); err != nil {
		return EnterpriseGovernanceQueuePayloadObject{}, err
	}
	bodyPath := enterpriseGovernanceQueuePayloadLocalObjectBodyPath(object.Id)
	metaPath := enterpriseGovernanceQueuePayloadLocalObjectMetaPath(object.Id)
	if err := os.WriteFile(bodyPath, body, 0600); err != nil {
		return EnterpriseGovernanceQueuePayloadObject{}, err
	}
	metaBytes, err := common.Marshal(object)
	if err != nil {
		_ = os.Remove(bodyPath)
		return EnterpriseGovernanceQueuePayloadObject{}, err
	}
	if err := os.WriteFile(metaPath, metaBytes, 0600); err != nil {
		_ = os.Remove(bodyPath)
		return EnterpriseGovernanceQueuePayloadObject{}, err
	}
	return object, nil
}

func (enterpriseGovernanceQueuePayloadLocalObjectStore) Load(ctx context.Context, id string) (EnterpriseGovernanceQueuePayloadObject, []byte, error) {
	if err := ctx.Err(); err != nil {
		return EnterpriseGovernanceQueuePayloadObject{}, nil, err
	}
	if !enterpriseGovernanceQueuePayloadObjectIdValid(id) {
		return EnterpriseGovernanceQueuePayloadObject{}, nil, errors.New("invalid enterprise queue payload object id")
	}
	metaBytes, err := os.ReadFile(enterpriseGovernanceQueuePayloadLocalObjectMetaPath(id))
	if err != nil {
		return EnterpriseGovernanceQueuePayloadObject{}, nil, err
	}
	var object EnterpriseGovernanceQueuePayloadObject
	if err := common.Unmarshal(metaBytes, &object); err != nil {
		return EnterpriseGovernanceQueuePayloadObject{}, nil, err
	}
	if object.Id != id {
		return EnterpriseGovernanceQueuePayloadObject{}, nil, errors.New("enterprise queue payload object metadata mismatch")
	}
	body, err := os.ReadFile(enterpriseGovernanceQueuePayloadLocalObjectBodyPath(id))
	if err != nil {
		return EnterpriseGovernanceQueuePayloadObject{}, nil, err
	}
	return object, body, nil
}

func (enterpriseGovernanceQueuePayloadLocalObjectStore) Delete(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !enterpriseGovernanceQueuePayloadObjectIdValid(id) {
		return errors.New("invalid enterprise queue payload object id")
	}
	dir, err := enterpriseGovernanceQueuePayloadLocalObjectDir(id)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(dir); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func enterpriseGovernanceQueuePayloadLocalObjectBaseDir() string {
	base := strings.TrimSpace(os.Getenv(enterpriseQueuePayloadObjectDirEnv))
	if base == "" {
		base = filepath.Join(os.TempDir(), enterpriseQueuePayloadObjectStoreDirName)
	}
	return base
}

func enterpriseGovernanceQueuePayloadLocalObjectDir(id string) (string, error) {
	if !enterpriseGovernanceQueuePayloadObjectIdValid(id) {
		return "", errors.New("invalid enterprise queue payload object id")
	}
	return filepath.Join(enterpriseGovernanceQueuePayloadLocalObjectBaseDir(), id[:2], id), nil
}

func enterpriseGovernanceQueuePayloadLocalObjectBodyPath(id string) string {
	dir, _ := enterpriseGovernanceQueuePayloadLocalObjectDir(id)
	return filepath.Join(dir, "body.bin")
}

func enterpriseGovernanceQueuePayloadLocalObjectMetaPath(id string) string {
	dir, _ := enterpriseGovernanceQueuePayloadLocalObjectDir(id)
	return filepath.Join(dir, "metadata.json")
}

type enterpriseGovernanceQueuePayloadS3ObjectStore struct {
	endpoint      string
	bucket        string
	region        string
	keyPrefix     string
	pathStyle     bool
	credentials   aws.Credentials
	client        *http.Client
	configuration error
}

func newEnterpriseGovernanceQueuePayloadS3ObjectStoreFromEnv() enterpriseGovernanceQueuePayloadObjectStore {
	store := &enterpriseGovernanceQueuePayloadS3ObjectStore{
		endpoint:  strings.TrimSpace(os.Getenv(enterpriseQueuePayloadObjectS3EndpointEnv)),
		bucket:    strings.TrimSpace(os.Getenv(enterpriseQueuePayloadObjectS3BucketEnv)),
		region:    strings.TrimSpace(os.Getenv(enterpriseQueuePayloadObjectS3RegionEnv)),
		keyPrefix: strings.Trim(strings.TrimSpace(os.Getenv(enterpriseQueuePayloadObjectS3KeyPrefixEnv)), "/"),
		pathStyle: enterpriseGovernanceQueuePayloadEnvBool(enterpriseQueuePayloadObjectS3PathStyleEnv, true),
		credentials: aws.Credentials{
			AccessKeyID:     strings.TrimSpace(os.Getenv(enterpriseQueuePayloadObjectS3AccessKeyEnv)),
			SecretAccessKey: strings.TrimSpace(os.Getenv(enterpriseQueuePayloadObjectS3SecretKeyEnv)),
			SessionToken:    strings.TrimSpace(os.Getenv(enterpriseQueuePayloadObjectS3SessionTokenEnv)),
			Source:          "enterprise-queue-payload-object-env",
		},
		client: http.DefaultClient,
	}
	if store.region == "" {
		store.region = enterpriseQueuePayloadObjectS3DefaultRegion
	}
	if store.keyPrefix == "" {
		store.keyPrefix = enterpriseQueuePayloadObjectS3DefaultPrefix
	}
	store.configuration = store.validate()
	return store
}

func (s *enterpriseGovernanceQueuePayloadS3ObjectStore) Provider() string {
	return "s3"
}

func (s *enterpriseGovernanceQueuePayloadS3ObjectStore) Save(ctx context.Context, object EnterpriseGovernanceQueuePayloadObject, body []byte) (EnterpriseGovernanceQueuePayloadObject, error) {
	if err := s.configuration; err != nil {
		return EnterpriseGovernanceQueuePayloadObject{}, err
	}
	if err := ctx.Err(); err != nil {
		return EnterpriseGovernanceQueuePayloadObject{}, err
	}
	object.Provider = s.Provider()
	object.StorageKey = s.objectPrefix(object.Id)
	metaBytes, err := common.Marshal(object)
	if err != nil {
		return EnterpriseGovernanceQueuePayloadObject{}, err
	}
	if err := s.putObject(ctx, s.bodyKey(object.Id), body, object.ContentType); err != nil {
		return EnterpriseGovernanceQueuePayloadObject{}, err
	}
	if err := s.putObject(ctx, s.metadataKey(object.Id), metaBytes, "application/json"); err != nil {
		_ = s.deleteObject(context.Background(), s.bodyKey(object.Id))
		return EnterpriseGovernanceQueuePayloadObject{}, err
	}
	return object, nil
}

func (s *enterpriseGovernanceQueuePayloadS3ObjectStore) Load(ctx context.Context, id string) (EnterpriseGovernanceQueuePayloadObject, []byte, error) {
	if err := s.configuration; err != nil {
		return EnterpriseGovernanceQueuePayloadObject{}, nil, err
	}
	if !enterpriseGovernanceQueuePayloadObjectIdValid(id) {
		return EnterpriseGovernanceQueuePayloadObject{}, nil, errors.New("invalid enterprise queue payload object id")
	}
	metaBytes, err := s.getObject(ctx, s.metadataKey(id))
	if err != nil {
		return EnterpriseGovernanceQueuePayloadObject{}, nil, err
	}
	var object EnterpriseGovernanceQueuePayloadObject
	if err := common.Unmarshal(metaBytes, &object); err != nil {
		return EnterpriseGovernanceQueuePayloadObject{}, nil, err
	}
	if object.Id != id {
		return EnterpriseGovernanceQueuePayloadObject{}, nil, errors.New("enterprise queue payload object metadata mismatch")
	}
	body, err := s.getObject(ctx, s.bodyKey(id))
	if err != nil {
		return EnterpriseGovernanceQueuePayloadObject{}, nil, err
	}
	return object, body, nil
}

func (s *enterpriseGovernanceQueuePayloadS3ObjectStore) Delete(ctx context.Context, id string) error {
	if err := s.configuration; err != nil {
		return err
	}
	if !enterpriseGovernanceQueuePayloadObjectIdValid(id) {
		return errors.New("invalid enterprise queue payload object id")
	}
	bodyErr := s.deleteObject(ctx, s.bodyKey(id))
	metaErr := s.deleteObject(ctx, s.metadataKey(id))
	if bodyErr != nil {
		return bodyErr
	}
	return metaErr
}

func (s *enterpriseGovernanceQueuePayloadS3ObjectStore) validate() error {
	if s.endpoint == "" {
		return errors.New("ENTERPRISE_QUEUE_PAYLOAD_OBJECT_S3_ENDPOINT is required when ENTERPRISE_QUEUE_PAYLOAD_OBJECT_PROVIDER=s3")
	}
	if _, err := url.ParseRequestURI(s.endpoint); err != nil {
		return fmt.Errorf("invalid ENTERPRISE_QUEUE_PAYLOAD_OBJECT_S3_ENDPOINT: %w", err)
	}
	if s.bucket == "" {
		return errors.New("ENTERPRISE_QUEUE_PAYLOAD_OBJECT_S3_BUCKET is required when ENTERPRISE_QUEUE_PAYLOAD_OBJECT_PROVIDER=s3")
	}
	if s.credentials.AccessKeyID == "" || s.credentials.SecretAccessKey == "" {
		return errors.New("ENTERPRISE_QUEUE_PAYLOAD_OBJECT_S3_ACCESS_KEY and ENTERPRISE_QUEUE_PAYLOAD_OBJECT_S3_SECRET_KEY are required when ENTERPRISE_QUEUE_PAYLOAD_OBJECT_PROVIDER=s3")
	}
	return nil
}

func (s *enterpriseGovernanceQueuePayloadS3ObjectStore) objectPrefix(id string) string {
	return enterpriseGovernanceQueuePayloadJoinS3Key(s.keyPrefix, id[:2], id)
}

func (s *enterpriseGovernanceQueuePayloadS3ObjectStore) bodyKey(id string) string {
	return enterpriseGovernanceQueuePayloadJoinS3Key(s.objectPrefix(id), "body.bin")
}

func (s *enterpriseGovernanceQueuePayloadS3ObjectStore) metadataKey(id string) string {
	return enterpriseGovernanceQueuePayloadJoinS3Key(s.objectPrefix(id), "metadata.json")
}

func (s *enterpriseGovernanceQueuePayloadS3ObjectStore) putObject(ctx context.Context, key string, content []byte, contentType string) error {
	req, err := s.newObjectRequest(ctx, http.MethodPut, key, bytes.NewReader(content))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", contentType)
	req.ContentLength = int64(len(content))
	payloadHash := enterpriseGovernanceQueuePayloadObjectSHA256(content)
	if err := s.sign(ctx, req, payloadHash); err != nil {
		return err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("s3 put enterprise queue payload object %s failed with status %d: %s", key, resp.StatusCode, enterpriseGovernanceQueuePayloadReadS3ErrorBody(resp.Body))
	}
	return nil
}

func (s *enterpriseGovernanceQueuePayloadS3ObjectStore) getObject(ctx context.Context, key string) ([]byte, error) {
	req, err := s.newObjectRequest(ctx, http.MethodGet, key, nil)
	if err != nil {
		return nil, err
	}
	if err := s.sign(ctx, req, enterpriseQueuePayloadObjectEmptySHA256); err != nil {
		return nil, err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("s3 get enterprise queue payload object %s failed with status %d: %s", key, resp.StatusCode, enterpriseGovernanceQueuePayloadReadS3ErrorBody(resp.Body))
	}
	return io.ReadAll(resp.Body)
}

func (s *enterpriseGovernanceQueuePayloadS3ObjectStore) deleteObject(ctx context.Context, key string) error {
	req, err := s.newObjectRequest(ctx, http.MethodDelete, key, nil)
	if err != nil {
		return err
	}
	if err := s.sign(ctx, req, enterpriseQueuePayloadObjectEmptySHA256); err != nil {
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
		return fmt.Errorf("s3 delete enterprise queue payload object %s failed with status %d: %s", key, resp.StatusCode, enterpriseGovernanceQueuePayloadReadS3ErrorBody(resp.Body))
	}
	return nil
}

func (s *enterpriseGovernanceQueuePayloadS3ObjectStore) newObjectRequest(ctx context.Context, method string, key string, body io.Reader) (*http.Request, error) {
	requestURL, err := s.objectURL(key)
	if err != nil {
		return nil, err
	}
	return http.NewRequestWithContext(ctx, method, requestURL, body)
}

func (s *enterpriseGovernanceQueuePayloadS3ObjectStore) objectURL(key string) (string, error) {
	base, err := url.Parse(s.endpoint)
	if err != nil {
		return "", err
	}
	if s.pathStyle {
		base.Path = enterpriseGovernanceQueuePayloadJoinURLPath(base.Path, s.bucket, key)
		return base.String(), nil
	}
	base.Host = s.bucket + "." + base.Host
	base.Path = enterpriseGovernanceQueuePayloadJoinURLPath(base.Path, key)
	return base.String(), nil
}

func (s *enterpriseGovernanceQueuePayloadS3ObjectStore) sign(ctx context.Context, req *http.Request, payloadHash string) error {
	req.Header.Set("X-Amz-Content-Sha256", payloadHash)
	return v4.NewSigner().SignHTTP(ctx, s.credentials, req, payloadHash, enterpriseQueuePayloadObjectS3Service, s.region, time.Now())
}

func enterpriseGovernanceQueuePayloadObjectIdValid(id string) bool {
	if len(strings.TrimSpace(id)) != 36 {
		return false
	}
	_, err := uuid.Parse(id)
	return err == nil
}

func enterpriseGovernanceQueuePayloadObjectSHA256(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func enterpriseGovernanceQueuePayloadJoinS3Key(parts ...string) string {
	cleaned := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.Trim(part, "/")
		if part != "" {
			cleaned = append(cleaned, part)
		}
	}
	return strings.Join(cleaned, "/")
}

func enterpriseGovernanceQueuePayloadJoinURLPath(base string, parts ...string) string {
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

func enterpriseGovernanceQueuePayloadReadS3ErrorBody(reader io.Reader) string {
	body, err := io.ReadAll(io.LimitReader(reader, 4096))
	if err != nil {
		return err.Error()
	}
	return enterpriseGovernanceQueuePayloadTruncateErrorBody(string(body))
}

func enterpriseGovernanceQueuePayloadTruncateErrorBody(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= 512 {
		return value
	}
	return value[:509] + "..."
}

func enterpriseGovernanceQueuePayloadEnvBool(name string, fallback bool) bool {
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
