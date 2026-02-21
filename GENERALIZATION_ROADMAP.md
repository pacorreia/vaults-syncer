# Generalization Implementation Guide

## Current Architecture (Vaultwarden-Centric)

```
┌─────────────────────────────────────────────────┐
│           Sync Daemon                           │
├─────────────────────────────────────────────────┤
│                                                 │
│  vault/client.go                                │
│  ├─ Assumes response.data[] = list              │
│  ├─ Hardcodes /api/ciphers → /identity/token   │
│  ├─ Assumes type=1 for logins                  │
│  └─ Device ID in OAuth params                  │
│                                                 │
│  sync/engine.go                                 │
│  └─ Generic (✓ works with any HTTP API)        │
│                                                 │
│  config/types.go                                │
│  └─ No vault type metadata                     │
│                                                 │
└──────────────┬──────────────────────────────────┘
               │
        ┌──────▼──────┐
        │ HTTP client │
        └──────┬──────┘
               │
    ┌──────────┴──────────┐
    ▼                     ▼
┌─────────┐          ┌──────────┐
│Vaultwarden    │AWS/Azure/Vault │ (Not well supported)
│(works!) ✓      │(hardcoded!)   │
└─────────┘          └──────────┘
```

---

## Target Architecture (Vault-Agnostic)

```
┌─────────────────────────────────────────────────────┐
│           Sync Daemon                               │
├─────────────────────────────────────────────────────┤
│                                                     │
│  vault/backend.go (Interface)                       │
│  ├─ Backend interface (List, Get, Set, Delete)     │
│  └─ BackendCapabilities metadata                   │
│                                                     │
│  vault/implementations/                             │
│  ├─ vaultwarden.go (Vaultwarden-specific logic)    │
│  ├─ hashicorp.go (Vault-specific logic)            │
│  ├─ azure.go (Azure Key Vault logic)               │
│  ├─ aws.go (AWS Secrets Manager logic)             │
│  └─ generic.go (Config-driven HTTP for others)     │
│                                                     │
│  vault/client.go (Factory)                          │
│  └─ NewClient(type) → Backend                      │
│                                                     │
│  vault/parser.go (Response Parsing)                 │
│  ├─ JsonPathParser (config-based)                  │
│  └─ VaultTypeParser (type-specific defaults)       │
│                                                     │
│  sync/engine.go                                     │
│  └─ Generic (works with any Backend)               │
│                                                     │
└──────────────┬───────────────────────────────────────┘
               │
        ┌──────▼──────────────────┐
        │  Backend interface      │
        │  ├─ List() []string    │
        │  ├─ Get(name) *Secret  │
        │  ├─ Set(name, value)   │
        │  └─ Delete(name)       │
        └──────┬──────────────────┘
               │
    ┌──────────┼──────────┬────────────┬────────────┐
    ▼          ▼          ▼            ▼            ▼
┌──────┐  ┌────────┐ ┌──────┐   ┌─────────┐  ┌───────┐
│VW    │  │Vault   │ │Azure │   │AWS/etc  │  │Generic│
│✓✓✓   │  │✓✓✓    │ │✓✓✓   │   │✓✓✓     │  │✓✓✓   │
└──────┘  └────────┘ └──────┘   └─────────┘  └───────┘
(Works!)  (Works!)   (Works!)   (Works!)     (Works!)
```

---

## Step-by-Step Refactoring

### Step 1: Make OAuth Token Endpoint Configurable (5 mins)

**File**: `config/types.go`

```go
// BEFORE
type VaultConfig struct {
    ID               string
    Endpoint         string
    AuthMethod       string
    AuthHeaders      map[string]string
}

// AFTER
type AuthConfig struct {
    Method       string            `yaml:"method"`  // bearer, basic, oauth2, etc.
    Headers      map[string]string `yaml:"headers"`
    OAuth        *OAuthConfig      `yaml:"oauth"`   // NEW
}

type OAuthConfig struct {
    TokenEndpoint string            `yaml:"token_endpoint"`  // NEW: Configurable!
    ClientID      string            `yaml:"client_id"`
    ClientSecret  string            `yaml:"client_secret"`
    Scope         string            `yaml:"scope"`
    ExtraParams   map[string]string `yaml:"extra_params"` // NEW: Device ID, etc.
}

type VaultConfig struct {
    ID             string              `yaml:"id"`
    Name           string              `yaml:"name"`
    Type           string              `yaml:"type"`           // NEW: vaultwarden, vault, azure
    Endpoint       string              `yaml:"endpoint"`
    AuthConfig     AuthConfig          `yaml:"auth"`           // NEW: Replaces AuthMethod/AuthHeaders
    FieldNames     FieldNamesConfig    `yaml:"field_names"`
    Headers        map[string]string   `yaml:"headers"`
    Timeout        int                 `yaml:"timeout"`
    SkipSSLVerify  bool                `yaml:"skip_ssl_verify"`
}
```

**Example Config**:
```yaml
vaults:
  - id: prod_vaultwarden
    type: vaultwarden              # NEW
    endpoint: https://vault.example.com/api/ciphers
    auth:                          # NEW structure
      method: oauth2
      oauth:
        token_endpoint: https://vault.example.com/identity/connect/token  # Configurable!
        client_id: "..."
        client_secret: "..."
        scope: "api"
        extra_params:              # For device identification
          device_identifier: sync-daemon
          device_name: sync-daemon
          device_type: "14"
```

---

### Step 2: Refactor OAuth Token Exchange (10 mins)

**File**: `vault/client.go`

```go
// BEFORE
func (c *Client) getOAuthToken() (string, error) {
    // Line 346-350: HARDCODED!
    baseURL := strings.TrimSuffix(c.cfg.Endpoint, "/api/ciphers")
    tokenURL := fmt.Sprintf("%s/identity/connect/token", baseURL)
    
    data := fmt.Sprintf(
        "grant_type=client_credentials&client_id=%s&client_secret=%s&scope=%s&device_identifier=sync-daemon&device_name=sync-daemon&device_type=14",
        clientID, clientSecret, scope,
    )
}

// AFTER
func (c *Client) getOAuthToken() (string, error) {
    // Get token endpoint from config
    tokenURL := c.cfg.AuthConfig.OAuth.TokenEndpoint
    if tokenURL == "" {
        // Smart default based on vault type
        tokenURL = c.getDefaultTokenEndpoint()
    }
    
    // Build request with configurable params
    params := map[string]string{
        "grant_type":   "client_credentials",
        "client_id":    c.cfg.AuthConfig.OAuth.ClientID,
        "client_secret": c.cfg.AuthConfig.OAuth.ClientSecret,
        "scope":        c.cfg.AuthConfig.OAuth.Scope,
    }
    
    // Add extra params from config (device ID, etc.)
    for k, v := range c.cfg.AuthConfig.OAuth.ExtraParams {
        params[k] = v
    }
    
    // Encode params
    data := encodeParams(params)
    
    // ... rest of implementation
}

// Helper function
func (c *Client) getDefaultTokenEndpoint() string {
    switch strings.ToLower(c.cfg.Type) {
    case "vaultwarden":
        baseURL := strings.TrimSuffix(c.cfg.Endpoint, "/api/ciphers")
        return fmt.Sprintf("%s/identity/connect/token", baseURL)
    case "vault":
        return fmt.Sprintf("%s/v1/auth/oauth/token", c.cfg.Endpoint)
    case "azure":
        return "https://login.microsoftonline.com/tenant/oauth2/v2.0/token"
    default:
        return c.cfg.Endpoint  // Assume endpoint is token endpoint
    }
}
```

---

### Step 3: Add Response Parser Interface (15 mins)

**File**: `vault/parser.go` (NEW)

```go
package vault

import (
    "encoding/json"
)

// ResponseParser handles parsing different vault response formats
type ResponseParser interface {
    ParseList([]byte) ([]string, error)
    ParseGet([]byte) (*Secret, error)
    ParseSet([]byte) error
}

// JsonPathParser extracts fields using JSONPath-like notation
type JsonPathParser struct {
    ListPath   string // e.g., "data"
    NameField  string // e.g., "name"
    ValueField string // e.g., "value"
}

func (p *JsonPathParser) ParseList(body []byte) ([]string, error) {
    var data map[string]interface{}
    if err := json.Unmarshal(body, &data); err != nil {
        return nil, err
    }
    
    // Navigate to ListPath (e.g., "data")
    var items []interface{}
    if p.ListPath != "" {
        if path, ok := data[p.ListPath].([]interface{}); ok {
            items = path
        }
    } else {
        // Try common defaults
        if path, ok := data["data"].([]interface{}); ok {
            items = path
        } else if keys, ok := data["keys"].([]interface{}); ok {
            items = keys
        }
    }
    
    var names []string
    for _, item := range items {
        if itemMap, ok := item.(map[string]interface{}); ok {
            if name, ok := itemMap[p.NameField].(string); ok {
                names = append(names, name)
            }
        }
    }
    
    return names, nil
}

// GetParserForVaultType returns appropriate parser based on vault type
func GetParserForVaultType(vaultType string) ResponseParser {
    switch vaultType {
    case "vaultwarden":
        return &JsonPathParser{
            ListPath:   "data",
            NameField:  "name",
            ValueField: "name",
        }
    case "vault":
        return &JsonPathParser{
            ListPath:   "data.keys",
            NameField:  "",
            ValueField: "value",
        }
    case "azure":
        return &JsonPathParser{
            ListPath:   "value",
            NameField:  "name",
            ValueField: "value",
        }
    default:
        // Generic parser with smart defaults
        return &JsonPathParser{
            ListPath:   "data",
            NameField:  "name",
            ValueField: "value",
        }
    }
}
```

---

### Step 4: Integrate Parser into Client (10 mins)

**File**: `vault/client.go`

```go
// Update Client struct
type Client struct {
    cfg          *config.VaultConfig
    client       *http.Client
    parser       ResponseParser  // NEW
    oauthToken   string
    oauthExpires time.Time
}

// Update NewClient
func NewClient(cfg *config.VaultConfig) *Client {
    // ... TLS setup ...
    
    parser := GetParserForVaultType(cfg.Type)
    
    return &Client{
        cfg:    cfg,
        client: client,
        parser: parser,  // NEW
    }
}

// Update ListSecrets to use parser
func (c *Client) ListSecrets() ([]string, error) {
    url := strings.TrimSuffix(c.cfg.Endpoint, "/")
    
    req, err := http.NewRequest("GET", url, nil)
    // ... request setup ...
    
    resp, err := c.client.Do(req)
    // ... error handling ...
    
    body, err := io.ReadAll(resp.Body)
    
    // Use parser instead of hardcoded logic
    return c.parser.ParseList(body)
}
```

---

### Step 5: Create Backend Interface (20 mins)

**File**: `vault/backend.go` (NEW)

```go
package vault

import (
    "context"
    "fmt"
    
    "github.com/pacorreia/vaults-syncer/config"
)

// Backend defines the interface all vault implementations must satisfy
type Backend interface {
    // Core operations
    ListSecrets(ctx context.Context) ([]string, error)
    GetSecret(ctx context.Context, name string) (*Secret, error)
    SetSecret(ctx context.Context, name, value string) error
    DeleteSecret(ctx context.Context, name string) error
    
    // Connection tests
    TestConnection() error
    
    // Metadata
    Type() string
    Capabilities() BackendCapabilities
}

// BackendCapabilities describes what a backend can do
type BackendCapabilities struct {
    SupportsDirectGet bool  // Can fetch individual secrets
    SupportsDelete    bool  // Can delete secrets
    SupportsList      bool  // Can list all secrets
    RequiresAuth      bool  // Needs authentication
    OAuth2Support     bool  // Supports OAuth 2.0
}

// VaultBackend wraps Client and implements Backend interface
type VaultBackend struct {
    client *Client
}

func (vb *VaultBackend) Type() string {
    return vb.client.cfg.Type
}

func (vb *VaultBackend) Capabilities() BackendCapabilities {
    caps := BackendCapabilities{
        SupportsList:  true,
        RequiresAuth:  true,
    }
    
    // Type-specific capabilities
    switch vb.client.cfg.Type {
    case "vaultwarden":
        caps.SupportsDirectGet = false  // Must list then filter
        caps.SupportsDelete = true
        caps.OAuth2Support = true
    case "vault":
        caps.SupportsDirectGet = true
        caps.SupportsDelete = true
        caps.OAuth2Support = true
    case "azure":
        caps.SupportsDirectGet = true
        caps.SupportsDelete = true
        caps.OAuth2Support = true
    }
    
    return caps
}

// NewBackend creates appropriate backend for vault type
func NewBackend(cfg *config.VaultConfig) (Backend, error) {
    if cfg.Type == "" {
        cfg.Type = "generic"  // Default
    }
    
    client := NewClient(cfg)
    
    // Add vault-specific logic here in the future
    // For now, generic client implements Backend
    return &VaultBackend{client: client}, nil
}
```

---

### Step 6: Update Sync Engine to Use Backend Interface (10 mins)

**File**: `sync/engine.go`

```go
// BEFORE
type Engine struct {
    cfg    *config.Config
    store  *storage.Store
    clients map[string]*vault.Client    // Concrete type
    logger *slog.Logger
}

// AFTER
type Engine struct {
    cfg    *config.Config
    store  *storage.Store
    backends map[string]vault.Backend   // Interface type
    logger *slog.Logger
}

// Update NewEngine
func NewEngine(cfg *config.Config, store *storage.Store, logger *slog.Logger) (*Engine, error) {
    engine := &Engine{
        cfg:      cfg,
        store:    store,
        backends: make(map[string]vault.Backend),
        logger:   logger,
    }
    
    for _, vaultCfg := range cfg.Vaults {
        backend, err := vault.NewBackend(&vaultCfg)
        if err != nil {
            engine.logger.Warn("failed to create backend",
                slog.String("vault_id", vaultCfg.ID),
                slog.String("error", err.Error()),
            )
            continue
        }
        
        if err := backend.TestConnection(); err != nil {
            engine.logger.Warn("failed to connect to backend",
                slog.String("vault_id", vaultCfg.ID),
                slog.String("error", err.Error()),
            )
        }
        
        caps := backend.Capabilities()
        engine.logger.Info("backend initialized",
            slog.String("vault_id", vaultCfg.ID),
            slog.String("type", backend.Type()),
            slog.Bool("supports_direct_get", caps.SupportsDirectGet),
        )
        
        engine.backends[vaultCfg.ID] = backend
    }
    
    return engine, nil
}

// Update ExecuteSync
func (e *Engine) ExecuteSync(syncCfg *config.SyncConfig) error {
    // ... existing code ...
    
    sourceBackend, ok := e.backends[syncCfg.Source]
    if !ok {
        return fmt.Errorf("source backend not found: %s", syncCfg.Source)
    }
    
    secrets, err := sourceBackend.ListSecrets(context.Background())
    // ... rest is the same ...
}
```

---

## Configuration Migration Examples

### Vaultwarden (OAuth)

```yaml
# OLD (still works)
vaults:
  - id: vw1
    endpoint: https://vault.example.com/api/ciphers
    auth_method: oauth2
    auth_headers:
      client_id: "..."
      client_secret: "..."

# NEW (recommended)
vaults:
  - id: vw1
    type: vaultwarden
    endpoint: https://vault.example.com/api/ciphers
    auth:
      method: oauth2
      oauth:
        token_endpoint: https://vault.example.com/identity/connect/token
        client_id: "..."
        client_secret: "..."
        scope: "api"
        extra_params:
          device_identifier: sync-daemon
```

### HashiCorp Vault

```yaml
vaults:
  - id: vault1
    type: vault
    endpoint: https://vault.example.com/v1
    auth:
      method: oauth2
      oauth:
        token_endpoint: https://vault.example.com/v1/auth/oauth/token
        client_id: "..."
        client_secret: "..."
        scope: "default"
```

### Azure Key Vault

```yaml
vaults:
  - id: azure1
    type: azure
    endpoint: https://myvault.vault.azure.net
    auth:
      method: oauth2
      oauth:
        token_endpoint: https://login.microsoftonline.com/{tenant}/oauth2/v2.0/token
        client_id: "..."
        client_secret: "..."
```

### Custom HTTP API

```yaml
vaults:
  - id: custom1
    type: generic
    endpoint: https://custom-api.example.com/secrets
    auth:
      method: bearer
      headers:
        Authorization: Bearer ${CUSTOM_TOKEN}
```

---

## Summary of Changes

| Component | Current | After | LOC Changed |
|-----------|---------|-------|------------|
| config/types.go | AuthMethod string | AuthConfig struct | +30 |
| vault/client.go | OAuth hardcoded | Uses config | -20 |
| vault/parser.go | N/A | NEW interface | +100 |
| vault/backend.go | N/A | NEW interface | +80 |
| sync/engine.go | Uses Client | Uses Backend | -10 |
| Total | - | - | +180 |

**Backward Compatibility**: ✅ 100% (all existing configs still work)

---

## Testing Strategy

1. **Unit Tests**: Parser with different response formats
2. **Integration Tests**: Each vault type with mock server
3. **Regression Tests**: Existing Vaultwarden configs still work
4. **New Vault Tests**: HashiCorp, Azure examples with real/mocked endpoints

