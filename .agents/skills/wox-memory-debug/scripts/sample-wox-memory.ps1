param(
    [int]$Samples = 1,
    [int]$IntervalSeconds = 10,
    [string[]]$ProcessNames = @("wox", "wox-windows-amd64", "wox-windows-arm64"),
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

function Get-WoxProcessRows {
    $allProcesses = Get-CimInstance Win32_Process
    if ($Pids.Count -gt 0) {
        $processRows = $allProcesses | Where-Object { $Pids -contains [int]$_.ProcessId }
    }
    else {
        $processRows = $allProcesses | Where-Object {
            $name = [IO.Path]::GetFileNameWithoutExtension($_.Name)
            $path = [string]$_.ExecutablePath
            $commandLine = [string]$_.CommandLine

            ($ProcessNames -contains $name) -or
            ($path -match "\\Wox\\wox\.core\\") -or
            ($commandLine -match "\\Wox\\wox\.core\\")
        }
    }

    if (-not $processRows) {
        throw "No Wox process found. Pass -Pids for debugger-launched processes with temporary names."
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
            Role = "Wox"
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
    $sample = [pscustomobject]@{
        Sample = $i
        TotalMB = $total
        ProcessCount = $rows.Count
        Processes = $rows
    }
    $allSamples += $sample

    if (-not $Json) {
        Write-Host ("Sample {0}: TotalMB={1}" -f $i, $total)
        if ($rows.Count -ne 1) {
            Write-Warning ("Expected one Wox app process, found {0}. Pass -Pids to select the intended process." -f $rows.Count)
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
