package engram_test

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"path/filepath"
	"testing"

	engram "github.com/pythondatascrape/engram/sdk/go"
)

type fakeDaemon struct {
	listener  net.Listener
	responses map[string]json.RawMessage
}

func newFakeDaemon(t testing.TB) (*fakeDaemon, string) {
	t.Helper()
	dir := t.TempDir()
	sock := filepath.Join(dir, "engram.sock")

	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	d := &fakeDaemon{
		listener:  ln,
		responses: make(map[string]json.RawMessage),
	}

	return d, sock
}

func (d *fakeDaemon) setResponse(method string, result any) {
	b, _ := json.Marshal(result)
	d.responses[method] = b
}

func (d *fakeDaemon) serve() {
	for {
		conn, err := d.listener.Accept()
		if err != nil {
			return
		}
		go d.handle(conn)
	}
}

func (d *fakeDaemon) handle(conn net.Conn) {
	defer conn.Close()
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		var req struct {
			JSONRPC string          `json:"jsonrpc"`
			ID      int64           `json:"id"`
			Method  string          `json:"method"`
			Params  json.RawMessage `json:"params"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			return
		}

		result, ok := d.responses[req.Method]
		if !ok {
			result = []byte(`{}`)
		}

		resp := map[string]any{
			"jsonrpc": "2.0",
			"id":      req.ID,
			"result":  json.RawMessage(result),
		}
		b, _ := json.Marshal(resp)
		b = append(b, '\n')
		conn.Write(b)
	}
}

func (d *fakeDaemon) close() {
	d.listener.Close()
}

func TestCompress(t *testing.T) {
	d, sock := newFakeDaemon(t)
	d.setResponse("engram.compress", map[string]any{
		"compressed":        "c:expert|t:formal",
		"original_tokens":   500,
		"compressed_tokens": 12,
	})
	go d.serve()
	defer d.close()

	client, err := engram.Connect(context.Background(), sock)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer client.Close()

	result, err := client.Compress(context.Background(), map[string]any{
		"identity": "test",
		"history":  []any{},
		"query":    "hello",
	})
	if err != nil {
		t.Fatalf("compress: %v", err)
	}

	if result["compressed"] != "c:expert|t:formal" {
		t.Errorf("got compressed=%v, want c:expert|t:formal", result["compressed"])
	}
}

func TestDeriveCodebook(t *testing.T) {
	d, sock := newFakeDaemon(t)
	d.setResponse("engram.deriveCodebook", map[string]any{
		"dimensions": []map[string]any{
			{"key": "expertise", "type": "enum", "values": []string{"novice", "expert"}},
		},
	})
	go d.serve()
	defer d.close()

	client, err := engram.Connect(context.Background(), sock)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer client.Close()

	result, err := client.DeriveCodebook(context.Background(), "content about expertise")
	if err != nil {
		t.Fatalf("deriveCodebook: %v", err)
	}

	dims, ok := result["dimensions"].([]any)
	if !ok || len(dims) == 0 {
		t.Fatalf("expected dimensions, got %v", result)
	}
}

func TestGetStats(t *testing.T) {
	d, sock := newFakeDaemon(t)
	d.setResponse("engram.getStats", map[string]any{
		"sessions":           1,
		"total_tokens_saved": 488,
		"compression_ratio":  0.976,
	})
	go d.serve()
	defer d.close()

	client, err := engram.Connect(context.Background(), sock)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer client.Close()

	result, err := client.GetStats(context.Background())
	if err != nil {
		t.Fatalf("getStats: %v", err)
	}

	saved, ok := result["total_tokens_saved"].(float64)
	if !ok || saved != 488 {
		t.Errorf("got total_tokens_saved=%v, want 488", result["total_tokens_saved"])
	}
}

func TestCheckRedundancy(t *testing.T) {
	d, sock := newFakeDaemon(t)
	d.setResponse("engram.checkRedundancy", map[string]any{
		"redundant": false,
		"patterns":  []any{},
	})
	go d.serve()
	defer d.close()

	client, err := engram.Connect(context.Background(), sock)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer client.Close()

	result, err := client.CheckRedundancy(context.Background(), "some content")
	if err != nil {
		t.Fatalf("checkRedundancy: %v", err)
	}

	if result["redundant"] != false {
		t.Errorf("got redundant=%v, want false", result["redundant"])
	}
}

func TestGenerateReport(t *testing.T) {
	d, sock := newFakeDaemon(t)
	d.setResponse("engram.generateReport", map[string]any{
		"report": "Session saved 488 tokens (97.6%)",
	})
	go d.serve()
	defer d.close()

	client, err := engram.Connect(context.Background(), sock)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer client.Close()

	result, err := client.GenerateReport(context.Background())
	if err != nil {
		t.Fatalf("generateReport: %v", err)
	}

	report, ok := result["report"].(string)
	if !ok || report == "" {
		t.Fatalf("expected string report, got %T", result["report"])
	}
}

func TestConnectMissingSocket(t *testing.T) {
	_, err := engram.Connect(context.Background(), "/tmp/nonexistent-engram-test.sock")
	if err == nil {
		t.Fatal("expected error for missing socket")
	}
}
