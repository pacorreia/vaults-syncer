# Configuration

Learn how to configure vaults-syncer for your environment.

## Configuration Overview

The application is configured through a YAML configuration file that defines:

- **Global settings**: Logging, sync intervals, timeouts
- **Vaults**: Where secrets are stored and how to connect
- **Syncs**: Which secrets to sync between vaults
- **Filters**: Which secrets to include/exclude
- **Transformations**: How to modify secret names and values

## Key Topics

### [Vaults](./vaults.md)

Define and configure vault connections using the generic HTTP adapter:

- **Azure Key Vault** (type `azure`)
- **Bitwarden / Vaultwarden** (types `bitwarden`, `vaultwarden`)
- **HashiCorp Vault** (type `vault`)
- **Keeper Secrets Manager** (type `keeper`)
- **Custom REST APIs** (type `generic`)

Learn how to:
- Add vault endpoints
- Configure authentication methods
- Set connection timeouts
- Handle retry logic

### [Authentication](./authentication.md)

Set up secure authentication for your vaults (generic HTTP adapter):

- **Bearer** (OAuth tokens, Azure AD access tokens, etc.)
- **OAuth2** (client credentials flow)
- **Basic** (username/password)
- **API Key**
- **Custom** (arbitrary headers)

### [Syncs](./syncs.md)

Define synchronization rules:

- **Sync Configuration**:
  - Source and target vaults
  - Scheduling (cron expressions)
  - Sync modes (one-way, bidirectional)
  - Conflict resolution strategies

- **Filtering and Selection**:
  - Include/exclude patterns
  - Regex matching
  - Tag-based filtering

- **Transformations**:
  - Name mapping
  - Value transformations
  - Custom scripts

### Global Settings

Configure global behavior:

- **Logging**:
  - Log levels (debug, info, warn, error)
  - Output formats (text, JSON)
  - Log destinations (stdout, file, syslog)

- **Sync Options**:
  - Global sync interval
  - Timeout values
  - Retry policies

- **Security**:
  - Encryption settings
  - TLS configuration
  - RBAC settings

- **Monitoring**:
  - Metrics collection
  - Health checks
  - Alerting

## Configuration File Structure

```yaml
logging:
  level: info                 # Log level: debug, info, warn, error
  format: json                # Format: json, text

server:
  port: 8080
  address: 0.0.0.0
  metrics_port: 9090
  metrics_address: 0.0.0.0

# Define available vaults
vaults:
  - id: vault-1
    name: "Primary Vault"
    type: azure
    endpoint: "https://vault.example.com/secrets"
    auth:
      method: bearer
      headers:
        token: ${AZURE_ACCESS_TOKEN}
    field_names:
      name_field: name
      value_field: value
    
  - id: vault-2
    name: "Secondary Vault"
    type: bitwarden
    endpoint: "https://vault2.example.com/api/ciphers"
    auth:
      method: oauth2
      oauth:
        client_id: ${CLIENT_ID}
        client_secret: ${CLIENT_SECRET}
        scope: api
    field_names:
      name_field: name
      value_field: login

# Define sync operations
syncs:
  - id: sync-1
    name: "Primary Sync"
    source: vault-1
    targets: [vault-2]
    schedule: "0 * * * *"     # Every hour
    sync_type: unidirectional # or bidirectional
    
    # Optional: Filtering
    filter:
      patterns:
        - "prod-*"
      exclude:
        - "*-dev"
```

## Quick Examples

### Example 1: Basic One-Way Sync

```yaml
vaults:
  - id: akv
    type: azure
    endpoint: https://myvault.vault.azure.net/secrets
    auth:
      method: bearer
      headers:
        token: ${AZURE_ACCESS_TOKEN}
    field_names:
      name_field: name
      value_field: value
  
  - id: bitwarden
    type: bitwarden
    endpoint: https://vault.example.com/api/ciphers
    auth:
      method: oauth2
      oauth:
        client_id: ${BW_CLIENT_ID}
        client_secret: ${BW_CLIENT_SECRET}
        scope: api
    field_names:
      name_field: name
      value_field: login

syncs:
  - id: main-sync
    source: akv
    targets: [bitwarden]
    schedule: "0 * * * *"
    sync_type: unidirectional
```

### Example 2: Multi-Vault Setup

```yaml
vaults:
  - id: source-vault
    type: azure
    endpoint: https://source.vault.azure.net/secrets
    auth:
      method: bearer
      headers:
        token: ${AZURE_ACCESS_TOKEN}
    field_names:
      name_field: name
      value_field: value
  
  - id: target-primary
    type: bitwarden
    endpoint: https://vault1.example.com/api/ciphers
    auth:
      method: oauth2
      oauth:
        client_id: ${BW_PRIMARY_CLIENT_ID}
        client_secret: ${BW_PRIMARY_CLIENT_SECRET}
        scope: api
    field_names:
      name_field: name
      value_field: login
  
  - id: target-secondary
    type: bitwarden
    endpoint: https://vault2.example.com/api/ciphers
    auth:
      method: oauth2
      oauth:
        client_id: ${BW_SECONDARY_CLIENT_ID}
        client_secret: ${BW_SECONDARY_CLIENT_SECRET}
        scope: api
    field_names:
      name_field: name
      value_field: login

syncs:
  - id: sync-to-primary
    source: source-vault
    targets: [target-primary]
    schedule: "0 */4 * * *"  # Every 4 hours
    sync_type: unidirectional
  
  - id: sync-to-secondary
    source: source-vault
    targets: [target-secondary]
    schedule: "0 */6 * * *"  # Every 6 hours
    sync_type: unidirectional
```

### Example 3: Filtered Sync

```yaml
syncs:
  - id: prod-only-sync
    source: akv
    targets: [bitwarden]
    schedule: "0 2 * * *"
    sync_type: unidirectional
    filter:
      patterns:
        - "prod-*"
        - "app-*"
```

## Configuration File Locations

The application looks for configuration in this order:

1. **Command-line flag**: `-config /path/to/config.yaml`
2. **Environment variable**: `CONFIG_PATH=/path/to/config.yaml`
3. **Current directory**: `./config.yaml`
4. **System directory**: `/etc/akv-sync/config.yaml`

### Example Usage

```bash
# Specify config file
./sync-daemon -config /etc/sync/config.yaml

# Use environment variable
export CONFIG_PATH=/etc/sync/config.yaml
./sync-daemon

# Docker
docker run -v /path/to/config.yaml:/etc/sync/config.yaml:ro \
  ghcr.io/pacorreia/vaults-syncer:latest
```

## Environment Variables

Use environment variables for sensitive values:

```yaml
vaults:
  - id: bitwarden
    type: bitwarden
    endpoint: https://vault.example.com/api/ciphers
    auth:
      method: oauth2
      oauth:
        client_id: ${BITWARDEN_CLIENT_ID}
        client_secret: ${BITWARDEN_CLIENT_SECRET}
        scope: api
    field_names:
      name_field: name
      value_field: login
```

Set environment variables:

```bash
# Linux/macOS
export BITWARDEN_CLIENT_ID=your-client-id
export BITWARDEN_CLIENT_SECRET=your-client-secret

# Windows PowerShell
$env:BITWARDEN_CLIENT_ID = "your-client-id"
$env:BITWARDEN_CLIENT_SECRET = "your-client-secret"

# Docker
docker run -e BITWARDEN_CLIENT_ID=... -e BITWARDEN_CLIENT_SECRET=... ...
```

## Configuration Validation

### Validate Configuration

```bash
# Validate syntax
./sync-daemon -validate-config config.yaml

# Docker
docker run --rm -v $(pwd)/config.yaml:/etc/sync/config.yaml:ro \
  ghcr.io/pacorreia/vaults-syncer:latest validate
```

### Common Configuration Errors

| Error | Cause | Solution |
|-------|-------|----------|
| `unknown vault type` | Invalid vault type | Check supported types: azure, bitwarden, vaultwarden, vault, keeper, aws, generic |
| `endpoint required` | Missing endpoint | Add endpoint URL for vault |
| `invalid cron schedule` | Bad cron syntax | Use 5-field cron format: `minute hour day month weekday` |
| `authentication failed` | Wrong credentials | Verify auth method and credentials |
| `vault not found` | Unknown vault reference | Check sync source/target references existing vault IDs |

## Best Practices

### Security

1. ✅ **Use environment variables** for sensitive values
2. ✅ **Encrypt credentials** using Azure Key Vault or similar
3. ✅ **Restrict file permissions**: `chmod 600 config.yaml`
4. ✅ **No hardcoded secrets** in configuration files
5. ✅ **Use service principals** instead of user accounts
6. ✅ **Enable audit logging** for compliance

### Reliability

1. ✅ **Test configurations** in non-production first
2. ✅ **Monitor sync operations** with metrics
3. ✅ **Start with one-way** syncs, move to bidirectional carefully
4. ✅ **Set appropriate intervals** (don't sync too frequently)
5. ✅ **Use filters** to limit scope and reduce errors

### Maintenance

1. ✅ **Document sync topology** for team understanding
2. ✅ **Review configurations** regularly
3. ✅ **Update credentials** when they expire
4. ✅ **Test disaster recovery** scenarios
5. ✅ **Keep backups** of critical configurations

## Configuration Examples

See the [Vaults](./vaults.md), [Authentication](./authentication.md), and [Syncs](./syncs.md) guides for detailed configuration examples.

## Next Steps

1. **[Configure Vaults](./vaults.md)** - Set up your vault connections
2. **[Set Up Authentication](./authentication.md)** - Configure how to authenticate
3. **[Create Syncs](./syncs.md)** - Define what to sync and when
