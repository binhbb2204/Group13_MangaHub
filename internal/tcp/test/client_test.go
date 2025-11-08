package tcp_test

import (
	"net"
	"testing"

	"github.com/binhbb2204/Manga-Hub-Group13/internal/tcp"
)

func TestClientManagerAdd(t *testing.T) {
	manager := tcp.NewClientManager()
	client := &tcp.Client{
		ID:   "test-client-1",
		Conn: nil,
	}
	manager.Add(client)
	retrieved, ok := manager.Get("test-client-1")
	if !ok {
		t.Error("Client should exist after Add()")
	}
	if retrieved.ID != "test-client-1" {
		t.Errorf("Expected client ID 'test-client-1', got '%s'", retrieved.ID)
	}
}

func TestClientManagerRemove(t *testing.T) {
	manager := tcp.NewClientManager()
	client := &tcp.Client{
		ID:   "test-client-2",
		Conn: nil,
	}
	manager.Add(client)
	manager.Remove("test-client-2")
	_, ok := manager.Get("test-client-2")
	if ok {
		t.Error("Client should not exist after Remove()")
	}
}

func TestClientManagerList(t *testing.T) {
	manager := tcp.NewClientManager()
	for i := 1; i <= 5; i++ {
		client := &tcp.Client{
			ID:   string(rune('A' + i - 1)),
			Conn: nil,
		}
		manager.Add(client)
	}
	clients := manager.List()
	if len(clients) != 5 {
		t.Errorf("Expected 5 clients, got %d", len(clients))
	}
}

func TestClientManagerConcurrentAccess(t *testing.T) {
	manager := tcp.NewClientManager()
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			client := &tcp.Client{
				ID:   string(rune('A' + id)),
				Conn: nil,
			}
			manager.Add(client)
			done <- true
		}(i)
	}
	for i := 0; i < 10; i++ {
		<-done
	}
	for i := 0; i < 10; i++ {
		go func(id int) {
			manager.Get(string(rune('A' + id)))
			done <- true
		}(i)
	}
	for i := 0; i < 10; i++ {
		<-done
	}
	clients := manager.List()
	if len(clients) != 10 {
		t.Errorf("Expected 10 clients after concurrent adds, got %d", len(clients))
	}
	for i := 0; i < 10; i++ {
		go func(id int) {
			manager.Remove(string(rune('A' + id)))
			done <- true
		}(i)
	}
	for i := 0; i < 10; i++ {
		<-done
	}
	clients = manager.List()
	if len(clients) != 0 {
		t.Errorf("Expected 0 clients after concurrent removes, got %d", len(clients))
	}
}

func TestClientManagerGetNonExistent(t *testing.T) {
	manager := tcp.NewClientManager()
	_, ok := manager.Get("non-existent")
	if ok {
		t.Error("Get() should return false for non-existent client")
	}
}

func TestNewClientManager(t *testing.T) {
	manager := tcp.NewClientManager()
	if manager == nil {
		t.Fatal("NewClientManager() returned nil")
	}
	clients := manager.List()
	if len(clients) != 0 {
		t.Errorf("New ClientManager should have 0 clients, got %d", len(clients))
	}
}

type mockConn struct {
	net.Conn
	closed bool
}

func (m *mockConn) Close() error {
	m.closed = true
	return nil
}

func TestClientWithRealConnection(t *testing.T) {
	manager := tcp.NewClientManager()
	mock := &mockConn{}
	client := &tcp.Client{
		ID:   "mock-client",
		Conn: mock,
	}
	manager.Add(client)
	retrieved, ok := manager.Get("mock-client")
	if !ok {
		t.Error("Client should exist")
	}
	if retrieved.Conn != mock {
		t.Error("Retrieved connection should match the original")
	}
}
