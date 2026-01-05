# TQServer

A high-performance function execution platform built with Go that provides
sub-second hot reloads with native Go performance.

## Features

- ✅ **Sub-second hot reloads** - Changes to page code are automatically
  detected, rebuilt, and deployed in ~0.3-1.0 seconds
- ✅ **Filesystem-based routing** - URL structure mirrors your filesystem layout
- ✅ **Graceful worker restarts** - Zero-downtime deployments with automatic
  traffic switching
- ✅ **Native Go performance** - Workers are compiled Go binaries, not
  interpreted scripts
- ✅ **Process isolation** - Each route runs in its own process
- ✅ **Automatic builds** - File watching and automatic compilation on changes

## Quick Start

### 1. Build the server

```bash
cd server
go build -o ../tqserver
```

### 2. Configure (optional)

Edit `config/server.yaml` to customize:
- Server port (default: 8080)
- Worker port range (default: 9000-9999)
- Timeouts (read, write, idle)
- Worker startup and restart delays
- Pages directory location

### 3. Run the server

```bash
./tqserver
```

The server will listen on port **8080** by default (or as configured) and serve
pages from the `pages/` directory.

Visit http://localhost:8080 to see it in action!

### 4. Edit and watch hot reload

Edit `pages/index/main.go` and save. The server will automatically rebuild and
reload in under 1 second with zero downtime.

## Command Line Options

```bash
./tqserver [options]

Options:
  -port int
        Port to listen on (default 8080)
  -pages string
        Directory containing page handlers (default "pages")
```

## Documentation

See [project_brief.md](project_brief.md) for complete architecture
documentation.
