package engine

import "time"

type Item[T any] struct {
	Value      T
	Expiration int64 // Unix timestamp dalam nanosecond
	CreatedAt  int64 // sTambahkan untuk debugging
}

func NewItem[T any](value T, ttl time.Duration) *Item[T] {
	now := time.Now()
	var exp int64

	if ttl > 0 {
		exp = now.Add(ttl).UnixNano()
	}

	return &Item[T]{
		Value:      value,
		Expiration: exp,
		CreatedAt:  now.UnixNano(),
	}
}

func (i *Item[T]) IsExpired() bool {
	if i.Expiration == 0 {
		return false // No expiration
	}
	return time.Now().UnixNano() > i.Expiration
}

// Tambahan untuk debugging
func (i *Item[T]) TTL() time.Duration {
	if i.Expiration == 0 {
		return -1 // No TTL
	}
	remaining := i.Expiration - time.Now().UnixNano()
	if remaining < 0 {
		return 0
	}
	return time.Duration(remaining)
}
