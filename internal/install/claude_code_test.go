package install

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCopyDir(t *testing.T) {
	src := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(src, "server.mjs"), []byte("// server"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(src, "lib"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(src, "lib", "client.mjs"), []byte("// client"), 0o644))

	dst := filepath.Join(t.TempDir(), "target")
	err := copyDir(src, dst)
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(dst, "server.mjs"))
	require.NoError(t, err)
	assert.Equal(t, "// server", string(data))

	data, err = os.ReadFile(filepath.Join(dst, "lib", "client.mjs"))
	require.NoError(t, err)
	assert.Equal(t, "// client", string(data))
}

func TestCopyDir_SkipsNodeModules(t *testing.T) {
	src := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(src, "index.js"), []byte("// main"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(src, "node_modules", "dep"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(src, "node_modules", "dep", "index.js"), []byte("// dep"), 0o644))

	dst := filepath.Join(t.TempDir(), "target")
	require.NoError(t, copyDir(src, dst))

	_, err := os.Stat(filepath.Join(dst, "node_modules"))
	assert.True(t, os.IsNotExist(err), "node_modules should be skipped")

	data, err := os.ReadFile(filepath.Join(dst, "index.js"))
	require.NoError(t, err)
	assert.Equal(t, "// main", string(data))
}

