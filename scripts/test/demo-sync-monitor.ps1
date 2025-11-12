#!/usr/bin/env pwsh

Write-Host "=== TCP Sync Monitor Demo ===" -ForegroundColor Cyan
Write-Host ""

$ProjectRoot = Split-Path -Parent (Split-Path -Parent (Split-Path -Parent $PSScriptRoot))
$MangaHubExe = Join-Path $ProjectRoot "bin\mangahub.exe"

if (-not (Test-Path $MangaHubExe)) {
    Write-Host "Error: mangahub.exe not found at: $MangaHubExe" -ForegroundColor Red
    Write-Host "Run: go build -o bin/mangahub.exe ./cmd/main.go" -ForegroundColor Yellow
    exit 1
}

Write-Host "Prerequisites:" -ForegroundColor Yellow
Write-Host "  1. TCP server must be running (.\bin\tcp-server.exe)" -ForegroundColor White
Write-Host "  2. You must be connected (mangahub sync connect)" -ForegroundColor White
Write-Host ""

$response = Read-Host "Are you connected? (y/n)"
if ($response -ne "y" -and $response -ne "Y") {
    Write-Host ""
    Write-Host "Please run first:" -ForegroundColor Yellow
    Write-Host "  1. .\bin\tcp-server.exe       (in Terminal 1)" -ForegroundColor Cyan
    Write-Host "  2. $MangaHubExe sync connect  (in Terminal 2)" -ForegroundColor Cyan
    Write-Host "  3. Then run this script again" -ForegroundColor Cyan
    exit 0
}

Write-Host ""
Write-Host "Starting real-time monitor..." -ForegroundColor Green
Write-Host "You will see sync events as they happen." -ForegroundColor White
Write-Host ""
Write-Host "Legend:" -ForegroundColor Yellow
Write-Host "  ← = Update from another device" -ForegroundColor White
Write-Host "  → = Update from this device" -ForegroundColor White
Write-Host "  ⚠ = Conflict or warning" -ForegroundColor White
Write-Host ""
Write-Host "Press Ctrl+C to stop monitoring" -ForegroundColor Cyan
Write-Host ""
Write-Host "----------------------------------------" -ForegroundColor Gray
Write-Host ""

& $MangaHubExe sync monitor

Write-Host ""
Write-Host "=== Demo Complete ===" -ForegroundColor Cyan
