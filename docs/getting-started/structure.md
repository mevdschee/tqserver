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
├── config/                 # Configuration files
├── docs/                   # Documentation
├── examples/               # Example code
├── logs/                   # Log files
├── pkg/                    # Shared library code
├── scripts/                # Build and utility scripts
├── server/                 # Main server application
│   ├── src/                # Server source code
│   ├── bin/                # Compiled server binary
│   ├── config/             # Server configuration
│   ├── views/              # HTML templates
│   ├── public/             # Public assets
│   └── data/               # Worker data files
├── spec/                   # Design specifications
├── workers/                # Worker applications
│   └── {worker_name}/      # Individual worker directories
│       ├── src/            # Worker source code
│       ├── bin/            # Compiled worker binary
│       ├── config/         # Worker configuration
│       ├── views/          # HTML templates
│       ├── public/         # Public assets
│       └── data/           # Worker data files
├── go.mod                  # Go module definition
├── go.sum                  # Go dependencies checksum
├── README.md               # Project documentation
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
├── index/                  # Example worker
│   ├── src/                # Source code
│   │   └── main.go
│   ├── bin/                # Compiled binaries (auto-generated)
│   │   └── index
│   ├── config/             # Worker configuration
│   │   └── worker.yaml
│   ├── views/              # HTML templates
│   │   ├── base.html
│   │   ├── hello.html
│   │   └── index.html
│   ├── public/             # Public static assets (CSS, JS, images)
│   └── data/               # Worker data files
└── api/                    # Another example worker
    ├── src/
    ├── bin/
    ├── config/
    ├── views/
    ├── public/
    └── data/
```

### Worker Naming

- Worker directory names are used for identification
- URL paths are configured in each worker's `config/worker.yaml` file
- The `path` field in worker.yaml determines the route

Examples:
- `workers/index/config/worker.yaml` with `path: "/"` → handles root requests
- `workers/api/config/worker.yaml` with `path: "/api"` → handles `/api/*` requests
- `workers/admin/config/worker.yaml` with `path: "/admin"` → handles `/admin/*` requests

## The Server Directory

The `server` directory contains the main TQServer application that manages workers:

```
server/
├── src/                  # Server source code
│   ├── config.go         # Configuration loading and parsing
│   ├── main.go           # Application entry point
│   ├── proxy.go          # HTTP proxy implementation
│   ├── router.go         # Request routing logic
│   └── supervisor.go     # Worker supervision and management
└── bin/                  # Compiled server binary (auto-generated)
    └── tqserver
```

### Server Components

#### config.go
Handles loading and parsing YAML configuration files. Provides the `Config` struct and worker configuration loading.

#### proxy.go
Implements HTTP proxy functionality to forward requests from the main server to worker processes.

#### router.go
Manages request routing based on worker configurations. Maps incoming requests to appropriate workers based on path prefixes.

#### supervisor.go
Orchestrates worker lifecycle management:
- Building workers from source
- Starting and stopping worker processes
- Port allocation
- File watching and hot reloading
- Graceful restarts

## The Pkg Directory

The `pkg` directory contains shared library code that could be imported by other projects:

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
├── views/                 # HTML templates
│   └── *.html
├── data/                  # Data files
├── config/                # Worker-specific config
│   └── worker.yaml
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

### Views Directory

The `views/` directory contains HTML templates:

```
views/
├── layout.html
├── home.html
└── partials/
    ├── header.html
    └── footer.html
```

### Config Directory

The `config/` directory contains worker-specific configuration:

```
config/
└── worker.yaml       # Worker configuration
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
- **Templates**: Rendered by handlers
  - `views/home.html` → Rendered by handlers

## Next Steps

- [Creating Workers](../workers/creating.md) - Build your first worker
- [Routing](../basics/routing.md) - Learn about URL routing
- [Templates](../basics/templates.md) - Use the template engine
- [Static Assets](../assets/organization.md) - Organize your assets
