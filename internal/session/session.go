// Package session manages ephemeral in-memory client sessions.
package session

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
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
	Turns              int
	TokensSent         int
	TokensSaved        int
	IdentityTokens     int
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

// SetIdentity stores a serialized identity blob on the session.
func (s *Session) SetIdentity(serialized string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.SerializedIdentity = serialized
}

// RecordTurn increments the turn counter and accumulates token counts directly.
func (s *Session) RecordTurn(tokensSent, tokensSaved int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Turns++
	s.TokensSent += tokensSent
	s.TokensSaved += tokensSaved
	s.LastActivity = time.Now()
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
}

// RequestCtx returns a lightweight snapshot for prompt assembly.
func (s *Session) RequestCtx() RequestContext {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return RequestContext{
		ID:                 s.ID,
		SerializedIdentity: s.SerializedIdentity,
		Model:              s.Opts.Model,
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
		TokensSent:         s.TokensSent,
		TokensSaved:        s.TokensSaved,
		IdentityTokens:     s.IdentityTokens,
	}
}
