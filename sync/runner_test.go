package sync

import (
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/pacorreia/vaults-syncer/config"
	"github.com/pacorreia/vaults-syncer/storage"
)

type mockEngine struct {
	executeErr error
	callCount  int
	lastSyncID string
}

func (m *mockEngine) ExecuteSync(syncCfg *config.SyncConfig) error {
	m.callCount++
	m.lastSyncID = syncCfg.ID
	return m.executeErr
}

func setupTestRunner(t *testing.T) (*Runner, *storage.Store, *mockEngine) {
	t.Helper()

	store, err := storage.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create test store: %v", err)
	}

	engine := &mockEngine{}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	runner := NewRunner(engine, logger)
	return runner, store, engine
}

func TestNewRunner(t *testing.T) {
	runner, _, _ := setupTestRunner(t)
	if runner == nil {
		t.Fatal("expected runner to be created")
	}
	if runner.engine == nil {
		t.Error("expected engine to be set")
	}
	if runner.cron == nil {
		t.Error("expected cron to be set")
	}
	if runner.syncMap == nil {
		t.Error("expected syncMap to be initialized")
	}
	if runner.logger == nil {
		t.Error("expected logger to be set")
	}
	if runner.running {
		t.Error("expected running to be false initially")
	}
}

func TestRunner_Start(t *testing.T) {
	runner, _, _ := setupTestRunner(t)

	cfg := &config.Config{
		Syncs: []config.SyncConfig{
			{ID: "sync1", Enabled: true, Schedule: "*/5 * * * *", Source: "vault1", Targets: []string{"vault2"}},
		},
	}

	if err := runner.Start(cfg); err != nil {
		t.Fatalf("failed to start runner: %v", err)
	}
	defer runner.Stop()

	if !runner.IsRunning() {
		t.Error("expected runner to be running")
	}
	if len(runner.syncMap) != 1 {
		t.Errorf("expected 1 scheduled sync, got %d", len(runner.syncMap))
	}
}

func TestRunner_Start_AlreadyRunning(t *testing.T) {
	runner, _, _ := setupTestRunner(t)

	cfg := &config.Config{
		Syncs: []config.SyncConfig{
			{ID: "sync1", Enabled: true, Schedule: "*/5 * * * *", Source: "vault1", Targets: []string{"vault2"}},
		},
	}

	if err := runner.Start(cfg); err != nil {
		t.Fatalf("failed to start runner: %v", err)
	}
	defer runner.Stop()

	if err := runner.Start(cfg); err == nil {
		t.Fatal("expected error when starting twice")
	}
}

func TestRunner_Start_DisabledSync(t *testing.T) {
	runner, _, _ := setupTestRunner(t)

	cfg := &config.Config{
		Syncs: []config.SyncConfig{
			{ID: "sync1", Enabled: false, Schedule: "*/5 * * * *", Source: "vault1", Targets: []string{"vault2"}},
		},
	}

	if err := runner.Start(cfg); err != nil {
		t.Fatalf("failed to start runner: %v", err)
	}
	defer runner.Stop()

	if len(runner.syncMap) != 0 {
		t.Errorf("expected 0 scheduled syncs, got %d", len(runner.syncMap))
	}
}

func TestRunner_Start_NoSchedule(t *testing.T) {
	runner, _, _ := setupTestRunner(t)

	cfg := &config.Config{
		Syncs: []config.SyncConfig{
			{ID: "sync1", Enabled: true, Schedule: "", Source: "vault1", Targets: []string{"vault2"}},
		},
	}

	if err := runner.Start(cfg); err != nil {
		t.Fatalf("failed to start runner: %v", err)
	}
	defer runner.Stop()

	if len(runner.syncMap) != 0 {
		t.Errorf("expected 0 scheduled syncs, got %d", len(runner.syncMap))
	}
}

func TestRunner_Start_InvalidSchedule(t *testing.T) {
	runner, _, _ := setupTestRunner(t)

	cfg := &config.Config{
		Syncs: []config.SyncConfig{
			{ID: "sync1", Enabled: true, Schedule: "not a cron", Source: "vault1", Targets: []string{"vault2"}},
		},
	}

	if err := runner.Start(cfg); err == nil {
		t.Fatal("expected error for invalid schedule")
	}
}

func TestRunner_Stop(t *testing.T) {
	runner, _, _ := setupTestRunner(t)

	cfg := &config.Config{
		Syncs: []config.SyncConfig{
			{ID: "sync1", Enabled: true, Schedule: "*/5 * * * *", Source: "vault1", Targets: []string{"vault2"}},
		},
	}

	if err := runner.Start(cfg); err != nil {
		t.Fatalf("failed to start runner: %v", err)
	}

	runner.Stop()

	if runner.IsRunning() {
		t.Error("expected runner to be stopped")
	}
}

func TestRunner_Stop_NotRunning(t *testing.T) {
	runner, _, _ := setupTestRunner(t)

	runner.Stop()

	if runner.IsRunning() {
		t.Error("expected runner to remain stopped")
	}
}

func TestRunner_ExecuteSyncNow(t *testing.T) {
	runner, _, engine := setupTestRunner(t)

	cfg := &config.Config{
		Syncs: []config.SyncConfig{
			{ID: "sync1", Enabled: true, Source: "vault1", Targets: []string{"vault2"}},
		},
	}

	if err := runner.ExecuteSyncNow("sync1", cfg); err != nil {
		t.Fatalf("failed to execute sync: %v", err)
	}

	if engine.callCount != 1 {
		t.Errorf("expected 1 engine call, got %d", engine.callCount)
	}
	if engine.lastSyncID != "sync1" {
		t.Errorf("expected lastSyncID sync1, got %s", engine.lastSyncID)
	}
}

func TestRunner_ExecuteSyncNow_NotFound(t *testing.T) {
	runner, _, _ := setupTestRunner(t)

	cfg := &config.Config{
		Syncs: []config.SyncConfig{
			{ID: "sync1", Enabled: true, Source: "vault1", Targets: []string{"vault2"}},
		},
	}

	if err := runner.ExecuteSyncNow("missing", cfg); err == nil {
		t.Fatal("expected error for missing sync")
	}
}

func TestRunner_ExecuteSyncNow_EngineError(t *testing.T) {
	runner, _, engine := setupTestRunner(t)
	engine.executeErr = fmt.Errorf("engine error")

	cfg := &config.Config{
		Syncs: []config.SyncConfig{
			{ID: "sync1", Enabled: true, Source: "vault1", Targets: []string{"vault2"}},
		},
	}

	if err := runner.ExecuteSyncNow("sync1", cfg); err == nil {
		t.Fatal("expected engine error")
	}
}

func TestRunner_GetSyncStatus(t *testing.T) {
	runner, store, _ := setupTestRunner(t)
	defer store.Close()

	syncID := "sync1"

	if err := store.RecordSyncRun(syncID, "success", 2, 0, 123, ""); err != nil {
		t.Fatalf("failed to record sync run: %v", err)
	}

	obj1 := &config.SyncObject{
		SyncID:         syncID,
		SourceVaultID:  "vault1",
		TargetVaultID:  "vault2",
		SecretName:     "secret1",
		LastSyncStatus: "success",
		LastSyncTime:   time.Now().Unix(),
	}
	obj2 := &config.SyncObject{
		SyncID:         syncID,
		SourceVaultID:  "vault1",
		TargetVaultID:  "vault2",
		SecretName:     "secret2",
		LastSyncStatus: "failed",
		LastSyncTime:   time.Now().Unix(),
	}
	store.UpsertSyncObject(obj1)
	store.UpsertSyncObject(obj2)

	status, err := runner.GetSyncStatus(syncID, store)
	if err != nil {
		t.Fatalf("failed to get sync status: %v", err)
	}

	if status["total_objects"] != 2 {
		t.Errorf("expected total_objects 2, got %v", status["total_objects"])
	}
	if status["synced_objects"] != 1 {
		t.Errorf("expected synced_objects 1, got %v", status["synced_objects"])
	}
	if status["failed_objects"] != 1 {
		t.Errorf("expected failed_objects 1, got %v", status["failed_objects"])
	}
}

func TestRunner_GetSyncStatus_StoreError(t *testing.T) {
	runner, store, _ := setupTestRunner(t)
	store.Close()

	if _, err := runner.GetSyncStatus("sync1", store); err == nil {
		t.Fatal("expected error when store is closed")
	}
}

func TestRunner_GetEntries(t *testing.T) {
	runner, _, _ := setupTestRunner(t)

	cfg := &config.Config{
		Syncs: []config.SyncConfig{
			{ID: "sync1", Enabled: true, Schedule: "*/5 * * * *", Source: "vault1", Targets: []string{"vault2"}},
			{ID: "sync2", Enabled: true, Schedule: "*/10 * * * *", Source: "vault2", Targets: []string{"vault1"}},
		},
	}

	if err := runner.Start(cfg); err != nil {
		t.Fatalf("failed to start runner: %v", err)
	}
	defer runner.Stop()

	entries := runner.GetEntries()
	if len(entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(entries))
	}
}
