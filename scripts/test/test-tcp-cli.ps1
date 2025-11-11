# Simplified TCP CLI Testing Script
Write-Host "=== TCP CLI Testing Script ===" -ForegroundColor Cyan
Write-Host ""

# Step 1: Check API server
Write-Host "Step 1: Checking API server..." -ForegroundColor Yellow
try {
    $health = Invoke-WebRequest -Uri "http://localhost:8080/health" -UseBasicParsing -TimeoutSec 2
    if ($health.StatusCode -eq 200) {
        Write-Host "OK API server running" -ForegroundColor Green
    }
}
catch {
    Write-Host "ERROR API server not running" -ForegroundColor Red
    exit 1
}

# Step 2: Register user
Write-Host ""
Write-Host "Step 2: Registering user..." -ForegroundColor Yellow
$registerBody = '{"username":"testuser","email":"test@example.com","password":"TestPass123"}'
try {
    $regResponse = Invoke-WebRequest -Uri "http://localhost:8080/auth/register" -Method POST -Body $registerBody -ContentType "application/json" -UseBasicParsing
    Write-Host "OK User registered" -ForegroundColor Green
}
catch {
    if ($_.Exception.Response.StatusCode -eq 400) {
        Write-Host "OK User already exists" -ForegroundColor Yellow
    }
    else {
        Write-Host "ERROR Registration failed" -ForegroundColor Red
    }
}

# Step 3: Login
Write-Host ""
Write-Host "Step 3: Logging in..." -ForegroundColor Yellow
$loginBody = '{"username":"testuser","password":"TestPass123"}'
try {
    $loginResponse = Invoke-WebRequest -Uri "http://localhost:8080/auth/login" -Method POST -Body $loginBody -ContentType "application/json" -UseBasicParsing
    $loginData = $loginResponse.Content | ConvertFrom-Json
    $token = $loginData.token
    
    # Update config
    $configPath = ".\.mangahub\config.yaml"
    $config = Get-Content $configPath -Raw
    $config = $config -replace 'token: """"', "token: ""$token"""
    $config = $config -replace 'username: """"', 'username: ""testuser""'
    Set-Content -Path $configPath -Value $config
    
    Write-Host "OK Login successful, token saved" -ForegroundColor Green
}
catch {
    Write-Host "ERROR Login failed" -ForegroundColor Red
    exit 1
}

# Step 4: Check TCP server
Write-Host ""
Write-Host "Step 4: Checking TCP server..." -ForegroundColor Yellow
$tcpListener = Get-NetTCPConnection -LocalPort 9090 -ErrorAction SilentlyContinue | Where-Object { $_.State -eq 'Listen' }
if ($tcpListener) {
    Write-Host "OK TCP server listening on port 9090" -ForegroundColor Green
}
else {
    Write-Host "Starting TCP server..." -ForegroundColor Yellow
    $tcpProcess = Start-Process -FilePath ".\bin\tcp-server.exe" -PassThru -WindowStyle Hidden
    Start-Sleep -Seconds 3
    Write-Host "OK TCP server started (PID: $($tcpProcess.Id))" -ForegroundColor Green
}

# Step 5: Test sync status
Write-Host ""
Write-Host "Step 5: Testing sync status command..." -ForegroundColor Yellow
$status = & ".\bin\mangahub.exe" sync status 2>&1 | Out-String
Write-Host $status
if ($status -match "Inactive" -or $status -match "Active") {
    Write-Host "OK Sync status command working" -ForegroundColor Green
}

Write-Host ""
Write-Host "=== Setup Complete ===" -ForegroundColor Cyan
Write-Host ""
Write-Host "To test TCP connection:" -ForegroundColor White
Write-Host "  1. Open NEW terminal: .\bin\mangahub.exe sync connect" -ForegroundColor Gray
Write-Host "  2. Keep it open, then in another terminal: .\bin\mangahub.exe sync status" -ForegroundColor Gray
Write-Host "  3. Should show 'Active' with heartbeat info" -ForegroundColor Gray
Write-Host ""
