package core

import (
	"net"
)

// Server is the generic TCP proxy server.
// It depends ONLY on interfaces, not concrete implementations.
type Server struct {
	Listener          net.Listener
	ConnectionHandler ConnectionHandler
}

// Serve starts accepting connections.
func (s *Server) Serve() error {
	for {
		conn, err := s.Listener.Accept()
		if err != nil {
			return err
		}
		go s.handleConnection(conn)
	}
}

func (s *Server) handleConnection(clientConn net.Conn) {
	// Delegate the entire lifecycle to the handler
	s.ConnectionHandler.HandleConnection(clientConn)
}
