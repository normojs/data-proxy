package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSetupProviderMetadataContextCurrentMappings(t *testing.T) {
	tests := []struct {
		name           string
		channelType    int
		other          string
		wantKey        string
		wantValue      string
		wantEmptyKey   string
		wantEmptyValue string
	}{
		{
			name:        "azure api version",
			channelType: constant.ChannelTypeAzure,
			other:       "2024-10-21",
			wantKey:     "api_version",
			wantValue:   "2024-10-21",
		},
		{
			name:        "xunfei api version",
			channelType: constant.ChannelTypeXunfei,
			other:       "v3.5",
			wantKey:     "api_version",
			wantValue:   "v3.5",
		},
		{
			name:        "gemini api version",
			channelType: constant.ChannelTypeGemini,
			other:       "v1beta",
			wantKey:     "api_version",
			wantValue:   "v1beta",
		},
		{
			name:        "cloudflare api version",
			channelType: constant.ChannelCloudflare,
			other:       "accounts/example/ai",
			wantKey:     "api_version",
			wantValue:   "accounts/example/ai",
		},
		{
			name:        "mokaai api version",
			channelType: constant.ChannelTypeMokaAI,
			other:       "2024-01-01",
			wantKey:     "api_version",
			wantValue:   "2024-01-01",
		},
		{
			name:         "vertex region",
			channelType:  constant.ChannelTypeVertexAi,
			other:        "us-central1",
			wantKey:      "region",
			wantValue:    "us-central1",
			wantEmptyKey: "api_version",
		},
		{
			name:         "ali plugin",
			channelType:  constant.ChannelTypeAli,
			other:        "plugin-123",
			wantKey:      "plugin",
			wantValue:    "plugin-123",
			wantEmptyKey: "api_version",
		},
		{
			name:         "coze bot id",
			channelType:  constant.ChannelTypeCoze,
			other:        "bot_123",
			wantKey:      "bot_id",
			wantValue:    "bot_123",
			wantEmptyKey: "api_version",
		},
		{
			name:        "openai does not set provider metadata",
			channelType: constant.ChannelTypeOpenAI,
			other:       "ignored",
			wantKey:     "api_version",
			wantValue:   "",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			c := newDistributorTestContext()
			setupProviderMetadataContext(c, &model.Channel{
				Type:  tt.channelType,
				Other: tt.other,
			})

			require.Equal(t, tt.wantValue, c.GetString(tt.wantKey))
			if tt.wantEmptyKey != "" {
				require.Equal(t, tt.wantEmptyValue, c.GetString(tt.wantEmptyKey))
			}
		})
	}
}

func TestSetupContextForSelectedChannelAppliesProviderMetadata(t *testing.T) {
	c := newDistributorTestContext()
	channel := &model.Channel{
		Id:    1001,
		Name:  "azure-test",
		Type:  constant.ChannelTypeAzure,
		Key:   "sk-test",
		Other: "2024-10-21",
	}

	err := SetupContextForSelectedChannel(c, channel, "gpt-4o")

	require.Nil(t, err)
	require.Equal(t, "2024-10-21", c.GetString("api_version"))
	require.Equal(t, "gpt-4o", c.GetString("original_model"))
}

func newDistributorTestContext() *gin.Context {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	return c
}
