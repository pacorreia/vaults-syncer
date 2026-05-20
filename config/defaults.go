package config

// applyConfigDefaults fills in sensible defaults for all sections of the config.
func applyConfigDefaults(cfg *Config) {
	applyServerDefaults(cfg)
	applyLoggingDefaults(cfg)
	applySyncDefaults(cfg)
	applyVaultDefaults(cfg)
}

// applyServerDefaults sets default values for the HTTP server configuration.
func applyServerDefaults(cfg *Config) {
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
}

// applyLoggingDefaults sets default values for the logging configuration.
func applyLoggingDefaults(cfg *Config) {
	if cfg.Logging.Level == "" {
		cfg.Logging.Level = "info"
	}
	if cfg.Logging.Format == "" {
		cfg.Logging.Format = "json"
	}
}

// applySyncDefaults sets default values for each sync entry.
func applySyncDefaults(cfg *Config) {
	for i := range cfg.Syncs {
		s := &cfg.Syncs[i]
		if s.RetryPolicy.MaxRetries == 0 {
			s.RetryPolicy.MaxRetries = 3
		}
		if s.RetryPolicy.InitialBackoff == 0 {
			s.RetryPolicy.InitialBackoff = 1000
		}
		if s.RetryPolicy.MaxBackoff == 0 {
			s.RetryPolicy.MaxBackoff = 60000
		}
		if s.RetryPolicy.Multiplier == 0 {
			s.RetryPolicy.Multiplier = 2.0
		}
		if !s.Enabled && s.ID != "" {
			s.Enabled = true
		}
		if s.SyncType == "" {
			s.SyncType = "unidirectional"
		}
	}
}

// applyVaultDefaults sets default values for each vault entry.
func applyVaultDefaults(cfg *Config) {
	for i := range cfg.Vaults {
		v := &cfg.Vaults[i]
		if v.Method == "" {
			v.Method = "PUT"
		}
		if v.Timeout == 0 {
			v.Timeout = 30
		}
		if v.Auth != nil && v.Auth.Headers == nil {
			v.Auth.Headers = make(map[string]string)
		}
	}
}
