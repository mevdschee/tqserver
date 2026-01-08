# WebSocket Support

*(Documentation In Progress)*

Standard WebSockets are supported by request forwarding. The Proxy automatically upgrades the connection.

## Live Reload
The Live Reload feature uses a system-dedicated WebSocket endpoint at `/ws/reload` to notify the browser when to refresh.
