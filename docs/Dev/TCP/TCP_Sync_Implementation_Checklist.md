# TCP Sync Implementation Checklist

## 🎯 Quick Reference Guide

Use this checklist to track implementation progress. Check off items as you complete them.

---

## Priority 1: Critical Features ⭐⭐⭐

### Task 1: Enhance `sync status` Command

#### 1.1 Server Query Implementation
- [ ] Create `querySyncStatus()` function in `cli/sync.go`
  - [ ] Connect to TCP server with timeout
  - [ ] Authenticate with JWT token
  - [ ] Send `status_request` message
  - [ ] Parse `StatusResponsePayload` response
  - [ ] Handle connection errors gracefully

- [ ] Update `syncStatusCmd.RunE` function
  - [ ] Check if connection is active
  - [ ] Call `querySyncStatus()` for live data
  - [ ] Display live statistics
  - [ ] Fall back to cached data on error

- [ ] Add helper functions
  - [ ] `displayCachedStatus()` - Show local sync_state
  - [ ] `parseTimestamp()` - Parse ISO timestamps
  - [ ] Update `formatDuration()` if needed

#### 1.2 Display Enhancements
- [ ] Show connection status (active/inactive)
- [ ] Show server address
- [ ] Show uptime with proper formatting
- [ ] Show last heartbeat time
- [ ] Show session info (user, session ID, device)
- [ ] Show device count (from server)
- [ ] Show sync statistics (messages sent/received)
- [ ] Show last sync details (manga, chapter, time)
- [ ] Show network quality and RTT

#### 1.3 Testing
- [ ] Test with active connection
- [ ] Test with no connection
- [ ] Test with stale connection
- [ ] Test with server unreachable
- [ ] Test fallback to cached data

**Estimated Time:** 2-3 hours  
**Files Modified:** `cli/sync.go`

---

### Task 2: Implement `sync monitor` Real-time Display

#### 2.1 Connection Setup
- [ ] Update `syncMonitorCmd.RunE` function
  - [ ] Check for active connection
  - [ ] Load configuration
  - [ ] Call `startMonitoring()`

- [ ] Create `startMonitoring()` function
  - [ ] Connect to TCP server
  - [ ] Authenticate
  - [ ] Send `subscribe_updates` message
  - [ ] Call `monitorEventLoop()`

#### 2.2 Event Loop
- [ ] Create `monitorEventLoop()` function
  - [ ] Setup Ctrl+C signal handler
  - [ ] Create event channel
  - [ ] Start reader goroutine
  - [ ] Display events as they arrive
  - [ ] Handle connection errors

#### 2.3 Event Display
- [ ] Create `displayEvent()` function
  - [ ] Parse JSON event message
  - [ ] Filter for `update_event` type
  - [ ] Parse and format timestamp
  - [ ] Show direction indicator (← or →)
  - [ ] Format action (updated/added/removed)
  - [ ] Display manga title and chapter
  - [ ] Show conflict messages if present

#### 2.4 Formatting Helpers
- [ ] Format timestamp to HH:MM:SS
- [ ] Format direction indicators
- [ ] Format event messages
- [ ] Optional: Add color support

#### 2.5 Testing
- [ ] Test with single device
- [ ] Test with multiple devices
- [ ] Test progress updates
- [ ] Test library add/remove
- [ ] Test Ctrl+C handling
- [ ] Test connection loss recovery
- [ ] Test with no active connection

**Estimated Time:** 4-6 hours  
**Files Modified:** `cli/sync.go`

---

### Task 3: Add Subscribe/Unsubscribe Handlers

#### 3.1 Session Manager Updates
- [ ] Modify `ClientSession` struct in `internal/tcp/session.go`
  - [ ] Add `Subscribed bool` field
  - [ ] Add `EventTypes []string` field

- [ ] Add subscription methods
  - [ ] `Subscribe(clientID, eventTypes)`
  - [ ] `Unsubscribe(clientID)`
  - [ ] `GetSubscribedClients()` - returns list
  - [ ] `IsSubscribed(clientID)` - check status

#### 3.2 Handler Implementation
- [ ] Update `routeMessage()` in `internal/tcp/handler.go`
  - [ ] Add case for `subscribe_updates`
  - [ ] Add case for `unsubscribe_updates`

- [ ] Create `handleSubscribeUpdates()` function
  - [ ] Check authentication
  - [ ] Parse `SubscribeUpdatesPayload`
  - [ ] Default to all event types if empty
  - [ ] Call `sessionMgr.Subscribe()`
  - [ ] Send success response
  - [ ] Log subscription

- [ ] Create `handleUnsubscribeUpdates()` function
  - [ ] Check authentication
  - [ ] Call `sessionMgr.Unsubscribe()`
  - [ ] Send success response
  - [ ] Log unsubscription

#### 3.3 Testing
- [ ] Test subscribe with specific event types
- [ ] Test subscribe with no event types (defaults)
- [ ] Test unsubscribe
- [ ] Test without authentication
- [ ] Test multiple clients subscribing

**Estimated Time:** 2-3 hours  
**Files Modified:** `internal/tcp/session.go`, `internal/tcp/handler.go`

---

## Priority 2: Enhanced Features ⭐⭐

### Task 4: Enhance Bridge Event Broadcasting

#### 4.1 Bridge Updates
- [ ] Add `sessionManager` field to `Bridge` struct
- [ ] Create `SetSessionManager()` method
- [ ] Update `NewBridge()` if needed

#### 4.2 Event Broadcasting
- [ ] Create `broadcastUpdateEvent()` method
  - [ ] Get subscribed clients for user
  - [ ] Check subscription status
  - [ ] Format `UpdateEventPayload`
  - [ ] Send to all subscribed clients
  - [ ] Handle write errors

- [ ] Update `NotifyProgressUpdate()`
  - [ ] Call `broadcastUpdateEvent()`
  - [ ] Include manga title
  - [ ] Set proper direction
  - [ ] Include device info

- [ ] Update `NotifyLibraryUpdate()`
  - [ ] Call `broadcastUpdateEvent()`
  - [ ] Format for library actions

#### 4.3 Server Integration
- [ ] Update `internal/tcp/server.go`
  - [ ] Pass `SessionManager` to bridge
  - [ ] Call `bridge.SetSessionManager()`

#### 4.4 Testing
- [ ] Test event broadcast to subscribed client
- [ ] Test no broadcast to unsubscribed client
- [ ] Test multiple subscribed clients
- [ ] Test event filtering by type

**Estimated Time:** 3-4 hours  
**Files Modified:** `internal/bridge/tcp_http_bridge.go`, `internal/tcp/server.go`

---

### Task 5: Add Multi-Device Tracking

#### 5.1 Session Manager Enhancement
- [ ] Add `userToSessions map[string][]string` field
- [ ] Update `NewSessionManager()` to initialize map

- [ ] Update `CreateSession()`
  - [ ] Track user to session mapping
  - [ ] Initialize user slice if needed
  - [ ] Append session ID to user's list

- [ ] Create `GetUserDeviceCount(userID)` method
  - [ ] Get sessions for user
  - [ ] Count only active sessions
  - [ ] Return count

- [ ] Create `GetUserSessions(userID)` method
  - [ ] Get all session IDs for user
  - [ ] Return session objects

- [ ] Update `RemoveSession()` and `RemoveSessionByClientID()`
  - [ ] Clean up user-to-sessions mapping
  - [ ] Remove session from user's list

#### 5.2 Status Handler Update
- [ ] Update `handleStatusRequest()`
  - [ ] Call `GetUserDeviceCount(client.UserID)`
  - [ ] Set `DevicesOnline` field accurately
  - [ ] Include in response

#### 5.3 Testing
- [ ] Test single device count
- [ ] Test multiple devices for same user
- [ ] Test different users don't interfere
- [ ] Test device count after disconnect
- [ ] Test cleanup of stale sessions

**Estimated Time:** 4-6 hours  
**Files Modified:** `internal/tcp/session.go`, `internal/tcp/handler.go`

---

### Task 6: Add Last Sync Tracking

#### 6.1 Session Structure Update
- [ ] Add `LastSyncMangaTitle string` to `ClientSession`
- [ ] Ensure `LastSyncTime`, `LastSyncManga`, `LastSyncChapter` fields exist

#### 6.2 Sync Handler Update
- [ ] Update `handleSyncProgress()`
  - [ ] Query manga title from database
  - [ ] Call `sessionMgr.UpdateLastSync()`
  - [ ] Store manga title in session

- [ ] Verify `UpdateLastSync()` method exists in `SessionManager`
  - [ ] Updates `LastSyncTime`
  - [ ] Updates `LastSyncManga`
  - [ ] Updates `LastSyncMangaTitle`
  - [ ] Updates `LastSyncChapter`

#### 6.3 Status Response Update
- [ ] Update `handleStatusRequest()`
  - [ ] Check if `LastSyncTime` is set
  - [ ] Create `LastSyncInfo` object
  - [ ] Populate manga ID, title, chapter
  - [ ] Set timestamp
  - [ ] Add to `StatusResponsePayload`

#### 6.4 Testing
- [ ] Test last sync appears after progress update
- [ ] Test last sync details are accurate
- [ ] Test last sync persists across status checks
- [ ] Test last sync shows correct manga title
- [ ] Test last sync time calculation

**Estimated Time:** 2-3 hours  
**Files Modified:** `internal/tcp/session.go`, `internal/tcp/handler.go`

---

## Priority 3: Polish & Testing ⭐

### Task 7: Add Helper Functions

- [ ] Create formatting utilities in `cli/sync.go`
  - [ ] `formatTimestamp()` - Format ISO time to HH:MM:SS
  - [ ] `formatDirection()` - Convert to arrow symbols
  - [ ] `formatUpdateEvent()` - Complete event formatting
  - [ ] `formatDuration()` - Already exists, verify

- [ ] Optional: Add color support
  - [ ] Check terminal color capability
  - [ ] Add ANSI color codes
  - [ ] Make colors configurable

**Estimated Time:** 1-2 hours  
**Files Modified:** `cli/sync.go`

---

### Task 8: Add Integration Tests for Sync Status

- [ ] Create test file `internal/tcp/test/status_test.go`

- [ ] Test status_request message handling
  - [ ] Test with active session
  - [ ] Test without session
  - [ ] Test with stale heartbeat
  - [ ] Test payload accuracy

- [ ] Test StatusResponsePayload
  - [ ] Verify all fields populated
  - [ ] Verify device count accuracy
  - [ ] Verify RTT calculation
  - [ ] Verify network quality

- [ ] Test status command end-to-end
  - [ ] Connect -> Check status -> Verify output
  - [ ] Multi-device status check
  - [ ] Status after disconnect

**Estimated Time:** 2-3 hours  
**Files Created:** `internal/tcp/test/status_test.go`

---

### Task 9: Add Integration Tests for Monitoring

- [ ] Create test file `internal/tcp/test/monitor_test.go`

- [ ] Test subscription flow
  - [ ] Subscribe with event types
  - [ ] Subscribe without event types
  - [ ] Unsubscribe
  - [ ] Multiple subscriptions

- [ ] Test event broadcasting
  - [ ] Progress update broadcasts
  - [ ] Library update broadcasts
  - [ ] Event received by subscribed client
  - [ ] Event not received by unsubscribed client

- [ ] Test UpdateEventPayload formatting
  - [ ] Verify timestamp format
  - [ ] Verify direction
  - [ ] Verify device info
  - [ ] Verify manga details

- [ ] Test concurrent monitoring
  - [ ] Multiple monitors for same user
  - [ ] Different users monitoring
  - [ ] Event delivery to all monitors

**Estimated Time:** 2-3 hours  
**Files Created:** `internal/tcp/test/monitor_test.go`

---

### Task 10: Update Documentation

#### 10.1 Update TCPCLI_FullGuide.md
- [ ] Add complete sync status section
  - [ ] Show expected output
  - [ ] Document all fields
  - [ ] Add examples

- [ ] Add complete sync monitor section
  - [ ] Show real-time output examples
  - [ ] Document event format
  - [ ] Add multi-device scenario

- [ ] Add troubleshooting section
  - [ ] Connection issues
  - [ ] Authentication failures
  - [ ] Missing statistics
  - [ ] Monitor not showing events

#### 10.2 Update TCPCLI_ShortGuide.md
- [ ] Add quick sync status example
- [ ] Add quick sync monitor example
- [ ] Update command reference

#### 10.3 Update README.md (if needed)
- [ ] Verify TCP sync features listed
- [ ] Update feature status
- [ ] Add links to detailed guides

#### 10.4 Create Examples
- [ ] Add example scripts to `scripts/test/`
  - [ ] `demo-sync-status.ps1`
  - [ ] `demo-sync-monitor.ps1`
  - [ ] `demo-multi-device.ps1`

**Estimated Time:** 2-3 hours  
**Files Modified:** `docs/TCPCLI_FullGuide.md`, `docs/TCPCLI_ShortGuide.md`, `README.md`

---

## 📊 Progress Tracking

### Overall Completion

```
Priority 1 (Critical):  ▢▢▢ (0/3 tasks)
Priority 2 (Enhanced): ▢▢▢ (0/3 tasks)
Priority 3 (Polish):   ▢▢▢▢ (0/4 tasks)

Total: 0/10 tasks complete (0%)
```

### Time Tracking

| Task | Estimated | Actual | Status |
|------|-----------|--------|--------|
| Task 1: Sync Status | 2-3h | - | ▢ Not Started |
| Task 2: Sync Monitor | 4-6h | - | ▢ Not Started |
| Task 3: Subscribe/Unsubscribe | 2-3h | - | ▢ Not Started |
| Task 4: Bridge Broadcasting | 3-4h | - | ▢ Not Started |
| Task 5: Multi-Device | 4-6h | - | ▢ Not Started |
| Task 6: Last Sync | 2-3h | - | ▢ Not Started |
| Task 7: Helpers | 1-2h | - | ▢ Not Started |
| Task 8: Status Tests | 2-3h | - | ▢ Not Started |
| Task 9: Monitor Tests | 2-3h | - | ▢ Not Started |
| Task 10: Documentation | 2-3h | - | ▢ Not Started |
| **Total** | **20-26h** | **-** | **0%** |

---

## 🚀 Quick Start

### Starting Implementation

1. **Setup Environment**
   ```powershell
   cd D:\GitHub\Manga-Hub-Group13
   git checkout udp
   git pull origin udp
   ```

2. **Start with Task 1** (Highest Value)
   - Focus on `cli/sync.go`
   - Implement `querySyncStatus()`
   - Test immediately

3. **Move to Task 2** (High Impact)
   - Continue in `cli/sync.go`
   - Implement monitoring loop
   - Test with real events

4. **Complete Priority 1** (Foundation)
   - Finish Task 3 (handlers)
   - Get basic features working end-to-end

5. **Add Enhancements** (Priority 2)
   - Task 4, 5, 6 can be done in parallel
   - Each is independent

6. **Polish** (Priority 3)
   - Add tests for confidence
   - Update docs for users

---

## 🎯 Definition of Done

### Feature Complete When:
- ✅ All checkboxes checked
- ✅ All tests passing
- ✅ Manual testing scenarios pass
- ✅ Documentation updated
- ✅ Code reviewed
- ✅ No known bugs

### Ready for Demo When:
- ✅ Tasks 1, 2, 3 complete (Priority 1)
- ✅ Basic multi-device working (Task 5)
- ✅ Documentation has examples (Task 10)

### Production Ready When:
- ✅ All tasks complete
- ✅ Test coverage > 80%
- ✅ Performance validated
- ✅ Error handling robust

---

## 📝 Notes

- Update this checklist as you progress
- Mark items complete with `[x]` instead of `[ ]`
- Add actual time spent for tracking
- Note any blockers or issues discovered
- Update progress percentages

Good luck with implementation! 🚀
