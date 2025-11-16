# Week 5: UDP Notification System - Implementation Plan

## Table of Contents
1. [Overview](#overview)
2. [Week 4 Status Review](#week-4-status-review)
3. [Week 5 Architecture](#week-5-architecture)
4. [Phase 1: Core UDP Server](#phase-1-core-udp-server)
5. [Phase 2: Client Registration](#phase-2-client-registration)
6. [Phase 3: Notification Broadcasting](#phase-3-notification-broadcasting)
7. [Phase 4: Integration Testing](#phase-4-integration-testing)
8. [Phase 5: Error Handling](#phase-5-error-handling)
9. [Phase 6: Build & Deployment](#phase-6-build--deployment)
10. [Phase 7: Documentation](#phase-7-documentation)
11. [Implementation Checklist](#implementation-checklist)
12. [Success Criteria](#success-criteria)

---

## Overview

**Goal:** Build a UDP-based notification broadcasting system that complements the TCP server by providing lightweight, fire-and-forget notifications for real-time updates across all connected clients.

**Why UDP?**
- Simpler protocol, no handshake overhead
- Fire-and-forget suitable for notifications
- Lower server resource usage
- Complements TCP for dual-mode operation

**Key Features:**
- JWT-based authentication
- Event type subscription filtering
- Multi-device support per user
- Heartbeat-based connection management
- Integration with existing TCP/HTTP bridge

---

## Week 4 Status Review

### ✅ WEEK 4: TCP INTEGRATION & TESTING - COMPLETE

All Week 4 requirements are fully satisfied and production-ready:

#### **1. TCP Server ↔ HTTP API Connection** ✅
- Full bidirectional bridge in `internal/bridge/tcp_http_bridge.go`
- TCP client registration/unregistration working
- Event broadcasting from HTTP to TCP clients functional
- Real-time synchronization implemented

#### **2. Client Connection Testing** ✅
- **12 TCP test files** covering all scenarios
- **4 bridge integration tests** for TCP-HTTP interaction
- Coverage: connectivity, durability, reliability, scalability, performance
- All critical paths tested

#### **3. Error Handling & Logging** ✅
- Structured error system with 5 categories, 20+ error codes
- Comprehensive logging throughout
- Proper error wrapping and client error responses
- JSON error serialization for clients

#### **4. User Progress Integration** ✅
- Complete handlers: `syncProgress`, `getProgress`, `getLibrary`
- Direct database integration with proper error handling
- Event broadcasting triggers on progress updates
- Session management tracking sync state
- Heartbeat system monitoring connection health

**Status:** ✅ **NO GAPS - WEEK 4 IS COMPLETE AND READY**

---

## Week 5 Architecture

### System Flow Diagram

```
┌─────────────────────────────────────────────────────────────┐
│                    UDP Notification Flow                     │
├─────────────────────────────────────────────────────────────┤
│                                                               │
│  HTTP API Request  →  Bridge  →  UDP Broadcaster            │
│                         ↓                ↓                    │
│                    TCP Clients    UDP Clients (Multicast)    │
│                                                               │
└─────────────────────────────────────────────────────────────┘
```

### Component Architecture

```
┌──────────────────────────────────────────────────────────────┐
│                      UDP Server Layer                         │
├──────────────────────────────────────────────────────────────┤
│                                                                │
│  ┌────────────┐  ┌──────────────┐  ┌────────────────┐       │
│  │   Server   │  │ Subscription │  │  Broadcaster   │       │
│  │            │→ │   Manager    │→ │                │       │
│  └────────────┘  └──────────────┘  └────────────────┘       │
│        ↓                 ↓                   ↓                │
│  ┌────────────────────────────────────────────────┐         │
│  │              Protocol Layer                      │         │
│  │  (REGISTER, HEARTBEAT, NOTIFICATION, etc.)      │         │
│  └────────────────────────────────────────────────┘         │
│                          ↓                                    │
│                 UDP Socket (Port 9091)                        │
└──────────────────────────────────────────────────────────────┘
```

---

## Phase 1: Core UDP Server

### File Structure to Create

```
internal/udp/
├── server.go              # Main UDP server (~200 lines)
├── client.go              # UDP client management (~100 lines)
├── protocol.go            # UDP message protocol (~150 lines)
├── broadcaster.go         # Notification broadcasting (~180 lines)
├── subscription.go        # Client subscription management (~200 lines)
├── errors.go              # UDP-specific errors (~100 lines)
└── test/
    ├── server_test.go
    ├── broadcaster_test.go
    ├── subscription_test.go
    ├── protocol_test.go
    ├── client_test.go
    └── integration_test.go

cmd/udp-server/
└── main.go                # UDP server entry point (~80 lines)
```

### 1.1 UDP Server (`internal/udp/server.go`)

**Core Structure:**
```go
type Server struct {
    Port              string
    conn              *net.UDPConn
    running           atomic.Bool
    subscriberManager *SubscriberManager
    broadcaster       *Broadcaster
    log               *logger.Logger
    bridge            *bridge.Bridge
}

// Key Methods:
// - Start() error               // Start UDP listener
// - Stop() error                // Graceful shutdown
// - handlePacket()              // Process incoming UDP packets
// - registerClient()            // Handle client registration
```

**Key Responsibilities:**
- Listen on UDP port (default 9091)
- Parse incoming packets
- Route messages to appropriate handlers
- Manage server lifecycle (start/stop)
- Integration with bridge for event propagation

### 1.2 Subscriber Manager (`internal/udp/subscription.go`)

**Core Structure:**
```go
type Subscriber struct {
    UserID       string
    Addr         *net.UDPAddr
    EventTypes   []string        // Filter: ["progress", "library", "all"]
    RegisteredAt time.Time
    LastSeen     time.Time
}

type SubscriberManager struct {
    subscribers map[string][]*Subscriber  // UserID -> [Subscribers]
    addrToUser  map[string]string         // Address -> UserID
    mu          sync.RWMutex
}

// Key Methods:
// - Subscribe(userID, addr, eventTypes)     // Register subscriber
// - Unsubscribe(addr)                        // Remove subscriber
// - GetSubscribers(userID, eventType)        // Get filtered subscribers
// - Heartbeat(addr)                          // Update last seen
// - CleanupStale()                           // Remove inactive subscribers
```

**Key Responsibilities:**
- Track active UDP clients by user ID
- Support multiple devices per user
- Filter subscribers by event type
- Heartbeat tracking (2-minute timeout)
- Automatic cleanup of stale connections

### 1.3 Broadcaster (`internal/udp/broadcaster.go`)

**Core Structure:**
```go
type Broadcaster struct {
    conn      *net.UDPConn
    subMgr    *SubscriberManager
    log       *logger.Logger
    eventChan chan BroadcastEvent
}

type BroadcastEvent struct {
    UserID    string
    EventType string
    Data      interface{}
}

// Key Methods:
// - BroadcastToUser(userID, event)          // Send to specific user
// - BroadcastToAll(event)                    // Global broadcast
// - processEvents()                          // Event queue processor
```

**Key Responsibilities:**
- Send notifications to UDP clients
- Handle broadcast to multiple devices
- Non-blocking send (log failures, don't block)
- Event queue processing

### 1.4 Protocol (`internal/udp/protocol.go`)

**Message Types:**
```
1. REGISTER    - Client registration with auth token
2. UNREGISTER  - Client deregistration
3. HEARTBEAT   - Keep-alive ping
4. SUBSCRIBE   - Subscribe to event types
5. NOTIFICATION - Server-to-client notification
```

**Message Structure:**
```json
{
    "type": "notification",
    "event_type": "progress_update",
    "user_id": "user123",
    "data": { 
        "manga_id": "manga-1",
        "chapter_id": 42,
        "status": "reading"
    },
    "timestamp": "2025-11-12T10:30:00Z"
}
```

**Key Message Definitions:**
```go
type Message struct {
    Type      string          `json:"type"`
    EventType string          `json:"event_type,omitempty"`
    UserID    string          `json:"user_id,omitempty"`
    Data      json.RawMessage `json:"data,omitempty"`
    Timestamp string          `json:"timestamp"`
}

type RegisterPayload struct {
    Token string `json:"token"`
}

type SubscribePayload struct {
    EventTypes []string `json:"event_types"` // ["progress", "library", "all"]
}

type HeartbeatPayload struct {
    ClientID string `json:"client_id"`
}

type NotificationPayload struct {
    MangaID   string `json:"manga_id,omitempty"`
    ChapterID int    `json:"chapter_id,omitempty"`
    Status    string `json:"status,omitempty"`
    Action    string `json:"action,omitempty"`
}
```

---

## Phase 2: Client Registration

### 2.1 Registration Flow

```
Client                    UDP Server               Bridge
  |                          |                       |
  |--REGISTER(token)-------->|                       |
  |                          |--Validate JWT-------->|
  |                          |                       |
  |<---REGISTER_SUCCESS------|                       |
  |                          |                       |
  |--SUBSCRIBE(events)------>|                       |
  |<---SUBSCRIBE_SUCCESS-----|                       |
  |                          |                       |
  |--HEARTBEAT (every 30s)-->|                       |
  |<---HEARTBEAT_ACK---------|                       |
```

### 2.2 Registration Handler Implementation

```go
func handleRegister(server *Server, addr *net.UDPAddr, msg RegisterMessage) {
    // 1. Validate JWT token
    jwtSecret := os.Getenv("JWT_SECRET")
    if jwtSecret == "" {
        jwtSecret = "your-secret-key-change-this-in-production"
    }
    
    claims, err := utils.ValidateJWT(msg.Token, jwtSecret)
    if err != nil {
        sendError(addr, "Invalid token")
        return
    }
    
    // 2. Create subscriber
    subscriber := &Subscriber{
        UserID:       claims.UserID,
        Addr:         addr,
        EventTypes:   []string{"all"},  // Default to all events
        RegisteredAt: time.Now(),
        LastSeen:     time.Now(),
    }
    
    // 3. Register with manager
    server.subscriberManager.Subscribe(subscriber)
    
    // 4. Send success response
    sendSuccess(addr, "Registered successfully")
}
```

### 2.3 Subscription Management

**Subscribe Handler:**
```go
func handleSubscribe(server *Server, addr *net.UDPAddr, msg SubscribeMessage) {
    // Validate event types
    validEvents := map[string]bool{
        "all":             true,
        "progress_update": true,
        "library_update":  true,
    }
    
    for _, eventType := range msg.EventTypes {
        if !validEvents[eventType] {
            sendError(addr, fmt.Sprintf("Invalid event type: %s", eventType))
            return
        }
    }
    
    // Update subscription
    server.subscriberManager.UpdateSubscription(addr, msg.EventTypes)
    sendSuccess(addr, "Subscribed successfully")
}
```

### 2.4 Heartbeat & Cleanup

**Heartbeat Handler:**
```go
func handleHeartbeat(server *Server, addr *net.UDPAddr) {
    server.subscriberManager.Heartbeat(addr)
    sendHeartbeatAck(addr)
}
```

**Automatic Cleanup:**
```go
func (sm *SubscriberManager) CleanupStale() {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    
    for range ticker.C {
        sm.mu.Lock()
        now := time.Now()
        
        for userID, subscribers := range sm.subscribers {
            filtered := []*Subscriber{}
            for _, sub := range subscribers {
                if now.Sub(sub.LastSeen) <= 2*time.Minute {
                    filtered = append(filtered, sub)
                } else {
                    sm.log.Info("removed_stale_subscriber",
                        "user_id", sub.UserID,
                        "addr", sub.Addr.String(),
                        "inactive_duration", now.Sub(sub.LastSeen))
                }
            }
            
            if len(filtered) > 0 {
                sm.subscribers[userID] = filtered
            } else {
                delete(sm.subscribers, userID)
            }
        }
        sm.mu.Unlock()
    }
}
```

---

## Phase 3: Notification Broadcasting

### 3.1 Integration with Existing Bridge

**Update `internal/bridge/tcp_http_bridge.go`:**

```go
type Bridge struct {
    logger         *logger.Logger
    clients        map[string][]*TCPClient
    udpBroadcaster *udp.Broadcaster  // ADD THIS
    clientsLock    sync.RWMutex
    eventChan      chan Event
    stopChan       chan struct{}
}

// Update constructor
func NewBridge(log *logger.Logger) *Bridge {
    return &Bridge{
        logger:         log,
        clients:        make(map[string][]*TCPClient),
        udpBroadcaster: nil,  // Will be set later
        eventChan:      make(chan Event, 100),
        stopChan:       make(chan struct{}),
    }
}

// Add setter for UDP broadcaster
func (b *Bridge) SetUDPBroadcaster(broadcaster *udp.Broadcaster) {
    b.udpBroadcaster = broadcaster
}

// Modify NotifyProgressUpdate
func (b *Bridge) NotifyProgressUpdate(event ProgressUpdateEvent) {
    data := map[string]interface{}{
        "manga_id":       event.MangaID,
        "chapter_id":     event.ChapterID,
        "status":         event.Status,
        "last_read_date": event.LastReadDate,
    }

    b.eventChan <- Event{
        Type:      EventTypeProgressUpdate,
        UserID:    event.UserID,
        Data:      data,
        Timestamp: event.LastReadDate,
    }

    b.logger.Debug("progress_update_queued",
        "user_id", event.UserID,
        "manga_id", event.MangaID,
        "chapter_id", event.ChapterID,
    )
    
    // ADD: UDP broadcast
    if b.udpBroadcaster != nil {
        b.udpBroadcaster.BroadcastToUser(event.UserID, udp.BroadcastEvent{
            EventType: "progress_update",
            Data:      data,
        })
    }
}

// Modify NotifyLibraryUpdate
func (b *Bridge) NotifyLibraryUpdate(event LibraryUpdateEvent) {
    data := map[string]interface{}{
        "manga_id": event.MangaID,
        "action":   event.Action,
    }

    b.eventChan <- Event{
        Type:   EventTypeLibraryUpdate,
        UserID: event.UserID,
        Data:   data,
    }

    b.logger.Debug("library_update_queued",
        "user_id", event.UserID,
        "manga_id", event.MangaID,
        "action", event.Action,
    )
    
    // ADD: UDP broadcast
    if b.udpBroadcaster != nil {
        b.udpBroadcaster.BroadcastToUser(event.UserID, udp.BroadcastEvent{
            EventType: "library_update",
            Data:      data,
        })
    }
}
```

### 3.2 Notification Types

**1. Progress Update Notification:**
```json
{
    "type": "notification",
    "event_type": "progress_update",
    "user_id": "user123",
    "data": {
        "manga_id": "manga-1",
        "chapter_id": 42,
        "status": "reading",
        "last_read_date": "2025-11-12T10:30:00Z"
    },
    "timestamp": "2025-11-12T10:30:00Z"
}
```

**2. Library Update Notification:**
```json
{
    "type": "notification",
    "event_type": "library_update",
    "user_id": "user123",
    "data": {
        "manga_id": "manga-1",
        "action": "added"
    },
    "timestamp": "2025-11-12T10:30:00Z"
}
```

**3. New Chapter Alert (Future):**
```json
{
    "type": "notification",
    "event_type": "new_chapter",
    "user_id": "user123",
    "data": {
        "manga_id": "manga-1",
        "chapter": 43,
        "title": "Chapter 43: New Adventure"
    },
    "timestamp": "2025-11-12T10:30:00Z"
}
```

### 3.3 Broadcasting Logic Implementation

```go
func (b *Broadcaster) BroadcastToUser(userID string, event BroadcastEvent) {
    // Get subscribers for this user filtered by event type
    subscribers := b.subMgr.GetSubscribers(userID, event.EventType)
    
    if len(subscribers) == 0 {
        b.log.Debug("no_udp_subscribers",
            "user_id", userID,
            "event_type", event.EventType)
        return
    }
    
    // Create notification message
    notification := Message{
        Type:      "notification",
        EventType: event.EventType,
        UserID:    userID,
        Data:      mustMarshal(event.Data),
        Timestamp: time.Now().Format(time.RFC3339),
    }
    
    messageBytes := mustMarshal(notification)
    
    successCount := 0
    failCount := 0
    
    // Send to all subscribers (non-blocking)
    for _, sub := range subscribers {
        _, err := b.conn.WriteToUDP(messageBytes, sub.Addr)
        if err != nil {
            failCount++
            b.log.Warn("broadcast_failed",
                "user_id", userID,
                "addr", sub.Addr.String(),
                "error", err.Error())
        } else {
            successCount++
        }
    }
    
    b.log.Info("udp_broadcast_complete",
        "user_id", userID,
        "event_type", event.EventType,
        "success_count", successCount,
        "fail_count", failCount)
}
```

---

## Phase 4: Integration Testing

### 4.1 Test Suite Structure

**A. Unit Tests:**

**`internal/udp/test/server_test.go`:**
```go
- TestServerStartStop
- TestServerStartWithInvalidPort
- TestHandleRegisterValidToken
- TestHandleRegisterInvalidToken
- TestHandleUnregister
- TestHandleHeartbeat
- TestConcurrentPacketHandling
```

**`internal/udp/test/subscription_test.go`:**
```go
- TestSubscribe
- TestUnsubscribe
- TestGetSubscribersFiltered
- TestMultipleSubscribersPerUser
- TestHeartbeatUpdate
- TestCleanupStale
- TestConcurrentSubscriptionOperations
```

**`internal/udp/test/broadcaster_test.go`:**
```go
- TestBroadcastToUser
- TestBroadcastToUserMultipleDevices
- TestBroadcastWithEventTypeFilter
- TestBroadcastToNonexistentUser
- TestBroadcastFailureHandling
- TestEventQueueProcessing
```

**`internal/udp/test/protocol_test.go`:**
```go
- TestParseRegisterMessage
- TestParseSubscribeMessage
- TestParseHeartbeatMessage
- TestCreateNotificationMessage
- TestInvalidMessageFormat
- TestMessageMarshaling
```

**`internal/udp/test/client_test.go`:**
```go
- TestClientManagerAdd
- TestClientManagerRemove
- TestClientManagerGetByUserID
- TestClientManagerConcurrentAccess
```

### 4.2 Integration Tests

**`internal/udp/test/integration_test.go`:**

```go
func TestUDPNotificationFlow(t *testing.T) {
    // 1. Start UDP server
    server := udp.NewServer("19091", nil)
    err := server.Start()
    require.NoError(t, err)
    defer server.Stop()
    
    // 2. Create UDP client
    conn, err := net.DialUDP("udp", nil, &net.UDPAddr{
        IP:   net.ParseIP("127.0.0.1"),
        Port: 19091,
    })
    require.NoError(t, err)
    defer conn.Close()
    
    // 3. Register with JWT
    token, _ := utils.GenerateJWT("user1", "testuser", jwtSecret)
    registerMsg := createRegisterMessage(token)
    conn.Write(registerMsg)
    
    // 4. Verify registration success
    buf := make([]byte, 1024)
    n, _ := conn.Read(buf)
    response := parseMessage(buf[:n])
    assert.Equal(t, "success", response.Type)
    
    // 5. Subscribe to progress_update events
    subscribeMsg := createSubscribeMessage([]string{"progress_update"})
    conn.Write(subscribeMsg)
    
    // 6. Trigger progress update
    server.broadcaster.BroadcastToUser("user1", udp.BroadcastEvent{
        EventType: "progress_update",
        Data: map[string]interface{}{
            "manga_id":   "manga-1",
            "chapter_id": 42,
            "status":     "reading",
        },
    })
    
    // 7. Verify notification received
    n, _ = conn.Read(buf)
    notification := parseMessage(buf[:n])
    assert.Equal(t, "notification", notification.Type)
    assert.Equal(t, "progress_update", notification.EventType)
}

func TestMultipleClientsNotification(t *testing.T) {
    // Test that multiple devices for same user all receive notifications
}

func TestSubscriptionFiltering(t *testing.T) {
    // Test that clients only receive notifications for subscribed event types
}

func TestStaleClientCleanup(t *testing.T) {
    // Test that clients without heartbeat are removed after 2 minutes
}

func TestHighVolumeNotifications(t *testing.T) {
    // Test server handling 1000+ notifications/second
}
```

**`internal/bridge/test/udp_tcp_integration_test.go`:**

```go
func TestBridgeBroadcastsBothTCPandUDP(t *testing.T) {
    // 1. Setup bridge with both TCP and UDP
    // 2. Connect TCP client
    // 3. Register UDP client
    // 4. Trigger progress update via HTTP API
    // 5. Verify both TCP and UDP clients receive notification
}

func TestUDPBroadcastFromHTTPAPI(t *testing.T) {
    // 1. Setup complete stack (HTTP + Bridge + UDP)
    // 2. Register UDP client
    // 3. Make HTTP POST to /user/progress
    // 4. Verify UDP notification sent
}
```

### 4.3 Critical Test Scenarios Checklist

- [ ] **Authentication:**
  - [ ] Valid JWT registration succeeds
  - [ ] Invalid JWT registration fails
  - [ ] Expired JWT registration fails
  
- [ ] **Subscription:**
  - [ ] Subscribe to specific event types
  - [ ] Subscribe to all events
  - [ ] Change subscription
  - [ ] Filter works correctly
  
- [ ] **Broadcasting:**
  - [ ] Single user, multiple devices all receive
  - [ ] Only subscribed users receive notifications
  - [ ] Event type filtering works
  - [ ] Failed sends don't block others
  
- [ ] **Reliability:**
  - [ ] Heartbeat keeps clients alive
  - [ ] Stale clients cleaned up (>2 min)
  - [ ] High volume handling (1000+ msg/sec)
  - [ ] Malformed packets don't crash server
  
- [ ] **Integration:**
  - [ ] Bridge broadcasts to TCP and UDP
  - [ ] HTTP API → UDP notification flow
  - [ ] TCP server → UDP notification flow
  - [ ] End-to-end user scenarios work

---

## Phase 5: Error Handling

### 5.1 UDP-Specific Error Codes

```go
// internal/udp/errors.go

type UDPErrorCode string

const (
    ErrUDPBindFailed         UDPErrorCode = "UDP-001"
    ErrUDPInvalidPacket      UDPErrorCode = "UDP-002"
    ErrUDPRegistrationFailed UDPErrorCode = "UDP-003"
    ErrUDPAuthFailed         UDPErrorCode = "UDP-004"
    ErrUDPBroadcastFailed    UDPErrorCode = "UDP-005"
    ErrUDPSubscriptionFailed UDPErrorCode = "UDP-006"
    ErrUDPHeartbeatFailed    UDPErrorCode = "UDP-007"
    ErrUDPInvalidEventType   UDPErrorCode = "UDP-008"
)

type UDPError struct {
    Code      UDPErrorCode `json:"code"`
    Message   string       `json:"message"`
    Cause     error        `json:"-"`
    Timestamp string       `json:"timestamp"`
}

func (e *UDPError) Error() string {
    if e.Cause != nil {
        return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
    }
    return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func NewUDPError(code UDPErrorCode, message string, cause error) *UDPError {
    return &UDPError{
        Code:      code,
        Message:   message,
        Cause:     cause,
        Timestamp: time.Now().Format(time.RFC3339),
    }
}

// Specific error constructors
func NewBindError(cause error) *UDPError {
    return NewUDPError(ErrUDPBindFailed, "Failed to bind UDP port", cause)
}

func NewAuthError() *UDPError {
    return NewUDPError(ErrUDPAuthFailed, "Authentication failed", nil)
}

func NewInvalidPacketError(cause error) *UDPError {
    return NewUDPError(ErrUDPInvalidPacket, "Invalid packet format", cause)
}
```

### 5.2 Error Response Format

**Client Error Response:**
```json
{
    "type": "error",
    "code": "UDP-004",
    "message": "Authentication failed: invalid token",
    "timestamp": "2025-11-12T10:30:00Z"
}
```

### 5.3 Error Handling Strategy

**Server-Side Logging:**
```go
func handleError(server *Server, addr *net.UDPAddr, err *UDPError) {
    // Log the error
    server.log.Error("udp_error",
        "code", string(err.Code),
        "message", err.Message,
        "addr", addr.String(),
        "cause", err.Cause)
    
    // Send error response to client
    errorMsg := createErrorMessage(err)
    server.conn.WriteToUDP(errorMsg, addr)
}
```

**Client-Side Handling:**
- Parse error responses
- Display user-friendly messages
- Implement retry logic for transient errors
- Fall back to TCP/HTTP for critical operations

---

## Phase 6: Build & Deployment

### 6.1 Build Commands

**Individual Build:**
```powershell
# Build UDP server
go build -o bin\udp-server.exe .\cmd\udp-server
```

**Build All Servers:**
```powershell
go build -o bin\api-server.exe .\cmd\api-server
go build -o bin\tcp-server.exe .\cmd\tcp-server
go build -o bin\udp-server.exe .\cmd\udp-server
```

**Run Tests:**
```powershell
# Test UDP package
go test ./internal/udp/... -v

# Test with coverage
go test ./internal/udp/... -cover -coverprofile=coverage.out

# View coverage
go tool cover -html=coverage.out
```

### 6.2 Environment Configuration

**`.env` File:**
```env
# Server Ports
API_PORT=8080
TCP_PORT=9090
UDP_PORT=9091

# Logging
LOG_LEVEL=info
LOG_FORMAT=json

# Database
DB_PATH=./data/mangahub.db

# Authentication
JWT_SECRET=your-super-secret-jwt-key

# MyAnimeList API
MAL_CLIENT_ID=your_mal_client_id
```

### 6.3 Startup Sequence

**Development Mode:**
```powershell
# Terminal 1: API Server
.\bin\api-server.exe

# Terminal 2: TCP Server
.\bin\tcp-server.exe

# Terminal 3: UDP Server
.\bin\udp-server.exe
```

**Production Mode (Docker Compose):**
```yaml
# docker-compose.yml
services:
  api-server:
    # ... existing config
  
  tcp-server:
    # ... existing config
  
  udp-server:
    build:
      context: ./cmd/udp-server
    ports:
      - "9091:9091/udp"
    environment:
      - UDP_PORT=9091
      - JWT_SECRET=${JWT_SECRET}
      - LOG_LEVEL=info
```

### 6.4 UDP Server Entry Point

**`cmd/udp-server/main.go`:**
```go
package main

import (
    "os"
    "os/signal"
    "syscall"
    
    "github.com/binhbb2204/Manga-Hub-Group13/internal/bridge"
    "github.com/binhbb2204/Manga-Hub-Group13/internal/udp"
    "github.com/binhbb2204/Manga-Hub-Group13/pkg/logger"
    "github.com/joho/godotenv"
)

func main() {
    // Load environment variables
    _ = godotenv.Load()
    
    // Initialize logger
    logLevel := logger.INFO
    if level := os.Getenv("LOG_LEVEL"); level != "" {
        logLevel = logger.LogLevel(level)
    }
    jsonFormat := os.Getenv("LOG_FORMAT") == "json"
    logger.Init(logLevel, jsonFormat, os.Stdout)
    
    log := logger.GetLogger().WithContext("component", "udp_main")
    log.Info("starting_udp_server", "version", "1.0.0")
    
    // Get UDP port
    port := os.Getenv("UDP_PORT")
    if port == "" {
        port = "9091"
        log.Warn("using_default_port", "port", port)
    }
    
    // Create bridge (can be shared with TCP if needed)
    udpBridge := bridge.NewBridge(logger.WithContext("component", "bridge"))
    udpBridge.Start()
    defer udpBridge.Stop()
    
    // Create and start UDP server
    server := udp.NewServer(port, udpBridge)
    if err := server.Start(); err != nil {
        log.Error("failed_to_start_udp_server",
            "error", err.Error(),
            "port", port)
        os.Exit(1)
    }
    defer server.Stop()
    
    log.Info("udp_server_running",
        "port", port,
        "pid", os.Getpid())
    
    // Wait for interrupt signal
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
    sig := <-sigChan
    
    log.Info("shutting_down_udp_server",
        "signal", sig.String())
}
```

---

## Phase 7: Documentation

### 7.1 Documentation Files to Create

**1. `docs/UDP_Guide.md`** - User guide for UDP notifications
**2. `docs/UDP_Protocol.md`** - Technical protocol specification
**3. `docs/UDP_Testing.md`** - Testing procedures
**4. `docs/UDP_Client_Examples.md`** - Client implementation examples

### 7.2 UDP_Guide.md Outline

```markdown
# UDP Notification System Guide

## Overview
- What is the UDP notification system?
- When to use UDP vs TCP
- System requirements

## Getting Started
- Server setup
- Client registration
- Receiving notifications

## Client Implementation
- JavaScript/Node.js example
- Python example
- Go example

## Event Types
- progress_update
- library_update
- Future event types

## Best Practices
- Heartbeat frequency
- Error handling
- Fallback strategies

## Troubleshooting
- Common issues
- Debug logging
- Testing connection
```

---

## Implementation Checklist

### Phase 1: Core UDP Server ✅
- [ ] Create `internal/udp/` directory structure
- [ ] Implement `protocol.go` with message definitions
- [ ] Implement `errors.go` with UDP error codes
- [ ] Implement `subscription.go` with subscriber management
- [ ] Implement `broadcaster.go` with notification logic
- [ ] Implement `server.go` with UDP server core
- [ ] Implement `client.go` with client management

### Phase 2: Client Registration ✅
- [ ] Implement registration handler with JWT validation
- [ ] Implement subscription handler
- [ ] Implement heartbeat handler
- [ ] Implement automatic stale client cleanup
- [ ] Test registration flow end-to-end

### Phase 3: Notification Broadcasting ✅
- [ ] Update `bridge.go` to integrate UDP broadcaster
- [ ] Implement `BroadcastToUser` method
- [ ] Implement event type filtering
- [ ] Test notification delivery
- [ ] Test multi-device scenarios

### Phase 4: Integration Testing ✅
- [ ] Write unit tests for server
- [ ] Write unit tests for subscription manager
- [ ] Write unit tests for broadcaster
- [ ] Write unit tests for protocol
- [ ] Write integration tests
- [ ] Write bridge integration tests
- [ ] Achieve >80% test coverage

### Phase 5: Error Handling ✅
- [ ] Define all UDP error codes
- [ ] Implement error response format
- [ ] Add error logging throughout
- [ ] Test error scenarios

### Phase 6: Build & Deployment ✅
- [ ] Create `cmd/udp-server/main.go`
- [ ] Test build process
- [ ] Update Docker configuration
- [ ] Create startup scripts
- [ ] Test all servers running together

### Phase 7: Documentation ✅
- [ ] Write UDP_Guide.md
- [ ] Write UDP_Protocol.md
- [ ] Write UDP_Testing.md
- [ ] Write client examples
- [ ] Update main README.md

---

## Success Criteria

### Functional Requirements ✅
- [ ] UDP server starts on configurable port
- [ ] Clients can register with JWT authentication
- [ ] Heartbeat mechanism maintains connections
- [ ] Stale clients are automatically removed (>2 min)
- [ ] Notifications reach all user devices
- [ ] Event type filtering works correctly
- [ ] Integration with bridge is seamless
- [ ] HTTP API triggers UDP notifications

### Non-Functional Requirements ✅
- [ ] Handle 1000+ concurrent subscribers
- [ ] Process 1000+ notifications/second
- [ ] Test coverage >80%
- [ ] Zero downtime deployment support
- [ ] Comprehensive error handling
- [ ] Structured logging throughout
- [ ] Documentation complete

### Integration Requirements ✅
- [ ] Works alongside TCP server
- [ ] Bridge broadcasts to both TCP and UDP
- [ ] HTTP API integration verified
- [ ] Database operations don't block UDP
- [ ] Graceful shutdown doesn't lose subscribers

---

## Implementation Effort Estimate

| Phase | Files | Lines of Code | Estimated Time |
|-------|-------|---------------|----------------|
| Phase 1: Core Server | 7 files | ~1,000 LOC | 8-10 hours |
| Phase 2: Registration | Add to Phase 1 | ~300 LOC | 3-4 hours |
| Phase 3: Broadcasting | 1 file + updates | ~200 LOC | 3-4 hours |
| Phase 4: Testing | 6 test files | ~800 LOC | 6-8 hours |
| Phase 5: Error Handling | Integrated | ~100 LOC | 2 hours |
| Phase 6: Integration | Updates | ~150 LOC | 2-3 hours |
| Phase 7: Documentation | 4 docs | - | 2-3 hours |
| **TOTAL** | **~17 files** | **~2,550 LOC** | **26-34 hours** |

---

## Next Steps for Implementation

### Recommended Order:
1. ✅ **Start with Protocol** (`protocol.go`) - Define message structures
2. ✅ **Build Error Handling** (`errors.go`) - UDP-specific errors
3. ✅ **Implement Subscription Manager** (`subscription.go`) - Core state management
4. ✅ **Implement Broadcaster** (`broadcaster.go`) - Notification delivery
5. ✅ **Implement Server** (`server.go`) - UDP listener and packet handler
6. ✅ **Add Client Management** (`client.go`) - Client tracking
7. ✅ **Update Bridge** - Integrate with existing TCP/HTTP bridge
8. ✅ **Write Tests** - Unit and integration tests
9. ✅ **Create Main Entry Point** (`cmd/udp-server/main.go`)
10. ✅ **Document** - User guides and protocol specs

---

## Key Design Decisions

### 1. Why UDP over WebSockets?
- Simpler protocol, no handshake overhead
- Fire-and-forget suitable for notifications
- Lower server resource usage
- Complements TCP for dual-mode operation

### 2. Subscription Management
- In-memory storage (no persistence needed)
- Heartbeat-based expiry (2-minute timeout)
- Per-user multi-device support
- Event type filtering for efficiency

### 3. Bridge Integration
- Extend existing bridge rather than create new one
- Parallel broadcast to TCP and UDP
- Shared event types and data structures
- Unified logging and metrics

### 4. Security
- JWT authentication on registration
- No sensitive data in UDP packets (by design)
- User ID-based filtering (isolation)
- Rate limiting considerations for future

### 5. Reliability Approach
- UDP is inherently unreliable (accepted trade-off)
- Critical actions still use TCP
- UDP for "nice-to-have" real-time updates
- Client can fall back to polling if UDP fails

---

## References

- **Week 4 Code:** `internal/tcp/`, `internal/bridge/`
- **Testing Examples:** `internal/tcp/test/`, `internal/bridge/test/`
- **Protocol Inspiration:** `internal/tcp/protocol.go`
- **Error Handling Pattern:** `internal/tcp/errors.go`
- **Logger Usage:** `pkg/logger/`
- **JWT Utils:** `pkg/utils/jwt.go`

---

**Document Version:** 1.0  
**Last Updated:** November 12, 2025  
**Status:** Ready for Implementation
