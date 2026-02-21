# Installation

Choose your installation method based on your environment and preferences.

## Docker (Recommended)

### Docker Run

Quick start with a single command:

```bash
docker run -d \
  --name akv-sync \
  -v $(pwd)/config.yaml:/etc/sync/config.yaml:ro \
  -v sync-data:/app/data \
  -p 8080:8080 \
  -p 9090:9090 \
  ghcr.io/pacorreia/vaults-syncer:latest
```

### Docker Compose

For production deployments:

```yaml
version: '3.8'

services:
  sync-daemon:
    image: ghcr.io/pacorreia/akv-vaultwarden-sync:latest
    container_name: akv-sync
    restart: unless-stopped
    volumes:
      - ./config.yaml:/etc/sync/config.yaml:ro
      - ./data:/app/data
    ports:
      - "8080:8080"  # HTTP API
      - "9090:9090"  # Metrics
    environment:
      - LOG_LEVEL=info
      - LOG_FORMAT=json
    networks:
      - backend
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 40s

networks:
  backend:
    driver: bridge
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
apiVersion: v1
kind: ConfigMap
metadata:
  name: akv-sync-config
data:
  config.yaml: |
    vaults:
      - id: source
        endpoint: https://vault.example.com/api/ciphers
        auth:
          method: oauth2
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: akv-sync
spec:
  replicas: 1
  selector:
    matchLabels:
      app: akv-sync
  template:
    metadata:
      labels:
        app: akv-sync
    spec:
      containers:
      - name: sync-daemon
        image: ghcr.io/pacorreia/vaults-syncer:latest
        ports:
        - containerPort: 8080
        - containerPort: 9090
        volumeMounts:
        - name: config
          mountPath: /etc/sync
          readOnly: true
        - name: data
          mountPath: /app/data
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
      volumes:
      - name: config
        configMap:
          name: akv-sync-config
      - name: data
        emptyDir: {}
```

## Binary Release

Download precompiled binaries for your platform:

### Linux (amd64)

```bash
wget https://github.com/pacorreia/vaults-syncer/releases/download/v$(VERSION)/sync-daemon-linux-amd64
chmod +x sync-daemon-linux-amd64
./sync-daemon-linux-amd64 -config config.yaml
```

### macOS (arm64)

```bash
wget https://github.com/pacorreia/vaults-syncer/releases/download/v$(VERSION)/sync-daemon-darwin-arm64
chmod +x sync-daemon-darwin-arm64
./sync-daemon-darwin-arm64 -config config.yaml
```

### Windows (amd64)

```powershell
# Download from releases page or use:
Invoke-WebRequest -Uri "https://github.com/pacorreia/vaults-syncer/releases/download/v$(VERSION)/sync-daemon-windows-amd64.exe" -OutFile sync-daemon.exe

# Run
.\sync-daemon.exe -config config.yaml
```

## Build from Source

### Prerequisites

- Go 1.22 or later
- Git

### Clone Repository

```bash
git clone https://github.com/pacorreia/vaults-syncer
cd vaults-syncer
```

### Build Binary

```bash
go build -o sync-daemon .
```

### Run

```bash
./sync-daemon -config config.yaml
```

### Build Docker Image

```bash
docker build -t vaults-syncer:latest .
docker run -d \
  -v $(pwd)/config.yaml:/etc/sync/config.yaml:ro \
  -v sync-data:/app/data \
  -p 8080:8080 \
  vaults-syncer:latest
```

## Systemd Service

Create `/etc/systemd/system/akv-sync.service`:

```ini
[Unit]
Description=AKV Vaultwarden Sync Daemon
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=akv-sync
Group=akv-sync
WorkingDirectory=/opt/akv-sync
ExecStart=/opt/akv-sync/sync-daemon -config /etc/akv-sync/config.yaml
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

Setup:

```bash
# Create user
sudo useradd -r -s /bin/false akv-sync

# Install binary
sudo mkdir -p /opt/akv-sync
sudo cp sync-daemon /opt/akv-sync/
sudo chown -R akv-sync:akv-sync /opt/akv-sync

# Install config
sudo mkdir -p /etc/akv-sync
sudo cp config.yaml /etc/akv-sync/
sudo chown -R akv-sync:akv-sync /etc/akv-sync
sudo chmod 600 /etc/akv-sync/config.yaml

# Enable service
sudo systemctl daemon-reload
sudo systemctl enable akv-sync
sudo systemctl start akv-sync

# Check status
sudo systemctl status akv-sync
```

## Verify Installation

### Health Check

```bash
curl http://localhost:8080/health
```

### List Syncs

```bash
curl http://localhost:8080/syncs
```

### View Metrics

```bash
curl http://localhost:9090/metrics
```

## Next Steps

- [Configure Vaults](../configuration/vaults.md)
- [Setup Authentication](../configuration/authentication.md)
- [Create Syncs](../configuration/syncs.md)
