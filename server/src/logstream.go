package main

import (
	"fmt"
	"io"
	"sync"
	"time"
)

// LogBroadcaster captures log output and broadcasts to SSE clients
type LogBroadcaster struct {
	clients   map[chan string]struct{}
	mu        sync.RWMutex
	buffer    []string
	bufferMax int
	original  io.Writer
}

// NewLogBroadcaster creates a new log broadcaster
func NewLogBroadcaster(original io.Writer, bufferSize int) *LogBroadcaster {
	return &LogBroadcaster{
		clients:   make(map[chan string]struct{}),
		buffer:    make([]string, 0, bufferSize),
		bufferMax: bufferSize,
		original:  original,
	}
}

// Write implements io.Writer - logs to original output and broadcasts to clients
func (lb *LogBroadcaster) Write(p []byte) (n int, err error) {
	// Write to original output first
	if lb.original != nil {
		n, err = lb.original.Write(p)
	} else {
		n = len(p)
	}

	// Format log line with timestamp
	line := string(p)
	if len(line) > 0 && line[len(line)-1] == '\n' {
		line = line[:len(line)-1] // Remove trailing newline
	}

	// Add to buffer
	lb.mu.Lock()
	lb.buffer = append(lb.buffer, line)
	if len(lb.buffer) > lb.bufferMax {
		lb.buffer = lb.buffer[1:] // Remove oldest
	}
	clients := make([]chan string, 0, len(lb.clients))
	for ch := range lb.clients {
		clients = append(clients, ch)
	}
	lb.mu.Unlock()

	// Broadcast to all clients (non-blocking)
	for _, ch := range clients {
		select {
		case ch <- line:
		default:
			// Client is slow, skip this message
		}
	}

	return n, err
}

// Subscribe adds a new SSE client
func (lb *LogBroadcaster) Subscribe() chan string {
	ch := make(chan string, 100) // Buffer to prevent blocking
	lb.mu.Lock()
	lb.clients[ch] = struct{}{}
	lb.mu.Unlock()
	return ch
}

// Unsubscribe removes an SSE client
func (lb *LogBroadcaster) Unsubscribe(ch chan string) {
	lb.mu.Lock()
	delete(lb.clients, ch)
	lb.mu.Unlock()
	close(ch)
}

// GetRecentLogs returns recent log lines from the buffer
func (lb *LogBroadcaster) GetRecentLogs() []string {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	// Return a copy
	result := make([]string, len(lb.buffer))
	copy(result, lb.buffer)
	return result
}

// ClientCount returns number of connected clients
func (lb *LogBroadcaster) ClientCount() int {
	lb.mu.RLock()
	defer lb.mu.RUnlock()
	return len(lb.clients)
}

// Global log broadcaster instance
var logBroadcaster *LogBroadcaster

// GetLogBroadcaster returns the global log broadcaster
func GetLogBroadcaster() *LogBroadcaster {
	return logBroadcaster
}

// InitLogBroadcaster initializes the global log broadcaster
func InitLogBroadcaster(original io.Writer, bufferSize int) *LogBroadcaster {
	logBroadcaster = NewLogBroadcaster(original, bufferSize)

	// Log initial message
	fmt.Fprintf(logBroadcaster, "%s Log streaming initialized\n", time.Now().Format("2006/01/02 15:04:05"))

	return logBroadcaster
}
