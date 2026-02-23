// Package main provides a daemon for synchronizing secrets across multiple vaults
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
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

// Version information. Set via ldflags at build time:
// go build -ldflags "-X main.Version=1.0.0"
var Version = "dev"

type appRunner interface {
	api.Runner
	Start(cfg *config.Config) error
	Stop()
}

type httpServer interface {
	ListenAndServe() error
	Shutdown(ctx context.Context) error
}

type appDeps struct {
	loadConfig    func(path string) (*config.Config, error)
	newStore      func(path string) (*storage.Store, error)
	newEngine     func(cfg *config.Config, store *storage.Store, logger *slog.Logger) (sync.EngineRunner, error)
	newRunner     func(engine sync.EngineRunner, logger *slog.Logger) appRunner
	newServer     func(addr string, handler http.Handler) httpServer
	waitForSignal func() <-chan os.Signal
}

func defaultDeps() appDeps {
	return appDeps{
		loadConfig: config.LoadConfig,
		newStore:   storage.NewStore,
		newEngine: func(cfg *config.Config, store *storage.Store, logger *slog.Logger) (sync.EngineRunner, error) {
			return sync.NewEngine(cfg, store, logger)
		},
		newRunner: func(engine sync.EngineRunner, logger *slog.Logger) appRunner {
			return sync.NewRunner(engine, logger)
		},
		newServer: func(addr string, handler http.Handler) httpServer {
			return &http.Server{
				Addr:         addr,
				Handler:      handler,
				ReadTimeout:  15 * time.Second,
				WriteTimeout: 15 * time.Second,
				IdleTimeout:  60 * time.Second,
			}
		},
		waitForSignal: func() <-chan os.Signal {
			ch := make(chan os.Signal, 1)
			signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
			return ch
		},
	}
}

var runFunc = run
var exitFunc = os.Exit

func main() {
	if err := runFunc(os.Args[1:], defaultDeps()); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		exitFunc(1)
	}
}

func run(args []string, deps appDeps) error {
	fs := flag.NewFlagSet("vaults-syncer", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	configPath := fs.String("config", "config.yaml", "Path to configuration file")
	dbPath := fs.String("db", "sync.db", "Path to SQLite database file")
	dryRun := fs.Bool("dry-run", false, "Validate config and test connections without starting")
	version := fs.Bool("version", false, "Print version information and exit")
	
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("failed to parse args: %w", err)
	}

	// Handle version flag
	if *version {
		fmt.Printf("vaults-syncer version %s\n", Version)
		return nil
	}

	// Setup logger
	logLevel := slog.LevelInfo
	opts := &slog.HandlerOptions{Level: logLevel}
	logHandler := slog.NewJSONHandler(os.Stdout, opts)
	logger := slog.New(logHandler)

	// Load configuration
	logger.Info("loading configuration", slog.String("config_path", *configPath))
	cfg, err := deps.loadConfig(*configPath)
	if err != nil {
		logger.Error("failed to load configuration", slog.String("error", err.Error()))
		return err
	}

	logger.Info("configuration loaded successfully",
		slog.Int("vaults", len(cfg.Vaults)),
		slog.Int("syncs", len(cfg.Syncs)),
	)

	// Setup database
	logger.Info("initializing database", slog.String("db_path", *dbPath))
	store, err := deps.newStore(*dbPath)
	if err != nil {
		logger.Error("failed to initialize database", slog.String("error", err.Error()))
		return err
	}
	defer store.Close()

	// Create sync engine
	logger.Info("creating sync engine")
	engine, err := deps.newEngine(cfg, store, logger)
	if err != nil {
		logger.Error("failed to create sync engine", slog.String("error", err.Error()))
		return err
	}

	// Dry-run mode
	if *dryRun {
		logger.Info("dry-run mode: configuration and connections validated successfully")
		return nil
	}

	// Create sync runner
	runner := deps.newRunner(engine, logger)

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
	server := deps.newServer(addr, mux)

	serverErrs := make(chan error, 2)
	logger.Info("starting HTTP server", slog.String("address", addr))
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", slog.String("error", err.Error()))
			serverErrs <- err
		}
	}()

	var metricsServer httpServer
	if cfg.Server.MetricsPort != cfg.Server.Port {
		metricsMux := http.NewServeMux()
		metricsMux.HandleFunc("GET /metrics", apiHandler.GetMetrics)

		metricsAddr := fmt.Sprintf("%s:%d", cfg.Server.MetricsAddress, cfg.Server.MetricsPort)
		metricsServer = deps.newServer(metricsAddr, metricsMux)

		logger.Info("starting metrics server", slog.String("address", metricsAddr))
		go func() {
			if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				logger.Error("metrics server error", slog.String("error", err.Error()))
				serverErrs <- err
			}
		}()
	}

	// Start sync runner
	if err := runner.Start(cfg); err != nil {
		logger.Error("failed to start sync runner", slog.String("error", err.Error()))
		return err
	}

	logger.Info("application started, waiting for signals")
	select {
	case <-deps.waitForSignal():
		logger.Info("received shutdown signal, gracefully shutting down")
	case err := <-serverErrs:
		return err
	}

	runner.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Error("server shutdown error", slog.String("error", err.Error()))
		return err
	}
	if metricsServer != nil {
		if err := metricsServer.Shutdown(ctx); err != nil {
			logger.Error("metrics server shutdown error", slog.String("error", err.Error()))
			return err
		}
	}

	logger.Info("application stopped")
	return nil
}
