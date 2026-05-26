package storage

import (
	"database/sql"
	"fmt"
	"time"
)

// SettingsStore persists key/value application settings.
type SettingsStore struct {
	db     *sql.DB
	dbType DBType
}

// SetSetting inserts or updates a setting.
func (s *SettingsStore) SetSetting(key, value string) error {
	now := time.Now().Unix()
	// Portable upsert.
	res, err := s.db.Exec(
		fmt.Sprintf(`UPDATE app_settings SET value=%s, updated_at=%s WHERE key=%s`,
			placeholder(s.dbType, 1), placeholder(s.dbType, 2), placeholder(s.dbType, 3)),
		value, now, key,
	)
	if err != nil {
		return fmt.Errorf("storage: SetSetting update: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("storage: SetSetting rowsAffected: %w", err)
	}
	if n == 0 {
		_, err = s.db.Exec(
			fmt.Sprintf(`INSERT INTO app_settings (key, value, created_at, updated_at) VALUES (%s)`,
				placeholders(s.dbType, 1, 4)),
			key, value, now, now,
		)
		if err != nil {
			return fmt.Errorf("storage: SetSetting insert: %w", err)
		}
	}
	return nil
}

// GetSetting retrieves a setting value. Returns ("", false, nil) if the key does not exist.
func (s *SettingsStore) GetSetting(key string) (string, bool, error) {
	var value string
	err := s.db.QueryRow(
		fmt.Sprintf(`SELECT value FROM app_settings WHERE key=%s`, placeholder(s.dbType, 1)),
		key,
	).Scan(&value)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("storage: GetSetting: %w", err)
	}
	return value, true, nil
}

// IsSetupComplete reports whether the application has been set up (admin account created,
// master key initialised).
func (s *SettingsStore) IsSetupComplete() (bool, error) {
	_, exists, err := s.GetSetting("setup_complete")
	return exists && err == nil, err
}

// MarkSetupComplete records that first-run setup has been completed.
func (s *SettingsStore) MarkSetupComplete() error {
	return s.SetSetting("setup_complete", "true")
}
