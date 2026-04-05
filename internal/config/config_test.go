package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pythondatascrape/engram/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeTempYAML(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "engram.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))
	return path
}

func TestLoad_MinimalYAML_AppliesDefaults(t *testing.T) {
	path := writeTempYAML(t, `
server:
  port: 9090
`)
	cfg, err := config.Load(path)
	require.NoError(t, err)

	// Overridden value
	assert.Equal(t, 9090, cfg.Server.Port)

	// Server defaults
	assert.Equal(t, "0.0.0.0", cfg.Server.Host)

	// Auth defaults
	assert.Equal(t, 60*time.Minute, cfg.Auth.JWTExpiry)
	assert.Equal(t, 24*time.Hour, cfg.Auth.JWTMaxExpiry)

	// Session defaults
	assert.Equal(t, 15*time.Minute, cfg.Sessions.IdleTimeout)
	assert.Equal(t, 4*time.Hour, cfg.Sessions.MaxTTL)
	assert.Equal(t, 60*time.Second, cfg.Sessions.EvictionSweepInterval)
	assert.Equal(t, 50000, cfg.Sessions.MaxSessions)

	// Resource defaults
	assert.Equal(t, "512MB", cfg.Resources.MemoryCeiling)
	assert.InDelta(t, 0.80, cfg.Resources.MemoryPressureHigh, 0.001)
	assert.InDelta(t, 0.70, cfg.Resources.MemoryPressureLow, 0.001)
	assert.Equal(t, 100000, cfg.Resources.MaxGoroutines)

	// Pool defaults
	assert.Equal(t, 50, cfg.Pools.DefaultMaxConnections)
	assert.Equal(t, 2*time.Minute, cfg.Pools.IdleTimeout)
	assert.Equal(t, 5*time.Minute, cfg.Pools.DrainTimeout)

	// Codebook defaults
	assert.Equal(t, "./codebooks", cfg.Codebooks.Directory)
	assert.Equal(t, true, cfg.Codebooks.Watch)
	assert.Equal(t, 50, cfg.Codebooks.MaxDimensions)
	assert.Equal(t, 20, cfg.Codebooks.MaxEnumValues)

	// Knowledge defaults
	assert.Equal(t, 10, cfg.Knowledge.MaxRefsPerIdentity)
	assert.Equal(t, 2000, cfg.Knowledge.MaxTokensPerRef)
	assert.Equal(t, 5*time.Second, cfg.Knowledge.ResolutionTimeout)

	// Security defaults
	assert.Equal(t, true, cfg.Security.InjectionDetection.Enabled)
	assert.Equal(t, "strict", cfg.Security.InjectionDetection.Mode)
	assert.Equal(t, true, cfg.Security.RateLimiting.Enabled)
	assert.Equal(t, 60, cfg.Security.RateLimiting.RPM)
	assert.Equal(t, 10, cfg.Security.RateLimiting.Burst)
	assert.Equal(t, true, cfg.Security.ResponseFiltering.Enabled)
	assert.Equal(t, "strict", cfg.Security.ResponseFiltering.Mode)

	// Plugin defaults
	assert.Equal(t, 50*time.Millisecond, cfg.Plugins.HookTimeout)
	assert.Len(t, cfg.Plugins.BuiltIn, 7)

	// Shutdown defaults
	assert.Equal(t, 30*time.Second, cfg.Shutdown.GracePeriod)
	assert.Equal(t, true, cfg.Shutdown.NotifyClients)
	assert.Equal(t, 60*time.Second, cfg.Shutdown.ForceKillTimeout)

	// Logging defaults
	assert.Equal(t, "info", cfg.Logging.Level)
	assert.Equal(t, "json", cfg.Logging.Format)
	assert.Equal(t, "stdout", cfg.Logging.Output)
	assert.Equal(t, true, cfg.Logging.RedactIdentity)
	assert.Equal(t, true, cfg.Logging.RedactQueries)
}

func TestLoad_EnvVarOverrides(t *testing.T) {
	path := writeTempYAML(t, `server:
  port: 4433
`)
	t.Setenv("ENGRAM_SERVER_PORT", "7777")
	t.Setenv("ENGRAM_SERVER_HOST", "127.0.0.1")
	t.Setenv("ENGRAM_LOGGING_LEVEL", "debug")

	cfg, err := config.Load(path)
	require.NoError(t, err)

	assert.Equal(t, 7777, cfg.Server.Port)
	assert.Equal(t, "127.0.0.1", cfg.Server.Host)
	assert.Equal(t, "debug", cfg.Logging.Level)
}

func TestLoad_MissingFile_ReturnsError(t *testing.T) {
	_, err := config.Load("/nonexistent/path/engram.yaml")
	assert.Error(t, err)
}

func TestLoad_MemoryCeilingBelowMinimum_ReturnsError(t *testing.T) {
	path := writeTempYAML(t, `
resources:
  memory_ceiling: "64MB"
`)
	_, err := config.Load(path)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "memory_ceiling")
}

func TestLoad_InvalidPort_ReturnsError(t *testing.T) {
	path := writeTempYAML(t, `
server:
  port: 99999
`)
	_, err := config.Load(path)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "port")
}

func TestLoad_InvalidYAML_ReturnsError(t *testing.T) {
	path := writeTempYAML(t, `
server:
  port: [[[this is not valid yaml
`)
	_, err := config.Load(path)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parse error")
}

func TestDefaults_ProxyConfig(t *testing.T) {
	path := writeTempYAML(t, `
server:
  port: 9090
`)
	cfg, err := config.Load(path)
	require.NoError(t, err)
	assert.Equal(t, 4242, cfg.Proxy.Port, "default proxy port")
	assert.Equal(t, 10, cfg.Proxy.WindowSize, "default window size")
}

func TestEnsureDefault_CreatesWhenMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "engram.yaml")

	err := config.EnsureDefault(path)
	require.NoError(t, err)
	require.FileExists(t, path)
}

func TestEnsureDefault_LoadsSuccessfully(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "engram.yaml")

	require.NoError(t, config.EnsureDefault(path))
	_, err := config.Load(path)
	require.NoError(t, err)
}

func TestEnsureDefault_PreservesExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "engram.yaml")

	original := "# custom config\nserver:\n  port: 9999\n"
	require.NoError(t, os.WriteFile(path, []byte(original), 0644))

	require.NoError(t, config.EnsureDefault(path))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, original, string(data))
}

func TestParseSize(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
		wantErr  bool
	}{
		{"512MB", 512 * 1024 * 1024, false},
		{"4KB", 4 * 1024, false},
		{"2GB", 2 * 1024 * 1024 * 1024, false},
		{"1024B", 1024, false},
		{"128MB", 128 * 1024 * 1024, false},
		{"invalid", 0, true},
		{"", 0, true},
		{"abcMB", 0, true},
		{"  4KB  ", 4 * 1024, false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := config.ParseSize(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, got)
			}
		})
	}
}
