package vault

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pacorreia/vaults-syncer/config"
)

func TestNewClient(t *testing.T) {
	cfg := &config.VaultConfig{
		ID:       "test",
		Endpoint: "https://example.com",
		Type:     "generic",
		Auth: &config.AuthConfig{
			Method: "bearer",
		},
		Timeout: 30,
	}

	client := NewClient(cfg)
	if client == nil {
		t.Fatal("expected client, got nil")
	}
	if client.cfg != cfg {
		t.Error("client config not set correctly")
	}
	if client.parser == nil {
		t.Error("parser not initialized")
	}
}

func TestClientListSecrets(t *testing.T) {
	tests := []struct {
		name           string
		responseBody   string
		responseStatus int
		wantSecrets    []string
		wantErr        bool
	}{
		{
			name: "successful list",
			responseBody: `{
				"data": [
					{"name": "secret1"},
					{"name": "secret2"},
					{"name": "secret3"}
				]
			}`,
			responseStatus: http.StatusOK,
			wantSecrets:    []string{"secret1", "secret2", "secret3"},
			wantErr:        false,
		},
		{
			name:           "empty list",
			responseBody:   `{"data": []}`,
			responseStatus: http.StatusOK,
			wantSecrets:    []string{},
			wantErr:        false,
		},
		{
			name:           "server error",
			responseBody:   `{"error": "internal server error"}`,
			responseStatus: http.StatusInternalServerError,
			wantSecrets:    nil,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.responseStatus)
				w.Write([]byte(tt.responseBody))
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

			client := NewClient(cfg)
			secrets, err := client.ListSecrets()

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(secrets) != len(tt.wantSecrets) {
				t.Errorf("expected %d secrets, got %d", len(tt.wantSecrets), len(secrets))
				return
			}

			for i, want := range tt.wantSecrets {
				if secrets[i] != want {
					t.Errorf("secret[%d]: expected %s, got %s", i, want, secrets[i])
				}
			}
		})
	}
}

func TestClientGetSecret(t *testing.T) {
	tests := []struct {
		name         string
		secretName   string
		response     string
		wantValue    string
		wantErr      bool
	}{
		{
			name:       "successful get",
			secretName: "my-secret",
			response: `{
				"data": [
					{"name": "my-secret", "value": "secret-value-123"},
					{"name": "other-secret", "value": "other-value"}
				]
			}`,
			wantValue: "secret-value-123",
			wantErr:   false,
		},
		{
			name:       "secret not found in list",
			secretName: "nonexistent",
			response: `{
				"data": [
					{"name": "my-secret", "value": "value1"}
				]
			}`,
			wantValue: "",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(tt.response))
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

			client := NewClient(cfg)
			secret, err := client.GetSecret(tt.secretName)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if secret.Value != tt.wantValue {
				t.Errorf("expected value %s, got %s", tt.wantValue, secret.Value)
			}
		})
	}
}

func TestClientSetSecret(t *testing.T) {
	tests := []struct {
		name           string
		secretName     string
		secretValue    string
		responseStatus int
		wantErr        bool
	}{
		{
			name:           "successful set",
			secretName:     "my-secret",
			secretValue:    "new-value",
			responseStatus: http.StatusOK,
			wantErr:        false,
		},
		{
			name:           "successful set with 201",
			secretName:     "my-secret",
			secretValue:    "new-value",
			responseStatus: http.StatusCreated,
			wantErr:        false,
		},
		{
			name:           "unauthorized",
			secretName:     "my-secret",
			secretValue:    "new-value",
			responseStatus: http.StatusUnauthorized,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify request method
				if r.Method != "PUT" && r.Method != "POST" {
					t.Errorf("expected PUT or POST, got %s", r.Method)
				}

				// Verify request body
				body, _ := io.ReadAll(r.Body)
				var data map[string]string
				json.Unmarshal(body, &data)
				if data["value"] != tt.secretValue {
					t.Errorf("expected value %s in body, got %s", tt.secretValue, data["value"])
				}

				w.WriteHeader(tt.responseStatus)
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

			client := NewClient(cfg)
			err := client.SetSecret(tt.secretName, tt.secretValue)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestClientDeleteSecret(t *testing.T) {
	tests := []struct {
		name           string
		secretName     string
		responseStatus int
		wantErr        bool
	}{
		{
			name:           "successful delete",
			secretName:     "my-secret",
			responseStatus: http.StatusOK,
			wantErr:        false,
		},
		{
			name:           "successful delete with 204",
			secretName:     "my-secret",
			responseStatus: http.StatusNoContent,
			wantErr:        false,
		},
		{
			name:           "not found",
			secretName:     "nonexistent",
			responseStatus: http.StatusNotFound,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "DELETE" {
					t.Errorf("expected DELETE, got %s", r.Method)
				}
				w.WriteHeader(tt.responseStatus)
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

			client := NewClient(cfg)
			err := client.DeleteSecret(tt.secretName)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestClientTestConnection(t *testing.T) {
	tests := []struct {
		name           string
		responseStatus int
		responseBody   string
		wantErr        bool
	}{
		{
			name:           "successful connection",
			responseStatus: http.StatusOK,
			responseBody:   `{"data": []}`,
			wantErr:        false,
		},
		{
			name:           "unauthorized still no error",
			responseStatus: http.StatusUnauthorized,
			responseBody:   `{"error": "unauthorized"}`,
			wantErr:        false, // TestConnection returns nil for both 200 and 401
		},
		{
			name:           "server error fails",
			responseStatus: http.StatusInternalServerError,
			responseBody:   `{"error": "internal error"}`,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.responseStatus)
				w.Write([]byte(tt.responseBody))
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

			client := NewClient(cfg)
			err := client.TestConnection()

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestAddAuthHeaders(t *testing.T) {
	tests := []struct {
		name           string
		authConfig     *config.AuthConfig
		expectedHeader string
		expectedValue  string
	}{
		{
			name: "bearer token",
			authConfig: &config.AuthConfig{
				Method: "bearer",
				Headers: map[string]string{
					"token": "my-token",
				},
			},
			expectedHeader: "Authorization",
			expectedValue:  "Bearer my-token",
		},
		{
			name: "basic auth",
			authConfig: &config.AuthConfig{
				Method: "basic",
				Headers: map[string]string{
					"username": "user",
					"password": "pass",
				},
			},
			expectedHeader: "Authorization",
			expectedValue:  "Basic dXNlcjpwYXNz",
		},
		{
			name: "custom headers",
			authConfig: &config.AuthConfig{
				Method: "custom",
				Headers: map[string]string{
					"X-Custom-Auth": "custom-value",
				},
			},
			expectedHeader: "X-Custom-Auth",
			expectedValue:  "custom-value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.VaultConfig{
				ID:       "test",
				Endpoint: "https://example.com",
				Auth:     tt.authConfig,
			}

			client := NewClient(cfg)
			req, _ := http.NewRequest("GET", "https://example.com", nil)
			client.addAuthHeaders(req)

			gotValue := req.Header.Get(tt.expectedHeader)
			if gotValue != tt.expectedValue {
				t.Errorf("expected header %s=%s, got %s", tt.expectedHeader, tt.expectedValue, gotValue)
			}
		})
	}
}

func TestAddCustomHeaders(t *testing.T) {
	cfg := &config.VaultConfig{
		ID:       "test",
		Endpoint: "https://example.com",
		Headers: map[string]string{
			"X-Custom-1": "value1",
			"X-Custom-2": "value2",
		},
		Auth: &config.AuthConfig{
			Method: "bearer",
		},
	}

	client := NewClient(cfg)
	req, _ := http.NewRequest("GET", "https://example.com", nil)
	client.addCustomHeaders(req)

	if req.Header.Get("X-Custom-1") != "value1" {
		t.Errorf("expected X-Custom-1 header to be value1, got %s", req.Header.Get("X-Custom-1"))
	}
	if req.Header.Get("X-Custom-2") != "value2" {
		t.Errorf("expected X-Custom-2 header to be value2, got %s", req.Header.Get("X-Custom-2"))
	}
}

func TestClientOAuth(t *testing.T) {
	// Create a mock OAuth token server
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/oauth/token" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"access_token": "test-oauth-token", "expires_in": 3600}`))
	}))
	defer tokenServer.Close()

	// Create a mock API server
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify OAuth token is in Authorization header
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-oauth-token" {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error": "unauthorized"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data": [{"name": "secret1", "value": "value1"}]}`))
	}))
	defer apiServer.Close()

	cfg := &config.VaultConfig{
		ID:       "test",
		Endpoint: apiServer.URL,
		Type:     "generic",
		Auth: &config.AuthConfig{
			Method: "oauth2",
			OAuth: &config.OAuthConfig{
				TokenEndpoint: tokenServer.URL + "/oauth/token",
				ClientID:      "test-client",
				ClientSecret:  "test-secret",
				Scope:         "read write",
			},
		},
		FieldNames: config.FieldNamesConfig{
			NameField:  "name",
			ValueField: "value",
		},
	}

	client := NewClient(cfg)
	
	// Test that OAuth token is fetched and used
	secrets, err := client.ListSecrets()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(secrets) != 1 {
		t.Errorf("expected 1 secret, got %d", len(secrets))
	}
}

func TestClientAPIKeyAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.Header.Get("X-API-Key")
		if apiKey != "test-api-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data": []}`))
	}))
	defer server.Close()

	cfg := &config.VaultConfig{
		ID:       "test",
		Endpoint: server.URL,
		Type:     "generic",
		Auth: &config.AuthConfig{
			Method: "api_key",
			Headers: map[string]string{
				"api_key": "test-api-key",
			},
		},
		FieldNames: config.FieldNamesConfig{
			NameField:  "name",
			ValueField: "value",
		},
	}

	client := NewClient(cfg)
	err := client.TestConnection()

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSetSecretWithComplexValue(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var data map[string]interface{}
		json.Unmarshal(body, &data)

		// Check if value field contains a parsed JSON object
		value := data["value"]
		if valueMap, ok := value.(map[string]interface{}); ok {
			if valueMap["username"] != "user" || valueMap["password"] != "pass" {
				t.Error("expected value to be parsed JSON object")
			}
		} else {
			t.Error("expected value to be a map")
		}

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

	client := NewClient(cfg)
	jsonValue := `{"username": "user", "password": "pass"}`
	err := client.SetSecret("my-secret", jsonValue)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValueToString(t *testing.T) {
	parser := &JsonPathParser{}

	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{
			name:     "string value",
			input:    "simple",
			expected: "simple",
		},
		{
			name:     "number value",
			input:    float64(42),
			expected: "42",
		},
		{
			name:     "boolean value",
			input:    true,
			expected: "true",
		},
		{
			name:     "map value preserves JSON",
			input:    map[string]interface{}{"key": "value"},
			expected: "{\"key\":\"value\"}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.valueToString(tt.input)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestGetDefaultTokenEndpoint(t *testing.T) {
	tests := []struct {
		name         string
		endpoint     string
		vaultType    string
		wantEndpoint string
	}{
		{
			name:         "vaultwarden type",
			endpoint:     "https://vault.example.com",
			vaultType:    "vaultwarden",
			wantEndpoint: "https://vault.example.com/identity/connect/token",
		},
		{
			name:         "generic type",
			endpoint:     "https://api.example.com",
			vaultType:    "generic",
			wantEndpoint: "https://api.example.com/oauth/token",
		},
		{
			name:         "endpoint with trailing slash",
			endpoint:     "https://vault.example.com/",
			vaultType:    "vaultwarden",
			wantEndpoint: "https://vault.example.com/identity/connect/token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.VaultConfig{
				ID:       "test",
				Endpoint: tt.endpoint,
				Type:     tt.vaultType,
				Auth: &config.AuthConfig{
					Method: "oauth2",
					OAuth:  &config.OAuthConfig{},
				},
			}

			client := NewClient(cfg)
			result := client.getDefaultTokenEndpoint()

			if result != tt.wantEndpoint {
				t.Errorf("expected %s, got %s", tt.wantEndpoint, result)
			}
		})
	}
}

func TestDeleteSecretError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error": "not found"}`))
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

	client := NewClient(cfg)
	err := client.DeleteSecret("nonexistent")

	if err == nil {
		t.Error("expected error for 404 response")
	}
}

func TestGetSecretComplexValue(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data": [{"name": "complex", "value": {"username": "user", "password": "pass"}}]}`))
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

	client := NewClient(cfg)
	secret, err := client.GetSecret("complex")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should be JSON serialized
	if !strings.Contains(secret.Value, "username") || !strings.Contains(secret.Value, "password") {
		t.Errorf("expected JSON value, got %s", secret.Value)
	}
}

func TestNewClientWithParserOverride(t *testing.T) {
	cfg := &config.VaultConfig{
		ID:       "test",
		Endpoint: "https://example.com",
		Type:     "generic",
		Auth: &config.AuthConfig{
			Method: "bearer",
		},
		OperationsOverride: map[string]*config.OperationConfig{
			"list": {
				ResponseParser: &config.ResponseParserConfig{
					ListPath:  "custom.path",
					NameField: "customName",
					ValuePath: "custom.value",
				},
			},
		},
	}

	client := NewClient(cfg)
	if client.parser == nil {
		t.Error("expected parser to be initialized")
	}

	// Verify parser is configured from override
	jsonParser, ok := client.parser.(*JsonPathParser)
	if !ok {
		t.Fatal("expected JsonPathParser type")
	}

	if jsonParser.ListPath != "custom.path" {
		t.Errorf("expected custom list path, got %s", jsonParser.ListPath)
	}
}

func TestListSecretsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "internal error"}`))
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

	client := NewClient(cfg)
	_, err := client.ListSecrets()

	if err == nil {
		t.Error("expected error for 500 response")
	}
}

func TestSetSecretErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": "bad request"}`))
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

	client := NewClient(cfg)
	err := client.SetSecret("test", "value")

	if err == nil {
		t.Error("expected error for 400 response")
	}
}

func TestSetSecretWithVaultwardenType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var data map[string]interface{}
		json.Unmarshal(body, &data)

		// Check if type field is set for Vaultwarden
		if typeVal, ok := data["type"].(float64); !ok || typeVal != 1 {
			t.Error("expected type field with value 1 for login field")
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok"}`))
	}))
	defer server.Close()

	cfg := &config.VaultConfig{
		ID:       "test",
		Endpoint: server.URL,
		Type:     "vaultwarden",
		Method:   "POST",
		Auth: &config.AuthConfig{
			Method: "bearer",
			Headers: map[string]string{
				"token": "test-token",
			},
		},
		FieldNames: config.FieldNamesConfig{
			NameField:  "name",
			ValueField: "login",
		},
	}

	client := NewClient(cfg)
	err := client.SetSecret("test","value")

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGetSecretNotInList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data": [{"name": "other", "value": "val"}]}`))
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

	client := NewClient(cfg)
	_, err := client.GetSecret("nonexistent")

	if err == nil {
		t.Error("expected error when secret not in list")
	}
}

func TestOAuthCachedToken(t *testing.T) {
	callCount := 0
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"access_token": "cached-token", "expires_in": 3600}`))
	}))
	defer tokenServer.Close()

	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data": []}`))
	}))
	defer apiServer.Close()

	cfg := &config.VaultConfig{
		ID:       "test",
		Endpoint: apiServer.URL,
		Type:     "generic",
		Auth: &config.AuthConfig{
			Method: "oauth2",
			OAuth: &config.OAuthConfig{
				TokenEndpoint: tokenServer.URL + "/token",
				ClientID:      "test-client",
				ClientSecret:  "test-secret",
			},
		},
		FieldNames: config.FieldNamesConfig{
			NameField:  "name",
			ValueField: "value",
		},
	}

	client := NewClient(cfg)
	
	// First call should fetch token
	client.ListSecrets()
	firstCallCount := callCount

	// Second call should use cached token
	client.ListSecrets()
	secondCallCount := callCount

	// Token endpoint should only be called once
	if firstCallCount != secondCallCount {
		t.Error("expected OAuth token to be cached")
	}
}

func TestOAuthWithExtraParams(t *testing.T) {
	var receivedBody string
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		receivedBody = string(body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"access_token": "test-token", "expires_in": 3600}`))
	}))
	defer tokenServer.Close()

	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data": []}`))
	}))
	defer apiServer.Close()

	cfg := &config.VaultConfig{
		ID:       "test",
		Endpoint: apiServer.URL,
		Type:     "generic",
		Auth: &config.AuthConfig{
			Method: "oauth2",
			OAuth: &config.OAuthConfig{
				TokenEndpoint: tokenServer.URL,
				ClientID:      "test-client",
				ClientSecret:  "test-secret",
				Scope:         "read write",
				ExtraParams: map[string]string{
					"device_id": "device-123",
				},
			},
		},
		FieldNames: config.FieldNamesConfig{
			NameField:  "name",
			ValueField: "value",
		},
	}

	client := NewClient(cfg)
	client.ListSecrets()

	// Check if extra params were included
	if !strings.Contains(receivedBody, "device_id") {
		t.Error("expected device_id in OAuth request body")
	}
}

func TestOAuthError(t *testing.T) {
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": "invalid_client"}`))
	}))
	defer tokenServer.Close()

	cfg := &config.VaultConfig{
		ID:       "test",
		Endpoint: "https://vault.example.com",
		Type:     "generic",
		Auth: &config.AuthConfig{
			Method: "oauth2",
			OAuth: &config.OAuthConfig{
				TokenEndpoint: tokenServer.URL,
				ClientID:      "test-client",
				ClientSecret:  "test-secret",
			},
		},
		FieldNames: config.FieldNamesConfig{
			NameField:  "name",
			ValueField: "value",
		},
	}

	client := NewClient(cfg)
	_, err := client.ListSecrets()

	if err == nil {
		t.Error("expected error for OAuth failure")
	}
}
