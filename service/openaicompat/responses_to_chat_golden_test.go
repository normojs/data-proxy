package openaicompat

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
)

type responsesToChatGoldenFixture struct {
	History []map[string]any           `json:"history,omitempty"`
	Request dto.OpenAIResponsesRequest `json:"request"`
	Options ResponsesToChatOptions     `json:"options,omitempty"`
	Want    responsesToChatGoldenWant  `json:"want"`
}

type responsesToChatGoldenWant struct {
	MessageRoles       []string        `json:"message_roles,omitempty"`
	ToolNames          []string        `json:"tool_names,omitempty"`
	ToolChoiceName     string          `json:"tool_choice_name,omitempty"`
	ReasoningEffort    string          `json:"reasoning_effort,omitempty"`
	Reasoning          json.RawMessage `json:"reasoning,omitempty"`
	Thinking           json.RawMessage `json:"thinking,omitempty"`
	EnableThinking     json.RawMessage `json:"enable_thinking,omitempty"`
	AssistantToolNames []string        `json:"assistant_tool_names,omitempty"`
	ToolMessageCallIDs []string        `json:"tool_message_call_ids,omitempty"`
	ContextMeta        map[string]any  `json:"context_meta,omitempty"`
}

func TestResponsesToChatGoldenFixtures(t *testing.T) {
	files, err := filepath.Glob("testdata/responses_to_chat/*.json")
	require.NoError(t, err)
	require.NotEmpty(t, files)

	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			ResetDefaultResponsesChatHistoryForTest()
			t.Cleanup(ResetDefaultResponsesChatHistoryForTest)

			raw, err := os.ReadFile(file)
			require.NoError(t, err)

			var fixture responsesToChatGoldenFixture
			require.NoError(t, common.Unmarshal(raw, &fixture))

			for _, response := range fixture.History {
				DefaultResponsesChatHistory().RecordResponseMap(response)
			}

			chatReq, ctx, err := ResponsesRequestToChatCompletionsRequestWithOptions(&fixture.Request, &fixture.Options)
			require.NoError(t, err)

			if len(fixture.Want.MessageRoles) > 0 {
				require.Equal(t, fixture.Want.MessageRoles, messageRoles(chatReq.Messages))
			}
			if len(fixture.Want.ToolNames) > 0 {
				require.Equal(t, fixture.Want.ToolNames, chatToolNames(chatReq.Tools))
			}
			if fixture.Want.ToolChoiceName != "" {
				require.Equal(t, fixture.Want.ToolChoiceName, chatToolChoiceName(t, chatReq.ToolChoice))
			}
			if fixture.Want.ReasoningEffort != "" {
				require.Equal(t, fixture.Want.ReasoningEffort, chatReq.ReasoningEffort)
			}
			if len(fixture.Want.Reasoning) > 0 {
				require.JSONEq(t, string(fixture.Want.Reasoning), string(chatReq.Reasoning))
			}
			if len(fixture.Want.Thinking) > 0 {
				require.JSONEq(t, string(fixture.Want.Thinking), string(chatReq.THINKING))
			}
			if len(fixture.Want.EnableThinking) > 0 {
				require.JSONEq(t, string(fixture.Want.EnableThinking), string(chatReq.EnableThinking))
			}
			if len(fixture.Want.AssistantToolNames) > 0 {
				require.Equal(t, fixture.Want.AssistantToolNames, assistantToolNames(chatReq.Messages))
			}
			if len(fixture.Want.ToolMessageCallIDs) > 0 {
				require.Equal(t, fixture.Want.ToolMessageCallIDs, toolMessageCallIDs(chatReq.Messages))
			}
			if len(fixture.Want.ContextMeta) > 0 {
				requireJSONSubset(t, fixture.Want.ContextMeta, ctx.RequestConversionMeta())
			}
		})
	}
}

func messageRoles(messages []dto.Message) []string {
	roles := make([]string, 0, len(messages))
	for _, message := range messages {
		roles = append(roles, message.Role)
	}
	return roles
}

func chatToolNames(tools []dto.ToolCallRequest) []string {
	names := make([]string, 0, len(tools))
	for _, tool := range tools {
		names = append(names, tool.Function.Name)
	}
	return names
}

func chatToolChoiceName(t *testing.T, toolChoice any) string {
	t.Helper()
	raw, err := common.Marshal(toolChoice)
	require.NoError(t, err)
	var obj map[string]any
	require.NoError(t, common.Unmarshal(raw, &obj))
	fn, ok := obj["function"].(map[string]any)
	require.True(t, ok)
	name, ok := fn["name"].(string)
	require.True(t, ok)
	return name
}

func assistantToolNames(messages []dto.Message) []string {
	names := make([]string, 0)
	for _, message := range messages {
		if message.Role != "assistant" {
			continue
		}
		for _, tool := range message.ParseToolCalls() {
			names = append(names, tool.Function.Name)
		}
	}
	return names
}

func toolMessageCallIDs(messages []dto.Message) []string {
	callIDs := make([]string, 0)
	for _, message := range messages {
		if message.Role == "tool" {
			callIDs = append(callIDs, message.ToolCallId)
		}
	}
	return callIDs
}

func requireJSONSubset(t *testing.T, want map[string]any, got map[string]interface{}) {
	t.Helper()
	subset := make(map[string]any, len(want))
	for key := range want {
		value, ok := got[key]
		require.Truef(t, ok, "missing context meta key %q", key)
		subset[key] = value
	}
	wantJSON, err := common.Marshal(want)
	require.NoError(t, err)
	gotJSON, err := common.Marshal(subset)
	require.NoError(t, err)
	require.JSONEq(t, string(wantJSON), string(gotJSON))
}
