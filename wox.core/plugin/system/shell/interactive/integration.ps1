if ($ExecutionContext.SessionState.LanguageMode -ne 'FullLanguage') { return }
if (-not (Get-Module PSReadLine)) {
    Import-Module PSReadLine -ErrorAction SilentlyContinue
}
if (-not (Get-Module PSReadLine) -or -not $function:PSConsoleHostReadLine) { return }

$global:WoxOriginalReadLine = $function:PSConsoleHostReadLine
$global:WoxOriginalPrompt = $function:Prompt
$global:WoxCommandActive = $false

function global:PSConsoleHostReadLine {
    $line = $global:WoxOriginalReadLine.InvokeReturnAsIs()
    if ($line -is [string] -and -not [string]::IsNullOrWhiteSpace($line)) {
        $title = $line -replace '[\x00-\x1f\x7f]', ' '
        [Console]::Write("$([char]27)]633;Wox;@WOX_TOKEN@;start;$title$([char]7)")
        $global:WoxCommandActive = $true
    }
    return $line
}

function global:Prompt {
    $failed = -not $?
    if ($global:WoxCommandActive) {
        $global:WoxCommandActive = $false
        [Console]::Write("$([char]27)]633;Wox;@WOX_TOKEN@;end;$([int]$failed)$([char]7)")
    }
    if ($failed) { Write-Error 'Shell command failed' -ErrorAction Ignore }
    $global:WoxOriginalPrompt.InvokeReturnAsIs()
}
