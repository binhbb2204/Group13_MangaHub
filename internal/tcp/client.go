package tcp

import (
	"net"
	"sync"
)

type Client struct {
	Conn          net.Conn
	ID            string
	UserID        string
	Username      string
	Authenticated bool
}

type ClientManager struct {
	clients map[string]*Client
	mu      sync.RWMutex
}

func NewClientManager() *ClientManager {
	return &ClientManager{
		clients: make(map[string]*Client),
	}
}

func (cm *ClientManager) Add(client *Client) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.clients[client.ID] = client
}

func (cm *ClientManager) Remove(id string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	delete(cm.clients, id)
}

func (cm *ClientManager) Get(id string) (*Client, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	c, ok := cm.clients[id]
	return c, ok
}

func (cm *ClientManager) List() []*Client {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	clients := make([]*Client, 0, len(cm.clients))
	for _, c := range cm.clients {
		clients = append(clients, c)
	}
	return clients
}
