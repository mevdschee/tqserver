# Live Reload System

## Overview

TQServer includes a lightweight WebSocket-based live reload system for development mode. When workers are rebuilt (successfully or with errors), all connected browser tabs automatically reload to show the updated application or error page.

## How It Works

### Architecture

```
┌─────────────┐         WebSocket          ┌─────────────┐
│   Browser   │ ←─────────────────────────→ │   Server    │
│             │   ws://localhost:8080/      │             │
│             │        /ws/reload           │             │
└─────────────┘                             └─────────────┘
       ↑                                           │
       │                                           │
       │ reload() on message                       │ broadcast on
       │                                           │ worker rebuild
       └───────────────────────────────────────────┘
```

### Components

1. **WebSocket Endpoint** (`/ws/reload`)
   - Accepts WebSocket connections from browsers
   - Maintains a list of connected clients
   - Only active in development mode

2. **Client Script** (`/dev-reload.js`)
   - Establishes WebSocket connection
   - Reloads page on receiving reload signal
   - Handles reconnection with exponential backoff
   - Cleans up on page navigation

3. **Reload Broadcaster**
   - Manages WebSocket connections
   - Broadcasts reload messages to all clients
   - Cleans up dead connections

4. **Supervisor Integration**
   - Calls broadcast after worker rebuild
   - Works for both successful builds and build errors

## Implementation Details

### Server-Side

**WebSocket Handler** (`server/src/reload.go`):
```go
type ReloadBroadcaster struct {
    clients map[*wsConn]bool
    mu      sync.RWMutex
}

func (rb *ReloadBroadcaster) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
    // Perform WebSocket handshake
    // Add client to connection pool
    // Wait for disconnect or close frame
}

func (rb *ReloadBroadcaster) BroadcastReload() {
    // Send reload message to all connected clients
    // Clean up dead connections
}
```

**Proxy Integration** (`server/src/proxy.go`):
```go
// Add WebSocket endpoint in dev mode
if p.config.IsDevelopmentMode() {
    mux.HandleFunc("/ws/reload", p.reloadBroadcaster.HandleWebSocket)
}

// Expose broadcast method
func (p *Proxy) BroadcastReload() {
    if p.reloadBroadcaster != nil {
        p.reloadBroadcaster.BroadcastReload()
    }
}
```

**Supervisor Integration** (`server/src/supervisor.go`):
```go
// After successful worker restart
log.Printf("✅ Worker reloaded for %s", worker.Route)
if s.config.IsDevelopmentMode() && s.proxy != nil {
    s.proxy.BroadcastReload()
}

// After build error (in dev mode)
if hasBuildError, _ := worker.GetBuildError(); hasBuildError {
    log.Printf("Worker %s has build error, not restarting", worker.Route)
    if s.config.IsDevelopmentMode() && s.proxy != nil {
        s.proxy.BroadcastReload()
    }
    return
}
```

### Client-Side

**Script** (`server/public/dev-reload.js`):
```javascript
(function() {
    'use strict';
    
    const wsUrl = 'ws://' + window.location.host + '/ws/reload';
    let ws;
    let isReloading = false;
    
    function connect() {
        ws = new WebSocket(wsUrl);
        
        ws.onopen = function() {
            console.log('[TQServer] Live reload connected');
        };
        
        ws.onmessage = function(event) {
            console.log('[TQServer] Reload signal received, reloading page...');
            isReloading = true;
            if (ws && ws.readyState === WebSocket.OPEN) {
                ws.close();
            }
            location.reload();
        };
        
        ws.onclose = function() {
            if (!isReloading) {
                // Reconnect with exponential backoff
                scheduleReconnect();
            }
        };
    }
    
    // Close WebSocket when navigating away
    window.addEventListener('beforeunload', function() {
        isReloading = true;
        if (ws && ws.readyState === WebSocket.OPEN) {
            ws.close();
        }
    });
    
    connect();
})();
```

**Template Integration**:
```html
<!-- In base template -->
{% if DevMode %}<script src="/dev-reload.js"></script>{% endif %}
```

**Worker Template Data**:
```go
// Pass DevMode to templates
data := map[string]interface{}{
    "DevMode": runtime.IsDevelopmentMode(),
    // ... other data
}
```

## Connection Management

### Client Lifecycle

1. **Page Load**: Script connects to WebSocket endpoint
2. **Connection Open**: Client added to server's connection pool
3. **Reload Signal**: Server sends message, client reloads page
4. **Page Unload**: `beforeunload` event closes WebSocket
5. **New Connection**: New page establishes new WebSocket

### Server Cleanup

Dead connections are cleaned up in two ways:

1. **Read Loop**: Detects when client sends close frame or connection breaks
2. **Broadcast Cleanup**: Removes connections that fail during broadcast

```go
// During broadcast
var deadClients []*wsConn
for client := range rb.clients {
    if _, err := client.conn.Write(frame); err != nil {
        client.conn.Close()
        deadClients = append(deadClients, client)
    }
}
// Remove dead connections
for _, client := range deadClients {
    delete(rb.clients, client)
}
```

## Protocol Details

### WebSocket Handshake

Standard HTTP upgrade to WebSocket:
```
GET /ws/reload HTTP/1.1
Upgrade: websocket
Connection: Upgrade
Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==

HTTP/1.1 101 Switching Protocols
Upgrade: websocket
Connection: Upgrade
Sec-WebSocket-Accept: s3pPLMBiTxaQ9kYGzzhZRbK+xOo=
```

### Frame Format

Simple text frame for reload message:
```
0x81 0x06 r e l o a d
│    │    └─ Payload (6 bytes)
│    └─ Payload length
└─ FIN=1, Opcode=1 (text)
```

Close frame:
```
0x88 0x00
│    └─ No payload
└─ FIN=1, Opcode=8 (close)
```

## Configuration

### Enable Live Reload

Automatic in development mode:
```bash
# Development mode (live reload enabled)
./server/bin/tqserver --mode dev

# Production mode (live reload disabled)
./server/bin/tqserver --mode prod
```

### Template Integration

Workers must pass `DevMode` to templates:
```go
data := map[string]interface{}{
    "DevMode": runtime.IsDevelopmentMode(),
}
```

Base template must include conditional script:
```html
{% if DevMode %}<script src="/dev-reload.js"></script>{% endif %}
```

## Troubleshooting

### Connection Count Increasing

If you see increasing connection counts, ensure:
- Client script properly closes WebSocket on `beforeunload`
- `isReloading` flag prevents reconnection during page reload
- Server detects and cleans up dead connections

### No Auto-Reload

Check:
1. Server is running in dev mode (`--mode dev`)
2. Template includes dev-reload.js script
3. `DevMode` is passed to template data
4. Browser console shows WebSocket connection
5. No browser extensions blocking WebSockets

### Frequent Disconnects

If connections drop frequently:
- Check network stability
- Review server logs for errors
- Ensure no aggressive timeout settings

## Performance Impact

The live reload system is designed to be lightweight:

- **Memory**: ~1KB per connection
- **CPU**: Minimal (idle until broadcast)
- **Network**: One WebSocket per browser tab
- **Latency**: Reload triggered in <10ms after build

## Production Behavior

In production mode:
- WebSocket endpoint is not registered
- Dev-reload.js is not included in templates
- No live reload functionality
- Zero overhead

This ensures production performance is not affected by development features.
