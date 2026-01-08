# Blog Worker (PHP Example)

This is a test worker to demonstrate TQServer's PHP-FPM alternative functionality.

## Structure

```
workers/blog/
├── config/
│   └── worker.yaml      # Worker configuration
├── public/
│   ├── hello.php        # Simple hello world
│   └── info.php         # PHP info page
└── README.md            # This file
```

## Quick Start

### 1. Verify PHP is installed

```bash
# Check PHP or php-fpm availability
php -v
# and
php-fpm -v
```

### 4. Test the setup

```bash
# Test hello.php
curl http://localhost:8080/hello.php

# Test info.php
curl http://localhost:8080/info.php
```

## Development Roadmap

### Phase 1: Basic FastCGI ✅ COMPLETE
- [x] FastCGI protocol implementation
- [x] Connection handling
- [x] Protocol tests
- [x] Integration with TQServer routing
- [x] Large response handling (tested with 122KB phpinfo)
- [x] Buffered reading for TCP packet handling

### Phase 2: PHP Integration (php-fpm-first) ✅ COMPLETE
- [x] Support php-fpm-first via launcher + adapter (`pkg/php/phpfpm`)
- [x] Legacy php-cgi per-process spawning removed in favor of php-fpm-managed pools
- [x] Configure via generated php-fpm pool config and CLI/INI overrides
- [x] Request proxying (FastCGI client → php-fpm)
- [x] Error handling and connection management
- [x] SCRIPT_FILENAME and CGI parameter mapping
- [x] REDIRECT_STATUS parameter support

### Phase 3: Pool Management ✅ COMPLETE
- [x] Static pool manager
- [x] Dynamic pool manager (min/max workers)
- [x] Health monitoring (socket verification)
- [x] Worker state tracking (idle/active/terminating)
- [x] Graceful shutdown with SIGTERM
- [ ] Ondemand pool manager
- [ ] Advanced metrics

### Phase 4: Advanced Features 🚧 IN PROGRESS
- [ ] Hot reload support
- [ ] Multiple PHP versions per route
- [ ] Slow request logging
- [ ] Performance metrics dashboard
- [ ] Request queuing
- [ ] Worker crash recovery

## Configuration Notes

The `worker.yaml` file demonstrates TQServer's approach:

1. **php-fpm-first**: TQServer can generate php-fpm pool/config files and launch `php-fpm` in the foreground; the legacy per-worker `php-cgi` spawning approach is deprecated.
2. **Optional php.ini**: Can use existing ini files as base configuration
3. **CLI overrides**: Individual settings via `-d` flags to the PHP process (CLI for `php-cgi` or launcher/env for `php-fpm`)
4. **Flexible pools**: Different configs per route/worker

## Testing Hello World

✅ **Now Working!** The system is fully operational:

```bash
# Start TQServer
bash start.sh

# TQServer automatically:
# 1. Reads workers/blog/config/worker.yaml
# 2. Generates php-fpm pool config and launches `php-fpm` (or connects to configured php-fpm)
# 3. Uses a pooled FastCGI client to forward requests to php-fpm
# 4. Handles requests: Browser → TQServer:8080 → FastCGI (php-fpm) → PHP workers
```

### Test Endpoints

```bash
# Simple hello world (small response)
curl http://localhost:8080/blog/hello.php
# Output: Hello from TQServer!

# PHP info page (large response ~122KB)
curl http://localhost:8080/blog/info.php | head -20
# Output: Full phpinfo() HTML

# Test concurrent requests
for i in {1..10}; do curl http://localhost:8080/blog/hello.php & done; wait
# All 10 requests succeed, load balanced across workers
```

### Performance

- ✅ Handles small responses (< 1KB)
- ✅ Handles large responses (tested up to 122KB phpinfo)
- ✅ Concurrent requests load balanced across worker pool
- ✅ Workers return to idle state after serving requests
- ✅ No hanging or blocking on large responses

## Next Steps

### Completed ✅
### Completed ✅
1. ✅ FastCGI client + php-fpm integration with TQServer
2. ✅ php-fpm-first process management (legacy php-cgi spawning removed)
3. ✅ Direct HTTP handling (no Nginx needed for dev)
4. ✅ Dynamic and static pool management
5. ✅ Health checks and socket verification
6. ✅ Large response handling (buffered reading)

### In Progress 🚧
1. Ondemand pool manager implementation
2. Worker crash detection and auto-restart
3. Slow request logging and alerts
4. Comprehensive metrics collection

### Planned 📋
1. Multiple PHP version support (e.g., PHP 8.2 for /admin, PHP 8.3 for /api)
2. Request queuing with overflow handling
3. Advanced monitoring dashboard
4. Hot reload for PHP configuration changes
5. Production-ready error handling and logging
