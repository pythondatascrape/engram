package main

import (
	"bytes"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pythondatascrape/engram/internal/config"
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

func TestGenerateLaunchdPlist_NonEmptySocket(t *testing.T) {
	plist := generateLaunchdPlist(
		"/usr/local/bin/engram",
		"/Users/test/.engram/engram.yaml",
		DefaultSocketPath(),
	)
	assert.NotContains(t, plist, `<string></string>`,
		"plist must not contain empty string arguments")
	assert.Contains(t, plist, ".engram/engram.sock")
}

func TestGenerateSystemdUnit_NonEmptySocket(t *testing.T) {
	unit := generateSystemdUnit(
		"/usr/local/bin/engram",
		"/home/test/.engram/engram.yaml",
		DefaultSocketPath(),
	)
	assert.NotContains(t, unit, "=\n",
		"unit must not contain empty ExecStart arguments")
	assert.Contains(t, unit, ".engram/engram.sock")
}

func TestInstallCmd_ClaudeCode_WritesStatusLine(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	fakeBase := t.TempDir()
	pluginSrc := filepath.Join(fakeBase, "plugins", "claude-code")
	require.NoError(t, os.MkdirAll(pluginSrc, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(pluginSrc, "server.mjs"), []byte("// plugin"), 0o644))

	// Create fake ~/.claude/ so DetectClaudeCode succeeds
	require.NoError(t, os.MkdirAll(filepath.Join(fakeHome, ".claude"), 0o755))

	rootCmd := newRootCmd()
	rootCmd.AddCommand(newInstallCmd())

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)

	rootCmd.SetArgs([]string{"install", "--claude-code", "--source", fakeBase})
	// Command will fail due to readiness check (no actual daemon), but that's ok for this test.
	// We're testing that statusline gets registered before the readiness check happens.
	rootCmd.Execute()

	settingsPath := filepath.Join(fakeHome, ".claude", "settings.json")
	data, err := os.ReadFile(settingsPath)
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, json.Unmarshal(data, &got))

	sl, ok := got["statusLine"].(map[string]any)
	require.True(t, ok, "statusLine should be a map[string]any")
	assert.Equal(t, "engram statusline", sl["command"])

	out := buf.String()
	assert.Contains(t, out, "settings.json")
}

func TestVerifyReadiness_FailsWhenSocketMissing(t *testing.T) {
	err := verifyReadiness("/nonexistent/engram.sock", 19999, 3*time.Second)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "socket")
}

func TestVerifyReadiness_FailsWhenPortNotListening(t *testing.T) {
	// Create a real socket so the socket check passes.
	sockPath := "/tmp/engram_test_sock_" + t.Name()
	t.Cleanup(func() {
		os.Remove(sockPath)
	})
	ln, err := net.Listen("unix", sockPath)
	require.NoError(t, err)
	defer ln.Close()

	err = verifyReadiness(sockPath, 19998, 500*time.Millisecond)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "proxy")
}

func TestVerifyReadiness_SucceedsWhenBothAvailable(t *testing.T) {
	sockPath := "/tmp/engram_test_sock_" + t.Name()
	t.Cleanup(func() {
		os.Remove(sockPath)
	})

	// Socket listener
	ln, err := net.Listen("unix", sockPath)
	require.NoError(t, err)
	defer ln.Close()

	// TCP listener on a free port
	tcpLn, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer tcpLn.Close()
	port := tcpLn.Addr().(*net.TCPAddr).Port

	err = verifyReadiness(sockPath, port, time.Second)
	require.NoError(t, err)
}

func TestInstallPluginForOS_UnsupportedOS(t *testing.T) {
	err := installPluginForOS("windows", "/bin/engram",
		"/tmp/engram.yaml",
		"/tmp/engram.sock")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported")
}

func TestInstallCmd_ClaudeCode_EndToEnd_ServicePathsValid(t *testing.T) {
	socketPath := DefaultSocketPath()
	configPath := DefaultConfigPath()
	binary := "/usr/local/bin/engram"

	plist := generateLaunchdPlist(binary, configPath, socketPath)
	assert.Contains(t, plist, binary, "plist must contain binary path")
	assert.Contains(t, plist, configPath, "plist must contain config path")
	assert.Contains(t, plist, socketPath, "plist must contain socket path")
	assert.NotContains(t, plist, "<string></string>", "plist must not have empty args")

	unit := generateSystemdUnit(binary, configPath, socketPath)
	assert.Contains(t, unit, binary, "unit must contain binary path")
	assert.Contains(t, unit, configPath, "unit must contain config path")
	assert.Contains(t, unit, socketPath, "unit must contain socket path")
}

func TestInstallCmd_ClaudeCode_EndToEnd_ConfigCreated(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	configPath := filepath.Join(fakeHome, ".engram", "engram.yaml")
	require.NoError(t, config.EnsureDefault(configPath))
	require.FileExists(t, configPath)

	_, err := config.Load(configPath)
	require.NoError(t, err)
}
