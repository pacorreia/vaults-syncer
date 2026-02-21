# Before/After Code Comparison Examples

## Example 1: OAuth Token Exchange

### ❌ CURRENT (Vaultwarden-Only)

```go
// vault/client.go Lines 346-350
func (c *Client) getOAuthToken() (string, error) {
    clientID, ok := c.cfg.AuthHeaders["client_id"]
    clientSecret, ok := c.cfg.AuthHeaders["client_secret"]
    scope, ok := c.cfg.AuthHeaders["scope"]
    
    // 🔴 HARDCODED: Assumes Vaultwarden endpoint structure
    baseURL := strings.TrimSuffix(c.cfg.Endpoint, "/api/ciphers")
    tokenURL := fmt.Sprintf("%s/identity/connect/token", baseURL)
    
    // 🔴 HARDCODED: Vaultwarden-specific parameters
    data := fmt.Sprintf(
        "grant_type=client_credentials&client_id=%s&client_secret=%s&scope=%s&device_identifier=sync-daemon&device_name=sync-daemon&device_type=14",
        clientID, clientSecret, scope,
    )
    
    // ... request ...
}

// ⚠️ PROBLEM: Only works for Vaultwarden
// Vault would fail: "resource not found" (wrong token endpoint)
// Azure would fail: "client_id not recognized" (different OAuth provider)
// AWS would fail: "invalid_request" (doesn't use client credentials)
```

### ✅ AFTER (Generic)

```go
// vault/client.go (refactored)
func (c *Client) getOAuthToken() (ctx context.Context) (string, error) {
    oauthCfg := c.cfg.AuthConfig.OAuth
    
    // 1. Get token endpoint from config or use smart default
    tokenURL := oauthCfg.TokenEndpoint
    if tokenURL == "" {
        tokenURL = c.getDefaultTokenEndpoint()
    }
    
    // 2. Build standard OAuth params
    params := map[string]string{
        "grant_type":    "client_credentials",
        "client_id":     oauthCfg.ClientID,
        "client_secret": oauthCfg.ClientSecret,
        "scope":         oauthCfg.Scope,
    }
    
    // 3. Add vault-specific extra params from config
    for k, v := range oauthCfg.ExtraParams {
        params[k] = v
    }
    
    data := encodeParams(params)
    
    // ... rest of request ...
}

// Helper with vault-specific defaults
func (c *Client) getDefaultTokenEndpoint() string {
    switch strings.ToLower(c.cfg.Type) {
    case "vaultwarden":
        baseURL := strings.TrimSuffix(c.cfg.Endpoint, "/api/ciphers")
        return fmt.Sprintf("%s/identity/connect/token", baseURL)
    case "vault":
        return fmt.Sprintf("%s/v1/auth/oauth/token", c.cfg.Endpoint)
    case "azure":
        return "https://login.microsoftonline.com/{tenant}/oauth2/v2.0/token"
    case "aws":
        return "https://sts.amazonaws.com"
    default:
        return c.cfg.Endpoint + "/oauth/token"  // Common pattern
    }
}

// ✅ WORKS WITH:
// - Vaultwarden: Uses smart default based on type
// - Vault: Uses Vault OAuth endpoint
// - Azure: Uses Azure AD endpoint
// - AWS: Can be configured in config.yaml
// - Custom: Anyone can specify their own endpoint
```

### Configuration Comparison

```yaml
# ❌ OLD (Vaultwarden-only with hardcoded assumptions)
vaults:
  - id: prod
    endpoint: https://vault.example.com/api/ciphers
    auth_method: oauth2
    auth_headers:
      client_id: "..."
      client_secret: "..."
      scope: "api"
  # No way to use with other vaults!

# ✅ NEW (Generic with explicit config)

# Vaultwarden
vaults:
  - id: vaultwarden_prod
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
          device_name: sync-daemon
          device_type: "14"

# HashiCorp Vault (just change type and endpoint!)
vaults:
  - id: vault_prod
    type: vault
    endpoint: https://vault.example.com/v1
    auth:
      method: oauth2
      oauth:
        # token_endpoint auto-defaults based on type
        client_id: "..."
        client_secret: "..."

# Azure Key Vault (all different!)
vaults:
  - id: azure_prod
    type: azure
    endpoint: https://myvault.vault.azure.net
    auth:
      method: oauth2
      oauth:
        token_endpoint: https://login.microsoftonline.com/{tenant}/oauth2/v2.0/token
        client_id: "..."
        client_secret: "..."
```

---

## Example 2: Response Parsing

### ❌ CURRENT (Hardcoded Paths)

```go
// vault/client.go Lines 186-206
func (c *Client) ListSecrets() ([]string, error) {
    // ... fetch response ...
    body, err := io.ReadAll(resp.Body)
    
    var data map[string]interface{}
    json.Unmarshal(body, &data)
    
    var names []string
    
    // 🔴 HARDCODED: Assumes "data" field (Vaultwarden)
    if items, ok := data["data"].([]interface{}); ok {
        for _, item := range items {
            if itemMap, ok := item.(map[string]interface{}); ok {
                nameField := c.cfg.FieldNames.NameField
                if nameField == "" {
                    nameField = "name"  // Vaultwarden default
                }
                if name, ok := itemMap[nameField].(string); ok {
                    names = append(names, name)
                }
            }
        }
    } else if keys, ok := data["keys"].([]interface{}); ok {
        // 🟡 FALLBACK: Try Vault format
        for _, key := range keys {
            names = append(names, fmt.Sprintf("%v", key))
        }
    }
    
    return names, nil
}

// ⚠️ EXAMPLES OF FAILURES:
// Azure response: {"value": [...]}
//   → No "data" field → Can't find "keys" → Returns []
// 
// AWS response: {"SecretList": [{...}]}
//   → No "data" field → No "keys" field → Returns []
//
// Pagination: {"items": [...], "cursor": "..."}
//   → No "data" field → No "keys" field → Returns []
```

### ✅ AFTER (Config-Driven)

```go
// vault/parser.go (NEW)
type ResponseParser interface {
    ParseList(body []byte) ([]string, error)
}

type JsonPathParser struct {
    ListPath  string // e.g., "data" or "value" or "SecretList"
    NameField string // e.g., "name" or "id" or "Name"
}

func (p *JsonPathParser) ParseList(body []byte) ([]string, error) {
    var data map[string]interface{}
    json.Unmarshal(body, &data)
    
    // Navigate to path
    var items []interface{}
    if p.ListPath != "" {
        if val, ok := data[p.ListPath].([]interface{}); ok {
            items = val
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

// Vault-specific parsers with smart defaults
func GetParserForVaultType(vaultType, configPath, nameField string) ResponseParser {
    // Use config if provided, otherwise smart defaults
    if configPath == "" {
        switch vaultType {
        case "vaultwarden":
            configPath = "data"
        case "vault":
            configPath = "data.keys"
        case "azure":
            configPath = "value"
        case "aws":
            configPath = "SecretList"
        default:
            configPath = "data"  // Try this first
        }
    }
    
    if nameField == "" {
        switch vaultType {
        case "aws":
            nameField = "Name"
        default:
            nameField = "name"
        }
    }
    
    return &JsonPathParser{
        ListPath:  configPath,
        NameField: nameField,
    }
}

// ✅ NOW SUPPORTS:
// - Vaultwarden: listPath="data", nameField="name"
// - Vault: listPath="data.keys", nameField="key"
// - Azure: listPath="value", nameField="name"
// - AWS: listPath="SecretList", nameField="Name"
// - Custom: Anything configured in config.yaml
```

### Configuration Comparison

```yaml
# ❌ OLD (Hardcoded, no options)
vaults:
  - id: prod
    endpoint: https://vault.example.com/api/ciphers
    field_names:
      name_field: "name"
      value_field: "name"
  # Can't change response parsing format!

# ✅ NEW (Flexible, config-driven)

# Vaultwarden (default smart behavior)
vaults:
  - id: vw
    type: vaultwarden
    endpoint: https://vault.example.com/api/ciphers
    # Auto-detects: listPath="data", nameField="name"

# Azure Key Vault (different format)
vaults:
  - id: azure
    type: azure
    endpoint: https://myvault.vault.azure.net
    operations_override:
      list:
        response:
          path: "value"         # Different path!
          name_field: "name"

# AWS Secrets Manager (even more different)
vaults:
  - id: aws
    type: aws
    endpoint: https://secretsmanager.amazonaws.com
    operations_override:
      list:
        response:
          path: "SecretList"    # Completely different!
          name_field: "Name"    # Capital N!

# Custom HTTP API (totally custom)
vaults:
  - id: custom
    type: generic
    endpoint: https://custom-api.example.com/secrets
    operations_override:
      list:
        response:
          path: "items"         # Whatever the API uses
          name_field: "id"      # Whatever makes sense
```

---

## Example 3: Backend Interface Pattern

### ❌ CURRENT (Direct Client Usage)

```go
// sync/engine.go
type Engine struct {
    cfg    *config.Config
    store  *storage.Store
    clients map[string]*vault.Client  // Concrete type
    logger *slog.Logger
}

func (e *Engine) ExecuteSync(syncCfg *config.SyncConfig) error {
    sourceClient, ok := e.clients[syncCfg.Source]
    if !ok {
        return fmt.Errorf("client not found")
    }
    
    secrets, err := sourceClient.ListSecrets()
    // ... rest of sync ...
}

// ⚠️ PROBLEMS:
// - Can't use vault-specific implementations
// - Must guess capabilities at runtime
// - No standard interface
// - Hard to test different vault types
// - Adding new vaults requires modifying sync logic
```

### ✅ AFTER (Backend Interface)

```go
// vault/backend.go (NEW)
type Backend interface {
    ListSecrets(ctx context.Context) ([]string, error)
    GetSecret(ctx context.Context, name string) (*Secret, error)
    SetSecret(ctx context.Context, name, value string) error
    DeleteSecret(ctx context.Context, name string) error
    TestConnection() error
    Type() string
    Capabilities() BackendCapabilities
}

type BackendCapabilities struct {
    SupportsDirectGet bool
    SupportsDelete    bool
    SupportsList      bool
}

// sync/engine.go (refactored)
type Engine struct {
    cfg     *config.Config
    store   *storage.Store
    backends map[string]Backend  // Interface type!
    logger  *slog.Logger
}

func (e *Engine) ExecuteSync(syncCfg *config.SyncConfig) error {
    sourceBackend, ok := e.backends[syncCfg.Source]
    if !ok {
        return fmt.Errorf("backend not found")
    }
    
    // Can now make intelligent decisions based on capabilities
    caps := sourceBackend.Capabilities()
    if !caps.SupportsList {
        return fmt.Errorf("vault doesn't support listing secrets")
    }
    
    secrets, err := sourceBackend.ListSecrets(context.Background())
    // ... rest of sync ...
}

// ✅ BENEFITS:
// - Easy to add new vault types (just implement Backend)
// - Sync engine is vault-agnostic
// - Can declare capabilities upfront
// - Easy to test (mock Backend)
// - Type-safe vault operations
```

---

## Example 4: Configuration Structure

### ❌ CURRENT

```yaml
vaults:
  - id: my_vault
    name: "My Vault"
    endpoint: "https://vault.example.com/api/ciphers"
    method: POST
    auth_method: oauth2  # But how do we know what OAuth scope?
    auth_headers:  # Mixed: OAuth and generic headers
      client_id: "xxx"
      client_secret: "xxx"
      scope: "api"
    field_names:
      name_field: "name"
      value_field: "name"
    headers:
      Accept: "application/json"
      Content-Type: "application/json"
    timeout: 30
    skip_ssl_verify: true

# ⚠️ PROBLEMS:
# 1. No "type" → Can't know what vault this is
# 2. OAuth fields mixed with other auth
# 3. No way to configure response parsing
# 4. No way to configure HTTP operations
# 5. No way to declare capabilities
```

### ✅ AFTER (Proposed)

```yaml
vaults:
  - id: my_vault
    type: vaultwarden          # NEW: Explicit type!
    name: "My Vault"
    endpoint: "https://vault.example.com/api/ciphers"
    
    auth:                      # NEW: Structured auth
      method: oauth2
      oauth:                   # NEW: OAuth-specific block
        token_endpoint: https://vault.example.com/identity/connect/token
        client_id: "xxx"
        client_secret: "xxx"
        scope: "api"
        extra_params:          # NEW: OAuth-specific params
          device_identifier: sync-daemon
    
    operations_override:       # NEW: Configure operations
      list:
        response:
          path: "data"         # JSONPath
          name_field: "name"
      get:
        endpoint: "{base}/{name}"
        method: GET
      set:
        endpoint: "{base}"
        method: POST
        status_codes: [200, 201]
      delete:
        endpoint: "{base}/{name}"
        method: DELETE
    
    field_names:               # LEGACY: Still supported (backward compat)
      name_field: "name"
      value_field: "name"
    
    headers:
      Accept: "application/json"
      Content-Type: "application/json"
    
    timeout: 30
    skip_ssl_verify: true      # Keep for dev/testing

syncs:
  - id: my_sync
    source: my_vault
    targets:
      - target_vault
    sync_type: unidirectional
    enabled: true
    concurrent_workers: 10
    schedule: "*/5 * * * *"
    filter:
      patterns: ["*"]
```

---

## Example 5: Adding a New Vault Type

### With Current Code (❌ Difficult)

```go
// To support Azure, you'd have to:

// 1. Modify vault/client.go
//    - Change response parsing for Azure format
//    - Handle Azure auth differently
//    - Change endpoint patterns
//    → Risk breaking Vaultwarden!

// 2. Hope Azure and Vaultwarden patterns don't conflict
// 3. Add conditional logic throughout

if c.cfg.Type == "azure" {
    // Azure-specific code
} else {
    // Vaultwarden code
}
// Repeat 20+ times...
```

### With Proposed Code (✅ Clean)

```go
// vault/implementations/azure.go (NEW FILE)
package implementations

import (
    "context"
    "github.com/pacorreia/vaults-syncer/vault"
)

type AzureBackend struct {
    client *http.Client
    cfg    *config.VaultConfig
}

func (a *AzureBackend) ListSecrets(ctx context.Context) ([]string, error) {
    // Azure-specific list implementation
}

func (a *AzureBackend) GetSecret(ctx context.Context, name string) (*vault.Secret, error) {
    // Azure-specific get implementation
}

func (a *AzureBackend) SetSecret(ctx context.Context, name, value string) error {
    // Azure-specific set implementation
}

func (a *AzureBackend) DeleteSecret(ctx context.Context, name string) error {
    // Azure-specific delete implementation
}

func (a *AzureBackend) Type() string {
    return "azure"
}

func (a *AzureBackend) Capabilities() vault.BackendCapabilities {
    return vault.BackendCapabilities{
        SupportsDirectGet: true,
        SupportsDelete:    true,
        SupportsList:      true,
    }
}

// Test file: vault/implementations/azure_test.go
func TestAzureListSecrets(t *testing.T) {
    // Azure-specific tests
    // No risk to Vaultwarden tests!
}

// That's it! No changes to vault/client.go needed!
```

---

## Summary of Improvements

| Aspect | Current Status | After Refactor |
|--------|---|---|
| **Vault Types Supported** | Vaultwarden (+ generic HTTP fallback) | Vaultwarden, Vault, Azure, AWS, Custom |
| **Code for New Vault Type** | Modify vault/client.go (risky) | Add new file (isolated) |
| **OAuth Config** | Hardcoded endpoint paths | Configurable endpoints + smart defaults |
| **Response Parsing** | Hardcoded JSON paths | Config-driven JSONPath extraction |
| **HTTP Methods** | Mostly hardcoded | Configurable per operation |
| **Capability Detection** | None (trial-and-error) | Explicit capabilities field |
| **Testing** | Risk of breaking others | Isolated per vault type |
| **Configuration** | Mixed/unclear | Structured and explicit |

---

## Migration Path

All changes are **100% backward compatible**:

1. **Add new config fields** (optional)
   - `type` field defaults to "vaultwarden" if missing
   - `operations_override` ignored if not present
   - New code checks config first, falls back to old behavior

2. **Deploy with new code**
   - Old configs work unchanged
   - New configs use enhanced features

3. **Gradually migrate configs**
   - Update one vault at a time
   - Test each migration
   - No downtime

4. **Remove legacy code** (v2.0)
   - Once all configs migrated
   - Clean up old hardcoded logic

