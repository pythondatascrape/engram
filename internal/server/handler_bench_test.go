package server

import (
	"context"
	"testing"

	"github.com/pythondatascrape/engram/internal/identity/codebook"
	"github.com/pythondatascrape/engram/internal/identity/serializer"
	"github.com/pythondatascrape/engram/internal/provider"
	"github.com/pythondatascrape/engram/internal/provider/pool"
	"github.com/pythondatascrape/engram/internal/session"
)

func newBenchDeps(b *testing.B) (*session.Manager, *serializer.Serializer, *codebook.Codebook, *pool.Pool) {
	b.Helper()

	cb, err := codebook.Parse([]byte(testCodebookYAML))
	if err != nil {
		b.Fatalf("codebook parse: %v", err)
	}

	mgr := session.NewManager(session.ManagerConfig{MaxSessions: 1000000})
	ser := serializer.New()

	fakeFactory := func(_ string) (provider.Provider, error) {
		return &fakeProvider{response: "benchmark response"}, nil
	}
	p := pool.New(pool.Config{MaxConnections: 100}, fakeFactory)

	return mgr, ser, cb, p
}

// BenchmarkHandleRequest_FirstRequest measures the full hot path for a new session:
// identity validation + session creation + serialization + prompt assembly + provider call.
func BenchmarkHandleRequest_FirstRequest(b *testing.B) {
	mgr, ser, cb, p := newBenchDeps(b)
	h := NewHandler(mgr, ser, cb, p)
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req := IncomingRequest{
			ClientID: "bench-client",
			APIKey:   "bench-key",
			Query:    "What are the egress requirements for commercial buildings?",
			Identity: map[string]string{"role": "admin", "domain": "fire"},
			Opts:     session.Opts{Provider: "fake", Model: "fake-model"},
		}
		_, err := h.HandleRequest(ctx, req)
		if err != nil {
			b.Fatalf("HandleRequest failed: %v", err)
		}
	}
}

// BenchmarkHandleRequest_SubsequentRequest measures the hot path for an existing session:
// session lookup + prompt assembly + provider call (no serialization).
func BenchmarkHandleRequest_SubsequentRequest(b *testing.B) {
	mgr, ser, cb, p := newBenchDeps(b)
	h := NewHandler(mgr, ser, cb, p)
	ctx := context.Background()

	// Create initial session.
	resp, err := h.HandleRequest(ctx, IncomingRequest{
		ClientID: "bench-client",
		APIKey:   "bench-key",
		Query:    "Initial query",
		Identity: map[string]string{"role": "admin", "domain": "fire"},
		Opts:     session.Opts{Provider: "fake", Model: "fake-model"},
	})
	if err != nil {
		b.Fatalf("setup failed: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req := IncomingRequest{
			ClientID:  "bench-client",
			APIKey:    "bench-key",
			SessionID: resp.SessionID,
			Query:     "Follow-up question about egress requirements?",
			Opts:      session.Opts{Provider: "fake", Model: "fake-model"},
		}
		_, err := h.HandleRequest(ctx, req)
		if err != nil {
			b.Fatalf("HandleRequest failed: %v", err)
		}
	}
}
