package service

import (
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

const tunnelMCPGatewaySessionTTL = 2 * time.Hour

var defaultTunnelMCPGatewaySessions = newTunnelMCPGatewaySessionStore(tunnelMCPGatewaySessionTTL)

type TunnelMCPSession struct {
	SessionId    string
	UserId       int
	AppId        int64
	ConnectionId int64
	Slug         string
	KeyPrefix    string
	CreatedAt    int64
	LastSeenAt   int64
}

type TunnelMCPSessionContext struct {
	ClientVersion string
	ClientIP      string
	UserAgent     string
}

type tunnelMCPGatewaySession struct {
	TunnelMCPSession
	sse       chan []byte
	sseCancel func()
}

type tunnelMCPGatewaySessionStore struct {
	mu       sync.Mutex
	ttl      time.Duration
	sessions map[string]*tunnelMCPGatewaySession
}

func newTunnelMCPGatewaySessionStore(ttl time.Duration) *tunnelMCPGatewaySessionStore {
	if ttl <= 0 {
		ttl = tunnelMCPGatewaySessionTTL
	}
	return &tunnelMCPGatewaySessionStore{
		ttl:      ttl,
		sessions: map[string]*tunnelMCPGatewaySession{},
	}
}

func setTunnelMCPSessionStoreForTest(store *tunnelMCPGatewaySessionStore) func() {
	previous := defaultTunnelMCPGatewaySessions
	if store == nil {
		store = newTunnelMCPGatewaySessionStore(tunnelMCPGatewaySessionTTL)
	}
	defaultTunnelMCPGatewaySessions = store
	return func() {
		defaultTunnelMCPGatewaySessions = previous
	}
}

func EnsureTunnelMCPSession(userId int, slug string, connectionKey string, sessionId string, context TunnelMCPSessionContext) (TunnelMCPSession, string, error) {
	app, connection, err := getAuthorizedTunnelMCPApp(slug, connectionKey, userId)
	if err != nil {
		return TunnelMCPSession{}, "", err
	}
	now := time.Now().Unix()
	sessionId = strings.TrimSpace(sessionId)
	store := defaultTunnelMCPGatewaySessions
	store.mu.Lock()
	store.pruneLocked(now)
	if sessionId != "" {
		if existing := store.sessions[sessionId]; existing != nil {
			if existing.UserId != userId || existing.AppId != app.Id || existing.ConnectionId != connection.Id {
				store.mu.Unlock()
				return TunnelMCPSession{}, "", errors.New("tunnel mcp session does not belong to this connection")
			}
			existing.LastSeenAt = now
			session := existing.TunnelMCPSession
			store.mu.Unlock()
			_ = model.TouchTunnelSession(sessionId)
			return session, sessionId, nil
		}
		store.mu.Unlock()
		rehydrated, err := rehydrateTunnelMCPSession(userId, *app, *connection, sessionId)
		if err != nil {
			return TunnelMCPSession{}, "", err
		}
		rehydrated.LastSeenAt = now
		store.mu.Lock()
		if existing := store.sessions[sessionId]; existing != nil {
			if existing.UserId != userId || existing.AppId != app.Id || existing.ConnectionId != connection.Id {
				store.mu.Unlock()
				return TunnelMCPSession{}, "", errors.New("tunnel mcp session does not belong to this connection")
			}
			existing.LastSeenAt = now
			session := existing.TunnelMCPSession
			store.mu.Unlock()
			_ = model.TouchTunnelSession(sessionId)
			return session, sessionId, nil
		}
		created := &tunnelMCPGatewaySession{TunnelMCPSession: rehydrated}
		store.sessions[sessionId] = created
		session := created.TunnelMCPSession
		store.mu.Unlock()
		_ = model.TouchTunnelSession(sessionId)
		return session, sessionId, nil
	}
	sessionId = "tmcp_" + common.GetRandomString(32)
	created := &tunnelMCPGatewaySession{
		TunnelMCPSession: TunnelMCPSession{
			SessionId:    sessionId,
			UserId:       userId,
			AppId:        app.Id,
			ConnectionId: connection.Id,
			Slug:         app.PublicSlug,
			KeyPrefix:    connection.KeyPrefix,
			CreatedAt:    now,
			LastSeenAt:   now,
		},
	}
	store.sessions[sessionId] = created
	session := created.TunnelMCPSession
	store.mu.Unlock()
	_ = model.CreateTunnelSession(&model.TunnelSession{
		AppId:          app.Id,
		UserId:         app.UserId,
		ConnectionId:   connection.Id,
		ConnectionName: connection.Name,
		KeyPrefix:      connection.KeyPrefix,
		SessionId:      sessionId,
		BridgeClientId: app.BridgeClientId,
		Status:         model.TunnelSessionStatusOnline,
		ClientVersion:  limitTunnelString(context.ClientVersion, 64),
		ClientIp:       limitTunnelString(context.ClientIP, 64),
		UserAgent:      limitTunnelString(context.UserAgent, 255),
		ConnectedAt:    now,
		LastSeenAt:     now,
	})
	return session, sessionId, nil
}

func BindTunnelMCPSSESession(userId int, slug string, connectionKey string, sessionId string, context TunnelMCPSessionContext) (<-chan []byte, TunnelMCPSession, string, error) {
	_, nextSessionId, err := EnsureTunnelMCPSession(userId, slug, connectionKey, sessionId, context)
	if err != nil {
		return nil, TunnelMCPSession{}, "", err
	}
	store := defaultTunnelMCPGatewaySessions
	store.mu.Lock()
	existing := store.sessions[nextSessionId]
	if existing == nil {
		store.mu.Unlock()
		return nil, TunnelMCPSession{}, "", ErrTunnelMCPSessionNotFound
	}
	previousCancel := existing.sseCancel
	if existing.sse != nil {
		close(existing.sse)
	}
	events := make(chan []byte, 32)
	existing.sse = events
	existing.sseCancel = nil
	existing.LastSeenAt = time.Now().Unix()
	session := existing.TunnelMCPSession
	store.mu.Unlock()
	cancelTunnelMCPSSESubscription(previousCancel)

	cancel, err := defaultTunnelMCPSSEBus.Subscribe(nextSessionId, func(body []byte) {
		sendTunnelMCPSSEToLocalStore(store, nextSessionId, events, body)
	})
	if err != nil {
		common.SysLog("tunnel mcp sse redis subscribe failed: " + err.Error())
	}
	store.mu.Lock()
	existing = store.sessions[nextSessionId]
	var cancelAfterUnlock func()
	if existing != nil && existing.sse == events {
		existing.sseCancel = cancel
	} else {
		cancelAfterUnlock = cancel
	}
	store.mu.Unlock()
	cancelTunnelMCPSSESubscription(cancelAfterUnlock)
	return events, session, nextSessionId, nil
}

func UnbindTunnelMCPSSESession(sessionId string, ch <-chan []byte) {
	sessionId = strings.TrimSpace(sessionId)
	if sessionId == "" {
		return
	}
	store := defaultTunnelMCPGatewaySessions
	store.mu.Lock()
	existing := store.sessions[sessionId]
	if existing == nil || existing.sse != ch {
		store.mu.Unlock()
		return
	}
	cancel := existing.sseCancel
	existing.sseCancel = nil
	close(existing.sse)
	existing.sse = nil
	store.mu.Unlock()
	cancelTunnelMCPSSESubscription(cancel)
}

func CloseTunnelMCPSession(userId int, slug string, connectionKey string, sessionId string) error {
	app, connection, err := getAuthorizedTunnelMCPApp(slug, connectionKey, userId)
	if err != nil {
		return err
	}
	sessionId = strings.TrimSpace(sessionId)
	if sessionId == "" {
		return ErrTunnelMCPSessionNotFound
	}
	now := time.Now().Unix()
	store := defaultTunnelMCPGatewaySessions
	store.mu.Lock()
	store.pruneLocked(now)
	existing := store.sessions[sessionId]
	if existing == nil {
		store.mu.Unlock()
		persisted, err := loadPersistedTunnelMCPSession(userId, *app, *connection, sessionId)
		if err != nil {
			return err
		}
		_ = persisted
		_ = model.CloseTunnelSession(sessionId, "client_closed_session")
		return nil
	}
	if existing.UserId != userId || existing.AppId != app.Id || existing.ConnectionId != connection.Id {
		store.mu.Unlock()
		return errors.New("tunnel mcp session does not belong to this connection")
	}
	cancel := closeTunnelMCPSSELocked(existing)
	delete(store.sessions, sessionId)
	store.mu.Unlock()
	cancelTunnelMCPSSESubscription(cancel)
	_ = model.CloseTunnelSession(sessionId, "client_closed_session")
	return nil
}

func SendTunnelMCPSSE(sessionId string, response dto.MCPResponse) bool {
	sessionId = strings.TrimSpace(sessionId)
	if sessionId == "" {
		return false
	}
	body, err := json.Marshal(response)
	if err != nil {
		return false
	}
	store := defaultTunnelMCPGatewaySessions
	store.mu.Lock()
	existing := store.sessions[sessionId]
	if existing != nil {
		existing.LastSeenAt = time.Now().Unix()
	}
	if existing != nil && existing.sse != nil {
		select {
		case existing.sse <- body:
			store.mu.Unlock()
			return true
		default:
			store.mu.Unlock()
			return false
		}
	}
	store.mu.Unlock()
	return defaultTunnelMCPSSEBus.Publish(sessionId, body)
}

func (s *tunnelMCPGatewaySessionStore) pruneLocked(now int64) {
	if s == nil || s.ttl <= 0 {
		return
	}
	threshold := now - int64(s.ttl.Seconds())
	for id, session := range s.sessions {
		if session == nil || session.LastSeenAt <= threshold {
			if session != nil {
				cancel := closeTunnelMCPSSELocked(session)
				if cancel != nil {
					go cancelTunnelMCPSSESubscription(cancel)
				}
			}
			delete(s.sessions, id)
			_ = model.CloseTunnelSession(id, "session_ttl_expired")
		}
	}
}

func sendTunnelMCPSSEToLocalStore(store *tunnelMCPGatewaySessionStore, sessionId string, ch chan []byte, body []byte) bool {
	if store == nil || ch == nil || len(body) == 0 {
		return false
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	existing := store.sessions[sessionId]
	if existing == nil || existing.sse != ch {
		return false
	}
	existing.LastSeenAt = time.Now().Unix()
	select {
	case existing.sse <- body:
		return true
	default:
		return false
	}
}

func closeTunnelMCPSSELocked(session *tunnelMCPGatewaySession) func() {
	if session == nil {
		return nil
	}
	cancel := session.sseCancel
	session.sseCancel = nil
	if session.sse != nil {
		close(session.sse)
		session.sse = nil
	}
	return cancel
}

func cancelTunnelMCPSSESubscription(cancel func()) {
	if cancel != nil {
		cancel()
	}
}

func rehydrateTunnelMCPSession(userId int, app model.TunnelApp, connection model.TunnelConnection, sessionId string) (TunnelMCPSession, error) {
	persisted, err := loadPersistedTunnelMCPSession(userId, app, connection, sessionId)
	if err != nil {
		return TunnelMCPSession{}, err
	}
	createdAt := persisted.ConnectedAt
	if createdAt == 0 {
		createdAt = persisted.CreatedAt
	}
	lastSeenAt := persisted.LastSeenAt
	if lastSeenAt == 0 {
		lastSeenAt = createdAt
	}
	return TunnelMCPSession{
		SessionId:    persisted.SessionId,
		UserId:       persisted.UserId,
		AppId:        persisted.AppId,
		ConnectionId: persisted.ConnectionId,
		Slug:         app.PublicSlug,
		KeyPrefix:    persisted.KeyPrefix,
		CreatedAt:    createdAt,
		LastSeenAt:   lastSeenAt,
	}, nil
}

func loadPersistedTunnelMCPSession(userId int, app model.TunnelApp, connection model.TunnelConnection, sessionId string) (*model.TunnelSession, error) {
	persisted, err := model.GetTunnelSessionBySessionId(sessionId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTunnelMCPSessionNotFound
		}
		return nil, err
	}
	if persisted.UserId != userId || persisted.UserId != app.UserId || persisted.AppId != app.Id || persisted.ConnectionId != connection.Id {
		return nil, errors.New("tunnel mcp session does not belong to this connection")
	}
	if persisted.Status != model.TunnelSessionStatusOnline {
		return nil, ErrTunnelMCPSessionNotFound
	}
	return persisted, nil
}

var ErrTunnelMCPSessionNotFound = errors.New("tunnel mcp session not found")
