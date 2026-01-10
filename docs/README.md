# TQServer Documentation

Welcome to the TQServer documentation. TQServer is a high-performance function execution platform built with Go that provides sub-second hot reloads with native Go performance and zero-downtime reconfiguration.

## Table of Contents

### 1. Server Platform & Orchestration
*Focuses on server infrastructure, worker orchestration, and deployment.*

**Getting Started**
- [Installation](getting-started/installation.md)
- [Configuration](getting-started/configuration.md)
- [Directory Structure](getting-started/structure.md)
- [Starter Kits](getting-started/starter-kits.md)
- [Deployment](getting-started/deployment.md)

**Architecture & Internals**
- [Request Lifecycle](architecture/lifecycle.md)
- [Worker Architecture](architecture/workers.md)
- [Process Isolation](architecture/isolation.md)
- [Supervisor Pattern](architecture/supervisor.md)
- [Hot Reload System](architecture/hot-reload.md)
- [Live Reload (WebSocket)](architecture/live-reload.md)

**Orchestration & Management**
- [Port Pool Management](advanced/port-pool.md) (TODO)
- [Process Management](advanced/process-management.md) (TODO)
- [Binary Cleanup](advanced/binary-cleanup.md) (TODO)
- [Graceful Restarts](advanced/graceful-restarts.md) (TODO)

**Networking & Proxy**
- [HTTP Proxy](proxy/http-proxy.md) (TODO)
- [Request Forwarding](proxy/forwarding.md) (TODO)
- [Load Balancing](proxy/load-balancing.md) (TODO)
- [WebSocket Support](proxy/websockets.md) (TODO)

**Monitoring**
- [Logging](monitoring/logging.md) (TODO)
- [Health Checks](monitoring/health-checks.md) (TODO)
- [SOCKS5 Proxy](monitoring/socks5-proxy.md)
- [Metrics](monitoring/metrics.md) (TODO)
- [Debugging](monitoring/debugging.md) (TODO)
- [Profiling](monitoring/profiling.md) (TODO)

### 2. Application Development (Go / PHP / TypeScript)
*Building applications and workers using Go, PHP, and TypeScript (Bun).*

**Core Concepts**
- [Routing](basics/routing.md)
- [Middleware](basics/middleware.md) (TODO)
- [Controllers](basics/controllers.md) (TODO)
- [Requests](basics/requests.md) (TODO)
- [Responses](basics/responses.md) (TODO)
- [URL Generation](basics/urls.md) (TODO)
- [Validation](basics/validation.md) (TODO)
- [Error Handling](basics/errors.md) (TODO)
- [Logging](basics/logging.md) (TODO)

**Worker Development**
- [Creating Workers (Go)](workers/creating.md)
- [PHP Workers](workers/php.md)
- [TypeScript Workers (Bun)](workers/typescript.md)
- [Worker Lifecycle](workers/lifecycle.md)
- [Worker Configuration](workers/configuration.md)
- [Building Workers](workers/building.md)
- [Testing Workers](workers/testing.md)
- [Health Checks](workers/health-checks.md)

**Frontend & Assets**
- [Views](basics/views.md) (TODO)
- [Templates](basics/templates.md) (TODO)
- [Public Directory](assets/public.md) (TODO)
- [Templates & Views](assets/views.md) (TODO)
- [Asset Organization](assets/organization.md) (TODO)

**Testing**
- [Getting Started](testing/getting-started.md) (TODO)
- [Worker Testing](testing/workers.md) (TODO)
- [HTTP Tests](testing/http-tests.md) (TODO)
- [Integration Tests](testing/integration.md) (TODO)
- [Mocking](testing/mocking.md) (TODO)

**Security**
- [Authentication](security/authentication.md) (TODO)
- [Authorization](security/authorization.md) (TODO)
- [CORS](security/cors.md) (TODO)
- [Encryption](security/encryption.md) (TODO)
- [Rate Limiting](security/rate-limiting.md) (TODO)

### 3. State Management
*Database, Sessions, and Caching.*

**Session**
- [Session Management](basics/session.md) (TODO)

**Database**
- [Getting Started](database/getting-started.md) (TODO)
- [Query Builder](database/query-builder.md) (TODO)
- [Migrations](database/migrations.md) (TODO)
- [Seeding](database/seeding.md) (TODO)

**Caching**
- [Caching Strategies](state/cache.md) (TODO)

### Appendix

**Packages**
- [Supervisor](packages/supervisor.md) (TODO)
- [Watcher](packages/watcher.md) (TODO)
- [Worker Runtime](packages/worker.md) (TODO)
- [Config](packages/config.md) (TODO)

**API Reference**
- [Configuration Reference](api/configuration.md) (TODO)
- [Router API](api/router.md) (TODO)
- [Supervisor API](api/supervisor.md) (TODO)
- [Worker API](api/worker.md) (TODO)
- [Template Functions](api/templates.md) (TODO)

**Meta & Help**
- [Release Notes](prologue/releases.md)
- [Upgrade Guide](prologue/upgrade.md)
- [Contribution Guide](prologue/contributions.md)
- [FAQ](appendix/faq.md) (TODO)
- [Glossary](appendix/glossary.md) (TODO)
- [Comparison with Other Frameworks](appendix/comparisons.md) (TODO)
- [Best Practices](appendix/best-practices.md) (TODO)
- [Troubleshooting](appendix/troubleshooting.md) (TODO)

## Contributing

Thank you for considering contributing to TQServer! Please read the [Contribution Guide](prologue/contributions.md) for details.

## License

TQServer is open-source software. Please see the License file for more information.
