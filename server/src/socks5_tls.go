package main

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// TLSInterceptor handles HTTPS MITM inspection
type TLSInterceptor struct {
	config    *HTTPSInspectionConfig
	caCert    *x509.Certificate
	caKey     *rsa.PrivateKey
	certCache sync.Map // domain -> *tls.Certificate
}

// NewTLSInterceptor creates a new TLS interceptor with CA certificate
func NewTLSInterceptor(config *HTTPSInspectionConfig, projectRoot string) (*TLSInterceptor, error) {
	t := &TLSInterceptor{
		config: config,
	}

	// Resolve paths
	certPath := config.CACert
	keyPath := config.CAKey
	if !filepath.IsAbs(certPath) {
		certPath = filepath.Join(projectRoot, certPath)
	}
	if !filepath.IsAbs(keyPath) {
		keyPath = filepath.Join(projectRoot, keyPath)
	}

	// Check if CA exists
	_, certErr := os.Stat(certPath)
	_, keyErr := os.Stat(keyPath)

	if os.IsNotExist(certErr) || os.IsNotExist(keyErr) {
		if config.AutoGenerate {
			if err := t.generateCA(certPath, keyPath); err != nil {
				return nil, fmt.Errorf("failed to generate CA: %w", err)
			}
		} else {
			return nil, fmt.Errorf("CA certificate not found: %s", certPath)
		}
	}

	// Load CA
	if err := t.loadCA(certPath, keyPath); err != nil {
		return nil, fmt.Errorf("failed to load CA: %w", err)
	}

	return t, nil
}

// generateCA generates a new CA certificate
func (t *TLSInterceptor) generateCA(certPath, keyPath string) error {
	// Generate private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("failed to generate private key: %w", err)
	}

	// Create certificate template
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"TQServer Development CA"},
			CommonName:   "TQServer Proxy CA",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0), // 10 years
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		MaxPathLen:            1,
	}

	// Self-sign certificate
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return fmt.Errorf("failed to create certificate: %w", err)
	}

	// Create directories if needed
	if err := os.MkdirAll(filepath.Dir(certPath), 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(keyPath), 0755); err != nil {
		return err
	}

	// Write certificate
	certFile, err := os.Create(certPath)
	if err != nil {
		return fmt.Errorf("failed to create cert file: %w", err)
	}
	defer certFile.Close()

	if err := pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}); err != nil {
		return fmt.Errorf("failed to encode certificate: %w", err)
	}

	// Write private key
	keyFile, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to create key file: %w", err)
	}
	defer keyFile.Close()

	keyDER := x509.MarshalPKCS1PrivateKey(privateKey)
	if err := pem.Encode(keyFile, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: keyDER}); err != nil {
		return fmt.Errorf("failed to encode private key: %w", err)
	}

	fmt.Printf("Generated TQServer CA certificate: %s\n", certPath)
	return nil
}

// loadCA loads the CA certificate and key
func (t *TLSInterceptor) loadCA(certPath, keyPath string) error {
	// Read certificate
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return fmt.Errorf("failed to read certificate: %w", err)
	}

	block, _ := pem.Decode(certPEM)
	if block == nil {
		return fmt.Errorf("failed to decode certificate PEM")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse certificate: %w", err)
	}
	t.caCert = cert

	// Read private key
	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return fmt.Errorf("failed to read private key: %w", err)
	}

	block, _ = pem.Decode(keyPEM)
	if block == nil {
		return fmt.Errorf("failed to decode private key PEM")
	}

	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse private key: %w", err)
	}
	t.caKey = key

	return nil
}

// generateDomainCert creates a certificate for a specific domain, signed by our CA
func (t *TLSInterceptor) generateDomainCert(domain string) (*tls.Certificate, error) {
	// Check cache first
	if cached, ok := t.certCache.Load(domain); ok {
		return cached.(*tls.Certificate), nil
	}

	// Generate new key for this domain
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("failed to generate domain key: %w", err)
	}

	// Create certificate template
	template := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject: pkix.Name{
			CommonName: domain,
		},
		DNSNames:    []string{domain},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().AddDate(0, 0, 1), // 1 day validity
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	// Sign with our CA
	certDER, err := x509.CreateCertificate(rand.Reader, template, t.caCert, &privateKey.PublicKey, t.caKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create domain certificate: %w", err)
	}

	// Create tls.Certificate
	cert := &tls.Certificate{
		Certificate: [][]byte{certDER},
		PrivateKey:  privateKey,
	}

	// Cache it
	t.certCache.Store(domain, cert)

	return cert, nil
}

// Intercept performs HTTPS MITM interception
func (t *TLSInterceptor) Intercept(clientConn, serverConn net.Conn, destHost string, destPort int, startTime time.Time, logFn func(*ConnectionLog)) {
	// Close the pre-connected server connection - we'll establish our own TLS connection
	serverConn.Close()

	// Generate certificate for the destination domain
	cert, err := t.generateDomainCert(destHost)
	if err != nil {
		logFn(&ConnectionLog{
			Timestamp:  startTime,
			DestHost:   destHost,
			DestPort:   destPort,
			Protocol:   "https",
			DurationMs: time.Since(startTime).Milliseconds(),
			Error:      fmt.Sprintf("failed to generate cert: %v", err),
		})
		return
	}

	// Wrap client connection with TLS (we become the "server")
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{*cert},
	}
	tlsClientConn := tls.Server(clientConn, tlsConfig)
	if err := tlsClientConn.Handshake(); err != nil {
		logFn(&ConnectionLog{
			Timestamp:  startTime,
			DestHost:   destHost,
			DestPort:   destPort,
			Protocol:   "https",
			DurationMs: time.Since(startTime).Milliseconds(),
			Error:      fmt.Sprintf("client TLS handshake failed: %v", err),
		})
		return
	}
	defer tlsClientConn.Close()

	// Connect to the real destination
	destAddr := fmt.Sprintf("%s:%d", destHost, destPort)
	realConn, err := net.DialTimeout("tcp", destAddr, 10*time.Second)
	if err != nil {
		logFn(&ConnectionLog{
			Timestamp:  startTime,
			DestHost:   destHost,
			DestPort:   destPort,
			Protocol:   "https",
			DurationMs: time.Since(startTime).Milliseconds(),
			Error:      fmt.Sprintf("failed to connect to destination: %v", err),
		})
		return
	}
	defer realConn.Close()

	// Establish TLS with the real server (we become the "client")
	tlsDestConn := tls.Client(realConn, &tls.Config{ServerName: destHost})
	if err := tlsDestConn.Handshake(); err != nil {
		logFn(&ConnectionLog{
			Timestamp:  startTime,
			DestHost:   destHost,
			DestPort:   destPort,
			Protocol:   "https",
			DurationMs: time.Since(startTime).Milliseconds(),
			Error:      fmt.Sprintf("server TLS handshake failed: %v", err),
		})
		return
	}
	defer tlsDestConn.Close()

	// If body logging is enabled, use HTTP-aware relay
	if t.config.LogBody {
		t.relayHTTPWithLogging(tlsClientConn, tlsDestConn, destHost, destPort, startTime, logFn)
	} else {
		// Simple relay with byte counting
		t.relayWithLogging(tlsClientConn, tlsDestConn, destHost, destPort, startTime, logFn)
	}
}

// relayWithLogging relays data and logs connection metadata
func (t *TLSInterceptor) relayWithLogging(clientConn, serverConn net.Conn, destHost string, destPort int, startTime time.Time, logFn func(*ConnectionLog)) {
	var bytesSent, bytesRecv int64
	var wg sync.WaitGroup
	wg.Add(2)

	// Client -> Server
	go func() {
		defer wg.Done()
		n, _ := io.Copy(serverConn, clientConn)
		bytesSent = n
	}()

	// Server -> Client
	go func() {
		defer wg.Done()
		n, _ := io.Copy(clientConn, serverConn)
		bytesRecv = n
	}()

	wg.Wait()

	logFn(&ConnectionLog{
		Timestamp:  startTime,
		DestHost:   destHost,
		DestPort:   destPort,
		Protocol:   "https",
		BytesSent:  bytesSent,
		BytesRecv:  bytesRecv,
		DurationMs: time.Since(startTime).Milliseconds(),
	})
}

// relayHTTPWithLogging parses HTTP requests/responses and logs full details
func (t *TLSInterceptor) relayHTTPWithLogging(clientConn, serverConn net.Conn, destHost string, destPort int, startTime time.Time, logFn func(*ConnectionLog)) {
	clientReader := bufio.NewReader(clientConn)

	for {
		// Read HTTP request from client
		req, err := http.ReadRequest(clientReader)
		if err != nil {
			if err != io.EOF {
				logFn(&ConnectionLog{
					Timestamp:  startTime,
					DestHost:   destHost,
					DestPort:   destPort,
					Protocol:   "https",
					DurationMs: time.Since(startTime).Milliseconds(),
					Error:      fmt.Sprintf("failed to read request: %v", err),
				})
			}
			return
		}

		reqStartTime := time.Now()

		// Capture request body if needed
		var reqBody []byte
		if t.config.LogBody && req.Body != nil {
			reqBody, _ = io.ReadAll(io.LimitReader(req.Body, int64(t.config.MaxBodySize)))
			req.Body.Close()
			req.Body = io.NopCloser(bytes.NewReader(reqBody))
		}

		// Forward request to server
		if err := req.Write(serverConn); err != nil {
			logFn(&ConnectionLog{
				Timestamp:  reqStartTime,
				DestHost:   destHost,
				DestPort:   destPort,
				Protocol:   "https",
				Method:     req.Method,
				Path:       req.URL.Path,
				UserAgent:  req.Header.Get("User-Agent"),
				DurationMs: time.Since(reqStartTime).Milliseconds(),
				Error:      fmt.Sprintf("failed to forward request: %v", err),
			})
			return
		}

		// Read response from server
		resp, err := http.ReadResponse(bufio.NewReader(serverConn), req)
		if err != nil {
			logFn(&ConnectionLog{
				Timestamp:  reqStartTime,
				DestHost:   destHost,
				DestPort:   destPort,
				Protocol:   "https",
				Method:     req.Method,
				Path:       req.URL.Path,
				UserAgent:  req.Header.Get("User-Agent"),
				DurationMs: time.Since(reqStartTime).Milliseconds(),
				Error:      fmt.Sprintf("failed to read response: %v", err),
			})
			return
		}

		// Capture response body if needed
		var respBody []byte
		if t.config.LogBody && resp.Body != nil {
			respBody, _ = io.ReadAll(io.LimitReader(resp.Body, int64(t.config.MaxBodySize)))
			resp.Body.Close()
			resp.Body = io.NopCloser(bytes.NewReader(respBody))
		}

		// Forward response to client
		if err := resp.Write(clientConn); err != nil {
			return
		}

		// Build log entry
		entry := &ConnectionLog{
			Timestamp:  reqStartTime,
			DestHost:   destHost,
			DestPort:   destPort,
			Protocol:   "https",
			Method:     req.Method,
			Path:       req.URL.Path,
			UserAgent:  req.Header.Get("User-Agent"),
			StatusCode: resp.StatusCode,
			BytesSent:  int64(len(reqBody)),
			BytesRecv:  int64(len(respBody)),
			DurationMs: time.Since(reqStartTime).Milliseconds(),
		}

		// Add request/response bodies as additional data if logging enabled
		if t.config.LogBody {
			// Create extended log with body data
			type ExtendedLog struct {
				*ConnectionLog
				RequestHeaders  map[string]string `json:"request_headers,omitempty"`
				RequestBody     string            `json:"request_body,omitempty"`
				ResponseHeaders map[string]string `json:"response_headers,omitempty"`
				ResponseBody    string            `json:"response_body,omitempty"`
			}

			extLog := ExtendedLog{ConnectionLog: entry}
			extLog.RequestHeaders = make(map[string]string)
			for k, v := range req.Header {
				if len(v) > 0 {
					// Redact sensitive headers
					if k == "Authorization" || k == "Cookie" {
						extLog.RequestHeaders[k] = "[REDACTED]"
					} else {
						extLog.RequestHeaders[k] = v[0]
					}
				}
			}
			if len(reqBody) > 0 {
				extLog.RequestBody = string(reqBody)
			}
			extLog.ResponseHeaders = make(map[string]string)
			for k, v := range resp.Header {
				if len(v) > 0 {
					extLog.ResponseHeaders[k] = v[0]
				}
			}
			if len(respBody) > 0 {
				extLog.ResponseBody = string(respBody)
			}

			// Log extended entry
			data, _ := json.Marshal(extLog)
			fmt.Println(string(data)) // TODO: use proper logger
		}

		logFn(entry)

		// Check if connection should be closed
		if resp.Close || req.Close {
			return
		}
	}
}
