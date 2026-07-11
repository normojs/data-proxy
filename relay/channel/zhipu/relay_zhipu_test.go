package zhipu

import "testing"

func TestStreamResponseZhipu2OpenAIUsesUpstreamModel(t *testing.T) {
	got := streamResponseZhipu2OpenAI("hello", "glm-4")

	if got.Model != "glm-4" {
		t.Fatalf("model = %q, want upstream model", got.Model)
	}
}

func TestStreamMetaResponseZhipu2OpenAIUsesUpstreamModel(t *testing.T) {
	got, _ := streamMetaResponseZhipu2OpenAI(&ZhipuStreamMetaResponse{}, "glm-4-air")

	if got.Model != "glm-4-air" {
		t.Fatalf("model = %q, want upstream model", got.Model)
	}
}

func TestStreamResponseZhipu2OpenAIFallsBackToLegacyModel(t *testing.T) {
	got := streamResponseZhipu2OpenAI("hello", "")

	if got.Model != "chatglm" {
		t.Fatalf("model = %q, want legacy fallback", got.Model)
	}
}

func TestResponseZhipu2OpenAIUsesUpstreamModel(t *testing.T) {
	got := responseZhipu2OpenAI(&ZhipuResponse{}, "glm-4-plus")

	if got.Model != "glm-4-plus" {
		t.Fatalf("model = %q, want upstream model", got.Model)
	}
}
