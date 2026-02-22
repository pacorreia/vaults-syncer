package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/pacorreia/vaults-syncer/config"
	"github.com/pacorreia/vaults-syncer/storage"
	"github.com/pacorreia/vaults-syncer/sync"
)

type fakeEngine struct{}

func (f *fakeEngine) ExecuteSync(cfg *config.SyncConfig) error {
	return nil
}

type mockRunner struct {
	startErr    error
	startCalled bool
	stopCalled  bool
}

func (m *mockRunner) Start(cfg *config.Config) error {
	m.startCalled = true
	return m.startErr
}

func (m *mockRunner) Stop() {
	m.stopCalled = true
}

func (m *mockRunner) IsRunning() bool {
	return m.startCalled && !m.stopCalled
}

func (m *mockRunner) GetSyncStatus(syncID string, store *storage.Store) (map[string]interface{}, error) {
	return map[string]interface{}{"sync_id": syncID}, nil
}

func (m *mockRunner) ExecuteSyncNow(syncID string, cfg *config.Config) error {
	return nil
}

type mockServer struct {
	listenErr      error
	shutdownErr    error
	listenCalled   bool
	shutdownCalled bool
}

func (m *mockServer) ListenAndServe() error {
	m.listenCalled = true
	return m.listenErr
}

func (m *mockServer) Shutdown(ctx context.Context) error {
	m.shutdownCalled = true
	return m.shutdownErr
}

type serverFactory struct {
	listenErrs   []error
	shutdownErrs []error
	servers      []*mockServer
}

func (s *serverFactory) newServer(addr string, handler http.Handler) httpServer {
	server := &mockServer{}
	if len(s.listenErrs) > 0 {
		server.listenErr = s.listenErrs[0]
		s.listenErrs = s.listenErrs[1:]
	}
	if len(s.shutdownErrs) > 0 {
		server.shutdownErr = s.shutdownErrs[0]
		s.shutdownErrs = s.shutdownErrs[1:]
	}
	s.servers = append(s.servers, server)
	return server
}

func baseConfig() *config.Config {
	return &config.Config{
		Server: config.ServerConfig{
			Address:        "127.0.0.1",
			Port:           8080,
			MetricsAddress: "127.0.0.1",
			MetricsPort:    8080,
		},
		Vaults: []config.VaultConfig{},
		Syncs:  []config.SyncConfig{},
	}
}

func TestRun_ConfigLoadError(t *testing.T) {
	deps := defaultDeps()
	deps.loadConfig = func(path string) (*config.Config, error) {
		return nil, errors.New("load error")
	}

	if err := run([]string{}, deps); err == nil {
		t.Fatal("expected error")
	}
}

func TestRun_StoreError(t *testing.T) {
	deps := defaultDeps()
	deps.loadConfig = func(path string) (*config.Config, error) {
		return baseConfig(), nil
	}
	deps.newStore = func(path string) (*storage.Store, error) {
		return nil, errors.New("store error")
	}

	if err := run([]string{}, deps); err == nil {
		t.Fatal("expected error")
	}
}

func TestRun_EngineError(t *testing.T) {
	deps := defaultDeps()
	deps.loadConfig = func(path string) (*config.Config, error) {
		return baseConfig(), nil
	}
	deps.newStore = func(path string) (*storage.Store, error) {
		return storage.NewStore(":memory:")
	}
	deps.newEngine = func(cfg *config.Config, store *storage.Store, logger *slog.Logger) (sync.EngineRunner, error) {
		return nil, errors.New("engine error")
	}

	if err := run([]string{}, deps); err == nil {
		t.Fatal("expected error")
	}
}

func TestRun_DryRun(t *testing.T) {
	deps := defaultDeps()
	deps.loadConfig = func(path string) (*config.Config, error) {
		return baseConfig(), nil
	}
	deps.newStore = func(path string) (*storage.Store, error) {
		return storage.NewStore(":memory:")
	}
	deps.newEngine = func(cfg *config.Config, store *storage.Store, logger *slog.Logger) (sync.EngineRunner, error) {
		return &fakeEngine{}, nil
	}

	var runnerCalled bool
	deps.newRunner = func(engine sync.EngineRunner, logger *slog.Logger) appRunner {
		runnerCalled = true
		return &mockRunner{}
	}

	var serverCalled bool
	deps.newServer = func(addr string, handler http.Handler) httpServer {
		serverCalled = true
		return &mockServer{}
	}

	if err := run([]string{"-dry-run"}, deps); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if runnerCalled || serverCalled {
		t.Fatal("expected no runner or server creation in dry-run")
	}
}

func TestRun_RunnerStartError(t *testing.T) {
	deps := defaultDeps()
	deps.loadConfig = func(path string) (*config.Config, error) {
		return baseConfig(), nil
	}
	deps.newStore = func(path string) (*storage.Store, error) {
		return storage.NewStore(":memory:")
	}
	deps.newEngine = func(cfg *config.Config, store *storage.Store, logger *slog.Logger) (sync.EngineRunner, error) {
		return &fakeEngine{}, nil
	}
	deps.newRunner = func(engine sync.EngineRunner, logger *slog.Logger) appRunner {
		return &mockRunner{startErr: errors.New("start error")}
	}
	deps.newServer = func(addr string, handler http.Handler) httpServer {
		return &mockServer{listenErr: http.ErrServerClosed}
	}
	deps.waitForSignal = func() <-chan os.Signal {
		return make(chan os.Signal)
	}

	if err := run([]string{}, deps); err == nil {
		t.Fatal("expected error")
	}
}

func TestRun_ServerError(t *testing.T) {
	deps := defaultDeps()
	deps.loadConfig = func(path string) (*config.Config, error) {
		return baseConfig(), nil
	}
	deps.newStore = func(path string) (*storage.Store, error) {
		return storage.NewStore(":memory:")
	}
	deps.newEngine = func(cfg *config.Config, store *storage.Store, logger *slog.Logger) (sync.EngineRunner, error) {
		return &fakeEngine{}, nil
	}
	deps.newRunner = func(engine sync.EngineRunner, logger *slog.Logger) appRunner {
		return &mockRunner{}
	}
	deps.newServer = func(addr string, handler http.Handler) httpServer {
		return &mockServer{listenErr: fmt.Errorf("server error")}
	}
	deps.waitForSignal = func() <-chan os.Signal {
		return make(chan os.Signal)
	}

	if err := run([]string{}, deps); err == nil {
		t.Fatal("expected error")
	}
}

func TestRun_Shutdown(t *testing.T) {
	cfg := baseConfig()
	cfg.Server.MetricsPort = 9090
	cfg.Server.MetricsAddress = "127.0.0.1"

	deps := defaultDeps()
	deps.loadConfig = func(path string) (*config.Config, error) {
		return cfg, nil
	}
	deps.newStore = func(path string) (*storage.Store, error) {
		return storage.NewStore(":memory:")
	}
	deps.newEngine = func(cfg *config.Config, store *storage.Store, logger *slog.Logger) (sync.EngineRunner, error) {
		return &fakeEngine{}, nil
	}
	mockRun := &mockRunner{}
	deps.newRunner = func(engine sync.EngineRunner, logger *slog.Logger) appRunner {
		return mockRun
	}

	factory := &serverFactory{listenErrs: []error{http.ErrServerClosed, http.ErrServerClosed}}
	deps.newServer = factory.newServer

	sigCh := make(chan os.Signal, 1)
	deps.waitForSignal = func() <-chan os.Signal {
		return sigCh
	}

	go func() {
		time.Sleep(10 * time.Millisecond)
		sigCh <- syscall.SIGTERM
	}()

	if err := run([]string{}, deps); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !mockRun.stopCalled {
		t.Error("expected runner to stop on shutdown")
	}
	if len(factory.servers) != 2 {
		t.Fatalf("expected 2 servers, got %d", len(factory.servers))
	}
	if !factory.servers[0].shutdownCalled || !factory.servers[1].shutdownCalled {
		t.Fatal("expected both servers to shut down")
	}
}

func TestMainSuccess(t *testing.T) {
	origRun := runFunc
	origExit := exitFunc
	defer func() {
		runFunc = origRun
		exitFunc = origExit
	}()

	runFunc = func(args []string, deps appDeps) error {
		return nil
	}

	exitCalled := false
	exitFunc = func(code int) {
		exitCalled = true
	}

	main()

	if exitCalled {
		t.Fatal("expected main to avoid exit on success")
	}
}

func TestMainError(t *testing.T) {
	origRun := runFunc
	origExit := exitFunc
	defer func() {
		runFunc = origRun
		exitFunc = origExit
	}()

	runFunc = func(args []string, deps appDeps) error {
		return errors.New("run error")
	}

	exitCode := 0
	exitFunc = func(code int) {
		exitCode = code
	}

	main()

	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", exitCode)
	}
}

func TestDefaultDeps(t *testing.T) {
	deps := defaultDeps()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	configContent := fmt.Sprintf(`
vaults:
  - id: vault1
    type: generic
    endpoint: %s
    auth:
      method: custom
      headers: {}
    field_names:
      name_field: name
      value_field: value
syncs: []
`, server.URL)

	configPath := t.TempDir() + "/config.yaml"
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := deps.loadConfig(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	store, err := deps.newStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	engine, err := deps.newEngine(cfg, store, slogLogger())
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	_ = deps.newRunner(engine, slogLogger())
	_ = deps.newServer("127.0.0.1:0", http.NewServeMux())
	_ = deps.waitForSignal()
}

func slogLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
}
