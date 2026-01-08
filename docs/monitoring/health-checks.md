# Health Checks

Health checks are critical for the Supervisor to determine when to restart a worker.

## Active Checks

1.  **Process Exit**: The most basic check. If the process ID (PID) disappears or the process exits with a code > 0, it is immediately dead.
2.  **TCP Probe (PHP)**: For PHP workers, the Supervisor periodically attempts to open a TCP connection to the FastCGI port.
    -   *Frequency*: Every 5 seconds (default monitor loop).
    -   *Timeout*: 100ms.
    -   *Failure*: If the connection is refused, the worker is marked unhealthy and restarted.

## Passive Checks

1.  **Request Limit**: Workers can be configured with `max_requests`. The Supervisor tracks the number of requests proxied to each worker. When the limit is reached, a graceful restart is triggered.
2.  **Proxy Errors**: If the Proxy fails to connect to an upstream worker (Network Error / 502), this does not currently trigger an immediate restart but is logged. Persistent failures will usually be caught by the active health checker or process monitor.
