package udp

import (
	"encoding/json"
	"net"
	"os"
	"sync/atomic"

	"github.com/binhbb2204/Manga-Hub-Group13/internal/bridge"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/logger"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/utils"
)

type Server struct {
	Port              string
	conn              *net.UDPConn
	running           atomic.Bool
	subscriberManager *SubscriberManager
	broadcaster       *Broadcaster
	log               *logger.Logger
	bridge            *bridge.Bridge
}

func NewServer(port string, br *bridge.Bridge) *Server {
	log := logger.WithContext("component", "udp_server")
	return &Server{
		Port:              port,
		subscriberManager: NewSubscriberManager(log),
		log:               log,
		bridge:            br,
	}
}

func (s *Server) Start() error {
	addr, err := net.ResolveUDPAddr("udp", ":"+s.Port)
	if err != nil {
		return NewBindError(err)
	}

	s.conn, err = net.ListenUDP("udp", addr)
	if err != nil {
		return NewBindError(err)
	}

	s.broadcaster = NewBroadcaster(s.conn, s.subscriberManager, s.log)
	s.running.Store(true)
	s.subscriberManager.StartCleanup()

	if s.bridge != nil {
		s.bridge.SetUDPBroadcaster(s.broadcaster)
	}

	s.log.Info("udp_server_started", "port", s.Port)
	go s.handlePackets()
	return nil
}

func (s *Server) Stop() error {
	s.running.Store(false)
	s.subscriberManager.Stop()

	if s.conn != nil {
		if err := s.conn.Close(); err != nil {
			s.log.Warn("error_closing_connection", "error", err.Error())
			return err
		}
	}

	s.log.Info("udp_server_stopped")
	return nil
}

func (s *Server) GetSubscriberCount() int {
	return s.subscriberManager.GetSubscriberCount()
}

func (s *Server) handlePackets() {
	buffer := make([]byte, 4096)

	for s.running.Load() {
		n, addr, err := s.conn.ReadFromUDP(buffer)
		if err != nil {
			if s.running.Load() {
				s.log.Warn("read_error", "error", err.Error())
			}
			continue
		}

		if n > 0 {
			// Make a copy of the buffer to avoid race conditions
			data := make([]byte, n)
			copy(data, buffer[:n])
			go s.processPacket(data, addr)
		}
	}
}

func (s *Server) processPacket(data []byte, addr *net.UDPAddr) {
	msg, err := ParseMessage(data)
	if err != nil {
		s.log.Warn("invalid_packet",
			"addr", addr.String(),
			"error", err.Error())
		s.sendError(addr, string(ErrUDPInvalidPacket), "Invalid packet format")
		return
	}

	s.log.Debug("received_packet",
		"type", msg.Type,
		"addr", addr.String())

	switch msg.Type {
	case "register":
		s.handleRegister(addr, msg.Data)
	case "unregister":
		s.handleUnregister(addr)
	case "subscribe":
		s.handleSubscribe(addr, msg.Data)
	case "heartbeat":
		s.handleHeartbeat(addr)
	default:
		s.log.Warn("unknown_message_type",
			"type", msg.Type,
			"addr", addr.String())
		s.sendError(addr, string(ErrUDPInvalidPacket), "Unknown message type")
	}
}

func (s *Server) handleRegister(addr *net.UDPAddr, payload json.RawMessage) {
	var regPayload RegisterPayload
	if err := json.Unmarshal(payload, &regPayload); err != nil {
		s.sendError(addr, string(ErrUDPRegistrationFailed), "Invalid registration payload")
		return
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "your-secret-key-change-this-in-production"
	}

	claims, err := utils.ValidateJWT(regPayload.Token, jwtSecret)
	if err != nil {
		s.log.Warn("authentication_failed",
			"addr", addr.String(),
			"error", err.Error())
		s.sendError(addr, string(ErrUDPAuthFailed), "Authentication failed")
		return
	}

	s.subscriberManager.Subscribe(claims.UserID, addr, []string{"all"})

	s.log.Info("client_registered",
		"user_id", claims.UserID,
		"username", claims.Username,
		"addr", addr.String())

	s.sendSuccess(addr, "Registered successfully")
}

func (s *Server) handleUnregister(addr *net.UDPAddr) {
	userID, exists := s.subscriberManager.GetUserByAddr(addr)
	if !exists {
		s.sendError(addr, string(ErrUDPRegistrationFailed), "Not registered")
		return
	}

	s.subscriberManager.Unsubscribe(addr)

	s.log.Info("client_unregistered",
		"user_id", userID,
		"addr", addr.String())

	s.sendSuccess(addr, "Unregistered successfully")
}

func (s *Server) handleSubscribe(addr *net.UDPAddr, payload json.RawMessage) {
	var subPayload SubscribePayload
	if err := json.Unmarshal(payload, &subPayload); err != nil {
		s.sendError(addr, string(ErrUDPSubscriptionFailed), "Invalid subscription payload")
		return
	}

	validEvents := map[string]bool{
		"all":             true,
		"progress_update": true,
		"library_update":  true,
	}

	for _, eventType := range subPayload.EventTypes {
		if !validEvents[eventType] {
			s.sendError(addr, string(ErrUDPInvalidEventType), "Invalid event type: "+eventType)
			return
		}
	}

	if !s.subscriberManager.UpdateSubscription(addr, subPayload.EventTypes) {
		s.sendError(addr, string(ErrUDPSubscriptionFailed), "Not registered")
		return
	}

	userID, _ := s.subscriberManager.GetUserByAddr(addr)
	s.log.Info("subscription_updated",
		"user_id", userID,
		"addr", addr.String(),
		"event_types", subPayload.EventTypes)

	s.sendSuccess(addr, "Subscription updated successfully")
}

func (s *Server) handleHeartbeat(addr *net.UDPAddr) {
	if !s.subscriberManager.Heartbeat(addr) {
		s.sendError(addr, string(ErrUDPHeartbeatFailed), "Not registered")
		return
	}

	response := CreateSuccessMessage("OK")
	s.conn.WriteToUDP(response, addr)
}

func (s *Server) sendSuccess(addr *net.UDPAddr, message string) {
	response := CreateSuccessMessage(message)
	_, err := s.conn.WriteToUDP(response, addr)
	if err != nil {
		s.log.Warn("failed_to_send_success",
			"addr", addr.String(),
			"error", err.Error())
	}
}

func (s *Server) sendError(addr *net.UDPAddr, code, message string) {
	response := CreateErrorMessage(code, message)
	_, err := s.conn.WriteToUDP(response, addr)
	if err != nil {
		s.log.Warn("failed_to_send_error",
			"addr", addr.String(),
			"error", err.Error())
	}
}
