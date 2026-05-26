#!/bin/bash

# Test script for Secrets Vault Sync Daemon with Vaultwarden
# This script sets up two Vaultwarden instances, injects test secrets, and tests synchronization

set -e  # Exit on error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
SOURCE_VAULT="http://localhost:8000"
TARGET_VAULT="http://localhost:8001"
SYNC_DAEMON="http://localhost:8080"
SOURCE_TOKEN="source_admin_token_12345"
TARGET_TOKEN="target_admin_token_12345"
ADMIN_USER="admin"
ADMIN_PASS="adminpassword"
TIMEOUT=300  # 5 minutes timeout
SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
COMPOSE_FILE="${SCRIPT_DIR}/docker-compose.test.yml"
AUTH_TOKEN=""

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Wait for a service to be healthy
wait_for_service() {
    local url=$1
    local service_name=$2
    local elapsed=0
    local interval=2

    log_info "Waiting for $service_name to be healthy..."

    while [ $elapsed -lt $TIMEOUT ]; do
        if curl -s -f "$url/alive" > /dev/null 2>&1; then
            log_success "$service_name is ready"
            return 0
        fi

        sleep $interval
        elapsed=$((elapsed + interval))
        echo -ne "${YELLOW}[WAITING]${NC} Elapsed: ${elapsed}s...\r"
    done

    log_error "$service_name did not become healthy within ${TIMEOUT}s"
    return 1
}

# Wait for sync daemon setup endpoint to become available
wait_for_sync_daemon() {
    local elapsed=0
    local interval=2

    log_info "Waiting for sync daemon to be healthy..."

    while [ $elapsed -lt $TIMEOUT ]; do
        if curl -s -f "$SYNC_DAEMON/api/setup" > /dev/null 2>&1; then
            log_success "Sync daemon is ready"
            return 0
        fi

        sleep $interval
        elapsed=$((elapsed + interval))
        echo -ne "${YELLOW}[WAITING]${NC} Elapsed: ${elapsed}s...\r"
    done

    log_error "Sync daemon did not become healthy within ${TIMEOUT}s"
    return 1
}

# Configure the sync daemon via API (setup, login, create vaults + syncs)
setup_daemon_config() {
    log_info "Running first-run setup for sync daemon..."

    # POST /api/setup — create admin account
    local setup_resp
    setup_resp=$(curl -s -w "\n%{http_code}" -X POST "$SYNC_DAEMON/api/setup" \
        -H "Content-Type: application/json" \
        -d "{\"admin_username\":\"$ADMIN_USER\",\"admin_password\":\"$ADMIN_PASS\"}")
    local setup_code
    setup_code=$(echo "$setup_resp" | tail -1)
    if [ "$setup_code" != "200" ]; then
        log_error "Setup failed (HTTP $setup_code): $(echo "$setup_resp" | head -1)"
        return 1
    fi
    log_success "Admin account created"

    # POST /api/auth/login — get session token
    local login_resp
    login_resp=$(curl -s -X POST "$SYNC_DAEMON/api/auth/login" \
        -H "Content-Type: application/json" \
        -d "{\"username\":\"$ADMIN_USER\",\"password\":\"$ADMIN_PASS\"}")
    AUTH_TOKEN=$(echo "$login_resp" | grep -o '"token":"[^"]*"' | cut -d'"' -f4)
    if [ -z "$AUTH_TOKEN" ]; then
        log_error "Login failed: $login_resp"
        return 1
    fi
    log_success "Logged in; got session token"

    # POST /api/config/vaults — create source vault
    local source_vault_json
    source_vault_json=$(cat <<EOF
{
  "ID": "vaultwarden_source",
  "Name": "Source Mock Vault",
  "Type": "vaultwarden",
  "Endpoint": "http://mock-vaults:8000",
  "Method": "POST",
  "Auth": {
    "Method": "bearer",
    "Headers": {"token": "$SOURCE_TOKEN"}
  },
  "FieldNames": {"NameField": "name", "ValueField": "login"},
  "Headers": {"Accept": "application/json", "Content-Type": "application/json"},
  "Timeout": 30,
  "SkipSSLVerify": true
}
EOF
)
    local vault_resp
    vault_resp=$(curl -s -w "\n%{http_code}" -X POST "$SYNC_DAEMON/api/config/vaults" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer $AUTH_TOKEN" \
        -d "$source_vault_json")
    local vault_code
    vault_code=$(echo "$vault_resp" | tail -1)
    if [ "$vault_code" != "201" ]; then
        log_error "Source vault creation failed (HTTP $vault_code): $(echo "$vault_resp" | head -1)"
        return 1
    fi
    log_success "Source vault created"

    # POST /api/config/vaults — create target vault
    local target_vault_json
    target_vault_json=$(cat <<EOF
{
  "ID": "vaultwarden_target",
  "Name": "Target Mock Vault",
  "Type": "vaultwarden",
  "Endpoint": "http://mock-vaults:8001",
  "Method": "POST",
  "Auth": {
    "Method": "bearer",
    "Headers": {"token": "$TARGET_TOKEN"}
  },
  "FieldNames": {"NameField": "name", "ValueField": "login"},
  "Headers": {"Accept": "application/json", "Content-Type": "application/json"},
  "Timeout": 30,
  "SkipSSLVerify": true
}
EOF
)
    vault_resp=$(curl -s -w "\n%{http_code}" -X POST "$SYNC_DAEMON/api/config/vaults" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer $AUTH_TOKEN" \
        -d "$target_vault_json")
    vault_code=$(echo "$vault_resp" | tail -1)
    if [ "$vault_code" != "201" ]; then
        log_error "Target vault creation failed (HTTP $vault_code): $(echo "$vault_resp" | head -1)"
        return 1
    fi
    log_success "Target vault created"

    # POST /api/config/syncs — create sync
    local enabled=true
    local sync_json
    sync_json=$(cat <<EOF
{
  "ID": "vaultwarden_sync",
  "Source": "vaultwarden_source",
  "Targets": ["vaultwarden_target"],
  "SyncType": "unidirectional",
  "Enabled": $enabled,
  "Schedule": "*/5 * * * *",
  "Filter": {"Patterns": ["*"], "Exclude": []},
  "RetryPolicy": {"MaxRetries": 3, "InitialBackoff": 1000, "MaxBackoff": 30000, "Multiplier": 2.0}
}
EOF
)
    local sync_resp
    sync_resp=$(curl -s -w "\n%{http_code}" -X POST "$SYNC_DAEMON/api/config/syncs" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer $AUTH_TOKEN" \
        -d "$sync_json")
    local sync_code
    sync_code=$(echo "$sync_resp" | tail -1)
    if [ "$sync_code" != "201" ]; then
        log_error "Sync creation failed (HTTP $sync_code): $(echo "$sync_resp" | head -1)"
        return 1
    fi
    log_success "Sync job created"

    log_info "Waiting for daemon to reload configuration..."
    sleep 3
}

# Get list of ciphers from Vaultwarden
list_ciphers() {
    local vault_url=$1
    local token=$2

    local response=$(curl -s -X GET "$vault_url/api/ciphers" \
        -H "Authorization: Bearer $token" \
        -H "Content-Type: application/json")

    echo "$response"
}

# Create a cipher (secret) in Vaultwarden
create_cipher() {
    local vault_url=$1
    local token=$2
    local name=$3
    local username=$4
    local password=$5

    # Vaultwarden/Bitwarden API format for login ciphers
    local payload=$(cat <<EOF
{
  "type": 1,
  "name": "$name",
  "login": {
    "username": "$username",
    "password": "$password"
  },
  "organizationId": null
}
EOF
)

    local response=$(curl -s -X POST "$vault_url/api/ciphers" \
        -H "Authorization: Bearer $token" \
        -H "Content-Type: application/json" \
        -d "$payload")

    echo "$response"
}

# Inject test secrets into source vault
inject_test_secrets() {
    log_info "Injecting test secrets into source Vaultwarden..."

    # Create test secrets
    declare -a secrets=(
        "db-password:dbuser:secret123pass"
        "api-key:apiuser:abc123def456"
        "aws-access-key:awsuser:AKIAIOSFODNN7EXAMPLE"
    )

    for secret in "${secrets[@]}"; do
        IFS=':' read -r name username password <<< "$secret"
        log_info "Creating secret: $name"

        response=$(create_cipher "$SOURCE_VAULT" "$SOURCE_TOKEN" "$name" "$username" "$password")

        if echo "$response" | grep -q "\"id\""; then
            log_success "Created secret: $name"
        else
            log_warning "Response: $response"
        fi
    done
}

# Wait for secrets to appear in target vault
verify_sync() {
    log_info "Verifying secrets were synced to target vault..."

    local elapsed=0
    local interval=5
    local max_wait=120  # 2 minutes

    while [ $elapsed -lt $max_wait ]; do
        log_info "Checking target vault (attempt $((elapsed / interval + 1)))..."

        local source_ciphers=$(list_ciphers "$SOURCE_VAULT" "$SOURCE_TOKEN")
        local target_ciphers=$(list_ciphers "$TARGET_VAULT" "$TARGET_TOKEN")

        local source_count=$(echo "$source_ciphers" | grep -o '"type"' | wc -l)
        local target_count=$(echo "$target_ciphers" | grep -o '"type"' | wc -l)

        log_info "Source vault has $source_count secrets, Target vault has $target_count secrets"

        if [ "$source_count" -gt 0 ] && [ "$target_count" -gt 0 ]; then
            log_success "Secrets synced! Source: $source_count, Target: $target_count"
            return 0
        fi

        sleep $interval
        elapsed=$((elapsed + interval))
    done

    log_error "Timeout waiting for secrets to sync"
    return 1
}

# Get sync status from daemon
check_sync_status() {
    log_info "Checking sync daemon status..."

    local response=$(curl -s -X GET "$SYNC_DAEMON/api/syncs/vaultwarden_sync/status" \
        -H "Authorization: Bearer $AUTH_TOKEN")

    echo "$response" | jq . 2>/dev/null || echo "$response"
}

# Main test execution
main() {
    log_info "=========================================="
    log_info "Secrets Vault Sync Daemon - Integration Test"
    log_info "=========================================="
    echo

    # Step 1: Start services
    log_info "Starting Docker Compose services..."
    docker compose -f "${COMPOSE_FILE}" up -d

    echo
    log_info "Waiting for all services to be ready..."
    echo

    # Step 2: Wait for Vaultwarden instances
    wait_for_service "$SOURCE_VAULT" "Source Vaultwarden" || exit 1
    echo
    wait_for_service "$TARGET_VAULT" "Target Vaultwarden" || exit 1
    echo

    # Step 3: Wait for sync daemon API to be ready
    wait_for_sync_daemon || exit 1
    echo

    # Step 4: Configure the daemon via API (setup admin + create vaults/syncs)
    setup_daemon_config || exit 1
    echo

    # Step 5: List initial state
    log_info "Initial state of vaults:"
    local source_initial=$(list_ciphers "$SOURCE_VAULT" "$SOURCE_TOKEN")
    local target_initial=$(list_ciphers "$TARGET_VAULT" "$TARGET_TOKEN")

    log_info "Source vault: $(echo "$source_initial" | grep -o '"type"' | wc -l) items"
    log_info "Target vault: $(echo "$target_initial" | grep -o '"type"' | wc -l) items"
    echo

    # Step 6: Inject secrets
    inject_test_secrets
    echo

    # Step 7: Verify secrets in source
    log_info "Verifying secrets in source vault..."
    local source_after=$(list_ciphers "$SOURCE_VAULT" "$SOURCE_TOKEN")
    log_info "Source vault after injection: $(echo "$source_after" | grep -o '"type"' | wc -l) items"
    echo

    # Step 8: Trigger manual sync
    log_info "Triggering manual sync..."
    curl -s -X POST "$SYNC_DAEMON/api/syncs/vaultwarden_sync/execute" \
        -H "Authorization: Bearer $AUTH_TOKEN" | jq . 2>/dev/null || true
    echo

    # Step 9: Verify sync
    echo
    verify_sync || exit 1
    echo

    # Step 10: Display sync status
    log_info "Final sync status:"
    echo
    check_sync_status
    echo

    log_success "=========================================="
    log_success "Integration test completed successfully!"
    log_success "=========================================="
    echo
    log_info "Available endpoints:"
    log_info "  Setup:         curl http://localhost:8080/api/setup"
    log_info "  Sync Status:   curl http://localhost:8080/api/syncs/vaultwarden_sync/status"
    log_info "  Metrics:       curl http://localhost:9090/metrics"
    log_info "  Source UI:     http://localhost:8000"
    log_info "  Target UI:     http://localhost:8001"
    echo
    log_info "To stop services: docker compose -f ${COMPOSE_FILE} down"
}

# Run if not sourced
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi
