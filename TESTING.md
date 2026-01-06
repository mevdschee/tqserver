# TQServer Testing Guide

This guide covers testing procedures for TQServer's development and production modes.

## Test Categories

### 1. Unit Tests

Run unit tests for individual packages:

```bash
# Test all packages
go test ./pkg/...

# Test specific package
go test ./pkg/supervisor
go test ./pkg/watcher
go test ./pkg/builder

# Test with coverage
go test -cover ./pkg/...

# Detailed coverage report
go test -coverprofile=coverage.out ./pkg/...
go tool cover -html=coverage.out
```

### 2. Integration Tests

#### Development Mode Integration Test

Test the full dev workflow: edit → detect → build → reload

```bash
# Terminal 1: Start server in dev mode
./start.sh dev

# Terminal 2: Make changes and verify
# Edit a worker file
echo "// Test change" >> workers/index/src/main.go

# Watch server logs for rebuild
# Expected: File change detected → Build started → Build complete → Worker reloaded

# Verify reload time < 1 second
# Verify zero downtime (requests continue to be served)

# Test the change
curl http://localhost:8080/

# Cleanup
git checkout workers/index/src/main.go
```

#### Production Mode Integration Test

Test the full prod workflow: deploy → signal → reload

```bash
# Build for production
./scripts/build-prod.sh

# Terminal 1: Start server in prod mode
./start.sh prod

# Terminal 2: Deploy changes
# Modify worker
touch workers/index/bin/tqworker_index

# Send reload signal
pkill -SIGHUP tqserver

# Watch server logs for reload
# Expected: Signal received → Timestamps checked → Changes detected → Workers rebuilt → Zero downtime

# Verify health
curl http://localhost:8080/health

# Test the application
curl http://localhost:8080/
```

### 3. Build System Tests

#### Development Build Test

```bash
./scripts/build-dev.sh

# Verify binaries created
ls -lh server/bin/tqserver
ls -lh workers/*/bin/tqworker_*

# Verify debug symbols present
file server/bin/tqserver | grep "not stripped"
```

#### Production Build Test

```bash
./scripts/build-prod.sh

# Verify binaries created
ls -lh server/bin/tqserver
ls -lh workers/*/bin/tqworker_*

# Verify optimizations applied
file server/bin/tqserver | grep "stripped"
```

### 4. Health Check Tests

#### Worker Health Monitoring

```bash
# Start server
./start.sh dev

# Check health endpoint
curl http://localhost:8080/health

# Expected response:
# {
#   "status": "healthy",
#   "workers": {
#     "index": "healthy"
#   }
# }

# Stop a worker to test unhealthy state
# Find worker PID
ps aux | grep tqworker_index

# Kill worker
kill <PID>

# Check health (should show unhealthy)
curl http://localhost:8080/health
```

### 5. Performance Tests

#### Reload Time Test

Measure development mode reload time:

```bash
# Start server in dev mode
./start.sh dev

# In another terminal, measure reload time
time bash -c '
  echo "// Change" >> workers/index/src/main.go
  sleep 2  # Wait for reload
  git checkout workers/index/src/main.go
'

# Expected: < 1 second for small changes
```

#### Concurrent Request Test

Test zero-downtime reload under load:

```bash
# Terminal 1: Start server
./start.sh dev

# Terminal 2: Generate load
for i in {1..100}; do
  curl -s http://localhost:8080/ > /dev/null &
done

# Terminal 3: Trigger reload
echo "// Change" >> workers/index/src/main.go
sleep 2
git checkout workers/index/src/main.go

# Verify: All requests should succeed (no 502/503 errors)
```

#### Load Test with Apache Bench

```bash
# Start server
./start.sh prod

# Simple load test
ab -n 1000 -c 10 http://localhost:8080/

# Load test during reload
# Terminal 1: Load
ab -n 10000 -c 50 http://localhost:8080/

# Terminal 2: Trigger reload during load
sleep 2
pkill -SIGHUP tqserver
```

### 6. Deployment Tests

#### Local Deployment Simulation

```bash
# Create mock remote directory
mkdir -p /tmp/tqserver_deploy_test
export DEPLOY_TEST_PATH="/tmp/tqserver_deploy_test"

# Build for deployment
./scripts/build-prod.sh

# Test pre-deploy hook
./scripts/hooks/pre-deploy.sh production server/bin/tqserver "index"
echo "Pre-deploy exit code: $?"

# Test rsync dry-run
rsync -avz --dry-run server/bin/tqserver $DEPLOY_TEST_PATH/

# Verify deployment script syntax
bash -n scripts/deploy.sh
echo "Syntax check exit code: $?"

# Cleanup
rm -rf /tmp/tqserver_deploy_test
```

#### Deployment Configuration Test

```bash
# Validate YAML syntax
python3 -c "import yaml; yaml.safe_load(open('config/deployment.yaml'))"
python3 -c "import yaml; yaml.safe_load(open('config/server.yaml'))"

# Test SSH connectivity (if configured)
# ssh deploy@staging.example.com echo "Connected"
```

### 7. Error Handling Tests

#### Worker Crash Recovery

```bash
# Start server
./start.sh dev

# Kill worker
pkill -9 tqworker_index

# Verify supervisor restarts worker
sleep 2
curl http://localhost:8080/

# Check logs for restart
tail logs/server_*.log
```

#### Build Failure Handling

```bash
# Start server
./start.sh dev

# Introduce syntax error
echo "INVALID GO CODE" >> workers/index/src/main.go

# Verify old worker continues serving
curl http://localhost:8080/

# Check logs for build error
tail logs/server_*.log

# Fix error
git checkout workers/index/src/main.go

# Verify rebuild succeeds
sleep 2
curl http://localhost:8080/
```

#### Port Exhaustion Test

```bash
# Edit config to limit port range
# port_range_start: 9000
# port_range_end: 9002  # Only 3 ports

# Try to start more workers than available ports
# Should handle gracefully with error messages
```

### 8. Configuration Tests

#### Hot Configuration Reload

```bash
# Start server
./start.sh prod

# Modify config
# Change worker timeout values in config/server.yaml

# Send reload signal
pkill -SIGHUP tqserver

# Verify config changes applied (check logs)
```

#### Invalid Configuration Handling

```bash
# Backup config
cp config/server.yaml config/server.yaml.backup

# Create invalid config
echo "invalid: [yaml" > config/server.yaml

# Try to start server
./start.sh dev

# Should fail with clear error message

# Restore config
mv config/server.yaml.backup config/server.yaml
```

## Automated Test Suite

Create a comprehensive test script:

```bash
#!/bin/bash
# test_all.sh - Run all tests

set -e

echo "=== Running Unit Tests ==="
go test ./pkg/...

echo "=== Running Build Tests ==="
./scripts/build-dev.sh
./scripts/build-prod.sh

echo "=== Validating Configuration ==="
python3 -c "import yaml; yaml.safe_load(open('config/deployment.yaml'))"
python3 -c "import yaml; yaml.safe_load(open('config/server.yaml'))"

echo "=== Checking Script Syntax ==="
bash -n scripts/deploy.sh
bash -n scripts/hooks/pre-deploy.sh
bash -n scripts/hooks/post-deploy.sh
bash -n scripts/build-dev.sh
bash -n scripts/build-prod.sh

echo "=== Verifying Binaries Exist ==="
test -f server/bin/tqserver
test -f workers/index/bin/tqworker_index

echo "=== All Tests Passed ==="
```

## Continuous Integration

### GitHub Actions Example

See [.github/workflows/test.yml](.github/workflows/test.yml) for CI configuration.

### Pre-commit Hooks

```bash
# .git/hooks/pre-commit
#!/bin/bash
go test ./pkg/...
go vet ./...
```

## Test Checklist

Before deploying to production:

- [ ] All unit tests pass
- [ ] Build scripts work (dev and prod)
- [ ] Development mode hot reload works
- [ ] Production mode SIGHUP reload works
- [ ] Health checks respond correctly
- [ ] Worker crash recovery works
- [ ] Build failure handling works
- [ ] Configuration validation works
- [ ] Deployment scripts syntax valid
- [ ] Zero downtime verified under load
- [ ] Log files created and rotated
- [ ] Port management works correctly

## Performance Benchmarks

Expected performance metrics:

| Metric | Target | Measurement |
|--------|--------|-------------|
| Dev mode reload | < 1s | Time from file save to new worker serving |
| Prod mode reload | < 2s | Time from SIGHUP to new worker serving |
| Worker startup | < 500ms | Time from process start to health check pass |
| Zero downtime | 100% | Request success rate during reload |
| Health check | < 100ms | Response time for /health endpoint |

## Troubleshooting Tests

### Debug Mode

Run with verbose logging:

```bash
# Set log level
export TQ_LOG_LEVEL=debug
./start.sh dev
```

### Check Process State

```bash
# List all TQServer processes
ps aux | grep tqserver
ps aux | grep tqworker

# Check open ports
lsof -i :8080
lsof -i :9000-9999

# Check file handles
lsof -p $(pgrep tqserver)
```

### Validate File Watchers

```bash
# Check inotify limits (Linux)
cat /proc/sys/fs/inotify/max_user_watches

# Increase if needed
echo 524288 | sudo tee /proc/sys/fs/inotify/max_user_watches
```

## Test Coverage Goals

Target coverage by package:

- `pkg/supervisor`: > 80%
- `pkg/watcher`: > 70%
- `pkg/builder`: > 70%
- `pkg/devmode`: > 60%
- `pkg/prodmode`: > 60%
- `pkg/coordinator`: > 75%

Generate coverage report:

```bash
./scripts/test-coverage.sh
open coverage.html
```
