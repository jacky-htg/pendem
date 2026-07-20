package handler

import (
	"pendem/internal/server"
	"strconv"
	"strings"
	"time"
)

func (h *Handler[V]) Get(args []string) server.RESPValue {
	if len(args) < 1 {
		return server.RESPValue{
			Type: server.Error,
			Str:  "ERR wrong number of arguments for 'get' command",
		}
	}

	key := args[0]
	val, found := h.cache.Get(key)
	if !found {
		// Redis returns null bulk string for non-existent key
		return server.RESPValue{
			Type:   server.BulkString,
			Str:    "",
			IsNull: true, // ← Null response untuk non-existent key
		}
	}

	return server.RESPValue{
		Type: server.BulkString,
		Str:  val,
	}
}

func (h *Handler[V]) Exists(args []string) server.RESPValue {
	if len(args) < 1 {
		return server.RESPValue{
			Type: server.Error,
			Str:  "ERR wrong number of arguments for 'exists' command",
		}
	}

	count := 0
	for _, key := range args {
		if h.cache.HasKey(key) {
			count++
		}
	}

	return server.RESPValue{
		Type: server.Integer,
		Int:  int64(count),
	}
}

func (h *Handler[V]) Set(args []string) server.RESPValue {
	if len(args) < 2 {
		return server.RESPValue{
			Type: server.Error,
			Str:  "ERR wrong number of arguments for 'set' command",
		}
	}

	key := args[0]
	value := args[1]
	ttl := time.Duration(0)

	// Support EX (seconds) dan PX (milliseconds)
	if len(args) >= 4 {
		if strings.ToUpper(args[2]) == "EX" {
			if secs, err := strconv.Atoi(args[3]); err == nil {
				ttl = time.Duration(secs) * time.Second
			}
		} else if strings.ToUpper(args[2]) == "PX" {
			if ms, err := strconv.Atoi(args[3]); err == nil {
				ttl = time.Duration(ms) * time.Millisecond
			}
		}
	}

	h.cache.Set(key, value, ttl)

	if h.persistenceMgr != nil {
		go h.persistenceMgr.LogCommand("SET", args...)
	}

	return server.RESPValue{
		Type: server.SimpleString,
		Str:  "OK",
	}
}

func (h *Handler[V]) Delete(args []string) server.RESPValue {
	if len(args) < 1 {
		return server.RESPValue{
			Type: server.Error,
			Str:  "ERR wrong number of arguments for 'del' command",
		}
	}

	count := 0
	for _, key := range args {
		if h.cache.Delete(key) {
			count++
		}
	}

	if h.persistenceMgr != nil {
		go h.persistenceMgr.LogCommand("DEL", args...)
	}

	return server.RESPValue{
		Type: server.Integer,
		Int:  int64(count),
	}
}

// Tambahan: TTL Command
func (h *Handler[V]) TTL(args []string) server.RESPValue {
	if len(args) < 1 {
		return server.RESPValue{
			Type: server.Error,
			Str:  "ERR wrong number of arguments for 'ttl' command",
		}
	}

	key := args[0]
	ttl := h.cache.TTL(key)

	return server.RESPValue{
		Type: server.Integer,
		Int:  ttl,
	}
}

func (h *Handler[V]) Append(args []string) server.RESPValue {
	if len(args) < 2 {
		return server.RESPValue{
			Type: server.Error,
			Str:  "ERR wrong number of arguments for 'append' command",
		}
	}

	key := args[0]
	value := args[1]

	// Get existing value
	oldVal, exists := h.cache.Get(key)
	newVal := value
	if exists {
		newVal = oldVal + value
	}

	h.cache.Set(key, newVal, 0)

	// Log ke AOF
	if h.persistenceMgr != nil {
		go h.persistenceMgr.LogCommand("APPEND", args...)
	}

	return server.RESPValue{
		Type: server.Integer,
		Int:  int64(len(newVal)),
	}
}

func (h *Handler[V]) Strlen(args []string) server.RESPValue {
	if len(args) < 1 {
		return server.RESPValue{
			Type: server.Error,
			Str:  "ERR wrong number of arguments for 'strlen' command",
		}
	}

	key := args[0]
	val, exists := h.cache.Get(key)
	if !exists {
		return server.RESPValue{
			Type: server.Integer,
			Int:  0,
		}
	}

	return server.RESPValue{
		Type: server.Integer,
		Int:  int64(len(val)),
	}
}
