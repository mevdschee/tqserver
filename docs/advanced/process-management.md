# Process Management

TQServer's Supervisor is responsible for managing the entire lifecycle of your worker processes. It handles building, starting, monitoring, and restarting workers to ensure high availability and sub-second hot reloads.

## Worker Lifecycle

1.  **Discovery**: The Supervisor scans the `workers/` directory and `config/tqserver.yaml` to identify enabled workers.
2.  **Build (Compiled Workers)**:
    -   **Go**: Compiles the worker using `go build` into a binary named after the worker.
    -   **Bun**: Installs dependencies using `bun install` if `package.json` exists.
    -   *PHP workers skip this step.*
3.  **Start**:
    -   **Go**: The binary is executed on an assigned port.
    -   **Bun**: Executed via `bun run index.ts`.
    -   **PHP**: A dedicated `php-fpm` pool is started.
4.  **Monitor**: The Supervisor watches the process for exit codes and health.
5.  **Restart**:
    -   **File Changes**: Refreshes the code and restarts the process.
    -   **Config Changes**: Restarts the worker with new settings.
    -   **Health Failure**: Automatically restarts unhealthy or crashed workers.

## Health Monitoring

The Supervisor employs multiple strategies to ensure workers are healthy:

### 1. Process Monitoring
The Supervisor watches the OS process ID (PID). If a worker process exits unexpectedly (non-zero exit code), it is immediately flagged as unhealthy and queued for a restart.

### 2. Active Health Checks
-   **PHP Workers**: The Supervisor runs a periodic TCP dial check.
-   **Go/Bun Workers**: Use HTTP health checks (`/health`).

### 3. Resource Limits
-   **Max Requests**: Go workers can be configured to restart after serving a set number of requests (`max_requests`) to prevent memory leaks.
-   **Timeouts**: PHP-FPM pools are configured with `process_idle_timeout` and `request_terminate_timeout` to cleanup stuck processes.

## Hot Reload & Development

In **Development Mode**, the emphasis is on speed:

-   **Go**: When a `.go` file changes, the Supervisor rebuilds and restarts *only* that specific worker.
-   **PHP**: When a `.php` file changes, no restart is needed. The Supervisor simply broadcasts a "reload" event to the browser via WebSocket, triggering an instant page refresh.
-   **Build Errors**: If a build fails, the error is captured and displayed in the browser instead of crashing the server.

## Production Resilience

In **Production Mode**:

-   **Zero-Downtime Reloads**:
    1.  The new worker process is started on a generic port.
    2.  **Port Readiness Check**: The Supervisor connects to the new port to ensure it is accepting connections.
    3.  **Restart Delay**: The configured `restart_delay_ms` timer starts *only after* the port is ready.
    4.  **Graceful Switch**: Traffic is routed to the new worker, and the old process is stopped.
-   **Graceful Shutdown**: Workers are sent `SIGINT` to finish current requests before being forced to exit.
