[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [Alias("Pid")]
    [ValidateRange(1, [int]::MaxValue)]
    [int]$ProcessId,

    [ValidateRange(1, [int]::MaxValue)]
    [int]$Samples = 30,

    [ValidateRange(0.1, 3600)]
    [double]$IntervalSeconds = 1,

    [string]$CsvPath = ""
)

$process = Get-Process -Id $ProcessId -ErrorAction Stop
$previousCPUSeconds = $process.TotalProcessorTime.TotalSeconds
$previousTimestamp = [DateTime]::UtcNow
$measurements = @()

for ($index = 1; $index -le $Samples; $index++) {
    Start-Sleep -Milliseconds ([int][Math]::Round($IntervalSeconds * 1000))
    $process = Get-Process -Id $ProcessId -ErrorAction Stop
    $timestamp = [DateTime]::UtcNow
    $elapsedSeconds = ($timestamp - $previousTimestamp).TotalSeconds
    $cpuPercent = (($process.TotalProcessorTime.TotalSeconds - $previousCPUSeconds) / $elapsedSeconds) * 100
    $measurements += [pscustomobject]@{
        Sample       = $index
        CPUPercent   = $cpuPercent
        WorkingSetMB = $process.WorkingSet64 / 1MB
        PrivateMB    = $process.PrivateMemorySize64 / 1MB
    }
    $previousCPUSeconds = $process.TotalProcessorTime.TotalSeconds
    $previousTimestamp = $timestamp
}

if (-not [string]::IsNullOrWhiteSpace($CsvPath)) {
    $parent = Split-Path -Parent $CsvPath
    if (-not [string]::IsNullOrWhiteSpace($parent)) {
        New-Item -ItemType Directory -Force -Path $parent | Out-Null
    }
    $measurements | Export-Csv -NoTypeInformation -Path $CsvPath
}

$sortedCPU = @($measurements.CPUPercent | Sort-Object)
if ($Samples % 2 -eq 0) {
    $medianCPU = ($sortedCPU[$Samples / 2 - 1] + $sortedCPU[$Samples / 2]) / 2
} else {
    $medianCPU = $sortedCPU[[int][Math]::Floor($Samples / 2)]
}

$measurements | Format-Table Sample, @{ Label = "CPUPercent"; Expression = { "{0:F2}" -f $_.CPUPercent } }, @{ Label = "WorkingSetMB"; Expression = { "{0:F2}" -f $_.WorkingSetMB } }, @{ Label = "PrivateMB"; Expression = { "{0:F2}" -f $_.PrivateMB } }
"Summary Samples={0} AverageCPUPercent={1:F2} MedianCPUPercent={2:F2} MaxCPUPercent={3:F2} AverageWorkingSetMB={4:F2} MaxWorkingSetMB={5:F2}" -f `
    $Samples,
    ($measurements.CPUPercent | Measure-Object -Average).Average,
    $medianCPU,
    ($measurements.CPUPercent | Measure-Object -Maximum).Maximum,
    ($measurements.WorkingSetMB | Measure-Object -Average).Average,
    ($measurements.WorkingSetMB | Measure-Object -Maximum).Maximum
