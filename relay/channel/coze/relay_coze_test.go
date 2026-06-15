package coze

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
)

func TestConvertOpenAIMessageToCozeEnterMessage_StringContent(t *testing.T) {
	t.Parallel()

	message, ok := convertOpenAIMessageToCozeEnterMessage(dto.Message{
		Role:    "user",
		Content: "hello coze",
	})
	require.True(t, ok)
	require.Equal(t, "user", message.Role)
	require.Equal(t, "text", message.ContentType)
	require.Equal(t, "hello coze", message.Content)
}

func TestConvertOpenAIMessageToCozeEnterMessage_TextOnlyMediaContent(t *testing.T) {
	t.Parallel()

	message, ok := convertOpenAIMessageToCozeEnterMessage(dto.Message{
		Role: "user",
		Content: []any{
			dto.MediaContent{Type: dto.ContentTypeText, Text: "alpha"},
			dto.MediaContent{Type: dto.ContentTypeText, Text: " beta"},
		},
	})
	require.True(t, ok)
	require.Equal(t, "text", message.ContentType)
	require.Equal(t, "alpha beta", message.Content)
}

func TestConvertOpenAIMessageToCozeEnterMessage_ImageContentUsesObjectString(t *testing.T) {
	t.Parallel()

	message, ok := convertOpenAIMessageToCozeEnterMessage(dto.Message{
		Role: "user",
		Content: []any{
			dto.MediaContent{Type: dto.ContentTypeText, Text: "look"},
			dto.MediaContent{
				Type: dto.ContentTypeImageURL,
				ImageUrl: &dto.MessageImageUrl{
					Url: "https://example.com/cat.png",
				},
			},
		},
	})
	require.True(t, ok)
	require.Equal(t, "object_string", message.ContentType)

	content := requireCozeObjectStringContent(t, message)
	require.Equal(t, []cozeObjectStringContent{
		{Type: "text", Text: "look"},
		{Type: "image", FileURL: "https://example.com/cat.png"},
	}, content)
}

func TestConvertOpenAIMessageToCozeEnterMessage_FileContentUsesObjectString(t *testing.T) {
	t.Parallel()

	message, ok := convertOpenAIMessageToCozeEnterMessage(dto.Message{
		Role: "user",
		Content: []any{
			dto.MediaContent{
				Type: dto.ContentTypeFile,
				File: &dto.MessageFile{FileId: "file_123"},
			},
			dto.MediaContent{
				Type: dto.ContentTypeFile,
				File: &dto.MessageFile{FileData: "https://example.com/report.pdf"},
			},
			dto.MediaContent{
				Type:     dto.ContentTypeVideoUrl,
				VideoUrl: &dto.MessageVideoUrl{Url: "https://example.com/clip.mp4"},
			},
		},
	})
	require.True(t, ok)
	require.Equal(t, "object_string", message.ContentType)

	content := requireCozeObjectStringContent(t, message)
	require.Equal(t, []cozeObjectStringContent{
		{Type: "file", FileID: "file_123"},
		{Type: "file", FileURL: "https://example.com/report.pdf"},
		{Type: "file", FileURL: "https://example.com/clip.mp4"},
	}, content)
}

func TestConvertOpenAIMessageToCozeEnterMessage_UnsupportedContentFallsBackToText(t *testing.T) {
	t.Parallel()

	message, ok := convertOpenAIMessageToCozeEnterMessage(dto.Message{
		Role: "user",
		Content: []any{
			dto.MediaContent{
				Type: dto.ContentTypeInputAudio,
				InputAudio: &dto.MessageInputAudio{
					Data:   "UklGRg==",
					Format: "wav",
				},
			},
		},
	})
	require.True(t, ok)
	require.Equal(t, "text", message.ContentType)
	require.Equal(t, "[unsupported input_audio content omitted]", message.Content)
}

func TestConvertOpenAIMessageToCozeEnterMessage_UnsupportedContentWithMediaKeepsObjectString(t *testing.T) {
	t.Parallel()

	message, ok := convertOpenAIMessageToCozeEnterMessage(dto.Message{
		Role: "user",
		Content: []any{
			dto.MediaContent{
				Type: dto.ContentTypeImageURL,
				ImageUrl: &dto.MessageImageUrl{
					Url: "https://example.com/chart.png",
				},
			},
			dto.MediaContent{
				Type: dto.ContentTypeInputAudio,
				InputAudio: &dto.MessageInputAudio{
					Data:   "UklGRg==",
					Format: "wav",
				},
			},
		},
	})
	require.True(t, ok)
	require.Equal(t, "object_string", message.ContentType)

	content := requireCozeObjectStringContent(t, message)
	require.Equal(t, []cozeObjectStringContent{
		{Type: "image", FileURL: "https://example.com/chart.png"},
		{Type: "text", Text: "[unsupported input_audio content omitted]"},
	}, content)
}

func requireCozeObjectStringContent(t *testing.T, message CozeEnterMessage) []cozeObjectStringContent {
	t.Helper()

	content, ok := message.Content.(string)
	require.True(t, ok)

	var parts []cozeObjectStringContent
	require.NoError(t, common.UnmarshalJsonStr(content, &parts))
	return parts
}
