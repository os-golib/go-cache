package config

import (
	"fmt"
	"os"
	"time"

	"github.com/go-playground/validator/v10"
	"gopkg.in/yaml.v3"
)

type Type string

const (
	TypeMemory Type = "memory"
	TypeRedis  Type = "redis"
)

type Config struct {
	Type            Type          `yaml:"type" validate:"required,oneof=memory redis"`
	TTL             time.Duration `yaml:"ttl" validate:"required,gt=0"`
	Prefix          string        `yaml:"prefix" validate:"required"`
	RefreshTTLOnHit bool          `yaml:"refresh_on_hit"`

	// Memory-specific
	MaxSize         int           `yaml:"max_size"`
	CleanupInterval time.Duration `yaml:"cleanup_interval"`
	EvictionPolicy  string        `yaml:"eviction_policy"`

	// Redis-specific
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

// Builder pattern for fluent configuration
type Builder struct {
	cfg Config
}

func NewBuilder(cacheType Type) *Builder {
	return &Builder{
		cfg: Config{
			Type:            cacheType,
			TTL:             10 * time.Minute,
			Prefix:          "cache:",
			RefreshTTLOnHit: false,
		},
	}
}

func (b *Builder) WithTTL(ttl time.Duration) *Builder {
	b.cfg.TTL = ttl
	return b
}

func (b *Builder) WithPrefix(prefix string) *Builder {
	b.cfg.Prefix = prefix
	return b
}

func (b *Builder) WithRefreshOnHit(refresh bool) *Builder {
	b.cfg.RefreshTTLOnHit = refresh
	return b
}

func (b *Builder) WithMemoryConfig(maxSize int, cleanupInterval time.Duration) *Builder {
	b.cfg.MaxSize = maxSize
	b.cfg.CleanupInterval = cleanupInterval
	b.cfg.EvictionPolicy = "lru"
	return b
}

func (b *Builder) WithRedisConfig(url string, poolSize int) *Builder {
	b.cfg.RedisURL = url
	b.cfg.PoolSize = poolSize
	b.cfg.MinIdleConn = 5
	b.cfg.MaxRetries = 3
	b.cfg.ConnTimeout = 5 * time.Second
	b.cfg.DialTimeout = 5 * time.Second
	b.cfg.ReadTimeout = 5 * time.Second
	b.cfg.WriteTimeout = 5 * time.Second
	return b
}

func (b *Builder) Build() (Config, error) {
	if err := b.cfg.Validate(); err != nil {
		return Config{}, err
	}
	return b.cfg, nil
}

func Defaults() Config {
	cfg, _ := NewBuilder(TypeMemory).
		WithMemoryConfig(10000, time.Minute).
		Build()
	return cfg
}

func Load(data []byte) (Config, error) {
	cfg := Defaults()
	if err := yaml.Unmarshal(data, &cfg); err != nil {
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
		return Config{}, fmt.Errorf("failed to read config file: %w", err)
	}
	return Load(data)
}

func (c *Config) Validate() error {
	return validator.New().Struct(c)
}
