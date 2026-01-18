# Wox UI Windows - Development Launch Script
# This script launches the WPF UI for testing

Write-Host "========================================"
Write-Host "  Wox UI Windows - Development Mode"
Write-Host "========================================"
Write-Host ""

# Check if .NET SDK is installed
try {
    $dotnetVersion = dotnet --version
    Write-Host ".NET SDK Version: $dotnetVersion"
    Write-Host ""
} catch {
    Write-Host "[ERROR] .NET SDK is not installed!" -ForegroundColor Red
    Write-Host ""
    Write-Host "Please install .NET 8 SDK from:"
    Write-Host "https://dotnet.microsoft.com/download/dotnet/8.0"
    Write-Host ""
    Read-Host "Press Enter to exit"
    exit 1
}

# Check if wox.core is running
Write-Host "[INFO] Make sure wox.core is running before starting UI" -ForegroundColor Yellow
Write-Host ""

# Build the project
Write-Host "[BUILD] Building Wox.UI.Windows..." -ForegroundColor Cyan
dotnet build --configuration Debug

if ($LASTEXITCODE -ne 0) {
    Write-Host ""
    Write-Host "[ERROR] Build failed!" -ForegroundColor Red
    Read-Host "Press Enter to exit"
    exit 1
}

Write-Host ""
Write-Host "[SUCCESS] Build completed!" -ForegroundColor Green
Write-Host ""

# Run the application
Write-Host "[RUN] Starting Wox UI..." -ForegroundColor Cyan
Write-Host ""
Write-Host "Command line args: <ServerPort> <ServerPid> <IsDev>"
Write-Host "Using default values for development..."
Write-Host ""

dotnet run --no-build --configuration Debug

Read-Host "Press Enter to exit"
