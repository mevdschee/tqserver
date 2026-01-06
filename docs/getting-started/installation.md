# Installation

- [Meet TQServer](#meet-tqserver)
- [System Requirements](#system-requirements)
- [Installing TQServer](#installing-tqserver)
- [Initial Configuration](#initial-configuration)
- [Next Steps](#next-steps)

## Meet TQServer

TQServer is a high-performance function execution platform built with Go that provides sub-second hot reloads with native Go performance. It combines the developer experience of hot reloading with the performance of compiled binaries.

### Why TQServer?

TQServer bridges the gap between development speed and runtime performance:

- **Fast Development**: Sub-second hot reloads (~0.3-1.0s) mean you see changes instantly
- **Native Performance**: Workers are compiled Go binaries, not interpreted scripts
- **Process Isolation**: Each route runs in its own process for better stability
- **Zero Downtime**: Graceful worker restarts ensure continuous availability
- **Filesystem Routing**: URL structure mirrors your filesystem for intuitive organization

## System Requirements

Before installing TQServer, ensure your development environment meets these requirements:

- **Go 1.24 or higher**
- **Operating System**: Linux, macOS, or Windows with WSL2
- **Memory**: Minimum 512MB RAM (1GB+ recommended)
- **Disk Space**: At least 100MB for the framework and dependencies

### Verifying Go Installation

Check your Go version:

```bash
go version
```

If Go is not installed, download it from [go.dev](https://go.dev/dl/).

## Installing TQServer

### Via Go Get

The quickest way to get started with TQServer:

```bash
# Clone the repository
git clone https://github.com/mevdschee/tqserver.git
cd tqserver

# Install dependencies
go mod download

# Build the server and workers
./scripts/build-dev.sh
```

### Via Git Clone

For development or contributing:

```bash
# Clone with full history
git clone https://github.com/mevdschee/tqserver.git
cd tqserver

# Install dependencies
go mod tidy

# Build all components
./scripts/build-dev.sh
```

### Via Docker (Coming Soon)

Docker support is planned for a future release.

## Initial Configuration

### Creating Your Configuration File

TQServer uses YAML for configuration. Create your server configuration:

```bash
cp config/server.example.yaml config/server.yaml
```

Edit `config/server.yaml` with your preferences:

```yaml
server:
  host: "0.0.0.0"
  port: 8080
  log_file: "logs/server-{date}.log"
  
  port_pool:
    start: 9000
    end: 9100

workers:
  base_path: "workers"
  health_check:
    enabled: true
    interval: 30s
    timeout: 5s
    path: "/health"
```

### Directory Structure

TQServer expects the following directory structure:

```
tqserver/
├── config/
│   └── server.yaml          # Server configuration
├── workers/                 # Worker applications
│   └── index/              # Example worker
│       ├── src/            # Worker source code
│       │   └── main.go
│       ├── bin/            # Compiled binaries (auto-created)
│       ├── config/         # Worker configuration
│       │   └── worker.yaml
│       ├── views/          # HTML templates
│       ├── public/         # Public assets
│       └── data/           # Worker data files
├── logs/                   # Log files (auto-created)
└── server/
    ├── src/                # Server source code
    └── bin/                # Server binary
        └── tqserver
```

### Environment Variables

TQServer can be configured via environment variables:

```bash
# Set the config file path
export TQSERVER_CONFIG=config/server.yaml

# Enable quiet mode (suppress console output)
export TQSERVER_QUIET=true
```

## Running TQServer

Start the server:

```bash
# Using the binary
./server/bin/tqserver

# With custom config
./server/bin/tqserver -config=config/server.yaml

# Using the start script
bash start.sh
```

The server will:
1. Load configuration
2. Discover and build workers
3. Start the HTTP server
4. Begin watching for file changes

Visit `http://localhost:8080` to see your application.

## Verifying Installation

### Check Server Status

The server logs should show:

```
Starting TQServer...
Loading configuration from config/server.yaml
Building 1 workers...
Successfully built worker: index
HTTP server listening on 0.0.0.0:8080
```

### Test the Example Worker

The default installation includes an example worker:

```bash
curl http://localhost:8080/
```

You should see the index page response.

### Test Hot Reload

1. Edit `workers/index/src/main.go`
2. Save the file
3. Watch the logs for automatic rebuild
4. Refresh your browser to see changes

## Next Steps

Congratulations! You've successfully installed TQServer. Here are some next steps:

- **[Configuration](configuration.md)** - Learn about advanced configuration options
- **[Directory Structure](structure.md)** - Understand the project organization
- **[Creating Workers](../workers/creating.md)** - Build your first worker
- **[Routing](../basics/routing.md)** - Learn about filesystem-based routing
- **[Templates](../basics/templates.md)** - Use the built-in template engine

## Troubleshooting

### Port Already in Use

If port 8080 is already in use:

```yaml
# config/server.yaml
server:
  port: 8081  # Change to an available port
```

### Permission Denied

If you encounter permission errors:

```bash
# Make the binary executable
chmod +x bin/tqserver

# Ensure log directory is writable
chmod 755 logs/
```

### Build Failures

If workers fail to build:

```bash
# Clean and rebuild
rm -rf workers/*/bin/*
./scripts/build-dev.sh
```

For more help, see the [Troubleshooting Guide](../appendix/troubleshooting.md).
