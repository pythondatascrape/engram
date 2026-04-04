package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateLaunchdPlist(t *testing.T) {
	plist := generateLaunchdPlist("/usr/local/bin/engram", "/Users/test/.engram/engram.yaml", "/Users/test/.engram/engram.sock")
	assert.Contains(t, plist, "com.engram.daemon")
	assert.Contains(t, plist, "/usr/local/bin/engram")
	assert.Contains(t, plist, "<key>RunAtLoad</key>")
	assert.Contains(t, plist, "<true/>")
}

func TestGenerateSystemdUnit(t *testing.T) {
	unit := generateSystemdUnit("/usr/local/bin/engram", "/home/test/.engram/engram.yaml", "/home/test/.engram/engram.sock")
	assert.Contains(t, unit, "[Unit]")
	assert.Contains(t, unit, "[Service]")
	assert.Contains(t, unit, "[Install]")
	assert.Contains(t, unit, "/usr/local/bin/engram")
}

func TestWriteServiceFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.plist")
	err := writeServiceFile(path, "<plist>test</plist>")
	require.NoError(t, err)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "<plist>test</plist>", string(data))
}

func TestWriteServiceFile_CreatesParentDirs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "deep", "test.service")
	err := writeServiceFile(path, "[Unit]\nDescription=test")
	require.NoError(t, err)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), "[Unit]")
}

func TestInstallCmd_OpenClaw_InstallsPlugin(t *testing.T) {
	// Fake HOME so RegisterOpenClaw writes into a temp dir.
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	// Create a fake binary base dir with a minimal openclaw plugin source.
	fakeBase := t.TempDir()
	pluginSrc := filepath.Join(fakeBase, "plugins", "openclaw")
	require.NoError(t, os.MkdirAll(pluginSrc, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(pluginSrc, "adapter.go"), []byte("package openclaw"), 0o644))

	// Create fake ~/.openclaw/ so DetectOpenClaw succeeds.
	require.NoError(t, os.MkdirAll(filepath.Join(fakeHome, ".openclaw"), 0o755))

	rootCmd := newRootCmd()
	rootCmd.AddCommand(newInstallCmd())

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)

	rootCmd.SetArgs([]string{"install", "--openclaw", "--source", fakeBase})
	err := rootCmd.Execute()
	require.NoError(t, err)

	out := buf.String()
	require.NotContains(t, out, "not yet implemented")
	require.Contains(t, out, ".openclaw/plugins/engram/engram/")
}
