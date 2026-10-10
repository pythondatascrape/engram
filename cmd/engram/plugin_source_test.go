package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeCheckout builds <root>/bin/engram and <root>/plugins/claude-code, the
// layout produced by `make build` in a source checkout.
func makeCheckout(t *testing.T) (root, exe string) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Join(root, "bin"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "plugins", "claude-code"), 0o755))
	exe = filepath.Join(root, "bin", "engram")
	require.NoError(t, os.WriteFile(exe, []byte("bin"), 0o755))
	return root, exe
}

func TestFindPluginBase_CheckoutLayout(t *testing.T) {
	root, exe := makeCheckout(t)
	got, err := findPluginBase(exe, "plugins/claude-code")
	require.NoError(t, err)
	assert.Equal(t, root, got)
}

func TestFindPluginBase_FollowsSymlinkedBinary(t *testing.T) {
	root, exe := makeCheckout(t)
	link := filepath.Join(t.TempDir(), "engram")
	require.NoError(t, os.Symlink(exe, link))

	got, err := findPluginBase(link, "plugins/claude-code")
	require.NoError(t, err)
	assert.Equal(t, root, got)
}

func TestFindPluginBase_PluginsNextToBinary(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "plugins", "claude-code"), 0o755))
	exe := filepath.Join(dir, "engram")
	require.NoError(t, os.WriteFile(exe, []byte("bin"), 0o755))

	got, err := findPluginBase(exe, "plugins/claude-code")
	require.NoError(t, err)
	assert.Equal(t, dir, got)
}

func TestFindPluginBase_NotFound(t *testing.T) {
	exe := filepath.Join(t.TempDir(), "engram")
	require.NoError(t, os.WriteFile(exe, []byte("bin"), 0o755))

	_, err := findPluginBase(exe, "plugins/claude-code")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--source")
}

func TestResolveMCPServerPath_CheckoutLayout(t *testing.T) {
	root, _ := makeCheckout(t)
	server := filepath.Join(root, "plugins", "claude-code", "server.mjs")
	require.NoError(t, os.WriteFile(server, []byte("// server"), 0o644))
	t.Setenv("ENGRAM_PLUGIN_ROOT", "")

	got, err := mcpServerPathFor(filepath.Join(root, "bin", "engram"))
	require.NoError(t, err)
	assert.Equal(t, server, got)
}

func TestUpdateCmd_HasSourceFlag(t *testing.T) {
	assert.NotNil(t, newUpdateCmd().Flags().Lookup("source"))
}
