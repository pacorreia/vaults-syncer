package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		envVars     map[string]string
		wantErr     bool
		validateFn  func(*testing.T, *Config)
	}{
		{
			name: "valid basic config",
			content: `
vaults:
  - id: test-vault
    name: Test Vault
    type: generic
    endpoint: https://example.com
    method: PUT
    auth:
      method: bearer
      headers:
        Authorization: Bearer token123
    field_names:
      name_field: name
      value_field: value
syncs:
  - id: test-sync
    source: test-vault
    targets:
      - test-vault
    enabled: true
server:
  port: 8080
logging:
  level: info
`,
			wantErr: false,
			validateFn: func(t *testing.T, cfg *Config) {
				if len(cfg.Vaults) != 1 {
					t.Errorf("expected 1 vault, got %d", len(cfg.Vaults))
				}
				if cfg.Vaults[0].ID != "test-vault" {
					t.Errorf("expected vault ID 'test-vault', got '%s'", cfg.Vaults[0].ID)
				}
				if cfg.Server.Port != 8080 {
					t.Errorf("expected port 8080, got %d", cfg.Server.Port)
				}
			},
		},
		{
			name: "config with environment variable substitution",
			content: `
vaults:
  - id: test-vault
    name: $TEST_VAULT_NAME
    type: generic
    endpoint: ${TEST_ENDPOINT}
    method: PUT
    auth:
      method: bearer
    field_names:
      name_field: name
      value_field: value
syncs: []
`,
			envVars: map[string]string{
				"TEST_VAULT_NAME": "My Test Vault",
				"TEST_ENDPOINT":   "https://test.example.com",
			},
			wantErr: false,
			validateFn: func(t *testing.T, cfg *Config) {
				if cfg.Vaults[0].Name != "My Test Vault" {
					t.Errorf("expected vault name 'My Test Vault', got '%s'", cfg.Vaults[0].Name)
				}
				if cfg.Vaults[0].Endpoint != "https://test.example.com" {
					t.Errorf("expected endpoint 'https://test.example.com', got '%s'", cfg.Vaults[0].Endpoint)
				}
			},
		},
		{
			name: "config with defaults",
			content: `
vaults:
  - id: default-vault
    type: generic
    endpoint: https://default.example.com
    auth:
      method: bearer
    field_names:
      name_field: name
      value_field: value
syncs: []
`,
			wantErr: false,
			validateFn: func(t *testing.T, cfg *Config) {
				if cfg.Server.Port != 8080 {
					t.Errorf("expected default port 8080, got %d", cfg.Server.Port)
				}
				if cfg.Server.Address != "0.0.0.0" {
					t.Errorf("expected default address '0.0.0.0', got '%s'", cfg.Server.Address)
				}
				if cfg.Logging.Level != "info" {
					t.Errorf("expected default logging level 'info', got '%s'", cfg.Logging.Level)
				}
				// Check vault defaults
				if cfg.Vaults[0].Method != "PUT" {
					t.Errorf("expected default method 'PUT', got '%s'", cfg.Vaults[0].Method)
				}
			},
		},
		{
			name: "invalid yaml",
			content: `
vaults: [
  - id: test
    invalid yaml here
`,
			wantErr: true,
		},
		{
			name:    "non-existent file",
			content: "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables
			for k, v := range tt.envVars {
				os.Setenv(k, v)
				defer os.Unsetenv(k)
			}

			var configPath string
			if tt.name != "non-existent file" {
				// Create temporary config file
				tmpDir := t.TempDir()
				configPath = filepath.Join(tmpDir, "config.yaml")
				if err := os.WriteFile(configPath, []byte(tt.content), 0644); err != nil {
					t.Fatalf("failed to create temp config file: %v", err)
				}
			} else {
				configPath = "/nonexistent/path/config.yaml"
			}

			// Load config
			cfg, err := LoadConfig(configPath)

			// Check error expectation
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Validate config
			if tt.validateFn != nil {
				tt.validateFn(t, cfg)
			}
		})
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: &Config{
				Vaults: []VaultConfig{
					{
						ID:       "vault1",
						Name:     "Test Vault",
						Type:     "generic",
						Endpoint: "https://example.com",
						Method:   "PUT",
						Auth: &AuthConfig{
							Method: "bearer",
						},
						FieldNames: FieldNamesConfig{
							NameField:  "name",
							ValueField: "value",
						},
					},
				},
				Syncs: []SyncConfig{
					{
						ID:       "sync1",
						Source:   "vault1",
						Targets:  []string{"vault1"},
						SyncType: "unidirectional",
						Enabled:  true,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "duplicate vault IDs",
			config: &Config{
				Vaults: []VaultConfig{
					{ID: "vault1", Name: "Vault 1", Type: "generic", Endpoint: "https://example.com",
						Method: "PUT", Auth: &AuthConfig{Method: "bearer"},
						FieldNames: FieldNamesConfig{NameField: "name", ValueField: "value"}},
					{ID: "vault1", Name: "Vault 2", Type: "generic", Endpoint: "https://example.com",
						Method: "PUT", Auth: &AuthConfig{Method: "bearer"},
						FieldNames: FieldNamesConfig{NameField: "name", ValueField: "value"}},
				},
				Syncs: []SyncConfig{},
			},
			wantErr: true,
			errMsg:  "duplicate vault ID",
		},
		{
			name: "missing vault endpoint",
			config: &Config{
				Vaults: []VaultConfig{
					{ID: "vault1", Name: "Vault 1", Type: "generic", Endpoint: "",
						Method:     "PUT",
						Auth:       &AuthConfig{Method: "bearer"},
						FieldNames: FieldNamesConfig{NameField: "name", ValueField: "value"}},
				},
				Syncs: []SyncConfig{},
			},
			wantErr: true,
			errMsg:  "endpoint",
		},
		{
			name: "sync references non-existent vault",
			config: &Config{
				Vaults: []VaultConfig{
					{ID: "vault1", Name: "Vault 1", Type: "generic", Endpoint: "https://example.com",
						Method:     "PUT",
						Auth:       &AuthConfig{Method: "bearer"},
						FieldNames: FieldNamesConfig{NameField: "name", ValueField: "value"}},
				},
				Syncs: []SyncConfig{
					{
						ID:      "sync1",
						Source:  "nonexistent",
						Targets: []string{"vault1"},
						Enabled: true,
					},
				},
			},
			wantErr: true,
			errMsg:  "source vault",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error containing '%s', got nil", tt.errMsg)
					return
				}
				if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing '%s', got '%s'", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && 
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || 
		 containsHelper(s, substr)))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestGetVaultType(t *testing.T) {
	tests := []struct {
		name     string
		config   VaultConfig
		expected string
	}{
		{
			name: "explicit vaultwarden type",
			config: VaultConfig{
				Type: "vaultwarden",
			},
			expected: "vaultwarden",
		},
		{
			name: "explicit vault type",
			config: VaultConfig{
				Type: "vault",
			},
			expected: "vault",
		},
		{
			name: "explicit generic type",
			config: VaultConfig{
				Type: "generic",
			},
			expected: "generic",
		},
		{
			name:     "empty type defaults to generic",
			config:   VaultConfig{},
			expected: "generic",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.config.GetVaultType()
			if got != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, got)
			}
		})
	}
}

func TestPopulateDefaults(t *testing.T) {
	tests := []struct {
		name   string
		config VaultConfig
		check  func(*testing.T, *VaultConfig)
	}{
		{
			name: "initializes nil headers",
			config: VaultConfig{
				Headers: nil,
			},
			check: func(t *testing.T, vc *VaultConfig) {
				if vc.Headers == nil {
					t.Error("expected headers map to be initialized")
				}
			},
		},
		{
			name: "initializes nil auth headers",
			config: VaultConfig{
				Auth: &AuthConfig{
					Headers: nil,
				},
			},
			check: func(t *testing.T, vc *VaultConfig) {
				if vc.Auth.Headers == nil {
					t.Error("expected auth headers map to be initialized")
				}
			},
		},
		{
			name: "sets default type to generic",
			config: VaultConfig{
				Type: "",
			},
			check: func(t *testing.T, vc *VaultConfig) {
				if vc.Type != "generic" {
					t.Errorf("expected type 'generic', got '%s'", vc.Type)
				}
			},
		},
		{
			name: "preserves existing type",
			config: VaultConfig{
				Type: "vaultwarden",
			},
			check: func(t *testing.T, vc *VaultConfig) {
				if vc.Type != "vaultwarden" {
					t.Errorf("expected type 'vaultwarden', got '%s'", vc.Type)
				}
			},
		},
		{
			name: "preserves existing headers",
			config: VaultConfig{
				Headers: map[string]string{
					"X-Custom": "value",
				},
			},
			check: func(t *testing.T, vc *VaultConfig) {
				if val, ok := vc.Headers["X-Custom"]; !ok || val != "value" {
					t.Error("expected existing header to be preserved")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.config.PopulateDefaults()
			tt.check(t, &tt.config)
		})
	}
}

func TestValidateLegacyAuth(t *testing.T) {
	cfg := &Config{
		Vaults: []VaultConfig{
			{
				ID:                 "vault1",
				Endpoint:           "https://example.com",
				LegacyAuthMethod:   "bearer",
				LegacyAuthHeaders:  map[string]string{"key": "value"},
				FieldNames: FieldNamesConfig{
					NameField:  "name",
					ValueField: "value",
				},
			},
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for legacy auth fields")
	}
	if !strings.Contains(err.Error(), "legacy") {
		t.Errorf("expected legacy auth error, got: %v", err)
	}
}

func TestValidateAuthMethod(t *testing.T) {
	tests := []struct {
		name    string
		method  string
		wantErr bool
	}{
		{"bearer valid", "bearer", false},
		{"basic valid", "basic", false},
		{"oauth2 valid", "oauth2", false},
		{"api_key valid", "api_key", false},
		{"custom valid", "custom", false},
		{"invalid method", "invalid", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Vaults: []VaultConfig{
					{
						ID:       "vault1",
						Endpoint: "https://example.com",
						Method:   "PUT",
						Auth: &AuthConfig{
							Method: tt.method,
						},
						FieldNames: FieldNamesConfig{
							NameField:  "name",
							ValueField: "value",
						},
					},
				},
			}

			err := cfg.Validate()
			if tt.wantErr {
				if err == nil {
					t.Error("expected error for invalid auth method")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestValidateSyncType(t *testing.T) {
	tests := []struct {
		name    string
		syncType string
		targets int
		wantErr bool
	}{
		{"unidirectional", "unidirectional", 2, false},
		{"bidirectional 1:1", "bidirectional", 1, false},
		{"bidirectional multiple targets", "bidirectional", 2, true},
		{"invalid sync type", "invalid", 1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			targets := make([]string, tt.targets)
			for i := 0; i < tt.targets; i++ {
				targets[i] = fmt.Sprintf("vault%d", i+2)
			}

			vaults := []VaultConfig{
				{
					ID:       "vault1",
					Endpoint: "https://example.com",
					Method:   "PUT",
					Auth: &AuthConfig{
						Method: "bearer",
					},
					FieldNames: FieldNamesConfig{
						NameField:  "name",
						ValueField: "value",
					},
				},
			}

			// Add target vaults
			for i := 2; i <= tt.targets+1; i++ {
				vaults = append(vaults, VaultConfig{
					ID:       fmt.Sprintf("vault%d", i),
					Endpoint: "https://example.com",
					Method:   "PUT",
					Auth: &AuthConfig{
						Method: "bearer",
					},
					FieldNames: FieldNamesConfig{
						NameField:  "name",
						ValueField: "value",
					},
				})
			}

			cfg := &Config{
				Vaults: vaults,
				Syncs: []SyncConfig{
					{
						ID:       "sync1",
						Source:   "vault1",
						Targets:  targets,
						SyncType: tt.syncType,
					},
				},
			}

			err := cfg.Validate()
			if tt.wantErr {
				if err == nil {
					t.Error("expected validation error")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestLoadConfigFileNotFound(t *testing.T) {
	_, err := LoadConfig("/nonexistent/file.yaml")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestLoadConfigInvalidYAML(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "invalid-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	tmpFile.WriteString("invalid: yaml: [")
	tmpFile.Close()

	_, err = LoadConfig(tmpFile.Name())
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}
