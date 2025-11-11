# TCP CLI Testing Guide (Shortened Version)

## Prerequisites
- Manga Hub built and ready (`.\bin\tcp-server.exe` and `.\bin\mangahub.exe`)
- 3 separate PowerShell terminals

---

## Setup (One-Time)

### Add binaries to PATH (Optional but Recommended)
In any terminal:
```powershell
$env:PATH = "<your-project-path>\bin;$env:PATH"
```
Example:
```powershell
$env:PATH = "D:\GitHub\Manga-Hub-Group13\bin;$env:PATH"
```

Now you can use `mangahub` instead of `.\bin\mangahub.exe`

### Login with Authentication
```powershell
mangahub auth login --username YourUsername
```
You'll be prompted to enter your password.

**Expected output:**
```
✓ Login successful
Token saved to config
```

⚠️ **Important:** You must be logged in before connecting to TCP sync!

---

## Quick Start (3 Steps)

### Step 1: Start TCP Server
**Terminal 1** - Run and leave open:
```powershell
.\bin\tcp-server.exe
```
Expected output: `tcp_server_ready port:9090`

---

### Step 2: Connect Client
**Terminal 2** - Run and **keep this terminal open**:
```powershell
.\bin\mangahub.exe sync connect --device-type desktop --device-name "YourName"
```
Or if you set PATH:
```powershell
mangahub sync connect --device-type desktop --device-name "YourName"
```

**What you'll see:**
```
✓ Connected successfully!
Session ID: sess_yourname_desktop_10112025T214359_a7f3
```

⚠️ **Important:** Don't close this terminal - it maintains your connection!

**Optional Flags:**
- `--device-type` → `mobile`, `desktop`, or `web` (default: `desktop`)
- `--device-name` → Friendly device name (default: your hostname)

---

### Step 3: Check Connection Status
**Terminal 3** - Run anytime:
```powershell
.\bin\mangahub.exe sync status
```
Or with PATH:
```powershell
mangahub sync status
```

**Expected output:**
```
TCP Sync Status:
  Connection: ✓ Active
  Server: localhost:9090
  Session ID: sess_yourname_desktop_10112025T214359_a7f3
  Network Quality: Good
```

---

## Commands Summary

| Command | Description |
|---------|-------------|
| `mangahub auth login --username <name>` | Login (required before sync) |
| `.\bin\tcp-server.exe` | Start TCP sync server |
| `mangahub sync connect` | Connect to server |
| `mangahub sync status` | Check connection status |
| `Ctrl+C` (in Terminal 2) | Disconnect gracefully |

---

## Troubleshooting

**Problem:** "Not logged in" error  
**Solution:** Run `mangahub auth login --username YourUsername` first

**Problem:** Connection shows "✗ Inactive"  
**Solution:** Make sure Terminal 2 (sync connect) is still running

**Problem:** Cannot connect to server  
**Solution:** Check if Terminal 1 (tcp-server) is running on port 9090

**Problem:** Session ID looks weird  
**Solution:** Make sure you rebuilt binaries after latest code changes:
```powershell
go build -o bin/tcp-server.exe cmd/tcp-server/main.go
go build -o bin/mangahub.exe cmd/main.go
```

**Problem:** "mangahub: command not found"  
**Solution:** Either set PATH or use `.\bin\mangahub.exe` instead

---

## Example Session IDs

- `sess_thuannm_desktop_10112025T214359_b7bb`
- `sess_johns_laptop_mobile_10112025T215530_a3f2`

---

## What Happens Behind the Scenes

1. **Authentication** - JWT token generated and stored in config
2. **TCP Server** listens on port 9090
3. **Client connects** and authenticates with JWT token
4. **Session created** with unique ID
5. **Heartbeats sent** every 30 seconds to keep connection alive
6. **Connection state saved** to `.mangahub/sync_state.yaml`
7. **Status command** reads the state file from another process

---

## Next Steps

- Test progress synchronization
- Test multi-device sync
- Monitor real-time updates: `mangahub sync monitor`

For detailed testing guide, see `TCPCLI_FullGuide.md`