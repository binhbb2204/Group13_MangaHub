package websocket

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/binhbb2204/Manga-Hub-Group13/internal/bridge"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/logger"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/metrics"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/utils"
	"github.com/gorilla/websocket"
)

type Client struct {
	ID          string
	Username    string
	Conn        *websocket.Conn
	Send        chan []byte
	Manager     *Manager
	Handler     *Handler
	LastActive  time.Time
	ConnectedAt time.Time
	rateTokens  int
	rateLast    time.Time
	mu          sync.Mutex
}

type Manager struct {
	clients    map[string]*Client
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
	rooms      map[string]map[*Client]struct{}
}

func NewManager() *Manager {
	return &Manager{
		clients:    make(map[string]*Client),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		rooms:      make(map[string]map[*Client]struct{}),
	}
}

func (m *Manager) Run() {
	for {
		select {
		case client := <-m.register:
			m.mu.Lock()
			m.clients[client.ID] = client
			client.rateTokens = 20
			client.rateLast = time.Now()
			if _, ok := m.rooms["global"]; !ok {
				m.rooms["global"] = make(map[*Client]struct{})
			}
			m.rooms["global"][client] = struct{}{}
			metrics.SetActiveConnections(int64(len(m.clients)))
			m.mu.Unlock()

		case client := <-m.unregister:
			m.mu.Lock()
			if _, ok := m.clients[client.ID]; ok {
				delete(m.clients, client.ID)
				close(client.Send)
			}
			metrics.SetActiveConnections(int64(len(m.clients)))
			affectedRooms := []string{}
			for room, set := range m.rooms {
				if _, ok := set[client]; ok {
					affectedRooms = append(affectedRooms, room)
					delete(set, client)
				}
				if len(set) == 0 {
					delete(m.rooms, room)
				}
			}
			m.mu.Unlock()

			// Only broadcast leave message if connection lasted more than 2 seconds
			// This suppresses spam from quick send commands
			connectionDuration := time.Since(client.ConnectedAt)
			if connectionDuration > 2*time.Second {
				for _, room := range affectedRooms {
					id, _ := utils.GenerateID(16)
					leaveMsg := ServerMessage{ID: id, Type: MessageTypePresence, From: "system", Room: room, Content: client.Username + " left the chat", Timestamp: time.Now()}
					if data, err := json.Marshal(leaveMsg); err == nil {
						m.broadcastRoom(room, data)
					}
				}
			}

		case message := <-m.broadcast:
			m.mu.RLock()
			for _, client := range m.clients {
				select {
				case client.Send <- message:
				default:
					close(client.Send)
					delete(m.clients, client.ID)
				}
			}
			m.mu.RUnlock()
		}
	}
}

func (m *Manager) joinRoom(c *Client, room string) {
	if room == "" {
		room = "global"
	}
	m.mu.Lock()
	// When joining a non-global room, remove from global to avoid cross-room message leakage
	if room != "global" {
		if globalSet, ok := m.rooms["global"]; ok {
			delete(globalSet, c)
			if len(globalSet) == 0 {
				delete(m.rooms, "global")
			}
		}
	}
	if _, ok := m.rooms[room]; !ok {
		m.rooms[room] = make(map[*Client]struct{})
	}
	m.rooms[room][c] = struct{}{}
	m.mu.Unlock()
}

func (m *Manager) broadcastRoom(room string, message []byte) {
	if room == "" {
		room = "global"
	}
	m.mu.RLock()
	set, ok := m.rooms[room]
	if !ok {
		m.mu.RUnlock()
		return
	}
	for c := range set {
		select {
		case c.Send <- message:
		default:
			close(c.Send)
			delete(m.clients, c.ID)
			delete(set, c)
		}
	}
	m.mu.RUnlock()
}
func (m *Manager) GetClient(userID string) (*Client, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	client, ok := m.clients[userID]
	return client, ok
}

func (m *Manager) GetActiveUsers() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	users := make([]string, 0, len(m.clients))
	for id := range m.clients {
		users = append(users, id)
	}
	return users
}

func (m *Manager) GetRoomUsers(room string) []map[string]interface{} {
	if room == "" {
		room = "global"
	}
	m.mu.RLock()
	defer m.mu.RUnlock()

	roomClients, ok := m.rooms[room]
	if !ok {
		return []map[string]interface{}{}
	}

	users := make([]map[string]interface{}, 0, len(roomClients))
	for client := range roomClients {
		users = append(users, map[string]interface{}{
			"id":       client.ID,
			"username": client.Username,
			"room":     room,
		})
	}
	return users
}

func (m *Manager) GetClientCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.clients)
}

func (m *Manager) GetRoomClientCount(room string) int {
	if room == "" {
		room = "global"
	}
	m.mu.RLock()
	defer m.mu.RUnlock()

	roomClients, ok := m.rooms[room]
	if !ok {
		return 0
	}
	return len(roomClients)
}

func (m *Manager) BroadcastMessage(message []byte) {
	m.broadcast <- message
}

func (m *Manager) SendToUser(userID string, message []byte) bool {
	m.mu.RLock()
	client, ok := m.clients[userID]
	m.mu.RUnlock()

	if !ok {
		return false
	}

	select {
	case client.Send <- message:
		return true
	default:
		return false
	}
}

func (c *Client) ReadPump(connID string, bridge *bridge.UnifiedBridge, broadcaster *WSBroadcaster) {
	defer func() {
		if bridge != nil && connID != "" {
			bridge.UnregisterProtocolClient(connID, c.ID)
			broadcaster.UnregisterConnection(connID)
		}
		c.Manager.unregister <- c
		c.Conn.Close()
	}()

	c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	c.Conn.SetReadLimit(maxMessageSize)
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		c.UpdateActivity()
		return nil
	})

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logger.Error("WebSocket read error", map[string]interface{}{"error": err.Error(), "client_id": c.ID})
			}
			break
		}
		c.UpdateActivity()

		// rate limiting: 20 messages per 10s window
		if !c.consumeRateToken() {
			metrics.IncrementRateLimited()
			id, _ := utils.GenerateID(8)
			msg := ServerMessage{ID: id, Type: MessageTypeSystem, From: "system", Room: "global", Content: "rate limit exceeded", Timestamp: time.Now(), Metadata: map[string]interface{}{"limit": 20}}
			if data, e := json.Marshal(msg); e == nil {
				select {
				case c.Send <- data:
				default:
				}
			}
			continue
		}

		if c.Handler != nil {
			metrics.IncrementMessages()
			if err := c.Handler.HandleClientMessage(c, message); err != nil {
				logger.Error("Failed to handle message", map[string]interface{}{"error": err.Error(), "client_id": c.ID})
			}
		} else {
			c.Manager.broadcast <- message
		}
	}
}

func (c *Client) consumeRateToken() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	if now.Sub(c.rateLast) >= 10*time.Second {
		c.rateTokens = 20
		c.rateLast = now
	}
	if c.rateTokens <= 0 {
		return false
	}
	c.rateTokens--
	return true
}

func (c *Client) UpdateActivity() {
	c.mu.Lock()
	c.LastActive = time.Now()
	c.mu.Unlock()
}

func (c *Client) GetLastActive() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.LastActive
}

func (c *Client) WritePump() {
	defer c.Conn.Close()
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()

	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
