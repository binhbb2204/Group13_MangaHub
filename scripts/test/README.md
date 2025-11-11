# Test Scripts Documentation

This directory contains PowerShell test scripts for validating MangaHub functionality.

## Available Scripts

### 1. `test-tcp-cli.ps1`
**Purpose:** Automated setup and basic validation of TCP CLI infrastructure

**What it does:**
- Checks if API server is running on port 8080
- Registers a test user (testuser@example.com)
- Authenticates and obtains JWT token
- Starts TCP server on port 9090 if not already running
- Tests the `mangahub sync status` command

**Usage:**
```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\test\test-tcp-cli.ps1
```

**Expected output:**
- ✅ API server running
- ✅ User registered/logged in
- ✅ TCP server listening
- ✅ Sync status command working

---

### 2. `test-sync-connection.ps1`
**Purpose:** Comprehensive TCP sync connection testing

**What it does:**
- Establishes a TCP sync connection in the background
- Monitors connection status over time
- Validates heartbeat mechanism
- Tests network quality reporting
- Gracefully disconnects

**Usage:**
```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\test\test-sync-connection.ps1
```

**Expected output:**
- Connection established with session ID
- Status shows "Active" with heartbeat tracking
- Network quality assessed
- Clean disconnect

---

### 3. `verify-tests.ps1`
**Purpose:** Quick verification that all systems are operational

**What it does:**
- Runs Go test suite with race detector on critical packages
- Validates TCP CLI status command
- Provides summary report

**Usage:**
```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\test\verify-tests.ps1
```

**Expected output:**
- ✅ All Go tests PASSED
- ✅ TCP CLI status command WORKING
- Summary of system status

---

### 4. `test-cli.ps1` (Existing)
**Purpose:** CLI command testing

See the script for details on CLI command validation.

---

### 5. `test-code.ps1` (Existing)
**Purpose:** Go code testing

Runs the full Go test suite across all packages.

---

## Prerequisites

Before running these scripts, ensure:

1. **Go is installed** and available in PATH
2. **Build the binaries:**
   ```powershell
   go build -o bin/api-server.exe cmd/api-server/main.go
   go build -o bin/tcp-server.exe cmd/tcp-server/main.go
   go build -o bin/mangahub.exe cmd/main.go
   ```

3. **Initialize MangaHub:**
   ```powershell
   .\bin\mangahub.exe init
   ```

4. **Firewall rules** (if needed):
   ```powershell
   .\scripts\firewall\add-firewall-rules.ps1
   ```

## Test Workflow

### Quick Verification (Recommended)
```powershell
# Run this first to check everything is working
.\scripts\test\verify-tests.ps1
```

### Full TCP Testing
```powershell
# Step 1: Setup infrastructure
.\scripts\test\test-tcp-cli.ps1

# Step 2: Test TCP connection lifecycle
.\scripts\test\test-sync-connection.ps1
```

### Complete Test Suite
```powershell
# Run all Go tests
.\scripts\test\test-code.ps1
```

## Troubleshooting

### Port Already in Use
If you see "port already in use" errors:
```powershell
# Find and kill process on port 8080 (API server)
netstat -ano | findstr :8080
Stop-Process -Id <PID> -Force

# Find and kill process on port 9090 (TCP server)
netstat -ano | findstr :9090
Stop-Process -Id <PID> -Force
```

### API Server Not Responding
```powershell
# Check if server is running
.\bin\mangahub.exe server status

# Start manually if needed
.\bin\api-server.exe
```

### TCP Server Not Starting
```powershell
# Check logs
Get-Content .\.mangahub\logs\tcp-server.log -Tail 20

# Check if port is available
Test-NetConnection -ComputerName localhost -Port 9090
```

### Authentication Failures
```powershell
# Verify config file exists
Get-Content .\.mangahub\config.yaml

# Re-login if token expired
# (Manual login not supported via CLI; use test scripts)
```

## Test Results

For detailed test results and analysis, see:
- `docs/TEST_SUMMARY.md` - Comprehensive test report
- `docs/TCPCLI_ShortGuide.md` - TCP CLI user guide
- `docs/TestGuide.md` - General testing guide

## CI/CD Integration

To integrate these tests into a CI/CD pipeline:

```yaml
# Example GitHub Actions workflow
- name: Run Go Tests
  run: go test -race ./... -count=1 -timeout 10m

- name: Build Binaries
  run: |
    go build -o bin/api-server.exe cmd/api-server/main.go
    go build -o bin/tcp-server.exe cmd/tcp-server/main.go
    go build -o bin/mangahub.exe cmd/main.go

- name: Verify Tests
  run: powershell -ExecutionPolicy Bypass -File .\scripts\test\verify-tests.ps1
```

## Notes

- All scripts require PowerShell 5.1 or later
- Scripts use `-ExecutionPolicy Bypass` for automation
- Test user credentials: `testuser` / `TestPass123`
- Logs are stored in `.mangahub/logs/` directory
- Test artifacts are cleaned up automatically

---

**Last Updated:** November 11, 2025  
**Maintainer:** MangaHub Development Team
