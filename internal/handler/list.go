// internal/handler/list.go
package handler

import (
	"pendem/internal/server"
	"strconv"
)

// LPUSH key value [value ...]
func (h *Handler[V]) LPush(args []string) server.RESPValue {
	if len(args) < 2 {
		return server.RESPValue{
			Type: server.Error,
			Str:  "ERR wrong number of arguments for 'lpush' command",
		}
	}

	key := args[0]
	values := args[1:]

	list, _ := h.cache.GetOrCreateList(key)
	count := list.LPush(values...)

	if h.persistenceMgr != nil {
		go h.persistenceMgr.LogCommand("LPUSH", args...)
	}

	return server.RESPValue{
		Type: server.Integer,
		Int:  int64(count),
	}
}

// RPUSH key value [value ...]
func (h *Handler[V]) RPush(args []string) server.RESPValue {
	if len(args) < 2 {
		return server.RESPValue{
			Type: server.Error,
			Str:  "ERR wrong number of arguments for 'rpush' command",
		}
	}

	key := args[0]
	values := args[1:]

	list, _ := h.cache.GetOrCreateList(key)
	count := list.RPush(values...)

	if h.persistenceMgr != nil {
		go h.persistenceMgr.LogCommand("RPUSH", args...)
	}

	return server.RESPValue{
		Type: server.Integer,
		Int:  int64(count),
	}
}

// LPOP key
func (h *Handler[V]) LPop(args []string) server.RESPValue {
	if len(args) < 1 {
		return server.RESPValue{
			Type: server.Error,
			Str:  "ERR wrong number of arguments for 'lpop' command",
		}
	}

	key := args[0]
	list, exists := h.cache.GetList(key)
	if !exists {
		return server.RESPValue{
			Type:   server.BulkString,
			Str:    "",
			IsNull: true,
		}
	}

	val, found := list.LPop()
	if !found {
		return server.RESPValue{
			Type:   server.BulkString,
			Str:    "",
			IsNull: true,
		}
	}

	if h.persistenceMgr != nil {
		go h.persistenceMgr.LogCommand("LPOP", args...)
	}

	return server.RESPValue{
		Type: server.BulkString,
		Str:  val,
	}
}

// RPOP key
func (h *Handler[V]) RPop(args []string) server.RESPValue {
	if len(args) < 1 {
		return server.RESPValue{
			Type: server.Error,
			Str:  "ERR wrong number of arguments for 'rpop' command",
		}
	}

	key := args[0]
	list, exists := h.cache.GetList(key)
	if !exists {
		return server.RESPValue{
			Type:   server.BulkString,
			Str:    "",
			IsNull: true,
		}
	}

	val, found := list.RPop()
	if !found {
		return server.RESPValue{
			Type:   server.BulkString,
			Str:    "",
			IsNull: true,
		}
	}

	if h.persistenceMgr != nil {
		go h.persistenceMgr.LogCommand("RPOP", args...)
	}

	return server.RESPValue{
		Type: server.BulkString,
		Str:  val,
	}
}

// LRANGE key start stop
func (h *Handler[V]) LRange(args []string) server.RESPValue {
	if len(args) < 3 {
		return server.RESPValue{
			Type: server.Error,
			Str:  "ERR wrong number of arguments for 'lrange' command",
		}
	}

	key := args[0]
	start, err1 := strconv.Atoi(args[1])
	stop, err2 := strconv.Atoi(args[2])

	if err1 != nil || err2 != nil {
		return server.RESPValue{
			Type: server.Error,
			Str:  "ERR value is not an integer or out of range",
		}
	}

	list, exists := h.cache.GetList(key)
	if !exists {
		return server.RESPValue{
			Type:  server.Array,
			Array: []server.RESPValue{},
		}
	}

	items := list.LRange(start, stop)
	result := make([]server.RESPValue, len(items))
	for i, item := range items {
		result[i] = server.RESPValue{
			Type: server.BulkString,
			Str:  item,
		}
	}

	return server.RESPValue{
		Type:  server.Array,
		Array: result,
	}
}

// LLEN key
func (h *Handler[V]) LLen(args []string) server.RESPValue {
	if len(args) < 1 {
		return server.RESPValue{
			Type: server.Error,
			Str:  "ERR wrong number of arguments for 'llen' command",
		}
	}

	key := args[0]
	list, exists := h.cache.GetList(key)
	if !exists {
		return server.RESPValue{
			Type: server.Integer,
			Int:  0,
		}
	}

	return server.RESPValue{
		Type: server.Integer,
		Int:  int64(list.LLen()),
	}
}
