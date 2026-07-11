package executor

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"hash"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
)

const (
	BuiltinToolServerTime = "server_time"
	BuiltinToolJSONPretty = "json_pretty"
	BuiltinToolTextHash   = "text_hash"
	BuiltinToolBase64     = "base64_codec"
	BuiltinToolURLCodec   = "url_codec"
	BuiltinToolTextStats  = "text_stats"
	BuiltinToolJSONQuery  = "json_query"
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
	case BuiltinToolServerTime, BuiltinToolJSONPretty, BuiltinToolTextHash, BuiltinToolBase64, BuiltinToolURLCodec, BuiltinToolTextStats, BuiltinToolJSONQuery:
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
	case BuiltinToolJSONQuery:
		return e.jsonQuery(req, startedAt)
	case BuiltinToolTextHash:
		return e.textHash(req, startedAt)
	case BuiltinToolBase64:
		return e.base64Codec(req, startedAt)
	case BuiltinToolURLCodec:
		return e.urlCodec(req, startedAt)
	case BuiltinToolTextStats:
		return e.textStats(req, startedAt)
	default:
		return Result{}, &ExecutionError{
			Code:    ErrorCodeUnsupported,
			Message: fmt.Sprintf("built-in tool %q is not supported by the local executor", req.Tool.Name),
			Err:     ErrExecutorUnsupported,
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

func (BuiltinExecutor) jsonQuery(req Request, startedAt time.Time) (Result, error) {
	raw := stringArg(req.Arguments, "json")
	if strings.TrimSpace(raw) == "" {
		return Result{}, &ExecutionError{
			Code:    ErrorCodeFailed,
			Message: "json argument is required",
		}
	}
	pointer := stringArg(req.Arguments, "pointer")
	if pointer == "" {
		pointer = stringArg(req.Arguments, "path")
	}
	var value any
	if err := common.UnmarshalJsonStr(raw, &value); err != nil {
		return Result{}, &ExecutionError{
			Code:    ErrorCodeFailed,
			Message: "invalid json",
			Err:     err,
		}
	}
	selected, exists, err := resolveJSONPointer(value, pointer)
	if err != nil {
		return Result{}, &ExecutionError{
			Code:    ErrorCodeFailed,
			Message: "invalid json pointer",
			Err:     err,
		}
	}
	valueType := "missing"
	payload := map[string]any{
		"pointer": pointer,
		"exists":  exists,
	}
	if exists {
		valueType = jsonValueType(selected)
		payload["type"] = valueType
		payload["value"] = selected
	} else {
		payload["type"] = valueType
	}
	data, err := common.MarshalIndent(payload, "", "  ")
	if err != nil {
		return Result{}, err
	}
	return Result{
		Content: []dto.MCPContentBlock{{Type: "text", Text: string(data)}},
		Metadata: map[string]any{
			"executor": "builtin",
			"tool":     BuiltinToolJSONQuery,
			"pointer":  pointer,
			"exists":   exists,
		},
		Summary:    fmt.Sprintf("json pointer %s (%s)", displayJSONPointer(pointer), valueType),
		DurationMS: int(time.Since(startedAt).Milliseconds()),
		ResultSize: len(data),
	}, nil
}

func resolveJSONPointer(value any, pointer string) (any, bool, error) {
	if pointer == "" || pointer == "$" {
		return value, true, nil
	}
	segments, err := jsonPointerSegments(pointer)
	if err != nil {
		return nil, false, err
	}
	current := value
	for _, segment := range segments {
		switch typed := current.(type) {
		case map[string]any:
			next, ok := typed[segment]
			if !ok {
				return nil, false, nil
			}
			current = next
		case []any:
			if segment == "-" {
				return nil, false, nil
			}
			index, err := strconv.Atoi(segment)
			if err != nil || index < 0 {
				return nil, false, fmt.Errorf("array pointer token %q must be a non-negative integer", segment)
			}
			if index >= len(typed) {
				return nil, false, nil
			}
			current = typed[index]
		default:
			return nil, false, nil
		}
	}
	return current, true, nil
}

func jsonPointerSegments(pointer string) ([]string, error) {
	if !strings.HasPrefix(pointer, "/") {
		return nil, fmt.Errorf("pointer must be empty, $, or start with /")
	}
	rawSegments := strings.Split(pointer[1:], "/")
	segments := make([]string, 0, len(rawSegments))
	for _, segment := range rawSegments {
		unescaped, err := unescapeJSONPointerSegment(segment)
		if err != nil {
			return nil, err
		}
		segments = append(segments, unescaped)
	}
	return segments, nil
}

func unescapeJSONPointerSegment(segment string) (string, error) {
	var builder strings.Builder
	for i := 0; i < len(segment); i++ {
		if segment[i] != '~' {
			builder.WriteByte(segment[i])
			continue
		}
		if i+1 >= len(segment) {
			return "", fmt.Errorf("invalid escape in pointer token %q", segment)
		}
		switch segment[i+1] {
		case '0':
			builder.WriteByte('~')
		case '1':
			builder.WriteByte('/')
		default:
			return "", fmt.Errorf("invalid escape in pointer token %q", segment)
		}
		i++
	}
	return builder.String(), nil
}

func jsonValueType(value any) string {
	switch value.(type) {
	case nil:
		return "null"
	case bool:
		return "boolean"
	case float64:
		return "number"
	case string:
		return "string"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	default:
		return fmt.Sprintf("%T", value)
	}
}

func displayJSONPointer(pointer string) string {
	if pointer == "" {
		return "$"
	}
	return pointer
}

func (BuiltinExecutor) textStats(req Request, startedAt time.Time) (Result, error) {
	text, ok := requiredStringArg(req.Arguments, "text")
	if !ok {
		return Result{}, &ExecutionError{
			Code:    ErrorCodeFailed,
			Message: "text argument is required",
		}
	}
	analyzed := text
	trim := boolArg(req.Arguments, "trim")
	normalizeSpaces := boolArg(req.Arguments, "normalize_spaces")
	if trim {
		analyzed = strings.TrimSpace(analyzed)
	}
	if normalizeSpaces {
		analyzed = strings.Join(strings.Fields(analyzed), " ")
	}
	bytesCount := len([]byte(analyzed))
	runesCount := utf8.RuneCountInString(analyzed)
	lineCount := countTextLines(analyzed)
	wordCount := len(strings.Fields(analyzed))
	payload := map[string]any{
		"bytes":             bytesCount,
		"runes":             runesCount,
		"lines":             lineCount,
		"words":             wordCount,
		"empty":             strings.TrimSpace(analyzed) == "",
		"trimmed":           trim,
		"normalized_spaces": normalizeSpaces,
		"original_bytes":    len([]byte(text)),
		"original_runes":    utf8.RuneCountInString(text),
	}
	data, err := common.MarshalIndent(payload, "", "  ")
	if err != nil {
		return Result{}, err
	}
	return Result{
		Content: []dto.MCPContentBlock{{Type: "text", Text: string(data)}},
		Metadata: map[string]any{
			"executor": "builtin",
			"tool":     BuiltinToolTextStats,
			"trimmed":  trim,
		},
		Summary:    fmt.Sprintf("text stats (%d words, %d lines)", wordCount, lineCount),
		DurationMS: int(time.Since(startedAt).Milliseconds()),
		ResultSize: len(data),
	}, nil
}

func countTextLines(text string) int {
	if text == "" {
		return 0
	}
	lines := strings.Count(text, "\n")
	if !strings.HasSuffix(text, "\n") {
		lines++
	}
	return lines
}

func (BuiltinExecutor) urlCodec(req Request, startedAt time.Time) (Result, error) {
	operation := normalizeURLCodecOperation(stringArg(req.Arguments, "operation"))
	if operation == "" {
		operation = normalizeURLCodecOperation(stringArg(req.Arguments, "action"))
	}
	if operation == "" {
		operation = "encode"
	}
	mode := normalizeURLCodecMode(stringArg(req.Arguments, "mode"))
	if mode == "" {
		mode = "query"
	}
	input, ok := requiredStringArg(req.Arguments, "input")
	if !ok {
		return Result{}, &ExecutionError{
			Code:    ErrorCodeFailed,
			Message: "input argument is required",
		}
	}
	payload := map[string]any{
		"operation": operation,
		"mode":      mode,
	}
	var output string
	var err error
	switch operation {
	case "encode":
		if mode == "path" {
			output = url.PathEscape(input)
		} else {
			output = url.QueryEscape(input)
		}
		payload["encoded"] = output
		payload["input_bytes"] = len([]byte(input))
		payload["output_bytes"] = len([]byte(output))
	case "decode":
		if mode == "path" {
			output, err = url.PathUnescape(input)
		} else {
			output, err = url.QueryUnescape(input)
		}
		if err != nil {
			return Result{}, &ExecutionError{
				Code:    ErrorCodeFailed,
				Message: "invalid url-encoded input",
				Err:     err,
			}
		}
		payload["decoded"] = output
		payload["input_bytes"] = len([]byte(input))
		payload["output_bytes"] = len([]byte(output))
	default:
		return Result{}, &ExecutionError{
			Code:    ErrorCodeFailed,
			Message: "operation must be encode or decode",
		}
	}
	data, err := common.MarshalIndent(payload, "", "  ")
	if err != nil {
		return Result{}, err
	}
	return Result{
		Content: []dto.MCPContentBlock{{Type: "text", Text: string(data)}},
		Metadata: map[string]any{
			"executor":  "builtin",
			"tool":      BuiltinToolURLCodec,
			"operation": operation,
			"mode":      mode,
		},
		Summary:    fmt.Sprintf("url %s %s (%d bytes)", mode, operation, len([]byte(output))),
		DurationMS: int(time.Since(startedAt).Milliseconds()),
		ResultSize: len(data),
	}, nil
}

func normalizeURLCodecOperation(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "enc", "encode", "escape":
		return "encode"
	case "dec", "decode", "unescape":
		return "decode"
	default:
		return strings.ToLower(strings.TrimSpace(value))
	}
}

func normalizeURLCodecMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "query", "querystring", "form":
		return "query"
	case "path", "segment":
		return "path"
	default:
		return strings.ToLower(strings.TrimSpace(value))
	}
}

func (BuiltinExecutor) base64Codec(req Request, startedAt time.Time) (Result, error) {
	operation := normalizeBase64Operation(stringArg(req.Arguments, "operation"))
	if operation == "" {
		operation = normalizeBase64Operation(stringArg(req.Arguments, "action"))
	}
	if operation == "" {
		operation = "encode"
	}
	input, ok := requiredStringArg(req.Arguments, "input")
	if !ok {
		return Result{}, &ExecutionError{
			Code:    ErrorCodeFailed,
			Message: "input argument is required",
		}
	}
	encodingName, encoding := base64Encoding(boolArg(req.Arguments, "url_safe"), boolArg(req.Arguments, "raw"))
	payload := map[string]any{
		"operation": operation,
		"encoding":  encodingName,
	}
	var resultSize int
	switch operation {
	case "encode":
		inputBytes := []byte(input)
		encoded := encoding.EncodeToString(inputBytes)
		payload["base64"] = encoded
		payload["input_bytes"] = len(inputBytes)
		payload["output_bytes"] = len([]byte(encoded))
		resultSize = len(encoded)
	case "decode":
		decoded, err := encoding.DecodeString(strings.TrimSpace(input))
		if err != nil {
			return Result{}, &ExecutionError{
				Code:    ErrorCodeFailed,
				Message: "invalid base64 input",
				Err:     err,
			}
		}
		payload["data_base64"] = base64.StdEncoding.EncodeToString(decoded)
		payload["bytes"] = len(decoded)
		payload["utf8"] = utf8.Valid(decoded)
		if utf8.Valid(decoded) {
			payload["text"] = string(decoded)
		}
		resultSize = len(decoded)
	default:
		return Result{}, &ExecutionError{
			Code:    ErrorCodeFailed,
			Message: "operation must be encode or decode",
		}
	}
	data, err := common.MarshalIndent(payload, "", "  ")
	if err != nil {
		return Result{}, err
	}
	return Result{
		Content: []dto.MCPContentBlock{{Type: "text", Text: string(data)}},
		Metadata: map[string]any{
			"executor":  "builtin",
			"tool":      BuiltinToolBase64,
			"operation": operation,
			"encoding":  encodingName,
		},
		Summary:    fmt.Sprintf("base64 %s (%d bytes)", operation, resultSize),
		DurationMS: int(time.Since(startedAt).Milliseconds()),
		ResultSize: len(data),
	}, nil
}

func normalizeBase64Operation(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "enc", "encode":
		return "encode"
	case "dec", "decode":
		return "decode"
	default:
		return strings.ToLower(strings.TrimSpace(value))
	}
}

func base64Encoding(urlSafe bool, raw bool) (string, *base64.Encoding) {
	switch {
	case urlSafe && raw:
		return "base64url_raw", base64.RawURLEncoding
	case urlSafe:
		return "base64url", base64.URLEncoding
	case raw:
		return "base64_raw", base64.RawStdEncoding
	default:
		return "base64", base64.StdEncoding
	}
}

func (BuiltinExecutor) textHash(req Request, startedAt time.Time) (Result, error) {
	text, ok := requiredStringArg(req.Arguments, "text")
	if !ok {
		return Result{}, &ExecutionError{
			Code:    ErrorCodeFailed,
			Message: "text argument is required",
		}
	}
	algorithm := normalizeHashAlgorithm(stringArg(req.Arguments, "algorithm"))
	if algorithm == "" {
		algorithm = "sha256"
	}
	hasherFactory, ok := builtinHashAlgorithms()[algorithm]
	if !ok {
		return Result{}, &ExecutionError{
			Code:    ErrorCodeFailed,
			Message: "algorithm must be one of: sha256, sha1, sha512, md5",
		}
	}
	hasher := hasherFactory()
	_, _ = hasher.Write([]byte(text))
	sum := hasher.Sum(nil)
	payload := map[string]any{
		"algorithm": algorithm,
		"hex":       hex.EncodeToString(sum),
		"base64":    base64.StdEncoding.EncodeToString(sum),
		"bytes":     len([]byte(text)),
	}
	data, err := common.MarshalIndent(payload, "", "  ")
	if err != nil {
		return Result{}, err
	}
	return Result{
		Content: []dto.MCPContentBlock{{Type: "text", Text: string(data)}},
		Metadata: map[string]any{
			"executor":  "builtin",
			"tool":      BuiltinToolTextHash,
			"algorithm": algorithm,
		},
		Summary:    fmt.Sprintf("%s hash (%d bytes)", algorithm, len([]byte(text))),
		DurationMS: int(time.Since(startedAt).Milliseconds()),
		ResultSize: len(data),
	}, nil
}

func builtinHashAlgorithms() map[string]func() hash.Hash {
	return map[string]func() hash.Hash{
		"sha256": sha256.New,
		"sha1":   sha1.New,
		"sha512": sha512.New,
		"md5":    md5.New,
	}
}

func normalizeHashAlgorithm(value string) string {
	return strings.ReplaceAll(strings.ReplaceAll(strings.ToLower(strings.TrimSpace(value)), "-", ""), "_", "")
}

func requiredStringArg(args map[string]any, key string) (string, bool) {
	if args == nil {
		return "", false
	}
	value, ok := args[key]
	if !ok || value == nil {
		return "", false
	}
	switch typed := value.(type) {
	case string:
		return typed, true
	default:
		return fmt.Sprint(typed), true
	}
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

func boolArg(args map[string]any, key string) bool {
	if args == nil {
		return false
	}
	value, ok := args[key]
	if !ok || value == nil {
		return false
	}
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		switch strings.ToLower(strings.TrimSpace(typed)) {
		case "1", "true", "yes", "y", "on":
			return true
		default:
			return false
		}
	case int:
		return typed != 0
	case int64:
		return typed != 0
	case float64:
		return typed != 0
	default:
		return false
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
