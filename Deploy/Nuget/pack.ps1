$root = $env:APPVEYOR_BUILD_FOLDER
Write-Host $root
$version = [System.Reflection.Assembly]::LoadFile("$root\Output\Release\Wox.Plugin.dll").GetName().Version
$versionStr = "{0}.{1}.{2}.{3}" -f ($version.Major, $version.Minor, $version.Build, $version.Revision)
& nuget pack $root\deploy\nuget\wox.plugin.nuspec -Version $versionStr
