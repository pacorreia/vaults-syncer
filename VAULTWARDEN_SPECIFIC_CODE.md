# Vaultwarden-Specific Code Locations

This file maps all Vaultwarden-specific assumptions in the codebase for easy refactoring.

---

## 1. OAuth Token Endpoint (CRITICAL)

**File**: `vault/client.go` lines 346-350

```go
// 🔴 VAULTWARDEN SPECIFIC - Hardcoded path transformation
baseURL := strings.TrimSuffix(c.cfg.Endpoint, "/api/ciphers")
tokenURL := fmt.Sprintf("%s/identity/connect/token", baseURL)
```

**Why it's an issue**:
- Assumes all endpoints end with `/api/ciphers`
- Only works for Vaultwarden
- Breaks for Vault, Azure, AWS, etc.

**Fix**: Use config `auth.oauth.token_endpoint` with smart defaults

**Impact**: 🔴 BLOCKS other vault types

---

## 2. Device Identification in OAuth Parameters

**File**: `vault/client.go` lines 330-333

```go
// 🟡 VAULTWARDEN SPECIFIC - Device params not standard OAuth
data := fmt.Sprintf(
    "grant_type=client_credentials&client_id=%s&client_secret=%s&scope=%s&device_identifier=sync-daemon&device_name=sync-daemon&device_type=14",
    clientID, clientSecret, scope,
)
```

**Why it's an issue**:
- `device_identifier`, `device_name`, `device_type` are Vaultwarden-specific
- Other OAuth providers don't recognize these
- Can cause "unknown parameter" warnings

**Fix**: Make OAuth parameters configurable via `auth.oauth.extra_params`

**Impact**: 🟡 PREVENTS other OAuth providers from working

---

## 3. Response Structure Parsing

**File**: `vault/client.go` lines 118-195

```go
// 🟡 VAULTWARDEN SPECIFIC/PARTIAL - Hardcoded response paths
if items, ok := data["data"].([]interface{}); ok {
    // Assumes response is {"data": [...]}
    // Vaultwarden format
    for _, item := range items {
        // ...
    }
} else if keys, ok := data["keys"].([]interface{}); ok {
    // Vault format as fallback
    for _, key := range keys {
        names = append(names, fmt.Sprintf("%v", key))
    }
}
```

**Why it's an issue**:
- Only supports `data[]` or `keys[]` top-level fields
- Different vaults use:
  - Azure: `value[]` (flat array)
  - Vault: `data.keys[]` (nested)
  - AWS: `SecretList[]` (paginated)
- No support for:
  - Custom response paths
  - Pagination/cursors
  - Nested JSON structures

**Fix**: Config-driven response extraction with JSONPath support

**Impact**: 🟡 PREVENTS Azure, AWS, and other vault types

---

## 4. Secret GetSecret Implementation

**File**: `vault/client.go` lines 56-108

```go
// 🟡 INEFFICIENT - Calls ListSecrets then filters
// This is Vaultwarden-specific pattern
func (c *Client) GetSecret(name string) (*Secret, error) {
    secrets, err := c.ListSecrets()  // Fetches ENTIRE list
    if err != nil {
        return nil, err
    }
    
    found := false
    for _, s := range secrets {
        if s == name {  // Find the name
            found = true
            break
        }
    }
    
    // Then fetches the entire list AGAIN
    // This is wasteful and not how most vaults work
    url := strings.TrimSuffix(c.cfg.Endpoint, "/")
    req, err := http.NewRequest("GET", url, nil)
}
```

**Why it's an issue**:
- Fetches entire list twice (memory and time waste)
- Most vaults have dedicated GET endpoints: `/secret/{id}`
- Vaultwarden doesn't support direct access; must list and filter
- Other vaults support: `GET /v1/secret/data/mydata` (Vault)

**Fix**: 
- Allow direct GET endpoint override in config
- Vaultwarden: Use list-based (current)
- Vault: Use `GET /v1/data/{secret_name}`
- Azure: Use `GET /secrets/{secret}`

**Impact**: 🟡 PERFORMANCE issue, potential blocker for some vaults

---

## 5. Type=1 Field for Login Objects

**File**: `vault/client.go` lines 233-235

```go
// 🟡 VAULTWARDEN SPECIFIC - Hardcoded object type
// If the value field is "login" (Vaultwarden-style), add type field
if strings.ToLower(valueField) == "login" {
    payload["type"] = 1  // Vaultwarden-specific: type=1 means Login
}
```

**Why it's an issue**:
- Vaultwarden requires `type=1` for login objects
- No other vault uses this pattern
- Adds unnecessary field for non-Vaultwarden targets

**Fix**: Make type mappings configurable

```yaml
optional_fields:
  - condition: 'value_field == "login"'
    fields:
      type: "1"
```

**Impact**: 🟢 LOW - mostly affects Vaultwarden targets

---

## 6. Complex Value Handling

**File**: `vault/client.go` lines 130-145

```go
// 🟡 VAULTWARDEN SPECIFIC - Assumes complex objects in response
// If the value is a complex object (map or slice), serialize it as JSON
switch v := valueData.(type) {
case map[string]interface{}, []interface{}:
    if jsonBytes, err := json.Marshal(v); err == nil {
        valueStr = string(jsonBytes)
    }
default:
    valueStr = fmt.Sprintf("%v", v)
}
```

**Why it's an issue**:
- Vaultwarden can return encrypted login objects as nested JSON
- Most vaults return simple string values
- Converting everything to JSON string works but is hacky
- Prevents proper handling of structured secrets

**Fix**: Support multiple response formats in config

```yaml
value_handling:
  format: json|string|base64
  flatten: true|false
```

**Impact**: 🟡 MEDIUM - affects sync accuracy

---

## 7. No Vault Type Declaration

**File**: `config/types.go` lines 10-21

```go
// ⚠️ MISSING - No vault type field
type VaultConfig struct {
    ID               string
    Name             string
    Endpoint         string
    Method           string
    AuthMethod       string  // bearer, basic, oauth2, api_key
    AuthHeaders      map[string]string
    FieldNames       FieldNamesConfig
    Headers          map[string]string
    Timeout          int
    SkipSSLVerify    bool
    // 🔴 MISSING: VaultType field
}
```

**Why it's an issue**:
- Impossible to know vault type from config alone
- Sync engine can't make intelligent decisions
- Must hardcode assumptions in code instead of config

**Fix**: Add explicit field

```go
type VaultConfig struct {
    Type string `yaml:"type"` // vaultwarden, vault, azure, aws, generic
    // ... rest of fields ...
}
```

**Impact**: 🔴 BLOCKS intelligent vault-specific handling

---

## 8. HTTP Verb Assumptions

**File**: `vault/client.go` lines 223 & 285

```go
// 🟡 VAULTWARDEN TYPICAL - May need HTTP method override
method := strings.ToUpper(c.cfg.Method)  // Uses configured method (POST/PUT)
req, err := http.NewRequest(method, url, bytes.NewReader(body))

// Also in DeleteSecret:
req, err := http.NewRequest("DELETE", url, nil)  // Hardcoded DELETE
```

**Why it's an issue**:
- SetSecret uses `c.cfg.Method` (configurable) ✓ GOOD
- DeleteSecret hardcodes `DELETE` ✗ BAD
- Some vaults may use:
  - DELETE with query params instead of blank body
  - POST to `/delete` endpoint instead of HTTP DELETE
  - Custom deletion patterns

**Fix**: Make all HTTP verbs configurable

```yaml
operations:
  set:
    method: POST        # or PUT
  delete:
    method: DELETE      # or POST to /delete
```

**Impact**: 🟡 MEDIUM - affects deletion capabilities

---

## 9. Endpoint Structure Assumptions

**File**: `vault/client.go` lines 238, 285, 152

```go
// 🟡 Various places assume simple endpoint patterns

// Line 238 - Set/Post assumes base endpoint
url := strings.TrimSuffix(c.cfg.Endpoint, "/")

// Line 285 - Delete assumes name appends to endpoint
url := fmt.Sprintf("%s/%s", strings.TrimSuffix(c.cfg.Endpoint, "/"), name)

// Line 152 - List assumes base endpoint
url := strings.TrimSuffix(c.cfg.Endpoint, "/")
```

**Why it's an issue**:
- Assumes simple URL patterns: `base/endpoint/{name}`
- Different vaults use:
  - Vault: `/v1/secret/data/{name}` (nested)
  - Azure: `/secrets/{name}?api-version=7.3`
  - AWS: Query-based access
- No support for:
  - URL templates
  - Query parameters
  - Different endpoints for different operations

**Fix**: Support endpoint templates

```yaml
operations:
  list:
    endpoint: "{base}"
  get:
    endpoint: "{base}/{name}"
  set:
    endpoint: "{base}"
  delete:
    endpoint: "{base}/{name}?force=true"
```

**Impact**: 🟡 HIGH - prevents proper URL construction for different vaults

---

## 10. Status Code Handling

**File**: `vault/client.go` lines 101, 175-176, 269-270

```go
// 🟡 Different vaults use different success codes
if resp.StatusCode != http.StatusOK {
    // Line 101: Only accepts 200 OK for GET
    
if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusNoContent {
    // Line 269: Accepts 200, 201, 204 for POST/PUT
    
if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
    // Line 309: Accepts 200, 204 for DELETE
}
```

**Why it's an issue**:
- Hardcoded success codes
- Different vaults return different codes:
  - Some return 201 for creates
  - Some return 204 for deletes
  - Some return 200 for everything
- Azure Key Vault uses 200/404 patterns
- AWS uses 200/400 patterns

**Fix**: Make success codes configurable

```yaml
operations:
  list:
    success_codes: [200]
  get:
    success_codes: [200]
  set:
    success_codes: [200, 201, 204]
  delete:
    success_codes: [200, 204]
```

**Impact**: 🟡 MEDIUM - affects error handling

---

## Summary Table

| Location | Issue | Severity | Vaultwarden-Specific | Fix |
|----------|-------|----------|----------------------|-----|
| client.go:346-350 | OAuth endpoint hardcoded | 🔴 CRITICAL | YES | Config field + smart defaults |
| client.go:330-333 | Device params hardcoded | 🟡 MEDIUM | YES | Optional OAuth params map |
| client.go:118-195 | Response structure hardcoded | 🟡 MEDIUM | PARTIAL | JSONPath config + parser |
| client.go:56-108 | GetSecret inefficient | 🟡 MEDIUM | PARTIAL | Direct GET endpoint support |
| client.go:233-235 | Type=1 hardcoded | 🟡 LOW | YES | Optional fields config |
| client.go:130-145 | Complex value handling | 🟡 LOW | YES | Value format config |
| types.go:10-21 | No vault type field | 🔴 CRITICAL | N/A | Add `type` field |
| client.go:223, 285 | HTTP verbs partly hardcoded | 🟡 MEDIUM | PARTIAL | Verb config in operations |
| client.go:238, 285, 152 | Endpoint patterns | 🟡 MEDIUM | PARTIAL | Endpoint templates |
| client.go:101-310 | Status codes hardcoded | 🟡 MEDIUM | PARTIAL | Success codes config |

---

## Refactoring Priority

### Phase 1 (Blocking other vaults)
1. ✅ Add `type` field to VaultConfig (types.go)
2. ✅ Make OAuth token endpoint configurable (client.go)
3. ✅ Add OAuth extra params support (client.go)

### Phase 2 (Core generalization)
4. ✅ Refactor response parsing to config-driven (parser.go)
5. ✅ Support direct GET endpoints (client.go)
6. ✅ Add endpoint templates support (types.go)

### Phase 3 (Edge cases)
7. ✅ Make HTTP verbs fully configurable
8. ✅ Support status code overrides
9. ✅ Value format/handling options

### Phase 4 (New vault support)
10. ✅ Implement HashiCorp Vault backend
11. ✅ Implement Azure Key Vault backend
12. ✅ Implement AWS Secrets Manager backend

