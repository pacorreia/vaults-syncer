package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/pacorreia/vaults-syncer/config"
	"github.com/pacorreia/vaults-syncer/storage"
)

type mockRunner struct {
	running    bool
	statuses   map[string]map[string]interface{}
	execCalls  []string
	executeErr error
	statusErr  error
	execCh     chan string
}

func (m *mockRunner) IsRunning() bool {
	return m.running
}

func (m *mockRunner) GetSyncStatus(syncID string, store *storage.Store) (map[string]interface{}, error) {
	if m.statusErr != nil {
		return nil, m.statusErr
	}
	if status, ok := m.statuses[syncID]; ok {
		return status, nil
	}
	return map[string]interface{}{
		"sync_id":        syncID,
		"total_objects":  0,
		"synced_objects": 0,
		"failed_objects": 0,
	}, nil
}

func (m *mockRunner) ExecuteSyncNow(syncID string, cfg *config.Config) error {
	m.execCalls = append(m.execCalls, syncID)
	if m.execCh != nil {
		m.execCh <- syncID
	}
	return m.executeErr
}

func setupTestHandler(t *testing.T) (*Handler, *mockRunner, *storage.Store) {
	t.Helper()

	store, err := storage.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create test store: %v", err)
	}

	cfg := &config.Config{
		Vaults: []config.VaultConfig{
			{ID: "vault1", Endpoint: "http://vault1.example.com"},
			{ID: "vault2", Endpoint: "http://vault2.example.com"},
		},
		Syncs: []config.SyncConfig{
			{ID: "sync1", Source: "vault1", Targets: []string{"vault2"}, Enabled: true, SyncType: "unidirectional"},
			{ID: "sync2", Source: "vault2", Targets: []string{"vault1"}, Enabled: false, SyncType: "bidirectional"},
		},
	}

	runner := &mockRunner{
		running: true,
		execCh:  make(chan string, 1),
		statuses: map[string]map[string]interface{}{
			"sync1": {
				"sync_id":        "sync1",
				"total_objects":  3,
				"synced_objects": 2,
				"failed_objects": 1,
			},
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	return NewHandler(runner, store, cfg, logger), runner, store
}

func TestNewHandler(t *testing.T) {
	handler, _, store := setupTestHandler(t)
	defer store.Close()

	if handler == nil {
		t.Fatal("expected handler to be created")
	}
	if handler.runner == nil {
		t.Error("expected runner to be set")
	}
	if handler.store == nil {
		t.Error("expected store to be set")
	}
	if handler.cfg == nil {
		t.Error("expected cfg to be set")
	}
	if handler.logger == nil {
		t.Error("expected logger to be set")
	}
}

func TestHealth(t *testing.T) {
	handler, _, store := setupTestHandler(t)
	defer store.Close()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	handler.Health(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response["status"] != "healthy" {
		t.Errorf("expected status 'healthy', got %v", response["status"])
	}

	if response["running"] != true {
		t.Errorf("expected running true, got %v", response["running"])
	}

	if response["syncs"] != float64(2) {
		t.Errorf("expected 2 syncs, got %v", response["syncs"])
	}
	if response["vaults"] != float64(2) {
		t.Errorf("expected 2 vaults, got %v", response["vaults"])
	}
}

func TestGetSyncStatus(t *testing.T) {
	handler, _, store := setupTestHandler(t)
	defer store.Close()

	req := httptest.NewRequest(http.MethodGet, "/syncs/sync1/status", nil)
	req.SetPathValue("sync_id", "sync1")
	w := httptest.NewRecorder()

	handler.GetSyncStatus(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response["sync_id"] != "sync1" {
		t.Errorf("expected sync_id 'sync1', got %v", response["sync_id"])
	}

	if response["total_objects"] != float64(3) {
		t.Errorf("expected total_objects 3, got %v", response["total_objects"])
	}
}

func TestGetSyncStatus_MissingID(t *testing.T) {
	handler, _, store := setupTestHandler(t)
	defer store.Close()

	req := httptest.NewRequest(http.MethodGet, "/syncs//status", nil)
	w := httptest.NewRecorder()

	handler.GetSyncStatus(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestGetSyncStatus_Error(t *testing.T) {
	handler, runner, store := setupTestHandler(t)
	defer store.Close()

	runner.statusErr = errTest

	req := httptest.NewRequest(http.MethodGet, "/syncs/sync1/status", nil)
	req.SetPathValue("sync_id", "sync1")
	w := httptest.NewRecorder()

	handler.GetSyncStatus(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}
}

func TestListSyncs(t *testing.T) {
	handler, _, store := setupTestHandler(t)
	defer store.Close()

	req := httptest.NewRequest(http.MethodGet, "/syncs", nil)
	w := httptest.NewRecorder()

	handler.ListSyncs(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	syncs, ok := response["syncs"].([]interface{})
	if !ok {
		t.Fatal("expected syncs to be an array")
	}

	if len(syncs) != 2 {
		t.Errorf("expected 2 syncs, got %d", len(syncs))
	}
}

func TestExecuteSync(t *testing.T) {
	handler, runner, store := setupTestHandler(t)
	defer store.Close()

	req := httptest.NewRequest(http.MethodPost, "/syncs/sync1/execute", nil)
	req.SetPathValue("sync_id", "sync1")
	w := httptest.NewRecorder()

	handler.ExecuteSync(w, req)

	if w.Code != http.StatusAccepted {
		t.Errorf("expected status 202, got %d", w.Code)
	}

	select {
	case syncID := <-runner.execCh:
		if syncID != "sync1" {
			t.Errorf("expected sync1 execute call, got %s", syncID)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for ExecuteSyncNow")
	}
}

func TestExecuteSync_WrongMethod(t *testing.T) {
	handler, _, store := setupTestHandler(t)
	defer store.Close()

	req := httptest.NewRequest(http.MethodGet, "/syncs/sync1/execute", nil)
	req.SetPathValue("sync_id", "sync1")
	w := httptest.NewRecorder()

	handler.ExecuteSync(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}
}

func TestExecuteSync_MissingID(t *testing.T) {
	handler, _, store := setupTestHandler(t)
	defer store.Close()

	req := httptest.NewRequest(http.MethodPost, "/syncs//execute", nil)
	w := httptest.NewRecorder()

	handler.ExecuteSync(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestGetMetrics(t *testing.T) {
	handler, _, store := setupTestHandler(t)
	defer store.Close()

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()

	handler.GetMetrics(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "syncs_configured 2") {
		t.Errorf("expected syncs_configured 2, got:\n%s", body)
	}
	if !strings.Contains(body, "syncs_enabled 1") {
		t.Errorf("expected syncs_enabled 1, got:\n%s", body)
	}
	if !strings.Contains(body, "runner_running 1") {
		t.Errorf("expected runner_running 1, got:\n%s", body)
	}
}

var errTest = &testError{"test error"}

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}
