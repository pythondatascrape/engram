// Package session manages ephemeral in-memory client sessions.
package session

import (
	"math/rand/v2"
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

const hexDigits = "0123456789abcdef"

// generateID produces a 32-char hex session identifier using the fast
// math/rand/v2 PRNG. Cryptographic uniqueness is not required for
// ephemeral session IDs — only uniqueness within the server's lifetime.
func generateID() string {
	hi := rand.Uint64()
	lo := rand.Uint64()
	buf := make([]byte, 32)
	for i := 0; i < 8; i++ {
		b := byte(hi >> (56 - uint(i)*8))
		buf[i*2] = hexDigits[b>>4]
		buf[i*2+1] = hexDigits[b&0x0f]
	}
	for i := 0; i < 8; i++ {
		b := byte(lo >> (56 - uint(i)*8))
		buf[16+i*2] = hexDigits[b>>4]
		buf[16+i*2+1] = hexDigits[b&0x0f]
	}
	return string(buf)
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
