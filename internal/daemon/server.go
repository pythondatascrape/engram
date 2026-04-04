package daemon

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pythondatascrape/engram/internal/server"
	"github.com/pythondatascrape/engram/internal/session"
)

// engramStats tracks cumulative compression accounting across all calls.
type engramStats struct {
	totalOriginal   atomic.Int64
	totalCompressed atomic.Int64
	totalSaved      atomic.Int64
	totalCalls      atomic.Int64
}

func (s *engramStats) record(original, compressed int) {
	saved := original - compressed
	if saved < 0 {
		saved = 0
	}
	s.totalOriginal.Add(int64(original))
	s.totalCompressed.Add(int64(compressed))
	s.totalSaved.Add(int64(saved))
	s.totalCalls.Add(1)
}

func (s *engramStats) snapshot() map[string]int64 {
	return map[string]int64{
		"totalOriginalTokens":   s.totalOriginal.Load(),
		"totalCompressedTokens": s.totalCompressed.Load(),
		"totalSaved":            s.totalSaved.Load(),
		"totalCalls":            s.totalCalls.Load(),
	}
}

// Server is a JSON-RPC server that accepts connections on a Listener and
// dispatches method calls. The handler field may be nil for health-only mode.
type Server struct {
	listener     *Listener
	handler      *server.Handler
	sessions     *session.Manager
	engramStats  engramStats
	wg           sync.WaitGroup
	ctx          context.Context
	cancel       context.CancelFunc
	started      time.Time
}

// NewServer creates a JSON-RPC server. handler may be nil for health-only mode.
func NewServer(listener *Listener, handler *server.Handler) *Server {
	ctx, cancel := context.WithCancel(context.Background())
	return &Server{
		listener: listener,
		handler:  handler,
		ctx:      ctx,
		cancel:   cancel,
		started:  time.Now(),
	}
}

// NewServerWithSessions creates a JSON-RPC server with session tracking for health stats.
func NewServerWithSessions(listener *Listener, handler *server.Handler, sessions *session.Manager) *Server {
	s := NewServer(listener, handler)
	s.sessions = sessions
	return s
}

// Serve accepts connections in a loop until the server is stopped.
func (s *Server) Serve() error {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.ctx.Done():
				return nil
			default:
				slog.Error("daemon: accept error", "error", err)
				continue
			}
		}
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.handleConn(conn)
		}()
	}
}

// Stop gracefully shuts down the server.
func (s *Server) Stop() {
	s.cancel()
	s.listener.Close()
	s.wg.Wait()
}

func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close()
	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	encoder := json.NewEncoder(conn)

	for scanner.Scan() {
		var req RPCRequest
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			_ = encoder.Encode(RPCResponse{
				JSONRPC: "2.0",
				Error:   &RPCError{Code: -32700, Message: "parse error"},
				ID:      nil,
			})
			continue
		}

		resp := s.dispatch(req)
		_ = encoder.Encode(resp)
	}
}

func mustMarshal(v interface{}) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

func (s *Server) dispatch(req RPCRequest) RPCResponse {
	switch req.Method {
	case "health":
		health := HealthResult{
			Status: "ok",
			Uptime: time.Since(s.started).Truncate(time.Second).String(),
		}
		if s.sessions != nil {
			health.ActiveSessions = s.sessions.Count()
		}
		return RPCResponse{
			JSONRPC: "2.0",
			Result:  mustMarshal(health),
			ID:      req.ID,
		}
	case "stats":
		return s.handleStats(req)
	case "compress":
		return s.handleCompress(req)
	case "engram.deriveCodebook":
		return s.handleEngram(req, s.engramDeriveCodebook)
	case "engram.compressIdentity":
		return s.handleEngram(req, s.engramCompressIdentity)
	case "engram.checkRedundancy":
		return s.handleEngram(req, s.engramCheckRedundancy)
	case "engram.getStats":
		return s.handleEngram(req, s.engramGetStats)
	case "engram.generateReport":
		return s.handleEngram(req, s.engramGenerateReport)
	default:
		return RPCResponse{
			JSONRPC: "2.0",
			Error:   &RPCError{Code: -32601, Message: fmt.Sprintf("method not found: %s", req.Method)},
			ID:      req.ID,
		}
	}
}

func (s *Server) handleStats(req RPCRequest) RPCResponse {
	result := StatsResult{}
	if s.sessions != nil {
		agg := s.sessions.Stats()
		result.ActiveSessions = agg.ActiveSessions
		result.TotalTurns = agg.TotalTurns
		result.CumulativeBaseline = agg.CumulativeBaseline
		result.TokensSent = agg.TokensSent
		result.TokensSaved = agg.TokensSaved
		result.ContextTokensSaved = agg.ContextTokensSaved
		result.TotalSaved = agg.TokensSaved + agg.ContextTokensSaved
	}
	return RPCResponse{JSONRPC: "2.0", Result: mustMarshal(result), ID: req.ID}
}

func (s *Server) handleCompress(req RPCRequest) RPCResponse {
	if s.handler == nil {
		return RPCResponse{
			JSONRPC: "2.0",
			Error:   &RPCError{Code: -32603, Message: "handler not configured"},
			ID:      req.ID,
		}
	}

	var cr CompressRequest
	if err := json.Unmarshal(req.Params, &cr); err != nil {
		return RPCResponse{
			JSONRPC: "2.0",
			Error:   &RPCError{Code: -32602, Message: "invalid params: " + err.Error()},
			ID:      req.ID,
		}
	}

	incoming := server.IncomingRequest{
		ClientID:  cr.ClientID,
		APIKey:    cr.APIKey,
		SessionID: cr.SessionID,
		Query:     cr.Query,
		Identity:  cr.Identity,
		Opts: session.Opts{
			Provider: cr.Provider,
			Model:    cr.Model,
		},
	}

	resp, err := s.handler.HandleRequest(s.ctx, incoming)
	if err != nil {
		return RPCResponse{
			JSONRPC: "2.0",
			Error:   &RPCError{Code: -32000, Message: err.Error()},
			ID:      req.ID,
		}
	}

	return RPCResponse{
		JSONRPC: "2.0",
		Result: mustMarshal(CompressResult{
			SessionID:   resp.SessionID,
			FullText:    resp.FullText,
			TotalTokens: resp.TotalTokens,
		}),
		ID: req.ID,
	}
}

// engramHandler processes params and returns a result or error.
type engramHandler func(params json.RawMessage) (interface{}, error)

func (s *Server) handleEngram(req RPCRequest, fn engramHandler) RPCResponse {
	result, err := fn(req.Params)
	if err != nil {
		return RPCResponse{
			JSONRPC: "2.0",
			Error:   &RPCError{Code: -32000, Message: err.Error()},
			ID:      req.ID,
		}
	}
	return RPCResponse{
		JSONRPC: "2.0",
		Result:  mustMarshal(result),
		ID:      req.ID,
	}
}

func (s *Server) engramDeriveCodebook(params json.RawMessage) (interface{}, error) {
	var p struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	if p.Content == "" {
		return nil, fmt.Errorf("content is required")
	}
	// Stub — full implementation will integrate with codebook.Parse
	return map[string]interface{}{
		"status":  "derived",
		"message": "Codebook derivation via daemon not yet wired to compression pipeline",
	}, nil
}

func (s *Server) engramCompressIdentity(params json.RawMessage) (interface{}, error) {
	var p struct {
		Dimensions map[string]string `json:"dimensions"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	// Serialize dimensions to compact key=value format.
	original, _ := json.Marshal(p.Dimensions)
	var parts []string
	for k, v := range p.Dimensions {
		parts = append(parts, k+"="+v)
	}
	compressed := ""
	for i, part := range parts {
		if i > 0 {
			compressed += " "
		}
		compressed += part
	}

	origTokens := len(original) / 4
	compTokens := len(compressed) / 4
	s.engramStats.record(origTokens, compTokens)
	s.flushStats()

	return map[string]interface{}{
		"serialized": compressed,
		"stats": map[string]int{
			"originalTokens":   origTokens,
			"compressedTokens": compTokens,
			"saved":            origTokens - compTokens,
		},
	}, nil
}

func (s *Server) engramCheckRedundancy(params json.RawMessage) (interface{}, error) {
	var p struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	if p.Content == "" {
		return nil, fmt.Errorf("content is required")
	}
	return map[string]interface{}{
		"redundant": false,
		"message":   "Redundancy check not yet implemented in daemon mode",
	}, nil
}

func (s *Server) engramGetStats(params json.RawMessage) (interface{}, error) {
	snap := s.engramStats.snapshot()
	result := map[string]interface{}{
		"totalOriginalTokens":   snap["totalOriginalTokens"],
		"totalCompressedTokens": snap["totalCompressedTokens"],
		"totalSaved":            snap["totalSaved"],
		"totalCalls":            snap["totalCalls"],
	}
	if s.sessions != nil {
		result["activeSessions"] = s.sessions.Count()
	}
	s.flushStats()
	return result, nil
}

// flushStats writes current engram compression stats to ~/.engram/stats.json
// so external tools (e.g. statusline scripts) can read live data.
func (s *Server) flushStats() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	dir := filepath.Join(home, ".engram")
	_ = os.MkdirAll(dir, 0700)

	snap := s.engramStats.snapshot()
	data, err := json.Marshal(snap)
	if err != nil {
		return
	}

	tmp := filepath.Join(dir, ".stats.json.tmp")
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return
	}
	_ = os.Rename(tmp, filepath.Join(dir, "stats.json"))
}

func (s *Server) engramGenerateReport(params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{
		"report":  "# Engram Report\n\nDaemon mode report generation not yet fully implemented.",
		"summary": "No data available yet.",
	}, nil
}
