// Package config loads, validates, and provides access to Engram's runtime configuration.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config is the root configuration object threaded through all components.
type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Auth      AuthConfig      `yaml:"auth"`
	Sessions  SessionConfig   `yaml:"sessions"`
	Resources ResourceConfig  `yaml:"resources"`
	Pools     PoolConfig      `yaml:"pools"`
	Codebooks CodebookConfig  `yaml:"codebooks"`
	Knowledge KnowledgeConfig `yaml:"knowledge"`
	Security  SecurityConfig  `yaml:"security"`
	Plugins   PluginConfig    `yaml:"plugins"`
	Shutdown  ShutdownConfig  `yaml:"shutdown"`
	Logging   LoggingConfig   `yaml:"logging"`
	Proxy     ProxyConfig     `yaml:"proxy"`
}

type TLSConfig struct {
	Enabled  bool   `yaml:"enabled"`
	CertFile string `yaml:"cert_file"`
	KeyFile  string `yaml:"key_file"`
}

type AdminConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type ServerConfig struct {
	Host  string      `yaml:"host"`
	Port  int         `yaml:"port"`
	TLS   TLSConfig   `yaml:"tls"`
	Admin AdminConfig `yaml:"admin"`
}

type RegistrationConfig struct {
	Enabled bool `yaml:"enabled"`
}

type SecretRotationConfig struct {
	Enabled  bool          `yaml:"enabled"`
	Interval time.Duration `yaml:"interval"`
}

type AuthConfig struct {
	SigningKeyFile  string               `yaml:"signing_key_file"`
	JWTExpiry      time.Duration        `yaml:"jwt_expiry"`
	JWTMaxExpiry   time.Duration        `yaml:"jwt_max_expiry"`
	Registration   RegistrationConfig   `yaml:"registration"`
	SecretRotation SecretRotationConfig `yaml:"secret_rotation"`
}

type SessionConfig struct {
	IdleTimeout           time.Duration `yaml:"idle_timeout"`
	MaxTTL                time.Duration `yaml:"max_ttl"`
	EvictionSweepInterval time.Duration `yaml:"eviction_sweep_interval"`
	MaxSessions           int           `yaml:"max_sessions"`
}

type ResourceConfig struct {
	MemoryCeiling      string  `yaml:"memory_ceiling"`
	MemoryPressureHigh float64 `yaml:"memory_pressure_high"`
	MemoryPressureLow  float64 `yaml:"memory_pressure_low"`
	MaxGoroutines      int     `yaml:"max_goroutines"`
	MaxRequestSize     string  `yaml:"max_request_size"`
	MaxIdentitySize    string  `yaml:"max_identity_size"`
	MaxQuerySize       string  `yaml:"max_query_size"`
}

type PoolConfig struct {
	DefaultMaxConnections int           `yaml:"default_max_connections"`
	IdleTimeout           time.Duration `yaml:"idle_timeout"`
	DrainTimeout          time.Duration `yaml:"drain_timeout"`
	ScaleUpQueueDepth     int           `yaml:"scale_up_queue_depth"`
	HealthcheckInterval   time.Duration `yaml:"healthcheck_interval"`
	HealthcheckTimeout    time.Duration `yaml:"healthcheck_timeout"`
}

type CodebookConfig struct {
	Directory       string `yaml:"directory"`
	DefaultCodebook string `yaml:"default_codebook"`
	Watch           bool   `yaml:"watch"`
	MaxDimensions   int    `yaml:"max_dimensions"`
	MaxEnumValues   int    `yaml:"max_enum_values"`
}

type KnowledgeConfig struct {
	MaxRefsPerIdentity int           `yaml:"max_refs_per_identity"`
	MaxTokensPerRef    int           `yaml:"max_tokens_per_ref"`
	ResolutionTimeout  time.Duration `yaml:"resolution_timeout"`
	CacheTTL           time.Duration `yaml:"cache_ttl"`
}

type InjectionDetectionConfig struct {
	Enabled bool   `yaml:"enabled"`
	Mode    string `yaml:"mode"`
}

type RateLimitingConfig struct {
	Enabled        bool `yaml:"enabled"`
	RPM            int  `yaml:"rpm"`
	Burst          int  `yaml:"burst"`
	AlertThreshold int  `yaml:"alert_threshold"`
}

type ResponseFilteringConfig struct {
	Enabled                  bool   `yaml:"enabled"`
	Mode                     string `yaml:"mode"`
	CheckIdentityLeakage     bool   `yaml:"check_identity_leakage"`
	CheckSystemPromptExposure bool  `yaml:"check_system_prompt_exposure"`
}

type SecurityConfig struct {
	InjectionDetection InjectionDetectionConfig `yaml:"injection_detection"`
	RateLimiting       RateLimitingConfig       `yaml:"rate_limiting"`
	ResponseFiltering  ResponseFilteringConfig  `yaml:"response_filtering"`
}

type GRPCPluginConfig struct {
	Name    string `yaml:"name"`
	Address string `yaml:"address"`
}

type PluginConfig struct {
	BuiltIn     []string           `yaml:"built_in"`
	GRPC        []GRPCPluginConfig `yaml:"grpc"`
	HookTimeout time.Duration      `yaml:"hook_timeout"`
}

type ShutdownConfig struct {
	GracePeriod      time.Duration `yaml:"grace_period"`
	NotifyClients    bool          `yaml:"notify_clients"`
	ForceKillTimeout time.Duration `yaml:"force_kill_timeout"`
}

type LoggingConfig struct {
	Level          string `yaml:"level"`
	Format         string `yaml:"format"`
	Output         string `yaml:"output"`
	RedactIdentity bool   `yaml:"redact_identity"`
	RedactQueries  bool   `yaml:"redact_queries"`
}

type ProxyConfig struct {
	Port       int `yaml:"port"`
	WindowSize int `yaml:"window_size"`
}

// Load reads the YAML file at path, applies defaults, applies env overrides,
// and validates the result.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: cannot read %q: %w", path, err)
	}

	cfg := defaults()

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("config: parse error: %w", err)
	}

	applyEnvOverrides(cfg)

	if err := validate(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// ParseSize parses human-readable byte sizes: "512MB", "4KB", "2GB", "1024B".
func ParseSize(s string) (int64, error) {
	if s == "" {
		return 0, fmt.Errorf("config: empty size string")
	}

	s = strings.TrimSpace(strings.ToUpper(s))

	units := []struct {
		suffix     string
		multiplier int64
	}{
		{"GB", 1024 * 1024 * 1024},
		{"MB", 1024 * 1024},
		{"KB", 1024},
		{"B", 1},
	}

	for _, u := range units {
		if strings.HasSuffix(s, u.suffix) {
			numStr := strings.TrimSuffix(s, u.suffix)
			n, err := strconv.ParseInt(strings.TrimSpace(numStr), 10, 64)
			if err != nil {
				return 0, fmt.Errorf("config: invalid size %q: %w", s, err)
			}
			return n * u.multiplier, nil
		}
	}

	return 0, fmt.Errorf("config: unrecognized size format %q (expected B/KB/MB/GB suffix)", s)
}

func defaults() *Config {
	return &Config{
		Server: ServerConfig{
			Host: "0.0.0.0",
			Port: 4433,
		},
		Auth: AuthConfig{
			JWTExpiry:    60 * time.Minute,
			JWTMaxExpiry: 24 * time.Hour,
		},
		Sessions: SessionConfig{
			IdleTimeout:           15 * time.Minute,
			MaxTTL:                4 * time.Hour,
			EvictionSweepInterval: 60 * time.Second,
			MaxSessions:           50000,
		},
		Resources: ResourceConfig{
			MemoryCeiling:      "512MB",
			MemoryPressureHigh: 0.80,
			MemoryPressureLow:  0.70,
			MaxGoroutines:      100000,
			MaxRequestSize:     "32KB",
			MaxIdentitySize:    "4KB",
			MaxQuerySize:       "32KB",
		},
		Pools: PoolConfig{
			DefaultMaxConnections: 50,
			IdleTimeout:           2 * time.Minute,
			DrainTimeout:          5 * time.Minute,
			ScaleUpQueueDepth:     1,
			HealthcheckInterval:   15 * time.Second,
			HealthcheckTimeout:    5 * time.Second,
		},
		Codebooks: CodebookConfig{
			Directory:     "./codebooks",
			Watch:         true,
			MaxDimensions: 50,
			MaxEnumValues: 20,
		},
		Knowledge: KnowledgeConfig{
			MaxRefsPerIdentity: 10,
			MaxTokensPerRef:    2000,
			ResolutionTimeout:  5 * time.Second,
		},
		Security: SecurityConfig{
			InjectionDetection: InjectionDetectionConfig{
				Enabled: true,
				Mode:    "strict",
			},
			RateLimiting: RateLimitingConfig{
				Enabled:        true,
				RPM:            60,
				Burst:          10,
				AlertThreshold: 5,
			},
			ResponseFiltering: ResponseFilteringConfig{
				Enabled:                  true,
				Mode:                     "strict",
				CheckIdentityLeakage:     true,
				CheckSystemPromptExposure: true,
			},
		},
		Plugins: PluginConfig{
			BuiltIn: []string{
				"compression",
				"summarization",
				"chunking",
				"deduplication",
				"embedding",
				"reranking",
				"filtering",
			},
			HookTimeout: 50 * time.Millisecond,
		},
		Shutdown: ShutdownConfig{
			GracePeriod:      30 * time.Second,
			NotifyClients:    true,
			ForceKillTimeout: 60 * time.Second,
		},
		Logging: LoggingConfig{
			Level:          "info",
			Format:         "json",
			Output:         "stdout",
			RedactIdentity: true,
			RedactQueries:  true,
		},
		Proxy: ProxyConfig{
			Port:       4242,
			WindowSize: 10,
		},
	}
}

func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("ENGRAM_SERVER_PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Server.Port = n
		}
	}
	if v := os.Getenv("ENGRAM_SERVER_HOST"); v != "" {
		cfg.Server.Host = v
	}
	if v := os.Getenv("ENGRAM_LOGGING_LEVEL"); v != "" {
		cfg.Logging.Level = v
	}
}

const minMemoryCeilingBytes = 128 * 1024 * 1024 // 128 MB

func validate(cfg *Config) error {
	if cfg.Server.Port < 1 || cfg.Server.Port > 65535 {
		return fmt.Errorf("config: port %d is invalid; must be 1-65535", cfg.Server.Port)
	}

	ceiling, err := ParseSize(cfg.Resources.MemoryCeiling)
	if err != nil {
		return fmt.Errorf("config: memory_ceiling is invalid: %w", err)
	}
	if ceiling < minMemoryCeilingBytes {
		return fmt.Errorf("config: memory_ceiling %q is below minimum 128MB", cfg.Resources.MemoryCeiling)
	}

	return nil
}
