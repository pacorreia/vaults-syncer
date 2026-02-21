# Authentication

Configure secure authentication for your vaults.

## Authentication Overview

Authentication is how vaults-syncer proves its identity to each vault. Different vault types support different authentication methods.

## Quick Reference

| Vault Type | Recommended Auth | Alternative Methods |
|------------|------------------|---------------------|
| Azure Key Vault | Managed Identity | Service Principal, Certificate, User |
| Bitwarden | OAuth2 | API Key |
| HashiCorp Vault | Kubernetes / AppRole | Token, LDAP |
| AWS Secrets Manager | IAM Role | IAM User Keys |
| Generic REST | Bearer Token | Basic Auth, API Key, OAuth2 |

## Azure Key Vault Authentication

### 1. Managed Identity (Recommended)

**Use when**: Running on Azure resources (VMs, App Service, AKS, Functions, Container Instances)

**Advantages**:
- ✅ No credentials to manage
- ✅ Automatic token rotation
- ✅ Best security practice
- ✅ No password expiration

**Configuration**:

```yaml
vaults:
  - id: akv-prod
    type: azure-keyvault
    endpoint: "https://myvault.vault.azure.net/"
    auth:
      method: managed-identity
      # Optional: for user-assigned identity
      client_id: "00000000-0000-0000-0000-000000000000"
```

**Setup for different Azure resources**:

#### Azure VM

```bash
# Enable system-assigned managed identity
az vm identity assign \
  --resource-group myResourceGroup \
  --name myVM

# Then grant permissions to Key Vault
az keyvault set-policy \
  --name myKeyVault \
  --object-id $(az vm show -d -g myResourceGroup -n myVM --query systemAssignedIdentity -o tsv) \
  --secret-permissions get list
```

#### Azure App Service

```bash
# Enable system-assigned managed identity in Azure Portal
# Settings → Identity → System assigned → On

# Then grant permissions
az keyvault set-policy \
  --name myKeyVault \
  --object-id <app-service-principal-id> \
  --secret-permissions get list
```

#### Azure Kubernetes Service (AKS)

```bash
# Create service account and workload identity
kubectl create serviceaccount sync-daemon -n akv-sync

# Create federated credential
az identity federated-credential create \
  --name sync-federated \
  --identity-name sync-identity \
  --resource-group myResourceGroup \
  --issuer "https://oidc.prod.workload.identity.azure.com/$(AZ_SUBSCRIPTION_ID).dfs.core.windows.net/akv-sync/" \
  --subject "system:serviceaccount:akv-sync:sync-daemon"

# Configure workload identity on pod
# See Kubernetes deployment example
```

#### Container Instances

```bash
# Assign managed identity to container
az container create \
  --resource-group myResourceGroup \
  --name akv-sync \
  --image ghcr.io/pacorreia/vaults-syncer:latest \
  --assign-identity /subscriptions/{sub}/resourcegroups/{rg}/providers/Microsoft.ManagedIdentity/userAssignedIdentities/{name}
```

### 2. Service Principal

**Use when**: Non-Azure environments, CI/CD pipelines, or testing

**Advantages**:
- ✅ Works anywhere
- ✅ Fine-grained permissions
- ✅ Easy to test

**Disadvantages**:
- ❌ Must manage credentials
- ❌ Credentials can expire
- ❌ Must refresh tokens manually

**Configuration**:

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

# Output:
# {
#   "appId": "12345678-1234-1234-1234-123456789012",
#   "displayName": "akv-sync-sp",
#   "password": "...",
#   "tenant": "abcdef12-1234-1234-1234-abcdef123456"
# }

# Set environment variables
export AZURE_TENANT_ID="abcdef12-..."
export AZURE_CLIENT_ID="12345678-..."
export AZURE_CLIENT_SECRET="..."
```

**RBAC Roles**:

```bash
# Option 1: Use built-in role (recommended)
az role assignment create \
  --assignee {client-id} \
  --role "Key Vault Secrets Officer" \
  --scope {key-vault-resource-id}

# Option 2: For read-only access
az role assignment create \
  --assignee {client-id} \
  --role "Key Vault Secrets User" \
  --scope {key-vault-resource-id}

# Option 3: Custom role - read-only
az role definition create --role-definition @custom-role.json
```

Example custom role file:

```json
{
  "Name": "Key Vault Secrets Reader",
  "Description": "Read-only access to Key Vault secrets",
  "Type": "CustomRole",
  "Actions": [
    "Microsoft.KeyVault/vaults/secrets/read",
    "Microsoft.KeyVault/vaults/read"
  ],
  "NotActions": [],
  "AssignableScopes": [
    "/subscriptions/{subscription-id}"
  ]
}
```

### 3. Client Certificate

**Use when**: Certificate-based authentication required

**Configuration**:

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

**Setup**:

```bash
# Generate certificate
openssl req -x509 -newkey rsa:2048 -keyout key.pem -out cert.pem -days 365 -nodes

# Create PKCS12 file
openssl pkcs12 -export -in cert.pem -inkey key.pem -out client.p12

# Upload to Azure AD application certificate
az ad app credential reset \
  --id {client-id} \
  --cert @cert.pem
```

### 4. User Authentication

**Use when**: Local development with user principal

**Configuration**:

```yaml
vaults:
  - id: akv-user
    type: azure-keyvault
    endpoint: "https://myvault.vault.azure.net/"
    auth:
      method: user-auth
```

**Setup**:

```bash
# Login with Azure CLI
az login

# Verify access
az keyvault secret list --vault-name myvault
```

## Bitwarden Authentication

### OAuth2

**Configuration**:

```yaml
vaults:
  - id: bitwarden
    type: bitwarden
    endpoint: "https://vault.example.com"
    auth:
      method: oauth2
      client_id: "${BITWARDEN_CLIENT_ID}"
      client_secret: "${BITWARDEN_CLIENT_SECRET}"
```

**Setup**:

1. Access Bitwarden admin panel as administrator
2. Navigate to Settings → Integrations → API/OAuth2
3. Enable API access if required
4. Create new OAuth2 application:
   - Application name: "akv-sync"
   - Grant type: Client credentials
   - Scopes needed:
     - `cipher:read`
     - `cipher:write`
     - `folder:read`
     - `folder:write`
5. Copy the generated credentials

**Scope Reference**:

| Scope | Permission | Use Case |
|-------|-----------|----------|
| `cipher:read` | Read ciphers (items) | Reading secrets from Bitwarden |
| `cipher:write` | Create/modify ciphers | Writing secrets to Bitwarden |
| `folder:read` | List folders | Organizing synced secrets |
| `folder:write` | Create/modify folders | Creating folder structure |
| `collection:read` | List collections | Filtering by collection |
| `collection:write` | Modify collections | Assigning to collections |

### API Key (Alternative)

```yaml
vaults:
  - id: bitwarden-api
    type: bitwarden
    endpoint: "https://vault.example.com"
    auth:
      method: api-key
      api_key: "${BITWARDEN_API_KEY}"
```

## HashiCorp Vault Authentication

### Token Authentication

```yaml
vaults:
  - id: vault-token
    type: vault
    endpoint: "https://vault.example.com:8200"
    auth:
      method: token
      token: "${VAULT_TOKEN}"
```

### AppRole Authentication

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

**Setup**:

```bash
# Enable AppRole auth method
vault auth enable approle

# Create AppRole
vault write auth/approle/role/sync-role \
  token_num_uses=0 \
  token_ttl=1h

# Get role ID
vault read auth/approle/role/sync-role/role-id

# Get secret ID
vault write -f auth/approle/role/sync-role/secret-id
```

### Kubernetes Authentication

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

## AWS Secrets Manager Authentication

### IAM Role (Recommended)

```yaml
vaults:
  - id: aws-sm
    type: aws-sm
    endpoint: "us-east-1"
    auth:
      method: iam-role
```

Works with:
- EC2 instance roles
- ECS task roles
- Lambda execution roles

### IAM User Keys

```yaml
vaults:
  - id: aws-sm-keys
    type: aws-sm
    endpoint: "us-east-1"
    auth:
      method: iam-user
      access_key_id: "${AWS_ACCESS_KEY_ID}"
      secret_access_key: "${AWS_SECRET_ACCESS_KEY}"
```

## Generic REST API Authentication

### Bearer Token

```yaml
vaults:
  - id: rest-bearer
    type: generic-rest
    endpoint: "https://api.example.com"
    auth:
      method: bearer-token
      token: "${API_TOKEN}"
```

### Basic Authentication

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

### API Key

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

### OAuth2

```yaml
vaults:
  - id: rest-oauth2
    type: generic-rest
    endpoint: "https://api.example.com"
    auth:
      method: oauth2
      token_url: "https://auth.example.com/oauth/token"
      client_id: "${CLIENT_ID}"
      client_secret: "${CLIENT_SECRET}"
```

## Credential Management

### Using Environment Variables

**Best practice for secrets**:

```yaml
vaults:
  - id: secure-vault
    type: bitwarden
    endpoint: "https://vault.example.com"
    auth:
      method: oauth2
      client_id: ${BITWARDEN_CLIENT_ID}
      client_secret: ${BITWARDEN_CLIENT_SECRET}
```

**Setting variables**:

```bash
# Linux/macOS
export BITWARDEN_CLIENT_ID="..."
export BITWARDEN_CLIENT_SECRET="..."
./sync-daemon -config config.yaml

# Docker with env file
echo 'BITWARDEN_CLIENT_ID=...' > .env
echo 'BITWARDEN_CLIENT_SECRET=...' >> .env
docker run --env-file .env ...

# Docker Compose
environment:
  - BITWARDEN_CLIENT_ID=${BITWARDEN_CLIENT_ID}
  - BITWARDEN_CLIENT_SECRET=${BITWARDEN_CLIENT_SECRET}
```

### Using Azure Key Vault for Credentials

Store credentials in Azure Key Vault and reference them:

```bash
# Store credential
az keyvault secret set \
  --vault-name mykeyvault \
  --name "bitwarden-client-id" \
  --value "actual-client-id"

# Use in configuration (requires special setup)
# See advanced documentation for details
```

### Using HashiCorp Vault

```bash
# Store in HashiCorp Vault
vault kv put secret/sync/bitwarden \
  client_id="..." \
  client_secret="..."

# Reference in config
auth:
  method: oauth2
  client_id: vault://secret/sync/bitwarden:client_id
  client_secret: vault://secret/sync/bitwarden:client_secret
```

## Certificate and Key Rotation

### Azure Service Principal

```bash
# Replace expiring credential
az ad sp credential reset \
  --name akv-sync-sp \
  --credential-description "new-key"

# Update environment variables
export AZURE_CLIENT_SECRET="new-secret"
```

### API Tokens

```bash
# Generate new token
BITWARDEN_CLIENT_ID=new-id
BITWARDEN_CLIENT_SECRET=new-secret

# Update without downtime (rolling deployment)
docker-compose up -d  # Pulls new config
```

## Security Best Practices

✅ **Do**:
- Use managed identities on cloud platforms (no credential management)
- Rotate credentials regularly (every 90 days recommended)
- Use least-privilege permissions
- Store credentials in environment variables or secret stores
- Enable audit logging for authentication events
- Use TLS 1.2+ for all connections
- Restrict network access to vaults

❌ **Don't**:
- Hardcode credentials in configuration files
- Commit credentials to version control
- Share credentials across multiple applications
- Use overly permissive authentication roles
- Skip certificate validation in production
- Log authentication tokens or secrets
- Disable SSL/TLS for "convenience"

## Troubleshooting

### "Authentication Failed"

1. Verify credentials are correct
2. Check credential expiration (especially for time-limited tokens)
3. Verify required permissions in the vault
4. Check network connectivity to vault endpoint
5. Review vault audit logs for permission errors

### "Token Expired"

1. Regenerate the token/credential
2. Update environment variables or configuration
3. Restart the daemon

### "Permission Denied"

1. Verify the authentication principal has required permissions
2. Check vault RBAC configuration
3. Verify correct scope/role assignment
4. Review vault audit logs

## Next Steps

- [Configure Vaults](./vaults.md)
- [Create Syncs](./syncs.md)
- [Go back to Configuration](./README.md)
