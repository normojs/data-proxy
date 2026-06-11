package bridge

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/dto"
)

const MessageTypeToolCall = "tool_call"

var (
	ErrClientNotFound     = errors.New("bridge client is not online")
	ErrClientUnavailable  = errors.New("bridge client is unavailable")
	ErrClientDisconnected = errors.New("bridge client disconnected")
	ErrRequestNotFound    = errors.New("bridge request was not found")
)

type Session struct {
	SessionId    string
	ClientId     string
	UserId       int
	TokenId      int
	Name         string
	Version      string
	Platform     string
	Workspace    string
	Capabilities []string
	ConnectedAt  time.Time
	LastSeenAt   time.Time
	Send         chan<- OutboundMessage
}

type SessionSnapshot struct {
	SessionId    string    `json:"session_id"`
	ClientId     string    `json:"client_id"`
	UserId       int       `json:"user_id"`
	TokenId      int       `json:"token_id"`
	Name         string    `json:"name"`
	Version      string    `json:"version"`
	Platform     string    `json:"platform"`
	Workspace    string    `json:"workspace"`
	Capabilities []string  `json:"capabilities"`
	ConnectedAt  time.Time `json:"connected_at"`
	LastSeenAt   time.Time `json:"last_seen_at"`
}

type Hub struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	byClient map[string]string
	pending  map[string]pendingCall
}

func NewHub() *Hub {
	return &Hub{
		sessions: make(map[string]*Session),
		byClient: make(map[string]string),
		pending:  make(map[string]pendingCall),
	}
}

type OutboundMessage struct {
	Type string
	Id   string
	Data any
}

type ToolCallRequest struct {
	Id        string
	ToolName  string
	Arguments map[string]any
}

type ToolCallResponse struct {
	Session SessionSnapshot
	Result  dto.BridgeToolCallResult
	Err     error
}

type ClientError struct {
	Code    string
	Message string
}

func (e *ClientError) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	if e.Code != "" {
		return e.Code
	}
	return "bridge client error"
}

type pendingCall struct {
	session  SessionSnapshot
	response chan ToolCallResponse
}

type CloseSessionOptions struct {
	Reason string
	Notify bool
}

type SessionMetadata struct {
	Name         string
	Version      string
	Platform     string
	Workspace    string
	Capabilities []string
}

func (h *Hub) Register(session Session) {
	if h == nil || session.SessionId == "" || session.ClientId == "" {
		return
	}
	now := time.Now()
	if session.ConnectedAt.IsZero() {
		session.ConnectedAt = now
	}
	if session.LastSeenAt.IsZero() {
		session.LastSeenAt = now
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if previousSessionId := h.byClient[session.ClientId]; previousSessionId != "" && previousSessionId != session.SessionId {
		delete(h.sessions, previousSessionId)
		h.failPendingForSessionLocked(previousSessionId, ErrClientDisconnected)
	}
	copied := session
	h.sessions[session.SessionId] = &copied
	h.byClient[session.ClientId] = session.SessionId
}

func (h *Hub) Unregister(sessionId string) (SessionSnapshot, bool) {
	if h == nil || sessionId == "" {
		return SessionSnapshot{}, false
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	session, ok := h.sessions[sessionId]
	if !ok || session == nil {
		return SessionSnapshot{}, false
	}
	delete(h.sessions, sessionId)
	if h.byClient[session.ClientId] == sessionId {
		delete(h.byClient, session.ClientId)
	}
	h.failPendingForSessionLocked(sessionId, ErrClientDisconnected)
	return snapshotSession(*session), true
}

func (h *Hub) CloseSession(sessionId string, options CloseSessionOptions) (SessionSnapshot, bool) {
	if h == nil || sessionId == "" {
		return SessionSnapshot{}, false
	}
	h.mu.Lock()
	session, ok := h.sessions[sessionId]
	if !ok || session == nil {
		h.mu.Unlock()
		return SessionSnapshot{}, false
	}
	snapshot := snapshotSession(*session)
	send := session.Send
	delete(h.sessions, sessionId)
	if h.byClient[session.ClientId] == sessionId {
		delete(h.byClient, session.ClientId)
	}
	h.failPendingForSessionLocked(sessionId, ErrClientDisconnected)
	h.mu.Unlock()

	if options.Notify && send != nil {
		message := options.Reason
		if message == "" {
			message = "session closed"
		}
		select {
		case send <- OutboundMessage{Type: "close", Data: map[string]any{"reason": message}}:
		default:
		}
	}
	return snapshot, true
}

func (h *Hub) UpdateClientMetadata(clientId string, metadata SessionMetadata) bool {
	if h == nil || clientId == "" {
		return false
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	sessionId := h.byClient[clientId]
	session, ok := h.sessions[sessionId]
	if !ok || session == nil {
		return false
	}
	session.Name = metadata.Name
	session.Version = metadata.Version
	session.Platform = metadata.Platform
	session.Workspace = metadata.Workspace
	session.Capabilities = append([]string(nil), metadata.Capabilities...)
	return true
}

func (h *Hub) Touch(sessionId string) bool {
	if h == nil || sessionId == "" {
		return false
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	session, ok := h.sessions[sessionId]
	if !ok || session == nil {
		return false
	}
	session.LastSeenAt = time.Now()
	return true
}

func (h *Hub) GetByClient(clientId string) (SessionSnapshot, bool) {
	if h == nil || clientId == "" {
		return SessionSnapshot{}, false
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	sessionId := h.byClient[clientId]
	session, ok := h.sessions[sessionId]
	if !ok || session == nil {
		return SessionSnapshot{}, false
	}
	return snapshotSession(*session), true
}

func (h *Hub) SelectSession(userId int, preferredClientId string, capability string) (SessionSnapshot, bool) {
	sessions := h.SelectSessions(userId, preferredClientId, capability)
	if len(sessions) == 0 {
		return SessionSnapshot{}, false
	}
	return sessions[0], true
}

func (h *Hub) SelectSessions(userId int, preferredClientId string, capability string) []SessionSnapshot {
	if h == nil || userId < 0 {
		return nil
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	if preferredClientId != "" {
		sessionId := h.byClient[preferredClientId]
		session, ok := h.sessions[sessionId]
		if !ok || session == nil || (userId > 0 && session.UserId != userId) {
			return nil
		}
		if !sessionSupports(*session, capability) {
			return nil
		}
		return []SessionSnapshot{snapshotSession(*session)}
	}
	snapshots := make([]SessionSnapshot, 0, len(h.sessions))
	for _, session := range h.sessions {
		if session == nil || (userId > 0 && session.UserId != userId) {
			continue
		}
		if !sessionSupports(*session, capability) {
			continue
		}
		snapshots = append(snapshots, snapshotSession(*session))
	}
	sortSessionSnapshots(snapshots)
	return snapshots
}

func (h *Hub) ForwardToolCall(ctx context.Context, sessionId string, req ToolCallRequest) (ToolCallResponse, error) {
	if h == nil || sessionId == "" || req.Id == "" {
		return ToolCallResponse{}, ErrClientNotFound
	}
	if ctx == nil {
		ctx = context.Background()
	}

	h.mu.Lock()
	session, ok := h.sessions[sessionId]
	if !ok || session == nil {
		h.mu.Unlock()
		return ToolCallResponse{}, ErrClientNotFound
	}
	if session.Send == nil {
		h.mu.Unlock()
		return ToolCallResponse{}, ErrClientUnavailable
	}
	if _, exists := h.pending[req.Id]; exists {
		h.mu.Unlock()
		return ToolCallResponse{}, errors.New("bridge request id already exists")
	}
	responseCh := make(chan ToolCallResponse, 1)
	snapshot := snapshotSession(*session)
	h.pending[req.Id] = pendingCall{
		session:  snapshot,
		response: responseCh,
	}
	send := session.Send
	h.mu.Unlock()

	message := OutboundMessage{
		Type: MessageTypeToolCall,
		Id:   req.Id,
		Data: dto.BridgeToolCallRequest{
			RequestId: req.Id,
			ToolName:  req.ToolName,
			Arguments: req.Arguments,
		},
	}
	select {
	case send <- message:
	case <-ctx.Done():
		h.removePending(req.Id)
		return ToolCallResponse{}, ctx.Err()
	}

	select {
	case response := <-responseCh:
		if response.Err != nil {
			return response, response.Err
		}
		return response, nil
	case <-ctx.Done():
		h.removePending(req.Id)
		return ToolCallResponse{}, ctx.Err()
	}
}

func (h *Hub) CompleteToolCall(requestId string, result dto.BridgeToolCallResult) bool {
	if h == nil || requestId == "" {
		return false
	}
	h.mu.Lock()
	pending, ok := h.pending[requestId]
	if ok {
		delete(h.pending, requestId)
	}
	h.mu.Unlock()
	if !ok {
		return false
	}
	pending.response <- ToolCallResponse{
		Session: pending.session,
		Result:  result,
	}
	return true
}

func (h *Hub) FailToolCall(requestId string, code string, message string) bool {
	if h == nil || requestId == "" {
		return false
	}
	h.mu.Lock()
	pending, ok := h.pending[requestId]
	if ok {
		delete(h.pending, requestId)
	}
	h.mu.Unlock()
	if !ok {
		return false
	}
	pending.response <- ToolCallResponse{
		Session: pending.session,
		Err: &ClientError{
			Code:    code,
			Message: message,
		},
	}
	return true
}

func (h *Hub) List() []SessionSnapshot {
	if h == nil {
		return nil
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	snapshots := make([]SessionSnapshot, 0, len(h.sessions))
	for _, session := range h.sessions {
		if session == nil {
			continue
		}
		snapshots = append(snapshots, snapshotSession(*session))
	}
	return snapshots
}

func (h *Hub) Count() int {
	if h == nil {
		return 0
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.sessions)
}

func (h *Hub) removePending(requestId string) {
	h.mu.Lock()
	delete(h.pending, requestId)
	h.mu.Unlock()
}

func (h *Hub) failPendingForSessionLocked(sessionId string, err error) {
	for requestId, pending := range h.pending {
		if pending.session.SessionId != sessionId {
			continue
		}
		delete(h.pending, requestId)
		pending.response <- ToolCallResponse{
			Session: pending.session,
			Err:     err,
		}
	}
}

func sessionSupports(session Session, capability string) bool {
	if capability == "" || len(session.Capabilities) == 0 {
		return true
	}
	for _, item := range session.Capabilities {
		if item == capability {
			return true
		}
	}
	return false
}

func sortSessionSnapshots(snapshots []SessionSnapshot) {
	sort.SliceStable(snapshots, func(i, j int) bool {
		left := snapshots[i]
		right := snapshots[j]
		if !left.LastSeenAt.Equal(right.LastSeenAt) {
			return left.LastSeenAt.After(right.LastSeenAt)
		}
		if !left.ConnectedAt.Equal(right.ConnectedAt) {
			return left.ConnectedAt.After(right.ConnectedAt)
		}
		if left.ClientId != right.ClientId {
			return left.ClientId < right.ClientId
		}
		return left.SessionId < right.SessionId
	})
}

func snapshotSession(session Session) SessionSnapshot {
	capabilities := append([]string(nil), session.Capabilities...)
	return SessionSnapshot{
		SessionId:    session.SessionId,
		ClientId:     session.ClientId,
		UserId:       session.UserId,
		TokenId:      session.TokenId,
		Name:         session.Name,
		Version:      session.Version,
		Platform:     session.Platform,
		Workspace:    session.Workspace,
		Capabilities: capabilities,
		ConnectedAt:  session.ConnectedAt,
		LastSeenAt:   session.LastSeenAt,
	}
}

var DefaultHub = NewHub()
