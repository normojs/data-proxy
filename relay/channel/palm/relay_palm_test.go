package palm

import "testing"

func TestStreamResponsePaLM2OpenAIUsesUpstreamModel(t *testing.T) {
	got := streamResponsePaLM2OpenAI(&PaLMChatResponse{
		Candidates: []PaLMChatMessage{{Content: "hello"}},
	}, "chat-bison-001")

	if got.Model != "chat-bison-001" {
		t.Fatalf("model = %q, want upstream model", got.Model)
	}
	if len(got.Choices) != 1 || got.Choices[0].Delta.GetContentString() != "hello" {
		t.Fatalf("unexpected choices: %#v", got.Choices)
	}
}

func TestStreamResponsePaLM2OpenAIFallsBackToLegacyModel(t *testing.T) {
	got := streamResponsePaLM2OpenAI(&PaLMChatResponse{}, "")

	if got.Model != "palm2" {
		t.Fatalf("model = %q, want legacy fallback", got.Model)
	}
}

func TestResponsePaLM2OpenAIUsesUpstreamModel(t *testing.T) {
	got := responsePaLM2OpenAI(&PaLMChatResponse{}, "chat-bison-001")

	if got.Model != "chat-bison-001" {
		t.Fatalf("model = %q, want upstream model", got.Model)
	}
}
