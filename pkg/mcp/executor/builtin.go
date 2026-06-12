package executor

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
)

const (
	BuiltinToolServerTime = "server_time"
	BuiltinToolJSONPretty = "json_pretty"
)

type BuiltinExecutor struct{}

func NewBuiltinExecutor() BuiltinExecutor {
	return BuiltinExecutor{}
}

func (BuiltinExecutor) Supports(tool model.MCPTool) bool {
	if tool.Source != model.MCPToolSourceBuiltin || tool.IsRemote {
		return false
	}
	switch tool.Name {
	case BuiltinToolServerTime, BuiltinToolJSONPretty:
		return true
	default:
		return false
	}
}

func (e BuiltinExecutor) Execute(ctx context.Context, req Request) (Result, error) {
	startedAt := time.Now()
	switch req.Tool.Name {
	case BuiltinToolServerTime:
		return e.serverTime(req, startedAt)
	case BuiltinToolJSONPretty:
		return e.jsonPretty(req, startedAt)
	default:
		return Result{}, &ExecutionError{
			Code:    ErrorCodeNotImplemented,
			Message: "built-in tool is not implemented",
			Err:     ErrExecutorNotImplemented,
		}
	}
}

func (BuiltinExecutor) serverTime(req Request, startedAt time.Time) (Result, error) {
	timezone := stringArg(req.Arguments, "timezone")
	location := time.Local
	if strings.TrimSpace(timezone) != "" {
		loaded, err := time.LoadLocation(timezone)
		if err != nil {
			return Result{}, &ExecutionError{
				Code:    ErrorCodeFailed,
				Message: fmt.Sprintf("invalid timezone: %s", timezone),
				Err:     err,
			}
		}
		location = loaded
	}
	now := time.Now().In(location)
	payload := map[string]any{
		"unix":     now.Unix(),
		"rfc3339":  now.Format(time.RFC3339Nano),
		"timezone": now.Location().String(),
	}
	data, err := common.MarshalIndent(payload, "", "  ")
	if err != nil {
		return Result{}, err
	}
	text := string(data)
	return Result{
		Content: []dto.MCPContentBlock{{Type: "text", Text: text}},
		Metadata: map[string]any{
			"executor": "builtin",
			"tool":     BuiltinToolServerTime,
			"timezone": now.Location().String(),
		},
		Summary:    now.Format(time.RFC3339),
		DurationMS: int(time.Since(startedAt).Milliseconds()),
		ResultSize: len(data),
	}, nil
}

func (BuiltinExecutor) jsonPretty(req Request, startedAt time.Time) (Result, error) {
	raw := stringArg(req.Arguments, "json")
	if strings.TrimSpace(raw) == "" {
		return Result{}, &ExecutionError{
			Code:    ErrorCodeFailed,
			Message: "json argument is required",
		}
	}
	indent := intArg(req.Arguments, "indent", 2)
	if indent < 0 || indent > 8 {
		return Result{}, &ExecutionError{
			Code:    ErrorCodeFailed,
			Message: "indent must be between 0 and 8",
		}
	}
	var value any
	if err := common.UnmarshalJsonStr(raw, &value); err != nil {
		return Result{}, &ExecutionError{
			Code:    ErrorCodeFailed,
			Message: "invalid json",
			Err:     err,
		}
	}
	var output []byte
	var err error
	if indent == 0 {
		output, err = common.Marshal(value)
	} else {
		output, err = common.MarshalIndent(value, "", strings.Repeat(" ", indent))
	}
	if err != nil {
		return Result{}, err
	}
	normalized := bytes.TrimSpace(output)
	return Result{
		Content: []dto.MCPContentBlock{{Type: "text", Text: string(normalized)}},
		Metadata: map[string]any{
			"executor": "builtin",
			"tool":     BuiltinToolJSONPretty,
			"indent":   indent,
		},
		Summary:    fmt.Sprintf("formatted JSON (%d bytes)", len(normalized)),
		DurationMS: int(time.Since(startedAt).Milliseconds()),
		ResultSize: len(normalized),
	}, nil
}

func stringArg(args map[string]any, key string) string {
	if args == nil {
		return ""
	}
	value, ok := args[key]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}

func intArg(args map[string]any, key string, fallback int) int {
	if args == nil {
		return fallback
	}
	value, ok := args[key]
	if !ok || value == nil {
		return fallback
	}
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case interface{ Int64() (int64, error) }:
		parsed, err := typed.Int64()
		if err == nil {
			return int(parsed)
		}
	case string:
		var parsed int
		if _, err := fmt.Sscanf(strings.TrimSpace(typed), "%d", &parsed); err == nil {
			return parsed
		}
	}
	return fallback
}
