package tencent

import "testing"

func TestStreamResponseTencent2OpenAIUsesUpstreamModel(t *testing.T) {
	got := streamResponseTencent2OpenAI(&TencentChatResponse{}, "hunyuan-standard")

	if got.Model != "hunyuan-standard" {
		t.Fatalf("model = %q, want upstream model", got.Model)
	}
}

func TestStreamResponseTencent2OpenAIFallsBackToLegacyModel(t *testing.T) {
	got := streamResponseTencent2OpenAI(&TencentChatResponse{}, "")

	if got.Model != "tencent-hunyuan" {
		t.Fatalf("model = %q, want legacy fallback", got.Model)
	}
}

func TestResponseTencent2OpenAIUsesUpstreamModel(t *testing.T) {
	got := responseTencent2OpenAI(&TencentChatResponse{}, "hunyuan-pro")

	if got.Model != "hunyuan-pro" {
		t.Fatalf("model = %q, want upstream model", got.Model)
	}
}
