package service

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/klauspost/compress/zstd"
	"gorm.io/gorm"
)

const (
	requestCaptureBundleMasterKeyEnv = "CAPTURE_BUNDLE_MASTER_KEY"
	requestCaptureBundleKeyIdEnv     = "CAPTURE_BUNDLE_KEY_ID"
	requestCaptureBundleMagic        = "DPCE1"
	requestCaptureBundleCompression  = "zstd"
	requestCaptureBundleEncryption   = "AES-256-GCM"
	requestCaptureBundleContentType  = "application/x-data-proxy-capture-bundle"
)

type RequestCaptureFinalizeOptions struct {
	SessionDir string
}

type RequestCaptureFinalizeResult struct {
	Manifest        RequestCaptureSpoolManifest  `json:"manifest"`
	Object          RequestCaptureObject         `json:"object"`
	Artifact        model.RequestCaptureArtifact `json:"artifact"`
	TarBytes        int64                        `json:"tar_bytes"`
	CompressedBytes int64                        `json:"compressed_bytes"`
	EncryptedBytes  int64                        `json:"encrypted_bytes"`
}

type RequestCaptureFinalizerWorkerOptions struct {
	SpoolDir        string
	Limit           int
	RemoveOnSuccess bool
}

type RequestCaptureFinalizerWorkerSummary struct {
	Scanned   int      `json:"scanned"`
	Succeeded int      `json:"succeeded"`
	Failed    int      `json:"failed"`
	Skipped   int      `json:"skipped"`
	Errors    []string `json:"errors,omitempty"`
}

func FinalizeRequestCaptureSpoolSession(ctx context.Context, options RequestCaptureFinalizeOptions) (RequestCaptureFinalizeResult, error) {
	sessionDir := strings.TrimSpace(options.SessionDir)
	if sessionDir == "" {
		return RequestCaptureFinalizeResult{}, errors.New("request capture finalizer session dir is required")
	}
	manifest, err := readRequestCaptureSpoolManifest(sessionDir)
	if err != nil {
		return RequestCaptureFinalizeResult{}, err
	}
	if manifest.Status != requestCaptureSpoolStatusFinalize {
		return RequestCaptureFinalizeResult{}, fmt.Errorf("request capture finalizer expected finalize status, got %q", manifest.Status)
	}
	tarBytes, err := buildRequestCaptureTarBundle(sessionDir)
	if err != nil {
		return RequestCaptureFinalizeResult{}, err
	}
	compressedBytes, err := compressRequestCaptureBundleZstd(tarBytes)
	if err != nil {
		return RequestCaptureFinalizeResult{}, err
	}
	key, keyId, err := requestCaptureBundleMasterKeyFromEnv()
	if err != nil {
		return RequestCaptureFinalizeResult{}, err
	}
	encryptedBytes, err := encryptRequestCaptureBundle(compressedBytes, key)
	if err != nil {
		return RequestCaptureFinalizeResult{}, err
	}
	object, err := SaveRequestCaptureObject(ctx, RequestCaptureObject{
		RequestId:   manifest.RequestId,
		Kind:        model.RequestCaptureArtifactKindRawBundle,
		ContentType: requestCaptureBundleContentType,
		CreatedAt:   manifest.CreatedAt,
	}, encryptedBytes)
	if err != nil {
		return RequestCaptureFinalizeResult{}, err
	}
	manifestJson, _ := json.Marshal(manifest)
	now := common.GetTimestamp()
	artifact := model.RequestCaptureArtifact{
		RequestId:           manifest.RequestId,
		Kind:                model.RequestCaptureArtifactKindRawBundle,
		Status:              model.RequestCaptureArtifactStatusAvailable,
		Provider:            object.Provider,
		Bucket:              object.Bucket,
		StorageKey:          object.StorageKey,
		ContentType:         requestCaptureBundleContentType,
		Compression:         requestCaptureBundleCompression,
		EncryptionAlgorithm: requestCaptureBundleEncryption,
		EncryptionKeyId:     keyId,
		SHA256:              object.SHA256,
		SizeBytes:           object.BodyBytes,
		ManifestJson:        string(manifestJson),
		UploadedAt:          now,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	return RequestCaptureFinalizeResult{
		Manifest:        manifest,
		Object:          object,
		Artifact:        artifact,
		TarBytes:        int64(len(tarBytes)),
		CompressedBytes: int64(len(compressedBytes)),
		EncryptedBytes:  int64(len(encryptedBytes)),
	}, nil
}

func FinalizeAndPersistRequestCaptureSpoolSession(ctx context.Context, options RequestCaptureFinalizeOptions) (RequestCaptureFinalizeResult, error) {
	result, err := FinalizeRequestCaptureSpoolSession(ctx, options)
	if err != nil {
		return RequestCaptureFinalizeResult{}, err
	}
	artifact, err := PersistRequestCaptureFinalizeResult(ctx, result)
	if err != nil {
		return RequestCaptureFinalizeResult{}, err
	}
	result.Artifact = artifact
	return result, nil
}

func FinalizePendingRequestCaptureSpool(ctx context.Context, options RequestCaptureFinalizerWorkerOptions) (RequestCaptureFinalizerWorkerSummary, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	spoolDir := strings.TrimSpace(options.SpoolDir)
	if spoolDir == "" {
		spoolDir = requestCaptureEnvString(requestCaptureSpoolDirEnv, requestCaptureDefaultSpoolDir)
	}
	limit := options.Limit
	if limit <= 0 {
		limit = 100
	}
	finalizeRoot := filepath.Join(spoolDir, requestCaptureSpoolStatusFinalize)
	entries, err := os.ReadDir(finalizeRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return RequestCaptureFinalizerWorkerSummary{}, nil
		}
		return RequestCaptureFinalizerWorkerSummary{}, err
	}
	sort.Slice(entries, func(i int, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})
	summary := RequestCaptureFinalizerWorkerSummary{}
	for _, entry := range entries {
		if summary.Scanned >= limit {
			break
		}
		if err := ctx.Err(); err != nil {
			return summary, err
		}
		if !entry.IsDir() {
			summary.Skipped++
			continue
		}
		summary.Scanned++
		sessionDir := filepath.Join(finalizeRoot, entry.Name())
		if _, err := FinalizeAndPersistRequestCaptureSpoolSession(ctx, RequestCaptureFinalizeOptions{SessionDir: sessionDir}); err != nil {
			summary.Failed++
			summary.Errors = append(summary.Errors, fmt.Sprintf("%s: %s", entry.Name(), err.Error()))
			continue
		}
		summary.Succeeded++
		if options.RemoveOnSuccess {
			if err := os.RemoveAll(sessionDir); err != nil {
				summary.Failed++
				summary.Errors = append(summary.Errors, fmt.Sprintf("%s cleanup: %s", entry.Name(), err.Error()))
			}
		}
	}
	return summary, nil
}

func PersistRequestCaptureFinalizeResult(ctx context.Context, result RequestCaptureFinalizeResult) (model.RequestCaptureArtifact, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if model.DB == nil {
		return model.RequestCaptureArtifact{}, errors.New("database is not initialized")
	}
	if strings.TrimSpace(result.Manifest.RequestId) == "" {
		return model.RequestCaptureArtifact{}, errors.New("request capture finalize result request id is empty")
	}
	artifact := result.Artifact
	if strings.TrimSpace(artifact.RequestId) == "" {
		artifact.RequestId = result.Manifest.RequestId
	}
	if artifact.Status == "" {
		artifact.Status = model.RequestCaptureArtifactStatusAvailable
	}
	if artifact.Kind == "" {
		artifact.Kind = model.RequestCaptureArtifactKindRawBundle
	}
	now := common.GetTimestamp()
	if artifact.UploadedAt == 0 {
		artifact.UploadedAt = now
	}
	return artifact, model.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var record model.RequestCaptureRecord
		recordFound := false
		err := tx.Where("request_id = ?", result.Manifest.RequestId).Order("id desc").First(&record).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err == nil {
			recordFound = true
			artifact.CaptureId = record.Id
		}
		if err := tx.Create(&artifact).Error; err != nil {
			return err
		}
		if recordFound {
			updates := map[string]interface{}{
				"capture_status": model.RequestCaptureStatusUploaded,
				"finalized_at":   artifact.UploadedAt,
				"total_bytes":    artifact.SizeBytes,
				"last_error":     "",
			}
			if err := tx.Model(&model.RequestCaptureRecord{}).Where("id = ?", record.Id).Updates(updates).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func readRequestCaptureSpoolManifest(sessionDir string) (RequestCaptureSpoolManifest, error) {
	body, err := os.ReadFile(filepath.Join(sessionDir, requestCaptureSpoolManifestName))
	if err != nil {
		return RequestCaptureSpoolManifest{}, err
	}
	var manifest RequestCaptureSpoolManifest
	if err := json.Unmarshal(body, &manifest); err != nil {
		return RequestCaptureSpoolManifest{}, err
	}
	return manifest, nil
}

func buildRequestCaptureTarBundle(sessionDir string) ([]byte, error) {
	var buffer bytes.Buffer
	writer := tar.NewWriter(&buffer)
	err := filepath.WalkDir(sessionDir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(sessionDir, path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(rel)
		header.Mode = 0600
		if err := writer.WriteHeader(header); err != nil {
			return err
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(writer, file)
		closeErr := file.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
	if err != nil {
		_ = writer.Close()
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func compressRequestCaptureBundleZstd(body []byte) ([]byte, error) {
	var buffer bytes.Buffer
	encoder, err := zstd.NewWriter(&buffer)
	if err != nil {
		return nil, err
	}
	if _, err := encoder.Write(body); err != nil {
		encoder.Close()
		return nil, err
	}
	if err := encoder.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func encryptRequestCaptureBundle(body []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	ciphertext := gcm.Seal(nil, nonce, body, nil)
	result := make([]byte, 0, len(requestCaptureBundleMagic)+len(nonce)+len(ciphertext))
	result = append(result, requestCaptureBundleMagic...)
	result = append(result, nonce...)
	result = append(result, ciphertext...)
	return result, nil
}

func requestCaptureBundleMasterKeyFromEnv() ([]byte, string, error) {
	value := strings.TrimSpace(os.Getenv(requestCaptureBundleMasterKeyEnv))
	if value == "" {
		return nil, "", errors.New("CAPTURE_BUNDLE_MASTER_KEY is required for request capture finalizer")
	}
	key, err := decodeRequestCaptureBundleMasterKey(value)
	if err != nil {
		return nil, "", err
	}
	if len(key) != 32 {
		return nil, "", fmt.Errorf("CAPTURE_BUNDLE_MASTER_KEY must decode to 32 bytes, got %d", len(key))
	}
	keyId := strings.TrimSpace(os.Getenv(requestCaptureBundleKeyIdEnv))
	if keyId == "" {
		keyId = "env:" + requestCaptureBundleMasterKeyEnv
	}
	return key, keyId, nil
}

func decodeRequestCaptureBundleMasterKey(value string) ([]byte, error) {
	switch {
	case strings.HasPrefix(value, "base64:"):
		return base64.StdEncoding.DecodeString(strings.TrimPrefix(value, "base64:"))
	case strings.HasPrefix(value, "hex:"):
		return hex.DecodeString(strings.TrimPrefix(value, "hex:"))
	default:
		return []byte(value), nil
	}
}
