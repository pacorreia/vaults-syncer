# Configuring Syncs

Syncs define how and when secrets are synchronized between vaults.

## Sync Basics

A sync configuration specifies:

- **Source vault**: Where to get secrets from
- **Target vaults**: One or more destinations
- **Schedule**: When to perform the sync
- **Sync type**: Unidirectional or bidirectional
- **Filter**: Which secrets to sync
- **Transformations**: How to modify secrets during sync

## Basic Configuration

```yaml
syncs:
  - id: my-first-sync
    name: "My First Sync"
    source: vault-1
    targets: [vault-2]
    schedule: "0 * * * *"        # Every hour
    sync_type: unidirectional       # or bidirectional
```

## Sync Configuration Reference

### Essential Options

| Option | Type | Required | Description | Example |
|--------|------|----------|-------------|---------|
| `id` | string | Yes | Unique sync identifier | `sync-prod-to-staging` |
| `name` | string | No | Human-readable name | `Prod to Staging Sync` |
| `source` | string | Yes | Source vault ID | `azure-prod` |
| `targets` | array | Yes | Target vault IDs | `["bitwarden-prod"]` |
| `schedule` | string | Yes | Cron expression | `0 * * * *` |
| `sync_type` | string | No | `unidirectional` or `bidirectional` | `unidirectional` |

### Optional Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `enabled` | boolean | `true` | Enable/disable this sync |
| `timeout` | integer | `300` | Sync timeout in seconds |
| `max_retries` | integer | `3` | Maximum retry attempts |
| `description` | string | `""` | Sync description |
| `tags` | array | `[]` | Labels for organization |

## Schedule (Cron Expressions)

Syncs are scheduled using standard cron format: `minute hour day month weekday`

### Cron Format

```
┌───────────── minute (0 - 59)
│ ┌───────────── hour (0 - 23)
│ │ ┌───────────── day of month (1 - 31)
│ │ │ ┌───────────── month (1 - 12)
│ │ │ │ ┌───────────── day of week (0 - 6) (0 is Sunday)
│ │ │ │ │
│ │ │ │ │
* * * * *
```

### Common Schedules

```yaml
# Every hour
schedule: "0 * * * *"

# Every 4 hours
schedule: "0 */4 * * *"

# Daily at 2 AM
schedule: "0 2 * * *"

# Every Monday at 3 AM
schedule: "0 3 * * 1"

# Every weekday at noon
schedule: "0 12 * * 1-5"

# Every 30 minutes
schedule: "*/30 * * * *"

# First day of month at midnight
schedule: "0 0 1 * *"
```

### Cron Reference

- `*` - Any value
- `,` - Value list separator
- `-` - Range of values
- `/` - Step values

```yaml
# Examples:
"0 9-17 * * 1-5"    # 9 AM to 5 PM, weekdays only
"0,30 * * * *"       # Every 30 minutes
"15 2 * * *"         # 2:15 AM daily
```

## Sync Modes

### One-Way Sync

Secrets flow only from source to target. Target changes are not synced back.

```yaml
syncs:
  - id: one-way-sync
    source: source-vault
    targets: [target-vault]
    sync_type: unidirectional
```

**Use cases**:
- Production → Staging/Dev
- Archive → Active (one-way replication)
- Migrate to new system

### Bidirectional Sync

Secrets synchronize in both directions. Changes in either vault are replicated.

```yaml
syncs:
  - id: bi-sync
    source: vault-1
    targets: [vault-2]
    sync_type: bidirectional
```

**Conflict Resolution Strategies**:

| Strategy | Behavior |
|----------|----------|
| `source-wins` | Source takes precedence on conflict |
| `target-wins` | Target takes precedence on conflict |
| `manual` | Conflicts require manual intervention |
| `newest` | Latest modified secret wins |

**Use cases**:
- Multi-region synchronization
- Cross-team access to shared credentials
- Disaster recovery with failover

## Filtering

Control which secrets are included in a sync.

### Include/Exclude Patterns

```yaml
syncs:
  - id: filtered-sync
    source: source-vault
    targets: [target-vault]
    sync_type: unidirectional
    schedule: "0 * * * *"
    
    filter:
      patterns:
        - "prod-*"
      exclude:
        - "*-db-password"
        - "temp-*"
```

### Filtering Examples

#### Sync only production secrets

```yaml
syncs:
  - id: prod-only
    source: all-vaults
    targets: [prod-target]
    sync_type: unidirectional
    filter:
      patterns:
        - "prod-*"
```

#### Multiple include patterns

```yaml
filter:
  patterns:
    - "app-*"
    - "db-*"
```

#### Exclude sensitive data

```yaml
filter:
  patterns:
    - "*"
  exclude:
    - "secret-*"
  - source_regex: ".*-master-key$"
    action: exclude
  - source_regex: ".*-admin-password$"
    action: exclude
```

## Transformations

Modify secret names and values during synchronization.

### Value Transformations

```yaml
syncs:
  - id: transform-sync
    source: source-vault
    targets: [target-vault]
    sync_type: unidirectional
    schedule: "0 * * * *"
    transforms:
      - field: value
        type: base64_encode
```

```yaml
transforms:
  - type: script
    match: ".*connection.*"
    script: |
      #!/bin/bash
      # Input passed as $1
      # Transform connection string
      echo "$1" | sed 's/;/\\n/g'
```

### Transformation Examples

#### Base64 encode values

```yaml
transforms:
  - field: value
    type: base64_encode
```

#### Base64 decode values

```yaml
transforms:
  - field: value
    type: base64_decode
```

## Advanced Options

### Retry Policy and Concurrency

```yaml
syncs:
  - id: tuned-sync
    source: source-vault
    targets: [target-vault]
    sync_type: unidirectional
    schedule: "0 * * * *"
    concurrent_workers: 5
    retry_policy:
      max_retries: 3
      initial_backoff: 1000
      max_backoff: 60000
      multiplier: 2.0
```

## Complete Sync Examples

### Example 1: Simple One-Way Sync

```yaml
syncs:
  - id: simple-sync
    name: "Azure to Bitwarden"
    source: azure-prod
    targets: [bitwarden]
    schedule: "0 * * * *"
    sync_type: unidirectional
```

### Example 2: Filtered Production Sync

```yaml
syncs:
  - id: prod-sync
    source: azure-prod
    targets: [bitwarden-prod]
    schedule: "0 */4 * * *"  # Every 4 hours
    sync_type: unidirectional
    filter:
      patterns:
        - "prod-*"
```

### Example 3: Bidirectional Sync with Transformations

```yaml
syncs:
  - id: bi-sync
    source: primary-vault
    targets: [secondary-vault]
    schedule: "*/30 * * * *"   # Every 30 minutes
    sync_type: bidirectional
    filter:
      patterns:
        - "shared-*"
    
    transforms:
      - match: "^shared-"
        replace: "synced/{source_name}"
    
    options:
      timeout: 600
      batch_size: 100
```

### Example 4: Multi-Environment Cascade

```yaml
syncs:
  # Dev → Staging
  - id: dev-to-staging
    source: vault-dev
    targets: [vault-staging]
    schedule: "0 6 * * *"      # 6 AM daily
    sync_type: unidirectional
    filter:
      patterns:
        - "app-*"
        - "db-*"
        - "api-*"
  
  # Staging → Production
  - id: staging-to-prod
    source: vault-staging
    targets: [vault-prod]
    schedule: "0 8 * * 0"      # 8 AM Sundays
    sync_type: unidirectional
    filter:
      patterns:
        - "app-*"
        - "db-*"
        - "api-*"
```

### Example 5: Selective Backup Sync

```yaml
syncs:
  - id: backup-sync
    name: "Production Backup"
    source: vault-prod
    targets: [vault-backup]
    schedule: "0 1 * * *"      # 1 AM daily
    sync_type: unidirectional
    
    filter:
      patterns:
        - "*"
      exclude:
        - "test-*"
        - "temp-*"
        - "debug-*"
    
    retry_policy:
      max_retries: 3
      initial_backoff: 1000
      max_backoff: 60000
      multiplier: 2.0
```

## Monitoring Syncs

### Check Sync Status

```bash
curl http://localhost:8080/syncs/my-sync
```

Response:

```json
{
  "id": "my-sync",
  "name": "My Sync",
  "enabled": true,
  "source": "vault-1",
  "targets": ["vault-2"],
  "schedule": "0 * * * *",
  "sync_type": "unidirectional",
  "status": "idle",
  "last_run": "2024-01-15T10:00:00Z",
  "last_run_status": "success",
  "last_run_duration": 2.345,
  "next_run": "2024-01-15T11:00:00Z",
  "stats": {
    "total_items": 42,
    "synced": 40,
    "failed": 0,
    "skipped": 2
  }
}
```

### View Sync History

```bash
curl http://localhost:8080/syncs/my-sync/history
```

### Manually Trigger Sync

```bash
curl -X POST http://localhost:8080/syncs/my-sync/run
```

## Best Practices

### Scheduling

✅ **Do**:
- Start with less frequent syncs (hourly or less)
- Use staggered schedules for multiple syncs
- Schedule backups during low-activity times
- Test schedules in staging first

❌ **Don't**:
- Sync too frequently (every minute)
- Overlap sync windows for same vaults
- Schedule during peak usage times
- Make schedule too complex

### Filtering

✅ **Do**:
- Use include/exclude patterns for clarity
- Start restrictive, then broaden
- Document filter logic
- Test filters before production

❌ **Don't**:
- Use overly complex regex patterns
- Accidentally exclude important secrets
- Forget to test filter impact
- Change filters without testing

### Transformations

✅ **Do**:
- Keep transformations simple
- Use placeholders for consistency
- Test transformations thoroughly
- Document transformation logic

❌ **Don't**:
- Use complex custom scripts
- Modify critical secret values
- Make irreversible transformations
- Skip testing transformations

## Troubleshooting

### Sync Not Running

1. Check if sync is enabled: `enabled: true`
2. Verify schedule syntax (cron format)
3. Check if vaults are healthy
4. Review logs for errors

### Secrets Not Syncing

1. Check filters are correct
2. Verify source vault has secrets matching filter
3. Review transformation logic
4. Check vault permissions

### Sync Takes Too Long

1. Reduce `batch_size`
2. Check network connectivity
3. Review vault API latency
4. Consider splitting into multiple syncs

## Next Steps

- [Go back to Configuration](./README.md)
- [Authentication Guide](./authentication.md)
- [Vaults Configuration](./vaults.md)
