# TQServer Documentation

Welcome to the TQServer documentation. TQServer is a high-performance function execution platform built with Go that provides sub-second hot reloads with native Go performance.

## Table of Contents

### Prologue
- [Release Notes](prologue/releases.md)
- [Upgrade Guide](prologue/upgrade.md)
- [Contribution Guide](prologue/contributions.md)

### Getting Started
- [Installation](getting-started/installation.md)
- [Configuration](getting-started/configuration.md)
- [Directory Structure](getting-started/structure.md)
- [Starter Kits](getting-started/starter-kits.md)
- [Deployment](getting-started/deployment.md)

### Architecture Concepts
- [Request Lifecycle](architecture/lifecycle.md)
- [Worker Architecture](architecture/workers.md)
- [Process Isolation](architecture/isolation.md)
- [Hot Reload System](architecture/hot-reload.md)
- [Supervisor Pattern](architecture/supervisor.md)

### The Basics
- [Routing](basics/routing.md)
- [Middleware](basics/middleware.md)
- [Controllers](basics/controllers.md)
- [Requests](basics/requests.md)
- [Responses](basics/responses.md)
- [Views](basics/views.md)
- [Templates](basics/templates.md)
- [URL Generation](basics/urls.md)
- [Session](basics/session.md)
- [Validation](basics/validation.md)
- [Error Handling](basics/errors.md)
- [Logging](basics/logging.md)

### Workers
- [Creating Workers](workers/creating.md)
- [Worker Lifecycle](workers/lifecycle.md)
- [Worker Configuration](workers/configuration.md)
- [Building Workers](workers/building.md)
- [Testing Workers](workers/testing.md)
- [Health Checks](workers/health-checks.md)

### Advanced Topics
- [Port Pool Management](advanced/port-pool.md)
- [Process Management](advanced/process-management.md)
- [File Watching](advanced/file-watching.md)
- [Graceful Restarts](advanced/graceful-restarts.md)
- [Configuration Hot Reload](advanced/config-hot-reload.md)
- [Performance Tuning](advanced/performance.md)
- [Binary Cleanup](advanced/binary-cleanup.md)

### Proxy & Routing
- [HTTP Proxy](proxy/http-proxy.md)
- [Request Forwarding](proxy/forwarding.md)
- [Load Balancing](proxy/load-balancing.md)
- [WebSocket Support](proxy/websockets.md)

### Static Assets
- [Public Directory](assets/public.md)
- [Private Assets](assets/private.md)
- [Asset Organization](assets/organization.md)

### Database
- [Getting Started](database/getting-started.md)
- [Query Builder](database/query-builder.md)
- [Migrations](database/migrations.md)
- [Seeding](database/seeding.md)

### Security
- [Authentication](security/authentication.md)
- [Authorization](security/authorization.md)
- [CORS](security/cors.md)
- [Encryption](security/encryption.md)
- [Rate Limiting](security/rate-limiting.md)

### Testing
- [Getting Started](testing/getting-started.md)
- [Worker Testing](testing/workers.md)
- [HTTP Tests](testing/http-tests.md)
- [Integration Tests](testing/integration.md)
- [Mocking](testing/mocking.md)

### Monitoring & Debugging
- [Logging](monitoring/logging.md)
- [Health Checks](monitoring/health-checks.md)
- [Metrics](monitoring/metrics.md)
- [Debugging](monitoring/debugging.md)
- [Profiling](monitoring/profiling.md)

### Packages
- [Supervisor](packages/supervisor.md)
- [Watcher](packages/watcher.md)
- [Worker Runtime](packages/worker.md)
- [Config](packages/config.md)

### API Reference
- [Configuration Reference](api/configuration.md)
- [Router API](api/router.md)
- [Supervisor API](api/supervisor.md)
- [Worker API](api/worker.md)
- [Template Functions](api/templates.md)

### Appendix
- [FAQ](appendix/faq.md)
- [Glossary](appendix/glossary.md)
- [Comparison with Other Frameworks](appendix/comparisons.md)
- [Best Practices](appendix/best-practices.md)
- [Troubleshooting](appendix/troubleshooting.md)

## Contributing

Thank you for considering contributing to TQServer! Please read the [Contribution Guide](prologue/contributions.md) for details.

## License

TQServer is open-source software. Please see the License file for more information.
