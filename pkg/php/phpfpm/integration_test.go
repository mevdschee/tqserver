package phpfpm

import (
    "context"
    "net"
    "testing"
    "time"

    "github.com/mevdschee/tqserver/pkg/fastcgi"
)

// TestEndToEndHandler verifies FastCGI -> Handler -> phpfpm.Client -> backend flow.
func TestEndToEndHandler(t *testing.T) {
    // start fake backend
    backendAddr, stopBackend := startFakeFCGIServer(t)
    defer stopBackend()

    // create client to backend
    client := NewClient(backendAddr, "tcp", 1, 2*time.Second, 2*time.Second)
    defer client.Close()

    // create handler and fastcgi server
    handler := NewHandler(client)

    srvAddr := "127.0.0.1:0"
    fcgiServer := fastcgi.NewServer(srvAddr, handler)

    // start server (listen on random port)
    ln, err := net.Listen("tcp", "127.0.0.1:0")
    if err != nil {
        t.Fatalf("listen: %v", err)
    }
    actualAddr := ln.Addr().String()
    go func() {
        _ = fcgiServer.Serve(ln)
    }()

    // dial into the fastcgi server as a client
    conn, err := net.DialTimeout("tcp", actualAddr, 2*time.Second)
    if err != nil {
        t.Fatalf("dial fcgi server: %v", err)
    }
    fc := fastcgi.NewConn(conn, 5*time.Second, 5*time.Second)

    // send a simple request
    var reqID uint16 = 1
    if err := fc.SendBeginRequest(reqID, fastcgi.RoleResponder, false); err != nil {
        t.Fatalf("SendBeginRequest: %v", err)
    }
    if err := fc.SendParams(reqID, map[string]string{"SCRIPT_FILENAME": "index.php"}); err != nil {
        t.Fatalf("SendParams: %v", err)
    }
    if err := fc.SendParams(reqID, nil); err != nil {
        t.Fatalf("SendParams(end): %v", err)
    }
    if err := fc.SendStdin(reqID, nil); err != nil {
        t.Fatalf("SendStdin(end): %v", err)
    }

    // collect response
    var outBuf []byte
    for {
        rec, err := fc.ReadRecord()
        if err != nil {
            t.Fatalf("ReadRecord: %v", err)
        }
        switch rec.Header.Type {
        case fastcgi.TypeStdout:
            outBuf = append(outBuf, rec.Content...)
        case fastcgi.TypeStderr:
            t.Fatalf("unexpected stderr: %s", string(rec.Content))
        case fastcgi.TypeEndRequest:
            // done
            if string(outBuf) != "hello from php-fpm" {
                t.Fatalf("unexpected stdout: %q", string(outBuf))
            }
            // shutdown server
            ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
            defer cancel()
            if err := fcgiServer.Shutdown(ctx); err != nil {
                t.Fatalf("shutdown: %v", err)
            }
            return
        }
    }
}
