# run-tests.ps1
# Smart test runner that pre-compiles test binaries to avoid Windows Firewall popups
# This script checks for code changes and rebuilds only when necessary

param(
    [switch]$Force,      # Force rebuild all test binaries
    [switch]$Verbose,    # Show detailed output
    [switch]$Coverage    # Run with coverage analysis
)

$ErrorActionPreference = "Stop"

# Determine repository root (parent of scripts/test)
$repoRoot = (Get-Item $PSScriptRoot).Parent.Parent.FullName

# Define test packages and their output binaries (absolute paths for clarity)
$testPackages = @(
    @{
        Package = Join-Path $repoRoot "internal/bridge/test"
        Binary  = Join-Path $repoRoot "bin/bridge-test.exe"
        Name    = "Bridge Tests"
    },
    @{
        Package = Join-Path $repoRoot "internal/tcp/test"
        Binary  = Join-Path $repoRoot "bin/tcp-test.exe"
        Name    = "TCP Tests"
    },
    @{
        Package = Join-Path $repoRoot "pkg/logger/test"
        Binary  = Join-Path $repoRoot "bin/logger-test.exe"
        Name    = "Logger Tests"
    }
)

# Colors for output
function Write-Success { Write-Host $args -ForegroundColor Green }
function Write-Info { Write-Host $args -ForegroundColor Cyan }
function Write-Warning { Write-Host $args -ForegroundColor Yellow }
function Write-Error { Write-Host $args -ForegroundColor Red }

# Check if binary needs rebuilding
function Test-NeedsRebuild {
    param($Package, $Binary)
    
    if ($Force) { return $true }
    if (-not (Test-Path $Binary)) { return $true }
    
    # Get binary modification time
    $binaryTime = (Get-Item $Binary).LastWriteTime
    
    # Get latest source file modification time in package
    $sourceFiles = Get-ChildItem -Path $Package -Filter "*.go" -Recurse -File
    $latestSource = ($sourceFiles | Measure-Object -Property LastWriteTime -Maximum).Maximum
    
    return $latestSource -gt $binaryTime
}

# Main execution
Write-Info "====================================="
Write-Info "  MangaHub Smart Test Runner"
Write-Info "====================================="
Write-Host ""

# Ensure bin directory exists
if (-not (Test-Path (Join-Path $repoRoot "bin"))) {
    New-Item -ItemType Directory -Path (Join-Path $repoRoot "bin") | Out-Null
    Write-Info "Created bin/ directory at $repoRoot"
}

$rebuildCount = 0
$testResults = @()

# Build test binaries
foreach ($test in $testPackages) {
    Write-Info "Checking $($test.Name)..."
    
    if (Test-NeedsRebuild -Package $test.Package -Binary $test.Binary) {
        Write-Warning "  Building $($test.Binary)..."
        $rebuildCount++
        
        try {
            if ($Coverage) {
                # Build with coverage support
                Push-Location $repoRoot | Out-Null
                go test -c -cover -o $test.Binary $test.Package
                Pop-Location | Out-Null
            }
            else {
                Push-Location $repoRoot | Out-Null
                go test -c -o $test.Binary $test.Package
                Pop-Location | Out-Null
            }
            
            if ($LASTEXITCODE -ne 0) {
                throw "Build failed"
            }
            
            Write-Success "  [OK] Built successfully"
        }
        catch {
            Write-Error "  [FAIL] Build failed for $($test.Name)"
            exit 1
        }
    }
    else {
        Write-Success "  [OK] Up to date (skipping rebuild)"
    }
}

Write-Host ""
if ($rebuildCount -gt 0) {
    Write-Info "Rebuilt $rebuildCount test package(s)"
    Write-Warning "Note: Windows Firewall may prompt you to allow these executables"
}
else {
    Write-Success "All test binaries are up to date"
}

Write-Host ""
Write-Info "====================================="
Write-Info "  Running Tests"
Write-Info "====================================="
Write-Host ""

# Run test binaries
$allPassed = $true
$totalTests = 0
$passedTests = 0
$failedTests = 0

foreach ($test in $testPackages) {
    Write-Info "Running $($test.Name)..."
    Write-Host ""
    
    $startTime = Get-Date
    
    try {
        if ($Verbose) {
            & $test.Binary '-test.v'
        }
        elseif ($Coverage) {
            $covArg = "-test.coverprofile=coverage_$($test.Name -replace ' ', '_').out"
            & $test.Binary $covArg
        }
        else {
            & $test.Binary
        }
        
        $exitCode = $LASTEXITCODE
        $duration = (Get-Date) - $startTime
        
        if ($exitCode -eq 0) {
            Write-Success "[PASS] $($test.Name) ($('{0:F2}' -f $duration.TotalSeconds)s)"
            $passedTests++
            $testResults += @{ Name = $test.Name; Status = "PASS"; Duration = $duration }
        }
        else {
            Write-Error "[FAIL] $($test.Name) ($('{0:F2}' -f $duration.TotalSeconds)s)"
            $allPassed = $false
            $failedTests++
            $testResults += @{ Name = $test.Name; Status = "FAIL"; Duration = $duration }
        }
    }
    catch {
        Write-Error "[ERROR] $($test.Name) FAILED TO RUN"
        $allPassed = $false
        $failedTests++
        $testResults += @{ Name = $test.Name; Status = "ERROR"; Duration = $null }
    }
    
    Write-Host ""
}

# Summary
Write-Info "====================================="
Write-Info "  Test Summary"
Write-Info "====================================="
Write-Host ""

foreach ($result in $testResults) {
    $statusColor = if ($result.Status -eq "PASS") { "Green" } else { "Red" }
    $durationStr = if ($result.Duration) { "($('{0:F2}' -f $result.Duration.TotalSeconds)s)" } else { "" }
    Write-Host "  $($result.Name): " -NoNewline
    Write-Host "$($result.Status) $durationStr" -ForegroundColor $statusColor
}

Write-Host ""
$totalTests = $passedTests + $failedTests
Write-Host "Total: $totalTests tests, " -NoNewline
Write-Host "$passedTests passed" -ForegroundColor Green -NoNewline
Write-Host ", " -NoNewline
Write-Host "$failedTests failed" -ForegroundColor $(if ($failedTests -gt 0) { "Red" } else { "Green" })

Write-Host ""

if ($Coverage) {
    Write-Info "Coverage reports generated: coverage_*.out"
    Write-Info "View coverage with: go tool cover -html=coverage_<name>.out"
    Write-Host ""
}

if ($allPassed) {
    Write-Success "====================================="
    Write-Success "  ALL TESTS PASSED"
    Write-Success "====================================="
    exit 0
}
else {
    Write-Error "====================================="
    Write-Error "  SOME TESTS FAILED"
    Write-Error "====================================="
    exit 1
}
