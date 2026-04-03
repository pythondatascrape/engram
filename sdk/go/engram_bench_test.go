package engram_test

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"

	engram "github.com/pythondatascrape/engram/sdk/go"
)

func newBenchDaemon(b *testing.B) (*fakeDaemon, string) {
	b.Helper()
	dir, err := os.MkdirTemp("", "eb")
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { os.RemoveAll(dir) })
	sock := filepath.Join(dir, "e.sock")

	ln, err := net.Listen("unix", sock)
	if err != nil {
		b.Fatalf("listen: %v", err)
	}

	d := &fakeDaemon{
		listener:  ln,
		responses: make(map[string]json.RawMessage),
	}
	return d, sock
}

func BenchmarkGetStats_PersistentConn(b *testing.B) {
	d, sock := newBenchDaemon(b)
	d.setResponse("engram.getStats", map[string]any{
		"sessions":           1,
		"total_tokens_saved": 488,
	})
	go d.serve()
	defer d.close()

	ctx := context.Background()
	client, err := engram.Connect(ctx, sock)
	if err != nil {
		b.Fatal(err)
	}
	defer client.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err = client.GetStats(ctx)
		if err != nil {
			b.Fatal(err)
		}
	}
}
