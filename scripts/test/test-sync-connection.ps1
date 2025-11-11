# TCP Sync Connection Test
Write-Host "Starting TCP sync connect in background..." -ForegroundColor Yellow

# Start connect command in background
$connectJob = Start-Job -ScriptBlock {
    Set-Location "D:\GitHub\Manga-Hub-Group13"
    & ".\bin\mangahub.exe" sync connect --device-type desktop --device-name TestDevice
}

Write-Host "Waiting for connection to establish (5 seconds)..." -ForegroundColor Yellow
Start-Sleep -Seconds 5

# Check status
Write-Host ""
Write-Host "Checking sync status:" -ForegroundColor Cyan
& ".\bin\mangahub.exe" sync status

# Wait a bit more
Start-Sleep -Seconds 2

# Check status again
Write-Host ""
Write-Host "Checking status again after heartbeat:" -ForegroundColor Cyan
& ".\bin\mangahub.exe" sync status

# Check job output
Write-Host ""
Write-Host "Connection output:" -ForegroundColor Cyan
$jobOutput = Receive-Job -Job $connectJob
if ($jobOutput) {
    Write-Host $jobOutput
}

# Keep job running for a bit
Write-Host ""
Write-Host "Connection will remain active for 10 more seconds..." -ForegroundColor Yellow
Start-Sleep -Seconds 10

# Final status check
Write-Host ""
Write-Host "Final status check:" -ForegroundColor Cyan
& ".\bin\mangahub.exe" sync status

# Stop the job
Write-Host ""
Write-Host "Stopping connection..." -ForegroundColor Yellow
Stop-Job -Job $connectJob
Remove-Job -Job $connectJob

Write-Host ""
Write-Host "Test complete!" -ForegroundColor Green
