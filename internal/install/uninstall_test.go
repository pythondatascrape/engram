package install

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func readSettings(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var got map[string]any
	require.NoError(t, json.Unmarshal(data, &got))
	return got
}

func TestUnregisterClaudeSettings_RoundTripRestoresOriginal(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	original := `{"theme":"dark","env":{"FOO":"bar"},"hooks":{"Stop":[{"hooks":[{"type":"command","command":"other"}]}]}}`
	require.NoError(t, os.WriteFile(path, []byte(original), 0o644))

	require.NoError(t, MergeClaudeSettings(path, "engram statusline"))
	require.NoError(t, MergeProxySettings(path, 4242, 4243))

	changed, err := UnregisterClaudeSettings(path, 4242, 4243)
	require.NoError(t, err)
	assert.True(t, changed)

	var want map[string]any
	require.NoError(t, json.Unmarshal([]byte(original), &want))
	assert.Equal(t, want, readSettings(t, path))
}

func TestUnregisterClaudeSettings_DropsEmptyContainers(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"theme":"dark"}`), 0o644))

	require.NoError(t, MergeClaudeSettings(path, "engram statusline"))
	require.NoError(t, MergeProxySettings(path, 4242, 0))

	_, err := UnregisterClaudeSettings(path, 4242)
	require.NoError(t, err)

	assert.Equal(t, map[string]any{"theme": "dark"}, readSettings(t, path))
}

func TestUnregisterClaudeSettings_KeepsForeignEntries(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	foreign := `{"statusLine":{"type":"command","command":"my-statusline"},"env":{"ANTHROPIC_BASE_URL":"https://gateway.example.com"}}`
	require.NoError(t, os.WriteFile(path, []byte(foreign), 0o644))

	changed, err := UnregisterClaudeSettings(path, 4242)
	require.NoError(t, err)
	assert.False(t, changed)

	var want map[string]any
	require.NoError(t, json.Unmarshal([]byte(foreign), &want))
	assert.Equal(t, want, readSettings(t, path))
}

func TestUnregisterClaudeSettings_MissingFileIsNoop(t *testing.T) {
	changed, err := UnregisterClaudeSettings(filepath.Join(t.TempDir(), "settings.json"), 4242)
	require.NoError(t, err)
	assert.False(t, changed)
}

func TestUnregisterClaudeCode_RemovesPluginDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	src := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(src, "plugin.json"), []byte(`{}`), 0o644))
	require.NoError(t, RegisterClaudeCode(src, "0.0.1"))

	require.NoError(t, UnregisterClaudeCode())

	_, err := os.Stat(filepath.Join(home, ".claude", "plugins", "cache", "engram"))
	assert.True(t, os.IsNotExist(err))
	_, err = os.Stat(filepath.Join(home, ".claude", "plugins", "cache"))
	assert.NoError(t, err, "parent cache dir must be left alone")
}
