# Requirements

Ensure your environment meets these requirements before installing vaults-syncer.

## System Requirements

### Minimum Specifications

| Requirement | Minimum | Recommended |
|------------|---------|-------------|
| CPU | 1 core | 2+ cores |
| Memory (RAM) | 256 MB | 512 MB - 1 GB |
| Storage | 50 MB | 500 MB |
| Network | 1 Mbps | 10 Mbps+ |

### Supported Operating Systems

- **Linux**: Ubuntu 20.04+, RHEL 8+, Debian 11+, Alpine 3.17+
- **macOS**: 10.14+ (Intel), 11+ (Apple Silicon)
- **Windows**: Windows Server 2019+, Windows 10+
- **Cloud**: AWS, Azure, GCP, any Kubernetes cluster

### Network Requirements

- **Outbound HTTPS**: Required to connect to vaults
- **Firewall**: Allow outbound HTTPS (port 443)
- **DNS**: Must resolve vault hostnames
- **Connectivity**: Network access to all configured vault endpoints

## Dependency Requirements

### Runtime

- **Go 1.22+** (if building from source)
- **Docker 20.10+** (for containerized deployment)
- **Docker Compose 2.0+** (for Compose deployments)
- **Kubernetes 1.24+** (for Kubernetes deployment)

### For Vault Connectivity

#### Azure Key Vault

- **Azure CLI 2.30+** (optional, for local testing)
- **Azure SDK** (included in binary)
- **Network**: Connectivity to `*.vault.azure.net`
- **Authentication**: One of:
  - Microsoft Entra ID (Managed Identity)
  - Service Principal
  - Client Certificate
  - User Authentication

#### Bitwarden

- **OAuth2 Support**: Bitwarden Server 2022.12+
- **Network**: HTTPS connectivity to Bitwarden endpoint
- **Authentication**: OAuth2 credentials (client ID + secret)

#### Other Vaults

Check specific vault integration documentation for:
- Required API versions
- Authentication mechanisms
- Network endpoints

## Key Vault Requirements

### Source Vault (Azure Key Vault)

```
Permissions Needed:
├── Get Secret
├── List Secrets
├── Get Key Properties (for key syncing)
└── Optional: Delete (for cleanup operations)
```

Configure with Azure RBAC:

```bash
# Create custom role
az role definition create --role-definition '{
  "Name": "AKV Sync Reader",
  "Description": "Role for vaults-syncer",
  "Type": "CustomRole",
  "Actions": [
    "Microsoft.KeyVault/vaults/secrets/read",
    "Microsoft.KeyVault/vaults/keys/read"
  ],
  "NotActions": [],
  "AssignableScopes": ["/subscriptions/{subscription-id}"]
}'

# Assign to Managed Identity or Service Principal
az role assignment create \
  --role "AKV Sync Reader" \
  --assignee-object-id <identity-object-id> \
  --scope /subscriptions/{sub}/resourceGroups/{rg}/providers/Microsoft.KeyVault/vaults/{vault}
```

### Target Vault (Bitwarden)

```
Permissions Needed:
├── Admin Access (for user/collection management)
├── Cipher Read & Write
├── Folder Create & Update
└── Collection Manage
```

Configure OAuth2 application:

1. Access Bitwarden admin panel
2. Create new OAuth2 integration
3. Grant minimum necessary scopes:
   - `cipher:read`
   - `cipher:write`
   - `folder:read`
   - `folder:write`
4. Generate credentials

## Storage Requirements

### Local Filesystem

- **State Files**: ~1-10 MB
- **Cache**: ~10-50 MB
- **Logs**: 10-100 MB (depends on log retention)
- **Total**: Minimum 50 MB free space

### Database (Optional)

For advanced deployments:

- **PostgreSQL 12+**
- **MySQL 5.7+**
- **SQLite** (embedded, no setup needed)

## Monitoring & Observability Requirements

### Metrics

- **Prometheus Endpoint**: HTTP port 9090
- **Metrics Retention**: Configurable (default: 30 days)

### Logging

- **Log Destination**: stdout, file, or centralized logging
- **Storage**: 10-100 MB depending on retention
- **Formats**: JSON, text, structured

### Alerting

- **Prometheus AlertManager** (optional)
- **Email notifications** (optional)
- **Webhook integrations** (optional)

## Security Requirements

### Encryption

- **Credentials**: Must be encrypted at rest
- **TLS**: HTTPS for all communication (TLS 1.2+)
- **Key Rotation**: Supported and recommended

### Access Control

- **Authentication**: Use managed identities or service principals
- **RBAC**: Implement least-privilege principles
- **Audit Logging**: Enable and retain for compliance

### Secrets Management

- **Environment Variables**: For sensitive config
- **Secret Stores**: Azure Key Vault, HashiCorp Vault, etc.
- **No Plaintext**: Never store credentials in config files

## API Requirements

### Vault APIs

All vaults must support:

```
REST API (HTTPS)
├── Authentication endpoint
├── List secrets/items endpoint
├── Get secret/item endpoint
├── Create/Update secret endpoint (for write operations)
└── Delete secret endpoint (optional)
```

### HTTP Compatibility

- HTTP/1.1 minimum
- HTTP/2 supported
- Keep-alive connections required

## Port Requirements

Default ports (configurable):

| Service | Port | Protocol | Purpose |
|---------|------|----------|---------|
| API Server | 8080 | HTTP | REST endpoints |
| Metrics | 9090 | HTTP | Prometheus metrics |
| Webhook | 8081 | HTTP | Event webhooks (optional) |

### Firewall Rules

```bash
# Allow inbound
- 8080/TCP (API access)
- 9090/TCP (Metrics scraping)

# Allow outbound
- 443/TCP (HTTPS to vaults)
- 53/UDP (DNS)
```

## Python Integration (Optional)

If using Python SDK:

- **Python 3.8+**
- **pip** or **poetry**
- **Dependencies**: requests, pydantic, pyyaml

## Telemetry & Analytics

### Optional Services

- **OpenTelemetry**: For distributed tracing
- **DataDog/New Relic**: For APM
- **CloudWatch/Azure Monitor**: For cloud logging

These are optional and can be enabled via configuration.

## Compliance Requirements

### Data Protection

- **GDPR**: Data retention policies supported
- **HIPAA**: Authentication and audit logging
- **SOC 2**: Supported with proper configuration

### Audit Requirements

- **Log Retention**: Configurable
- **Audit Trail**: All operations logged
- **Immutable Logs**: Can be sent to external storage

## Capacity Planning

### For 100 Secrets

```
CPU: <1 core (minimal usage)
Memory: ~200 MB
Storage: ~5 MB
Network: <1 Mbps
Sync Time: <5 seconds
```

### For 10,000 Secrets

```
CPU: 1-2 cores
Memory: ~500 MB
Storage: ~50 MB
Network: 5-10 Mbps
Sync Time: 30-60 seconds
```

### For 100,000+ Secrets

```
CPU: 2-4 cores
Memory: 1-2 GB
Storage: 100-500 MB
Network: 10+ Mbps
Sync Time: 5-10 minutes
Recommend: Pagination/batching
```

## Pre-Installation Checklist

- [ ] System meets minimum requirements
- [ ] Network connectivity verified to all vaults
- [ ] Credentials obtained and stored securely
- [ ] Necessary permissions granted in source/target vaults
- [ ] Ports 8080, 9090 available (or configured differently)
- [ ] TLS certificates valid and up-to-date
- [ ] Storage capacity adequate
- [ ] Backup strategy planned
- [ ] Monitoring and alerting configured
- [ ] Compliance requirements understood

## Next Steps

1. [Installation](./installation.md)
2. [Quick Start](./quick-start.md)
3. [Configuration](../configuration/README.md)
