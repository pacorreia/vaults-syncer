# Development Guide

## Project Overview

This is a containerized secrets vault synchronization daemon written in Go. It enables syncing secrets between multiple vault backends including Azure Key Vault, Vaultwarden, HashiCorp Vault, and generic HTTP-based APIs.

## Project Structure

```
akv-vaultwarden-sync/
├── .dockerignore              # Docker build exclusions
├── .env.example               # Environment variable template
├── .gitignore                 # Git exclusions
├── Dockerfile                 # Multi-stage Docker build
├── Makefile                   # Build tasks (optional)
├── README.md                  # Main documentation
├── DEVELOPMENT.md             # This file
├── config.example.yaml        # Example configuration
├── docker-compose.yml         # Docker Compose setup
├── go.mod                     # Go module definition
├── go.sum                     # Go dependency checksums
├── main.go                    # Application entry point
│
├── api/                       # REST API handlers
│   └── handler.go            # HTTP endpoint handlers
│
├── config/                    # Configuration management
│   ├── loader.go             # YAML config loading & validation
│   └── types.go              # Configuration data structures
│
├── sync/                      # Core sync logic
│   ├── engine.go             # Main sync engine
│   └── runner.go             # Scheduler and execution runner
│
├── storage/                   # Persistence layer
│   └── store.go              # SQLite database operations
│
├── vault/                     # Vault client abstraction
│   └── client.go             # HTTP vault client
│
└── bin/
    └── sync-daemon           # Compiled binary (after build)
```

## Building the Project

### Local Development

```bash
# Install dependencies
go mod download

# Build binary
CGO_ENABLED=1 go build -o bin/sync-daemon .

# Build with version
CGO_ENABLED=1 go build -ldflags "-X main.Version=1.0.0" -o bin/sync-daemon .

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

The `config.example.yaml` includes examples for:

1. **Azure Key Vault**
   - Bearer token authentication
   - REST API endpoints

2. **Vaultwarden**
   - Bitwarden-compatible vault
   - Bearer token auth

3. **HashiCorp Vault**
   - Generic vault platform
   - Bearer token auth

4. **Custom HTTP API**
   - API key authentication
   - Flexible field mapping

### Creating a Production Config

1. Copy the example: `cp config.example.yaml config.yaml`
2. Define your vaults with actual endpoints
3. Set authentication via environment variables (recommended)
4. Define sync relationships
5. Validate with dry-run: `./bin/sync-daemon -config config.yaml -dry-run`

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
| Bearer Token | Authorization header | APIs, Azure, Vault |
| Basic Auth | Base64 encoded credentials | Legacy systems |
| API Key | Custom header (X-API-Key) | Third-party APIs |
| Custom | Arbitrary headers | Proprietary systems |

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
./bin/sync-daemon -config config.yaml -dry-run
```
- Validates configuration
- Tests connections to all vaults
- Initializes database schema
- Exits without starting scheduler

### Manual Sync Execution
```bash
curl -X POST http://localhost:8080/syncs/sync_id/execute
```
- Triggers immediate sync
- Bypasses cron schedule
- Useful for testing changes

### Check Sync Status
```bash
curl http://localhost:8080/syncs/sync_id/status | jq
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
GET /health
```
Returns:
- Status (healthy/unhealthy)
- Runner state (enabled/disabled)
- Configuration summary

### Metrics
```bash
GET /metrics
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

## Future Enhancements

### Vault Backends
- AWS Secrets Manager
- Google Secret Manager
- Kubernetes Secrets
- CyberArk
- Thycotic Secret Server

### Features
- Webhook-based triggers for event-driven sync
- Secret transformation/encryption during transit
- Field-level filtering and mapping
- Backup/snapshot functionality
- Multi-tenant support

### Operations
- Web UI for configuration
- Metrics dashboard (Grafana integration)
- Advanced search in audit trail
- Scheduled reports
- API versioning and stability

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

**Error**: Configuration validation failed
- **Solution**: Run dry-run mode for detailed errors
  ```bash
  ./bin/sync-daemon -config config.yaml -dry-run
  ```

**Error**: Connection to vault failed
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

For more information, see [README.md](README.md)
