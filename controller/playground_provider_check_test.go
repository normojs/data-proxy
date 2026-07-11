package controller

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizePlaygroundProviderBaseURL(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "host only",
			raw:  "api.example.com",
			want: "https://api.example.com/v1/chat/completions",
		},
		{
			name: "root URL",
			raw:  "https://api.example.com",
			want: "https://api.example.com/v1/chat/completions",
		},
		{
			name: "v1 base",
			raw:  "https://api.example.com/v1",
			want: "https://api.example.com/v1/chat/completions",
		},
		{
			name: "custom path v1 base",
			raw:  "https://gateway.example.com/openai/v1/",
			want: "https://gateway.example.com/openai/v1/chat/completions",
		},
		{
			name: "full endpoint",
			raw:  "https://api.example.com/v1/chat/completions",
			want: "https://api.example.com/v1/chat/completions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizePlaygroundProviderBaseURL(tt.raw)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestNormalizePlaygroundProviderBaseURLRejectsInvalidInput(t *testing.T) {
	_, err := normalizePlaygroundProviderBaseURL("ftp://api.example.com")
	require.Error(t, err)

	_, err = normalizePlaygroundProviderBaseURL("")
	require.Error(t, err)
}

func TestNormalizePlaygroundProviderBearerKey(t *testing.T) {
	require.Equal(t, "Bearer sk-test", normalizePlaygroundProviderBearerKey("sk-test"))
	require.Equal(t, "Bearer sk-test", normalizePlaygroundProviderBearerKey("Bearer sk-test"))
}

func TestRedactPlaygroundProviderCheckSecret(t *testing.T) {
	redacted := redactPlaygroundProviderCheckSecret("Authorization: Bearer sk-test", "sk-test")
	require.NotContains(t, redacted, "sk-test")
	require.True(t, strings.Contains(redacted, "[redacted]"))
}
