# Two-PC Setup Guide

This guide explains how to set up the MangaHub project across two separate computers (PC1 for backend, PC2 for frontend).

## Prerequisites

- PC1 and PC2 must be on the **same local network** (same WiFi/LAN)
- Both PCs should have Git, Go, and Node.js installed

---

## PC1 Setup (Backend Server)

### Step 1: Get PC1's Local IP Address

**On Windows:**
```powershell
ipconfig
```
Look for `IPv4 Address` under your active network adapter (e.g., `192.168.1.100`)

**On macOS/Linux:**
```bash
ifconfig
# or
ip addr show
```

**Note this IP address** - you'll need it for PC2 configuration.

### Step 2: Configure Backend .env File

Navigate to the backend directory:
```powershell
cd d:\GitHub\Manga-Hub-Group13
```

Create or edit `.env` file with the following configuration:

```env
# Server Configuration
HOST=0.0.0.0  # Important: Bind to all network interfaces
API_PORT=8080
UDP_PORT=8081
TCP_PORT=8082
WEBSOCKET_PORT=8083
GRPC_PORT=50051

# Database (if applicable)
DB_HOST=localhost
DB_PORT=5432
DB_NAME=mangahub
DB_USER=your_db_user
DB_PASSWORD=your_db_password

# CORS Configuration
ALLOWED_ORIGINS=http://localhost:3000,http://PC2_IP_ADDRESS:3000

# Other configurations
JWT_SECRET=your_secret_key
```

**Replace `PC2_IP_ADDRESS`** with PC2's actual IP address (found using same method as Step 1).

### Step 3: Configure Windows Firewall (PC1)

Allow incoming connections on the required ports:

```powershell
# Allow API Server
New-NetFirewallRule -DisplayName "MangaHub API" -Direction Inbound -Protocol TCP -LocalPort 8080 -Action Allow

# Allow UDP Server
New-NetFirewallRule -DisplayName "MangaHub UDP" -Direction Inbound -Protocol UDP -LocalPort 8081 -Action Allow

# Allow TCP Server
New-NetFirewallRule -DisplayName "MangaHub TCP" -Direction Inbound -Protocol TCP -LocalPort 8082 -Action Allow

# Allow WebSocket Server
New-NetFirewallRule -DisplayName "MangaHub WebSocket" -Direction Inbound -Protocol TCP -LocalPort 8083 -Action Allow

# Allow gRPC Server
New-NetFirewallRule -DisplayName "MangaHub gRPC" -Direction Inbound -Protocol TCP -LocalPort 50051 -Action Allow
```

### Step 4: Start Backend Servers (PC1)

Run all backend servers:

```powershell
# In separate terminal windows, run:
go run cmd/api-server/main.go
go run cmd/udp-server/main.go
go run cmd/tcp-server/main.go
go run cmd/websocket-server/main.go
go run cmd/grpc-server/main.go
```

Verify all servers are running and listening on `0.0.0.0` (accepting external connections).

---

## PC2 Setup (Frontend)

### Step 1: Configure Frontend .env File

Navigate to the frontend directory:
```powershell
cd d:\GitHub\group13_mangaHub_frontend
```

Create or edit `.env` file:

```env
# Replace PC1_IP_ADDRESS with PC1's actual IP address
REACT_APP_API_URL=http://PC1_IP_ADDRESS:8080
REACT_APP_WEBSOCKET_URL=ws://PC1_IP_ADDRESS:8083
REACT_APP_GRPC_URL=http://PC1_IP_ADDRESS:50051
```

**Example** (if PC1's IP is `192.168.1.100`):
```env
REACT_APP_API_URL=http://192.168.1.100:8080
REACT_APP_WEBSOCKET_URL=ws://192.168.1.100:8083
REACT_APP_GRPC_URL=http://192.168.1.100:50051
```

### Step 2: Install Dependencies (if not already done)

```powershell
npm install
```

### Step 3: Start Frontend Development Server

```powershell
npm run start
```

The frontend should start on `http://localhost:3000` (on PC2) and connect to backend on PC1.

---

## Verification Steps

1. **On PC2**, open a browser and navigate to `http://localhost:3000`
2. Open browser DevTools (F12) → Console tab
3. Check for successful API/WebSocket connections to PC1
4. Test functionality (login, search, real-time features)
5. In PC1's terminal, verify incoming requests are being logged

---

## Troubleshooting

### Issue: Cannot connect from PC2 to PC1

**Solutions:**
- Verify both PCs are on the same network
- Double-check PC1's IP address hasn't changed (routers may reassign IPs)
- Ensure Windows Firewall rules are active on PC1
- Try temporarily disabling PC1's firewall to test connectivity
- Ping PC1 from PC2: `ping PC1_IP_ADDRESS`

### Issue: CORS errors in browser console

**Solution:**
- Verify `ALLOWED_ORIGINS` in PC1's `.env` includes PC2's IP
- Restart backend servers after changing `.env`

### Issue: WebSocket connection fails

**Solution:**
- Check WebSocket URL uses `ws://` (not `wss://`)
- Verify port 8083 is accessible from PC2
- Check browser console for specific WebSocket error messages

### Issue: Environment variables not updating

**Solution:**
- **Frontend**: Delete `node_modules/.cache` and restart dev server
- **Backend**: Restart all Go servers after `.env` changes

---

## Notes

- **Static IP Recommendation**: Consider setting static IP addresses for both PCs to avoid reconfiguration when IPs change
- **Production**: This setup is for development only. Production deployments require proper security configurations
- **Database**: If using a database on PC1, ensure it's also configured to accept connections from PC2's IP
