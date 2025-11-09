# remove-firewall-rules.ps1
# This script removes Windows Firewall rules added by add-firewall-rules.ps1
# Run this with Administrator privileges

# Check if running as Administrator
$isAdmin = ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)

if (-not $isAdmin) {
    Write-Host "ERROR: This script must be run as Administrator!" -ForegroundColor Red
    Write-Host ""
    Write-Host "To run as Administrator:" -ForegroundColor Yellow
    Write-Host "  1. Right-click PowerShell" -ForegroundColor Yellow
    Write-Host "  2. Select 'Run as Administrator'" -ForegroundColor Yellow
    Write-Host "  3. Navigate to this directory: cd '$PWD'" -ForegroundColor Yellow
    Write-Host "  4. Run: .\remove-firewall-rules.ps1" -ForegroundColor Yellow
    Write-Host ""
    exit 1
}

Write-Host "====================================" -ForegroundColor Cyan
Write-Host "  Remove Firewall Rules for Tests" -ForegroundColor Cyan
Write-Host "====================================" -ForegroundColor Cyan
Write-Host ""

# Provide contextual info if run from outside scripts/firewall
if (-not (Test-Path (Join-Path (Get-Location) 'scripts'))) {
    Write-Host "Hint: You can run this from repository root: .\\scripts\\firewall\\remove-firewall-rules.ps1" -ForegroundColor DarkCyan
    Write-Host "" 
}

$ruleNames = @(
    "MangaHub Bridge Tests",
    "MangaHub TCP Tests",
    "MangaHub Logger Tests"
)

$rulesRemoved = 0
$rulesNotFound = 0

foreach ($ruleName in $ruleNames) {
    Write-Host "Processing: $ruleName" -ForegroundColor White
    
    $existingRule = Get-NetFirewallRule -DisplayName $ruleName -ErrorAction SilentlyContinue
    
    if ($existingRule) {
        try {
            Remove-NetFirewallRule -DisplayName $ruleName
            Write-Host "  [REMOVED] Firewall rule deleted" -ForegroundColor Green
            $rulesRemoved++
        }
        catch {
            Write-Host "  [FAIL] Could not remove rule: $_" -ForegroundColor Red
        }
    }
    else {
        Write-Host "  [NOT FOUND] Rule does not exist" -ForegroundColor Yellow
        $rulesNotFound++
    }
    
    Write-Host ""
}

# Summary
Write-Host "====================================" -ForegroundColor Cyan
Write-Host "  Summary" -ForegroundColor Cyan
Write-Host "====================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "Rules Removed:  $rulesRemoved" -ForegroundColor $(if ($rulesRemoved -gt 0) { "Green" } else { "White" })
Write-Host "Rules Not Found: $rulesNotFound" -ForegroundColor $(if ($rulesNotFound -gt 0) { "Yellow" } else { "White" })
Write-Host ""

if ($rulesRemoved -gt 0) {
    Write-Host "Firewall rules removed successfully!" -ForegroundColor Green
}
else {
    Write-Host "No firewall rules were removed." -ForegroundColor Yellow
}
