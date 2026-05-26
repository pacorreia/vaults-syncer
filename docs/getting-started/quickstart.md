# Quick Start

Get Secrets Vault Sync running in 5 minutes!

## 1. Run with Docker

```bash
docker run -d \
  --name vaults-syncer \
  -v sync-data:/app/data \
  -p 8080:8080 \
  -p 9090:9090 \
  ghcr.io/pacorreia/vaults-syncer:latest
```

Check the logs for the generated encryption key on first start:

```bash
docker logs vaults-syncer
```

Look for a banner with `MASTER ENCRYPTION KEY – SAVE THIS NOW` and copy the printed key. Set it before the next restart:

```bash
export MASTER_ENCRYPTION_KEY=<printed-value>
```

## 2. Complete Setup

Open `http://localhost:8080` and follow the Setup Wizard to create an admin account.

## 3. Add Vaults

Via the Web UI, navigate to **Vaults Config**, or use the API:

```bash
# Get auth token
TOKEN=$(curl -s -X POST http://localhost:8080/api/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"your-password"}' | jq -r .token)

# Add source vault (Vaultwarden)
curl -s -X POST http://localhost:8080/api/config/vaults \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "id": "vault-prod",
    "type": "vaultwarden",
    "endpoint": "https://vault.example.com/api/ciphers",
    "method": "POST",
    "auth": {
      "method": "oauth2",
      "oauth": {
        "token_endpoint": "https://vault.example.com/identity/connect/token",
        "client_id": "your-client-id",
        "client_secret": "your-secret",
        "scope": "api",
        "extra_params": {"deviceIdentifier": "sync-daemon"}
      }
    },
    "field_names": {"name_field": "name", "value_field": "login"}
  }'

# Add target vault (backup Vaultwarden)
curl -s -X POST http://localhost:8080/api/config/vaults \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "id": "vault-backup",
    "type": "vaultwarden",
    "endpoint": "https://backup.example.com/api/ciphers",
    "method": "POST",
    "auth": {"method": "bearer", "headers": {"token": "your-bearer-token"}},
    "field_names": {"name_field": "name", "value_field": "login"}
  }'
```

## 4. Create a Sync

```bash
curl -s -X POST http://localhost:8080/api/config/syncs \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "id": "backup-sync",
    "source": "vault-prod",
    "targets": ["vault-backup"],
    "sync_type": "unidirectional",
    "schedule": "0 */4 * * *",
    "enabled": true
  }'
```

## 5. Trigger and Monitor

```bash
# Trigger sync immediately
curl -X POST -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/syncs/backup-sync/execute

# Check status
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/syncs/backup-sync/status

# View run history
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/syncs/backup-sync/runs
```

## Common Configuration Patterns

### Backup Strategy

Sync all secrets from production to backup every 4 hours:

```json
{
  "id": "prod-to-backup",
  "source": "production",
  "targets": ["backup"],
  "sync_type": "unidirectional",
  "schedule": "0 */4 * * *",
  "enabled": true
}
```

### Multi-Environment Sync

Keep dev and staging in sync with filtered production secrets:

```json
{
  "id": "prod-to-dev",
  "source": "production",
  "targets": ["development", "staging"],
  "sync_type": "unidirectional",
  "schedule": "0 6 * * *",
  "filter": {
    "patterns": ["dev-*", "shared-*"],
    "exclude": ["*-prod"]
  },
  "enabled": true
}
```

## Next Steps

- 📖 [Full Configuration Guide](../configuration/README.md)
- 🔐 [Authentication Setup](../configuration/authentication.md)
- 🔄 [Sync Configuration](../configuration/syncs.md)

## Need Help?

- Open an [issue on GitHub](https://github.com/pacorreia/vaults-syncer/issues)
- Check the [Configuration Guide](../configuration/README.md)
