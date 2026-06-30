package relay

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/relay/channel"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/openaicompat"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

func responsesCompactViaChatCompletions(c *gin.Context, info *relaycommon.RelayInfo, adaptor channel.Adaptor, request *dto.OpenAIResponsesRequest) (*dto.Usage, *types.NewAPIError) {
	chatReq, compatCtx, err := service.ResponsesRequestToChatCompletionsRequestWithOptions(request, &openaicompat.ResponsesToChatOptions{
		ChannelType:      info.ChannelType,
		ReasoningAdapter: info.ChannelOtherSettings.ResponsesReasoningAdapter,
	})
	if err != nil {
		return nil, types.NewErrorWithStatusCode(err, types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}
	info.AppendRequestConversion(types.RelayFormatOpenAI)
	info.SetRequestConversionMeta("responses_compact_converted", true)
	mergeResponsesCompatMeta(info, compatCtx)

	savedRelayMode := info.RelayMode
	savedRequestURLPath := info.RequestURLPath
	defer func() {
		info.RelayMode = savedRelayMode
		info.RequestURLPath = savedRequestURLPath
	}()

	info.RelayMode = relayconstant.RelayModeChatCompletions
	info.RequestURLPath = "/v1/chat/completions"

	convertedRequest, err := adaptor.ConvertOpenAIRequest(c, info, chatReq)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}
	relaycommon.AppendRequestConversionFromRequest(info, convertedRequest)

	jsonData, err := common.Marshal(convertedRequest)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}
	jsonData, err = relaycommon.RemoveDisabledFields(jsonData, info.ChannelOtherSettings, info.ChannelSetting.PassThroughBodyEnabled)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}
	if len(info.ParamOverride) > 0 {
		jsonData, err = relaycommon.ApplyParamOverrideWithRelayInfo(jsonData, info)
		if err != nil {
			return nil, newAPIErrorFromParamOverride(err)
		}
	}

	logger.LogDebug(c, "responses compact chat compatibility requestBody: %s", jsonData)
	body, size, closer, err := relaycommon.NewOutboundJSONBody(jsonData)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}
	defer closer.Close()
	info.UpstreamRequestBodySize = size

	resp, err := adaptor.DoRequest(c, info, body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
	}
	if resp == nil {
		return nil, types.NewOpenAIError(nil, types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}

	httpResp := resp.(*http.Response)
	statusCodeMappingStr := c.GetString("status_code_mapping")
	if httpResp.StatusCode != http.StatusOK {
		newAPIError := service.RelayErrorHandler(c.Request.Context(), httpResp, false)
		service.ResetStatusCode(newAPIError, statusCodeMappingStr)
		return nil, newAPIError
	}
	usage, newAPIError := chatCompletionsToResponsesCompactHandler(c, info, httpResp, request, compatCtx)
	if newAPIError != nil {
		service.ResetStatusCode(newAPIError, statusCodeMappingStr)
		return nil, newAPIError
	}
	return usage, nil
}

func responsesViaChatCompletions(c *gin.Context, info *relaycommon.RelayInfo, adaptor channel.Adaptor, request *dto.OpenAIResponsesRequest) (*dto.Usage, *types.NewAPIError) {
	chatReq, compatCtx, err := service.ResponsesRequestToChatCompletionsRequestWithOptions(request, &openaicompat.ResponsesToChatOptions{
		ChannelType:      info.ChannelType,
		ReasoningAdapter: info.ChannelOtherSettings.ResponsesReasoningAdapter,
	})
	if err != nil {
		return nil, types.NewErrorWithStatusCode(err, types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}
	info.AppendRequestConversion(types.RelayFormatOpenAI)
	mergeResponsesCompatMeta(info, compatCtx)

	savedRelayMode := info.RelayMode
	savedRequestURLPath := info.RequestURLPath
	defer func() {
		info.RelayMode = savedRelayMode
		info.RequestURLPath = savedRequestURLPath
	}()

	info.RelayMode = relayconstant.RelayModeChatCompletions
	info.RequestURLPath = "/v1/chat/completions"

	convertedRequest, err := adaptor.ConvertOpenAIRequest(c, info, chatReq)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}
	relaycommon.AppendRequestConversionFromRequest(info, convertedRequest)

	jsonData, err := common.Marshal(convertedRequest)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}
	jsonData, err = relaycommon.RemoveDisabledFields(jsonData, info.ChannelOtherSettings, info.ChannelSetting.PassThroughBodyEnabled)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}
	if len(info.ParamOverride) > 0 {
		jsonData, err = relaycommon.ApplyParamOverrideWithRelayInfo(jsonData, info)
		if err != nil {
			return nil, newAPIErrorFromParamOverride(err)
		}
	}

	logger.LogDebug(c, "responses chat compatibility requestBody: %s", jsonData)
	body, size, closer, err := relaycommon.NewOutboundJSONBody(jsonData)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}
	defer closer.Close()
	info.UpstreamRequestBodySize = size
	var requestBody io.Reader = body

	resp, err := adaptor.DoRequest(c, info, requestBody)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
	}
	if resp == nil {
		return nil, types.NewOpenAIError(nil, types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}

	httpResp := resp.(*http.Response)
	info.IsStream = info.IsStream || strings.HasPrefix(httpResp.Header.Get("Content-Type"), "text/event-stream")
	statusCodeMappingStr := c.GetString("status_code_mapping")
	if httpResp.StatusCode != http.StatusOK {
		newAPIError := service.RelayErrorHandler(c.Request.Context(), httpResp, false)
		service.ResetStatusCode(newAPIError, statusCodeMappingStr)
		return nil, newAPIError
	}

	if info.IsStream {
		usage, newAPIError := chatCompletionsStreamToResponsesHandler(c, info, httpResp, request, compatCtx)
		if newAPIError != nil {
			service.ResetStatusCode(newAPIError, statusCodeMappingStr)
			return nil, newAPIError
		}
		return usage, nil
	}

	usage, newAPIError := chatCompletionsToResponsesHandler(c, info, httpResp, request, compatCtx)
	if newAPIError != nil {
		service.ResetStatusCode(newAPIError, statusCodeMappingStr)
		return nil, newAPIError
	}
	return usage, nil
}

func chatCompletionsToResponsesCompactHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response, request *dto.OpenAIResponsesRequest, compatCtx *openaicompat.ResponsesToChatContext) (*dto.Usage, *types.NewAPIError) {
	defer service.CloseResponseBodyGracefully(resp)

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}
	text, responsesUsage, newAPIError := compactTextAndUsageFromChatBody(responseBody, request, compatCtx, resp.StatusCode)
	if newAPIError != nil {
		return nil, newAPIError
	}
	if responsesUsage == nil || responsesUsage.TotalTokens == 0 && responsesUsage.InputTokens == 0 && responsesUsage.OutputTokens == 0 {
		responsesUsage = openaicompat.ChatUsageToResponsesUsage(service.ResponseText2Usage(c, text, info.UpstreamModelName, info.GetEstimatePromptTokens()))
	}

	response := dto.OpenAIResponsesCompactionResponse{
		ID:        "resp_" + common.GetUUID(),
		Object:    "response",
		CreatedAt: int(time.Now().Unix()),
		Output:    compactOutputMessage(text),
		Usage:     responsesUsage,
	}
	responseJSON, err := common.Marshal(response)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	info.SetRequestConversionMeta("responses_compact_output_text_chars", len(text))
	mergeResponsesCompatMeta(info, compatCtx)
	service.IOCopyBytesGracefully(c, resp, responseJSON)
	return openaicompat.ResponsesUsageToChatUsage(responsesUsage), nil
}

func chatCompletionsToResponsesHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response, request *dto.OpenAIResponsesRequest, compatCtx *openaicompat.ResponsesToChatContext) (*dto.Usage, *types.NewAPIError) {
	defer service.CloseResponseBodyGracefully(resp)

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}
	var chatResp dto.OpenAITextResponse
	if err := common.Unmarshal(responseBody, &chatResp); err != nil {
		responsesResp, usage, ok, aggregateErr := service.ChatCompletionsStreamBodyToResponses(responseBody, request, compatCtx)
		if aggregateErr != nil {
			return nil, types.NewOpenAIError(aggregateErr, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
		}
		if !ok || responsesResp == nil {
			return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
		}
		info.SetRequestConversionMeta("chat_sse_fallback", true)
		recordResponsesTerminalStatus(info, responsesResp)
		mergeResponsesCompatMeta(info, compatCtx)
		responseJSON, marshalErr := common.Marshal(responsesResp)
		if marshalErr != nil {
			return nil, types.NewOpenAIError(marshalErr, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
		}
		service.IOCopyBytesGracefully(c, resp, responseJSON)
		return openaicompat.ResponsesUsageToChatUsage(usage), nil
	}
	if oaiError := chatResp.GetOpenAIError(); oaiError != nil && oaiError.Type != "" {
		return nil, types.WithOpenAIError(*oaiError, resp.StatusCode)
	}

	responsesResp, usage, err := service.ChatCompletionResponseToResponses(&chatResp, request, compatCtx)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	recordResponsesTerminalStatus(info, responsesResp)
	mergeResponsesCompatMeta(info, compatCtx)
	responseJSON, err := common.Marshal(responsesResp)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	service.IOCopyBytesGracefully(c, resp, responseJSON)
	return openaicompat.ResponsesUsageToChatUsage(usage), nil
}

func compactTextAndUsageFromChatBody(responseBody []byte, request *dto.OpenAIResponsesRequest, compatCtx *openaicompat.ResponsesToChatContext, statusCode int) (string, *dto.Usage, *types.NewAPIError) {
	var chatResp dto.OpenAITextResponse
	if err := common.Unmarshal(responseBody, &chatResp); err != nil {
		responsesResp, usage, ok, aggregateErr := service.ChatCompletionsStreamBodyToResponses(responseBody, request, compatCtx)
		if aggregateErr != nil {
			return "", nil, types.NewOpenAIError(aggregateErr, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
		}
		if !ok || responsesResp == nil {
			return "", nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
		}
		return compactTextFromResponsesOutput(responsesResp["output"]), usage, nil
	}
	if oaiError := chatResp.GetOpenAIError(); oaiError != nil && oaiError.Type != "" {
		return "", nil, types.WithOpenAIError(*oaiError, statusCode)
	}
	text := ""
	if len(chatResp.Choices) > 0 {
		msg := chatResp.Choices[0].Message
		text = msg.StringContent()
		if strings.TrimSpace(text) == "" {
			text = msg.GetReasoningContent()
		}
	}
	return text, openaicompat.ChatUsageToResponsesUsage(&chatResp.Usage), nil
}

func compactTextFromResponsesOutput(output any) string {
	items, ok := output.([]any)
	if !ok {
		return ""
	}
	var parts []string
	for _, rawItem := range items {
		item, ok := rawItem.(map[string]any)
		if !ok {
			continue
		}
		content, ok := item["content"].([]any)
		if !ok {
			continue
		}
		for _, rawPart := range content {
			part, ok := rawPart.(map[string]any)
			if !ok {
				continue
			}
			if text := strings.TrimSpace(common.Interface2String(part["text"])); text != "" {
				parts = append(parts, text)
			}
		}
	}
	return strings.Join(parts, "\n")
}

func compactOutputMessage(text string) json.RawMessage {
	output := []any{
		map[string]any{
			"id":     "msg_" + common.GetUUID(),
			"type":   "message",
			"role":   "assistant",
			"status": "completed",
			"content": []any{
				map[string]any{
					"type":        "output_text",
					"text":        text,
					"annotations": []any{},
				},
			},
		},
	}
	raw, err := common.Marshal(output)
	if err != nil {
		return json.RawMessage("[]")
	}
	return raw
}

func chatCompletionsStreamToResponsesHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response, request *dto.OpenAIResponsesRequest, compatCtx *openaicompat.ResponsesToChatContext) (*dto.Usage, *types.NewAPIError) {
	if resp == nil || resp.Body == nil {
		return nil, types.NewOpenAIError(fmt.Errorf("invalid response"), types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}
	defer service.CloseResponseBodyGracefully(resp)

	converter := openaicompat.NewChatToResponsesStreamConverter(request, compatCtx)
	var streamErr *types.NewAPIError

	sendEvent := func(event openaicompat.ChatToResponsesStreamEvent) bool {
		event.Payload["type"] = event.Event
		jsonData, err := common.Marshal(event.Payload)
		if err != nil {
			streamErr = types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
			return false
		}
		c.Render(-1, common.CustomEvent{Data: fmt.Sprintf("event: %s\n", event.Event)})
		c.Render(-1, common.CustomEvent{Data: "data: " + string(jsonData)})
		if err := helper.FlushWriter(c); err != nil {
			streamErr = types.NewOpenAIError(err, types.ErrorCodeBadResponse, http.StatusInternalServerError)
			return false
		}
		return true
	}

	mappedErr := helper.StreamScannerHandler(c, resp, info, func(data string, sr *helper.StreamResult) {
		if streamErr != nil {
			sr.Stop(streamErr)
			return
		}
		if strings.TrimSpace(data) == "[DONE]" {
			return
		}
		var chunk dto.ChatCompletionsStreamResponse
		if err := common.UnmarshalJsonStr(data, &chunk); err != nil {
			streamErr = types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
			sr.Stop(streamErr)
			return
		}
		for _, event := range converter.HandleChunk(chunk) {
			if !sendEvent(event) {
				sr.Stop(streamErr)
				return
			}
		}
	})
	if mappedErr != nil {
		return nil, mappedErr
	}
	if streamErr != nil {
		return nil, streamErr
	}

	usage := converter.Usage()
	if usage.TotalTokens == 0 && usage.PromptTokens == 0 && usage.CompletionTokens == 0 {
		usage = service.ResponseText2Usage(c, converter.EstimatedOutputText(), info.UpstreamModelName, info.GetEstimatePromptTokens())
		converter.SetUsage(usage)
	}
	finishEvents := converter.Finish()
	for _, event := range finishEvents {
		if response, ok := event.Payload["response"].(map[string]any); ok {
			recordResponsesTerminalStatus(info, response)
		}
		if !sendEvent(event) {
			return nil, streamErr
		}
	}
	mergeResponsesCompatMeta(info, compatCtx)
	return usage, nil
}
