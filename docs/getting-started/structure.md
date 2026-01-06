# Directory Structure

- [Introduction](#introduction)
- [Root Directory](#root-directory)
- [The Config Directory](#the-config-directory)
- [The Workers Directory](#the-workers-directory)
- [The Internal Directory](#the-internal-directory)
- [The Pkg Directory](#the-pkg-directory)
- [The Scripts Directory](#the-scripts-directory)
- [Worker Structure](#worker-structure)

## Introduction

TQServer follows a well-organized directory structure that separates concerns and makes the codebase easy to navigate. Understanding this structure is essential for effective development.

## Root Directory

```
tqserver/
├── bin/                    # Compiled binaries
├── cmd/                    # Application entry points
├── config/                 # Configuration files
├── internal/               # Private application code
├── logs/                   # Log files
├── pages/                  # Legacy pages (being phased out)
├── pkg/                    # Public library code
├── scripts/                # Build and utility scripts
├── server/                 # Legacy server (being phased out)
├── spec/                   # Design specifications
├── templates/              # Server-level templates
├── workers/                # Worker applications
├── go.mod                  # Go module definition
├── go.sum                  # Go dependencies checksum
├── README.md               # Project documentation
├── REFACTORING_SUMMARY.md  # Refactoring notes
└── start.sh                # Quick start script
```

## The Config Directory

The `config` directory contains all configuration files:

```
config/
├── server.yaml         # Main server configuration
└── server.example.yaml # Example configuration template
```

### Configuration Files

- **server.yaml**: Active server configuration (ignored by git)
- **server.example.yaml**: Template for new deployments

Learn more about configuration in the [Configuration](configuration.md) guide.

## The Workers Directory

The `workers` directory contains all worker applications. Each worker is a self-contained application with its own source code, assets, and compiled binaries:

```
workers/
├── index/              # Example worker
│   ├── bin/           # Compiled binaries (auto-generated)
│   │   └── index
│   ├── private/       # Private templates and assets
│   │   └── views/
│   │       ├── hello.html
│   │       └── index.html
│   ├── public/        # Public static assets
│   ├── src/           # Source code
│   │   └── main.go
│   └── config.yaml    # Worker-specific configuration (optional)
└── api/               # Another example worker
    ├── bin/
    ├── private/
    ├── public/
    └── src/
```

### Worker Naming

- Worker directory names map to URL paths
- `index` worker handles `/` requests
- Other workers handle `/{worker-name}/*` requests

Examples:
- `workers/index/` → `http://localhost:8080/`
- `workers/api/` → `http://localhost:8080/api/*`
- `workers/admin/` → `http://localhost:8080/admin/*`

## The Internal Directory

The `internal` directory contains private application code that is not intended to be imported by other projects:

```
internal/
├── config/            # Configuration loading and parsing
│   └── config.go
├── proxy/             # HTTP proxy implementation
│   └── proxy.go
├── router/            # Request routing logic
│   ├── router.go
│   └── worker.go
└── supervisor/        # Worker supervision and management
    ├── cleanup.go     # Binary cleanup
    ├── healthcheck.go # Health checking
    ├── ports.go       # Port pool management
    └── supervisor.go  # Main supervisor logic
```

### Internal Packages

#### config
Handles loading and parsing YAML configuration files. Provides the `Config` struct used throughout the application.

#### proxy
Implements HTTP proxy functionality to forward requests from the main server to worker processes.

#### router
Manages request routing based on URL paths and filesystem structure. Maps incoming requests to appropriate workers.

#### supervisor
Orchestrates worker lifecycle management:
- Building workers from source
- Starting and stopping worker processes
- Health checking
- Graceful restarts
- Port allocation
- Binary cleanup

## The Pkg Directory

The `pkg` directory contains library code that could be imported by other projects:

```
pkg/
├── supervisor/        # Supervisor utilities
│   ├── checker.go     # Health check logic
│   ├── registry.go    # Worker registry
│   ├── timestamps.go  # Timestamp tracking
│   └── timestamps_test.go
├── watcher/           # File watching
│   └── filewatcher.go
└── worker/            # Worker runtime utilities
    └── runtime.go
```

### Public Packages

#### supervisor
Reusable components for worker supervision:
- Health check implementations
- Worker state tracking
- Timestamp-based cleanup logic

#### watcher
File system watching using `fsnotify`:
- Monitors source files for changes
- Triggers automatic rebuilds
- Debounces rapid changes

#### worker
Runtime utilities for worker processes:
- Environment setup
- Signal handling
- Graceful shutdown

## The Scripts Directory

The `scripts` directory contains build and utility scripts:

```
scripts/
├── build-dev.sh       # Development build
└── build-prod.sh      # Production build
```

### Build Scripts

#### build-dev.sh
Builds the server and all workers for development:
- Includes debug symbols
- Faster compilation
- Verbose output

```bash
./scripts/build-dev.sh
```

#### build-prod.sh
Optimized production build:
- Strips debug symbols
- Smaller binaries
- Optimized compilation

```bash
./scripts/build-prod.sh
```

## Worker Structure

Each worker follows a consistent structure:

```
workers/{worker-name}/
├── bin/                   # Compiled binaries
│   └── {worker-name}      # The executable
├── private/               # Private server-side assets
│   ├── views/            # Templates
│   │   └── *.html
│   ├── data/             # Data files
│   └── config/           # Worker-specific config
├── public/                # Public static assets
│   ├── css/
│   ├── js/
│   ├── images/
│   └── fonts/
├── src/                   # Go source code
│   ├── main.go           # Entry point
│   ├── handlers.go       # HTTP handlers
│   ├── models.go         # Data models
│   └── utils.go          # Utility functions
├── config.yaml            # Worker configuration (optional)
└── README.md              # Worker documentation (optional)
```

### Source Directory

The `src/` directory contains all Go source code:

```
src/
├── main.go            # Entry point with main() function
├── handlers/          # HTTP request handlers
│   ├── home.go
│   └── api.go
├── models/            # Data structures
│   └── user.go
├── services/          # Business logic
│   └── auth.go
└── middleware/        # Middleware functions
    └── logger.go
```

### Private Directory

The `private/` directory contains server-side assets not served directly:

```
private/
├── views/             # HTML templates
│   ├── layout.html
│   ├── home.html
│   └── partials/
│       ├── header.html
│       └── footer.html
├── data/              # JSON, YAML, etc.
│   └── config.json
└── keys/              # Certificates, keys
    └── private.key
```

### Public Directory

The `public/` directory contains static assets served directly:

```
public/
├── css/
│   ├── style.css
│   └── theme.css
├── js/
│   ├── app.js
│   └── vendor/
│       └── jquery.js
├── images/
│   ├── logo.png
│   └── icons/
└── fonts/
    └── custom.woff2
```

## Best Practices

### Organization Tips

1. **Keep workers focused**: Each worker should handle a specific domain (API, admin, public site)
2. **Use consistent structure**: Follow the same structure across all workers
3. **Separate concerns**: Keep handlers, models, and services in separate files
4. **Use packages**: Group related functionality into packages within `src/`

### File Naming

- Use lowercase with underscores: `user_handler.go`
- Group related files: `user.go`, `user_test.go`, `user_repository.go`
- Main entry point: Always `main.go` in the `src/` directory

### Asset Organization

- **Public assets**: Directly accessible URLs
  - `public/css/style.css` → `/css/style.css`
- **Private assets**: Only accessible via server code
  - `private/views/home.html` → Rendered by handlers

## Next Steps

- [Creating Workers](../workers/creating.md) - Build your first worker
- [Routing](../basics/routing.md) - Learn about URL routing
- [Templates](../basics/templates.md) - Use the template engine
- [Static Assets](../assets/organization.md) - Organize your assets
