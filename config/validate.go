package config

import (
	"fmt"
	"strings"
)

// Validate checks the configuration for errors.
func (c *Config) Validate() error {
	if len(c.Vaults) == 0 {
		return fmt.Errorf("no vaults configured")
	}

	vaultIDs, err := validateVaults(c.Vaults)
	if err != nil {
		return err
	}

	return validateSyncs(c.Syncs, vaultIDs)
}

// validateVaults validates each vault entry and returns a set of valid vault IDs.
func validateVaults(vaults []VaultConfig) (map[string]bool, error) {
	vaultIDs := make(map[string]bool)
	for _, v := range vaults {
		if v.ID == "" {
			return nil, fmt.Errorf("vault must have an ID")
		}
		if vaultIDs[v.ID] {
			return nil, fmt.Errorf("duplicate vault ID: %s", v.ID)
		}
		vaultIDs[v.ID] = true

		if err := validateSingleVault(v); err != nil {
			return nil, err
		}
	}
	return vaultIDs, nil
}

// validateSingleVault validates a single vault configuration entry.
func validateSingleVault(v VaultConfig) error {
	// Check for legacy format - NO LONGER SUPPORTED
	if v.LegacyAuthMethod != "" || len(v.LegacyAuthHeaders) > 0 {
		return fmt.Errorf("vault %s: legacy 'auth_method' and 'auth_headers' fields are no longer supported. Please migrate to the new 'auth' structure. See README.md for examples", v.ID)
	}

	if strings.ToLower(v.Type) == "tool" {
		return validateToolVault(v)
	}
	return validateHTTPVault(v)
}

// validateToolVault validates a tool-type vault (CLI-backed, no HTTP endpoint required).
func validateToolVault(v VaultConfig) error {
	if v.ToolConfig == "" {
		return fmt.Errorf("vault %s: 'tool_config' is required for type 'tool'", v.ID)
	}
	if v.ResolvedTool == nil {
		return fmt.Errorf("vault %s: tool config file '%s' was not loaded", v.ID, v.ToolConfig)
	}
	ops := v.ResolvedTool.Operations
	if ops == nil || ops["list"] == nil {
		return fmt.Errorf("vault %s: tool config must define an 'operations.list' entry", v.ID)
	}
	if ops["list"].Command == "" {
		return fmt.Errorf("vault %s: tool config 'operations.list.command' must not be empty", v.ID)
	}
	if ops["get"] == nil {
		return fmt.Errorf("vault %s: tool config must define an 'operations.get' entry", v.ID)
	}
	if ops["get"].Command == "" {
		return fmt.Errorf("vault %s: tool config 'operations.get.command' must not be empty", v.ID)
	}
	return nil
}

// validateHTTPVault validates an HTTP-based vault configuration.
func validateHTTPVault(v VaultConfig) error {
	if v.Endpoint == "" {
		return fmt.Errorf("vault %s must have an endpoint", v.ID)
	}
	if v.FieldNames.NameField == "" || v.FieldNames.ValueField == "" {
		return fmt.Errorf("vault %s must define field_names.name_field and field_names.value_field", v.ID)
	}
	if v.Auth == nil {
		return fmt.Errorf("vault %s: 'auth' configuration is required", v.ID)
	}
	if v.Auth.Method == "" {
		return fmt.Errorf("vault %s: 'auth.method' is required", v.ID)
	}

	authMethod := strings.ToLower(v.Auth.Method)
	if authMethod != "bearer" && authMethod != "basic" && authMethod != "oauth2" && authMethod != "api_key" && authMethod != "custom" {
		return fmt.Errorf("vault %s: invalid auth method '%s', must be one of: bearer, basic, oauth2, api_key, custom", v.ID, v.Auth.Method)
	}

	method := strings.ToUpper(v.Method)
	if method != "PUT" && method != "POST" {
		return fmt.Errorf("vault %s has invalid method %s, must be PUT or POST", v.ID, v.Method)
	}
	return nil
}

// validateSyncs validates each sync entry against the known vault IDs.
func validateSyncs(syncs []SyncConfig, vaultIDs map[string]bool) error {
	for _, s := range syncs {
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

		if s.SyncType == "bidirectional" && len(s.Targets) != 1 {
			return fmt.Errorf("sync %s: bidirectional sync only allowed for 1:1 (has %d targets)", s.ID, len(s.Targets))
		}
		if s.SyncType != "unidirectional" && s.SyncType != "bidirectional" {
			return fmt.Errorf("sync %s: invalid sync_type %s, must be unidirectional or bidirectional", s.ID, s.SyncType)
		}
	}
	return nil
}
