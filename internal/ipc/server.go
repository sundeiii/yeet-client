package ipc

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/rpc"
	"sync"
	"time"
)

const (
	inboxCapacity     = maxUploadPaths
	requestLimit      = 256 * 1024 // 256 KiB
	connectionTimeout = 5 * time.Second
)

var (
	ErrShuttingDown = errors.New("puush is shutting down")
	ErrInboxFull    = errors.New("ipc command inbox is full")
)

// Server owns the IPC endpoint and exposes accepted commands.
type Server struct {
	listener  net.Listener
	rpc       *rpc.Server
	inbox     chan Command
	done      chan struct{}
	closeOnce sync.Once
}

func newServer() (*Server, error) {
	server := &Server{
		rpc:   rpc.NewServer(),
		inbox: make(chan Command, inboxCapacity),
		done:  make(chan struct{}),
	}
	if err := server.rpc.RegisterName(rpcServiceName, server); err != nil {
		return nil, fmt.Errorf("register IPC service: %w", err)
	}
	return server, nil
}

// Incoming returns commands accepted into the IPC inbox.
func (s *Server) Incoming() <-chan Command {
	return s.inbox
}

// Done closes when the IPC endpoint is shutting down.
func (s *Server) Done() <-chan struct{} {
	return s.done
}

// Close releases the IPC endpoint.
func (s *Server) Close() (err error) {
	s.closeOnce.Do(func() {
		close(s.done)
		if s.listener != nil {
			err = s.listener.Close()
		}
	})
	return err
}

func (s *Server) serve() {
	for {
		// Listen for incoming ipc connections & handle them in a go routine
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.done:
				return
			default:
				log.Printf("IPC accept error: %v", err)
				return
			}
		}

		go func() {
			// Set a deadline to avoid hanging connections & limit the request size to avoid DoS attacks
			conn.SetDeadline(time.Now().Add(connectionTimeout))
			limited := &limitedReadConnection{Conn: conn, reader: io.LimitReader(conn, requestLimit)}
			s.rpc.ServeConn(limited)
		}()
	}
}

func (s *Server) enqueue(command Command) error {
	validated, err := command.ValidateReceived()
	if err != nil {
		return err
	}

	// Prefer a shutdown over accepting more work when `Close` has already run
	select {
	case <-s.done:
		return ErrShuttingDown
	default:
	}

	select {
	case s.inbox <- validated:
		// Command accepted into the inbox
		return nil
	case <-s.done:
		// Server is shutting down
		return ErrShuttingDown
	default:
		// Inbox is full, but we allow ActionAttention to avoid deadlocks
		if validated.Action == ActionAttention {
			return nil
		}
		return ErrInboxFull
	}
}

type limitedReadConnection struct {
	net.Conn
	reader io.Reader
}

func (c *limitedReadConnection) Read(buffer []byte) (int, error) {
	return c.reader.Read(buffer)
}
