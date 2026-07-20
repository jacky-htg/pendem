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

	default:
		a.logger.Printf("AOF replay: unknown command '%s', skipping", cmd)
	}

	return nil
}
