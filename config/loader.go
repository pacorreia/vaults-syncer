package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// LoadConfig loads and validates the configuration from a YAML file at the given path.
// It performs environment variable expansion, applies defaults, resolves external tool
// config files, and validates the resulting configuration.
func LoadConfig(path string) (*Config, error) {
	data, err := readConfigFile(path)
	if err != nil {
		return nil, err
	}

	expanded := expandConfigEnvVars(data)

	cfg, err := parseConfigYAML(expanded)
	if err != nil {
		return nil, err
	}

	applyConfigDefaults(cfg)

	configDir := filepath.Dir(path)
	if err := resolveToolConfigs(cfg, configDir); err != nil {
		return nil, err
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// readConfigFile reads the raw bytes of a config file.
func readConfigFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	return data, nil
}

// expandConfigEnvVars replaces ${VAR} and $VAR references with their environment values.
func expandConfigEnvVars(data []byte) string {
	return os.ExpandEnv(string(data))
}

// parseConfigYAML unmarshals the YAML string into a Config struct.
func parseConfigYAML(data string) (*Config, error) {
	var cfg Config
	if err := yaml.Unmarshal([]byte(data), &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}
	return &cfg, nil
}

// resolveToolConfigs loads the ExternalToolConfig YAML file referenced by each
// tool-type vault and stores the result in VaultConfig.ResolvedTool.
// Paths in tool_config are resolved relative to configDir (the directory of the main config file).
func resolveToolConfigs(cfg *Config, configDir string) error {
	for i := range cfg.Vaults {
		v := &cfg.Vaults[i]
		if strings.ToLower(v.Type) != "tool" {
			continue
		}
		if v.ToolConfig == "" {
			// Validation will produce a descriptive error; skip here.
			continue
		}

		toolPath := v.ToolConfig
		if !filepath.IsAbs(toolPath) {
			toolPath = filepath.Join(configDir, toolPath)
		}

		toolCfg, err := loadExternalToolConfig(toolPath)
		if err != nil {
			return fmt.Errorf("vault %s: failed to load tool config '%s': %w", v.ID, v.ToolConfig, err)
		}
		v.ResolvedTool = toolCfg
	}
	return nil
}

// loadExternalToolConfig reads and parses a single ExternalToolConfig YAML file.
func loadExternalToolConfig(path string) (*ExternalToolConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read tool config file: %w", err)
	}

	expanded := os.ExpandEnv(string(data))

	var toolCfg ExternalToolConfig
	if err := yaml.Unmarshal([]byte(expanded), &toolCfg); err != nil {
		return nil, fmt.Errorf("failed to parse tool config file: %w", err)
	}

	// Default success_exit_codes to [0] for any operation that omits it.
	for _, op := range toolCfg.Operations {
		if op != nil && len(op.SuccessExitCodes) == 0 {
			op.SuccessExitCodes = []int{0}
		}
	}

	return &toolCfg, nil
}
