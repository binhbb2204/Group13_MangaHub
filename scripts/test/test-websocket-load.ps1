param(
    [int]$Clients = 10,
    [int]$MessagesPerClient = 20,
    [string]$ServerURL = "ws://localhost:9093/ws/chat",
    [string]$Token = ""
)

if ($Token -eq "") {
    Write-Host "Error: Token required. Use -Token parameter" -ForegroundColor Red
    exit 1
}

Write-Host "WebSocket Load Test" -ForegroundColor Cyan
Write-Host "Clients: $Clients" -ForegroundColor Yellow
Write-Host "Messages per client: $MessagesPerClient" -ForegroundColor Yellow
Write-Host "Target: $ServerURL" -ForegroundColor Yellow
Write-Host ""

$script = @'
param($url, $token, $clientId, $messageCount)
Add-Type -AssemblyName System.Net.WebSockets
Add-Type -AssemblyName System.Threading.Tasks

$ws = New-Object System.Net.WebSockets.ClientWebSocket
$cts = New-Object System.Threading.CancellationTokenSource
$uri = [Uri]"$url?token=$token"

try {
    $connectTask = $ws.ConnectAsync($uri, $cts.Token)
    $connectTask.Wait()
    if (-not $connectTask.IsCompleted) { throw "Connection timeout" }
    
    $received = 0
    $buffer = New-Object byte[] 4096
    $segment = New-Object System.ArraySegment[byte] -ArgumentList @(,$buffer)
    
    $readTask = {
        param($ws, $seg, $cts)
        while ($ws.State -eq [System.Net.WebSockets.WebSocketState]::Open) {
            try {
                $result = $ws.ReceiveAsync($seg, $cts.Token)
                $result.Wait(1000) | Out-Null
                if ($result.IsCompleted) { return 1 }
            } catch { break }
        }
        return 0
    }
    
    $startTime = Get-Date
    for ($i = 1; $i -le $messageCount; $i++) {
        $msg = "{`"type`":`"text`",`"content`":`"load-test-$clientId-$i`",`"room`":`"global`"}"
        $bytes = [System.Text.Encoding]::UTF8.GetBytes($msg)
        $segment2 = New-Object System.ArraySegment[byte] -ArgumentList @(,$bytes)
        
        $sendTask = $ws.SendAsync($segment2, [System.Net.WebSockets.WebSocketMessageType]::Text, $true, $cts.Token)
        $sendTask.Wait(500) | Out-Null
        if (-not $sendTask.IsCompleted) { throw "Send timeout" }
        
        Start-Sleep -Milliseconds 50
    }
    $endTime = Get-Date
    $duration = ($endTime - $startTime).TotalSeconds
    
    $ws.CloseAsync([System.Net.WebSockets.WebSocketCloseStatus]::NormalClosure, "done", $cts.Token).Wait(1000) | Out-Null
    
    Write-Output "Client-$clientId,OK,$messageCount,$duration"
} catch {
    Write-Output "Client-$clientId,ERROR,0,0"
} finally {
    if ($ws) { $ws.Dispose() }
    if ($cts) { $cts.Dispose() }
}
'@

$jobs = @()
$startOverall = Get-Date

for ($i = 1; $i -le $Clients; $i++) {
    $job = Start-Job -ScriptBlock ([scriptblock]::Create($script)) -ArgumentList $ServerURL, $Token, $i, $MessagesPerClient
    $jobs += $job
    Start-Sleep -Milliseconds 100
}

Write-Host "Started $Clients client jobs..." -ForegroundColor Green
Write-Host "Waiting for completion..." -ForegroundColor Yellow

$results = $jobs | Wait-Job | Receive-Job
$jobs | Remove-Job

$endOverall = Get-Date
$totalDuration = ($endOverall - $startOverall).TotalSeconds

Write-Host ""
Write-Host "=== Results ===" -ForegroundColor Cyan
$successCount = 0
$errorCount = 0
$totalMessages = 0
$totalTime = 0

foreach ($line in $results) {
    $parts = $line -split ','
    if ($parts[1] -eq "OK") {
        $successCount++
        $totalMessages += [int]$parts[2]
        $totalTime += [double]$parts[3]
    }
    else {
        $errorCount++
    }
}

Write-Host "Successful clients: $successCount / $Clients" -ForegroundColor Green
Write-Host "Failed clients: $errorCount" -ForegroundColor $(if ($errorCount -eq 0) { "Green" } else { "Red" })
Write-Host "Total messages sent: $totalMessages" -ForegroundColor Yellow
Write-Host "Total duration: $([math]::Round($totalDuration, 2))s" -ForegroundColor Yellow

if ($totalTime -gt 0) {
    $avgRate = $totalMessages / $totalTime
    Write-Host "Average rate: $([math]::Round($avgRate, 2)) msgs/s" -ForegroundColor Cyan
}

Write-Host ""
Write-Host "Load test complete!" -ForegroundColor Green
