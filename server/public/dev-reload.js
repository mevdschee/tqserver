// TQServer Live Reload - Development Mode Only
(function() {
    'use strict';
    
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = protocol + '//' + window.location.host + '/ws/reload';
    
    let ws;
    let reconnectInterval = 1000;
    let reconnectTimeout;
    
    function connect() {
        console.log('[TQServer] Connecting to live reload...');
        
        ws = new WebSocket(wsUrl);
        
        ws.onopen = function() {
            console.log('[TQServer] Live reload connected');
            reconnectInterval = 1000; // Reset reconnect interval
        };
        
        ws.onmessage = function(event) {
            console.log('[TQServer] Reload signal received, reloading page...');
            location.reload();
        };
        
        ws.onclose = function() {
            console.log('[TQServer] Live reload disconnected, reconnecting...');
            scheduleReconnect();
        };
        
        ws.onerror = function(error) {
            console.log('[TQServer] WebSocket error, reconnecting...');
            ws.close();
        };
    }
    
    function scheduleReconnect() {
        clearTimeout(reconnectTimeout);
        reconnectTimeout = setTimeout(function() {
            connect();
            reconnectInterval = Math.min(reconnectInterval * 1.5, 10000); // Max 10s
        }, reconnectInterval);
    }
    
    // Start connection
    connect();
})();
