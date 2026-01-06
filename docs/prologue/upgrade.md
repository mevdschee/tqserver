# Upgrade Guide

- [General Upgrade Instructions](#general-upgrade-instructions)
- [Upgrade Strategies](#upgrade-strategies)
- [Pre-Upgrade Checklist](#pre-upgrade-checklist)
- [Version-Specific Guides](#version-specific-guides)
- [Rollback Procedures](#rollback-procedures)
- [Common Issues](#common-issues)

## General Upgrade Instructions

Upgrading TQServer involves updating the server binary and ensuring your workers are compatible with the new version.

### Basic Upgrade Steps

1. **Backup Everything**
   ```bash
   # Backup configuration
   cp -r config config.backup
   
   # Backup workers
   cp -r workers workers.backup
   
   # Backup data (if applicable)
   tar -czf data-backup-$(date +%Y%m%d).tar.gz data/
   ```

2. **Check Release Notes**
   - Read the [Release Notes](releases.md) for breaking changes
   - Review new features and improvements
   - Note deprecated features
   - Check system requirements

3. **Update Dependencies**
   ```bash
   # Update Go version if required
   go version
   
   # Update project dependencies
   go get -u github.com/mevdschee/tqserver@latest
   go mod tidy
   ```

4. **Build New Version**
   ```bash
   # Build the server
   go build -o bin/tqserver ./cmd/tqserver
   
   # Verify build
   ./bin/tqserver -version
   ```

5. **Test in Development**
   ```bash
   # Start server in test mode
   ./bin/tqserver -config=config/server.test.yaml
   
   # Test your workers
   curl http://localhost:8080/health
   ```

6. **Deploy to Production**
   - Use graceful upgrade strategy (see below)
   - Monitor logs for errors
   - Check worker health
   - Verify functionality

## Upgrade Strategies

### Strategy 1: Rolling Upgrade (Zero Downtime)

Best for production environments with load balancing.

```bash
# 1. Deploy new version to staging
./deploy-staging.sh

# 2. Test thoroughly
./run-integration-tests.sh

# 3. Deploy to first production server
ssh prod-1 'systemctl stop tqserver'
scp bin/tqserver prod-1:/usr/local/bin/
ssh prod-1 'systemctl start tqserver'

# 4. Monitor for 5-10 minutes
./monitor.sh prod-1

# 5. Repeat for other servers
for server in prod-2 prod-3; do
    ssh $server 'systemctl stop tqserver'
    scp bin/tqserver $server:/usr/local/bin/
    ssh $server 'systemctl start tqserver'
    sleep 300  # Wait 5 minutes
done
```

### Strategy 2: Blue-Green Deployment

Run old and new versions simultaneously, then switch.

```bash
# 1. Deploy new version to green environment
./deploy-green.sh

# 2. Test green environment
./test-green.sh

# 3. Switch load balancer to green
./switch-to-green.sh

# 4. Monitor for issues
sleep 600  # Monitor for 10 minutes

# 5. Shutdown blue environment
./shutdown-blue.sh
```

### Strategy 3: In-Place Upgrade

Simplest but causes brief downtime.

```bash
# 1. Stop the server
systemctl stop tqserver

# 2. Backup current binary
cp /usr/local/bin/tqserver /usr/local/bin/tqserver.backup

# 3. Replace binary
cp bin/tqserver /usr/local/bin/tqserver

# 4. Start the server
systemctl start tqserver

# 5. Check status
systemctl status tqserver
journalctl -u tqserver -f
```

### Strategy 4: Graceful In-Place Upgrade

Minimize downtime with graceful shutdown.

```bash
#!/bin/bash
# upgrade.sh

# Send shutdown signal (allows workers to finish)
kill -TERM $(cat /var/run/tqserver.pid)

# Wait for graceful shutdown (max 30 seconds)
timeout=30
while [ $timeout -gt 0 ] && kill -0 $(cat /var/run/tqserver.pid) 2>/dev/null; do
    sleep 1
    ((timeout--))
done

# Force kill if still running
if kill -0 $(cat /var/run/tqserver.pid) 2>/dev/null; then
    kill -9 $(cat /var/run/tqserver.pid)
fi

# Replace binary
cp bin/tqserver /usr/local/bin/tqserver

# Start new version
systemctl start tqserver

# Verify startup
sleep 5
systemctl is-active tqserver || exit 1
```

## Pre-Upgrade Checklist

Before upgrading, ensure:

### System Requirements
- [ ] Go version meets minimum requirements
- [ ] Operating system is supported
- [ ] Sufficient disk space (check binaries)
- [ ] Memory requirements met
- [ ] Port availability confirmed

### Configuration
- [ ] Configuration file backed up
- [ ] Configuration validated for new version
- [ ] Environment variables documented
- [ ] Secrets/credentials secured
- [ ] TLS certificates valid (if applicable)

### Application
- [ ] Workers backed up
- [ ] Database migrations prepared (if applicable)
- [ ] Static assets backed up
- [ ] Custom templates backed up
- [ ] Third-party integrations tested

### Operations
- [ ] Maintenance window scheduled
- [ ] Team notified
- [ ] Monitoring alerts configured
- [ ] Rollback plan prepared
- [ ] Support on standby

### Testing
- [ ] Staging environment updated
- [ ] Integration tests passed
- [ ] Load tests performed
- [ ] Security scan completed
- [ ] User acceptance testing done

## Version-Specific Guides

### Upgrading to 1.0.0 (Planned Q2 2026)

**Breaking Changes**: TBD

**New Features**:
- TLS/HTTPS support
- Middleware system
- WebSocket support
- Enhanced CLI

**Migration Steps**:
1. Update configuration for TLS
2. Migrate custom authentication to middleware
3. Update worker code for new API
4. Test WebSocket endpoints

### Upgrading to 0.9.0 from 0.8.0

**Breaking Changes**:
- Configuration file format changed
- Worker registration API updated
- Environment variable prefix changed to `TQSERVER_`

**Migration Steps**:

1. **Update Configuration File**
   ```yaml
   # Old format (0.8.0)
   host: "0.0.0.0"
   port: 8080
   
   # New format (0.9.0)
   server:
     port: 3000
     read_timeout_seconds: 60
     write_timeout_seconds: 60
     log_file: "logs/tqserver_{date}.log"
   ```

2. **Update Environment Variables**
   ```bash
   # Old (0.8.0)
   export SERVER_PORT=8080
   
   # New (0.9.0)
   export TQSERVER_SERVER_PORT=8080
   ```

3. **Update Worker Health Checks**
   ```go
   // Old (0.8.0)
   func healthHandler(w http.ResponseWriter, r *http.Request) {
       w.Write([]byte("OK"))
   }
   
   // New (0.9.0)
   func healthHandler(w http.ResponseWriter, r *http.Request) {
       w.WriteHeader(http.StatusOK)
       json.NewEncoder(w).Encode(map[string]string{
           "status": "healthy",
       })
   }
   ```

4. **Rebuild All Workers**
   ```bash
   # Clean old binaries
   find workers -type d -name "bin" -exec rm -rf {} +
   
   # Rebuild
   ./scripts/build-dev.sh
   ```

### Upgrading to 0.8.0 from 0.7.0

**Breaking Changes**:
- Port allocation changed to dynamic
- Worker directory structure changed

**Migration Steps**:

1. **Reorganize Workers**
   ```bash
   # Old structure (0.7.0)
   pages/
     index/
       main.go
   
   # New structure (0.8.0)
   workers/
     index/
       src/
         main.go
       bin/
       public/
       views/
       config/
   ```

2. **Update Port Handling**
   ```go
   // Old (0.7.0)
   port := "9000"  // Hardcoded
   
   // New (0.8.0)
   port := os.Getenv("WORKER_PORT")  // Dynamic
   ```

## Rollback Procedures

If the upgrade fails, rollback to the previous version:

### Quick Rollback

```bash
#!/bin/bash
# rollback.sh

echo "Starting rollback..."

# Stop current version
systemctl stop tqserver

# Restore previous binary
cp /usr/local/bin/tqserver.backup /usr/local/bin/tqserver

# Restore configuration
cp -r config.backup/* config/

# Restore workers
cp -r workers.backup/* workers/

# Start previous version
systemctl start tqserver

# Verify
sleep 5
if systemctl is-active tqserver; then
    echo "Rollback successful"
    exit 0
else
    echo "Rollback failed"
    exit 1
fi
```

### Blue-Green Rollback

```bash
# Simply switch load balancer back to blue environment
./switch-to-blue.sh
```

### Database Rollback

If database migrations were applied:

```bash
# Rollback migrations
./migrate down

# Or restore from backup
psql -U user -d database < backup.sql
```

## Common Issues

### Issue: Configuration Validation Errors

**Symptom**: Server fails to start with config errors

**Solution**:
```bash
# Validate configuration
./bin/tqserver -validate -config=config/server.yaml

# Check configuration format
diff config/server.yaml config/server.example.yaml
```

### Issue: Worker Build Failures

**Symptom**: Workers fail to compile after upgrade

**Solution**:
```bash
# Update Go modules
cd workers/myworker
go mod tidy
go get -u

# Check Go version compatibility
go version
```

### Issue: Port Conflicts

**Symptom**: Workers fail to start due to port conflicts

**Solution**:
```yaml
# Increase port pool size in config/server.yaml
workers:
  port_range_start: 10000
  port_range_end: 20000  # Increased range
```

### Issue: Health Check Failures

**Symptom**: Workers marked as unhealthy after upgrade

**Solution**:
```bash
# Check health endpoint
curl http://localhost:PORT/health

# Review health check configuration
# Ensure worker implements health endpoint correctly
```

### Issue: Performance Degradation

**Symptom**: Slower response times after upgrade

**Solution**:
1. Check resource usage: `htop`
2. Review logs for errors
3. Profile the application (see [Profiling](../monitoring/profiling.md))
4. Check database connection pool
5. Verify worker count matches load

### Issue: Memory Leaks

**Symptom**: Memory usage grows over time

**Solution**:
```bash
# Check memory usage
ps aux | grep tqserver

# Profile memory
go tool pprof http://localhost:8080/debug/pprof/heap

# Restart workers periodically as workaround
# (Report to developers for fix)
```

## Post-Upgrade Tasks

After successful upgrade:

1. **Verify Functionality**
   - Test all routes
   - Check worker health
   - Verify integrations
   - Run smoke tests

2. **Monitor Performance**
   - CPU usage
   - Memory usage
   - Response times
   - Error rates

3. **Update Documentation**
   - Update version in docs
   - Document any configuration changes
   - Update deployment procedures
   - Share learnings with team

4. **Clean Up**
   ```bash
   # Remove backups after 7 days
   find . -name "*.backup" -mtime +7 -delete
   
   # Remove old binaries
   find workers -path "*/bin/*" -mtime +7 -delete
   ```

## Getting Help

If you encounter issues during upgrade:

1. Check [Troubleshooting Guide](../appendix/troubleshooting.md)
2. Search [GitHub Issues](https://github.com/mevdschee/tqserver/issues)
3. Ask on [Discord](https://discord.gg/tqserver)
4. Email support@tqserver.dev

## Next Steps

- [Release Notes](releases.md) - Check what's new
- [Configuration](../getting-started/configuration.md) - Update your config
- [Deployment](../getting-started/deployment.md) - Deploy to production
