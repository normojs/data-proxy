package controller

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

type requestLogTraceResponse struct {
	Query              string                 `json:"query"`
	Scope              string                 `json:"scope"`
	Total              int                    `json:"total"`
	RequestIds         []string               `json:"request_ids"`
	UpstreamRequestIds []string               `json:"upstream_request_ids"`
	Summary            requestLogTraceSummary `json:"summary"`
	Diagnostics        map[string]interface{} `json:"diagnostics"`
	Logs               []requestLogTraceItem  `json:"logs"`
}

type requestLogTraceSummary struct {
	Status           string         `json:"status"`
	TypeCounts       map[string]int `json:"type_counts"`
	UserId           int            `json:"user_id,omitempty"`
	Username         string         `json:"username,omitempty"`
	TokenId          int            `json:"token_id,omitempty"`
	TokenName        string         `json:"token_name,omitempty"`
	ModelName        string         `json:"model_name,omitempty"`
	ChannelId        int            `json:"channel,omitempty"`
	ChannelName      string         `json:"channel_name,omitempty"`
	Group            string         `json:"group,omitempty"`
	Quota            int            `json:"quota"`
	PromptTokens     int            `json:"prompt_tokens"`
	CompletionTokens int            `json:"completion_tokens"`
	MaxUseTime       int            `json:"max_use_time"`
	IsStream         bool           `json:"is_stream"`
	CreatedAtStart   int64          `json:"created_at_start,omitempty"`
	CreatedAtEnd     int64          `json:"created_at_end,omitempty"`
}

type requestLogTraceItem struct {
	Id                int                    `json:"id"`
	UserId            int                    `json:"user_id"`
	CreatedAt         int64                  `json:"created_at"`
	Type              int                    `json:"type"`
	TypeName          string                 `json:"type_name"`
	Content           string                 `json:"content"`
	Username          string                 `json:"username"`
	TokenId           int                    `json:"token_id"`
	TokenName         string                 `json:"token_name"`
	ModelName         string                 `json:"model_name"`
	Quota             int                    `json:"quota"`
	PromptTokens      int                    `json:"prompt_tokens"`
	CompletionTokens  int                    `json:"completion_tokens"`
	UseTime           int                    `json:"use_time"`
	IsStream          bool                   `json:"is_stream"`
	ChannelId         int                    `json:"channel"`
	ChannelName       string                 `json:"channel_name"`
	Group             string                 `json:"group"`
	Ip                string                 `json:"ip,omitempty"`
	RequestId         string                 `json:"request_id,omitempty"`
	UpstreamRequestId string                 `json:"upstream_request_id,omitempty"`
	Other             map[string]interface{} `json:"other,omitempty"`
}

func GetAllLogs(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	logType, _ := strconv.Atoi(c.Query("type"))
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	username := c.Query("username")
	tokenName := c.Query("token_name")
	modelName := c.Query("model_name")
	channel, _ := strconv.Atoi(c.Query("channel"))
	group := c.Query("group")
	requestId := c.Query("request_id")
	upstreamRequestId := c.Query("upstream_request_id")
	logs, total, err := model.GetAllLogs(logType, startTimestamp, endTimestamp, modelName, username, tokenName, pageInfo.GetStartIdx(), pageInfo.GetPageSize(), channel, group, requestId, upstreamRequestId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(logs)
	common.ApiSuccess(c, pageInfo)
	return
}

func GetRequestLogTrace(c *gin.Context) {
	requestId := requestLogTraceQuery(c)
	if requestId == "" {
		common.ApiErrorMsg(c, "request_id is required")
		return
	}
	logs, err := model.GetLogsByRequestId(requestId, 0, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, buildRequestLogTraceResponse(requestId, "admin", logs, true))
}

func GetSelfRequestLogTrace(c *gin.Context) {
	requestId := requestLogTraceQuery(c)
	if requestId == "" {
		common.ApiErrorMsg(c, "request_id is required")
		return
	}
	logs, err := model.GetLogsByRequestId(requestId, c.GetInt("id"), true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, buildRequestLogTraceResponse(requestId, "self", logs, false))
}

func requestLogTraceQuery(c *gin.Context) string {
	requestId := strings.TrimSpace(c.Param("request_id"))
	if requestId == "" {
		requestId = strings.TrimSpace(c.Query("request_id"))
	}
	return requestId
}

func GetUserLogs(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	userId := c.GetInt("id")
	logType, _ := strconv.Atoi(c.Query("type"))
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	tokenName := c.Query("token_name")
	modelName := c.Query("model_name")
	group := c.Query("group")
	requestId := c.Query("request_id")
	upstreamRequestId := c.Query("upstream_request_id")
	logs, total, err := model.GetUserLogs(userId, logType, startTimestamp, endTimestamp, modelName, tokenName, pageInfo.GetStartIdx(), pageInfo.GetPageSize(), group, requestId, upstreamRequestId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(logs)
	common.ApiSuccess(c, pageInfo)
	return
}

func GetLogFilterOptions(c *gin.Context) {
	logType, _ := strconv.Atoi(c.Query("type"))
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	username := c.Query("username")
	channel, _ := strconv.Atoi(c.Query("channel"))

	options, err := model.GetLogFilterOptions(0, false, logType, startTimestamp, endTimestamp, username, channel)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, options)
}

func GetSelfLogFilterOptions(c *gin.Context) {
	logType, _ := strconv.Atoi(c.Query("type"))
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)

	options, err := model.GetLogFilterOptions(c.GetInt("id"), true, logType, startTimestamp, endTimestamp, "", 0)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, options)
}

func buildRequestLogTraceResponse(query string, scope string, logs []*model.Log, includeAdmin bool) requestLogTraceResponse {
	response := requestLogTraceResponse{
		Query:              query,
		Scope:              scope,
		Total:              len(logs),
		RequestIds:         make([]string, 0),
		UpstreamRequestIds: make([]string, 0),
		Summary:            requestLogTraceSummary{Status: "not_found", TypeCounts: map[string]int{}},
		Diagnostics:        map[string]interface{}{},
		Logs:               make([]requestLogTraceItem, 0, len(logs)),
	}
	if len(logs) == 0 {
		return response
	}

	requestIds := map[string]bool{}
	upstreamRequestIds := map[string]bool{}
	containsConsume := false
	containsError := false
	errors := make([]string, 0)

	for i, log := range logs {
		other := parseRequestLogOther(log.Other, includeAdmin)
		typeName := logTypeName(log.Type)
		response.Summary.TypeCounts[typeName]++
		response.Summary.Quota += log.Quota
		response.Summary.PromptTokens += log.PromptTokens
		response.Summary.CompletionTokens += log.CompletionTokens
		if log.UseTime > response.Summary.MaxUseTime {
			response.Summary.MaxUseTime = log.UseTime
		}
		if log.IsStream {
			response.Summary.IsStream = true
		}
		if response.Summary.CreatedAtStart == 0 || log.CreatedAt < response.Summary.CreatedAtStart {
			response.Summary.CreatedAtStart = log.CreatedAt
		}
		if log.CreatedAt > response.Summary.CreatedAtEnd {
			response.Summary.CreatedAtEnd = log.CreatedAt
		}
		if i == 0 {
			response.Summary.UserId = log.UserId
			response.Summary.Username = log.Username
			response.Summary.TokenId = log.TokenId
			response.Summary.TokenName = log.TokenName
			response.Summary.ModelName = log.ModelName
			response.Summary.ChannelId = log.ChannelId
			response.Summary.ChannelName = log.ChannelName
			response.Summary.Group = log.Group
		}
		if log.RequestId != "" && !requestIds[log.RequestId] {
			requestIds[log.RequestId] = true
			response.RequestIds = append(response.RequestIds, log.RequestId)
		}
		if log.UpstreamRequestId != "" && !upstreamRequestIds[log.UpstreamRequestId] {
			upstreamRequestIds[log.UpstreamRequestId] = true
			response.UpstreamRequestIds = append(response.UpstreamRequestIds, log.UpstreamRequestId)
		}
		if log.Type == model.LogTypeConsume {
			containsConsume = true
		}
		if log.Type == model.LogTypeError {
			containsError = true
			if strings.TrimSpace(log.Content) != "" {
				errors = append(errors, log.Content)
			}
		}

		mergeRequestLogDiagnostics(response.Diagnostics, other)
		response.Logs = append(response.Logs, requestLogTraceItem{
			Id:                log.Id,
			UserId:            log.UserId,
			CreatedAt:         log.CreatedAt,
			Type:              log.Type,
			TypeName:          typeName,
			Content:           log.Content,
			Username:          log.Username,
			TokenId:           log.TokenId,
			TokenName:         log.TokenName,
			ModelName:         log.ModelName,
			Quota:             log.Quota,
			PromptTokens:      log.PromptTokens,
			CompletionTokens:  log.CompletionTokens,
			UseTime:           log.UseTime,
			IsStream:          log.IsStream,
			ChannelId:         log.ChannelId,
			ChannelName:       log.ChannelName,
			Group:             log.Group,
			Ip:                log.Ip,
			RequestId:         log.RequestId,
			UpstreamRequestId: log.UpstreamRequestId,
			Other:             other,
		})
	}

	switch {
	case containsError:
		response.Summary.Status = "error"
	case containsConsume:
		response.Summary.Status = "completed"
	default:
		response.Summary.Status = "logged"
	}
	response.Diagnostics["log_count"] = len(logs)
	response.Diagnostics["contains_error"] = containsError
	response.Diagnostics["contains_consume"] = containsConsume
	if len(errors) > 0 {
		response.Diagnostics["errors"] = errors
	}
	return response
}

func parseRequestLogOther(raw string, includeAdmin bool) map[string]interface{} {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	other, err := common.StrToMap(raw)
	if err != nil || len(other) == 0 {
		return nil
	}
	if !includeAdmin {
		delete(other, "admin_info")
		delete(other, "stream_status")
	}
	return redactRequestLogSensitiveValue(other).(map[string]interface{})
}

func mergeRequestLogDiagnostics(diagnostics map[string]interface{}, other map[string]interface{}) {
	if len(other) == 0 {
		return
	}
	keys := []string{
		"request_path",
		"request_conversion",
		"request_conversion_meta",
		"stream_status",
		"admin_info",
		"billing_source",
		"billing_preference",
		"upstream_model_name",
		"reasoning_effort",
		"is_model_mapped",
		"is_system_prompt_overwritten",
		"error_type",
		"error_code",
		"status_code",
		"channel_id",
		"channel_name",
		"channel_type",
		"po",
	}
	for _, key := range keys {
		value, ok := other[key]
		if !ok {
			continue
		}
		if key == "request_conversion_meta" {
			mergeMapDiagnostic(diagnostics, key, value)
			continue
		}
		if _, exists := diagnostics[key]; !exists {
			diagnostics[key] = value
		}
	}
}

func mergeMapDiagnostic(diagnostics map[string]interface{}, key string, value interface{}) {
	incoming, ok := value.(map[string]interface{})
	if !ok {
		if _, exists := diagnostics[key]; !exists {
			diagnostics[key] = value
		}
		return
	}
	existing, ok := diagnostics[key].(map[string]interface{})
	if !ok {
		existing = map[string]interface{}{}
		diagnostics[key] = existing
	}
	for metaKey, metaValue := range incoming {
		if _, exists := existing[metaKey]; !exists {
			existing[metaKey] = metaValue
		}
	}
}

func redactRequestLogSensitiveValue(value interface{}) interface{} {
	switch current := value.(type) {
	case map[string]interface{}:
		redacted := make(map[string]interface{}, len(current))
		for key, item := range current {
			if isSensitiveRequestLogKey(key) {
				redacted[key] = "***"
				continue
			}
			redacted[key] = redactRequestLogSensitiveValue(item)
		}
		return redacted
	case []interface{}:
		redacted := make([]interface{}, len(current))
		for i, item := range current {
			redacted[i] = redactRequestLogSensitiveValue(item)
		}
		return redacted
	default:
		return value
	}
}

func isSensitiveRequestLogKey(key string) bool {
	normalized := strings.ToLower(strings.TrimSpace(key))
	switch normalized {
	case "api_key", "apikey", "key", "secret", "password", "authorization", "credentials", "credential", "private_key", "access_token", "refresh_token", "id_token", "client_secret", "webhook_secret":
		return true
	}
	return strings.Contains(normalized, "api_key") ||
		strings.Contains(normalized, "private_key") ||
		strings.Contains(normalized, "access_token") ||
		strings.Contains(normalized, "refresh_token") ||
		strings.Contains(normalized, "client_secret") ||
		strings.Contains(normalized, "webhook_secret")
}

func logTypeName(logType int) string {
	switch logType {
	case model.LogTypeTopup:
		return "topup"
	case model.LogTypeConsume:
		return "consume"
	case model.LogTypeManage:
		return "manage"
	case model.LogTypeSystem:
		return "system"
	case model.LogTypeError:
		return "error"
	case model.LogTypeRefund:
		return "refund"
	default:
		return "unknown"
	}
}

// Deprecated: SearchAllLogs 已废弃，前端未使用该接口。
func SearchAllLogs(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": false,
		"message": "该接口已废弃",
	})
}

// Deprecated: SearchUserLogs 已废弃，前端未使用该接口。
func SearchUserLogs(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": false,
		"message": "该接口已废弃",
	})
}

func GetLogByKey(c *gin.Context) {
	tokenId := c.GetInt("token_id")
	if tokenId == 0 {
		c.JSON(200, gin.H{
			"success": false,
			"message": "无效的令牌",
		})
		return
	}
	logs, err := model.GetLogByTokenId(tokenId)
	if err != nil {
		c.JSON(200, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(200, gin.H{
		"success": true,
		"message": "",
		"data":    logs,
	})
}

func GetLogsStat(c *gin.Context) {
	logType, _ := strconv.Atoi(c.Query("type"))
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	tokenName := c.Query("token_name")
	username := c.Query("username")
	modelName := c.Query("model_name")
	channel, _ := strconv.Atoi(c.Query("channel"))
	group := c.Query("group")
	stat, err := model.SumUsedQuota(logType, startTimestamp, endTimestamp, modelName, username, tokenName, channel, group)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	//tokenNum := model.SumUsedToken(logType, startTimestamp, endTimestamp, modelName, username, "")
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"quota":  stat.Quota,
			"rpm":    stat.Rpm,
			"tpm":    stat.Tpm,
			"tokens": stat.Tokens,
		},
	})
	return
}

func GetLogsSelfStat(c *gin.Context) {
	username := c.GetString("username")
	logType, _ := strconv.Atoi(c.Query("type"))
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	tokenName := c.Query("token_name")
	modelName := c.Query("model_name")
	channel, _ := strconv.Atoi(c.Query("channel"))
	group := c.Query("group")
	quotaNum, err := model.SumUsedQuota(logType, startTimestamp, endTimestamp, modelName, username, tokenName, channel, group)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	//tokenNum := model.SumUsedToken(logType, startTimestamp, endTimestamp, modelName, username, tokenName)
	c.JSON(200, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"quota":  quotaNum.Quota,
			"rpm":    quotaNum.Rpm,
			"tpm":    quotaNum.Tpm,
			"tokens": quotaNum.Tokens,
			//"token": tokenNum,
		},
	})
	return
}

func DeleteHistoryLogs(c *gin.Context) {
	targetTimestamp, _ := strconv.ParseInt(c.Query("target_timestamp"), 10, 64)
	if targetTimestamp == 0 {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "target timestamp is required",
		})
		return
	}
	count, err := model.DeleteOldLog(c.Request.Context(), targetTimestamp, 100)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    count,
	})
	return
}
