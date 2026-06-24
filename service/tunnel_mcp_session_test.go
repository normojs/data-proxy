package service

import (
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestEnsureTunnelMCPSessionCreatesAndReusesSession(t *testing.T) {
	db := setupTunnelTestDB(t)
	restore := setTunnelMCPSessionStoreForTest(newTunnelMCPGatewaySessionStore(time.Hour))
	defer restore()
	app := seedTunnelMCPApp(t, model.TunnelApp{})
	seedTunnelMCPConnection(t, app, "tc_session_key")

	session, sessionId, err := EnsureTunnelMCPSession(100, app.PublicSlug, "tc_session_key", "", TunnelMCPSessionContext{
		ClientVersion: "codex@1.0.0",
		ClientIP:      "127.0.0.1",
		UserAgent:     "codex-test",
	})
	require.NoError(t, err)
	require.NotEmpty(t, sessionId)
	require.Equal(t, app.Id, session.AppId)

	var persisted model.TunnelSession
	require.NoError(t, db.First(&persisted, "session_id = ?", sessionId).Error)
	require.Equal(t, app.Id, persisted.AppId)
	require.Equal(t, session.ConnectionId, persisted.ConnectionId)
	require.Equal(t, session.KeyPrefix, persisted.KeyPrefix)
	require.Equal(t, model.TunnelSessionStatusOnline, persisted.Status)
	require.Equal(t, "codex@1.0.0", persisted.ClientVersion)
	require.Equal(t, "127.0.0.1", persisted.ClientIp)
	require.Equal(t, "codex-test", persisted.UserAgent)

	reused, reusedId, err := EnsureTunnelMCPSession(100, app.PublicSlug, "tc_session_key", sessionId, TunnelMCPSessionContext{})
	require.NoError(t, err)
	require.Equal(t, sessionId, reusedId)
	require.Equal(t, session.ConnectionId, reused.ConnectionId)
}

func TestEnsureTunnelMCPSessionRehydratesPersistedSession(t *testing.T) {
	db := setupTunnelTestDB(t)
	restore := setTunnelMCPSessionStoreForTest(newTunnelMCPGatewaySessionStore(time.Hour))
	defer restore()
	app := seedTunnelMCPApp(t, model.TunnelApp{})
	connection := seedTunnelMCPConnection(t, app, "tc_rehydrate_key")

	created, sessionId, err := EnsureTunnelMCPSession(100, app.PublicSlug, "tc_rehydrate_key", "", TunnelMCPSessionContext{})
	require.NoError(t, err)
	require.NotEmpty(t, sessionId)

	restoreEmptyStore := setTunnelMCPSessionStoreForTest(newTunnelMCPGatewaySessionStore(time.Hour))
	defer restoreEmptyStore()

	rehydrated, rehydratedId, err := EnsureTunnelMCPSession(100, app.PublicSlug, "tc_rehydrate_key", sessionId, TunnelMCPSessionContext{})
	require.NoError(t, err)
	require.Equal(t, sessionId, rehydratedId)
	require.Equal(t, created.AppId, rehydrated.AppId)
	require.Equal(t, connection.Id, rehydrated.ConnectionId)
	require.Equal(t, connection.KeyPrefix, rehydrated.KeyPrefix)

	var persisted model.TunnelSession
	require.NoError(t, db.First(&persisted, "session_id = ?", sessionId).Error)
	require.Equal(t, model.TunnelSessionStatusOnline, persisted.Status)
	require.NotZero(t, persisted.LastSeenAt)
}

func TestEnsureTunnelMCPSessionRejectsOfflinePersistedSession(t *testing.T) {
	_ = setupTunnelTestDB(t)
	restore := setTunnelMCPSessionStoreForTest(newTunnelMCPGatewaySessionStore(time.Hour))
	defer restore()
	app := seedTunnelMCPApp(t, model.TunnelApp{})
	seedTunnelMCPConnection(t, app, "tc_offline_key")

	_, sessionId, err := EnsureTunnelMCPSession(100, app.PublicSlug, "tc_offline_key", "", TunnelMCPSessionContext{})
	require.NoError(t, err)
	require.NoError(t, model.CloseTunnelSession(sessionId, "test_offline"))

	restoreEmptyStore := setTunnelMCPSessionStoreForTest(newTunnelMCPGatewaySessionStore(time.Hour))
	defer restoreEmptyStore()

	_, _, err = EnsureTunnelMCPSession(100, app.PublicSlug, "tc_offline_key", sessionId, TunnelMCPSessionContext{})
	require.ErrorIs(t, err, ErrTunnelMCPSessionNotFound)
}

func TestTunnelMCPSSESessionDispatch(t *testing.T) {
	_ = setupTunnelTestDB(t)
	restore := setTunnelMCPSessionStoreForTest(newTunnelMCPGatewaySessionStore(time.Hour))
	defer restore()
	app := seedTunnelMCPApp(t, model.TunnelApp{})
	seedTunnelMCPConnection(t, app, "tc_sse_key")

	events, _, sessionId, err := BindTunnelMCPSSESession(100, app.PublicSlug, "tc_sse_key", "", TunnelMCPSessionContext{})
	require.NoError(t, err)
	require.NotEmpty(t, sessionId)
	ok := SendTunnelMCPSSE(sessionId, testTunnelMCPResponse("ping-1"))
	require.True(t, ok)

	select {
	case body := <-events:
		require.Contains(t, string(body), `"id":"ping-1"`)
	case <-time.After(time.Second):
		t.Fatal("expected SSE dispatch")
	}
	UnbindTunnelMCPSSESession(sessionId, events)
	require.False(t, SendTunnelMCPSSE(sessionId, testTunnelMCPResponse("ping-2")))
}

func TestTunnelMCPSSESessionDispatchAcrossStoresThroughBus(t *testing.T) {
	_ = setupTunnelTestDB(t)
	bus := newMemoryTunnelMCPSSEBus()
	restoreBus := setTunnelMCPSSEBusForTest(bus)
	defer restoreBus()
	nodeA := newTunnelMCPGatewaySessionStore(time.Hour)
	restoreStore := setTunnelMCPSessionStoreForTest(nodeA)
	defer restoreStore()
	app := seedTunnelMCPApp(t, model.TunnelApp{})
	seedTunnelMCPConnection(t, app, "tc_sse_bus_key")

	events, _, sessionId, err := BindTunnelMCPSSESession(100, app.PublicSlug, "tc_sse_bus_key", "", TunnelMCPSessionContext{})
	require.NoError(t, err)
	require.NotEmpty(t, sessionId)

	restoreNodeA := setTunnelMCPSessionStoreForTest(newTunnelMCPGatewaySessionStore(time.Hour))
	ok := SendTunnelMCPSSE(sessionId, testTunnelMCPResponse("bus-ping-1"))
	require.True(t, ok)
	restoreNodeA()

	select {
	case body := <-events:
		require.Contains(t, string(body), `"id":"bus-ping-1"`)
	case <-time.After(time.Second):
		t.Fatal("expected SSE dispatch through bus")
	}

	UnbindTunnelMCPSSESession(sessionId, events)
	restoreNodeB := setTunnelMCPSessionStoreForTest(newTunnelMCPGatewaySessionStore(time.Hour))
	require.False(t, SendTunnelMCPSSE(sessionId, testTunnelMCPResponse("bus-ping-2")))
	restoreNodeB()
}

func TestCloseTunnelMCPSessionDeletesAndClosesSSE(t *testing.T) {
	db := setupTunnelTestDB(t)
	restore := setTunnelMCPSessionStoreForTest(newTunnelMCPGatewaySessionStore(time.Hour))
	defer restore()
	app := seedTunnelMCPApp(t, model.TunnelApp{})
	seedTunnelMCPConnection(t, app, "tc_close_key")

	events, _, sessionId, err := BindTunnelMCPSSESession(100, app.PublicSlug, "tc_close_key", "", TunnelMCPSessionContext{})
	require.NoError(t, err)
	require.NotEmpty(t, sessionId)

	require.NoError(t, CloseTunnelMCPSession(100, app.PublicSlug, "tc_close_key", sessionId))
	_, ok := <-events
	require.False(t, ok)
	require.False(t, SendTunnelMCPSSE(sessionId, testTunnelMCPResponse("after-close")))

	var persisted model.TunnelSession
	require.NoError(t, db.First(&persisted, "session_id = ?", sessionId).Error)
	require.Equal(t, model.TunnelSessionStatusOffline, persisted.Status)
	require.Equal(t, "client_closed_session", persisted.CloseReason)
	require.NotZero(t, persisted.DisconnectedAt)

	_, _, err = EnsureTunnelMCPSession(100, app.PublicSlug, "tc_close_key", sessionId, TunnelMCPSessionContext{})
	require.ErrorIs(t, err, ErrTunnelMCPSessionNotFound)
	err = CloseTunnelMCPSession(100, app.PublicSlug, "tc_close_key", sessionId)
	require.ErrorIs(t, err, ErrTunnelMCPSessionNotFound)
}

func TestCloseTunnelMCPSessionClosesPersistedSessionAfterStoreRestart(t *testing.T) {
	db := setupTunnelTestDB(t)
	restore := setTunnelMCPSessionStoreForTest(newTunnelMCPGatewaySessionStore(time.Hour))
	defer restore()
	app := seedTunnelMCPApp(t, model.TunnelApp{})
	seedTunnelMCPConnection(t, app, "tc_restart_close_key")

	_, sessionId, err := EnsureTunnelMCPSession(100, app.PublicSlug, "tc_restart_close_key", "", TunnelMCPSessionContext{})
	require.NoError(t, err)

	restoreEmptyStore := setTunnelMCPSessionStoreForTest(newTunnelMCPGatewaySessionStore(time.Hour))
	defer restoreEmptyStore()

	require.NoError(t, CloseTunnelMCPSession(100, app.PublicSlug, "tc_restart_close_key", sessionId))

	var persisted model.TunnelSession
	require.NoError(t, db.First(&persisted, "session_id = ?", sessionId).Error)
	require.Equal(t, model.TunnelSessionStatusOffline, persisted.Status)
	require.Equal(t, "client_closed_session", persisted.CloseReason)
}

func testTunnelMCPResponse(id string) dto.MCPResponse {
	return dto.MCPResponse{
		JSONRPC: dto.MCPJSONRPCVersion,
		ID:      []byte(`"` + id + `"`),
		Result:  map[string]any{},
	}
}

type memoryTunnelMCPSSEBus struct {
	mu          sync.Mutex
	nextId      int
	subscribers map[string]map[int]func([]byte)
}

func newMemoryTunnelMCPSSEBus() *memoryTunnelMCPSSEBus {
	return &memoryTunnelMCPSSEBus{
		subscribers: map[string]map[int]func([]byte){},
	}
}

func (b *memoryTunnelMCPSSEBus) Publish(sessionId string, body []byte) bool {
	b.mu.Lock()
	subscribers := make([]func([]byte), 0, len(b.subscribers[sessionId]))
	for _, subscriber := range b.subscribers[sessionId] {
		subscribers = append(subscribers, subscriber)
	}
	b.mu.Unlock()
	if len(subscribers) == 0 {
		return false
	}
	for _, subscriber := range subscribers {
		subscriber(append([]byte(nil), body...))
	}
	return true
}

func (b *memoryTunnelMCPSSEBus) Subscribe(sessionId string, handler func([]byte)) (func(), error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.nextId++
	id := b.nextId
	if b.subscribers[sessionId] == nil {
		b.subscribers[sessionId] = map[int]func([]byte){}
	}
	b.subscribers[sessionId][id] = handler
	var once sync.Once
	return func() {
		once.Do(func() {
			b.mu.Lock()
			defer b.mu.Unlock()
			delete(b.subscribers[sessionId], id)
			if len(b.subscribers[sessionId]) == 0 {
				delete(b.subscribers, sessionId)
			}
		})
	}, nil
}
