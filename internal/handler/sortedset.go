// internal/handler/sortedset.go
package handler

import (
	"pendem/internal/server"
	"strconv"
)

// ZADD key score member [score member ...]
func (h *Handler[V]) ZAdd(args []string) server.RESPValue {
	if len(args) < 3 {
		return server.RESPValue{
			Type: server.Error,
			Str:  "ERR wrong number of arguments for 'zadd' command",
		}
	}

	key := args[0]
	pairs := args[1:]

	if len(pairs)%2 != 0 {
		return server.RESPValue{
			Type: server.Error,
			Str:  "ERR wrong number of arguments for 'zadd' command",
		}
	}

	ss, _ := h.cache.GetOrCreateSortedSet(key)

	count := 0
	for i := 0; i < len(pairs); i += 2 {
		score, err := strconv.ParseFloat(pairs[i], 64)
		if err != nil {
			return server.RESPValue{
				Type: server.Error,
				Str:  "ERR value is not a valid float",
			}
		}
		member := pairs[i+1]
		if ss.Add(score, member) {
			count++
		}
	}

	if h.persistenceMgr != nil {
		go h.persistenceMgr.LogCommand("ZADD", args...)
	}

	return server.RESPValue{
		Type: server.Integer,
		Int:  int64(count),
	}
}

// ZRANGE key start stop [WITHSCORES]
func (h *Handler[V]) ZRange(args []string) server.RESPValue {
	if len(args) < 3 {
		return server.RESPValue{
			Type: server.Error,
			Str:  "ERR wrong number of arguments for 'zrange' command",
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

	withScores := false
	if len(args) >= 4 && args[3] == "WITHSCORES" {
		withScores = true
	}

	ss, exists := h.cache.GetSortedSet(key)
	if !exists {
		return server.RESPValue{
			Type:  server.Array,
			Array: []server.RESPValue{},
		}
	}

	members := ss.Range(start, stop)
	result := make([]server.RESPValue, 0)
	for _, m := range members {
		result = append(result, server.RESPValue{
			Type: server.BulkString,
			Str:  m.Member,
		})
		if withScores {
			result = append(result, server.RESPValue{
				Type: server.BulkString,
				Str:  strconv.FormatFloat(m.Score, 'f', -1, 64),
			})
		}
	}

	return server.RESPValue{
		Type:  server.Array,
		Array: result,
	}
}

// ZREM key member [member ...]
func (h *Handler[V]) ZRem(args []string) server.RESPValue {
	if len(args) < 2 {
		return server.RESPValue{
			Type: server.Error,
			Str:  "ERR wrong number of arguments for 'zrem' command",
		}
	}

	key := args[0]
	members := args[1:]

	ss, exists := h.cache.GetSortedSet(key)
	if !exists {
		return server.RESPValue{
			Type: server.Integer,
			Int:  0,
		}
	}

	count := 0
	for _, member := range members {
		if ss.Remove(member) {
			count++
		}
	}

	if h.persistenceMgr != nil {
		go h.persistenceMgr.LogCommand("ZREM", args...)
	}

	return server.RESPValue{
		Type: server.Integer,
		Int:  int64(count),
	}
}

// ZCARD key
func (h *Handler[V]) ZCard(args []string) server.RESPValue {
	if len(args) < 1 {
		return server.RESPValue{
			Type: server.Error,
			Str:  "ERR wrong number of arguments for 'zcard' command",
		}
	}

	key := args[0]
	ss, exists := h.cache.GetSortedSet(key)
	if !exists {
		return server.RESPValue{
			Type: server.Integer,
			Int:  0,
		}
	}

	return server.RESPValue{
		Type: server.Integer,
		Int:  int64(ss.Len()),
	}
}

// ZSCORE key member
func (h *Handler[V]) ZScore(args []string) server.RESPValue {
	if len(args) < 2 {
		return server.RESPValue{
			Type: server.Error,
			Str:  "ERR wrong number of arguments for 'zscore' command",
		}
	}

	key := args[0]
	member := args[1]

	ss, exists := h.cache.GetSortedSet(key)
	if !exists {
		return server.RESPValue{
			Type:   server.BulkString,
			Str:    "",
			IsNull: true,
		}
	}

	score, found := ss.Score(member)
	if !found {
		return server.RESPValue{
			Type:   server.BulkString,
			Str:    "",
			IsNull: true,
		}
	}

	return server.RESPValue{
		Type: server.BulkString,
		Str:  strconv.FormatFloat(score, 'f', -1, 64),
	}
}
