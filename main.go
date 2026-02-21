package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pacorreia/vaults-syncer/api"
	"github.com/pacorreia/vaults-syncer/config"
	"github.com/pacorreia/vaults-syncer/storage"
	"github.com/pacorreia/vaults-syncer/sync"
)

func main() {
	configPath := flag.String("config", "config.yaml", "Path to configuration file")
	dbPath := flag.String("db", "sync.db", "Path to SQLite database file")
	dryRun := flag.Bool("dry-run", false, "Validate config and test connections without starting")
	flag.Parse()

	// Setup logger
	logLevel := slog.LevelInfo
	opts := &slog.HandlerOptions{Level: logLevel}
	logHandler := slog.NewJSONHandler(os.Stdout, opts)
	logger := slog.New(logHandler)

	// Load configuration
	logger.Info("loading configuration", slog.String("config_path", *configPath))
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		logger.Error("failed to load configuration", slog.String("error", err.Error()))
		os.Exit(1)
	}

	logger.Info("configuration loaded successfully",
		slog.Int("vaults", len(cfg.Vaults)),
		slog.Int("syncs", len(cfg.Syncs)),
	)

	// Setup database
	logger.Info("initializing database", slog.String("db_path", *dbPath))
	store, err := storage.NewStore(*dbPath)
	if err != nil {
		logger.Error("failed to initialize database", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer store.Close()

	// Create sync engine
	logger.Info("creating sync engine")
	engine, err := sync.NewEngine(cfg, store, logger)
	if err != nil {
		logger.Error("failed to create sync engine", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// Validate connections (already done in engine creation)

	// Dry-run mode
	if *dryRun {
		logger.Info("dry-run mode: configuration and connections validated successfully")
		return
	}

	// Create sync runner
	runner := sync.NewRunner(engine, logger)

	// Setup HTTP server
	mux := http.NewServeMux()
	apiHandler := api.NewHandler(runner, store, cfg, logger)

	// API endpoints
	mux.HandleFunc("GET /health", apiHandler.Health)
	mux.HandleFunc("GET /syncs", apiHandler.ListSyncs)
	mux.HandleFunc("GET /syncs/{sync_id}/status", apiHandler.GetSyncStatus)
	mux.HandleFunc("POST /syncs/{sync_id}/execute", apiHandler.ExecuteSync)
	mux.HandleFunc("GET /metrics", apiHandler.GetMetrics)

	// Start HTTP server
	addr := fmt.Sprintf("%s:%d", cfg.Server.Address, cfg.Server.Port)
	server := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	logger.Info("starting HTTP server", slog.String("address", addr))
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	// Start metrics server if different port
	if cfg.Server.MetricsPort != cfg.Server.Port {
		metricsMux := http.NewServeMux()
		metricsMux.HandleFunc("GET /metrics", apiHandler.GetMetrics)

		metricsAddr := fmt.Sprintf("%s:%d", cfg.Server.MetricsAddress, cfg.Server.MetricsPort)
		metricsServer := &http.Server{
			Addr:         metricsAddr,
			Handler:      metricsMux,
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
			IdleTimeout:  60 * time.Second,
		}

		logger.Info("starting metrics server", slog.String("address", metricsAddr))
		go func() {
			if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				logger.Error("metrics server error", slog.String("error", err.Error()))
			}
		}()
	}

	// Start sync runner
	if err := runner.Start(cfg); err != nil {
		logger.Error("failed to start sync runner", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	logger.Info("application started, waiting for signals")
	<-sigChan

	logger.Info("received shutdown signal, gracefully shutting down")

	runner.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Error("server shutdown error", slog.String("error", err.Error()))
	}

	logger.Info("application stopped")
}
