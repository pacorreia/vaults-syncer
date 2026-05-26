package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/pacorreia/vaults-syncer/config"
	"github.com/pacorreia/vaults-syncer/security"
)

// ConfigStore persists vault and sync configurations in the database.
// Sensitive vault fields (auth headers, credentials) are encrypted with
// the provided Encryptor before storage.
type ConfigStore struct {
	db        *sql.DB
	dbType    DBType
	encryptor security.Encryptor
}

// SetEncryptor attaches an encryptor. Must be called before any vault operations
// if encryption of sensitive fields is desired.
func (s *ConfigStore) SetEncryptor(enc security.Encryptor) {
	s.encryptor = enc
}

// ---------------------------------------------------------------------------
// Vault CRUD
// ---------------------------------------------------------------------------

// SaveVault inserts or replaces a vault configuration. Sensitive auth fields
// are encrypted when an encryptor is set.
func (s *ConfigStore) SaveVault(v config.VaultConfig) error {
	raw := v
	if s.encryptor != nil {
		if err := s.encryptVaultAuth(&raw); err != nil {
			return err
		}
	}

	data, err := json.Marshal(raw)
	if err != nil {
		return fmt.Errorf("storage: SaveVault marshal: %w", err)
	}

	return s.upsertConfigRow("config_vaults", v.ID, string(data))
}

// GetVault retrieves a single vault config by ID and decrypts sensitive fields.
func (s *ConfigStore) GetVault(id string) (*config.VaultConfig, error) {
	raw, err := s.getConfigJSON("config_vaults", id)
	if err != nil {
		return nil, err
	}
	if raw == "" {
		return nil, nil
	}
	var v config.VaultConfig
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return nil, fmt.Errorf("storage: GetVault unmarshal: %w", err)
	}
	if s.encryptor != nil {
		if err := s.decryptVaultAuth(&v); err != nil {
			return nil, err
		}
	}
	return &v, nil
}

// ListVaults returns all stored vault configurations with sensitive fields decrypted.
func (s *ConfigStore) ListVaults() ([]config.VaultConfig, error) {
	rows, err := s.db.Query(`SELECT config_json FROM config_vaults ORDER BY created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("storage: ListVaults query: %w", err)
	}
	defer rows.Close()

	var vaults []config.VaultConfig
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var v config.VaultConfig
		if err := json.Unmarshal([]byte(raw), &v); err != nil {
			return nil, fmt.Errorf("storage: ListVaults unmarshal: %w", err)
		}
		if s.encryptor != nil {
			if err := s.decryptVaultAuth(&v); err != nil {
				return nil, err
			}
		}
		vaults = append(vaults, v)
	}
	return vaults, rows.Err()
}

// DeleteVault removes a vault configuration by ID.
func (s *ConfigStore) DeleteVault(id string) error {
	_, err := s.db.Exec(
		fmt.Sprintf(`DELETE FROM config_vaults WHERE id=%s`, placeholder(s.dbType, 1)),
		id,
	)
	if err != nil {
		return fmt.Errorf("storage: DeleteVault: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Sync CRUD
// ---------------------------------------------------------------------------

// SaveSync inserts or replaces a sync configuration.
func (s *ConfigStore) SaveSync(sc config.SyncConfig) error {
	data, err := json.Marshal(sc)
	if err != nil {
		return fmt.Errorf("storage: SaveSync marshal: %w", err)
	}
	return s.upsertConfigRow("config_syncs", sc.ID, string(data))
}

// GetSync retrieves a single sync config by ID.
func (s *ConfigStore) GetSync(id string) (*config.SyncConfig, error) {
	raw, err := s.getConfigJSON("config_syncs", id)
	if err != nil {
		return nil, err
	}
	if raw == "" {
		return nil, nil
	}
	var sc config.SyncConfig
	if err := json.Unmarshal([]byte(raw), &sc); err != nil {
		return nil, fmt.Errorf("storage: GetSync unmarshal: %w", err)
	}
	return &sc, nil
}

// ListSyncs returns all stored sync configurations.
func (s *ConfigStore) ListSyncs() ([]config.SyncConfig, error) {
	rows, err := s.db.Query(`SELECT config_json FROM config_syncs ORDER BY created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("storage: ListSyncs query: %w", err)
	}
	defer rows.Close()

	var syncs []config.SyncConfig
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var sc config.SyncConfig
		if err := json.Unmarshal([]byte(raw), &sc); err != nil {
			return nil, fmt.Errorf("storage: ListSyncs unmarshal: %w", err)
		}
		syncs = append(syncs, sc)
	}
	return syncs, rows.Err()
}

// DeleteSync removes a sync configuration by ID.
func (s *ConfigStore) DeleteSync(id string) error {
	_, err := s.db.Exec(
		fmt.Sprintf(`DELETE FROM config_syncs WHERE id=%s`, placeholder(s.dbType, 1)),
		id,
	)
	if err != nil {
		return fmt.Errorf("storage: DeleteSync: %w", err)
	}
	return nil
}

// LoadConfig assembles a config.Config from the database — used by the sync engine.
func (s *ConfigStore) LoadConfig(serverCfg config.ServerConfig, loggingCfg config.LoggingConfig) (*config.Config, error) {
	vaults, err := s.ListVaults()
	if err != nil {
		return nil, fmt.Errorf("storage: LoadConfig vaults: %w", err)
	}
	syncs, err := s.ListSyncs()
	if err != nil {
		return nil, fmt.Errorf("storage: LoadConfig syncs: %w", err)
	}
	return &config.Config{
		Vaults:  vaults,
		Syncs:   syncs,
		Server:  serverCfg,
		Logging: loggingCfg,
	}, nil
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

func (s *ConfigStore) upsertConfigRow(table, id, jsonValue string) error {
	now := time.Now().Unix()
	res, err := s.db.Exec(
		fmt.Sprintf(`UPDATE %s SET config_json=%s, updated_at=%s WHERE id=%s`,
			table, placeholder(s.dbType, 1), placeholder(s.dbType, 2), placeholder(s.dbType, 3)),
		jsonValue, now, id,
	)
	if err != nil {
		return fmt.Errorf("storage: upsert %s update: %w", table, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("storage: upsert %s rowsAffected: %w", table, err)
	}
	if n == 0 {
		_, err = s.db.Exec(
			fmt.Sprintf(`INSERT INTO %s (id, config_json, created_at, updated_at) VALUES (%s)`,
				table, placeholders(s.dbType, 1, 4)),
			id, jsonValue, now, now,
		)
		if err != nil {
			return fmt.Errorf("storage: upsert %s insert: %w", table, err)
		}
	}
	return nil
}

func (s *ConfigStore) getConfigJSON(table, id string) (string, error) {
	var raw string
	err := s.db.QueryRow(
		fmt.Sprintf(`SELECT config_json FROM %s WHERE id=%s`, table, placeholder(s.dbType, 1)), id,
	).Scan(&raw)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("storage: getConfigJSON %s: %w", table, err)
	}
	return raw, nil
}

// encryptVaultAuth encrypts sensitive auth fields in-place.
func (s *ConfigStore) encryptVaultAuth(v *config.VaultConfig) error {
	if v.Auth == nil {
		return nil
	}
	for k, val := range v.Auth.Headers {
		enc, err := s.encryptor.EncryptString(val)
		if err != nil {
			return fmt.Errorf("storage: encrypt auth header %q: %w", k, err)
		}
		v.Auth.Headers[k] = enc
	}
	if v.Auth.OAuth != nil {
		if v.Auth.OAuth.ClientSecret != "" {
			enc, err := s.encryptor.EncryptString(v.Auth.OAuth.ClientSecret)
			if err != nil {
				return fmt.Errorf("storage: encrypt OAuth client_secret: %w", err)
			}
			v.Auth.OAuth.ClientSecret = enc
		}
	}
	return nil
}

// decryptVaultAuth decrypts sensitive auth fields in-place.
func (s *ConfigStore) decryptVaultAuth(v *config.VaultConfig) error {
	if v.Auth == nil {
		return nil
	}
	for k, val := range v.Auth.Headers {
		dec, err := s.encryptor.DecryptString(val)
		if err != nil {
			return fmt.Errorf("storage: decrypt auth header %q: %w", k, err)
		}
		v.Auth.Headers[k] = dec
	}
	if v.Auth.OAuth != nil && v.Auth.OAuth.ClientSecret != "" {
		dec, err := s.encryptor.DecryptString(v.Auth.OAuth.ClientSecret)
		if err != nil {
			return fmt.Errorf("storage: decrypt OAuth client_secret: %w", err)
		}
		v.Auth.OAuth.ClientSecret = dec
	}
	return nil
}
