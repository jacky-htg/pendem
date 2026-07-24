package config

import (
	"fmt"
	"time"
)

// ServerConfig berisi konfigurasi server
type ServerConfig struct {
	Port           string
	MaxConnections int // Maksimum koneksi simultan
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
	IdleTimeout    time.Duration // Timeout koneksi idle
	RequirePass    string        // authentication password
}

type EngineConfig struct {
	MaxMemory       int64  // Max memory in bytes
	EvictionPolicy  string // "lru", "lfu", "ttl"
	EvictorCapacity int
	ShardCount      int
	DefaultTTL      time.Duration
}

type JSONConfig struct {
	Enabled      bool
	Path         string
	Interval     time.Duration
	MaxSnapshots int
}

type AOFConfig struct {
	Enabled       bool
	FilePath      string
	FlushInterval time.Duration // Default: 5 menit
	SyncOnWrite   bool          // fsync setiap write
}

type RDBConfig struct {
	Enabled  bool
	Path     string
	Interval time.Duration
}

type PersistenceConfig struct {
	JSON JSONConfig
	RDB  RDBConfig
	AOF  AOFConfig
}

type Config struct {
	Server      ServerConfig
	Engine      EngineConfig
	Persistence PersistenceConfig
}

// DefaultConfig mengembalikan konfigurasi default
func DefaultConfig() Config {
	return Config{
		Server: ServerConfig{
			Port:           "6379",
			MaxConnections: 50_000,
			ReadTimeout:    2 * time.Minute,
			WriteTimeout:   30 * time.Second,
			IdleTimeout:    10 * time.Minute,
		},
		Engine: EngineConfig{
			MaxMemory:       1024 * 1024 * 1024, // 1GB
			EvictionPolicy:  "lru",
			EvictorCapacity: 10_000,
			ShardCount:      16,
			DefaultTTL:      0, // No default TTL
		},
		Persistence: PersistenceConfig{
			JSON: JSONConfig{
				Enabled:      true,
				Path:         "pendem.snapshot.json",
				Interval:     1 * time.Hour,
				MaxSnapshots: 5,
			},
			RDB: RDBConfig{
				Enabled:  true,
				Path:     "pendem.rdb",
				Interval: 1 * time.Hour,
			},
			AOF: AOFConfig{
				Enabled:       true,
				FilePath:      "pendem.aof",
				FlushInterval: 5 * time.Minute, // 5 menit
				SyncOnWrite:   false,
			},
		},
	}
}

func LoadConfig(path string) (Config, error) {
	parser := NewConfigParser()
	if err := parser.Parse(path); err != nil {
		// Fallback to default config
		return DefaultConfig(), fmt.Errorf("failed to parse config: %w", err)
	}

	cfg := DefaultConfig()

	// [server] section
	if v, ok := parser.GetString("server", "port"); ok {
		cfg.Server.Port = v
	}
	if v, ok := parser.GetInt("server", "max_connections"); ok {
		cfg.Server.MaxConnections = v
	}
	if v, ok := parser.GetDuration("server", "read_timeout"); ok {
		cfg.Server.ReadTimeout = v
	}
	if v, ok := parser.GetDuration("server", "write_timeout"); ok {
		cfg.Server.WriteTimeout = v
	}
	if v, ok := parser.GetDuration("server", "idle_timeout"); ok {
		cfg.Server.IdleTimeout = v
	}
	if v, ok := parser.GetString("server", "requirepass"); ok {
		cfg.Server.RequirePass = v
	}

	// [engine] section
	if v, ok := parser.GetBytes("engine", "max_memory"); ok {
		cfg.Engine.MaxMemory = v
	}
	if v, ok := parser.GetString("engine", "eviction_policy"); ok {
		cfg.Engine.EvictionPolicy = v
	}
	if v, ok := parser.GetInt("engine", "evictor_capacity"); ok {
		cfg.Engine.EvictorCapacity = v
	}
	if v, ok := parser.GetInt("engine", "shard_count"); ok {
		cfg.Engine.ShardCount = v
	}
	if v, ok := parser.GetDuration("engine", "default_ttl"); ok {
		cfg.Engine.DefaultTTL = v
	}

	// [persistence.*] sections
	if v, ok := parser.GetBool("persistence.rdb", "enabled"); ok {
		cfg.Persistence.RDB.Enabled = v
	}
	if v, ok := parser.GetString("persistence.rdb", "path"); ok {
		cfg.Persistence.RDB.Path = v
	}
	if v, ok := parser.GetDuration("persistence.rdb", "interval"); ok {
		cfg.Persistence.RDB.Interval = v
	}

	if v, ok := parser.GetBool("persistence.aof", "enabled"); ok {
		cfg.Persistence.AOF.Enabled = v
	}
	if v, ok := parser.GetString("persistence.aof", "path"); ok {
		cfg.Persistence.AOF.FilePath = v
	}
	if v, ok := parser.GetDuration("persistence.aof", "flush_interval"); ok {
		cfg.Persistence.AOF.FlushInterval = v
	}
	if v, ok := parser.GetBool("persistence.aof", "sync_on_write"); ok {
		cfg.Persistence.AOF.SyncOnWrite = v
	}

	if v, ok := parser.GetBool("persistence.json", "enabled"); ok {
		cfg.Persistence.JSON.Enabled = v
	}
	if v, ok := parser.GetString("persistence.json", "path"); ok {
		cfg.Persistence.JSON.Path = v
	}
	if v, ok := parser.GetDuration("persistence.json", "interval"); ok {
		cfg.Persistence.JSON.Interval = v
	}
	if v, ok := parser.GetInt("persistence.json", "max_snapshots"); ok {
		cfg.Persistence.JSON.MaxSnapshots = v
	}

	return cfg, nil
}

// Validate checks if config is valid
func (c *Config) Validate() error {
	if c.Server.MaxConnections < 1 {
		return fmt.Errorf("max_connections must be > 0")
	}
	if c.Engine.MaxMemory < 1 {
		return fmt.Errorf("max_memory must be > 0")
	}
	if c.Engine.ShardCount <= 0 || c.Engine.ShardCount&(c.Engine.ShardCount-1) != 0 {
		return fmt.Errorf("shard_count must be power of 2 (got %d)", c.Engine.ShardCount)
	}
	if c.Persistence.RDB.Enabled && c.Persistence.RDB.Interval <= 0 {
		return fmt.Errorf("rdb.interval must be > 0 if enabled")
	}
	if c.Persistence.AOF.Enabled && c.Persistence.AOF.FlushInterval <= 0 {
		return fmt.Errorf("aof.flush_interval must be > 0 if enabled")
	}
	return nil
}

// String returns a human-readable representation of the config
func (c *Config) String() string {
	return fmt.Sprintf(
		"Server: MaxConnections=%d, ReadTimeout=%v, WriteTimeout=%v, IdleTimeout=%v\n"+
			"Engine: MaxMemory=%d, EvictionPolicy=%s, EvictorCapacity=%d, ShardCount=%d, DefaultTTL=%v\n"+
			"Persistence: RDB=%v, AOF=%v, JSON=%v",
		c.Server.MaxConnections,
		c.Server.ReadTimeout,
		c.Server.WriteTimeout,
		c.Server.IdleTimeout,
		c.Engine.MaxMemory,
		c.Engine.EvictionPolicy,
		c.Engine.EvictorCapacity,
		c.Engine.ShardCount,
		c.Engine.DefaultTTL,
		c.Persistence.RDB.Enabled,
		c.Persistence.AOF.Enabled,
		c.Persistence.JSON.Enabled,
	)
}
