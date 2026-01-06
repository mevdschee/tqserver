package watcher

import (
"log"
"os"
"path/filepath"
"strings"
"sync"
"time"

"github.com/fsnotify/fsnotify"
)

type ChangeEvent struct {
Path       string
WorkerName string
ChangeType string
}

type ChangeHandler func(event ChangeEvent)

type FileWatcher struct {
watcher        *fsnotify.Watcher
handler        ChangeHandler
debounceMs     int
workersDir     string
serverDir      string
configDir      string
stopChan       chan struct{}
debounceMu     sync.Mutex
debounceTimers map[string]*time.Timer
}

func NewFileWatcher(workersDir, serverDir, configDir string, debounceMs int, handler ChangeHandler) (*FileWatcher, error) {
watcher, err := fsnotify.NewWatcher()
if err != nil {
return nil, err
}

fw := &FileWatcher{
watcher:        watcher,
handler:        handler,
debounceMs:     debounceMs,
workersDir:     workersDir,
serverDir:      serverDir,
configDir:      configDir,
stopChan:       make(chan struct{}),
debounceTimers: make(map[string]*time.Timer),
}

return fw, nil
}

func (fw *FileWatcher) Start() error {
if err := fw.addRecursive(fw.workersDir); err != nil {
return err
}
if err := fw.addRecursive(fw.serverDir); err != nil {
return err
}
if err := fw.watcher.Add(fw.configDir); err != nil {
return err
}

go fw.watchLoop()
log.Println("File watcher started")
return nil
}

func (fw *FileWatcher) Stop() {
close(fw.stopChan)
fw.watcher.Close()
log.Println("File watcher stopped")
}

func (fw *FileWatcher) watchLoop() {
for {
select {
case event, ok := <-fw.watcher.Events:
if !ok {
return
}
fw.handleEvent(event)

case err, ok := <-fw.watcher.Errors:
if !ok {
return
}
log.Printf("File watcher error: %v", err)

case <-fw.stopChan:
return
}
}
}

func (fw *FileWatcher) handleEvent(event fsnotify.Event) {
if event.Op&fsnotify.Write == 0 && event.Op&fsnotify.Create == 0 {
return
}

if strings.Contains(event.Name, "/.") || strings.HasSuffix(event.Name, "~") {
return
}

if event.Op&fsnotify.Create != 0 {
if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
fw.watcher.Add(event.Name)
}
}

fw.debounceMu.Lock()
defer fw.debounceMu.Unlock()

if timer, exists := fw.debounceTimers[event.Name]; exists {
timer.Stop()
}

fw.debounceTimers[event.Name] = time.AfterFunc(
time.Duration(fw.debounceMs)*time.Millisecond,
func() {
fw.processChange(event.Name)
fw.debounceMu.Lock()
delete(fw.debounceTimers, event.Name)
fw.debounceMu.Unlock()
},
)
}

func (fw *FileWatcher) processChange(path string) {
var event ChangeEvent
event.Path = path

if strings.HasPrefix(path, fw.workersDir+"/") {
relPath := strings.TrimPrefix(path, fw.workersDir+"/")
parts := strings.Split(relPath, "/")
if len(parts) > 0 {
event.WorkerName = parts[0]
}

if strings.Contains(path, "/src/") && strings.HasSuffix(path, ".go") {
event.ChangeType = "source"
} else if strings.Contains(path, "/public/") {
event.ChangeType = "asset"
} else if strings.Contains(path, "/private/") {
event.ChangeType = "asset"
} else {
return
}

} else if strings.HasPrefix(path, fw.serverDir+"/") {
event.WorkerName = ""
if strings.HasSuffix(path, ".go") {
event.ChangeType = "source"
} else {
return
}

} else if strings.HasPrefix(path, fw.configDir+"/") {
event.WorkerName = ""
event.ChangeType = "config"

} else {
return
}

if fw.handler != nil {
log.Printf("File changed: %s (worker: %s, type: %s)", path, event.WorkerName, event.ChangeType)
fw.handler(event)
}
}

func (fw *FileWatcher) addRecursive(root string) error {
return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
if err != nil {
return err
}

if strings.Contains(path, "/.") {
if info.IsDir() {
return filepath.SkipDir
}
return nil
}

if info.IsDir() {
if addErr := fw.watcher.Add(path); addErr != nil {
log.Printf("Warning: could not watch %s: %v", path, addErr)
}
}

return nil
})
}
