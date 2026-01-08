# Debugging

## Development Mode

Enable development mode in `config/tqserver.yaml` by setting `mode: development`.

### Features
1.  **Build Error Pages**: If a Go worker fails to compile, the browser shows a formatted error page with the compiler output instead of a generic 500 error.
2.  **Live Reload**: Saving a file triggers an automatic browser refresh.
3.  **Debug Headers**: Responses include `X-TQServer-Worker-*` headers to trace which worker and port handled the request.

## Common Issues

### "Bind: Address already in use"
-   **Cause**: The port range is too small, or a previous worker process failed to exit cleanly (zombie process).
-   **Fix**: Kill stray `tqserver` or `php-fpm` processes (`pkill -f tqserver`) and restart.

### "Connection Refused" (PHP)
-   **Cause**: The `php-fpm` process crashed or failed to bind to the assigned port. Check the console logs for `[PHP stderr]` output.
-   **Effect**: You will see a "503 Service Unavailable" error page.

### Infinite Restarts
-   **Cause**: A worker crashes immediately upon startup (e.g., config error or panic in `main()`).
-   **Fix**: Check the console log. The Supervisor will keep trying to restart it. Fix the code error to stabilize the loop.
