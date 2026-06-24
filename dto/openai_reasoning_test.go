package dto

import (
	"encoding/json"
	"testing"
)

func TestMessageGetReasoningContentExtractsObjectFields(t *testing.T) {
	raw := []byte(`{
		"role": "assistant",
		"content": "",
		"reasoning": {"summary": [{"type": "summary_text", "text": "plan"}]},
		"reasoning_details": [{"type": "reasoning.text", "text": "detail"}]
	}`)

	var msg Message
	if err := json.Unmarshal(raw, &msg); err != nil {
		t.Fatal(err)
	}
	if got, want := msg.GetReasoningContent(), "plan\ndetail"; got != want {
		t.Fatalf("reasoning = %q, want %q", got, want)
	}
}

func TestStreamDeltaGetReasoningContentExtractsDetails(t *testing.T) {
	raw := []byte(`{
		"reasoning_details": [
			{"type": "reasoning.summary", "summary": [{"text": "first"}]},
			{"type": "reasoning.text", "content": "second"}
		]
	}`)

	var delta ChatCompletionsStreamResponseChoiceDelta
	if err := json.Unmarshal(raw, &delta); err != nil {
		t.Fatal(err)
	}
	if got, want := delta.GetReasoningContent(), "first\nsecond"; got != want {
		t.Fatalf("reasoning = %q, want %q", got, want)
	}
}
