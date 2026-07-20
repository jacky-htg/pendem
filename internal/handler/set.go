// internal/handler/set.go
package handler

import (
	"pendem/internal/server"
)

// SADD key member [member ...]
func (h *Handler[V]) SAdd(args []string) server.RESPValue {
	if len(args) < 2 {
		return server.RESPValue{
			Type: server.Error,
			Str:  "ERR wrong number of arguments for 'sadd' command",
		}
	}

	key := args[0]
	members := args[1:]

	set, _ := h.cache.GetOrCreateSet(key)
	count := set.Add(members...)

	if h.persistenceMgr != nil {
		go h.persistenceMgr.LogCommand("SADD", args...)
	}

	return server.RESPValue{
		Type: server.Integer,
		Int:  int64(count),
	}
}

// SREM key member [member ...]
func (h *Handler[V]) SRem(args []string) server.RESPValue {
	if len(args) < 2 {
		return server.RESPValue{
			Type: server.Error,
			Str:  "ERR wrong number of arguments for 'srem' command",
		}
	}

	key := args[0]
	members := args[1:]

	set, exists := h.cache.GetSet(key)
	if !exists {
		return server.RESPValue{
			Type: server.Integer,
			Int:  0,
		}
	}

	count := set.Remove(members...)

	if h.persistenceMgr != nil {
		go h.persistenceMgr.LogCommand("SREM", args...)
	}

	return server.RESPValue{
		Type: server.Integer,
		Int:  int64(count),
	}
}

// SMEMBERS key
func (h *Handler[V]) SMembers(args []string) server.RESPValue {
	if len(args) < 1 {
		return server.RESPValue{
			Type: server.Error,
			Str:  "ERR wrong number of arguments for 'smembers' command",
		}
	}

	key := args[0]
	set, exists := h.cache.GetSet(key)
	if !exists {
		return server.RESPValue{
			Type:  server.Array,
			Array: []server.RESPValue{},
		}
	}

	members := set.Members()
	result := make([]server.RESPValue, len(members))
	for i, m := range members {
		result[i] = server.RESPValue{
			Type: server.BulkString,
			Str:  m,
		}
	}

	return server.RESPValue{
		Type:  server.Array,
		Array: result,
	}
}

// SISMEMBER key member
func (h *Handler[V]) SIsMember(args []string) server.RESPValue {
	if len(args) < 2 {
		return server.RESPValue{
			Type: server.Error,
			Str:  "ERR wrong number of arguments for 'sismember' command",
		}
	}

	key := args[0]
	member := args[1]

	set, exists := h.cache.GetSet(key)
	if !exists {
		return server.RESPValue{
			Type: server.Integer,
			Int:  0,
		}
	}

	if set.IsMember(member) {
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

// SCARD key
func (h *Handler[V]) SCard(args []string) server.RESPValue {
	if len(args) < 1 {
		return server.RESPValue{
			Type: server.Error,
			Str:  "ERR wrong number of arguments for 'scard' command",
		}
	}

	key := args[0]
	set, exists := h.cache.GetSet(key)
	if !exists {
		return server.RESPValue{
			Type: server.Integer,
			Int:  0,
		}
	}

	return server.RESPValue{
		Type: server.Integer,
		Int:  int64(set.Len()),
	}
}
