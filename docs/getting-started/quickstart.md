# Quick Start

Get Secrets Vault Sync running in 5 minutes!

## 1. Create Configuration

Save this as `config.yaml`:

```yaml
vaults:
  # Source vault (where secrets are read from)
  - id: vault-prod
    name: Production Vault
    type: vaultwarden
    endpoint: https://vault.example.com/api/ciphers
    method: POST
    auth:
      method: oauth2
      oauth:
        token_endpoint: https://vault.example.com/identity/connect/token
        client_id: your-client-id
        client_secret: your-secret
        scope: api
        extra_params:
          deviceIdentifier: akv-sync
    field_names:
      name_field: name
      value_field: name
    headers:
      Accept: application/json
      Content-Type: application/json

  # Target vault (where secrets are written)
  - id: vault-backup
    name: Backup Vault
    type: vaultwarden
    endpoint: https://backup.example.com/api/ciphers
    method: POST
    auth:
      method: bearer
      headers:
        token: your-bearer-token
    field_names:
      name_field: name
      value_field: name
    headers:
      Accept: application/json
      Content-Type: application/json

# Define sync relationships
syncs:
  - id: backup-sync
    source: vault-prod
    targets:
      - vault-backup
    sync_type: unidirectional
    schedule: "0 */4 * * *"  # Every 4 hours
    filter:
      patterns:
        - "*"  # Sync all secrets

# Server configuration
server:
  port: 8080
  address: 0.0.0.0

# Logging
logging:
  level: info
  format: json
```

## 2. Run with Docker

```bash
docker run -d \
  --name akv-sync \
  -v $(pwd)/config.yaml:/etc/sync/config.yaml:ro \
  -v sync-data:/app/data \
  -p 8080:8080 \
  ghcr.io/pacorreia/vaults-syncer:latest
```

Or with Docker Compose:

```yaml
version: '3.8'

services:
  sync-daemon:
    image: ghcr.io/pacorreia/vaults-syncer:latest
    volumes:
      - ./config.yaml:/etc/sync/config.yaml:ro
      - sync-data:/app/data
    ports:
      - "8080:8080"
      - "9090:9090"
    environment:
      - LOG_LEVEL=info

volumes:
  sync-data:
```

Run it:

```bash
docker compose up -d
```

## 3. Verify It's Working

Check daemon health:

```bash
curl http://localhost:8080/health
```

Expected response:

```json
{
  "running": true,
  "status": "healthy",
  "syncs": 1,
  "vaults": 2
}
```

## 4. Trigger First Sync

```bash
curl -X POST http://localhost:8080/syncs/backup-sync/execute
```

Monitor progress:

```bash
curl http://localhost:8080/syncs/backup-sync/status
```

## 5. View Logs

```bash
# Docker
docker logs akv-sync -f

# Or view directly
tail -f /path/to/logs.json
```

## Common Configuration Patterns

### Backup Strategy

Sync all secrets from production to backup every 4 hours:

```yaml
syncs:
  - id: prod-to-backup
    source: production
    targets:
      - backup
    sync_type: unidirectional
    schedule: "0 */4 * * *"
```

### Multi-Cloud

Sync between Azure and AWS:

```yaml
syncs:
  - id: az-to-aws
    source: azure-vault
    targets:
      - aws-vault
    sync_type: bidirectional
```

### Development Sync

Keep dev and staging in sync with production non-prod secrets:

```yaml
syncs:
  - id: prod-to-dev
    source: production
    targets:
      - development
      - staging
    filter:
      patterns:
        - "dev-*"
        - "shared-*"
      exclude:
        - "*-prod"
```

## Next Steps

- 📖 [Full Configuration Guide](../configuration/README.md)
- 🔐 [Authentication Setup](../configuration/authentication.md)
- 🔄 [Sync Configuration](../configuration/syncs.md)

## Need Help?

- Open an [issue on GitHub](https://github.com/pacorreia/vaults-syncer/issues)
- Check the [Configuration Guide](../configuration/README.md)
