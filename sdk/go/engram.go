// Package engram provides a thin client for the Engram compression daemon.
//
// The client maintains a pool of persistent Unix socket connections,
// enabling concurrent RPC calls without blocking.
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

const (
	defaultSocket  = ".engram/engram.sock"
	defaultPoolSize = 4
)

var requestID atomic.Int64

// conn wraps a single persistent daemon connection.
type conn struct {
	raw net.Conn
	enc *json.Encoder
	dec *json.Decoder
}

// Client communicates with the Engram daemon using a pool of Unix socket connections.
type Client struct {
	socketPath string
	pool       chan *conn
	mu         sync.Mutex
	closed     bool
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

func dialConn(socketPath string) (*conn, error) {
	raw, err := net.Dial("unix", socketPath)
	if err != nil {
		return nil, err
	}
	return &conn{raw: raw, enc: json.NewEncoder(raw), dec: json.NewDecoder(raw)}, nil
}

// Connect dials the Engram daemon and pre-warms a connection pool.
// If socketPath is empty, it defaults to ~/.engram/engram.sock.
func Connect(_ context.Context, socketPath string) (*Client, error) {
	if socketPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("engram: cannot determine home directory: %w", err)
		}
		socketPath = filepath.Join(home, defaultSocket)
	}

	// Verify reachability with one connection, then seed the pool.
	first, err := dialConn(socketPath)
	if err != nil {
		return nil, fmt.Errorf("engram: daemon not reachable: %w", err)
	}

	pool := make(chan *conn, defaultPoolSize)
	pool <- first

	return &Client{socketPath: socketPath, pool: pool}, nil
}

func (c *Client) get() (*conn, error) {
	// Try to grab an idle connection from the pool.
	select {
	case cn := <-c.pool:
		return cn, nil
	default:
	}
	// Pool empty — dial a new one (up to channel capacity will be retained on put).
	return dialConn(c.socketPath)
}

func (c *Client) put(cn *conn) {
	// Return to pool if there's room, otherwise close the excess connection.
	select {
	case c.pool <- cn:
	default:
		cn.raw.Close()
	}
}

func (c *Client) discard(cn *conn) {
	cn.raw.Close()
}

func (c *Client) call(_ context.Context, method string, params any) (map[string]any, error) {
	cn, err := c.get()
	if err != nil {
		return nil, fmt.Errorf("engram: connect failed: %w", err)
	}

	req := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      requestID.Add(1),
		Method:  method,
		Params:  params,
	}

	if err := cn.enc.Encode(req); err != nil {
		c.discard(cn)
		return nil, fmt.Errorf("engram: write failed: %w", err)
	}

	var resp jsonRPCResponse
	if err := cn.dec.Decode(&resp); err != nil {
		c.discard(cn)
		return nil, fmt.Errorf("engram: read failed: %w", err)
	}

	c.put(cn)

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

// Close drains the pool and closes all connections.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	close(c.pool)
	for cn := range c.pool {
		cn.raw.Close()
	}
	return nil
}
