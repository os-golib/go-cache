package config

import (
	"fmt"
	"os"
	"time"

	"github.com/go-playground/validator/v10"
	"gopkg.in/yaml.v3"
)

// Type represents the cache backend type
type Type string

// Supported cache types
const (
	TypeMemory Type = "memory"
	TypeRedis  Type = "redis"
)

// Config holds configuration for cache instances
type Config struct {
	Type            Type          `yaml:"type" validate:"required,oneof=memory redis"`
	TTL             time.Duration `yaml:"ttl" validate:"required,gt=0"`
	Prefix          string        `yaml:"prefix" validate:"required"`
	RefreshTTLOnHit bool          `yaml:"refresh_on_hit"`

	// Memory-specific configuration
	MaxSize         int           `yaml:"max_size"`
	CleanupInterval time.Duration `yaml:"cleanup_interval"`
	EvictionPolicy  string        `yaml:"eviction_policy"`

	// Redis-specific configuration
	URL            string        `yaml:"redis_url"`
	PoolSize       int           `yaml:"pool_size"`
	MinIdleConn    int           `yaml:"min_idle"`
	MaxRetries     int           `yaml:"max_retries"`
	MaxConnAge     time.Duration `yaml:"max_conn_age"`
	ConnTimeout    time.Duration `yaml:"conn_timeout"`
	DialTimeout    time.Duration `yaml:"dial_timeout"`
	ReadTimeout    time.Duration `yaml:"read_timeout"`
	WriteTimeout   time.Duration `yaml:"write_timeout"`
	HealthCheck    time.Duration `yaml:"health_check"`
	RetryOnStart   bool          `yaml:"retry_on_start"`
	StartupRetries int           `yaml:"startup_retries"`
}

// Defaults returns a default configuration suitable for most use cases
func Defaults() Config {
	return Config{
		Type:            TypeMemory,
		TTL:             10 * time.Minute,
		Prefix:          "cache:",
		RefreshTTLOnHit: false,
		MaxSize:         10000,
		CleanupInterval: 1 * time.Minute,
	}
}

// Load configuration from YAML data
func Load(data []byte) (Config, error) {
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}

	// Apply defaults for zero values
	cfg = Defaults()

	if err := cfg.Validate(); err != nil {
		return cfg, err
	}

	return cfg, nil
}

// LoadFromFile loads configuration from a YAML file
func LoadFromFile(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("failed to read config file: %w", err)
	}

	return Load(data)
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	return validator.New().Struct(c)
}

// WithPrefix returns a new config with updated prefix
func (c Config) WithPrefix(prefix string) Config {
	c.Prefix = prefix
	return c
}

// WithTTL returns a new config with updated TTL
func (c Config) WithTTL(ttl time.Duration) Config {
	c.TTL = ttl
	return c
}

// MemoryConfig returns a configuration optimized for memory cache
func MemoryConfig() Config {
	cfg := Defaults()
	cfg.Type = TypeMemory
	cfg.MaxSize = 10000
	cfg.CleanupInterval = time.Minute
	cfg.EvictionPolicy = "lru"
	return cfg
}

// RedisConfig returns a configuration optimized for Redis cache
func RedisConfig(url string) Config {
	cfg := Defaults()
	cfg.Type = TypeRedis
	cfg.URL = url
	cfg.PoolSize = 20
	cfg.MinIdleConn = 5
	cfg.MaxRetries = 3
	cfg.RetryOnStart = false
	cfg.MaxConnAge = 0
	cfg.ConnTimeout = 5 * time.Second
	cfg.HealthCheck = 0
	cfg.DialTimeout = 5 * time.Second
	cfg.ReadTimeout = 5 * time.Second
	cfg.WriteTimeout = 5 * time.Second
	return cfg
}
