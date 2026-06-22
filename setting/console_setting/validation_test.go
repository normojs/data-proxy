package console_setting

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateDownloadsAcceptsValidConfig(t *testing.T) {
	raw := `[{"id":1,"name":"Codex","description":"Terminal coding agent","url":"https://github.com/openai/codex","icon":"terminal","customIconUrl":"https://example.com/codex.png","openInNewTab":true,"enabled":true}]`

	require.NoError(t, ValidateConsoleSettings(raw, "Downloads"))
}

func TestValidateDownloadsRejectsInvalidIcon(t *testing.T) {
	raw := `[{"id":1,"name":"Codex","url":"https://github.com/openai/codex","icon":"sparkles","openInNewTab":true,"enabled":true}]`

	require.ErrorContains(t, ValidateConsoleSettings(raw, "Downloads"), "图标值不合法")
}

func TestValidateDownloadsRejectsInvalidCustomIconURL(t *testing.T) {
	raw := `[{"id":1,"name":"Codex","url":"https://github.com/openai/codex","icon":"terminal","customIconUrl":"javascript:alert(1)","openInNewTab":true,"enabled":true}]`

	require.ErrorContains(t, ValidateConsoleSettings(raw, "Downloads"), "URL格式不正确")
}

func TestGetDownloadsFiltersDisabledItems(t *testing.T) {
	original := GetConsoleSetting().Downloads
	t.Cleanup(func() {
		GetConsoleSetting().Downloads = original
	})

	GetConsoleSetting().Downloads = `[{"id":1,"name":"Codex","url":"https://github.com/openai/codex","icon":"terminal","enabled":true},{"id":2,"name":"Draft","url":"https://example.com/draft","icon":"download","enabled":false}]`

	require.Len(t, GetDownloads(), 1)
	require.Equal(t, "Codex", GetDownloads()[0]["name"])
}
