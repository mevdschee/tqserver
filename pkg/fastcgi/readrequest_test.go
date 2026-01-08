package fastcgi

import (
	"log"
	"net"
	"testing"
	"time"
)

// TestMultipleRecordsInSingleRead tests if ReadRequest can handle multiple records arriving together
func TestMultipleRecordsInSingleRead(t *testing.T) {
	// Create a pipe
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	serverFCGI := NewConn(serverConn, 5*time.Second, 5*time.Second)
	clientFCGI := NewConn(clientConn, 5*time.Second, 5*time.Second)

	requestID := uint16(1)
	params := map[string]string{
		"REQUEST_METHOD":  "GET",
		"SCRIPT_FILENAME": "/test.php",
	}

	// Send all records quickly so they arrive together
	go func() {
		// Don't add any delays - send everything at once
		clientFCGI.SendBeginRequest(requestID, RoleResponder, false)
		clientFCGI.SendParams(requestID, params)
		clientFCGI.SendParams(requestID, nil) // Empty params
		clientFCGI.SendStdin(requestID, nil)  // Empty stdin
	}()

	// Try to read
	requestRead := make(chan *Request, 1)
	errorRead := make(chan error, 1)

	go func() {
		req, err := serverFCGI.ReadRequest()
		if err != nil {
			errorRead <- err
			return
		}
		requestRead <- req
	}()

	select {
	case req := <-requestRead:
		log.Printf("Success: RequestID=%d", req.RequestID)
	case err := <-errorRead:
		t.Fatalf("Failed to read: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout - ReadRequest can't handle multiple records in buffer")
	}
}

// TestRecordBoundaries tests if records can be read across multiple Read() calls
func TestRecordBoundaries(t *testing.T) {
	// This test simulates TCP behavior where data might arrive in chunks
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	serverFCGI := NewConn(serverConn, 5*time.Second, 5*time.Second)
	clientFCGI := NewConn(clientConn, 5*time.Second, 5*time.Second)

	requestID := uint16(1)

	// Send records with small delays to simulate TCP chunking
	go func() {
		log.Printf("Sending BeginRequest")
		clientFCGI.SendBeginRequest(requestID, RoleResponder, false)
		time.Sleep(10 * time.Millisecond)

		log.Printf("Sending Params")
		params := map[string]string{"KEY": "VALUE"}
		clientFCGI.SendParams(requestID, params)
		time.Sleep(10 * time.Millisecond)

		log.Printf("Sending empty Params")
		clientFCGI.SendParams(requestID, nil)
		time.Sleep(10 * time.Millisecond)

		log.Printf("Sending empty Stdin")
		clientFCGI.SendStdin(requestID, nil)
		log.Printf("All records sent")
	}()

	// Try to read
	requestRead := make(chan *Request, 1)
	errorRead := make(chan error, 1)

	go func() {
		log.Printf("Starting ReadRequest")
		req, err := serverFCGI.ReadRequest()
		if err != nil {
			log.Printf("ReadRequest error: %v", err)
			errorRead <- err
			return
		}
		log.Printf("ReadRequest success")
		requestRead <- req
	}()

	select {
	case req := <-requestRead:
		t.Logf("Success: RequestID=%d, Params=%v", req.RequestID, req.Params)
	case err := <-errorRead:
		t.Fatalf("Failed to read: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("Timeout - ReadRequest blocked on chunked data")
	}
}
