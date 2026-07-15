package service

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"

	"github.com/stretchr/testify/assert"
)

func TestComputeModelTokenPackageConsume(t *testing.T) {
	assert.EqualValues(t, 1700, ComputeModelTokenPackageConsume(1000, 500, 200, 1, 1, 1))
	assert.EqualValues(t, 1600, ComputeModelTokenPackageConsume(1000, 500, 200, 1, 1, 0.5))
	assert.EqualValues(t, 2200, ComputeModelTokenPackageConsume(1000, 500, 200, 1, 2, 1))
	assert.EqualValues(t, 0, ComputeModelTokenPackageConsume(0, 0, 0, 1, 1, 1))
	// ceil
	assert.EqualValues(t, 1, ComputeModelTokenPackageConsume(1, 0, 0, 0.1, 1, 1))
}

func TestExtractUsageTokenParts(t *testing.T) {
	usage := &dto.Usage{
		PromptTokens:     100,
		CompletionTokens: 40,
		PromptTokensDetails: dto.InputTokenDetails{
			CachedTokens:         20,
			CachedCreationTokens: 5,
		},
	}
	p, c, cache := ExtractUsageTokenParts(usage)
	assert.Equal(t, 100, p)
	assert.Equal(t, 40, c)
	assert.Equal(t, 25, cache)
}

func TestResolveModelTokenPackageRatioDefaults(t *testing.T) {
	assert.Equal(t, 1.0, model.ResolveModelTokenPackageRatio(-1))
	assert.Equal(t, 0.0, model.NormalizeModelTokenPackageRatio(0))
	assert.Equal(t, 10.0, model.NormalizeModelTokenPackageRatio(99))
}
