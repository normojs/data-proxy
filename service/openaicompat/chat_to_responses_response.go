package openaicompat

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

func ChatCompletionResponseToResponses(chat *dto.OpenAITextResponse, req *dto.OpenAIResponsesRequest, ctx *ResponsesToChatContext) (map[string]any, *dto.Usage, error) {
	if chat == nil {
		return nil, nil, fmt.Errorf("chat response is nil")
	}

	usage := chatUsageToResponsesUsage(&chat.Usage)
	responseID := ResponseIDFromChatID(chat.Id)
	model := chat.Model
	if model == "" && req != nil {
		model = req.Model
	}
	created := chatCreatedAt(chat.Created)
	output := make([]any, 0)

	if len(chat.Choices) > 0 {
		msg := chat.Choices[0].Message
		reasoning := strings.TrimSpace(msg.GetReasoningContent())
		toolCalls := msg.ParseToolCalls()
		if len(toolCalls) > 0 {
			if reasoning != "" {
				output = append(output, responseReasoningItem("rs_"+responseID, reasoning))
			}
			if text := msg.StringContent(); text != "" {
				output = append(output, responseMessageItem("msg_"+responseID, text))
			}
			for _, toolCall := range toolCalls {
				item := chatToolCallToResponsesItem(toolCall, ctx)
				setResponseItemReasoningText(item, reasoning)
				output = append(output, item)
			}
		} else if text := msg.StringContent(); text != "" {
			if reasoning != "" {
				output = append(output, responseReasoningItem("rs_"+responseID, reasoning))
			}
			output = append(output, responseMessageItem("msg_"+responseID, text))
		} else if reasoning != "" {
			output = append(output, responseMessageItem("msg_"+responseID, reasoning))
		}
	}

	finishReason := ""
	if len(chat.Choices) > 0 {
		finishReason = chat.Choices[0].FinishReason
	}
	status := responseStatusFromChatFinishReason(finishReason)
	response := map[string]any{
		"id":                  responseID,
		"object":              "response",
		"created_at":          created,
		"status":              status,
		"model":               model,
		"output":              output,
		"parallel_tool_calls": true,
		"usage":               usage,
	}
	if status == "incomplete" {
		response["incomplete_details"] = map[string]any{"reason": "max_output_tokens"}
	}
	if req != nil {
		if req.PreviousResponseID != "" {
			response["previous_response_id"] = req.PreviousResponseID
		}
		if req.Instructions != nil {
			response["instructions"] = json.RawMessage(req.Instructions)
		}
		if req.Tools != nil {
			response["tools"] = json.RawMessage(req.Tools)
		}
		if req.ToolChoice != nil {
			response["tool_choice"] = json.RawMessage(req.ToolChoice)
		}
		if req.Reasoning != nil {
			response["reasoning"] = req.Reasoning
		}
	}
	if recorded := DefaultResponsesChatHistory().RecordResponseMap(response); recorded > 0 && ctx != nil {
		ctx.HistoryRecordedCount += recorded
	}
	return response, usage, nil
}

func responseStatusFromChatFinishReason(finishReason string) string {
	if strings.TrimSpace(finishReason) == "length" {
		return "incomplete"
	}
	return "completed"
}

func ChatUsageToResponsesUsage(usage *dto.Usage) *dto.Usage {
	return chatUsageToResponsesUsage(usage)
}

func ResponseMessageItem(id string, text string) map[string]any {
	return responseMessageItem(id, text)
}

func ChatToolCallToResponsesItem(toolCall dto.ToolCallRequest, ctx *ResponsesToChatContext) map[string]any {
	return chatToolCallToResponsesItem(toolCall, ctx)
}

func ResponseIDFromChatID(chatID string) string {
	chatID = strings.TrimSpace(chatID)
	chatID = strings.TrimPrefix(chatID, "chatcmpl-")
	chatID = strings.TrimPrefix(chatID, "cmpl-")
	if chatID == "" {
		chatID = common.GetUUID()
	}
	chatID = strings.NewReplacer("/", "_", ":", "_", " ", "_").Replace(chatID)
	if strings.HasPrefix(chatID, "resp_") {
		return chatID
	}
	return "resp_" + chatID
}

func ChatStreamIDToResponsesID(chatID string) string {
	return ResponseIDFromChatID(chatID)
}

func ChatMessageOutputText(message dto.Message) string {
	if text := message.StringContent(); text != "" {
		return text
	}
	return message.GetReasoningContent()
}

func ChatStreamDeltaOutputText(delta dto.ChatCompletionsStreamResponseChoiceDelta) string {
	if text := delta.GetContentString(); text != "" {
		return text
	}
	return delta.GetReasoningContent()
}

func chatCreatedAt(value any) int64 {
	switch v := value.(type) {
	case int64:
		return v
	case int:
		return int64(v)
	case int32:
		return int64(v)
	case float64:
		return int64(v)
	case json.Number:
		if n, err := v.Int64(); err == nil {
			return n
		}
	default:
	}
	return time.Now().Unix()
}

func chatUsageToResponsesUsage(usage *dto.Usage) *dto.Usage {
	out := &dto.Usage{}
	if usage == nil {
		return out
	}
	out.InputTokens = firstPositive(usage.InputTokens, usage.PromptTokens)
	out.OutputTokens = firstPositive(usage.OutputTokens, usage.CompletionTokens)
	out.TotalTokens = usage.TotalTokens
	if out.TotalTokens == 0 {
		out.TotalTokens = out.InputTokens + out.OutputTokens
	}
	out.InputTokensDetails = &dto.InputTokenDetails{
		CachedTokens: usage.PromptTokensDetails.CachedTokens,
		TextTokens:   usage.PromptTokensDetails.TextTokens,
		AudioTokens:  usage.PromptTokensDetails.AudioTokens,
		ImageTokens:  usage.PromptTokensDetails.ImageTokens,
	}
	out.CompletionTokenDetails = usage.CompletionTokenDetails
	return out
}

func ResponsesUsageToChatUsage(usage *dto.Usage) *dto.Usage {
	out := &dto.Usage{}
	if usage == nil {
		return out
	}
	out.PromptTokens = firstPositive(usage.PromptTokens, usage.InputTokens)
	out.CompletionTokens = firstPositive(usage.CompletionTokens, usage.OutputTokens)
	out.TotalTokens = usage.TotalTokens
	if out.TotalTokens == 0 {
		out.TotalTokens = out.PromptTokens + out.CompletionTokens
	}
	if usage.InputTokensDetails != nil {
		out.PromptTokensDetails.CachedTokens = usage.InputTokensDetails.CachedTokens
		out.PromptTokensDetails.TextTokens = usage.InputTokensDetails.TextTokens
		out.PromptTokensDetails.AudioTokens = usage.InputTokensDetails.AudioTokens
		out.PromptTokensDetails.ImageTokens = usage.InputTokensDetails.ImageTokens
	}
	out.CompletionTokenDetails = usage.CompletionTokenDetails
	out.InputTokens = out.PromptTokens
	out.OutputTokens = out.CompletionTokens
	return out
}

func firstPositive(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func responseMessageItem(id string, text string) map[string]any {
	return map[string]any{
		"id":     id,
		"type":   "message",
		"status": "completed",
		"role":   "assistant",
		"content": []map[string]any{
			{
				"type":        "output_text",
				"text":        text,
				"annotations": []any{},
			},
		},
	}
}

func responseReasoningItem(id string, text string) map[string]any {
	return map[string]any{
		"id":     id,
		"type":   "reasoning",
		"status": "completed",
		"summary": []map[string]any{
			{
				"type": "summary_text",
				"text": text,
			},
		},
	}
}

func setResponseItemReasoningText(item map[string]any, reasoning string) {
	reasoning = strings.TrimSpace(reasoning)
	if item == nil || reasoning == "" {
		return
	}
	item["reasoning_content"] = reasoning
}

func responseItemReasoningText(item map[string]any) string {
	if item == nil {
		return ""
	}
	values := []string{
		dto.ExtractReasoningText(item["reasoning_content"]),
		dto.ExtractReasoningText(item["reasoning"]),
		dto.ExtractReasoningText(item["reasoning_details"]),
		dto.ExtractReasoningText(item["text"]),
	}
	if summary := collectTextFromResponseParts(item["summary"]); summary != "" {
		values = append(values, summary)
	}
	if content := collectTextFromResponseParts(item["content"]); content != "" {
		values = append(values, content)
	}
	return joinReasoningText(values...)
}

func collectTextFromResponseParts(value any) string {
	switch parts := value.(type) {
	case []any:
		texts := make([]string, 0, len(parts))
		for _, rawPart := range parts {
			part, ok := rawPart.(map[string]any)
			if !ok {
				continue
			}
			texts = append(texts, common.Interface2String(part["text"]))
		}
		return joinReasoningText(texts...)
	case []map[string]any:
		texts := make([]string, 0, len(parts))
		for _, part := range parts {
			texts = append(texts, common.Interface2String(part["text"]))
		}
		return joinReasoningText(texts...)
	default:
		return ""
	}
}

func chatToolCallToResponsesItem(toolCall dto.ToolCallRequest, ctx *ResponsesToChatContext) map[string]any {
	callID := normalizeResponsesCallID(toolCall.ID)
	chatName := strings.TrimSpace(toolCall.Function.Name)
	spec := ResponseToolSpec{Kind: ResponseToolKindFunction, Name: chatName, ChatName: chatName}
	if ctx != nil {
		if found, ok := ctx.ToolsByChatName[chatName]; ok {
			spec = found
		}
	}

	switch spec.Kind {
	case ResponseToolKindCustom:
		return map[string]any{
			"id":      "ctc_" + callID,
			"type":    "custom_tool_call",
			"status":  "completed",
			"call_id": callID,
			"name":    spec.Name,
			"input":   customToolInput(toolCall.Function.Arguments),
		}
	case ResponseToolKindToolSearch:
		return map[string]any{
			"id":        "tsc_" + callID,
			"type":      "tool_search_call",
			"status":    "completed",
			"call_id":   callID,
			"arguments": json.RawMessage(argumentsJSON(toolCall.Function.Arguments)),
		}
	case ResponseToolKindHosted:
		return hostedToolCallToResponsesItem(toolCall, spec, callID)
	default:
		item := map[string]any{
			"id":        "fc_" + callID,
			"type":      "function_call",
			"status":    "completed",
			"call_id":   callID,
			"name":      spec.Name,
			"arguments": functionCallArgumentsString(toolCall.Function.Arguments),
		}
		if spec.Namespace != "" {
			item["namespace"] = spec.Namespace
		}
		return item
	}
}

func normalizeResponsesCallID(callID string) string {
	callID = strings.TrimSpace(callID)
	if callID == "" {
		callID = common.GetUUID()
	}
	callID = strings.NewReplacer("/", "_", ":", "_", " ", "_").Replace(callID)
	if strings.HasPrefix(callID, "call_") {
		return callID
	}
	return "call_" + callID
}

func hostedToolCallToResponsesItem(toolCall dto.ToolCallRequest, spec ResponseToolSpec, callID string) map[string]any {
	hostedType := normalizeHostedToolType(spec.HostedType)
	args := parseToolArgumentsMap(toolCall.Function.Arguments)
	item := map[string]any{
		"id":      hostedCallItemID(hostedType, callID),
		"type":    hostedCallItemType(hostedType),
		"status":  "completed",
		"call_id": callID,
	}
	if spec.Name != "" {
		item["name"] = spec.Name
	}
	switch hostedType {
	case "web_search":
		if query := firstStringValue(args, "query", "q", "search_query"); query != "" {
			item["action"] = map[string]any{
				"type":  "search",
				"query": query,
			}
		}
	case "file_search":
		if query := firstStringValue(args, "query", "q"); query != "" {
			item["queries"] = []string{query}
		}
		if results, ok := args["results"]; ok {
			item["results"] = results
		}
	case "computer":
		if action, ok := args["action"]; ok {
			item["action"] = action
		}
		if pending, ok := args["pending_safety_checks"]; ok {
			item["pending_safety_checks"] = pending
		} else {
			item["pending_safety_checks"] = []any{}
		}
	case "image_generation":
		item["arguments"] = json.RawMessage(argumentsJSON(toolCall.Function.Arguments))
	case "code_interpreter":
		if code := firstStringValue(args, "code", "input"); code != "" {
			item["code"] = code
		}
		if language := firstStringValue(args, "language"); language != "" {
			item["language"] = language
		}
	case "mcp":
		if toolName := firstStringValue(args, "tool_name", "name"); toolName != "" {
			item["tool_name"] = toolName
		}
		if arguments, ok := args["arguments"]; ok {
			item["arguments"] = arguments
		} else {
			item["arguments"] = json.RawMessage(argumentsJSON(toolCall.Function.Arguments))
		}
	default:
		item["arguments"] = json.RawMessage(argumentsJSON(toolCall.Function.Arguments))
	}
	return item
}

func hostedCallItemType(hostedType string) string {
	switch normalizeHostedToolType(hostedType) {
	case "web_search":
		return "web_search_call"
	case "file_search":
		return "file_search_call"
	case "computer":
		return "computer_call"
	case "image_generation":
		return "image_generation_call"
	case "code_interpreter":
		return "code_interpreter_call"
	case "mcp":
		return "mcp_call"
	default:
		return "function_call"
	}
}

func hostedCallItemID(hostedType string, callID string) string {
	switch normalizeHostedToolType(hostedType) {
	case "web_search":
		return "wsc_" + callID
	case "file_search":
		return "fsc_" + callID
	case "computer":
		return "cc_" + callID
	case "image_generation":
		return "igc_" + callID
	case "code_interpreter":
		return "cic_" + callID
	case "mcp":
		return "mcpc_" + callID
	default:
		return "fc_" + callID
	}
}

func parseToolArgumentsMap(arguments string) map[string]any {
	var obj map[string]any
	if err := common.Unmarshal([]byte(strings.TrimSpace(arguments)), &obj); err == nil && obj != nil {
		return obj
	}
	return map[string]any{}
}

func firstStringValue(values map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(common.Interface2String(values[key])); value != "" {
			return value
		}
	}
	return ""
}

func customToolInput(arguments string) string {
	var obj map[string]any
	if err := common.Unmarshal([]byte(arguments), &obj); err == nil {
		if input, ok := obj["input"]; ok {
			return common.Interface2String(input)
		}
	}
	return arguments
}

func argumentsJSON(arguments string) string {
	arguments = strings.TrimSpace(arguments)
	if arguments == "" {
		return "{}"
	}
	var js any
	if err := common.Unmarshal([]byte(arguments), &js); err == nil {
		return canonicalizeJSONStringIfParseable(arguments)
	}
	encoded, _ := common.Marshal(arguments)
	return string(encoded)
}

func functionCallArgumentsString(arguments string) string {
	arguments = strings.TrimSpace(arguments)
	if arguments == "" {
		return ""
	}
	return argumentsJSON(arguments)
}
