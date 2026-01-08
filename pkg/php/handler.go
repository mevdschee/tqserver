package php

import (
	"fmt"
	"io"
	"log"
	"net"
	"time"

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

// ServeFastCGI processes a FastCGI request by proxying to a PHP worker
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

	// Ensure SCRIPT_FILENAME is set correctly
	if scriptFilename, ok := req.Params["SCRIPT_FILENAME"]; ok {
		log.Printf("[Worker %d] SCRIPT_FILENAME: %s", worker.ID, scriptFilename)
	} else {
		log.Printf("[Worker %d] WARNING: SCRIPT_FILENAME not set in params", worker.ID)
	}

	// Connect to the worker's internal FastCGI socket
	phpConn, err := net.DialTimeout("tcp", worker.socketPath, 5*time.Second)
	if err != nil {
		log.Printf("[Worker %d] Failed to connect to %s: %v", worker.ID, worker.socketPath, err)
		conn.SendStderr(req.RequestID, []byte(fmt.Sprintf("Failed to connect to PHP worker: %v", err)))
		conn.SendEndRequest(req.RequestID, 1, uint8(fastcgi.StatusRequestComplete))
		return err
	}
	defer phpConn.Close()

	// Set read/write deadlines to prevent hanging
	phpConn.SetDeadline(time.Now().Add(30 * time.Second))

	// Create a FastCGI connection to the PHP worker
	phpFCGI := fastcgi.NewConn(phpConn, 60*time.Second, 60*time.Second)

	// Forward the begin request
	if err := phpFCGI.SendBeginRequest(req.RequestID, req.Role, req.KeepConn); err != nil {
		log.Printf("[Worker %d] Failed to send begin request: %v", worker.ID, err)
		conn.SendStderr(req.RequestID, []byte(fmt.Sprintf("Failed to send request to PHP: %v", err)))
		conn.SendEndRequest(req.RequestID, 1, uint8(fastcgi.StatusRequestComplete))
		return err
	}

	// Forward parameters
	if err := phpFCGI.SendParams(req.RequestID, req.Params); err != nil {
		log.Printf("[Worker %d] Failed to send params: %v", worker.ID, err)
		conn.SendStderr(req.RequestID, []byte(fmt.Sprintf("Failed to send parameters: %v", err)))
		conn.SendEndRequest(req.RequestID, 1, uint8(fastcgi.StatusRequestComplete))
		return err
	}

	// Send empty params to signal end of params
	if err := phpFCGI.SendParams(req.RequestID, nil); err != nil {
		log.Printf("[Worker %d] Failed to send empty params: %v", worker.ID, err)
		return err
	}

	// Forward stdin (request body)
	if len(req.Stdin) > 0 {
		if err := phpFCGI.SendStdin(req.RequestID, req.Stdin); err != nil {
			log.Printf("[Worker %d] Failed to send stdin: %v", worker.ID, err)
			conn.SendStderr(req.RequestID, []byte(fmt.Sprintf("Failed to send request body: %v", err)))
			conn.SendEndRequest(req.RequestID, 1, uint8(fastcgi.StatusRequestComplete))
			return err
		}
	}

	// Send empty stdin to signal end of stdin
	if err := phpFCGI.SendStdin(req.RequestID, nil); err != nil {
		log.Printf("[Worker %d] Failed to send empty stdin: %v", worker.ID, err)
		return err
	}

	log.Printf("[Worker %d] Request sent, reading response...", worker.ID)

	// Read response from PHP worker and forward to client
	for {
		record, err := phpFCGI.ReadRecord()
		if err != nil {
			if err == io.EOF || err == fastcgi.ErrConnClosed {
				log.Printf("[Worker %d] Connection closed by worker", worker.ID)
				break
			}
			log.Printf("[Worker %d] Failed to read response: %v", worker.ID, err)
			conn.SendStderr(req.RequestID, []byte(fmt.Sprintf("Worker error: %v", err)))
			conn.SendEndRequest(req.RequestID, 1, uint8(fastcgi.StatusRequestComplete))
			return err
		}

		log.Printf("[Worker %d] Received record type: %v, length: %d", worker.ID, record.Header.Type, len(record.Content))

		// Forward the record to the client
		switch record.Header.Type {
		case fastcgi.TypeStdout:
			if len(record.Content) > 0 {
				if err := conn.SendStdout(req.RequestID, record.Content); err != nil {
					log.Printf("[Worker %d] Failed to forward stdout: %v", worker.ID, err)
					return err
				}
			}

		case fastcgi.TypeStderr:
			if len(record.Content) > 0 {
				if err := conn.SendStderr(req.RequestID, record.Content); err != nil {
					log.Printf("[Worker %d] Failed to forward stderr: %v", worker.ID, err)
					return err
				}
			}

		case fastcgi.TypeEndRequest:
			// Forward end request and we're done
			if err := conn.SendEndRequest(req.RequestID, 0, uint8(fastcgi.StatusRequestComplete)); err != nil {
				log.Printf("[Worker %d] Failed to send end request: %v", worker.ID, err)
				return err
			}
			return nil
		}
	}

	// If we got here without an explicit end request, send one
	if err := conn.SendEndRequest(req.RequestID, 0, uint8(fastcgi.StatusRequestComplete)); err != nil {
		log.Printf("[Worker %d] Failed to send final end request: %v", worker.ID, err)
		return err
	}

	return nil
}
