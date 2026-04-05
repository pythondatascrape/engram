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
	ln  net.Listener // retained after Start() so Addr() can return the bound address
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

// Start binds the TCP listener and starts serving in a goroutine.
// Returns an error if the port cannot be bound. After a successful Start,
// Addr() returns the effective bound address.
func (s *Server) Start() error {
	ln, err := net.Listen("tcp", s.srv.Addr)
	if err != nil {
		return fmt.Errorf("proxy listen %s: %w", s.srv.Addr, err)
	}
	s.ln = ln
	go s.srv.Serve(ln)
	return nil
}

// Stop gracefully shuts down the server with the given context.
func (s *Server) Stop(ctx context.Context) error {
	return s.srv.Shutdown(ctx)
}

// Addr returns the effective address the server is listening on after Start.
// Returns nil if Start has not been called or failed.
func (s *Server) Addr() net.Addr {
	if s.ln == nil {
		return nil
	}
	return s.ln.Addr()
}
