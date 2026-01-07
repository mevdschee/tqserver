# Mode Controller Usage Guide

## Overview

The mode controller provides unified management of development and production deployment modes with automatic file watching (dev) or SIGHUP-triggered checking (prod).

## Environment Variables

- `TQ_MODE` or `DEPLOYMENT_MODE`: Set to `dev` or `prod`
- Default: `dev` if not set

## Development Mode

### Features
- Automatic file watching using fsnotify
- Auto-rebuild on source file changes
- Hot reload of workers after successful build
- Immediate feedback for rapid development

### Usage

```bash
# Set dev mode
export TQ_MODE=dev

# Run server - file watching starts automatically
./server/bin/tqserver
```

### What happens when you edit files:

1. **Worker Source (`workers/*/src/*.go`)**:
   - File watcher detects change
   - Builder rebuilds the worker
   - Worker is automatically restarted
   - New binary loaded

2. **Server Source (`server/src/*.go`)**:
   - File watcher detects change
   - Server is rebuilt
   - Manual restart required (external process manager)

3. **Config files (`config/*.yaml`)**:
   - File watcher detects change
   - Server restart recommended

## Production Mode

### Features
- SIGHUP signal-triggered checking (no continuous polling)
- File timestamp comparison (mtime)
- Smart restart: binary vs assets detection
- Zero-downtime restart coordination
- Efficient and predictable

### Usage

```bash
# Set prod mode
export TQ_MODE=prod

# Run server - SIGHUP handler registers
./server/bin/tqserver
```

### Deployment Workflow

1. **Deploy new files via rsync**:
```bash
rsync -avz --checksum workers/index/ server:/opt/tqserver/workers/index/
```

2. **Trigger reload via SIGHUP**:
```bash
ssh server 'killall -HUP tqserver'
```

3. **Server detects changes**:
   - Compares file mtimes against recorded values
   - Detects binary changes
   - Restarts affected workers automatically

### Change Detection

- **Binary changed**: Full worker restart (new port, health check, traffic switch)

## Integration Example

```go
import (
    "github.com/mevdschee/tqserver/pkg/modecontroller"
    "github.com/mevdschee/tqserver/pkg/coordinator"
)

// Create worker manager (implements WorkerRestarter interface)
type MyWorkerManager struct {
    // ... your worker management code
}

func (m *MyWorkerManager) RestartWorker(name string) error {
    // Your restart logic
    return nil
}

func (m *MyWorkerManager) RestartServer() error {
    // Your server restart logic
    return nil
}

// Initialize mode controller
manager := &MyWorkerManager{}

controller, err := modecontroller.New(modecontroller.Config{
    Mode:            modecontroller.GetModeFromEnv(),
    WorkersDir:      "workers",
    ServerDir:       "server",
    ConfigDir:       "config",
    ServerBinPath:   "server/bin/tqserver",
    DebounceMs:      100,
    WorkerRestarter: manager,
})

// Start watching/listening
controller.Start()
defer controller.Stop()
```

## Architecture

```
┌─────────────────────────────────────────┐
│       Mode Controller                   │
│  (reads TQ_MODE environment)            │
└─────────────┬───────────────────────────┘
              │
    ┌─────────┴─────────┐
    │                   │
┌───▼────────┐   ┌─────▼────────┐
│  Dev Mode  │   │  Prod Mode   │
├────────────┤   ├──────────────┤
│ • Watcher  │   │ • SIGHUP     │
│ • Builder  │   │ • Timestamps │
│ • Auto     │   │ • Registry   │
│   Restart  │   │ • Smart      │
└────────────┘   │   Restart    │
                 └──────────────┘
```

## Testing

Run the example:
```bash
# Dev mode
TQ_MODE=dev go run examples/mode_example.go

# Prod mode (in another terminal)
TQ_MODE=prod go run examples/mode_example.go
killall -HUP mode_example  # Trigger check
```

## Benefits

### Development
✅ Instant feedback on code changes
✅ No manual rebuild/restart needed
✅ Fast iteration cycle
✅ Automatic error detection

### Production
✅ No continuous file system polling
✅ Predictable behavior (SIGHUP trigger)
✅ Zero-downtime updates
✅ Smart restart logic
✅ Efficient resource usage

## Next Steps

1. Integrate with existing supervisor code
2. Add worker registry population on startup
3. Implement zero-downtime port switching
4. Add health check integration
5. Create deployment scripts
