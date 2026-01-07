package fastcgi

import (
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

var (
	ErrConnClosed = fmt.Errorf("connection closed")
)

// Conn represents a FastCGI connection
type Conn struct {
	netConn      net.Conn
	readTimeout  time.Duration
	writeTimeout time.Duration
	mu           sync.Mutex
}

// Request represents a FastCGI request
type Request struct {
	RequestID uint16
	Role      uint16
	Flags     uint8
	Params    map[string]string
	Stdin     []byte
	Data      []byte
	KeepConn  bool
}

// NewConn creates a new FastCGI connection wrapper
func NewConn(netConn net.Conn, readTimeout, writeTimeout time.Duration) *Conn {
	return &Conn{
		netConn:      netConn,
		readTimeout:  readTimeout,
		writeTimeout: writeTimeout,
	}
}

// ReadRequest reads a complete FastCGI request from the connection
func (c *Conn) ReadRequest() (*Request, error) {
	req := &Request{
		Params: make(map[string]string),
	}

	for {
		if c.readTimeout > 0 {
			c.netConn.SetReadDeadline(time.Now().Add(c.readTimeout))
		}

		record, err := DecodeRecord(c.netConn)
		if err != nil {
			if err == io.EOF {
				return nil, ErrConnClosed
			}
			return nil, fmt.Errorf("decode record: %w", err)
		}

		switch record.Header.Type {
		case BeginRequest:
			body, err := DecodeBeginRequestBody(record.ContentReader())
			if err != nil {
				return nil, fmt.Errorf("decode begin request: %w", err)
			}
			req.RequestID = record.Header.RequestID
			req.Role = body.Role
			req.Flags = body.Flags
			req.KeepConn = body.KeepConn()

		case Params:
			if len(record.Content) > 0 {
				params, err := DecodeParams(record.Content)
				if err != nil {
					return nil, fmt.Errorf("decode params: %w", err)
				}
				// Merge params
				for k, v := range params {
					req.Params[k] = v
				}
			}
			// Empty params record signals end of params

		case Stdin:
			if len(record.Content) > 0 {
				req.Stdin = append(req.Stdin, record.Content...)
			} else {
				// Empty stdin signals end of request
				return req, nil
			}

		case Data:
			if len(record.Content) > 0 {
				req.Data = append(req.Data, record.Content...)
			}

		case AbortRequest:
			return nil, fmt.Errorf("request aborted")

		default:
			// Unknown record type, send unknown type record
			c.SendUnknownType(record.Header.Type)
		}
	}
}

// SendStdout sends stdout data to the client
func (c *Conn) SendStdout(requestID uint16, data []byte) error {
	return c.sendStream(Stdout, requestID, data)
}

// SendStderr sends stderr data to the client
func (c *Conn) SendStderr(requestID uint16, data []byte) error {
	return c.sendStream(Stderr, requestID, data)
}

// sendStream sends a stream of data, splitting into multiple records if needed
func (c *Conn) sendStream(streamType uint8, requestID uint16, data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.writeTimeout > 0 {
		c.netConn.SetWriteDeadline(time.Now().Add(c.writeTimeout))
	}

	// Split into chunks if necessary
	const maxChunkSize = 65535
	for len(data) > 0 {
		chunkSize := len(data)
		if chunkSize > maxChunkSize {
			chunkSize = maxChunkSize
		}

		record := NewRecord(streamType, requestID, data[:chunkSize])
		if err := record.Encode(c.netConn); err != nil {
			return fmt.Errorf("encode record: %w", err)
		}

		data = data[chunkSize:]
	}

	// Send empty record to signal end of stream
	record := NewRecord(streamType, requestID, nil)
	if err := record.Encode(c.netConn); err != nil {
		return fmt.Errorf("encode end record: %w", err)
	}

	return nil
}

// SendEndRequest sends an end request record
func (c *Conn) SendEndRequest(requestID uint16, appStatus uint32, protocolStatus uint8) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.writeTimeout > 0 {
		c.netConn.SetWriteDeadline(time.Now().Add(c.writeTimeout))
	}

	body := &EndRequestBody{
		AppStatus:      appStatus,
		ProtocolStatus: protocolStatus,
	}

	var buf [8]byte
	if err := body.Encode(&fixedWriter{buf[:]}); err != nil {
		return fmt.Errorf("encode end request body: %w", err)
	}

	record := NewRecord(EndRequest, requestID, buf[:])
	if err := record.Encode(c.netConn); err != nil {
		return fmt.Errorf("encode record: %w", err)
	}

	return nil
}

// SendUnknownType sends an unknown type record
func (c *Conn) SendUnknownType(unknownType uint8) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.writeTimeout > 0 {
		c.netConn.SetWriteDeadline(time.Now().Add(c.writeTimeout))
	}

	content := []byte{unknownType, 0, 0, 0, 0, 0, 0, 0}
	record := NewRecord(UnknownType, NullRequestID, content)

	if err := record.Encode(c.netConn); err != nil {
		return fmt.Errorf("encode record: %w", err)
	}

	return nil
}

// Close closes the connection
func (c *Conn) Close() error {
	return c.netConn.Close()
}

// fixedWriter wraps a byte slice to implement io.Writer
type fixedWriter struct {
	buf []byte
	pos int
}

func (w *fixedWriter) Write(p []byte) (n int, err error) {
	n = copy(w.buf[w.pos:], p)
	w.pos += n
	if n < len(p) {
		return n, io.ErrShortWrite
	}
	return n, nil
}

// ContentReader returns an io.Reader for the record content
func (r *Record) ContentReader() io.Reader {
	return &contentReader{content: r.Content}
}

type contentReader struct {
	content []byte
	pos     int
}

func (r *contentReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.content) {
		return 0, io.EOF
	}
	n = copy(p, r.content[r.pos:])
	r.pos += n
	return n, nil
}
