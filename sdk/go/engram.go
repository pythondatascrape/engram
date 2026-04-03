// Package engram provides a thin client for the Engram compression daemon.
//
// The client connects to the daemon's Unix socket (~/.engram/engram.sock)
// and sends JSON-RPC requests. All compression logic lives in the daemon.
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
	"sync/atomic"
)

const defaultSocket = ".engram/engram.sock"

var requestID atomic.Int64

// Client communicates with the Engram daemon over a Unix socket.
type Client struct {
	socketPath string
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

// Connect creates a new client connected to the Engram daemon.
// If socketPath is empty, it defaults to ~/.engram/engram.sock.
func Connect(_ context.Context, socketPath string) (*Client, error) {
	if socketPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("engram: cannot determine home directory: %w", err)
		}
		socketPath = filepath.Join(home, defaultSocket)
	}

	if _, err := os.Stat(socketPath); err != nil {
		return nil, fmt.Errorf("engram: daemon socket not found: %s", socketPath)
	}

	return &Client{socketPath: socketPath}, nil
}

func (c *Client) call(ctx context.Context, method string, params any) (map[string]any, error) {
	conn, err := net.Dial("unix", c.socketPath)
	if err != nil {
		return nil, fmt.Errorf("engram: connect failed: %w", err)
	}
	defer conn.Close()

	req := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      requestID.Add(1),
		Method:  method,
		Params:  params,
	}

	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return nil, fmt.Errorf("engram: write failed: %w", err)
	}

	var resp jsonRPCResponse
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
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

// Close disconnects from the daemon. Since each call opens a fresh connection,
// this is a no-op but satisfies the interface contract.
func (c *Client) Close() error {
	return nil
}
