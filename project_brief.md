# TQServer: High-Performance Function Execution System

## Executive Summary

TQServer is a high-performance function execution platform built with Go. The
architecture accepts 1-second build times as optimal, delivering sub-second
reload times with native Go performance. The system consists of a Go supervisor
managing dynamically rebuilt worker binaries, using HTTP workers behind a
reverse proxy.

## Core Technology Stack

**Architecture:** Go supervisor + Go builds + HTTP workers

**Key Components:**

- Supervisor process (Go)
- Worker binaries (compiled Go functions)
- HTTP-based communication (no SSL, localhost)

## Performance Characteristics

The system delivers exceptional performance across all metrics:

| Metric           | Performance                                    |
| ---------------- | ---------------------------------------------- |
| **Cold start**   | ~1–5 ms (Go binary startup)                    |
| **Rebuild time** | ~0.3–1.0 seconds for small functions           |
| **Reload time**  | ~5–20 ms (supervisor swap)                     |
| **Throughput**   | As fast as native Go (because it is native Go) |

## System Architecture

### Component 1: Supervisor Process

**Implementation Language:** Go

**Location:** `server/` directory in project root

**Components:**

- **supervisor.go** - Core supervisor logic (file watching, builds, process
  management)
- **proxy.go** - HTTP reverse proxy and load balancer
- **router.go** - Route discovery and request routing
- **main.go** - Entry point that ties everything together

**Core Responsibilities:**

#### Directory Monitoring

- Uses `fsnotify` to watch directories for changes
- Detects file modifications in real-time

#### Build Pipeline

On change detection:

- Execute `go build -o foo` from page directory
- Compile Go source files from page into worker binary
- Each page directory contains Go source files (main.go, handlers, etc.)

#### Routing Management

- Adjust routing table to point to new worker
- Maintain traffic flow during transitions

### Component 2: Function Binaries

Each function runs as an independent, self-contained executable:

- Tiny HTTP listening Go program
- Built from Go source files in page directories
- Runs in isolation from other functions
- Hot-swappable during runtime

#### Source Code Organization

**Page Directories:** Each page lives in its own directory containing Go source
files:

```
pages/
├── api/
│   └── users/
│       ├── main.go       # Entry point
│       ├── handlers.go   # HTTP handlers
│       └── models.go     # Data models
```

**Build Process:** The supervisor compiles all Go files in a page directory into
a single worker binary.

#### Working Directory Configuration

**Critical:** Each worker process has its working directory set to the **project
root**, not the page directory.

**Benefits:**

- All file paths are relative to project root
- Shared resources easily accessible (config files, templates, static assets)
- Consistent path resolution across all workers

**Example:**

```
project-root/
├── server/              # Supervisor and HTTP server/LB
│   ├── main.go
│   ├── supervisor.go
│   ├── proxy.go
│   └── router.go
├── config/
│   └── settings.yaml
├── templates/
│   └── email.html
├── pages/
│   └── api/
│       └── users/
│           └── main.go
```

**In worker code:**

```go
// Working directory is project-root/, so paths are relative to root
config, _ := os.ReadFile("config/settings.yaml")  // ✅ Works
template, _ := os.ReadFile("templates/email.html") // ✅ Works
```

**Supervisor ensures:**

- Worker binary built from `pages/api/users/`
- Worker process started with `cwd = project-root/`
- All file operations relative to project root

### Component 3: Graceful Restart Mechanism

The supervisor implements zero-downtime restarts through a coordinated sequence:

**Restart Sequence:**

1. Starts new worker process
2. Waits for it to bind the socket
3. Stops routing to old worker
4. Kills old worker after configured timeout

### Component 4: PHP-FPM Style Process Management

The system mirrors PHP-FPM's robust process management patterns:

**Worker Lifecycle Features:**

- **min/max workers configuration** - Dynamic pool sizing
- **restart after N requests** - Periodic worker recycling
- **memory limits enforcement** - Resource constraint management
- **automatic crash restart** - Self-healing on failures

## Graceful Restart Strategy

### Design Philosophy

Graceful restarts with HTTP workers provide a straightforward, robust, and
Go-idiomatic approach. The supervisor manages:

- Port allocation
- Health checks
- Connection draining

### Core Restart Pattern

**Multi-Worker Model:** Each function maintains multiple worker processes
simultaneously.

**Rebuild Workflow:**

1. **Start** - Launch new worker on a new port
2. **Validate** - Wait until it's healthy
3. **Switch** - Update routing so new requests go to the new worker
4. **Drain** - Stop sending traffic to the old worker
5. **Terminate** - Kill old worker after timeout

**Industry Precedent:** This pattern mirrors the approach used by
production-grade systems:

- Envoy (service mesh)
- Caddy (web server)
- systemd socket activation

The implementation is simplified for this specific use case while maintaining
the same reliability guarantees.

## Detailed Graceful HTTP Worker Restart Architecture

### Supervisor Responsibilities

The supervisor orchestrates the entire restart process:

- **Code monitoring** - Watches for code changes
- **Binary compilation** - Rebuilds the worker binary
- **Process management** - Starts new worker processes
- **Traffic routing** - Maintains a routing table
- **Connection draining** - Drains old workers
- **Health verification** - Handles health checks

### Worker Requirements

Workers are designed as simple, stateless HTTP servers:

- **Plain Go HTTP servers** - Standard `net/http` implementation
- **Dynamic port binding** - Bind to a port assigned by the supervisor
- **Health endpoint** - Expose a `/health` endpoint
- **Graceful shutdown** - Shut down gracefully when signaled

## HTTP Reverse Proxy Architecture

### Supervisor as Reverse Proxy

The supervisor functions as a reverse proxy accepting HTTP connections (no SSL,
localhost only) and routing them to backend HTTP workers. This provides a clean,
unified entry point for all function execution.

### Supervisor Capabilities

The supervisor provides comprehensive HTTP management:

- **Accepts HTTP** (localhost only) - Direct HTTP connections without SSL
- **Acts as a reverse proxy** - Routes traffic to backend workers
- **Routes to HTTP workers** - Distributes requests across worker pool
- **Manages graceful restarts** - Zero-downtime deployments
- **Provides hot reloads after rebuilds** - Instant code updates

## Filesystem-Based Routing

### Overview

The routing system uses directory structure to automatically determine URL
routes. Each page directory contains Go source files that are compiled into a
worker binary. This provides an intuitive, convention-based routing mechanism
where the filesystem layout directly maps to the API structure.

### Source Code Structure

**Project Layout:** The system is organized with clear separation between
infrastructure and application code.

```
project-root/
├── server/              # Supervisor and HTTP infrastructure
│   ├── main.go          # Entry point for supervisor
│   ├── supervisor.go    # Process management, builds, restarts
│   ├── proxy.go         # HTTP reverse proxy/load balancer
│   ├── router.go        # Route discovery and matching
│   └── health.go        # Health check management
├── config/
│   └── settings.yaml
├── shared/              # Shared resources accessible from project root
│   ├── templates/
│   └── utils/
└── pages/
    ├── api/
    │   ├── users/
    │   │   ├── main.go       # Serves /api/users
    │   │   └── handlers.go
    │   ├── posts/
    │   │   └── main.go       # Serves /api/posts
    │   └── v1/
    │       ├── users/
    │       │   └── main.go   # Serves /api/v1/users
    │       └── orders/
    │           └── main.go   # Serves /api/v1/orders
    ├── index/
    │   └── main.go           # Serves / (fallback)
    └── webhooks/
        ├── github/
        │   └── main.go       # Serves /webhooks/github
        └── stripe/
            └── main.go       # Serves /webhooks/stripe
```

**Server Directory:** Contains the supervisor and HTTP server implementation:

- **main.go** - Starts supervisor, initializes proxy and router
- **supervisor.go** - Watches pages, triggers builds, manages worker lifecycle
- **proxy.go** - HTTP reverse proxy and load balancer functionality
- **router.go** - Filesystem-based route discovery and request routing
- **health.go** - Worker health checks and monitoring

**Pages Directory:** Each route is implemented as a directory containing Go
source files.

### Routing Conventions

#### Option 1: Directory Path Mapping (Recommended)

The directory path relative to the pages root determines the URL path.

**Routing Logic:**

- Page at `pages/api/users/` → Routes to `/api/users`
- Page at `pages/api/v1/orders/` → Routes to `/api/v1/orders`
- Page at `pages/index/` → Routes to `/` (catch-all fallback)

**Worker Binary:** Supervisor compiles all Go files in page directories into
worker executables.

**Working Directory:** All workers run with `cwd = project-root/`, enabling
relative file access, while also allowing access to files relative to the
executable path (from argv[0]).

### Implementation Details

#### Discovery Process

1. **Scan pages directory** - Recursively find all workers
2. **Parse paths** - Extract route from relative path
3. **Build routing table** - Map routes to executables (and their ports)
4. **Watch for changes** - Update routing table on file changes
5. **Fallback handler** - Optional catch-all for 404s

### Advantages

- **Intuitive** - URL structure mirrors filesystem
- **Zero configuration** - Routes auto-discovered from filesystem
- **Flexible** - Support for method-specific and method-agnostic handlers
- **Scalable** - Easy to add new routes by creating files
- **Version control friendly** - Routes defined by file structure
- **Zero downtime** - On binary replacement zero downtime deployment starts

## Cluster Deployment Architecture

### Scaling from Single-Node to Multi-Node

The transition from single-node to multi-node deployment requires additional
infrastructure components.

**Key additions for cluster mode:**

### 1. Shared Code Distribution

**Challenge:** Updated function code must reach all cluster nodes.

**Solution:** Git pull on all nodes and rebuild binaries on each node.

**Implementation:**

- Each node maintains a local Git repository
- Nodes pull from a central repository when notified
- Notifications trigger rebuild and restart process

**Recommendation:** This is the simplest approach for code distribution.

### 2. Shared Configuration

**Challenge:** Configuration and views must be consistent across all nodes.

**Solution:** Read config and views during runtime, distribute with the code.

**Implementation:**

- Configuration file distributed alongside code
- Version control ensures consistency
- Updates deployed atomically with code changes

**Recommendation:** This is the simplest approach for configuration management.

### 3. Optional: Distributed Cache

**Purpose:** Share state and cached data across cluster nodes for improved
performance and consistency.
