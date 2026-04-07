package daemon

import "encoding/json"

// RPCRequest is a JSON-RPC 2.0 request object.
type RPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	ID      interface{} `json:"id"`
}

// RPCResponse is a JSON-RPC 2.0 response object.
type RPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
	ID      interface{}     `json:"id"`
}

// RPCError represents a JSON-RPC 2.0 error.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// HealthResult is the response payload for the "health" method.
type HealthResult struct {
	Status         string `json:"status"`
	Uptime         string `json:"uptime,omitempty"`
	ActiveSessions int    `json:"active_sessions,omitempty"`
	TotalTurns     int    `json:"total_turns,omitempty"`
	TotalSaved     int    `json:"total_saved,omitempty"`
}

// StatsResult is the response payload for the "stats" method.
// All token counts are cumulative across all active sessions.
type StatsResult struct {
	ActiveSessions int `json:"active_sessions"`
	TotalTurns     int `json:"total_turns"`
	TokensSent     int `json:"tokens_sent"`  // tokens actually sent
	TokensSaved    int `json:"tokens_saved"` // identity tokens saved
	TotalSaved     int `json:"total_saved"`  // total tokens saved
}

// CompressRequest is the params payload for the "compress" method.
type CompressRequest struct {
	ClientID  string            `json:"client_id"`
	APIKey    string            `json:"api_key"`
	SessionID string            `json:"session_id,omitempty"`
	Query     string            `json:"query"`
	Identity  map[string]string `json:"identity,omitempty"`
	Provider  string            `json:"provider,omitempty"`
	Model     string            `json:"model,omitempty"`
}

// CompressResult is the response payload for the "compress" method.
type CompressResult struct {
	SessionID   string `json:"session_id"`
	FullText    string `json:"full_text"`
	TotalTokens int    `json:"total_tokens"`
}
