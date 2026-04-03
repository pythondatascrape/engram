package daemon

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDaemon_FullLifecycle(t *testing.T) {
	dir := t.TempDir()
	sockPath := filepath.Join(dir, "lifecycle.sock")

	ln, err := NewListener(sockPath)
	require.NoError(t, err)

	srv := NewServer(ln, nil)
	go srv.Serve()
	time.Sleep(50 * time.Millisecond)

	client, err := NewClient(sockPath)
	require.NoError(t, err)

	health, err := client.Health()
	require.NoError(t, err)
	assert.Equal(t, "ok", health.Status)

	health2, err := client.Health()
	require.NoError(t, err)
	assert.Equal(t, "ok", health2.Status)

	client.Close()
	srv.Stop()
	time.Sleep(100 * time.Millisecond)

	_, err = NewClient(sockPath)
	assert.Error(t, err)
}

func TestDaemon_MultipleClients(t *testing.T) {
	dir := t.TempDir()
	sockPath := filepath.Join(dir, "multi.sock")

	ln, err := NewListener(sockPath)
	require.NoError(t, err)

	srv := NewServer(ln, nil)
	go srv.Serve()
	defer srv.Stop()
	time.Sleep(50 * time.Millisecond)

	c1, err := NewClient(sockPath)
	require.NoError(t, err)
	defer c1.Close()

	c2, err := NewClient(sockPath)
	require.NoError(t, err)
	defer c2.Close()

	h1, err := c1.Health()
	require.NoError(t, err)
	assert.Equal(t, "ok", h1.Status)

	h2, err := c2.Health()
	require.NoError(t, err)
	assert.Equal(t, "ok", h2.Status)
}
