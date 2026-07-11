package channel_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/ali"
	"github.com/QuantumNous/new-api/relay/channel/aws"
	"github.com/QuantumNous/new-api/relay/channel/baidu"
	"github.com/QuantumNous/new-api/relay/channel/baidu_v2"
	"github.com/QuantumNous/new-api/relay/channel/claude"
	"github.com/QuantumNous/new-api/relay/channel/cloudflare"
	"github.com/QuantumNous/new-api/relay/channel/cohere"
	"github.com/QuantumNous/new-api/relay/channel/coze"
	"github.com/QuantumNous/new-api/relay/channel/deepseek"
	"github.com/QuantumNous/new-api/relay/channel/dify"
	"github.com/QuantumNous/new-api/relay/channel/gemini"
	"github.com/QuantumNous/new-api/relay/channel/jimeng"
	"github.com/QuantumNous/new-api/relay/channel/jina"
	"github.com/QuantumNous/new-api/relay/channel/minimax"
	"github.com/QuantumNous/new-api/relay/channel/mistral"
	"github.com/QuantumNous/new-api/relay/channel/mokaai"
	"github.com/QuantumNous/new-api/relay/channel/moonshot"
	"github.com/QuantumNous/new-api/relay/channel/ollama"
	"github.com/QuantumNous/new-api/relay/channel/palm"
	"github.com/QuantumNous/new-api/relay/channel/perplexity"
	"github.com/QuantumNous/new-api/relay/channel/replicate"
	"github.com/QuantumNous/new-api/relay/channel/siliconflow"
	"github.com/QuantumNous/new-api/relay/channel/tencent"
	"github.com/QuantumNous/new-api/relay/channel/vertex"
	"github.com/QuantumNous/new-api/relay/channel/volcengine"
	"github.com/QuantumNous/new-api/relay/channel/xai"
	"github.com/QuantumNous/new-api/relay/channel/xunfei"
	"github.com/QuantumNous/new-api/relay/channel/zhipu"
	"github.com/QuantumNous/new-api/relay/channel/zhipu_4v"
)

func TestUnsupportedClaudeAdaptorsReturnErrors(t *testing.T) {
	tests := []struct {
		name    string
		convert func() (any, error)
	}{
		{
			name:    "baidu",
			convert: func() (any, error) { return (&baidu.Adaptor{}).ConvertClaudeRequest(nil, nil, &dto.ClaudeRequest{}) },
		},
		{
			name: "cloudflare",
			convert: func() (any, error) {
				return (&cloudflare.Adaptor{}).ConvertClaudeRequest(nil, nil, &dto.ClaudeRequest{})
			},
		},
		{
			name:    "cohere",
			convert: func() (any, error) { return (&cohere.Adaptor{}).ConvertClaudeRequest(nil, nil, &dto.ClaudeRequest{}) },
		},
		{
			name:    "dify",
			convert: func() (any, error) { return (&dify.Adaptor{}).ConvertClaudeRequest(nil, nil, &dto.ClaudeRequest{}) },
		},
		{
			name:    "jina",
			convert: func() (any, error) { return (&jina.Adaptor{}).ConvertClaudeRequest(nil, nil, &dto.ClaudeRequest{}) },
		},
		{
			name:    "mistral",
			convert: func() (any, error) { return (&mistral.Adaptor{}).ConvertClaudeRequest(nil, nil, &dto.ClaudeRequest{}) },
		},
		{
			name:    "mokaai",
			convert: func() (any, error) { return (&mokaai.Adaptor{}).ConvertClaudeRequest(nil, nil, &dto.ClaudeRequest{}) },
		},
		{
			name:    "palm",
			convert: func() (any, error) { return (&palm.Adaptor{}).ConvertClaudeRequest(nil, nil, &dto.ClaudeRequest{}) },
		},
		{
			name:    "tencent",
			convert: func() (any, error) { return (&tencent.Adaptor{}).ConvertClaudeRequest(nil, nil, &dto.ClaudeRequest{}) },
		},
		{
			name:    "xunfei",
			convert: func() (any, error) { return (&xunfei.Adaptor{}).ConvertClaudeRequest(nil, nil, &dto.ClaudeRequest{}) },
		},
		{
			name:    "zhipu",
			convert: func() (any, error) { return (&zhipu.Adaptor{}).ConvertClaudeRequest(nil, nil, &dto.ClaudeRequest{}) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if recovered := recover(); recovered != nil {
					t.Fatalf("ConvertClaudeRequest panicked: %v", recovered)
				}
			}()

			result, err := tt.convert()
			if err == nil {
				t.Fatal("expected unsupported error")
			}
			if result != nil {
				t.Fatalf("expected nil result, got %T", result)
			}
			var unsupported *channel.UnsupportedFeatureError
			if !errors.As(err, &unsupported) {
				t.Fatalf("expected UnsupportedFeatureError, got %T: %v", err, err)
			}
			if unsupported.Provider != tt.name || unsupported.Feature != "ConvertClaudeRequest" {
				t.Fatalf("unexpected unsupported feature context: provider=%q feature=%q", unsupported.Provider, unsupported.Feature)
			}
			if !strings.Contains(err.Error(), "not implemented") {
				t.Fatalf("expected not implemented error, got %q", err.Error())
			}
		})
	}
}

func TestTypedUnsupportedAdaptorFeatures(t *testing.T) {
	tests := []struct {
		name     string
		convert  func() (any, error)
		provider string
		feature  string
	}{
		{
			name:     "ali gemini",
			convert:  func() (any, error) { return (&ali.Adaptor{}).ConvertGeminiRequest(nil, nil, &dto.GeminiChatRequest{}) },
			provider: "ali",
			feature:  "ConvertGeminiRequest",
		},
		{
			name: "ali audio",
			convert: func() (any, error) {
				reader, err := (&ali.Adaptor{}).ConvertAudioRequest(nil, nil, dto.AudioRequest{})
				return reader, err
			},
			provider: "ali",
			feature:  "ConvertAudioRequest",
		},
		{
			name:     "xai gemini",
			convert:  func() (any, error) { return (&xai.Adaptor{}).ConvertGeminiRequest(nil, nil, &dto.GeminiChatRequest{}) },
			provider: "xai",
			feature:  "ConvertGeminiRequest",
		},
		{
			name:     "xai claude",
			convert:  func() (any, error) { return (&xai.Adaptor{}).ConvertClaudeRequest(nil, nil, &dto.ClaudeRequest{}) },
			provider: "xai",
			feature:  "ConvertClaudeRequest",
		},
		{
			name: "xai audio",
			convert: func() (any, error) {
				reader, err := (&xai.Adaptor{}).ConvertAudioRequest(nil, nil, dto.AudioRequest{})
				return reader, err
			},
			provider: "xai",
			feature:  "ConvertAudioRequest",
		},
		{
			name:     "xai embedding",
			convert:  func() (any, error) { return (&xai.Adaptor{}).ConvertEmbeddingRequest(nil, nil, dto.EmbeddingRequest{}) },
			provider: "xai",
			feature:  "ConvertEmbeddingRequest",
		},
		{
			name: "deepseek gemini",
			convert: func() (any, error) {
				return (&deepseek.Adaptor{}).ConvertGeminiRequest(nil, nil, &dto.GeminiChatRequest{})
			},
			provider: "deepseek",
			feature:  "ConvertGeminiRequest",
		},
		{
			name: "gemini audio",
			convert: func() (any, error) {
				reader, err := (&gemini.Adaptor{}).ConvertAudioRequest(nil, nil, dto.AudioRequest{})
				return reader, err
			},
			provider: "gemini",
			feature:  "ConvertAudioRequest",
		},
		{
			name: "minimax gemini",
			convert: func() (any, error) {
				return (&minimax.Adaptor{}).ConvertGeminiRequest(nil, nil, &dto.GeminiChatRequest{})
			},
			provider: "minimax",
			feature:  "ConvertGeminiRequest",
		},
		{
			name: "moonshot gemini",
			convert: func() (any, error) {
				return (&moonshot.Adaptor{}).ConvertGeminiRequest(nil, nil, &dto.GeminiChatRequest{})
			},
			provider: "moonshot",
			feature:  "ConvertGeminiRequest",
		},
		{
			name: "ollama gemini",
			convert: func() (any, error) {
				return (&ollama.Adaptor{}).ConvertGeminiRequest(nil, nil, &dto.GeminiChatRequest{})
			},
			provider: "ollama",
			feature:  "ConvertGeminiRequest",
		},
		{
			name: "perplexity gemini",
			convert: func() (any, error) {
				return (&perplexity.Adaptor{}).ConvertGeminiRequest(nil, nil, &dto.GeminiChatRequest{})
			},
			provider: "perplexity",
			feature:  "ConvertGeminiRequest",
		},
		{
			name: "siliconflow gemini",
			convert: func() (any, error) {
				return (&siliconflow.Adaptor{}).ConvertGeminiRequest(nil, nil, &dto.GeminiChatRequest{})
			},
			provider: "siliconflow",
			feature:  "ConvertGeminiRequest",
		},
		{
			name: "aws gemini",
			convert: func() (any, error) {
				return (&aws.Adaptor{}).ConvertGeminiRequest(nil, nil, &dto.GeminiChatRequest{})
			},
			provider: "aws",
			feature:  "ConvertGeminiRequest",
		},
		{
			name: "baidu v2 gemini",
			convert: func() (any, error) {
				return (&baidu_v2.Adaptor{}).ConvertGeminiRequest(nil, nil, &dto.GeminiChatRequest{})
			},
			provider: "baidu_v2",
			feature:  "ConvertGeminiRequest",
		},
		{
			name: "claude gemini",
			convert: func() (any, error) {
				return (&claude.Adaptor{}).ConvertGeminiRequest(nil, nil, &dto.GeminiChatRequest{})
			},
			provider: "claude",
			feature:  "ConvertGeminiRequest",
		},
		{
			name: "coze gemini",
			convert: func() (any, error) {
				return (&coze.Adaptor{}).ConvertGeminiRequest(nil, nil, &dto.GeminiChatRequest{})
			},
			provider: "coze",
			feature:  "ConvertGeminiRequest",
		},
		{
			name: "jimeng header",
			convert: func() (any, error) {
				return nil, (&jimeng.Adaptor{}).SetupRequestHeader(nil, nil, nil)
			},
			provider: "jimeng",
			feature:  "SetupRequestHeader",
		},
		{
			name: "replicate gemini",
			convert: func() (any, error) {
				return (&replicate.Adaptor{}).ConvertGeminiRequest(nil, nil, &dto.GeminiChatRequest{})
			},
			provider: "replicate",
			feature:  "ConvertGeminiRequest",
		},
		{
			name: "vertex audio",
			convert: func() (any, error) {
				reader, err := (&vertex.Adaptor{}).ConvertAudioRequest(nil, nil, dto.AudioRequest{})
				return reader, err
			},
			provider: "vertex",
			feature:  "ConvertAudioRequest",
		},
		{
			name: "volcengine gemini",
			convert: func() (any, error) {
				return (&volcengine.Adaptor{}).ConvertGeminiRequest(nil, nil, &dto.GeminiChatRequest{})
			},
			provider: "volcengine",
			feature:  "ConvertGeminiRequest",
		},
		{
			name: "zhipu 4v gemini",
			convert: func() (any, error) {
				return (&zhipu_4v.Adaptor{}).ConvertGeminiRequest(nil, nil, &dto.GeminiChatRequest{})
			},
			provider: "zhipu_4v",
			feature:  "ConvertGeminiRequest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.convert()
			if err == nil {
				t.Fatal("expected unsupported feature error")
			}
			if result != nil {
				t.Fatalf("expected nil result, got %T", result)
			}

			var unsupported *channel.UnsupportedFeatureError
			if !errors.As(err, &unsupported) {
				t.Fatalf("expected UnsupportedFeatureError, got %T: %v", err, err)
			}
			if unsupported.Provider != tt.provider || unsupported.Feature != tt.feature {
				t.Fatalf("unexpected unsupported feature context: provider=%q feature=%q", unsupported.Provider, unsupported.Feature)
			}
			if !strings.Contains(err.Error(), "not implemented") {
				t.Fatalf("expected not implemented wording, got %q", err.Error())
			}
		})
	}
}

func TestUnsupportedRerankAdaptorsReturnTypedErrors(t *testing.T) {
	tests := []struct {
		provider string
		convert  func() (any, error)
	}{
		{provider: "aws", convert: func() (any, error) { return (&aws.Adaptor{}).ConvertRerankRequest(nil, 0, dto.RerankRequest{}) }},
		{provider: "baidu", convert: func() (any, error) { return (&baidu.Adaptor{}).ConvertRerankRequest(nil, 0, dto.RerankRequest{}) }},
		{provider: "claude", convert: func() (any, error) { return (&claude.Adaptor{}).ConvertRerankRequest(nil, 0, dto.RerankRequest{}) }},
		{provider: "deepseek", convert: func() (any, error) { return (&deepseek.Adaptor{}).ConvertRerankRequest(nil, 0, dto.RerankRequest{}) }},
		{provider: "dify", convert: func() (any, error) { return (&dify.Adaptor{}).ConvertRerankRequest(nil, 0, dto.RerankRequest{}) }},
		{provider: "gemini", convert: func() (any, error) { return (&gemini.Adaptor{}).ConvertRerankRequest(nil, 0, dto.RerankRequest{}) }},
		{provider: "minimax", convert: func() (any, error) { return (&minimax.Adaptor{}).ConvertRerankRequest(nil, 0, dto.RerankRequest{}) }},
		{provider: "mistral", convert: func() (any, error) { return (&mistral.Adaptor{}).ConvertRerankRequest(nil, 0, dto.RerankRequest{}) }},
		{provider: "mokaai", convert: func() (any, error) { return (&mokaai.Adaptor{}).ConvertRerankRequest(nil, 0, dto.RerankRequest{}) }},
		{provider: "ollama", convert: func() (any, error) { return (&ollama.Adaptor{}).ConvertRerankRequest(nil, 0, dto.RerankRequest{}) }},
		{provider: "palm", convert: func() (any, error) { return (&palm.Adaptor{}).ConvertRerankRequest(nil, 0, dto.RerankRequest{}) }},
		{provider: "perplexity", convert: func() (any, error) { return (&perplexity.Adaptor{}).ConvertRerankRequest(nil, 0, dto.RerankRequest{}) }},
		{provider: "tencent", convert: func() (any, error) { return (&tencent.Adaptor{}).ConvertRerankRequest(nil, 0, dto.RerankRequest{}) }},
		{provider: "vertex", convert: func() (any, error) { return (&vertex.Adaptor{}).ConvertRerankRequest(nil, 0, dto.RerankRequest{}) }},
		{provider: "xai", convert: func() (any, error) { return (&xai.Adaptor{}).ConvertRerankRequest(nil, 0, dto.RerankRequest{}) }},
		{provider: "xunfei", convert: func() (any, error) { return (&xunfei.Adaptor{}).ConvertRerankRequest(nil, 0, dto.RerankRequest{}) }},
		{provider: "zhipu", convert: func() (any, error) { return (&zhipu.Adaptor{}).ConvertRerankRequest(nil, 0, dto.RerankRequest{}) }},
		{provider: "zhipu_4v", convert: func() (any, error) { return (&zhipu_4v.Adaptor{}).ConvertRerankRequest(nil, 0, dto.RerankRequest{}) }},
	}

	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			result, err := tt.convert()
			if err == nil {
				t.Fatal("expected unsupported feature error")
			}
			if result != nil {
				t.Fatalf("expected nil result, got %T", result)
			}

			var unsupported *channel.UnsupportedFeatureError
			if !errors.As(err, &unsupported) {
				t.Fatalf("expected UnsupportedFeatureError, got %T: %v", err, err)
			}
			if unsupported.Provider != tt.provider || unsupported.Feature != "ConvertRerankRequest" {
				t.Fatalf("unexpected unsupported feature context: provider=%q feature=%q", unsupported.Provider, unsupported.Feature)
			}
		})
	}
}

func TestVolcengineRerankAdaptorKeepsOpenAICompatibleRequest(t *testing.T) {
	req := dto.RerankRequest{Model: "doubao-rerank"}

	result, err := (&volcengine.Adaptor{}).ConvertRerankRequest(nil, 0, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := result.(dto.RerankRequest)
	if !ok {
		t.Fatalf("expected dto.RerankRequest, got %T", result)
	}
	if got.Model != req.Model {
		t.Fatalf("model = %q, want %q", got.Model, req.Model)
	}
}

func TestBaiduV2EmbeddingAdaptorKeepsOpenAICompatibleRequest(t *testing.T) {
	req := dto.EmbeddingRequest{Model: "embedding-v1", Input: "hello"}

	result, err := (&baidu_v2.Adaptor{}).ConvertEmbeddingRequest(nil, nil, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := result.(dto.EmbeddingRequest)
	if !ok {
		t.Fatalf("expected dto.EmbeddingRequest, got %T", result)
	}
	if got.Model != req.Model || got.Input != req.Input {
		t.Fatalf("embedding request = %#v, want %#v", got, req)
	}
}

func TestMistralEmbeddingAdaptorKeepsOpenAICompatibleRequest(t *testing.T) {
	req := dto.EmbeddingRequest{Model: "mistral-embed", Input: []string{"hello", "world"}}

	result, err := (&mistral.Adaptor{}).ConvertEmbeddingRequest(nil, nil, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := result.(dto.EmbeddingRequest)
	if !ok {
		t.Fatalf("expected dto.EmbeddingRequest, got %T", result)
	}
	if got.Model != req.Model || got.Input == nil {
		t.Fatalf("embedding request = %#v, want %#v", got, req)
	}
}

func TestBaiduV2RerankAdaptorKeepsOpenAICompatibleRequest(t *testing.T) {
	req := dto.RerankRequest{Model: "bce-reranker-base_v1", Query: "hello", Documents: []any{"world"}}

	result, err := (&baidu_v2.Adaptor{}).ConvertRerankRequest(nil, 0, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := result.(dto.RerankRequest)
	if !ok {
		t.Fatalf("expected dto.RerankRequest, got %T", result)
	}
	if got.Model != req.Model || got.Query != req.Query || len(got.Documents) != len(req.Documents) {
		t.Fatalf("rerank request = %#v, want %#v", got, req)
	}
}
