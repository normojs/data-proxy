package executor

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	mcpopenapi "github.com/QuantumNous/new-api/pkg/mcp/openapi"
	"github.com/QuantumNous/new-api/pkg/mcp/secretref"
)

const (
	defaultOpenAPITimeout       = 30 * time.Second
	defaultOpenAPIResultMaxSize = 1024 * 1024
)

type OpenAPIExecutor struct {
	Client        *http.Client
	Timeout       time.Duration
	MaxResultSize int
}

func NewOpenAPIExecutor(client *http.Client) *OpenAPIExecutor {
	if client == nil {
		client = http.DefaultClient
	}
	return &OpenAPIExecutor{
		Client:        client,
		Timeout:       defaultOpenAPITimeout,
		MaxResultSize: defaultOpenAPIResultMaxSize,
	}
}

func (e *OpenAPIExecutor) Supports(tool model.MCPTool) bool {
	return tool.Source == model.MCPToolSourceOpenAPI
}

func (e *OpenAPIExecutor) Execute(ctx context.Context, req Request) (Result, error) {
	if e == nil || e.Client == nil {
		return Result{}, &ExecutionError{
			Code:    ErrorCodeFailed,
			Message: "openapi executor is not configured",
		}
	}
	mapping, err := model.GetMCPOpenAPIToolByMCPToolId(req.Tool.Id)
	if err != nil {
		return Result{}, &ExecutionError{
			Code:    ErrorCodeFailed,
			Message: "openapi tool mapping not found",
			Err:     err,
		}
	}
	parameters, err := openAPIParameters(mapping.Parameters)
	if err != nil {
		return Result{}, &ExecutionError{
			Code:    ErrorCodeFailed,
			Message: "invalid openapi tool parameters mapping",
			Err:     err,
		}
	}
	if ctx == nil {
		ctx = context.Background()
	}
	timeout := e.Timeout
	if timeout <= 0 {
		timeout = defaultOpenAPITimeout
	}
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	startedAt := time.Now()
	httpReq, err := buildOpenAPIHTTPRequest(callCtx, *mapping, parameters, req.Arguments)
	if err != nil {
		return Result{}, &ExecutionError{
			Code:    ErrorCodeFailed,
			Message: err.Error(),
			Err:     err,
		}
	}
	httpResp, err := e.Client.Do(httpReq)
	if err != nil {
		code := ErrorCodeFailed
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(callCtx.Err(), context.DeadlineExceeded) {
			code = ErrorCodeTimeout
		}
		return Result{
				Metadata:   openAPIExecutionMetadata(*mapping, httpReq.URL.String(), 0, ""),
				DurationMS: int(time.Since(startedAt).Milliseconds()),
			}, &ExecutionError{
				Code:    code,
				Message: err.Error(),
				Err:     err,
			}
	}
	defer httpResp.Body.Close()

	maxSize := e.MaxResultSize
	if maxSize <= 0 {
		maxSize = defaultOpenAPIResultMaxSize
	}
	body, tooLarge, err := readOpenAPIResponse(httpResp.Body, maxSize)
	if err != nil {
		return Result{}, &ExecutionError{
			Code:    ErrorCodeFailed,
			Message: "failed to read openapi response",
			Err:     err,
		}
	}
	responseContentType := httpResp.Header.Get("Content-Type")
	metadata := openAPIExecutionMetadata(*mapping, httpReq.URL.String(), httpResp.StatusCode, responseContentType)
	durationMS := int(time.Since(startedAt).Milliseconds())
	if tooLarge {
		return Result{
				Metadata:   metadata,
				DurationMS: durationMS,
				ResultSize: len(body),
			}, &ExecutionError{
				Code:    ErrorCodeFailed,
				Message: "openapi response exceeds max_result_size",
			}
	}
	annotateOpenAPIResponseMetadata(metadata, body, responseContentType, req, *mapping)
	text := formatOpenAPIResponseText(body, responseContentType)
	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return Result{
				Metadata:   metadata,
				Summary:    fmt.Sprintf("%s %s returned %d", mapping.Method, mapping.Path, httpResp.StatusCode),
				DurationMS: durationMS,
				ResultSize: len(body),
			}, &ExecutionError{
				Code:    ErrorCodeFailed,
				Message: truncateOpenAPIText(text, 512),
			}
	}
	return Result{
		Content: []dto.MCPContentBlock{
			{Type: "text", Text: text},
		},
		Metadata:   metadata,
		Summary:    fmt.Sprintf("%s %s returned %d", mapping.Method, mapping.Path, httpResp.StatusCode),
		DurationMS: durationMS,
		ResultSize: len(body),
	}, nil
}

func buildOpenAPIHTTPRequest(ctx context.Context, mapping model.MCPOpenAPITool, parameters []mcpopenapi.Parameter, args map[string]any) (*http.Request, error) {
	targetURL, err := buildOpenAPIURL(mapping, parameters, args)
	if err != nil {
		return nil, err
	}
	var body io.Reader
	contentType := strings.TrimSpace(mapping.RequestContentType)
	if value, ok := args["body"]; ok && value != nil {
		requestBody, requestContentType, err := buildOpenAPIRequestBody(contentType, mapping.RequestBodySchema, value)
		if err != nil {
			return nil, err
		}
		body = requestBody
		contentType = requestContentType
	}
	method := strings.ToUpper(strings.TrimSpace(mapping.Method))
	if method == "" {
		method = http.MethodGet
	}
	req, err := http.NewRequestWithContext(ctx, method, targetURL, body)
	if err != nil {
		return nil, err
	}
	if body != nil {
		if contentType == "" {
			contentType = "application/json"
		}
		req.Header.Set("Content-Type", contentType)
	}
	for _, parameter := range parameters {
		if parameter.In != "header" {
			continue
		}
		value, ok := args[parameter.Name]
		if !ok || value == nil {
			continue
		}
		req.Header.Set(parameter.Name, fmt.Sprint(value))
	}
	if err := applyOpenAPIAuth(req, mapping); err != nil {
		return nil, err
	}
	return req, nil
}

func buildOpenAPIRequestBody(contentType string, requestBodySchema string, value any) (io.Reader, string, error) {
	contentType = strings.TrimSpace(contentType)
	if contentType == "" {
		contentType = "application/json"
	}
	mediaType := openAPIMediaType(contentType)
	switch {
	case isOpenAPIJSONMediaType(mediaType):
		bodyBytes, err := json.Marshal(value)
		if err != nil {
			return nil, "", fmt.Errorf("failed to marshal openapi body: %w", err)
		}
		return bytes.NewReader(bodyBytes), contentType, nil
	case mediaType == "application/x-www-form-urlencoded":
		values, err := openAPIFormValues(value)
		if err != nil {
			return nil, "", err
		}
		return strings.NewReader(values.Encode()), contentType, nil
	case mediaType == "multipart/form-data":
		body, multipartContentType, err := openAPIMultipartBody(requestBodySchema, value)
		if err != nil {
			return nil, "", err
		}
		return body, multipartContentType, nil
	case isOpenAPITextRequestMediaType(mediaType):
		bodyBytes, err := openAPITextRequestBody(value)
		if err != nil {
			return nil, "", err
		}
		return bytes.NewReader(bodyBytes), contentType, nil
	case isOpenAPIBinaryRequestBody(mediaType, requestBodySchema):
		bodyBytes, binaryContentType, err := openAPIBinaryRequestBody(contentType, value)
		if err != nil {
			return nil, "", err
		}
		return bytes.NewReader(bodyBytes), binaryContentType, nil
	default:
		bodyBytes, err := json.Marshal(value)
		if err != nil {
			return nil, "", fmt.Errorf("failed to marshal openapi body: %w", err)
		}
		return bytes.NewReader(bodyBytes), contentType, nil
	}
}

func openAPIMediaType(contentType string) string {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err == nil {
		return strings.ToLower(strings.TrimSpace(mediaType))
	}
	mediaType, _, _ = strings.Cut(contentType, ";")
	return strings.ToLower(strings.TrimSpace(mediaType))
}

func isOpenAPIBinaryRequestBody(mediaType string, requestBodySchema string) bool {
	if mediaType == "application/octet-stream" {
		return true
	}
	schema, err := openAPIRequestBodySchema(requestBodySchema)
	if err != nil {
		return false
	}
	return openAPISchemaIsBinary(schema)
}

func isOpenAPIJSONMediaType(mediaType string) bool {
	return mediaType == "application/json" || strings.HasSuffix(mediaType, "+json")
}

func isOpenAPITextRequestMediaType(mediaType string) bool {
	if strings.HasPrefix(mediaType, "text/") || strings.HasSuffix(mediaType, "+xml") {
		return true
	}
	switch mediaType {
	case "application/xml", "application/yaml", "application/x-yaml":
		return true
	default:
		return false
	}
}

func openAPITextRequestBody(value any) ([]byte, error) {
	switch typed := value.(type) {
	case []byte:
		return typed, nil
	case string:
		return []byte(typed), nil
	default:
		bodyBytes, err := json.Marshal(value)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal openapi text body: %w", err)
		}
		return bodyBytes, nil
	}
}

func openAPIFormValues(value any) (url.Values, error) {
	bodyMap, ok := openAPIBodyMap(value)
	if !ok {
		return nil, fmt.Errorf("openapi form body must be an object")
	}
	values := url.Values{}
	keys := make([]string, 0, len(bodyMap))
	for key := range bodyMap {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		appendOpenAPIFormValue(values, key, bodyMap[key])
	}
	return values, nil
}

func appendOpenAPIFormValue(values url.Values, key string, value any) {
	if value == nil {
		return
	}
	switch typed := value.(type) {
	case []any:
		for _, item := range typed {
			appendOpenAPIFormValue(values, key, item)
		}
	case []string:
		for _, item := range typed {
			values.Add(key, item)
		}
	case []int:
		for _, item := range typed {
			values.Add(key, fmt.Sprint(item))
		}
	case []float64:
		for _, item := range typed {
			values.Add(key, fmt.Sprint(item))
		}
	case map[string]any:
		encoded, err := json.Marshal(typed)
		if err != nil {
			values.Add(key, fmt.Sprint(value))
			return
		}
		values.Add(key, string(encoded))
	default:
		values.Add(key, fmt.Sprint(value))
	}
}

func openAPIMultipartBody(requestBodySchema string, value any) (io.Reader, string, error) {
	bodyMap, ok := openAPIBodyMap(value)
	if !ok {
		return nil, "", fmt.Errorf("openapi multipart body must be an object")
	}
	schema, err := openAPIRequestBodySchema(requestBodySchema)
	if err != nil {
		return nil, "", err
	}
	var buffer bytes.Buffer
	writer := multipart.NewWriter(&buffer)
	keys := make([]string, 0, len(bodyMap))
	for key := range bodyMap {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		fieldSchema := openAPIMultipartFieldSchema(schema, key)
		if openAPISchemaIsBinary(fieldSchema) {
			if err := writeOpenAPIMultipartFileField(writer, key, bodyMap[key]); err != nil {
				_ = writer.Close()
				return nil, "", err
			}
			continue
		}
		if openAPISchemaContainsBinary(fieldSchema) {
			_ = writer.Close()
			return nil, "", fmt.Errorf("unsupported openapi multipart complex file field: %s", key)
		}
		if err := writer.WriteField(key, openAPIFieldString(bodyMap[key])); err != nil {
			_ = writer.Close()
			return nil, "", fmt.Errorf("failed to write openapi multipart field %s: %w", key, err)
		}
	}
	if err := writer.Close(); err != nil {
		return nil, "", fmt.Errorf("failed to finalize openapi multipart body: %w", err)
	}
	return bytes.NewReader(buffer.Bytes()), writer.FormDataContentType(), nil
}

func openAPIBinaryRequestBody(contentType string, value any) ([]byte, string, error) {
	contentType = strings.TrimSpace(contentType)
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	switch typed := value.(type) {
	case []byte:
		return typed, contentType, nil
	case string:
		content, err := decodeOpenAPIBase64Content(typed)
		if err != nil {
			return nil, "", fmt.Errorf("openapi binary body has invalid base64 content: %w", err)
		}
		return content, contentType, nil
	default:
		bodyMap, ok := openAPIBodyMap(value)
		if !ok {
			return nil, "", fmt.Errorf("openapi binary body must be base64 string or object with content_base64")
		}
		contentBase64 := strings.TrimSpace(openAPIStringField(bodyMap, "content_base64"))
		if contentBase64 == "" {
			return nil, "", fmt.Errorf("openapi binary body requires content_base64")
		}
		content, err := decodeOpenAPIBase64Content(contentBase64)
		if err != nil {
			return nil, "", fmt.Errorf("openapi binary body has invalid content_base64: %w", err)
		}
		if overrideContentType := strings.TrimSpace(openAPIStringField(bodyMap, "content_type")); overrideContentType != "" {
			contentType = overrideContentType
		}
		return content, contentType, nil
	}
}

func writeOpenAPIMultipartFileField(writer *multipart.Writer, fieldName string, value any) error {
	file, err := openAPIFileUploadValue(fieldName, value)
	if err != nil {
		return err
	}
	header := textproto.MIMEHeader{}
	header.Set("Content-Disposition", mime.FormatMediaType("form-data", map[string]string{
		"name":     fieldName,
		"filename": file.Filename,
	}))
	header.Set("Content-Type", file.ContentType)
	part, err := writer.CreatePart(header)
	if err != nil {
		return fmt.Errorf("failed to create openapi multipart file field %s: %w", fieldName, err)
	}
	if _, err := part.Write(file.Content); err != nil {
		return fmt.Errorf("failed to write openapi multipart file field %s: %w", fieldName, err)
	}
	return nil
}

type openAPIFileUpload struct {
	Filename    string
	ContentType string
	Content     []byte
}

func openAPIFileUploadValue(fieldName string, value any) (openAPIFileUpload, error) {
	bodyMap, ok := openAPIBodyMap(value)
	if !ok {
		return openAPIFileUpload{}, fmt.Errorf("openapi multipart file field %s must be an object with content_base64", fieldName)
	}
	contentBase64 := strings.TrimSpace(openAPIStringField(bodyMap, "content_base64"))
	if contentBase64 == "" {
		return openAPIFileUpload{}, fmt.Errorf("openapi multipart file field %s requires content_base64", fieldName)
	}
	content, err := decodeOpenAPIBase64Content(contentBase64)
	if err != nil {
		return openAPIFileUpload{}, fmt.Errorf("openapi multipart file field %s has invalid content_base64: %w", fieldName, err)
	}
	filename := strings.TrimSpace(openAPIStringField(bodyMap, "filename"))
	if filename == "" {
		filename = fieldName
	}
	contentType := strings.TrimSpace(openAPIStringField(bodyMap, "content_type"))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	return openAPIFileUpload{
		Filename:    filename,
		ContentType: contentType,
		Content:     content,
	}, nil
}

func decodeOpenAPIBase64Content(value string) ([]byte, error) {
	if _, payload, ok := strings.Cut(value, ","); ok && strings.Contains(strings.ToLower(value[:strings.Index(value, ",")]), "base64") {
		value = payload
	}
	value = strings.TrimSpace(value)
	encodings := []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	}
	var lastErr error
	for _, encoding := range encodings {
		content, err := encoding.DecodeString(value)
		if err == nil {
			return content, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

func openAPIStringField(values map[string]any, key string) string {
	value, ok := values[key]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return fmt.Sprint(typed)
	}
}

func openAPIBodyMap(value any) (map[string]any, bool) {
	switch typed := value.(type) {
	case map[string]any:
		return typed, true
	case map[string]string:
		result := make(map[string]any, len(typed))
		for key, value := range typed {
			result[key] = value
		}
		return result, true
	default:
		return nil, false
	}
}

func openAPIRequestBodySchema(value string) (map[string]any, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return map[string]any{}, nil
	}
	var schema map[string]any
	if err := json.Unmarshal([]byte(value), &schema); err != nil {
		return nil, fmt.Errorf("invalid openapi request body schema: %w", err)
	}
	return schema, nil
}

func openAPIMultipartFieldSchema(schema map[string]any, fieldName string) map[string]any {
	if len(schema) == 0 {
		return nil
	}
	if openAPISchemaIsBinary(schema) {
		return schema
	}
	return openAPIMapValue(openAPIMapValue(schema["properties"])[fieldName])
}

func openAPISchemaContainsBinary(schema map[string]any) bool {
	if len(schema) == 0 {
		return false
	}
	if openAPISchemaIsBinary(schema) {
		return true
	}
	for _, key := range []string{"items", "additionalProperties"} {
		if openAPISchemaContainsBinary(openAPIMapValue(schema[key])) {
			return true
		}
	}
	for _, value := range openAPIMapValue(schema["properties"]) {
		if openAPISchemaContainsBinary(openAPIMapValue(value)) {
			return true
		}
	}
	for _, key := range []string{"allOf", "oneOf", "anyOf"} {
		for _, item := range openAPISliceValue(schema[key]) {
			if openAPISchemaContainsBinary(openAPIMapValue(item)) {
				return true
			}
		}
	}
	return false
}

func openAPISchemaIsBinary(schema map[string]any) bool {
	schemaType := strings.ToLower(strings.TrimSpace(fmt.Sprint(schema["type"])))
	format := strings.ToLower(strings.TrimSpace(fmt.Sprint(schema["format"])))
	return schemaType == "file" || (schemaType == "string" && (format == "binary" || format == "base64"))
}

func openAPIFieldString(value any) string {
	if value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	case []any, []string, map[string]any:
		encoded, err := json.Marshal(typed)
		if err == nil {
			return string(encoded)
		}
	}
	return fmt.Sprint(value)
}

func openAPIMapValue(value any) map[string]any {
	switch typed := value.(type) {
	case map[string]any:
		return typed
	default:
		return nil
	}
}

func openAPISliceValue(value any) []any {
	switch typed := value.(type) {
	case []any:
		return typed
	default:
		return nil
	}
}

func applyOpenAPIAuth(req *http.Request, mapping model.MCPOpenAPITool) error {
	authType := strings.ToLower(strings.TrimSpace(mapping.AuthType))
	if authType == "" || authType == model.MCPProxyAuthTypeNone {
		return nil
	}
	secret, err := secretref.ResolveEnv(mapping.AuthRef, "openapi auth")
	if err != nil {
		return err
	}
	switch authType {
	case model.MCPProxyAuthTypeBearer:
		req.Header.Set("Authorization", "Bearer "+secret)
	case model.MCPProxyAuthTypeHeader:
		headerName := strings.TrimSpace(mapping.AuthHeaderName)
		if headerName == "" {
			return fmt.Errorf("openapi auth_header_name is required")
		}
		req.Header.Set(headerName, secret)
	case model.MCPProxyAuthTypeBasic:
		username, password, ok := strings.Cut(secret, ":")
		if !ok {
			return fmt.Errorf("openapi basic auth secret must be username:password")
		}
		req.SetBasicAuth(username, password)
	default:
		return fmt.Errorf("unsupported openapi auth_type: %s", authType)
	}
	return nil
}

func buildOpenAPIURL(mapping model.MCPOpenAPITool, parameters []mcpopenapi.Parameter, args map[string]any) (string, error) {
	base := strings.TrimRight(strings.TrimSpace(mapping.ServerURL), "/")
	pathValue := strings.TrimSpace(mapping.Path)
	if base == "" {
		return "", fmt.Errorf("openapi server_url is required")
	}
	if pathValue == "" {
		return "", fmt.Errorf("openapi path is required")
	}
	for _, parameter := range parameters {
		if parameter.In != "path" {
			continue
		}
		value, ok := args[parameter.Name]
		if !ok || value == nil {
			return "", fmt.Errorf("missing required path parameter: %s", parameter.Name)
		}
		encoded := url.PathEscape(fmt.Sprint(value))
		pathValue = strings.ReplaceAll(pathValue, "{"+parameter.Name+"}", encoded)
		pathValue = strings.ReplaceAll(pathValue, ":"+parameter.Name, encoded)
	}
	target, err := url.Parse(base + "/" + strings.TrimLeft(pathValue, "/"))
	if err != nil {
		return "", err
	}
	query := target.Query()
	for _, parameter := range parameters {
		if parameter.In != "query" {
			continue
		}
		value, ok := args[parameter.Name]
		if !ok || value == nil {
			continue
		}
		query.Set(parameter.Name, fmt.Sprint(value))
	}
	target.RawQuery = query.Encode()
	return target.String(), nil
}

func openAPIParameters(value string) ([]mcpopenapi.Parameter, error) {
	if strings.TrimSpace(value) == "" {
		return []mcpopenapi.Parameter{}, nil
	}
	var parameters []mcpopenapi.Parameter
	if err := json.Unmarshal([]byte(value), &parameters); err != nil {
		return nil, err
	}
	return parameters, nil
}

func readOpenAPIResponse(reader io.Reader, maxSize int) ([]byte, bool, error) {
	limited := io.LimitReader(reader, int64(maxSize)+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, false, err
	}
	if len(body) > maxSize {
		return body[:maxSize], true, nil
	}
	return body, false, nil
}

func formatOpenAPIResponseText(body []byte, contentType string) string {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return ""
	}
	var parsed any
	if err := json.Unmarshal(trimmed, &parsed); err == nil {
		if pretty, marshalErr := json.MarshalIndent(parsed, "", "  "); marshalErr == nil {
			return string(pretty)
		}
	}
	if !isOpenAPITextResponse(contentType, trimmed) {
		sum := sha256.Sum256(body)
		return fmt.Sprintf("Binary OpenAPI response omitted. content_type=%s bytes=%d sha256=%x", strings.TrimSpace(contentType), len(body), sum)
	}
	return string(trimmed)
}

func annotateOpenAPIResponseMetadata(metadata map[string]any, body []byte, contentType string, req Request, mapping model.MCPOpenAPITool) {
	if metadata == nil || len(body) == 0 || isOpenAPITextResponse(contentType, bytes.TrimSpace(body)) {
		return
	}
	sum := sha256.Sum256(body)
	sha256Hex := fmt.Sprintf("%x", sum)
	metadata["response_binary"] = true
	metadata["response_body_sha256"] = sha256Hex
	object, err := mcpopenapi.SaveBinaryObject(body, contentType, sha256Hex)
	if err != nil {
		metadata["response_object_error"] = err.Error()
		return
	}
	expiresAt := int64(0)
	if ttl := mcpopenapi.BinaryObjectTTLSeconds(); ttl > 0 {
		expiresAt = object.CreatedAt + ttl
	}
	if err := model.CreateMCPOpenAPIBinaryObject(&model.MCPOpenAPIBinaryObject{
		ObjectId:      object.Id,
		Provider:      object.Provider,
		StorageKey:    object.StorageKey,
		ContentType:   object.ContentType,
		ContentFamily: openAPIResponseContentFamily(object.ContentType, body),
		SHA256:        object.SHA256,
		Size:          object.Size,
		Filename:      object.Filename,
		Disposition:   "attachment",
		MCPToolCallId: req.CallId,
		MCPToolId:     req.Tool.Id,
		OpenAPIToolId: mapping.Id,
		UserId:        req.UserId,
		TokenId:       req.TokenId,
		RequestId:     req.RequestId,
		OperationKey:  mapping.OperationKey,
		ExpiresAt:     expiresAt,
	}); err != nil {
		metadata["response_object_error"] = err.Error()
		return
	}
	metadata["response_object_id"] = object.Id
	metadata["response_object_size"] = object.Size
	metadata["response_object_content_type"] = object.ContentType
	metadata["response_object_filename"] = object.Filename
	metadata["response_download_url"] = mcpopenapi.BinaryObjectDownloadURL(object.Id)
}

func openAPIResponseContentFamily(contentType string, body []byte) string {
	mediaType := openAPIMediaType(contentType)
	switch {
	case isOpenAPIJSONMediaType(mediaType):
		return "json"
	case strings.HasPrefix(mediaType, "text/"):
		return "text"
	case mediaType == "application/xml" || strings.HasSuffix(mediaType, "+xml"):
		return "xml"
	case mediaType == "application/yaml" || mediaType == "application/x-yaml":
		return "yaml"
	case strings.HasPrefix(mediaType, "image/"):
		return "image"
	case strings.HasPrefix(mediaType, "audio/"):
		return "audio"
	case strings.HasPrefix(mediaType, "video/"):
		return "video"
	case mediaType == "application/pdf":
		return "pdf"
	case strings.Contains(mediaType, "zip") || strings.Contains(mediaType, "gzip") || strings.Contains(mediaType, "tar"):
		return "archive"
	case strings.Contains(mediaType, "word") || strings.Contains(mediaType, "excel") || strings.Contains(mediaType, "powerpoint") || strings.Contains(mediaType, "officedocument"):
		return "office"
	case mediaType == "" && utf8.Valid(bytes.TrimSpace(body)):
		return "text"
	default:
		return "binary"
	}
}

func isOpenAPITextResponse(contentType string, body []byte) bool {
	mediaType := openAPIMediaType(contentType)
	if mediaType == "" {
		return utf8.Valid(body)
	}
	if isOpenAPIJSONMediaType(mediaType) || strings.HasPrefix(mediaType, "text/") {
		return true
	}
	if strings.HasSuffix(mediaType, "+xml") {
		return true
	}
	switch mediaType {
	case "application/xml", "application/yaml", "application/x-yaml", "application/x-www-form-urlencoded":
		return true
	default:
		return false
	}
}

func openAPIExecutionMetadata(mapping model.MCPOpenAPITool, url string, statusCode int, contentType string) map[string]any {
	metadata := map[string]any{
		"executor":        "openapi",
		"openapi_tool_id": mapping.Id,
		"operation_id":    mapping.OperationId,
		"operation_key":   mapping.OperationKey,
		"method":          mapping.Method,
		"path":            mapping.Path,
		"url":             url,
	}
	if statusCode > 0 {
		metadata["status_code"] = statusCode
	}
	if strings.TrimSpace(contentType) != "" {
		metadata["content_type"] = contentType
	}
	if strings.TrimSpace(mapping.AuthType) != "" && mapping.AuthType != model.MCPProxyAuthTypeNone {
		metadata["auth_type"] = mapping.AuthType
	}
	return metadata
}

func truncateOpenAPIText(value string, limit int) string {
	if limit <= 0 || len(value) <= limit {
		return value
	}
	return value[:limit]
}
