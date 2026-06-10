package secretref

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeEnvSecretReference(t *testing.T) {
	normalized, err := Normalize("  ENV: MCP_PROXY_TOKEN  ")
	require.NoError(t, err)
	require.Equal(t, "env:MCP_PROXY_TOKEN", normalized)
}

func TestRejectsMalformedSecretReferences(t *testing.T) {
	for _, raw := range []string{
		"",
		"raw:test-token",
		"env:",
		"env:1TOKEN",
		"env:MCP-PROXY-TOKEN",
		"env:MCP PROXY TOKEN",
		"env:MCP_PROXY_TOKEN:extra",
	} {
		t.Run(raw, func(t *testing.T) {
			_, err := Normalize(raw)
			require.Error(t, err)
			require.Contains(t, err.Error(), "env:NAME")
		})
	}
}

func TestResolveEnvSecretReferenceDoesNotLeakReferenceOnMissingEnv(t *testing.T) {
	value, err := ResolveEnv("env:MCP_SECRET_REF_TEST_TOKEN", "mcp proxy auth")
	require.Error(t, err)
	require.Empty(t, value)
	require.NotContains(t, err.Error(), "MCP_SECRET_REF_TEST_TOKEN")
	require.NotContains(t, err.Error(), "env:")

	t.Setenv("MCP_SECRET_REF_TEST_TOKEN", "test-token")
	value, err = ResolveEnv("env:MCP_SECRET_REF_TEST_TOKEN", "mcp proxy auth")
	require.NoError(t, err)
	require.Equal(t, "test-token", value)
}
