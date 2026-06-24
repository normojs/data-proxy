package dpagent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/dto"
)

type localAuditEntry struct {
	Timestamp  string         `json:"timestamp"`
	RequestID  string         `json:"request_id"`
	ToolName   string         `json:"tool_name"`
	Success    bool           `json:"success"`
	DurationMS int            `json:"duration_ms"`
	ResultSize int            `json:"result_size,omitempty"`
	ErrorCode  string         `json:"error_code,omitempty"`
	Error      string         `json:"error,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

func (c BridgeClient) auditBridgeToolCall(requestID string, toolName string, result dto.BridgeToolCallResult, callErr error, duration time.Duration) error {
	path := localAuditPath(c.Config)
	if path == "" {
		return nil
	}
	entry := localAuditEntry{
		Timestamp:  time.Now().Format(time.RFC3339Nano),
		RequestID:  strings.TrimSpace(requestID),
		ToolName:   strings.TrimSpace(toolName),
		Success:    callErr == nil,
		DurationMS: int(duration.Milliseconds()),
		ResultSize: result.ResultSize,
		Metadata:   localAuditMetadata(result.Metadata),
	}
	if callErr != nil {
		toolErr := toolErrorFromError(callErr)
		entry.ErrorCode = toolErr.Code
		entry.Error = truncateString(toolErr.Message, 512)
	}
	return appendLocalAuditEntry(path, entry)
}

func localAuditPath(cfg Config) string {
	path := strings.TrimSpace(cfg.Logging.LocalAuditJSONL)
	if path == "" {
		return ""
	}
	return expandPath(path)
}

func defaultLocalAuditPath() string {
	switch runtime.GOOS {
	case "windows":
		if base := strings.TrimSpace(os.Getenv("LOCALAPPDATA")); base != "" {
			return filepath.Join(base, "DataProxyAgent", "audit.jsonl")
		}
		if base := strings.TrimSpace(os.Getenv("APPDATA")); base != "" {
			return filepath.Join(base, "DataProxyAgent", "audit.jsonl")
		}
	case "darwin":
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			return filepath.Join(home, "Library", "Logs", "DataProxyAgent", "audit.jsonl")
		}
	default:
		if base := strings.TrimSpace(os.Getenv("XDG_STATE_HOME")); base != "" {
			return filepath.Join(base, "data-proxy-agent", "audit.jsonl")
		}
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			return filepath.Join(home, ".local", "state", "data-proxy-agent", "audit.jsonl")
		}
	}
	return "audit.jsonl"
}

func appendLocalAuditEntry(path string, entry localAuditEntry) error {
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), DefaultConfigFolderMode); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, DefaultConfigFileMode)
	if err != nil {
		return err
	}
	defer file.Close()
	bytes, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	if _, err := file.Write(append(bytes, '\n')); err != nil {
		return err
	}
	return nil
}

func localAuditMetadata(metadata map[string]any) map[string]any {
	if len(metadata) == 0 {
		return nil
	}
	allowed := map[string]bool{
		"target":         true,
		"transport":      true,
		"method":         true,
		"tool_name":      true,
		"status":         true,
		"status_code":    true,
		"http_status":    true,
		"route":          true,
		"route_name":     true,
		"event":          true,
		"server_name":    true,
		"session_key":    true,
		"command_prefix": true,
		"pid":            true,
		"exit_error":     true,
		"workdir":        true,
		"exit_code":      true,
		"timed_out":      true,
		"truncated":      true,
	}
	result := map[string]any{}
	for key, value := range metadata {
		if !allowed[key] {
			continue
		}
		if scalar, ok := localAuditScalar(value); ok {
			result[key] = scalar
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func localAuditScalar(value any) (any, bool) {
	switch typed := value.(type) {
	case nil:
		return nil, false
	case string:
		return truncateString(typed, 256), true
	case bool:
		return typed, true
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return typed, true
	default:
		return fmt.Sprint(typed), true
	}
}
