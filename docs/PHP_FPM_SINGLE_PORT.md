# PHP-FPM Single-Port Refactor Plan

Overview
- Goal: Migrate PHP handling to a single php-fpm endpoint (one listen address) and route all PHP requests through it.
- Motivation: simplify topology, improve resource sharing and caching, reduce per-worker overhead.

Scope
- Implement a FastCGI adapter that forwards requests to php-fpm over a single listen endpoint (TCP or UNIX socket).
- Update router/proxy to forward PHP requests to the adapter.
- Preserve devmode features and provide backward-compatible config where practical.

High-level design decisions
- Listen transport: prefer TCP (127.0.0.1:PORT) for portability and simple networking; support UNIX socket as optional for performance.
- Central adapter: single `phpfpm` adapter with an internal connection pool to php-fpm.
- Timeouts: configurable request, connect, and read/write timeouts.
- Health: periodic health-checks and circuit-breaker behavior to prevent cascading failures.

Config additions
- `php_fpm.enabled` (bool)
- `php_fpm.listen` (string) — e.g. `127.0.0.1:9001` or `/var/run/php-fpm.sock`
- `php_fpm.transport` (tcp|unix)
- `php_fpm.pool_size` (int)
- `php_fpm.connect_timeout_ms`, `php_fpm.request_timeout_ms`
- Backwards compatibility: if legacy worker mode configured, keep current behavior until migration enabled.

Component changes
- `pkg/php/worker.go`: refactor to call the `phpfpm` adapter instead of spawning/round-robin worker processes.
- New package: `pkg/php/phpfpm` (or `pkg/php/adapter_phpfpm`) implementing FastCGI client, pooling, retries, and metrics.
- Router/Proxy: forward PHP requests (based on configured patterns) to the single adapter endpoint.
- Config: update `config/*.yaml` examples and `server.example.yaml` to include `php_fpm` keys.

Behavioral details
- Connection pooling: maintain a fixed pool of persistent FastCGI connections (size = `pool_size`).
- Request lifecycle: acquire connection -> send FastCGI request -> read response -> release connection.
- Failure handling: on connection error, mark connection unhealthy, optionally create replacement connections; expose metrics and logs.
- Devmode: allow spawning a local php-fpm (sandbox) on the configured listen for developers; enable verbose logs and rebuild-on-change hooks.

Migration steps (recommended sequence)
1. Add config schema and example values in `config/*.yaml`.
2. Implement `phpfpm` adapter with a basic FastCGI client and a simple pool.
3. Add unit tests for adapter behaviors (connect, request/response, timeouts, errors).
4. Add integration test that uses a test php-fpm (or a mocked FastCGI server) to validate end-to-end.
5. Update `pkg/php/worker.go` to call the adapter; keep a compatibility shim so existing manager code compiles.
6. Update router/proxy to route PHP requests to the adapter.
7. Run full test-suite; fix regressions.
8. Update docs (`README.md`, `DEPLOYMENT.md`) and provide migration steps for operators.

Acceptance criteria
- All PHP requests are served through the configured single php-fpm listen address.
- No regressions in existing tests covering PHP request handling.
- Devmode workflows continue to work with minimal developer configuration changes.
- Metrics and logging show healthy pooling and acceptable latencies under load.

Rollout & rollback
- Rollout: enable feature behind config flag; run in staging with traffic mirroring before switching live traffic.
- Rollback: revert to legacy worker mode by toggling the config flag; ensure both code paths remain deployable during transition.

Risks and mitigations
- Risk: php-fpm becomes a single point of failure — mitigate with health checks, autoscaling (multiple php-fpm instances) or supervisor restart logic.
- Risk: connection saturation — mitigated with proper `pool_size` default and connection queueing/backpressure.
- Risk: subtle behavioral differences between per-worker PHP and shared php-fpm — mitigate via comprehensive integration tests and smoke-testing.

Next actions
- Implement adapter skeleton in `pkg/php/phpfpm` and add unit tests.
- Begin analysis of current `pkg/php/worker.go` to list required API shims and touch points.

Estimated timeline
- Design & config updates: 1–2 days
- Adapter implementation + unit tests: 2–4 days
- Router + worker refactor + integration tests: 2–3 days
- Docs, staging rollout, validation: 1–2 days

Contact
- For questions or trade-offs, open an issue titled: "RFC: php-fpm single-port" and assign the platform owners.
