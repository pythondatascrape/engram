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

func BenchmarkHandleRequest_WithHistory(b *testing.B) {
	ctx := context.Background()

	cb, err := codebook.Parse([]byte(testCodebookYAML))
	if err != nil {
		b.Fatalf("codebook parse: %v", err)
	}
	mgr := session.NewManager(session.ManagerConfig{MaxSessions: 100})
	ser := serializer.New()
	fakeFactory := func(_ string) (provider.Provider, error) {
		return &fakeProvider{response: "hello from LLM"}, nil
	}
	p := pool.New(pool.Config{MaxConnections: 2}, fakeFactory)
	h := NewHandler(mgr, ser, cb, p, 10)

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
