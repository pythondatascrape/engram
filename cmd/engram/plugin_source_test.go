package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeLayout creates <root>/<binDir>/engram and <root>/plugins/claude-code/server.mjs.
// binDir "bin" is the layout `make build` produces in a source checkout.
func makeLayout(t *testing.T, binDir string) (root, exe string) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Join(root, binDir), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "plugins", "claude-code"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "plugins", "claude-code", "server.mjs"), []byte("// server"), 0o644))
	exe = filepath.Join(root, binDir, "engram")
	require.NoError(t, os.WriteFile(exe, []byte("bin"), 0o755))
	return root, exe
}

func TestFindNearBinary(t *testing.T) {
	tests := []struct {
		name    string
		binDir  string
		symlink bool
	}{
		{"checkout layout", "bin", false},
		{"symlinked binary", "bin", true},
		{"plugins next to binary", ".", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root, exe := makeLayout(t, tt.binDir)
			if tt.symlink {
				link := filepath.Join(t.TempDir(), "engram")
				require.NoError(t, os.Symlink(exe, link))
				exe = link
			}
			got, err := findNearBinary(exe, "plugins/claude-code")
			require.NoError(t, err)
			assert.Equal(t, filepath.Join(root, "plugins", "claude-code"), got)
		})
	}
}

func TestFindNearBinary_NotFound(t *testing.T) {
	exe := filepath.Join(t.TempDir(), "engram")
	require.NoError(t, os.WriteFile(exe, []byte("bin"), 0o755))

	_, err := findNearBinary(exe, "plugins/claude-code")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "plugins/claude-code not found")
}

func TestMCPServerPathFor_CheckoutLayout(t *testing.T) {
	root, exe := makeLayout(t, "bin")
	got, err := mcpServerPathFor(exe)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(root, "plugins", "claude-code", "server.mjs"), got)
}
