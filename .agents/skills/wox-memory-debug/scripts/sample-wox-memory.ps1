param(
    [int]$Samples = 1,
    [int]$IntervalSeconds = 10,
    [double]$BudgetMB = 200,
    [string[]]$ProcessNames = @("wox", "wox-ui", "wox-windows-amd64", "wox-windows-arm64"),
    [int[]]$Pids = @(),
    [switch]$Json
)

$ErrorActionPreference = "Stop"

if ([Environment]::OSVersion.Platform -ne [PlatformID]::Win32NT) {
    throw "sample-wox-memory.ps1 only supports Windows."
}

function Convert-ToMB {
    param([UInt64]$Bytes)

    return [Math]::Round($Bytes / 1MB, 1)
}

function Test-ContainsIgnoreCase {
    param(
        [string]$Value,
        [string]$Needle
    )

    return $Value.IndexOf($Needle, [StringComparison]::OrdinalIgnoreCase) -ge 0
}

function Get-Role {
    param($Process)

    $name = [IO.Path]::GetFileNameWithoutExtension($Process.Name).ToLowerInvariant()
    $path = [string]$Process.ExecutablePath
    $commandLine = [string]$Process.CommandLine

    if ($name -eq "wox-ui" -or (Test-ContainsIgnoreCase $path "\wox-ui.exe") -or (Test-ContainsIgnoreCase $commandLine "wox.ui.flutter")) {
        return "Flutter"
    }
    return "Core"
}

function Get-WoxProcessRows {
    $processRows = Get-CimInstance Win32_Process | Where-Object {
        $name = [IO.Path]::GetFileNameWithoutExtension($_.Name)
        $path = [string]$_.ExecutablePath
        $commandLine = [string]$_.CommandLine

        ($Pids -contains [int]$_.ProcessId) -or
        ($ProcessNames -contains $name) -or
        ($path -match "\\Wox\\wox\.core\\") -or
        ($commandLine -match "\\Wox\\wox\.core\\") -or
        ($path -match "\\Wox\\wox\.ui\.flutter\\") -or
        ($commandLine -match "\\Wox\\wox\.ui\.flutter\\")
    }

    if (-not $processRows) {
        throw "No Wox core or wox-ui process found. Pass -Pids for debugger-launched processes with temporary names."
    }

    $perfByPid = @{}
    Get-CimInstance Win32_PerfFormattedData_PerfProc_Process | ForEach-Object {
        if ($_.IDProcess -gt 0) {
            $perfByPid[[int]$_.IDProcess] = $_
        }
    }

    $timestamp = (Get-Date).ToString("o")
    foreach ($process in $processRows | Sort-Object ProcessId) {
        $processId = [int]$process.ProcessId
        $perf = $perfByPid[$processId]
        if ($null -eq $perf -or $null -eq $perf.WorkingSetPrivate) {
            throw "Private working-set counter is unavailable for pid $processId."
        }

        [pscustomobject]@{
            Timestamp = $timestamp
            Role = Get-Role $process
            Pid = $processId
            Name = $process.Name
            PrivateWorkingSetMB = Convert-ToMB ([UInt64]$perf.WorkingSetPrivate)
            Path = $process.ExecutablePath
        }
    }
}

if ($Samples -lt 1) {
    throw "-Samples must be at least 1."
}

$allSamples = @()
for ($i = 1; $i -le $Samples; $i++) {
    $rows = @(Get-WoxProcessRows)
    $total = [Math]::Round((($rows | Measure-Object PrivateWorkingSetMB -Sum).Sum), 1)
    $roles = @($rows | ForEach-Object { $_.Role } | Sort-Object -Unique)
    $missingRoles = @("Core", "Flutter") | Where-Object { $roles -notcontains $_ }
    $sample = [pscustomobject]@{
        Sample = $i
        TotalMB = $total
        BudgetMB = $BudgetMB
        OverBudget = $total -gt $BudgetMB
        MissingRoles = $missingRoles
        Processes = $rows
    }
    $allSamples += $sample

    if (-not $Json) {
        Write-Host ("Sample {0}: TotalMB={1} BudgetMB={2} OverBudget={3}" -f $i, $total, $BudgetMB, ($total -gt $BudgetMB))
        if ($missingRoles.Count -gt 0) {
            Write-Warning ("Missing expected role(s): {0}. Pass -Pids for debugger-launched processes with temporary names." -f ($missingRoles -join ", "))
        }
        $rows | Format-Table Role, Pid, Name, PrivateWorkingSetMB, Path -AutoSize
    }

    if ($i -lt $Samples) {
        Start-Sleep -Seconds $IntervalSeconds
    }
}

if ($Json) {
    $allSamples | ConvertTo-Json -Depth 5
}
