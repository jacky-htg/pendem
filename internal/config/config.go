package config

import "time"

// ServerConfig berisi konfigurasi server
type ServerConfig struct {
	MaxConnections int // Maksimum koneksi simultan
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
	IdleTimeout    time.Duration // Timeout koneksi idle
}

type EngineConfig struct {
	MaxMemory       int64  // Max memory in bytes
	EvictionPolicy  string // "lru", "lfu", "ttl"
	EvictorCapacity int
	ShardCount      int
	DefaultTTL      time.Duration
}

type Config struct {
	Server ServerConfig
	Engine EngineConfig
}

// DefaultConfig mengembalikan konfigurasi default
func DefaultConfig() Config {
	return Config{
		Server: ServerConfig{
			MaxConnections: 1_0000,
			ReadTimeout:    30 * time.Second,
			WriteTimeout:   30 * time.Second,
			IdleTimeout:    60 * time.Second,
		},
		Engine: EngineConfig{
			MaxMemory:       1024 * 1024 * 1024, // 1GB
			EvictionPolicy:  "lru",
			EvictorCapacity: 10_000,
			ShardCount:      16,
			DefaultTTL:      0, // No default TTL
		},
	}
}
