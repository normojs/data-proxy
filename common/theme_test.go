package common

import "testing"

func TestSetThemeNormalizesToDefault(t *testing.T) {
	SetTheme("classic")
	if got := GetTheme(); got != "default" {
		t.Fatalf("expected classic value to normalize to default, got %q", got)
	}

	SetTheme("unexpected")
	if got := GetTheme(); got != "default" {
		t.Fatalf("expected unexpected value to normalize to default, got %q", got)
	}
}

func TestThemeAwarePathAlwaysUsesDefaultRoutes(t *testing.T) {
	SetTheme("classic")

	tests := map[string]string{
		"/console/topup":    "/wallet",
		"/console/log":      "/usage-logs",
		"/console/personal": "/profile",
		"/console/other":    "/console/other",
	}

	for input, want := range tests {
		if got := ThemeAwarePath(input); got != want {
			t.Fatalf("ThemeAwarePath(%q) = %q, want %q", input, got, want)
		}
	}
}
