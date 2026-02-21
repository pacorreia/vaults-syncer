package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// LoadConfig loads the configuration from a YAML file
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Perform environment variable substitution
	configStr := os.ExpandEnv(string(data))

	var cfg Config
	if err := yaml.Unmarshal([]byte(configStr), &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Set defaults
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8080
	}
	if cfg.Server.Address == "" {
		cfg.Server.Address = "0.0.0.0"
	}
	if cfg.Server.MetricsPort == 0 {
		cfg.Server.MetricsPort = 9090
	}
	if cfg.Server.MetricsAddress == "" {
		cfg.Server.MetricsAddress = "0.0.0.0"
	}
	if cfg.Logging.Level == "" {
		cfg.Logging.Level = "info"
	}
	if cfg.Logging.Format == "" {
		cfg.Logging.Format = "json"
	}

	// Set retry policy defaults
	for i := range cfg.Syncs {
		if cfg.Syncs[i].RetryPolicy.MaxRetries == 0 {
			cfg.Syncs[i].RetryPolicy.MaxRetries = 3
		}
		if cfg.Syncs[i].RetryPolicy.InitialBackoff == 0 {
			cfg.Syncs[i].RetryPolicy.InitialBackoff = 1000
		}
		if cfg.Syncs[i].RetryPolicy.MaxBackoff == 0 {
			cfg.Syncs[i].RetryPolicy.MaxBackoff = 60000
		}
		if cfg.Syncs[i].RetryPolicy.Multiplier == 0 {
			cfg.Syncs[i].RetryPolicy.Multiplier = 2.0
		}
		if cfg.Syncs[i].Enabled == false && cfg.Syncs[i].ID != "" {
			// Default to enabled if not explicitly set
			cfg.Syncs[i].Enabled = true
		}
		if cfg.Syncs[i].SyncType == "" {
			cfg.Syncs[i].SyncType = "unidirectional"
		}
	}

	// Set vault defaults (only optional fields)
	for i := range cfg.Vaults {
		if cfg.Vaults[i].Method == "" {
			cfg.Vaults[i].Method = "PUT"
		}
		if cfg.Vaults[i].Timeout == 0 {
			cfg.Vaults[i].Timeout = 30
		}
		// Initialize Auth.Headers map if Auth exists but Headers is nil
		if cfg.Vaults[i].Auth != nil && cfg.Vaults[i].Auth.Headers == nil {
			cfg.Vaults[i].Auth.Headers = make(map[string]string)
		}
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// Validate checks the configuration for errors
func (c *Config) Validate() error {
	if len(c.Vaults) == 0 {
		return fmt.Errorf("no vaults configured")
	}

	vaultIDs := make(map[string]bool)
	for _, v := range c.Vaults {
		if v.ID == "" {
			return fmt.Errorf("vault must have an ID")
		}
		if v.Endpoint == "" {
			return fmt.Errorf("vault %s must have an endpoint", v.ID)
		}
		if v.FieldNames.NameField == "" || v.FieldNames.ValueField == "" {
			return fmt.Errorf("vault %s must define field_names.name_field and field_names.value_field", v.ID)
		}
		if vaultIDs[v.ID] {
			return fmt.Errorf("duplicate vault ID: %s", v.ID)
		}
		vaultIDs[v.ID] = true

		// Check for legacy format - NO LONGER SUPPORTED
		if v.LegacyAuthMethod != "" || len(v.LegacyAuthHeaders) > 0 {
			return fmt.Errorf("vault %s: legacy 'auth_method' and 'auth_headers' fields are no longer supported. Please migrate to the new 'auth' structure. See README.md for examples", v.ID)
		}

		// Validate Auth structure is present
		if v.Auth == nil {
			return fmt.Errorf("vault %s: 'auth' configuration is required", v.ID)
		}
		if v.Auth.Method == "" {
			return fmt.Errorf("vault %s: 'auth.method' is required", v.ID)
		}

		// Validate auth method is supported
		authMethod := strings.ToLower(v.Auth.Method)
		if authMethod != "bearer" && authMethod != "basic" && authMethod != "oauth2" && authMethod != "api_key" && authMethod != "custom" {
			return fmt.Errorf("vault %s: invalid auth method '%s', must be one of: bearer, basic, oauth2, api_key, custom", v.ID, v.Auth.Method)
		}

		// Validate method
		method := strings.ToUpper(v.Method)
		if method != "PUT" && method != "POST" {
			return fmt.Errorf("vault %s has invalid method %s, must be PUT or POST", v.ID, v.Method)
		}
	}

	for _, s := range c.Syncs {
		if s.ID == "" {
			return fmt.Errorf("sync must have an ID")
		}
		if s.Source == "" {
			return fmt.Errorf("sync %s must have a source", s.ID)
		}
		if len(s.Targets) == 0 {
			return fmt.Errorf("sync %s must have at least one target", s.ID)
		}
		if !vaultIDs[s.Source] {
			return fmt.Errorf("sync %s references unknown source vault %s", s.ID, s.Source)
		}
		for _, t := range s.Targets {
			if !vaultIDs[t] {
				return fmt.Errorf("sync %s references unknown target vault %s", s.ID, t)
			}
		}

		// Bidirectional only allowed for 1:1
		if s.SyncType == "bidirectional" && len(s.Targets) != 1 {
			return fmt.Errorf("sync %s: bidirectional sync only allowed for 1:1 (has %d targets)", s.ID, len(s.Targets))
		}
		if s.SyncType != "unidirectional" && s.SyncType != "bidirectional" {
			return fmt.Errorf("sync %s: invalid sync_type %s, must be unidirectional or bidirectional", s.ID, s.SyncType)
		}
	}

	return nil
}
