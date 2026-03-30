package session_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	engramErrors "github.com/pythondatascrape/engram/internal/errors"
	"github.com/pythondatascrape/engram/internal/session"
	engramctx "github.com/pythondatascrape/engram/internal/context"
)

func defaultConfig() session.ManagerConfig {
	return session.ManagerConfig{
		IdleTimeout: 30 * time.Minute,
		MaxTTL:      2 * time.Hour,
		MaxSessions: 100,
	}
}

func defaultOpts() session.Opts {
	return session.Opts{
		Provider:   "anthropic",
		Model:      "claude-3-5-sonnet",
		Codebook:   "default",
		Serializer: "json",
	}
}

// 1. Create and Get
func TestCreateAndGet(t *testing.T) {
	m := session.NewManager(defaultConfig())
	ctx := context.Background()

	s, err := m.Create(ctx, "client-1", defaultOpts())
	require.NoError(t, err)
	require.NotNil(t, s)
	assert.NotEmpty(t, s.ID)
	assert.Equal(t, "client-1", s.ClientID)
	assert.Equal(t, session.StatusActive, s.Status)

	got, err := m.Get(s.ID)
	require.NoError(t, err)
	assert.Equal(t, s.ID, got.ID)
}

// 2. Get non-existent returns error
func TestGetNonExistent(t *testing.T) {
	m := session.NewManager(defaultConfig())

	_, err := m.Get("does-not-exist")
	require.Error(t, err)
	assert.Equal(t, engramErrors.SESSION_NOT_FOUND, err)
}

// 3. Ownership check
func TestOwnershipCheck(t *testing.T) {
	m := session.NewManager(defaultConfig())
	ctx := context.Background()

	s, err := m.Create(ctx, "client-owner", defaultOpts())
	require.NoError(t, err)

	// Correct owner
	err = m.CheckOwnership(s.ID, "client-owner")
	assert.NoError(t, err)

	// Wrong owner
	err = m.CheckOwnership(s.ID, "client-intruder")
	require.Error(t, err)
	assert.Equal(t, engramErrors.PERMISSION_DENIED, err)
}

// 4. Close session
func TestCloseSession(t *testing.T) {
	m := session.NewManager(defaultConfig())
	ctx := context.Background()

	s, err := m.Create(ctx, "client-1", defaultOpts())
	require.NoError(t, err)

	closed, err := m.Close(s.ID)
	require.NoError(t, err)
	assert.Equal(t, session.StatusCompleted, closed.Status)
	assert.Equal(t, 0, m.Count())

	// Get should now fail
	_, err = m.Get(s.ID)
	assert.Equal(t, engramErrors.SESSION_NOT_FOUND, err)
}

// 5. Max sessions limit
func TestMaxSessionsLimit(t *testing.T) {
	cfg := session.ManagerConfig{
		MaxSessions: 2,
	}
	m := session.NewManager(cfg)
	ctx := context.Background()

	_, err := m.Create(ctx, "c1", defaultOpts())
	require.NoError(t, err)

	_, err = m.Create(ctx, "c2", defaultOpts())
	require.NoError(t, err)

	_, err = m.Create(ctx, "c3", defaultOpts())
	require.Error(t, err)
	assert.Equal(t, engramErrors.SESSION_LIMIT_REACHED, err)
}

// 6. SetIdentity
func TestSetIdentity(t *testing.T) {
	m := session.NewManager(defaultConfig())
	ctx := context.Background()

	s, err := m.Create(ctx, "client-1", defaultOpts())
	require.NoError(t, err)

	err = m.SetIdentity(s.ID, "serialized-blob-xyz")
	require.NoError(t, err)

	snap := s.Snapshot()
	assert.Equal(t, "serialized-blob-xyz", snap.SerializedIdentity)
}

// 7. RecordTurn — turns counter + tokens
func TestRecordTurn(t *testing.T) {
	m := session.NewManager(defaultConfig())
	ctx := context.Background()

	s, err := m.Create(ctx, "client-1", defaultOpts())
	require.NoError(t, err)

	err = m.RecordTurn(s.ID, 100, 50, 0, 0, 0)
	require.NoError(t, err)

	err = m.RecordTurn(s.ID, 200, 75, 0, 0, 0)
	require.NoError(t, err)

	snap := s.Snapshot()
	assert.Equal(t, 2, snap.Turns)
	assert.Equal(t, 300, snap.TokensSent)
	assert.Equal(t, 125, snap.TokensSaved)
}

func TestEvictIdle(t *testing.T) {
	cfg := session.ManagerConfig{
		IdleTimeout: 1 * time.Millisecond,
		MaxTTL:      1 * time.Hour,
		MaxSessions: 100,
	}
	m := session.NewManager(cfg)
	ctx := context.Background()

	s, err := m.Create(ctx, "client-1", defaultOpts())
	require.NoError(t, err)

	time.Sleep(5 * time.Millisecond)

	evicted := m.EvictIdle()
	assert.Contains(t, evicted, s.ID)
	assert.Equal(t, 0, m.Count())
}

func TestEvictAll(t *testing.T) {
	m := session.NewManager(defaultConfig())
	ctx := context.Background()

	m.Create(ctx, "c1", defaultOpts())
	m.Create(ctx, "c2", defaultOpts())

	evicted := m.EvictAll()
	assert.Len(t, evicted, 2)
	assert.Equal(t, 0, m.Count())
}

func TestTouch(t *testing.T) {
	m := session.NewManager(defaultConfig())
	ctx := context.Background()

	s, err := m.Create(ctx, "client-1", defaultOpts())
	require.NoError(t, err)

	before := s.Snapshot().LastActivity
	time.Sleep(2 * time.Millisecond)
	s.Touch()
	after := s.Snapshot().LastActivity

	assert.True(t, after.After(before), "Touch should advance LastActivity")
}

func TestCheckOwnershipNonexistent(t *testing.T) {
	m := session.NewManager(defaultConfig())

	err := m.CheckOwnership("bogus", "client-1")
	require.Error(t, err)
	assert.Equal(t, engramErrors.SESSION_NOT_FOUND, err)
}

func TestSetIdentityNonexistent(t *testing.T) {
	m := session.NewManager(defaultConfig())

	err := m.SetIdentity("bogus", "data")
	require.Error(t, err)
	assert.Equal(t, engramErrors.SESSION_NOT_FOUND, err)
}

func TestRecordTurnNonexistent(t *testing.T) {
	m := session.NewManager(defaultConfig())

	err := m.RecordTurn("bogus", 100, 50, 0, 0, 0)
	require.Error(t, err)
	assert.Equal(t, engramErrors.SESSION_NOT_FOUND, err)
}

func TestCloseNonexistent(t *testing.T) {
	m := session.NewManager(defaultConfig())

	_, err := m.Close("bogus")
	require.Error(t, err)
	assert.Equal(t, engramErrors.SESSION_NOT_FOUND, err)
}

func TestEvictIdleByMaxTTL(t *testing.T) {
	cfg := session.ManagerConfig{
		IdleTimeout: 1 * time.Hour,
		MaxTTL:      1 * time.Millisecond,
		MaxSessions: 100,
	}
	m := session.NewManager(cfg)
	ctx := context.Background()

	s, err := m.Create(ctx, "client-1", defaultOpts())
	require.NoError(t, err)

	time.Sleep(5 * time.Millisecond)

	evicted := m.EvictIdle()
	assert.Contains(t, evicted, s.ID)
	assert.Equal(t, 0, m.Count())
}

func TestSession_HistoryIntegration(t *testing.T) {
	ctx := context.Background()
	m := session.NewManager(session.ManagerConfig{MaxSessions: 10})
	s, err := m.Create(ctx, "client-1", session.Opts{})
	require.NoError(t, err)

	cb, _ := engramctx.DeriveCodebook("app", map[string]string{"role": "text", "content": "text"})
	s.SetContextCodebook(cb)
	s.SetHistory(engramctx.NewHistory())

	snap := s.Snapshot()
	assert.NotNil(t, snap.History)
	assert.NotNil(t, snap.ContextCodebook)
}

func TestEvictIdleKeepsActive(t *testing.T) {
	cfg := session.ManagerConfig{
		IdleTimeout: 1 * time.Hour,
		MaxTTL:      1 * time.Hour,
		MaxSessions: 100,
	}
	m := session.NewManager(cfg)
	ctx := context.Background()

	_, err := m.Create(ctx, "client-1", defaultOpts())
	require.NoError(t, err)

	evicted := m.EvictIdle()
	assert.Empty(t, evicted)
	assert.Equal(t, 1, m.Count())
}
