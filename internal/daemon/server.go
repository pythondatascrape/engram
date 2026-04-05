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
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pythondatascrape/engram/internal/identity/codebook"
	"github.com/pythondatascrape/engram/internal/redundancy"
	"github.com/pythondatascrape/engram/internal/server"
	"github.com/pythondatascrape/engram/internal/session"
)

// engramStatsSnapshot is the serialisable form of engramStats.
type engramStatsSnapshot struct {
	TotalOriginalTokens   int64 `json:"totalOriginalTokens"`
	TotalCompressedTokens int64 `json:"totalCompressedTokens"`
	TotalSaved            int64 `json:"totalSaved"`
	TotalCalls            int64 `json:"totalCalls"`
	TotalRedundancyHits   int64 `json:"totalRedundancyHits"`
}

// engramStats tracks cumulative compression accounting across all calls.
type engramStats struct {
	totalOriginal    atomic.Int64
	totalCompressed  atomic.Int64
	totalSaved       atomic.Int64
	totalCalls       atomic.Int64
	totalRedundancy  atomic.Int64
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

func (s *engramStats) snapshot() engramStatsSnapshot {
	return engramStatsSnapshot{
		TotalOriginalTokens:   s.totalOriginal.Load(),
		TotalCompressedTokens: s.totalCompressed.Load(),
		TotalSaved:            s.totalSaved.Load(),
		TotalCalls:            s.totalCalls.Load(),
		TotalRedundancyHits:   s.totalRedundancy.Load(),
	}
}

// Server is a JSON-RPC server that accepts connections on a Listener and
// dispatches method calls. The handler field may be nil for health-only mode.
type Server struct {
	listener         *Listener
	handler          *server.Handler
	sessions         *session.Manager
	engramStats      engramStats
	redundancyChecker *redundancy.Checker
	statsDir         string // cached once at construction: ~/.engram
	wg               sync.WaitGroup
	ctx              context.Context
	cancel           context.CancelFunc
	started          time.Time
}

// NewServer creates a JSON-RPC server. handler may be nil for health-only mode.
func NewServer(listener *Listener, handler *server.Handler) *Server {
	ctx, cancel := context.WithCancel(context.Background())
	home, _ := os.UserHomeDir()
	statsDir := filepath.Join(home, ".engram")
	_ = os.MkdirAll(statsDir, 0700)
	return &Server{
		listener:         listener,
		handler:          handler,
		statsDir:         statsDir,
		redundancyChecker: redundancy.NewChecker(0.9),
		ctx:              ctx,
		cancel:           cancel,
		started:          time.Now(),
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
	case "compress", "engram.compress":
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
	dims, serialized := codebook.Derive(p.Content)
	return map[string]interface{}{
		"status":     "derived",
		"codebook":   dims,
		"serialized": serialized,
	}, nil
}

func (s *Server) engramCompressIdentity(params json.RawMessage) (interface{}, error) {
	var p struct {
		Dimensions     map[string]string `json:"dimensions"`
		OriginalTokens int               `json:"originalTokens"` // token count of the prose identity being replaced
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	// Serialize dimensions to deterministic sorted key=value format.
	keys := make([]string, 0, len(p.Dimensions))
	for k := range p.Dimensions {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = k + "=" + p.Dimensions[k]
	}
	compressed := strings.Join(parts, " ")

	// byte/4 is a standard rough token approximation.
	compTokens := len(compressed) / 4

	// If the caller supplied the prose identity size, use that as the baseline so
	// the stats reflect the real savings (prose → codebook), not key=value → key=value.
	origTokens := p.OriginalTokens
	if origTokens <= 0 {
		origBytes := 0
		for k, v := range p.Dimensions {
			origBytes += len(k) + len(v) + 2 // rough: k=v + separator
		}
		origTokens = origBytes / 4
	}

	s.engramStats.record(origTokens, compTokens)
	s.flushStats()

	saved := origTokens - compTokens
	if saved < 0 {
		saved = 0
	}
	return map[string]interface{}{
		"serialized": compressed,
		"block":      "[identity]\n" + compressed + "\n[/identity]",
		"stats": map[string]int{
			"original":   origTokens,
			"compressed": compTokens,
			"saved":      saved,
			"ratio":      saved * 100 / max(origTokens, 1),
		},
	}, nil
}

func (s *Server) engramCheckRedundancy(params json.RawMessage) (interface{}, error) {
	var p struct {
		Content   string  `json:"content"`
		Threshold float64 `json:"threshold"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	if p.Content == "" {
		return nil, fmt.Errorf("content is required")
	}

	var result redundancy.Result
	if p.Threshold > 0 {
		result = s.redundancyChecker.CheckWithThreshold(p.Content, p.Threshold)
	} else {
		result = s.redundancyChecker.Check(p.Content)
	}
	s.redundancyChecker.Record(p.Content)
	if result.IsRedundant {
		s.engramStats.totalRedundancy.Add(1)
	}
	return result, nil
}

func (s *Server) engramGetStats(params json.RawMessage) (interface{}, error) {
	snap := s.engramStats.snapshot()
	if s.sessions != nil {
		return map[string]interface{}{
			"totalOriginalTokens":   snap.TotalOriginalTokens,
			"totalCompressedTokens": snap.TotalCompressedTokens,
			"totalSaved":            snap.TotalSaved,
			"totalCalls":            snap.TotalCalls,
			"activeSessions":        s.sessions.Count(),
		}, nil
	}
	return snap, nil
}

// flushStats atomically writes compression stats to ~/.engram/stats.json
// so external tools (statusline, etc.) can read live data without a socket.
// Only called after mutations — not on read paths.
func (s *Server) flushStats() {
	if s.statsDir == "" {
		return
	}
	data, err := json.Marshal(s.engramStats.snapshot())
	if err != nil {
		return
	}
	tmp := filepath.Join(s.statsDir, ".stats.json.tmp")
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return
	}
	_ = os.Rename(tmp, filepath.Join(s.statsDir, "stats.json"))
}

func (s *Server) engramGenerateReport(params json.RawMessage) (interface{}, error) {
	snap := s.engramStats.snapshot()

	var savingsPct float64
	if snap.TotalOriginalTokens > 0 {
		savingsPct = float64(snap.TotalOriginalTokens-snap.TotalCompressedTokens) / float64(snap.TotalOriginalTokens) * 100
	}

	md := fmt.Sprintf("# Engram Session Report\n\n"+
		"- Compression events: %d\n"+
		"- Tokens before: %d\n"+
		"- Tokens after: %d\n"+
		"- Tokens saved: %d\n"+
		"- Estimated savings: %.1f%%\n"+
		"- Redundancy hits: %d\n",
		snap.TotalCalls,
		snap.TotalOriginalTokens,
		snap.TotalCompressedTokens,
		snap.TotalSaved,
		savingsPct,
		snap.TotalRedundancyHits,
	)

	return map[string]interface{}{
		"compressionEvents":   snap.TotalCalls,
		"tokensBefore":        snap.TotalOriginalTokens,
		"tokensAfter":         snap.TotalCompressedTokens,
		"estimatedSavingsPct": savingsPct,
		"redundancyHits":      snap.TotalRedundancyHits,
		"markdown":            md,
	}, nil
}
