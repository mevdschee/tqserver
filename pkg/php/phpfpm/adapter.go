package phpfpm

import (
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/mevdschee/tqserver/pkg/fastcgi"
)

// Client is a simple pooled FastCGI client for php-fpm.
type Client struct {
	addr        string
	transport   string // "tcp" or "unix"
	pool        chan net.Conn
	dialTimeout time.Duration
	rwTimeout   time.Duration
	mu          sync.Mutex
}

// NewClient constructs a new Client. poolSize==0 disables pooling.
func NewClient(listen, transport string, poolSize int, dialTimeout, rwTimeout time.Duration) *Client {
	var pool chan net.Conn
	if poolSize > 0 {
		pool = make(chan net.Conn, poolSize)
	}
	if transport == "" {
		if strings.Contains(listen, "/") {
			transport = "unix"
		} else {
			transport = "tcp"
		}
	}
	return &Client{
		addr:        listen,
		transport:   transport,
		pool:        pool,
		dialTimeout: dialTimeout,
		rwTimeout:   rwTimeout,
	}
}

// DoRequest sends a FastCGI request with params and stdin, returning stdout, stderr and the end request appStatus.
func (c *Client) DoRequest(params map[string]string, stdin []byte) (stdout []byte, stderr []byte, appStatus uint32, err error) {
	conn, err := c.getConn()
	if err != nil {
		return nil, nil, 0, err
	}

	// Wrap in fastcgi.Conn
	fcgi := fastcgi.NewConn(conn, c.rwTimeout, c.rwTimeout)
	// We do not multiplex on a single connection in this simple client; use requestID=1
	var reqID uint16 = 1

	if err := fcgi.SendBeginRequest(reqID, fastcgi.RoleResponder, false); err != nil {
		c.closeConn(conn)
		return nil, nil, 0, fmt.Errorf("SendBeginRequest: %w", err)
	}

	if err := fcgi.SendParams(reqID, params); err != nil {
		c.closeConn(conn)
		return nil, nil, 0, fmt.Errorf("SendParams: %w", err)
	}
	if err := fcgi.SendParams(reqID, nil); err != nil {
		c.closeConn(conn)
		return nil, nil, 0, fmt.Errorf("SendParams(end): %w", err)
	}

	if len(stdin) > 0 {
		if err := fcgi.SendStdin(reqID, stdin); err != nil {
			c.closeConn(conn)
			return nil, nil, 0, fmt.Errorf("SendStdin: %w", err)
		}
	}
	if err := fcgi.SendStdin(reqID, nil); err != nil {
		c.closeConn(conn)
		return nil, nil, 0, fmt.Errorf("SendStdin(end): %w", err)
	}

	// Read response
	var outBuf []byte
	var errBuf []byte
	var endStatus uint32

	for {
		rec, rerr := fcgi.ReadRecord()
		if rerr != nil {
			if rerr == io.EOF || rerr == fastcgi.ErrConnClosed {
				// connection closed unexpectedly
				c.closeConn(conn)
				return outBuf, errBuf, endStatus, fmt.Errorf("connection closed: %w", rerr)
			}
			c.closeConn(conn)
			return outBuf, errBuf, endStatus, fmt.Errorf("read record: %w", rerr)
		}

		switch rec.Header.Type {
		case fastcgi.TypeStdout:
			if len(rec.Content) > 0 {
				outBuf = append(outBuf, rec.Content...)
			}
		case fastcgi.TypeStderr:
			if len(rec.Content) > 0 {
				errBuf = append(errBuf, rec.Content...)
			}
		case fastcgi.TypeEndRequest:
			if body, derr := fastcgi.DecodeEndRequestBody(rec.Content); derr == nil {
				endStatus = body.AppStatus
			} else {
				// unable to decode, return an error
				c.closeConn(conn)
				return outBuf, errBuf, endStatus, fmt.Errorf("decode end request: %w", derr)
			}
			// finished
			// Return connection to pool if pooling enabled
			c.putConn(conn)
			return outBuf, errBuf, endStatus, nil
		}
	}
}

func (c *Client) dial() (net.Conn, error) {
	if c.transport == "unix" {
		return net.DialTimeout("unix", c.addr, c.dialTimeout)
	}
	return net.DialTimeout("tcp", c.addr, c.dialTimeout)
}

func (c *Client) getConn() (net.Conn, error) {
	// try pool first
	if c.pool != nil {
		select {
		case conn := <-c.pool:
			return conn, nil
		default:
		}
	}
	// create new
	conn, err := c.dial()
	if err != nil {
		return nil, fmt.Errorf("dial %s %s: %w", c.transport, c.addr, err)
	}
	return conn, nil
}

func (c *Client) putConn(conn net.Conn) {
	if c.pool == nil || conn == nil {
		if conn != nil {
			conn.Close()
		}
		return
	}
	select {
	case c.pool <- conn:
		// returned to pool
	default:
		// pool full, close
		conn.Close()
	}
}

func (c *Client) closeConn(conn net.Conn) {
	if conn != nil {
		conn.Close()
	}
}

// Close closes all pooled connections.
func (c *Client) Close() {
	if c.pool == nil {
		return
	}
	for {
		select {
		case conn := <-c.pool:
			conn.Close()
		default:
			return
		}
	}
}
