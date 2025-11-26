param(
    [string]$ServerURL = "ws://localhost:9093/ws/chat",
    [string]$Token1 = "",
    [string]$Token2 = ""
)

if ($Token1 -eq "" -or $Token2 -eq "") {
    Write-Host "Error: Two tokens required. Use -Token1 and -Token2 parameters" -ForegroundColor Red
    exit 1
}

Write-Host "Starting Demo Chat Session..." -ForegroundColor Cyan
Write-Host "User 1 (Token1) vs User 2 (Token2)" -ForegroundColor Yellow
Write-Host "-----------------------------------"

$script = @'
param($url, $token, $name, $messages)
Add-Type -AssemblyName System.Net.WebSockets
Add-Type -AssemblyName System.Threading.Tasks

$ws = New-Object System.Net.WebSockets.ClientWebSocket
$cts = New-Object System.Threading.CancellationTokenSource
$uri = [Uri]"$url?token=$token"

try {
    $ws.ConnectAsync($uri, $cts.Token).Wait()
    
    foreach ($msgText in $messages) {
        Start-Sleep -Seconds (Get-Random -Minimum 1 -Maximum 3)
        
        # Send typing indicator
        $typing = "{`"type`":`"typing`",`"room`":`"global`"}"
        $bytes = [System.Text.Encoding]::UTF8.GetBytes($typing)
        $segment = New-Object System.ArraySegment[byte] -ArgumentList @(,$bytes)
        $ws.SendAsync($segment, [System.Net.WebSockets.WebSocketMessageType]::Text, $true, $cts.Token).Wait()
        
        Start-Sleep -Milliseconds 500
        
        # Send message
        $msg = "{`"type`":`"text`",`"content`":`"$msgText`",`"room`":`"global`"}"
        $bytes = [System.Text.Encoding]::UTF8.GetBytes($msg)
        $segment = New-Object System.ArraySegment[byte] -ArgumentList @(,$bytes)
        $ws.SendAsync($segment, [System.Net.WebSockets.WebSocketMessageType]::Text, $true, $cts.Token).Wait()
        
        Write-Host "[$name] Sent: $msgText" -ForegroundColor Green
    }
    
    Start-Sleep -Seconds 2
    $ws.CloseAsync([System.Net.WebSockets.WebSocketCloseStatus]::NormalClosure, "Done", $cts.Token).Wait()
} catch {
    Write-Host "[$name] Error: $_" -ForegroundColor Red
}
'@

$msgs1 = @("Hello there!", "How is the weather?", "That sounds nice.", "Bye!")
$msgs2 = @("Hi!", "It's sunny here.", "Yeah, I love it.", "See ya!")

$job1 = Start-Job -ScriptBlock ([scriptblock]::Create($script)) -ArgumentList $ServerURL, $Token1, "User1", $msgs1
$job2 = Start-Job -ScriptBlock ([scriptblock]::Create($script)) -ArgumentList $ServerURL, $Token2, "User2", $msgs2

Write-Host "Chat simulation running in background..." -ForegroundColor Cyan

$job1, $job2 | Wait-Job | Receive-Job
$job1, $job2 | Remove-Job

Write-Host "Demo complete!" -ForegroundColor Green
