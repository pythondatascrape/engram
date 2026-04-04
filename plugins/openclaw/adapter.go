// Package openclaw provides an engram gateway adapter for OpenClaw.
//
// This adapter intercepts LLM requests, compresses identity context via the
// engram daemon, and forwards the compressed request to the LLM provider.
package openclaw

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Adapter bridges OpenClaw to the engram daemon.
type Adapter struct {
	socketPath string
	conn       net.Conn
	scanner    *bufio.Scanner
	mu         sync.Mutex
	nextID     int
}

// New creates a new OpenClaw adapter. Returns an error if the home directory
// cannot be determined.
func New() (*Adapter, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot determine home directory: %w", err)
	}
	return &Adapter{
		socketPath: filepath.Join(home, ".engram", "engram.sock"),
		nextID:     1,
	}, nil
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
	a.scanner = bufio.NewScanner(conn)
	a.scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	return nil
}

// parseDimensions splits a space-separated string of key=value pairs into a
// map[string]string. Entries without an '=' are ignored.
func parseDimensions(s string) map[string]string {
	dims := make(map[string]string)
	for _, part := range strings.Fields(s) {
		k, v, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		dims[k] = v
	}
	return dims
}

// CompressContext sends content to the daemon for compression.
// content must be a space-separated list of key=value pairs.
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
		"params":  map[string]interface{}{"dimensions": parseDimensions(content)},
	}

	data, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	if _, err := a.conn.Write(append(data, '\n')); err != nil {
		return "", fmt.Errorf("send request: %w", err)
	}

	if !a.scanner.Scan() {
		if err := a.scanner.Err(); err != nil {
			return "", fmt.Errorf("read response: %w", err)
		}
		return "", fmt.Errorf("read response: connection closed")
	}

	var resp struct {
		Result struct {
			Serialized string `json:"serialized"`
		} `json:"result"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(a.scanner.Bytes(), &resp); err != nil {
		return "", fmt.Errorf("unmarshal response: %w", err)
	}

	if resp.Error != nil {
		return "", fmt.Errorf("daemon error: %s", resp.Error.Message)
	}

	if resp.Result.Serialized == "" {
		return "", fmt.Errorf("daemon response missing 'serialized' field")
	}

	return resp.Result.Serialized, nil
}

// Close disconnects from the daemon.
func (a *Adapter) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.conn != nil {
		err := a.conn.Close()
		a.conn = nil
		a.scanner = nil
		return err
	}
	return nil
}
