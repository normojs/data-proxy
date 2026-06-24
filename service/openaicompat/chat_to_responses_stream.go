package openaicompat

import (
	"sort"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

type ChatToResponsesStreamEvent struct {
	Event   string
	Payload map[string]any
}

const (
	inlineThinkModeText      = "text"
	inlineThinkModeReasoning = "reasoning"
	inlineThinkOpenTag       = "<think>"
	inlineThinkCloseTag      = "</think>"
)

type ChatToResponsesStreamConverter struct {
	request *dto.OpenAIResponsesRequest
	ctx     *ResponsesToChatContext

	usage      *dto.Usage
	responseID string
	model      string
	createdAt  int64

	outputText strings.Builder
	textIndex  int
	textID     string
	textOpen   bool
	textDone   bool

	reasoningText  strings.Builder
	reasoningIndex int
	reasoningID    string
	reasoningOpen  bool
	reasoningDone  bool

	nextOutputIndex int
	responseCreated bool
	toolStates      map[int]*chatToResponsesStreamToolState
	finishReason    string

	inlineThinkMode   string
	inlineThinkBuffer strings.Builder
}

func NewChatToResponsesStreamConverter(request *dto.OpenAIResponsesRequest, ctx *ResponsesToChatContext) *ChatToResponsesStreamConverter {
	model := ""
	if request != nil {
		model = request.Model
	}
	responseID := "resp_" + common.GetUUID()
	return &ChatToResponsesStreamConverter{
		request:        request,
		ctx:            ctx,
		usage:          &dto.Usage{},
		responseID:     responseID,
		model:          model,
		createdAt:      time.Now().Unix(),
		textIndex:      -1,
		textID:         "msg_" + responseID,
		reasoningIndex: -1,
		reasoningID:    "rs_" + responseID,
		toolStates:     map[int]*chatToResponsesStreamToolState{},
	}
}

func (c *ChatToResponsesStreamConverter) HandleChunk(chunk dto.ChatCompletionsStreamResponse) []ChatToResponsesStreamEvent {
	events := make([]ChatToResponsesStreamEvent, 0)
	if chunk.Id != "" {
		c.setResponseID(ChatStreamIDToResponsesID(chunk.Id))
	}
	if chunk.Model != "" {
		c.model = chunk.Model
	}
	if chunk.Created != 0 {
		c.createdAt = chunk.Created
	}
	if chunk.Usage != nil {
		c.usage = chunk.Usage
	}
	if len(chunk.Choices) == 0 {
		return events
	}

	choice := chunk.Choices[0]
	if reasoningDelta := choice.Delta.GetReasoningContent(); reasoningDelta != "" {
		c.emitReasoningDelta(reasoningDelta, &events)
	}

	if textDelta := choice.Delta.GetContentString(); textDelta != "" {
		c.handleContentDelta(textDelta, &events)
	}

	if len(choice.Delta.ToolCalls) > 0 {
		c.flushInlineThinkBuffers(&events)
		c.ensureCreated(&events)
		for _, tool := range choice.Delta.ToolCalls {
			index := 0
			if tool.Index != nil {
				index = *tool.Index
			}
			state := c.toolStates[index]
			if state == nil {
				state = &chatToResponsesStreamToolState{Index: index, OutputIndex: -1}
				c.toolStates[index] = state
			}
			if tool.ID != "" && (!state.Started || strings.TrimSpace(state.ID) == "") {
				state.ID = tool.ID
			}
			if tool.Function.Name != "" {
				state.Name = tool.Function.Name
			}
			if tool.Function.Arguments != "" {
				state.Arguments.WriteString(tool.Function.Arguments)
			}
			c.sendPendingToolArguments(state, &events)
		}
	}
	if choice.FinishReason != nil {
		c.finishReason = strings.TrimSpace(*choice.FinishReason)
	}

	return events
}

func (c *ChatToResponsesStreamConverter) Finish() []ChatToResponsesStreamEvent {
	events := make([]ChatToResponsesStreamEvent, 0)
	output := make([]indexedResponsesOutputItem, 0)

	c.flushInlineThinkBuffers(&events)

	if c.reasoningText.Len() > 0 {
		c.finalizeReasoning(&events)
		output = append(output, indexedResponsesOutputItem{
			index: c.reasoningIndex,
			item:  responseReasoningItem(c.reasoningID, c.reasoningText.String()),
		})
	}

	if c.outputText.Len() == 0 && c.reasoningText.Len() > 0 && len(c.toolStates) == 0 {
		text := c.reasoningText.String()
		c.ensureTextStarted(&events)
		c.outputText.WriteString(text)
		events = append(events, ChatToResponsesStreamEvent{
			Event: "response.output_text.delta",
			Payload: map[string]any{
				"output_index":  c.textIndex,
				"item_id":       c.textID,
				"content_index": 0,
				"delta":         text,
			},
		})
	}
	if c.outputText.Len() > 0 {
		c.finalizeText(&events)
		output = append(output, indexedResponsesOutputItem{
			index: c.textIndex,
			item:  ResponseMessageItem(c.textID, c.outputText.String()),
		})
	}
	for _, state := range c.sortedToolStates() {
		if !c.finalizeTool(state, &events) {
			continue
		}
		output = append(output, indexedResponsesOutputItem{
			index: state.OutputIndex,
			item:  c.toolItemForState(state, state.Arguments.String(), "completed"),
		})
	}

	c.ensureCreated(&events)
	finalOutput := sortedResponsesOutputItems(output)
	if c.finishReason == "" && len(finalOutput) == 0 {
		response := mergeResponsesUsage(c.baseResponse("failed", finalOutput), ChatUsageToResponsesUsage(c.usage))
		response["error"] = map[string]any{
			"type":    "stream_truncated",
			"message": "Upstream Chat Completions stream ended before sending finish_reason.",
		}
		events = append(events, ChatToResponsesStreamEvent{
			Event: "response.failed",
			Payload: map[string]any{
				"response": response,
			},
		})
		return events
	}
	finishReason := c.finishReason
	if finishReason == "" {
		finishReason = "length"
	}
	status := responseStatusFromChatFinishReason(finishReason)
	response := mergeResponsesUsage(c.baseResponse(status, finalOutput), ChatUsageToResponsesUsage(c.usage))
	if status == "incomplete" {
		response["incomplete_details"] = map[string]any{"reason": "max_output_tokens"}
	}
	if recorded := DefaultResponsesChatHistory().RecordResponseMap(response); recorded > 0 && c.ctx != nil {
		c.ctx.HistoryRecordedCount += recorded
	}
	events = append(events, ChatToResponsesStreamEvent{
		Event: "response.completed",
		Payload: map[string]any{
			"response": response,
		},
	})
	return events
}

func (c *ChatToResponsesStreamConverter) Usage() *dto.Usage {
	if c.usage == nil {
		return &dto.Usage{}
	}
	return c.usage
}

func (c *ChatToResponsesStreamConverter) SetUsage(usage *dto.Usage) {
	if usage == nil {
		c.usage = &dto.Usage{}
		return
	}
	c.usage = usage
}

func (c *ChatToResponsesStreamConverter) handleContentDelta(delta string, events *[]ChatToResponsesStreamEvent) {
	if delta == "" {
		return
	}
	switch c.inlineThinkMode {
	case inlineThinkModeText:
		c.emitTextDelta(delta, events)
	case inlineThinkModeReasoning:
		c.inlineThinkBuffer.WriteString(delta)
		c.drainInlineThinkReasoning(events, false)
	default:
		c.inlineThinkBuffer.WriteString(delta)
		c.resolveInitialInlineThinkMode(events)
	}
}

func (c *ChatToResponsesStreamConverter) flushInlineThinkBuffers(events *[]ChatToResponsesStreamEvent) {
	switch c.inlineThinkMode {
	case inlineThinkModeReasoning:
		c.drainInlineThinkReasoning(events, true)
	default:
		if c.inlineThinkBuffer.Len() == 0 {
			return
		}
		buffered := c.inlineThinkBuffer.String()
		c.inlineThinkBuffer.Reset()
		c.inlineThinkMode = inlineThinkModeText
		c.emitTextDelta(buffered, events)
	}
}

func (c *ChatToResponsesStreamConverter) resolveInitialInlineThinkMode(events *[]ChatToResponsesStreamEvent) {
	buffered := c.inlineThinkBuffer.String()
	if buffered == "" {
		return
	}
	trimmed := strings.TrimLeft(buffered, " \t\r\n")
	if trimmed == "" {
		return
	}
	if strings.HasPrefix(inlineThinkOpenTag, trimmed) && len(trimmed) < len(inlineThinkOpenTag) {
		return
	}
	if strings.HasPrefix(trimmed, inlineThinkOpenTag) {
		c.inlineThinkBuffer.Reset()
		c.inlineThinkBuffer.WriteString(trimmed[len(inlineThinkOpenTag):])
		c.inlineThinkMode = inlineThinkModeReasoning
		c.drainInlineThinkReasoning(events, false)
		return
	}
	c.inlineThinkBuffer.Reset()
	c.inlineThinkMode = inlineThinkModeText
	c.emitTextDelta(buffered, events)
}

func (c *ChatToResponsesStreamConverter) drainInlineThinkReasoning(events *[]ChatToResponsesStreamEvent, final bool) {
	buffered := c.inlineThinkBuffer.String()
	if buffered == "" {
		return
	}
	if idx := strings.Index(buffered, inlineThinkCloseTag); idx >= 0 {
		if reasoning := buffered[:idx]; reasoning != "" {
			c.emitReasoningDelta(reasoning, events)
		}
		after := buffered[idx+len(inlineThinkCloseTag):]
		c.inlineThinkBuffer.Reset()
		c.inlineThinkMode = inlineThinkModeText
		c.finalizeReasoning(events)
		if after != "" {
			c.emitTextDelta(after, events)
		}
		return
	}

	keep := 0
	if !final {
		keep = longestSuffixPrefixLen(buffered, inlineThinkCloseTag)
	}
	emitLen := len(buffered) - keep
	if emitLen <= 0 {
		return
	}
	c.emitReasoningDelta(buffered[:emitLen], events)
	c.inlineThinkBuffer.Reset()
	if keep > 0 {
		c.inlineThinkBuffer.WriteString(buffered[emitLen:])
	}
}

func longestSuffixPrefixLen(value string, prefix string) int {
	maxLen := len(value)
	if len(prefix)-1 < maxLen {
		maxLen = len(prefix) - 1
	}
	for i := maxLen; i > 0; i-- {
		if strings.HasSuffix(value, prefix[:i]) {
			return i
		}
	}
	return 0
}

func (c *ChatToResponsesStreamConverter) emitReasoningDelta(delta string, events *[]ChatToResponsesStreamEvent) {
	if delta == "" {
		return
	}
	c.ensureReasoningStarted(events)
	c.reasoningText.WriteString(delta)
	*events = append(*events, ChatToResponsesStreamEvent{
		Event: "response.reasoning_summary_text.delta",
		Payload: map[string]any{
			"output_index":  c.reasoningIndex,
			"item_id":       c.reasoningID,
			"summary_index": 0,
			"delta":         delta,
		},
	})
}

func (c *ChatToResponsesStreamConverter) emitTextDelta(delta string, events *[]ChatToResponsesStreamEvent) {
	if delta == "" {
		return
	}
	c.ensureTextStarted(events)
	c.outputText.WriteString(delta)
	*events = append(*events, ChatToResponsesStreamEvent{
		Event: "response.output_text.delta",
		Payload: map[string]any{
			"output_index":  c.textIndex,
			"item_id":       c.textID,
			"content_index": 0,
			"delta":         delta,
		},
	})
}

func (c *ChatToResponsesStreamConverter) EstimatedOutputText() string {
	if c.outputText.Len() > 0 {
		return c.outputText.String()
	}
	return c.reasoningText.String()
}

func (c *ChatToResponsesStreamConverter) setResponseID(responseID string) {
	if responseID == "" || responseID == c.responseID {
		return
	}
	oldID := c.responseID
	c.responseID = responseID
	if c.textID == "msg_"+oldID {
		c.textID = "msg_" + responseID
	}
	if c.reasoningID == "rs_"+oldID {
		c.reasoningID = "rs_" + responseID
	}
}

func (c *ChatToResponsesStreamConverter) ensureCreated(events *[]ChatToResponsesStreamEvent) {
	if c.responseCreated {
		return
	}
	c.responseCreated = true
	*events = append(*events, ChatToResponsesStreamEvent{
		Event: "response.created",
		Payload: map[string]any{
			"response": c.baseResponse("in_progress", []any{}),
		},
	}, ChatToResponsesStreamEvent{
		Event: "response.in_progress",
		Payload: map[string]any{
			"response": c.baseResponse("in_progress", []any{}),
		},
	})
}

func (c *ChatToResponsesStreamConverter) baseResponse(status string, output []any) map[string]any {
	resp := map[string]any{
		"id":         c.responseID,
		"object":     "response",
		"created_at": c.createdAt,
		"status":     status,
		"model":      c.model,
		"output":     output,
	}
	if c.request != nil && c.request.PreviousResponseID != "" {
		resp["previous_response_id"] = c.request.PreviousResponseID
	}
	return resp
}

func (c *ChatToResponsesStreamConverter) allocateOutputIndex() int {
	index := c.nextOutputIndex
	c.nextOutputIndex++
	return index
}

func (c *ChatToResponsesStreamConverter) toolOutputIndex(state *chatToResponsesStreamToolState) int {
	if state == nil {
		return 0
	}
	if state.OutputIndex < 0 {
		state.OutputIndex = c.allocateOutputIndex()
	}
	return state.OutputIndex
}

func (c *ChatToResponsesStreamConverter) toolCallForState(state *chatToResponsesStreamToolState, arguments string) dto.ToolCallRequest {
	callID := normalizeResponsesCallID(state.ID)
	state.ID = callID
	name := strings.TrimSpace(state.Name)
	return dto.ToolCallRequest{
		ID:   callID,
		Type: "function",
		Function: dto.FunctionRequest{
			Name:      name,
			Arguments: arguments,
		},
	}
}

func (c *ChatToResponsesStreamConverter) toolItemForState(state *chatToResponsesStreamToolState, arguments string, status string) map[string]any {
	item := ChatToolCallToResponsesItem(c.toolCallForState(state, arguments), c.ctx)
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
	if c.reasoningText.Len() > 0 {
		setResponseItemReasoningText(item, c.reasoningText.String())
	}
	return item
}

func (c *ChatToResponsesStreamConverter) ensureToolStarted(state *chatToResponsesStreamToolState, events *[]ChatToResponsesStreamEvent) {
	if state == nil || state.Started {
		return
	}
	if strings.TrimSpace(state.Name) == "" || strings.TrimSpace(state.ID) == "" {
		return
	}
	c.ensureCreated(events)
	state.Started = true
	item := c.toolItemForState(state, "", "in_progress")
	*events = append(*events, ChatToResponsesStreamEvent{
		Event: "response.output_item.added",
		Payload: map[string]any{
			"output_index": c.toolOutputIndex(state),
			"item":         item,
		},
	})
}

func (c *ChatToResponsesStreamConverter) sendPendingToolArguments(state *chatToResponsesStreamToolState, events *[]ChatToResponsesStreamEvent) {
	if state == nil {
		return
	}
	c.ensureToolStarted(state, events)
	if !state.Started {
		return
	}
	arguments := state.Arguments.String()
	if len(arguments) <= state.SentArgumentsLen {
		return
	}
	delta := arguments[state.SentArgumentsLen:]
	state.SentArgumentsLen = len(arguments)
	item := c.toolItemForState(state, state.Arguments.String(), "in_progress")
	if common.Interface2String(item["type"]) != "function_call" {
		return
	}
	*events = append(*events, ChatToResponsesStreamEvent{
		Event: "response.function_call_arguments.delta",
		Payload: map[string]any{
			"output_index": c.toolOutputIndex(state),
			"item_id":      state.ItemID,
			"delta":        delta,
		},
	})
}

func (c *ChatToResponsesStreamConverter) finalizeTool(state *chatToResponsesStreamToolState, events *[]ChatToResponsesStreamEvent) bool {
	if state == nil || state.Done {
		return state != nil && state.Done
	}
	if strings.TrimSpace(state.Name) == "" {
		state.Done = true
		return false
	}
	state.ID = normalizeResponsesCallID(state.ID)
	c.ensureToolStarted(state, events)
	if !state.Started {
		return false
	}
	c.sendPendingToolArguments(state, events)
	state.Done = true
	item := c.toolItemForState(state, state.Arguments.String(), "completed")
	switch common.Interface2String(item["type"]) {
	case "function_call":
		*events = append(*events, ChatToResponsesStreamEvent{
			Event: "response.function_call_arguments.done",
			Payload: map[string]any{
				"output_index": c.toolOutputIndex(state),
				"item_id":      state.ItemID,
				"arguments":    common.Interface2String(item["arguments"]),
			},
		})
	case "custom_tool_call":
		input := common.Interface2String(item["input"])
		if input != "" {
			*events = append(*events, ChatToResponsesStreamEvent{
				Event: "response.custom_tool_call_input.delta",
				Payload: map[string]any{
					"output_index": c.toolOutputIndex(state),
					"item_id":      state.ItemID,
					"delta":        input,
				},
			})
		}
		*events = append(*events, ChatToResponsesStreamEvent{
			Event: "response.custom_tool_call_input.done",
			Payload: map[string]any{
				"output_index": c.toolOutputIndex(state),
				"item_id":      state.ItemID,
				"input":        input,
			},
		})
	}
	*events = append(*events, ChatToResponsesStreamEvent{
		Event: "response.output_item.done",
		Payload: map[string]any{
			"output_index": c.toolOutputIndex(state),
			"item":         item,
		},
	})
	return true
}

func (c *ChatToResponsesStreamConverter) sortedToolStates() []*chatToResponsesStreamToolState {
	states := make([]*chatToResponsesStreamToolState, 0, len(c.toolStates))
	for _, state := range c.toolStates {
		states = append(states, state)
	}
	sort.Slice(states, func(i, j int) bool {
		return states[i].Index < states[j].Index
	})
	return states
}

func (c *ChatToResponsesStreamConverter) ensureReasoningStarted(events *[]ChatToResponsesStreamEvent) {
	if c.reasoningOpen {
		return
	}
	c.ensureCreated(events)
	c.reasoningOpen = true
	if c.reasoningIndex < 0 {
		c.reasoningIndex = c.allocateOutputIndex()
	}
	*events = append(*events, ChatToResponsesStreamEvent{
		Event: "response.output_item.added",
		Payload: map[string]any{
			"output_index": c.reasoningIndex,
			"item":         responseReasoningItem(c.reasoningID, ""),
		},
	}, ChatToResponsesStreamEvent{
		Event: "response.reasoning_summary_part.added",
		Payload: map[string]any{
			"output_index":  c.reasoningIndex,
			"item_id":       c.reasoningID,
			"summary_index": 0,
			"part": map[string]any{
				"type": "summary_text",
				"text": "",
			},
		},
	})
}

func (c *ChatToResponsesStreamConverter) finalizeReasoning(events *[]ChatToResponsesStreamEvent) {
	if !c.reasoningOpen || c.reasoningDone {
		return
	}
	c.reasoningDone = true
	text := c.reasoningText.String()
	*events = append(*events, ChatToResponsesStreamEvent{
		Event: "response.reasoning_summary_text.done",
		Payload: map[string]any{
			"output_index":  c.reasoningIndex,
			"item_id":       c.reasoningID,
			"summary_index": 0,
			"text":          text,
		},
	}, ChatToResponsesStreamEvent{
		Event: "response.reasoning_summary_part.done",
		Payload: map[string]any{
			"output_index":  c.reasoningIndex,
			"item_id":       c.reasoningID,
			"summary_index": 0,
			"part": map[string]any{
				"type": "summary_text",
				"text": text,
			},
		},
	}, ChatToResponsesStreamEvent{
		Event: "response.output_item.done",
		Payload: map[string]any{
			"output_index": c.reasoningIndex,
			"item":         responseReasoningItem(c.reasoningID, text),
		},
	})
}

func (c *ChatToResponsesStreamConverter) ensureTextStarted(events *[]ChatToResponsesStreamEvent) {
	if c.textOpen {
		return
	}
	c.ensureCreated(events)
	c.textOpen = true
	if c.textIndex < 0 {
		c.textIndex = c.allocateOutputIndex()
	}
	item := ResponseMessageItem(c.textID, "")
	*events = append(*events, ChatToResponsesStreamEvent{
		Event: "response.output_item.added",
		Payload: map[string]any{
			"output_index": c.textIndex,
			"item":         item,
		},
	}, ChatToResponsesStreamEvent{
		Event: "response.content_part.added",
		Payload: map[string]any{
			"output_index":  c.textIndex,
			"item_id":       c.textID,
			"content_index": 0,
			"part": map[string]any{
				"type":        "output_text",
				"text":        "",
				"annotations": []any{},
			},
		},
	})
}

func (c *ChatToResponsesStreamConverter) finalizeText(events *[]ChatToResponsesStreamEvent) {
	if !c.textOpen || c.textDone {
		return
	}
	c.textDone = true
	text := c.outputText.String()
	*events = append(*events, ChatToResponsesStreamEvent{
		Event: "response.output_text.done",
		Payload: map[string]any{
			"output_index":  c.textIndex,
			"item_id":       c.textID,
			"content_index": 0,
			"text":          text,
		},
	}, ChatToResponsesStreamEvent{
		Event: "response.content_part.done",
		Payload: map[string]any{
			"output_index":  c.textIndex,
			"item_id":       c.textID,
			"content_index": 0,
			"part": map[string]any{
				"type":        "output_text",
				"text":        text,
				"annotations": []any{},
			},
		},
	}, ChatToResponsesStreamEvent{
		Event: "response.output_item.done",
		Payload: map[string]any{
			"output_index": c.textIndex,
			"item":         ResponseMessageItem(c.textID, text),
		},
	})
}

type chatToResponsesStreamToolState struct {
	Index            int
	OutputIndex      int
	ID               string
	Name             string
	Arguments        strings.Builder
	SentArgumentsLen int
	ItemID           string
	Started          bool
	Done             bool
}

type indexedResponsesOutputItem struct {
	index int
	item  map[string]any
}

func sortedResponsesOutputItems(items []indexedResponsesOutputItem) []any {
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].index < items[j].index
	})
	output := make([]any, 0, len(items))
	for _, item := range items {
		if item.item != nil {
			output = append(output, item.item)
		}
	}
	return output
}

func mergeResponsesUsage(response map[string]any, usage *dto.Usage) map[string]any {
	response["usage"] = usage
	return response
}
