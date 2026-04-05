// cmd/engram/readiness.go
package main

import (
	"fmt"
	"net"
	"time"
)

// verifyReadiness checks that the daemon socket and proxy TCP port are reachable
// within the given timeout. Returns an error if either is unavailable.
func verifyReadiness(socketPath string, proxyPort int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	// Poll for the Unix socket.
	for {
		conn, err := net.DialTimeout("unix", socketPath, 100*time.Millisecond)
		if err == nil {
			conn.Close()
			break
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("readiness: socket %q not reachable after %s: %w", socketPath, timeout, err)
		}
		time.Sleep(200 * time.Millisecond)
	}

	// Poll for the proxy TCP port.
	addr := fmt.Sprintf("127.0.0.1:%d", proxyPort)
	for {
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			conn.Close()
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("readiness: proxy port %d not reachable after %s: %w", proxyPort, timeout, err)
		}
		time.Sleep(200 * time.Millisecond)
	}
}
