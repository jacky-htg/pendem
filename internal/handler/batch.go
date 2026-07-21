package handler

import (
	"pendem/internal/server"
)

// MGET key [key ...]
func (h *Handler[V]) MGet(args []string) server.RESPValue {
	if len(args) < 1 {
		return server.RESPValue{
			Type: server.Error,
			Str:  "ERR wrong number of arguments for 'mget' command",
		}
	}

	result := make([]server.RESPValue, len(args))
	for i, key := range args {
		val, found := h.cache.Get(key)
		if !found {
			result[i] = server.RESPValue{
				Type:   server.BulkString,
				Str:    "",
				IsNull: true,
			}
		} else {
			result[i] = server.RESPValue{
				Type: server.BulkString,
				Str:  val,
			}
		}
	}

	return server.RESPValue{
		Type:  server.Array,
		Array: result,
	}
}

func (h *Handler[V]) MSet(args []string) server.RESPValue {
	if len(args) < 2 {
		return server.RESPValue{
			Type: server.Error,
			Str:  "ERR wrong number of arguments for 'mset' command",
		}
	}

	if len(args)%2 != 0 {
		return server.RESPValue{
			Type: server.Error,
			Str:  "ERR wrong number of arguments for 'mset' command",
		}
	}

	for i := 0; i < len(args); i += 2 {
		key := args[i]
		value := args[i+1]
		h.cache.Set(key, value, 0)
	}

	if h.persistenceMgr != nil {
		go h.persistenceMgr.LogCommand("MSET", args...)
	}

	return server.RESPValue{
		Type: server.SimpleString,
		Str:  "OK",
	}
}

// MSETNX key value [key value ...]
func (h *Handler[V]) MSetNX(args []string) server.RESPValue {
	if len(args) < 2 {
		return server.RESPValue{
			Type: server.Error,
			Str:  "ERR wrong number of arguments for 'msetnx' command",
		}
	}

	if len(args)%2 != 0 {
		return server.RESPValue{
			Type: server.Error,
			Str:  "ERR wrong number of arguments for 'msetnx' command",
		}
	}

	// Check if any key exists
	for i := 0; i < len(args); i += 2 {
		key := args[i]
		if exists := h.cache.HasKey(key); exists {
			return server.RESPValue{
				Type: server.Integer,
				Int:  0,
			}
		}
	}

	// Set all keys
	for i := 0; i < len(args); i += 2 {
		key := args[i]
		value := args[i+1]
		h.cache.Set(key, value, 0)
	}

	if h.persistenceMgr != nil {
		go h.persistenceMgr.LogCommand("MSETNX", args...)
	}

	return server.RESPValue{
		Type: server.Integer,
		Int:  1,
	}
}
