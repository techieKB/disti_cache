package server

import (
	"bufio"
	"errors"
	"net"
	"sync"

	"github.com/techieKB/disti_cache/internal/protocol"
)

// maxLineBytes bounds memory per connection: a client that never sends
// '\n' gets an error, not our whole heap.
const maxLineBytes = 1 << 20

type Handler interface {
	Handle(cmd protocol.Command) string
}

type HandlerFunc func(protocol.Command) string

func (f HandlerFunc) Handle(c protocol.Command) string { return f(c) }

type Server struct {
	h      Handler
	mu     sync.Mutex
	ln     net.Listener
	conns  map[net.Conn]struct{}
	closed bool
	wg     sync.WaitGroup // one per live connection goroutine
}

func New(h Handler) *Server {
	return &Server{h: h, conns: make(map[net.Conn]struct{})}
}

// Serve accepts connections until Close is called, then returns nil.
func (s *Server) Serve(ln net.Listener) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return ln.Close()
	}
	s.ln = ln
	s.mu.Unlock()

	for {
		conn, err := ln.Accept()
		if err != nil {
			s.mu.Lock()
			closed := s.closed
			s.mu.Unlock()
			if closed {
				return nil
			}
			return err
		}
		if !s.track(conn) {
			conn.Close()
			continue
		}
		go func() {
			defer s.untrack(conn)
			s.handleConn(conn)
		}()
	}
}

// Close stops accepting, closes every live connection, and waits for their
// goroutines to exit. Safe to call more than once.
func (s *Server) Close() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	if s.ln != nil {
		s.ln.Close()
	}
	for c := range s.conns {
		c.Close()
	}
	s.mu.Unlock()
	s.wg.Wait()
}

// track registers conn and bumps the WaitGroup under the same lock that
// Close takes, so wg.Add can never race with wg.Wait.
func (s *Server) track(c net.Conn) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return false
	}
	s.conns[c] = struct{}{}
	s.wg.Add(1)
	return true
}

func (s *Server) untrack(c net.Conn) {
	s.mu.Lock()
	delete(s.conns, c)
	s.mu.Unlock()
	s.wg.Done()
}

func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close()
	sc := bufio.NewScanner(conn)
	sc.Buffer(make([]byte, 0, 4096), maxLineBytes)
	w := bufio.NewWriter(conn)
	for sc.Scan() {
		w.WriteString(s.dispatch(sc.Text()))
		w.WriteByte('\n')
		if err := w.Flush(); err != nil {
			return
		}
	}
	if errors.Is(sc.Err(), bufio.ErrTooLong) {
		w.WriteString(protocol.Err("line too long") + "\n")
		w.Flush()
	}
}

func (s *Server) dispatch(line string) string {
	cmd, err := protocol.Parse(line)
	if err != nil {
		return protocol.Err(err.Error())
	}
	return s.h.Handle(cmd)
}
