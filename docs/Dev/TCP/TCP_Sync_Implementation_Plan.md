# TCP Sync Features - Implementation Plan

## 📋 Executive Summary

This document provides a detailed implementation plan for completing the TCP Progress Synchronization features in MangaHub. Based on the verification analysis, we need to implement 2 major features and several enhancements.

**Current Status:** 60% Complete  
**Target:** 100% Complete  
**Estimated Total Effort:** 20-26 hours

---

## 🎯 Implementation Priorities

### Priority 1: Critical Features (Must Have)
1. ✅ **Enhance `sync status` to query live server** - 2-3 hours
2. ✅ **Implement `sync monitor` real-time display** - 4-6 hours

### Priority 2: Enhanced Features (Should Have)
3. ✅ **Add multi-device tracking** - 4-6 hours
4. ✅ **Implement last sync tracking** - 2-3 hours

### Priority 3: Polish & Testing (Nice to Have)
5. ✅ **Add comprehensive tests** - 4-6 hours
6. ✅ **Update documentation** - 2-3 hours

---

## 📝 Task Breakdown

## Task 1: Enhance `sync status` Command ⭐ HIGH PRIORITY

**Objective:** Make `sync status` query the TCP server for live statistics instead of only reading local cache.

### Current Behavior
```go
// cli/sync.go - syncStatusCmd (Lines 242-307)
// Only reads from sync_state.yaml
active, connInfo, err := config.IsConnectionActive()
// Shows "N/A" for statistics
```

### Target Behavior
```bash
$ mangahub sync status
TCP Sync Status:
  Connection: ✓ Active
  Server: localhost:9090
  Uptime: 2h 15m 30s
  Last heartbeat: 2 seconds ago

Session Info:
  User: johndoe
  Session ID: sess_9x8y7z6w5v
  Device: My-Desktop (desktop)
  Devices online: 3

Sync Statistics:
  Messages sent: 47
  Messages received: 23
  Last sync: 30 seconds ago (One Piece ch. 1095)
  Sync conflicts: 0

Network Quality: Excellent (RTT: 15ms)
```

### Implementation Steps

#### 1.1 Modify `syncStatusCmd` in `cli/sync.go`

```go
// After line 250: Instead of just checking local state, connect to server

func querySyncStatus(cfg *config.Config, connInfo *config.ConnectionInfo) (*tcp.StatusResponsePayload, error) {
    serverAddr := net.JoinHostPort(cfg.Server.Host, fmt.Sprintf("%d", cfg.Server.TCPPort))
    
    // Connect with short timeout
    conn, err := net.DialTimeout("tcp", serverAddr, 5*time.Second)
    if err != nil {
        return nil, fmt.Errorf("failed to connect: %w", err)
    }
    defer conn.Close()
    
    // Authenticate
    authMsg := map[string]interface{}{
        "type": "auth",
        "payload": map[string]string{"token": cfg.User.Token},
    }
    authJSON, _ := json.Marshal(authMsg)
    authJSON = append(authJSON, '\n')
    conn.Write(authJSON)
    
    // Wait for auth response
    reader := bufio.NewReader(conn)
    response, _ := reader.ReadString('\n')
    
    var authResponse map[string]interface{}
    json.Unmarshal([]byte(response), &authResponse)
    
    if authResponse["type"] != "success" {
        return nil, fmt.Errorf("authentication failed")
    }
    
    // Send status_request
    statusMsg := map[string]interface{}{
        "type": "status_request",
        "payload": map[string]interface{}{},
    }
    statusJSON, _ := json.Marshal(statusMsg)
    statusJSON = append(statusJSON, '\n')
    conn.Write(statusJSON)
    
    // Read status response
    response, err = reader.ReadString('\n')
    if err != nil {
        return nil, fmt.Errorf("failed to read status response: %w", err)
    }
    
    var statusResponse struct {
        Type    string `json:"type"`
        Payload tcp.StatusResponsePayload `json:"payload"`
    }
    
    if err := json.Unmarshal([]byte(response), &statusResponse); err != nil {
        return nil, fmt.Errorf("failed to parse status response: %w", err)
    }
    
    return &statusResponse.Payload, nil
}
```

#### 1.2 Update Status Display Logic

```go
// Update syncStatusCmd RunE function

fmt.Println("TCP Sync Status:")
fmt.Println()

if !active || connInfo == nil {
    fmt.Println("  Connection: ✗ Inactive")
    fmt.Println()
    fmt.Println("To connect: mangahub sync connect")
    return nil
}

// NEW: Query live status from server
cfg, err := config.Load()
if err != nil {
    // Fall back to cached display
    displayCachedStatus(connInfo)
    return nil
}

liveStatus, err := querySyncStatus(cfg, connInfo)
if err != nil {
    fmt.Printf("  ⚠ Unable to fetch live status: %s\n", err.Error())
    fmt.Println("  Showing cached information:")
    displayCachedStatus(connInfo)
    return nil
}

// Display live status
fmt.Println("  Connection: ✓ Active")
fmt.Printf("  Server: %s\n", liveStatus.ServerAddress)
fmt.Printf("  Uptime: %s\n", formatDuration(time.Duration(liveStatus.Uptime)*time.Second))
fmt.Printf("  Last heartbeat: %s\n", liveStatus.LastHeartbeat)

fmt.Println()
fmt.Println("Session Info:")
fmt.Printf("  User: %s\n", cfg.User.Username)
fmt.Printf("  Session ID: %s\n", liveStatus.SessionID)
fmt.Printf("  Devices online: %d\n", liveStatus.DevicesOnline)

fmt.Println()
fmt.Println("Sync Statistics:")
fmt.Printf("  Messages sent: %d\n", liveStatus.MessagesSent)
fmt.Printf("  Messages received: %d\n", liveStatus.MessagesReceived)

if liveStatus.LastSync != nil {
    timeSince := time.Since(parseTimestamp(liveStatus.LastSync.Timestamp))
    fmt.Printf("  Last sync: %s ago (%s ch. %d)\n", 
        formatDuration(timeSince),
        liveStatus.LastSync.MangaTitle,
        liveStatus.LastSync.Chapter)
} else {
    fmt.Println("  Last sync: N/A")
}

fmt.Println("  Sync conflicts: 0") // TODO: Track conflicts

fmt.Println()
fmt.Printf("Network Quality: %s (RTT: %dms)\n", liveStatus.NetworkQuality, liveStatus.RTT)
```

#### 1.3 Add Helper Functions

```go
func displayCachedStatus(connInfo *config.ConnectionInfo) {
    fmt.Println("  Connection: ✓ Active (cached)")
    fmt.Printf("  Server: %s\n", connInfo.Server)
    // ... existing cached display logic
}

func parseTimestamp(ts string) time.Time {
    t, _ := time.Parse(time.RFC3339, ts)
    return t
}
```

### Files to Modify
- ✏️ `cli/sync.go` - Add `querySyncStatus()`, update `syncStatusCmd`
- 📖 `internal/tcp/protocol.go` - Already has `StatusResponsePayload` ✅

### Testing
```bash
# Test 1: Active connection
mangahub sync connect
# In another terminal:
mangahub sync status

# Test 2: No connection
mangahub sync status

# Test 3: Connection lost
mangahub sync connect
# Stop TCP server
# In another terminal:
mangahub sync status
```

---

## Task 2: Implement `sync monitor` Real-time Display ⭐ HIGH PRIORITY

**Objective:** Create a real-time event monitoring display that shows progress updates from all devices.

### Current Behavior
```go
// cli/sync.go - syncMonitorCmd (Lines 309-326)
// Just shows placeholder message
fmt.Println("Real-time monitoring is not yet fully implemented.")
```

### Target Behavior
```bash
$ mangahub sync monitor
Monitoring real-time sync updates... (Press Ctrl+C to exit)

[17:05:12] ← Device 'mobile' updated: Jujutsu Kaisen → Chapter 248
[17:05:45] → Broadcasting update: Attack on Titan → Chapter 90
[17:06:23] ← Device 'web' updated: Demon Slayer → Chapter 157
[17:07:01] ← Device 'mobile' updated: One Piece → Chapter 1096
[17:07:35] → Broadcasting update: One Piece → Chapter 1096 (sync conflict resolved)

Real-time sync monitoring active. Updates appear as they happen.
^C
Monitoring stopped
```

### Implementation Steps

#### 2.1 Implement Event Listener in `cli/sync.go`

```go
var syncMonitorCmd = &cobra.Command{
    Use:   "monitor",
    Short: "Monitor real-time sync updates",
    Long:  `Display real-time synchronization updates as they happen across devices.`,
    RunE: func(cmd *cobra.Command, args []string) error {
        // Check if connected
        active, connInfo, err := config.IsConnectionActive()
        if err != nil {
            printError("Failed to check connection status")
            return err
        }
        
        if !active || connInfo == nil {
            printError("Not connected to sync server")
            fmt.Println("Run: mangahub sync connect")
            return fmt.Errorf("no active connection")
        }
        
        cfg, err := config.Load()
        if err != nil {
            printError("Failed to load configuration")
            return err
        }
        
        fmt.Println("Monitoring real-time sync updates... (Press Ctrl+C to exit)")
        fmt.Println()
        
        return startMonitoring(cfg, connInfo)
    },
}
```

#### 2.2 Create Monitoring Connection

```go
func startMonitoring(cfg *config.Config, connInfo *config.ConnectionInfo) error {
    serverAddr := net.JoinHostPort(cfg.Server.Host, fmt.Sprintf("%d", cfg.Server.TCPPort))
    
    conn, err := net.DialTimeout("tcp", serverAddr, 10*time.Second)
    if err != nil {
        printError(fmt.Sprintf("Failed to connect: %s", err.Error()))
        return err
    }
    defer conn.Close()
    
    // Authenticate
    authMsg := map[string]interface{}{
        "type": "auth",
        "payload": map[string]string{"token": cfg.User.Token},
    }
    authJSON, _ := json.Marshal(authMsg)
    authJSON = append(authJSON, '\n')
    
    if _, err := conn.Write(authJSON); err != nil {
        printError("Failed to authenticate")
        return err
    }
    
    reader := bufio.NewReader(conn)
    response, _ := reader.ReadString('\n')
    
    var authResponse map[string]interface{}
    json.Unmarshal([]byte(response), &authResponse)
    
    if authResponse["type"] != "success" {
        printError("Authentication failed")
        return fmt.Errorf("authentication rejected")
    }
    
    // Subscribe to updates
    subscribeMsg := map[string]interface{}{
        "type": "subscribe_updates",
        "payload": map[string]interface{}{
            "event_types": []string{"progress", "library"},
        },
    }
    subscribeJSON, _ := json.Marshal(subscribeMsg)
    subscribeJSON = append(subscribeJSON, '\n')
    
    if _, err := conn.Write(subscribeJSON); err != nil {
        printError("Failed to subscribe to updates")
        return err
    }
    
    // Start monitoring loop
    return monitorEventLoop(conn, reader)
}
```

#### 2.3 Event Display Loop

```go
func monitorEventLoop(conn net.Conn, reader *bufio.Reader) error {
    // Setup Ctrl+C handler
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
    
    // Event channel
    eventChan := make(chan string, 10)
    errorChan := make(chan error, 1)
    
    // Start reader goroutine
    go func() {
        for {
            line, err := reader.ReadString('\n')
            if err != nil {
                errorChan <- err
                return
            }
            eventChan <- line
        }
    }()
    
    // Display loop
    for {
        select {
        case <-sigChan:
            fmt.Println("\n\nMonitoring stopped")
            return nil
            
        case err := <-errorChan:
            fmt.Printf("\n\n✗ Connection error: %s\n", err.Error())
            return err
            
        case event := <-eventChan:
            displayEvent(event)
        }
    }
}
```

#### 2.4 Event Formatting

```go
func displayEvent(eventJSON string) {
    var msg struct {
        Type    string `json:"type"`
        Payload struct {
            Timestamp   string `json:"timestamp"`
            Direction   string `json:"direction"`
            DeviceType  string `json:"device_type"`
            DeviceName  string `json:"device_name"`
            MangaTitle  string `json:"manga_title"`
            Chapter     int    `json:"chapter"`
            Action      string `json:"action"`
            ConflictMsg string `json:"conflict_msg,omitempty"`
        } `json:"payload"`
    }
    
    if err := json.Unmarshal([]byte(eventJSON), &msg); err != nil {
        return
    }
    
    // Only display update_event messages
    if msg.Type != "update_event" {
        return
    }
    
    // Parse timestamp and format to local time
    eventTime, err := time.Parse(time.RFC3339, msg.Payload.Timestamp)
    if err != nil {
        eventTime = time.Now()
    }
    
    // Format time as HH:MM:SS
    timeStr := eventTime.Format("15:04:05")
    
    // Direction indicator
    var directionIndicator string
    if msg.Payload.Direction == "incoming" {
        directionIndicator = "←"
    } else {
        directionIndicator = "→"
    }
    
    // Build event message
    var eventMsg string
    if msg.Payload.Action == "updated" {
        eventMsg = fmt.Sprintf("[%s] %s Device '%s' updated: %s → Chapter %d",
            timeStr,
            directionIndicator,
            msg.Payload.DeviceName,
            msg.Payload.MangaTitle,
            msg.Payload.Chapter)
    } else if msg.Payload.Action == "added" {
        eventMsg = fmt.Sprintf("[%s] %s Device '%s' added to library: %s",
            timeStr,
            directionIndicator,
            msg.Payload.DeviceName,
            msg.Payload.MangaTitle)
    } else if msg.Payload.Action == "removed" {
        eventMsg = fmt.Sprintf("[%s] %s Device '%s' removed from library: %s",
            timeStr,
            directionIndicator,
            msg.Payload.DeviceName,
            msg.Payload.MangaTitle)
    }
    
    // Add conflict message if present
    if msg.Payload.ConflictMsg != "" {
        eventMsg += fmt.Sprintf(" (%s)", msg.Payload.ConflictMsg)
    }
    
    fmt.Println(eventMsg)
}
```

### Files to Modify
- ✏️ `cli/sync.go` - Rewrite `syncMonitorCmd`, add `startMonitoring()`, `monitorEventLoop()`, `displayEvent()`
- 📖 `internal/tcp/protocol.go` - Already has `UpdateEventPayload` ✅

### Testing
```bash
# Terminal 1: Start monitoring
mangahub sync monitor

# Terminal 2: Make changes
mangahub progress update manga_001 25
mangahub library add manga_002

# Terminal 1: Should see updates appear in real-time
```

---

## Task 3: Add Subscribe/Unsubscribe Handlers ⭐ HIGH PRIORITY

**Objective:** Implement server-side handlers for subscribe_updates and unsubscribe_updates messages.

### Implementation Steps

#### 3.1 Add Subscription Tracking to SessionManager

```go
// internal/tcp/session.go

type ClientSession struct {
    SessionID        string
    DeviceType       string
    DeviceName       string
    ConnectedAt      time.Time
    LastHeartbeat    time.Time
    MessagesSent     int64
    MessagesReceived int64
    LastSyncTime     time.Time
    LastSyncManga    string
    LastSyncChapter  int
    Subscribed       bool              // NEW
    EventTypes       []string          // NEW: filter event types
}

func (sm *SessionManager) Subscribe(clientID string, eventTypes []string) {
    sm.mu.Lock()
    defer sm.mu.Unlock()
    
    if sessionID, exists := sm.clientToSession[clientID]; exists {
        if session, ok := sm.sessions[sessionID]; ok {
            session.Subscribed = true
            session.EventTypes = eventTypes
        }
    }
}

func (sm *SessionManager) Unsubscribe(clientID string) {
    sm.mu.Lock()
    defer sm.mu.Unlock()
    
    if sessionID, exists := sm.clientToSession[clientID]; exists {
        if session, ok := sm.sessions[sessionID]; ok {
            session.Subscribed = false
            session.EventTypes = nil
        }
    }
}

func (sm *SessionManager) GetSubscribedClients() []string {
    sm.mu.RLock()
    defer sm.mu.RUnlock()
    
    clients := make([]string, 0)
    for clientID, sessionID := range sm.clientToSession {
        if session, ok := sm.sessions[sessionID]; ok && session.Subscribed {
            clients = append(clients, clientID)
        }
    }
    return clients
}
```

#### 3.2 Add Handlers in `internal/tcp/handler.go`

```go
// Add to routeMessage() switch statement (around line 69)

case "subscribe_updates":
    return handleSubscribeUpdates(client, msg.Payload, log, sessionMgr)
case "unsubscribe_updates":
    return handleUnsubscribeUpdates(client, msg.Payload, log, sessionMgr)
```

```go
// Add new handler functions

func handleSubscribeUpdates(client *Client, payload json.RawMessage, log *logger.Logger, sessionMgr *SessionManager) error {
    if !client.Authenticated {
        authErr := NewAuthNotAuthenticatedError()
        SendError(client, authErr)
        return authErr
    }
    
    var subscribePayload SubscribeUpdatesPayload
    if err := json.Unmarshal(payload, &subscribePayload); err != nil {
        protoErr := NewProtocolInvalidPayloadError("Invalid subscribe_updates payload")
        SendError(client, protoErr)
        return protoErr
    }
    
    // Default to all event types if not specified
    eventTypes := subscribePayload.EventTypes
    if len(eventTypes) == 0 {
        eventTypes = []string{"progress", "library"}
    }
    
    sessionMgr.Subscribe(client.ID, eventTypes)
    
    log.Info("client_subscribed_to_updates",
        "client_id", client.ID,
        "event_types", eventTypes)
    
    client.Conn.Write(CreateSuccessMessage("Subscribed to updates"))
    return nil
}

func handleUnsubscribeUpdates(client *Client, payload json.RawMessage, log *logger.Logger, sessionMgr *SessionManager) error {
    if !client.Authenticated {
        authErr := NewAuthNotAuthenticatedError()
        SendError(client, authErr)
        return authErr
    }
    
    sessionMgr.Unsubscribe(client.ID)
    
    log.Info("client_unsubscribed_from_updates", "client_id", client.ID)
    
    client.Conn.Write(CreateSuccessMessage("Unsubscribed from updates"))
    return nil
}
```

### Files to Modify
- ✏️ `internal/tcp/session.go` - Add subscription tracking
- ✏️ `internal/tcp/handler.go` - Add handlers
- 📖 `internal/tcp/protocol.go` - Already has payloads ✅

---

## Task 4: Enhance Bridge Event Broadcasting ⭐ MEDIUM PRIORITY

**Objective:** Make the bridge send properly formatted UpdateEventPayload to subscribed clients.

### Implementation Steps

#### 4.1 Modify Bridge to Access SessionManager

```go
// internal/bridge/tcp_http_bridge.go

type Bridge struct {
    logger         *logger.Logger
    clients        map[string][]*TCPClient
    udpBroadcaster UDPBroadcaster
    clientsLock    sync.RWMutex
    eventChan      chan Event
    stopChan       chan struct{}
    sessionManager *SessionManager  // NEW
}

func (b *Bridge) SetSessionManager(sm *SessionManager) {
    b.sessionManager = sm
}
```

#### 4.2 Update NotifyProgressUpdate

```go
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
    
    // NEW: Broadcast formatted update to subscribed clients
    b.broadcastUpdateEvent(event.UserID, UpdateEventPayload{
        Timestamp:  event.LastReadDate.Format(time.RFC3339),
        Direction:  "outgoing",  // From this client's perspective
        DeviceType: "unknown",   // TODO: Get from session
        DeviceName: "Device",    // TODO: Get from session
        MangaTitle: event.MangaTitle,
        Chapter:    event.ChapterID,
        Action:     "updated",
    })
    
    if b.udpBroadcaster != nil {
        b.udpBroadcaster.BroadcastToUser(event.UserID, BroadcastEvent{
            EventType: "progress_update",
            Data:      data,
        })
    }
}
```

#### 4.3 Add Broadcast Helper

```go
func (b *Bridge) broadcastUpdateEvent(userID string, event UpdateEventPayload) {
    if b.sessionManager == nil {
        return
    }
    
    // Get all subscribed clients for this user
    b.clientsLock.RLock()
    clients := b.clients[userID]
    b.clientsLock.RUnlock()
    
    if len(clients) == 0 {
        return
    }
    
    message := CreateUpdateEventMessage(event)
    
    for _, client := range clients {
        // Check if this client is subscribed
        // TODO: Get subscription status from session manager
        
        _, err := client.Conn.Write(message)
        if err != nil {
            b.logger.Warn("failed_to_send_update_event",
                "user_id", userID,
                "error", err.Error())
        }
    }
}
```

### Files to Modify
- ✏️ `internal/bridge/tcp_http_bridge.go`
- ✏️ `internal/tcp/server.go` - Pass SessionManager to Bridge

---

## Task 5: Add Multi-Device Tracking 🔧 MEDIUM PRIORITY

**Objective:** Track all devices per user and show device count in status.

### Implementation Steps

#### 5.1 Add User-to-Sessions Mapping

```go
// internal/tcp/session.go

type SessionManager struct {
    sessions        map[string]*ClientSession
    clientToSession map[string]string
    userToSessions  map[string][]string  // NEW: UserID -> []SessionID
    mu              sync.RWMutex
}

func (sm *SessionManager) CreateSession(clientID, userID, deviceType, deviceName string) *ClientSession {
    sm.mu.Lock()
    defer sm.mu.Unlock()
    
    session := &ClientSession{
        SessionID:        generateSessionID(deviceName, deviceType),
        DeviceType:       deviceType,
        DeviceName:       deviceName,
        ConnectedAt:      time.Now(),
        LastHeartbeat:    time.Now(),
        MessagesSent:     0,
        MessagesReceived: 0,
    }
    
    sm.sessions[session.SessionID] = session
    sm.clientToSession[clientID] = session.SessionID
    
    // NEW: Track user sessions
    if _, exists := sm.userToSessions[userID]; !exists {
        sm.userToSessions[userID] = make([]string, 0)
    }
    sm.userToSessions[userID] = append(sm.userToSessions[userID], session.SessionID)
    
    return session
}

func (sm *SessionManager) GetUserDeviceCount(userID string) int {
    sm.mu.RLock()
    defer sm.mu.RUnlock()
    
    sessions, exists := sm.userToSessions[userID]
    if !exists {
        return 0
    }
    
    // Count only active sessions
    count := 0
    for _, sessionID := range sessions {
        if _, ok := sm.sessions[sessionID]; ok {
            count++
        }
    }
    return count
}

func (sm *SessionManager) GetUserSessions(userID string) []*ClientSession {
    sm.mu.RLock()
    defer sm.mu.RUnlock()
    
    sessionIDs, exists := sm.userToSessions[userID]
    if !exists {
        return nil
    }
    
    sessions := make([]*ClientSession, 0)
    for _, sessionID := range sessionIDs {
        if session, ok := sm.sessions[sessionID]; ok {
            sessions = append(sessions, session)
        }
    }
    return sessions
}
```

#### 5.2 Update Status Handler

```go
// internal/tcp/handler.go - handleStatusRequest

func handleStatusRequest(client *Client, log *logger.Logger, sessionMgr *SessionManager, heartbeatMgr *HeartbeatManager) error {
    if !client.Authenticated {
        authErr := NewAuthNotAuthenticatedError()
        SendError(client, authErr)
        return authErr
    }
    
    session, ok := sessionMgr.GetSessionByClientID(client.ID)
    if !ok {
        bizErr := NewProtocolInvalidPayloadError("No active session")
        SendError(client, bizErr)
        return bizErr
    }
    
    lastHeartbeat, ok := heartbeatMgr.GetLastHeartbeat(client.ID)
    var lastHeartbeatStr string
    if ok {
        lastHeartbeatStr = lastHeartbeat.Format(time.RFC3339)
    }
    
    rtt, _ := heartbeatMgr.GetRTT(client.ID)
    quality := heartbeatMgr.GetNetworkQuality(client.ID)
    uptime := int64(time.Since(session.ConnectedAt).Seconds())
    
    // NEW: Get device count for this user
    devicesOnline := sessionMgr.GetUserDeviceCount(client.UserID)
    
    statusPayload := StatusResponsePayload{
        ConnectionStatus: "active",
        ServerAddress:    client.Conn.LocalAddr().String(),
        Uptime:           uptime,
        LastHeartbeat:    lastHeartbeatStr,
        SessionID:        session.SessionID,
        DevicesOnline:    devicesOnline,  // NOW ACCURATE
        MessagesSent:     session.MessagesSent,
        MessagesReceived: session.MessagesReceived,
        NetworkQuality:   quality,
        RTT:              int64(rtt.Milliseconds()),
    }
    
    response := CreateStatusResponseMessage(statusPayload)
    
    log.Debug("status_request_handled",
        "session_id", session.SessionID,
        "uptime", uptime,
        "devices_online", devicesOnline,
        "network_quality", quality)
    
    _, err := client.Conn.Write(response)
    if err != nil {
        return NewNetworkWriteError(err)
    }
    return nil
}
```

### Files to Modify
- ✏️ `internal/tcp/session.go`
- ✏️ `internal/tcp/handler.go`

---

## Task 6: Add Last Sync Tracking 🔧 MEDIUM PRIORITY

**Objective:** Track and display the last sync operation details.

### Implementation Steps

#### 6.1 Enhance handleSyncProgress

```go
// internal/tcp/handler.go - Add after successful sync (around line 240)

// After database update succeeds:
_, err = database.DB.Exec(query, ...)
if err != nil {
    // ... error handling
}

// NEW: Update session with last sync info
sessionMgr.UpdateLastSync(client.ID, syncPayload.MangaID, syncPayload.CurrentChapter)

// Also get manga title for better display
var mangaTitle string
database.DB.QueryRow("SELECT title FROM manga WHERE id = ?", syncPayload.MangaID).Scan(&mangaTitle)

session, _ := sessionMgr.GetSessionByClientID(client.ID)
if session != nil {
    session.LastSyncMangaTitle = mangaTitle  // NEW field
}
```

#### 6.2 Update Session Structure

```go
// internal/tcp/session.go

type ClientSession struct {
    SessionID          string
    DeviceType         string
    DeviceName         string
    ConnectedAt        time.Time
    LastHeartbeat      time.Time
    MessagesSent       int64
    MessagesReceived   int64
    LastSyncTime       time.Time
    LastSyncManga      string
    LastSyncMangaTitle string  // NEW
    LastSyncChapter    int
    Subscribed         bool
    EventTypes         []string
}
```

#### 6.3 Populate LastSyncInfo in Status

```go
// internal/tcp/handler.go - handleStatusRequest

statusPayload := StatusResponsePayload{
    ConnectionStatus: "active",
    ServerAddress:    client.Conn.LocalAddr().String(),
    Uptime:           uptime,
    LastHeartbeat:    lastHeartbeatStr,
    SessionID:        session.SessionID,
    DevicesOnline:    devicesOnline,
    MessagesSent:     session.MessagesSent,
    MessagesReceived: session.MessagesReceived,
    NetworkQuality:   quality,
    RTT:              int64(rtt.Milliseconds()),
}

// NEW: Add last sync info if available
if !session.LastSyncTime.IsZero() {
    statusPayload.LastSync = &LastSyncInfo{
        MangaID:    session.LastSyncManga,
        MangaTitle: session.LastSyncMangaTitle,
        Chapter:    session.LastSyncChapter,
        Timestamp:  session.LastSyncTime.Format(time.RFC3339),
    }
}
```

### Files to Modify
- ✏️ `internal/tcp/session.go`
- ✏️ `internal/tcp/handler.go`

---

## Task 7-10: Testing & Documentation 📚 LOW PRIORITY

### Task 7: Add Formatting Helpers
Create utility functions for better display formatting.

### Task 8-9: Integration Tests
Add comprehensive test coverage for new features.

### Task 10: Documentation Updates
Update all CLI guides with complete examples.

---

## 🗓️ Implementation Timeline

### Week 1: Core Features
- **Days 1-2:** Task 1 - Enhance sync status (2-3 hours)
- **Days 3-5:** Task 2 - Implement sync monitor (4-6 hours)
- **Day 6:** Task 3 - Subscribe/unsubscribe handlers (2-3 hours)

### Week 2: Enhancements
- **Days 1-2:** Task 4 - Bridge event broadcasting (3-4 hours)
- **Days 3-4:** Task 5 - Multi-device tracking (4-6 hours)
- **Day 5:** Task 6 - Last sync tracking (2-3 hours)

### Week 3: Polish
- **Days 1-2:** Tasks 7-9 - Testing (4-6 hours)
- **Day 3:** Task 10 - Documentation (2-3 hours)

---

## 📊 Success Criteria

### Definition of Done

✅ **Task 1 Complete When:**
- `mangahub sync status` queries live server
- Displays all statistics accurately
- Shows RTT and network quality
- Gracefully falls back to cached data if server unreachable

✅ **Task 2 Complete When:**
- `mangahub sync monitor` connects to server
- Displays real-time updates with timestamps
- Shows direction indicators
- Handles Ctrl+C gracefully
- Works with multiple simultaneous monitors

✅ **Task 3 Complete When:**
- Subscribe/unsubscribe messages handled
- Session manager tracks subscriptions
- Clients can filter event types

✅ **Task 4 Complete When:**
- Bridge sends formatted UpdateEventPayload
- All subscribed clients receive events
- Events include device information

✅ **Task 5 Complete When:**
- Multiple devices per user tracked
- Status shows accurate device count
- Each device maintains separate session

✅ **Task 6 Complete When:**
- Last sync details stored in session
- Status displays manga title and chapter
- Timestamp shows time since last sync

---

## 🧪 Testing Strategy

### Unit Tests
- Session manager methods
- Event formatting functions
- Subscription tracking

### Integration Tests
- Full connect/status/monitor flow
- Multi-device scenarios
- Event broadcasting
- Connection failure recovery

### Manual Testing Scenarios

#### Scenario 1: Single Device
```bash
mangahub sync connect
mangahub sync status  # Should show 1 device online
mangahub progress update manga_001 10
mangahub sync status  # Should show last sync details
```

#### Scenario 2: Multi-Device
```bash
# Terminal 1
mangahub sync connect --device-name Device1

# Terminal 2
mangahub sync connect --device-name Device2

# Terminal 3
mangahub sync status  # Should show 2 devices online

# Terminal 4
mangahub sync monitor

# Terminal 1
mangahub progress update manga_001 15

# Terminal 4 should show the update
```

#### Scenario 3: Real-time Monitoring
```bash
# Terminal 1
mangahub sync monitor

# Terminal 2
mangahub progress update manga_001 20
mangahub library add manga_002
mangahub progress update manga_003 5

# Terminal 1 should show all three events
```

---

## 🚨 Risks & Mitigations

### Risk 1: Connection Overhead
**Risk:** Opening new connection for every status check may be slow  
**Mitigation:** Add connection pooling or reuse existing connection  
**Alternative:** Cache status for 5-10 seconds

### Risk 2: Event Flooding
**Risk:** Too many events may overwhelm monitor display  
**Mitigation:** Add rate limiting, batching, or filtering options  
**Alternative:** Buffer and display summary every N seconds

### Risk 3: Session State Sync
**Risk:** Session state may become inconsistent  
**Mitigation:** Use proper locking, add state validation  
**Alternative:** Periodic session cleanup and reconciliation

### Risk 4: Backward Compatibility
**Risk:** Changes may break existing functionality  
**Mitigation:** Thorough testing of existing features  
**Alternative:** Feature flags for new functionality

---

## 📦 Deliverables

1. ✅ Enhanced `mangahub sync status` with live data
2. ✅ Working `mangahub sync monitor` with real-time display
3. ✅ Subscribe/unsubscribe message handlers
4. ✅ Bridge broadcasting formatted events
5. ✅ Multi-device tracking per user
6. ✅ Last sync tracking and display
7. ✅ Comprehensive test suite
8. ✅ Updated documentation and examples

---

## 🎓 Conclusion

This implementation plan provides a clear path from 60% to 100% feature completion. The modular approach allows incremental implementation and testing. Priority 1 tasks (sync status and monitor) will provide immediate value, while Priority 2 and 3 tasks add polish and robustness.

**Estimated Total Effort:** 20-26 hours  
**Recommended Team Size:** 1-2 developers  
**Timeline:** 2-3 weeks for complete implementation

The solid server-side foundation means most work is client-side CLI enhancements, reducing risk and complexity.
