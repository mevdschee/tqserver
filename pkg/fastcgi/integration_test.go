package fastcgi

import (
	"fmt"
	"net"
	"testing"
	"time"
)

// Simple echo handler for testing
type EchoHandler struct{}

func (h *EchoHandler) ServeFastCGI(conn *Conn, req *Request) error {
	// Echo back the request parameters
	response := fmt.Sprintf("Request ID: %d\nRole: %d\n", req.RequestID, req.Role)
	response += fmt.Sprintf("Parameters:\n")
	for k, v := range req.Params {
		response += fmt.Sprintf("  %s = %s\n", k, v)
	}
	if len(req.Stdin) > 0 {
		response += fmt.Sprintf("Stdin: %s\n", string(req.Stdin))
	}

	// Send response
	if err := conn.SendStdout(req.RequestID, []byte(response)); err != nil {
		return err
	}

	// Send end request
	return conn.SendEndRequest(req.RequestID, 0, uint8(StatusRequestComplete))
}

// TestServerBasic tests basic server functionality
func TestServerBasic(t *testing.T) {
	// Create server
	handler := &EchoHandler{}
	// Start a simple TCP listener and serve using handler for the test.
	ln, err := net.Listen("tcp", "127.0.0.1:19000")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	// Serve in background
	serveDone := make(chan error, 1)
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				serveDone <- err
				return
			}
			go func(c net.Conn) {
				fc := NewConn(c, 60*time.Second, 60*time.Second)
				for {
					req, err := fc.ReadRequest()
					if err != nil {
						c.Close()
						return
					}
					_ = handler.ServeFastCGI(fc, req)
					if !req.KeepConn {
						c.Close()
						return
					}
				}
			}(conn)
		}
	}()

	// Give listener time to start
	time.Sleep(100 * time.Millisecond)

	// Shutdown listener
	ln.Close()
	select {
	case <-serveDone:
	case <-time.After(1 * time.Second):
	}
}
