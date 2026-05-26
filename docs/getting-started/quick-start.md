# Quick Start Guide

Get vaults-syncer up and running in 5 minutes.

## 1. Run with Docker

```bash
docker run -d \
  --name vaults-syncer \
  -v sync-data:/app/data \
  -p 8080:8080 \
  -p 9090:9090 \
  ghcr.io/pacorreia/vaults-syncer:latest
```

Or with Docker Compose (see `docker-compose.yml` in the repository):

```bash
docker compose up -d
```

## 2. Save the Encryption Key

On the **first start**, if `MASTER_ENCRYPTION_KEY` is not set, a key is generated and printed to the container logs:

```bash
docker logs vaults-syncer
```

Look for the printed key:

```
╔══════════════════════════════════════════════════════════════╗
║              MASTER ENCRYPTION KEY – SAVE THIS NOW          ║
╠══════════════════════════════════════════════════════════════╣
║  <your-generated-key>                                        ║
╚══════════════════════════════════════════════════════════════╝
```

Set it as an environment variable before the next restart:

```bash
export MASTER_ENCRYPTION_KEY=<printed-value>
```

Losing this key means losing access to the encrypted vault credentials stored in the database.

## 3. Complete the Setup Wizard

Open `http://localhost:8080` in your browser. The **Setup Wizard** will prompt you to create an admin username and password. After setup, log in with those credentials.

## 4. Add Vaults and Syncs

Use the Web UI or the admin API to configure vaults and syncs.

### Via Web UI

1. Navigate to **Vaults Config** and add your source and target vaults.
2. Navigate to **Syncs Config** to define a sync between them.
3. Return to the **Dashboard** to trigger syncs and view run history.

### Via API

Get an auth token first:

```bash
TOKEN=$(curl -s -X POST http://localhost:8080/api/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"your-password"}' | jq -r .token)
```

Create vaults:

```bash
# Source vault — Azure Key Vault
curl -s -X POST http://localhost:8080/api/config/vaults \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "id": "azure-prod",
    "type": "azure",
    "endpoint": "https://myvault.vault.azure.net/secrets",
    "auth": {
      "method": "bearer",
      "headers": {"token": "'"${AZURE_ACCESS_TOKEN}"'"}
    },
    "field_names": {"name_field": "name", "value_field": "value"}
  }'

# Target vault — Vaultwarden
curl -s -X POST http://localhost:8080/api/config/vaults \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "id": "vaultwarden-prod",
    "type": "vaultwarden",
    "endpoint": "https://vault.example.com/api/ciphers",
    "method": "POST",
    "auth": {
      "method": "oauth2",
      "oauth": {
        "client_id": "'"${VW_CLIENT_ID}"'",
        "client_secret": "'"${VW_CLIENT_SECRET}"'",
        "scope": "api"
      }
    },
    "field_names": {"name_field": "name", "value_field": "login"}
  }'
```

Create a sync:

```bash
curl -s -X POST http://localhost:8080/api/config/syncs \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "id": "my-first-sync",
    "source": "azure-prod",
    "targets": ["vaultwarden-prod"],
    "schedule": "0 * * * *",
    "sync_type": "unidirectional",
    "enabled": true
  }'
```

## 5. Monitor Your Sync

```bash
# Check sync status
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/syncs/my-first-sync/status

# Trigger an immediate sync
curl -X POST -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/syncs/my-first-sync/execute

# View run history
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/syncs/my-first-sync/runs

# View Prometheus metrics (no auth, port 9090)
curl http://localhost:9090/metrics | grep sync
```

## 6. Check Logs

```bash
# Docker
docker logs vaults-syncer -f

# Docker Compose
docker compose logs -f sync-daemon
```

## Best Practices

### Security

1. ✅ Use managed identities or service principals; never hardcode credentials
2. ✅ Use read-only credentials for source vaults when possible
3. ✅ Enable TLS for all vault endpoints
4. ✅ Rotate credentials regularly

### Reliability

1. ✅ Test syncs in non-production first
2. ✅ Start with unidirectional syncs before using bidirectional
3. ✅ Monitor and alert on sync failures
4. ✅ Set appropriate sync intervals (avoid syncing too frequently)

### Operations

1. ✅ Use containerization (Docker/Kubernetes)
2. ✅ Implement health checks in your orchestrator
3. ✅ Monitor metrics and logs
4. ✅ Document your sync topology

## Troubleshooting

### Sync not running

1. Check logs: `docker logs vaults-syncer`
2. Verify schedule syntax is valid cron format: `0 */4 * * *`
3. Ensure vaults are reachable from the daemon host
4. Verify credentials and authentication

### Authentication failures

Test your vault credentials directly from the daemon host:

```bash
# Test Azure bearer token
curl -H "Authorization: Bearer $AZURE_ACCESS_TOKEN" \
  "https://myvault.vault.azure.net/secrets?api-version=7.4"

# Test Vaultwarden OAuth
curl -X POST https://vault.example.com/identity/connect/token \
  -d "grant_type=client_credentials&client_id=...&client_secret=...&scope=api"
```

## Next Steps

- [Full Configuration Guide](../configuration/README.md)
- [Vault Configuration](../configuration/vaults.md)
- [Authentication Setup](../configuration/authentication.md)
- [Tool Backend](../configuration/tool-backend.md)
