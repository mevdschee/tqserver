# Session Management

One of the most critical aspects of distributed systems and microservices is handling shared state and user sessions.

- [The Stateless Principle](#the-stateless-principle)
- [Recommended Architecture](#recommended-architecture)
- [Implementation Workflow](#implementation-workflow)
- [Future Native Support](#future-native-support)

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

## Implementation Workflow

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
