$binPath = Join-Path $PSScriptRoot "bin"

if ($env:PATH -notlike "*$binPath*") {
    $env:PATH = "$binPath;$env:PATH"
    Write-Host "✓ Added $binPath to PATH" -ForegroundColor Green
    Write-Host ""
    Write-Host "You can now use 'mangahub' command directly!" -ForegroundColor Cyan
    Write-Host "Example: mangahub --help" -ForegroundColor White
    Write-Host ""
    Write-Host "Note: This is temporary. To make it permanent:" -ForegroundColor Yellow
    Write-Host "  1. Open System Properties > Environment Variables" -ForegroundColor White
    Write-Host "  2. Edit PATH variable" -ForegroundColor White
    Write-Host "  3. Add: $binPath" -ForegroundColor White
} else {
    Write-Host "✓ PATH already contains bin directory" -ForegroundColor Green
}

Write-Host ""
Write-Host "Testing mangahub command..." -ForegroundColor Cyan
mangahub --version