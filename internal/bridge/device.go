package bridge

import (
	"encoding/json"
	"time"
)

type DeviceInfo struct {
	DeviceType    string    `json:"device_type"`
	DeviceName    string    `json:"device_name"`
	SessionID     string    `json:"session_id"`
	ConnectedAt   time.Time `json:"connected_at"`
	LastHeartbeat time.Time `json:"last_heartbeat"`
	IsOnline      bool      `json:"is_online"`
}

type DeviceConnection struct {
	TCPClient
	SessionID     string
	DeviceType    string
	DeviceName    string
	ConnectedAt   time.Time
	LastHeartbeat time.Time
}

func (b *Bridge) GetUserDevices(userID string) []DeviceInfo {
	b.clientsLock.RLock()
	defer b.clientsLock.RUnlock()

	clients, exists := b.clients[userID]
	if !exists {
		return []DeviceInfo{}
	}

	devices := make([]DeviceInfo, 0, len(clients))

	for _, client := range clients {
		deviceInfo := DeviceInfo{
			DeviceType:    "unknown",
			DeviceName:    client.Conn.RemoteAddr().String(),
			SessionID:     "",
			ConnectedAt:   time.Now(),
			LastHeartbeat: time.Now(),
			IsOnline:      true,
		}
		devices = append(devices, deviceInfo)
	}

	return devices
}

func (b *Bridge) GetDeviceCount(userID string) int {
	b.clientsLock.RLock()
	defer b.clientsLock.RUnlock()

	clients, exists := b.clients[userID]
	if !exists {
		return 0
	}

	return len(clients)
}

func (b *Bridge) GetAllDevices() map[string][]DeviceInfo {
	b.clientsLock.RLock()
	defer b.clientsLock.RUnlock()

	result := make(map[string][]DeviceInfo)

	for userID := range b.clients {
		result[userID] = b.GetUserDevices(userID)
	}

	return result
}

func (b *Bridge) BroadcastToUserExcept(userID string, event Event, exceptConnAddr string) {
	b.clientsLock.RLock()
	clients := b.clients[userID]
	b.clientsLock.RUnlock()

	if len(clients) == 0 {
		b.logger.Debug("no_tcp_clients_for_user", "user_id", userID)
		return
	}

	messageBytes, err := json.Marshal(event)
	if err != nil {
		b.logger.Error("failed_to_marshal_event",
			"user_id", userID,
			"error", err.Error(),
		)
		return
	}

	message := string(messageBytes) + "\n"
	sentCount := 0

	for _, client := range clients {
		if client.Conn.RemoteAddr().String() == exceptConnAddr {
			continue
		}

		_, err := client.Conn.Write([]byte(message))
		if err != nil {
			b.logger.Warn("failed_to_send_to_client",
				"user_id", userID,
				"client_addr", client.Conn.RemoteAddr().String(),
				"error", err.Error(),
			)
		} else {
			sentCount++
		}
	}

	b.logger.Debug("event_broadcast_to_devices",
		"user_id", userID,
		"event_type", event.Type,
		"devices_notified", sentCount,
	)
}
