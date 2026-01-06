# TQServer Deployment Guide

## Overview

TQServer uses rsync-based incremental deployment to efficiently transfer binaries and configuration files to remote servers. The deployment system supports multiple environments and selective worker deployment.

## Directory Structure

```
scripts/
  deploy.sh           # Main deployment script
  hooks/
    pre-deploy.sh     # Pre-deployment checks
    post-deploy.sh    # Post-deployment actions
config/
  deployment.yaml     # Deployment configuration
  server.yaml         # Server configuration
workers/
  {name}/
    config/           # Worker-specific configuration
    views/            # HTML templates
    data/             # Worker data files
```

## Configuration

Edit `config/deployment.yaml` to configure your deployment targets:

```yaml
targets:
  staging:
    server:
      address: staging.example.com
      user: deploy
      path: /opt/tqserver
    workers:
      - name: index
        path: /opt/tqserver/workers/index
```

### Configuration Options

- **targets**: Define deployment environments (staging, production, development)
- **rsync.options**: rsync command-line options
- **rsync.excludes**: Files/directories to exclude from deployment
- **pre_deploy_checks**: Validation steps before deployment
- **post_deploy_actions**: Actions after deployment completes
- **backup**: Backup settings for remote files
- **notifications**: Optional notification settings

## Building for Deployment

### Production Build

Build all binaries for production:

```bash
./scripts/build-prod.sh
```

This creates:
- `server/bin/tqserver` - Main server binary
- `workers/{name}/bin/tqworker_{name}` - Worker binaries

### Development Build

For development with file watching:

```bash
./scripts/build-dev.sh
```

## Deployment Commands

### Deploy Everything

Deploy server and all workers to a target:

```bash
./scripts/deploy.sh production
```

### Deploy Specific Worker

Deploy only a specific worker:

```bash
./scripts/deploy.sh production index
```

### Deploy with Custom Target

Deploy to a custom server:

```bash
./scripts/deploy.sh custom user@hostname:/path
```

## Deployment Process

1. **Pre-deployment Checks** (`hooks/pre-deploy.sh`)
   - Verifies local binaries exist
   - Checks configuration files
   - Optionally runs tests
   - Creates backup timestamp

2. **Server Deployment**
   - Syncs server binary to remote
   - Creates necessary directories
   - Preserves file permissions

3. **Worker Deployment**
   - Syncs worker binaries and resources
   - Deploys public and private files
   - Maintains directory structure

4. **Configuration Deployment**
   - Syncs server.yaml to remote

5. **Post-deployment Actions** (`hooks/post-deploy.sh`)
   - Sends SIGHUP to server (production reload)
   - Performs health checks
   - Optionally sends notifications

## Production Mode Reload

In production mode, the server listens for SIGHUP signals to trigger zero-downtime reloads:

```bash
# On remote server
pkill -SIGHUP tqserver
```

The server will:
1. Detect worker binary changes via mtime comparison
2. Build new worker instances
3. Switch to new instances atomically
4. Clean up old instances

## Development Mode

For local development with automatic reloading:

```bash
./server/bin/tqserver --mode dev
```

This enables:
- File watching on worker source files
- Automatic rebuilds on changes
- Hot reloading of workers
- No manual deployment needed

## Deployment Hooks

### Pre-deployment Hook

Customize `scripts/hooks/pre-deploy.sh` to:
- Run additional tests
- Validate configuration
- Check dependencies
- Create database backups

### Post-deployment Hook

Customize `scripts/hooks/post-deploy.sh` to:
- Run database migrations
- Clear caches
- Update service status
- Send notifications (Slack, email)

## Remote Server Setup

### Directory Structure

The remote server should have this structure:

```
/opt/tqserver/
  server/
    bin/
      tqserver
  workers/
    index/
      bin/
        tqworker_index
      public/
      views/
      config/
      data/
  config/
    server.yaml
  logs/
```

### Systemd Service

Create `/etc/systemd/system/tqserver.service`:

```ini
[Unit]
Description=TQServer
After=network.target

[Service]
Type=simple
User=tqserver
WorkingDirectory=/opt/tqserver
ExecStart=/opt/tqserver/server/bin/tqserver --mode prod
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
```

Enable and start:

```bash
sudo systemctl enable tqserver
sudo systemctl start tqserver
```

## Health Checks

The server exposes a health endpoint:

```bash
curl http://localhost:8080/health
```

Response:
```json
{
  "status": "healthy",
  "workers": {
    "index": "healthy"
  }
}
```

## Troubleshooting

### Deployment Fails

1. Check SSH connectivity:
   ```bash
   ssh user@hostname echo "Connected"
   ```

2. Verify remote directories exist:
   ```bash
   ssh user@hostname ls -la /opt/tqserver
   ```

3. Check rsync output for errors

### Workers Not Reloading

1. Verify SIGHUP signal was received:
   ```bash
   # Check server logs
   tail -f /opt/tqserver/logs/server.log
   ```

2. Verify worker binary timestamps changed:
   ```bash
   ssh user@hostname ls -l /opt/tqserver/workers/*/bin/*
   ```

3. Check worker health:
   ```bash
   curl http://hostname:8080/health
   ```

### Health Check Failures

1. Check if server is running:
   ```bash
   ssh user@hostname ps aux | grep tqserver
   ```

2. Check server logs for errors:
   ```bash
   ssh user@hostname tail -100 /opt/tqserver/logs/server.log
   ```

3. Verify port is accessible:
   ```bash
   telnet hostname 8080
   ```

## Rollback Procedure

If deployment fails, rollback to previous version:

```bash
# On remote server
cd /opt/tqserver/backups/$(ls -t | head -1)
cp -r * /opt/tqserver/
sudo systemctl restart tqserver
```

## Best Practices

1. **Always test in staging first** before production deployment
2. **Run pre-deployment checks** to catch issues early
3. **Monitor health checks** after deployment
4. **Keep backups** of previous versions
5. **Use version tags** for production deployments
6. **Document changes** in deployment notes
7. **Test rollback procedures** regularly

## Security

1. Use SSH key-based authentication (no passwords)
2. Restrict deployment user permissions
3. Use firewall rules to limit access
4. Keep deployment configs out of version control (`.gitignore`)
5. Rotate SSH keys regularly

## Monitoring

Monitor these metrics after deployment:

- Server uptime and restarts
- Worker health status
- Response times
- Error rates
- Resource usage (CPU, memory)

## Additional Resources

- [Development Mode Documentation](README.md#development-mode)
- [Production Mode Documentation](README.md#production-mode)
- [Worker Development Guide](README.md#worker-development)
- [Configuration Reference](config/server.example.yaml)
