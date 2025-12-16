package cache

import (
	"context"
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/os-golib/go-cache/config"
)

/* ------------------ Builder ------------------ */

type Builder struct {
	cfg config.Config
	err error
}

/* ------------------ Constructor ------------------ */

func NewBuilder() *Builder {
	return &Builder{
		cfg: config.DefaultConfig(),
	}
}

/* ------------------ Load from File ------------------ */

func (b *Builder) WithLoadFromFile(path string) *Builder {
	if b.err != nil {
		return b
	}

	data, err := os.ReadFile(path)
	if err != nil {
		b.err = fmt.Errorf("read config file: %w", err)
		return b
	}

	var fileCfg config.Config
	if err := yaml.Unmarshal(data, &fileCfg); err != nil {
		b.err = fmt.Errorf("parse config file: %w", err)
		return b
	}

	mergeConfig(&b.cfg, fileCfg)
	return b
}

func mergeConfig(dst *config.Config, src config.Config) {
	mergeCore(dst, &src)
	mergeMemory(dst, &src)
	mergeRedis(dst, &src)
	mergeTimeouts(dst, &src)
	mergeHealth(dst, &src)
}

func mergeCore(dst, src *config.Config) {
	if src.Type != "" {
		dst.Type = src.Type
	}
	if src.TTL > 0 {
		dst.TTL = src.TTL
	}
	if src.Prefix != "" {
		dst.Prefix = src.Prefix
	}
	if src.RefreshTTLOnHit {
		dst.RefreshTTLOnHit = true
	}
}

func mergeMemory(dst, src *config.Config) {
	if src.MaxEntries > 0 {
		dst.MaxEntries = src.MaxEntries
	}
	if src.MaxBytes > 0 {
		dst.MaxBytes = src.MaxBytes
	}
	if src.EvictionPolicy != "" {
		dst.EvictionPolicy = src.EvictionPolicy
	}
	if src.CleanupInterval > 0 {
		dst.CleanupInterval = src.CleanupInterval
	}
}

func mergeRedis(dst, src *config.Config) {
	if src.RedisURL != "" {
		dst.RedisURL = src.RedisURL
	}
	if src.PoolSize > 0 {
		dst.PoolSize = src.PoolSize
	}
	if src.MinIdleConn > 0 {
		dst.MinIdleConn = src.MinIdleConn
	}
	if src.MaxRetries > 0 {
		dst.MaxRetries = src.MaxRetries
	}
	if src.MaxConnAge > 0 {
		dst.MaxConnAge = src.MaxConnAge
	}
}

func mergeTimeouts(dst, src *config.Config) {
	if src.ConnTimeout > 0 {
		dst.ConnTimeout = src.ConnTimeout
	}
	if src.DialTimeout > 0 {
		dst.DialTimeout = src.DialTimeout
	}
	if src.ReadTimeout > 0 {
		dst.ReadTimeout = src.ReadTimeout
	}
	if src.WriteTimeout > 0 {
		dst.WriteTimeout = src.WriteTimeout
	}
}

func mergeHealth(dst, src *config.Config) {
	if src.HealthCheck > 0 {
		dst.HealthCheck = src.HealthCheck
	}
	if src.RetryOnStart {
		dst.RetryOnStart = true
	}
	if src.StartupRetries > 0 {
		dst.StartupRetries = src.StartupRetries
	}
}

/* ------------------ Common ------------------ */

func (b *Builder) WithType(t config.Type) *Builder {
	b.cfg.Type = t
	return b
}

func (b *Builder) WithTTL(ttl time.Duration) *Builder {
	b.cfg.TTL = ttl
	return b
}

func (b *Builder) WithPrefix(prefix string) *Builder {
	b.cfg.Prefix = prefix
	return b
}

func (b *Builder) WithRefreshOnHit(v bool) *Builder {
	b.cfg.RefreshTTLOnHit = v
	return b
}

func (b *Builder) WithTimeout(timeout time.Duration) *Builder {
	if timeout > 0 {
		b.cfg.ConnTimeout = timeout
		b.cfg.DialTimeout = timeout
		b.cfg.ReadTimeout = timeout
		b.cfg.WriteTimeout = timeout
	}
	return b
}

/* ------------------ Memory ------------------ */

func (b *Builder) WithMemory() *Builder {
	b.cfg.Type = config.TypeMemory
	b.cfg.EvictionPolicy = config.EvictLRU
	return b
}

func (b *Builder) WithMaxEntries(n int) *Builder {
	b.cfg.MaxEntries = n
	return b
}

func (b *Builder) WithMaxBytes(n int) *Builder {
	b.cfg.MaxBytes = n
	return b
}

func (b *Builder) WithCleanupInterval(d time.Duration) *Builder {
	b.cfg.CleanupInterval = d
	return b
}

func (b *Builder) WithEvictionPolicy(p config.EvictionPolicy) *Builder {
	b.cfg.EvictionPolicy = p
	return b
}

/* ------------------ Redis ------------------ */

func (b *Builder) WithRedis(url string) *Builder {
	b.cfg.Type = config.TypeRedis
	b.cfg.RedisURL = url
	return b
}

func (b *Builder) WithPoolSize(size int) *Builder {
	b.cfg.PoolSize = size
	return b
}

func (b *Builder) WithMinIdleConn(n int) *Builder {
	b.cfg.MinIdleConn = n
	return b
}

func (b *Builder) WithMaxRetries(n int) *Builder {
	b.cfg.MaxRetries = n
	return b
}

func (b *Builder) WithMaxConnAge(d time.Duration) *Builder {
	b.cfg.MaxConnAge = d
	return b
}

func (b *Builder) WithConnTimeout(d time.Duration) *Builder {
	b.cfg.ConnTimeout = d
	return b
}

func (b *Builder) WithDialTimeout(d time.Duration) *Builder {
	b.cfg.DialTimeout = d
	return b
}

func (b *Builder) WithReadTimeout(d time.Duration) *Builder {
	b.cfg.ReadTimeout = d
	return b
}

func (b *Builder) WithWriteTimeout(d time.Duration) *Builder {
	b.cfg.WriteTimeout = d
	return b
}

func (b *Builder) WithHealthCheckInterval(d time.Duration) *Builder {
	b.cfg.HealthCheck = d
	return b
}

func (b *Builder) WithRetryOnStart(v bool) *Builder {
	b.cfg.RetryOnStart = v
	return b
}

func (b *Builder) WithStartupRetries(n int) *Builder {
	b.cfg.StartupRetries = n
	return b
}

/* ------------------ Build ------------------ */

func (b *Builder) Build() (config.Config, error) {
	if b.err != nil {
		return config.Config{}, b.err
	}
	if err := b.cfg.Validate(); err != nil {
		return config.Config{}, err
	}
	return b.cfg, nil
}

func (b *Builder) MustBuild() config.Config {
	cfg, err := b.Build()
	if err != nil {
		panic(err)
	}
	return cfg
}

/* ------------------ Helpers ------------------ */

// WithContext creates a context with timeout
func WithContext(
	ctx context.Context,
	timeout time.Duration,
) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, timeout)
}

/* ------------------ KeyBuilder ------------------ */

// KeyBuilder is a lightweight helper for user-defined keys
// (does NOT apply cache prefix automatically)
type KeyBuilder struct {
	prefix string
	parts  []string
}

func NewKeyBuilder(prefix string) *KeyBuilder {
	return &KeyBuilder{prefix: prefix}
}

func (k *KeyBuilder) Add(part string) *KeyBuilder {
	if part != "" {
		k.parts = append(k.parts, part)
	}
	return k
}

func (k *KeyBuilder) Reset() *KeyBuilder {
	k.parts = nil
	return k
}

func (k *KeyBuilder) Build() string {
	if len(k.parts) == 0 {
		return k.prefix
	}

	out := k.prefix
	for _, p := range k.parts {
		if out != "" {
			out += ":"
		}
		out += p
	}
	return out
}
