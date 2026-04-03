package optimizer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScanProject_DetectsGoProject(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/test\n\ngo 1.22\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("# CLAUDE.md\n\nThis is a test project with some identity content that should be detected.\n"), 0644))

	profile, err := ScanProject(dir)
	require.NoError(t, err)
	assert.Equal(t, ProjectTypeGo, profile.Type)
	assert.Len(t, profile.IdentityFiles, 1)
	assert.Equal(t, "CLAUDE.md", profile.IdentityFiles[0].Name)
	assert.Greater(t, profile.IdentityFiles[0].TokenCount, 0)
}

func TestScanProject_DetectsNodeProject(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name": "test"}`), 0644))

	profile, err := ScanProject(dir)
	require.NoError(t, err)
	assert.Equal(t, ProjectTypeNode, profile.Type)
}

func TestScanProject_DetectsPythonProject(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte("[project]\nname = \"test\"\n"), 0644))

	profile, err := ScanProject(dir)
	require.NoError(t, err)
	assert.Equal(t, ProjectTypePython, profile.Type)
}

func TestScanProject_FindsMultipleIdentityFiles(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("identity content here repeated many times for token counting\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("agent instructions\n"), 0644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".claude"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".claude", "CLAUDE.md"), []byte("nested identity\n"), 0644))

	profile, err := ScanProject(dir)
	require.NoError(t, err)
	assert.Len(t, profile.IdentityFiles, 3)
}

func TestScanProject_EmptyDirReturnsUnknown(t *testing.T) {
	dir := t.TempDir()

	profile, err := ScanProject(dir)
	require.NoError(t, err)
	assert.Equal(t, ProjectTypeUnknown, profile.Type)
	assert.Empty(t, profile.IdentityFiles)
}
