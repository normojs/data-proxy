package dto

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGeminiChatGenerationConfigPreservesExplicitZeroValuesCamelCase(t *testing.T) {
	raw := []byte(`{
		"contents":[{"role":"user","parts":[{"text":"hello"}]}],
		"generationConfig":{
			"topP":0,
			"topK":0,
			"maxOutputTokens":0,
			"candidateCount":0,
			"seed":0,
			"responseLogprobs":false
		}
	}`)

	var req GeminiChatRequest
	require.NoError(t, common.Unmarshal(raw, &req))

	encoded, err := common.Marshal(req)
	require.NoError(t, err)

	var out map[string]any
	require.NoError(t, common.Unmarshal(encoded, &out))

	generationConfig, ok := out["generationConfig"].(map[string]any)
	require.True(t, ok)

	assert.Contains(t, generationConfig, "topP")
	assert.Contains(t, generationConfig, "topK")
	assert.Contains(t, generationConfig, "maxOutputTokens")
	assert.Contains(t, generationConfig, "candidateCount")
	assert.Contains(t, generationConfig, "seed")
	assert.Contains(t, generationConfig, "responseLogprobs")

	assert.Equal(t, float64(0), generationConfig["topP"])
	assert.Equal(t, float64(0), generationConfig["topK"])
	assert.Equal(t, float64(0), generationConfig["maxOutputTokens"])
	assert.Equal(t, float64(0), generationConfig["candidateCount"])
	assert.Equal(t, float64(0), generationConfig["seed"])
	assert.Equal(t, false, generationConfig["responseLogprobs"])
}

func TestGeminiChatGenerationConfigPreservesExplicitZeroValuesSnakeCase(t *testing.T) {
	raw := []byte(`{
		"contents":[{"role":"user","parts":[{"text":"hello"}]}],
		"generationConfig":{
			"top_p":0,
			"top_k":0,
			"max_output_tokens":0,
			"candidate_count":0,
			"seed":0,
			"response_logprobs":false
		}
	}`)

	var req GeminiChatRequest
	require.NoError(t, common.Unmarshal(raw, &req))

	encoded, err := common.Marshal(req)
	require.NoError(t, err)

	var out map[string]any
	require.NoError(t, common.Unmarshal(encoded, &out))

	generationConfig, ok := out["generationConfig"].(map[string]any)
	require.True(t, ok)

	assert.Contains(t, generationConfig, "topP")
	assert.Contains(t, generationConfig, "topK")
	assert.Contains(t, generationConfig, "maxOutputTokens")
	assert.Contains(t, generationConfig, "candidateCount")
	assert.Contains(t, generationConfig, "seed")
	assert.Contains(t, generationConfig, "responseLogprobs")

	assert.Equal(t, float64(0), generationConfig["topP"])
	assert.Equal(t, float64(0), generationConfig["topK"])
	assert.Equal(t, float64(0), generationConfig["maxOutputTokens"])
	assert.Equal(t, float64(0), generationConfig["candidateCount"])
	assert.Equal(t, float64(0), generationConfig["seed"])
	assert.Equal(t, false, generationConfig["responseLogprobs"])
}

func TestGeminiThinkingConfigPreservesBudgetAndLevelCamelCase(t *testing.T) {
	raw := []byte(`{
		"contents":[{"role":"user","parts":[{"text":"hello"}]}],
		"generationConfig":{
			"thinkingConfig":{
				"includeThoughts":true,
				"thinkingBudget":1024,
				"thinkingLevel":"high"
			}
		}
	}`)

	var req GeminiChatRequest
	require.NoError(t, common.Unmarshal(raw, &req))
	require.NotNil(t, req.GenerationConfig.ThinkingConfig)
	require.NotNil(t, req.GenerationConfig.ThinkingConfig.ThinkingBudget)
	require.Equal(t, 1024, *req.GenerationConfig.ThinkingConfig.ThinkingBudget)
	require.Equal(t, "high", req.GenerationConfig.ThinkingConfig.ThinkingLevel)
	require.True(t, req.GenerationConfig.ThinkingConfig.IncludeThoughts)

	encoded, err := common.Marshal(req)
	require.NoError(t, err)

	var out map[string]any
	require.NoError(t, common.Unmarshal(encoded, &out))
	generationConfig := out["generationConfig"].(map[string]any)
	thinkingConfig := generationConfig["thinkingConfig"].(map[string]any)
	assert.Equal(t, float64(1024), thinkingConfig["thinkingBudget"])
	assert.Equal(t, "high", thinkingConfig["thinkingLevel"])
	assert.Equal(t, true, thinkingConfig["includeThoughts"])
}

func TestGeminiThinkingConfigPreservesBudgetAndLevelSnakeCase(t *testing.T) {
	raw := []byte(`{
		"contents":[{"role":"user","parts":[{"text":"hello"}]}],
		"generationConfig":{
			"thinking_config":{
				"include_thoughts":true,
				"thinking_budget":2048,
				"thinking_level":"medium"
			}
		}
	}`)

	var req GeminiChatRequest
	require.NoError(t, common.Unmarshal(raw, &req))
	require.NotNil(t, req.GenerationConfig.ThinkingConfig)
	require.NotNil(t, req.GenerationConfig.ThinkingConfig.ThinkingBudget)
	require.Equal(t, 2048, *req.GenerationConfig.ThinkingConfig.ThinkingBudget)
	require.Equal(t, "medium", req.GenerationConfig.ThinkingConfig.ThinkingLevel)
	require.True(t, req.GenerationConfig.ThinkingConfig.IncludeThoughts)
}

func TestGeminiThinkingConfigSnakeCaseOverridesCamelCase(t *testing.T) {
	raw := []byte(`{
		"contents":[{"role":"user","parts":[{"text":"hello"}]}],
		"generationConfig":{
			"thinkingConfig":{
				"includeThoughts":false,
				"include_thoughts":true,
				"thinkingBudget":128,
				"thinking_budget":256,
				"thinkingLevel":"low",
				"thinking_level":"high"
			}
		}
	}`)

	var req GeminiChatRequest
	require.NoError(t, common.Unmarshal(raw, &req))
	require.NotNil(t, req.GenerationConfig.ThinkingConfig)
	require.NotNil(t, req.GenerationConfig.ThinkingConfig.ThinkingBudget)
	require.Equal(t, 256, *req.GenerationConfig.ThinkingConfig.ThinkingBudget)
	require.Equal(t, "high", req.GenerationConfig.ThinkingConfig.ThinkingLevel)
	require.True(t, req.GenerationConfig.ThinkingConfig.IncludeThoughts)
}
