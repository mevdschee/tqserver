package php

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"strings"

	"github.com/mevdschee/tqserver/pkg/fastcgi"
)

// FastCGIHandler handles FastCGI requests using PHP workers
type FastCGIHandler struct {
	manager *Manager
}

// NewFastCGIHandler creates a new FastCGI handler for PHP
func NewFastCGIHandler(manager *Manager) *FastCGIHandler {
	return &FastCGIHandler{
		manager: manager,
	}
}

// ServeFastCGI processes a FastCGI request using a PHP worker
func (h *FastCGIHandler) ServeFastCGI(conn *fastcgi.Conn, req *fastcgi.Request) error {
	// Get an idle worker from the pool
	worker, err := h.manager.GetIdleWorker()
	if err != nil {
		log.Printf("Failed to get idle worker: %v", err)
		// Send error response
		conn.SendStderr(req.RequestID, []byte(fmt.Sprintf("Service unavailable: %v", err)))
		conn.SendEndRequest(req.RequestID, 1, uint8(fastcgi.StatusRequestComplete))
		return err
	}

	defer h.manager.ReleaseWorker(worker)

	// Mark worker as active
	worker.MarkActive()
	defer worker.MarkIdle()

	// Build CGI environment from FastCGI params
	cgiEnv := buildCGIEnvironment(req.Params)

	// Write CGI request to worker's stdin
	if err := writeCGIRequest(worker.stdin, cgiEnv, req.Stdin); err != nil {
		log.Printf("[Worker %d] Failed to write CGI request: %v", worker.ID, err)
		conn.SendStderr(req.RequestID, []byte(fmt.Sprintf("Failed to send request to PHP: %v", err)))
		conn.SendEndRequest(req.RequestID, 1, uint8(fastcgi.StatusRequestComplete))
		return err
	}

	// Read CGI response from worker's stdout
	if err := readCGIResponse(worker.stdout, conn, req.RequestID); err != nil {
		log.Printf("[Worker %d] Failed to read CGI response: %v", worker.ID, err)
		return err
	}

	return nil
}

// buildCGIEnvironment converts FastCGI params to CGI environment format
func buildCGIEnvironment(params map[string]string) string {
	var env strings.Builder
	for key, value := range params {
		env.WriteString(key)
		env.WriteString("=")
		env.WriteString(value)
		env.WriteString("\n")
	}
	return env.String()
}

// writeCGIRequest writes a CGI request (environment + body) to the worker
func writeCGIRequest(stdin io.Writer, env string, body []byte) error {
	// Write environment variables
	if _, err := io.WriteString(stdin, env); err != nil {
		return fmt.Errorf("failed to write environment: %w", err)
	}

	// Write separator (empty line)
	if _, err := io.WriteString(stdin, "\n"); err != nil {
		return fmt.Errorf("failed to write separator: %w", err)
	}

	// Write request body
	if len(body) > 0 {
		if _, err := stdin.Write(body); err != nil {
			return fmt.Errorf("failed to write body: %w", err)
		}
	}

	return nil
}

// readCGIResponse reads a CGI response from the worker and forwards it to the FastCGI client
func readCGIResponse(stdout io.Reader, conn *fastcgi.Conn, requestID uint16) error {
	reader := bufio.NewReader(stdout)

	// Read headers until blank line
	var headers bytes.Buffer
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read headers: %w", err)
		}

		headers.WriteString(line)

		// Blank line indicates end of headers
		if line == "\n" || line == "\r\n" {
			break
		}
	}

	// Send headers as stdout to FastCGI client
	if headers.Len() > 0 {
		if err := conn.SendStdout(requestID, headers.Bytes()); err != nil {
			return fmt.Errorf("failed to send headers: %w", err)
		}
	}

	// Read and forward body
	buf := make([]byte, 8192)
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			if err := conn.SendStdout(requestID, buf[:n]); err != nil {
				return fmt.Errorf("failed to send body: %w", err)
			}
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("failed to read body: %w", err)
		}
	}

	// Send empty stdout to signal end
	if err := conn.SendStdout(requestID, nil); err != nil {
		return fmt.Errorf("failed to send empty stdout: %w", err)
	}

	// Send end request
	if err := conn.SendEndRequest(requestID, 0, uint8(fastcgi.StatusRequestComplete)); err != nil {
		return fmt.Errorf("failed to send end request: %w", err)
	}

	return nil
}
