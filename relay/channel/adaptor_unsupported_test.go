package channel_test

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel/baidu"
	"github.com/QuantumNous/new-api/relay/channel/cloudflare"
	"github.com/QuantumNous/new-api/relay/channel/cohere"
	"github.com/QuantumNous/new-api/relay/channel/dify"
	"github.com/QuantumNous/new-api/relay/channel/jina"
	"github.com/QuantumNous/new-api/relay/channel/mistral"
	"github.com/QuantumNous/new-api/relay/channel/mokaai"
	"github.com/QuantumNous/new-api/relay/channel/palm"
	"github.com/QuantumNous/new-api/relay/channel/tencent"
	"github.com/QuantumNous/new-api/relay/channel/xunfei"
	"github.com/QuantumNous/new-api/relay/channel/zhipu"
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
			if !strings.Contains(err.Error(), "not implemented") {
				t.Fatalf("expected not implemented error, got %q", err.Error())
			}
		})
	}
}
