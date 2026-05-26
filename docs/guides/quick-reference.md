# Quick Reference Guide

**Quick access to the most commonly needed information**

## 📦 Installation

### Docker (Fastest)
```bash
docker run -d \
  --name vaults-syncer \
  -v sync-data:/app/data \
  -p 8080:8080 \
  -p 9090:9090 \
  -e MASTER_ENCRYPTION_KEY=${MASTER_ENCRYPTION_KEY} \
  ghcr.io/pacorreia/vaults-syncer:latest
```

### Docker Compose
```bash
# See getting-started/installation.md for full example
docker compose up -d
```

### Binary
```bash
# Download from releases, then:
# Set required env vars
export MASTER_ENCRYPTION_KEY=<your-key>
./sync-daemon
```

## ⚙️ Basic Configuration

The daemon stores configuration in its database. Use the Web UI at `http://localhost:8080` or the admin API to add vaults and syncs. Below is the data format used by the API.

```json
// POST /api/config/vaults
{
  "id": "source",
  "type": "azure",
  "endpoint": "https://myvault.vault.azure.net/secrets",
  "auth": {
    "method": "bearer",
    "headers": {"token": "${AZURE_ACCESS_TOKEN}"}
  },
  "field_names": {"name_field": "name", "value_field": "value"}
}

// POST /api/config/syncs
{
  "id": "my-sync",
  "source": "source",
  "targets": ["target"],
  "sync_type": "unidirectional",
  "schedule": "0 * * * *",
  "enabled": true
}
```

## 🔐 Authentication Quick Reference

| Vault | Recommended Auth | Setup Complexity |
|-------|------------------|------------------|
| Azure Key Vault | Bearer Token (Azure AD) | Medium |
| Bitwarden | OAuth2 | Medium |
| HashiCorp Vault | X-Vault-Token header | Medium |
| AWS Secrets Manager | Bearer Token / Custom | Medium |

## 📅 Common Schedules

```yaml
# Every hour
schedule: "0 * * * *"

# Every 4 hours
schedule: "0 */4 * * *"

# Daily at 2 AM
schedule: "0 2 * * *"

# Monday at 3 AM
schedule: "0 3 * * 1"

# Every weekday at 9 AM
schedule: "0 9 * * 1-5"

# Every 30 minutes
schedule: "*/30 * * * *"
```

## 🔍 Useful Commands

```bash
# Check version
./sync-daemon --version

# Validate database connection only
./sync-daemon -dry-run

# Get an auth token (all API calls below require this)
TOKEN=$(curl -s -X POST http://localhost:8080/api/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"your-password"}' | jq -r .token)

# Check health (authenticated)
curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/health

# List vaults
curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/vaults

# List syncs
curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/syncs

# Get sync status
curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/syncs/{sync-id}/status

# View run history
curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/syncs/{sync-id}/runs

# Run sync now
curl -X POST -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/syncs/{sync-id}/execute

# View metrics (no auth, port 9090)
curl http://localhost:9090/metrics | grep sync

# Check logs
docker logs vaults-syncer
docker compose logs -f sync-daemon
```

## 🚀 Quick Start Steps

1. **Create config file** → `config.yaml`
2. **Run with Docker** → `docker run ...`
3. **Verify** → `curl http://localhost:8080/health`
4. **Check status** → `curl http://localhost:8080/syncs`

## 📚 Documentation Map

| Need | Document |
|------|----------|
| Getting started | [Getting Started](../getting-started/index.md) |
| Install | [Installation](../getting-started/installation.md) |
| Requirements | [Requirements](../getting-started/requirements.md) |
| 5-min guide | [Quick Start](../getting-started/quick-start.md) |
| Configure vaults | [Vaults](../configuration/vaults.md) |
| Set up auth | [Authentication](../configuration/authentication.md) |
| Create syncs | [Syncs](../configuration/syncs.md) |
| Config options | [Configuration](../configuration/README.md) |
| Main index | [Documentation](../index.md) |

## ✅ Pre-Flight Checklist

Before running in production:

- [ ] Read [Requirements](../getting-started/requirements.md)
- [ ] Verify network access to all vaults
- [ ] Test authentication credentials
- [ ] Create test sync first
- [ ] Verify sync works as expected
- [ ] Set up monitoring/alerts
- [ ] Enable audit logging
- [ ] Document your sync topology
- [ ] Create backup procedure
- [ ] Plan credential rotation

## 🆘 Quick Troubleshooting

### Application won't start
```bash
# Check logs
docker logs vaults-syncer

# Validate database connection
./sync-daemon -dry-run
```

### Sync not running
1. Get a token: `TOKEN=$(curl -s -X POST http://localhost:8080/api/auth/login -H 'Content-Type: application/json' -d '{"username":"admin","password":"pass"}' | jq -r .token)`
2. Check health: `curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/health`
3. Verify vaults are accessible
4. Check schedule syntax (must be valid cron)
5. Review logs: `docker logs vaults-syncer`

### Authentication failed
1. Verify credentials/tokens
2. Check expiration
3. Verify permissions in target vault
4. Test connectivity to vault endpoint

### Performance issues
1. Check batch size in config
2. Reduce frequency of syncs
3. Monitor vault API response times
4. Check network bandwidth

## 🔒 Security Checklist

- [ ] Use managed identities (not hardcoded credentials)
- [ ] Store secrets in environment variables
- [ ] Restrict config file permissions (chmod 600)
- [ ] Use least-privilege vault permissions
- [ ] Enable TLS for all connections
- [ ] Enable audit logging
- [ ] Rotate credentials regularly
- [ ] Restrict network access to vaults

## 🎯 Common Tasks

### Create first sync
1. Define source and target vaults in config
2. Add sync with schedule
3. Test with `curl -X POST .../syncs/{id}/run`
4. Verify in target vault

### Enable bidirectional sync
```yaml
syncs:
  - id: bi-sync
    source: vault1
    targets: [vault2]
    sync_type: bidirectional
```

### Filter secrets
```yaml
syncs:
  - id: filtered
    source: source
    targets: [target]
    sync_type: unidirectional
    filter:
      patterns:
        - "prod-*"
      exclude:
        - "*"
```

### Transform names
```yaml
syncs:
  - id: transform
    source: source
    targets: [target]
    sync_type: unidirectional
    transforms:
      - field: value
        type: base64_encode
```

## 📞 Get Help

- 📖 Full documentation: [docs index](../index.md)
- 🐋 Report bugs: [GitHub Issues](https://github.com/pacorreia/vaults-syncer/issues)
- 💡 Ask questions: [GitHub Discussions](https://github.com/pacorreia/vaults-syncer/discussions)
- 🤝 Contribute: [CONTRIBUTING.md](https://github.com/pacorreia/vaults-syncer/blob/main/CONTRIBUTING.md)

## 📊 API Quick Ref

All endpoints under `/api/` (except login and setup) require `Authorization: Bearer <token>`.

```
POST /api/auth/login          - Login and get token (no auth)
GET  /api/setup               - Check setup status (no auth)
POST /api/setup               - Complete first-time setup (no auth)
GET  /api/health              - Health check
GET  /api/vaults              - List vaults
GET  /api/syncs               - List all syncs
GET  /api/syncs/{id}/status   - Get sync status
GET  /api/syncs/{id}/runs     - Sync run history
POST /api/syncs/{id}/execute  - Run sync now
GET  /api/metrics             - Prometheus metrics (also on :9090/metrics without auth)
GET  /api/config/vaults       - List vault configs (admin)
POST /api/config/vaults       - Create vault (admin)
GET  /api/config/syncs        - List sync configs (admin)
POST /api/config/syncs        - Create sync (admin)
```

## 🔗 Useful Links

- **Repository**: https://github.com/pacorreia/vaults-syncer
- **Issues**: https://github.com/pacorreia/vaults-syncer/issues
- **Releases**: https://github.com/pacorreia/vaults-syncer/releases
- **Docker Hub**: ghcr.io/pacorreia/vaults-syncer

---

For detailed information, see the [full documentation](../index.md).

**Last Updated**: 2024-01-15  
**Version**: 1.0.0+
