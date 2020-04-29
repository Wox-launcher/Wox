param(
    [string]$solution = "."
)
$ErrorActionPreference = "Stop"

$path = "$solution\Output\packages"
$installer = "$path\Wox-Full-Installer.exe"
$version = (Get-Command $installer).FileVersionInfo.ProductVersion
$newName = "$path\Wox-Full-Installer.$version.exe"
Move-Item -Force $installer $newName