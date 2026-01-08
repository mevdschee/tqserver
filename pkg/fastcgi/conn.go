package fastcgi

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

var (
	ErrConnClosed        = fmt.Errorf("connection closed")
	NullRequestID uint16 = 0
)

// Conn represents a FastCGI connection
type Conn struct {
	netConn      net.Conn
	reader       *bufio.Reader
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
		reader:       bufio.NewReader(netConn),
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

		// Peek at the header to determine record size
		headerBytes, err := c.reader.Peek(HeaderSize)
		if err != nil {
			if err == io.EOF {
				return nil, ErrConnClosed
			}
			return nil, fmt.Errorf("peek header: %w", err)
		}

		header, err := DecodeHeader(headerBytes)
		if err != nil {
			return nil, fmt.Errorf("decode header: %w", err)
		}

		// Calculate total record size
		totalSize := HeaderSize + int(header.ContentLength) + int(header.PaddingLength)

		// Read the complete record
		recordBytes := make([]byte, totalSize)
		if _, err := io.ReadFull(c.reader, recordBytes); err != nil {
			if err == io.EOF {
				return nil, ErrConnClosed
			}
			return nil, fmt.Errorf("read record: %w", err)
		}

		// Decode the record
		record, _, err := DecodeRecord(recordBytes)
		if err != nil {
			return nil, fmt.Errorf("decode record: %w", err)
		}

		switch record.Header.Type {
		case TypeBeginRequest:
			body, err := DecodeBeginRequestBody(record.Content)
			if err != nil {
				return nil, fmt.Errorf("decode begin request: %w", err)
			}
			req.RequestID = record.Header.RequestID
			req.Role = body.Role
			req.Flags = body.Flags
			req.KeepConn = (body.Flags & FlagKeepConn) != 0

		case TypeParams:
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

		case TypeStdin:
			if len(record.Content) > 0 {
				req.Stdin = append(req.Stdin, record.Content...)
			} else {
				// Empty stdin signals end of request
				return req, nil
			}

		case TypeData:
			if len(record.Content) > 0 {
				req.Data = append(req.Data, record.Content...)
			}

		case TypeAbortRequest:
			return nil, fmt.Errorf("request aborted")

		default:
			// Unknown record type, send unknown type record
			c.SendUnknownType(record.Header.Type)
		}
	}
}

// SendStdout sends stdout data to the client
func (c *Conn) SendStdout(requestID uint16, data []byte) error {
	return c.sendStream(TypeStdout, requestID, data)
}

// SendStderr sends stderr data to the client
func (c *Conn) SendStderr(requestID uint16, data []byte) error {
	return c.sendStream(TypeStderr, requestID, data)
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
		encoded := record.Encode()
		if _, err := c.netConn.Write(encoded); err != nil {
			return fmt.Errorf("write record: %w", err)
		}

		data = data[chunkSize:]
	}

	// Send empty record to signal end of stream
	record := NewRecord(streamType, requestID, nil)
	encoded := record.Encode()
	if _, err := c.netConn.Write(encoded); err != nil {
		return fmt.Errorf("write end record: %w", err)
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
		ProtocolStatus: uint8(protocolStatus),
	}

	content := body.Encode()
	record := NewRecord(TypeEndRequest, requestID, content)
	encoded := record.Encode()

	if _, err := c.netConn.Write(encoded); err != nil {
		return fmt.Errorf("write record: %w", err)
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
	record := NewRecord(TypeUnknownType, NullRequestID, content)
	encoded := record.Encode()

	if _, err := c.netConn.Write(encoded); err != nil {
		return fmt.Errorf("write record: %w", err)
	}

	return nil
}

// SendBeginRequest sends a begin request record
func (c *Conn) SendBeginRequest(requestID uint16, role uint16, keepConn bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.writeTimeout > 0 {
		c.netConn.SetWriteDeadline(time.Now().Add(c.writeTimeout))
	}

	flags := uint8(0)
	if keepConn {
		flags = FlagKeepConn
	}

	body := &BeginRequestBody{
		Role:  role,
		Flags: flags,
	}

	content := body.Encode()
	record := NewRecord(TypeBeginRequest, requestID, content)
	encoded := record.Encode()

	if _, err := c.netConn.Write(encoded); err != nil {
		return fmt.Errorf("write begin request: %w", err)
	}

	return nil
}

// SendParams sends parameters to the FastCGI application
func (c *Conn) SendParams(requestID uint16, params map[string]string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.writeTimeout > 0 {
		c.netConn.SetWriteDeadline(time.Now().Add(c.writeTimeout))
	}

	var content []byte
	if params != nil && len(params) > 0 {
		content = EncodeParams(params)
	}

	record := NewRecord(TypeParams, requestID, content)
	encoded := record.Encode()

	if _, err := c.netConn.Write(encoded); err != nil {
		return fmt.Errorf("write params: %w", err)
	}

	return nil
}

// SendStdin sends stdin data to the FastCGI application
func (c *Conn) SendStdin(requestID uint16, data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.writeTimeout > 0 {
		c.netConn.SetWriteDeadline(time.Now().Add(c.writeTimeout))
	}

	record := NewRecord(TypeStdin, requestID, data)
	encoded := record.Encode()

	if _, err := c.netConn.Write(encoded); err != nil {
		return fmt.Errorf("write stdin: %w", err)
	}

	return nil
}

// ReadRecord reads a single FastCGI record from the connection
func (c *Conn) ReadRecord() (*Record, error) {
	if c.readTimeout > 0 {
		c.netConn.SetReadDeadline(time.Now().Add(c.readTimeout))
	}

	// Peek at the header to determine record size
	headerBytes, err := c.reader.Peek(HeaderSize)
	if err != nil {
		if err == io.EOF {
			return nil, ErrConnClosed
		}
		return nil, fmt.Errorf("peek header: %w", err)
	}

	header, err := DecodeHeader(headerBytes)
	if err != nil {
		return nil, fmt.Errorf("decode header: %w", err)
	}

	// Calculate total record size
	totalSize := HeaderSize + int(header.ContentLength) + int(header.PaddingLength)

	// Read the complete record
	recordBytes := make([]byte, totalSize)
	if _, err := io.ReadFull(c.reader, recordBytes); err != nil {
		if err == io.EOF {
			return nil, ErrConnClosed
		}
		return nil, fmt.Errorf("read record: %w", err)
	}

	// Decode the record
	record, _, err := DecodeRecord(recordBytes)
	if err != nil {
		return nil, fmt.Errorf("decode record: %w", err)
	}

	return record, nil
}

// Close closes the connection
func (c *Conn) Close() error {
	return c.netConn.Close()
}

// ContentReader returns an io.Reader for the record content
func (r *Record) ContentReader() io.Reader {
	return bytes.NewReader(r.Content)
}
