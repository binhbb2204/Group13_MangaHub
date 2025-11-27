param(
    [string]$ServerURL = "ws://localhost:9093/ws/chat",
    [string]$Token = ""
)

if ($Token -eq "") {
    Write-Host "Error: Token required. Use -Token parameter" -ForegroundColor Red
    Write-Host "Example: ./test-websocket.ps1 -Token <your_jwt_token>" -ForegroundColor Gray
    exit 1
}

Add-Type -AssemblyName System.Net.WebSockets
Add-Type -AssemblyName System.Threading.Tasks

$ws = New-Object System.Net.WebSockets.ClientWebSocket
$cts = New-Object System.Threading.CancellationTokenSource
$uri = [Uri]"$ServerURL?token=$Token"

try {
    Write-Host "Connecting to $uri..." -ForegroundColor Cyan
    $connectTask = $ws.ConnectAsync($uri, $cts.Token)
    $connectTask.Wait()
    
    if ($ws.State -eq [System.Net.WebSockets.WebSocketState]::Open) {
        Write-Host "Connected!" -ForegroundColor Green
    } else {
        throw "Failed to connect"
    }

    # Start a background job to read messages
    $readJob = Start-Job -ScriptBlock {
        param($ws, $cts)
        $buffer = New-Object byte[] 4096
        $segment = New-Object System.ArraySegment[byte] -ArgumentList @(,$buffer)
        
        while ($ws.State -eq [System.Net.WebSockets.WebSocketState]::Open) {
            try {
                $result = $ws.ReceiveAsync($segment, $cts.Token).Result
                if ($result.MessageType -eq [System.Net.WebSockets.WebSocketMessageType]::Close) {
                    break
                }
                
                $message = [System.Text.Encoding]::UTF8.GetString($buffer, 0, $result.Count)
                Write-Host "`nReceived: $message" -ForegroundColor Cyan
                Write-Host "Enter message (or 'quit' to exit): " -NoNewline -ForegroundColor Yellow
            } catch {
                break
            }
        }
    } -ArgumentList $ws, $cts

    # Main loop for sending messages
    while ($true) {
        $input = Read-Host "Enter message (or 'quit' to exit)"
        if ($input -eq "quit") {
            break
        }

        if ($input -ne "") {
            $msgObj = @{
                type = "text"
                content = $input
                room = "global"
            }
            $json = ConvertTo-Json $msgObj -Compress
            $bytes = [System.Text.Encoding]::UTF8.GetBytes($json)
            $segment = New-Object System.ArraySegment[byte] -ArgumentList @(,$bytes)
            
            $ws.SendAsync($segment, [System.Net.WebSockets.WebSocketMessageType]::Text, $true, $cts.Token).Wait()
            Write-Host "Sent: $json" -ForegroundColor Gray
        }
    }

} catch {
    Write-Host "Error: $_" -ForegroundColor Red
} finally {
    if ($ws.State -eq [System.Net.WebSockets.WebSocketState]::Open) {
        $ws.CloseAsync([System.Net.WebSockets.WebSocketCloseStatus]::NormalClosure, "Bye", $cts.Token).Wait()
    }
    $ws.Dispose()
    $cts.Dispose()
    Write-Host "Disconnected" -ForegroundColor Yellow
}
