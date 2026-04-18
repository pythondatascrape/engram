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
	srv       *http.Server
	openaiSrv *http.Server
	ln        net.Listener // retained after Start() so Addr() can return the bound address
	openaiLn  net.Listener
}

// New creates a Server from config values.
// sessionsDir is typically ~/.engram/sessions.
// upstream is the real Anthropic API base URL.
// openaiUpstream is the OpenAI API base URL (e.g. https://api.openai.com).
// If openaiPort <= 0, the OpenAI listener is disabled.
func New(port, windowSize int, sessionsDir, upstream string, openaiPort int, openaiUpstream string) *Server {
	handler := NewHandler(windowSize, sessionsDir, upstream, openaiUpstream)
	s := &Server{
		srv: &http.Server{
			Addr:    fmt.Sprintf(":%d", port),
			Handler: handler,
		},
	}
	if openaiPort > 0 && openaiUpstream != "" {
		s.openaiSrv = &http.Server{
			Addr:    fmt.Sprintf(":%d", openaiPort),
			Handler: http.HandlerFunc(handler.ServeOpenAI),
		}
	}
	return s
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

	if s.openaiSrv != nil {
		oln, err := net.Listen("tcp", s.openaiSrv.Addr)
		if err != nil {
			return fmt.Errorf("openai proxy listen %s: %w", s.openaiSrv.Addr, err)
		}
		s.openaiLn = oln
		go s.openaiSrv.Serve(oln)
	}
	return nil
}

// Stop gracefully shuts down the server with the given context.
func (s *Server) Stop(ctx context.Context) error {
	err := s.srv.Shutdown(ctx)
	if s.openaiSrv != nil {
		if e2 := s.openaiSrv.Shutdown(ctx); e2 != nil && err == nil {
			err = e2
		}
	}
	return err
}

// Addr returns the effective address the server is listening on after Start.
// Returns nil if Start has not been called or failed.
func (s *Server) Addr() net.Addr {
	if s.ln == nil {
		return nil
	}
	return s.ln.Addr()
}

// OpenAIAddr returns the effective address of the OpenAI listener after Start.
func (s *Server) OpenAIAddr() net.Addr {
	if s.openaiLn == nil {
		return nil
	}
	return s.openaiLn.Addr()
}
