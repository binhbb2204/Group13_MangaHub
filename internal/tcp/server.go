package tcp

import (
	"net"
	"sync/atomic"

	"github.com/binhbb2204/Manga-Hub-Group13/internal/bridge"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/logger"
)

type Server struct {
	Port             string
	listener         net.Listener
	running          atomic.Bool
	clientManager    *ClientManager
	log              *logger.Logger
	bridge           *bridge.Bridge
	sessionManager   *SessionManager
	heartbeatManager *HeartbeatManager
}

func NewServer(port string, br *bridge.Bridge) *Server {
	sm := NewSessionManager()
	if br != nil {
		br.SetSessionManager(sm.AsInterface())
	}
	return &Server{
		Port:             port,
		clientManager:    NewClientManager(),
		log:              logger.WithContext("component", "tcp_server"),
		bridge:           br,
		sessionManager:   sm,
		heartbeatManager: NewHeartbeatManager(DefaultHeartbeatConfig()),
	}
}

func (s *Server) Start() error {
	var err error
	s.listener, err = net.Listen("tcp", ":"+s.Port)
	if err != nil {
		netErr := NewNetworkConnectionError(err)
		s.log.Error("failed_to_start_tcp_server", "error", netErr.Error(), "port", s.Port)
		return netErr
	}
	s.running.Store(true)
	s.heartbeatManager.Start()
	s.log.Info("tcp_server_started", "port", s.Port)
	go s.acceptConnections()
	return nil
}

func (s *Server) acceptConnections() {
	for s.running.Load() {
		conn, err := s.listener.Accept()
		if err != nil {
			if s.running.Load() {
				netErr := NewNetworkConnectionError(err)
				s.log.Warn("accept_connection_error", "error", netErr.Error())
			}
			continue
		}
		clientID := conn.RemoteAddr().String()
		client := &Client{Conn: conn, ID: clientID}
		s.clientManager.Add(client)
		s.log.Debug("new_client_accepted", "client_id", clientID)
		go HandleConnection(client, s.clientManager, s.removeClient, s.bridge, s.sessionManager, s.heartbeatManager)
	}
}

func (s *Server) Stop() error {
	s.running.Store(false)
	s.heartbeatManager.Stop()
	s.log.Info("tcp_server_stopping", "active_clients", len(s.clientManager.List()))

	for _, client := range s.clientManager.List() {
		s.sessionManager.RemoveSessionByClientID(client.ID)
		client.Conn.Close()
		s.clientManager.Remove(client.ID)
	}

	if s.listener != nil {
		if err := s.listener.Close(); err != nil {
			s.log.Warn("error_closing_listener", "error", err.Error())
			return err
		}
	}

	s.log.Info("tcp_server_stopped")
	return nil
}

func (s *Server) removeClient(userID string) {
	s.clientManager.Remove(userID)
	s.log.Debug("client_removed", "client_id", userID)
}

func (s *Server) GetClientCount() int {
	return len(s.clientManager.List())
}
