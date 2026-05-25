package storage

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/pacorreia/vaults-syncer/config"
)

// Store handles all database operations for vaults-syncer.
// It owns the sql.DB connection and exposes specialised sub-stores
// that each enforce a single responsibility.
type Store struct {
	db     *sql.DB
	dbType DBType

	// Sub-stores are embedded so their methods are promoted on Store for
	// backward-compatibility with existing callers.
	*SyncObjectStore
	*SettingsStore
	*UserStore
	*ConfigStore
}

// NewStore creates a Store backed by SQLite at the given file path.
// This function is retained for backward-compatibility with existing code
// that was written before multi-backend support was added.
func NewStore(dbPath string) (*Store, error) {
	return NewStoreFromConfig(DBConfig{Type: DBTypeSQLite, Path: dbPath})
}

// NewStoreFromEnv creates a Store using configuration read from environment variables
// (DB_TYPE, DB_DSN, DB_PATH).
func NewStoreFromEnv() (*Store, error) {
	return NewStoreFromConfig(DBConfigFromEnv())
}

// NewStoreFromConfig creates a Store for the given DBConfig.
func NewStoreFromConfig(cfg DBConfig) (*Store, error) {
	db, err := openDB(cfg)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("storage: cannot reach database: %w", err)
	}

	s := &Store{
		db:     db,
		dbType: cfg.Type,
	}

	s.SyncObjectStore = &SyncObjectStore{db: db, dbType: cfg.Type}
	s.SettingsStore = &SettingsStore{db: db, dbType: cfg.Type}
	s.UserStore = &UserStore{db: db, dbType: cfg.Type}
	s.ConfigStore = &ConfigStore{db: db, dbType: cfg.Type}

	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("storage: migration failed: %w", err)
	}

	return s, nil
}

// Close closes the underlying database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// DB returns the raw *sql.DB for use in tests or advanced scenarios.
func (s *Store) DB() *sql.DB { return s.db }

// migrate runs all schema migrations in order.
func (s *Store) migrate() error {
	if err := migrateCore(s.db, s.dbType); err != nil {
		return err
	}
	if err := migrateAppSettings(s.db, s.dbType); err != nil {
		return err
	}
	if err := migrateUsers(s.db, s.dbType); err != nil {
		return err
	}
	if err := migrateConfigTables(s.db, s.dbType); err != nil {
		return err
	}
	return nil
}

// ---------------------------------------------------------------------------
// Migrations
// ---------------------------------------------------------------------------

func migrateCore(db *sql.DB, dbType DBType) error {
	var schema string
	switch dbType {
	case DBTypePostgres:
		schema = `
CREATE TABLE IF NOT EXISTS sync_objects (
id BIGSERIAL PRIMARY KEY,
sync_id TEXT NOT NULL,
source_vault_id TEXT NOT NULL,
target_vault_id TEXT NOT NULL,
secret_name TEXT NOT NULL,
external_id TEXT,
source_checksum TEXT,
target_checksum TEXT,
last_sync_time BIGINT,
last_sync_status TEXT,
last_sync_error TEXT,
sync_count INTEGER DEFAULT 0,
failure_count INTEGER DEFAULT 0,
direction_last TEXT,
created_at BIGINT DEFAULT EXTRACT(EPOCH FROM NOW())::BIGINT,
updated_at BIGINT DEFAULT EXTRACT(EPOCH FROM NOW())::BIGINT,
UNIQUE(sync_id, source_vault_id, target_vault_id, secret_name)
);

CREATE TABLE IF NOT EXISTS sync_history (
id BIGSERIAL PRIMARY KEY,
sync_object_id BIGINT NOT NULL REFERENCES sync_objects(id),
sync_type TEXT NOT NULL,
status TEXT NOT NULL,
error_message TEXT,
duration_ms BIGINT,
created_at BIGINT DEFAULT EXTRACT(EPOCH FROM NOW())::BIGINT
);

CREATE TABLE IF NOT EXISTS syncs_run (
id BIGSERIAL PRIMARY KEY,
sync_id TEXT NOT NULL,
status TEXT NOT NULL,
total_synced INTEGER DEFAULT 0,
total_failed INTEGER DEFAULT 0,
duration_ms BIGINT,
error_message TEXT,
created_at BIGINT DEFAULT EXTRACT(EPOCH FROM NOW())::BIGINT
);

CREATE INDEX IF NOT EXISTS idx_sync_objects_sync_id ON sync_objects(sync_id);
CREATE INDEX IF NOT EXISTS idx_sync_objects_vaults ON sync_objects(source_vault_id, target_vault_id);
CREATE INDEX IF NOT EXISTS idx_sync_history_sync_object_id ON sync_history(sync_object_id);
CREATE INDEX IF NOT EXISTS idx_syncs_run_sync_id ON syncs_run(sync_id);
`
	case DBTypeMSSQL:
		stmts := []string{
			`IF NOT EXISTS (SELECT * FROM sys.tables WHERE name='sync_objects')
CREATE TABLE sync_objects (
id BIGINT IDENTITY(1,1) PRIMARY KEY,
sync_id NVARCHAR(255) NOT NULL,
source_vault_id NVARCHAR(255) NOT NULL,
target_vault_id NVARCHAR(255) NOT NULL,
secret_name NVARCHAR(1000) NOT NULL,
external_id NVARCHAR(1000),
source_checksum NVARCHAR(255),
target_checksum NVARCHAR(255),
last_sync_time BIGINT,
last_sync_status NVARCHAR(50),
last_sync_error NVARCHAR(MAX),
sync_count INT DEFAULT 0,
failure_count INT DEFAULT 0,
direction_last NVARCHAR(50),
created_at BIGINT DEFAULT DATEDIFF(SECOND,'1970-01-01',GETUTCDATE()),
updated_at BIGINT DEFAULT DATEDIFF(SECOND,'1970-01-01',GETUTCDATE()),
CONSTRAINT uq_sync_objects UNIQUE (sync_id, source_vault_id, target_vault_id, secret_name)
)`,
			`IF NOT EXISTS (SELECT * FROM sys.tables WHERE name='sync_history')
CREATE TABLE sync_history (
id BIGINT IDENTITY(1,1) PRIMARY KEY,
sync_object_id BIGINT NOT NULL,
sync_type NVARCHAR(50) NOT NULL,
status NVARCHAR(50) NOT NULL,
error_message NVARCHAR(MAX),
duration_ms BIGINT,
created_at BIGINT DEFAULT DATEDIFF(SECOND,'1970-01-01',GETUTCDATE())
)`,
			`IF NOT EXISTS (SELECT * FROM sys.tables WHERE name='syncs_run')
CREATE TABLE syncs_run (
id BIGINT IDENTITY(1,1) PRIMARY KEY,
sync_id NVARCHAR(255) NOT NULL,
status NVARCHAR(50) NOT NULL,
total_synced INT DEFAULT 0,
total_failed INT DEFAULT 0,
duration_ms BIGINT,
error_message NVARCHAR(MAX),
created_at BIGINT DEFAULT DATEDIFF(SECOND,'1970-01-01',GETUTCDATE())
)`,
		}
		for _, stmt := range stmts {
			if _, err := db.Exec(stmt); err != nil {
				return fmt.Errorf("MSSQL migration failed: %w", err)
			}
		}
		return nil
	default: // SQLite
		schema = `
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
	}
	_, err := db.Exec(schema)
	return err
}

func migrateAppSettings(db *sql.DB, dbType DBType) error {
	var schema string
	switch dbType {
	case DBTypePostgres:
		schema = `
CREATE TABLE IF NOT EXISTS app_settings (
key TEXT PRIMARY KEY,
value TEXT NOT NULL,
created_at BIGINT DEFAULT EXTRACT(EPOCH FROM NOW())::BIGINT,
updated_at BIGINT DEFAULT EXTRACT(EPOCH FROM NOW())::BIGINT
);`
	case DBTypeMSSQL:
		schema = `
IF NOT EXISTS (SELECT * FROM sys.tables WHERE name='app_settings')
CREATE TABLE app_settings (
key NVARCHAR(255) PRIMARY KEY,
value NVARCHAR(MAX) NOT NULL,
created_at BIGINT DEFAULT DATEDIFF(SECOND,'1970-01-01',GETUTCDATE()),
updated_at BIGINT DEFAULT DATEDIFF(SECOND,'1970-01-01',GETUTCDATE())
)`
	default:
		schema = `
CREATE TABLE IF NOT EXISTS app_settings (
key TEXT PRIMARY KEY,
value TEXT NOT NULL,
created_at INTEGER DEFAULT (cast(strftime('%s', 'now') as int)),
updated_at INTEGER DEFAULT (cast(strftime('%s', 'now') as int))
);`
	}
	_, err := db.Exec(schema)
	return err
}

func migrateUsers(db *sql.DB, dbType DBType) error {
	var usersSchema, sessionsSchema string
	switch dbType {
	case DBTypePostgres:
		usersSchema = `
CREATE TABLE IF NOT EXISTS users (
id BIGSERIAL PRIMARY KEY,
username TEXT NOT NULL UNIQUE,
password_hash TEXT NOT NULL,
role TEXT NOT NULL DEFAULT 'user',
created_at BIGINT DEFAULT EXTRACT(EPOCH FROM NOW())::BIGINT,
updated_at BIGINT DEFAULT EXTRACT(EPOCH FROM NOW())::BIGINT
);`
		sessionsSchema = `
CREATE TABLE IF NOT EXISTS sessions (
id BIGSERIAL PRIMARY KEY,
user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
token TEXT NOT NULL UNIQUE,
expires_at BIGINT NOT NULL,
created_at BIGINT DEFAULT EXTRACT(EPOCH FROM NOW())::BIGINT
);
CREATE INDEX IF NOT EXISTS idx_sessions_token ON sessions(token);
CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id);`
	case DBTypeMSSQL:
		stmts := []string{
			`IF NOT EXISTS (SELECT * FROM sys.tables WHERE name='users')
CREATE TABLE users (
id BIGINT IDENTITY(1,1) PRIMARY KEY,
username NVARCHAR(255) NOT NULL UNIQUE,
password_hash NVARCHAR(MAX) NOT NULL,
role NVARCHAR(50) NOT NULL DEFAULT 'user',
created_at BIGINT DEFAULT DATEDIFF(SECOND,'1970-01-01',GETUTCDATE()),
updated_at BIGINT DEFAULT DATEDIFF(SECOND,'1970-01-01',GETUTCDATE())
)`,
			`IF NOT EXISTS (SELECT * FROM sys.tables WHERE name='sessions')
CREATE TABLE sessions (
id BIGINT IDENTITY(1,1) PRIMARY KEY,
user_id BIGINT NOT NULL,
token NVARCHAR(255) NOT NULL UNIQUE,
expires_at BIGINT NOT NULL,
created_at BIGINT DEFAULT DATEDIFF(SECOND,'1970-01-01',GETUTCDATE())
)`,
		}
		for _, stmt := range stmts {
			if _, err := db.Exec(stmt); err != nil {
				return fmt.Errorf("MSSQL users migration failed: %w", err)
			}
		}
		return nil
	default:
		usersSchema = `
CREATE TABLE IF NOT EXISTS users (
id INTEGER PRIMARY KEY AUTOINCREMENT,
username TEXT NOT NULL UNIQUE,
password_hash TEXT NOT NULL,
role TEXT NOT NULL DEFAULT 'user',
created_at INTEGER DEFAULT (cast(strftime('%s', 'now') as int)),
updated_at INTEGER DEFAULT (cast(strftime('%s', 'now') as int))
);`
		sessionsSchema = `
CREATE TABLE IF NOT EXISTS sessions (
id INTEGER PRIMARY KEY AUTOINCREMENT,
user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
token TEXT NOT NULL UNIQUE,
expires_at INTEGER NOT NULL,
created_at INTEGER DEFAULT (cast(strftime('%s', 'now') as int))
);
CREATE INDEX IF NOT EXISTS idx_sessions_token ON sessions(token);
CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id);`
	}
	if _, err := db.Exec(usersSchema); err != nil {
		return err
	}
	_, err := db.Exec(sessionsSchema)
	return err
}

func migrateConfigTables(db *sql.DB, dbType DBType) error {
	var vaultsSchema, syncsSchema string
	switch dbType {
	case DBTypePostgres:
		vaultsSchema = `
CREATE TABLE IF NOT EXISTS config_vaults (
id TEXT PRIMARY KEY,
config_json TEXT NOT NULL,
created_at BIGINT DEFAULT EXTRACT(EPOCH FROM NOW())::BIGINT,
updated_at BIGINT DEFAULT EXTRACT(EPOCH FROM NOW())::BIGINT
);`
		syncsSchema = `
CREATE TABLE IF NOT EXISTS config_syncs (
id TEXT PRIMARY KEY,
config_json TEXT NOT NULL,
created_at BIGINT DEFAULT EXTRACT(EPOCH FROM NOW())::BIGINT,
updated_at BIGINT DEFAULT EXTRACT(EPOCH FROM NOW())::BIGINT
);`
	case DBTypeMSSQL:
		stmts := []string{
			`IF NOT EXISTS (SELECT * FROM sys.tables WHERE name='config_vaults')
CREATE TABLE config_vaults (
id NVARCHAR(255) PRIMARY KEY,
config_json NVARCHAR(MAX) NOT NULL,
created_at BIGINT DEFAULT DATEDIFF(SECOND,'1970-01-01',GETUTCDATE()),
updated_at BIGINT DEFAULT DATEDIFF(SECOND,'1970-01-01',GETUTCDATE())
)`,
			`IF NOT EXISTS (SELECT * FROM sys.tables WHERE name='config_syncs')
CREATE TABLE config_syncs (
id NVARCHAR(255) PRIMARY KEY,
config_json NVARCHAR(MAX) NOT NULL,
created_at BIGINT DEFAULT DATEDIFF(SECOND,'1970-01-01',GETUTCDATE()),
updated_at BIGINT DEFAULT DATEDIFF(SECOND,'1970-01-01',GETUTCDATE())
)`,
		}
		for _, stmt := range stmts {
			if _, err := db.Exec(stmt); err != nil {
				return fmt.Errorf("MSSQL config migration failed: %w", err)
			}
		}
		return nil
	default:
		vaultsSchema = `
CREATE TABLE IF NOT EXISTS config_vaults (
id TEXT PRIMARY KEY,
config_json TEXT NOT NULL,
created_at INTEGER DEFAULT (cast(strftime('%s', 'now') as int)),
updated_at INTEGER DEFAULT (cast(strftime('%s', 'now') as int))
);`
		syncsSchema = `
CREATE TABLE IF NOT EXISTS config_syncs (
id TEXT PRIMARY KEY,
config_json TEXT NOT NULL,
created_at INTEGER DEFAULT (cast(strftime('%s', 'now') as int)),
updated_at INTEGER DEFAULT (cast(strftime('%s', 'now') as int))
);`
	}
	if _, err := db.Exec(vaultsSchema); err != nil {
		return err
	}
	_, err := db.Exec(syncsSchema)
	return err
}

// ---------------------------------------------------------------------------
// SyncObjectStore - existing sync object persistence (unchanged API)
// ---------------------------------------------------------------------------

// SyncObjectStore handles sync object and sync-run persistence.
type SyncObjectStore struct {
	db     *sql.DB
	dbType DBType
}

// UpsertSyncObject inserts or updates a sync object.
func (s *SyncObjectStore) UpsertSyncObject(obj *config.SyncObject) error {
	now := time.Now().Unix()

	// Portable upsert: attempt UPDATE then INSERT.
	res, err := s.db.Exec(
		`UPDATE sync_objects SET
external_id=?, source_checksum=?, target_checksum=?, last_sync_time=?,
last_sync_status=?, last_sync_error=?, sync_count=?, failure_count=?,
direction_last=?, updated_at=?
WHERE sync_id=? AND source_vault_id=? AND target_vault_id=? AND secret_name=?`,
		obj.ExternalID, obj.SourceChecksum, obj.TargetChecksum, obj.LastSyncTime,
		obj.LastSyncStatus, obj.LastSyncError, obj.SyncCount, obj.FailureCount,
		obj.DirectionLast, now,
		obj.SyncID, obj.SourceVaultID, obj.TargetVaultID, obj.SecretName,
	)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		_, err = s.db.Exec(
			`INSERT INTO sync_objects (sync_id, source_vault_id, target_vault_id, secret_name,
external_id, source_checksum, target_checksum, last_sync_time, last_sync_status,
last_sync_error, sync_count, failure_count, direction_last, updated_at)
VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			obj.SyncID, obj.SourceVaultID, obj.TargetVaultID, obj.SecretName,
			obj.ExternalID, obj.SourceChecksum, obj.TargetChecksum, obj.LastSyncTime,
			obj.LastSyncStatus, obj.LastSyncError, obj.SyncCount, obj.FailureCount,
			obj.DirectionLast, now,
		)
	}
	return err
}

// GetSyncObject retrieves a sync object.
func (s *SyncObjectStore) GetSyncObject(syncID, sourceVaultID, targetVaultID, secretName string) (*config.SyncObject, error) {
	obj := &config.SyncObject{}
	err := s.db.QueryRow(
		`SELECT id, sync_id, source_vault_id, target_vault_id, secret_name,
        external_id, source_checksum, target_checksum, last_sync_time,
        last_sync_status, last_sync_error, sync_count, failure_count, direction_last
FROM sync_objects
WHERE sync_id=? AND source_vault_id=? AND target_vault_id=? AND secret_name=?`,
		syncID, sourceVaultID, targetVaultID, secretName,
	).Scan(
		&obj.ID, &obj.SyncID, &obj.SourceVaultID, &obj.TargetVaultID, &obj.SecretName,
		&obj.ExternalID, &obj.SourceChecksum, &obj.TargetChecksum, &obj.LastSyncTime,
		&obj.LastSyncStatus, &obj.LastSyncError, &obj.SyncCount, &obj.FailureCount, &obj.DirectionLast,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return obj, err
}

// GetSyncObjectsBySync retrieves all sync objects for a sync.
func (s *SyncObjectStore) GetSyncObjectsBySync(syncID string) ([]*config.SyncObject, error) {
	rows, err := s.db.Query(
		`SELECT id, sync_id, source_vault_id, target_vault_id, secret_name,
        external_id, source_checksum, target_checksum, last_sync_time,
        last_sync_status, last_sync_error, sync_count, failure_count, direction_last
FROM sync_objects WHERE sync_id=? ORDER BY updated_at DESC`, syncID,
	)
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

// RecordSyncHistory records a sync event.
func (s *SyncObjectStore) RecordSyncHistory(syncObjectID int64, syncType, status, errorMsg string, durationMs int64) error {
	_, err := s.db.Exec(
		`INSERT INTO sync_history (sync_object_id, sync_type, status, error_message, duration_ms)
VALUES (?,?,?,?,?)`,
		syncObjectID, syncType, status, errorMsg, durationMs,
	)
	return err
}

// RecordSyncRun records a complete sync run.
func (s *SyncObjectStore) RecordSyncRun(syncID, status string, totalSynced, totalFailed int, durationMs int64, errorMsg string) error {
	_, err := s.db.Exec(
		`INSERT INTO syncs_run (sync_id, status, total_synced, total_failed, duration_ms, error_message)
VALUES (?,?,?,?,?,?)`,
		syncID, status, totalSynced, totalFailed, durationMs, errorMsg,
	)
	return err
}

// GetSyncRuns retrieves sync runs for a sync.
func (s *SyncObjectStore) GetSyncRuns(syncID string, limit int) ([]*SyncRun, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.Query(
		`SELECT id, sync_id, status, total_synced, total_failed, duration_ms, error_message, created_at
FROM syncs_run WHERE sync_id=? ORDER BY created_at DESC LIMIT ?`,
		syncID, limit,
	)
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

// SyncRun represents a sync run record.
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
