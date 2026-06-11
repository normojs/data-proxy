package openapi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/signer/v4"
)

const (
	openAPIBinaryObjectS3EndpointEnv      = "OPENAPI_BINARY_OBJECT_S3_ENDPOINT"
	openAPIBinaryObjectS3BucketEnv        = "OPENAPI_BINARY_OBJECT_S3_BUCKET"
	openAPIBinaryObjectS3RegionEnv        = "OPENAPI_BINARY_OBJECT_S3_REGION"
	openAPIBinaryObjectS3AccessKeyEnv     = "OPENAPI_BINARY_OBJECT_S3_ACCESS_KEY"
	openAPIBinaryObjectS3SecretKeyEnv     = "OPENAPI_BINARY_OBJECT_S3_SECRET_KEY"
	openAPIBinaryObjectS3SessionTokenEnv  = "OPENAPI_BINARY_OBJECT_S3_SESSION_TOKEN"
	openAPIBinaryObjectS3KeyPrefixEnv     = "OPENAPI_BINARY_OBJECT_S3_KEY_PREFIX"
	openAPIBinaryObjectS3PathStyleEnv     = "OPENAPI_BINARY_OBJECT_S3_PATH_STYLE"
	openAPIBinaryObjectS3DefaultRegion    = "us-east-1"
	openAPIBinaryObjectS3DefaultKeyPrefix = "openapi-binary-objects"
	openAPIBinaryObjectS3ListMaxKeys      = 1000
	openAPIBinaryObjectS3Service          = "s3"
	openAPIBinaryObjectEmptyPayloadSHA256 = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
)

type s3BinaryObjectStore struct {
	endpoint      string
	bucket        string
	region        string
	keyPrefix     string
	pathStyle     bool
	credentials   aws.Credentials
	client        *http.Client
	configuration error
}

type s3ListBucketResult struct {
	IsTruncated           bool   `xml:"IsTruncated"`
	NextContinuationToken string `xml:"NextContinuationToken"`
	Contents              []struct {
		Key  string `xml:"Key"`
		Size int64  `xml:"Size"`
	} `xml:"Contents"`
}

func newS3BinaryObjectStoreFromEnv() BinaryObjectStore {
	store := &s3BinaryObjectStore{
		endpoint:  strings.TrimSpace(os.Getenv(openAPIBinaryObjectS3EndpointEnv)),
		bucket:    strings.TrimSpace(os.Getenv(openAPIBinaryObjectS3BucketEnv)),
		region:    strings.TrimSpace(os.Getenv(openAPIBinaryObjectS3RegionEnv)),
		keyPrefix: strings.Trim(strings.TrimSpace(os.Getenv(openAPIBinaryObjectS3KeyPrefixEnv)), "/"),
		pathStyle: envBool(openAPIBinaryObjectS3PathStyleEnv, true),
		credentials: aws.Credentials{
			AccessKeyID:     strings.TrimSpace(os.Getenv(openAPIBinaryObjectS3AccessKeyEnv)),
			SecretAccessKey: strings.TrimSpace(os.Getenv(openAPIBinaryObjectS3SecretKeyEnv)),
			SessionToken:    strings.TrimSpace(os.Getenv(openAPIBinaryObjectS3SessionTokenEnv)),
			Source:          "openapi-binary-object-env",
		},
		client: http.DefaultClient,
	}
	if store.region == "" {
		store.region = openAPIBinaryObjectS3DefaultRegion
	}
	if store.keyPrefix == "" {
		store.keyPrefix = openAPIBinaryObjectS3DefaultKeyPrefix
	}
	store.configuration = store.validate()
	return store
}

func (s *s3BinaryObjectStore) Provider() string {
	return "s3"
}

func (s *s3BinaryObjectStore) Save(ctx context.Context, object BinaryObject, content []byte) (BinaryObject, error) {
	if err := s.configuration; err != nil {
		return BinaryObject{}, err
	}
	if err := ctx.Err(); err != nil {
		return BinaryObject{}, err
	}
	object.Provider = s.Provider()
	object.StorageKey = s.objectPrefix(object.Id)
	metaBytes, err := json.Marshal(object)
	if err != nil {
		return BinaryObject{}, err
	}
	if err := s.putObject(ctx, s.bodyKey(object.Id), content, object.ContentType); err != nil {
		return BinaryObject{}, err
	}
	if err := s.putObject(ctx, s.metadataKey(object.Id), metaBytes, "application/json"); err != nil {
		_ = s.deleteObject(context.Background(), s.bodyKey(object.Id))
		return BinaryObject{}, err
	}
	return object, nil
}

func (s *s3BinaryObjectStore) Load(ctx context.Context, id string) (BinaryObject, []byte, error) {
	if err := s.configuration; err != nil {
		return BinaryObject{}, nil, err
	}
	id = strings.TrimSpace(id)
	if !isValidBinaryObjectId(id) {
		return BinaryObject{}, nil, errors.New("invalid openapi binary object id")
	}
	metaBytes, err := s.getObject(ctx, s.metadataKey(id))
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
	content, err := s.getObject(ctx, s.bodyKey(id))
	if err != nil {
		return BinaryObject{}, nil, err
	}
	return object, content, nil
}

func (s *s3BinaryObjectStore) Cleanup(ctx context.Context, options BinaryObjectCleanupOptions) (BinaryObjectCleanupResult, error) {
	result := BinaryObjectCleanupResult{
		Provider:   s.Provider(),
		CutoffUnix: options.CutoffUnix,
		DryRun:     options.DryRun,
		Errors:     []string{},
	}
	if err := s.configuration; err != nil {
		return result, err
	}
	if options.CutoffUnix <= 0 {
		return result, errors.New("cutoff_unix is required")
	}
	prefix := s.keyPrefix
	if prefix != "" {
		prefix += "/"
	}
	var continuationToken string
	for {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		page, err := s.listObjects(ctx, prefix, continuationToken)
		if err != nil {
			return result, err
		}
		for _, item := range page.Contents {
			if options.Limit > 0 && result.Deleted >= options.Limit {
				if len(result.Errors) == 0 {
					result.Errors = nil
				}
				return result, nil
			}
			if !strings.HasSuffix(item.Key, "/metadata.json") {
				continue
			}
			metaBytes, err := s.getObject(ctx, item.Key)
			if err != nil {
				result.Errors = append(result.Errors, err.Error())
				continue
			}
			var object BinaryObject
			if err := json.Unmarshal(metaBytes, &object); err != nil {
				result.Errors = append(result.Errors, err.Error())
				continue
			}
			result.Scanned++
			if object.CreatedAt <= 0 || object.CreatedAt >= options.CutoffUnix {
				continue
			}
			if options.DryRun {
				result.Deleted++
				result.DeletedBytes += int64(object.Size)
				result.DeletedObjectIds = append(result.DeletedObjectIds, object.Id)
				continue
			}
			if err := s.deleteObject(ctx, s.bodyKey(object.Id)); err != nil {
				result.Errors = append(result.Errors, err.Error())
				continue
			}
			if err := s.deleteObject(ctx, s.metadataKey(object.Id)); err != nil {
				result.Errors = append(result.Errors, err.Error())
				continue
			}
			result.Deleted++
			result.DeletedBytes += int64(object.Size)
			result.DeletedObjectIds = append(result.DeletedObjectIds, object.Id)
		}
		if !page.IsTruncated || strings.TrimSpace(page.NextContinuationToken) == "" {
			break
		}
		continuationToken = page.NextContinuationToken
	}
	if len(result.Errors) == 0 {
		result.Errors = nil
	}
	return result, nil
}

func (s *s3BinaryObjectStore) validate() error {
	if s.endpoint == "" {
		return errors.New("OPENAPI_BINARY_OBJECT_S3_ENDPOINT is required when OPENAPI_BINARY_OBJECT_PROVIDER=s3")
	}
	if _, err := url.ParseRequestURI(s.endpoint); err != nil {
		return fmt.Errorf("invalid OPENAPI_BINARY_OBJECT_S3_ENDPOINT: %w", err)
	}
	if s.bucket == "" {
		return errors.New("OPENAPI_BINARY_OBJECT_S3_BUCKET is required when OPENAPI_BINARY_OBJECT_PROVIDER=s3")
	}
	if s.credentials.AccessKeyID == "" || s.credentials.SecretAccessKey == "" {
		return errors.New("OPENAPI_BINARY_OBJECT_S3_ACCESS_KEY and OPENAPI_BINARY_OBJECT_S3_SECRET_KEY are required when OPENAPI_BINARY_OBJECT_PROVIDER=s3")
	}
	return nil
}

func (s *s3BinaryObjectStore) objectPrefix(id string) string {
	return joinS3Key(s.keyPrefix, id[:2], id)
}

func (s *s3BinaryObjectStore) bodyKey(id string) string {
	return joinS3Key(s.objectPrefix(id), "body.bin")
}

func (s *s3BinaryObjectStore) metadataKey(id string) string {
	return joinS3Key(s.objectPrefix(id), "metadata.json")
}

func (s *s3BinaryObjectStore) putObject(ctx context.Context, key string, content []byte, contentType string) error {
	req, err := s.newObjectRequest(ctx, http.MethodPut, key, bytes.NewReader(content))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", contentType)
	req.ContentLength = int64(len(content))
	payloadHash := sha256Hex(content)
	if err := s.sign(ctx, req, payloadHash); err != nil {
		return err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("s3 put object %s failed with status %d: %s", key, resp.StatusCode, readS3ErrorBody(resp.Body))
	}
	return nil
}

func (s *s3BinaryObjectStore) getObject(ctx context.Context, key string) ([]byte, error) {
	req, err := s.newObjectRequest(ctx, http.MethodGet, key, nil)
	if err != nil {
		return nil, err
	}
	if err := s.sign(ctx, req, openAPIBinaryObjectEmptyPayloadSHA256); err != nil {
		return nil, err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("s3 get object %s failed with status %d: %s", key, resp.StatusCode, readS3ErrorBody(resp.Body))
	}
	return io.ReadAll(resp.Body)
}

func (s *s3BinaryObjectStore) deleteObject(ctx context.Context, key string) error {
	req, err := s.newObjectRequest(ctx, http.MethodDelete, key, nil)
	if err != nil {
		return err
	}
	if err := s.sign(ctx, req, openAPIBinaryObjectEmptyPayloadSHA256); err != nil {
		return err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("s3 delete object %s failed with status %d: %s", key, resp.StatusCode, readS3ErrorBody(resp.Body))
	}
	return nil
}

func (s *s3BinaryObjectStore) listObjects(ctx context.Context, prefix string, continuationToken string) (s3ListBucketResult, error) {
	req, err := s.newBucketRequest(ctx, http.MethodGet)
	if err != nil {
		return s3ListBucketResult{}, err
	}
	query := req.URL.Query()
	query.Set("list-type", "2")
	query.Set("prefix", prefix)
	query.Set("max-keys", strconv.Itoa(openAPIBinaryObjectS3ListMaxKeys))
	if strings.TrimSpace(continuationToken) != "" {
		query.Set("continuation-token", continuationToken)
	}
	req.URL.RawQuery = query.Encode()
	if err := s.sign(ctx, req, openAPIBinaryObjectEmptyPayloadSHA256); err != nil {
		return s3ListBucketResult{}, err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return s3ListBucketResult{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return s3ListBucketResult{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return s3ListBucketResult{}, fmt.Errorf("s3 list objects failed with status %d: %s", resp.StatusCode, truncateS3ErrorBody(string(body)))
	}
	var result s3ListBucketResult
	if err := xml.Unmarshal(body, &result); err != nil {
		return s3ListBucketResult{}, err
	}
	return result, nil
}

func (s *s3BinaryObjectStore) newObjectRequest(ctx context.Context, method string, key string, body io.Reader) (*http.Request, error) {
	requestURL, err := s.objectURL(key)
	if err != nil {
		return nil, err
	}
	return http.NewRequestWithContext(ctx, method, requestURL, body)
}

func (s *s3BinaryObjectStore) newBucketRequest(ctx context.Context, method string) (*http.Request, error) {
	requestURL, err := s.bucketURL()
	if err != nil {
		return nil, err
	}
	return http.NewRequestWithContext(ctx, method, requestURL, nil)
}

func (s *s3BinaryObjectStore) objectURL(key string) (string, error) {
	base, err := url.Parse(s.endpoint)
	if err != nil {
		return "", err
	}
	if s.pathStyle {
		base.Path = joinURLPath(base.Path, s.bucket, key)
		return base.String(), nil
	}
	base.Host = s.bucket + "." + base.Host
	base.Path = joinURLPath(base.Path, key)
	return base.String(), nil
}

func (s *s3BinaryObjectStore) bucketURL() (string, error) {
	base, err := url.Parse(s.endpoint)
	if err != nil {
		return "", err
	}
	if s.pathStyle {
		base.Path = joinURLPath(base.Path, s.bucket)
		return base.String(), nil
	}
	base.Host = s.bucket + "." + base.Host
	return base.String(), nil
}

func (s *s3BinaryObjectStore) sign(ctx context.Context, req *http.Request, payloadHash string) error {
	req.Header.Set("X-Amz-Content-Sha256", payloadHash)
	return v4.NewSigner().SignHTTP(ctx, s.credentials, req, payloadHash, openAPIBinaryObjectS3Service, s.region, time.Now())
}

func joinS3Key(parts ...string) string {
	cleaned := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.Trim(part, "/")
		if part != "" {
			cleaned = append(cleaned, part)
		}
	}
	return strings.Join(cleaned, "/")
}

func joinURLPath(base string, parts ...string) string {
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

func sha256Hex(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

func readS3ErrorBody(reader io.Reader) string {
	body, err := io.ReadAll(io.LimitReader(reader, 4096))
	if err != nil {
		return err.Error()
	}
	return truncateS3ErrorBody(string(body))
}

func truncateS3ErrorBody(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= 512 {
		return value
	}
	return value[:509] + "..."
}

func envBool(name string, fallback bool) bool {
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
