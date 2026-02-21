# Code Review: Generalization for Multi-Vault Support

## Executive Summary
The codebase has a good foundation for supporting multiple vault types, but there are **Vaultwarden-specific assumptions** embedded throughout. This review identifies areas that need abstraction for true vault-agnostic operation.

---

## 1. Vaultwarden-Specific Code Issues

### 1.1 **OAuth Token Endpoint (vault/client.go:346-350)** ⚠️ CRITICAL
```go
// Line 346-350
baseURL := strings.TrimSuffix(c.cfg.Endpoint, "/api/ciphers")
tokenURL := fmt.Sprintf("%s/identity/connect/token", baseURL)
```
**Problem**: 
- Hardcoded Vaultwarden path `/api/ciphers` → `/identity/connect/token` assumption
- Only works for Vaultwarden; other vaults (Vault, Azure, etc.) have completely different OAuth endpoints

**Solution**:
- Make OAuth token endpoint configurable in the config
- Add `oauth_token_endpoint` field to `VaultConfig`

### 1.2 **Secret Response Structure Parsing (vault/client.go:118-125)** ⚠️ MEDIUM
```go
// Line 118-125 in ListSecrets()
if items, ok := data["data"].([]interface{}); ok {
    // Vaultwarden specific
} else if keys, ok := data["keys"].([]interface{}); ok {
    // Vault-specific fallback
}
```
**Problem**:
- Assumes response has either `"data"` or `"keys"` top-level field
- doesn't support other formats (flat arrays, nested pagination, etc.)

**Solution**:
- Add configurable `response_parser` type
- Support multiple response formats (flat_array, nested_object, pagination)

### 1.3 **Type Field for Login Objects (vault/client.go:233-235)** ⚠️ MEDIUM
```go
// Line 233-235 in SetSecret()
if strings.ToLower(valueField) == "login" {
    payload["type"] = 1  // Vaultwarden-specific type=1 for login
}
```
**Problem**:
- Hardcoded Vaultwarden object type logic
- Other vaults don't use this pattern

**Solution**:
- Remove or make configurable via `optional_fields` config
- Allow defining "if field is X, add Y with value Z"

### 1.4 **Device Identification in OAuth (vault/client.go:330-333)** ⚠️ LOW
```go
data := fmt.Sprintf(
    "grant_type=client_credentials&client_id=%s&client_secret=%s&scope=%s&device_identifier=sync-daemon&device_name=sync-daemon&device_type=14",
    clientID, clientSecret, scope,
)
```
**Problem**:
- Vaultwarden-specific device fields (`device_identifier`, `device_name`, `device_type`)
- Not applicable to other OAuth providers

**Solution**:
- Make extra OAuth parameters configurable
- Use a `oauth_params` map in config

---

## 2. Generic Issues (Not Vaultwarden-Specific)

### 2.1 **Missing Vault Type Metadata** ⚠️ MEDIUM
```go
// config/types.go
type VaultConfig struct {
    ID               string
    Name             string
    Endpoint         string
    // Missing: VaultType (vaultwarden, vault, azure_keyvault, etc.)
}
```
**Problem**:
- No explicit vault type declaration
- Impossible to know which vault you're connecting to without trial-and-error

**Solution**:
```yaml
- id: my_vault
  type: vaultwarden        # New field
  endpoint: https://...
  auth_method: oauth2
```

### 2.2 **Hardcoded HTTP Methods** ⚠️ MEDIUM
```go
// CurrentlyVault operations always use GET for list, POST/PUT for set
// Different vaults use different methods
```
**Problem**:
- Azure Key Vault uses different HTTP patterns than Vaultwarden
- No flexibility for vault-specific HTTP methods

**Solution**:
```yaml
operations:
  list:
    method: GET              # or POST
    endpoint_suffix: ""      # or "/list"
  get:
    method: GET
    endpoint_suffix: "/{name}"
  set:
    method: PUT              # or POST
```

### 2.3 **JSON Response Parsing Fragility** ⚠️ HIGH
```go
// vault/client.go:146-156
// Hard-coded assumptions about response structure
if items, ok := data["data"].([]interface{}); ok {
    // Extract from items...
} 
// Falls back to ["keys"] but doesn't support other formats
```
**Problem**:
- JSON path assumptions fail for different vault types
- No JMESPath or JSONPath support

**Solution**:
```yaml
response_format:
  list_endpoint:
    path: "data"           # JSONPath to array of items
    name_field: "name"
  get_endpoint:
    path: "value"          # JSONPath to value
  set_endpoint:
    status_codes: [200, 201, 204]
```

### 2.4 **No List vs Get Distinction** ⚠️ MEMORY LEAK
```go
// CurrentGetSecret() calls ListSecrets() then re-fetches full list
// Inefficient and some vaults may not support dual-endpoint pattern
```
**Problem**:
- GetSecret re-fetches entire list
- Assumes list-based retrieval pattern works
- Some vaults (like Azure) have dedicated GET endpoints

**Solution**:
```yaml
operations:
  list:
    endpoint: "%{endpoint}/list"
    response_path: "items"
  get:
    endpoint: "%{endpoint}/{secret_id}"  # Direct access
    response_path: "value"
  set:
    endpoint: "%{endpoint}"
    method: PUT
```

---

## 3. Architecture Improvements

### 3.1 **Create Vault Backend Interface** 🎯 RECOMMENDED
```go
// vault/backend.go (NEW)
package vault

type Backend interface {
    // Core operations
    ListSecrets(ctx context.Context) ([]string, error)
    GetSecret(ctx context.Context, name string) (*Secret, error)
    SetSecret(ctx context.Context, name, value string) error
    DeleteSecret(ctx context.Context, name string) error
    
    // Metadata
    Type() string  // "vaultwarden", "vault", "azure", etc.
    Capabilities() BackendCapabilities
}

type BackendCapabilities struct {
    SupportsDirectAccess  bool  // GET /secret/{id}
    SupportsList          bool  // GET /list
    SupportsDelete        bool
    SupportsMetadata      bool
    RequiresRefresh       bool  // Needs periodic token refresh?
}
```

### 3.2 **Vault-Specific Implementations** 🎯 RECOMMENDED
```
vault/
├── backend.go          (Interface definition)
├── client.go           (Generic HTTP client wrapper)
├── implementations/
│   ├── vaultwarden.go  (Vaultwarden-specific impl)
│   ├── hashicorp.go    (HashiCorp Vault impl)
│   ├── azure.go        (Azure Key Vault impl)
│   └── generic_http.go (Fallback for unknown APIs)
```

### 3.3 **Configuration-Driven Response Parsing** 🎯 RECOMMENDED

Instead of hardcoded logic, use configuration with template-based parsing:

```yaml
vaults:
  - id: my_vaultwarden
    type: vaultwarden
    endpoint: https://vault.example.com/api/ciphers
    operations:
      list:
        response_extractors:
          items_path: "data"           # JSONPath to array
          name_field: "name"            # Field containing secret name
      get:
        endpoint_template: "{base}"    # Reuse list endpoint
        response_extractors:
          match_by: "name"             # Match response items by this field
          value_path: "data"           # JSONPath to extract from matched item
      set:
        endpoint_template: "{base}"
        request_template:
          name_field: "name"
          value_field: "name"          # Just echo the name back
        response_validators:
          status_codes: [200, 201]
        optional_fields:               # Add conditional fields
          - if_field: "name"
            add_field: "type"
            value: "1"
```

---

## 4. Configuration Schema Improvements

### 4.1 **Current Issues**
```yaml
vaults:
  - id: id
    endpoint: url
    auth_method: bearer
    # No way to know which vault type this is!
```

### 4.2 **Proposed Enhancement**
```yaml
vaults:
  - id: my_vault
    type: vaultwarden                    # NEW: Explicit type
    name: "Vaultwarden Instance"
    endpoint: https://vault.example.com
    
    # Authentication
    auth_method: oauth2
    auth_config:                         # NEW: Structured auth config
      oauth:
        token_endpoint: https://vault.example.com/identity/connect/token  # Configurable!
        client_id: "..."
        client_secret: "..."
        scope: "api"
        extra_params:                    # NEW: Support extra OAuth params
          device_identifier: sync-daemon
          device_name: sync-daemon
          device_type: "14"
    
    # Vault-specific behavior
    operations_override:                 # NEW: Override default operations
      list:
        method: GET
        endpoint: /api/ciphers
        response:
          path: "data"                   # JSONPath
          name_field: "name"
      get:
        # If not specified, falls back to list + filter
      set:
        method: POST
        endpoint: /api/ciphers
        optional_fields:
          type: "1"                      # Add type=1 for login objects
    
    # Capabilities
    capabilities:                        # NEW: Declare what this vault can do
      list: true
      get_individual: false              # Can't fetch individual secrets
      set: true
      delete: true
      batch_operations: false
    
    timeout: 30
    skip_ssl_verify: false
```

---

## 5. Recommendations by Priority

### 🔴 HIGH PRIORITY (Blockers for generalization)

1. **Extract OAuth Token Endpoint from Hardcoded Path**
   - Make configurable: `auth_config.oauth.token_endpoint`
   - Current: `strings.TrimSuffix(endpoint, "/api/ciphers")` → hardcoded

2. **Add Vault Type Field**
   - New field: `type: vaultwarden|vault|azure|generic`
   - Enables vault-specific logic when needed

3. **Refactor Response Parsing**
   - Use JSONPath-style config instead of Go code
   - Remove hardcoded `data[]` and `keys[]` assumptions

### 🟡 MEDIUM PRIORITY (Improves quality)

4. **Create Backend Interface**
   - Define what all vault backends must implement
   - Enable easier addition of new vault types

5. **Make HTTP Operations Configurable**
   - Allow endpoint templates, method overrides
   - Support vault-specific HTTP patterns

6. **Extract Vaultwarden-Specific Logic**
   - `type=1` for login objects → config option
   - Device identification in OAuth → optional OAuth params

### 🟢 LOW PRIORITY (Nice-to-have)

7. **Add Vault Capabilities Declaration**
   - Metadata about what operations are supported
   - Better error messages when unsupported operations requested

8. **Multiple Response Formats**
   - Support flat arrays, nested objects, pagination
   - Template-based field extraction

---

## 6. Implementation Roadmap

### Phase 1: Low-Risk Configuration Changes (30 mins)
- Add `type` field to VaultConfig
- Add `operations_override` section
- Add `auth_config.oauth.token_endpoint`
- Keep backward compatibility (defaults to current behavior)

### Phase 2: Refactor Response Parsing (2-3 hours)
- Extract response parsing logic from vault/client.go
- Create `ResponseParser` interface
- Implement config-based parser
- Keep current parser as default for existing configs

### Phase 3: Backend Interface (1-2 hours)
- Define `Backend` interface
- Implement wrapper that calls config-based operations
- Prepare structure for vault-specific implementations

### Phase 4: Additional Vault Support (Per vault, 2-4h each)
- HashiCorp Vault backend
- Azure Key Vault backend
- AWS Secrets Manager backend

---

## 7. Backward Compatibility Strategy

All changes can be made **backward compatible**:

```go
// In client.go, when parsing responses:
if cfg.ResponseParser != nil {
    // Use configured parser
    return cfg.ResponseParser.List(resp)
} else {
    // Fall back to current hardcoded logic
    return parseListResponseLegacy(resp)
}
```

**Result**: Existing configs work unchanged while new configs can use enhanced features.

---

## 8. Code Examples

### Before (Vaultwarden-specific):
```go
// vault/client.go
baseURL := strings.TrimSuffix(c.cfg.Endpoint, "/api/ciphers")
tokenURL := fmt.Sprintf("%s/identity/connect/token", baseURL)
```

### After (Generic):
```go
// Get from config
tokenURL := c.cfg.AuthConfig.OAuth.TokenEndpoint
if tokenURL == "" {
    tokenURL = c.getDefaultTokenEndpoint()  // Smart default per vault type
}
```

---

## 9. Summary Table

| Issue | Severity | Vaultwarden-Specific? | Solution |
|-------|----------|----------------------|----------|
| Hardcoded token endpoint | HIGH | YES | Config: `auth_config.oauth.token_endpoint` |
| Hardcoded response structure | HIGH | PARTIAL | Config: JSONPath-based extractors |
| Hardcoded type=1 field | MEDIUM | YES | Config: `optional_fields` |
| No vault type metadata | MEDIUM | NO | Add `type` field to VaultConfig |
| No HTTP method flexibility | MEDIUM | NO | Config: `operations_override` |
| No Backend interface | MEDIUM | NO | Create `vault.Backend` interface |
| Response parsing logic in Go | MEDIUM | PARTIAL | Config-driven with fallback |
| Device params hardcoded | LOW | YES | Config: `auth_config.oauth.extra_params` |

---

## Next Steps

1. **Review & Approve**: Get user sign-off on approach
2. **Phase 1 Implementation**: Add config fields (30 mins)
3. **Phase 2 Implementation**: Refactor response parsing (2-3h)
4. **Phase 3 Implementation**: Create Backend interface (1-2h)
5. **Integration Testing**: Verify backward compatibility
6. **Documentation**: Update README with new options

