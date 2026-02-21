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

Define and configure vault connections:

- **Azure Key Vault** - Microsoft's cloud secrets store
- **Bitwarden** - Open-source password manager
- **HashiCorp Vault** - Enterprise secrets management
- **AWS Secrets Manager** - AWS managed secrets
- **Custom REST APIs** - Any compatible vault

Learn how to:
- Add vault endpoints
- Configure authentication methods
- Set connection timeouts
- Handle retry logic

### [Authentication](./authentication.md)

Set up secure authentication for your vaults:

- **Azure Authentication**:
  - Managed Identity (recommended for Azure resources)
  - Service Principal
  - User Authentication
  - Client Certificates

- **OAuth2** (for Bitwarden, GitHub, etc.)
- **API Keys** (for custom systems)
- **Credentials Management**:
  - Environment variables
  - Secret stores
  - Encrypted configuration

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
# Global settings
sync:
  interval: 3600              # Default sync interval in seconds
  timeout: 300                # Default timeout for operations
  max_retries: 3              # Maximum retry attempts
  retry_delay: 60             # Delay between retries in seconds

logging:
  level: info                 # Log level: debug, info, warn, error
  format: json                # Format: json, text
  output: stdout              # Output: stdout, file, syslog

# Define available vaults
vaults:
  - id: vault-1
    name: "Primary Vault"
    type: azure-keyvault      # or bitwarden, vault, aws-sm, etc.
    endpoint: "https://vault.example.com"
    auth:
      method: managed-identity # or oauth2, api-key, etc.
    
  - id: vault-2
    name: "Secondary Vault"
    type: bitwarden
    endpoint: "https://vault2.example.com"
    auth:
      method: oauth2
      client_id: ${CLIENT_ID}
      client_secret: ${CLIENT_SECRET}

# Define sync operations
syncs:
  - id: sync-1
    name: "Primary Sync"
    source: vault-1
    target: vault-2
    schedule: "0 * * * *"     # Every hour
    mode: one-way             # or bidirectional
    
    # Optional: Filtering
    filters:
      - source_regex: "^prod-.*"
        target_name: "sync-{source_name}"
    
    # Optional: Conflict resolution (for bidirectional)
    conflict_resolution: source-wins  # or target-wins
```

## Quick Examples

### Example 1: Basic One-Way Sync

```yaml
vaults:
  - id: akv
    type: azure-keyvault
    endpoint: https://myvault.vault.azure.net/
    auth:
      method: managed-identity
  
  - id: bitwarden
    type: bitwarden
    endpoint: https://vault.example.com
    auth:
      method: oauth2
      client_id: ${BW_CLIENT_ID}
      client_secret: ${BW_CLIENT_SECRET}

syncs:
  - id: main-sync
    source: akv
    target: bitwarden
    schedule: "0 * * * *"
```

### Example 2: Multi-Vault Setup

```yaml
vaults:
  - id: source-vault
    type: azure-keyvault
    endpoint: https://source.vault.azure.net/
    auth:
      method: service-principal
      tenant_id: ${AZURE_TENANT_ID}
      client_id: ${AZURE_CLIENT_ID}
      client_secret: ${AZURE_CLIENT_SECRET}
  
  - id: target-primary
    type: bitwarden
    endpoint: https://vault1.example.com
    auth:
      method: oauth2
      client_id: ${BW_PRIMARY_CLIENT_ID}
      client_secret: ${BW_PRIMARY_CLIENT_SECRET}
  
  - id: target-secondary
    type: bitwarden
    endpoint: https://vault2.example.com
    auth:
      method: oauth2
      client_id: ${BW_SECONDARY_CLIENT_ID}
      client_secret: ${BW_SECONDARY_CLIENT_SECRET}

syncs:
  - id: sync-to-primary
    source: source-vault
    target: target-primary
    schedule: "0 */4 * * *"  # Every 4 hours
  
  - id: sync-to-secondary
    source: source-vault
    target: target-secondary
    schedule: "0 */6 * * *"  # Every 6 hours
```

### Example 3: Filtered Sync

```yaml
syncs:
  - id: prod-only-sync
    source: akv
    target: bitwarden
    schedule: "0 2 * * *"
    filters:
      - source_regex: "^prod-"
        target_name: "production/{source_name}"
      - source_regex: "^app-"
        target_name: "applications/{source_name}"
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
    endpoint: https://vault.example.com
    auth:
      method: oauth2
      client_id: ${BITWARDEN_CLIENT_ID}
      client_secret: ${BITWARDEN_CLIENT_SECRET}
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
| `unknown vault type` | Invalid vault type | Check supported types: azure-keyvault, bitwarden, vault, aws-sm |
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
