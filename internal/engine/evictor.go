package engine

// Evictor defines the interface for eviction policies
type Evictor[V any] interface {
	// Add adds or updates an item
	Add(key string, value *Item[V])

	// Get retrieves an item and marks it as recently used
	Get(key string) (*Item[V], bool)
	Has(key string) bool

	// Remove deletes an item
	Remove(key string) bool

	// Len returns the number of items
	Len() int

	// MemoryUsage returns current memory usage in bytes
	MemoryUsage() int64

	// ForEach iterates over all items
	ForEach(fn func(key string, item *Item[V]) bool)

	MaxCapacity() int
	GetItems() map[string]Item[V]
}
