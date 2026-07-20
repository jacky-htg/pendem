// internal/engine/hash.go
package engine

import "sync"

type Hash struct {
	fields map[string]string
	mu     sync.RWMutex
}

func NewHash() *Hash {
	return &Hash{
		fields: make(map[string]string),
	}
}

func (h *Hash) Has(field string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	_, exists := h.fields[field]
	return exists
}

func (h *Hash) Set(field, value string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.fields[field] = value
}

func (h *Hash) Get(field string) (string, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	val, exists := h.fields[field]
	return val, exists
}

func (h *Hash) Delete(field string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, exists := h.fields[field]; exists {
		delete(h.fields, field)
		return true
	}
	return false
}

func (h *Hash) GetAll() map[string]string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	result := make(map[string]string, len(h.fields))
	for k, v := range h.fields {
		result[k] = v
	}
	return result
}

func (h *Hash) Len() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.fields)
}

func (h *Hash) Keys() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	keys := make([]string, 0, len(h.fields))
	for k := range h.fields {
		keys = append(keys, k)
	}
	return keys
}

func (h *Hash) Values() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	values := make([]string, 0, len(h.fields))
	for _, v := range h.fields {
		values = append(values, v)
	}
	return values
}
