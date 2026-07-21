package handler

import (
	"fmt"
	"pendem/internal/engine"
	"pendem/internal/persistence"
	"pendem/internal/server"
	"strconv"
	"strings"
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

func (h *Handler[V]) Echo(args []string) server.RESPValue {
	if len(args) < 1 {
		return server.RESPValue{
			Type: server.Error,
			Str:  "ERR wrong number of arguments for 'echo' command",
		}
	}
	return server.RESPValue{
		Type: server.BulkString,
		Str:  args[0],
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

func (h *Handler[V]) Scan(args []string) server.RESPValue {
	if len(args) < 1 {
		return server.RESPValue{
			Type: server.Error,
			Str:  "ERR wrong number of arguments for 'scan' command",
		}
	}

	cursor, err := strconv.Atoi(args[0])
	if err != nil {
		return server.RESPValue{
			Type: server.Error,
			Str:  "ERR value is not an integer or out of range",
		}
	}

	pattern := "*"
	count := 10

	// Parse MATCH dan COUNT
	for i := 1; i < len(args); i += 2 {
		if i+1 >= len(args) {
			break
		}
		switch strings.ToUpper(args[i]) {
		case "MATCH":
			pattern = args[i+1]
		case "COUNT":
			if c, err := strconv.Atoi(args[i+1]); err == nil {
				count = c
			}
		}
	}

	keys, nextCursor := h.cache.Scan(cursor, pattern, count)

	result := make([]server.RESPValue, 2)
	result[0] = server.RESPValue{
		Type: server.BulkString,
		Str:  strconv.Itoa(nextCursor),
	}

	keyArray := make([]server.RESPValue, len(keys))
	for i, key := range keys {
		keyArray[i] = server.RESPValue{
			Type: server.BulkString,
			Str:  key,
		}
	}
	result[1] = server.RESPValue{
		Type:  server.Array,
		Array: keyArray,
	}

	return server.RESPValue{
		Type:  server.Array,
		Array: result,
	}
}
