#Requires -Version 5.1
# Keep installer variables and preferences in their own scope when piped into iex.
& {
  $ErrorActionPreference = 'Stop'

  $repo = 'https://github.com/Wox-launcher/Wox'
  $architecture = $env:PROCESSOR_ARCHITEW6432
  if (-not $architecture) {
    $architecture = $env:PROCESSOR_ARCHITECTURE
  }
  if ($env:OS -ne 'Windows_NT' -or $architecture -ne 'AMD64') {
    throw 'Wox requires Windows x64.'
  }
  if (-not (Get-Command curl.exe -CommandType Application -ErrorAction SilentlyContinue)) {
    throw 'This installer requires curl.exe.'
  }

  $programs = [Environment]::GetFolderPath('Programs')
  $commonPrograms = [Environment]::GetFolderPath('CommonPrograms')
  $localAppData = [Environment]::GetFolderPath('LocalApplicationData')
  if (-not $programs -or -not $localAppData) {
    throw 'Run this installer from your Windows desktop account.'
  }

  $installDirectory = Join-Path $localAppData 'Programs\Wox'
  $executable = Join-Path $installDirectory 'wox.exe'
  $shortcut = Join-Path $programs 'Wox.lnk'
  $knownPaths = @($installDirectory, $shortcut)
  if ($commonPrograms) {
    $knownPaths += (Join-Path $commonPrograms 'Wox.lnk')
  }
  $existingPath = $knownPaths | Where-Object { Test-Path -LiteralPath $_ } | Select-Object -First 1
  if (-not $existingPath) {
    $existingPath = Get-Command wox.exe -CommandType Application -ErrorAction SilentlyContinue |
      Select-Object -First 1 -ExpandProperty Source
  }
  if ($existingPath) {
    Write-Host "Wox is already installed ($existingPath)."
    Write-Host 'Use its existing installation method or update it from Wox.'
    return
  }

  $work = Join-Path ([IO.Path]::GetTempPath()) ('wox-install-' + [Guid]::NewGuid())
  $curlOptions = @(
    '--disable', '--fail', '--silent', '--show-error', '--location', '--proto', '=https',
    '--proto-redir', '=https', '--connect-timeout', '15', '--max-time', '300'
  )
  $stagedShortcut = $null
  $installCreated = $false
  $shortcutInstalled = $false
  New-Item -ItemType Directory -Path $work | Out-Null
  try {
    $latest = & curl.exe @curlOptions --head --output NUL --write-out '%{url_effective}' "$repo/releases/latest"
    if ($LASTEXITCODE -ne 0 -or $latest -cnotmatch '^https://github\.com/Wox-launcher/Wox/releases/tag/(v[0-9]+\.[0-9]+\.[0-9]+)$') {
      throw 'Could not identify the latest stable Wox release.'
    }
    $tag = $Matches[1]
    $release = "$repo/releases/download/$tag"

    $checksums = Join-Path $work 'SHA256SUMS'
    & curl.exe @curlOptions --output $checksums "$release/SHA256SUMS"
    if ($LASTEXITCODE -ne 0) {
      throw "Could not download checksums for Wox $tag. See $repo/releases."
    }
    $asset = 'wox-windows-amd64.exe'
    $expected = @(Get-Content -LiteralPath $checksums | ForEach-Object {
      if ($_ -cmatch '^([a-f0-9]{64})  wox-windows-amd64\.exe$') { $Matches[1] }
    })
    if ($expected.Length -ne 1) {
      throw "Missing or invalid SHA-256 checksum for $asset."
    }
    $downloaded = Join-Path $work $asset
    Write-Host "Downloading $asset..."
    & curl.exe @curlOptions --no-silent --progress-bar --output $downloaded "$release/$asset"
    if ($LASTEXITCODE -ne 0) {
      throw "Could not download $asset."
    }
    if ((Get-FileHash -LiteralPath $downloaded -Algorithm SHA256).Hash -ine $expected[0]) {
      throw "SHA-256 mismatch for $asset."
    }
    $signature = Get-AuthenticodeSignature -LiteralPath $downloaded
    if ($signature.Status -ne 'Valid') {
      throw "Windows could not verify the Wox signature: $($signature.StatusMessage)"
    }

    [IO.Directory]::CreateDirectory((Split-Path -Parent $installDirectory)) | Out-Null
    New-Item -ItemType Directory -Path $installDirectory | Out-Null
    $installCreated = $true
    [IO.File]::Copy($downloaded, $executable)

    [IO.Directory]::CreateDirectory($programs) | Out-Null
    $stagedShortcut = Join-Path $programs ('.wox-install-' + [Guid]::NewGuid() + '.lnk')
    $shell = New-Object -ComObject WScript.Shell
    $link = $shell.CreateShortcut($stagedShortcut)
    $link.TargetPath = $executable
    $link.WorkingDirectory = $installDirectory
    $link.Save()
    [IO.File]::Move($stagedShortcut, $shortcut)
    $shortcutInstalled = $true
    Write-Host "Installed Wox $tag. Open Wox from the Start menu."
  } finally {
    if ($installCreated -and -not $shortcutInstalled) {
      [IO.File]::Delete($executable)
      [IO.Directory]::Delete($installDirectory)
    }
    if ($stagedShortcut) {
      [IO.File]::Delete($stagedShortcut)
    }
    Remove-Item -LiteralPath $work -Recurse -Force
  }
}
