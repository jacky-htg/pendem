package handler

import (
	"fmt"
	"pendem/internal/engine"
	"pendem/internal/persistence"
	"pendem/internal/server"
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
