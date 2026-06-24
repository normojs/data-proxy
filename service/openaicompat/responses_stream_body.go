package openaicompat

import (
	"bufio"
	"bytes"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

func ResponsesStreamBodyToResponse(body []byte) (*dto.OpenAIResponsesResponse, bool, error) {
	if !looksLikeResponsesSSE(body) {
		return nil, false, nil
	}

	scanner := bufio.NewScanner(bytes.NewReader(body))
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)

	var (
		dataLines []string
		seen      bool
		base      *dto.OpenAIResponsesResponse
		completed *dto.OpenAIResponsesResponse
		text      strings.Builder
		output    []dto.ResponsesOutput
	)

	flush := func() error {
		if len(dataLines) == 0 {
			return nil
		}
		data := strings.TrimSpace(strings.Join(dataLines, "\n"))
		dataLines = dataLines[:0]
		if data == "" || data == "[DONE]" {
			return nil
		}

		var event dto.ResponsesStreamResponse
		if err := common.UnmarshalJsonStr(data, &event); err != nil {
			return err
		}
		seen = true

		switch event.Type {
		case "response.created", "response.in_progress":
			if event.Response != nil {
				base = event.Response
			}
		case "response.output_text.delta":
			text.WriteString(event.Delta)
		case "response.output_item.done":
			if event.Item != nil {
				output = append(output, *event.Item)
			}
		case "response.completed":
			if event.Response != nil {
				completed = event.Response
			}
		case "response.failed":
			if event.Response != nil {
				completed = event.Response
			}
		}
		return nil
	}

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			if err := flush(); err != nil {
				return nil, true, err
			}
			continue
		}
		if strings.HasPrefix(line, "data:") {
			dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, true, err
	}
	if err := flush(); err != nil {
		return nil, true, err
	}

	if completed != nil {
		return completed, true, nil
	}
	if !seen {
		return nil, true, nil
	}

	if base == nil {
		base = &dto.OpenAIResponsesResponse{
			ID:        "resp_" + common.GetUUID(),
			Object:    "response",
			CreatedAt: int(time.Now().Unix()),
		}
	}
	base.Status = rawJSONString("completed")
	if len(output) == 0 && text.Len() > 0 {
		output = append(output, dto.ResponsesOutput{
			Type:   "message",
			ID:     "msg_" + base.ID,
			Status: "completed",
			Role:   "assistant",
			Content: []dto.ResponsesOutputContent{
				{
					Type:        "output_text",
					Text:        text.String(),
					Annotations: []interface{}{},
				},
			},
		})
	}
	base.Output = output
	return base, true, nil
}

func ChatCompletionsStreamBodyToResponses(body []byte, req *dto.OpenAIResponsesRequest, ctx *ResponsesToChatContext) (map[string]any, *dto.Usage, bool, error) {
	if !looksLikeChatCompletionsSSE(body) {
		return nil, nil, false, nil
	}

	scanner := bufio.NewScanner(bytes.NewReader(body))
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)

	converter := NewChatToResponsesStreamConverter(req, ctx)
	dataLines := make([]string, 0)
	seen := false

	flush := func() error {
		if len(dataLines) == 0 {
			return nil
		}
		data := strings.TrimSpace(strings.Join(dataLines, "\n"))
		dataLines = dataLines[:0]
		if data == "" || data == "[DONE]" {
			return nil
		}

		var chunk dto.ChatCompletionsStreamResponse
		if err := common.UnmarshalJsonStr(data, &chunk); err != nil {
			return err
		}
		seen = true
		_ = converter.HandleChunk(chunk)
		return nil
	}

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			if err := flush(); err != nil {
				return nil, nil, true, err
			}
			continue
		}
		if strings.HasPrefix(line, "data:") {
			dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, true, err
	}
	if err := flush(); err != nil {
		return nil, nil, true, err
	}
	if !seen {
		return nil, nil, true, nil
	}

	var response map[string]any
	for _, event := range converter.Finish() {
		if event.Event != "response.completed" && event.Event != "response.failed" {
			continue
		}
		if value, ok := event.Payload["response"].(map[string]any); ok {
			response = value
		}
	}
	if response == nil {
		return nil, nil, true, nil
	}
	usage, _ := response["usage"].(*dto.Usage)
	if usage == nil {
		usage = ChatUsageToResponsesUsage(converter.Usage())
	}
	return response, usage, true, nil
}

func looksLikeResponsesSSE(body []byte) bool {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return false
	}
	return strings.HasPrefix(trimmed, "event: response.") || strings.HasPrefix(trimmed, "data: {") || strings.Contains(trimmed, "\nevent: response.") || strings.Contains(trimmed, "\ndata: {")
}

func looksLikeChatCompletionsSSE(body []byte) bool {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return false
	}
	return strings.Contains(trimmed, "\"choices\"") &&
		(strings.HasPrefix(trimmed, "data: {") ||
			strings.Contains(trimmed, "\ndata: {") ||
			strings.HasPrefix(trimmed, "event:") ||
			strings.Contains(trimmed, "\nevent:"))
}

func rawJSONString(value string) []byte {
	raw, _ := common.Marshal(value)
	return raw
}
