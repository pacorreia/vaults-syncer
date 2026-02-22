package vault

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pacorreia/vaults-syncer/config"
)

func TestNewGenericBackend(t *testing.T) {
	cfg := &config.VaultConfig{
		ID:       "test",
		Endpoint: "https://example.com",
		Type:     "generic",
		Auth: &config.AuthConfig{
			Method: "bearer",
		},
	}

	client := NewClient(cfg)
	backend := NewGenericBackend(client)

	if backend == nil {
		t.Fatal("expected backend, got nil")
	}

	if backend.client != client {
		t.Error("backend client not set correctly")
	}
}

func TestNewBackend(t *testing.T) {
	tests := []struct {
		name      string
		vaultType string
		wantErr   bool
	}{
		{
			name:      "vaultwarden backend",
			vaultType: "vaultwarden",
			wantErr:   false,
		},
		{
			name:      "vault backend",
			vaultType: "vault",
			wantErr:   false,
		},
		{
			name:      "azure backend",
			vaultType: "azure",
			wantErr:   false,
		},
		{
			name:      "aws backend",
			vaultType: "aws",
			wantErr:   false,
		},
		{
			name:      "generic backend",
			vaultType: "generic",
			wantErr:   false,
		},
		{
			name:      "unknown backend defaults to generic",
			vaultType: "unknown",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.VaultConfig{
				ID:       "test",
				Endpoint: "https://example.com",
				Type:     tt.vaultType,
				Auth: &config.AuthConfig{
					Method: "bearer",
				},
			}

			backend, err := NewBackend(cfg)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if backend == nil {
				t.Fatal("expected backend, got nil")
			}

			if backend.Type() != tt.vaultType {
				t.Errorf("expected type %s, got %s", tt.vaultType, backend.Type())
			}
		})
	}
}

func TestGenericBackendListSecrets(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data": [{"name": "secret1"}, {"name": "secret2"}]}`))
	}))
	defer server.Close()

	cfg := &config.VaultConfig{
		ID:       "test",
		Endpoint: server.URL,
		Type:     "generic",
		Auth: &config.AuthConfig{
			Method: "bearer",
			Headers: map[string]string{
				"token": "test-token",
			},
		},
		FieldNames: config.FieldNamesConfig{
			NameField:  "name",
			ValueField: "value",
		},
	}

	backend, _ := NewBackend(cfg)
	secrets, err := backend.ListSecrets()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(secrets) != 2 {
		t.Errorf("expected 2 secrets, got %d", len(secrets))
	}
}

func TestGenericBackendGetSecret(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"value": "my-value"}`))
	}))
	defer server.Close()

	cfg := &config.VaultConfig{
		ID:       "test",
		Endpoint: server.URL,
		Type:     "generic",
		Auth: &config.AuthConfig{
			Method: "bearer",
			Headers: map[string]string{
				"token": "test-token",
			},
		},
		FieldNames: config.FieldNamesConfig{
			NameField:  "name",
			ValueField: "value",
		},
	}

	backend, _ := NewBackend(cfg)
	secret, err := backend.GetSecret("my-secret")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if secret.Value != "my-value" {
		t.Errorf("expected value my-value, got %s", secret.Value)
	}
}

func TestGenericBackendSetSecret(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok"}`))
	}))
	defer server.Close()

	cfg := &config.VaultConfig{
		ID:       "test",
		Endpoint: server.URL,
		Type:     "generic",
		Method:   "PUT",
		Auth: &config.AuthConfig{
			Method: "bearer",
			Headers: map[string]string{
				"token": "test-token",
			},
		},
		FieldNames: config.FieldNamesConfig{
			NameField:  "name",
			ValueField: "value",
		},
	}

	backend, _ := NewBackend(cfg)
	err := backend.SetSecret("my-secret", "new-value")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGenericBackendDeleteSecret(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &config.VaultConfig{
		ID:       "test",
		Endpoint: server.URL,
		Type:     "generic",
		Auth: &config.AuthConfig{
			Method: "bearer",
			Headers: map[string]string{
				"token": "test-token",
			},
		},
		FieldNames: config.FieldNamesConfig{
			NameField:  "name",
			ValueField: "value",
		},
	}

	backend, _ := NewBackend(cfg)
	err := backend.DeleteSecret("my-secret")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGenericBackendTestConnection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data": []}`))
	}))
	defer server.Close()

	cfg := &config.VaultConfig{
		ID:       "test",
		Endpoint: server.URL,
		Type:     "generic",
		Auth: &config.AuthConfig{
			Method: "bearer",
			Headers: map[string]string{
				"token": "test-token",
			},
		},
		FieldNames: config.FieldNamesConfig{
			NameField:  "name",
			ValueField: "value",
		},
	}

	backend, _ := NewBackend(cfg)
	err := backend.TestConnection()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGenericBackendType(t *testing.T) {
	tests := []struct {
		name      string
		vaultType string
	}{
		{"vaultwarden type", "vaultwarden"},
		{"vault type", "vault"},
		{"generic type", "generic"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.VaultConfig{
				ID:       "test",
				Endpoint: "https://example.com",
				Type:     tt.vaultType,
				Auth: &config.AuthConfig{
					Method: "bearer",
				},
			}

			backend, _ := NewBackend(cfg)
			typ := backend.Type()

			if typ != tt.vaultType {
				t.Errorf("expected type %s, got %s", tt.vaultType, typ)
			}
		})
	}
}

func TestGenericBackendCapabilities(t *testing.T) {
	cfg := &config.VaultConfig{
		ID:       "test",
		Endpoint: "https://example.com",
		Type:     "generic",
		Auth: &config.AuthConfig{
			Method: "bearer",
		},
	}

	backend, _ := NewBackend(cfg)
	caps := backend.Capabilities()

	if !caps.CanList {
		t.Error("expected CanList to be true")
	}
	if !caps.CanGet {
		t.Error("expected CanGet to be true")
	}
	if !caps.CanSet {
		t.Error("expected CanSet to be true")
	}
	if !caps.CanDelete {
		t.Error("expected CanDelete to be true")
	}
	if !caps.CanSync {
		t.Error("expected CanSync to be true")
	}
}
