$ErrorActionPreference = 'Stop'

$toolsDir = Split-Path -Parent $MyInvocation.MyCommand.Definition
$fileFullPath = Join-Path $toolsDir 'wox.exe'

if (Test-Path $fileFullPath) {
  Remove-Item $fileFullPath -Force
}

$programs = [Environment]::GetFolderPath('CommonPrograms')
if ([string]::IsNullOrWhiteSpace($programs)) {
  $programs = Join-Path $env:ProgramData 'Microsoft\Windows\Start Menu\Programs'
}

$shortcutPath = Join-Path $programs 'Wox.lnk'
if (Test-Path $shortcutPath) {
  Remove-Item $shortcutPath -Force
}
