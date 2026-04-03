package daemon

import (
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// shortSock creates a short socket path to avoid macOS 104-char limit.
func shortSock(t *testing.T, name string) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "ed-")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(dir) })
	return filepath.Join(dir, name)
}

func TestListener_StartsAndAcceptsConnection(t *testing.T) {
	sockPath := shortSock(t, "t.sock")
	l, err := NewListener(sockPath)
	require.NoError(t, err)
	defer l.Close()

	accepted := make(chan net.Conn, 1)
	go func() {
		conn, err := l.Accept()
		if err == nil {
			accepted <- conn
		}
	}()

	conn, err := net.DialTimeout("unix", sockPath, 2*time.Second)
	require.NoError(t, err)
	defer conn.Close()

	select {
	case srv := <-accepted:
		assert.NotNil(t, srv)
		srv.Close()
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}
}

func TestListener_CreatesParentDirectory(t *testing.T) {
	base := shortSock(t, "sub")
	sockPath := filepath.Join(base, "t.sock")
	// Remove the "sub" file that shortSock didn't create as dir
	os.Remove(base)
	l, err := NewListener(sockPath)
	require.NoError(t, err)
	defer l.Close()
	_, err = os.Stat(filepath.Dir(sockPath))
	assert.NoError(t, err)
}

func TestListener_RemovesStaleSocket(t *testing.T) {
	sockPath := shortSock(t, "s.sock")
	require.NoError(t, os.WriteFile(sockPath, []byte("stale"), 0600))
	l, err := NewListener(sockPath)
	require.NoError(t, err)
	defer l.Close()
	conn, err := net.DialTimeout("unix", sockPath, time.Second)
	require.NoError(t, err)
	conn.Close()
}

func TestListener_SocketPath(t *testing.T) {
	sockPath := shortSock(t, "p.sock")
	l, err := NewListener(sockPath)
	require.NoError(t, err)
	defer l.Close()
	assert.Equal(t, sockPath, l.SocketPath())
}
