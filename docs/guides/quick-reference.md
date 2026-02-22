# Quick Reference Guide

**Quick access to the most commonly needed information**

## 📦 Installation

### Docker (Fastest)
```bash
docker run -d \
  --name akv-sync \
  -v $(pwd)/config.yaml:/etc/sync/config.yaml:ro \
  -p 8080:8080 \
  ghcr.io/pacorreia/vaults-syncer:latest
```

### Docker Compose
```bash
# See docs/getting-started/installation.md for full example
docker compose up -d
```

### Binary
```bash
# Download from releases, then
./sync-daemon -config config.yaml
```

## ⚙️ Basic Configuration

```yaml
vaults:
  - id: source
    type: azure
    endpoint: https://myvault.vault.azure.net/secrets
    auth:
      method: bearer
      headers:
        token: ${AZURE_ACCESS_TOKEN}
    field_names:
      name_field: name
      value_field: value
  
  - id: target
    type: bitwarden
    endpoint: https://vault.example.com/api/ciphers
    auth:
      method: oauth2
      oauth:
        client_id: ${CLIENT_ID}
        client_secret: ${CLIENT_SECRET}
        scope: api
    field_names:
      name_field: name
      value_field: login

syncs:
  - id: my-sync
    source: source
    targets: [target]
    sync_type: unidirectional
    schedule: "0 * * * *"
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
# Check health
curl http://localhost:8080/health

# List vaults
curl http://localhost:8080/vaults

# List syncs
curl http://localhost:8080/syncs

# Get sync status
curl http://localhost:8080/syncs/{sync-id}

# Run sync now
curl -X POST http://localhost:8080/syncs/{sync-id}/run

# View metrics
curl http://localhost:9090/metrics | grep sync

# Check logs
docker logs akv-sync
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
| Getting started | [Getting Started](./docs/getting-started/README.md) |
| Install | [Installation](./docs/getting-started/installation.md) |
| Requirements | [Requirements](./docs/getting-started/requirements.md) |
| 5-min guide | [Quick Start](./docs/getting-started/quick-start.md) |
| Configure vaults | [Vaults](./docs/configuration/vaults.md) |
| Set up auth | [Authentication](./docs/configuration/authentication.md) |
| Create syncs | [Syncs](./docs/configuration/syncs.md) |
| Config options | [Configuration](./docs/configuration/README.md) |
| Main index | [Documentation](./docs/README.md) |

## ✅ Pre-Flight Checklist

Before running in production:

- [ ] Read [Requirements](./docs/getting-started/requirements.md)
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
docker logs akv-sync

# Validate config
./sync-daemon -validate-config config.yaml
```

### Sync not running
1. Check health: `curl http://localhost:8080/health`
2. Verify vaults are accessible
3. Check schedule syntax (must be valid cron)
4. Review logs: `docker logs akv-sync`

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

- 📖 Full documentation: [docs/README.md](./docs/README.md)
- 🐋 Report bugs: [GitHub Issues](https://github.com/pacorreia/vaults-syncer/issues)
- 💡 Ask questions: [GitHub Discussions](https://github.com/pacorreia/vaults-syncer/discussions)
- 🤝 Contribute: [CONTRIBUTING.md](./CONTRIBUTING.md)

## 📊 API Quick Ref

```
GET /health              - Health check
GET /vaults              - List vaults
GET /vaults/health       - Vault health status
GET /syncs               - List all syncs
GET /syncs/{id}          - Get sync details
GET /syncs/{id}/history  - Sync history
POST /syncs/{id}/run     - Run sync now
GET /metrics             - Prometheus metrics
```

## 🔗 Useful Links

- **Repository**: https://github.com/pacorreia/vaults-syncer
- **Issues**: https://github.com/pacorreia/vaults-syncer/issues
- **Releases**: https://github.com/pacorreia/vaults-syncer/releases
- **Docker Hub**: ghcr.io/pacorreia/vaults-syncer

---

For detailed information, see the [full documentation](./docs/README.md).

**Last Updated**: 2024-01-15  
**Version**: 1.0.0+
