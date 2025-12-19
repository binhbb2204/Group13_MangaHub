package udp

import (
	"encoding/json"
	"fmt"
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
	bridge            *bridge.UnifiedBridge
	sseBroker         *SSEBroker
}

func NewServer(port string) *Server {
	log := logger.WithContext("component", "udp_server")
	return &Server{
		Port:              port,
		subscriberManager: NewSubscriberManager(log),
		log:               log,
		bridge:            nil,
		sseBroker:         NewSSEBroker(),
	}
}

func (s *Server) SetBridge(b *bridge.UnifiedBridge) {
	s.bridge = b
	if s.broadcaster != nil {
		s.broadcaster.SetBridge(b)
	}
	s.log.Info("udp_server_bridge_set")
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
	if s.bridge != nil {
		s.broadcaster.SetBridge(s.bridge)
		s.bridge.SetUDPBroadcaster(s.broadcaster)
	}

	s.running.Store(true)
	s.subscriberManager.StartCleanup()

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
	case "notification":
		s.handleNotificationForward(msg)
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
		"chapter_release": true,
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

// handleNotificationForward accepts notification messages forwarded from API server
// and broadcasts them to subscribed clients
func (s *Server) handleNotificationForward(msg *Message) {
	if msg.EventType == "" {
		s.log.Warn("invalid_notification_forward", "event_type", msg.EventType)
		return
	}

	if s.broadcaster == nil {
		s.log.Warn("broadcaster_not_initialized")
		return
	}

	// Create unified event from the forwarded notification
	var eventData map[string]interface{}
	if len(msg.Data) > 0 {
		json.Unmarshal(msg.Data, &eventData)
	}

	unifiedEvent := bridge.UnifiedEvent{
		Type: bridge.EventType(msg.EventType),
		Data: eventData,
	}

	if msg.UserID == "" {
		// Global broadcast
		s.broadcaster.BroadcastToAll(bridge.BroadcastEvent{EventType: msg.EventType, Data: eventData})
		s.log.Info("notification_broadcast_all", "event_type", msg.EventType)

		// Also broadcast to SSE clients (frontend)
		s.broadcastToSSE(msg.EventType, eventData)
		return
	}

	// Broadcast to a specific user
	s.broadcaster.BroadcastUnifiedEvent(msg.UserID, unifiedEvent)
	s.log.Info("notification_forwarded_and_broadcast", "user_id", msg.UserID, "event_type", msg.EventType)

	// Also broadcast to SSE clients (frontend)
	s.broadcastToSSE(msg.EventType, eventData)
}

// broadcastToSSE sends notification to frontend SSE clients
func (s *Server) broadcastToSSE(eventType string, data map[string]interface{}) {
	if s.sseBroker == nil {
		return
	}

	var message string
	switch eventType {
	case "manga_created":
		if title, ok := data["title"].(string); ok {
			message = "New manga added: " + title
		} else {
			message = "New manga added"
		}
	case "chapter_release":
		if title, ok := data["title"].(string); ok {
			if delta, ok := data["delta"].(float64); ok {
				message = fmt.Sprintf("%.0f new chapter(s) for %s", delta, title)
			} else {
				message = "New chapters for " + title
			}
		} else {
			message = "New chapters available"
		}
	case "library_update":
		mangaID := ""
		if id, ok := data["manga_id"].(float64); ok {
			mangaID = fmt.Sprintf("%.0f", id)
		} else if id, ok := data["manga_id"].(string); ok {
			mangaID = id
		}

		if action, ok := data["action"].(string); ok {
			switch action {
			case "add":
				if mangaID != "" {
					message = fmt.Sprintf("Added manga #%s to library", mangaID)
				} else {
					message = "Added to your library"
				}
			case "remove":
				if mangaID != "" {
					message = fmt.Sprintf("Removed manga #%s from library", mangaID)
				} else {
					message = "Removed from your library"
				}
			case "update":
				message = "Library updated"
			default:
				message = "Library changed"
			}
		} else {
			message = "Library updated"
		}
	case "progress_update":
		if chapterNum, ok := data["chapter_number"].(float64); ok {
			message = fmt.Sprintf("Progress updated: Chapter %.0f", chapterNum)
		} else {
			message = "Reading progress updated"
		}
	default:
		message = "Notification"
	}

	s.sseBroker.Broadcast(eventType, message, data)
	s.log.Info("notification_sent_to_sse", "event_type", eventType, "clients", s.sseBroker.GetClientCount())
}

// GetSSEBroker returns the SSE broker for HTTP endpoint
func (s *Server) GetSSEBroker() *SSEBroker {
	return s.sseBroker
}
