package storage

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/pacorreia/vaults-syncer/config"
)

// Store handles database operations
type Store struct {
	db *sql.DB
}

// NewStore creates a new Store
func NewStore(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	store := &Store{db: db}
	if err := store.migrate(); err != nil {
		return nil, fmt.Errorf("migration failed: %w", err)
	}

	return store, nil
}

// Close closes the database connection
func (s *Store) Close() error {
	return s.db.Close()
}

// migrate runs database migrations
func (s *Store) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS sync_objects (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		sync_id TEXT NOT NULL,
		source_vault_id TEXT NOT NULL,
		target_vault_id TEXT NOT NULL,
		secret_name TEXT NOT NULL,
		external_id TEXT,
		source_checksum TEXT,
		target_checksum TEXT,
		last_sync_time INTEGER,
		last_sync_status TEXT,
		last_sync_error TEXT,
		sync_count INTEGER DEFAULT 0,
		failure_count INTEGER DEFAULT 0,
		direction_last TEXT,
		created_at INTEGER DEFAULT (cast(strftime('%s', 'now') as int)),
		updated_at INTEGER DEFAULT (cast(strftime('%s', 'now') as int)),
		UNIQUE(sync_id, source_vault_id, target_vault_id, secret_name)
	);

	CREATE TABLE IF NOT EXISTS sync_history (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		sync_object_id INTEGER NOT NULL,
		sync_type TEXT NOT NULL,
		status TEXT NOT NULL,
		error_message TEXT,
		duration_ms INTEGER,
		created_at INTEGER DEFAULT (cast(strftime('%s', 'now') as int)),
		FOREIGN KEY(sync_object_id) REFERENCES sync_objects(id)
	);

	CREATE TABLE IF NOT EXISTS syncs_run (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		sync_id TEXT NOT NULL,
		status TEXT NOT NULL,
		total_synced INTEGER DEFAULT 0,
		total_failed INTEGER DEFAULT 0,
		duration_ms INTEGER,
		error_message TEXT,
		created_at INTEGER DEFAULT (cast(strftime('%s', 'now') as int))
	);

	CREATE INDEX IF NOT EXISTS idx_sync_objects_sync_id ON sync_objects(sync_id);
	CREATE INDEX IF NOT EXISTS idx_sync_objects_vaults ON sync_objects(source_vault_id, target_vault_id);
	CREATE INDEX IF NOT EXISTS idx_sync_history_sync_object_id ON sync_history(sync_object_id);
	CREATE INDEX IF NOT EXISTS idx_syncs_run_sync_id ON syncs_run(sync_id);
	`

	_, err := s.db.Exec(schema)
	return err
}

// UpsertSyncObject inserts or updates a sync object
func (s *Store) UpsertSyncObject(obj *config.SyncObject) error {
	now := time.Now().Unix()

	query := `
	INSERT INTO sync_objects (
		sync_id, source_vault_id, target_vault_id, secret_name, 
		external_id, source_checksum, target_checksum, last_sync_time,
		last_sync_status, last_sync_error, sync_count, failure_count,
		direction_last, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(sync_id, source_vault_id, target_vault_id, secret_name) DO UPDATE SET
		external_id = excluded.external_id,
		source_checksum = excluded.source_checksum,
		target_checksum = excluded.target_checksum,
		last_sync_time = excluded.last_sync_time,
		last_sync_status = excluded.last_sync_status,
		last_sync_error = excluded.last_sync_error,
		sync_count = excluded.sync_count,
		failure_count = excluded.failure_count,
		direction_last = excluded.direction_last,
		updated_at = ?
	`

	_, err := s.db.Exec(query,
		obj.SyncID, obj.SourceVaultID, obj.TargetVaultID, obj.SecretName,
		obj.ExternalID, obj.SourceChecksum, obj.TargetChecksum, obj.LastSyncTime,
		obj.LastSyncStatus, obj.LastSyncError, obj.SyncCount, obj.FailureCount,
		obj.DirectionLast, now, now,
	)

	return err
}

// GetSyncObject retrieves a sync object
func (s *Store) GetSyncObject(syncID, sourceVaultID, targetVaultID, secretName string) (*config.SyncObject, error) {
	obj := &config.SyncObject{}

	query := `
	SELECT id, sync_id, source_vault_id, target_vault_id, secret_name, 
	       external_id, source_checksum, target_checksum, last_sync_time,
	       last_sync_status, last_sync_error, sync_count, failure_count, direction_last
	FROM sync_objects
	WHERE sync_id = ? AND source_vault_id = ? AND target_vault_id = ? AND secret_name = ?
	`

	err := s.db.QueryRow(query, syncID, sourceVaultID, targetVaultID, secretName).Scan(
		&obj.ID, &obj.SyncID, &obj.SourceVaultID, &obj.TargetVaultID, &obj.SecretName,
		&obj.ExternalID, &obj.SourceChecksum, &obj.TargetChecksum, &obj.LastSyncTime,
		&obj.LastSyncStatus, &obj.LastSyncError, &obj.SyncCount, &obj.FailureCount, &obj.DirectionLast,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}

	return obj, err
}

// GetSyncObjectsBySync retrieves all sync objects for a sync
func (s *Store) GetSyncObjectsBySync(syncID string) ([]*config.SyncObject, error) {
	query := `
	SELECT id, sync_id, source_vault_id, target_vault_id, secret_name, 
	       external_id, source_checksum, target_checksum, last_sync_time,
	       last_sync_status, last_sync_error, sync_count, failure_count, direction_last
	FROM sync_objects
	WHERE sync_id = ?
	ORDER BY updated_at DESC
	`

	rows, err := s.db.Query(query, syncID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var objects []*config.SyncObject
	for rows.Next() {
		obj := &config.SyncObject{}
		if err := rows.Scan(
			&obj.ID, &obj.SyncID, &obj.SourceVaultID, &obj.TargetVaultID, &obj.SecretName,
			&obj.ExternalID, &obj.SourceChecksum, &obj.TargetChecksum, &obj.LastSyncTime,
			&obj.LastSyncStatus, &obj.LastSyncError, &obj.SyncCount, &obj.FailureCount, &obj.DirectionLast,
		); err != nil {
			return nil, err
		}
		objects = append(objects, obj)
	}

	return objects, rows.Err()
}

// RecordSyncHistory records a sync event
func (s *Store) RecordSyncHistory(syncObjectID int64, syncType, status, errorMsg string, durationMs int64) error {
	query := `
	INSERT INTO sync_history (sync_object_id, sync_type, status, error_message, duration_ms)
	VALUES (?, ?, ?, ?, ?)
	`

	_, err := s.db.Exec(query, syncObjectID, syncType, status, errorMsg, durationMs)
	return err
}

// RecordSyncRun records a complete sync run
func (s *Store) RecordSyncRun(syncID, status string, totalSynced, totalFailed int, durationMs int64, errorMsg string) error {
	query := `
	INSERT INTO syncs_run (sync_id, status, total_synced, total_failed, duration_ms, error_message)
	VALUES (?, ?, ?, ?, ?, ?)
	`

	_, err := s.db.Exec(query, syncID, status, totalSynced, totalFailed, durationMs, errorMsg)
	return err
}

// GetSyncRuns retrieves sync runs for a sync
func (s *Store) GetSyncRuns(syncID string, limit int) ([]*SyncRun, error) {
	if limit <= 0 {
		limit = 100
	}

	query := `
	SELECT id, sync_id, status, total_synced, total_failed, duration_ms, error_message, created_at
	FROM syncs_run
	WHERE sync_id = ?
	ORDER BY created_at DESC
	LIMIT ?
	`

	rows, err := s.db.Query(query, syncID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var runs []*SyncRun
	for rows.Next() {
		run := &SyncRun{}
		if err := rows.Scan(
			&run.ID, &run.SyncID, &run.Status, &run.TotalSynced, &run.TotalFailed,
			&run.DurationMs, &run.ErrorMessage, &run.CreatedAt,
		); err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}

	return runs, rows.Err()
}

// SyncRun represents a sync run record
type SyncRun struct {
	ID           int64
	SyncID       string
	Status       string
	TotalSynced  int
	TotalFailed  int
	DurationMs   int64
	ErrorMessage string
	CreatedAt    int64
}
