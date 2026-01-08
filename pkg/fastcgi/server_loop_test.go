package fastcgi

import (
	"log"
	"net"
	"testing"
	"time"
)

// TestServerConnectionLoop tests the server's connection handling loop
func TestServerConnectionLoop(t *testing.T) {
	// Recreate the exact server scenario
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	addr := listener.Addr().String()

	// Server goroutine that mimics server.go handleConnection()
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		log.Printf("Server: Connection accepted")
		fcgiConn := NewConn(conn, 5*time.Second, 5*time.Second)

		// This is what server.go does:
		log.Printf("Server: About to call ReadRequest()")
		req, err := fcgiConn.ReadRequest()
		if err != nil {
			log.Printf("Server: ReadRequest failed: %v", err)
			return
		}
		log.Printf("Server: ReadRequest succeeded! RequestID=%d", req.RequestID)

		// Send response
		response := []byte("Test response")
		fcgiConn.SendStdout(req.RequestID, response)
		fcgiConn.SendStdout(req.RequestID, nil) // Empty to signal end
		fcgiConn.SendEndRequest(req.RequestID, 0, uint8(StatusRequestComplete))
		log.Printf("Server: Response sent")
	}()

	// Give server time to start
	time.Sleep(50 * time.Millisecond)

	// Client connects and sends request
	clientConn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		t.Fatalf("Client: Failed to connect: %v", err)
	}
	defer clientConn.Close()

	clientFCGI := NewConn(clientConn, 5*time.Second, 5*time.Second)

	requestID := uint16(1)
	params := map[string]string{
		"SCRIPT_FILENAME": "/test.php",
		"REQUEST_METHOD":  "GET",
	}

	log.Printf("Client: Sending request...")
	if err := clientFCGI.SendBeginRequest(requestID, RoleResponder, false); err != nil {
		t.Fatalf("SendBeginRequest: %v", err)
	}

	if err := clientFCGI.SendParams(requestID, params); err != nil {
		t.Fatalf("SendParams: %v", err)
	}

	if err := clientFCGI.SendParams(requestID, nil); err != nil {
		t.Fatalf("SendParams (empty): %v", err)
	}

	if err := clientFCGI.SendStdin(requestID, nil); err != nil {
		t.Fatalf("SendStdin: %v", err)
	}

	log.Printf("Client: Request sent, waiting for response...")

	// Read response with timeout
	responseChan := make(chan bool, 1)
	go func() {
		for {
			record, err := clientFCGI.ReadRecord()
			if err != nil {
				log.Printf("Client: ReadRecord error: %v", err)
				return
			}

			log.Printf("Client: Received record type %d", record.Header.Type)

			if record.Header.Type == TypeEndRequest {
				log.Printf("Client: Got EndRequest")
				responseChan <- true
				return
			}
		}
	}()

	select {
	case <-responseChan:
		log.Printf("✅ Test passed!")
	case <-time.After(3 * time.Second):
		t.Fatal("❌ Timeout waiting for response")
	}
}
