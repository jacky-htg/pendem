package handler

import (
	"fmt"
	"pendem/internal/engine"
	"pendem/internal/persistence"
	"pendem/internal/server"
	"strconv"
	"strings"
	"time"
)

type Handler[V any] struct {
	server         *server.Server
	cache          *engine.Cache[string]
	persistenceMgr *persistence.Manager[V]
}

func NewHandler[V any](server *server.Server, cache *engine.Cache[string], manager *persistence.Manager[V]) *Handler[V] {
	return &Handler[V]{
		server:         server,
		cache:          cache,
		persistenceMgr: manager,
	}
}

func (h *Handler[V]) Ping(args []string) server.RESPValue {
	if len(args) > 0 {
		return server.RESPValue{
			Type: server.BulkString,
			Str:  args[0],
		}
	}
	return server.RESPValue{
		Type: server.SimpleString,
		Str:  "PONG",
	}
}

func (h *Handler[V]) Policy(args []string) server.RESPValue {
	if len(args) > 0 {
		return server.RESPValue{
			Type: server.SimpleString,
			Str:  fmt.Sprintf("Policy for key '%s' not implemented yet", args[0]),
		}
	}

	return server.RESPValue{
		Type: server.SimpleString,
		Str: fmt.Sprintf("Eviction policy: %s (items: %d, max: %d)",
			h.cache.Policy(),
			h.cache.Size(),        // ← Current items
			h.cache.MaxCapacity(), // ← Max capacity
		),
	}
}

func (h *Handler[V]) Memory(args []string) server.RESPValue {
	if len(args) > 0 && strings.ToUpper(args[0]) == "USAGE" {
		return h.MemoryUsage(args[1:])
	}

	if len(args) > 0 {
		return server.RESPValue{
			Type: server.BulkString,
			Str:  args[0],
		}
	}

	return server.RESPValue{
		Type: server.SimpleString,
		Str:  h.server.PrintMemoryUsage(),
	}
}

func (h *Handler[V]) MemoryUsage(args []string) server.RESPValue {
	if len(args) > 0 {
		return server.RESPValue{
			Type: server.SimpleString,
			Str:  fmt.Sprintf("Memory usage for key '%s' not implemented yet", args[0]),
		}
	}

	memBytes := h.cache.MemoryUsage()
	memKB := float64(memBytes) / 1024
	memMB := memKB / 1024

	var display string
	if memMB > 1 {
		display = fmt.Sprintf("%.2f MB", memMB)
	} else if memKB > 1 {
		display = fmt.Sprintf("%.2f KB", memKB)
	} else {
		display = fmt.Sprintf("%d bytes", memBytes)
	}

	return server.RESPValue{
		Type: server.SimpleString,
		Str:  display,
	}
}

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
