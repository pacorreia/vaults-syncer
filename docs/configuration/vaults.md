# Configuring Vaults

A vault is a credentials storage system where secrets are stored. This guide covers how to configure different vault types.

## Supported Vault Types

- [Azure Key Vault](#azure-key-vault) - Microsoft's cloud secrets management
- [Bitwarden](#bitwarden) - Open-source password manager
- [HashiCorp Vault](#hashicorp-vault) - Enterprise secrets management
- [AWS Secrets Manager](#aws-secrets-manager) - AWS managed secrets
- [Generic REST API](#generic-rest-api) - Custom HTTP-based vaults

## Azure Key Vault

Azure Key Vault is Microsoft's cloud service for securely storing and managing secrets.

### Basic Configuration

```yaml
vaults:
  - id: azure-prod
    name: "Azure Key Vault - Production"
    type: azure
    endpoint: "https://myprodvault.vault.azure.net/secrets"
    auth:
      method: bearer
      headers:
        token: "${AZURE_ACCESS_TOKEN}"
    field_names:
      name_field: "name"
      value_field: "value"
```

### Notes

- The adapter expects a bearer token. Obtain it via Azure AD (CLI, managed identity, or service principal) and provide it as `auth.headers.token`.
- If your endpoint does not include `api-version`, the client appends `api-version=7.4` by default. Override with `operations_override` if needed.

### Optional Overrides

```yaml
vaults:
  - id: azure-prod
    type: azure
    endpoint: "https://myprodvault.vault.azure.net/secrets"
    auth:
      method: bearer
      headers:
        token: "${AZURE_ACCESS_TOKEN}"
    field_names:
      name_field: "name"
      value_field: "value"
    operations_override:
      list:
        endpoint: "https://myprodvault.vault.azure.net/secrets?api-version=7.4"
        response:
          path: "value"
          name_field: "name"
      get:
        endpoint: "https://myprodvault.vault.azure.net/secrets/{name}?api-version=7.4"
        response:
          value_path: "value"
```

## Bitwarden

Bitwarden is an open-source, self-hosted password manager with a modern API.

### Basic Configuration

```yaml
vaults:
  - id: bitwarden-prod
    name: "Bitwarden - Production"
    type: bitwarden
    endpoint: "https://vault.example.com/api/ciphers"
    auth:
      method: oauth2
      oauth:
        client_id: "${BITWARDEN_CLIENT_ID}"
        client_secret: "${BITWARDEN_CLIENT_SECRET}"
        scope: api
    field_names:
      name_field: "name"
      value_field: "login"

**Bitwarden Cloud**: use `https://api.bitwarden.com/ciphers` as the endpoint. The OAuth token endpoint defaults to `https://identity.bitwarden.com/connect/token` when the API host is detected.
```

### Connection Options

| Option | Type | Required | Description |
|--------|------|----------|-------------|
| `id` | string | Yes | Unique identifier |
| `name` | string | No | Human-readable name |
| `type` | string | Yes | Must be `bitwarden` |
| `endpoint` | string | Yes | Bitwarden-compatible ciphers endpoint |
| `auth` | object | Yes | Authentication config |

### Authentication

#### OAuth2 (Recommended)

```yaml
vaults:
  - id: bitwarden
    type: bitwarden
    endpoint: "https://vault.example.com/api/ciphers"
    auth:
      method: oauth2
      oauth:
        client_id: "${BW_CLIENT_ID}"
        client_secret: "${BW_CLIENT_SECRET}"
        scope: api
    field_names:
      name_field: "name"
      value_field: "login"
```

**Setup**:

1. Log in to Bitwarden admin panel
2. Go to Settings → Integrations → OAuth2 Applications
3. Create new application
4. Grant required scopes:
   - `cipher:read` - Read ciphers
   - `cipher:write` - Write ciphers
   - `folder:read` - Read folders
   - `folder:write` - Write folders
5. Copy `client_id` and `client_secret`

#### API Key

```yaml
vaults:
  - id: bitwarden-api
    type: bitwarden
    endpoint: "https://vault.example.com/api/ciphers"
    auth:
      method: api_key
      headers:
        api_key: "${BITWARDEN_API_KEY}"
    field_names:
      name_field: "name"
      value_field: "login"
```

### Advanced Options

```yaml
vaults:
  - id: bitwarden-adv
    type: bitwarden
    endpoint: "https://vault.example.com/api/ciphers"
    auth:
      method: oauth2
      oauth:
        client_id: "${BW_CLIENT_ID}"
        client_secret: "${BW_CLIENT_SECRET}"
        scope: api
    field_names:
      name_field: "name"
      value_field: "login"

### Bitwarden Cloud Example

```yaml
vaults:
  - id: bitwarden-cloud
    type: bitwarden
    endpoint: "https://api.bitwarden.com/ciphers"
    auth:
      method: oauth2
      oauth:
        client_id: "${BITWARDEN_CLIENT_ID}"
        client_secret: "${BITWARDEN_CLIENT_SECRET}"
        scope: api
    field_names:
      name_field: "name"
      value_field: "login"
    operations_override:
      list:
        response:
          path: "data"
          name_field: "name"
      get:
        response:
          value_path: "data"
```

## Keeper Secrets Manager

Keeper provides a JSON API for secrets. Use operations overrides to map list/get/set/delete paths.

```yaml
vaults:
  - id: keeper
    type: keeper
    endpoint: "https://keeper.example.com/api/secrets"
    auth:
      method: bearer
      headers:
        token: "${KEEPER_TOKEN}"
    field_names:
      name_field: "title"
      value_field: "data"
    operations_override:
      list:
        endpoint: "https://keeper.example.com/api/secrets"
        response:
          path: "records"
          name_field: "title"
      get:
        endpoint: "https://keeper.example.com/api/secrets/{name}"
        response:
          value_path: "data"
      set:
        endpoint: "https://keeper.example.com/api/secrets/{name}"
        method: PUT
      delete:
        endpoint: "https://keeper.example.com/api/secrets/{name}"
        method: DELETE
```
```

## HashiCorp Vault

HashiCorp Vault is an enterprise secrets management solution.

### Basic Configuration

```yaml
vaults:
  - id: hashicorp-prod
    name: "HashiCorp Vault - Production"
    type: vault
    endpoint: "https://vault.example.com:8200/v1/secret/data"
    auth:
      method: custom
      headers:
        X-Vault-Token: "${VAULT_TOKEN}"
    field_names:
      name_field: "name"
      value_field: "value"
```

### Notes

- The generic adapter expects Vault KV v2 semantics (list via `metadata`, read/write via `data`).
- If you authenticate with AppRole/Kubernetes, exchange credentials for a token externally and pass it in `X-Vault-Token`.
```

## AWS Secrets Manager

AWS managed secrets service integrated with IAM.

### Basic Configuration

```yaml
vaults:
  - id: aws-prod
    name: "AWS Secrets Manager - Production"
    type: aws
    endpoint: "https://secretsmanager.us-east-1.amazonaws.com"
    auth:
      method: custom
      headers:
        X-Amz-Security-Token: "${AWS_SESSION_TOKEN}"
    field_names:
      name_field: "Name"
      value_field: "SecretString"
```

### Notes

- The generic adapter does not implement AWS signing. Use a proxy or pre-signed endpoints if required.
```

## Generic REST API

Configure any HTTP-based vault system with REST API:

### Basic Configuration

```yaml
vaults:
  - id: generic-api
    name: "Generic REST API"
    type: generic
    endpoint: "https://secrets.example.com"
    auth:
      method: bearer
      headers:
        token: "${API_TOKEN}"
    field_names:
      name_field: "name"
      value_field: "value"
```

### Authentication Methods

#### Bearer Token

```yaml
vaults:
  - id: rest-bearer
    type: generic
    endpoint: "https://api.example.com"
    auth:
      method: bearer
      headers:
        token: "${API_TOKEN}"
    field_names:
      name_field: "name"
      value_field: "value"
```

#### Basic Authentication

```yaml
vaults:
  - id: rest-basic
    type: generic
    endpoint: "https://api.example.com"
    auth:
      method: basic
      headers:
        username: "${API_USER}"
        password: "${API_PASSWORD}"
    field_names:
      name_field: "name"
      value_field: "value"
```

#### API Key (Header)

```yaml
vaults:
  - id: rest-apikey
    type: generic
    endpoint: "https://api.example.com"
    auth:
      method: api_key
      headers:
        api_key: "${API_KEY}"
    field_names:
      name_field: "name"
      value_field: "value"
```

#### OAuth2

```yaml
vaults:
  - id: rest-oauth2
    type: generic
    endpoint: "https://api.example.com"
    auth:
      method: oauth2
      oauth:
        token_endpoint: "https://auth.example.com/token"
        client_id: "${CLIENT_ID}"
        client_secret: "${CLIENT_SECRET}"
        scope: "api"
    field_names:
      name_field: "name"
      value_field: "value"
```

### Custom API Configuration

```yaml
vaults:
  - id: rest-custom
    type: generic
    endpoint: "https://api.example.com"
    auth:
      method: bearer
      headers:
        token: "${API_TOKEN}"
    field_names:
      name_field: "name"
      value_field: "value"
    operations_override:
      list:
        endpoint: "https://api.example.com/v1/secrets"
        response:
          path: "data"
          name_field: "name"
      get:
        endpoint: "https://api.example.com/v1/secrets/{name}"
        response:
          value_path: "value"
      set:
        endpoint: "https://api.example.com/v1/secrets/{name}"
        method: PUT
      delete:
        endpoint: "https://api.example.com/v1/secrets/{name}"
        method: DELETE
```

## Multiple Vaults of Same Type

You can configure multiple vaults of the same type:

```yaml
vaults:
  - id: akv-prod
    type: azure
    endpoint: "https://prod.vault.azure.net/secrets"
    auth:
      method: bearer
      headers:
        token: "${AZURE_ACCESS_TOKEN}"
    field_names:
      name_field: "name"
      value_field: "value"
  
  - id: akv-staging
    type: azure
    endpoint: "https://staging.vault.azure.net/secrets"
    auth:
      method: bearer
      headers:
        token: "${AZURE_ACCESS_TOKEN}"
    field_names:
      name_field: "name"
      value_field: "value"
  
  - id: akv-dev
    type: azure
    endpoint: "https://dev.vault.azure.net/secrets"
    auth:
      method: bearer
      headers:
        token: "${AZURE_ACCESS_TOKEN}"
    field_names:
      name_field: "name"
      value_field: "value"
```

## Vault Health Check

Verify vault connectivity:

```bash
# Health checks are performed on startup and periodically
curl http://localhost:8080/vaults/health

# Response:
{
  "vaults": {
    "azure-prod": {
      "status": "healthy",
      "endpoint": "https://prod.vault.azure.net/",
      "last_check": "2024-01-15T10:30:00Z"
    },
    "bitwarden": {
      "status": "healthy",
      "endpoint": "https://vault.example.com",
      "last_check": "2024-01-15T10:30:00Z"
    }
  }
}
```

## Common Issues

### Connection Timeout

**Problem**: Vault not reachable

**Solutions**:
- Verify endpoint URL
- Check network connectivity
- Verify firewall rules
- Check DNS resolution

### Authentication Failed

**Problem**: Wrong credentials or permissions

**Solutions**:
- Verify credentials
- Check credential expiration
- Verify required permissions
- Check managed identity is enabled

### SSL Certificate Error

**Problem**: TLS verification fails

**Solutions**:
- Verify certificate is valid
- Update system certificates
- For development only: use `disable_ssl_verify: true`

## Best Practices

✅ **Do**:
- Use managed identities on cloud platforms
- Store sensitive values in environment variables
- Verify connectivity during startup
- Enable audit logging
- Use least-privilege permissions
- Rotate credentials regularly

❌ **Don't**:
- Hardcode credentials in configuration files
- Use `disable_ssl_verify: true` in production
- Share configuration files with secrets
- Use overly permissive credentials
- Forget to test backup/restore procedures

## Next Steps

- [Configure Authentication](./authentication.md)
- [Create Syncs](./syncs.md)
- [Go back to Configuration](./README.md)
