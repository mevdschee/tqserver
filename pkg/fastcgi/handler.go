package fastcgi

// Handler processes FastCGI requests
type Handler interface {
	ServeFastCGI(conn *Conn, req *Request) error
}

// HandlerFunc is an adapter to allow ordinary functions to be used as handlers
type HandlerFunc func(conn *Conn, req *Request) error

func (f HandlerFunc) ServeFastCGI(conn *Conn, req *Request) error {
	return f(conn, req)
}
