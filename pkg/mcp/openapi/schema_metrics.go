package openapi

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/QuantumNous/new-api/common"
)

type SchemaMetrics struct {
	OperationCount    int
	SchemaCount       int
	UniqueSchemaCount int
	ReusedSchemaCount int
}

func BuildSchemaMetrics(spec *Spec) SchemaMetrics {
	if spec == nil {
		return SchemaMetrics{}
	}
	metrics := SchemaMetrics{
		OperationCount: len(spec.Operations),
	}
	seen := map[string]bool{}
	for _, operation := range spec.Operations {
		metrics.recordSchema(operation.InputSchema, seen)
		if len(operation.RequestBodySchema) > 0 {
			metrics.recordSchema(operation.RequestBodySchema, seen)
		}
	}
	metrics.UniqueSchemaCount = len(seen)
	if metrics.SchemaCount > metrics.UniqueSchemaCount {
		metrics.ReusedSchemaCount = metrics.SchemaCount - metrics.UniqueSchemaCount
	}
	return metrics
}

func (m *SchemaMetrics) recordSchema(schema map[string]any, seen map[string]bool) {
	if m == nil || len(schema) == 0 {
		return
	}
	m.SchemaCount++
	hash := schemaHash(schema)
	if hash == "" {
		return
	}
	seen[hash] = true
}

func schemaHash(schema map[string]any) string {
	bytes, err := common.Marshal(schema)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(bytes)
	return hex.EncodeToString(sum[:])
}
