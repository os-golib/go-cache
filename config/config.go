package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

/* ------------------ Enums ------------------ */

type Type string

const (
	TypeMemory Type = "memory"
	TypeRedis  Type = "redis"
)

func (t Type) Valid() bool {
	return t == TypeMemory || t == TypeRedis
}

type EvictionPolicy string

const (
	EvictLRU  EvictionPolicy = "lru"
	EvictLFU  EvictionPolicy = "lfu"
	EvictFIFO EvictionPolicy = "fifo"
	EvictARC  EvictionPolicy = "arc"
	EvictTiny EvictionPolicy = "tinylfu"
)

func (e EvictionPolicy) Valid() bool {
	switch e {
	case EvictLRU, EvictLFU, EvictFIFO, EvictARC, EvictTiny:
		return true
	default:
		return false
	}
}

/* ------------------ Config ------------------ */

type Config struct {
	// Common
	Type            Type          `yaml:"type"`
	TTL             time.Duration `yaml:"ttl"`
	Prefix          string        `yaml:"prefix"`
	RefreshTTLOnHit bool          `yaml:"refresh_on_hit"`

	// Memory cache
	MaxSize         int            `yaml:"max_size"`
	MaxEntries      int            `yaml:"max_entries"`
	MaxBytes        int            `yaml:"max_bytes"`
	CleanupInterval time.Duration  `yaml:"cleanup_interval"`
	EvictionPolicy  EvictionPolicy `yaml:"eviction_policy"`

	// Redis cache
	RedisURL       string        `yaml:"redis_url"`
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

/* ------------------ Loaders ------------------ */

func Load(data []byte) (Config, error) {
	cfg := DefaultConfig()

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("yaml unmarshal: %w", err)
	}

	applyEnvOverrides(&cfg)

	if err := cfg.Normalize(); err != nil {
		return cfg, err
	}

	if err := cfg.Validate(); err != nil {
		return cfg, err
	}

	return cfg, nil
}

func LoadFromFile(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config file: %w", err)
	}
	return Load(data)
}

/* ------------------ Normalize ------------------ */

// Normalize sets derived or default values WITHOUT validation logic.
func (c *Config) Normalize() error {
	c.Type = Type(strings.ToLower(string(c.Type)))
	c.Prefix = strings.TrimSpace(c.Prefix)

	if c.EvictionPolicy == "" {
		c.EvictionPolicy = EvictLRU
	}

	if c.CleanupInterval == 0 {
		c.CleanupInterval = time.Minute
	}

	return nil
}

/* ------------------ Validation ------------------ */

func (c *Config) Validate() error {
	if !c.Type.Valid() {
		return fmt.Errorf("invalid cache type: %q", c.Type)
	}

	if c.TTL <= 0 {
		return errors.New("ttl must be > 0")
	}

	switch c.Type {
	case TypeMemory:
		return validateMemory(c)
	case TypeRedis:
		return validateRedis(c)
	default:
		return fmt.Errorf("unsupported cache type: %s", c.Type)
	}
}

func validateMemory(c *Config) error {
	if c.MaxSize <= 0 && c.MaxEntries <= 0 && c.MaxBytes <= 0 {
		return errors.New("one of max_size, max_entries, or max_bytes must be set")
	}

	if !c.EvictionPolicy.Valid() {
		return fmt.Errorf("invalid eviction_policy: %q", c.EvictionPolicy)
	}

	return nil
}

func validateRedis(c *Config) error {
	if c.RedisURL == "" {
		return errors.New("redis_url is required for redis cache")
	}

	if c.PoolSize <= 0 {
		return errors.New("pool_size must be > 0")
	}

	if c.ConnTimeout <= 0 {
		return errors.New("conn_timeout must be > 0")
	}

	return nil
}

/* ------------------ Defaults ------------------ */

func DefaultConfig() Config {
	return Config{
		Type:            TypeMemory,
		TTL:             5 * time.Minute,
		Prefix:          "cache:",
		CleanupInterval: time.Minute,
		EvictionPolicy:  EvictLRU,

		MaxSize:    1024,
		MaxEntries: 10_000,
		MaxBytes:   64 << 20, // 64MB

		PoolSize:       10,
		MinIdleConn:    2,
		MaxRetries:     3,
		ConnTimeout:    5 * time.Second,
		DialTimeout:    5 * time.Second,
		ReadTimeout:    3 * time.Second,
		WriteTimeout:   3 * time.Second,
		HealthCheck:    10 * time.Second,
		StartupRetries: 5,
	}
}

/* ------------------ ENV Overrides ------------------ */

func applyEnvOverrides(c *Config) {
	if v := os.Getenv("CACHE_TYPE"); v != "" {
		c.Type = Type(strings.ToLower(v))
	}

	if v := os.Getenv("CACHE_PREFIX"); v != "" {
		c.Prefix = v
	}

	if v := os.Getenv("CACHE_TTL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			c.TTL = d
		}
	}

	if v := os.Getenv("REDIS_URL"); v != "" {
		c.RedisURL = v
	}
}
