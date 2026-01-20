package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"pupervisor/internal/api"
	"pupervisor/internal/config"
	"pupervisor/internal/service"
	"pupervisor/internal/storage"
	"pupervisor/web"
)

func main() {
	configPath := flag.String("config", "pupervisor.yaml", "Path to process configuration file")
	dbPath := flag.String("db", "pupervisor.db", "Path to SQLite database file")
	flag.Parse()

	// Load server config
	cfg := config.LoadConfig()

	// Initialize storage
	store, err := storage.New(*dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database at %s: %v", *dbPath, err)
	}
	defer store.Close()

	absDbPath, _ := filepath.Abs(*dbPath)
	log.Printf("Database initialized at %s", absDbPath)

	// Load process configuration
	procCfg, err := config.LoadProcessConfig(*configPath)
	if err != nil {
		log.Printf("Warning: Could not load process config from %s: %v", *configPath, err)
		log.Println("Starting with empty process list. Create pupervisor.yaml to define processes.")
		procCfg = &config.SupervisorConfig{Processes: []config.ProcessConfig{}}
	}

	// Initialize process manager
	pm := service.NewProcessManager(procCfg, store)

	// Get embedded filesystems
	templatesFS := web.GetTemplatesFS()
	staticFS := web.GetStaticFS()

	// Create router
	router, err := api.NewRouter(pm, templatesFS, staticFS)
	if err != nil {
		log.Fatalf("Failed to create router: %v", err)
	}

	// Create server
	srv := &http.Server{
		Addr:         cfg.Server.Address,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start auto-start processes
	pm.StartAll()

	// Start server in goroutine
	go func() {
		log.Printf("Starting Pupervisor Web UI server on %s", cfg.Server.Address)
		log.Printf("Loaded %d process(es) from configuration", len(procCfg.Processes))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Stop all managed processes
	pm.StopAll()

	// Shutdown HTTP server
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited gracefully")
}
