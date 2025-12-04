package cache

import (
	cfg "github.com/os-golib/go-cache/config"
)

// Public type aliases
type (
	Config              = cfg.Config
	Options[T any]      = cfg.Options[T]
	CacheOptions[T any] = cfg.CacheOptions[T]

	Serializer[T any]     = cfg.Serializer[T]
	JsonSerializer[T any] = cfg.JsonSerializer[T]
	BinarySerializer      = cfg.BinarySerializer
	StringSerializer      = cfg.StringSerializer
)

// var (
//     ErrSerialization   = cfg.ErrSerialization
//     ErrDeserialization = cfg.ErrDeserialization
// )

// Public re-exports of configuration constructors
var (
	Defaults     = cfg.Defaults
	MemoryConfig = cfg.MemoryConfig
	RedisConfig  = cfg.RedisConfig
	Load         = cfg.Load
	LoadFromFile = cfg.LoadFromFile
)
