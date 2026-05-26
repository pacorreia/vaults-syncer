# Development Guide

## Project Overview

This is a containerized secrets vault synchronization daemon written in Go. It enables syncing secrets between multiple vault backends including Azure Key Vault, Bitwarden, Vaultwarden, HashiCorp Vault, AWS Secrets Manager (via CLI tool backend), Keeper Secrets Manager, and generic HTTP-based APIs.

## Project Structure

```
vaults-syncer/
├── .dockerignore              # Docker build exclusions
├── .gitignore                 # Git exclusions
├── Dockerfile                 # Multi-stage Docker build
├── README.md                  # Main documentation
├── examples/                  # Example configurations
│   ├── config.example.yaml    # Example configuration
│   └── tools/                 # Tool backend config examples
├── docker-compose.yml         # Docker Compose setup
├── go.mod                     # Go module definition
├── go.sum                     # Go dependency checksums
├── main.go                    # Application entry point
│
├── api/                       # REST API handlers
│   ├── handler.go            # Runtime HTTP endpoint handlers
│   ├── handler_auth.go       # Auth handlers (login/logout/me)
│   ├── handler_config.go     # Admin config CRUD handlers
│   ├── handler_setup.go      # First-run setup handler
│   ├── handler_users.go      # User management handlers
│   └── ui.go                 # Embedded Web UI serving
│
├── auth/                      # Authentication middleware
│
├── config/                    # Configuration management
│   ├── loader.go             # YAML config loading & validation
│   ├── defaults.go           # Default value application
│   ├── types.go              # Configuration data structures
│   └── validate.go           # Configuration validation
│
├── security/                  # Encryption utilities
│
├── sync/                      # Core sync logic
│   ├── engine.go             # Main sync engine
│   └── runner.go             # Scheduler and execution runner
│
├── storage/                   # Persistence layer
│   ├── db.go                 # Database connection (SQLite/PostgreSQL/MSSQL)
│   ├── store.go              # Core sync state storage
│   ├── config_store.go       # Vault/sync config persistence
│   ├── settings_store.go     # Key-value settings storage
│   └── user_store.go         # User management
│
└── vault/                     # Vault client abstraction
    ├── backend.go            # Backend interface and GenericBackend
    ├── client.go             # HTTP vault client
    ├── parser.go             # Response parsing
    └── tool_backend.go       # CLI-backed vault backend
```

## Building the Project

### Local Development

```bash
# Install dependencies
go mod download

# Build binary (CGO_ENABLED=1 required for sqlite3)
CGO_ENABLED=1 go build -o bin/sync-daemon .

# Build with complete version information
VERSION=$(git describe --tags --always --dirty)
BUILD_DATE=$(date -u +'%Y-%m-%dT%H:%M:%SZ')
GIT_COMMIT=$(git rev-parse HEAD)

CGO_ENABLED=1 go build \
  -ldflags "-X main.Version=${VERSION} -X main.BuildDate=${BUILD_DATE} -X main.GitCommit=${GIT_COMMIT}" \
  -o bin/sync-daemon .

# Verify binary
./bin/sync-daemon --version
./bin/sync-daemon -h
```

### Docker Build

```bash
# Build production image
docker build -t secrets-sync:latest .

# Build with custom tag
docker build -t myregistry.azurecr.io/secrets-sync:v1.0.0 .
```

## Configuration

### Example Vault Definitions

The `examples/config.example.yaml` includes examples for:

1. **Azure Key Vault** (type `azure`)
   - Bearer token authentication
   - REST API endpoints

2. **Vaultwarden** (type `vaultwarden`)
   - OAuth2 authentication with device parameters
   - Self-hosted Bitwarden-compatible API

3. **HashiCorp Vault** (type `vault`)
   - Custom header authentication (X-Vault-Token)
   - KV v2 API

4. **AWS Secrets Manager** (type `tool`)
   - CLI-backed via the AWS CLI
   - No HTTP credentials needed in config

5. **Generic REST API** (type `generic`)
   - API key authentication
   - Flexible field mapping

### Creating a Production Config

1. Configure vaults via the Web UI at `http://localhost:8080`
2. Alternatively, use the admin API: `POST /api/config/vaults`
3. Validate by triggering a sync: `POST /api/syncs/{id}/execute`

## Architecture Decisions

### Language: Go

**Benefits:**
- Single compiled binary (easy containerization)
- Excellent concurrency (goroutines for parallel syncs)
- Strong standard library (http, sql, crypto)
- Fast startup and minimal resource footprint
- Great for microservices and daemons

### Database: SQLite

**Benefits:**
- Zero configuration, file-based
- Suitable for single-instance deployments
- Full SQL support
- Built-in schema versioning via migrations
- Can migrate to PostgreSQL if needed

**Why not alternatives:**
- PostgreSQL: Would add operational complexity
- In-memory: Would lose sync history on restart
- MongoDB/NoSQL: Overkill for relational audit data

### HTTP Framework: Standard Library

**Benefits:**
- No external dependencies needed
- Proven in production
- Small binary size
- Built-in multiplexing and routing
- Full control over implementation

### Scheduling: robfig/cron

**Benefits:**
- Widely-used, battle-tested
- Supports standard cron expressions
- Integrates seamlessly with Go
- Minimal dependencies

## Key Design Patterns

### 1. Strategy Pattern (Sync Types)
- Unidirectional and bidirectional syncs as different strategies
- Engine switches based on configuration

### 2. Client Pattern (Vaults)
- Abstraction layer for different vault types
- Extensible for new vault backends
- Consistent interface: GetSecret, SetSecret, ListSecrets

### 3. Retry Pattern (Transient Failures)
- Exponential backoff with configurable multiplier
- Max backoff cap to prevent excessive delays
- Automatic retry for network errors

### 4. Audit Trail Pattern (Tracking)
- SQLite for persistent audit records
- Checksum comparison for change detection
- Bi-directional sync conflict resolution

## Sync Algorithm

### Unidirectional Sync
```
1. Get all secrets from source
2. Apply filters (patterns/exclusions)
3. For each secret:
   a. Retrieve secret value from source
   b. For each target:
      i. Set secret in target
      ii. Record in database
      iii. Retry on failure with backoff
```

### Bidirectional Sync
```
1. Get all secrets from source
2. Apply filters
3. For each secret:
   a. Get source value
   b. Get target value
   c. If values match by checksum:
      - Mark as in_sync, done
   d. If mismatch:
      - Determine direction using sync history (last write wins)
      - Sync in that direction
      - Record the direction
```

## Authentication Support

| Method | Implementation | Use Case |
|--------|-----------------|----------|
| Bearer Token | `Authorization: Bearer <token>` header | APIs, Azure AD, generic tokens |
| Basic Auth | Base64 encoded `username:password` | Legacy systems |
| OAuth 2.0 | Client credentials flow, automatic token refresh | Bitwarden, Vaultwarden, Azure SP |
| API Key | Custom header (e.g. `api_key`, `X-API-Key`) | Third-party APIs |
| Custom | Arbitrary headers (e.g. `X-Vault-Token`) | HashiCorp Vault, proprietary systems |

## Error Handling

### Transient Errors
- Network timeouts
- Temporary API unavailability
- Rate limiting (429 status)

**Handling**: Automatic retry with exponential backoff

### Permanent Errors
- Invalid credentials
- Secret not found
- Unsupported field mappings
- Configuration errors

**Handling**: Logged, recorded in database, sync marked as failed

### Recovery
- Database persistence means no data loss on restart
- Syncs resume from last known state
- Manual re-execution via API possible

## Testing Configuration

### Dry-Run Mode
```bash
./bin/sync-daemon -dry-run
```
- Validates database connection
- Exits without starting the scheduler

### Manual Sync Execution
```bash
# Requires authentication (Bearer token)
TOKEN=$(curl -s -X POST http://localhost:8080/api/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"pass"}' | jq -r .token)
curl -X POST -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/syncs/sync_id/execute
```
- Triggers immediate sync
- Bypasses cron schedule
- Useful for testing changes

### Check Sync Status
```bash
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/syncs/sync_id/status
```
- View last run results
- Inspect failed objects
- Monitor sync trends

## Performance Tuning

### Parallel Syncing
- Syncs to multiple targets run in parallel (goroutines)
- Each target processes independently
- No shared state between targets

### Retry Tuning
```yaml
retry_policy:
  max_retries: 3
  initial_backoff: 1000      # 1 second
  max_backoff: 60000         # 60 seconds
  multiplier: 2.0
```

Adjust based on:
- Network latency
- API response times
- Frequency of transient errors

### Schedule Optimization
```yaml
schedule: "0 */6 * * *"    # Every 6 hours
```

Consider:
- Frequency vs. freshness needs
- API rate limits
- Database size
- Network utilization

## Monitoring

### Health Endpoint
```bash
GET /api/health  (requires auth)
```
Returns:
- Status (healthy/unhealthy)
- Runner state (enabled/disabled)
- Configuration summary

### Metrics
```bash
GET /metrics  (port 9090, no auth)
```
Prometheus-compatible format:
- `syncs_configured`
- `syncs_enabled`
- `runner_running`

### Logs
- JSON structured logs to stdout
- Easily parsed by log aggregators (ELK, Splunk, etc.)
- Supports debug level for troubleshooting

### Database Audit
```sql
-- Recent sync runs
SELECT * FROM syncs_run ORDER BY created_at DESC LIMIT 10;

-- Failed syncs
SELECT * FROM sync_objects WHERE last_sync_status = 'failed';

-- Sync history for a secret
SELECT * FROM sync_history 
WHERE sync_object_id = ? 
ORDER BY created_at DESC;
```

## Security Considerations

### Secrets Management
- Credentials stored in environment variables (not config)
- Config file can be version controlled after removing secrets
- SQLite unencrypted by default (consider encryption layer)

### Access Control
- No built-in authentication (use reverse proxy)
- Network policies restrict vault access
- Consider mTLS for vault communication

### Audit Trail
- All sync operations logged in database
- Immutable once written
- Helps detect unauthorized changes

## Possible Future Enhancements

### Vault Backends
- Google Secret Manager native adapter
- Kubernetes Secrets native adapter
- CyberArk PAM integration

### Features
- Webhook-based triggers for event-driven sync
- Field-level filtering and value mapping
- Backup/snapshot functionality
- Multi-tenant support

### Operations
- Advanced search in audit trail
- Scheduled reports
- API versioning and stability guarantees

## Troubleshooting

### Build Issues

**Error**: `gcc: command not found`
- **Solution**: Install build tools
  ```bash
  apt-get install build-essential sqlite3 libsqlite3-dev  # Ubuntu/Debian
  brew install gcc sqlite                                  # macOS
  ```

**Error**: `CGO_ENABLED=1 not set`
- **Solution**: Use `CGO_ENABLED=1` when building

### Runtime Issues

**Error**: `Configuration validation failed`
- **Solution**: Run dry-run mode for detailed errors
  ```bash
  ./bin/sync-daemon -dry-run
  ```

**Error**: `Connection to vault failed`
- **Solution**: Check endpoint, verify token expiration, test endpoint
  ```bash
  curl -H "Authorization: Bearer $TOKEN" $ENDPOINT
  ```

**Error**: Database locked
- **Solution**: Ensure only one instance running, check file permissions

## Contributing

Code style:
- Follow Go conventions
- Use meaningful variable names
- Add comments for exported functions
- Keep functions focused and small

Testing:
- Add tests for new sync algorithms
- Test configuration validation
- Test error handling

Documentation:
- Update README for new features
- Add examples for new vault types
- Document breaking changes

---

For more information, see the [docs home](../index.md).
