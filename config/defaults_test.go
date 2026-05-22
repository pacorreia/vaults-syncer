package config

import (
	"testing"
)

func TestApplyServerDefaults(t *testing.T) {
	cfg := &Config{}
	applyServerDefaults(cfg)

	if cfg.Server.Port != 8080 {
		t.Errorf("expected port 8080, got %d", cfg.Server.Port)
	}
	if cfg.Server.Address != "0.0.0.0" {
		t.Errorf("expected address '0.0.0.0', got %q", cfg.Server.Address)
	}
	if cfg.Server.MetricsPort != 9090 {
		t.Errorf("expected metrics port 9090, got %d", cfg.Server.MetricsPort)
	}
	if cfg.Server.MetricsAddress != "0.0.0.0" {
		t.Errorf("expected metrics address '0.0.0.0', got %q", cfg.Server.MetricsAddress)
	}
}

func TestApplyServerDefaultsPreservesExisting(t *testing.T) {
	cfg := &Config{}
	cfg.Server.Port = 9000
	cfg.Server.Address = "127.0.0.1"
	applyServerDefaults(cfg)

	if cfg.Server.Port != 9000 {
		t.Errorf("expected port 9000, got %d", cfg.Server.Port)
	}
	if cfg.Server.Address != "127.0.0.1" {
		t.Errorf("expected address '127.0.0.1', got %q", cfg.Server.Address)
	}
}

func TestApplyLoggingDefaults(t *testing.T) {
	cfg := &Config{}
	applyLoggingDefaults(cfg)

	if cfg.Logging.Level != "info" {
		t.Errorf("expected level 'info', got %q", cfg.Logging.Level)
	}
	if cfg.Logging.Format != "json" {
		t.Errorf("expected format 'json', got %q", cfg.Logging.Format)
	}
}

func TestApplyLoggingDefaultsPreservesExisting(t *testing.T) {
	cfg := &Config{Logging: LoggingConfig{Level: "debug", Format: "text"}}
	applyLoggingDefaults(cfg)

	if cfg.Logging.Level != "debug" {
		t.Errorf("expected level 'debug', got %q", cfg.Logging.Level)
	}
}

func TestApplySyncDefaults(t *testing.T) {
	cfg := &Config{
		Syncs: []SyncConfig{
			{ID: "s1"},
		},
	}
	applySyncDefaults(cfg)

	s := cfg.Syncs[0]
	if s.RetryPolicy.MaxRetries != 3 {
		t.Errorf("expected max_retries 3, got %d", s.RetryPolicy.MaxRetries)
	}
	if s.RetryPolicy.InitialBackoff != 1000 {
		t.Errorf("expected initial_backoff 1000, got %d", s.RetryPolicy.InitialBackoff)
	}
	if s.RetryPolicy.MaxBackoff != 60000 {
		t.Errorf("expected max_backoff 60000, got %d", s.RetryPolicy.MaxBackoff)
	}
	if s.RetryPolicy.Multiplier != 2.0 {
		t.Errorf("expected multiplier 2.0, got %f", s.RetryPolicy.Multiplier)
	}
	if s.SyncType != "unidirectional" {
		t.Errorf("expected sync_type 'unidirectional', got %q", s.SyncType)
	}
	if !s.IsEnabled() {
		t.Error("expected sync to be enabled by default")
	}
}

func TestApplyVaultDefaults(t *testing.T) {
	cfg := &Config{
		Vaults: []VaultConfig{
			{ID: "v1", Auth: &AuthConfig{}},
		},
	}
	applyVaultDefaults(cfg)

	v := cfg.Vaults[0]
	if v.Method != "PUT" {
		t.Errorf("expected method 'PUT', got %q", v.Method)
	}
	if v.Timeout != 30 {
		t.Errorf("expected timeout 30, got %d", v.Timeout)
	}
	if v.Auth.Headers == nil {
		t.Error("expected auth headers map to be initialized")
	}
}
