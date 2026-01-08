package fastcgi

import (
	"log"
	"net"
	"testing"
	"time"
)

// TestTCPSocketBehavior tests ReadRequest with actual TCP socket to reproduce the hang
func TestTCPSocketBehavior(t *testing.T) {
	// Start a TCP listener
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	addr := listener.Addr().String()
	log.Printf("Test listener on %s", addr)

	// Accept connection in background
	serverConnChan := make(chan net.Conn, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Accept error: %v", err)
			return
		}
		serverConnChan <- conn
	}()

	// Connect as client
	clientConn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer clientConn.Close()

	// Wait for server to accept
	serverConn := <-serverConnChan
	defer serverConn.Close()

	log.Printf("Client and server connected")

	// Create FastCGI connections
	serverFCGI := NewConn(serverConn, 5*time.Second, 5*time.Second)
	clientFCGI := NewConn(clientConn, 5*time.Second, 5*time.Second)

	// Send request from client
	requestID := uint16(1)
	params := map[string]string{
		"REQUEST_METHOD":  "GET",
		"SCRIPT_FILENAME": "/test.php",
	}

	go func() {
		log.Printf("Client: Sending BeginRequest")
		if err := clientFCGI.SendBeginRequest(requestID, RoleResponder, false); err != nil {
			log.Printf("SendBeginRequest error: %v", err)
			return
		}

		log.Printf("Client: Sending Params")
		if err := clientFCGI.SendParams(requestID, params); err != nil {
			log.Printf("SendParams error: %v", err)
			return
		}

		log.Printf("Client: Sending empty Params")
		if err := clientFCGI.SendParams(requestID, nil); err != nil {
			log.Printf("SendParams (empty) error: %v", err)
			return
		}

		log.Printf("Client: Sending empty Stdin")
		if err := clientFCGI.SendStdin(requestID, nil); err != nil {
			log.Printf("SendStdin error: %v", err)
			return
		}

		log.Printf("Client: All data sent successfully")
	}()

	// Try to read on server
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

	select {
	case req := <-requestRead:
		log.Printf("✅ SUCCESS: RequestID=%d, Params=%v", req.RequestID, req.Params)
	case err := <-errorRead:
		t.Fatalf("❌ FAILED: ReadRequest error: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("❌ FAILED: Timeout - This proves the bug! ReadRequest() hangs with TCP sockets")
	}
}

// TestDiagnoseReadRequestProblem shows exactly what's wrong with the current implementation
func TestDiagnoseReadRequestProblem(t *testing.T) {
	t.Log("DIAGNOSIS OF THE PROBLEM:")
	t.Log("1. Client sends 4 FastCGI records (BeginRequest, Params, empty Params, empty Stdin)")
	t.Log("2. These might all arrive in a single TCP packet (or a few packets)")
	t.Log("3. ReadRequest() calls Read() once, getting all the data")
	t.Log("4. It decodes ONE record (BeginRequest)")
	t.Log("5. It loops back and calls Read() AGAIN")
	t.Log("6. Read() BLOCKS because:")
	t.Log("   - All data was already read in step 3")
	t.Log("   - The remaining records were in the SAME buffer but discarded")
	t.Log("   - Client has finished sending, so Read() waits indefinitely")
	t.Log("")
	t.Log("THE FIX:")
	t.Log("- Use a buffered reader (bufio.Reader) to accumulate data")
	t.Log("- Parse ALL available records from the buffer before calling Read() again")
	t.Log("- Track how many bytes were consumed after decoding each record")
	t.Log("- Only call Read() when buffer is exhausted")
}
