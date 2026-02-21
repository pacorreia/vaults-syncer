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
    type: azure-keyvault
    endpoint: "https://myprodvault.vault.azure.net/"
    auth:
      method: managed-identity
```

### Connection Options

| Option | Type | Required | Description |
|--------|------|----------|-------------|
| `id` | string | Yes | Unique identifier for this vault |
| `name` | string | No | Human-readable name |
| `type` | string | Yes | Must be `azure-keyvault` |
| `endpoint` | string | Yes | Vault endpoint URL (ends with /) |
| `auth` | object | Yes | Authentication configuration |

### Authentication Methods

#### Managed Identity (Recommended)

Use Azure Managed Identity for Azure resources (VMs, App Service, AKS, etc.):

```yaml
vaults:
  - id: akv-managed
    type: azure-keyvault
    endpoint: "https://myvault.vault.azure.net/"
    auth:
      method: managed-identity
      # Optional: specify specific identity
      client_id: "00000000-0000-0000-0000-000000000000"
```

**When to use**: Running on Azure VMs, App Service, AKS, Container Instances, Functions

**Setup**: Enable system-assigned or user-assigned managed identity on your resource

#### Service Principal

Use a service principal for non-Azure environments or specific use cases:

```yaml
vaults:
  - id: akv-sp
    type: azure-keyvault
    endpoint: "https://myvault.vault.azure.net/"
    auth:
      method: service-principal
      tenant_id: "${AZURE_TENANT_ID}"
      client_id: "${AZURE_CLIENT_ID}"
      client_secret: "${AZURE_CLIENT_SECRET}"
```

**Setup**:

```bash
# Create service principal
az ad sp create-for-rbac \
  --name "akv-sync-sp" \
  --role "Key Vault Secrets Officer" \
  --scope /subscriptions/{subscription-id}/resourceGroups/{rg}/providers/Microsoft.KeyVault/vaults/{vault-name}

# This outputs:
# {
#   "appId": "...",
#   "password": "...",
#   "tenant": "..."
# }
```

#### Client Certificate

Use certificate-based authentication:

```yaml
vaults:
  - id: akv-cert
    type: azure-keyvault
    endpoint: "https://myvault.vault.azure.net/"
    auth:
      method: client-certificate
      tenant_id: "${AZURE_TENANT_ID}"
      client_id: "${AZURE_CLIENT_ID}"
      certificate_path: "/etc/certs/client.pem"
      certificate_password: "${CERT_PASSWORD}"
```

#### User Authentication (Development)

For local development:

```yaml
vaults:
  - id: akv-dev
    type: azure-keyvault
    endpoint: "https://myvault.vault.azure.net/"
    auth:
      method: user-auth
```

**Setup**: Run `az login` locally to authenticate

### Azure Advanced Options

```yaml
vaults:
  - id: akv-advanced
    type: azure-keyvault
    endpoint: "https://myvault.vault.azure.net/"
    auth:
      method: managed-identity
    
    # Optional: Connection settings
    options:
      timeout: 30              # Request timeout in seconds
      max_retries: 3           # Number of retries on failure
      retry_delay: 1000        # Delay between retries in ms
      disable_ssl_verify: false # Don't disable in production!
      
      # Optional: Proxy settings
      http_proxy: "http://proxy.example.com:8080"
      https_proxy: "https://proxy.example.com:8080"
```

## Bitwarden

Bitwarden is an open-source, self-hosted password manager with a modern API.

### Basic Configuration

```yaml
vaults:
  - id: bitwarden-prod
    name: "Bitwarden - Production"
    type: bitwarden
    endpoint: "https://vault.example.com"
    auth:
      method: oauth2
      client_id: "${BITWARDEN_CLIENT_ID}"
      client_secret: "${BITWARDEN_CLIENT_SECRET}"
```

### Connection Options

| Option | Type | Required | Description |
|--------|------|----------|-------------|
| `id` | string | Yes | Unique identifier |
| `name` | string | No | Human-readable name |
| `type` | string | Yes | Must be `bitwarden` |
| `endpoint` | string | Yes | Bitwarden server URL (no trailing slash) |
| `auth` | object | Yes | Authentication config |

### Authentication

#### OAuth2 (Recommended)

```yaml
vaults:
  - id: bitwarden
    type: bitwarden
    endpoint: "https://vault.example.com"
    auth:
      method: oauth2
      client_id: "${BW_CLIENT_ID}"
      client_secret: "${BW_CLIENT_SECRET}"
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
    endpoint: "https://vault.example.com"
    auth:
      method: api-key
      api_key: "${BITWARDEN_API_KEY}"
```

### Advanced Options

```yaml
vaults:
  - id: bitwarden-adv
    type: bitwarden
    endpoint: "https://vault.example.com"
    auth:
      method: oauth2
      client_id: "${BW_CLIENT_ID}"
      client_secret: "${BW_CLIENT_SECRET}"
    
    options:
      # Connection settings
      timeout: 30
      max_retries: 3
      
      # Organization ID (if syncing org vault)
      organization_id: "org-uuid-here"
      
      # Collection filtering
      collections:
        - "collection-uuid-1"
        - "collection-uuid-2"
```

## HashiCorp Vault

HashiCorp Vault is an enterprise secrets management solution.

### Basic Configuration

```yaml
vaults:
  - id: hashicorp-prod
    name: "HashiCorp Vault - Production"
    type: vault
    endpoint: "https://vault.example.com:8200"
    auth:
      method: token
      token: "${VAULT_TOKEN}"
```

### Authentication Methods

#### Token Authentication

```yaml
vaults:
  - id: vault-token
    type: vault
    endpoint: "https://vault.example.com:8200"
    auth:
      method: token
      token: "${VAULT_TOKEN}"
```

#### AppRole Authentication

```yaml
vaults:
  - id: vault-approle
    type: vault
    endpoint: "https://vault.example.com:8200"
    auth:
      method: approle
      role_id: "${VAULT_ROLE_ID}"
      secret_id: "${VAULT_SECRET_ID}"
```

#### Kubernetes Authentication

```yaml
vaults:
  - id: vault-k8s
    type: vault
    endpoint: "https://vault.example.com:8200"
    auth:
      method: kubernetes
      role: "sync-role"
      jwt_path: "/var/run/secrets/kubernetes.io/serviceaccount/token"
```

### Secret Path Configuration

```yaml
vaults:
  - id: vault-prod
    type: vault
    endpoint: "https://vault.example.com:8200"
    auth:
      method: token
      token: "${VAULT_TOKEN}"
    options:
      secret_path: "secret/data"  # KV v2 mount point
      namespace: "admin"          # Vault namespace (enterprise)
```

## AWS Secrets Manager

AWS managed secrets service integrated with IAM.

### Basic Configuration

```yaml
vaults:
  - id: aws-prod
    name: "AWS Secrets Manager - Production"
    type: aws-sm
    endpoint: "us-east-1"  # AWS region
    auth:
      method: iam-role
```

### Authentication Methods

#### IAM Role (Recommended for AWS)

```yaml
vaults:
  - id: aws-iam
    type: aws-sm
    endpoint: "us-east-1"
    auth:
      method: iam-role
      # Uses EC2 instance role or ECS task role
```

#### IAM User (with Keys)

```yaml
vaults:
  - id: aws-keys
    type: aws-sm
    endpoint: "us-east-1"
    auth:
      method: iam-user
      access_key_id: "${AWS_ACCESS_KEY_ID}"
      secret_access_key: "${AWS_SECRET_ACCESS_KEY}"
```

### Advanced Options

```yaml
vaults:
  - id: aws-advanced
    type: aws-sm
    endpoint: "us-east-1"
    auth:
      method: iam-role
    options:
      # Cross-account access
      role_arn: "arn:aws:iam::ACCOUNT:role/ROLE"
      
      # Secrets filtering
      secret_name_prefix: "prod/"
```

## Generic REST API

Configure any HTTP-based vault system with REST API:

### Basic Configuration

```yaml
vaults:
  - id: generic-api
    name: "Generic REST API"
    type: generic-rest
    endpoint: "https://secrets.example.com"
    auth:
      method: bearer-token
      token: "${API_TOKEN}"
```

### Authentication Methods

#### Bearer Token

```yaml
vaults:
  - id: rest-bearer
    type: generic-rest
    endpoint: "https://api.example.com"
    auth:
      method: bearer-token
      token: "${API_TOKEN}"
```

#### Basic Authentication

```yaml
vaults:
  - id: rest-basic
    type: generic-rest
    endpoint: "https://api.example.com"
    auth:
      method: basic
      username: "${API_USER}"
      password: "${API_PASSWORD}"
```

#### API Key (Header)

```yaml
vaults:
  - id: rest-apikey
    type: generic-rest
    endpoint: "https://api.example.com"
    auth:
      method: api-key
      header_name: "X-API-Key"
      api_key: "${API_KEY}"
```

#### OAuth2

```yaml
vaults:
  - id: rest-oauth2
    type: generic-rest
    endpoint: "https://api.example.com"
    auth:
      method: oauth2
      token_url: "https://auth.example.com/token"
      client_id: "${CLIENT_ID}"
      client_secret: "${CLIENT_SECRET}"
```

### Custom API Configuration

```yaml
vaults:
  - id: rest-custom
    type: generic-rest
    endpoint: "https://api.example.com"
    auth:
      method: bearer-token
      token: "${API_TOKEN}"
    options:
      # API endpoints (relative to endpoint)
      list_endpoint: "/v1/secrets"
      get_endpoint: "/v1/secrets/{id}"
      create_endpoint: "/v1/secrets"
      update_endpoint: "/v1/secrets/{id}"
      delete_endpoint: "/v1/secrets/{id}"
      
      # Request body format
      payload_format: "json"  # json, form, xml
      
      # Response parsing
      list_key: "data"        # JSON key containing secret list
      id_field: "id"          # Field containing secret ID
      name_field: "name"      # Field containing secret name
      value_field: "value"    # Field containing secret value
```

## Multiple Vaults of Same Type

You can configure multiple vaults of the same type:

```yaml
vaults:
  - id: akv-prod
    type: azure-keyvault
    endpoint: "https://prod.vault.azure.net/"
    auth:
      method: managed-identity
  
  - id: akv-staging
    type: azure-keyvault
    endpoint: "https://staging.vault.azure.net/"
    auth:
      method: managed-identity
  
  - id: akv-dev
    type: azure-keyvault
    endpoint: "https://dev.vault.azure.net/"
    auth:
      method: user-auth
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
