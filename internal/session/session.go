// Package session manages ephemeral in-memory client sessions.
package session

import (
	"sync"
	"time"

	"github.com/google/uuid"
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

// newSession constructs a new active session with a generated UUID.
func newSession(clientID string, opts Opts) *Session {
	now := time.Now()
	return &Session{
		ID:           uuid.New().String(),
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
