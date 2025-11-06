Write-Host "=== MangaHub CLI Test ===" -ForegroundColor Cyan
Write-Host ""

# Test 1: Check version
Write-Host "Test 1: Check version" -ForegroundColor Yellow
.\bin\mangahub.exe --version
if ($LASTEXITCODE -eq 0) {
    Write-Host "✓ Version check passed" -ForegroundColor Green
} else {
    Write-Host "✗ Version check failed" -ForegroundColor Red
    exit 1
}
Write-Host ""


# Test 2: Check help
Write-Host "Test 2: Check help" -ForegroundColor Yellow
.\bin\mangahub.exe --help | Select-Object -First 1
if ($LASTEXITCODE -eq 0) {
    Write-Host "✓ Help command passed" -ForegroundColor Green
} else {
    Write-Host "✗ Help command failed" -ForegroundColor Red
    exit 1
}
Write-Host ""

# Test 3: Check auth command
Write-Host "Test 3: Check auth command" -ForegroundColor Yellow
.\bin\mangahub.exe auth --help | Select-Object -First 1
if ($LASTEXITCODE -eq 0) {
    Write-Host "✓ Auth command passed" -ForegroundColor Green
} else {
    Write-Host "✗ Auth command failed" -ForegroundColor Red
    exit 1
}
Write-Host ""

# Test 4: Check configuration
Write-Host "Test 4: Check configuration" -ForegroundColor Yellow
$configPath = "$env:USERPROFILE\.mangahub\config.yaml"
if (Test-Path $configPath) {
    Write-Host "✓ Configuration file exists at: $configPath" -ForegroundColor Green
} else {
    Write-Host "⚠ Configuration not initialized. Run: .\bin\mangahub.exe init" -ForegroundColor Yellow
}
Write-Host ""

Write-Host "=== All Tests Passed ===" -ForegroundColor Green
Write-Host ""
Write-Host "Next steps:" -ForegroundColor Cyan
Write-Host "  1. Initialize (if not done): .\bin\mangahub.exe init" -ForegroundColor White
Write-Host "  2. Start server: go run cmd/api-server/main.go" -ForegroundColor White
Write-Host "  3. Test registration: .\bin\mangahub.exe auth register --username testuser --email test@example.com" -ForegroundColor White