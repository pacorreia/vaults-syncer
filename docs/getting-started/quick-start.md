# Quick Start Guide

Get vaults-syncer up and running in 5 minutes.

## 1. Basic Setup

### Step 1: Create Configuration

Create `config.yaml` in your working directory:

```yaml
sync:
  interval: 3600  # Sync every hour
  timeout: 300    # 5 minute timeout

vaults:
  - id: akv-prod
    name: Azure Key Vault (Production)
    type: azure-keyvault
    endpoint: https://myvault.vault.azure.net/
    auth:
      method: managed-identity
  
  - id: bitwarden-prod
    name: Bitwarden Production
    type: bitwarden
    endpoint: https://vault.example.com
    auth:
      method: oauth2
      client_id: your-client-id
      client_secret: your-client-secret

syncs:
  - id: akv-to-bitwarden
    name: AKV to Bitwarden Sync
    source: akv-prod
    target: bitwarden-prod
    schedule: "0 * * * *"  # Hourly
    mode: one-way
```

### Step 2: Run with Docker

```bash
docker run -d \
  --name akv-sync \
  -v $(pwd)/config.yaml:/etc/sync/config.yaml:ro \
  -p 8080:8080 \
  ghcr.io/pacorreia/vaults-syncer:latest
```

### Step 3: Verify It's Running

```bash
curl http://localhost:8080/health
```

You should see:
```json
{
  "status": "healthy",
  "timestamp": "2024-01-15T10:30:00Z"
}
```

## 2. Set Up Your First Sync

### Create Azure Key Vault Source

1. In your `config.yaml`, ensure Azure Key Vault is configured
2. Authenticate using one of these methods:
   - **Managed Identity** (recommended for Azure VMs/AKS)
   - **Service Principal** (for automated scenarios)
   - **Client Certificate** (for applications)

### Create Bitwarden Target

1. Create an OAuth2 application in Bitwarden
2. Get your `client_id` and `client_secret`
3. Add to config:

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

### Configure Sync Rule

Add a new sync configuration:

```yaml
syncs:
  - id: my-first-sync
    source: akv-prod
    target: bitwarden
    schedule: "0 */4 * * *"  # Every 4 hours
    mode: one-way
    filters:
      - source_regex: "^app-.*"
        target_name: "Auto-synced: {source_name}"
```

## 3. Monitor Your Sync

### View Sync Status

```bash
curl http://localhost:8080/syncs/my-first-sync
```

Response:
```json
{
  "id": "my-first-sync",
  "name": "My First Sync",
  "source": "akv-prod",
  "target": "bitwarden",
  "status": "running",
  "last_run": "2024-01-15T10:30:00Z",
  "next_run": "2024-01-15T14:30:00Z",
  "stats": {
    "total_items": 42,
    "synced": 40,
    "failed": 0,
    "skipped": 2
  }
}
```

### View Metrics

```bash
curl http://localhost:9090/metrics | grep sync
```

### Check Logs

```bash
# Docker
docker logs akv-sync

# Docker Compose
docker compose logs -f sync-daemon

# Systemd
sudo journalctl -u akv-sync -f
```

## 4. Common Tasks

### Change Sync Interval

Edit `config.yaml` and update the sync schedule:

```yaml
syncs:
  - id: my-sync
    schedule: "0 */2 * * *"  # Every 2 hours instead
```

Restart the service:
```bash
docker restart akv-sync
```

### Add More Vaults

Simply add additional vault configurations:

```yaml
vaults:
  - id: secondary-vault
    type: bitwarden
    endpoint: https://vault2.example.com
    auth:
      method: oauth2
      client_id: ${SECOND_CLIENT_ID}
      client_secret: ${SECOND_CLIENT_SECRET}
```

### Enable Bidirectional Sync

Change sync mode:

```yaml
syncs:
  - id: my-sync
    mode: bidirectional
    conflict_resolution: source-wins  # or target-wins
```

### Add Filtering

Sync only specific items:

```yaml
syncs:
  - id: filtered-sync
    filters:
      - source_regex: "^prod-"
        target_regex: "^sync-"
```

## 5. Best Practices

### Security

1. ✅ Use managed identities or service principals, never store credentials in config
2. ✅ Restrict file permissions: `chmod 600 config.yaml`
3. ✅ Use read-only credentials for source vaults when possible
4. ✅ Enable audit logging
5. ✅ Use TLS for all vault endpoints

### Reliability

1. ✅ Test syncs in non-production first
2. ✅ Start with one-way syncs, move to bidirectional carefully
3. ✅ Monitor and alert on sync failures
4. ✅ Keep backups of critical secrets
5. ✅ Set appropriate sync intervals (don't sync too frequently)

### Operations

1. ✅ Use containerization (Docker/Kubernetes)
2. ✅ Implement health checks
3. ✅ Monitor metrics and logs
4. ✅ Set up alerting on failures
5. ✅ Document your sync topology

## Troubleshooting

### Sync not running

1. Check logs: `docker logs akv-sync`
2. Verify schedule syntax: `0 */4 * * *` (cron format)
3. Ensure vaults are accessible: `curl https://vault.example.com`
4. Check credentials and authentication

### Authentication failures

```bash
# Test Azure authentication
az account show

# Test Bitwarden authentication
curl -X POST https://vault.example.com/identity/connect/token \
  -d "grant_type=client_credentials&client_id=..." \
  -d "client_secret=..."
```

### Performance issues

1. Increase sync timeout in config
2. Reduce number of items per sync
3. Use filters to limit scope
4. Check vault API rate limits

## Next Steps

- [Full Configuration Guide](../configuration/README.md)
- [Vault Configuration](../configuration/vaults.md)
- [Authentication Setup](../configuration/authentication.md)
