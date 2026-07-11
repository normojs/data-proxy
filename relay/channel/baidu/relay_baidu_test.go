package baidu

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
)

func TestEmbeddingResponseBaidu2OpenAIUsesUpstreamModel(t *testing.T) {
	response := &BaiduEmbeddingResponse{
		Object: "list",
		Data: []BaiduEmbeddingData{
			{Object: "embedding", Index: 0, Embedding: []float64{0.1, 0.2}},
		},
		Usage: dto.Usage{PromptTokens: 2, TotalTokens: 2},
	}

	got := embeddingResponseBaidu2OpenAI(response, "Embedding-V1")

	if got.Model != "Embedding-V1" {
		t.Fatalf("model = %q, want %q", got.Model, "Embedding-V1")
	}
	if len(got.Data) != 1 || got.Data[0].Index != 0 || len(got.Data[0].Embedding) != 2 {
		t.Fatalf("unexpected embedding data: %#v", got.Data)
	}
	if got.Usage.TotalTokens != 2 {
		t.Fatalf("usage = %#v, want total tokens 2", got.Usage)
	}
}

func TestEmbeddingResponseBaidu2OpenAIFallsBackToLegacyModel(t *testing.T) {
	got := embeddingResponseBaidu2OpenAI(&BaiduEmbeddingResponse{}, "")

	if got.Model != "baidu-embedding" {
		t.Fatalf("model = %q, want legacy fallback", got.Model)
	}
}

func TestStreamResponseBaidu2OpenAIUsesUpstreamModel(t *testing.T) {
	got := streamResponseBaidu2OpenAI(&BaiduChatStreamResponse{
		BaiduChatResponse: BaiduChatResponse{Result: "hello"},
	}, "ERNIE-Speed-8K")

	if got.Model != "ERNIE-Speed-8K" {
		t.Fatalf("model = %q, want upstream model", got.Model)
	}
}

func TestResponseBaidu2OpenAIUsesUpstreamModel(t *testing.T) {
	got := responseBaidu2OpenAI(&BaiduChatResponse{Result: "hello"}, "ERNIE-4.0")

	if got.Model != "ERNIE-4.0" {
		t.Fatalf("model = %q, want upstream model", got.Model)
	}
}
