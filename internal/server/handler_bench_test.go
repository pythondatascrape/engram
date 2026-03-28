package server

import (
	"context"
	"testing"

	"github.com/pythondatascrape/engram/internal/session"
)

func BenchmarkHandleRequest_WithHistory(b *testing.B) {
	ctx := context.Background()

	// Create a temporary test instance for the helper
	t := &testing.T{}
	mgr, ser, cb, p := newTestDeps(t)
	h := NewHandler(mgr, ser, cb, p)

	// First request to establish session with identity and context schema
	first, err := h.HandleRequest(ctx, IncomingRequest{
		ClientID: "bench-client",
		APIKey:   "key-1",
		Query:    "initial query",
		Identity: map[string]string{"role": "admin", "domain": "fire"},
		Opts:     session.Opts{Provider: "fake", Model: "fake-model"},
	})
	if err != nil {
		b.Fatalf("setup failed: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := h.HandleRequest(ctx, IncomingRequest{
			ClientID:  "bench-client",
			APIKey:    "key-1",
			SessionID: first.SessionID,
			Query:     "benchmark query turn",
			Opts:      session.Opts{Provider: "fake", Model: "fake-model"},
		})
		if err != nil {
			b.Fatalf("benchmark iteration failed: %v", err)
		}
	}
}
