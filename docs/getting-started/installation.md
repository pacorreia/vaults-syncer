# Installation

Choose your installation method based on your environment and preferences.

## Docker (Recommended)

### Docker Run

Quick start with a single command:

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

For production deployments:

```yaml
services:
  sync-daemon:
    image: ghcr.io/pacorreia/vaults-syncer:latest
    container_name: vaults-syncer
    restart: unless-stopped
    volumes:
      - sync-data:/app/data          # use a named volume, not a host path
    ports:
      - "8080:8080"  # HTTP API + Web UI
      - "9090:9090"  # Prometheus metrics
    environment:
      - MASTER_ENCRYPTION_KEY=${MASTER_ENCRYPTION_KEY}
      - DB_TYPE=sqlite               # or postgres / mssql
    healthcheck:
      test: ["CMD", "wget", "-qO-", "http://localhost:8080/api/setup"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 10s

volumes:
  sync-data:
```

Run with:

```bash
docker compose up -d
```

### Kubernetes

Deploy on Kubernetes with Helm:

```bash
helm repo add akv-sync https://charts.example.com
helm repo update
helm install akv-sync akv-sync/akv-sync \
  -n akv-sync \
  --create-namespace \
  -f values.yaml
```

Or with kubectl directly:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: vaults-syncer
spec:
  replicas: 1
  selector:
    matchLabels:
      app: vaults-syncer
  template:
    metadata:
      labels:
        app: vaults-syncer
    spec:
      containers:
      - name: sync-daemon
        image: ghcr.io/pacorreia/vaults-syncer:latest
        ports:
        - containerPort: 8080
        - containerPort: 9090
        env:
        - name: MASTER_ENCRYPTION_KEY
          valueFrom:
            secretKeyRef:
              name: vaults-syncer-secret
              key: master-encryption-key
        volumeMounts:
        - name: data
          mountPath: /app/data
        livenessProbe:
          httpGet:
            path: /api/setup
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
      volumes:
      - name: data
        persistentVolumeClaim:
          claimName: vaults-syncer-data
```

## Binary Release

Download precompiled binaries for your platform:

### Linux (amd64)

```bash
wget https://github.com/pacorreia/vaults-syncer/releases/download/v$(VERSION)/sync-daemon-linux-amd64
chmod +x sync-daemon-linux-amd64
export MASTER_ENCRYPTION_KEY=<your-key>
./sync-daemon-linux-amd64
```

### macOS (arm64)

```bash
wget https://github.com/pacorreia/vaults-syncer/releases/download/v$(VERSION)/sync-daemon-darwin-arm64
chmod +x sync-daemon-darwin-arm64
export MASTER_ENCRYPTION_KEY=<your-key>
./sync-daemon-darwin-arm64
```

### Windows (amd64)

```powershell
# Download from releases page or use:
Invoke-WebRequest -Uri "https://github.com/pacorreia/vaults-syncer/releases/download/v$(VERSION)/sync-daemon-windows-amd64.exe" -OutFile sync-daemon.exe

# Set env and run
$env:MASTER_ENCRYPTION_KEY = "<your-key>"
.\sync-daemon.exe
```

## Build from Source

### Prerequisites

- Go 1.26 or later (with CGO support for sqlite3; requires `libsqlite3-dev` on Debian/Ubuntu or `sqlite` via Homebrew on macOS)
- Git

### Clone Repository

```bash
git clone https://github.com/pacorreia/vaults-syncer
cd vaults-syncer
```

### Build Binary

```bash
# CGO_ENABLED=1 is required for the sqlite3 driver
CGO_ENABLED=1 go build -o sync-daemon .
```

### Run

```bash
export MASTER_ENCRYPTION_KEY=<your-key>   # Required after first start
./sync-daemon
```

### Build Docker Image

```bash
docker build -t vaults-syncer:latest .
docker run -d \
  -v sync-data:/app/data \
  -p 8080:8080 \
  -e MASTER_ENCRYPTION_KEY=${MASTER_ENCRYPTION_KEY} \
  vaults-syncer:latest
```

## Systemd Service

Create `/etc/systemd/system/vaults-syncer.service`:

```ini
[Unit]
Description=Secrets Vault Sync Daemon
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=vaults-syncer
Group=vaults-syncer
WorkingDirectory=/opt/vaults-syncer
EnvironmentFile=/etc/vaults-syncer/env
ExecStart=/opt/vaults-syncer/sync-daemon
Restart=on-failure
RestartSec=5s

# Security hardening
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true

[Install]
WantedBy=multi-user.target
```

Create `/etc/vaults-syncer/env`:

```bash
# Required after first start (see logs for generated key)
MASTER_ENCRYPTION_KEY=<your-key>
DB_TYPE=sqlite
DB_PATH=/opt/vaults-syncer/data/sync.db
SERVER_PORT=8080
METRICS_PORT=9090
```

Setup:

```bash
# Create user
sudo useradd -r -s /bin/false vaults-syncer

# Install binary
sudo mkdir -p /opt/vaults-syncer/data
sudo cp sync-daemon /opt/vaults-syncer/
sudo chown -R vaults-syncer:vaults-syncer /opt/vaults-syncer

# Install env file (protect secrets)
sudo mkdir -p /etc/vaults-syncer
sudo cp env /etc/vaults-syncer/
sudo chown root:vaults-syncer /etc/vaults-syncer/env
sudo chmod 640 /etc/vaults-syncer/env

# Enable service
sudo systemctl daemon-reload
sudo systemctl enable vaults-syncer
sudo systemctl start vaults-syncer

# Check status
sudo systemctl status vaults-syncer
```

## Verify Installation

### Setup Check (no auth required)

```bash
curl http://localhost:8080/api/setup
```

### Login and Check Health

```bash
TOKEN=$(curl -s -X POST http://localhost:8080/api/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"your-password"}' | jq -r .token)

curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/health
```

### List Syncs

```bash
curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/syncs
```

### View Metrics

```bash
curl http://localhost:9090/metrics
```

## Next Steps

- [Configure Vaults](../configuration/vaults.md)
- [Setup Authentication](../configuration/authentication.md)
- [Create Syncs](../configuration/syncs.md)
