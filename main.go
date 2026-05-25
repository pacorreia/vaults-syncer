// Package main provides a daemon for synchronizing secrets across multiple vaults.
// This daemon delivers robust enterprise-grade secure and reliable multi-vault sync with automated release versioning.
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
	"sync"
	"syscall"
	"time"

	"github.com/pacorreia/vaults-syncer/api"
	"github.com/pacorreia/vaults-syncer/auth"
	"github.com/pacorreia/vaults-syncer/config"
	"github.com/pacorreia/vaults-syncer/security"
	"github.com/pacorreia/vaults-syncer/storage"
	syncp "github.com/pacorreia/vaults-syncer/sync"
)

// Version information. Set via ldflags at build time:
// go build -ldflags "-X main.Version=1.0.0 -X main.BuildDate=$(date -u +'%Y-%m-%dT%H:%M:%SZ') -X main.GitCommit=$(git rev-parse HEAD)"
var (
	Version   = "dev"
	BuildDate = "unknown"
	GitCommit = "unknown"
)

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
	newStore      func(cfg storage.DBConfig) (*storage.Store, error)
	newEngine     func(cfg *config.Config, store *storage.Store, logger *slog.Logger) (syncp.EngineRunner, error)
	newRunner     func(engine syncp.EngineRunner, logger *slog.Logger) appRunner
	newServer     func(addr string, handler http.Handler) httpServer
	waitForSignal func() <-chan os.Signal
}

func defaultDeps() appDeps {
	return appDeps{
		newStore: func(cfg storage.DBConfig) (*storage.Store, error) {
			return storage.NewStoreFromConfig(cfg)
		},
		newEngine: func(cfg *config.Config, store *storage.Store, logger *slog.Logger) (syncp.EngineRunner, error) {
			return syncp.NewEngine(cfg, store, logger)
		},
		newRunner: func(engine syncp.EngineRunner, logger *slog.Logger) appRunner {
			return syncp.NewRunner(engine, logger)
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
	dryRun := fs.Bool("dry-run", false, "Validate database connection without starting")
	version := fs.Bool("version", false, "Print version information and exit")

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("failed to parse args: %w", err)
	}

	// Handle version flag
	if *version {
		fmt.Printf("vaults-syncer version %s\n", Version)
		if BuildDate != "unknown" {
			fmt.Printf("Build date: %s\n", BuildDate)
		}
		if GitCommit != "unknown" {
			fmt.Printf("Git commit: %s\n", GitCommit)
		}
		return nil
	}

	// Setup logger
	logLevel := slog.LevelInfo
	opts := &slog.HandlerOptions{Level: logLevel}
	logHandler := slog.NewJSONHandler(os.Stdout, opts)
	logger := slog.New(logHandler)

	// Resolve database configuration from environment.
	dbCfg := storage.DBConfigFromEnv()
	logger.Info("initializing database",
		slog.String("db_type", string(dbCfg.Type)),
		slog.String("db_path", dbCfg.Path),
	)

	store, err := deps.newStore(dbCfg)
	if err != nil {
		logger.Error("failed to initialize database", slog.String("error", err.Error()))
		return err
	}
	defer store.Close()

	// Resolve or generate the master encryption key.
	encryptor, err := resolveEncryptor(store, logger)
	if err != nil {
		return err
	}
	store.ConfigStore.SetEncryptor(encryptor)

	// Dry-run mode: validate DB connection only.
	if *dryRun {
		logger.Info("dry-run mode: database connection validated successfully")
		return nil
	}

	// Auth service.
	authSvc := auth.NewService(store)

	// Server config from env with sensible defaults.
	serverCfg := serverConfigFromEnv()
	loggingCfg := config.LoggingConfig{Level: "info", Format: "json"}

	// Load initial config from database (may be empty on first start).
	initialCfg, err := store.LoadConfig(serverCfg, loggingCfg)
	if err != nil {
		logger.Error("failed to load config from database", slog.String("error", err.Error()))
		return err
	}

	// Create sync engine and runner with the initial (possibly empty) config.
	engine, err := deps.newEngine(initialCfg, store, logger)
	if err != nil {
		logger.Error("failed to create sync engine", slog.String("error", err.Error()))
		return err
	}
	runner := deps.newRunner(engine, logger)

	// apiHandler is shared across route registrations; its config is updated dynamically.
	apiHandler := api.NewHandler(runner, store, initialCfg, logger)

	// mu protects concurrent calls to reloadConfig.
	var reloadMu sync.Mutex

	// reloadConfig reloads the sync engine and schedules from the database.
	reloadConfig := func() {
		reloadMu.Lock()
		defer reloadMu.Unlock()

		cfg, err := store.LoadConfig(serverCfg, loggingCfg)
		if err != nil {
			logger.Error("config reload failed", slog.String("error", err.Error()))
			return
		}

		runner.Stop()

		newEngine, err := deps.newEngine(cfg, store, logger)
		if err != nil {
			logger.Error("engine reload failed", slog.String("error", err.Error()))
			return
		}

		newRunner := deps.newRunner(newEngine, logger)
		if err := newRunner.Start(cfg); err != nil {
			logger.Error("runner restart failed", slog.String("error", err.Error()))
			return
		}

		apiHandler.SetConfig(cfg)
		logger.Info("configuration reloaded from database",
			slog.Int("vaults", len(cfg.Vaults)),
			slog.Int("syncs", len(cfg.Syncs)),
		)
	}

	// Setup HTTP server.
	mux := http.NewServeMux()

	// Setup + auth handlers (no authentication required).
	setupHandler := api.NewSetupHandler(store, authSvc, logger, reloadConfig)
	authApiHandler := api.NewAuthHandler(authSvc, logger)

	// Authenticated API sub-mux (read-only runtime endpoints).
	requireAuth := auth.RequireAuth(authSvc)
	requireAdmin := auth.RequireAdmin(authSvc)

	authedAPIMux := http.NewServeMux()
	authedAPIMux.HandleFunc("POST /api/auth/logout", authApiHandler.Logout)
	authedAPIMux.HandleFunc("GET /api/auth/me", authApiHandler.Me)
	authedAPIMux.HandleFunc("GET /health", apiHandler.Health)
	authedAPIMux.HandleFunc("GET /vaults", apiHandler.ListVaults)
	authedAPIMux.HandleFunc("GET /syncs", apiHandler.ListSyncs)
	authedAPIMux.HandleFunc("GET /syncs/{sync_id}/status", apiHandler.GetSyncStatus)
	authedAPIMux.HandleFunc("GET /syncs/{sync_id}/runs", apiHandler.GetSyncRuns)
	authedAPIMux.HandleFunc("POST /syncs/{sync_id}/execute", apiHandler.ExecuteSync)
	authedAPIMux.HandleFunc("GET /metrics", apiHandler.GetMetrics)

	// Admin-only API sub-mux.
	adminMux := http.NewServeMux()
	configHandler := api.NewConfigHandler(store, logger, reloadConfig)
	adminMux.HandleFunc("GET /api/config/vaults", configHandler.ListVaultsConfig)
	adminMux.HandleFunc("POST /api/config/vaults", configHandler.CreateVault)
	adminMux.HandleFunc("GET /api/config/vaults/{vault_id}", configHandler.GetVaultConfig)
	adminMux.HandleFunc("PUT /api/config/vaults/{vault_id}", configHandler.UpdateVault)
	adminMux.HandleFunc("DELETE /api/config/vaults/{vault_id}", configHandler.DeleteVaultConfig)

	adminMux.HandleFunc("GET /api/config/syncs", configHandler.ListSyncsConfig)
	adminMux.HandleFunc("POST /api/config/syncs", configHandler.CreateSync)
	adminMux.HandleFunc("GET /api/config/syncs/{sync_id}", configHandler.GetSyncConfig)
	adminMux.HandleFunc("PUT /api/config/syncs/{sync_id}", configHandler.UpdateSync)
	adminMux.HandleFunc("DELETE /api/config/syncs/{sync_id}", configHandler.DeleteSyncConfig)

	usersHandler := api.NewUsersHandler(authSvc, logger)
	adminMux.HandleFunc("GET /api/users", usersHandler.ListUsers)
	adminMux.HandleFunc("POST /api/users", usersHandler.CreateUser)
	adminMux.HandleFunc("PUT /api/users/{user_id}", usersHandler.UpdateUser)
	adminMux.HandleFunc("DELETE /api/users/{user_id}", usersHandler.DeleteUserAccount)

	// Mount all routes on the main mux.
	// Public routes (no auth required).
	mux.HandleFunc("GET /api/setup", setupHandler.GetSetupStatus)
	mux.HandleFunc("POST /api/setup", setupHandler.CompleteSetup)
	mux.HandleFunc("POST /api/auth/login", authApiHandler.Login)

	// Protected routes.
	mux.Handle("/api/config/", requireAuth(requireAdmin(adminMux)))
	mux.Handle("/api/users", requireAuth(requireAdmin(adminMux)))
	mux.Handle("/api/users/", requireAuth(requireAdmin(adminMux)))
	mux.Handle("/api/", requireAuth(authedAPIMux))

	// Web UI (served publicly — authentication is enforced by the frontend JavaScript).
	mux.Handle("/", api.ServeUI())

	// Start sync runner only if there is config to run.
	if len(initialCfg.Syncs) > 0 {
		if err := runner.Start(initialCfg); err != nil {
			logger.Error("failed to start sync runner", slog.String("error", err.Error()))
			return err
		}
	} else {
		logger.Info("no sync configurations found in database; waiting for setup")
	}

	// Start HTTP server.
	addr := fmt.Sprintf("%s:%d", serverCfg.Address, serverCfg.Port)
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
	if serverCfg.MetricsPort != serverCfg.Port {
		metricsMux := http.NewServeMux()
		metricsMux.HandleFunc("GET /metrics", apiHandler.GetMetrics)

		metricsAddr := fmt.Sprintf("%s:%d", serverCfg.MetricsAddress, serverCfg.MetricsPort)
		metricsServer = deps.newServer(metricsAddr, metricsMux)

		logger.Info("starting metrics server", slog.String("address", metricsAddr))
		go func() {
			if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				logger.Error("metrics server error", slog.String("error", err.Error()))
				serverErrs <- err
			}
		}()
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

// resolveEncryptor creates an AESEncryptor from the MASTER_ENCRYPTION_KEY env variable.
// On first start (key not set), it generates a new key, prints it, and stores a marker
// in the database so future starts can detect a missing key.
func resolveEncryptor(store *storage.Store, logger *slog.Logger) (*security.AESEncryptor, error) {
	keyStr := os.Getenv("MASTER_ENCRYPTION_KEY")
	if keyStr == "" {
		// Check if a key has been initialised before (marker in DB).
		_, keyExists, err := store.GetSetting("master_key_initialised")
		if err != nil {
			return nil, fmt.Errorf("failed to check master key status: %w", err)
		}
		if keyExists {
			return nil, fmt.Errorf(
				"MASTER_ENCRYPTION_KEY environment variable is required but not set.\n" +
					"Set it to the key that was generated and printed on first start.\n" +
					"If you have lost the key, you must reset the database.",
			)
		}

		// First start: generate and print the key.
		generated, err := security.GenerateMasterKey()
		if err != nil {
			return nil, fmt.Errorf("failed to generate master key: %w", err)
		}

		fmt.Println("╔══════════════════════════════════════════════════════════════╗")
		fmt.Println("║              MASTER ENCRYPTION KEY – SAVE THIS NOW          ║")
		fmt.Println("╠══════════════════════════════════════════════════════════════╣")
		fmt.Printf("║  %s  ║\n", generated)
		fmt.Println("╠══════════════════════════════════════════════════════════════╣")
		fmt.Println("║  Set MASTER_ENCRYPTION_KEY=<above> before restarting.       ║")
		fmt.Println("║  Losing this key means losing access to encrypted secrets.  ║")
		fmt.Println("╚══════════════════════════════════════════════════════════════╝")

		if err := store.SetSetting("master_key_initialised", "true"); err != nil {
			return nil, fmt.Errorf("failed to persist key marker: %w", err)
		}
		keyStr = generated
		logger.Warn("first-run: generated master encryption key (printed above)")
	}

	keyBytes, err := security.MasterKeyFromString(keyStr)
	if err != nil {
		return nil, fmt.Errorf("invalid MASTER_ENCRYPTION_KEY: %w", err)
	}

	enc, err := security.NewAESEncryptor(keyBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to create encryptor: %w", err)
	}
	return enc, nil
}

// serverConfigFromEnv builds a ServerConfig from env vars with sensible defaults.
func serverConfigFromEnv() config.ServerConfig {
	port := 8080
	metricsPort := 9090
	addr := "0.0.0.0"

	if v := os.Getenv("SERVER_PORT"); v != "" {
		if p := parseInt(v, port); p > 0 {
			port = p
		}
	}
	if v := os.Getenv("METRICS_PORT"); v != "" {
		if p := parseInt(v, metricsPort); p > 0 {
			metricsPort = p
		}
	}
	if v := os.Getenv("SERVER_ADDRESS"); v != "" {
		addr = v
	}

	return config.ServerConfig{
		Port:           port,
		Address:        addr,
		MetricsPort:    metricsPort,
		MetricsAddress: addr,
	}
}

func parseInt(s string, defaultVal int) int {
	var n int
	if _, err := fmt.Sscanf(s, "%d", &n); err != nil {
		return defaultVal
	}
	return n
}
