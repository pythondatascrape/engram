package session

import (
	"context"
	"sync"
	"time"

	engramErrors "github.com/pythondatascrape/engram/internal/errors"
)

// ManagerConfig controls eviction and capacity limits for the Manager.
type ManagerConfig struct {
	IdleTimeout time.Duration
	MaxTTL      time.Duration
	MaxSessions int
}

// Manager holds all active sessions and enforces capacity and eviction policies.
type Manager struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	cfg      ManagerConfig
}

// NewManager constructs a Manager with the given configuration.
func NewManager(cfg ManagerConfig) *Manager {
	return &Manager{
		sessions: make(map[string]*Session),
		cfg:      cfg,
	}
}

// Create allocates a new session for clientID if capacity permits.
func (m *Manager) Create(_ context.Context, clientID string, opts Opts) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.cfg.MaxSessions > 0 && len(m.sessions) >= m.cfg.MaxSessions {
		return nil, engramErrors.SESSION_LIMIT_REACHED
	}

	s := newSession(clientID, opts)
	m.sessions[s.ID] = s
	return s, nil
}

// Get retrieves a session by ID.
func (m *Manager) Get(id string) (*Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	s, ok := m.sessions[id]
	if !ok {
		return nil, engramErrors.SESSION_NOT_FOUND
	}
	return s, nil
}

// CheckOwnership returns an error if the session does not belong to clientID.
func (m *Manager) CheckOwnership(id, clientID string) error {
	s, err := m.Get(id)
	if err != nil {
		return err
	}

	s.mu.RLock()
	owner := s.ClientID
	s.mu.RUnlock()

	if owner != clientID {
		return engramErrors.PERMISSION_DENIED
	}
	return nil
}

// SetIdentity stores a serialized identity blob on the session.
func (m *Manager) SetIdentity(id, serialized string) error {
	s, err := m.Get(id)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.SerializedIdentity = serialized
	return nil
}

// RecordTurn increments the turn counter, accumulates token counts, and touches LastActivity.
func (m *Manager) RecordTurn(id string, tokensSent, identitySaved, contextSaved, baselineThisTurn, rawTurnBytes int) error {
	s, err := m.Get(id)
	if err != nil {
		return err
	}
	s.RecordTurn(tokensSent, identitySaved, contextSaved, baselineThisTurn, rawTurnBytes)
	return nil
}

// Close marks a session as completed and removes it from the active map.
func (m *Manager) Close(id string) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, ok := m.sessions[id]
	if !ok {
		return nil, engramErrors.SESSION_NOT_FOUND
	}

	s.mu.Lock()
	s.Status = StatusCompleted
	s.mu.Unlock()

	delete(m.sessions, id)
	return s, nil
}

// EvictIdle removes sessions that have exceeded IdleTimeout or MaxTTL and returns their IDs.
func (m *Manager) EvictIdle() []string {
	now := time.Now()
	var evicted []string

	m.mu.Lock()
	defer m.mu.Unlock()

	for id, s := range m.sessions {
		s.mu.Lock()
		idleExpired := m.cfg.IdleTimeout > 0 && now.Sub(s.LastActivity) > m.cfg.IdleTimeout
		ttlExpired := m.cfg.MaxTTL > 0 && now.Sub(s.CreatedAt) > m.cfg.MaxTTL
		if idleExpired || ttlExpired {
			s.Status = StatusEvicted
			s.mu.Unlock()
			delete(m.sessions, id)
			evicted = append(evicted, id)
		} else {
			s.mu.Unlock()
		}
	}
	return evicted
}

// EvictAll evicts every remaining session (used during shutdown).
func (m *Manager) EvictAll() []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	ids := make([]string, 0, len(m.sessions))
	for id, s := range m.sessions {
		s.mu.Lock()
		s.Status = StatusEvicted
		s.mu.Unlock()
		delete(m.sessions, id)
		ids = append(ids, id)
	}
	return ids
}

// Count returns the number of active sessions.
func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.sessions)
}

// AggregateStats holds cumulative token accounting across all active sessions.
type AggregateStats struct {
	ActiveSessions     int
	TotalTurns         int
	CumulativeBaseline int
	TokensSent         int
	TokensSaved        int
	ContextTokensSaved int
}

// Stats returns cumulative token accounting summed across all active sessions.
func (m *Manager) Stats() AggregateStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var s AggregateStats
	s.ActiveSessions = len(m.sessions)
	for _, sess := range m.sessions {
		snap := sess.Snapshot()
		s.TotalTurns += snap.Turns
		s.CumulativeBaseline += snap.CumulativeBaseline
		s.TokensSent += snap.TokensSent
		s.TokensSaved += snap.TokensSaved
		s.ContextTokensSaved += snap.ContextTokensSaved
	}
	return s
}
