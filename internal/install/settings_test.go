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

	sl, ok := got["statusLine"].(map[string]any)
	require.True(t, ok, "statusLine should be a map[string]any")
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

	env, ok := got["env"].(map[string]any)
	require.True(t, ok, "env should be a map[string]any")
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

	sl, ok := got["statusLine"].(map[string]any)
	require.True(t, ok, "statusLine should be a map[string]any")
	assert.Equal(t, "command", sl["type"])
	assert.Equal(t, "engram statusline", sl["command"])
}

func TestMergeProxySettings_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".claude", "settings.json")

	require.NoError(t, MergeProxySettings(path, 4242, 0))

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, json.Unmarshal(data, &got))

	env, ok := got["env"].(map[string]any)
	require.True(t, ok, "env should be a map[string]any")
	assert.Equal(t, "http://localhost:4242", env["ANTHROPIC_BASE_URL"])
	_, hasOpenAI := env["OPENAI_BASE_URL"]
	assert.False(t, hasOpenAI, "OPENAI_BASE_URL should not be set when openaiPort is disabled")

	rh, ok := got["requestHeaders"].(map[string]any)
	require.True(t, ok, "requestHeaders should be a map[string]any")
	assert.Equal(t, "${session_id}", rh["X-Engram-Session"])
}

func TestMergeProxySettings_PreservesExistingKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"env":{"FOO":"bar"},"requestHeaders":{"X-Other":"val"}}`), 0o644))

	require.NoError(t, MergeProxySettings(path, 4242, 0))

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, json.Unmarshal(data, &got))

	env, ok := got["env"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "bar", env["FOO"])
	assert.Equal(t, "http://localhost:4242", env["ANTHROPIC_BASE_URL"])

	rh, ok := got["requestHeaders"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "val", rh["X-Other"])
	assert.Equal(t, "${session_id}", rh["X-Engram-Session"])
}

func TestMergeProxySettings_UpdatesPort(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")

	require.NoError(t, MergeProxySettings(path, 4242, 0))
	require.NoError(t, MergeProxySettings(path, 9999, 0))

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, json.Unmarshal(data, &got))

	env, ok := got["env"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "http://localhost:9999", env["ANTHROPIC_BASE_URL"])
}

func TestMergeProxySettings_SetsOptionalOpenAIBaseURL(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")

	require.NoError(t, MergeProxySettings(path, 4242, 4243))

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, json.Unmarshal(data, &got))

	env, ok := got["env"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "http://localhost:4242", env["ANTHROPIC_BASE_URL"])
	assert.Equal(t, "http://localhost:4243", env["OPENAI_BASE_URL"])
}

func TestMergeClaudeSettings_ReturnsErrorForInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	require.NoError(t, os.WriteFile(path, []byte(`not json`), 0o644))

	err := MergeClaudeSettings(path, "engram statusline")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse")
}
