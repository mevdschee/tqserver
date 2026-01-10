package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// SOCKS5 protocol constants
const (
	socks5Version = 0x05

	// Authentication methods
	authNoAuth = 0x00

	// Commands
	cmdConnect = 0x01

	// Address types
	addrTypeIPv4   = 0x01
	addrTypeDomain = 0x03
	addrTypeIPv6   = 0x04

	// Reply codes
	replySuccess          = 0x00
	replyGeneralFailure   = 0x01
	replyConnNotAllowed   = 0x02
	replyNetworkUnreach   = 0x03
	replyHostUnreach      = 0x04
	replyConnRefused      = 0x05
	replyTTLExpired       = 0x06
	replyCmdNotSupported  = 0x07
	replyAddrNotSupported = 0x08
)

// ConnectionLog represents a logged connection through the SOCKS5 proxy
type ConnectionLog struct {
	Timestamp     time.Time `json:"timestamp"`
	CorrelationID string    `json:"correlation_id,omitempty"`
	WorkerName    string    `json:"worker_name,omitempty"`
	UserAgent     string    `json:"user_agent,omitempty"`
	DestHost      string    `json:"dest_host"`
	DestPort      int       `json:"dest_port"`
	Protocol      string    `json:"protocol"` // "http" | "https" | "tcp"
	Method        string    `json:"method,omitempty"`
	Path          string    `json:"path,omitempty"`
	StatusCode    int       `json:"status_code,omitempty"`
	BytesSent     int64     `json:"bytes_sent"`
	BytesRecv     int64     `json:"bytes_recv"`
	DurationMs    int64     `json:"duration_ms"`
	Error         string    `json:"error,omitempty"`
}

// Socks5Server implements a SOCKS5 proxy server for logging outgoing API calls
type Socks5Server struct {
	config         *Socks5Config
	projectRoot    string
	listener       net.Listener
	logger         *log.Logger
	logFile        *os.File
	mu             sync.Mutex
	running        atomic.Bool
	wg             sync.WaitGroup
	tlsInterceptor *TLSInterceptor
}

// NewSocks5Server creates a new SOCKS5 proxy server
func NewSocks5Server(config *Socks5Config, projectRoot string) *Socks5Server {
	return &Socks5Server{
		config:      config,
		projectRoot: projectRoot,
	}
}

// Start starts the SOCKS5 proxy server
func (s *Socks5Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running.Load() {
		return fmt.Errorf("SOCKS5 server already running")
	}

	// Set up logging
	if err := s.setupLogging(); err != nil {
		return fmt.Errorf("failed to setup logging: %w", err)
	}

	// Initialize TLS interceptor if HTTPS inspection is enabled
	if s.config.HTTPSInspection != nil && s.config.HTTPSInspection.Enabled {
		interceptor, err := NewTLSInterceptor(s.config.HTTPSInspection, s.projectRoot)
		if err != nil {
			return fmt.Errorf("failed to initialize TLS interceptor: %w", err)
		}
		s.tlsInterceptor = interceptor
		log.Printf("SOCKS5: HTTPS inspection enabled with CA: %s", s.config.HTTPSInspection.CACert)
	}

	// Start listening
	addr := fmt.Sprintf("127.0.0.1:%d", s.config.Port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}
	s.listener = listener
	s.running.Store(true)

	log.Printf("SOCKS5 proxy listening on %s", addr)

	// Accept connections
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		for s.running.Load() {
			conn, err := s.listener.Accept()
			if err != nil {
				if s.running.Load() {
					log.Printf("SOCKS5: Accept error: %v", err)
				}
				continue
			}
			s.wg.Add(1)
			go func(c net.Conn) {
				defer s.wg.Done()
				s.handleConnection(c)
			}(conn)
		}
	}()

	return nil
}

// Stop stops the SOCKS5 proxy server
func (s *Socks5Server) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running.Load() {
		return
	}

	s.running.Store(false)
	if s.listener != nil {
		s.listener.Close()
	}

	// Wait for all connections to finish (with timeout)
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		log.Printf("SOCKS5: Timeout waiting for connections to close")
	}

	if s.logFile != nil {
		s.logFile.Close()
	}

	log.Printf("SOCKS5 proxy stopped")
}

// setupLogging sets up the log file
func (s *Socks5Server) setupLogging() error {
	logPath := s.config.LogFile
	if logPath == "" {
		logPath = "logs/socks5_{date}.log"
	}

	// Replace {date} placeholder
	logPath = strings.ReplaceAll(logPath, "{date}", time.Now().Format("2006-01-02"))

	// Make path absolute
	if !filepath.IsAbs(logPath) {
		logPath = filepath.Join(s.projectRoot, logPath)
	}

	// Create directory if needed
	if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
		return err
	}

	// Open log file
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	s.logFile = f
	s.logger = log.New(f, "", 0)

	return nil
}

// handleConnection handles a single SOCKS5 connection
func (s *Socks5Server) handleConnection(conn net.Conn) {
	defer conn.Close()

	// Set timeouts
	conn.SetDeadline(time.Now().Add(60 * time.Second))

	// Step 1: Version/method negotiation
	if err := s.handleHandshake(conn); err != nil {
		log.Printf("SOCKS5: Handshake failed: %v", err)
		return
	}

	// Step 2: Request (CONNECT)
	destHost, destPort, err := s.handleRequest(conn)
	if err != nil {
		log.Printf("SOCKS5: Request failed: %v", err)
		return
	}

	// Step 3: Connect to destination
	startTime := time.Now()
	destAddr := fmt.Sprintf("%s:%d", destHost, destPort)
	destConn, err := net.DialTimeout("tcp", destAddr, 10*time.Second)
	if err != nil {
		s.sendReply(conn, replyHostUnreach, nil)
		s.logConnection(&ConnectionLog{
			Timestamp:  startTime,
			DestHost:   destHost,
			DestPort:   destPort,
			Protocol:   s.detectProtocol(destPort),
			DurationMs: time.Since(startTime).Milliseconds(),
			Error:      err.Error(),
		})
		return
	}
	defer destConn.Close()

	// Send success reply
	if err := s.sendReply(conn, replySuccess, destConn.LocalAddr()); err != nil {
		log.Printf("SOCKS5: Failed to send reply: %v", err)
		return
	}

	// Clear deadline for relay
	conn.SetDeadline(time.Time{})
	destConn.SetDeadline(time.Time{})

	// Check if we should intercept HTTPS
	if s.tlsInterceptor != nil && destPort == 443 {
		s.tlsInterceptor.Intercept(conn, destConn, destHost, destPort, startTime, s.logConnection)
		return
	}

	// Relay data
	bytesSent, bytesRecv := s.relay(conn, destConn)

	// Log connection
	s.logConnection(&ConnectionLog{
		Timestamp:  startTime,
		DestHost:   destHost,
		DestPort:   destPort,
		Protocol:   s.detectProtocol(destPort),
		BytesSent:  bytesSent,
		BytesRecv:  bytesRecv,
		DurationMs: time.Since(startTime).Milliseconds(),
	})
}

// handleHandshake performs SOCKS5 handshake
func (s *Socks5Server) handleHandshake(conn net.Conn) error {
	// Read version and number of methods
	buf := make([]byte, 2)
	if _, err := io.ReadFull(conn, buf); err != nil {
		return fmt.Errorf("failed to read handshake: %w", err)
	}

	if buf[0] != socks5Version {
		return fmt.Errorf("unsupported SOCKS version: %d", buf[0])
	}

	numMethods := int(buf[1])
	methods := make([]byte, numMethods)
	if _, err := io.ReadFull(conn, methods); err != nil {
		return fmt.Errorf("failed to read methods: %w", err)
	}

	// Check for no-auth method
	hasNoAuth := false
	for _, m := range methods {
		if m == authNoAuth {
			hasNoAuth = true
			break
		}
	}

	if !hasNoAuth {
		// Send no acceptable methods
		conn.Write([]byte{socks5Version, 0xFF})
		return fmt.Errorf("no acceptable authentication method")
	}

	// Send no-auth response
	if _, err := conn.Write([]byte{socks5Version, authNoAuth}); err != nil {
		return fmt.Errorf("failed to send handshake response: %w", err)
	}

	return nil
}

// handleRequest handles SOCKS5 CONNECT request
func (s *Socks5Server) handleRequest(conn net.Conn) (string, int, error) {
	// Read request header: VER, CMD, RSV, ATYP
	buf := make([]byte, 4)
	if _, err := io.ReadFull(conn, buf); err != nil {
		return "", 0, fmt.Errorf("failed to read request: %w", err)
	}

	if buf[0] != socks5Version {
		return "", 0, fmt.Errorf("unsupported SOCKS version: %d", buf[0])
	}

	if buf[1] != cmdConnect {
		s.sendReply(conn, replyCmdNotSupported, nil)
		return "", 0, fmt.Errorf("unsupported command: %d", buf[1])
	}

	// Parse address based on type
	var destHost string
	switch buf[3] {
	case addrTypeIPv4:
		addr := make([]byte, 4)
		if _, err := io.ReadFull(conn, addr); err != nil {
			return "", 0, err
		}
		destHost = net.IP(addr).String()

	case addrTypeDomain:
		lenBuf := make([]byte, 1)
		if _, err := io.ReadFull(conn, lenBuf); err != nil {
			return "", 0, err
		}
		domain := make([]byte, lenBuf[0])
		if _, err := io.ReadFull(conn, domain); err != nil {
			return "", 0, err
		}
		destHost = string(domain)

	case addrTypeIPv6:
		addr := make([]byte, 16)
		if _, err := io.ReadFull(conn, addr); err != nil {
			return "", 0, err
		}
		destHost = net.IP(addr).String()

	default:
		s.sendReply(conn, replyAddrNotSupported, nil)
		return "", 0, fmt.Errorf("unsupported address type: %d", buf[3])
	}

	// Read port
	portBuf := make([]byte, 2)
	if _, err := io.ReadFull(conn, portBuf); err != nil {
		return "", 0, err
	}
	destPort := int(binary.BigEndian.Uint16(portBuf))

	return destHost, destPort, nil
}

// sendReply sends SOCKS5 reply
func (s *Socks5Server) sendReply(conn net.Conn, replyCode byte, bindAddr net.Addr) error {
	reply := []byte{socks5Version, replyCode, 0x00, addrTypeIPv4, 0, 0, 0, 0, 0, 0}

	if bindAddr != nil {
		if tcpAddr, ok := bindAddr.(*net.TCPAddr); ok {
			ip := tcpAddr.IP.To4()
			if ip != nil {
				copy(reply[4:8], ip)
				binary.BigEndian.PutUint16(reply[8:10], uint16(tcpAddr.Port))
			}
		}
	}

	_, err := conn.Write(reply)
	return err
}

// relay copies data bidirectionally between connections
func (s *Socks5Server) relay(client, server net.Conn) (bytesSent, bytesRecv int64) {
	var wg sync.WaitGroup
	wg.Add(2)

	// Client -> Server
	go func() {
		defer wg.Done()
		n, _ := io.Copy(server, client)
		atomic.AddInt64(&bytesSent, n)
		server.(*net.TCPConn).CloseWrite()
	}()

	// Server -> Client
	go func() {
		defer wg.Done()
		n, _ := io.Copy(client, server)
		atomic.AddInt64(&bytesRecv, n)
		client.(*net.TCPConn).CloseWrite()
	}()

	wg.Wait()
	return bytesSent, bytesRecv
}

// detectProtocol guesses protocol from port
func (s *Socks5Server) detectProtocol(port int) string {
	switch port {
	case 80:
		return "http"
	case 443:
		return "https"
	default:
		return "tcp"
	}
}

// logConnection logs a connection event
func (s *Socks5Server) logConnection(entry *ConnectionLog) {
	if s.logger == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.config.LogFormat == "text" {
		s.logger.Printf("[%s] [%s] CONNECT %s:%d -> %d sent, %d recv, %dms",
			entry.Timestamp.Format("2006-01-02 15:04:05"),
			entry.WorkerName,
			entry.DestHost, entry.DestPort,
			entry.BytesSent, entry.BytesRecv, entry.DurationMs)
	} else {
		data, _ := json.Marshal(entry)
		s.logger.Println(string(data))
	}
}
