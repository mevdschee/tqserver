package fastcgi

import (
	"context"
	"fmt"
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
	server := NewServer("127.0.0.1:19000", handler)

	// Start server in background
	errChan := make(chan error, 1)
	go func() {
		errChan <- server.ListenAndServe()
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// TODO: Add client connection test once we have a FastCGI client
	// For now, just verify server starts without error

	// Shutdown server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown failed: %v", err)
	}

	// Check for server errors
	select {
	case err := <-errChan:
		if err != nil {
			t.Fatalf("Server error: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Server did not shutdown in time")
	}
}
