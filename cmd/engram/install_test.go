package main

import (
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
