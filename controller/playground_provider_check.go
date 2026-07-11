package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/system_setting"

	"github.com/gin-gonic/gin"
)

const (
	playgroundProviderCheckDefaultTimeoutSeconds = 20
	playgroundProviderCheckMaxTimeoutSeconds     = 60
	playgroundProviderCheckMaxResponseBytes      = 128 * 1024
	playgroundProviderCheckPreviewBytes          = 16 * 1024
)

type playgroundProviderCheckRequest struct {
	BaseURL        string `json:"base_url"`
	Key            string `json:"key"`
	Model          string `json:"model"`
	Prompt         string `json:"prompt"`
	TimeoutSeconds int    `json:"timeout_seconds"`
}

type playgroundProviderCheckResult struct {
	OK                bool           `json:"ok"`
	Endpoint          string         `json:"endpoint"`
	StatusCode        int            `json:"status_code,omitempty"`
	Status            string         `json:"status,omitempty"`
	DurationMs        int64          `json:"duration_ms"`
	ResponseID        string         `json:"response_id,omitempty"`
	ResponseModel     string         `json:"response_model,omitempty"`
	OutputPreview     string         `json:"output_preview,omitempty"`
	ErrorMessage      string         `json:"error_message,omitempty"`
	ErrorCode         string         `json:"error_code,omitempty"`
	ResponsePreview   string         `json:"response_preview,omitempty"`
	ResponseTruncated bool           `json:"response_truncated,omitempty"`
	RequestBody       map[string]any `json:"request_body"`
}

func CheckPlaygroundProvider(c *gin.Context) {
	var req playgroundProviderCheckRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}

	result, err := runPlaygroundProviderCheck(c.Request.Context(), req)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

func runPlaygroundProviderCheck(ctx context.Context, req playgroundProviderCheckRequest) (*playgroundProviderCheckResult, error) {
	normalized, err := normalizePlaygroundProviderBaseURL(req.BaseURL)
	if err != nil {
		return nil, err
	}
	key := strings.TrimSpace(req.Key)
	model := strings.TrimSpace(req.Model)
	if key == "" {
		return nil, errors.New("key is required")
	}
	if model == "" {
		return nil, errors.New("model is required")
	}
	if err := validatePlaygroundProviderEndpointURL(normalized); err != nil {
		return nil, fmt.Errorf("base URL blocked by server-side URL policy: %w", err)
	}

	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		prompt = "Reply exactly with OK."
	}
	payload := playgroundProviderCheckPayload(model, prompt)
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	timeoutSeconds := req.TimeoutSeconds
	if timeoutSeconds <= 0 {
		timeoutSeconds = playgroundProviderCheckDefaultTimeoutSeconds
	}
	if timeoutSeconds > playgroundProviderCheckMaxTimeoutSeconds {
		timeoutSeconds = playgroundProviderCheckMaxTimeoutSeconds
	}

	startedAt := time.Now()
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, normalized, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", normalizePlaygroundProviderBearerKey(key))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("User-Agent", "data-proxy-playground-provider-check/1.0")

	client := *service.GetHttpClientWithTimeout(time.Duration(timeoutSeconds) * time.Second)
	client.CheckRedirect = playgroundProviderCheckRedirect
	resp, err := client.Do(httpReq)
	durationMs := time.Since(startedAt).Milliseconds()
	result := &playgroundProviderCheckResult{
		OK:          false,
		Endpoint:    normalized,
		DurationMs:  durationMs,
		RequestBody: payload,
	}
	if err != nil {
		result.ErrorMessage = err.Error()
		return result, nil
	}
	defer resp.Body.Close()

	result.StatusCode = resp.StatusCode
	result.Status = resp.Status

	responseBody, truncated, err := readPlaygroundProviderCheckBody(resp.Body)
	if err != nil {
		result.ErrorMessage = err.Error()
		return result, nil
	}
	result.ResponseTruncated = truncated
	result.ResponsePreview = redactPlaygroundProviderCheckSecret(
		string(trimPlaygroundProviderCheckPreview(responseBody)),
		key,
	)

	var decoded any
	if len(responseBody) > 0 && json.Unmarshal(responseBody, &decoded) == nil {
		result.ResponseID = stringFromMap(decoded, "id")
		result.ResponseModel = stringFromMap(decoded, "model")
		result.OutputPreview = redactPlaygroundProviderCheckSecret(
			playgroundProviderCheckOutputPreview(decoded),
			key,
		)
		result.ErrorMessage, result.ErrorCode = playgroundProviderCheckError(decoded)
	}

	if result.ErrorMessage == "" && (resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices) {
		result.ErrorMessage = resp.Status
	}
	result.OK = resp.StatusCode >= http.StatusOK &&
		resp.StatusCode < http.StatusMultipleChoices &&
		result.ErrorMessage == ""
	return result, nil
}

func playgroundProviderCheckPayload(model string, prompt string) map[string]any {
	return map[string]any{
		"model": model,
		"messages": []map[string]string{
			{
				"role":    "user",
				"content": prompt,
			},
		},
		"temperature": 0,
		"max_tokens":  8,
		"stream":      false,
	}
}

func normalizePlaygroundProviderBaseURL(rawBaseURL string) (string, error) {
	raw := strings.TrimSpace(rawBaseURL)
	if raw == "" {
		return "", errors.New("base_url is required")
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid base_url: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("unsupported base_url scheme: %s", parsed.Scheme)
	}
	if parsed.Host == "" {
		return "", errors.New("base_url host is required")
	}

	cleanPath := strings.TrimRight(parsed.Path, "/")
	switch {
	case cleanPath == "":
		parsed.Path = "/v1/chat/completions"
	case strings.HasSuffix(cleanPath, "/chat/completions"):
		parsed.Path = cleanPath
	case strings.HasSuffix(cleanPath, "/v1"):
		parsed.Path = cleanPath + "/chat/completions"
	default:
		parsed.Path = cleanPath + "/v1/chat/completions"
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}

func validatePlaygroundProviderEndpointURL(endpoint string) error {
	fetchSetting := system_setting.GetFetchSetting()
	return common.ValidateURLWithFetchSetting(
		endpoint,
		fetchSetting.EnableSSRFProtection,
		fetchSetting.AllowPrivateIp,
		fetchSetting.DomainFilterMode,
		fetchSetting.IpFilterMode,
		fetchSetting.DomainList,
		fetchSetting.IpList,
		fetchSetting.AllowedPorts,
		fetchSetting.ApplyIPFilterForDomain,
	)
}

func playgroundProviderCheckRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 5 {
		return errors.New("stopped after 5 redirects")
	}
	if err := validatePlaygroundProviderEndpointURL(req.URL.String()); err != nil {
		return fmt.Errorf("redirect to %s blocked: %w", req.URL.String(), err)
	}
	return nil
}

func normalizePlaygroundProviderBearerKey(key string) string {
	trimmed := strings.TrimSpace(key)
	if strings.HasPrefix(strings.ToLower(trimmed), "bearer ") {
		return trimmed
	}
	return "Bearer " + trimmed
}

func readPlaygroundProviderCheckBody(body io.Reader) ([]byte, bool, error) {
	limited, err := io.ReadAll(io.LimitReader(body, playgroundProviderCheckMaxResponseBytes+1))
	if err != nil {
		return nil, false, err
	}
	if len(limited) <= playgroundProviderCheckMaxResponseBytes {
		return limited, false, nil
	}
	return limited[:playgroundProviderCheckMaxResponseBytes], true, nil
}

func trimPlaygroundProviderCheckPreview(body []byte) []byte {
	if len(body) <= playgroundProviderCheckPreviewBytes {
		return body
	}
	return body[:playgroundProviderCheckPreviewBytes]
}

func playgroundProviderCheckError(decoded any) (string, string) {
	root, ok := decoded.(map[string]any)
	if !ok {
		return "", ""
	}
	errorValue, ok := root["error"].(map[string]any)
	if !ok {
		return "", ""
	}
	message, _ := errorValue["message"].(string)
	code, _ := errorValue["code"].(string)
	if code == "" {
		code, _ = errorValue["type"].(string)
	}
	return message, code
}

func playgroundProviderCheckOutputPreview(decoded any) string {
	root, ok := decoded.(map[string]any)
	if !ok {
		return ""
	}
	choices, ok := root["choices"].([]any)
	if !ok || len(choices) == 0 {
		return ""
	}
	firstChoice, ok := choices[0].(map[string]any)
	if !ok {
		return ""
	}
	message, ok := firstChoice["message"].(map[string]any)
	if !ok {
		return ""
	}
	content := message["content"]
	switch value := content.(type) {
	case string:
		return value
	case nil:
		return ""
	default:
		encoded, err := json.Marshal(value)
		if err != nil {
			return ""
		}
		return string(encoded)
	}
}

func stringFromMap(decoded any, key string) string {
	root, ok := decoded.(map[string]any)
	if !ok {
		return ""
	}
	value, _ := root[key].(string)
	return value
}

func redactPlaygroundProviderCheckSecret(value string, key string) string {
	trimmed := strings.TrimSpace(key)
	if trimmed == "" || value == "" {
		return value
	}
	value = strings.ReplaceAll(value, trimmed, "[redacted]")
	return strings.ReplaceAll(value, normalizePlaygroundProviderBearerKey(trimmed), "Bearer [redacted]")
}
