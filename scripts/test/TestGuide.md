# run-tests.ps1 - Usage Guide

## Purpose

This script pre-compiles test binaries to fixed paths in the `bin/` directory, which helps manage Windows Firewall prompts more effectively. The test binaries create TCP servers for testing, which requires firewall approval.

## First Run Instructions

### Option 1: Automatic Firewall Rules (Recommended)

1. **Build the test binaries first:**
   ```powershell
   .\scripts\test\test-code.ps1
   ```

2. **Add firewall rules (run PowerShell as Administrator):**
   ```powershell
   .\scripts\firewall\add-firewall-rules.ps1
   ```

3. **Never see the prompt again!** 🎉

### Option 2: Manual Firewall Approval

1. **Run the script:**
   ```powershell
   .\scripts\test\test-code.ps1
   ```

2. **Windows Firewall will prompt** (click "Allow access" each time)
   - This may prompt **every time you run tests** because the test binaries create network servers on different ports

3. **To avoid repeated prompts**, use Option 1 instead

## Basic Usage

```powershell
# Run all tests (recommended)
.\scripts\test\test-code.ps1

# Force rebuild all test binaries
.\scripts\test\test-code.ps1 -Force

# Show detailed test output
.\scripts\test\test-code.ps1 -Verbose

# Run with coverage analysis
.\scripts\test\test-code.ps1 -Coverage
```

## How It Works

1. **Smart Rebuilding**: The script checks if source code has changed since the last build
   - If code is newer → Rebuilds test binary
   - If binary is up-to-date → Skips rebuild (faster)

2. **Fixed Paths**: Test binaries are always built to:
   - `bin/bridge-test.exe` - Bridge tests
   - `bin/tcp-test.exe` - TCP tests
   - `bin/logger-test.exe` - Logger tests

3. **Firewall**: Network tests require incoming connections, which triggers Windows Firewall. Use `scripts\firewall\add-firewall-rules.ps1` to permanently allow these binaries.

## Options

| Flag | Description |
|------|-------------|
| `-Force` | Force rebuild all test binaries even if up-to-date |
| `-Verbose` | Show detailed test output with `-test.v` flag |
| `-Coverage` | Collect coverage data and generate coverage reports |

## Coverage Reports

When using `-Coverage`:

```powershell
.\scripts\test\test-code.ps1 -Coverage
```

This generates coverage files:
- `coverage_Bridge_Tests.out`
- `coverage_TCP_Tests.out`
- `coverage_Logger_Tests.out`

View coverage in browser:
```powershell
go tool cover -html=coverage_Bridge_Tests.out
```

## Troubleshooting

### Firewall still prompting?

If you're still seeing firewall prompts after clicking "Allow":

**Root Cause:** Windows Firewall prompts appear when programs try to accept incoming network connections. The TCP tests create servers on different ports, triggering the firewall check each time.

**Solution:** Add permanent firewall rules (requires Administrator):

```powershell
# Run PowerShell as Administrator
.\scripts\firewall\add-firewall-rules.ps1
```

This creates firewall rules that allow the test binaries to accept TCP connections permanently.

**Alternative:** If you can't run as Administrator, you'll need to click "Allow" each time, or disable Windows Firewall for Private networks (not recommended).

### Remove firewall rules

To remove the firewall rules later:

```powershell
# Run PowerShell as Administrator
.\scripts\firewall\remove-firewall-rules.ps1
```

### Tests fail but `go test ./...` works?

The pre-compiled binaries might be out of date. Run:
```powershell
.\scripts\test\test-code.ps1 -Force
```

### Want to use `go test` instead?

You can always use the standard Go test command:
```powershell
go test ./...
```

The script is optional and designed for convenience on Windows.

## Firewall Scripts

### add-firewall-rules.ps1

Adds Windows Firewall rules to permanently allow test binaries to accept incoming connections.

**Requirements:**
- Must run as Administrator
- Test binaries must exist in `bin/` (run `.\run-tests.ps1` first)

**What it does:**
- Creates inbound firewall rules for each test binary
- Allows TCP connections on any port
- Applies to all network profiles (Domain, Private, Public)

### remove-firewall-rules.ps1

Removes the firewall rules created by `add-firewall-rules.ps1`.

**Requirements:**
- Must run as Administrator

**When to use:**
- Cleaning up test environment
- Reverting firewall changes
- Security policy requires removal

## Comparison with `go test`

| Feature | `go test ./...` | `.\run-tests.ps1` |
|---------|----------------|-------------------|
| Firewall popups | Every run (temp binaries) | Can be avoided with firewall rules |
| Speed (no changes) | Fast (cached) | Very fast (skips build) |
| Speed (with changes) | Fast | Fast |
| Binary location | Temp directory (random) | Fixed `bin/` directory |
| Detailed output | Manual `-v` flag | `-Verbose` flag |
| Coverage | Manual flags | `-Coverage` flag |

