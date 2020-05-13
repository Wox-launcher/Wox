param(
    [string]$config = "Release", 
    [string]$solution = "."
)
$ErrorActionPreference = "Stop"

function Build-Version {
	if ([string]::IsNullOrEmpty($env:APPVEYOR_BUILD_VERSION)) {
		$v = "1.5.0"
	} else {
        $v = $env:APPVEYOR_BUILD_VERSION
    }

    Write-Host "Build Version: $v"
    return $v
}

function Build-Path {
    if (![string]::IsNullOrEmpty($env:APPVEYOR_BUILD_FOLDER)) {
        $p = $env:APPVEYOR_BUILD_FOLDER
    } elseif (![string]::IsNullOrEmpty($solution)) {
        $p = $solution
    } else {
        $p = Get-Location
    }

    Write-Host "Build Folder: $p"
    Set-Location $p

    return $p
}

function Copy-Resources ($path, $config) {
    $project = "$path\Wox"
    $output = "$path\Output"
    $target = "$output\$config"
    Copy-Item -Recurse -Force $path\Plugins\HelloWorldPython $target\Plugins
    Copy-Item -Recurse -Force $path\JsonRPC $target\JsonRPC
}

function Delete-Unused ($path, $config) {
    $target = "$path\Output\$config"
    $included = Get-ChildItem $target -Filter "*.dll"
    foreach ($i in $included){
        Remove-Item -Path $target\Plugins -Include $i -Recurse 
        Write-Host "Deleting duplicated $i"
    }
    $included = Get-ChildItem $target -Filter "*.pdb"
    foreach ($i in $included){
        Remove-Item -Path $target\Plugins -Include $i -Recurse 
        Write-Host "Deleting duplicated $i"
    }
    Remove-Item -Path $target -Include "JetBrains.Annotations.dll" -Recurse 
    Remove-Item -Path $target -Include "*.xml" -Recurse 
    Remove-Item -Path $target -Include "nunit.*" -Recurse 
    Remove-Item -Path $target -Include "NUnit3.*" -Recurse 
    Remove-Item -Path $target -Include "Moq.*" -Recurse 
    Remove-Item -Path $target -Include "Wox.Test.*" -Recurse 
    Remove-Item -Path $target -Include "win-arm" -Recurse 
}

function Validate-Directory ($output) {
    New-Item $output -ItemType Directory -Force
}

function Pack-Nuget-API ($path, $version, $output) {
    Write-Host "Begin build nuget library"

    $spec = "$path\Wox.Installer\wox.plugin.nuspec"
    Write-Host "nuspec path: $spec"
    Write-Host "Output path: $output"

    Nuget pack $spec -Version $version -OutputDirectory $output

    Write-Host "End build nuget library"
}

function Pack-Zip ($path, $version, $output) {
    Write-Host "Begin pack zip"

    $input = "$path\Output\$config"
    Write-Host "Input path:  $input"
    $file = "$output\Wox-$version.zip"
    Write-Host "Filename: $file"

    [Reflection.Assembly]::LoadWithPartialName("System.IO.Compression.FileSystem")
    [System.IO.Compression.ZipFile]::CreateFromDirectory($input, $file)

    Write-Host "End pack zip"
}

function Pack-Squirrel-Installer ($path, $version, $output) {
    # msbuild based installer generation is not working in appveyor, not sure why
    Write-Host "Begin pack squirrel installer"

    $spec = "$path\Wox.Installer\wox.nuspec"
    Write-Host "nuspec path: $spec"
    $input = "$path\Output\$config"
    Write-Host "Input path:  $input"
    Nuget pack $spec -Version $version -Properties Configuration=$config -BasePath $input -OutputDirectory  $output

    $nupkg = "$output\Wox.$version.nupkg"
    Write-Host "nupkg path: $nupkg"
    
    # Squirrel.com: https://github.com/Squirrel/Squirrel.Windows/issues/369
    New-Alias Squirrel $env:USERPROFILE\.nuget\packages\squirrel.windows\1.9.1\tools\Squirrel.exe -Force
    # why we need Write-Output: https://github.com/Squirrel/Squirrel.Windows/issues/489#issuecomment-156039327
    # directory of releaseDir in squirrel can't be same as directory ($nupkg) in releasify
    $temp = "$output\Temp"
    Write-Host "temp: $temp"
    $icon = "$path\Wox\Resources\app.ico"
    Write-Host "icon: $icon"

    Squirrel --releasify $nupkg --releaseDir $temp --setupIcon $icon --no-msi | Write-Output
    Move-Item $temp\* $output -Force
    Remove-Item $temp
    
    $file = "$output\Wox-$version.exe"
    Write-Host "Filename: $file"

    Move-Item "$output\Setup.exe" $file -Force

    Write-Host "End pack squirrel installer"
}

function Main {
    Write-Host $(Get-Location)
    Write-Host "Config: $config"
    $p = Build-Path
    $v = Build-Version
    Copy-Resources $p $config

    Delete-Unused $p $config
    $o = "$p\Output\Packages"
    Validate-Directory $o
    # making version static as multiple versions can exist in the nuget folder and in the case a breaking change is introduced.
    New-Alias Nuget $env:USERPROFILE\.nuget\packages\NuGet.CommandLine\5.5.1\tools\NuGet.exe -Force
    Pack-Squirrel-Installer $p $v $o
    
    $isInCI = $env:APPVEYOR
    if ($isInCI) {
        Pack-Nuget-API $p $v $o
        Pack-Zip $p $v $o
    }

    Write-Host "List output directory"
    Get-ChildItem $o
}

Main