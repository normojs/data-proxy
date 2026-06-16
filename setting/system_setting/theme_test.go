package system_setting

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
)

func TestUpdateAndSyncThemeNormalizesLegacyClassic(t *testing.T) {
	themeSettings.Frontend = "classic"
	UpdateAndSyncTheme()

	if got := themeSettings.Frontend; got != "default" {
		t.Fatalf("expected stored frontend theme to normalize to default, got %q", got)
	}
	if got := common.GetTheme(); got != "default" {
		t.Fatalf("expected common theme to normalize to default, got %q", got)
	}
}
