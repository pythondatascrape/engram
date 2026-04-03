package daemon

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/pythondatascrape/engram/internal/server"
	"github.com/pythondatascrape/engram/internal/session"
)

// Server is a JSON-RPC server that accepts connections on a Listener and
// dispatches method calls. The handler field may be nil for health-only mode.
type Server struct {
	listener *Listener
	handler  *server.Handler
	sessions *session.Manager
	wg       sync.WaitGroup
	ctx      context.Context
	cancel   context.CancelFunc
	started  time.Time
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
			Result:  health,
			ID:      req.ID,
		}
	case "compress":
		return s.handleCompress(req)
	default:
		return RPCResponse{
			JSONRPC: "2.0",
			Error:   &RPCError{Code: -32601, Message: fmt.Sprintf("method not found: %s", req.Method)},
			ID:      req.ID,
		}
	}
}

func (s *Server) handleCompress(req RPCRequest) RPCResponse {
	if s.handler == nil {
		return RPCResponse{
			JSONRPC: "2.0",
			Error:   &RPCError{Code: -32603, Message: "handler not configured"},
			ID:      req.ID,
		}
	}

	paramsBytes, err := json.Marshal(req.Params)
	if err != nil {
		return RPCResponse{
			JSONRPC: "2.0",
			Error:   &RPCError{Code: -32602, Message: "invalid params"},
			ID:      req.ID,
		}
	}

	var cr CompressRequest
	if err := json.Unmarshal(paramsBytes, &cr); err != nil {
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
		Result: CompressResult{
			SessionID:   resp.SessionID,
			FullText:    resp.FullText,
			TotalTokens: resp.TotalTokens,
		},
		ID: req.ID,
	}
}
