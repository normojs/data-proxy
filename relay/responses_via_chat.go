package relay

import (
	"fmt"
	"io"
	"net/http"
	"sort"
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

func responsesViaChatCompletions(c *gin.Context, info *relaycommon.RelayInfo, adaptor channel.Adaptor, request *dto.OpenAIResponsesRequest) (*dto.Usage, *types.NewAPIError) {
	chatReq, compatCtx, err := service.ResponsesRequestToChatCompletionsRequest(request)
	if err != nil {
		return nil, types.NewErrorWithStatusCode(err, types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}
	info.AppendRequestConversion(types.RelayFormatOpenAI)

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

func chatCompletionsToResponsesHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response, request *dto.OpenAIResponsesRequest, compatCtx *openaicompat.ResponsesToChatContext) (*dto.Usage, *types.NewAPIError) {
	defer service.CloseResponseBodyGracefully(resp)

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}
	var chatResp dto.OpenAITextResponse
	if err := common.Unmarshal(responseBody, &chatResp); err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	if oaiError := chatResp.GetOpenAIError(); oaiError != nil && oaiError.Type != "" {
		return nil, types.WithOpenAIError(*oaiError, resp.StatusCode)
	}

	responsesResp, usage, err := service.ChatCompletionResponseToResponses(&chatResp, request, compatCtx)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	responseJSON, err := common.Marshal(responsesResp)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	service.IOCopyBytesGracefully(c, resp, responseJSON)
	return openaicompat.ResponsesUsageToChatUsage(usage), nil
}

func chatCompletionsStreamToResponsesHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response, request *dto.OpenAIResponsesRequest, compatCtx *openaicompat.ResponsesToChatContext) (*dto.Usage, *types.NewAPIError) {
	if resp == nil || resp.Body == nil {
		return nil, types.NewOpenAIError(fmt.Errorf("invalid response"), types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}
	defer service.CloseResponseBodyGracefully(resp)

	usage := &dto.Usage{}
	responseID := "resp_" + common.GetUUID()
	model := request.Model
	createdAt := time.Now().Unix()
	var outputText strings.Builder
	var fallbackReasoningText strings.Builder
	var textItemStarted bool
	var textDone bool
	toolStates := map[int]*streamToolState{}
	var streamErr *types.NewAPIError

	send := func(event string, payload map[string]any) bool {
		payload["type"] = event
		jsonData, err := common.Marshal(payload)
		if err != nil {
			streamErr = types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
			return false
		}
		c.Render(-1, common.CustomEvent{Data: fmt.Sprintf("event: %s\n", event)})
		c.Render(-1, common.CustomEvent{Data: "data: " + string(jsonData)})
		if err := helper.FlushWriter(c); err != nil {
			streamErr = types.NewOpenAIError(err, types.ErrorCodeBadResponse, http.StatusInternalServerError)
			return false
		}
		return true
	}

	baseResponse := func(status string, output []any) map[string]any {
		resp := map[string]any{
			"id":         responseID,
			"object":     "response",
			"created_at": createdAt,
			"status":     status,
			"model":      model,
			"output":     output,
		}
		if request.PreviousResponseID != "" {
			resp["previous_response_id"] = request.PreviousResponseID
		}
		return resp
	}

	sendCreated := false
	ensureCreated := func() bool {
		if sendCreated {
			return true
		}
		sendCreated = true
		return send("response.created", map[string]any{
			"response": baseResponse("in_progress", []any{}),
		})
	}

	toolOutputIndex := func(state *streamToolState) int {
		if state == nil {
			return 0
		}
		if textItemStarted {
			return state.Index + 1
		}
		return state.Index
	}

	toolCallForState := func(state *streamToolState, arguments string) dto.ToolCallRequest {
		callID := strings.TrimSpace(state.ID)
		if callID == "" {
			callID = "call_" + common.GetUUID()
			state.ID = callID
		}
		name := strings.TrimSpace(state.Name)
		if name == "" {
			name = "tool"
		}
		return dto.ToolCallRequest{
			ID:   callID,
			Type: "function",
			Function: dto.FunctionRequest{
				Name:      name,
				Arguments: arguments,
			},
		}
	}

	toolItemForState := func(state *streamToolState, arguments string, status string) map[string]any {
		item := openaicompat.ChatToolCallToResponsesItem(toolCallForState(state, arguments), compatCtx)
		if status != "" {
			item["status"] = status
		}
		itemID := strings.TrimSpace(common.Interface2String(item["id"]))
		if itemID == "" {
			itemID = "fc_" + state.ID
			item["id"] = itemID
		}
		if state.ItemID == "" {
			state.ItemID = itemID
		}
		return item
	}

	ensureToolStarted := func(state *streamToolState) bool {
		if state == nil {
			return true
		}
		if state.Started {
			return true
		}
		if !ensureCreated() {
			return false
		}
		state.Started = true
		item := toolItemForState(state, "", "in_progress")
		return send("response.output_item.added", map[string]any{
			"output_index": toolOutputIndex(state),
			"item":         item,
		})
	}

	sendToolArgumentsDelta := func(state *streamToolState, delta string) bool {
		if state == nil || delta == "" {
			return true
		}
		if !ensureToolStarted(state) {
			return false
		}
		item := toolItemForState(state, state.Arguments.String(), "in_progress")
		if common.Interface2String(item["type"]) != "function_call" {
			return true
		}
		return send("response.function_call_arguments.delta", map[string]any{
			"output_index": toolOutputIndex(state),
			"item_id":      state.ItemID,
			"delta":        delta,
		})
	}

	finalizeTool := func(state *streamToolState) bool {
		if state == nil || state.Done {
			return true
		}
		if !ensureToolStarted(state) {
			return false
		}
		state.Done = true
		arguments := state.Arguments.String()
		item := toolItemForState(state, arguments, "completed")
		if common.Interface2String(item["type"]) == "function_call" {
			if !send("response.function_call_arguments.done", map[string]any{
				"output_index": toolOutputIndex(state),
				"item_id":      state.ItemID,
				"arguments":    arguments,
			}) {
				return false
			}
		}
		return send("response.output_item.done", map[string]any{
			"output_index": toolOutputIndex(state),
			"item":         item,
		})
	}

	sortedToolStates := func() []*streamToolState {
		states := make([]*streamToolState, 0, len(toolStates))
		for _, state := range toolStates {
			states = append(states, state)
		}
		sort.Slice(states, func(i, j int) bool {
			return states[i].Index < states[j].Index
		})
		return states
	}

	ensureTextStarted := func() bool {
		if textItemStarted {
			return true
		}
		if !ensureCreated() {
			return false
		}
		textItemStarted = true
		item := openaicompat.ResponseMessageItem("msg_"+responseID, "")
		return send("response.output_item.added", map[string]any{
			"output_index": 0,
			"item":         item,
		}) && send("response.content_part.added", map[string]any{
			"output_index":  0,
			"content_index": 0,
			"part": map[string]any{
				"type":        "output_text",
				"text":        "",
				"annotations": []any{},
			},
		})
	}

	finalizeText := func() bool {
		if !textItemStarted || textDone {
			return true
		}
		textDone = true
		text := outputText.String()
		return send("response.output_text.done", map[string]any{
			"output_index":  0,
			"content_index": 0,
			"text":          text,
		}) && send("response.content_part.done", map[string]any{
			"output_index":  0,
			"content_index": 0,
			"part": map[string]any{
				"type":        "output_text",
				"text":        text,
				"annotations": []any{},
			},
		}) && send("response.output_item.done", map[string]any{
			"output_index": 0,
			"item":         openaicompat.ResponseMessageItem("msg_"+responseID, text),
		})
	}

	helper.StreamScannerHandler(c, resp, info, func(data string, sr *helper.StreamResult) {
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
		if chunk.Id != "" {
			responseID = openaicompat.ChatStreamIDToResponsesID(chunk.Id)
		}
		if chunk.Model != "" {
			model = chunk.Model
		}
		if chunk.Created != 0 {
			createdAt = chunk.Created
		}
		if chunk.Usage != nil {
			usage = chunk.Usage
		}
		if len(chunk.Choices) == 0 {
			return
		}
		choice := chunk.Choices[0]
		if textDelta := choice.Delta.GetContentString(); textDelta != "" {
			if !ensureTextStarted() {
				sr.Stop(streamErr)
				return
			}
			outputText.WriteString(textDelta)
			if !send("response.output_text.delta", map[string]any{
				"output_index":  0,
				"content_index": 0,
				"delta":         textDelta,
			}) {
				sr.Stop(streamErr)
				return
			}
		} else if fallbackDelta := openaicompat.ChatStreamDeltaOutputText(choice.Delta); fallbackDelta != "" {
			fallbackReasoningText.WriteString(fallbackDelta)
		}
		if len(choice.Delta.ToolCalls) > 0 {
			if !ensureCreated() {
				sr.Stop(streamErr)
				return
			}
			for _, tool := range choice.Delta.ToolCalls {
				index := 0
				if tool.Index != nil {
					index = *tool.Index
				}
				state := toolStates[index]
				if state == nil {
					state = &streamToolState{Index: index}
					toolStates[index] = state
				}
				if tool.ID != "" {
					state.ID = tool.ID
				}
				if tool.Function.Name != "" {
					state.Name = tool.Function.Name
				}
				if tool.Function.Arguments != "" {
					state.Arguments.WriteString(tool.Function.Arguments)
				}
				if !ensureToolStarted(state) {
					sr.Stop(streamErr)
					return
				}
				if tool.Function.Arguments != "" {
					if !sendToolArgumentsDelta(state, tool.Function.Arguments) {
						sr.Stop(streamErr)
						return
					}
				}
			}
		}
	})
	if streamErr != nil {
		return nil, streamErr
	}

	output := make([]any, 0)
	if outputText.Len() == 0 && fallbackReasoningText.Len() > 0 {
		text := fallbackReasoningText.String()
		if !ensureTextStarted() {
			return nil, streamErr
		}
		outputText.WriteString(text)
		if !send("response.output_text.delta", map[string]any{
			"output_index":  0,
			"content_index": 0,
			"delta":         text,
		}) {
			return nil, streamErr
		}
	}
	if outputText.Len() > 0 {
		if !finalizeText() {
			return nil, streamErr
		}
		output = append(output, openaicompat.ResponseMessageItem("msg_"+responseID, outputText.String()))
	}
	for _, state := range sortedToolStates() {
		if !finalizeTool(state) {
			return nil, streamErr
		}
		output = append(output, toolItemForState(state, state.Arguments.String(), "completed"))
	}
	if usage.TotalTokens == 0 && usage.PromptTokens == 0 && usage.CompletionTokens == 0 {
		usage = service.ResponseText2Usage(c, outputText.String(), info.UpstreamModelName, info.GetEstimatePromptTokens())
	}
	responsesUsage := openaicompat.ChatUsageToResponsesUsage(usage)
	if !ensureCreated() {
		return nil, streamErr
	}
	if !send("response.completed", map[string]any{
		"response": mergeResponseUsageForRelay(baseResponse("completed", output), responsesUsage),
	}) {
		return nil, streamErr
	}
	return usage, nil
}

type streamToolState struct {
	Index     int
	ID        string
	Name      string
	Arguments strings.Builder
	ItemID    string
	Started   bool
	Done      bool
}

func mergeResponseUsageForRelay(response map[string]any, usage *dto.Usage) map[string]any {
	response["usage"] = usage
	return response
}
