package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"pendem/internal/config"
	"pendem/internal/engine"
	"pendem/internal/handler"
	"pendem/internal/server"
	"syscall"
	"time"
)

func main() {
	log := log.New(os.Stdout, "[PENDEM] ", log.LstdFlags|log.Lshortfile)
	config := config.Config{
		Server: config.ServerConfig{
			MaxConnections: 50_000,
			ReadTimeout:    30 * time.Second,
			WriteTimeout:   10 * time.Second,
			IdleTimeout:    60 * time.Second,
		},
		Engine: config.DefaultConfig().Engine,
	}
	srv := server.NewServerWithConfig(":6378", log, config.Server)
	h := handler.NewHandler(srv, engine.NewCache[string](config.Engine, log))
	srv.RegisterHandler("PING", h.Ping)
	srv.RegisterHandler("MEMORY", h.Memory)
	srv.RegisterHandler("GET", h.Get)
	srv.RegisterHandler("SET", h.Set)
	srv.RegisterHandler("DEL", h.Delete)
	srv.RegisterHandler("TTL", h.TTL)
	srv.RegisterHandler("POLICY", h.Policy)

	fmt.Println("╔═══════════════════════════════════════════════════════╗")
	fmt.Println("║                     P E N D E M                       ║")
	fmt.Println("║              Simple Cache Server in Go                ║")
	fmt.Println("╠═══════════════════════════════════════════════════════╣")
	fmt.Printf("║  Address			: %-22s║\n", "0.0.0.0:6378")
	fmt.Printf("║  Max Connection		: %-22d║\n", config.Server.MaxConnections)
	fmt.Printf("║  Read Timeout			: %-22s║\n", config.Server.ReadTimeout)
	fmt.Printf("║  Write Timeout		: %-22s║\n", config.Server.WriteTimeout)
	fmt.Printf("║  Idle Timeout			: %-22s║\n", config.Server.IdleTimeout)
	fmt.Println("╚═══════════════════════════════════════════════════════╝")
	fmt.Println()

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
