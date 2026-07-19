package integration

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
	"pendem/internal/config"
	"pendem/internal/engine"
	"pendem/internal/handler"
	"pendem/internal/server"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ============================================
// SETUP: Test Server
// ============================================

func setupTestServer() (*server.Server, *engine.Cache[string], string) {
	log := log.New(os.Stdout, "PENDEM:", log.LstdFlags|log.Lshortfile)
	// Create cache
	cache := engine.NewCache[string](config.DefaultConfig().Engine, log)

	// Create server
	cfg := config.ServerConfig{
		MaxConnections: 100,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		IdleTimeout:    30 * time.Second,
	}

	srv := server.NewServerWithConfig(":16378", log, cfg) // Use different port

	// Register handlers
	h := handler.NewHandler(srv, cache)
	srv.RegisterHandler("PING", h.Ping)
	srv.RegisterHandler("GET", h.Get)
	srv.RegisterHandler("SET", h.Set)
	srv.RegisterHandler("DEL", h.Delete)

	return srv, cache, "localhost:16378"
}

// ============================================
// HELPER: Send RESP Command
// ============================================

func sendRESPCommand(addr, command string, args ...string) (string, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return "", err
	}
	defer conn.Close()

	// Build RESP array
	resp := fmt.Sprintf("*%d\r\n", 1+len(args))
	resp += fmt.Sprintf("$%d\r\n%s\r\n", len(command), command)
	for _, arg := range args {
		resp += fmt.Sprintf("$%d\r\n%s\r\n", len(arg), arg)
	}

	// Send command
	_, err = conn.Write([]byte(resp))
	if err != nil {
		return "", err
	}

	// Read response
	reader := bufio.NewReader(conn)
	response, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}

	return response, nil
}

func sendCommand(addr, cmd string, args ...string) (string, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return "", err
	}
	defer conn.Close()

	// Send command as simple RESP
	command := fmt.Sprintf("*%d\r\n", 1+len(args))
	command += fmt.Sprintf("$%d\r\n%s\r\n", len(cmd), cmd)
	for _, arg := range args {
		command += fmt.Sprintf("$%d\r\n%s\r\n", len(arg), arg)
	}

	_, err = conn.Write([]byte(command))
	if err != nil {
		return "", err
	}

	// Read response
	reader := bufio.NewReader(conn)
	response, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}

	return response, nil
}

// ============================================
// TEST 1: Concurrent SET Operations
// ============================================

func TestConcurrentSET(t *testing.T) {
	srv, _, addr := setupTestServer()

	// Start server
	go func() {
		if err := srv.Start(); err != nil {
			t.Logf("Server error: %v", err)
		}
	}()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)
	defer srv.Shutdown()

	t.Run("100 concurrent SETs", func(t *testing.T) {
		var wg sync.WaitGroup
		var success atomic.Int32
		var failed atomic.Int32

		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				key := fmt.Sprintf("concurrent_key_%d", id)
				_, err := sendCommand(addr, "SET", key, fmt.Sprintf("value_%d", id))
				if err == nil {
					success.Add(1)
				} else {
					failed.Add(1)
				}
			}(i)
		}
		wg.Wait()

		t.Logf("Success: %d, Failed: %d", success.Load(), failed.Load())
		if failed.Load() > 0 {
			t.Errorf("Some SET operations failed: %d", failed.Load())
		}
	})
}

// ============================================
// TEST 2: Concurrent GET Operations
// ============================================

func TestConcurrentGET(t *testing.T) {
	srv, cache, addr := setupTestServer()

	// Pre-populate data
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("get_key_%d", i)
		cache.Set(key, fmt.Sprintf("value_%d", i), 0)
	}

	// Start server
	go func() {
		if err := srv.Start(); err != nil {
			t.Logf("Server error: %v", err)
		}
	}()

	time.Sleep(100 * time.Millisecond)
	defer srv.Shutdown()

	t.Run("100 concurrent GETs", func(t *testing.T) {
		var wg sync.WaitGroup
		var success atomic.Int32
		var failed atomic.Int32

		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				key := fmt.Sprintf("get_key_%d", id)
				resp, err := sendCommand(addr, "GET", key)
				if err == nil && resp != "" {
					success.Add(1)
				} else {
					failed.Add(1)
				}
			}(i)
		}
		wg.Wait()

		t.Logf("Success: %d, Failed: %d", success.Load(), failed.Load())
		if failed.Load() > 0 {
			t.Errorf("Some GET operations failed: %d", failed.Load())
		}
	})
}

// ============================================
// TEST 3: Concurrent READs and WRITEs (Race Test)
// ============================================

func TestConcurrentReadWrite(t *testing.T) {
	srv, _, addr := setupTestServer()

	// Start server
	go func() {
		if err := srv.Start(); err != nil {
			t.Logf("Server error: %v", err)
		}
	}()

	time.Sleep(100 * time.Millisecond)
	defer srv.Shutdown()

	t.Run("500 concurrent mixed operations", func(t *testing.T) {
		var wg sync.WaitGroup
		var success atomic.Int32
		var failed atomic.Int32

		for i := 0; i < 500; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				key := fmt.Sprintf("rw_key_%d", id%50)
				var err error

				switch id % 3 {
				case 0: // SET
					_, err = sendCommand(addr, "SET", key, fmt.Sprintf("value_%d", id))
				case 1: // GET
					_, err = sendCommand(addr, "GET", key)
				case 2: // DEL
					_, err = sendCommand(addr, "DEL", key)
				}

				if err == nil {
					success.Add(1)
				} else {
					failed.Add(1)
				}
			}(i)
		}
		wg.Wait()

		t.Logf("Success: %d, Failed: %d", success.Load(), failed.Load())
		if failed.Load() > 0 {
			t.Errorf("Some operations failed: %d", failed.Load())
		}
	})
}

// ============================================
// TEST 4: TTL Expiration with Concurrency
// ============================================

func TestConcurrentTTL(t *testing.T) {
	srv, _, addr := setupTestServer()

	// Start server
	go func() {
		if err := srv.Start(); err != nil {
			t.Logf("Server error: %v", err)
		}
	}()

	time.Sleep(100 * time.Millisecond)
	defer srv.Shutdown()

	t.Run("TTL expiration with concurrent access", func(t *testing.T) {
		// Set items with short TTL
		for i := 0; i < 50; i++ {
			key := fmt.Sprintf("ttl_key_%d", i)
			_, err := sendCommand(addr, "SET", key, fmt.Sprintf("value_%d", i), "EX", "1")
			if err != nil {
				t.Fatalf("Failed to set key: %v", err)
			}
		}

		// Read before expiration
		var wg sync.WaitGroup
		var found atomic.Int32

		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				key := fmt.Sprintf("ttl_key_%d", id)
				resp, _ := sendCommand(addr, "GET", key)
				if resp != "" && resp != "$-1\r\n" {
					found.Add(1)
				}
			}(i)
		}
		wg.Wait()
		t.Logf("Found before expiration: %d", found.Load())

		// Wait for expiration
		time.Sleep(1500 * time.Millisecond)

		// Read after expiration
		found.Store(0)
		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				key := fmt.Sprintf("ttl_key_%d", id)
				resp, _ := sendCommand(addr, "GET", key)
				if resp != "" && resp != "$-1\r\n" {
					found.Add(1)
				}
			}(i)
		}
		wg.Wait()

		t.Logf("Found after expiration: %d", found.Load())
		if found.Load() > 0 {
			t.Errorf("Keys should be expired, but %d found", found.Load())
		}
	})
}

// ============================================
// TEST 5: High Load (Stress Test)
// ============================================

func TestHighLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	srv, _, addr := setupTestServer()

	// Start server
	go func() {
		if err := srv.Start(); err != nil {
			t.Logf("Server error: %v", err)
		}
	}()

	time.Sleep(100 * time.Millisecond)
	defer srv.Shutdown()

	t.Run("1000 concurrent operations", func(t *testing.T) {
		var wg sync.WaitGroup
		var success atomic.Int32
		var failed atomic.Int32

		start := time.Now()

		for i := 0; i < 1000; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				key := fmt.Sprintf("stress_key_%d", id%100)

				var err error
				switch id % 3 {
				case 0:
					_, err = sendCommand(addr, "SET", key, fmt.Sprintf("value_%d", id))
				case 1:
					_, err = sendCommand(addr, "GET", key)
				case 2:
					_, err = sendCommand(addr, "DEL", key)
				}

				if err == nil {
					success.Add(1)
				} else {
					failed.Add(1)
				}
			}(i)
		}
		wg.Wait()

		elapsed := time.Since(start)
		t.Logf("Completed %d operations in %v", 1000, elapsed)
		t.Logf("Success: %d, Failed: %d", success.Load(), failed.Load())
		t.Logf("Throughput: %.0f ops/sec", float64(1000)/elapsed.Seconds())

		if failed.Load() > 10 { // Allow small failure rate
			t.Errorf("Too many failures: %d", failed.Load())
		}
	})
}

// ============================================
// TEST 6: Connection Pool Test
// ============================================

func TestConnectionPool(t *testing.T) {
	srv, _, addr := setupTestServer()

	go func() {
		if err := srv.Start(); err != nil {
			t.Logf("Server error: %v", err)
		}
	}()

	time.Sleep(100 * time.Millisecond)
	defer srv.Shutdown()

	t.Run("100 connections concurrently", func(t *testing.T) {
		var wg sync.WaitGroup
		var success atomic.Int32
		var failed atomic.Int32

		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				// Open connection
				conn, err := net.Dial("tcp", addr)
				if err != nil {
					failed.Add(1)
					return
				}
				defer conn.Close()

				// Send command
				_, err = conn.Write([]byte("*2\r\n$3\r\nSET\r\n$5\r\nkey_1\r\n$5\r\nvalue\r\n"))
				if err != nil {
					failed.Add(1)
					return
				}

				// Read response
				reader := bufio.NewReader(conn)
				_, err = reader.ReadString('\n')
				if err == nil {
					success.Add(1)
				} else {
					failed.Add(1)
				}
			}(i)
		}
		wg.Wait()

		t.Logf("Success: %d, Failed: %d", success.Load(), failed.Load())
		if failed.Load() > 0 {
			t.Errorf("Some connections failed: %d", failed.Load())
		}
	})
}

// ============================================
// BENCHMARK: Concurrent Operations
// ============================================

func BenchmarkConcurrentSET(b *testing.B) {
	srv, _, addr := setupTestServer()

	go func() {
		srv.Start()
	}()
	time.Sleep(100 * time.Millisecond)
	defer srv.Shutdown()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("bench_key_%d", i)
			sendCommand(addr, "SET", key, "value")
			i++
		}
	})
}
