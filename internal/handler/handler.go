package handler

import (
	"fmt"
	"pendem/internal/engine"
	"pendem/internal/server"
	"strconv"
	"strings"
	"time"
)

type Handler struct {
	server *server.Server
	cache  *engine.Cache[string]
}

func NewHandler(server *server.Server, cache *engine.Cache[string]) *Handler {
	return &Handler{
		server: server,
		cache:  cache,
	}
}

func (h *Handler) Ping(args []string) server.RESPValue {
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

func (h *Handler) Policy(args []string) server.RESPValue {
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

func (h *Handler) Memory(args []string) server.RESPValue {
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

func (h *Handler) MemoryUsage(args []string) server.RESPValue {
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

func (h *Handler) Get(args []string) server.RESPValue {
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

func (h *Handler) Set(args []string) server.RESPValue {
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

	return server.RESPValue{
		Type: server.SimpleString,
		Str:  "OK",
	}
}

func (h *Handler) Delete(args []string) server.RESPValue {
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

	return server.RESPValue{
		Type: server.Integer,
		Int:  int64(count),
	}
}

// Tambahan: TTL Command
func (h *Handler) TTL(args []string) server.RESPValue {
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
