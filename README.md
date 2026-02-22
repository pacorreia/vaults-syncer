# Secrets Vault Sync Daemon

[![Build Status](https://github.com/pacorreia/vaults-syncer/actions/workflows/go-ci.yml/badge.svg)](https://github.com/pacorreia/vaults-syncer/actions/workflows/go-ci.yml)
[![Integration Tests](https://github.com/pacorreia/vaults-syncer/actions/workflows/integration-tests.yml/badge.svg)](https://github.com/pacorreia/vaults-syncer/actions/workflows/integration-tests.yml)
[![Unit Tests](https://github.com/pacorreia/vaults-syncer/actions/workflows/go-ci.yml/badge.svg?branch=main)](https://github.com/pacorreia/vaults-syncer/actions/workflows/go-ci.yml?query=branch%3Amain)
[![Go Version](https://img.shields.io/badge/Go-1.22.1-00ADD8?logo=go)](https://go.dev/)
[![License](https://img.shields.io/github/license/pacorreia/vaults-syncer)](LICENSE)

A containerized, highly customizable daemon for synchronizing secrets across multiple vaults and secret management systems.

## Features

- **Multi-vault Support**: Sync secrets between Azure Key Vault, Vaultwarden, HashiCorp Vault, AWS Secrets Manager, and custom APIs
- **Vault Type Awareness**: Smart defaults for popular vault types (Vaultwarden, HashiCorp Vault, Azure, AWS) with automatic response parsing
- **Flexible Sync Modes**:
  - **Unidirectional**: One-to-one or one-to-many syncs from a source to multiple targets
  - **Bidirectional**: Two-way sync between exactly two vaults for full synchronization
- **Concurrent Processing**: Parallel secret synchronization (up to 3x faster than sequential)
- **Scheduled Syncing**: Cron-based scheduling for automatic synchronization
- **Manual Execution**: Trigger syncs on-demand via REST API
- **Secret Filtering**: Include/exclude patterns to control which secrets are synced
- **Retry Logic**: Exponential backoff with configurable retry policies
- **Audit Trail**: SQLite-based tracking of all sync operations and history
- **Multiple Auth Methods**: OAuth 2.0, Bearer tokens, Basic auth, API keys, custom headers, mTLS support
- **Customizable Response Parsing**: JSONPath-based extraction for non-standard vault APIs
- **Observable**: Prometheus-compatible metrics endpoint and structured JSON logging
- **Health Checks**: Built-in health check endpoint for container orchestration

## Architecture

```
┌─────────────────────────────────────────────────┐
│         Sync Daemon (Go Binary)                 │
├─────────────────────────────────────────────────┤
│                                                 │
│  ┌──────────────┐        ┌──────────────┐     │
│  │  HTTP API    │        │Sync Scheduler│     │
│  │  - Health    │        │  - Cron Jobs │     │
│  │  - Status    │        │  - Execution │     │
│  │  - Execute   │        │  - Runner    │     │
│  └──────────────┘        └──────────────┘     │
│         │                       │               │
│  ┌──────────────────────────────────────┐     │
│  │     Sync Engine                      │     │
│  │  - Unidirectional                    │     │
│  │  - Bidirectional with conflict res   │     │
│  │  - Retry with backoff                │     │
│  └──────────────────────────────────────┘     │
│         │                       │               │
│         └──────────┬────────────┘               │
│                    │                            │
│  ┌─────────────────────────────────────┐      │
│  │    Vault Clients                    │      │
│  │  - Azure Key Vault                  │      │
│  │  - Vaultwarden                      │      │
│  │  - HashiCorp Vault                  │      │
│  │  - Generic HTTP Vault               │      │
│  └─────────────────────────────────────┘      │
│                    │                            │
└────────────────────┼────────────────────────────┘
                     │
         ┌───────────┼───────────┐
         ↓           ↓           ↓
    ┌────────┐  ┌────────┐  ┌────────┐
    │SQLite  │  │Vault 1 │  │Vault 2 │
    │Database│  │        │  │        │
    └────────┘  └────────┘  └────────┘
```

## Quick Start

### Prerequisites

- Docker & Docker Compose
- Go 1.21+ (for development)
- Environment variables for vault access tokens

### 1. Clone and Setup

```bash
git clone https://github.com/pacorreia/vaults-syncer.git
cd vaults-syncer

# Copy example config and environment
cp config.example.yaml config.yaml
cp .env.example .env
```

### 2. Configure

Edit `config.yaml` to define:
- Vault endpoints and authentication
- Sync relationships (source → targets)
- Schedules and filters

Edit `.env` with your authentication tokens.

### 3. Run with Docker Compose

```bash
# Build and start
docker-compose up -d

# Check status
docker-compose logs -f

# Check health
curl http://localhost:8080/health
```

### 4. Manual Sync

```bash
# List all syncs
curl http://localhost:8080/syncs

# Get sync status
curl http://localhost:8080/syncs/azure_to_vaultwarden/status

# Trigger sync manually
curl -X POST http://localhost:8080/syncs/azure_to_vaultwarden/execute
```

## Configuration

### Configuration File Format (YAML)

The daemon uses a structured YAML configuration format with vault type awareness and flexible authentication options.

#### Configuration Example

```yaml
vaults:
  - id: vaultwarden_prod
    name: "Production Vaultwarden"
    type: vaultwarden              # Vault type (vaultwarden, vault, azure, aws, generic)
    endpoint: "https://vault.example.com/api/ciphers"
    method: POST
    auth:                          # Structured authentication
      method: oauth2               # bearer, basic, api_key, oauth2, custom
      oauth:                       # OAuth 2.0 configuration
        client_id: "${CLIENT_ID}"
        client_secret: "${CLIENT_SECRET}"
        scope: "api"
        token_endpoint: "https://vault.example.com/identity/connect/token"  # Optional (auto-detected)
        extra_params:              # Additional OAuth parameters
          device_identifier: "sync-daemon"
          device_type: "14"
      headers:                     # Additional auth headers (non-OAuth)
        X-Custom-Header: "value"
    operations_override:           # Custom response parsing per operation
      list:
        response:
          path: "data"             # JSONPath to secret list
          name_field: "name"       # Field containing secret name
      get:
        response:
          path: "data"             # JSONPath to secret data
          value_field: "value"     # Field containing secret value
    field_names:
      name_field: "name"
      value_field: "value"
    headers:
      Accept: "application/json"
      Content-Type: "application/json"
    timeout: 30
    skip_ssl_verify: false

  - id: hashicorp_vault
    name: "HashiCorp Vault"
    type: vault                    # HashiCorp Vault KV v2
    endpoint: "https://vault.internal.com/v1/secret/data"
    method: POST
    auth:
      method: bearer
      headers:
        token: "${VAULT_TOKEN}"
    # Custom response parsing for Vault KV v2 structure
    operations_override:
      list:
        response:
          path: "data.keys"        # Vault returns keys in data.keys array
      get:
        response:
          path: "data.data"        # Vault wraps secrets in data.data
          value_field: "value"

syncs:
  - id: vaultwarden_to_vault
    source: vaultwarden_prod
    targets:
      - hashicorp_vault
    sync_type: unidirectional
    enabled: true
    concurrent_workers: 10         # Parallel processing for faster syncs
    schedule: "0 * * * *"          # Hourly
    filter:
      patterns: ["*"]
      exclude: []
    retry_policy:
      max_retries: 3
      initial_backoff: 500
      max_backoff: 5000
      multiplier: 2.0

server:
  port: 8080
  address: 0.0.0.0
  metrics_port: 9090
  metrics_address: 0.0.0.0

logging:
  level: info                      # debug, info, warn, error
  format: json                     # json or text
```
  level: info
  format: json
```

### Vault Type Support

The daemon supports multiple vault types with smart defaults:

| Vault Type | Authentication | Response Format | Notes |
|------------|---------------|-----------------|-------|
| **vaultwarden** | OAuth 2.0, Bearer | `{data: [{name, value}]}` | Full support with device params |
| **vault** | Bearer, AppRole | `{data: {keys: []}}` for list, `{data: {data: {}}}` for get | HashiCorp Vault KV v2 |
| **azure** | Bearer (Azure AD) | `{value: [{id, ...}]}` | Azure Key Vault |
| **aws** | IAM, Bearer | `{SecretList: [{Name}]}` | AWS Secrets Manager |
| **generic** | Any | Configurable via `operations_override` | Custom HTTP APIs |

**Smart Defaults**: When you specify a vault `type`, the daemon automatically configures appropriate:
- OAuth token endpoints
- Response parsing paths
- Field name mappings

### Authentication Methods

#### Bearer Token

```yaml
auth:
  method: bearer
  headers:
    token: "${VAULT_TOKEN}"
```

#### OAuth 2.0 (Vaultwarden, Custom)

```yaml
auth:
  method: oauth2
  oauth:
    client_id: "${CLIENT_ID}"
    client_secret: "${CLIENT_SECRET}"
    scope: "api"
    token_endpoint: "https://vault.example.com/identity/connect/token"  # Optional
    extra_params:                  # Optional device identity
      device_identifier: "sync-daemon"
      device_type: "14"
      device_name: "Sync Daemon"
```

#### Basic Auth

```yaml
auth:
  method: basic
  headers:
    username: "user"
    password: "${PASSWORD}"
```

#### API Key
```yaml
auth:
  method: api_key
  headers:
    api_key: "${API_KEY}"
```

#### Custom Headers
```yaml
auth:
  method: custom
  headers:
    X-Custom-Auth: "${CUSTOM_TOKEN}"
    X-Another-Header: "value"
```

### Response Parsing Customization

For vaults with non-standard response formats, use `operations_override`:

```yaml
vaults:
  - id: custom_vault
    type: generic
    endpoint: "https://custom.vault.com/api"
    method: POST
    auth:
      method: bearer
      headers:
        token: "${TOKEN}"
    operations_override:
      list:                        # Customize list secrets response parsing
        response:
          path: "results.secrets"  # JSONPath to secret array
          name_field: "id"         # Field containing secret identifier
      get:                         # Customize get secret response parsing
        response:
          path: "result.data"      # JSONPath to secret data
          value_field: "secret"    # Field containing secret value
```

**JSONPath Examples**:
- `"data"`: Root-level data field
- `"data.keys"`: Nested keys array
- `"data[].name"`: Array where each item has a name field
- `"response.items"`: Multi-level nesting

### Concurrent Processing

For faster syncs, use the `concurrent_workers` setting:

```yaml
syncs:
  - id: fast_sync
    source: vault_a
    targets: [vault_b]
    concurrent_workers: 10         # Process 10 secrets in parallel
```

**Performance**:
- Sequential (1 worker): ~500 secrets in 90s
- Concurrent (10 workers): ~500 secrets in 30s
- **3x faster** with parallelization!

## API Endpoints

### Health Check
```bash
GET /health
```
Returns daemon health status and configuration summary.

### List All Syncs
```bash
GET /syncs
```
Returns all configured syncs with their settings.

### Get Sync Status
```bash
GET /syncs/{sync_id}/status
```
Returns detailed status including:
- Last run time
- Total synced objects
- Failed objects
- Recent run history

### Execute Sync Now
```bash
POST /syncs/{sync_id}/execute
```
Immediately executes a sync (bypasses schedule).

### Metrics (Prometheus)
```bash
GET /metrics
```
Returns Prometheus-compatible metrics:
- `syncs_configured`: Total configured syncs
- `syncs_enabled`: Currently enabled syncs
- `runner_running`: Runner status (0/1)

## Database Schema

### sync_objects
Tracks managed secrets and their sync status.

| Column | Type | Description |
|--------|------|-------------|
| id | INTEGER | Primary key |
| sync_id | TEXT | Sync configuration ID |
| source_vault_id | TEXT | Source vault ID |
| target_vault_id | TEXT | Target vault ID |
| secret_name | TEXT | Secret name |
| source_checksum | TEXT | MD5 hash of source value |
| target_checksum | TEXT | MD5 hash of target value |
| last_sync_time | INTEGER | Unix timestamp |
| last_sync_status | TEXT | success, failed, in_sync |
| last_sync_error | TEXT | Error message if failed |
| sync_count | INTEGER | Total sync attempts |
| failure_count | INTEGER | Total failures |
| direction_last | TEXT | source_to_target or target_to_source |

### sync_history
Audit trail of all sync operations.

| Column | Type | Description |
|--------|------|-------------|
| id | INTEGER | Primary key |
| sync_object_id | INTEGER | Reference to sync_objects |
| sync_type | TEXT | unidirectional or bidirectional |
| status | TEXT | success or failed |
| error_message | TEXT | Error details |
| duration_ms | INTEGER | Execution time |
| created_at | INTEGER | Unix timestamp |

### syncs_run
High-level sync execution records.

| Column | Type | Description |
|--------|------|-------------|
| id | INTEGER | Primary key |
| sync_id | TEXT | Sync configuration ID |
| status | TEXT | success, partial, failed |
| total_synced | INTEGER | Secrets synced successfully |
| total_failed | INTEGER | Failed syncs |
| duration_ms | INTEGER | Total execution time |
| error_message | TEXT | Overall error if any |
| created_at | INTEGER | Unix timestamp |

## Sync Types

### Unidirectional (1:1 or 1:many)
One-way synchronization from source to target(s).

```yaml
sync_type: unidirectional
source: vault_a
targets:
  - vault_b
  - vault_c
```

**Behavior**:
- Source is the source of truth
- Secrets from source are copied to all targets
- Target-only secrets are left untouched
- Changes in targets are overwritten by source

### Bidirectional (1:1 only)
Two-way synchronization between exactly two vaults.

```yaml
sync_type: bidirectional
source: vault_a
targets:
  - vault_b
```

**Behavior**:
- Both vaults act as sources and targets
- Conflict resolution: Last write wins (based on sync history)
- Secrets present in one vault are replicated to the other
- Checksums are compared to detect changes
- Requires exactly one target (validation error otherwise)

## Building Locally

### Prerequisites
- Go 1.21+
- SQLite development libraries

### Build Binary
```bash
# Download dependencies
go mod download

# Build
CGO_ENABLED=1 go build -o bin/sync-daemon .

# Run
./bin/sync-daemon -config config.yaml -db sync.db
```

### Flags
```
  -config string
        Path to configuration file (default "config.yaml")
  -db string
        Path to SQLite database file (default "sync.db")
  -dry-run
        Validate config and test connections without starting
```

### Development

```bash
# Test configuration without running
./bin/sync-daemon -config config.yaml -db sync.db -dry-run

# Run with debug logging (edit config.yaml)
logging:
  level: debug
  format: json

# Watch logs
docker-compose logs -f sync-daemon
```

## Docker

### Build Image
```bash
docker build -t secrets-sync:latest .
```

### Run Container
```bash
docker run -d \
  -v $(pwd)/config.yaml:/etc/sync/config.yaml:ro \
  -v sync-data:/app/data \
  -p 8080:8080 \
  -p 9090:9090 \
  -e VAULT_TOKEN=your_token \
  secrets-sync:latest
```

### Image Details
- **Base**: Alpine Linux 3.19 (~20MB)
- **User**: Non-root `sync:sync` (uid 1000)
- **Health Check**: Built-in via `/health` endpoint
- **Volumes**: `/etc/sync` for config, `/app/data` for database

## Performance Considerations

- **Binary Size**: ~15MB (statically compiled)
- **Memory**: ~50-100MB under normal operation
- **DB Performance**: SQLite suitable for most workloads; migration to PostgreSQL possible
- **Concurrency**: Goroutines for parallel target syncing in 1:many scenarios
- **Network**: Timeout of 30 seconds per vault operation (configurable)

## Security Best Practices

1. **Use environment variables** for sensitive values (auth tokens)
2. **Never commit** actual `config.yaml` with secrets
3. **Use read-only** volume for config file
4. **Enable SSL verification** in production (`skip_ssl_verify: false`)
5. **Rotate tokens** regularly
6. **Use network policies** to restrict access to vaults
7. **Monitor logs** for unauthorized access attempts
8. **Encrypt database** with SQLite encryption for sensitive deployments

## Troubleshooting

### Connection Errors
```bash
# Test with dry-run mode
./bin/sync-daemon -dry-run

# Check logs for specific vault errors
docker-compose logs sync-daemon | grep -i error
```

### Sync Status Check
```bash
curl http://localhost:8080/syncs/sync_id/status | jq

# View database directly
sqlite3 sync.db "SELECT * FROM sync_objects WHERE sync_id='sync_id';"
```

### Backoff/Retry Delays
Increase max_retries and adjust backoff multiplier in config:
```yaml
retry_policy:
  max_retries: 5
  initial_backoff: 2000
  max_backoff: 120000
  multiplier: 1.5
```

## Monitoring & Observability

### Metrics
Prometheus metrics available at `http://localhost:9090/metrics`:
```
syncs_configured 3
syncs_enabled 2
runner_running 1
```

### Logging
Structured JSON logs to stdout:
```json
{
  "time": "2024-02-20T10:30:00Z",
  "level": "INFO",
  "msg": "sync completed",
  "sync_id": "azure_to_vaultwarden",
  "succeeded": 42,
  "failed": 2,
  "duration_ms": 5234
}
```

### Database Audit
Query sync history:
```sql
-- Recent sync runs
SELECT sync_id, status, total_synced, total_failed, created_at 
FROM syncs_run 
ORDER BY created_at DESC 
LIMIT 20;

-- Failed syncs with errors
SELECT sync_id, secret_name, last_sync_error, last_sync_time 
FROM sync_objects 
WHERE last_sync_status = 'failed' 
ORDER BY last_sync_time DESC;
```

## Contributing

Contributions welcome! Areas for enhancement:
- Additional vault backends (Kubernetes Secrets, AWS Secrets Manager, etc.)
- Enhanced filtering/transformation capabilities
- Web UI for configuration management
- Webhook-based triggers
- Multi-tenant support

## License

MIT

## Support

For issues and questions:
1. Check the [troubleshooting section](#troubleshooting)
2. Review logs: `docker-compose logs sync-daemon`
3. Open an issue with configuration and logs
