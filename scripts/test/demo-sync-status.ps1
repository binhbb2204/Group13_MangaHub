#!/usr/bin/env pwsh

Write-Host "=== TCP Sync Status Demo ===" -ForegroundColor Cyan
Write-Host ""

$ProjectRoot = Split-Path -Parent (Split-Path -Parent (Split-Path -Parent $PSScriptRoot))
$MangaHubExe = Join-Path $ProjectRoot "bin\mangahub.exe"

if (-not (Test-Path $MangaHubExe)) {
    Write-Host "Error: mangahub.exe not found at: $MangaHubExe" -ForegroundColor Red
    Write-Host "Run: go build -o bin/mangahub.exe ./cmd/main.go" -ForegroundColor Yellow
    exit 1
}

Write-Host "Checking TCP sync status..." -ForegroundColor Green
Write-Host ""

& $MangaHubExe sync status

Write-Host ""
Write-Host "=== Demo Complete ===" -ForegroundColor Cyan
Write-Host ""
Write-Host "What you see above:" -ForegroundColor Yellow
Write-Host "  - Live server statistics (if connected)" -ForegroundColor White
Write-Host "  - Cached data fallback (if server unreachable)" -ForegroundColor White
Write-Host "  - Multi-device count" -ForegroundColor White
Write-Host "  - Network quality and RTT" -ForegroundColor White
Write-Host "  - Last sync details with manga title" -ForegroundColor White
Write-Host ""
Write-Host "To connect: $MangaHubExe sync connect" -ForegroundColor Cyan
