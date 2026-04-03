package install

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDetectClaudeCode(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home directory")
	}

	claudeDir := filepath.Join(home, ".claude")
	_, dirExists := os.Stat(claudeDir)

	client, found := DetectClaudeCode()

	if dirExists == nil {
		assert.True(t, found)
		assert.Equal(t, "claude-code", client.Name)
		assert.Equal(t, claudeDir, client.Dir)
	} else {
		assert.False(t, found)
	}
}

func TestPluginSourceDir(t *testing.T) {
	tests := []struct {
		name    string
		client  string
		want    string
		wantErr bool
	}{
		{"claude-code", "claude-code", "plugins/claude-code", false},
		{"openclaw", "openclaw", "plugins/openclaw", false},
		{"unknown", "unknown-client", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := PluginSourceDir(tt.client)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}
