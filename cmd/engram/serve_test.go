package main

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServeCommand_Help(t *testing.T) {
	cmd := newServeCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--help"})
	err := cmd.Execute()
	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "daemon")
	assert.Contains(t, buf.String(), "--socket")
}

func TestServeCommand_DefaultSocketPath(t *testing.T) {
	cmd := newServeCmd()
	f := cmd.Flag("socket")
	assert.NotNil(t, f)
}

func TestRunServe_RemovesSocketWhenProxyFailsToStart(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// Hold the proxy port so the proxy fails after the socket is already bound.
	busy, err := net.Listen("tcp", ":0")
	require.NoError(t, err)
	defer busy.Close()
	port := busy.Addr().(*net.TCPAddr).Port

	cfgPath := filepath.Join(home, "engram.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte(fmt.Sprintf("proxy:\n  port: %d\n  openai_port: 0\n", port)), 0o600))

	sockDir, err := os.MkdirTemp("/tmp", "es-")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(sockDir) })
	sockPath := filepath.Join(sockDir, "e.sock")

	cmd := newServeCmd()
	cmd.SetArgs([]string{"--foreground", "--config", cfgPath, "--socket", sockPath})
	require.Error(t, cmd.Execute())

	_, err = os.Stat(sockPath)
	assert.True(t, errors.Is(err, os.ErrNotExist), "a failed start must not leave a dead socket behind")
}
