# PHP Workers

TQServer provides native support for PHP workers through FastCGI integration, functioning as a Go-based alternative to PHP-FPM. PHP workers enable you to run PHP applications with dynamic pool management, hot-reloading, and the same supervisor patterns used by other TQServer workers.

## Overview

PHP workers in TQServer use the FastCGI protocol to communicate with php-cgi processes. This approach provides:

- **No PHP-FPM Required** - Direct php-cgi integration
- **Dynamic Pool Management** - Auto-scaling worker pools (2-10 workers by default)
- **Multiple Pool Modes** - Dynamic, static, and ondemand pool managers
- **TCP Port Architecture** - Better isolation and debugging than Unix sockets
- **YAML Configuration** - No complex .conf files
- **Hot Reload Support** - Automatic worker restarts on code changes
- **Production Ready** - Comprehensive testing with concurrent requests and large responses

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    TQServer (Main)                       │
│  ┌──────────────┐  ┌─────────────┐  ┌────────────────┐ │
│  │   Router     │  │   Proxy     │  │  Supervisor    │ │
│  │  (HTTP:8080) │→ │   Handler   │→ │   (monitors)   │ │
│  └──────────────┘  └─────────────┘  └────────────────┘ │
│         ↓                                                │
│  ┌─────────────────────────────────────────────────┐    │
│  │     FastCGI Server (Public Port :9001)          │    │
│  │  ┌──────────────────────────────────────────┐   │    │
│  │  │         PHP Handler                       │   │    │
│  │  │  - Maps HTTP → FastCGI protocol          │   │    │
│  │  │  - Routes to available worker            │   │    │
│  │  └──────────────────────────────────────────┘   │    │
│  └─────────────────────────────────────────────────┘    │
│         ↓                                                │
│  ┌────────────────────────────────────────────────┐     │
│  │         PHP Manager (Dynamic Pool)             │     │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐     │     │
│  │  │ php-cgi  │  │ php-cgi  │  │ php-cgi  │ ... │     │
│  │  │ Worker 1 │  │ Worker 2 │  │ Worker 3 │     │     │
│  │  │ :9002    │  │ :9003    │  │ :9004    │     │     │
│  │  └──────────┘  └──────────┘  └──────────┘     │     │
│  │                                                 │     │
│  │  Auto-scaling: 2-10 workers based on load     │     │
│  └────────────────────────────────────────────────┘     │
└─────────────────────────────────────────────────────────┘
```

## Request Flow

1. HTTP Request arrives at TQServer (`:8080`)
2. Router identifies PHP worker route
3. Proxy forwards to FastCGI server (`:9001`)
4. FastCGI Handler translates HTTP → FastCGI protocol
5. PHP Manager selects idle worker
6. Worker (php-cgi process) executes PHP script
7. Response flows back through FastCGI → HTTP
8. Client receives response

## Prerequisites

PHP workers require php-cgi to be installed on your system:

```bash
# Ubuntu/Debian
sudo apt-get install php-cgi

# macOS (Homebrew)
brew install php

# Verify installation
which php-cgi
php-cgi -v
```

**Note:** You do NOT need php-fpm. TQServer manages the worker pool directly.

## Configuration

PHP workers are configured through `worker.yaml` files in the worker directory.

### Basic Configuration

Create `workers/blog/config/worker.yaml`:

```yaml
name: blog
type: php
port: 9001  # Public FastCGI server port

php:
  # Pool management mode
  pool:
    mode: dynamic          # dynamic, static, or ondemand
    min_workers: 2         # Minimum idle workers (dynamic mode)
    max_workers: 10        # Maximum total workers
    start_workers: 3       # Initial workers on startup (dynamic mode)
    max_requests: 1000     # Recycle worker after N requests
    idle_timeout: 60s      # Kill idle workers after timeout (ondemand mode)

  # PHP runtime settings
  binary: /usr/bin/php-cgi  # Path to php-cgi binary
  config_file: /etc/php/8.2/cgi/php.ini  # Optional: base php.ini
  
  # PHP ini overrides
  settings:
    memory_limit: "128M"
    max_execution_time: "30"
    display_errors: "1"
    error_reporting: "E_ALL"
    opcache.enable: "1"
    
  # Application settings
  document_root: /home/maurits/projects/tqserver/workers/blog/public
  script_name: index.php   # Main entry point
  
  # Worker ports (dynamically assigned)
  worker_base_port: 9002   # Workers get 9002, 9003, 9004, etc.
```

## Pool Management Modes

TQServer supports three pool management modes, matching PHP-FPM's behavior:

### Dynamic Mode (Recommended)

Automatically scales worker count based on load:

```yaml
php:
  pool:
    mode: dynamic
    min_workers: 2        # Always keep 2 idle workers ready
    max_workers: 10       # Never exceed 10 total workers
    start_workers: 3      # Start with 3 workers
    max_requests: 1000    # Recycle after 1000 requests
```

**Behavior:**
- Starts with `start_workers` processes
- Maintains `min_workers` idle workers at all times
- Spawns new workers when demand increases
- Caps at `max_workers` total
- Recycles workers after `max_requests` to prevent memory leaks

**Use Case:** Production applications with variable traffic

### Static Mode

Fixed number of workers, always running:

```yaml
php:
  pool:
    mode: static
    max_workers: 5        # Always exactly 5 workers
    max_requests: 1000
```

**Behavior:**
- Spawns exactly `max_workers` processes at startup
- Never scales up or down
- All workers always active

**Use Case:** Predictable load, maximum performance

### Ondemand Mode

Spawns workers only when needed:

```yaml
php:
  pool:
    mode: ondemand
    max_workers: 10       # Maximum workers to spawn
    max_requests: 1000
    idle_timeout: 60s     # Kill idle workers after 60s
```

**Behavior:**
- Starts with 0 workers
- Spawns worker on first request
- Kills workers after `idle_timeout` of inactivity
- Caps at `max_workers` total

**Use Case:** Low-traffic applications, development environments

## PHP Settings

Configure PHP runtime behavior through the `settings` map:

```yaml
php:
  settings:
    # Memory & Performance
    memory_limit: "256M"
    max_execution_time: "60"
    max_input_time: "60"
    
    # Error Handling
    display_errors: "1"
    error_reporting: "E_ALL"
    log_errors: "1"
    error_log: "/var/log/php/error.log"
    
    # OPcache
    opcache.enable: "1"
    opcache.memory_consumption: "128"
    opcache.interned_strings_buffer: "8"
    opcache.max_accelerated_files: "4000"
    opcache.validate_timestamps: "1"
    opcache.revalidate_freq: "2"
    
    # Session
    session.save_handler: "files"
    session.save_path: "/tmp"
    
    # Upload
    upload_max_filesize: "10M"
    post_max_size: "10M"
```

Settings are passed to php-cgi via `-d` flags:
```bash
php-cgi -d memory_limit=256M -d max_execution_time=60 ...
```

## Creating a PHP Worker

### 1. Create Worker Directory Structure

```bash
mkdir -p workers/myapp/{public,config}
```

### 2. Create Configuration

`workers/myapp/config/worker.yaml`:
```yaml
name: myapp
type: php
port: 9001

php:
  pool:
    mode: dynamic
    min_workers: 2
    max_workers: 10
    start_workers: 3
  
  document_root: ./workers/myapp/public
  script_name: index.php
  worker_base_port: 9002
```

### 3. Create PHP Application

`workers/myapp/public/index.php`:
```php
<?php
header('Content-Type: application/json');

echo json_encode([
    'message' => 'Hello from TQServer PHP Worker!',
    'timestamp' => date('Y-m-d H:i:s'),
    'php_version' => phpversion(),
    'worker_pid' => getmypid(),
]);
```

### 4. Start TQServer

```bash
./server/bin/tqserver
```

### 5. Test Your Worker

```bash
curl http://localhost:8080/myapp/
```

## Testing PHP Workers

### Manual Testing

```bash
# Test single request
curl http://localhost:9001/

# Test with query parameters
curl "http://localhost:9001/?name=TQServer"

# Test POST request
curl -X POST -d "key=value" http://localhost:9001/

# Test with headers
curl -H "X-Custom: test" http://localhost:9001/

# Test large response (phpinfo)
curl http://localhost:9001/info.php
```

### Load Testing

Test pool scaling behavior:

```bash
# Install Apache Bench
sudo apt-get install apache2-utils

# Test with 100 requests, 20 concurrent
ab -n 100 -c 20 http://localhost:9001/

# Monitor worker count while testing
watch -n 1 'ps aux | grep php-cgi | grep -v grep'
```

### Concurrent Request Testing

```bash
# Test 5 consecutive requests
for i in {1..5}; do 
  curl http://localhost:9001/hello.php
  echo ""
done

# Test 20 concurrent requests with xargs
seq 1 20 | xargs -P 20 -I {} curl -s http://localhost:9001/
```

## Debugging

### Enable Debug Logging

Add to `server/src/main.go` or use environment variables:

```go
log.SetLevel(log.DebugLevel)
```

### View Worker Status

Check running php-cgi processes:

```bash
ps aux | grep php-cgi
```

### Test Worker Port Directly

Bypass FastCGI server and test worker directly:

```bash
# Install cgi-fcgi tool
sudo apt-get install libfcgi-bin

# Test worker on port 9002
SCRIPT_FILENAME=/path/to/index.php \
REQUEST_METHOD=GET \
cgi-fcgi -bind -connect localhost:9002
```

### Common Issues

#### "No input file specified."

**Cause:** Missing or incorrect `SCRIPT_FILENAME` parameter.

**Fix:** Ensure `document_root` and `script_name` are correctly set:
```yaml
php:
  document_root: /full/path/to/workers/myapp/public
  script_name: index.php
```

#### Connection Hangs

**Cause:** FastCGI protocol buffering issue (fixed in current version).

**Solution:** Ensure you're using the latest version with bufio.Reader implementation.

#### "502 Bad Gateway"

**Cause:** Worker crashed or not listening on expected port.

**Debug:**
```bash
# Check if worker is running
netstat -tlnp | grep 9002

# Check TQServer logs
tail -f logs/tqserver.log

# Test worker directly
curl http://localhost:9002/
```

## FastCGI Protocol Details

TQServer implements the FastCGI binary protocol for PHP communication:

### Record Types
- `BeginRequest` - Start new request
- `Params` - Environment variables (SCRIPT_FILENAME, REQUEST_METHOD, etc.)
- `Stdin` - Request body data
- `Stdout` - Response data from PHP
- `Stderr` - Error output from PHP
- `EndRequest` - Request completion

### Protocol Implementation
- **Buffered Reading:** Uses `bufio.Reader` for efficient record parsing
- **Large Response Support:** Handles responses up to 122KB+ (tested with phpinfo)
- **Multiple Records:** Correctly processes multiple FastCGI records in single TCP packet
- **Connection Pooling:** Reuses connections to workers for performance

## Comparison with PHP-FPM

| Feature | PHP-FPM | TQServer PHP Workers |
|---------|---------|---------------------|
| **Pool Manager** | pm.dynamic | ✅ Dynamic mode |
| **Min Workers** | pm.min_spare_servers | ✅ min_workers |
| **Max Workers** | pm.max_children | ✅ max_workers |
| **Start Workers** | pm.start_servers | ✅ start_workers |
| **Max Requests** | pm.max_requests | ✅ max_requests |
| **Config Format** | .conf files | ✅ YAML |
| **Management** | systemctl | ✅ TQServer supervisor |
| **Hot Reload** | kill -USR2 | ✅ File watcher |
| **Monitoring** | slow log | ✅ Go metrics |
| **Port Type** | Unix sockets | ✅ TCP ports |

## Performance

Based on production testing:

- **Worker Spawn Time:** < 30ms (php-cgi startup)
- **Request Latency:** ~5ms (comparable to PHP-FPM)
- **Memory per Worker:** ~25MB (php-cgi process)
- **Pool Scaling:** < 200ms (goroutine-based)
- **Throughput:** > 5000 req/s (simple PHP script)
- **Large Responses:** 122KB+ handled correctly

## Advanced Usage

### Multiple PHP Workers

Run multiple PHP applications with separate worker pools:

```yaml
# workers/app1/config/worker.yaml
name: app1
type: php
port: 9001
php:
  worker_base_port: 9002
  pool: { mode: dynamic, max_workers: 10 }
```

```yaml
# workers/app2/config/worker.yaml
name: app2
type: php
port: 9011
php:
  worker_base_port: 9012
  pool: { mode: dynamic, max_workers: 5 }
```

### Custom PHP Binary

Use specific PHP version:

```yaml
php:
  binary: /usr/local/php-8.3/bin/php-cgi
  config_file: /usr/local/php-8.3/etc/php.ini
```

### Environment Variables

Pass environment to PHP workers:

```yaml
php:
  env:
    APP_ENV: production
    DB_HOST: localhost
    DB_NAME: myapp
```

Access in PHP:
```php
$env = getenv('APP_ENV');  // "production"
```

## Next Steps

- Explore [Worker Configuration](configuration.md) for advanced options
- Learn about [Health Checks](health-checks.md) for PHP workers
- Read about [Hot Reload System](../architecture/hot-reload.md)
- See [Testing Workers](testing.md) for testing strategies

## Additional Resources

- [FastCGI Specification](https://fastcgi-archives.github.io/FastCGI_Specification.html)
- [PHP-CGI Documentation](https://www.php.net/manual/en/install.fpm.php)
- [TQServer Architecture](../architecture/workers.md)
