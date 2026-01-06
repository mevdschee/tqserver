package devmode

import (
"log"
"sync"

"github.com/mevdschee/tqserver/pkg/builder"
"github.com/mevdschee/tqserver/pkg/watcher"
)

// RestartHandler is called when a worker needs to be restarted.
type RestartHandler func(workerName string)

// DevMode manages development mode with file watching and auto-rebuild.
type DevMode struct {
watcher        *watcher.FileWatcher
builder        *builder.Builder
restartHandler RestartHandler
serverRestart  func()
mu             sync.Mutex
running        bool
}

// Config holds dev mode configuration.
type Config struct {
WorkersDir     string
ServerDir      string
ConfigDir      string
DebounceMs     int
RestartHandler RestartHandler
ServerRestart  func()
}

// New creates a new dev mode manager.
func New(cfg Config) (*DevMode, error) {
dm := &DevMode{
builder:        builder.NewBuilder(cfg.WorkersDir, cfg.ServerDir),
restartHandler: cfg.RestartHandler,
serverRestart:  cfg.ServerRestart,
}

// Create file watcher with our change handler
fw, err := watcher.NewFileWatcher(
cfg.WorkersDir,
cfg.ServerDir,
cfg.ConfigDir,
cfg.DebounceMs,
dm.handleChange,
)
if err != nil {
return nil, err
}
dm.watcher = fw

return dm, nil
}

// Start begins watching for file changes.
func (dm *DevMode) Start() error {
dm.mu.Lock()
defer dm.mu.Unlock()

if dm.running {
return nil
}

log.Println("Starting development mode...")
if err := dm.watcher.Start(); err != nil {
return err
}

dm.running = true
log.Println("Development mode started - watching for file changes")
return nil
}

// Stop stops the file watcher.
func (dm *DevMode) Stop() {
dm.mu.Lock()
defer dm.mu.Unlock()

if !dm.running {
return
}

log.Println("Stopping development mode...")
dm.watcher.Stop()
dm.running = false
}

// handleChange processes file change events.
func (dm *DevMode) handleChange(event watcher.ChangeEvent) {
switch event.ChangeType {
case "source":
if event.WorkerName != "" {
// Worker source changed - rebuild and restart
log.Printf("Worker %s source changed, rebuilding...", event.WorkerName)
if err := dm.rebuildWorker(event.WorkerName); err != nil {
log.Printf("Failed to rebuild worker %s: %v", event.WorkerName, err)
return
}
// Trigger restart
if dm.restartHandler != nil {
dm.restartHandler(event.WorkerName)
}
} else {
// Server source changed - rebuild and restart server
log.Println("Server source changed, rebuilding...")
if err := dm.builder.BuildServer(); err != nil {
log.Printf("Failed to rebuild server: %v", err)
return
}
log.Println("Server rebuilt successfully - restart required")
if dm.serverRestart != nil {
dm.serverRestart()
}
}

case "asset":
// Asset changed - just notify, worker can reload templates
log.Printf("Worker %s assets changed", event.WorkerName)
if dm.restartHandler != nil {
dm.restartHandler(event.WorkerName)
}

case "config":
// Config changed - reload configuration
log.Println("Configuration changed - server restart recommended")
if dm.serverRestart != nil {
dm.serverRestart()
}
}
}

// rebuildWorker rebuilds a specific worker.
func (dm *DevMode) rebuildWorker(workerName string) error {
result, err := dm.builder.BuildWorker(workerName)
if err != nil {
return err
}
if !result.Success {
return result.Error
}
return nil
}

// BuildAll builds all workers and the server.
func (dm *DevMode) BuildAll() error {
return dm.builder.BuildAll()
}
