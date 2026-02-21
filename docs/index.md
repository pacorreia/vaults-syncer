# AKV Vaultwarden Sync

A versatile, multi-vault secret synchronization daemon with OAuth 2.0 support for seamless integration between Vaultwarden, Azure Key Vault, HashiCorp Vault, AWS Secrets Manager, and custom vault backends.

## Features

✨ **Multi-Vault Support**
- Vaultwarden
- HashiCorp Vault
- Azure Key Vault
- AWS Secrets Manager
- Generic HTTP-based vaults

🔐 **Flexible Authentication**
- OAuth 2.0 with Vaultwarden support
- Bearer Token
- Basic Authentication
- API Key
- Custom Headers

🔄 **Powerful Sync**
- Unidirectional and bidirectional sync
- Concurrent processing (configurable workers)
- Scheduled execution (cron format)
- Configurable retry policies
- Pattern-based filtering (include/exclude)

📊 **Monitoring & Observability**
- HTTP REST API for operations
- Prometheus metrics export
- Structured JSON logging
- Health check endpoints

🏗️ **Production Ready**
- SQLite state database
- Transaction support
- Error recovery and retry logic
- Docker & Kubernetes ready

## Quick Links

- **[Getting Started](getting-started/index.md)** - New to AKV Sync? Start here
- **[Installation](getting-started/installation.md)** - Setup in 5 minutes
- **[Configuration](configuration/README.md)** - Complete configuration guide
- **[Vaults](configuration/vaults.md)** - Configure your vault connections
- **[Authentication](configuration/authentication.md)** - Set up secure access

## Common Use Cases

### Backup & Replication
Synchronize secrets from your production vault to a backup vault for disaster recovery.

```yaml
syncs:
  - id: prod_to_backup
    source: production
    targets:
      - backup
    sync_type: unidirectional
    schedule: "0 */4 * * *"  # Every 4 hours
```

### Multi-Cloud Deployment
Keep secrets in sync across different cloud providers.

```yaml
syncs:
  - id: aws_to_azure
    source: aws-vault
    targets:
      - azure-vault
    sync_type: bidirectional
```

### Development Environment Sync
Maintain synchronized secrets across dev, staging, and production.

```yaml
syncs:
  - id: to_development
    source: production
    targets:
      - development
      - staging
    filter:
      patterns:
        - "non-prod-*"
        - "shared-*"
```

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                    AKV Sync Daemon                          │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  ┌──────────────┐         ┌──────────────┐                 │
│  │   Vaults     │ OAuth2  │   Backends   │                 │
│  │              │────────▶│              │                 │
│  │ Vaultwarden  │         │ Generic HTTP │                 │
│  │ Azure KV     │         │ Vault        │                 │
│  │ AWS SM       │         │ AWS          │                 │
│  │ HashiCorp    │         │ Azure        │                 │
│  └──────────────┘         └──────────────┘                 │
│         △                         △                         │
│         │                         │                         │
│         └────────┬────────────────┘                         │
│                  │                                          │
│         ┌────────▼────────┐                                │
│         │   Sync Engine   │                                │
│         │                 │                                │
│         │ • Scheduling    │                                │
│         │ • Filtering     │                                │
│         │ • Retries       │                                │
│         └────────┬────────┘                                │
│                  │                                          │
│         ┌────────▼────────┐                                │
│         │  State Database │                                │
│         │   (SQLite)      │                                │
│         └─────────────────┘                                │
│                                                              │
│  ┌─────────────────────────────────────────┐              │
│  │  REST API & Monitoring                  │              │
│  │  • Sync Control & Status                │              │
│  │  • Prometheus Metrics                   │              │
│  │  • Health Checks                        │              │
│  └─────────────────────────────────────────┘              │
└─────────────────────────────────────────────────────────────┘
```

## Getting Started

### Installation

=== "Docker"

    ```bash
    docker run -d \
      --name akv-sync \
      -v $(pwd)/config.yaml:/etc/sync/config.yaml:ro \
      -v $(pwd)/data:/app/data \
      -p 8080:8080 \
      -p 9090:9090 \
      ghcr.io/pacorreia/vaults-syncer:latest
    ```

=== "Binary"

    ```bash
    # Download latest release
    wget https://github.com/pacorreia/vaults-syncer/releases/download/v1.0.0/sync-daemon-linux-amd64
    chmod +x sync-daemon-linux-amd64
    
    # Run
    ./sync-daemon-linux-amd64 -config config.yaml
    ```

=== "Source"

    ```bash
    git clone https://github.com/pacorreia/vaults-syncer
    cd vaults-syncer
    go build -o sync-daemon .
    ./sync-daemon -config config.yaml
    ```

### Minimal Configuration

Create `config.yaml`:

```yaml
vaults:
  - id: source
    name: Source Vault
    type: vaultwarden
    endpoint: https://vault.example.com/api/ciphers
    auth:
      method: oauth2
      oauth:
        token_endpoint: https://vault.example.com/identity/connect/token
        client_id: your-client-id
        client_secret: your-client-secret
        scope: api

  - id: target
    name: Target Vault
    type: vaultwarden
    endpoint: https://backup.example.com/api/ciphers
    auth:
      method: bearer
      headers:
        token: your-bearer-token

syncs:
  - id: backup-sync
    source: source
    targets:
      - target
    sync_type: unidirectional
    schedule: "0 */4 * * *"  # Every 4 hours

server:
  port: 8080

logging:
  level: info
  format: json
```

Then run:

```bash
./sync-daemon -config config.yaml
```

Check status:

```bash
curl http://localhost:8080/health
```

## Development

This project is written in **Go 1.22** with a modular architecture:

- **Zero backward compatibility compromises** - removed all legacy code paths after generalization
- **Interface-driven design** - pluggable vault backends
- **Production-ready** - comprehensive error handling and retry logic
- **Well-tested** - unit tests, integration tests, and real vault testing

### Build

```bash
go build -o sync-daemon .
```

### Test

```bash
# Unit tests
go test ./...

# Integration tests
./test-integration.sh
```

## Support Us

If you find this project useful, please consider:

- ⭐ Starring the repository
- 🐛 Reporting issues
- 💡 Contributing features
- 📚 Improving documentation

## License

MIT License - See [LICENSE](https://github.com/pacorreia/vaults-syncer/blob/main/LICENSE) for details.

## Contributing

We welcome contributions! Visit [CONTRIBUTING](https://github.com/pacorreia/vaults-syncer/blob/main/CONTRIBUTING.md) for guidelines.
