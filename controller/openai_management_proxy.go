package controller

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	relaychannel "github.com/QuantumNous/new-api/relay/channel"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

const managementModelHeader = "X-NewAPI-Management-Model"

type openAIManagementModelRequest struct {
	Model string `json:"model"`
}

// RelayOpenAIManagement proxies OpenAI-compatible management endpoints that do
// not fit the normal model-priced relay flow, such as Files and Fine-tunes.
func RelayOpenAIManagement(c *gin.Context) {
	modelName, err := openAIManagementModelName(c)
	if err != nil {
		openAIManagementError(c, http.StatusBadRequest, err.Error(), types.ErrorCodeInvalidRequest)
		return
	}

	channel, err := resolveOpenAIManagementChannel(c, modelName)
	if err != nil {
		openAIManagementError(c, openAIManagementStatus(err), err.Error(), openAIManagementCode(err))
		return
	}
	defer service.FinishAllChannelMultiKeyRequests(c)

	info := &relaycommon.RelayInfo{
		RelayMode:       relayconstant.RelayModeUnknown,
		OriginModelName: modelName,
		RequestURLPath:  normalizeOpenAIManagementRequestPath(c),
		RequestHeaders:  cloneManagementRequestHeaders(c),
		StartTime:       time.Now(),
	}
	info.InitChannelMeta(c)

	resp, err := doOpenAIManagementProxyRequest(c, info)
	if err != nil {
		openAIManagementError(c, http.StatusBadGateway, err.Error(), types.ErrorCodeDoRequestFailed)
		return
	}
	defer service.CloseResponseBodyGracefully(resp)

	copyOpenAIManagementResponse(c, resp)
	if resp.StatusCode < http.StatusBadRequest {
		service.RecordChannelAffinity(c, channel.Id)
	}
}

func openAIManagementModelName(c *gin.Context) (string, error) {
	if c == nil || c.Request == nil {
		return "", errors.New("missing request")
	}
	if modelName := strings.TrimSpace(c.GetHeader(managementModelHeader)); modelName != "" {
		return modelName, nil
	}
	if modelName := strings.TrimSpace(c.Param("model")); modelName != "" {
		return modelName, nil
	}
	if c.Request.Body == nil || c.Request.Body == http.NoBody || c.Request.ContentLength == 0 {
		return defaultOpenAIManagementModelName(c), nil
	}
	if contentType := strings.TrimSpace(c.GetHeader("Content-Type")); contentType != "" && !isOpenAIManagementJSONContentType(contentType) {
		return defaultOpenAIManagementModelName(c), nil
	}
	var req openAIManagementModelRequest
	if err := common.UnmarshalBodyReusable(c, &req); err != nil {
		return "", fmt.Errorf("invalid management request body: %w", err)
	}
	if modelName := strings.TrimSpace(req.Model); modelName != "" {
		return modelName, nil
	}
	return defaultOpenAIManagementModelName(c), nil
}

func defaultOpenAIManagementModelName(c *gin.Context) string {
	if strings.HasPrefix(normalizeOpenAIManagementRequestPath(c), "/v1/images/variations") {
		return "dall-e-2"
	}
	return ""
}

func isOpenAIManagementJSONContentType(contentType string) bool {
	mediaType := strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	return mediaType == "application/json" || strings.HasSuffix(mediaType, "+json")
}

func resolveOpenAIManagementChannel(c *gin.Context, modelName string) (*model.Channel, error) {
	if channelId := strings.TrimSpace(c.GetString("specific_channel_id")); channelId != "" {
		id, err := strconv.Atoi(channelId)
		if err != nil {
			return nil, openAIManagementClientError("invalid channel id", types.ErrorCodeInvalidRequest)
		}
		channel, err := model.GetChannelById(id, true)
		if err != nil {
			return nil, openAIManagementClientError("invalid channel id", types.ErrorCodeInvalidRequest)
		}
		if channel.Status != common.ChannelStatusEnabled {
			return nil, openAIManagementClientError("channel is disabled", types.ErrorCodeAccessDenied)
		}
		if channel.SubsiteId != c.GetInt64("subsite_id") {
			return nil, openAIManagementClientError("channel is not available for this API scope", types.ErrorCodeAccessDenied)
		}
		if err := setupOpenAIManagementChannelContext(c, channel, modelName); err != nil {
			return nil, err
		}
		return channel, nil
	}

	if modelName == "" {
		return nil, openAIManagementClientError("OpenAI management endpoint requires a model-bearing request or an admin channel-specific token", types.ErrorCodeInvalidRequest)
	}
	if err := enforceOpenAIManagementModelLimit(c, modelName); err != nil {
		return nil, err
	}

	usingGroup := common.GetContextKeyString(c, constant.ContextKeyUsingGroup)
	channel, _, err := service.CacheGetRandomSatisfiedChannel(&service.RetryParam{
		Ctx:        c,
		ModelName:  modelName,
		TokenGroup: usingGroup,
		Retry:      common.GetPointer(0),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to select upstream channel for model %s: %w", modelName, err)
	}
	if channel == nil {
		return nil, openAIManagementClientError(fmt.Sprintf("no available channel for model %s", modelName), types.ErrorCodeModelNotFound)
	}
	if err := setupOpenAIManagementChannelContext(c, channel, modelName); err != nil {
		return nil, err
	}
	return channel, nil
}

func enforceOpenAIManagementModelLimit(c *gin.Context, modelName string) error {
	if !common.GetContextKeyBool(c, constant.ContextKeyTokenModelLimitEnabled) {
		return nil
	}
	limitsValue, ok := common.GetContextKey(c, constant.ContextKeyTokenModelLimit)
	if !ok {
		return openAIManagementClientError("this token has no model access", types.ErrorCodeAccessDenied)
	}
	limits, ok := limitsValue.(map[string]bool)
	if !ok {
		limits = map[string]bool{}
	}
	matchName := ratio_setting.FormatMatchingModelName(modelName)
	if !limits[matchName] {
		return openAIManagementClientError(fmt.Sprintf("this token cannot access model %s", modelName), types.ErrorCodeAccessDenied)
	}
	return nil
}

func setupOpenAIManagementChannelContext(c *gin.Context, channel *model.Channel, modelName string) error {
	if channel.Type == constant.ChannelTypeAzure {
		return openAIManagementClientError("OpenAI management endpoint proxy does not support Azure channels yet", types.ErrorCodeInvalidRequest)
	}
	if strings.Contains(channel.GetBaseURL(), "{model}") {
		return openAIManagementClientError("OpenAI management endpoint proxy requires a plain upstream base URL", types.ErrorCodeInvalidRequest)
	}
	if apiErr := middleware.SetupContextForSelectedChannel(c, channel, modelName); apiErr != nil {
		return openAIManagementUpstreamError(apiErr.Error(), apiErr.GetErrorCode())
	}
	return nil
}

func doOpenAIManagementProxyRequest(c *gin.Context, info *relaycommon.RelayInfo) (*http.Response, error) {
	if info == nil || info.ChannelMeta == nil {
		return nil, errors.New("missing upstream channel metadata")
	}
	baseURL := strings.TrimRight(strings.TrimSpace(info.ChannelBaseUrl), "/")
	if baseURL == "" {
		return nil, errors.New("missing upstream base URL")
	}
	fullRequestURL := relaycommon.GetFullRequestURL(baseURL, info.RequestURLPath, info.ChannelType)

	body, contentLength, err := openAIManagementRequestBody(c)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(c.Request.Context(), c.Request.Method, fullRequestURL, body)
	if err != nil {
		return nil, err
	}
	if contentLength > 0 {
		req.ContentLength = contentLength
	}
	setupOpenAIManagementRequestHeaders(c, req, info)
	return openAIManagementHTTPClient().Do(req)
}

func openAIManagementRequestBody(c *gin.Context) (io.Reader, int64, error) {
	if c == nil || c.Request == nil || c.Request.Body == nil || c.Request.Body == http.NoBody || c.Request.ContentLength == 0 {
		return nil, 0, nil
	}
	storage, err := common.GetBodyStorage(c)
	if err != nil {
		return nil, 0, err
	}
	if _, err := storage.Seek(0, io.SeekStart); err != nil {
		return nil, 0, err
	}
	return common.ReaderOnly(storage), storage.Size(), nil
}

func setupOpenAIManagementRequestHeaders(c *gin.Context, req *http.Request, info *relaycommon.RelayInfo) {
	for name, values := range c.Request.Header {
		if shouldSkipOpenAIManagementHeader(name) {
			continue
		}
		for _, value := range values {
			req.Header.Add(name, value)
		}
	}
	req.Header.Del(managementModelHeader)
	req.Header.Set("Authorization", "Bearer "+info.ApiKey)
	if info.ChannelType == constant.ChannelTypeOpenAI && info.Organization != "" {
		req.Header.Set("OpenAI-Organization", info.Organization)
	}
	if overrides, err := relaychannel.ResolveHeaderOverride(info, c); err == nil {
		for key, value := range overrides {
			req.Header.Set(key, value)
			if strings.EqualFold(key, "Host") {
				req.Host = value
			}
		}
	} else {
		logger.LogError(c, "OpenAI management header override failed: "+err.Error())
	}
}

func shouldSkipOpenAIManagementHeader(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "host", "content-length", "authorization", "x-api-key", "x-goog-api-key",
		"connection", "keep-alive", "proxy-authenticate", "proxy-authorization",
		"te", "trailer", "transfer-encoding", "upgrade", "accept-encoding",
		"cookie", "sec-websocket-key", "sec-websocket-version", "sec-websocket-extensions":
		return true
	default:
		return false
	}
}

func openAIManagementHTTPClient() *http.Client {
	if client := service.GetHttpClient(); client != nil {
		return client
	}
	return http.DefaultClient
}

func copyOpenAIManagementResponse(c *gin.Context, resp *http.Response) {
	for key, values := range resp.Header {
		if !service.ShouldCopyUpstreamHeader(c, key, values) {
			continue
		}
		for _, value := range values {
			c.Writer.Header().Add(key, value)
		}
	}
	c.Writer.WriteHeader(resp.StatusCode)
	if resp.Body != nil {
		if _, err := io.Copy(c.Writer, resp.Body); err != nil {
			logger.LogError(c, "failed to copy OpenAI management response: "+err.Error())
		}
	}
	c.Writer.Flush()
}

func normalizeOpenAIManagementRequestPath(c *gin.Context) string {
	if c == nil || c.Request == nil || c.Request.URL == nil {
		return "/v1"
	}
	path := c.Request.URL.Path
	if slug := c.Param("slug"); slug != "" {
		prefix := "/s/" + slug + "/v1"
		if strings.HasPrefix(path, prefix) {
			path = "/v1" + strings.TrimPrefix(path, prefix)
		}
	}
	if c.Request.URL.RawQuery != "" {
		path += "?" + c.Request.URL.RawQuery
	}
	return path
}

func cloneManagementRequestHeaders(c *gin.Context) map[string]string {
	if c == nil || c.Request == nil {
		return nil
	}
	headers := make(map[string]string, len(c.Request.Header))
	for key := range c.Request.Header {
		if shouldSkipOpenAIManagementHeader(key) {
			continue
		}
		value := strings.TrimSpace(c.Request.Header.Get(key))
		if value != "" {
			headers[key] = value
		}
	}
	return headers
}

type openAIManagementErrorInfo struct {
	message string
	status  int
	code    types.ErrorCode
}

func (e openAIManagementErrorInfo) Error() string {
	return e.message
}

func openAIManagementClientError(message string, code types.ErrorCode) error {
	return openAIManagementErrorInfo{message: message, status: http.StatusBadRequest, code: code}
}

func openAIManagementUpstreamError(message string, code types.ErrorCode) error {
	return openAIManagementErrorInfo{message: message, status: http.StatusBadGateway, code: code}
}

func openAIManagementStatus(err error) int {
	var info openAIManagementErrorInfo
	if errors.As(err, &info) && info.status > 0 {
		return info.status
	}
	return http.StatusServiceUnavailable
}

func openAIManagementCode(err error) types.ErrorCode {
	var info openAIManagementErrorInfo
	if errors.As(err, &info) {
		return info.code
	}
	return types.ErrorCodeGetChannelFailed
}

func openAIManagementError(c *gin.Context, statusCode int, message string, code types.ErrorCode) {
	c.JSON(statusCode, gin.H{
		"error": types.OpenAIError{
			Message: common.MessageWithRequestId(message, c.GetString(common.RequestIdKey)),
			Type:    "new_api_error",
			Code:    string(code),
		},
	})
}
