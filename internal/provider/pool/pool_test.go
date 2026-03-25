package pool_test

import (
	"context"
	"testing"
	"time"

	"github.com/pythondatascrape/engram/internal/provider"
	"github.com/pythondatascrape/engram/internal/provider/pool"
)

// mockProvider is a no-op provider used in tests.
type mockProvider struct {
	name string
}

func (m *mockProvider) Name() string { return m.name }
func (m *mockProvider) Send(_ context.Context, _ *provider.Request) (<-chan provider.Chunk, error) {
	ch := make(chan provider.Chunk, 1)
	ch <- provider.Chunk{Text: "ok", Index: 0, Done: true}
	close(ch)
	return ch, nil
}
func (m *mockProvider) Healthcheck(_ context.Context) error { return nil }
func (m *mockProvider) Capabilities() provider.Capabilities {
	return provider.Capabilities{
		Models:           []string{"mock-model"},
		MaxContextWindow: 4096,
		SupportsStreaming: true,
	}
}
func (m *mockProvider) Close() error { return nil }

// factory returns a fresh mockProvider for any API key.
func factory(apiKey string) (provider.Provider, error) {
	return &mockProvider{name: "mock-" + apiKey}, nil
}

// TestGetAndReturn verifies basic acquire-and-release behaviour.
func TestGetAndReturn(t *testing.T) {
	p := pool.New(pool.Config{MaxConnections: 2}, factory)

	conn, err := p.Get(context.Background(), "key-a")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if conn == nil {
		t.Fatal("expected non-nil conn")
	}
	if conn.Provider == nil {
		t.Fatal("expected non-nil Provider")
	}

	// Return and re-acquire; should reuse the same underlying conn.
	p.Return(conn)

	conn2, err := p.Get(context.Background(), "key-a")
	if err != nil {
		t.Fatalf("second Get failed: %v", err)
	}
	// The returned connection should be the same object (last-in first-out).
	if conn2.Provider != conn.Provider {
		t.Error("expected same provider instance after return")
	}
	p.Return(conn2)
}

// TestMaxConnectionsEnforced verifies that a Get blocks and fails when the
// pool is exhausted and the context is cancelled.
func TestMaxConnectionsEnforced(t *testing.T) {
	p := pool.New(pool.Config{MaxConnections: 1}, factory)

	// Acquire the only allowed connection.
	conn, err := p.Get(context.Background(), "key-b")
	if err != nil {
		t.Fatalf("first Get failed: %v", err)
	}

	// A second Get with a short-lived context must fail.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err = p.Get(ctx, "key-b")
	if err == nil {
		t.Fatal("expected error when pool exhausted and ctx cancelled, got nil")
	}

	// Return the first connection to clean up.
	p.Return(conn)
}

// TestDifferentAPIKeysSeparatePools verifies isolation between API keys.
func TestDifferentAPIKeysSeparatePools(t *testing.T) {
	p := pool.New(pool.Config{MaxConnections: 1}, factory)

	connA, err := p.Get(context.Background(), "key-x")
	if err != nil {
		t.Fatalf("Get key-x failed: %v", err)
	}

	// key-y has its own sub-pool; should succeed even though key-x is exhausted.
	connB, err := p.Get(context.Background(), "key-y")
	if err != nil {
		t.Fatalf("Get key-y failed: %v", err)
	}

	if connA.Provider == connB.Provider {
		t.Error("different keys should yield different provider instances")
	}

	stats := p.AllStats()
	if len(stats) != 2 {
		t.Errorf("expected 2 sub-pools, got %d", len(stats))
	}

	p.Return(connA)
	p.Return(connB)
}

// TestAllStats verifies the stats snapshot is accurate.
func TestAllStats(t *testing.T) {
	p := pool.New(pool.Config{MaxConnections: 3}, factory)

	conn1, _ := p.Get(context.Background(), "key-s")
	conn2, _ := p.Get(context.Background(), "key-s")

	stats := p.AllStats()
	if len(stats) != 1 {
		t.Fatalf("expected 1 sub-pool, got %d", len(stats))
	}
	s := stats[0]
	if s.Active != 2 {
		t.Errorf("expected Active=2, got %d", s.Active)
	}
	if s.Available != 0 {
		t.Errorf("expected Available=0, got %d", s.Available)
	}
	if s.MaxConns != 3 {
		t.Errorf("expected MaxConns=3, got %d", s.MaxConns)
	}

	p.Return(conn1)
	p.Return(conn2)

	stats = p.AllStats()
	if stats[0].Active != 0 || stats[0].Available != 2 {
		t.Errorf("after return: Active=%d Available=%d", stats[0].Active, stats[0].Available)
	}
}
