# Deployment

- [Introduction](#introduction)
- [Production Checklist](#production-checklist)
- [Build for Production](#build-for-production)
- [Systemd Service](#systemd-service)
- [Reverse Proxy Setup](#reverse-proxy-setup)
- [Docker Deployment](#docker-deployment)
- [Cloud Deployment](#cloud-deployment)
- [Monitoring and Maintenance](#monitoring-and-maintenance)

## Introduction

Deploying TQServer to production requires careful planning and configuration to ensure reliability, security, and performance. This guide covers various deployment strategies and best practices.

## Production Checklist

Before deploying to production:

### Security
- [ ] Enable HTTPS/TLS
- [ ] Set secure file permissions
- [ ] Use environment variables for secrets
- [ ] Disable debug mode
- [ ] Configure firewall rules
- [ ] Set up rate limiting
- [ ] Enable CORS properly
- [ ] Review authentication mechanisms

### Performance
- [ ] Optimize worker build settings
- [ ] Configure appropriate timeouts
- [ ] Set resource limits
- [ ] Enable connection pooling
- [ ] Configure caching
- [ ] Optimize database queries

### Reliability
- [ ] Set up health checks
- [ ] Configure log rotation
- [ ] Set up monitoring
- [ ] Configure alerting
- [ ] Test graceful shutdown
- [ ] Plan backup strategy

### Operations
- [ ] Document deployment process
- [ ] Set up CI/CD pipeline
- [ ] Configure log aggregation
- [ ] Set up error tracking
- [ ] Plan rollback procedure
- [ ] Test disaster recovery

## Build for Production

### Optimized Build

```bash
#!/bin/bash
# scripts/build-prod.sh

set -e

echo "Building TQServer for production..."

# Build with optimizations
go build -o bin/tqserver \
    -ldflags="-s -w" \
    -trimpath \
    ./cmd/tqserver

# Build all workers
for worker in workers/*/; do
    if [ -d "$worker/src" ]; then
        name=$(basename "$worker")
        echo "Building worker: $name"
        
        go build -o "$worker/bin/$name" \
            -ldflags="-s -w" \
            -trimpath \
            "$worker/src"
    fi
done

echo "Build complete!"
```

Build flags explained:
- `-ldflags="-s -w"`: Strip debugging information (smaller binaries)
- `-trimpath`: Remove file system paths from binary
- Optimizations reduce binary size by 30-40%

### Version Information

```go
// cmd/tqserver/version.go
package main

var (
    Version   = "dev"
    GitCommit = "unknown"
    BuildTime = "unknown"
)
```

```bash
# Build with version info
go build -o bin/tqserver \
    -ldflags="-s -w \
    -X main.Version=$(git describe --tags) \
    -X main.GitCommit=$(git rev-parse HEAD) \
    -X main.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    ./cmd/tqserver
```

### Configuration for Production

```yaml
# config/server.prod.yaml
server:
  host: "0.0.0.0"
  port: 8080
  log_file: "/var/log/tqserver/server-{date}.log"
  read_timeout: 30s
  write_timeout: 30s
  idle_timeout: 120s
  
  port_pool:
    start: 9000
    end: 9100

workers:
  base_path: "/opt/tqserver/workers"
  
  health_check:
    enabled: true
    interval: 60s
    timeout: 10s
    failure_threshold: 5
  
  file_watcher:
    enabled: false  # Disable hot reload in production

cleanup:
  enabled: true
  interval: 6h
  max_age: 168h  # 7 days
```

## Systemd Service

### Service File

Create `/etc/systemd/system/tqserver.service`:

```ini
[Unit]
Description=TQServer - High-performance function execution platform
After=network.target
Documentation=https://tqserver.dev

[Service]
Type=simple
User=tqserver
Group=tqserver
WorkingDirectory=/opt/tqserver
ExecStart=/usr/local/bin/tqserver -config=/opt/tqserver/config/server.yaml
ExecReload=/bin/kill -HUP $MAINPID
Restart=on-failure
RestartSec=5s
LimitNOFILE=65536

# Security
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/opt/tqserver/logs /opt/tqserver/workers

# Environment
Environment="TQ_MODE=production"
EnvironmentFile=-/etc/tqserver/environment

[Install]
WantedBy=multi-user.target
```

### Setup

```bash
# Create user
sudo useradd -r -s /bin/false tqserver

# Create directories
sudo mkdir -p /opt/tqserver/{config,logs,workers}
sudo mkdir -p /var/log/tqserver

# Copy files
sudo cp bin/tqserver /usr/local/bin/
sudo cp -r workers /opt/tqserver/
sudo cp config/server.prod.yaml /opt/tqserver/config/server.yaml

# Set permissions
sudo chown -R tqserver:tqserver /opt/tqserver
sudo chown -R tqserver:tqserver /var/log/tqserver
sudo chmod 755 /usr/local/bin/tqserver

# Enable and start service
sudo systemctl daemon-reload
sudo systemctl enable tqserver
sudo systemctl start tqserver

# Check status
sudo systemctl status tqserver
```

### Service Management

```bash
# Start service
sudo systemctl start tqserver

# Stop service
sudo systemctl stop tqserver

# Restart service
sudo systemctl restart tqserver

# Reload configuration (graceful)
sudo systemctl reload tqserver

# View logs
sudo journalctl -u tqserver -f

# View recent errors
sudo journalctl -u tqserver -p err -n 50
```

## Reverse Proxy Setup

### Nginx

```nginx
# /etc/nginx/sites-available/tqserver
upstream tqserver {
    server 127.0.0.1:8080;
    keepalive 32;
}

server {
    listen 80;
    server_name example.com www.example.com;
    
    # Redirect to HTTPS
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    server_name example.com www.example.com;
    
    # SSL Configuration
    ssl_certificate /etc/letsencrypt/live/example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/example.com/privkey.pem;
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;
    ssl_prefer_server_ciphers on;
    
    # Security Headers
    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header X-XSS-Protection "1; mode=block" always;
    
    # Logging
    access_log /var/log/nginx/tqserver-access.log;
    error_log /var/log/nginx/tqserver-error.log;
    
    # Client settings
    client_max_body_size 50M;
    client_body_buffer_size 128k;
    
    # Proxy settings
    location / {
        proxy_pass http://tqserver;
        proxy_http_version 1.1;
        
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header Connection "";
        
        proxy_buffering off;
        proxy_request_buffering off;
        
        proxy_connect_timeout 30s;
        proxy_send_timeout 30s;
        proxy_read_timeout 30s;
    }
    
    # WebSocket support (future)
    location /ws {
        proxy_pass http://tqserver;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_read_timeout 86400;
    }
    
    # Static files (optional, if serving directly)
    location /static {
        alias /opt/tqserver/public;
        expires 1y;
        add_header Cache-Control "public, immutable";
    }
}
```

```bash
# Enable site
sudo ln -s /etc/nginx/sites-available/tqserver /etc/nginx/sites-enabled/
sudo nginx -t
sudo systemctl reload nginx
```

### Caddy

```caddyfile
# /etc/caddy/Caddyfile
example.com {
    reverse_proxy localhost:8080
    
    # Automatic HTTPS with Let's Encrypt
    tls {
        protocols tls1.2 tls1.3
    }
    
    # Headers
    header {
        Strict-Transport-Security "max-age=31536000; includeSubDomains"
        X-Content-Type-Options "nosniff"
        X-Frame-Options "SAMEORIGIN"
        X-XSS-Protection "1; mode=block"
    }
    
    # Logging
    log {
        output file /var/log/caddy/tqserver.log
    }
}
```

## Docker Deployment

### Dockerfile

```dockerfile
# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -o tqserver \
    ./cmd/tqserver

# Build workers
RUN for worker in workers/*/; do \
    if [ -d "$worker/src" ]; then \
        name=$(basename "$worker"); \
        CGO_ENABLED=0 GOOS=linux go build \
            -ldflags="-s -w" \
            -o "$worker/bin/$name" \
            "$worker/src"; \
    fi \
done

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Copy binary and workers
COPY --from=builder /build/tqserver /app/
COPY --from=builder /build/workers /app/workers
COPY --from=builder /build/config /app/config

# Create non-root user
RUN addgroup -g 1000 tqserver && \
    adduser -D -u 1000 -G tqserver tqserver && \
    chown -R tqserver:tqserver /app

USER tqserver

EXPOSE 8080

CMD ["./tqserver", "-config=/app/config/server.yaml"]
```

### Docker Compose

```yaml
# docker-compose.yml
version: '3.8'

services:
  tqserver:
    build: .
    ports:
      - "8080:8080"
    environment:
      - TQ_MODE=production
      - DATABASE_URL=postgresql://user:pass@db:5432/mydb
    volumes:
      - ./config/server.yaml:/app/config/server.yaml:ro
      - logs:/app/logs
      - ./workers:/app/workers:ro
    restart: unless-stopped
    depends_on:
      - db
    healthcheck:
      test: ["CMD", "wget", "--quiet", "--tries=1", "--spider", "http://localhost:8080/health"]
      interval: 30s
      timeout: 10s
      retries: 3
  
  db:
    image: postgres:15-alpine
    environment:
      - POSTGRES_USER=user
      - POSTGRES_PASSWORD=pass
      - POSTGRES_DB=mydb
    volumes:
      - pgdata:/var/lib/postgresql/data
    restart: unless-stopped

volumes:
  logs:
  pgdata:
```

### Build and Run

```bash
# Build image
docker build -t tqserver:latest .

# Run container
docker run -d \
    --name tqserver \
    -p 8080:8080 \
    -v $(pwd)/config:/app/config:ro \
    -e TQ_MODE=production \
    tqserver:latest

# Or use compose
docker-compose up -d

# View logs
docker-compose logs -f tqserver

# Scale workers (future feature)
docker-compose up -d --scale tqserver=3
```

## Cloud Deployment

### AWS EC2

```bash
#!/bin/bash
# deploy-aws.sh

# Launch EC2 instance
aws ec2 run-instances \
    --image-id ami-0c55b159cbfafe1f0 \
    --instance-type t3.medium \
    --key-name my-key \
    --security-group-ids sg-xxxxx \
    --subnet-id subnet-xxxxx \
    --tag-specifications 'ResourceType=instance,Tags=[{Key=Name,Value=tqserver}]'

# After instance is running, deploy
ssh ec2-user@instance-ip << 'EOF'
    # Install dependencies
    sudo yum update -y
    sudo yum install -y golang
    
    # Setup TQServer
    git clone https://github.com/mevdschee/tqserver.git
    cd tqserver
    ./scripts/build-prod.sh
    sudo ./scripts/install-systemd.sh
EOF
```

### Google Cloud Platform

```bash
# Create instance
gcloud compute instances create tqserver \
    --image-family=ubuntu-2004-lts \
    --image-project=ubuntu-os-cloud \
    --machine-type=e2-medium \
    --zone=us-central1-a \
    --tags=http-server,https-server

# Deploy
gcloud compute scp --recurse . tqserver:~/tqserver
gcloud compute ssh tqserver << 'EOF'
    cd tqserver
    ./scripts/build-prod.sh
    sudo ./scripts/install-systemd.sh
EOF
```

### DigitalOcean

```bash
# Create droplet via API or UI
# Then deploy with this script

#!/bin/bash
HOST="droplet-ip"

# Copy files
rsync -avz --exclude 'bin' ./ root@$HOST:/opt/tqserver/

# Setup
ssh root@$HOST << 'EOF'
    cd /opt/tqserver
    apt-get update
    apt-get install -y golang-go
    ./scripts/build-prod.sh
    ./scripts/install-systemd.sh
    systemctl start tqserver
EOF
```

## Monitoring and Maintenance

### Health Monitoring

```bash
# Simple health check script
#!/bin/bash
# scripts/healthcheck.sh

URL="http://localhost:8080/health"
TIMEOUT=5

if curl -f -s --max-time $TIMEOUT "$URL" > /dev/null; then
    echo "OK: Service is healthy"
    exit 0
else
    echo "ERROR: Service is unhealthy"
    exit 1
fi
```

Add to cron:
```bash
# Run every 5 minutes
*/5 * * * * /opt/tqserver/scripts/healthcheck.sh || systemctl restart tqserver
```

### Log Rotation

```bash
# /etc/logrotate.d/tqserver
/var/log/tqserver/*.log {
    daily
    rotate 14
    compress
    delaycompress
    notifempty
    create 0640 tqserver tqserver
    sharedscripts
    postrotate
        systemctl reload tqserver > /dev/null 2>&1 || true
    endscript
}
```

### Backup Script

```bash
#!/bin/bash
# scripts/backup.sh

BACKUP_DIR="/backups/tqserver"
DATE=$(date +%Y%m%d-%H%M%S)

mkdir -p "$BACKUP_DIR"

# Backup configuration
tar -czf "$BACKUP_DIR/config-$DATE.tar.gz" /opt/tqserver/config

# Backup workers
tar -czf "$BACKUP_DIR/workers-$DATE.tar.gz" /opt/tqserver/workers

# Cleanup old backups (keep 7 days)
find "$BACKUP_DIR" -name "*.tar.gz" -mtime +7 -delete
```

### Update Script

```bash
#!/bin/bash
# scripts/update.sh

set -e

cd /opt/tqserver

# Pull latest code
git fetch origin
git checkout $(git describe --tags $(git rev-list --tags --max-count=1))

# Build
./scripts/build-prod.sh

# Restart service
sudo systemctl restart tqserver

# Verify
sleep 5
if ! ./scripts/healthcheck.sh; then
    echo "Health check failed, rolling back"
    git checkout -
    ./scripts/build-prod.sh
    sudo systemctl restart tqserver
    exit 1
fi

echo "Update successful"
```

## Next Steps

- [Configuration](configuration.md) - Production configuration
- [Monitoring](../monitoring/logging.md) - Set up monitoring
- [Performance Tuning](../advanced/performance.md) - Optimize performance
- [Troubleshooting](../appendix/troubleshooting.md) - Common issues
