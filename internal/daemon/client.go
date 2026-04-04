package daemon

import (
	"encoding/json"
	"fmt"
	"net"
	"time"
)

// Client connects to an Engram daemon over a Unix socket and issues JSON-RPC requests.
type Client struct {
	conn net.Conn
	enc  *json.Encoder
	dec  *json.Decoder
	id   int
}

// NewClient dials the daemon at socketPath and returns a ready Client.
func NewClient(socketPath string) (*Client, error) {
	conn, err := net.DialTimeout("unix", socketPath, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("daemon: connect to %q: %w", socketPath, err)
	}
	return &Client{conn: conn, enc: json.NewEncoder(conn), dec: json.NewDecoder(conn)}, nil
}

// Close closes the underlying connection.
func (c *Client) Close() error { return c.conn.Close() }

// Health sends a health RPC and returns the result.
func (c *Client) Health() (*HealthResult, error) {
	c.id++
	req := RPCRequest{JSONRPC: "2.0", Method: "health", ID: c.id}
	if err := c.enc.Encode(req); err != nil {
		return nil, fmt.Errorf("daemon: send health request: %w", err)
	}
	var resp RPCResponse
	if err := c.dec.Decode(&resp); err != nil {
		return nil, fmt.Errorf("daemon: read health response: %w", err)
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("daemon: health error %d: %s", resp.Error.Code, resp.Error.Message)
	}
	var health HealthResult
	if err := json.Unmarshal(resp.Result, &health); err != nil {
		return nil, fmt.Errorf("daemon: unmarshal health: %w", err)
	}
	return &health, nil
}

// Stats sends a stats RPC and returns cumulative token accounting.
func (c *Client) Stats() (*StatsResult, error) {
	c.id++
	req := RPCRequest{JSONRPC: "2.0", Method: "stats", ID: c.id}
	if err := c.enc.Encode(req); err != nil {
		return nil, fmt.Errorf("daemon: send stats request: %w", err)
	}
	var resp RPCResponse
	if err := c.dec.Decode(&resp); err != nil {
		return nil, fmt.Errorf("daemon: read stats response: %w", err)
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("daemon: stats error %d: %s", resp.Error.Code, resp.Error.Message)
	}
	var result StatsResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("daemon: unmarshal stats: %w", err)
	}
	return &result, nil
}

// Compress sends a compress RPC and returns the result.
func (c *Client) Compress(req *CompressRequest) (*CompressResult, error) {
	c.id++
	params, _ := json.Marshal(req)
	rpcReq := RPCRequest{JSONRPC: "2.0", Method: "compress", Params: params, ID: c.id}
	if err := c.enc.Encode(rpcReq); err != nil {
		return nil, fmt.Errorf("daemon: send compress request: %w", err)
	}
	var resp RPCResponse
	if err := c.dec.Decode(&resp); err != nil {
		return nil, fmt.Errorf("daemon: read compress response: %w", err)
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("daemon: compress error %d: %s", resp.Error.Code, resp.Error.Message)
	}
	var result CompressResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("daemon: unmarshal compress result: %w", err)
	}
	return &result, nil
}
