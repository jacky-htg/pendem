package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"pendem/internal/config"
	"pendem/internal/engine"
	"pendem/internal/handler"
	"pendem/internal/persistence"
	"pendem/internal/server"
	"syscall"
)

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "", "path to config file")
	flag.StringVar(&configPath, "c", "", "path to config file (shorthand)")
	flag.Parse()

	var path string

	// Priority 1: Flag
	if configPath != "" {
		if found, err := config.FindConfigFile(configPath); err == nil {
			path = found
		}
	}

	// Priority 2: Environment
	if path == "" {
		if envPath := os.Getenv("PENDEM_CONFIG"); envPath != "" {
			if found, err := config.FindConfigFile(envPath); err == nil {
				path = found
			}
		}
	}

	// Priority 3: Default locations (done by FindConfigFile with empty path)
	if path == "" {
		if found, err := config.FindConfigFile(""); err == nil {
			path = found
		}
	}

	cfg, err := config.LoadConfig(path)
	if err != nil {
		log.Printf("Warning: Failed to load config, using defaults: %v", err)
	}

	if err := cfg.Validate(); err != nil {
		log.Printf("⚠️ Config validation warning: %v", err)
	}

	fmt.Println("╔═══════════════════════════════════════════════════════╗")
	fmt.Println("║                     P E N D E M                       ║")
	fmt.Println("║              Simple Cache Server in Go                ║")
	fmt.Println("╠═══════════════════════════════════════════════════════╣")
	fmt.Printf("║  Address			: %-22s║\n", "0.0.0.0:"+cfg.Server.Port)
	fmt.Printf("║  Max Connection		: %-22d║\n", cfg.Server.MaxConnections)
	fmt.Printf("║  Read Timeout			: %-22s║\n", cfg.Server.ReadTimeout)
	fmt.Printf("║  Write Timeout		: %-22s║\n", cfg.Server.WriteTimeout)
	fmt.Printf("║  Idle Timeout			: %-22s║\n", cfg.Server.IdleTimeout)
	fmt.Println("╚═══════════════════════════════════════════════════════╝")
	fmt.Println()

	log := log.New(os.Stdout, "[PENDEM] ", log.LstdFlags|log.Lshortfile)
	cache := engine.NewCache[string](cfg.Engine, log)
	persistence := persistence.NewManager[string](log, cache, cfg.Persistence)
	if err := persistence.Start(); err != nil {
		log.Printf("Warning: Failed to start persistence: %v", err)
	}
	defer persistence.Stop()

	srv := server.NewServerWithConfig(":"+cfg.Server.Port, log, cfg.Server)
	h := handler.NewHandler(srv, cache, persistence)
	srv.RegisterHandler("PING", h.Ping)
	srv.RegisterHandler("ECHO", h.Echo)
	srv.RegisterHandler("MEMORY", h.Memory)
	srv.RegisterHandler("POLICY", h.Policy)

	// String
	srv.RegisterHandler("GET", h.Get)
	srv.RegisterHandler("EXISTS", h.Exists)
	srv.RegisterHandler("SET", h.Set)
	srv.RegisterHandler("DEL", h.Delete)
	srv.RegisterHandler("TTL", h.TTL)
	srv.RegisterHandler("APPEND", h.Append)
	srv.RegisterHandler("STRLEN", h.Strlen)

	// Hash commands
	srv.RegisterHandler("HSET", h.HSet)
	srv.RegisterHandler("HGET", h.HGet)
	srv.RegisterHandler("HGETALL", h.HGetAll)
	srv.RegisterHandler("HDEL", h.HDel)
	srv.RegisterHandler("HLEN", h.HLen)
	srv.RegisterHandler("HEXISTS", h.HExists)
	srv.RegisterHandler("HKEYS", h.HKeys)
	srv.RegisterHandler("HVALS", h.HVals)

	// List commands
	srv.RegisterHandler("LPUSH", h.LPush)
	srv.RegisterHandler("RPUSH", h.RPush)
	srv.RegisterHandler("LPOP", h.LPop)
	srv.RegisterHandler("RPOP", h.RPop)
	srv.RegisterHandler("LRANGE", h.LRange)
	srv.RegisterHandler("LLEN", h.LLen)

	// Set commands
	srv.RegisterHandler("SADD", h.SAdd)
	srv.RegisterHandler("SREM", h.SRem)
	srv.RegisterHandler("SMEMBERS", h.SMembers)
	srv.RegisterHandler("SISMEMBER", h.SIsMember)
	srv.RegisterHandler("SCARD", h.SCard)

	// Sorted Set commands
	srv.RegisterHandler("ZADD", h.ZAdd)
	srv.RegisterHandler("ZRANGE", h.ZRange)
	srv.RegisterHandler("ZREM", h.ZRem)
	srv.RegisterHandler("ZCARD", h.ZCard)
	srv.RegisterHandler("ZSCORE", h.ZScore)

	// Batch Operations
	srv.RegisterHandler("MGET", h.MGet)
	srv.RegisterHandler("MSET", h.MSet)
	srv.RegisterHandler("MSETNX", h.MSetNX)

	go func() {
		if err := srv.Start(); err != nil {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Gracefull shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	sig := <-quit
	fmt.Printf("\n\n╔══════════════════════════════════════════════════════╗\n")
	fmt.Printf("║  Received signal: %-34s ║\n", sig)
	fmt.Printf("║  Shutting down gracefully...                         ║\n")
	fmt.Printf("╚══════════════════════════════════════════════════════╝\n")

	if err := srv.Shutdown(); err != nil {
		log.Printf("Shutdown error: %v", err)
		os.Exit(1)
	}

	fmt.Println("\n✅ Server stopped gracefully")
}
