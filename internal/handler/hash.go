// internal/handler/hash.go
package handler

import (
	"pendem/internal/server"
)

// HSET key field value [field value ...]
func (h *Handler[V]) HSet(args []string) server.RESPValue {
	if len(args) < 3 {
		return server.RESPValue{
			Type: server.Error,
			Str:  "ERR wrong number of arguments for 'hset' command",
		}
	}

	key := args[0]
	fields := args[1:]

	if len(fields)%2 != 0 {
		return server.RESPValue{
			Type: server.Error,
			Str:  "ERR wrong number of arguments for 'hset' command",
		}
	}

	// Get or create hash
	hash, _ := h.cache.GetOrCreateHash(key)

	count := 0
	for i := 0; i < len(fields); i += 2 {
		field := fields[i]
		value := fields[i+1]
		if !hash.Has(field) {
			count++
		}
		hash.Set(field, value)
	}

	if h.persistenceMgr != nil {
		go h.persistenceMgr.LogCommand("HSET", args...)
	}

	return server.RESPValue{
		Type: server.Integer,
		Int:  int64(count),
	}
}

// HGET key field
func (h *Handler[V]) HGet(args []string) server.RESPValue {
	if len(args) < 2 {
		return server.RESPValue{
			Type: server.Error,
			Str:  "ERR wrong number of arguments for 'hget' command",
		}
	}

	key := args[0]
	field := args[1]

	hash, exists := h.cache.GetHash(key)
	if !exists {
		return server.RESPValue{
			Type:   server.BulkString,
			Str:    "",
			IsNull: true,
		}
	}

	val, found := hash.Get(field)
	if !found {
		return server.RESPValue{
			Type:   server.BulkString,
			Str:    "",
			IsNull: true,
		}
	}

	return server.RESPValue{
		Type: server.BulkString,
		Str:  val,
	}
}

// HGETALL key
func (h *Handler[V]) HGetAll(args []string) server.RESPValue {
	if len(args) < 1 {
		return server.RESPValue{
			Type: server.Error,
			Str:  "ERR wrong number of arguments for 'hgetall' command",
		}
	}

	key := args[0]
	hash, exists := h.cache.GetHash(key)
	if !exists {
		return server.RESPValue{
			Type:  server.Array,
			Array: []server.RESPValue{},
		}
	}

	all := hash.GetAll()
	result := make([]server.RESPValue, 0, len(all)*2)
	for field, value := range all {
		result = append(result, server.RESPValue{
			Type: server.BulkString,
			Str:  field,
		})
		result = append(result, server.RESPValue{
			Type: server.BulkString,
			Str:  value,
		})
	}

	return server.RESPValue{
		Type:  server.Array,
		Array: result,
	}
}

// HDEL key field [field ...]
func (h *Handler[V]) HDel(args []string) server.RESPValue {
	if len(args) < 2 {
		return server.RESPValue{
			Type: server.Error,
			Str:  "ERR wrong number of arguments for 'hdel' command",
		}
	}

	key := args[0]
	fields := args[1:]

	hash, exists := h.cache.GetHash(key)
	if !exists {
		return server.RESPValue{
			Type: server.Integer,
			Int:  0,
		}
	}

	count := 0
	for _, field := range fields {
		if hash.Delete(field) {
			count++
		}
	}

	if h.persistenceMgr != nil {
		go h.persistenceMgr.LogCommand("HDEL", args...)
	}

	return server.RESPValue{
		Type: server.Integer,
		Int:  int64(count),
	}
}

// HLEN key
func (h *Handler[V]) HLen(args []string) server.RESPValue {
	if len(args) < 1 {
		return server.RESPValue{
			Type: server.Error,
			Str:  "ERR wrong number of arguments for 'hlen' command",
		}
	}

	key := args[0]
	hash, exists := h.cache.GetHash(key)
	if !exists {
		return server.RESPValue{
			Type: server.Integer,
			Int:  0,
		}
	}

	return server.RESPValue{
		Type: server.Integer,
		Int:  int64(hash.Len()),
	}
}

// HEXISTS key field
func (h *Handler[V]) HExists(args []string) server.RESPValue {
	if len(args) < 2 {
		return server.RESPValue{
			Type: server.Error,
			Str:  "ERR wrong number of arguments for 'hexists' command",
		}
	}

	key := args[0]
	field := args[1]

	hash, exists := h.cache.GetHash(key)
	if !exists {
		return server.RESPValue{
			Type: server.Integer,
			Int:  0,
		}
	}

	_, found := hash.Get(field)
	if found {
		return server.RESPValue{
			Type: server.Integer,
			Int:  1,
		}
	}

	return server.RESPValue{
		Type: server.Integer,
		Int:  0,
	}
}

// HKEYS key
func (h *Handler[V]) HKeys(args []string) server.RESPValue {
	if len(args) < 1 {
		return server.RESPValue{
			Type: server.Error,
			Str:  "ERR wrong number of arguments for 'hkeys' command",
		}
	}

	key := args[0]
	hash, exists := h.cache.GetHash(key)
	if !exists {
		return server.RESPValue{
			Type:  server.Array,
			Array: []server.RESPValue{},
		}
	}

	keys := hash.Keys()
	result := make([]server.RESPValue, len(keys))
	for i, k := range keys {
		result[i] = server.RESPValue{
			Type: server.BulkString,
			Str:  k,
		}
	}

	return server.RESPValue{
		Type:  server.Array,
		Array: result,
	}
}

// HVALS key
func (h *Handler[V]) HVals(args []string) server.RESPValue {
	if len(args) < 1 {
		return server.RESPValue{
			Type: server.Error,
			Str:  "ERR wrong number of arguments for 'hvals' command",
		}
	}

	key := args[0]
	hash, exists := h.cache.GetHash(key)
	if !exists {
		return server.RESPValue{
			Type:  server.Array,
			Array: []server.RESPValue{},
		}
	}

	values := hash.Values()
	result := make([]server.RESPValue, len(values))
	for i, v := range values {
		result[i] = server.RESPValue{
			Type: server.BulkString,
			Str:  v,
		}
	}

	return server.RESPValue{
		Type:  server.Array,
		Array: result,
	}
}
