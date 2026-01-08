# Request Forwarding

When TQServer routes a request to a worker, it modifies the request to ensure the worker receives a clean, context-aware path.

## Path Stripping

Workers are often mounted on sub-paths (e.g., `/api` or `/blog`). However, it is convenient for the worker to write routes relative to its own root.

**Example:**
-   **Worker Route**: `/api`
-   **Client Request**: `GET /api/users/123`
-   **Upstream Request**: `GET /users/123`

The proxy automatically strips the registered `route` prefix before forwarding the request to the upstream worker. This allows a worker to just handle `/users/123` without knowing it is mounted under `/api`.

## Header Manipulation

Standard proxy headers are forwarded:
-   `Host`: Preserved from original request
-   `User-Agent`: Preserved
-   `X-Forwarded-For`: Appended with client IP
-   `X-Forwarded-Host`: Original host
-   `X-Forwarded-Proto`: Original protocol (http/https)

## PHP FastCGI Forwarding

For PHP workers, headers are converted to FastCGI variables:
-   `Content-Type` -> `CONTENT_TYPE`
-   `Content-Length` -> `CONTENT_LENGTH`
-   `Authorization` -> `HTTP_AUTHORIZATION`
-   All other `Header-Name` -> `HTTP_HEADER_NAME` (upercased, hyphens to underscores)
