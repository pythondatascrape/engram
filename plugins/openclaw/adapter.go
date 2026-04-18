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

// CompressContext sends content to the daemon for compression.
// content may be prose or pre-structured key=value identity text.
func (a *Adapter) CompressContext(ctx context.Context, content string) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.conn == nil {
		return "", fmt.Errorf("not connected to engram daemon")
	}

	deriveID := a.nextID
	a.nextID++

	deriveReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      deriveID,
		"method":  "engram.deriveCodebook",
		"params":  map[string]interface{}{"content": content},
	}

	data, err := json.Marshal(deriveReq)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	if _, err := a.conn.Write(append(data, '\n')); err != nil {
		return "", fmt.Errorf("send request: %w", err)
	}

	var deriveResp struct {
		Result struct {
			Codebook map[string]string `json:"codebook"`
		} `json:"result"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := a.scanInto(&deriveResp); err != nil {
		return "", err
	}
	if deriveResp.Error != nil {
		return "", fmt.Errorf("daemon error: %s", deriveResp.Error.Message)
	}

	compressID := a.nextID
	a.nextID++
	compressReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      compressID,
		"method":  "engram.compressIdentity",
		"params": map[string]interface{}{
			"dimensions":     deriveResp.Result.Codebook,
			"originalTokens": estimateTokens(content),
		},
	}

	data, err = json.Marshal(compressReq)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	if _, err := a.conn.Write(append(data, '\n')); err != nil {
		return "", fmt.Errorf("send request: %w", err)
	}

	var resp struct {
		Result struct {
			Serialized string `json:"serialized"`
		} `json:"result"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := a.scanInto(&resp); err != nil {
		return "", err
	}

	if resp.Error != nil {
		return "", fmt.Errorf("daemon error: %s", resp.Error.Message)
	}

	if resp.Result.Serialized == "" {
		return "", fmt.Errorf("daemon response missing 'serialized' field")
	}

	return resp.Result.Serialized, nil
}

func (a *Adapter) scanInto(dst any) error {
	if !a.scanner.Scan() {
		if err := a.scanner.Err(); err != nil {
			return fmt.Errorf("read response: %w", err)
		}
		return fmt.Errorf("read response: connection closed")
	}
	if err := json.Unmarshal(a.scanner.Bytes(), dst); err != nil {
		return fmt.Errorf("unmarshal response: %w", err)
	}
	return nil
}

func estimateTokens(text string) int {
	tokens := len(strings.TrimSpace(text)) / 4
	if tokens < 1 {
		return 1
	}
	return tokens
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
