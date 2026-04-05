package proxy

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"
)

func TestServerStartStop(t *testing.T) {
	srv := New(0, 10, t.TempDir(), "http://127.0.0.1:1") // port 0 = OS assigns
	if err := srv.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := srv.Stop(ctx); err != nil {
		t.Fatalf("Stop: %v", err)
	}
}

func TestServerAddr(t *testing.T) {
	srv := New(0, 10, t.TempDir(), "http://127.0.0.1:1")
	if err := srv.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Stop(ctx)
	}()

	addr := srv.Addr()
	if addr == nil {
		t.Fatal("Addr() returned nil after Start()")
	}
	// Verify the address is actually reachable.
	resp, err := http.Get(fmt.Sprintf("http://%s/v1/models", addr.String()))
	if err != nil {
		t.Fatalf("GET /v1/models: %v", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	// Any response (even 502) means the server is listening.
}

func TestAddrBeforeStart(t *testing.T) {
	srv := New(0, 10, t.TempDir(), "http://127.0.0.1:1")
	if addr := srv.Addr(); addr != nil {
		t.Fatalf("Addr() should be nil before Start(), got %v", addr)
	}
}
