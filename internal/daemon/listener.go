package daemon

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
)

// Listener wraps a Unix domain socket listener with lifecycle management.
type Listener struct {
	net.Listener
	socketPath string
}

// NewListener creates a Unix domain socket at socketPath, cleaning up any stale
// socket file that may exist from a previous run.
func NewListener(socketPath string) (*Listener, error) {
	dir := filepath.Dir(socketPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("daemon: create directory %q: %w", dir, err)
	}
	if _, err := os.Stat(socketPath); err == nil {
		if err := os.Remove(socketPath); err != nil {
			return nil, fmt.Errorf("daemon: remove stale socket %q: %w", socketPath, err)
		}
	}
	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("daemon: listen on %q: %w", socketPath, err)
	}
	if err := os.Chmod(socketPath, 0600); err != nil {
		ln.Close()
		return nil, fmt.Errorf("daemon: chmod %q: %w", socketPath, err)
	}
	return &Listener{Listener: ln, socketPath: socketPath}, nil
}

// Close stops the listener and removes the socket file.
func (l *Listener) Close() error {
	err := l.Listener.Close()
	os.Remove(l.socketPath)
	return err
}

// SocketPath returns the filesystem path of the Unix socket.
func (l *Listener) SocketPath() string { return l.socketPath }
