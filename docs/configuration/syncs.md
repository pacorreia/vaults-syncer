# Configuring Syncs

Syncs define how and when secrets are synchronized between vaults.

## Sync Basics

A sync configuration specifies:

- **Source vault**: Where to get secrets from
- **Target vault**: Where to send secrets to
- **Schedule**: When to perform the sync
- **Mode**: One-way or bidirectional
- **Filters**: Which secrets to sync
- **Transformations**: How to modify secrets during sync

## Basic Configuration

```yaml
syncs:
  - id: my-first-sync
    name: "My First Sync"
    source: vault-1
    target: vault-2
    schedule: "0 * * * *"        # Every hour
    mode: one-way                  # or bidirectional
```

## Sync Configuration Reference

### Essential Options

| Option | Type | Required | Description | Example |
|--------|------|----------|-------------|---------|
| `id` | string | Yes | Unique sync identifier | `sync-prod-to-staging` |
| `name` | string | No | Human-readable name | `Prod to Staging Sync` |
| `source` | string | Yes | Source vault ID | `azure-prod` |
| `target` | string | Yes | Target vault ID | `bitwarden-prod` |
| `schedule` | string | Yes | Cron expression | `0 * * * *` |
| `mode` | string | No | `one-way` or `bidirectional` | `one-way` |

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
    target: target-vault
    mode: one-way
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
    target: vault-2
    mode: bidirectional
    conflict_resolution: source-wins
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
    target: target-vault
    schedule: "0 * * * *"
    
    filters:
      # Sync only production secrets
      - source_regex: "^prod-"
        action: include
      
      # Exclude database passwords
      - source_regex: ".*-db-password"
        action: exclude
      
      # Exclude temporary secrets
      - source_regex: "^temp-"
        action: exclude
```

### Filter Options

| Option | Type | Description |
|--------|------|-------------|
| `source_regex` | string | Regex pattern for source secret name |
| `source_tags` | array | Match secrets with specific tags |
| `action` | string | `include` or `exclude` |
| `target_name` | string | Transform secret name (uses placeholders) |

### Placeholders in Target Names

When specifying `target_name`, use placeholders:

```yaml
filters:
  - source_regex: "^app-"
    target_name: "imported/{source_name}"
    # Input: "app-api-key" → Output: "imported/app-api-key"
  
  - source_regex: "^db-prod-(.*)"
    target_name: "database/{1}/{source_name}"
    # Input: "db-prod-main-password" → Output: "database/main/db-prod-main-password"
```

**Available placeholders**:
- `{source_name}` - Original secret name
- `{1}`, `{2}`, etc. - Regex capture groups
- `{timestamp}` - Current timestamp
- `{env}` - Environment name (if configured)

### Filtering Examples

#### Sync only production secrets

```yaml
syncs:
  - id: prod-only
    source: all-vaults
    target: prod-target
    filters:
      - source_regex: "^prod-"
        action: include
      - source_regex: ".*"
        action: exclude
```

#### Multiple include patterns

```yaml
filters:
  - source_regex: "^app-"
    action: include
  - source_regex: "^db-"
    action: include
  - source_regex: ".*"
    action: exclude
```

#### Exclude sensitive data

```yaml
filters:
  - source_regex: ".*-master-key$"
    action: exclude
  - source_regex: ".*-admin-password$"
    action: exclude
```

## Transformations

Modify secret names and values during synchronization.

### Name Transformations

```yaml
syncs:
  - id: transform-sync
    source: source-vault
    target: target-vault
    schedule: "0 * * * *"
    
    transforms:
      - match: "^old-prefix-"
        replace: "new-prefix-"
        
      - match: "^(.+)-(prod)$"
        replace: "production/{1}"
```

### Value Transformations

```yaml
transforms:
  - type: base64-encode
    match: ".*api-key"
  
  - type: base64-decode
    match: ".*base64.*"
  
  - type: uppercase
    match: ".*code"
  
  - type: lowercase
    match: ".*email"
  
  - type: shell-escape
    match: ".*password"
```

### Custom Script Transformations

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

#### Add folder structure to names

```yaml
transforms:
  - match: "^(app|db|api)-"
    replace: "{1}/{source_name}"
    # app-api-key → app/app-api-key
```

#### Normalize naming convention

```yaml
transforms:
  - match: ".*"
    replace: "sync-{timestamp}-{source_name}"
    # Convert to: sync-2024-01-15T10:30:00Z-original-name
```

## Advanced Options

### Metadata Handling

```yaml
syncs:
  - id: with-metadata
    source: source-vault
    target: target-vault
    schedule: "0 * * * *"
    
    options:
      # Preserve metadata
      preserve_tags: true
      preserve_labels: true
      preserve_metadata: true
      
      # Handle secret types
      secret_types:
        password: "generic"        # Map password to generic
        api-key: "authentication"
        connection-string: "text"
```

### Batch Operations

```yaml
options:
  batch_size: 50          # Sync 50 secrets at a time
  batch_delay: 100        # Delay 100ms between batches (in milliseconds)
  disable_parallel: false # Enable parallel processing
```

### Error Handling

```yaml
options:
  on_error: continue      # continue, retry, or abort
  stop_on_first_error: false
  retryable_errors:
    - timeout
    - rate-limit
    - temporary-failure
```

## Complete Sync Examples

### Example 1: Simple One-Way Sync

```yaml
syncs:
  - id: simple-sync
    name: "Azure to Bitwarden"
    source: azure-prod
    target: bitwarden
    schedule: "0 * * * *"
    mode: one-way
```

### Example 2: Filtered Production Sync

```yaml
syncs:
  - id: prod-sync
    source: azure-prod
    target: bitwarden-prod
    schedule: "0 */4 * * *"  # Every 4 hours
    
    filters:
      - source_regex: "^prod-"
        action: include
        target_name: "prod/{source_name}"
```

### Example 3: Bidirectional Sync with Transformations

```yaml
syncs:
  - id: bi-sync
    source: primary-vault
    target: secondary-vault
    schedule: "*/30 * * * *"   # Every 30 minutes
    mode: bidirectional
    conflict_resolution: source-wins
    
    filters:
      - source_regex: "^shared-"
        action: include
    
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
    target: vault-staging
    schedule: "0 6 * * *"      # 6 AM daily
    mode: one-way
    
    filters:
      - source_regex: "^(app|db|api)-"
        action: include
    
    options:
      timeout: 300
      on_error: continue
  
  # Staging → Production
  - id: staging-to-prod
    source: vault-staging
    target: vault-prod
    schedule: "0 8 * * 0"      # 8 AM Sundays
    mode: one-way
    
    filters:
      - source_regex: "^(app|db|api)-"
        action: include
    
    options:
      timeout: 600
      stop_on_first_error: true
```

### Example 5: Selective Backup Sync

```yaml
syncs:
  - id: backup-sync
    name: "Production Backup"
    source: vault-prod
    target: vault-backup
    schedule: "0 1 * * *"      # 1 AM daily
    mode: one-way
    
    filters:
      # Exclude test and temporary secrets
      - source_regex: "^test-"
        action: exclude
      - source_regex: "^temp-"
        action: exclude
      - source_regex: "^debug-"
        action: exclude
    
    transforms:
      - match: ".*"
        replace: "backup-{timestamp}/{source_name}"
    
    options:
      batch_size: 50
      on_error: continue
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
  "target": "vault-2",
  "schedule": "0 * * * *",
  "mode": "one-way",
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
