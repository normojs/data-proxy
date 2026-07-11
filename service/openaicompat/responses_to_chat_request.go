package openaicompat

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
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

const (
	ResponsesReasoningAdapterLegacy                = "legacy"
	ResponsesReasoningAdapterAuto                  = "auto"
	ResponsesReasoningAdapterOff                   = "off"
	ResponsesReasoningAdapterOpenAI                = "openai"
	ResponsesReasoningAdapterDeepSeek              = "deepseek"
	ResponsesReasoningAdapterOpenRouter            = "openrouter"
	ResponsesReasoningAdapterQwenEnableThinking    = "qwen_enable_thinking"
	ResponsesReasoningAdapterMiniMaxReasoningSplit = "minimax_reasoning_split"
	ResponsesReasoningAdapterLowHigh               = "low_high"
)

type ResponsesToChatOptions struct {
	ChannelType      int
	ReasoningAdapter string
}

type ResponseToolKind string

const (
	ResponseToolKindFunction   ResponseToolKind = "function"
	ResponseToolKindCustom     ResponseToolKind = "custom"
	ResponseToolKindToolSearch ResponseToolKind = "tool_search"
	ResponseToolKindHosted     ResponseToolKind = "hosted"
)

type ResponseToolSpec struct {
	Kind       ResponseToolKind
	Name       string
	ChatName   string
	HostedType string
	Namespace  string
}

type ResponsesToChatContext struct {
	ToolsByChatName map[string]ResponseToolSpec

	HistoryRestoredCount     int
	HistoryRestoreSources    []string
	HistoryRecordedCount     int
	InputProvidedToolsCount  int
	NamespaceToolsFlattened  int
	ReasoningBackfilledCount int
	HostedToolsFunctionized  []string
	HostedToolsFiltered      []string
	HostedToolsDirectHint    bool
	UnsupportedToolsFiltered []string
	ReasoningAdapter         string
	ReasoningAdapterSource   string
	ReasoningForwarded       bool
	ReasoningParams          []string
	ReasoningEffort          string
}

func (ctx *ResponsesToChatContext) RequestConversionMeta() map[string]interface{} {
	if ctx == nil {
		return nil
	}
	meta := map[string]interface{}{}
	if ctx.HistoryRestoredCount > 0 {
		meta["history_restored_count"] = ctx.HistoryRestoredCount
	}
	if sources := uniqueSortedStrings(ctx.HistoryRestoreSources); len(sources) > 0 {
		meta["history_restore_sources"] = sources
	}
	if ctx.HistoryRecordedCount > 0 {
		meta["history_recorded_count"] = ctx.HistoryRecordedCount
	}
	if ctx.InputProvidedToolsCount > 0 {
		meta["input_provided_tools_count"] = ctx.InputProvidedToolsCount
	}
	if ctx.NamespaceToolsFlattened > 0 {
		meta["namespace_tools_flattened"] = ctx.NamespaceToolsFlattened
	}
	if ctx.ReasoningBackfilledCount > 0 {
		meta["reasoning_backfilled_count"] = ctx.ReasoningBackfilledCount
	}
	if hostedTools := uniqueSortedStrings(ctx.HostedToolsFunctionized); len(hostedTools) > 0 {
		meta["hosted_tools_functionized"] = hostedTools
	}
	if hostedTools := uniqueSortedStrings(ctx.HostedToolsFiltered); len(hostedTools) > 0 {
		meta["hosted_tools_filtered"] = hostedTools
	}
	if ctx.HostedToolsDirectHint {
		meta["hosted_tools_direct_answer_hint"] = true
	}
	if unsupportedTools := uniqueSortedStrings(ctx.UnsupportedToolsFiltered); len(unsupportedTools) > 0 {
		meta["unsupported_tools_filtered"] = unsupportedTools
	}
	if ctx.ReasoningAdapter != "" && ctx.ReasoningAdapter != ResponsesReasoningAdapterLegacy {
		meta["responses_reasoning_adapter"] = ctx.ReasoningAdapter
		if ctx.ReasoningAdapterSource != "" {
			meta["responses_reasoning_adapter_source"] = ctx.ReasoningAdapterSource
		}
	}
	if ctx.ReasoningForwarded {
		meta["reasoning_forwarded"] = true
	}
	if params := uniqueSortedStrings(ctx.ReasoningParams); len(params) > 0 {
		meta["reasoning_params"] = params
	}
	if ctx.ReasoningEffort != "" {
		meta["reasoning_effort_mapped"] = ctx.ReasoningEffort
	}
	return meta
}

func (ctx *ResponsesToChatContext) addHostedToolFunctionized(hostedType string) {
	if ctx == nil {
		return
	}
	hostedType = strings.TrimSpace(hostedType)
	if hostedType == "" {
		return
	}
	ctx.HostedToolsFunctionized = append(ctx.HostedToolsFunctionized, hostedType)
}

func (ctx *ResponsesToChatContext) addHostedToolFiltered(hostedType string) {
	if ctx == nil {
		return
	}
	hostedType = strings.TrimSpace(hostedType)
	if hostedType == "" {
		return
	}
	ctx.HostedToolsFiltered = append(ctx.HostedToolsFiltered, hostedType)
}

func (ctx *ResponsesToChatContext) addUnsupportedToolFiltered(toolType string) {
	if ctx == nil {
		return
	}
	toolType = strings.TrimSpace(toolType)
	if toolType == "" {
		toolType = "unknown"
	}
	ctx.UnsupportedToolsFiltered = append(ctx.UnsupportedToolsFiltered, toolType)
}

func (ctx *ResponsesToChatContext) addReasoningParam(param string) {
	if ctx == nil {
		return
	}
	param = strings.TrimSpace(param)
	if param == "" {
		return
	}
	ctx.ReasoningParams = append(ctx.ReasoningParams, param)
	ctx.ReasoningForwarded = true
}

func uniqueSortedStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
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
		constant.ChannelTypeMistral,
		constant.ChannelTypeOpenRouter,
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

func NormalizeResponsesReasoningAdapter(adapter string) string {
	switch strings.TrimSpace(strings.ToLower(adapter)) {
	case "", "default", ResponsesReasoningAdapterLegacy:
		return ResponsesReasoningAdapterLegacy
	case ResponsesReasoningAdapterAuto:
		return ResponsesReasoningAdapterAuto
	case ResponsesReasoningAdapterOff, "disabled", "none":
		return ResponsesReasoningAdapterOff
	case ResponsesReasoningAdapterOpenAI, "reasoning_effort":
		return ResponsesReasoningAdapterOpenAI
	case ResponsesReasoningAdapterDeepSeek:
		return ResponsesReasoningAdapterDeepSeek
	case ResponsesReasoningAdapterOpenRouter, "reasoning.effort":
		return ResponsesReasoningAdapterOpenRouter
	case ResponsesReasoningAdapterQwenEnableThinking, "qwen", "enable_thinking":
		return ResponsesReasoningAdapterQwenEnableThinking
	case ResponsesReasoningAdapterMiniMaxReasoningSplit, "minimax", "reasoning_split":
		return ResponsesReasoningAdapterMiniMaxReasoningSplit
	case ResponsesReasoningAdapterLowHigh:
		return ResponsesReasoningAdapterLowHigh
	default:
		return ResponsesReasoningAdapterLegacy
	}
}

type responsesReasoningConfig struct {
	supportsThinking bool
	thinkingParam    string
	supportsEffort   bool
	effortParam      string
	effortValueMode  string
}

func applyResponsesReasoningToChat(chatReq *dto.GeneralOpenAIRequest, req *dto.OpenAIResponsesRequest, opts *ResponsesToChatOptions, ctx *ResponsesToChatContext) {
	if chatReq == nil || req == nil || req.Reasoning == nil {
		return
	}
	adapter := ResponsesReasoningAdapterLegacy
	channelType := 0
	if opts != nil {
		adapter = NormalizeResponsesReasoningAdapter(opts.ReasoningAdapter)
		channelType = opts.ChannelType
	}
	source := "configured"
	if adapter == ResponsesReasoningAdapterAuto {
		adapter = inferResponsesReasoningAdapter(channelType)
		source = "auto"
	}
	if ctx != nil {
		ctx.ReasoningAdapter = adapter
		ctx.ReasoningAdapterSource = source
	}
	if adapter == ResponsesReasoningAdapterOff {
		return
	}
	if adapter == ResponsesReasoningAdapterLegacy {
		if strings.TrimSpace(req.Reasoning.Effort) != "" {
			chatReq.ReasoningEffort = req.Reasoning.Effort
			if ctx != nil {
				ctx.ReasoningEffort = req.Reasoning.Effort
				ctx.addReasoningParam("reasoning_effort")
			}
		}
		return
	}

	config, ok := responsesReasoningConfigForAdapter(adapter)
	if !ok {
		return
	}
	reasoningEnabled, explicit := responsesReasoningRequested(req)
	if !explicit {
		return
	}
	if config.supportsThinking {
		switch config.thinkingParam {
		case "thinking":
			chatReq.THINKING = mustMarshalRaw(map[string]any{
				"type": lo.Ternary(reasoningEnabled, "enabled", "disabled"),
			})
			if ctx != nil {
				ctx.addReasoningParam("thinking")
			}
		case "enable_thinking":
			chatReq.EnableThinking = mustMarshalRaw(reasoningEnabled)
			if ctx != nil {
				ctx.addReasoningParam("enable_thinking")
			}
		case "reasoning_split":
			chatReq.ReasoningSplit = mustMarshalRaw(reasoningEnabled)
			if ctx != nil {
				ctx.addReasoningParam("reasoning_split")
			}
		}
	}

	effortParam := config.effortParam
	if effortParam == "" {
		effortParam = "reasoning_effort"
	}
	if !reasoningEnabled {
		if effortParam == "reasoning.effort" {
			chatReq.Reasoning = mustMarshalRaw(map[string]any{"effort": "none"})
			if ctx != nil {
				ctx.ReasoningEffort = "none"
				ctx.addReasoningParam("reasoning.effort")
			}
		}
		return
	}
	if !config.supportsEffort {
		return
	}
	mapped := mapResponsesReasoningEffort(req.Reasoning.Effort, config.effortValueMode)
	if mapped == "" {
		return
	}
	switch effortParam {
	case "reasoning_effort":
		chatReq.ReasoningEffort = mapped
		if ctx != nil {
			ctx.ReasoningEffort = mapped
			ctx.addReasoningParam("reasoning_effort")
		}
	case "reasoning.effort":
		chatReq.Reasoning = mustMarshalRaw(map[string]any{"effort": mapped})
		if ctx != nil {
			ctx.ReasoningEffort = mapped
			ctx.addReasoningParam("reasoning.effort")
		}
	}
}

func inferResponsesReasoningAdapter(channelType int) string {
	switch channelType {
	case constant.ChannelTypeDeepSeek:
		return ResponsesReasoningAdapterDeepSeek
	case constant.ChannelTypeOpenRouter:
		return ResponsesReasoningAdapterOpenRouter
	case constant.ChannelTypeMiniMax:
		return ResponsesReasoningAdapterMiniMaxReasoningSplit
	case constant.ChannelTypeAli:
		return ResponsesReasoningAdapterQwenEnableThinking
	default:
		return ResponsesReasoningAdapterLegacy
	}
}

func InferResponsesReasoningAdapter(channelType int) string {
	return inferResponsesReasoningAdapter(channelType)
}

func responsesReasoningConfigForAdapter(adapter string) (responsesReasoningConfig, bool) {
	switch adapter {
	case ResponsesReasoningAdapterOpenAI:
		return responsesReasoningConfig{
			supportsEffort:  true,
			effortParam:     "reasoning_effort",
			effortValueMode: "passthrough",
		}, true
	case ResponsesReasoningAdapterDeepSeek:
		return responsesReasoningConfig{
			supportsThinking: true,
			thinkingParam:    "thinking",
			supportsEffort:   true,
			effortParam:      "reasoning_effort",
			effortValueMode:  "deepseek",
		}, true
	case ResponsesReasoningAdapterOpenRouter:
		return responsesReasoningConfig{
			supportsEffort:  true,
			effortParam:     "reasoning.effort",
			effortValueMode: "openrouter",
		}, true
	case ResponsesReasoningAdapterQwenEnableThinking:
		return responsesReasoningConfig{
			supportsThinking: true,
			thinkingParam:    "enable_thinking",
		}, true
	case ResponsesReasoningAdapterMiniMaxReasoningSplit:
		return responsesReasoningConfig{
			supportsThinking: true,
			thinkingParam:    "reasoning_split",
		}, true
	case ResponsesReasoningAdapterLowHigh:
		return responsesReasoningConfig{
			supportsEffort:  true,
			effortParam:     "reasoning_effort",
			effortValueMode: "low_high",
		}, true
	default:
		return responsesReasoningConfig{}, false
	}
}

func responsesReasoningRequested(req *dto.OpenAIResponsesRequest) (bool, bool) {
	if req == nil || req.Reasoning == nil {
		return false, false
	}
	effort := strings.TrimSpace(strings.ToLower(req.Reasoning.Effort))
	if effort != "" {
		return !isDisabledReasoningEffort(effort), true
	}
	return true, true
}

func isDisabledReasoningEffort(effort string) bool {
	switch strings.TrimSpace(strings.ToLower(effort)) {
	case "none", "off", "disabled":
		return true
	default:
		return false
	}
}

func mapResponsesReasoningEffort(effort string, mode string) string {
	effort = strings.TrimSpace(strings.ToLower(effort))
	if effort == "" || isDisabledReasoningEffort(effort) {
		return ""
	}
	switch strings.TrimSpace(strings.ToLower(mode)) {
	case "deepseek":
		if effort == "max" || effort == "xhigh" {
			return "max"
		}
		return "high"
	case "low_high":
		if effort == "minimal" || effort == "low" {
			return "low"
		}
		return "high"
	case "openrouter":
		switch effort {
		case "max", "xhigh":
			return "xhigh"
		case "high", "medium", "low", "minimal":
			return effort
		default:
			return ""
		}
	default:
		switch effort {
		case "minimal", "low", "medium", "high", "xhigh", "max":
			return effort
		default:
			return ""
		}
	}
}

func mustMarshalRaw(value any) json.RawMessage {
	raw, _ := common.Marshal(value)
	return raw
}

func ResponsesRequestToChatCompletionsRequest(req *dto.OpenAIResponsesRequest) (*dto.GeneralOpenAIRequest, *ResponsesToChatContext, error) {
	return ResponsesRequestToChatCompletionsRequestWithOptions(req, nil)
}

func ResponsesRequestToChatCompletionsRequestWithOptions(req *dto.OpenAIResponsesRequest, opts *ResponsesToChatOptions) (*dto.GeneralOpenAIRequest, *ResponsesToChatContext, error) {
	if req == nil {
		return nil, nil, errors.New("request is nil")
	}
	if strings.TrimSpace(req.Model) == "" {
		return nil, nil, errors.New("model is required")
	}

	ctx := &ResponsesToChatContext{ToolsByChatName: map[string]ResponseToolSpec{}}
	enriched := DefaultResponsesChatHistory().EnrichRequestWithMeta(req)
	ctx.HistoryRestoredCount = enriched.Count
	ctx.HistoryRestoreSources = enriched.Sources
	tools, err := convertResponsesToolsToChatTools(req.Tools, ctx)
	if err != nil {
		return nil, nil, err
	}
	tools = append(tools, collectResponsesInputProvidedChatTools(req.Input, ctx)...)

	messages, err := convertResponsesInputToChatMessages(req.Input, ctx)
	if err != nil {
		return nil, nil, err
	}
	if len(req.Instructions) > 0 {
		if text := strings.TrimSpace(rawJSONText(req.Instructions)); text != "" {
			messages = append([]dto.Message{{Role: "system", Content: text}}, messages...)
		}
	}
	if text := hostedToolFallbackInstruction(ctx); text != "" {
		messages = append([]dto.Message{{Role: "system", Content: text}}, messages...)
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
	applyResponsesReasoningToChat(chatReq, req, opts, ctx)
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
	var tools []any
	if err := common.Unmarshal(raw, &tools); err != nil {
		return nil, fmt.Errorf("invalid responses tools: %w", err)
	}

	return convertResponsesToolValuesToChatTools(tools, ctx), nil
}

func convertResponsesToolValuesToChatTools(tools []any, ctx *ResponsesToChatContext) []dto.ToolCallRequest {
	chatTools := make([]dto.ToolCallRequest, 0, len(tools))
	for _, rawTool := range tools {
		var tool map[string]any
		switch value := rawTool.(type) {
		case string:
			tool = map[string]any{"type": "custom", "name": value}
		case map[string]any:
			tool = value
		default:
			continue
		}

		toolType := strings.TrimSpace(common.Interface2String(tool["type"]))
		switch toolType {
		case "function":
			name := strings.TrimSpace(common.Interface2String(tool["name"]))
			if name == "" {
				continue
			}
			if _, exists := ctx.ToolsByChatName[name]; exists {
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
			if _, exists := ctx.ToolsByChatName[name]; exists {
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
			if _, exists := ctx.ToolsByChatName[name]; exists {
				continue
			}
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
		case "web_search", "web_search_preview", "file_search", "computer", "computer_use_preview", "image_generation", "code_interpreter", "mcp":
			ctx.addHostedToolFiltered(normalizeHostedToolType(toolType))
		case "namespace":
			children, _ := tool["tools"].([]any)
			namespace := strings.TrimSpace(common.Interface2String(tool["name"]))
			for _, child := range children {
				childMap, ok := child.(map[string]any)
				if !ok {
					ctx.addUnsupportedToolFiltered("namespace.unknown")
					continue
				}
				childType := strings.TrimSpace(common.Interface2String(childMap["type"]))
				if childType != "function" {
					if childType == "" {
						childType = "unknown"
					}
					ctx.addUnsupportedToolFiltered("namespace." + childType)
					continue
				}
				name := strings.TrimSpace(common.Interface2String(childMap["name"]))
				if name == "" {
					continue
				}
				chatName := name
				if namespace != "" {
					chatName = flattenNamespaceToolName(namespace, name)
					if chatName != name {
						ctx.NamespaceToolsFlattened++
					}
				}
				if _, exists := ctx.ToolsByChatName[chatName]; exists {
					continue
				}
				chatTools = append(chatTools, dto.ToolCallRequest{
					Type: "function",
					Function: dto.FunctionRequest{
						Name:        chatName,
						Description: common.Interface2String(childMap["description"]),
						Parameters:  childMap["parameters"],
					},
				})
				ctx.ToolsByChatName[chatName] = ResponseToolSpec{Kind: ResponseToolKindFunction, Name: name, ChatName: chatName, Namespace: namespace}
			}
		default:
			ctx.addUnsupportedToolFiltered(toolType)
			continue
		}
	}
	return chatTools
}

func collectResponsesInputProvidedChatTools(raw json.RawMessage, ctx *ResponsesToChatContext) []dto.ToolCallRequest {
	if len(raw) == 0 || common.GetJsonType(raw) == "string" {
		return nil
	}

	var items []map[string]any
	if err := common.Unmarshal(raw, &items); err != nil {
		var single map[string]any
		if err2 := common.Unmarshal(raw, &single); err2 == nil {
			items = []map[string]any{single}
		} else {
			return nil
		}
	}

	var chatTools []dto.ToolCallRequest
	for _, item := range items {
		if strings.TrimSpace(common.Interface2String(item["type"])) != "tool_search_output" {
			continue
		}
		rawTools, ok := item["tools"].([]any)
		if !ok || len(rawTools) == 0 {
			continue
		}
		added := convertResponsesToolValuesToChatTools(rawTools, ctx)
		chatTools = append(chatTools, added...)
		if ctx != nil {
			ctx.InputProvidedToolsCount += len(added)
		}
	}
	return chatTools
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

	outputCallIDs := map[string]bool{}
	for _, item := range items {
		itemType := strings.TrimSpace(common.Interface2String(item["type"]))
		callID := responseItemCallID(item)
		if callID != "" && isResponsesCallOutputItemType(itemType) {
			outputCallIDs[callID] = true
		}
	}

	messages := make([]dto.Message, 0, len(items))
	pendingReasoning := ""
	pendingToolCalls := make([]dto.ToolCallRequest, 0)
	pendingToolReasoning := ""
	pendingToolCallIDs := make([]string, 0)
	awaitingToolOutputs := map[string]bool{}
	deferredMessages := make([]dto.Message, 0)

	hasAwaitingToolOutput := func() bool {
		for callID := range awaitingToolOutputs {
			if outputCallIDs[callID] {
				return true
			}
		}
		return false
	}
	appendRegularMessage := func(msg dto.Message) {
		if hasAwaitingToolOutput() {
			deferredMessages = append(deferredMessages, msg)
			return
		}
		messages = append(messages, msg)
	}
	flushDeferredMessages := func() {
		if len(deferredMessages) == 0 {
			return
		}
		messages = append(messages, deferredMessages...)
		deferredMessages = deferredMessages[:0]
	}
	appendPendingReasoningMessage := func() {
		reasoning := consumePendingReasoning(&pendingReasoning)
		if reasoning == "" {
			return
		}
		msg := dto.Message{Role: "assistant", Content: ""}
		attachReasoningToAssistantMessage(&msg, reasoning, false)
		appendRegularMessage(msg)
	}
	flushPendingToolCalls := func() {
		if len(pendingToolCalls) == 0 {
			return
		}
		msg := dto.Message{Role: "assistant", Content: nil}
		msg.SetToolCalls(pendingToolCalls)
		attachReasoningToAssistantMessage(&msg, pendingToolReasoning, true)
		if len(messages) > 0 && messages[len(messages)-1].Role == "assistant" {
			prev := &messages[len(messages)-1]
			toolCalls := prev.ParseToolCalls()
			toolCalls = append(toolCalls, pendingToolCalls...)
			prev.SetToolCalls(toolCalls)
			attachReasoningToAssistantMessage(prev, pendingToolReasoning, true)
			if prev.Content == "" {
				prev.Content = nil
			}
		} else {
			messages = append(messages, msg)
		}
		for _, callID := range pendingToolCallIDs {
			if strings.TrimSpace(callID) != "" {
				awaitingToolOutputs[callID] = true
			}
		}
		pendingToolCalls = pendingToolCalls[:0]
		pendingToolCallIDs = pendingToolCallIDs[:0]
		pendingToolReasoning = ""
	}
	bufferToolCall := func(msg dto.Message) {
		toolCalls := msg.ParseToolCalls()
		if len(toolCalls) == 0 {
			return
		}
		pendingToolCalls = append(pendingToolCalls, toolCalls...)
		for _, toolCall := range toolCalls {
			pendingToolCallIDs = append(pendingToolCallIDs, toolCall.ID)
		}
		pendingToolReasoning = joinReasoningText(pendingToolReasoning, consumePendingReasoning(&pendingReasoning), msg.GetReasoningContent())
	}

	for _, item := range items {
		itemType := strings.TrimSpace(common.Interface2String(item["type"]))
		if itemType == "" && strings.TrimSpace(common.Interface2String(item["role"])) != "" {
			itemType = "message"
		}
		if !isResponsesCallItemType(itemType) {
			flushPendingToolCalls()
		}
		switch itemType {
		case "", "message":
			role := strings.TrimSpace(common.Interface2String(item["role"]))
			if role == "" {
				role = "user"
			}
			if responsesRoleToChat(role) != "assistant" {
				appendPendingReasoningMessage()
			}
			msg := dto.Message{Role: responsesRoleToChat(role), Content: responsesContentToChatContent(item["content"], role)}
			attachReasoningToAssistantMessage(&msg, consumePendingReasoning(&pendingReasoning), false)
			appendRegularMessage(msg)
		case "input_text":
			appendPendingReasoningMessage()
			appendRegularMessage(dto.Message{Role: "user", Content: common.Interface2String(item["text"])})
		case "output_text":
			msg := dto.Message{Role: "assistant", Content: common.Interface2String(item["text"])}
			attachReasoningToAssistantMessage(&msg, consumePendingReasoning(&pendingReasoning), false)
			appendRegularMessage(msg)
		case "function_call":
			name := responsesFunctionCallChatName(item, ctx)
			msg := assistantToolCallMessage(item, name, item["arguments"])
			attachReasoningToAssistantMessage(&msg, joinReasoningText(consumePendingReasoning(&pendingReasoning), responseItemReasoningText(item)), false)
			bufferToolCall(msg)
		case "custom_tool_call":
			input := common.Interface2String(item["input"])
			name := common.Interface2String(item["name"])
			msg := assistantToolCallMessage(item, name, map[string]any{"input": input})
			attachReasoningToAssistantMessage(&msg, joinReasoningText(consumePendingReasoning(&pendingReasoning), responseItemReasoningText(item)), false)
			bufferToolCall(msg)
		case "tool_search_call":
			msg := assistantToolCallMessage(item, "tool_search", item["arguments"])
			attachReasoningToAssistantMessage(&msg, joinReasoningText(consumePendingReasoning(&pendingReasoning), responseItemReasoningText(item)), false)
			bufferToolCall(msg)
		case "web_search_call", "file_search_call", "computer_call", "image_generation_call", "code_interpreter_call", "mcp_call":
			hostedType := hostedToolTypeFromCallItemType(itemType)
			if ctx != nil {
				ctx.addHostedToolFiltered(hostedType)
			}
			msg := dto.Message{Role: "assistant", Content: hostedToolCallFallbackText(itemType, item)}
			attachReasoningToAssistantMessage(&msg, joinReasoningText(consumePendingReasoning(&pendingReasoning), responseItemReasoningText(item)), false)
			appendRegularMessage(msg)
		case "function_call_output", "custom_tool_call_output", "tool_search_output":
			messages = append(messages, dto.Message{
				Role:       "tool",
				ToolCallId: common.Interface2String(item["call_id"]),
				Content:    toolOutputContent(item),
			})
			if callID := responseItemCallID(item); callID != "" {
				delete(awaitingToolOutputs, callID)
			}
			if !hasAwaitingToolOutput() {
				flushDeferredMessages()
			}
		case "reasoning":
			pendingReasoning = joinReasoningText(pendingReasoning, responseItemReasoningText(item))
			continue
		default:
			appendPendingReasoningMessage()
			appendRegularMessage(dto.Message{Role: "user", Content: stringifyJSONValue(item)})
		}
	}
	flushPendingToolCalls()
	appendPendingReasoningMessage()
	flushDeferredMessages()
	messages = mergeAdjacentToolCalls(messages, ctx)
	if ctx != nil {
		ctx.ReasoningBackfilledCount += backfillAssistantToolCallReasoning(messages)
	} else {
		backfillAssistantToolCallReasoning(messages)
	}
	return messages, nil
}

func responsesHostedCallChatName(itemType string, item map[string]any, ctx *ResponsesToChatContext) string {
	if name := strings.TrimSpace(common.Interface2String(item["name"])); name != "" {
		return name
	}
	hostedType := hostedToolTypeFromCallItemType(itemType)
	if ctx != nil {
		for chatName, spec := range ctx.ToolsByChatName {
			if spec.Kind == ResponseToolKindHosted && spec.HostedType == hostedType {
				return chatName
			}
		}
	}
	switch hostedType {
	case "web_search":
		return "web_search"
	case "file_search":
		return "file_search"
	case "computer":
		return "computer"
	case "image_generation":
		return "image_generation"
	case "code_interpreter":
		return "code_interpreter"
	case "mcp":
		return "mcp"
	default:
		return "tool"
	}
}

func hostedToolTypeFromCallItemType(itemType string) string {
	switch strings.TrimSpace(itemType) {
	case "web_search_call":
		return "web_search"
	case "file_search_call":
		return "file_search"
	case "computer_call":
		return "computer"
	case "image_generation_call":
		return "image_generation"
	case "code_interpreter_call":
		return "code_interpreter"
	case "mcp_call":
		return "mcp"
	default:
		return ""
	}
}

func responsesHostedCallArguments(itemType string, item map[string]any) any {
	switch hostedToolTypeFromCallItemType(itemType) {
	case "web_search":
		if action, ok := item["action"].(map[string]any); ok {
			if query := strings.TrimSpace(common.Interface2String(action["query"])); query != "" {
				return map[string]any{"query": query}
			}
			return map[string]any{"action": action}
		}
	case "file_search":
		args := map[string]any{}
		if queries, ok := item["queries"]; ok {
			args["queries"] = queries
			if list, ok := queries.([]any); ok && len(list) > 0 {
				args["query"] = common.Interface2String(list[0])
			}
			if list, ok := queries.([]string); ok && len(list) > 0 {
				args["query"] = list[0]
			}
		}
		if results, ok := item["results"]; ok {
			args["results"] = results
		}
		if len(args) > 0 {
			return args
		}
	case "computer":
		args := map[string]any{}
		if action, ok := item["action"]; ok {
			args["action"] = action
		}
		if pending, ok := item["pending_safety_checks"]; ok {
			args["pending_safety_checks"] = pending
		}
		if len(args) > 0 {
			return args
		}
	case "image_generation":
		if arguments, ok := item["arguments"]; ok {
			return arguments
		}
	case "code_interpreter":
		args := map[string]any{}
		if code := strings.TrimSpace(common.Interface2String(item["code"])); code != "" {
			args["code"] = code
		}
		if language := strings.TrimSpace(common.Interface2String(item["language"])); language != "" {
			args["language"] = language
		}
		if len(args) > 0 {
			return args
		}
	case "mcp":
		args := map[string]any{}
		if toolName := strings.TrimSpace(common.Interface2String(item["tool_name"])); toolName != "" {
			args["tool_name"] = toolName
		}
		if arguments, ok := item["arguments"]; ok {
			args["arguments"] = arguments
		}
		if len(args) > 0 {
			return args
		}
	}
	if arguments, ok := item["arguments"]; ok {
		return arguments
	}
	return map[string]any{}
}

func hostedToolCallFallbackText(itemType string, item map[string]any) string {
	hostedType := hostedToolTypeFromCallItemType(itemType)
	if hostedType == "" {
		hostedType = strings.TrimSuffix(strings.TrimSpace(itemType), "_call")
	}
	args := responsesHostedCallArguments(itemType, item)
	argsJSON, err := json.Marshal(args)
	if err != nil || string(argsJSON) == "{}" || string(argsJSON) == "null" {
		return "A hosted Responses tool call (" + hostedType + ") was requested earlier, but no hosted tool executor is available in this Chat Completions upstream."
	}
	return "A hosted Responses tool call (" + hostedType + ") was requested earlier, but no hosted tool executor is available in this Chat Completions upstream. Tool arguments: " + string(argsJSON)
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

func responsesFunctionCallChatName(item map[string]any, ctx *ResponsesToChatContext) string {
	name := strings.TrimSpace(common.Interface2String(item["name"]))
	namespace := strings.TrimSpace(common.Interface2String(item["namespace"]))
	if namespace != "" && name != "" {
		if ctx != nil {
			chatName := flattenNamespaceToolName(namespace, name)
			if _, ok := ctx.ToolsByChatName[chatName]; ok {
				return chatName
			}
		}
	}
	if ctx != nil {
		matchedChatName := ""
		matches := 0
		for chatName, spec := range ctx.ToolsByChatName {
			if spec.Kind == ResponseToolKindFunction && spec.Name == name {
				if chatName == name {
					return chatName
				}
				matchedChatName = chatName
				matches++
			}
		}
		if matches == 1 {
			return matchedChatName
		}
	}
	return name
}

func flattenNamespaceToolName(namespace, name string) string {
	namespace = strings.TrimSpace(namespace)
	name = strings.TrimSpace(name)
	if namespace == "" || name == "" || strings.HasPrefix(name, "mcp__") {
		return name
	}
	if strings.HasPrefix(name, namespace) {
		return name
	}
	combined := namespace + "__" + name
	if len(combined) <= 64 {
		return combined
	}
	sum := sha256.Sum256([]byte(combined))
	suffix := "__" + hex.EncodeToString(sum[:])[:10]
	prefixLimit := 64 - len(suffix)
	if prefixLimit < 1 {
		return hex.EncodeToString(sum[:])[:64]
	}
	prefix := truncateStringBytes(combined, prefixLimit)
	prefix = strings.TrimRight(prefix, "_")
	if prefix == "" {
		prefix = "tool"
	}
	return prefix + suffix
}

func truncateStringBytes(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	var out strings.Builder
	for _, r := range value {
		if out.Len()+len(string(r)) > limit {
			break
		}
		out.WriteRune(r)
	}
	return out.String()
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
				attachReasoningToAssistantMessage(prev, msg.GetReasoningContent(), true)
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

func attachReasoningToAssistantMessage(msg *dto.Message, reasoning string, allowBackfill bool) {
	if msg == nil || msg.Role != "assistant" {
		return
	}
	reasoning = strings.TrimSpace(reasoning)
	if reasoning == "" && allowBackfill && len(msg.ParseToolCalls()) > 0 {
		reasoning = "tool call"
	}
	if reasoning == "" {
		return
	}
	if existing := strings.TrimSpace(msg.GetReasoningContent()); existing != "" {
		reasoning = joinReasoningText(existing, reasoning)
	}
	msg.ReasoningContent = &reasoning
}

func backfillAssistantToolCallReasoning(messages []dto.Message) int {
	count := 0
	for i := range messages {
		if messages[i].Role != "assistant" || len(messages[i].ParseToolCalls()) == 0 || strings.TrimSpace(messages[i].GetReasoningContent()) != "" {
			continue
		}
		reasoning := "tool call"
		messages[i].ReasoningContent = &reasoning
		count++
	}
	return count
}

func consumePendingReasoning(reasoning *string) string {
	if reasoning == nil {
		return ""
	}
	out := *reasoning
	*reasoning = ""
	return out
}

func joinReasoningText(values ...string) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			parts = append(parts, value)
		}
	}
	return strings.Join(parts, "\n")
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
	choiceType := strings.TrimSpace(common.Interface2String(m["type"]))
	switch choiceType {
	case "function":
		name := common.Interface2String(m["name"])
		if spec, ok := ctx.ToolsByChatName[name]; ok && spec.ChatName != "" {
			name = spec.ChatName
		} else {
			return nil
		}
		if name != "" {
			return map[string]any{
				"type": "function",
				"function": map[string]any{
					"name": name,
				},
			}
		}
	case "custom":
		name := common.Interface2String(m["name"])
		if spec, ok := ctx.ToolsByChatName[name]; ok && spec.ChatName != "" {
			name = spec.ChatName
		} else {
			return nil
		}
		return map[string]any{
			"type": "function",
			"function": map[string]any{
				"name": name,
			},
		}
	case "tool_search":
		if _, ok := ctx.ToolsByChatName["tool_search"]; !ok {
			return nil
		}
		return map[string]any{
			"type": "function",
			"function": map[string]any{
				"name": "tool_search",
			},
		}
	case "auto", "none", "required":
		return choiceType
	}
	if isSkippedResponsesHostedToolType(choiceType) {
		if ctx != nil {
			ctx.addHostedToolFiltered(normalizeHostedToolType(choiceType))
		}
		return nil
	}
	return nil
}

func hostedToolFallbackInstruction(ctx *ResponsesToChatContext) string {
	if ctx == nil {
		return ""
	}
	hostedTools := uniqueSortedStrings(ctx.HostedToolsFiltered)
	if len(hostedTools) == 0 {
		return ""
	}
	ctx.HostedToolsDirectHint = true
	return "The original Responses request included OpenAI hosted tools (" + strings.Join(hostedTools, ", ") + "), but this Chat Completions upstream cannot execute hosted tools. Do not call or invent those tools. Answer directly from the conversation and the model's available knowledge; if fresh external, file, computer, or code execution results are required, briefly state that limitation and provide the best answer possible."
}

func isSkippedResponsesHostedToolType(toolType string) bool {
	switch strings.TrimSpace(toolType) {
	case "web_search", "web_search_preview", "file_search", "computer", "computer_use_preview", "image_generation", "code_interpreter", "mcp":
		return true
	default:
		return false
	}
}

func normalizeHostedToolType(toolType string) string {
	switch strings.TrimSpace(toolType) {
	case "web_search_preview":
		return "web_search"
	case "computer_use_preview":
		return "computer"
	default:
		return strings.TrimSpace(toolType)
	}
}

func hostedToolChatName(toolType string, tool map[string]any) string {
	if name := strings.TrimSpace(common.Interface2String(tool["name"])); name != "" {
		return truncateHostedToolName(name)
	}
	switch normalizeHostedToolType(toolType) {
	case "web_search":
		return "web_search"
	case "file_search":
		return "file_search"
	case "computer":
		return "computer"
	case "image_generation":
		return "image_generation"
	case "code_interpreter":
		return "code_interpreter"
	case "mcp":
		if label := strings.TrimSpace(common.Interface2String(tool["server_label"])); label != "" {
			return flattenNamespaceToolName("mcp", label)
		}
		return "mcp"
	default:
		return ""
	}
}

func truncateHostedToolName(name string) string {
	name = strings.TrimSpace(name)
	if len(name) <= 64 {
		return name
	}
	return flattenNamespaceToolName("hosted", name)
}

func hostedToolChoiceChatName(choiceType string, choice map[string]any, ctx *ResponsesToChatContext) string {
	if ctx == nil {
		return ""
	}
	name := hostedToolChatName(choiceType, choice)
	if name != "" {
		if spec, ok := ctx.ToolsByChatName[name]; ok && spec.Kind == ResponseToolKindHosted {
			return spec.ChatName
		}
	}
	normalized := normalizeHostedToolType(choiceType)
	for chatName, spec := range ctx.ToolsByChatName {
		if spec.Kind == ResponseToolKindHosted && spec.HostedType == normalized {
			return chatName
		}
	}
	return ""
}

func hostedToolDescription(toolType string, tool map[string]any) string {
	if desc := strings.TrimSpace(common.Interface2String(tool["description"])); desc != "" {
		return desc
	}
	switch normalizeHostedToolType(toolType) {
	case "web_search":
		return "Search the web for up-to-date information."
	case "file_search":
		return "Search files or vector stores available to the original Responses request."
	case "computer":
		return "Request a computer-use action from the client or upstream runtime."
	case "image_generation":
		return "Generate or edit an image."
	case "code_interpreter":
		return "Run code in an interpreter."
	case "mcp":
		return "Call a hosted MCP tool."
	default:
		return "Hosted Responses tool represented as a Chat function."
	}
}

func hostedToolParameters(toolType string, tool map[string]any) any {
	if params, ok := tool["parameters"]; ok && params != nil {
		return params
	}
	switch normalizeHostedToolType(toolType) {
	case "web_search":
		return map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{"type": "string", "description": "Search query."},
			},
			"required": []string{"query"},
		}
	case "file_search":
		return map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{"type": "string", "description": "Search query."},
			},
			"required": []string{"query"},
		}
	case "computer":
		return map[string]any{
			"type": "object",
			"properties": map[string]any{
				"action": map[string]any{"type": "string"},
				"input":  map[string]any{"type": "object"},
			},
			"required": []string{"action"},
		}
	case "image_generation":
		return map[string]any{
			"type": "object",
			"properties": map[string]any{
				"prompt": map[string]any{"type": "string"},
			},
			"required": []string{"prompt"},
		}
	case "code_interpreter":
		return map[string]any{
			"type": "object",
			"properties": map[string]any{
				"code":     map[string]any{"type": "string"},
				"language": map[string]any{"type": "string"},
			},
			"required": []string{"code"},
		}
	case "mcp":
		return map[string]any{
			"type": "object",
			"properties": map[string]any{
				"tool_name": map[string]any{"type": "string"},
				"arguments": map[string]any{"type": "object"},
			},
			"required": []string{"tool_name"},
		}
	default:
		return map[string]any{"type": "object"}
	}
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
		return canonicalizeJSONStringIfParseable(v)
	case json.RawMessage:
		if len(v) == 0 {
			return "{}"
		}
		return canonicalizeJSONStringIfParseable(string(v))
	default:
		return canonicalizeJSONStringIfParseable(stringifyJSONValue(v))
	}
}

func valueToOutputString(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return canonicalizeJSONStringIfParseable(v)
	default:
		return canonicalizeJSONStringIfParseable(stringifyJSONValue(v))
	}
}

func canonicalizeJSONStringIfParseable(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return value
	}
	var decoded any
	if err := common.Unmarshal([]byte(trimmed), &decoded); err != nil {
		return value
	}
	encoded, err := common.Marshal(decoded)
	if err != nil {
		return value
	}
	return string(encoded)
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
