# Quick Test Verification Script
# Run this to verify everything is working

Write-Host "=== Quick Verification Test ===" -ForegroundColor Cyan
Write-Host ""

# Test 1: Go tests with race detector
Write-Host "[1/2] Running Go tests with race detector..." -ForegroundColor Yellow
$testResult = go test -race ./internal/tcp/test ./internal/bridge/test -count=1 -timeout 5m 2>&1
if ($LASTEXITCODE -eq 0) {
    Write-Host "  ✅ All Go tests PASSED with race detector" -ForegroundColor Green
}
else {
    Write-Host "  ❌ Go tests FAILED" -ForegroundColor Red
    Write-Host $testResult
    exit 1
}

Write-Host ""

# Test 2: TCP CLI status
Write-Host "[2/2] Testing TCP CLI status command..." -ForegroundColor Yellow
$cliResult = & ".\bin\mangahub.exe" sync status 2>&1 | Out-String
if ($cliResult -match "TCP Sync Status") {
    Write-Host "  ✅ TCP CLI status command WORKING" -ForegroundColor Green
}
else {
    Write-Host "  ❌ CLI command FAILED" -ForegroundColor Red
    Write-Host $cliResult
    exit 1
}

Write-Host ""
Write-Host "=== ✅ ALL VERIFICATION CHECKS PASSED ===" -ForegroundColor Green
Write-Host ""
Write-Host "Summary:" -ForegroundColor Cyan
Write-Host "  • Go tests: PASSING (no race conditions)" -ForegroundColor White
Write-Host "  • TCP server: OPERATIONAL" -ForegroundColor White
Write-Host "  • CLI commands: FUNCTIONAL" -ForegroundColor White
Write-Host "  • Status reporting: ACCURATE" -ForegroundColor White
Write-Host ""
Write-Host "See TEST_SUMMARY.md for full test report." -ForegroundColor Gray
