// Package engram provides a thin client for the Engram compression daemon.
//
// The client holds a persistent Unix socket connection and reuses it
// across calls, avoiding per-call connect/close overhead.
//
// Usage:
//
//	client, err := engram.Connect(ctx, "")  // empty string = default socket
//	if err != nil { ... }
//	defer client.Close()
//
//	result, err := client.Compress(ctx, map[string]any{...})
package engram

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
)

const defaultSocket = ".engram/engram.sock"

var requestID atomic.Int64

// Client communicates with the Engram daemon over a persistent Unix socket.
type Client struct {
	conn net.Conn
	enc  *json.Encoder
	dec  *json.Decoder
	mu   sync.Mutex // serializes concurrent calls on the shared connection
}

type jsonRPCRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int64  `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params"`
}

type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jsonRPCError   `json:"error,omitempty"`
}

type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Connect dials the Engram daemon and holds the connection open.
// If socketPath is empty, it defaults to ~/.engram/engram.sock.
func Connect(_ context.Context, socketPath string) (*Client, error) {
	if socketPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("engram: cannot determine home directory: %w", err)
		}
		socketPath = filepath.Join(home, defaultSocket)
	}

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("engram: daemon not reachable: %w", err)
	}

	return &Client{
		conn: conn,
		enc:  json.NewEncoder(conn),
		dec:  json.NewDecoder(conn),
	}, nil
}

func (c *Client) call(_ context.Context, method string, params any) (map[string]any, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	req := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      requestID.Add(1),
		Method:  method,
		Params:  params,
	}

	if err := c.enc.Encode(req); err != nil {
		return nil, fmt.Errorf("engram: write failed: %w", err)
	}

	var resp jsonRPCResponse
	if err := c.dec.Decode(&resp); err != nil {
		return nil, fmt.Errorf("engram: read failed: %w", err)
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("engram: daemon error (%d): %s", resp.Error.Code, resp.Error.Message)
	}

	var result map[string]any
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("engram: unmarshal result: %w", err)
	}

	return result, nil
}

// Compress sends context (identity + history + query) to the daemon for compression.
func (c *Client) Compress(ctx context.Context, compressCtx map[string]any) (map[string]any, error) {
	return c.call(ctx, "engram.compress", compressCtx)
}

// DeriveCodebook derives codebook dimensions from content.
func (c *Client) DeriveCodebook(ctx context.Context, content string) (map[string]any, error) {
	return c.call(ctx, "engram.deriveCodebook", map[string]string{"content": content})
}

// GetStats returns session statistics from the daemon.
func (c *Client) GetStats(ctx context.Context) (map[string]any, error) {
	return c.call(ctx, "engram.getStats", nil)
}

// CheckRedundancy checks for redundant patterns in content.
func (c *Client) CheckRedundancy(ctx context.Context, content string) (map[string]any, error) {
	return c.call(ctx, "engram.checkRedundancy", map[string]string{"content": content})
}

// GenerateReport produces a savings report.
func (c *Client) GenerateReport(ctx context.Context) (map[string]any, error) {
	return c.call(ctx, "engram.generateReport", nil)
}

// Close closes the underlying connection to the daemon.
func (c *Client) Close() error {
	return c.conn.Close()
}
