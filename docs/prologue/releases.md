# Release Notes

- [Versioning Scheme](#versioning-scheme)
- [Support Policy](#support-policy)
- [Current Release](#current-release)
- [Version History](#version-history)

## Versioning Scheme

TQServer follows [Semantic Versioning](https://semver.org/):

- **Major.Minor.Patch** (e.g., 1.2.3)
- **Major**: Breaking changes, major new features
- **Minor**: New features, backward compatible
- **Patch**: Bug fixes, security patches

## Support Policy

### Active Support
- Latest major version receives active development
- Security patches for 12 months after release
- Bug fixes for 6 months after release

### Security Support
- Critical security fixes for previous major version
- 6 months extended security support

### End of Life
- Announced 3 months before EOL
- No further updates or support

## Current Release

### Version 0.9.0 (Alpha) - January 2026

**Status**: Active Development  
**Release Date**: January 6, 2026

TQServer is currently in active development and has not reached a stable 1.0 release. The current version includes core functionality but some features are still being implemented.

#### Current Features
- ✅ Sub-second hot reloads (~0.3-1.0s)
- ✅ Filesystem-based routing
- ✅ Graceful worker restarts
- ✅ Native Go performance
- ✅ Process isolation
- ✅ Automatic builds
- ✅ Port pool management
- ✅ Health monitoring
- ✅ Binary cleanup
- ✅ Per-route configuration
- ✅ Configuration hot reload
- ✅ Structured logging
- ✅ Quiet mode

#### Known Issues
- Worker pooling not yet implemented
- Template caching disabled in development
- Limited error recovery in edge cases
- No cluster support yet

#### Breaking Changes
None (pre-1.0)

#### Migration Guide
Not applicable for alpha versions.

## Version History

### Upcoming: Version 1.0.0 (Planned Q2 2026)

**Target Features**:
- TLS/HTTPS support
- Metrics & monitoring (Prometheus/OpenTelemetry)
- Global and per-route middleware
- WebSocket support
- Enhanced static file serving
- Request logging with multiple formats
- Correlation ID tracking
- Load balancing with multiple worker instances
- Circuit breaker pattern
- Docker support
- Graceful shutdown improvements
- Worker pooling
- Template caching
- Rate limiting
- Authentication middleware
- Database connection pooling
- Background job support
- Admin dashboard
- Testing framework
- Enhanced CLI

### Version 0.9.0 - January 6, 2026

**Type**: Alpha Release  
**Focus**: Core Framework

#### Added
- Complete refactoring to modular architecture
- Supervisor-based worker management
- Port pool for dynamic allocation
- File watcher with debouncing
- Health check system
- Configuration hot reload
- Binary cleanup system
- Mode controller (dev/prod modes)
- Structured logging system
- Graceful restart mechanism

#### Changed
- Moved from monolithic to package-based architecture
- Improved worker build system
- Enhanced error handling
- Better signal handling
- Optimized port allocation

#### Fixed
- Port exhaustion issues
- Memory leaks in long-running workers
- File watcher race conditions
- Health check false positives
- Configuration reload bugs

#### Internal
- Added `pkg/supervisor` package
- Added `pkg/watcher` package
- Added `pkg/worker` package
- Added `internal/config` package
- Added `internal/proxy` package
- Added `internal/router` package
- Added `internal/supervisor` package

### Version 0.8.0 - December 2025

**Type**: Pre-Alpha  
**Focus**: Proof of Concept

#### Added
- Initial hot reload implementation
- Basic worker system
- Simple routing
- File watching (basic)
- Port allocation (static)

#### Known Issues
- Port conflicts
- No health checking
- Manual restart required for server changes
- Limited error handling

### Version 0.7.0 - November 2025

**Type**: Experimental  
**Focus**: Architecture Exploration

#### Added
- Worker isolation concept
- Process-per-route model
- Basic proxy implementation
- Configuration system

## Upgrading

See the [Upgrade Guide](upgrade.md) for detailed instructions on upgrading between versions.

## Reporting Issues

Found a bug or have a feature request?

1. Check [existing issues](https://github.com/mevdschee/tqserver/issues)
2. Create a new issue with:
   - TQServer version
   - Go version
   - Operating system
   - Steps to reproduce
   - Expected vs actual behavior
   - Relevant logs

## Security Vulnerabilities

To report a security vulnerability, please email security@tqserver.dev instead of using the issue tracker.

We aim to:
- Acknowledge within 48 hours
- Provide a fix within 7 days for critical issues
- Credit reporters in release notes (if desired)

## Release Channels

### Stable
- Production-ready releases
- Semantic versioning
- Full testing
- Release notes

### Beta
- Feature-complete pre-releases
- Community testing
- May have minor bugs
- 2-4 weeks before stable

### Alpha
- Early feature previews
- Active development
- May be unstable
- Frequent updates

### Development
- Latest commits
- Cutting edge
- May be broken
- For contributors only

## Stay Updated

- **GitHub**: Watch the repository for releases
- **Twitter**: Follow @tqserver
- **Blog**: blog.tqserver.dev
- **Newsletter**: Subscribe for major releases
- **Discord**: Join the community

## Contributing

Want to help with development? See the [Contribution Guide](contributions.md) for details on how to contribute to TQServer.

## License

TQServer is open-source software. See the LICENSE file for more information.
