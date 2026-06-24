package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/google/uuid"
)

const (
	requestCaptureSpoolStatusActive   = "active"
	requestCaptureSpoolStatusFinalize = "finalize"
	requestCaptureSpoolStatusFailed   = "failed"

	requestCaptureSpoolManifestName = "manifest.json"
	requestCaptureSpoolArtifactsDir = "artifacts"
)

type RequestCaptureSessionOptions struct {
	RequestId        string
	CaptureLevel     string
	SpoolDir         string
	ModelName        string
	RequestPath      string
	ProtocolChain    string
	IsStream         bool
	MaxArtifactBytes int64
	Now              func() time.Time
}

type RequestCaptureSpoolManifest struct {
	RequestId        string                        `json:"request_id"`
	CaptureLevel     string                        `json:"capture_level"`
	Status           string                        `json:"status"`
	ModelName        string                        `json:"model_name,omitempty"`
	RequestPath      string                        `json:"request_path,omitempty"`
	ProtocolChain    string                        `json:"protocol_chain,omitempty"`
	IsStream         bool                          `json:"is_stream"`
	MaxArtifactBytes int64                         `json:"max_artifact_bytes,omitempty"`
	Artifacts        []RequestCaptureSpoolArtifact `json:"artifacts"`
	Error            string                        `json:"error,omitempty"`
	CreatedAt        int64                         `json:"created_at"`
	UpdatedAt        int64                         `json:"updated_at"`
	FinishedAt       int64                         `json:"finished_at,omitempty"`
	Metadata         map[string]any                `json:"metadata,omitempty"`
}

type RequestCaptureSpoolArtifact struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	ContentType string `json:"content_type"`
	Bytes       int64  `json:"bytes"`
	SHA256      string `json:"sha256,omitempty"`
	Truncated   bool   `json:"truncated,omitempty"`
}

type RequestCaptureSession struct {
	mu               sync.Mutex
	spoolDir         string
	dir              string
	artifactsDir     string
	manifest         RequestCaptureSpoolManifest
	artifactIdx      map[string]int
	finalized        bool
	maxArtifactBytes int64
}

func NewRequestCaptureSession(options RequestCaptureSessionOptions) (*RequestCaptureSession, error) {
	requestId := strings.TrimSpace(options.RequestId)
	if requestId == "" {
		return nil, errors.New("request capture session request id is required")
	}
	spoolDir := strings.TrimSpace(options.SpoolDir)
	if spoolDir == "" {
		spoolDir = requestCaptureEnvString(requestCaptureSpoolDirEnv, requestCaptureDefaultSpoolDir)
	}
	now := requestCaptureSessionNow(options).Unix()
	level := strings.TrimSpace(options.CaptureLevel)
	if level == "" {
		level = model.RequestCaptureLevelMetadata
	}
	sessionDir, err := requestCaptureCreateSessionDir(spoolDir, requestId)
	if err != nil {
		return nil, err
	}
	artifactsDir := filepath.Join(sessionDir, requestCaptureSpoolArtifactsDir)
	if err := os.MkdirAll(artifactsDir, 0700); err != nil {
		_ = os.RemoveAll(sessionDir)
		return nil, err
	}
	session := &RequestCaptureSession{
		spoolDir:     spoolDir,
		dir:          sessionDir,
		artifactsDir: artifactsDir,
		manifest: RequestCaptureSpoolManifest{
			RequestId:        requestId,
			CaptureLevel:     level,
			Status:           requestCaptureSpoolStatusActive,
			ModelName:        strings.TrimSpace(options.ModelName),
			RequestPath:      strings.TrimSpace(options.RequestPath),
			ProtocolChain:    strings.TrimSpace(options.ProtocolChain),
			IsStream:         options.IsStream,
			MaxArtifactBytes: requestCapturePositiveInt64(options.MaxArtifactBytes),
			CreatedAt:        now,
			UpdatedAt:        now,
		},
		artifactIdx:      map[string]int{},
		maxArtifactBytes: requestCapturePositiveInt64(options.MaxArtifactBytes),
	}
	if err := session.writeManifestLocked(); err != nil {
		_ = os.RemoveAll(sessionDir)
		return nil, err
	}
	return session, nil
}

func (s *RequestCaptureSession) Dir() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.dir
}

func (s *RequestCaptureSession) WriteArtifact(name string, contentType string, body []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureWritableLocked(); err != nil {
		return err
	}
	artifact, err := s.ensureArtifactLocked(name, contentType)
	if err != nil {
		return err
	}
	body, truncated := requestCaptureTruncateBytes(body, s.maxArtifactBytes)
	path := filepath.Join(s.dir, artifact.Path)
	if err := os.WriteFile(path, body, 0600); err != nil {
		return err
	}
	artifact.Bytes = int64(len(body))
	artifact.SHA256 = requestCaptureObjectSHA256(body)
	artifact.Truncated = truncated
	s.setArtifactLocked(artifact)
	s.touchLocked(0)
	return s.writeManifestLocked()
}

func (s *RequestCaptureSession) AppendArtifact(name string, contentType string, chunk []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureWritableLocked(); err != nil {
		return err
	}
	artifact, err := s.ensureArtifactLocked(name, contentType)
	if err != nil {
		return err
	}
	if artifact.Truncated {
		return nil
	}
	chunk, truncated := requestCaptureChunkWithinLimit(chunk, artifact.Bytes, s.maxArtifactBytes)
	path := filepath.Join(s.dir, artifact.Path)
	if len(chunk) > 0 || artifact.Bytes == 0 {
		file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		if len(chunk) > 0 {
			if _, err := file.Write(chunk); err != nil {
				_ = file.Close()
				return err
			}
		}
		if err := file.Close(); err != nil {
			return err
		}
	}
	artifact.Bytes += int64(len(chunk))
	artifact.SHA256 = ""
	if truncated {
		artifact.Truncated = true
	}
	s.setArtifactLocked(artifact)
	s.touchLocked(0)
	return s.writeManifestLocked()
}

func (s *RequestCaptureSession) MarkArtifactTruncated(name string, contentType string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureWritableLocked(); err != nil {
		return err
	}
	artifact, err := s.ensureArtifactLocked(name, contentType)
	if err != nil {
		return err
	}
	path := filepath.Join(s.dir, artifact.Path)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	artifact.Truncated = true
	artifact.SHA256 = ""
	s.setArtifactLocked(artifact)
	s.touchLocked(0)
	return s.writeManifestLocked()
}

func (s *RequestCaptureSession) Finish() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureWritableLocked(); err != nil {
		return err
	}
	if err := s.refreshArtifactHashesLocked(); err != nil {
		return err
	}
	now := common.GetTimestamp()
	s.manifest.Status = requestCaptureSpoolStatusFinalize
	s.touchLocked(now)
	s.manifest.FinishedAt = now
	if err := s.writeManifestLocked(); err != nil {
		return err
	}
	target, err := requestCaptureMoveSessionDir(s.dir, filepath.Join(s.spoolDir, requestCaptureSpoolStatusFinalize))
	if err != nil {
		return err
	}
	s.dir = target
	s.artifactsDir = filepath.Join(target, requestCaptureSpoolArtifactsDir)
	s.finalized = true
	return nil
}

func (s *RequestCaptureSession) Fail(reason error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.finalized {
		return errors.New("request capture session is already finalized")
	}
	now := common.GetTimestamp()
	s.manifest.Status = requestCaptureSpoolStatusFailed
	if reason != nil {
		s.manifest.Error = reason.Error()
	}
	s.touchLocked(now)
	s.manifest.FinishedAt = now
	_ = s.refreshArtifactHashesLocked()
	if err := s.writeManifestLocked(); err != nil {
		return err
	}
	target, err := requestCaptureMoveSessionDir(s.dir, filepath.Join(s.spoolDir, requestCaptureSpoolStatusFailed))
	if err != nil {
		return err
	}
	s.dir = target
	s.artifactsDir = filepath.Join(target, requestCaptureSpoolArtifactsDir)
	s.finalized = true
	return nil
}

func (s *RequestCaptureSession) ensureWritableLocked() error {
	if s == nil {
		return errors.New("request capture session is nil")
	}
	if s.finalized {
		return errors.New("request capture session is finalized")
	}
	return nil
}

func (s *RequestCaptureSession) ensureArtifactLocked(name string, contentType string) (RequestCaptureSpoolArtifact, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return RequestCaptureSpoolArtifact{}, errors.New("request capture artifact name is required")
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	if idx, ok := s.artifactIdx[name]; ok {
		artifact := s.manifest.Artifacts[idx]
		if artifact.ContentType == "" {
			artifact.ContentType = contentType
		}
		return artifact, nil
	}
	fileName := requestCaptureSpoolArtifactFileName(name)
	artifact := RequestCaptureSpoolArtifact{
		Name:        name,
		Path:        filepath.ToSlash(filepath.Join(requestCaptureSpoolArtifactsDir, fileName)),
		ContentType: contentType,
	}
	s.artifactIdx[name] = len(s.manifest.Artifacts)
	s.manifest.Artifacts = append(s.manifest.Artifacts, artifact)
	return artifact, nil
}

func (s *RequestCaptureSession) setArtifactLocked(artifact RequestCaptureSpoolArtifact) {
	idx, ok := s.artifactIdx[artifact.Name]
	if !ok {
		s.artifactIdx[artifact.Name] = len(s.manifest.Artifacts)
		s.manifest.Artifacts = append(s.manifest.Artifacts, artifact)
		return
	}
	s.manifest.Artifacts[idx] = artifact
}

func (s *RequestCaptureSession) refreshArtifactHashesLocked() error {
	for i := range s.manifest.Artifacts {
		artifact := &s.manifest.Artifacts[i]
		path := filepath.Join(s.dir, artifact.Path)
		size, sha256Value, err := requestCaptureFileSHA256(path)
		if err != nil {
			return err
		}
		artifact.Bytes = size
		artifact.SHA256 = sha256Value
	}
	return nil
}

func (s *RequestCaptureSession) touchLocked(now int64) {
	if now == 0 {
		now = common.GetTimestamp()
	}
	s.manifest.UpdatedAt = now
}

func requestCapturePositiveInt64(value int64) int64 {
	if value < 0 {
		return 0
	}
	return value
}

func requestCaptureTruncateBytes(body []byte, maxBytes int64) ([]byte, bool) {
	if maxBytes <= 0 || int64(len(body)) <= maxBytes {
		return body, false
	}
	return body[:maxBytes], true
}

func requestCaptureChunkWithinLimit(chunk []byte, currentBytes int64, maxBytes int64) ([]byte, bool) {
	if maxBytes <= 0 {
		return chunk, false
	}
	if currentBytes >= maxBytes {
		return nil, len(chunk) > 0
	}
	remaining := maxBytes - currentBytes
	if int64(len(chunk)) <= remaining {
		return chunk, false
	}
	return chunk[:remaining], true
}

func (s *RequestCaptureSession) writeManifestLocked() error {
	manifestBytes, err := json.MarshalIndent(s.manifest, "", "  ")
	if err != nil {
		return err
	}
	return requestCaptureWriteFileAtomic(filepath.Join(s.dir, requestCaptureSpoolManifestName), manifestBytes, 0600)
}

func requestCaptureSessionNow(options RequestCaptureSessionOptions) time.Time {
	if options.Now != nil {
		return options.Now()
	}
	return time.Now()
}

func requestCaptureCreateSessionDir(spoolDir string, requestId string) (string, error) {
	activeRoot := filepath.Join(spoolDir, requestCaptureSpoolStatusActive)
	if err := os.MkdirAll(activeRoot, 0700); err != nil {
		return "", err
	}
	baseName := requestCaptureSanitizeKeySegment(requestId)
	for i := 0; i < 10; i++ {
		name := baseName
		if i > 0 {
			name = fmt.Sprintf("%s-%s", baseName, uuid.NewString()[:8])
		}
		dir := filepath.Join(activeRoot, name)
		if err := os.Mkdir(dir, 0700); err == nil {
			return dir, nil
		} else if !os.IsExist(err) {
			return "", err
		}
	}
	return "", fmt.Errorf("request capture session directory already exists for request id %q", requestId)
}

func requestCaptureMoveSessionDir(currentDir string, targetRoot string) (string, error) {
	if err := os.MkdirAll(targetRoot, 0700); err != nil {
		return "", err
	}
	target := filepath.Join(targetRoot, filepath.Base(currentDir))
	if _, err := os.Stat(target); err == nil {
		target = filepath.Join(targetRoot, filepath.Base(currentDir)+"-"+uuid.NewString()[:8])
	} else if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	if err := os.Rename(currentDir, target); err != nil {
		return "", err
	}
	return target, nil
}

func requestCaptureSpoolArtifactFileName(name string) string {
	name = strings.Trim(strings.TrimSpace(name), "/")
	name = strings.ReplaceAll(name, string(os.PathSeparator), "_")
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")
	return requestCaptureSanitizeKeySegment(name)
}

func requestCaptureFileSHA256(path string) (int64, string, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, "", err
	}
	defer file.Close()
	hash := sha256.New()
	size, err := io.Copy(hash, file)
	if err != nil {
		return 0, "", err
	}
	return size, hex.EncodeToString(hash.Sum(nil)), nil
}

func requestCaptureWriteFileAtomic(path string, body []byte, mode os.FileMode) error {
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, body, mode); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
