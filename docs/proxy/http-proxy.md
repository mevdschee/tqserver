# HTTP Proxy

The TQServer HTTP Proxy sits at the front of the architecture (usually listening on port 8080) and acts as the intelligent ingress for all requests. It abstracts the complexity of dynamic worker ports from the client.

## Request Lifecycle

1.  **Routing**: The proxy matches the request URL to a registered worker via the `Router`.
2.  **Static File Check**:
    -   *Priority 1*: Checks the worker's `public/` directory.
    -   *Priority 2*: Checks the global `server/public/` directory.
    -   If found, the file is served immediately.
3.  **Error Handling (Dev Mode)**: Checks if the worker has a pending build error. If so, an HTML error page is served.
4.  **Forwarding**: The request is proxied to the worker's current dynamic port (e.g., `localhost:9005`).

## Features

### Static Asset Serving
The proxy handles static files efficiently without involving worker processes.
-   Worker-specific assets (e.g., `workers/blog/public/style.css`) are served from `/blog/style.css`.
-   Global assets (e.g., `server/public/favicon.ico`) are served as fallbacks.

### Development Headers
In Development Mode, the proxy injects debugging headers into matched responses:
-   `X-TQWorker-Name`: Name of the worker (e.g., "blog")
-   `X-TQWorker-Type`: Runtime type (e.g., "php", "go")
-   `X-TQWorker-Route`: Configured route (e.g., "/blog")
-   `X-TQWorker-Port`: Internal upstream port (e.g., "9005")

### PHP FastCGI
For PHP workers, the proxy acts as a FastCGI client, translating HTTP requests into the FastCGI protocol and communicating directly with the `php-fpm` pool managed by the Supervisor.

### Error Pages
-   **Build Errors**: Displays compilation errors for Go/Kotlin workers.
-   **502 Bad Gateway**: Displays a branded error page if the worker process is unreachable.
-   **503 Service Unavailable**: Displays if a worker is marked unhealthy.
