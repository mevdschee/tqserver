# TQServer

A high-performance function execution platform built with Go that provides sub-second hot reloads with native Go performance.

## Overview

TQServer bridges the gap between a high-performance web server and a flexible function-as-a-service development platform. 

### Key Features

- **Sub-second hot reloads** - Changes to page code are automatically detected, rebuilt, and deployed in ~0.3-1.0 seconds
- **Graceful restarts** - Zero-downtime deployments with traffic switching
- **Go (binary)** - Go workers are compiled binaries, not interpreted scripts
- **PHP (php-fpm)** - PHP workers are started as php-fpm processes
- **Typescript (Bun)** - Typescript workers are started as Bun processes

## Quick Start

### 1. Build the server and workers

For development:
```bash
./scripts/build-dev.sh
```

For production:
```bash
./scripts/build-prod.sh
```

### 2. Configure (optional)

Edit `config/server.yaml` to customize ports and timeouts.

### 3. Run the server

Development mode (watching for changes):
```bash
./server/bin/tqserver --mode dev
```

Production mode:
```bash
./server/bin/tqserver --mode prod
```

Visit http://localhost:8080 to see it in action!

---

# Part 1: The Web Server Platform

This section covers the operational aspects of TQServer: configuration, deployment, and management.

## Server Features

- **Port pool management** - Efficient port allocation prevents port exhaustion
- **Health monitoring** - Periodic HTTP health checks on worker processes
- **Configuration hot reload** - Automatically detect and apply configuration changes without server restart
- **Structured logging** - Server and worker logs with date-based rotation
- **Quiet mode** - Suppress console output for production deployments

## Command Line Options

```bash
bin/tqserver [options]

Options:
  -config string
        Path to config file (default "config/server.yaml")
  -quiet
        Suppress log output to stdout/stderr
```

The `-quiet` flag is useful for production environments where you want logs only written to files.

## Server Configuration

The server is configured via `config/server.yaml`.

### Server Settings

| Option                  | Type   | Default                  | Description                                                                                                    |
| ----------------------- | ------ | ------------------------ | -------------------------------------------------------------------------------------------------------------- |
| `port`                  | int    | 8080                     | HTTP server listening port                                                                                     |
| `read_timeout_seconds`  | int    | 30                       | Maximum time to read request                                                                                   |
| `write_timeout_seconds` | int    | 30                       | Maximum time to write response                                                                                 |
| `idle_timeout_seconds`  | int    | 120                      | Keep-alive timeout                                                                                             |
| `log_file`              | string | `logs/server_{date}.log` | Server log file path. Supports `{date}` placeholder. Use `~`, `null`, or empty string to disable file logging. |

### Global Worker Defaults

These settings in `config/server.yaml` define the default behavior for all workers managed by the server.

| Option                     | Type   | Default     | Description                                        |
| -------------------------- | ------ | ----------- | -------------------------------------------------- |
| `port_range_start`         | int    | 9000        | First port in worker port pool                     |
| `port_range_end`           | int    | 9999        | Last port in worker port pool                      |
| `startup_delay_ms`         | int    | 100         | Delay after starting worker before routing traffic |
| `restart_delay_ms`         | int    | 100         | Delay before stopping old worker during restart    |
| `shutdown_grace_period_ms` | int    | 500         | Time allowed for graceful shutdown                 |

### Configuration Hot Reload

TQServer automatically detects changes to `config/server.yaml` and applies them without requiring a server restart. This allows for zero-downtime adjustments to timeouts, resource limits, and worker policies.

**Note:** Changes to `server.port` require a manual restart.

## Deployment

For production deployment details, see [DEPLOYMENT.md](DEPLOYMENT.md).

### Quick Deployment
```bash
# Build & Deploy
./scripts/build-prod.sh
./scripts/deploy.sh production

# Deploy specific worker only
./scripts/deploy.sh production index
```

### Cluster Architecture

To scale from single-node to multi-node:

1.  **Shared Code Distribution**: Use Git to pull code on all nodes. TQServer will rebuild binaries locally on each node.
2.  **Shared Configuration**: `server.yaml` should be version controlled and identical across nodes.

---

# Part 2: Software Development Platform

This section guides developers on building applications (workers) for TQServer.

## Project Structure

```
tqserver/
├── workers/              # Worker applications
│   └── {name}/            # Individual worker
│       ├── src/            # Worker source code
│       ├── bin/            # Compiled worker binary
│       ├── config/         # Worker-specific config
│       └── views/          # Templates/HTML (optional)
```

## Developing Workers

TQServer supports polyglot workers.

### Go Workers (Native)
Provide the highest performance (~300-1000ms reload).
-   **Structure**: Standard Go executable.
-   **Runtime**: Use `pkg/worker` for easy setup.
-   **Example**: `workers/index/`

### Kotlin Workers
Run as standalone JARs.
-   **Framework**: Ktor recommended.
-   **Example**: `workers/api/` and [docs/workers/kotlin.md](docs/workers/kotlin.md)

### PHP Workers
Run via FastCGI (php-fpm).
-   **Connection**: Supervisor manages a PHP-FPM adapter.
-   **Example**: `workers/blog/` and [pkg/php/README.md](pkg/php/README.md)

## Worker Configuration (`worker.yaml`)

Each worker has its own configuration file at `workers/{name}/config/worker.yaml`.

| Option                  | Type   | Default                         | Description                                                                                                 |
| ----------------------- | ------ | ------------------------------- | ----------------------------------------------------------------------------------------------------------- |
| `path`                  | string | (required)                      | URL path prefix for this worker (e.g., "/", "/api")                                                       |
| `go_max_procs`          | int    | 2                               | Sets Go's GOMAXPROCS (CPU threads). 0 = NumCPU                                                              |
| `max_requests`          | int    | 0                               | Restart worker after N requests. 0 = unlimited                                                              |
| `go_mem_limit`          | string | ""                              | Go's GOMEMLIMIT (e.g., "512MiB").                                                                           |
| `log_file`              | string | `logs/worker_{name}_{date}.log` | Worker log file template.                                                                                   |

## Environment Variables

Workers receive configuration via environment variables at runtime.

| Variable                         | Description                                      |
| -------------------------------- | ------------------------------------------------ |
| `WORKER_PORT`                    | Port number assigned to this worker              |
| `WORKER_NAME`                    | Worker folder name                               |
| `WORKER_ROUTE`                   | URL path prefix                                  |
| `WORKER_MODE`                    | `development` or `production`                    |

Example usage (Go):
```go
port := os.Getenv("WORKER_PORT")
runtime := worker.NewRuntime() // Automatically processes Env Vars
```

## Health Checks

Every worker **must** implement a `/health` endpoint returning `200 OK`. TQServer actively polls this endpoint.

**Go Example:**
```go
http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)
    w.Write([]byte("OK"))
})
```

**Bun Example:**
```typescript
app.get('/health', (req, res) => {
    res.json({ status: 'ok' });
});
```

---

# Part 3: Session Management

One of the most critical aspects of distributed systems and microservices is handling shared state and user sessions.

## The Stateless Principle

TQServer workers are designed to be **stateless**.
-   **Ephemeral Processes**: Workers can be restarted at any time (due to configuration changes, deployments, or `max_requests` limits).
-   **Process Isolation**: Each request might be handled by a new instance or a different process.
-   **Scalability**: A stateless design allows you to run multiple instances of the same worker or scale across multiple servers (cluster mode) without worrying about synchronization.

**Rule of Thumb:** Never store session data (user login status, shopping carts, temp data) in global variables or local files within the worker. It will be lost on restart.

## Recommended Architecture

### 1. External Session Store
Use a fast, external key-value store for session data.
-   **Redis** (Recommended): High performance, persistent, supports expiration.
-   **Memcached**: Good for pure caching, simple string keys.
-   **Database**: PostgreSQL/MySQL (slower, but durable).

### 2. Session Identifiers (Cookies)
-   The client (browser) should hold a **Session ID** in a secure, HTTP-only cookie.
-   The Worker reads this Cookie on every request.
-   The Worker retrieves the session payload from Redis using the Session ID.

### 3. Implementation Workflow

1.  **User Logs In**:
    -   Worker validates credentials against DB.
    -   Worker generates a random `session_id`.
    -   Worker stores `session_id -> user_data` in Redis (e.g., with 24h TTL).
    -   Worker sets `Set-Cookie: session_id=...` header in response.

2.  **Subsequent Requests**:
    -   Browser sends `Cookie: session_id=...`.
    -   Worker extracts `session_id`.
    -   Worker fetches data from Redis.
    -   If not found, redirect to login.

## Future Native Support
We are planning built-in support for session management to simplify this workflow:
-   **Session Middleware**: Automatic cookie handling and transparent session storage.
-   **Pluggable Stores**: Configurable backends (Redis, File, Memory) in `server.yaml`.

---

# Roadmap / Missing Features

The following features are planned but not yet implemented:

- **TLS/HTTPS Support** - Currently only HTTP is supported
- **Metrics & Monitoring** - Prometheus/OpenTelemetry integration
- **Middleware Support** - Global and per-route middleware
- **Session Management** - Built-in abstract session storage (as described above)
- **WebSocket Support** - Proxying implementation needed
- **Static File Serving** - Efficient serving without worker overhead
- **Load Balancing** - Multiple worker instances per route
- **Docker Support** - Containerization with multi-stage builds
