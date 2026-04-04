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

func TestRegisterOpenClaw_CopiesFilesToOpenClawPluginsDir(t *testing.T) {
	src := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(src, "adapter.go"), []byte("package openclaw"), 0o644))

	target := t.TempDir()
	t.Setenv("HOME", target)

	err := RegisterOpenClaw(src, "0.2.0")
	require.NoError(t, err)

	installed := filepath.Join(target, ".openclaw", "plugins", "engram", "engram", "0.2.0", "adapter.go")
	data, err := os.ReadFile(installed)
	require.NoError(t, err)
	require.Equal(t, "package openclaw", string(data))
}

func TestRegisterOpenClaw_RemovesPreviousInstallation(t *testing.T) {
	src := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(src, "new.go"), []byte("new"), 0o644))

	target := t.TempDir()
	t.Setenv("HOME", target)

	targetDir := filepath.Join(target, ".openclaw", "plugins", "engram", "engram", "0.2.0")
	require.NoError(t, os.MkdirAll(targetDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(targetDir, "old.go"), []byte("old"), 0o644))

	require.NoError(t, RegisterOpenClaw(src, "0.2.0"))

	_, err := os.Stat(filepath.Join(targetDir, "old.go"))
	require.True(t, os.IsNotExist(err), "old file should be removed")

	data, err := os.ReadFile(filepath.Join(targetDir, "new.go"))
	require.NoError(t, err)
	require.Equal(t, "new", string(data))
}

