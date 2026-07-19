package engine

import (
	"container/list"
	"log"
)

type LRU[V any] struct {
	capacity   int
	maxMemory  int64
	currentMem int64 // ← Track memory usage
	items      map[string]*list.Element
	order      *list.List
	logger     *log.Logger
}

type lruEntry[V any] struct {
	key   string
	value *Item[V]
	size  int64
}

func NewLRU[V any](capacity int) *LRU[V] {
	return &LRU[V]{
		capacity:  capacity,
		maxMemory: 0,
		items:     make(map[string]*list.Element),
		order:     list.New(),
	}
}

func NewLRUWithMemory[V any](capacity int, maxMemory int64, logger *log.Logger) *LRU[V] {
	return &LRU[V]{
		capacity:   capacity,
		maxMemory:  maxMemory,
		currentMem: 0,
		items:      make(map[string]*list.Element),
		order:      list.New(),
		logger:     logger,
	}
}

func (l *LRU[V]) Add(key string, value *Item[V]) {
	itemSize := l.calculateSize(key, value.Value)

	// Check memory limit
	if l.maxMemory > 0 && l.currentMem+itemSize > l.maxMemory {
		// Evict until enough memory
		for l.currentMem+itemSize > l.maxMemory && l.order.Len() > 0 {
			l.removeOldest()
		}
	}

	// Jika key sudah ada, update dan pindahkan ke depan
	if elem, exists := l.items[key]; exists {
		l.order.MoveToFront(elem)
		entry := elem.Value.(*lruEntry[V])
		// Update memory usage
		l.currentMem -= entry.size
		entry.value = value
		entry.size = itemSize
		l.currentMem += itemSize
		return
	}

	// Jika penuh, hapus yang paling belakang (LRU)
	if l.capacity > 0 && l.order.Len() >= l.capacity {
		l.removeOldest()
	}

	// Tambahkan yang baru di depan
	elem := l.order.PushFront(&lruEntry[V]{
		key:   key,
		value: value,
		size:  itemSize,
	})
	l.items[key] = elem
	l.currentMem += itemSize
}

func (l *LRU[V]) Get(key string) (*Item[V], bool) {
	if elem, exists := l.items[key]; exists {
		// Pindahkan ke depan (most recently used)
		l.order.MoveToFront(elem)
		entry := elem.Value.(*lruEntry[V])
		return entry.value, true
	}
	return nil, false
}

func (l *LRU[V]) Remove(key string) bool {
	if elem, exists := l.items[key]; exists {
		entry := elem.Value.(*lruEntry[V])
		l.currentMem -= entry.size
		l.order.Remove(elem)
		delete(l.items, key)
		return true
	}
	return false
}

func (l *LRU[V]) ForEach(fn func(key string, item *Item[V]) bool) {
	for key, elem := range l.items {
		entry := elem.Value.(*lruEntry[V])
		if !fn(key, entry.value) {
			break
		}
	}
}

func (l *LRU[V]) Len() int {
	return l.order.Len()
}

func (l *LRU[V]) MemoryUsage() int64 {
	return l.currentMem
}

func (l *LRU[V]) MaxCapacity() int {
	return l.capacity
}

func (l *LRU[V]) removeOldest() {
	elem := l.order.Back()
	if elem != nil {
		entry := elem.Value.(*lruEntry[V])
		l.currentMem -= entry.size
		l.order.Remove(elem)
		delete(l.items, entry.key)

		if l.logger != nil {
			l.logger.Printf("LRU evicted: key=%s, size=%d bytes",
				entry.key, entry.size)
		}
	}
}

func (l *LRU[V]) calculateSize(key string, value V) int64 {
	size := int64(len(key))

	// Type-based size estimation
	switch v := any(value).(type) {
	case string:
		size += int64(len(v))
	case int, int8, int16, int32, int64:
		size += 8
	case uint, uint8, uint16, uint32, uint64:
		size += 8
	case float32, float64:
		size += 8
	case bool:
		size += 1
	default:
		// Fallback estimation
		size += 64
	}

	return size
}
