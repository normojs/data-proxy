package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

func TestRealtimeWebSocketSubprotocolNegotiation(t *testing.T) {
	tests := []struct {
		name      string
		requested []string
		want      string
	}{
		{
			name:      "realtime protocol",
			requested: []string{"realtime"},
			want:      "realtime",
		},
		{
			name:      "openai beta realtime protocol",
			requested: []string{"openai-beta.realtime-v1"},
			want:      "openai-beta.realtime-v1",
		},
		{
			name:      "client protocol header with credential token",
			requested: []string{"realtime", "openai-insecure-api-key.sk-test", "openai-beta.realtime-v1"},
			want:      "realtime",
		},
		{
			name:      "credential token alone is not echoed",
			requested: []string{"openai-insecure-api-key.sk-test"},
			want:      "",
		},
		{
			name:      "unsupported protocol is not selected",
			requested: []string{"unsupported.realtime-v0"},
			want:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, dialRealtimeSubprotocol(t, tt.requested))
		})
	}
}

func TestRealtimeWebSocketSubprotocolListDoesNotExposeCredentialPrefix(t *testing.T) {
	for _, protocol := range realtimeWebSocketSubprotocols {
		require.False(t, strings.HasPrefix(protocol, "openai-insecure-api-key."))
	}
}

func dialRealtimeSubprotocol(t *testing.T, requested []string) string {
	t.Helper()

	upgradeErr := make(chan error, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			upgradeErr <- err
			return
		}
		defer conn.Close()
		upgradeErr <- nil
	}))
	t.Cleanup(server.Close)

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	dialer := websocket.Dialer{Subprotocols: requested}
	conn, resp, err := dialer.Dial(wsURL, nil)
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}
	require.NoError(t, err)
	defer conn.Close()

	require.NoError(t, <-upgradeErr)
	return conn.Subprotocol()
}
