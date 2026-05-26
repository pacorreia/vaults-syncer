# Configuration

Learn how to configure vaults-syncer for your environment.

## Configuration Overview

All vault and sync configuration is stored in the database and managed through the **Web UI** (`http://localhost:8080`) or the **admin API** (`/api/config/vaults`, `/api/config/syncs`). There is no configuration file loaded at startup.

The configuration covers:

- **Vaults**: Where secrets are stored and how to connect
- **Syncs**: Which secrets to sync between vaults and when
- **Filters**: Which secrets to include/exclude
- **Transformations**: How to modify secret values during sync

## Key Topics

### [Vaults](./vaults.md)

Define and configure vault connections using the generic HTTP adapter:

- **Azure Key Vault** (type `azure`)
- **Bitwarden** (type `bitwarden`)
- **Vaultwarden** (type `vaultwarden`)
- **HashiCorp Vault** (type `vault`)
- **AWS Secrets Manager** (type `aws`)
- **Keeper Secrets Manager** (type `keeper`)
- **CLI-backed vaults** (type `tool`) — see [Tool Backend](./tool-backend.md)
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

## Admin API Payload Format

Vaults and syncs are configured via the admin API using JSON with snake_case field names. The examples below illustrate the expected payload structure for `POST /api/config/vaults` and `POST /api/config/syncs` calls.

```json
// POST /api/config/vaults
{
  "id": "vault-1",
  "name": "Primary Vault",
  "type": "azure",
  "endpoint": "https://vault.example.com/secrets",
  "auth": {
    "method": "bearer",
    "headers": {"token": "${AZURE_ACCESS_TOKEN}"}
  },
  "field_names": {
    "name_field": "name",
    "value_field": "value"
  }
}
```

```json
// POST /api/config/syncs
{
  "id": "sync-1",
  "source": "vault-1",
  "targets": ["vault-2"],
  "schedule": "0 * * * *",
  "sync_type": "unidirectional",
  "filter": {
    "patterns": ["prod-*"],
    "exclude": ["*-dev"]
  }
}
```

## Quick Examples

The examples below show the JSON payload sent to the admin API (`POST /api/config/vaults` or `POST /api/config/syncs`).

### Example 1: Basic One-Way Sync

```bash
# Add source vault (Azure Key Vault)
curl -s -X POST http://localhost:8080/api/config/vaults \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "id": "akv",
    "type": "azure",
    "endpoint": "https://myvault.vault.azure.net/secrets",
    "auth": {"method": "bearer", "headers": {"token": "${AZURE_ACCESS_TOKEN}"}},
    "field_names": {"name_field": "name", "value_field": "value"}
  }'

# Add target vault (Bitwarden)
curl -s -X POST http://localhost:8080/api/config/vaults \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "id": "bitwarden",
    "type": "bitwarden",
    "endpoint": "https://vault.example.com/api/ciphers",
    "auth": {
      "method": "oauth2",
      "oauth": {
        "client_id": "${BW_CLIENT_ID}",
        "client_secret": "${BW_CLIENT_SECRET}",
        "scope": "api"
      }
    },
    "field_names": {"name_field": "name", "value_field": "login"}
  }'

# Create sync
curl -s -X POST http://localhost:8080/api/config/syncs \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "id": "main-sync",
    "source": "akv",
    "targets": ["bitwarden"],
    "schedule": "0 * * * *",
    "sync_type": "unidirectional"
  }'
```

### Example 2: Multi-Vault Setup

```bash
# Add vaults (repeat POST /api/config/vaults for each)
# source-vault, target-primary, target-secondary ...

# Create syncs
curl -s -X POST http://localhost:8080/api/config/syncs \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"id": "sync-to-primary", "source": "source-vault", "targets": ["target-primary"], "schedule": "0 */4 * * *", "sync_type": "unidirectional"}'

curl -s -X POST http://localhost:8080/api/config/syncs \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"id": "sync-to-secondary", "source": "source-vault", "targets": ["target-secondary"], "schedule": "0 */6 * * *", "sync_type": "unidirectional"}'
```

### Example 3: Filtered Sync

```bash
curl -s -X POST http://localhost:8080/api/config/syncs \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "id": "prod-only-sync",
    "source": "akv",
    "targets": ["bitwarden"],
    "schedule": "0 2 * * *",
    "sync_type": "unidirectional",
    "filter": {"patterns": ["prod-*", "app-*"]}
  }'
```

## Configuration Storage

All configuration (vaults and syncs) is stored in the database and managed through the Web UI or the admin API. There is no configuration file read at startup.

### Environment Variables

The daemon reads environment variables at startup:

```bash
export DB_TYPE=sqlite             # sqlite (default), postgres, or mssql
export DB_PATH=sync.db            # SQLite file path
export DB_DSN=                    # PostgreSQL/MSSQL connection string
export MASTER_ENCRYPTION_KEY=     # Required after first start (32-byte base64 key)
export SERVER_PORT=8080           # HTTP API + Web UI port
export SERVER_ADDRESS=0.0.0.0     # HTTP listen address
export METRICS_PORT=9090          # Prometheus metrics port
```

### Example Usage

```bash
# Run with custom database path
DB_PATH=/data/sync.db ./sync-daemon

# Run with PostgreSQL
DB_TYPE=postgres DB_DSN="host=db port=5432 user=sync password=secret dbname=sync sslmode=disable" ./sync-daemon

# Docker
docker run \
  -e DB_TYPE=sqlite \
  -e MASTER_ENCRYPTION_KEY=<key> \
  -v sync-data:/app/data \
  ghcr.io/pacorreia/vaults-syncer:latest
```

### Common Configuration Errors

| Error | Cause | Solution |
|-------|-------|----------|
| `unknown vault type` | Invalid vault type | Check supported types: azure, bitwarden, vaultwarden, vault, aws, keeper, generic, tool |
| `endpoint required` | Missing endpoint | Add endpoint URL for vault (not required for type `tool`) |
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
