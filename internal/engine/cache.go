package engine

import (
	"hash/fnv"
	"log"
	"pendem/internal/config"
	"time"
)

type Cache[V any] struct {
	shards    []*Shard[V]
	numShards int
}

func NewCache[V any](cfg config.EngineConfig, logger *log.Logger) *Cache[V] {
	if cfg.ShardCount <= 0 {
		cfg.ShardCount = config.DefaultConfig().Engine.ShardCount
	}

	if cfg.ShardCount&(cfg.ShardCount-1) != 0 {
		logger.Printf("Warning: ShardCount %d is not power of 2 (1, 2, 4, 8, 16, 32, 64, 128, 256), performance may be affected",
			cfg.ShardCount)
	}

	cache := &Cache[V]{
		shards:    make([]*Shard[V], cfg.ShardCount),
		numShards: cfg.ShardCount,
	}

	for i := 0; i < cfg.ShardCount; i++ {
		cache.shards[i] = NewShard[V](cfg, logger)
	}

	go cache.cleanupLoop()
	return cache
}

// getShard menentukan shard berdasarkan key menggunakan hash
func (c *Cache[V]) getShard(key string) *Shard[V] {
	hasher := fnv.New32a()
	hasher.Write([]byte(key))
	hash := hasher.Sum32()
	idx := int(hash) & (c.numShards - 1)
	return c.shards[idx]
}

// ============================================
// KEY-BASED OPERATIONS (1 shard)
// ============================================

func (c *Cache[V]) Set(key string, value V, ttl time.Duration) {
	shard := c.getShard(key)
	shard.Set(key, value, ttl)
}

func (c *Cache[V]) Get(key string) (V, bool) {
	shard := c.getShard(key)
	return shard.Get(key)
}

func (c *Cache[V]) Delete(key string) bool {
	shard := c.getShard(key)
	return shard.Delete(key)
}

func (c *Cache[V]) TTL(key string) int64 {
	shard := c.getShard(key)
	return shard.TTL(key)
}

// ============================================
// GLOBAL OPERATIONS (Semua shards)
// ============================================

func (c *Cache[V]) MemoryUsage() int64 {
	var total int64
	for _, shard := range c.shards {
		total += shard.MemoryUsage()
	}
	return total
}

func (c *Cache[V]) Size() int {
	total := 0
	for _, shard := range c.shards {
		total += shard.Size()
	}
	return total
}

func (c *Cache[V]) Policy() string {
	if len(c.shards) == 0 {
		return "unknown"
	}
	return c.shards[0].Policy()
}

func (c *Cache[V]) MaxCapacity() int {
	if len(c.shards) == 0 {
		return 0
	}
	return c.shards[0].MaxCapacity()
}

func (c *Cache[V]) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		for _, shard := range c.shards {
			shard.CleanupExpired()
		}
	}
}
