# Chat Feature Guide

## Quick Start

### 1. Start WebSocket Server
```powershell
go run cmd/websocket-server/main.go
# OR
.\bin\websocket-server.exe
```

### 2. Login (get authentication token)
```powershell
mangahub auth login --username your_username
```

### 3. Join Chat
```powershell
mangahub chat join
```

---

## Features Implemented ✅

### Interactive Chat Mode
- **Real-time messaging** - Send and receive messages instantly
- **Custom prompt** - Shows your username (`username>`)
- **Welcome message** - Displays connected users count and recent messages
- **Clean formatting** - Professional message display with timestamps

### Chat Commands (use in interactive mode)

| Command | Description | Example |
|---------|-------------|---------|
| `/help` | Show all available commands | `/help` |
| `/rooms` | List all available chat rooms | `/rooms` |
| `/create <name>` | Create a new custom room (you become owner) | `/create berserk-fans` |
| `/users` | List all online users | `/users` |
| `/quit` | Leave chat (or use `/exit`) | `/quit` |
| `/pm <user> <msg>` | Send private message | `/pm alice Hello!` |
| `/manga <id>` | Switch to manga discussion | `/manga one-piece` |
| `/history` | Show recent chat history | `/history` |
| `/status` | Check connection status | `/status` |

### Command Line Options

#### Join Chat
```powershell
# Join general chat
mangahub chat join

# Join manga-specific discussion
mangahub chat join --manga-id one-piece

# Join custom room
mangahub chat join -r "team-room"
```

#### Send Quick Message
```powershell
# Send to general chat
mangahub chat send "Hello everyone!"

# Send to manga chat
mangahub chat send "Great chapter!" --manga-id one-piece

# Send to custom room
mangahub chat send "Team update" -r "team-room"
```

#### View Chat History
```powershell
# View recent messages (default: 20)
mangahub chat history

# View more messages
mangahub chat history --limit 50

# View manga chat history
mangahub chat history --manga-id one-piece --limit 30
```

---

## Example Usage

### Scenario 1: General Chat

**Terminal 1:**
```powershell
mangahub auth login --username alice
mangahub chat join
```

**Output:**
```
Connecting to WebSocket chat server at ws://localhost:9093...
✓ Connected to General Chat
Chat Room: #general
Connected users: 3
Your status: Online

Recent messages:
[16:45] bob: Just finished reading the latest chapter!
[16:47] charlie: Which manga are you reading?
─────────────────────────────────────────────────────────────
You are now in chat. Type your message and press Enter.
Type /help for commands or /quit to leave.
─────────────────────────────────────────────────────────────
[global/presence] system: alice joined the chat
alice>
```

**Type messages:**
```
alice> Hello everyone!
[17:02] alice: Hello everyone!

[17:02] bob: Hey alice! Welcome to the chat

alice> /users
```

**Output:**
```
Online Users (3):
● alice (Global)
● bob (Global)
● charlie (Global)

alice>
```

### Scenario 2: Manga-Specific Chat

**Terminal 1:**
```powershell
mangahub chat join --manga-id one-piece
```

**Terminal 2:**
```powershell
mangahub chat send "Luffy's new gear is amazing!" --manga-id one-piece
```

### Scenario 3: Private Messaging

**In chat:**
```
alice> /pm bob Hey, want to discuss that chapter privately?
[PM to bob] Hey, want to discuss that chapter privately?

alice>
```

### Scenario 4: View History

```powershell
mangahub chat history --limit 10
```

**Output:**
```
Chat History (last 10 messages):
─────────────────────────────────────────────────────────────
[17:05] charlie: See you later!
[17:03] bob: That sounds interesting
[17:02] alice: Hello everyone!
[17:00] bob: Just finished reading the latest chapter!
─────────────────────────────────────────────────────────────
```

### Scenario 5: Browse and Create Rooms

**In chat:**
```
alice> /rooms
```

**Output:**
```
Available rooms

Total Rooms: 3
─────────────────────────────────────────────────────────────
📁 global
   Type: global | Members: 5 | Created: 2025-12-17 10:00:00
   Last activity: 2025-12-17 14:30:00
rooms",
  "room": "global"
}

{
  "type": "command",
  "command": "/create my-custom-room",
  "room": "global"
}

{
  "type": "command",
  "command": "/users",
  "room": "global"
}

{
  "type": "command",
  "command": "/history 50",
  "room": "custom-room-name
   Type: custom | Members: 3 | Created: 2025-12-17 12:00:00
   Last activity: 2025-12-17 14:00:00

📁 manga-1
   Type: manga | Members: 12 | Created: 2025-12-17 09:00:00
   Last activity: 2025-12-17 14:35:00

─────────────────────────────────────────────────────────────
Use: mangahub chat join -r "<room-name>" to join a room
```

**Create a new room:**
```
alice> /create one-piece-discussion
```

**Output:**
```
Room 'one-piece-discussion' created successfully! You are the owner.
✓ Room ID: conv_abc123
✓ Your role: owner
✓ Join with: mangahub chat join -r "one-piece-discussion"
```

**Join the new room:**
```powershell
mangahub chat join -r "one-piece-discussion"
```

---

## Frontend Integration Ready

The WebSocket implementation is ready for frontend integration with these message types:

### Outgoing (Client → Server)
```json
{
  "type": "text",
  "content": "message content",
  "room": "global"
}

{
  "type": "command",
  "command": "/users",
  "room": "global"
}

{
  "type": "text",
  "to": "user_id",
  "content": "private message",
  "room": "global"
}
```

### Incoming (Server → Client)
```json
{
  "id": "msg_id",
  "type": "welcome",
  "from": "system",
  "content": "Connected to General Chat",
  "room": "global",
  "timestamp": "2025-12-16T17:00:00Z",
  "metadata": {
    "user_count": 3,
    "room": "general",
    "recent_messages": []
  }
}

{
  "id": "msg_id",
  "type": "text",
  "from": "username",
  "content": "message content",

{
  "id": "msg_id",
  "type": "system",
  "from": "system",
  "content": "Available rooms",
  "room": "global",
  "timestamp": "2025-12-16T17:00:00Z",
  "metadata": {
    "rooms": [
      {
        "id": "conv_123",
        "name": "berserk-fans",
        "type": "custom",
        "member_count": 5,
        "created_at": "2025-12-17 10:00:00",
        "last_message_at": "2025-12-17 14:30:00"
      },
      {
        "id": "conv_456",
        "name": "manga-1",
        "type": "manga",
        "member_count": 12,
        "created_at": "2025-12-17 09:00:00"
      }
    ],
    "count": 2
  }
}

{
  "id": "msg_id",
  "type": "system",
  "from": "system",
  "content": "Room 'one-piece-fans' created successfully! You are the owner.",
  "room": "one-piece-fans",
  "timestamp": "2025-12-16T17:00:00Z",
  "metadata": {
    "room_id": "conv_789",
    "room_name": "one-piece-fans",
    "role": "owner"
  }
}

{
  "id": "msg_id",
  "type": "error",
  "from": "system",
  "content": "Failed to create room: room 'berserk-fans' already exists",
  "room": "global",
  "timestamp": "2025-12-16T17:00:00Z"
}
  "room": "global",
  "timestamp": "2025-12-16T17:00:00Z"
}

{
  "id": "msg_id",
  "type": "userlist",
  "from": "system",
  "room": "global",
  "timestamp": "2025-12-16T17:00:00Z",
  "metadata": {
    "users": [
      {"id": "user1", "username": "alice", "room": "global"},
      {"id": "user2", "username": "bob", "room": "global"}
    ],
    "count": 2
  }
}
```

---

## Architecture

### Backend (Go WebSocket Server)
- **Location:** `cmd/websocket-server/main.go`
- **Port:** 9093 (configurable via `WEBSOCKET_PORT`)
- **Authentication:** JWT token via query parameter
- **Endpoint:** `ws://localhost:9093/ws/chat?token=<jwt>`

### Server Components
- **Manager** (`internal/websocket/manager.go`) - Connection & room management
- **Handler** (`internal/websocket/handler.go`) - Message & command processing
- **Protocol** (`internal/websocket/protocol.go`) - Message type definitions
- **Server** (`internal/websocket/server.go`) - WebSocket server & lifecycle

### CLI Client
- **Location:** `cli/chat.go`
- **Features:** Interactive mode, commands, history
- **Configuration:** Stored in `~/.mangahub-cli/config.yaml`

---

## Troubleshooting
**Room List Sidebar:** Display all available rooms from `/rooms` command
   - **Chat message list:** Show messages filtered by current room
   - **User list sidebar:** Online users in current room
   - **Message input:** Send to current active room
   - **Create Room Button:** Modal to create new custom rooms
   - **Room info panel:** Show room type, member count, and owner

4. **Room Management Flow:**
   ```javascript
   // On connect - get rooms list
   ws.onopen = () => {
     ws.send(JSON.stringify({ type: "command", command: "/rooms" }));
   };
   
   // User clicks room - join it
   function joinRoom(roomName) {
     currentRoom = roomName;
     ws.send(JSON.stringify({ 
### Conversations (Chat Rooms)
```sql
CREATE TABLE IF NOT EXISTS conversations (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    type TEXT NOT NULL,  -- 'global', 'manga', 'custom'
    manga_id TEXT,       -- NULL for non-manga rooms
    created_by TEXT,
    created_at DATETIME NOT NULL,
    last_message_at DATETIME,
    FOREIGN KEY (manga_id) REFERENCES manga(id),
    FOREIGN KEY (created_by) REFERENCES users(id)
);
```

### Messages (Room Messages)
```sql
CREATE TABLE IF NOT EXISTS messages (
    id TEXT PRIMARY KEY,
    conversation_id TEXT NOT NULL,
    sender_id TEXT NOT NULL,
    content TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    FOREIGN KEY (conversation_id) REFERENCES conversations(id),
    FOREIGN KEY (sender_id) REFERENCES users(id)
);
```

### User Conversation History
```sql
CREATE TABLE IF NOT EXISTS user_conversation_history (
    user_id TEXT NOT NULL,
    conversation_id TEXT NOT NULL,
    role TEXT,  -- 'owner' for room creators, NULL for regular members
    joined_at DATETIME NOT NULL,
    unread_count INTEGER DEFAULT 0,
    PRIMARY KEY (user_id, conversation_id),
    FOREIGN KEY (user_id) REFERENCES users(id),
    FOREIGN KEY (conversation_id) REFERENCES conversations(id)
);
```

### Direct Messages (Legacy)
       command: "/history 50",
       room: roomName 
     }));
   }
   
   // User creates new room
   function createRoom(roomName) {
     ws.send(JSON.stringify({ 
       type: "command", 
       command: `/create ${roomName}` 
     }));
   }
   
   // Send message to current room
   function sendMessage(text) {
     ws.send(JSON.stringify({ 
       type: "text", 
       content: text,
       room: currentRoom 
     }));
   }
   ```

5. **Additional features to add:**
   - Typing indicators
   - Read receipts
   - Emoji support
   - File sharing
   - Voice messages
   - Video chat
   - Room search/filter
   - Room ownership managemenmessages showing
The chat history is stored in the database. Send some messages first!

### Messages not appearing
- Check if both terminals are connected to the same room
- Verify the WebSocket server is running
- Check for errors in the server logs

---

## Testing

### Load Testing
```powershell
.\scripts\test\test-websocket-load.ps1
```

### Demo Chat (2 users)
```powershell
.\scripts\test\demo-chat.ps1 -Token1 <token1> -Token2 <token2>
```

### Unit Tests
```powershell
go test ./internal/websocket/...
```

---

## Next Steps for Frontend

1. **Connect to WebSocket** using the same endpoint format:
   ```javascript
   const token = "your_jwt_token";
   const ws = new WebSocket(`ws://localhost:9093/ws/chat?token=${token}`);
   ```

2. **Handle message types** based on the protocol defined above

3. **Implement UI components:**
   - Chat message list
   - User list sidebar
   - Message input
   - Command palette
   - Room switcher

4. **Features to add:**
   - Typing indicators
   - Read receipts
   - Emoji support
   - File sharing
   - Voice messages
   - Video chat

---

## Database Schema

Chat messages are stored in the `chat_messages` table:
```sql
CREATE TABLE IF NOT EXISTS chat_messages (
    id TEXT PRIMARY KEY,
    from_user_id TEXT NOT NULL,
    to_user_id TEXT,  -- NULL for public messages
    content TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    FOREIGN KEY (from_user_id) REFERENCES users(id),
    FOREIGN KEY (to_user_id) REFERENCES users(id)
);
```

---

## Performance

- **Max message size:** 512 KB
- **Rate limit:** 20 messages per 10 seconds per user
- **Ping/Pong:** 30s interval for keep-alive
- **Connection timeout:** 60s without pong response
- **Concurrent connections:** Unlimited (limited by system resources)

---

## Security

✅ JWT authentication required  
✅ Token validation on connection  
✅ Rate limiting per user  
✅ Message size limits  
✅ SQL injection prevention (prepared statements)  
✅ CORS enabled (for frontend integration)  

---

Enjoy your real-time chat! 🚀💬
