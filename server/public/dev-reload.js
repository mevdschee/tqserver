// TQServer Live Reload - Development Mode Only
(function() {
    'use strict';
    
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = protocol + '//' + window.location.host + '/ws/reload';
    
    let ws;
    let reconnectInterval = 1000;
    let reconnectTimeout;
    let isReloading = false;
    
    function connect() {
        console.log('[TQServer] Connecting to live reload...');
        
        ws = new WebSocket(wsUrl);
        
        ws.onopen = function() {
            console.log('[TQServer] Live reload connected');
            reconnectInterval = 1000; // Reset reconnect interval
        };
        
        ws.onmessage = function(event) {
            console.log('[TQServer] Reload signal received, reloading page...');
            isReloading = true;
            // Close WebSocket cleanly before reload
            if (ws && ws.readyState === WebSocket.OPEN) {
                ws.close();
            }
            location.reload();
        };
        
        ws.onclose = function() {
            // Don't reconnect if we're reloading
            if (isReloading) {
                return;
            }
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
    
    // Close WebSocket when navigating away
    window.addEventListener('beforeunload', function() {
        isReloading = true;
        if (ws && ws.readyState === WebSocket.OPEN) {
            ws.close();
        }
    });
    
    // Start connection
    connect();
})();
