package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pacorreia/vaults-syncer/config"
)

func TestNewStore(t *testing.T) {
	tests := []struct {
		name    string
		dbPath  string
		wantErr bool
	}{
		{
			name:    "valid in-memory database",
			dbPath:  ":memory:",
			wantErr: false,
		},
		{
			name:    "valid file database",
			dbPath:  "", // will be set to temp file
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dbPath := tt.dbPath
			if dbPath == "" {
				// Create temp file
				tmpDir := t.TempDir()
				dbPath = filepath.Join(tmpDir, "test.db")
			}

			store, err := NewStore(dbPath)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			defer store.Close()

			// Verify tables were created
			rows, err := store.db.Query("SELECT name FROM sqlite_master WHERE type='table'")
			if err != nil {
				t.Fatalf("failed to query tables: %v", err)
			}
			defer rows.Close()

			tables := make(map[string]bool)
			for rows.Next() {
				var name string
				if err := rows.Scan(&name); err != nil {
					t.Fatalf("failed to scan table name: %v", err)
				}
				tables[name] = true
			}

			expectedTables := []string{"sync_objects", "sync_history", "syncs_run"}
			for _, table := range expectedTables {
				if !tables[table] {
					t.Errorf("expected table %s to exist", table)
				}
			}
		})
	}
}

func TestUpsertSyncObject(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	obj := &config.SyncObject{
		SyncID:         "test-sync",
		SourceVaultID:  "vault1",
		TargetVaultID:  "vault2",
		SecretName:     "my-secret",
		ExternalID:     "ext-123",
		SourceChecksum: "abc123",
		TargetChecksum: "abc123",
		LastSyncTime:   time.Now().Unix(),
		LastSyncStatus: "success",
		SyncCount:      1,
		FailureCount:   0,
	}

	// Insert
	err = store.UpsertSyncObject(obj)
	if err != nil {
		t.Fatalf("failed to insert sync object: %v", err)
	}

	// Verify insert
	retrieved, err := store.GetSyncObject("test-sync", "vault1", "vault2", "my-secret")
	if err != nil {
		t.Fatalf("failed to get sync object: %v", err)
	}

	if retrieved.SecretName != "my-secret" {
		t.Errorf("expected secret name 'my-secret', got '%s'", retrieved.SecretName)
	}
	if retrieved.SourceChecksum != "abc123" {
		t.Errorf("expected checksum 'abc123', got '%s'", retrieved.SourceChecksum)
	}

	// Update
	obj.SourceChecksum = "def456"
	obj.SyncCount = 2
	err = store.UpsertSyncObject(obj)
	if err != nil {
		t.Fatalf("failed to update sync object: %v", err)
	}

	// Verify update
	retrieved, err = store.GetSyncObject("test-sync", "vault1", "vault2", "my-secret")
	if err != nil {
		t.Fatalf("failed to get updated sync object: %v", err)
	}

	if retrieved.SourceChecksum != "def456" {
		t.Errorf("expected updated checksum 'def456', got '%s'", retrieved.SourceChecksum)
	}
	if retrieved.SyncCount != 2 {
		t.Errorf("expected sync count 2, got %d", retrieved.SyncCount)
	}
}

func TestGetSyncObject(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	// Insert test data
	obj := &config.SyncObject{
		SyncID:         "sync1",
		SourceVaultID:  "source",
		TargetVaultID:  "target",
		SecretName:     "secret1",
		SourceChecksum: "checksum1",
		LastSyncStatus: "success",
		SyncCount:      5,
	}
	if err := store.UpsertSyncObject(obj); err != nil {
		t.Fatalf("failed to insert test data: %v", err)
	}

	tests := []struct {
		name           string
		syncID         string
		sourceVaultID  string
		targetVaultID  string
		secretName     string
		wantNil        bool
		expectedName   string
	}{
		{
			name:           "existing object",
			syncID:         "sync1",
			sourceVaultID:  "source",
			targetVaultID:  "target",
			secretName:     "secret1",
			wantNil:        false,
			expectedName:   "secret1",
		},
		{
			name:           "non-existent object",
			syncID:         "sync1",
			sourceVaultID:  "source",
			targetVaultID:  "target",
			secretName:     "nonexistent",
			wantNil:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := store.GetSyncObject(tt.syncID, tt.sourceVaultID, tt.targetVaultID, tt.secretName)

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.wantNil {
				if got != nil {
					t.Errorf("expected nil object for non-existent item, got %+v", got)
				}
				return
			}

			if got == nil {
				t.Fatalf("expected object, got nil")
			}

			if got.SecretName != tt.expectedName {
				t.Errorf("expected name '%s', got '%s'", tt.expectedName, got.SecretName)
			}
		})
	}
}

func TestRecordSyncRun(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	err = store.RecordSyncRun("test-sync", "success", 10, 2, 1500, "")
	if err != nil {
		t.Fatalf("failed to record sync run: %v", err)
	}

	// Verify it was recorded
	var count int
	err = store.db.QueryRow("SELECT COUNT(*) FROM syncs_run WHERE sync_id = ?", "test-sync").Scan(&count)
	if err != nil {
		t.Fatalf("failed to query sync runs: %v", err)
	}

	if count != 1 {
		t.Errorf("expected 1 sync run, got %d", count)
	}
}

func TestGetSyncRuns(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	// Insert test data
	for i := 0; i < 5; i++ {
		err := store.RecordSyncRun("sync1", "success", 10, 0, 1000, "")
		if err != nil {
			t.Fatalf("failed to insert test data: %v", err)
		}
	}

	// Get runs with limit
	runs, err := store.GetSyncRuns("sync1", 3)
	if err != nil {
		t.Fatalf("failed to get sync runs: %v", err)
	}

	if len(runs) != 3 {
		t.Errorf("expected 3 runs with limit, got %d", len(runs))
	}

	// Check fields
	if runs[0].SyncID != "sync1" {
		t.Errorf("expected sync ID 'sync1', got '%s'", runs[0].SyncID)
	}
	if runs[0].Status != "success" {
		t.Errorf("expected status 'success', got '%s'", runs[0].Status)
	}
}

func TestClose(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// Close the store
	err = store.Close()
	if err != nil {
		t.Errorf("failed to close store: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Errorf("database file should exist after close")
	}
}

func TestGetSyncObjectsBySync(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	// Insert multiple test objects for same sync
	objects := []*config.SyncObject{
		{
			SyncID:         "sync1",
			SourceVaultID:  "source",
			TargetVaultID:  "target",
			SecretName:     "secret1",
			SourceChecksum: "checksum1",
		},
		{
			SyncID:         "sync1",
			SourceVaultID:  "source",
			TargetVaultID:  "target",
			SecretName:     "secret2",
			SourceChecksum: "checksum2",
		},
		{
			SyncID:         "sync2",
			SourceVaultID:  "source",
			TargetVaultID:  "target",
			SecretName:     "secret3",
			SourceChecksum: "checksum3",
		},
	}

	for _, obj := range objects {
		if err := store.UpsertSyncObject(obj); err != nil {
			t.Fatalf("failed to insert test data: %v", err)
		}
	}

	// Test getting objects for sync1
	results, err := store.GetSyncObjectsBySync("sync1")
	if err != nil {
		t.Fatalf("failed to get sync objects: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("expected 2 objects for sync1, got %d", len(results))
	}

	// Verify object content
	secretNames := make(map[string]bool)
	for _, obj := range results {
		secretNames[obj.SecretName] = true
		if obj.SyncID != "sync1" {
			t.Errorf("expected sync ID 'sync1', got '%s'", obj.SyncID)
		}
	}

	if !secretNames["secret1"] || !secretNames["secret2"] {
		t.Error("expected both secret1 and secret2 in results")
	}

	// Test getting objects for sync2
	results, err = store.GetSyncObjectsBySync("sync2")
	if err != nil {
		t.Fatalf("failed to get sync objects for sync2: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("expected 1 object for sync2, got %d", len(results))
	}

	// Test getting objects for non-existent sync
	results, err = store.GetSyncObjectsBySync("nonexistent")
	if err != nil {
		t.Fatalf("failed to get sync objects for nonexistent: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("expected 0 objects for nonexistent sync, got %d", len(results))
	}
}

func TestRecordSyncHistory(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	// First create a sync object to get its ID
	obj := &config.SyncObject{
		SyncID:         "sync1",
		SourceVaultID:  "source",
		TargetVaultID:  "target",
		SecretName:     "secret1",
		SourceChecksum: "checksum1",
	}
	if err := store.UpsertSyncObject(obj); err != nil {
		t.Fatalf("failed to insert sync object: %v", err)
	}

	// Retrieve the object to get its ID
	retrieved, err := store.GetSyncObject("sync1", "source", "target", "secret1")
	if err != nil {
		t.Fatalf("failed to get sync object: %v", err)
	}

	// Record sync history
	tests := []struct {
		name         string
		syncObjectID int64
		syncType     string
		status       string
		errorMsg     string
		durationMs   int64
		wantErr      bool
	}{
		{
			name:         "successful sync",
			syncObjectID: retrieved.ID,
			syncType:     "push",
			status:       "success",
			errorMsg:     "",
			durationMs:   100,
			wantErr:      false,
		},
		{
			name:         "failed sync with error",
			syncObjectID: retrieved.ID,
			syncType:     "pull",
			status:       "failed",
			errorMsg:     "connection timeout",
			durationMs:   5000,
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := store.RecordSyncHistory(tt.syncObjectID, tt.syncType, tt.status, tt.errorMsg, tt.durationMs)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			// Verify the record was inserted by querying directly
			var count int
			query := `SELECT COUNT(*) FROM sync_history WHERE sync_object_id = ? AND sync_type = ? AND status = ?`
			err = store.db.QueryRow(query, tt.syncObjectID, tt.syncType, tt.status).Scan(&count)
			if err != nil {
				t.Fatalf("failed to verify history record: %v", err)
			}

			if count == 0 {
				t.Error("history record was not inserted")
			}
		})
	}
}

func TestNewStoreError(t *testing.T) {
	// Try to create store with invalid path
	_, err := NewStore("/invalid/path/to/db.sqlite")
	if err == nil {
		t.Error("expected error for invalid database path")
	}
}

func TestGetSyncRunsEmpty(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	// Get runs for non-existent sync
	runs, err := store.GetSyncRuns("nonexistent", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(runs) != 0 {
		t.Errorf("expected 0 runs, got %d", len(runs))
	}
}
