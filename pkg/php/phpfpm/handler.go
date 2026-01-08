package phpfpm

import (
	"fmt"

	"github.com/mevdschee/tqserver/pkg/fastcgi"
)

// Handler implements fastcgi.Handler by forwarding requests to a php-fpm Client.
type Handler struct {
	client *Client
}

// NewHandler creates a new Handler bound to the given Client.
func NewHandler(c *Client) *Handler {
	return &Handler{client: c}
}

// ServeFastCGI implements fastcgi.Handler by performing a DoRequest on the client
// and streaming the returned stdout/stderr back to the FastCGI connection.
func (h *Handler) ServeFastCGI(conn *fastcgi.Conn, req *fastcgi.Request) error {
	stdout, stderr, _, err := h.client.DoRequest(req.Params, req.Stdin)
	if err != nil {
		// indicate error
		conn.SendStderr(req.RequestID, []byte(fmt.Sprintf("phpfpm error: %v", err)))
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

	// End request successfully
	if err := conn.SendEndRequest(req.RequestID, 0, uint8(fastcgi.StatusRequestComplete)); err != nil {
		return err
	}

	return nil
}
