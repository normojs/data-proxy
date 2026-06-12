package controller

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDecodeBridgeToolResultFromMapPayload(t *testing.T) {
	result, err := decodeBridgeToolResult(map[string]any{
		"content": []any{
			map[string]any{
				"type": "text",
				"text": "ok",
			},
		},
		"metadata": map[string]any{
			"source": "bridge",
		},
		"summary":     "done",
		"duration_ms": 12,
		"result_size": 42,
	})

	require.NoError(t, err)
	require.Equal(t, "done", result.Summary)
	require.Equal(t, 12, result.DurationMS)
	require.Equal(t, 42, result.ResultSize)
	require.Len(t, result.Content, 1)
	require.Equal(t, "text", result.Content[0].Type)
	require.Equal(t, "ok", result.Content[0].Text)
	require.Equal(t, "bridge", result.Metadata["source"])
}

func TestDecodeBridgeToolErrorFromMapPayload(t *testing.T) {
	result, err := decodeBridgeToolError(map[string]any{
		"code":    "REMOTE_PERMISSION_DENIED",
		"message": "write disabled",
	})

	require.NoError(t, err)
	require.Equal(t, "REMOTE_PERMISSION_DENIED", result.Code)
	require.Equal(t, "write disabled", result.Message)
}
