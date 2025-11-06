package tcp

import (
	"log"
	"net"
	"sync"
)

type Server struct {
	Port     string
	listener net.Listener
	clients  map[string]net.Conn
	mu       sync.RWMutex
	running  bool
}

func NewServer(port string) *Server {
	return &Server{
		Port:    port,
		clients: make(map[string]net.Conn),
		running: false,
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
		log.Printf("New connection from %s", conn.RemoteAddr())
		go s.handleClient(conn)
	}
}

func (s *Server) Stop() error {
	s.running = false
	s.mu.Lock()
	for userID, conn := range s.clients {
		conn.Close()
		delete(s.clients, userID)
	}
	s.mu.Unlock()
	if s.listener != nil {
		return s.listener.Close()
	}
	log.Println("TCP server stopped")
	return nil
}

func (s *Server) addClient(userID string, conn net.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clients[userID] = conn
	log.Printf("Client %s connected (total: %d)", userID, len(s.clients))
}

func (s *Server) removeClient(userID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if conn, exists := s.clients[userID]; exists {
		if conn != nil {
			conn.Close()
		}
		delete(s.clients, userID)
		log.Printf("Client %s disconnected (total: %d)", userID, len(s.clients))
	}
}

func (s *Server) GetClientCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.clients)
}

func (s *Server) handleClient(conn net.Conn) {
	defer conn.Close()
	log.Printf("Handling connection from %s", conn.RemoteAddr())
	buf := make([]byte, 1024)
	for {
		_, err := conn.Read(buf)
		if err != nil {
			log.Printf("Connection closed: %v", err)
			return
		}
	}
}
