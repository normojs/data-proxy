package openaicompat

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/samber/lo"
)

const (
	ResponsesProtocolAuto            = "auto"
	ResponsesProtocolNative          = "native"
	ResponsesProtocolChatCompletions = "chat_completions"
	ResponsesProtocolDisabled        = "disabled"
)

type ResponseToolKind string

const (
	ResponseToolKindFunction   ResponseToolKind = "function"
	ResponseToolKindCustom     ResponseToolKind = "custom"
	ResponseToolKindToolSearch ResponseToolKind = "tool_search"
)

type ResponseToolSpec struct {
	Kind     ResponseToolKind
	Name     string
	ChatName string
}

type ResponsesToChatContext struct {
	ToolsByChatName map[string]ResponseToolSpec
}

func NormalizeResponsesProtocol(protocol string) string {
	switch strings.TrimSpace(strings.ToLower(protocol)) {
	case "", ResponsesProtocolAuto:
		return ResponsesProtocolAuto
	case ResponsesProtocolNative:
		return ResponsesProtocolNative
	case ResponsesProtocolChatCompletions, "chat", "openai_chat", "chat_completions_compat":
		return ResponsesProtocolChatCompletions
	case ResponsesProtocolDisabled, "off":
		return ResponsesProtocolDisabled
	default:
		return ResponsesProtocolAuto
	}
}

func ChannelSupportsNativeResponses(channelType int) bool {
	switch channelType {
	case constant.ChannelTypeOpenAI,
		constant.ChannelTypeAzure,
		constant.ChannelTypeAli,
		constant.ChannelCloudflare,
		constant.ChannelTypeCodex,
		constant.ChannelTypePerplexity,
		constant.ChannelTypeSubmodel,
		constant.ChannelTypeVolcEngine,
		constant.ChannelTypeXai:
		return true
	default:
		return false
	}
}

func ChannelPrefersChatResponsesCompatibility(channelType int) bool {
	switch channelType {
	case constant.ChannelTypeSiliconFlow,
		constant.ChannelTypeDeepSeek,
		constant.ChannelTypeMoonshot,
		constant.ChannelTypeMiniMax,
		constant.ChannelTypeMistral,
		constant.ChannelTypeOpenRouter,
		constant.ChannelTypeOllama,
		constant.ChannelTypeTencent,
		constant.ChannelTypeZhipu_v4,
		constant.ChannelTypeBaiduV2:
		return true
	default:
		return false
	}
}

func ShouldConvertResponsesToChat(channelType int, protocol string) bool {
	switch NormalizeResponsesProtocol(protocol) {
	case ResponsesProtocolChatCompletions:
		return true
	case ResponsesProtocolNative, ResponsesProtocolDisabled:
		return false
	default:
		return !ChannelSupportsNativeResponses(channelType) && ChannelPrefersChatResponsesCompatibility(channelType)
	}
}

func IsResponsesProtocolDisabled(protocol string) bool {
	return NormalizeResponsesProtocol(protocol) == ResponsesProtocolDisabled
}

func ResponsesRequestToChatCompletionsRequest(req *dto.OpenAIResponsesRequest) (*dto.GeneralOpenAIRequest, *ResponsesToChatContext, error) {
	if req == nil {
		return nil, nil, errors.New("request is nil")
	}
	if strings.TrimSpace(req.Model) == "" {
		return nil, nil, errors.New("model is required")
	}

	ctx := &ResponsesToChatContext{ToolsByChatName: map[string]ResponseToolSpec{}}
	tools, err := convertResponsesToolsToChatTools(req.Tools, ctx)
	if err != nil {
		return nil, nil, err
	}

	messages, err := convertResponsesInputToChatMessages(req.Input, ctx)
	if err != nil {
		return nil, nil, err
	}
	if len(req.Instructions) > 0 {
		if text := strings.TrimSpace(rawJSONText(req.Instructions)); text != "" {
			messages = append([]dto.Message{{Role: "system", Content: text}}, messages...)
		}
	}
	messages = collapseSystemMessages(messages)
	if len(messages) == 0 {
		messages = []dto.Message{{Role: "user", Content: ""}}
	}

	chatReq := &dto.GeneralOpenAIRequest{
		Model:               req.Model,
		Messages:            messages,
		Stream:              req.Stream,
		StreamOptions:       req.StreamOptions,
		MaxCompletionTokens: req.MaxOutputTokens,
		Temperature:         req.Temperature,
		TopP:                req.TopP,
		Tools:               tools,
		User:                req.User,
		Metadata:            req.Metadata,
		ServiceTier:         rawStringToRawMessage(req.ServiceTier),
		Store:               req.Store,
	}
	if req.Reasoning != nil && req.Reasoning.Effort != "" {
		chatReq.ReasoningEffort = req.Reasoning.Effort
	}
	if len(req.ToolChoice) > 0 {
		chatReq.ToolChoice = responsesToolChoiceToChat(req.ToolChoice, ctx)
	}
	if len(req.ParallelToolCalls) > 0 {
		var enabled bool
		if err := common.Unmarshal(req.ParallelToolCalls, &enabled); err == nil {
			chatReq.ParallelTooCalls = lo.ToPtr(enabled)
		}
	}
	if len(req.Text) > 0 {
		chatReq.ResponseFormat = responsesTextFormatToChatResponseFormat(req.Text)
	}
	if lo.FromPtrOr(req.Stream, false) {
		if chatReq.StreamOptions == nil {
			chatReq.StreamOptions = &dto.StreamOptions{}
		}
		chatReq.StreamOptions.IncludeUsage = true
	}
	if len(chatReq.Tools) == 0 {
		chatReq.ToolChoice = nil
		chatReq.ParallelTooCalls = nil
	}

	return chatReq, ctx, nil
}

func convertResponsesToolsToChatTools(raw json.RawMessage, ctx *ResponsesToChatContext) ([]dto.ToolCallRequest, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var tools []map[string]any
	if err := common.Unmarshal(raw, &tools); err != nil {
		return nil, fmt.Errorf("invalid responses tools: %w", err)
	}

	chatTools := make([]dto.ToolCallRequest, 0, len(tools))
	for _, tool := range tools {
		toolType := strings.TrimSpace(common.Interface2String(tool["type"]))
		switch toolType {
		case "function":
			name := strings.TrimSpace(common.Interface2String(tool["name"]))
			if name == "" {
				continue
			}
			chatTools = append(chatTools, dto.ToolCallRequest{
				Type: "function",
				Function: dto.FunctionRequest{
					Name:        name,
					Description: common.Interface2String(tool["description"]),
					Parameters:  tool["parameters"],
				},
			})
			ctx.ToolsByChatName[name] = ResponseToolSpec{Kind: ResponseToolKindFunction, Name: name, ChatName: name}
		case "custom":
			name := strings.TrimSpace(common.Interface2String(tool["name"]))
			if name == "" {
				continue
			}
			chatTools = append(chatTools, dto.ToolCallRequest{
				Type: "function",
				Function: dto.FunctionRequest{
					Name:        name,
					Description: customToolDescription(tool),
					Parameters: map[string]any{
						"type": "object",
						"properties": map[string]any{
							"input": map[string]any{
								"type":        "string",
								"description": "Raw string input for the original custom tool.",
							},
						},
						"required": []string{"input"},
					},
				},
			})
			ctx.ToolsByChatName[name] = ResponseToolSpec{Kind: ResponseToolKindCustom, Name: name, ChatName: name}
		case "tool_search":
			name := "tool_search"
			chatTools = append(chatTools, dto.ToolCallRequest{
				Type: "function",
				Function: dto.FunctionRequest{
					Name:        name,
					Description: "Search and load available tools for the current task.",
					Parameters: map[string]any{
						"type": "object",
						"properties": map[string]any{
							"query": map[string]any{"type": "string"},
							"limit": map[string]any{"type": "integer"},
						},
						"required": []string{"query"},
					},
				},
			})
			ctx.ToolsByChatName[name] = ResponseToolSpec{Kind: ResponseToolKindToolSearch, Name: name, ChatName: name}
		case "namespace":
			children, _ := tool["tools"].([]any)
			namespace := strings.TrimSpace(common.Interface2String(tool["name"]))
			for _, child := range children {
				childMap, ok := child.(map[string]any)
				if !ok || common.Interface2String(childMap["type"]) != "function" {
					continue
				}
				name := strings.TrimSpace(common.Interface2String(childMap["name"]))
				if name == "" {
					continue
				}
				chatName := name
				if namespace != "" {
					chatName = namespace + "__" + name
				}
				chatTools = append(chatTools, dto.ToolCallRequest{
					Type: "function",
					Function: dto.FunctionRequest{
						Name:        chatName,
						Description: common.Interface2String(childMap["description"]),
						Parameters:  childMap["parameters"],
					},
				})
				ctx.ToolsByChatName[chatName] = ResponseToolSpec{Kind: ResponseToolKindFunction, Name: name, ChatName: chatName}
			}
		case "web_search", "web_search_preview", "file_search", "computer", "computer_use_preview", "image_generation", "code_interpreter", "mcp":
			return nil, fmt.Errorf("responses tool %q cannot be converted to chat completions; use a native Responses channel", toolType)
		default:
			return nil, fmt.Errorf("responses tool %q is not supported by chat compatibility mode", toolType)
		}
	}
	return chatTools, nil
}

func convertResponsesInputToChatMessages(raw json.RawMessage, ctx *ResponsesToChatContext) ([]dto.Message, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	if common.GetJsonType(raw) == "string" {
		return []dto.Message{{Role: "user", Content: rawJSONText(raw)}}, nil
	}

	var items []map[string]any
	if err := common.Unmarshal(raw, &items); err != nil {
		var single map[string]any
		if err2 := common.Unmarshal(raw, &single); err2 == nil {
			items = []map[string]any{single}
		} else {
			return nil, fmt.Errorf("invalid responses input: %w", err)
		}
	}

	messages := make([]dto.Message, 0, len(items))
	for _, item := range items {
		itemType := strings.TrimSpace(common.Interface2String(item["type"]))
		switch itemType {
		case "", "message":
			role := strings.TrimSpace(common.Interface2String(item["role"]))
			if role == "" {
				role = "user"
			}
			messages = append(messages, dto.Message{Role: responsesRoleToChat(role), Content: responsesContentToChatContent(item["content"], role)})
		case "input_text":
			messages = append(messages, dto.Message{Role: "user", Content: common.Interface2String(item["text"])})
		case "output_text":
			messages = append(messages, dto.Message{Role: "assistant", Content: common.Interface2String(item["text"])})
		case "function_call":
			messages = append(messages, assistantToolCallMessage(item, common.Interface2String(item["name"]), item["arguments"]))
		case "custom_tool_call":
			input := common.Interface2String(item["input"])
			messages = append(messages, assistantToolCallMessage(item, common.Interface2String(item["name"]), map[string]any{"input": input}))
		case "tool_search_call":
			messages = append(messages, assistantToolCallMessage(item, "tool_search", item["arguments"]))
		case "function_call_output", "custom_tool_call_output", "tool_search_output":
			messages = append(messages, dto.Message{
				Role:       "tool",
				ToolCallId: common.Interface2String(item["call_id"]),
				Content:    toolOutputContent(item),
			})
		case "reasoning":
			continue
		default:
			messages = append(messages, dto.Message{Role: "user", Content: stringifyJSONValue(item)})
		}
	}
	return mergeAdjacentToolCalls(messages, ctx), nil
}

func assistantToolCallMessage(item map[string]any, name string, arguments any) dto.Message {
	callID := common.Interface2String(item["call_id"])
	if callID == "" {
		callID = common.Interface2String(item["id"])
	}
	if name == "" {
		name = "tool"
	}
	args := argumentsToString(arguments)
	toolCalls := []dto.ToolCallRequest{{
		ID:   callID,
		Type: "function",
		Function: dto.FunctionRequest{
			Name:      name,
			Arguments: args,
		},
	}}
	msg := dto.Message{Role: "assistant", Content: nil}
	msg.SetToolCalls(toolCalls)
	return msg
}

func mergeAdjacentToolCalls(messages []dto.Message, _ *ResponsesToChatContext) []dto.Message {
	if len(messages) < 2 {
		return messages
	}
	merged := make([]dto.Message, 0, len(messages))
	for _, msg := range messages {
		if msg.Role == "assistant" && msg.Content == nil && len(msg.ParseToolCalls()) > 0 && len(merged) > 0 {
			prev := &merged[len(merged)-1]
			if prev.Role == "assistant" {
				toolCalls := prev.ParseToolCalls()
				toolCalls = append(toolCalls, msg.ParseToolCalls()...)
				prev.SetToolCalls(toolCalls)
				if prev.Content == "" {
					prev.Content = nil
				}
				continue
			}
		}
		merged = append(merged, msg)
	}
	return merged
}

func responsesContentToChatContent(value any, role string) any {
	if value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return v
	case []any:
		parts := make([]map[string]any, 0, len(v))
		for _, rawPart := range v {
			part, ok := rawPart.(map[string]any)
			if !ok {
				continue
			}
			switch common.Interface2String(part["type"]) {
			case "input_text", "output_text":
				parts = append(parts, map[string]any{
					"type": "text",
					"text": common.Interface2String(part["text"]),
				})
			case "input_image":
				parts = append(parts, map[string]any{
					"type":      "image_url",
					"image_url": normalizeResponsesImageURL(part["image_url"]),
				})
			case "input_file":
				parts = append(parts, map[string]any{
					"type": "file",
					"file": firstNonEmptyValue(part["file"], part["file_url"]),
				})
			default:
				parts = append(parts, part)
			}
		}
		if len(parts) == 1 && role != "user" && common.Interface2String(parts[0]["type"]) == "text" {
			return common.Interface2String(parts[0]["text"])
		}
		return parts
	default:
		return v
	}
}

func responsesRoleToChat(role string) string {
	switch role {
	case "developer":
		return "system"
	case "assistant", "system", "tool", "user":
		return role
	default:
		return "user"
	}
}

func collapseSystemMessages(messages []dto.Message) []dto.Message {
	var systemParts []string
	out := make([]dto.Message, 0, len(messages))
	for _, msg := range messages {
		if msg.Role == "system" || msg.Role == "developer" {
			if text := strings.TrimSpace(messageContentText(msg.Content)); text != "" {
				systemParts = append(systemParts, text)
			}
			continue
		}
		out = append(out, msg)
	}
	if len(systemParts) == 0 {
		return out
	}
	return append([]dto.Message{{Role: "system", Content: strings.Join(systemParts, "\n\n")}}, out...)
}

func responsesToolChoiceToChat(raw json.RawMessage, ctx *ResponsesToChatContext) any {
	if len(raw) == 0 {
		return nil
	}
	var s string
	if err := common.Unmarshal(raw, &s); err == nil {
		return s
	}
	var m map[string]any
	if err := common.Unmarshal(raw, &m); err != nil {
		return raw
	}
	if common.Interface2String(m["type"]) == "function" {
		name := common.Interface2String(m["name"])
		if spec, ok := ctx.ToolsByChatName[name]; ok && spec.ChatName != "" {
			name = spec.ChatName
		}
		if name != "" {
			return map[string]any{
				"type": "function",
				"function": map[string]any{
					"name": name,
				},
			}
		}
	}
	return m
}

func responsesTextFormatToChatResponseFormat(raw json.RawMessage) *dto.ResponseFormat {
	var wrapper map[string]any
	if err := common.Unmarshal(raw, &wrapper); err != nil {
		return nil
	}
	format, ok := wrapper["format"].(map[string]any)
	if !ok {
		return nil
	}
	formatType := common.Interface2String(format["type"])
	if formatType == "" {
		return nil
	}
	out := &dto.ResponseFormat{Type: formatType}
	if formatType == "json_schema" {
		b, _ := common.Marshal(format)
		out.JsonSchema = b
	}
	return out
}

func customToolDescription(tool map[string]any) string {
	var parts []string
	if desc := strings.TrimSpace(common.Interface2String(tool["description"])); desc != "" {
		parts = append(parts, desc)
	}
	if len(tool) > 0 {
		parts = append(parts, "Original tool definition: "+stringifyJSONValue(tool))
	}
	return strings.Join(parts, "\n\n")
}

func toolOutputContent(item map[string]any) string {
	if output, ok := item["output"]; ok {
		return valueToOutputString(output)
	}
	if output, ok := item["content"]; ok {
		return valueToOutputString(output)
	}
	if results, ok := item["results"]; ok {
		return valueToOutputString(results)
	}
	return stringifyJSONValue(item)
}

func messageContentText(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	case []any:
		var sb strings.Builder
		for _, item := range v {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if common.Interface2String(m["type"]) == "text" {
				sb.WriteString(common.Interface2String(m["text"]))
			}
		}
		return sb.String()
	default:
		return stringifyJSONValue(v)
	}
}

func rawJSONText(raw json.RawMessage) string {
	var s string
	if err := common.Unmarshal(raw, &s); err == nil {
		return s
	}
	return strings.TrimSpace(string(raw))
}

func rawStringToRawMessage(value string) json.RawMessage {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	raw, _ := common.Marshal(value)
	return raw
}

func stringifyJSONValue(value any) string {
	b, err := common.Marshal(value)
	if err != nil {
		return fmt.Sprintf("%v", value)
	}
	return string(b)
}

func argumentsToString(value any) string {
	switch v := value.(type) {
	case nil:
		return "{}"
	case string:
		if strings.TrimSpace(v) == "" {
			return "{}"
		}
		return v
	case json.RawMessage:
		if len(v) == 0 {
			return "{}"
		}
		return string(v)
	default:
		return stringifyJSONValue(v)
	}
}

func valueToOutputString(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	default:
		return stringifyJSONValue(v)
	}
}

func normalizeResponsesImageURL(value any) any {
	switch v := value.(type) {
	case string:
		return map[string]any{"url": v}
	case map[string]any:
		if _, ok := v["url"]; ok {
			return v
		}
		return map[string]any{"url": common.Interface2String(v)}
	default:
		return value
	}
}

func firstNonEmptyValue(values ...any) any {
	for _, value := range values {
		if value == nil {
			continue
		}
		if s, ok := value.(string); ok && strings.TrimSpace(s) == "" {
			continue
		}
		return value
	}
	return nil
}
