package fastcgi

import (
	"bytes"
	"context"
	"io"
	"log"
	"net"
	"testing"
	"time"
)

// TestClientServerCommunication tests the full client-server communication
// This reproduces the issue where ReadRequest() hangs
func TestClientServerCommunication(t *testing.T) {
	// Create a simple handler
	handler := HandlerFunc(func(conn *Conn, req *Request) error {
		log.Printf("Handler called with RequestID: %d", req.RequestID)

		// Send a simple response
		response := []byte("Hello from handler")
		if err := conn.SendStdout(req.RequestID, response); err != nil {
			return err
		}

		// Send empty stdout to signal end
		if err := conn.SendStdout(req.RequestID, nil); err != nil {
			return err
		}

		// Send end request
		return conn.SendEndRequest(req.RequestID, 0, uint8(StatusRequestComplete))
	})

	// Create server
	server := NewServer("127.0.0.1:19001", handler)

	// Start server in background
	errChan := make(chan error, 1)
	go func() {
		errChan <- server.ListenAndServe()
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Connect as a client (simulating proxy.go)
	conn, err := net.DialTimeout("tcp", "127.0.0.1:19001", 5*time.Second)
	if err != nil {
		t.Fatalf("Failed to connect to server: %v", err)
	}
	defer conn.Close()

	log.Printf("Client connected to server")

	// Create FastCGI connection
	fcgiConn := NewConn(conn, 5*time.Second, 5*time.Second)

	// Build request parameters (simulating what proxy.go does)
	params := make(map[string]string)
	params["GATEWAY_INTERFACE"] = "CGI/1.1"
	params["SERVER_SOFTWARE"] = "TQServer"
	params["SERVER_PROTOCOL"] = "HTTP/1.1"
	params["REQUEST_METHOD"] = "GET"
	params["REQUEST_URI"] = "/test.php"
	params["SCRIPT_FILENAME"] = "/tmp/test.php"
	params["REDIRECT_STATUS"] = "200"

	requestID := uint16(1)

	log.Printf("Client sending BeginRequest")
	// Send BeginRequest
	if err := fcgiConn.SendBeginRequest(requestID, RoleResponder, false); err != nil {
		t.Fatalf("Failed to send BeginRequest: %v", err)
	}

	log.Printf("Client sending Params")
	// Send Params
	if err := fcgiConn.SendParams(requestID, params); err != nil {
		t.Fatalf("Failed to send Params: %v", err)
	}

	log.Printf("Client sending empty Params")
	// Send empty params to signal end
	if err := fcgiConn.SendParams(requestID, nil); err != nil {
		t.Fatalf("Failed to send empty Params: %v", err)
	}

	log.Printf("Client sending Stdin")
	// Send Stdin (empty)
	if err := fcgiConn.SendStdin(requestID, nil); err != nil {
		t.Fatalf("Failed to send Stdin: %v", err)
	}

	log.Printf("Client waiting for response...")

	// Read response with timeout
	responseChan := make(chan []byte, 1)
	errorChan := make(chan error, 1)

	go func() {
		var stdout bytes.Buffer
		for {
			record, err := fcgiConn.ReadRecord()
			if err != nil {
				if err == io.EOF || err == ErrConnClosed {
					errorChan <- nil
					return
				}
				errorChan <- err
				return
			}

			log.Printf("Client received record type: %d, length: %d", record.Header.Type, record.Header.ContentLength)

			switch record.Header.Type {
			case TypeStdout:
				if len(record.Content) > 0 {
					stdout.Write(record.Content)
				}
			case TypeStderr:
				if len(record.Content) > 0 {
					log.Printf("Stderr: %s", string(record.Content))
				}
			case TypeEndRequest:
				responseChan <- stdout.Bytes()
				return
			}
		}
	}()

	// Wait for response or timeout
	select {
	case response := <-responseChan:
		log.Printf("Client received response: %s", string(response))
		if string(response) != "Hello from handler" {
			t.Errorf("Unexpected response: got %q, want %q", string(response), "Hello from handler")
		}
	case err := <-errorChan:
		if err != nil {
			t.Fatalf("Error reading response: %v", err)
		}
		t.Fatal("Connection closed without response")
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for response - ReadRequest() likely hanging")
	}

	// Shutdown server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown failed: %v", err)
	}
}

// TestReadRequestWithRealData tests ReadRequest() with actual FastCGI protocol data
func TestReadRequestWithRealData(t *testing.T) {
	// Create a pipe to simulate network connection
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	// Create server-side connection
	serverFCGI := NewConn(serverConn, 5*time.Second, 5*time.Second)

	// Create client-side connection
	clientFCGI := NewConn(clientConn, 5*time.Second, 5*time.Second)

	// Send a complete FastCGI request from client
	requestID := uint16(1)
	params := map[string]string{
		"REQUEST_METHOD":  "GET",
		"SCRIPT_FILENAME": "/test.php",
	}

	requestSent := make(chan error, 1)
	go func() {
		log.Printf("Client: Sending BeginRequest")
		if err := clientFCGI.SendBeginRequest(requestID, RoleResponder, false); err != nil {
			requestSent <- err
			return
		}

		log.Printf("Client: Sending Params")
		if err := clientFCGI.SendParams(requestID, params); err != nil {
			requestSent <- err
			return
		}

		log.Printf("Client: Sending empty Params")
		if err := clientFCGI.SendParams(requestID, nil); err != nil {
			requestSent <- err
			return
		}

		log.Printf("Client: Sending empty Stdin")
		if err := clientFCGI.SendStdin(requestID, nil); err != nil {
			requestSent <- err
			return
		}

		log.Printf("Client: All data sent")
		requestSent <- nil
	}()

	// Try to read the request on server side
	requestRead := make(chan *Request, 1)
	errorRead := make(chan error, 1)

	go func() {
		log.Printf("Server: Calling ReadRequest()")
		req, err := serverFCGI.ReadRequest()
		if err != nil {
			log.Printf("Server: ReadRequest error: %v", err)
			errorRead <- err
			return
		}
		log.Printf("Server: ReadRequest completed successfully")
		requestRead <- req
	}()

	// Wait for request to be sent
	select {
	case err := <-requestSent:
		if err != nil {
			t.Fatalf("Failed to send request: %v", err)
		}
		log.Printf("Request sent successfully")
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout sending request")
	}

	// Wait for request to be read
	select {
	case req := <-requestRead:
		log.Printf("Request read successfully: RequestID=%d, Role=%d", req.RequestID, req.Role)
		if req.RequestID != requestID {
			t.Errorf("RequestID = %d, want %d", req.RequestID, requestID)
		}
		if req.Role != RoleResponder {
			t.Errorf("Role = %d, want %d", req.Role, RoleResponder)
		}
		if len(req.Params) != len(params) {
			t.Errorf("Params count = %d, want %d", len(req.Params), len(params))
		}
	case err := <-errorRead:
		t.Fatalf("Failed to read request: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout reading request - ReadRequest() is hanging")
	}
}

// TestParamEncoding tests if params are encoded/decoded correctly
func TestParamEncoding(t *testing.T) {
	params := map[string]string{
		"SCRIPT_FILENAME": "/home/maurits/projects/tqserver/workers/blog/public/hello.php",
		"REQUEST_METHOD":  "GET",
		"REDIRECT_STATUS": "200",
	}

	// Encode params
	encoded := EncodeParams(params)
	log.Printf("Encoded params length: %d bytes", len(encoded))

	// Decode params
	decoded, err := DecodeParams(encoded)
	if err != nil {
		t.Fatalf("Failed to decode params: %v", err)
	}

	// Verify all params match
	if len(decoded) != len(params) {
		t.Errorf("Decoded params count = %d, want %d", len(decoded), len(params))
	}

	for k, v := range params {
		if decoded[k] != v {
			t.Errorf("Param %s = %q, want %q", k, decoded[k], v)
		}
	}
}
