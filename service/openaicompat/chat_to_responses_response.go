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
		toolCalls := msg.ParseToolCalls()
		if len(toolCalls) > 0 {
			for _, toolCall := range toolCalls {
				output = append(output, chatToolCallToResponsesItem(toolCall, ctx))
			}
		} else if text := msg.StringContent(); text != "" {
			output = append(output, responseMessageItem("msg_"+responseID, text))
		}
	}

	status := "completed"
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
	return response, usage, nil
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

func chatToolCallToResponsesItem(toolCall dto.ToolCallRequest, ctx *ResponsesToChatContext) map[string]any {
	callID := strings.TrimSpace(toolCall.ID)
	if callID == "" {
		callID = "call_" + common.GetUUID()
	}
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
	default:
		return map[string]any{
			"id":        "fc_" + callID,
			"type":      "function_call",
			"status":    "completed",
			"call_id":   callID,
			"name":      spec.Name,
			"arguments": json.RawMessage(argumentsJSON(toolCall.Function.Arguments)),
		}
	}
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
		return arguments
	}
	encoded, _ := common.Marshal(arguments)
	return string(encoded)
}
