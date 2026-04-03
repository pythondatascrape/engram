// Package openclaw provides an engram gateway adapter for OpenClaw.
//
// This adapter intercepts LLM requests, compresses identity context via the
// engram daemon, and forwards the compressed request to the LLM provider.
package openclaw

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
)

// Adapter bridges OpenClaw to the engram daemon.
type Adapter struct {
	socketPath string
	conn       net.Conn
	mu         sync.Mutex
	nextID     int
}

// New creates a new OpenClaw adapter.
func New() *Adapter {
	home, _ := os.UserHomeDir()
	return &Adapter{
		socketPath: filepath.Join(home, ".engram", "engram.sock"),
		nextID:     1,
	}
}

// Connect establishes a connection to the engram daemon.
func (a *Adapter) Connect(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	var d net.Dialer
	conn, err := d.DialContext(ctx, "unix", a.socketPath)
	if err != nil {
		return fmt.Errorf("connect to engram daemon: %w", err)
	}
	a.conn = conn
	return nil
}

// CompressContext sends content to the daemon for compression.
func (a *Adapter) CompressContext(ctx context.Context, content string) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.conn == nil {
		return "", fmt.Errorf("not connected to engram daemon")
	}

	id := a.nextID
	a.nextID++

	req := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  "engram.compressIdentity",
		"params":  map[string]interface{}{"content": content},
	}

	data, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	if _, err := a.conn.Write(append(data, '\n')); err != nil {
		return "", fmt.Errorf("send request: %w", err)
	}

	buf := make([]byte, 64*1024)
	n, err := a.conn.Read(buf)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	var resp struct {
		Result struct {
			Compressed string `json:"compressed"`
		} `json:"result"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(buf[:n], &resp); err != nil {
		return "", fmt.Errorf("unmarshal response: %w", err)
	}

	if resp.Error != nil {
		return "", fmt.Errorf("daemon error: %s", resp.Error.Message)
	}

	return resp.Result.Compressed, nil
}

// Close disconnects from the daemon.
func (a *Adapter) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.conn != nil {
		err := a.conn.Close()
		a.conn = nil
		return err
	}
	return nil
}
