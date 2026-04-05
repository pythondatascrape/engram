// internal/proxy/proxy.go
package proxy

import (
	"context"
	"fmt"
	"net"
	"net/http"
)

// Server wraps the HTTP proxy with lifecycle management.
type Server struct {
	srv *http.Server
}

// New creates a Server from config values.
// sessionsDir is typically ~/.engram/sessions.
// upstream is the real Anthropic API base URL.
func New(port, windowSize int, sessionsDir, upstream string) *Server {
	handler := NewHandler(windowSize, sessionsDir, upstream)
	return &Server{
		srv: &http.Server{
			Addr:    fmt.Sprintf(":%d", port),
			Handler: handler,
		},
	}
}

// Start starts the HTTP server in a goroutine. Returns immediately.
// Returns error if the listener cannot be bound.
func (s *Server) Start() error {
	ln, err := net.Listen("tcp", s.srv.Addr)
	if err != nil {
		return fmt.Errorf("proxy listen %s: %w", s.srv.Addr, err)
	}
	go s.srv.Serve(ln)
	return nil
}

// Stop gracefully shuts down the server with the given context.
func (s *Server) Stop(ctx context.Context) error {
	return s.srv.Shutdown(ctx)
}
