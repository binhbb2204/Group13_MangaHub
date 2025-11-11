# TCP CLI Testing Guide (Full Version)
## Overview
The TCP sync feature provides real-time progress synchroniza**Connection: ✗ Inactive

To connect: .\bin\mangahub.exe sync connect
``` across multiple devices. When connected to the sync server, any progress updates you make on one device are immediately reflected on all your other connected devices.

## Commands
### `mangahub sync connect`
Establishes a persistent TCP connection to the sync server.

**Important:**
- **Always use `.\bin\mangahub.exe` if mangahub is not in your PATH.**
- To add mangahub to your PATH permanently, run: `.\scripts\setup\setup-path.ps1` and restart your terminal.
- The TCP sync server must be running before you connect. Start it with:
  ```powershell
  .\bin\tcp-server.exe
  ```

**Usage:**
```bash
mangahub sync connect [flags]
```

**Flags:**
- `--device-type <type>` - Specify device type: mobile, desktop, or web (default: desktop)
- `--device-name <name>` - Specify a friendly device name (default: hostname)

**Example:**
```bash
./bin/mangahub.exe sync connect --device-type desktop --device-name "thuannm"
```

**Expected Output:**
```
Connecting to TCP sync server at localhost:9090...
✓ Connected successfully!

Connection Details:
  Server: localhost:9090
  User: johndoe
  Session ID: sess_abc123xyz
  Device: Work Laptop (desktop)
  Connected at: 2024-01-20 17:00:00 MST

Sync Status:
  Auto-sync: true
  Conflict resolution: last_write_wins

Real-time sync is now active. Your progress will be synchronized across
all devices.

Commands:
  mangahub sync status   - View connection status
  mangahub sync monitor  - Monitor real-time updates
  mangahub sync disconnect - Disconnect from server
```

---

### `mangahub sync disconnect`
Gracefully closes the TCP sync connection.

**Usage:**
```bash
.\bin\mangahub.exe sync disconnect
```

**Note:** Always use `.\bin\mangahub.exe` if mangahub is not in your PATH.

**Expected Output:**
```
✓ Disconnected from sync server
Session ID: sess_abc123xyz
Duration: 2h 15m 30s
```

---

### `mangahub sync status`
Displays detailed information about the current sync connection.

**Usage:**
```bash
.\bin\mangahub.exe sync status
```

**Note:** Always use `.\bin\mangahub.exe` if mangahub is not in your PATH.

**Expected Output (Connected):**
```
TCP Sync Status:

  Connection: ✓ Active
  Server: localhost:9090
  Uptime: 2h 15m 30s
  Last heartbeat: 2 seconds ago

Session Info:
  User: johndoe
  Session ID: sess_abc123xyz
  Device: Work Laptop (desktop)

Sync Statistics:
  Messages sent: 47
  Messages received: 23
  Last sync: 30 seconds ago
  Sync conflicts: 0

Network Quality: Good
```

**Expected Output (Disconnected):**
```
TCP Sync Status:

  Connection: ✗ Inactive

To connect: mangahub sync connect
```

---

### `mangahub sync monitor`
Displays real-time synchronization updates as they happen.

**Usage:**
```bash
.\bin\mangahub.exe sync monitor
```

**Note:** Always use `.\bin\mangahub.exe` if mangahub is not in your PATH.

**Expected Output:**
```
Monitoring real-time sync updates... (Press Ctrl+C to exit)

[17:05:12] ← Device 'mobile' updated: Jujutsu Kaisen → Chapter 248
[17:05:45] → Broadcasting update: Attack on Titan → Chapter 90
[17:06:23] ← Device 'web' updated: Demon Slayer → Chapter 157
[17:07:01] ← Device 'mobile' updated: One Piece → Chapter 1096
[17:07:35] → Broadcasting update: One Piece → Chapter 1096 (sync conflict resolved)

Real-time sync monitoring active. Updates appear as they happen.
```

**Legend:**
- `←` Incoming update from another device
- `→` Outgoing update from this device

---

## How It Works

### Connection Lifecycle
1. **Connect**: Establishes TCP connection and authenticates
2. **Heartbeat**: Automatic keep-alive messages every 30 seconds
3. **Sync**: Progress updates are sent and received in real-time
4. **Disconnect**: Graceful shutdown or automatic on connection loss

### Multi-Device Synchronization
- Multiple devices can connect simultaneously
- Each device gets a unique session ID
- Progress updates are broadcast to all connected devices
- Conflicts are resolved using configured strategy (default: last_write_wins)

### Connection States
- **Active**: Connected and receiving heartbeats
- **Inactive**: Not connected to sync server
- **Stale**: Connected but no recent heartbeat (may indicate network issues)

---

## Configuration

### Config File (`~/.mangahub/config.yaml`)
```yaml
sync:
  auto_sync: true                      # Enable automatic synchronization
  auto_connect: true                   # Connect on startup
  conflict_resolution: last_write_wins # Conflict resolution strategy
  heartbeat_interval: 30s              # How often to send heartbeats
  heartbeat_timeout: 90s               # Timeout for connection
  reconnect_attempts: 5                # Number of reconnect attempts
  reconnect_delay: 5s                  # Delay between reconnect attempts
```

### State File (`~/.mangahub/sync_state.yaml`)
Automatically managed. Contains current connection state:
```yaml
active_connection:
  connected: true
  session_id: "sess_abc123xyz"
  server: "localhost:9090"
  connected_at: "2024-01-20T17:00:00Z"
  device_type: "desktop"
  device_name: "Work Laptop"
  last_heartbeat: "2024-01-20T17:15:30Z"
  pid: 12345
```

---

## Troubleshooting

### Connection Fails
```
✗ Failed to connect: connection refused
```

**Common Causes & Solutions:**
1. **TCP sync server is not running.**
   - Start it in a separate terminal:
     ```powershell
     .\bin\tcp-server.exe
     ```
2. **`mangahub` is not recognized as a command.**
   - **Always use the full path:**
     ```powershell
     .\bin\mangahub.exe sync connect [...]
     ```
   - **Or add to PATH permanently:**
     ```powershell
     .\scripts\setup\setup-path.ps1
     ```
     Then restart your terminal.
3. **Check firewall settings** (port 9090 must be open).
4. **Verify configuration:**
   ```bash
   .\bin\mangahub.exe init
   ```

### Already Connected Error
```
✗ Already connected to sync server
```

**Solution:**
Disconnect first:
```bash
.\bin\mangahub.exe sync disconnect
.\bin\mangahub.exe sync connect
```

### Stale Connection
```
Network Quality: Poor (connection may be stale)
```

**Solution:**
Reconnect to refresh:
```bash
.\bin\mangahub.exe sync disconnect
.\bin\mangahub.exe sync connect
```

### Authentication Failed
```
✗ Authentication failed
```

**Solution:**
Log in again:
```bash
.\bin\mangahub.exe auth login --username <your_username>
```

---

## Examples

### Basic Workflow
```bash
# Initialize configuration
.\bin\mangahub.exe init

# Login
.\bin\mangahub.exe auth login --username johndoe

# Connect to sync server
.\bin\mangahub.exe sync connect

# Check status
.\bin\mangahub.exe sync status

# Update progress (will sync automatically)
.\bin\mangahub.exe progress update --manga-id manga_123 --chapter 50

# Monitor updates from other devices
.\bin\mangahub.exe sync monitor

# Disconnect when done
.\bin\mangahub.exe sync disconnect
```

### Multiple Devices
```bash
# Desktop
.\bin\mangahub.exe sync connect --device-type desktop --device-name "Work Laptop"

# Mobile (different device)
.\bin\mangahub.exe sync connect --device-type mobile --device-name "iPhone"

# Web (another device)
.\bin\mangahub.exe sync connect --device-type web --device-name "Browser"
```

All three devices will receive real-time updates when any of them changes manga progress.

---

## Technical Details

### Protocol
- Uses JSON-based messaging over TCP
- Line-delimited messages (newline separator)
- Binary-safe for future enhancements

### Message Types
- `auth` - Authentication with JWT token
- `connect` - Initialize sync session with device info
- `disconnect` - Graceful disconnect
- `heartbeat` - Keep-alive message
- `sync_progress` - Progress update
- `status_request` - Request connection statistics
- `subscribe_updates` - Start monitoring updates
- `update_event` - Real-time update notification

### Security
- JWT token-based authentication
- Token required before any sync operations
- Connections timeout after 90 seconds without heartbeat

---

## Future Enhancements

### Planned Features
- [ ] Automatic reconnection on network failure
- [ ] Offline mode with queue and sync on reconnect
- [ ] Custom conflict resolution strategies
- [ ] Device-specific settings (e.g., sync only certain manga)
- [ ] Bandwidth optimization for mobile devices
- [ ] End-to-end encryption for sensitive data
- [ ] Push notifications for important events

---

## FAQ

**Q: How many devices can I connect simultaneously?**  
A: Unlimited. The server supports multiple concurrent connections per user.

**Q: What happens if I lose network connectivity?**  
A: The connection will timeout after 90 seconds. You'll need to reconnect manually.

**Q: Can I use sync without staying connected all the time?**  
A: Currently, sync only works when connected. Offline queuing is planned for future versions.

**Q: Does sync work across different platforms?**  
A: Yes! The CLI works on Windows, macOS, and Linux. Sync works across all platforms.

**Q: How much bandwidth does sync use?**  
A: Very little. Only small JSON messages are sent (typically <1KB per update). Heartbeats are sent every 30 seconds.

**Q: What if two devices update the same manga at the same time?**  
A: The configured conflict resolution strategy is used (default: last write wins).

---

## Support

For issues or questions:
1. Check this documentation
2. Review the troubleshooting section
3. Check server logs: `~/.mangahub/logs/`
4. Open an issue on GitHub

---

**Last Updated**: November 10, 2025  
**Version**: 1.0.0
