// Package session manages ephemeral in-memory client sessions.
package session

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	engramErrors "github.com/pythondatascrape/engram/internal/errors"
	engramctx "github.com/pythondatascrape/engram/internal/context"
)

// Status represents the lifecycle state of a session.
type Status string

const (
	StatusActive    Status = "ACTIVE"
	StatusCompleted Status = "COMPLETED"
	StatusEvicted   Status = "EVICTED"
)

// Opts holds provider/model configuration chosen at session creation.
type Opts struct {
	Provider   string
	Model      string
	Codebook   string
	Serializer string
}

// Session is a single client conversation session.
type Session struct {
	mu sync.RWMutex

	ID                 string
	ClientID           string
	Status             Status
	CreatedAt          time.Time
	LastActivity       time.Time
	Opts               Opts
	SerializedIdentity string
	Turns               int
	TokensSent          int
	TokensSaved         int
	ContextTokensSaved  int
	CumulativeBaseline  int // sum of per-turn "what would have been sent without Engram"
	RawHistoryBytes     int // running total of raw turn sizes (for next turn's baseline)
	IdentityTokens      int
	History            *engramctx.History
	ContextCodebook    *engramctx.ContextCodebook
}

// generateID produces a 32-char hex session identifier using crypto/rand.
func generateID() string {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(buf[:])
}

// newSession constructs a new active session with a generated ID.
func newSession(clientID string, opts Opts) *Session {
	now := time.Now()
	return &Session{
		ID:           generateID(),
		ClientID:     clientID,
		Status:       StatusActive,
		CreatedAt:    now,
		LastActivity: now,
		Opts:         opts,
	}
}

// CheckOwnership returns PERMISSION_DENIED if clientID does not own the session.
func (s *Session) CheckOwnership(clientID string) error {
	s.mu.RLock()
	owner := s.ClientID
	s.mu.RUnlock()
	if owner != clientID {
		return engramErrors.PERMISSION_DENIED
	}
	return nil
}

// SetIdentity stores a serialized identity blob on the session.
func (s *Session) SetIdentity(serialized string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.SerializedIdentity = serialized
}

// SetIdentityTokens stores the raw (uncompressed) identity character count for savings tracking.
func (s *Session) SetIdentityTokens(n int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.IdentityTokens = n
}

// SetContextCodebook stores the derived context codebook on the session.
func (s *Session) SetContextCodebook(cb *engramctx.ContextCodebook) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ContextCodebook = cb
}

// SetHistory stores the conversation history on the session.
func (s *Session) SetHistory(h *engramctx.History) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.History = h
}

// IdentityBaseline returns the raw identity char count used for savings tracking.
func (s *Session) IdentityBaseline() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.IdentityTokens
}

// RecordTurn increments the turn counter and accumulates all token counts.
// baselineThisTurn is what would have been sent without any Engram compression.
// rawTurnBytes is the raw size of this turn's query+response (for next turn's baseline).
func (s *Session) RecordTurn(tokensSent, identitySaved, contextSaved, baselineThisTurn, rawTurnBytes int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Turns++
	s.TokensSent += tokensSent
	s.TokensSaved += identitySaved
	s.ContextTokensSaved += contextSaved
	s.CumulativeBaseline += baselineThisTurn
	s.RawHistoryBytes += rawTurnBytes
	s.LastActivity = time.Now()
}

// RawHistory returns the accumulated raw turn bytes (query+response) from all prior turns.
func (s *Session) RawHistory() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.RawHistoryBytes
}

// Touch updates LastActivity to now.
func (s *Session) Touch() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.LastActivity = time.Now()
}

// RequestContext holds the minimal fields needed for request handling.
type RequestContext struct {
	ID                 string
	SerializedIdentity string
	Model              string
	History            *engramctx.History
	ContextCodebook    *engramctx.ContextCodebook
}

// RequestCtx returns a lightweight snapshot for prompt assembly.
func (s *Session) RequestCtx() RequestContext {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return RequestContext{
		ID:                 s.ID,
		SerializedIdentity: s.SerializedIdentity,
		Model:              s.Opts.Model,
		History:            s.History,
		ContextCodebook:    s.ContextCodebook,
	}
}

// Snapshot returns a value copy of the session safe for reading without holding the lock.
func (s *Session) Snapshot() Session {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return Session{
		ID:                 s.ID,
		ClientID:           s.ClientID,
		Status:             s.Status,
		CreatedAt:          s.CreatedAt,
		LastActivity:       s.LastActivity,
		Opts:               s.Opts,
		SerializedIdentity: s.SerializedIdentity,
		Turns:              s.Turns,
		TokensSent:          s.TokensSent,
		TokensSaved:         s.TokensSaved,
		ContextTokensSaved:  s.ContextTokensSaved,
		CumulativeBaseline:  s.CumulativeBaseline,
		RawHistoryBytes:     s.RawHistoryBytes,
		IdentityTokens:      s.IdentityTokens,
		History:            s.History,
		ContextCodebook:    s.ContextCodebook,
	}
}
