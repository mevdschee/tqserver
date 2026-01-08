# Logging

TQServer provides a centralized logging system that aggregates logs from the Supervisor, Proxy, and all active Workers.

## Unified Log Stream

All components write to standard output (stdout/stderr). In a standard deployment (e.g., using `systemd` or Docker), these logs are captured in a single stream.

## Log Formats

### System Logs
The Supervisor and Proxy emit structured logs indicating system events:

```text
2024/01/20 10:00:01 Route configured: /api -> api
2024/01/20 10:00:02 ✅ Worker started for /api on port 9005 (PID: 12345)
2024/01/20 10:00:05 GET /api/status -> worker on port 9005 (proxied path: /status)
```

### Worker Logs
Go and PHP workers should write to `stdout` or `stderr`. These lines are captured by the parent process and prefixed or merged into the main log stream.

-   **Go**: Use `fmt.Println` or `log.Println`.
-   **PHP**: Use `error_log()`. TQServer catches PHP's `stderr` and prefixes it with `[PHP stderr]`.

### PHP Error Logs
PHP errors are specifically captured and tagged:
```text
2024/01/20 10:05:00 [PHP stderr] PHP Fatal error:  Uncaught Error...
```
