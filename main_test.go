package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/pacorreia/vaults-syncer/config"
	"github.com/pacorreia/vaults-syncer/security"
	"github.com/pacorreia/vaults-syncer/storage"
	syncp "github.com/pacorreia/vaults-syncer/sync"
)

// testMasterKey is a fixed base64-encoded 32-byte key for tests.
var testMasterKey string

func init() {
	k, err := security.GenerateMasterKey()
	if err != nil {
		panic(err)
	}
	testMasterKey = k
}

// setTestMasterKey sets the MASTER_ENCRYPTION_KEY env var for a test and
// restores it on cleanup.
func setTestMasterKey(t *testing.T) {
	t.Helper()
	old := os.Getenv("MASTER_ENCRYPTION_KEY")
	os.Setenv("MASTER_ENCRYPTION_KEY", testMasterKey)
	t.Cleanup(func() { os.Setenv("MASTER_ENCRYPTION_KEY", old) })
}

type fakeEngine struct{}

func (f *fakeEngine) ExecuteSync(cfg *config.SyncConfig) error { return nil }

type mockRunner struct {
	startErr    error
	startCalled bool
	stopCalled  bool
}

func (m *mockRunner) Start(cfg *config.Config) error {
	m.startCalled = true
	return m.startErr
}

func (m *mockRunner) Stop() { m.stopCalled = true }

func (m *mockRunner) IsRunning() bool { return m.startCalled && !m.stopCalled }

func (m *mockRunner) GetSyncStatus(syncID string, store *storage.Store) (map[string]interface{}, error) {
	return map[string]interface{}{"sync_id": syncID}, nil
}

func (m *mockRunner) ExecuteSyncNow(syncID string, cfg *config.Config) error { return nil }

func (m *mockRunner) GetNextRun(syncID string) *time.Time { return nil }

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

func inMemoryDeps(t *testing.T) appDeps {
	t.Helper()
	deps := defaultDeps()
	deps.newStore = func(cfg storage.DBConfig) (*storage.Store, error) {
		return storage.NewStore(":memory:")
	}
	deps.newEngine = func(cfg *config.Config, store *storage.Store, logger *slog.Logger) (syncp.EngineRunner, error) {
		return &fakeEngine{}, nil
	}
	return deps
}

func TestRun_StoreError(t *testing.T) {
	setTestMasterKey(t)
	deps := defaultDeps()
	deps.newStore = func(cfg storage.DBConfig) (*storage.Store, error) {
		return nil, errors.New("store error")
	}

	if err := run([]string{}, deps); err == nil {
		t.Fatal("expected error")
	}
}

func TestRun_EngineError(t *testing.T) {
	setTestMasterKey(t)
	deps := inMemoryDeps(t)
	deps.newEngine = func(cfg *config.Config, store *storage.Store, logger *slog.Logger) (syncp.EngineRunner, error) {
		return nil, errors.New("engine error")
	}

	if err := run([]string{}, deps); err == nil {
		t.Fatal("expected error")
	}
}

func TestRun_DryRun(t *testing.T) {
	setTestMasterKey(t)
	deps := inMemoryDeps(t)

	var runnerCalled, serverCalled bool
	deps.newRunner = func(engine syncp.EngineRunner, logger *slog.Logger) appRunner {
		runnerCalled = true
		return &mockRunner{}
	}
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
	setTestMasterKey(t)
	deps := inMemoryDeps(t)
	deps.newStore = func(cfg storage.DBConfig) (*storage.Store, error) {
		store, err := storage.NewStore(":memory:")
		if err != nil {
			return nil, err
		}
		// Pre-populate with a sync so the runner is started.
		enabled := true
		_ = store.SaveSync(config.SyncConfig{
			ID:       "s1",
			Source:   "v1",
			Targets:  []string{"v2"},
			Schedule: "*/5 * * * *",
			Enabled:  &enabled,
		})
		return store, nil
	}
	deps.newRunner = func(engine syncp.EngineRunner, logger *slog.Logger) appRunner {
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
	setTestMasterKey(t)
	deps := inMemoryDeps(t)
	deps.newRunner = func(engine syncp.EngineRunner, logger *slog.Logger) appRunner {
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
	setTestMasterKey(t)
	deps := inMemoryDeps(t)
	mockRun := &mockRunner{}
	deps.newRunner = func(engine syncp.EngineRunner, logger *slog.Logger) appRunner {
		return mockRun
	}

	factory := &serverFactory{listenErrs: []error{http.ErrServerClosed, http.ErrServerClosed}}
	deps.newServer = factory.newServer

	sigCh := make(chan os.Signal, 1)
	deps.waitForSignal = func() <-chan os.Signal { return sigCh }

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
}

func TestMainSuccess(t *testing.T) {
	origRun := runFunc
	origExit := exitFunc
	defer func() {
		runFunc = origRun
		exitFunc = origExit
	}()

	runFunc = func(args []string, deps appDeps) error { return nil }

	exitCalled := false
	exitFunc = func(code int) { exitCalled = true }

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

	runFunc = func(args []string, deps appDeps) error { return errors.New("run error") }

	exitCode := 0
	exitFunc = func(code int) { exitCode = code }

	main()

	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", exitCode)
	}
}

func TestDefaultDeps(t *testing.T) {
	deps := defaultDeps()

	store, err := deps.newStore(storage.DBConfig{Type: storage.DBTypeSQLite, Path: ":memory:"})
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	cfg := &config.Config{Vaults: []config.VaultConfig{}, Syncs: []config.SyncConfig{}}
	engine, err := deps.newEngine(cfg, store, slogLogger())
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	_ = deps.newRunner(engine, slogLogger())
	_ = deps.newServer("127.0.0.1:0", http.NewServeMux())
	_ = deps.waitForSignal()
}

func TestRun_Version(t *testing.T) {
	deps := defaultDeps()

	// Ensure these are NOT called when -version is used.
	deps.newStore = func(cfg storage.DBConfig) (*storage.Store, error) {
		t.Fatal("newStore should not be called with -version flag")
		return nil, nil
	}

	if err := run([]string{"-version"}, deps); err != nil {
		t.Fatalf("unexpected error with -version flag: %v", err)
	}
}

func slogLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
}
