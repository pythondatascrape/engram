package daemon

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_Health(t *testing.T) {
	dir := t.TempDir()
	sockPath := filepath.Join(dir, "test.sock")

	listener, err := NewListener(sockPath)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	srv := NewServer(listener, nil)
	go srv.Serve()
	defer srv.Stop()
	time.Sleep(50 * time.Millisecond)

	client, err := NewClient(sockPath)
	require.NoError(t, err)
	defer client.Close()

	health, err := client.Health()
	require.NoError(t, err)
	assert.Equal(t, "ok", health.Status)
	_ = ctx
}

func TestClient_ConnectFails(t *testing.T) {
	_, err := NewClient("/nonexistent/path.sock")
	assert.Error(t, err)
}
