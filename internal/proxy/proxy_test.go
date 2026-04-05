package proxy

import (
	"context"
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
