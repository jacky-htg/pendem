package engine

import (
	"hash/fnv"
	"log"
	"pendem/internal/config"
	"sync"
	"time"
)

type Cache[V any] struct {
	shards    []*Shard[V]
	numShards int
	mu        sync.RWMutex
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

func (c *Cache[V]) HasKey(key string) bool {
	shard := c.getShard(key)
	return shard.HasKey(key)
}

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

func (c *Cache[V]) TotalCapacity() int {
	if len(c.shards) == 0 {
		return 0
	}
	return c.shards[0].MaxCapacity() * c.numShards
}

func (c *Cache[V]) GetShard(id int) *Shard[V] {
	return c.shards[id]
}

func (c *Cache[V]) NumShards() int {
	return c.numShards
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

// ============================================
// HASH OPERATIONS
// ============================================

func (c *Cache[V]) GetHash(key string) (*Hash, bool) {
	shard := c.getShard(key)
	return shard.GetHash(key)
}

func (c *Cache[V]) GetOrCreateHash(key string) (*Hash, bool) {
	shard := c.getShard(key)
	return shard.GetOrCreateHash(key)
}

// ============================================
// LIST OPERATIONS
// ============================================

func (c *Cache[V]) GetList(key string) (*List, bool) {
	shard := c.getShard(key)
	return shard.GetList(key)
}

func (c *Cache[V]) GetOrCreateList(key string) (*List, bool) {
	shard := c.getShard(key)
	return shard.GetOrCreateList(key)
}

// ============================================
// SET OPERATIONS
// ============================================

func (c *Cache[V]) GetSet(key string) (*Set, bool) {
	shard := c.getShard(key)
	return shard.GetSet(key)
}

func (c *Cache[V]) GetOrCreateSet(key string) (*Set, bool) {
	shard := c.getShard(key)
	return shard.GetOrCreateSet(key)
}

// ============================================
// SORTED SET OPERATIONS
// ============================================

func (c *Cache[V]) GetSortedSet(key string) (*SortedSet, bool) {
	shard := c.getShard(key)
	return shard.GetSortedSet(key)
}

func (c *Cache[V]) GetOrCreateSortedSet(key string) (*SortedSet, bool) {
	shard := c.getShard(key)
	return shard.GetOrCreateSortedSet(key)
}

func (c *Cache[V]) Scan(cursor int, pattern string, count int) ([]string, int) {
	// Cursor format: [shardIndex][offset]
	// Misal: cursor 1005 → shard 1, offset 5
	shardIdx := cursor / 10000
	offset := cursor % 10000

	var keys []string
	var matched int

	// Iterasi shard
	for i := shardIdx; i < len(c.shards); i++ {
		shard := c.shards[i]
		shardKeys, shardOffset := shard.Scan(offset, pattern, count-matched)

		keys = append(keys, shardKeys...)
		matched += len(shardKeys)

		if matched >= count {
			// Next cursor: shard index + offset
			nextCursor := i*10000 + shardOffset
			return keys, nextCursor
		}

		offset = 0 // Reset untuk shard berikutnya
	}

	// Selesai semua shard
	return keys, 0
}
