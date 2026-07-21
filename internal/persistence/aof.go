package persistence

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"pendem/internal/engine"
	"pendem/internal/server"
	"strconv"
	"strings"
	"sync"
	"time"
)

type AOF struct {
	file           *os.File
	writer         *bufio.Writer
	mu             sync.Mutex
	logger         *log.Logger
	cache          *engine.Cache[string]
	flushInterval  time.Duration
	stopChan       chan struct{}
	wg             sync.WaitGroup
	commandHandler func(cmd string, args []string) // ✅ Handler untuk replay
}

func NewAOF(logger *log.Logger, cache *engine.Cache[string], flushInterval time.Duration) *AOF {
	return &AOF{
		logger:        logger,
		cache:         cache,
		flushInterval: flushInterval,
		stopChan:      make(chan struct{}),
	}
}

func (a *AOF) Start() error {
	// Buka file AOF
	file, err := os.OpenFile("pendem.aof", os.O_CREATE|os.O_APPEND|os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("failed to open AOF file: %w", err)
	}

	a.file = file
	a.writer = bufio.NewWriter(file)

	// Start background flush
	go a.flushLoop()

	return nil
}

func (a *AOF) LogCommand(cmd string, args ...string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Format: *<count>\r\n$<len>\r\n<cmd>\r\n...
	resp := fmt.Sprintf("*%d\r\n", 1+len(args))
	resp += fmt.Sprintf("$%d\r\n%s\r\n", len(cmd), cmd)
	for _, arg := range args {
		resp += fmt.Sprintf("$%d\r\n%s\r\n", len(arg), arg)
	}

	_, err := a.writer.WriteString(resp)
	return err
}

func (a *AOF) Flush() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if err := a.writer.Flush(); err != nil {
		a.logger.Printf("Failed to flush AOF: %v", err)
	}
}

func (a *AOF) Load() error {
	// Baca file AOF dan replay ke cache
	file, err := os.Open("pendem.aof")
	if err != nil {
		return err
	}
	defer file.Close()

	startTime := time.Now()
	parser := server.NewRESPParser(file)
	commandCount := 0

	for {
		val, err := parser.Read()
		if err != nil {
			break
		}

		if err := a.replayCommand(val); err != nil {
			a.logger.Printf("Error replaying command: %v", err)
			continue
		}
		commandCount++
	}

	elapsed := time.Since(startTime)
	a.logger.Printf("AOF loaded successfully in %v (%d commands replayed)",
		elapsed, commandCount)

	return nil
}

func (a *AOF) Stop() {
	close(a.stopChan)
	a.wg.Wait()
	a.Flush()
	a.file.Close()
}

func (a *AOF) flushLoop() {
	ticker := time.NewTicker(a.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			a.Flush()
		case <-a.stopChan:
			a.Flush()
			return
		}
	}
}

func (a *AOF) replayCommand(val *server.RESPValue) error {
	// Harus array (perintah)
	if val.Type != server.Array || len(val.Array) < 1 {
		return fmt.Errorf("invalid AOF command format")
	}

	// Perintah pertama adalah command
	cmd := strings.ToUpper(val.Array[0].Str)
	args := make([]string, 0)

	// Kumpulkan arguments (bulk strings)
	for i := 1; i < len(val.Array); i++ {
		if val.Array[i].Type == server.BulkString {
			args = append(args, val.Array[i].Str)
		}
	}

	// Replay command ke cache
	switch cmd {
	case "SET":
		if len(args) < 2 {
			return fmt.Errorf("invalid SET command: %v", args)
		}
		key := args[0]
		value := args[1]
		ttl := time.Duration(0)

		// Parse TTL jika ada
		if len(args) >= 4 && strings.ToUpper(args[2]) == "EX" {
			if secs, err := strconv.Atoi(args[3]); err == nil {
				ttl = time.Duration(secs) * time.Second
			}
		}

		a.cache.Set(key, value, ttl)
		a.logger.Printf("AOF replay: SET %s = %s (TTL: %v)", key, value, ttl)

	case "DEL":
		if len(args) < 1 {
			return fmt.Errorf("invalid DEL command: %v", args)
		}
		for _, key := range args {
			a.cache.Delete(key)
			a.logger.Printf("AOF replay: DEL %s", key)
		}

		// internal/persistence/aof.go - replayCommand()

	case "HSET":
		if len(args) < 3 {
			return fmt.Errorf("invalid HSET command: %v", args)
		}

		key := args[0]
		fields := args[1:]

		if len(fields)%2 != 0 {
			return fmt.Errorf("wrong number of arguments for 'hset' command: %v", fields)
		}

		// Get or create hash dan set fields
		hash, _ := a.cache.GetOrCreateHash(key)
		count := 0
		for i := 0; i < len(fields); i += 2 {
			field := fields[i]
			value := fields[i+1]
			if !hash.Has(field) {
				count++
			}
			hash.Set(field, value)
		}
		a.logger.Printf("AOF replay: HSET %s = %d fields added", key, count)

	case "HDEL":
		if len(args) < 2 {
			return fmt.Errorf("invalid HDEL command: %v", args)
		}

		key := args[0]
		fields := args[1:]

		hash, exists := a.cache.GetHash(key)
		if !exists {
			a.logger.Printf("AOF replay: HDEL %s - hash not found, skipping", key)
			return nil
		}

		count := 0
		for _, field := range fields {
			if hash.Delete(field) {
				count++
			}
		}
		a.logger.Printf("AOF replay: HDEL %s = %d fields deleted", key, count)

	case "LPUSH":
		if len(args) < 2 {
			return fmt.Errorf("invalid LPUSH command: %v", args)
		}
		key := args[0]
		values := args[1:]
		list, _ := a.cache.GetOrCreateList(key)
		count := list.LPush(values...)
		a.logger.Printf("AOF replay: LPUSH %s = %d items", key, count)

	case "RPUSH":
		if len(args) < 2 {
			return fmt.Errorf("invalid RPUSH command: %v", args)
		}
		key := args[0]
		values := args[1:]
		list, _ := a.cache.GetOrCreateList(key)
		count := list.RPush(values...)
		a.logger.Printf("AOF replay: RPUSH %s = %d items", key, count)

	case "LPOP":
		if len(args) < 1 {
			return fmt.Errorf("invalid LPOP command: %v", args)
		}
		key := args[0]
		list, exists := a.cache.GetList(key)
		if !exists {
			return nil
		}
		val, _ := list.LPop()
		a.logger.Printf("AOF replay: LPOP %s = %s", key, val)

	case "RPOP":
		if len(args) < 1 {
			return fmt.Errorf("invalid RPOP command: %v", args)
		}
		key := args[0]
		list, exists := a.cache.GetList(key)
		if !exists {
			return nil
		}
		val, _ := list.RPop()
		a.logger.Printf("AOF replay: RPOP %s = %s", key, val)

	case "SADD":
		if len(args) < 2 {
			return fmt.Errorf("invalid SADD command: %v", args)
		}
		key := args[0]
		members := args[1:]
		set, _ := a.cache.GetOrCreateSet(key)
		count := set.Add(members...)
		a.logger.Printf("AOF replay: SADD %s = %d members added", key, count)

	case "SREM":
		if len(args) < 2 {
			return fmt.Errorf("invalid SREM command: %v", args)
		}
		key := args[0]
		members := args[1:]
		set, exists := a.cache.GetSet(key)
		if !exists {
			return nil
		}
		count := set.Remove(members...)
		a.logger.Printf("AOF replay: SREM %s = %d members removed", key, count)

	case "ZADD":
		if len(args) < 3 {
			return fmt.Errorf("invalid ZADD command: %v", args)
		}
		key := args[0]
		pairs := args[1:]
		if len(pairs)%2 != 0 {
			return fmt.Errorf("wrong number of arguments for 'zadd' command: %v", pairs)
		}
		ss, _ := a.cache.GetOrCreateSortedSet(key)
		count := 0
		for i := 0; i < len(pairs); i += 2 {
			score, err := strconv.ParseFloat(pairs[i], 64)
			if err != nil {
				return fmt.Errorf("invalid score: %v", pairs[i])
			}
			member := pairs[i+1]
			if ss.Add(score, member) {
				count++
			}
		}
		a.logger.Printf("AOF replay: ZADD %s = %d members added", key, count)

	case "ZREM":
		if len(args) < 2 {
			return fmt.Errorf("invalid ZREM command: %v", args)
		}
		key := args[0]
		members := args[1:]
		ss, exists := a.cache.GetSortedSet(key)
		if !exists {
			return nil
		}
		count := 0
		for _, member := range members {
			if ss.Remove(member) {
				count++
			}
		}
		a.logger.Printf("AOF replay: ZREM %s = %d members removed", key, count)

	case "MSET":
		if len(args) < 2 {
			return fmt.Errorf("invalid MSET command: %v", args)
		}
		if len(args)%2 != 0 {
			return fmt.Errorf("wrong number of arguments for 'mset' command: %v", args)
		}
		for i := 0; i < len(args); i += 2 {
			key := args[i]
			value := args[i+1]
			a.cache.Set(key, value, 0)
		}
		a.logger.Printf("AOF replay: MSET = %d keys", len(args)/2)

	case "MSETNX":
		if len(args) < 2 {
			return fmt.Errorf("invalid MSETNX command: %v", args)
		}
		if len(args)%2 != 0 {
			return fmt.Errorf("wrong number of arguments for 'msetnx' command: %v", args)
		}

		// Check if any key exists
		allNew := true
		for i := 0; i < len(args); i += 2 {
			key := args[i]
			if exists := a.cache.HasKey(key); exists {
				allNew = false
				break
			}
		}

		// Set all keys if all are new
		if allNew {
			for i := 0; i < len(args); i += 2 {
				key := args[i]
				value := args[i+1]
				a.cache.Set(key, value, 0)
			}
			a.logger.Printf("AOF replay: MSETNX = %d keys set (all new)", len(args)/2)
		} else {
			a.logger.Printf("AOF replay: MSETNX = 0 keys set (some keys already exist)")
		}

	default:
		a.logger.Printf("AOF replay: unknown command '%s', skipping", cmd)
	}

	return nil
}
