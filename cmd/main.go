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
	fmt.Printf("║  Address			: %-22s║\n", "0.0.0.0:6378")
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

	srv := server.NewServerWithConfig(":6378", log, cfg.Server)
	h := handler.NewHandler(srv, cache, persistence)
	srv.RegisterHandler("PING", h.Ping)
	srv.RegisterHandler("MEMORY", h.Memory)
	srv.RegisterHandler("GET", h.Get)
	srv.RegisterHandler("SET", h.Set)
	srv.RegisterHandler("DEL", h.Delete)
	srv.RegisterHandler("TTL", h.TTL)
	srv.RegisterHandler("POLICY", h.Policy)

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
