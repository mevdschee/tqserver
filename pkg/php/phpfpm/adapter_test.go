package phpfpm

import (
	"net"
	"testing"
	"time"

	"github.com/mevdschee/tqserver/pkg/fastcgi"
)

// startFakeFCGIServer starts a minimal FastCGI server that reads one request and
// responds with a stdout payload and an end request record, then closes.
func startFakeFCGIServer(t *testing.T) (addr string, stop func()) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	done := make(chan struct{})

	go func() {
		defer ln.Close()
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		fc := fastcgi.NewConn(conn, 5*time.Second, 5*time.Second)
		// Read request
		req, err := fc.ReadRequest()
		if err != nil {
			conn.Close()
			close(done)
			return
		}

		// Send a small stdout body
		_ = fc.SendStdout(req.RequestID, []byte("hello from php-fpm"))
		// Send empty stdout to terminate stream
		_ = fc.SendStdout(req.RequestID, nil)
		// Send end request
		_ = fc.SendEndRequest(req.RequestID, 0, uint8(fastcgi.StatusRequestComplete))

		conn.Close()
		close(done)
	}()

	return ln.Addr().String(), func() {
		<-done
		ln.Close()
	}
}

func TestClientDoRequest(t *testing.T) {
	addr, stop := startFakeFCGIServer(t)
	defer stop()

	client := NewClient(addr, "tcp", 0, 2*time.Second, 2*time.Second)
	defer client.Close()

	params := map[string]string{"SCRIPT_FILENAME": "index.php"}
	stdout, stderr, appStatus, err := client.DoRequest(params, nil)
	if err != nil {
		t.Fatalf("DoRequest error: %v", err)
	}

	if appStatus != 0 {
		t.Fatalf("unexpected appStatus: %d", appStatus)
	}
	if len(stderr) != 0 {
		t.Fatalf("unexpected stderr: %s", string(stderr))
	}
	if string(stdout) != "hello from php-fpm" {
		t.Fatalf("unexpected stdout: %q", string(stdout))
	}
}
