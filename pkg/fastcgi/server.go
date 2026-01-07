package fastcgi

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

// Server represents a FastCGI server
type Server struct {
	Addr         string
	Handler      Handler
	MaxConns     int
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	listener     net.Listener
	mu           sync.RWMutex
	activeConns  map[net.Conn]struct{}
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
}

// Handler processes FastCGI requests
type Handler interface {
	ServeFastCGI(conn *Conn, req *Request) error
}

// HandlerFunc is an adapter to allow ordinary functions to be used as handlers
type HandlerFunc func(conn *Conn, req *Request) error

func (f HandlerFunc) ServeFastCGI(conn *Conn, req *Request) error {
	return f(conn, req)
}

// NewServer creates a new FastCGI server
func NewServer(addr string, handler Handler) *Server {
	ctx, cancel := context.WithCancel(context.Background())
	return &Server{
		Addr:         addr,
		Handler:      handler,
		MaxConns:     1000,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 60 * time.Second,
		activeConns:  make(map[net.Conn]struct{}),
		ctx:          ctx,
		cancel:       cancel,
	}
}

// ListenAndServe listens on the TCP network address and serves FastCGI requests
func (s *Server) ListenAndServe() error {
	listener, err := net.Listen("tcp", s.Addr)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	s.listener = listener
	log.Printf("FastCGI server listening on %s", s.Addr)

	return s.Serve(listener)
}

// ListenAndServeUnix listens on a Unix socket and serves FastCGI requests
func (s *Server) ListenAndServeUnix(socketPath string) error {
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return fmt.Errorf("failed to listen on unix socket: %w", err)
	}

	s.listener = listener
	log.Printf("FastCGI server listening on unix:%s", socketPath)

	return s.Serve(listener)
}

// Serve accepts incoming connections on the listener
func (s *Server) Serve(listener net.Listener) error {
	defer listener.Close()

	for {
		select {
		case <-s.ctx.Done():
			return s.ctx.Err()
		default:
		}

		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-s.ctx.Done():
				return nil
			default:
				log.Printf("Accept error: %v", err)
				continue
			}
		}

		// Track active connection
		s.mu.Lock()
		if len(s.activeConns) >= s.MaxConns {
			s.mu.Unlock()
			conn.Close()
			log.Printf("Max connections reached, rejecting connection")
			continue
		}
		s.activeConns[conn] = struct{}{}
		s.mu.Unlock()

		s.wg.Add(1)
		go s.handleConnection(conn)
	}
}

// handleConnection handles a single FastCGI connection
func (s *Server) handleConnection(netConn net.Conn) {
	defer s.wg.Done()
	defer func() {
		s.mu.Lock()
		delete(s.activeConns, netConn)
		s.mu.Unlock()
		netConn.Close()
	}()

	conn := NewConn(netConn, s.ReadTimeout, s.WriteTimeout)

	// Handle multiple requests on this connection
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		// Read request
		req, err := conn.ReadRequest()
		if err != nil {
			if err != ErrConnClosed {
				log.Printf("Read request error: %v", err)
			}
			return
		}

		// Handle request
		if err := s.Handler.ServeFastCGI(conn, req); err != nil {
			log.Printf("Handler error: %v", err)
			conn.SendEndRequest(req.RequestID, 1, RequestComplete)
			return
		}

		// Check if we should keep the connection open
		if !req.KeepConn {
			return
		}
	}
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	s.cancel()

	if s.listener != nil {
		s.listener.Close()
	}

	// Wait for active connections to finish or timeout
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		// Force close remaining connections
		s.mu.Lock()
		for conn := range s.activeConns {
			conn.Close()
		}
		s.mu.Unlock()
		return ctx.Err()
	}
}
