package install

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMergeClaudeSettings_CreatesFileWhenMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".claude", "settings.json")

	require.NoError(t, MergeClaudeSettings(path, "engram statusline"))

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, json.Unmarshal(data, &got))

	sl := got["statusLine"].(map[string]any)
	assert.Equal(t, "command", sl["type"])
	assert.Equal(t, "engram statusline", sl["command"])
}

func TestMergeClaudeSettings_PreservesExistingKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"env":{"FOO":"bar"}}`), 0o644))

	require.NoError(t, MergeClaudeSettings(path, "engram statusline"))

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, json.Unmarshal(data, &got))

	env := got["env"].(map[string]any)
	assert.Equal(t, "bar", env["FOO"])
	assert.NotNil(t, got["statusLine"])
}

func TestMergeClaudeSettings_OverwritesExistingStatusLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"statusLine":{"type":"command","command":"old-cmd"}}`), 0o644))

	require.NoError(t, MergeClaudeSettings(path, "engram statusline"))

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, json.Unmarshal(data, &got))

	sl := got["statusLine"].(map[string]any)
	assert.Equal(t, "engram statusline", sl["command"])
}

func TestMergeClaudeSettings_ReturnsErrorForInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	require.NoError(t, os.WriteFile(path, []byte(`not json`), 0o644))

	err := MergeClaudeSettings(path, "engram statusline")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse")
}
