package openaicompat

import (
	"strings"

	"github.com/QuantumNous/new-api/dto"
)

type ResponsesToChatStreamConverter struct {
	responseID string
	createdAt  int64
	model      string

	usage      *dto.Usage
	outputText strings.Builder
	usageText  strings.Builder

	sentStart   bool
	sentStop    bool
	sawToolCall bool

	toolCallIndexByID         map[string]int
	toolCallNameByID          map[string]string
	toolCallArgsByID          map[string]string
	toolCallNameSent          map[string]bool
	toolCallCanonicalIDByItem map[string]string

	hasSentReasoningSummary        bool
	needsReasoningSummarySeparator bool
}

func NewResponsesToChatStreamConverter(responseID string, createdAt int64, model string) *ResponsesToChatStreamConverter {
	return &ResponsesToChatStreamConverter{
		responseID:                     responseID,
		createdAt:                      createdAt,
		model:                          model,
		usage:                          &dto.Usage{},
		toolCallIndexByID:              map[string]int{},
		toolCallNameByID:               map[string]string{},
		toolCallArgsByID:               map[string]string{},
		toolCallNameSent:               map[string]bool{},
		toolCallCanonicalIDByItem:      map[string]string{},
		needsReasoningSummarySeparator: false,
	}
}

func (c *ResponsesToChatStreamConverter) HandleEvent(event dto.ResponsesStreamResponse) []*dto.ChatCompletionsStreamResponse {
	if c == nil {
		return nil
	}
	switch event.Type {
	case "response.created":
		c.updateResponseMeta(event.Response)
	case "response.reasoning_summary_text.delta":
		return c.reasoningSummaryDelta(event.Delta)
	case "response.reasoning_summary_text.done":
		if c.hasSentReasoningSummary {
			c.needsReasoningSummarySeparator = true
		}
	case "response.output_text.delta":
		return c.outputTextDelta(event.Delta)
	case "response.output_item.added", "response.output_item.done":
		return c.outputItem(event.Item)
	case "response.function_call_arguments.delta":
		return c.functionCallArgumentsDelta(event.ItemID, event.Delta)
	case "response.completed":
		c.updateResponseMeta(event.Response)
		return c.stopIfNeeded()
	}
	return nil
}

func (c *ResponsesToChatStreamConverter) Finish() []*dto.ChatCompletionsStreamResponse {
	if c == nil {
		return nil
	}
	chunks := make([]*dto.ChatCompletionsStreamResponse, 0, 2)
	if !c.sentStart {
		chunks = append(chunks, c.startChunk())
		c.sentStart = true
	}
	chunks = append(chunks, c.stopIfNeeded()...)
	return chunks
}

func (c *ResponsesToChatStreamConverter) Usage() *dto.Usage {
	if c == nil || c.usage == nil {
		return &dto.Usage{}
	}
	return c.usage
}

func (c *ResponsesToChatStreamConverter) SetUsage(usage *dto.Usage) {
	if c == nil {
		return
	}
	if usage == nil {
		c.usage = &dto.Usage{}
		return
	}
	c.usage = usage
}

func (c *ResponsesToChatStreamConverter) ResponseID() string {
	if c == nil {
		return ""
	}
	return c.responseID
}

func (c *ResponsesToChatStreamConverter) CreatedAt() int64 {
	if c == nil {
		return 0
	}
	return c.createdAt
}

func (c *ResponsesToChatStreamConverter) Model() string {
	if c == nil {
		return ""
	}
	return c.model
}

func (c *ResponsesToChatStreamConverter) UsageText() string {
	if c == nil {
		return ""
	}
	return c.usageText.String()
}

func (c *ResponsesToChatStreamConverter) updateResponseMeta(resp *dto.OpenAIResponsesResponse) {
	if c == nil || resp == nil {
		return
	}
	if resp.Model != "" {
		c.model = resp.Model
	}
	if resp.CreatedAt != 0 {
		c.createdAt = int64(resp.CreatedAt)
	}
	if resp.Usage != nil {
		c.setResponsesUsage(resp.Usage)
	}
}

func (c *ResponsesToChatStreamConverter) setResponsesUsage(usage *dto.Usage) {
	if c == nil || usage == nil {
		return
	}
	if usage.InputTokens != 0 {
		c.usage.PromptTokens = usage.InputTokens
		c.usage.InputTokens = usage.InputTokens
	}
	if usage.OutputTokens != 0 {
		c.usage.CompletionTokens = usage.OutputTokens
		c.usage.OutputTokens = usage.OutputTokens
	}
	if usage.TotalTokens != 0 {
		c.usage.TotalTokens = usage.TotalTokens
	} else {
		c.usage.TotalTokens = c.usage.PromptTokens + c.usage.CompletionTokens
	}
	if usage.InputTokensDetails != nil {
		c.usage.PromptTokensDetails.CachedTokens = usage.InputTokensDetails.CachedTokens
		c.usage.PromptTokensDetails.ImageTokens = usage.InputTokensDetails.ImageTokens
		c.usage.PromptTokensDetails.AudioTokens = usage.InputTokensDetails.AudioTokens
	}
	if usage.CompletionTokenDetails.ReasoningTokens != 0 {
		c.usage.CompletionTokenDetails.ReasoningTokens = usage.CompletionTokenDetails.ReasoningTokens
	}
}

func (c *ResponsesToChatStreamConverter) reasoningSummaryDelta(delta string) []*dto.ChatCompletionsStreamResponse {
	if delta == "" {
		return nil
	}
	if c.needsReasoningSummarySeparator {
		if strings.HasPrefix(delta, "\n\n") {
			c.needsReasoningSummarySeparator = false
		} else if strings.HasPrefix(delta, "\n") {
			delta = "\n" + delta
			c.needsReasoningSummarySeparator = false
		} else {
			delta = "\n\n" + delta
			c.needsReasoningSummarySeparator = false
		}
	}

	c.usageText.WriteString(delta)
	c.hasSentReasoningSummary = true
	return append(c.startIfNeeded(), c.reasoningChunk(delta))
}

func (c *ResponsesToChatStreamConverter) outputTextDelta(delta string) []*dto.ChatCompletionsStreamResponse {
	if delta == "" {
		return c.startIfNeeded()
	}
	c.outputText.WriteString(delta)
	c.usageText.WriteString(delta)
	return append(c.startIfNeeded(), c.contentChunk(delta))
}

func (c *ResponsesToChatStreamConverter) outputItem(item *dto.ResponsesOutput) []*dto.ChatCompletionsStreamResponse {
	if item == nil || item.Type != "function_call" {
		return nil
	}

	itemID := strings.TrimSpace(item.ID)
	callID := strings.TrimSpace(item.CallId)
	if callID == "" {
		callID = itemID
	}
	if itemID != "" && callID != "" {
		c.toolCallCanonicalIDByItem[itemID] = callID
	}
	name := strings.TrimSpace(item.Name)
	if name != "" {
		c.toolCallNameByID[callID] = name
	}

	newArgs := item.ArgumentsString()
	prevArgs := c.toolCallArgsByID[callID]
	argsDelta := ""
	if newArgs != "" {
		if strings.HasPrefix(newArgs, prevArgs) {
			argsDelta = newArgs[len(prevArgs):]
		} else {
			argsDelta = newArgs
		}
		c.toolCallArgsByID[callID] = newArgs
	}
	return c.toolCallDelta(callID, name, argsDelta)
}

func (c *ResponsesToChatStreamConverter) functionCallArgumentsDelta(itemID string, delta string) []*dto.ChatCompletionsStreamResponse {
	callID := c.toolCallCanonicalIDByItem[strings.TrimSpace(itemID)]
	if callID == "" {
		callID = strings.TrimSpace(itemID)
	}
	if callID == "" {
		return nil
	}
	c.toolCallArgsByID[callID] += delta
	return c.toolCallDelta(callID, "", delta)
}

func (c *ResponsesToChatStreamConverter) toolCallDelta(callID string, name string, argsDelta string) []*dto.ChatCompletionsStreamResponse {
	if callID == "" {
		return nil
	}
	if c.outputText.Len() > 0 {
		// Match non-stream behavior: once assistant text exists, do not also emit tool calls.
		return nil
	}
	chunks := c.startIfNeeded()

	idx, ok := c.toolCallIndexByID[callID]
	if !ok {
		idx = len(c.toolCallIndexByID)
		c.toolCallIndexByID[callID] = idx
	}
	if name != "" {
		c.toolCallNameByID[callID] = name
	}
	if c.toolCallNameByID[callID] != "" {
		name = c.toolCallNameByID[callID]
	}

	tool := dto.ToolCallResponse{
		ID:   callID,
		Type: "function",
		Function: dto.FunctionResponse{
			Arguments: argsDelta,
		},
	}
	tool.SetIndex(idx)
	if name != "" && !c.toolCallNameSent[callID] {
		tool.Function.Name = name
		c.toolCallNameSent[callID] = true
	}

	c.sawToolCall = true
	if tool.Function.Name != "" {
		c.usageText.WriteString(tool.Function.Name)
	}
	if argsDelta != "" {
		c.usageText.WriteString(argsDelta)
	}
	return append(chunks, c.toolCallChunk(tool))
}

func (c *ResponsesToChatStreamConverter) startIfNeeded() []*dto.ChatCompletionsStreamResponse {
	if c.sentStart {
		return nil
	}
	c.sentStart = true
	return []*dto.ChatCompletionsStreamResponse{c.startChunk()}
}

func (c *ResponsesToChatStreamConverter) stopIfNeeded() []*dto.ChatCompletionsStreamResponse {
	if c.sentStop {
		return nil
	}
	c.sentStop = true
	finishReason := "stop"
	if c.sawToolCall && c.outputText.Len() == 0 {
		finishReason = "tool_calls"
	}
	return []*dto.ChatCompletionsStreamResponse{c.stopChunk(finishReason)}
}

func (c *ResponsesToChatStreamConverter) startChunk() *dto.ChatCompletionsStreamResponse {
	empty := ""
	return &dto.ChatCompletionsStreamResponse{
		Id:      c.responseID,
		Object:  "chat.completion.chunk",
		Created: c.createdAt,
		Model:   c.model,
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{
				Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
					Role:    "assistant",
					Content: &empty,
				},
			},
		},
	}
}

func (c *ResponsesToChatStreamConverter) stopChunk(finishReason string) *dto.ChatCompletionsStreamResponse {
	return &dto.ChatCompletionsStreamResponse{
		Id:      c.responseID,
		Object:  "chat.completion.chunk",
		Created: c.createdAt,
		Model:   c.model,
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{
				FinishReason: &finishReason,
			},
		},
	}
}

func (c *ResponsesToChatStreamConverter) reasoningChunk(delta string) *dto.ChatCompletionsStreamResponse {
	return &dto.ChatCompletionsStreamResponse{
		Id:      c.responseID,
		Object:  "chat.completion.chunk",
		Created: c.createdAt,
		Model:   c.model,
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{
				Index: 0,
				Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
					ReasoningContent: &delta,
				},
			},
		},
	}
}

func (c *ResponsesToChatStreamConverter) contentChunk(delta string) *dto.ChatCompletionsStreamResponse {
	return &dto.ChatCompletionsStreamResponse{
		Id:      c.responseID,
		Object:  "chat.completion.chunk",
		Created: c.createdAt,
		Model:   c.model,
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{
				Index: 0,
				Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
					Content: &delta,
				},
			},
		},
	}
}

func (c *ResponsesToChatStreamConverter) toolCallChunk(tool dto.ToolCallResponse) *dto.ChatCompletionsStreamResponse {
	return &dto.ChatCompletionsStreamResponse{
		Id:      c.responseID,
		Object:  "chat.completion.chunk",
		Created: c.createdAt,
		Model:   c.model,
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{
				Index: 0,
				Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
					ToolCalls: []dto.ToolCallResponse{tool},
				},
			},
		},
	}
}
