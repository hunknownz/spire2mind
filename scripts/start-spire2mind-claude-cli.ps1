param(
    [ValidateSet("tui", "headless-smoke", "long-soak")]
    [string]$Mode = "tui",
    [int]$Attempts = 0,
    [int]$MaxCycles = 0,
    [int]$TimeoutSeconds = 900,
    [int]$IdleTimeoutSeconds = 120,
    [string]$Language = "zh",
    [string]$FastMode = "instant",
    [string]$Planner = "mcts",
    [string]$ReplaceExisting = "1"
)

$ErrorActionPreference = "Stop"

$scriptRoot = $PSScriptRoot

function Start-VisibleTuiProcess {
    param(
        [string]$RunTuiScript,
        [int]$Attempts,
        [int]$MaxCycles,
        [string]$Language,
        [string]$Provider,
        [string]$FastMode,
        [string]$Planner
    )

    $powershellArgs = @(
        "powershell.exe",
        "-NoExit",
        "-ExecutionPolicy",
        "Bypass",
        "-File",
        $RunTuiScript,
        "-Attempts",
        [string]$Attempts,
        "-MaxCycles",
        [string]$MaxCycles,
        "-Language",
        $Language,
        "-Provider",
        $Provider,
        "-FastMode",
        $FastMode,
        "-Planner",
        $Planner
    )

    $useWindowsTerminal = $env:SPIRE2MIND_TUI_USE_WINDOWS_TERMINAL -match '^(1|true|yes|on)$'
    $wt = Get-Command wt.exe -ErrorAction SilentlyContinue
    if ($useWindowsTerminal -and $wt) {
        $wtArgs = @(
            "new-tab",
            "--title",
            "Spire2Mind TUI"
        ) + $powershellArgs
        Start-Process -FilePath $wt.Source -ArgumentList $wtArgs | Out-Null
        return
    }

    Start-Process -FilePath "powershell.exe" -ArgumentList $powershellArgs[1..($powershellArgs.Length - 1)] | Out-Null
}

function Stop-ExistingSpire2MindInstances {
    param(
        [int]$CurrentPid
    )

    try {
        $processes = Get-CimInstance Win32_Process | Where-Object {
            $_.CommandLine -and (
                $_.CommandLine -match 'start-spire2mind-(claude-cli|api)\.ps1' -or
                $_.CommandLine -match 'run-tui\.ps1' -or
                $_.CommandLine -match 'headless-smoke\.ps1' -or
                $_.CommandLine -match 'long-soak\.ps1' -or
                $_.CommandLine -match 'go\.exe"\s+run\s+\.\\cmd\\spire2mind\s+play(?:\s|$)'
            )
        }
    }
    catch {
        Write-Warning "Unable to inspect existing Spire2Mind processes; skipping cleanup: $($_.Exception.Message)"
        return
    }

    foreach ($process in $processes) {
        if ($process.ProcessId -eq $CurrentPid) {
            continue
        }

        try {
            Stop-Process -Id $process.ProcessId -Force -ErrorAction Stop
            Write-Host "Stopped existing Spire2Mind process: $($process.ProcessId) [$($process.Name)]"
        }
        catch {
            Write-Warning "Failed to stop existing process $($process.ProcessId): $($_.Exception.Message)"
        }
    }
}

if ($ReplaceExisting -match '^(1|true|yes|on)$') {
    Stop-ExistingSpire2MindInstances -CurrentPid $PID
}

$env:SPIRE2MIND_MODEL_PROVIDER = "claude-cli"

switch ($Mode) {
    "tui" {
        Start-VisibleTuiProcess `
            -RunTuiScript (Join-Path $scriptRoot "run-tui.ps1") `
            -Attempts $Attempts `
            -MaxCycles $MaxCycles `
            -Language $Language `
            -Provider "claude-cli" `
            -FastMode $FastMode `
            -Planner $Planner
    }
    "headless-smoke" {
        & (Join-Path $scriptRoot "headless-smoke.ps1") `
            -TimeoutSeconds $TimeoutSeconds `
            -IdleTimeoutSeconds $IdleTimeoutSeconds `
            -Attempts $Attempts `
            -FastMode $FastMode `
            -Planner $Planner
    }
    "long-soak" {
        & (Join-Path $scriptRoot "long-soak.ps1") `
            -Attempts $Attempts `
            -TimeoutSeconds $TimeoutSeconds `
            -IdleTimeoutSeconds $IdleTimeoutSeconds `
            -FastMode $FastMode `
            -Planner $Planner
    }
}

exit $LASTEXITCODE
