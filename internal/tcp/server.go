package tcp

import (
	"log"
	"net"
)

type Server struct {
	Port          string
	listener      net.Listener
	running       bool
	clientManager *ClientManager
}

func NewServer(port string) *Server {
	return &Server{
		Port:          port,
		running:       false,
		clientManager: NewClientManager(),
	}
}

func (s *Server) Start() error {
	var err error
	s.listener, err = net.Listen("tcp", ":"+s.Port)
	if err != nil {
		return err
	}
	s.running = true
	log.Printf("TCP server listening on port %s", s.Port)
	go s.acceptConnections()
	return nil
}

func (s *Server) acceptConnections() {
	for s.running {
		conn, err := s.listener.Accept()
		if err != nil {
			if s.running {
				log.Printf("Accept error: %v", err)
			}
			continue
		}
		clientID := conn.RemoteAddr().String()
		client := &Client{Conn: conn, ID: clientID}
		s.clientManager.Add(client)
		go HandleConnection(client, s.clientManager, s.removeClient)
	}
}

func (s *Server) Stop() error {
	s.running = false
	for _, client := range s.clientManager.List() {
		client.Conn.Close()
		s.clientManager.Remove(client.ID)
	}
	if s.listener != nil {
		return s.listener.Close()
	}
	log.Println("TCP server stopped")
	return nil
}

func (s *Server) removeClient(userID string) {
	s.clientManager.Remove(userID)
}

func (s *Server) GetClientCount() int {
	return len(s.clientManager.List())
}
