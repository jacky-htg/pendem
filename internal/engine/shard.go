package engine

import (
	"log"
	"pendem/internal/config"
	"sync"
	"time"
)

type Shard[V any] struct {
	mu      sync.RWMutex
	policy  string
	evictor Evictor[V]
}

func NewShard[V any](cfg config.EngineConfig, logger *log.Logger) *Shard[V] {
	var evictor Evictor[V]
	policy := cfg.EvictionPolicy
	if policy == "" {
		policy = "lru"
	}

	switch policy {
	case "lru":
		evictor = NewLRUWithMemory[V](cfg.EvictorCapacity, cfg.MaxMemory, logger)
	case "lfu":
		// Future: NewLFUWithMemory[V](cfg.EvictorCapacity, cfg.MaxMemory, logger)
		logger.Printf("LFU not implemented yet, falling back to LRU")
		evictor = NewLRUWithMemory[V](cfg.EvictorCapacity, cfg.MaxMemory, logger)
		policy = "lru"
	case "ttl":
		// Future: NewTTLEvictor[V](cfg.MaxMemory, logger)
		logger.Printf("TTL eviction not implemented yet, falling back to LRU")
		evictor = NewLRUWithMemory[V](cfg.EvictorCapacity, cfg.MaxMemory, logger)
		policy = "lru"
	default:
		logger.Printf("Unknown eviction policy '%s', using LRU", cfg.EvictionPolicy)
		evictor = NewLRUWithMemory[V](cfg.EvictorCapacity, cfg.MaxMemory, logger)
		policy = "lru"
	}

	return &Shard[V]{
		policy:  policy,
		evictor: evictor,
	}
}

func (s *Shard[V]) Set(key string, value V, ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	item := NewItem(value, ttl)
	s.evictor.Add(key, item)
}

func (s *Shard[V]) Get(key string) (V, bool) {
	var zero V

	s.mu.RLock()
	item, exists := s.evictor.Get(key)
	s.mu.RUnlock()

	if !exists {
		return zero, false
	}

	// Jika expired, hapus dan return not found
	if item.IsExpired() {
		s.mu.Lock()
		s.evictor.Remove(key)
		s.mu.Unlock()
		return zero, false
	}

	return item.Value, true
}

func (s *Shard[V]) Delete(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.evictor.Remove(key)
}

func (s *Shard[V]) TTL(key string) int64 {
	s.mu.RLock()
	item, exists := s.evictor.Get(key)
	s.mu.RUnlock()

	if !exists {
		return -2 // Key doesn't exist
	}

	if item.IsExpired() {
		s.mu.Lock()
		s.evictor.Remove(key)
		s.mu.Unlock()
		return -2
	}

	ttl := item.TTL()
	if ttl < 0 {
		return -1 // No TTL
	}

	return int64(ttl.Seconds())
}

func (s *Shard[V]) MemoryUsage() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.evictor.MemoryUsage()
}

func (s *Shard[V]) Policy() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.policy
}

// Size returns the number of items currently in cache
func (s *Shard[V]) Size() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.evictor.Len()
}

// MaxCapacity returns the maximum capacity
func (s *Shard[V]) MaxCapacity() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.evictor.MaxCapacity()
}

func (s *Shard[V]) CleanupExpired() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.evictor.ForEach(func(key string, item *Item[V]) bool {
		if item.IsExpired() {
			s.evictor.Remove(key)
		}
		return true
	})
}

// internal/engine/shard.go
func (s *Shard[V]) GetItems() map[string]Item[V] {
	s.mu.RLock()
	items := make(map[string]Item[V], s.evictor.Len())

	// ✅ Iterasi cepat, ambil data
	s.evictor.ForEach(func(key string, item *Item[V]) bool {
		if !item.IsExpired() {
			items[key] = *item
		}
		return true
	})
	s.mu.RUnlock()

	return items
}

func (s *Shard[V]) Restore(items map[string]Item[V]) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for key, item := range items {
		s.evictor.Add(key, &item)
	}
}
