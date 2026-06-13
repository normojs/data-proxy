package service

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/setting"
	"github.com/stretchr/testify/require"
)

func TestCheckSensitiveMessagesScansTextAndImageURLFields(t *testing.T) {
	resetSensitiveWordsForTest(t, []string{"blocked_term"})

	t.Run("text content", func(t *testing.T) {
		words, err := CheckSensitiveMessages([]dto.Message{{
			Role:    "user",
			Content: "hello blocked_term",
		}})
		require.Error(t, err)
		require.Contains(t, words, "blocked_term")
	})

	t.Run("image url string", func(t *testing.T) {
		words, err := CheckSensitiveMessages([]dto.Message{{
			Role: "user",
			Content: []any{
				map[string]any{
					"type":      dto.ContentTypeImageURL,
					"image_url": "https://example.com/blocked_term.png",
				},
			},
		}})
		require.Error(t, err)
		require.Contains(t, words, "blocked_term")
	})

	t.Run("image url object", func(t *testing.T) {
		words, err := CheckSensitiveMessages([]dto.Message{{
			Role: "user",
			Content: []any{
				map[string]any{
					"type": dto.ContentTypeImageURL,
					"image_url": map[string]any{
						"url":    "https://example.com/safe.png?label=blocked_term",
						"detail": "high",
					},
				},
			},
		}})
		require.Error(t, err)
		require.Contains(t, words, "blocked_term")
	})

	t.Run("mixed clean content", func(t *testing.T) {
		words, err := CheckSensitiveMessages([]dto.Message{{
			Role: "user",
			Content: []any{
				map[string]any{
					"type": "text",
					"text": "hello",
				},
				map[string]any{
					"type": dto.ContentTypeImageURL,
					"image_url": map[string]any{
						"url": "https://example.com/safe.png",
					},
				},
			},
		}})
		require.NoError(t, err)
		require.Empty(t, words)
	})
}

func TestImageURLSensitiveTextsScansDirectMediaMetadata(t *testing.T) {
	resetSensitiveWordsForTest(t, []string{"blocked_term"})

	words, err := CheckSensitiveMessages([]dto.Message{{
		Role: "user",
		Content: []any{
			dto.MediaContent{
				Type: dto.ContentTypeImageURL,
				ImageUrl: map[string]any{
					"url": "https://example.com/safe.png",
					"metadata": map[string]any{
						"alt": "blocked_term",
					},
				},
			},
		},
	}})

	require.Error(t, err)
	require.Contains(t, words, "blocked_term")
}

func resetSensitiveWordsForTest(t *testing.T, words []string) {
	t.Helper()
	original := append([]string(nil), setting.SensitiveWords...)
	setting.SensitiveWords = words
	t.Cleanup(func() {
		setting.SensitiveWords = original
	})
}
