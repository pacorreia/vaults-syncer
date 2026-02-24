# Integration Testing Guide

This guide explains how to run a complete end-to-end integration test of the Secrets Vault Sync Daemon with two Vaultwarden instances.

## What's Tested

The integration test verifies:
- ✅ Docker Compose orchestration with multiple services
- ✅ Two Vaultwarden instances (source and target)
- ✅ Sync daemon startup and health checks
- ✅ Secret injection into source vault
- ✅ Automatic synchronization to target vault
- ✅ REST API endpoints (health, status, execute)
- ✅ Database persistence and audit trail

## Test Architecture

```mermaid
graph TB
    subgraph network["Docker Compose Network"]
        direction TB
        
        postgres[("PostgreSQL<br/>(vaultwarden_db)")]
        
        daemon["Sync Daemon<br/>- Health: 8080<br/>- Metrics: 9090"]
        
        postgres -.-> daemon
        
        source["Vaultwarden<br/>(Source)<br/>Port: 8000<br/>UI + API"]
        target["Vaultwarden<br/>(Target)<br/>Port: 8001<br/>UI + API"]
        
        postgres --> source
        postgres --> target
        
        daemon -->|Secrets Synced| source
        daemon -->|Secrets Synced| target
    end
    
    style network fill:none,stroke:#4a9eff,stroke-width:3px
    style daemon fill:none,stroke:#4a9eff,stroke-width:2px
    style source fill:none,stroke:#ff6b35,stroke-width:2px
    style target fill:none,stroke:#00cc66,stroke-width:2px
```

## Prerequisites

- Docker and Docker Compose installed
- `curl` and `jq` (for the test script)
- At least 3GB free RAM (PostgreSQL + 2 Vaultwarden instances + daemon)
- Ports available: 5432, 8000, 8001, 8080, 9090

## Quick Start

### 1. Build the Docker Image

```bash
docker build -t secrets-sync:latest .
```

This creates the sync daemon image used in the test.

### 2. Run the Integration Test

```bash
./e2e/test-integration.sh
```

The script will:
1. Start all Docker services
2. Wait for services to be healthy
3. Inject test secrets into source Vaultwarden
4. Trigger a manual sync
5. Verify secrets appeared in target Vaultwarden
6. Show final status

### 3. Example Output

```text
[INFO] ==========================================
[INFO] Secrets Vault Sync Daemon - Integration Test
[INFO] ==========================================

[INFO] Starting Docker Compose services...
[INFO] Waiting for all services to be ready...
[INFO] Waiting for Source Vaultwarden to be healthy...
[SUCCESS] Source Vaultwarden is ready

[INFO] Waiting for Target Vaultwarden to be healthy...
[SUCCESS] Target Vaultwarden is ready

[INFO] Waiting for sync daemon to be healthy...
[SUCCESS] Sync daemon is ready

[INFO] Initial state of vaults:
[INFO] Source vault: 0 items
[INFO] Target vault: 0 items

[INFO] Injecting test secrets into source Vaultwarden...
[INFO] Creating secret: db-password
[SUCCESS] Created secret: db-password
[INFO] Creating secret: api-key
[SUCCESS] Created secret: api-key
[INFO] Creating secret: aws-access-key
[SUCCESS] Created secret: aws-access-key

[INFO] Verifying secrets in source vault...
[INFO] Source vault after injection: 3 items

[INFO] Triggering manual sync...
{
  "sync_id": "vaultwarden_sync",
  "status": "executing"
}

[INFO] Verifying secrets were synced to target vault...
[INFO] Checking target vault (attempt 1)...
[INFO] Source vault has 3 secrets, Target vault has 3 secrets
[SUCCESS] Secrets synced! Source: 3, Target: 3

[SUCCESS] Integration test completed successfully!
```

## Manual Testing

### Start Services Without Test Script

```bash
docker-compose -f e2e/docker-compose.test.yml up -d
```

### Wait for Services to Be Ready

```bash
# Check Vaultwarden Source
curl http://localhost:8000/alive

# Check Vaultwarden Target
curl http://localhost:8001/alive

# Check Sync Daemon
curl http://localhost:8080/health
```

### Access Vaultwarden Web UIs

- **Source**: http://localhost:8000
- **Target**: http://localhost:8001

### Add Secrets Manually via Web UI

1. Go to http://localhost:8000
2. Create a new vault item (Login)
3. Enter credentials as a test secret

### Manually Trigger Sync

```bash
curl -X POST http://localhost:8080/syncs/vaultwarden_sync/execute
```

### Check Sync Status

```bash
curl http://localhost:8080/syncs/vaultwarden_sync/status | jq
```

Example response:
```json
{
  "sync_id": "vaultwarden_sync",
  "last_run": {
    "id": 1,
    "sync_id": "vaultwarden_sync",
    "status": "success",
    "total_synced": 3,
    "total_failed": 0,
    "duration_ms": 1234,
    "error_message": null,
    "created_at": 1708444234
  },
  "total_objects": 3,
  "synced_objects": 3,
  "failed_objects": 0,
  "recent_runs": [
    {
      "id": 1,
      "sync_id": "vaultwarden_sync",
      "status": "success",
      "total_synced": 3,
      "total_failed": 0,
      "duration_ms": 1234,
      "error_message": null,
      "created_at": 1708444234
    }
  ]
}
```

### View Metrics

```bash
curl http://localhost:9090/metrics
```

### View Daemon Logs

```bash
docker-compose -f e2e/docker-compose.test.yml logs -f sync-daemon
```

### Access SQLite Database

```bash
docker exec -it secrets-sync-daemon sqlite3 /app/data/sync.db

# View sync objects
sqlite> SELECT sync_id, secret_name, last_sync_status, direction_last FROM sync_objects;

# View sync runs
sqlite> SELECT * FROM syncs_run ORDER BY created_at DESC LIMIT 5;
```

## Test Scenarios

### Scenario 1: Basic One-Way Sync

**Steps**:
1. Run test script
2. Secrets are created in source
3. Sync daemon syncs them to target
4. Verify secrets appear in target

**Expected Result**: All secrets from source appear in target

### Scenario 2: Incremental Updates

**Steps**:
1. Create 3 secrets in source
2. Run sync (should sync 3)
3. Add 2 more secrets in source
4. Run sync again (should sync 5 total)

**Expected Result**: New secrets synced, existing ones unchanged

### Scenario 3: Failed Secrets Recovery

**Steps**:
1. Create secrets in source
2. Temporarily stop target service
3. Run sync (will fail)
4. Restart target service
5. Run sync again

**Expected Result**: Sync retries and succeeds on second attempt

### Scenario 4: Filtered Sync

**Edit** `e2e/config.test.yaml`:
```yaml
filter:
  patterns:
    - "db-*"  # Only sync secrets starting with db-
```

**Steps**:
1. Create secrets: db-password, api-key, aws-token
2. Run sync

**Expected Result**: Only db-password synced, others ignored

## Cleanup

### Stop All Services

```bash
docker-compose -f e2e/docker-compose.test.yml down
```

### Remove Volumes (Clean Slate)

```bash
docker-compose -f e2e/docker-compose.test.yml down -v
```

### Remove All Docker Resources

```bash
docker-compose -f e2e/docker-compose.test.yml down -v
docker rmi secrets-sync:latest
```

## Troubleshooting

### Services Won't Start

```bash
# Check Docker Compose logs
docker-compose -f e2e/docker-compose.test.yml logs

# Verify ports are available
netstat -tuln | grep -E ":(5432|8000|8001|8080|9090)"
```

### Vaultwarden Not Healthy

```bash
# Check Vaultwarden logs
docker logs vaultwarden-source
docker logs vaultwarden-target

# Verify database connection
docker logs vaultwarden-postgres
```

### Secrets Not Syncing

```bash
# Check daemon logs
docker logs secrets-sync-daemon

# Verify sync status
curl http://localhost:8080/syncs/vaultwarden_sync/status | jq

# Check database
docker exec -it secrets-sync-daemon sqlite3 /app/data/sync.db \
  "SELECT * FROM syncs_run ORDER BY created_at DESC LIMIT 1;"
```

### High Memory Usage

```bash
# Reduce max concurrent operations in config
retry_policy:
  max_retries: 1
  initial_backoff: 500
  max_backoff: 10000

# Or stop unnecessary services
docker-compose -f e2e/docker-compose.test.yml stop vaultwarden-target
```

### "Connection refused" Errors

```bash
# Ensure services have started
sleep 30 && ./e2e/test-integration.sh

# Or manually verify health
curl -v http://localhost:8000/alive
curl -v http://localhost:8001/alive
curl -v http://localhost:8080/health
```

## Environment Variables

### Vaultwarden Configuration

Set in `e2e/docker-compose.test.yml`:
- `ADMIN_TOKEN`: Admin authentication token
- `SIGNUPS_ALLOWED`: Allow user registration
- `LOG_LEVEL`: vaultwarden log level

### Sync Daemon Configuration

Set in environment or `.env.test`:
- `VAULTWARDEN_SOURCE_TOKEN`: Source vault admin token
- `VAULTWARDEN_TARGET_TOKEN`: Target vault admin token
- `CONFIG_PATH`: Configuration file location
- `DATABASE_PATH`: SQLite database location

## Advanced Testing

### Performance Testing

Measure sync performance with large secret counts:

```bash
# Inject 100 secrets
for i in {1..100}; do
  curl -X POST http://localhost:8000/api/ciphers \
    -H "Authorization: Bearer source_admin_token_12345" \
    -H "Content-Type: application/json" \
    -d "{\"type\":1,\"name\":\"secret-$i\",\"login\":{\"username\":\"user\",\"password\":\"pass\"}}"
done

# Measure sync time
time curl -X POST http://localhost:8080/syncs/vaultwarden_sync/execute

# Check status
curl http://localhost:8080/syncs/vaultwarden_sync/status | jq '.last_run.duration_ms'
```

### Network Failure Simulation

```bash
# Simulate network issues with tc (traffic control)
docker exec vaultwarden-target tc qdisc add dev eth0 root netem loss 10%

# Run sync (will retry and eventually succeed)
curl -X POST http://localhost:8080/syncs/vaultwarden_sync/execute

# Check retry behavior in logs
docker logs secrets-sync-daemon | grep retry
```

### Config Validation

```bash
# Test config without running daemon
./bin/sync-daemon -config e2e/config.test.yaml -dry-run

# Should output connection test results
```

## CI/CD Integration

Example GitHub Actions workflow:

```yaml
name: Integration Tests
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
      - name: Build image
        run: docker build -t secrets-sync:test .
      - name: Run integration tests
        run: ./e2e/test-integration.sh
      - name: Upload logs on failure
        if: failure()
        uses: actions/upload-artifact@v2
        with:
          name: docker-logs
          path: /tmp/docker-logs
```

## Notes

- Test uses demo tokens (not production-safe)
- Vaultwarden instances share PostgreSQL database (consider separate instances for prod tests)
- Sync runs every 5 minutes by default (configurable in `e2e/config.test.yaml`)
- Database persistence survives container restarts
- Integration test is non-destructive (can be run multiple times)

## Next Steps

- Adapt `e2e/config.test.yaml` to your vault backends
- Modify `e2e/test-integration.sh` for your testing scenarios
- Create production Docker Compose with your actual vaults
- Set up CI/CD pipeline using `e2e/test-integration.sh`
- Monitor sync daemon with your observability stack
