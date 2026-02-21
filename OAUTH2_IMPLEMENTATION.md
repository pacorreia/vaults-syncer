# OAuth 2.0 Implementation Summary

## Overview
Successfully implemented OAuth 2.0 (Client Credentials flow) support for authenticating with Vaultwarden instances. The sync daemon can now authenticate against production Vaultwarden servers using client credentials without needing bearer tokens.

## Changes Made

### 1. **vault/client.go** - OAuth 2.0 Authentication Support
- **Added `getOAuthToken()` method**:
  - Implements OAuth 2.0 Client Credentials flow
  - Exchanges client credentials for JWT access token via `/identity/connect/token` endpoint
  - Caches tokens and manages expiration with 5-minute buffer
  - Supports configurable scope parameter
  - Returns detailed error messages for debugging

- **Updated `addAuthHeaders()` method**:
  - Now returns error to properly propagate OAuth failures
  - Routes `oauth2` auth_method to `getOAuthToken()`
  - Applies Bearer token to Authorization header

- **Updated all HTTP request methods**:
  - `GetSecret()`, `SetSecret()`, `DeleteSecret()`, `TestConnection()`
  - All now properly handle OAuth token exchange errors
  - Errors are propagated upward instead of silently ignored

### 2. **OAuth Token Configuration**
Users can configure OAuth authentication in YAML config:

```yaml
vaults:
  - id: vaultwarden_prod
    endpoint: "https://vault.pfmchome.cc/api/ciphers"
    auth_method: oauth2
    auth_headers:
      client_id: "user.682f58c6-e2fe-4774-a6f3-be010a061224"
      client_secret: "bcUnuYv78gzg5NFxh51vlgMbfQYVyi"
      scope: "api"
    skip_ssl_verify: true  # For development/testing
```

### 3. **Token Management Features**
- **Token Caching**: Reuses valid tokens until expiration
- **Expiration Buffer**: Refreshes tokens 5 minutes before actual expiration
- **Error Logging**: Returns detailed error messages for failed token exchanges
- **Device Parameters**: Identifies sync daemon with device_identifier, device_name, device_type for audit trails

## Implementation Details

### Token Exchange Flow
```
1. Application needs vault data
2. Calls addAuthHeaders(req) with auth_method: "oauth2"
3. getOAuthToken() checks cache
4. If expired/missing, POSTs to /identity/connect/token with:
   - grant_type: client_credentials
   - client_id, client_secret, scope
   - device_identifier, device_name, device_type
5. Vaultwarden returns access_token + expires_in
6. Token cached with expiry = (expires_in - 300) seconds
7. Bearer token applied to subsequent API requests
```

### Key Fix: Custom Headers Issue
**Problem**: Token endpoint returned 415 Unsupported Media Type
**Root Cause**: Custom headers from vault config (Content-Type: application/json) were overriding the token endpoint's required Content-Type: application/x-www-form-urlencoded
**Solution**: OAuth token request explicitly sets its own Content-Type and does NOT inherit custom headers from vault config

## Testing & Verification

### Test Results
✅ **Mock Vault Sync**: Successfully synced 2 test secrets between mock vaults
✅ **OAuth Token Exchange**: Successfully obtained access token from production Vaultwarden
✅ **Production Sync**: Successfully synced **51 secrets** from production Vaultwarden using OAuth 2.0

### Test Configuration
```yaml
vaults:
  - id: vaultwarden_oauth_source
    endpoint: "https://vault.pfmchome.cc/api/ciphers"
    auth_method: oauth2
    auth_headers:
      client_id: "user.682f58c6-e2fe-4774-a6f3-be010a061224"
      client_secret: "bcUnuYv78gzg5NFxh51vlgMbfQYVyi"
      scope: "api"
    skip_ssl_verify: true

  - id: mock_target
    endpoint: "http://mock-vaults:8001/api/ciphers"
    auth_method: bearer
    auth_headers:
      token: "target_admin_token_12345"
```

### Docker Network Configuration
Fixed container DNS resolution by adding explicit bridge network to docker-compose.test.yml:
```yaml
networks:
  sync-network:
    driver: bridge
    ipam:
      config:
        - subnet: 172.20.0.0/16
```

## Security Considerations

1. **Client Credentials Handling**: OAuth client secrets should be:
   - Stored in environment variables or secure vaults
   - Never committed to version control
   - Only accessible by authorized services

2. **Token Storage**: Tokens are:
   - Cached in memory only (volatile storage)
   - Cleared on application restart
   - Subject to the 5-minute expiration buffer

3. **HTTPS**: Production environments should:
   - Use HTTPS for OAuth token endpoints (required by Vaultwarden)
   - Set skip_ssl_verify: false (or omit) for production

## Supported Authentication Methods

The vault client now supports:
- ✅ OAuth 2.0 (Client Credentials) - NEW
- ✅ Bearer Token
- ✅ Basic Authentication
- ✅ API Key
- ✅ Custom Headers

## Future Enhancements

Possible improvements:
1. OAuth refresh token support for extended-lived credentials
2. Multiple device profiles for different sync scenarios
3. Metrics for OAuth token exchanges (failures, cache hits)
4. Rotation of client credentials without restarting daemon
5. Support for other OAuth 2.0 flows (Authorization Code, etc.)

## Files Changed
- `vault/client.go` - OAuth 2.0 implementation
- `docker-compose.test.yml` - Network configuration fix
- `config.real-oauth.yaml` - Example OAuth configuration (new)
