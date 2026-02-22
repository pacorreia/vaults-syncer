package sync

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/pacorreia/vaults-syncer/config"
	"github.com/pacorreia/vaults-syncer/storage"
	"github.com/pacorreia/vaults-syncer/vault"
)

type mockBackend struct {
	secrets     map[string]string
	listError   error
	getError    error
	setError    error
	deleteError error
}

func (m *mockBackend) ListSecrets() ([]string, error) {
	if m.listError != nil {
		return nil, m.listError
	}
	keys := make([]string, 0, len(m.secrets))
	for k := range m.secrets {
		keys = append(keys, k)
	}
	return keys, nil
}

func (m *mockBackend) GetSecret(name string) (*vault.Secret, error) {
	if m.getError != nil {
		return nil, m.getError
	}
	if val, ok := m.secrets[name]; ok {
		return &vault.Secret{Name: name, Value: val}, nil
	}
	return nil, fmt.Errorf("secret not found: %s", name)
}

func (m *mockBackend) SetSecret(name, value string) error {
	if m.setError != nil {
		return m.setError
	}
	m.secrets[name] = value
	return nil
}

func (m *mockBackend) DeleteSecret(name string) error {
	if m.deleteError != nil {
		return m.deleteError
	}
	delete(m.secrets, name)
	return nil
}

func (m *mockBackend) TestConnection() error {
	return nil
}

func (m *mockBackend) Type() string {
	return "mock"
}

func (m *mockBackend) Capabilities() vault.BackendCapabilities {
	return vault.BackendCapabilities{
		CanList:   true,
		CanGet:    true,
		CanSet:    true,
		CanDelete: true,
		CanSync:   true,
	}
}

func setupEngine(t *testing.T) (*Engine, *storage.Store, *mockBackend, *mockBackend) {
	t.Helper()

	store, err := storage.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create test store: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	source := &mockBackend{secrets: map[string]string{"secret1": "value1", "secret2": "value2"}}
	target := &mockBackend{secrets: map[string]string{}}

	engine := &Engine{
		store:    store,
		backends: map[string]vault.Backend{"source": source, "target": target},
		logger:   logger,
	}

	return engine, store, source, target
}

func TestNewEngine_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	store, err := storage.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	cfg := &config.Config{
		Vaults: []config.VaultConfig{
			{
				ID:       "vault1",
				Endpoint: server.URL,
				Auth: &config.AuthConfig{
					Method:  "custom",
					Headers: map[string]string{},
				},
			},
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	engine, err := NewEngine(cfg, store, logger)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}
	if engine.backends["vault1"] == nil {
		t.Fatal("expected backend to be registered")
	}
}

func TestNewEngine_TestConnectionFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	store, err := storage.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	cfg := &config.Config{
		Vaults: []config.VaultConfig{
			{
				ID:       "vault1",
				Endpoint: server.URL,
				Auth: &config.AuthConfig{
					Method:  "custom",
					Headers: map[string]string{},
				},
			},
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	engine, err := NewEngine(cfg, store, logger)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}
	if engine.backends["vault1"] == nil {
		t.Fatal("expected backend to be registered even when connection fails")
	}
}

func TestNewEngineWithBackendFactory_ErrorSkip(t *testing.T) {
	store, err := storage.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	cfg := &config.Config{
		Vaults: []config.VaultConfig{
			{ID: "vault1", Endpoint: "https://example.com"},
			{ID: "vault2", Endpoint: "https://example.com"},
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	call := 0
	factory := func(v *config.VaultConfig) (vault.Backend, error) {
		call++
		if call == 1 {
			return nil, fmt.Errorf("backend error")
		}
		return &mockBackend{secrets: map[string]string{}}, nil
	}

	engine, err := newEngineWithBackendFactory(cfg, store, logger, factory)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(engine.backends) != 1 {
		t.Fatalf("expected 1 backend, got %d", len(engine.backends))
	}
}

type errBackend struct{}

func (e *errBackend) ListSecrets() ([]string, error)               { return nil, nil }
func (e *errBackend) GetSecret(name string) (*vault.Secret, error) { return nil, nil }
func (e *errBackend) SetSecret(name, value string) error           { return nil }
func (e *errBackend) DeleteSecret(name string) error               { return nil }
func (e *errBackend) TestConnection() error                        { return fmt.Errorf("conn error") }
func (e *errBackend) Type() string                                 { return "err" }
func (e *errBackend) Capabilities() vault.BackendCapabilities      { return vault.BackendCapabilities{} }

func TestNewEngineWithBackendFactory_TestConnectionError(t *testing.T) {
	store, err := storage.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	cfg := &config.Config{
		Vaults: []config.VaultConfig{{ID: "vault1", Endpoint: "https://example.com"}},
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	engine, err := newEngineWithBackendFactory(cfg, store, logger, func(v *config.VaultConfig) (vault.Backend, error) {
		return &errBackend{}, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if engine.backends["vault1"] == nil {
		t.Fatal("expected backend to be registered")
	}
}

func TestExecuteSync_SourceNotFound(t *testing.T) {
	engine, store, _, _ := setupEngine(t)
	defer store.Close()

	syncCfg := &config.SyncConfig{ID: "sync1", Source: "missing", Targets: []string{"target"}, SyncType: "unidirectional"}
	if err := engine.ExecuteSync(syncCfg); err == nil {
		t.Fatal("expected error for missing source")
	}
}

func TestExecuteSync_Unidirectional(t *testing.T) {
	engine, store, _, target := setupEngine(t)
	defer store.Close()

	syncCfg := &config.SyncConfig{ID: "sync1", Source: "source", Targets: []string{"target"}, SyncType: "unidirectional"}
	if err := engine.ExecuteSync(syncCfg); err != nil {
		t.Fatalf("failed to execute sync: %v", err)
	}

	if len(target.secrets) != 2 {
		t.Errorf("expected 2 secrets in target, got %d", len(target.secrets))
	}
}

func TestExecuteSync_ListSecretsError(t *testing.T) {
	engine, store, source, _ := setupEngine(t)
	defer store.Close()

	source.listError = fmt.Errorf("list error")

	syncCfg := &config.SyncConfig{ID: "sync1", Source: "source", Targets: []string{"target"}, SyncType: "unidirectional"}
	if err := engine.ExecuteSync(syncCfg); err == nil {
		t.Fatal("expected list error")
	}
}

func TestSyncSecretUnidirectional_Success(t *testing.T) {
	engine, store, _, target := setupEngine(t)
	defer store.Close()

	if err := engine.syncSecretUnidirectional("sync1", "source", "target", "secret1", engine.backends["source"]); err != nil {
		t.Fatalf("failed to sync secret: %v", err)
	}

	if target.secrets["secret1"] != "value1" {
		t.Errorf("expected secret1=value1, got %s", target.secrets["secret1"])
	}

	obj, err := store.GetSyncObject("sync1", "source", "target", "secret1")
	if err != nil {
		t.Fatalf("failed to get sync object: %v", err)
	}
	if obj == nil || obj.LastSyncStatus != "success" {
		t.Fatalf("expected success sync object")
	}
}

func TestSyncSecretUnidirectional_GetError(t *testing.T) {
	engine, store, source, _ := setupEngine(t)
	defer store.Close()

	source.getError = fmt.Errorf("get error")

	if err := engine.syncSecretUnidirectional("sync1", "source", "target", "secret1", engine.backends["source"]); err == nil {
		t.Fatal("expected get error")
	}
}

func TestSyncSecretUnidirectional_SetError(t *testing.T) {
	engine, store, _, target := setupEngine(t)
	defer store.Close()

	target.setError = fmt.Errorf("set error")

	if err := engine.syncSecretUnidirectional("sync1", "source", "target", "secret1", engine.backends["source"]); err == nil {
		t.Fatal("expected set error")
	}

	obj, err := store.GetSyncObject("sync1", "source", "target", "secret1")
	if err != nil {
		t.Fatalf("failed to get sync object: %v", err)
	}
	if obj == nil || obj.LastSyncStatus != "failed" {
		t.Fatal("expected failed sync object")
	}
}

func TestSyncSecretUnidirectional_TargetMissing(t *testing.T) {
	engine, store, _, _ := setupEngine(t)
	defer store.Close()

	if err := engine.syncSecretUnidirectional("sync1", "source", "missing", "secret1", engine.backends["source"]); err == nil {
		t.Fatal("expected error for missing target backend")
	}
}

func TestSyncSecretBidirectional_InSync(t *testing.T) {
	engine, store, source, target := setupEngine(t)
	defer store.Close()

	source.secrets["secret1"] = "same"
	target.secrets["secret1"] = "same"

	if err := engine.syncSecretBidirectional("sync1", "source", "target", "secret1", engine.backends["source"], engine.backends["target"]); err != nil {
		t.Fatalf("failed to sync secret: %v", err)
	}

	obj, err := store.GetSyncObject("sync1", "source", "target", "secret1")
	if err != nil {
		t.Fatalf("failed to get sync object: %v", err)
	}
	if obj == nil || obj.LastSyncStatus != "in_sync" {
		t.Fatalf("expected in_sync status")
	}
}

func TestSyncSecretBidirectional_TargetMissing(t *testing.T) {
	engine, store, _, target := setupEngine(t)
	defer store.Close()

	if err := engine.syncSecretBidirectional("sync1", "source", "target", "secret1", engine.backends["source"], engine.backends["target"]); err != nil {
		t.Fatalf("failed to sync secret: %v", err)
	}

	if target.secrets["secret1"] != "value1" {
		t.Errorf("expected secret1=value1, got %s", target.secrets["secret1"])
	}
}

func TestSyncSecretBidirectional_ExistingDirectionTargetToSource(t *testing.T) {
	engine, store, source, target := setupEngine(t)
	defer store.Close()

	source.secrets["secret1"] = "source-value"
	target.secrets["secret1"] = "target-value"

	existing := &config.SyncObject{
		SyncID:        "sync1",
		SourceVaultID: "source",
		TargetVaultID: "target",
		SecretName:    "secret1",
		DirectionLast: "target_to_source",
	}
	if err := store.UpsertSyncObject(existing); err != nil {
		t.Fatalf("failed to insert sync object: %v", err)
	}

	if err := engine.syncSecretBidirectional("sync1", "source", "target", "secret1", engine.backends["source"], engine.backends["target"]); err != nil {
		t.Fatalf("failed to sync secret: %v", err)
	}

	if target.secrets["secret1"] != "source-value" {
		t.Errorf("expected target to be updated from source, got %s", target.secrets["secret1"])
	}
}

func TestSyncSecretBidirectional_GetSyncObjectError(t *testing.T) {
	engine, store, source, target := setupEngine(t)
	defer store.Close()

	source.secrets["secret1"] = "source-value"
	target.secrets["secret1"] = "target-value"

	store.Close()
	if err := engine.syncSecretBidirectional("sync1", "source", "target", "secret1", engine.backends["source"], engine.backends["target"]); err == nil {
		t.Fatal("expected error when store is closed")
	}
}

func TestSyncSecretBidirectional_SourceToTarget(t *testing.T) {
	engine, store, source, target := setupEngine(t)
	defer store.Close()

	source.secrets["secret1"] = "source-value"
	target.secrets["secret1"] = "target-value"

	if err := engine.syncSecretBidirectional("sync1", "source", "target", "secret1", engine.backends["source"], engine.backends["target"]); err != nil {
		t.Fatalf("failed to sync secret: %v", err)
	}

	if target.secrets["secret1"] != "source-value" {
		t.Errorf("expected target to be updated from source, got %s", target.secrets["secret1"])
	}

	obj, err := store.GetSyncObject("sync1", "source", "target", "secret1")
	if err != nil {
		t.Fatalf("failed to get sync object: %v", err)
	}
	if obj == nil || obj.DirectionLast != "source_to_target" {
		t.Fatalf("expected direction source_to_target")
	}
}

func TestSyncSecretBidirectional_TargetToSource(t *testing.T) {
	engine, store, source, target := setupEngine(t)
	defer store.Close()

	source.secrets["secret1"] = "source-value"
	target.secrets["secret1"] = "target-value"

	// Existing sync object indicates last direction was source->target
	existing := &config.SyncObject{
		SyncID:        "sync1",
		SourceVaultID: "source",
		TargetVaultID: "target",
		SecretName:    "secret1",
		DirectionLast: "source_to_target",
	}
	if err := store.UpsertSyncObject(existing); err != nil {
		t.Fatalf("failed to insert sync object: %v", err)
	}

	if err := engine.syncSecretBidirectional("sync1", "source", "target", "secret1", engine.backends["source"], engine.backends["target"]); err != nil {
		t.Fatalf("failed to sync secret: %v", err)
	}

	if source.secrets["secret1"] != "target-value" {
		t.Errorf("expected source to be updated from target, got %s", source.secrets["secret1"])
	}
}

func TestSyncSecretBidirectional_SetError(t *testing.T) {
	engine, store, source, target := setupEngine(t)
	defer store.Close()

	source.secrets["secret1"] = "source-value"
	target.secrets["secret1"] = "target-value"
	target.setError = fmt.Errorf("set error")

	if err := engine.syncSecretBidirectional("sync1", "source", "target", "secret1", engine.backends["source"], engine.backends["target"]); err == nil {
		t.Fatal("expected set error")
	}

	obj, err := store.GetSyncObject("sync1", "source", "target", "secret1")
	if err != nil {
		t.Fatalf("failed to get sync object: %v", err)
	}
	if obj == nil || obj.LastSyncStatus != "failed" {
		t.Fatalf("expected failed sync object")
	}
}

func TestExecuteSync_Bidirectional_TargetNotFound(t *testing.T) {
	engine, store, _, _ := setupEngine(t)
	defer store.Close()

	syncCfg := &config.SyncConfig{ID: "sync1", Source: "source", Targets: []string{"missing"}, SyncType: "bidirectional"}
	if err := engine.ExecuteSync(syncCfg); err == nil {
		t.Fatal("expected error for missing target")
	}
}

func TestExecuteSync_Bidirectional(t *testing.T) {
	engine, store, source, target := setupEngine(t)
	defer store.Close()

	source.secrets["secret3"] = "value3"
	target.secrets["secret4"] = "value4"

	syncCfg := &config.SyncConfig{ID: "sync1", Source: "source", Targets: []string{"target"}, SyncType: "bidirectional"}
	if err := engine.ExecuteSync(syncCfg); err != nil {
		t.Fatalf("failed to execute sync: %v", err)
	}

	if len(source.secrets) == 0 || len(target.secrets) == 0 {
		t.Fatal("expected secrets to exist after bidirectional sync")
	}
}

func TestExecuteSyncBidirectionalConcurrent_Success(t *testing.T) {
	engine, store, _, _ := setupEngine(t)
	defer store.Close()

	syncCfg := &config.SyncConfig{ID: "sync1", Source: "source", Targets: []string{"target"}, SyncType: "bidirectional"}
	result := engine.executeSyncBidirectionalConcurrent(syncCfg, engine.backends["source"], engine.backends["target"], 2)
	if result.Success != 2 || result.Failure != 0 {
		t.Errorf("expected 2/0 results, got %d/%d", result.Success, result.Failure)
	}
}

func TestExecuteSyncBidirectionalConcurrent_ListError(t *testing.T) {
	engine, store, source, _ := setupEngine(t)
	defer store.Close()

	source.listError = fmt.Errorf("list error")
	syncCfg := &config.SyncConfig{ID: "sync1", Source: "source", Targets: []string{"target"}, SyncType: "bidirectional"}
	result := engine.executeSyncBidirectionalConcurrent(syncCfg, engine.backends["source"], engine.backends["target"], 2)
	if result.Success != 0 || result.Failure != 0 {
		t.Errorf("expected 0/0 results, got %d/%d", result.Success, result.Failure)
	}
}

func TestExecuteSyncBidirectionalConcurrent_FailureCount(t *testing.T) {
	engine, store, source, _ := setupEngine(t)
	defer store.Close()

	source.getError = fmt.Errorf("get error")
	syncCfg := &config.SyncConfig{ID: "sync1", Source: "source", Targets: []string{"target"}, SyncType: "bidirectional"}
	result := engine.executeSyncBidirectionalConcurrent(syncCfg, engine.backends["source"], engine.backends["target"], 2)
	if result.Success != 0 {
		t.Errorf("expected 0 success, got %d", result.Success)
	}
	if result.Failure != 2 {
		t.Errorf("expected 2 failures, got %d", result.Failure)
	}
}

func TestExecuteSyncUnidirectionalConcurrent_FailureCount(t *testing.T) {
	engine, store, _, _ := setupEngine(t)
	defer store.Close()

	syncCfg := &config.SyncConfig{ID: "sync1", Source: "source", Targets: []string{"missing"}, SyncType: "unidirectional"}
	result := engine.executeSyncUnidirectionalConcurrent(syncCfg, engine.backends["source"], 2)

	if result.Success != 0 {
		t.Errorf("expected 0 success, got %d", result.Success)
	}
	if result.Failure != 2 {
		t.Errorf("expected 2 failures, got %d", result.Failure)
	}
}

func TestExecuteSyncUnidirectionalConcurrent_ListError(t *testing.T) {
	engine, store, source, _ := setupEngine(t)
	defer store.Close()

	source.listError = fmt.Errorf("list error")
	syncCfg := &config.SyncConfig{ID: "sync1", Source: "source", Targets: []string{"target"}, SyncType: "unidirectional"}

	result := engine.executeSyncUnidirectionalConcurrent(syncCfg, engine.backends["source"], 2)
	if result.Success != 0 || result.Failure != 0 {
		t.Errorf("expected 0/0 results, got %d/%d", result.Success, result.Failure)
	}
}

func TestFilterSecrets(t *testing.T) {
	secrets := []string{"secret1", "secret2", "other"}

	filtered := filterSecrets(secrets, config.FilterConfig{Patterns: []string{"secret*"}})
	if len(filtered) != 2 {
		t.Errorf("expected 2 secrets, got %d", len(filtered))
	}

	filtered = filterSecrets(secrets, config.FilterConfig{Exclude: []string{"secret2"}})
	if len(filtered) != 2 {
		t.Errorf("expected 2 secrets, got %d", len(filtered))
	}
}

func TestMatchPattern(t *testing.T) {
	tests := []struct {
		pattern  string
		name     string
		expected bool
	}{
		{"*", "anything", true},
		{"secret1", "secret1", true},
		{"secret1", "secret2", false},
		{"secret*", "secret1", true},
		{"secret*", "secret2", true},
		{"*secret", "mysecret", true},
		{"*secret", "notasecre", false},
		{"prefix*", "prefix123", true},
		{"prefix*", "other", false},
	}

	for _, tt := range tests {
		result := matchPattern(tt.pattern, tt.name)
		if result != tt.expected {
			t.Errorf("matchPattern(%q, %q) = %v, expected %v", tt.pattern, tt.name, result, tt.expected)
		}
	}
}

func TestHashString(t *testing.T) {
	if hashString("test") != "098f6bcd4621d373cade4e832627b4f6" {
		t.Error("unexpected hash for test")
	}
}

func TestWithRetry_SuccessAfterRetry(t *testing.T) {
	policy := config.RetryPolicy{
		MaxRetries:     2,
		InitialBackoff: 1,
		MaxBackoff:     10,
		Multiplier:     2.0,
	}

	callCount := 0
	err := withRetry(policy, func() error {
		callCount++
		if callCount < 2 {
			return fmt.Errorf("temporary")
		}
		return nil
	})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestWithRetry_MaxRetriesExceeded(t *testing.T) {
	policy := config.RetryPolicy{
		MaxRetries:     1,
		InitialBackoff: 1,
		MaxBackoff:     2,
		Multiplier:     2.0,
	}

	start := time.Now()
	err := withRetry(policy, func() error {
		return fmt.Errorf("fail")
	})

	if err == nil {
		t.Fatal("expected error for max retries")
	}
	if time.Since(start) < time.Millisecond {
		t.Error("expected retry backoff")
	}
}
