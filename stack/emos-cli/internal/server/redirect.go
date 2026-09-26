package server

import (
	"bufio"
	"net"
	"net/http"
	"sync"
	"time"
)

// A dashboard served over HTTPS still answers plain HTTP on its port with a
// redirect. A TLS handshake starts with the record type 0x16, an HTTP request
// with a method.

const tlsHandshakeRecord = 0x16

// peekTimeout bounds how long a connection may stay silent before it is
// dropped unclassified.
const peekTimeout = 10 * time.Second

// splitListener hands each connection of ln to one of two listeners by its
// first byte. Closing either closes ln.
type splitListener struct {
	ln    net.Listener
	tls   chan net.Conn
	plain chan net.Conn
	done  chan struct{}
	once  sync.Once
}

func splitTLS(ln net.Listener) (tlsConns, plainConns net.Listener) {
	s := &splitListener{
		ln:    ln,
		tls:   make(chan net.Conn),
		plain: make(chan net.Conn),
		done:  make(chan struct{}),
	}
	go s.accept()
	return &half{s, s.tls}, &half{s, s.plain}
}

func (s *splitListener) accept() {
	for {
		c, err := s.ln.Accept()
		if err != nil {
			s.shutdown()
			return
		}
		go s.route(c)
	}
}

func (s *splitListener) route(c net.Conn) {
	r := bufio.NewReader(c)
	c.SetReadDeadline(time.Now().Add(peekTimeout))
	first, err := r.Peek(1)
	c.SetReadDeadline(time.Time{})
	if err != nil {
		c.Close()
		return
	}
	dst := s.plain
	if first[0] == tlsHandshakeRecord {
		dst = s.tls
	}
	select {
	case dst <- &peekedConn{Conn: c, r: r}:
	case <-s.done:
		c.Close()
	}
}

func (s *splitListener) shutdown() {
	s.once.Do(func() {
		close(s.done)
		s.ln.Close()
	})
}

// half is one side of a splitListener.
type half struct {
	*splitListener
	conns chan net.Conn
}

func (h *half) Accept() (net.Conn, error) {
	select {
	case c := <-h.conns:
		return c, nil
	case <-h.done:
		return nil, net.ErrClosed
	}
}

func (h *half) Close() error {
	h.shutdown()
	return nil
}

func (h *half) Addr() net.Addr { return h.ln.Addr() }

// peekedConn replays the byte read to classify the connection.
type peekedConn struct {
	net.Conn
	r *bufio.Reader
}

func (c *peekedConn) Read(p []byte) (int, error) { return c.r.Read(p) }

// redirectToHTTPS sends a plain HTTP request to the same URL over HTTPS.
func redirectToHTTPS(w http.ResponseWriter, r *http.Request) {
	if r.Host == "" {
		http.Error(w, "This dashboard is served over HTTPS", http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "https://"+r.Host+r.URL.RequestURI(), http.StatusTemporaryRedirect)
}
