package storage

import (
	"database/sql"
	"fmt"
	"os"
	"strings"

	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	_ "github.com/microsoft/go-mssqldb"
)

// DBType identifies the database backend.
type DBType string

const (
	DBTypeSQLite   DBType = "sqlite"
	DBTypePostgres DBType = "postgres"
	DBTypeMSSQL    DBType = "mssql"
)

// DBConfig holds configuration for opening a database connection.
// Values are read from environment variables by NewDBFromEnv.
type DBConfig struct {
	// Type is the database backend: sqlite, postgres, or mssql.
	Type DBType
	// DSN is the connection string used for postgres and mssql.
	DSN string
	// Path is the file path used for sqlite (default: sync.db).
	Path string
}

// DBConfigFromEnv reads DB_TYPE, DB_DSN, and DB_PATH from the environment.
// DB_TYPE defaults to "sqlite"; DB_PATH defaults to "sync.db".
func DBConfigFromEnv() DBConfig {
	dbType := strings.ToLower(strings.TrimSpace(os.Getenv("DB_TYPE")))
	if dbType == "" {
		dbType = string(DBTypeSQLite)
	}
	return DBConfig{
		Type: DBType(dbType),
		DSN:  os.Getenv("DB_DSN"),
		Path: func() string {
			if p := os.Getenv("DB_PATH"); p != "" {
				return p
			}
			return "sync.db"
		}(),
	}
}

// openDB opens a sql.DB connection for the given DBConfig.
func openDB(cfg DBConfig) (*sql.DB, error) {
	switch cfg.Type {
	case DBTypeSQLite:
		dsn := cfg.Path
		if dsn == "" {
			dsn = "sync.db"
		}
		db, err := sql.Open("sqlite3", dsn+"?_journal_mode=WAL&_foreign_keys=on")
		if err != nil {
			return nil, fmt.Errorf("storage: failed to open SQLite database: %w", err)
		}
		return db, nil
	case DBTypePostgres:
		if cfg.DSN == "" {
			return nil, fmt.Errorf("storage: DB_DSN is required for postgres backend")
		}
		db, err := sql.Open("postgres", cfg.DSN)
		if err != nil {
			return nil, fmt.Errorf("storage: failed to open PostgreSQL database: %w", err)
		}
		return db, nil
	case DBTypeMSSQL:
		if cfg.DSN == "" {
			return nil, fmt.Errorf("storage: DB_DSN is required for mssql backend")
		}
		db, err := sql.Open("sqlserver", cfg.DSN)
		if err != nil {
			return nil, fmt.Errorf("storage: failed to open MSSQL database: %w", err)
		}
		return db, nil
	default:
		return nil, fmt.Errorf("storage: unsupported DB_TYPE %q; must be sqlite, postgres, or mssql", cfg.Type)
	}
}

// placeholder returns the parameter placeholder for the given DB type and position.
// SQLite/MSSQL use positional '?'/'@pN'; PostgreSQL uses '$N'.
func placeholder(dbType DBType, n int) string {
	if dbType == DBTypePostgres {
		return fmt.Sprintf("$%d", n)
	}
	if dbType == DBTypeMSSQL {
		return fmt.Sprintf("@p%d", n)
	}
	return "?"
}

// placeholders returns a comma-separated list of n placeholders.
func placeholders(dbType DBType, start, count int) string {
	parts := make([]string, count)
	for i := range parts {
		parts[i] = placeholder(dbType, start+i)
	}
	return strings.Join(parts, ", ")
}
