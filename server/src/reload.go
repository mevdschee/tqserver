package main

import (
	"crypto/sha1"
	"encoding/base64"
	"log"
	"net"
	"net/http"
	"sync"
)

// wsConn wraps a net.Conn for WebSocket communication
type wsConn struct {
	conn net.Conn
}

// ReloadBroadcaster manages WebSocket connections for live reload
type ReloadBroadcaster struct {
	clients map[*wsConn]bool
	mu      sync.RWMutex
}

// NewReloadBroadcaster creates a new reload broadcaster
func NewReloadBroadcaster() *ReloadBroadcaster {
	return &ReloadBroadcaster{
		clients: make(map[*wsConn]bool),
	}
}

// HandleWebSocket handles WebSocket connections for reload notifications
func (rb *ReloadBroadcaster) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Perform WebSocket handshake
	if r.Header.Get("Upgrade") != "websocket" {
		http.Error(w, "Not a websocket handshake", http.StatusBadRequest)
		return
	}

	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Server doesn't support hijacking", http.StatusInternalServerError)
		return
	}

	conn, bufrw, err := hj.Hijack()
	if err != nil {
		log.Printf("Hijack failed: %v", err)
		return
	}

	// Complete WebSocket handshake
	key := r.Header.Get("Sec-WebSocket-Key")
	acceptKey := computeAcceptKey(key)

	response := "HTTP/1.1 101 Switching Protocols\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Accept: " + acceptKey + "\r\n\r\n"

	if _, err := bufrw.WriteString(response); err != nil {
		conn.Close()
		return
	}
	if err := bufrw.Flush(); err != nil {
		conn.Close()
		return
	}

	wsConn := &wsConn{conn: conn}

	rb.mu.Lock()
	rb.clients[wsConn] = true
	clientCount := len(rb.clients)
	rb.mu.Unlock()

	log.Printf("WebSocket client connected (total: %d)", clientCount)

	// Keep connection alive and wait for close
	defer func() {
		rb.mu.Lock()
		delete(rb.clients, wsConn)
		clientCount := len(rb.clients)
		rb.mu.Unlock()
		conn.Close()
		log.Printf("WebSocket client disconnected (total: %d)", clientCount)
	}()

	// Read messages to detect disconnect
	buf := make([]byte, 1024)
	for {
		if _, err := conn.Read(buf); err != nil {
			break
		}
	}
}

// BroadcastReload sends a reload message to all connected clients
func (rb *ReloadBroadcaster) BroadcastReload() {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	if len(rb.clients) == 0 {
		return
	}

	log.Printf("Broadcasting reload to %d client(s)", len(rb.clients))

	// Create WebSocket text frame with "reload" message
	message := []byte("reload")
	frame := makeTextFrame(message)

	for client := range rb.clients {
		if _, err := client.conn.Write(frame); err != nil {
			log.Printf("Failed to send reload message: %v", err)
			client.conn.Close()
			delete(rb.clients, client)
		}
	}
}

// computeAcceptKey computes the Sec-WebSocket-Accept key
func computeAcceptKey(key string) string {
	const magic = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"
	h := sha1.New()
	h.Write([]byte(key + magic))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// makeTextFrame creates a WebSocket text frame
func makeTextFrame(payload []byte) []byte {
	length := len(payload)
	frame := []byte{0x81} // FIN=1, opcode=1 (text)

	if length <= 125 {
		frame = append(frame, byte(length))
	} else if length <= 65535 {
		frame = append(frame, 126, byte(length>>8), byte(length))
	} else {
		frame = append(frame, 127)
		for i := 7; i >= 0; i-- {
			frame = append(frame, byte(length>>(i*8)))
		}
	}

	frame = append(frame, payload...)
	return frame
}
