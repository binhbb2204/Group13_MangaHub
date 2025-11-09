# add-firewall-rules.ps1
# This script adds Windows Firewall rules to allow test binaries
# Run this ONCE with Administrator privileges

# Check if running as Administrator
$isAdmin = ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)

if (-not $isAdmin) {
    Write-Host "ERROR: This script must be run as Administrator!" -ForegroundColor Red
    Write-Host ""
    Write-Host "To run as Administrator:" -ForegroundColor Yellow
    Write-Host "  1. Right-click PowerShell" -ForegroundColor Yellow
    Write-Host "  2. Select 'Run as Administrator'" -ForegroundColor Yellow
    Write-Host "  3. Navigate to this directory: cd '$PWD'" -ForegroundColor Yellow
    Write-Host "  4. Run: .\add-firewall-rules.ps1" -ForegroundColor Yellow
    Write-Host ""
    exit 1
}

Write-Host "====================================" -ForegroundColor Cyan
Write-Host "  Add Firewall Rules for Tests" -ForegroundColor Cyan
Write-Host "====================================" -ForegroundColor Cyan
Write-Host ""

# Determine repository root (two levels up from scripts/firewall)
$repoRoot = (Get-Item $PSScriptRoot).Parent.Parent.FullName
$binPath = Join-Path $repoRoot "bin"

# Test binaries to add rules for
$testBinaries = @(
    @{
        Name = "MangaHub Bridge Tests"
        Path = Join-Path $binPath "bridge-test.exe"
    },
    @{
        Name = "MangaHub TCP Tests"
        Path = Join-Path $binPath "tcp-test.exe"
    },
    @{
        Name = "MangaHub Logger Tests"
        Path = Join-Path $binPath "logger-test.exe"
    }
)

$rulesAdded = 0
$rulesSkipped = 0
$rulesFailed = 0

foreach ($binary in $testBinaries) {
    $exePath = $binary.Path
    $ruleName = $binary.Name
    
    Write-Host "Processing: $ruleName" -ForegroundColor White
    
    # Check if binary exists
    if (-not (Test-Path $exePath)) {
        Write-Host "  [SKIP] Binary not found: $exePath" -ForegroundColor Yellow
        Write-Host "         Run .\scripts\test\test-code.ps1 first to build the binaries" -ForegroundColor Yellow
        $rulesSkipped++
        Write-Host ""
        continue
    }
    
    # Check if rule already exists
    $existingRule = Get-NetFirewallRule -DisplayName $ruleName -ErrorAction SilentlyContinue
    
    if ($existingRule) {
        Write-Host "  [EXISTS] Firewall rule already exists" -ForegroundColor Green
        
        # Update the rule to ensure it's correct
        try {
            Set-NetFirewallRule -DisplayName $ruleName -Enabled True -Action Allow
            Write-Host "  [OK] Rule updated and enabled" -ForegroundColor Green
            $rulesAdded++
        }
        catch {
            Write-Host "  [WARN] Could not update rule: $_" -ForegroundColor Yellow
            $rulesSkipped++
        }
    }
    else {
        # Create new firewall rule
        try {
            New-NetFirewallRule `
                -DisplayName $ruleName `
                -Direction Inbound `
                -Program $exePath `
                -Action Allow `
                -Profile Any `
                -Enabled True `
                -Description "Allow $ruleName to accept TCP connections for testing" | Out-Null
            
            Write-Host "  [ADDED] Firewall rule created successfully" -ForegroundColor Green
            $rulesAdded++
        }
        catch {
            Write-Host "  [FAIL] Could not create rule: $_" -ForegroundColor Red
            $rulesFailed++
        }
    }
    
    Write-Host ""
}

# Summary
Write-Host "====================================" -ForegroundColor Cyan
Write-Host "  Summary" -ForegroundColor Cyan
Write-Host "====================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "Rules Added/Updated: $rulesAdded" -ForegroundColor $(if ($rulesAdded -gt 0) { "Green" } else { "White" })
Write-Host "Rules Skipped:       $rulesSkipped" -ForegroundColor $(if ($rulesSkipped -gt 0) { "Yellow" } else { "White" })
Write-Host "Rules Failed:        $rulesFailed" -ForegroundColor $(if ($rulesFailed -gt 0) { "Red" } else { "White" })
Write-Host ""

if ($rulesFailed -gt 0) {
    Write-Host "Some rules failed to be created. Check the errors above." -ForegroundColor Red
    exit 1
}
elseif ($rulesSkipped -eq $testBinaries.Count) {
    Write-Host "No binaries found. Run .\run-tests.ps1 first to build them." -ForegroundColor Yellow
    exit 1
}
else {
    Write-Host "Firewall rules configured successfully!" -ForegroundColor Green
    Write-Host "You should no longer see firewall popups when running tests." -ForegroundColor Green
    Write-Host ""
    Write-Host "To remove these rules later, run: .\scripts\firewall\remove-firewall-rules.ps1" -ForegroundColor Cyan
    exit 0
}
