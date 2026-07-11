package constant

import "testing"

func TestPath2RelayModePlaygroundResponses(t *testing.T) {
	if got := Path2RelayMode("/pg/responses"); got != RelayModeResponses {
		t.Fatalf("Path2RelayMode(/pg/responses) = %d, want %d", got, RelayModeResponses)
	}
}

func TestPath2RelayModeSubsiteRelayPaths(t *testing.T) {
	tests := []struct {
		path string
		want int
	}{
		{path: "/s/team-a/v1/chat/completions", want: RelayModeChatCompletions},
		{path: "/s/team-a/v1/responses/compact", want: RelayModeResponsesCompact},
		{path: "/s/team-a/v1beta/models/gemini-2.0-flash:generateContent", want: RelayModeGemini},
	}

	for _, tt := range tests {
		if got := Path2RelayMode(tt.path); got != tt.want {
			t.Fatalf("Path2RelayMode(%s) = %d, want %d", tt.path, got, tt.want)
		}
	}
}
