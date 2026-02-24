# Quick Start Guide - Integration Test

## One Command Test

```bash
./e2e/test-integration.sh
```

That's it! This will:
1. Build the Docker image (if needed)
2. Start 2 Vaultwarden instances + PostgreSQL + sync daemon
3. Inject 3 test secrets into the source
4. Verify they sync to the target
5. Show final status

**Time to complete**: ~3-5 minutes (first run), ~2 minutes (subsequent runs)

## What Happens

```text
START
  ↓
[Docker] Start PostgreSQL
  ↓
[Docker] Start Vaultwarden Source (port 8000)
  ↓
[Docker] Start Vaultwarden Target (port 8001)
  ↓
[Docker] Start Sync Daemon (ports 8080, 9090)
  ↓
[Script] Inject 3 test secrets: db-password, api-key, aws-access-key
  ↓
[Script] Trigger sync manually
  ↓
[Daemon] Reads secrets from source (http://vaultwarden-source)
  ↓
[Daemon] Writes secrets to target (http://vaultwarden-target)
  ↓
[Script] Verify secrets appear in target
  ↓
DONE ✓
```

## Test Results

You should see:

```text
[SUCCESS] Source Vaultwarden is ready
[SUCCESS] Target Vaultwarden is ready
[SUCCESS] Sync daemon is ready
[SUCCESS] Created secret: db-password
[SUCCESS] Created secret: api-key
[SUCCESS] Created secret: aws-access-key
[SUCCESS] Secrets synced! Source: 3, Target: 3
[SUCCESS] Integration test completed successfully!
```

## After Test - What You Can Do

### View Vaultwarden UIs
- Source: http://localhost:8000
- Target: http://localhost:8001

(No password required for test setup)

### Check Sync Status
```bash
curl http://localhost:8080/syncs/vaultwarden_sync/status | jq
```

### View Metrics
```bash
curl http://localhost:9090/metrics
```

### Query Database
```bash
docker exec -it secrets-sync-daemon sqlite3 /app/data/sync.db
sqlite> .tables
sqlite> SELECT * FROM sync_objects;
```

### View Daemon Logs
```bash
docker logs secrets-sync-daemon
```

## Cleanup

```bash
# Stop services
docker-compose -f e2e/docker-compose.test.yml down

# Complete cleanup (remove volumes)
docker-compose -f e2e/docker-compose.test.yml down -v
```

## Troubleshooting

### Test Hangs
```bash
# Check if ports are in use
lsof -i :8000 :8001 :8080 :9090 :5432

# Or manually check services
curl http://localhost:8000/alive
curl http://localhost:8001/alive
curl http://localhost:8080/health
```

### Build Fails
```bash
# Force rebuild
docker-compose -f e2e/docker-compose.test.yml build --no-cache

# Then run test
./e2e/test-integration.sh
```

### Need More Info
See [TESTING.md](TESTING.md) for detailed documentation.

## Files Used

| File | Purpose |
|------|---------|
| `e2e/docker-compose.test.yml` | Services definition |
| `e2e/config.test.yaml` | Sync daemon configuration |
| `e2e/test-integration.sh` | Test automation script |
| `.env.test` | Test credentials |

---

**Pro Tip**: Run test periodically to ensure everything still works:
```bash
# In a cron job
0 2 * * * cd /path/to/repo && ./e2e/test-integration.sh > /tmp/test.log 2>&1
```
