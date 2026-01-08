# Port Pool Management

TQServer manages a pool of TCP ports to assign to workers. This allows for zero-downtime reloads by spinning up new worker processes on fresh ports before terminating old ones.

## Configuration

The port range is defined in your main configuration file (usually `config/tqserver.yaml`):

```yaml
workers:
  port_range_start: 9000
  port_range_end: 9999
```

The Supervisor will cycle through these ports, assigning them sequentially to workers as they start or restart.

## Assignment Strategy

The allocation strategy is a simple round-robin increment:

1.  The Supervisor maintains an internal `nextPort` counter, initialized to `port_range_start`.
2.  When a worker needs to start (or restart), `getFreePort()` is called.
3.  The current `nextPort` is assigned.
4.  `nextPort` is incremented.
5.  If `nextPort` exceeds `port_range_end`, it wraps back to `port_range_start`.

### Collision Avoidance

Since multiple processes might be starting simultaneously or external applications might use ports in the range, TQServer includes safety checks:

-   **Go/Kotlin Workers**: Ports are assigned directly. It is assumed the range is reserved for TQServer.
-   **PHP Workers**: The Supervisor performs an active probe. It attempts to bind a `net.Listen` on the candidate port.
    -   If successful, the port is truly free. The listener is closed, and the port is assigned to the PHP-FPM pool.
    -   If the bind fails (port in use), the Supervisor increments to the next port and retries until a free one is found or the range is exhausted.

## Port Exhaustion

If you have many workers or very frequent restarts, ensure your port range is large enough. If the Supervisor cycles through the entire range and finds no free ports (specifically for PHP workers where it probes), it will return an error.

For Go workers, if the port is actually in use by a zombie process (not tracked by Supervisor), the worker startup will fail with a "bind: address already in use" error, which will be captured in the logs.
