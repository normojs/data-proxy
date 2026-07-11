package mokaai

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
)

func TestEmbeddingResponseMoka2OpenAIUsesUpstreamModel(t *testing.T) {
	response := &dto.EmbeddingResponse{
		Object: "list",
		Model:  "moka-response-model",
		Data: []dto.EmbeddingResponseItem{
			{Object: "embedding", Index: 0, Embedding: []float64{0.1, 0.2}},
		},
		Usage: dto.Usage{PromptTokens: 2, TotalTokens: 2},
	}

	got := embeddingResponseMoka2OpenAI(response, "moka-embed")

	if got.Model != "moka-embed" {
		t.Fatalf("model = %q, want %q", got.Model, "moka-embed")
	}
	if len(got.Data) != 1 || got.Data[0].Index != 0 || len(got.Data[0].Embedding) != 2 {
		t.Fatalf("unexpected embedding data: %#v", got.Data)
	}
	if got.Usage.TotalTokens != 2 {
		t.Fatalf("usage = %#v, want total tokens 2", got.Usage)
	}
}

func TestEmbeddingResponseMoka2OpenAIFallsBackToResponseModel(t *testing.T) {
	response := &dto.EmbeddingResponse{Model: "moka-response-model"}

	got := embeddingResponseMoka2OpenAI(response, "")

	if got.Model != "moka-response-model" {
		t.Fatalf("model = %q, want response model", got.Model)
	}
}
