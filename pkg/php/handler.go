package php

import (
	"log"

	"github.com/mevdschee/tqserver/pkg/fastcgi"
	phpfpmpkg "github.com/mevdschee/tqserver/pkg/php/phpfpm"
)

// FastCGIHandler forwards requests to a central php-fpm client instead of per-worker sockets.
type FastCGIHandler struct {
	client *phpfpmpkg.Client
}

// NewFastCGIHandler creates a new FastCGI handler that uses the provided phpfpm Client.
func NewFastCGIHandler(client *phpfpmpkg.Client) *FastCGIHandler {
	return &FastCGIHandler{client: client}
}

// ServeFastCGI forwards the FastCGI request to php-fpm via the pooled client and returns the response.
func (h *FastCGIHandler) ServeFastCGI(conn *fastcgi.Conn, req *fastcgi.Request) error {
	// Delegate to the central client
	stdout, stderr, _, err := h.client.DoRequest(req.Params, req.Stdin)
	if err != nil {
		log.Printf("phpfpm client error: %v", err)
		if len(stderr) > 0 {
			conn.SendStderr(req.RequestID, stderr)
		} else {
			conn.SendStderr(req.RequestID, []byte(err.Error()))
		}
		conn.SendEndRequest(req.RequestID, 1, uint8(fastcgi.StatusRequestComplete))
		return err
	}

	if len(stdout) > 0 {
		if err := conn.SendStdout(req.RequestID, stdout); err != nil {
			return err
		}
	}
	if len(stderr) > 0 {
		if err := conn.SendStderr(req.RequestID, stderr); err != nil {
			return err
		}
	}

	if err := conn.SendEndRequest(req.RequestID, 0, uint8(fastcgi.StatusRequestComplete)); err != nil {
		return err
	}
	return nil
}
