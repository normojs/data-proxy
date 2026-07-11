package xunfei

import "testing"

func TestStreamResponseXunfei2OpenAIUsesUpstreamModel(t *testing.T) {
	got := streamResponseXunfei2OpenAI(&XunfeiChatResponse{}, "spark-max")

	if got.Model != "spark-max" {
		t.Fatalf("model = %q, want upstream model", got.Model)
	}
	if len(got.Choices) != 1 {
		t.Fatalf("choices = %d, want 1", len(got.Choices))
	}
}

func TestStreamResponseXunfei2OpenAIFallsBackToLegacyModel(t *testing.T) {
	got := streamResponseXunfei2OpenAI(&XunfeiChatResponse{}, "")

	if got.Model != "SparkDesk" {
		t.Fatalf("model = %q, want legacy fallback", got.Model)
	}
}

func TestResponseXunfei2OpenAIUsesUpstreamModel(t *testing.T) {
	got := responseXunfei2OpenAI(&XunfeiChatResponse{}, "SparkDesk-v4.0")

	if got.Model != "SparkDesk-v4.0" {
		t.Fatalf("model = %q, want upstream model", got.Model)
	}
}
