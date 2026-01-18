@echo off
REM Wox UI Windows - Development Launch Script
REM This script launches the WPF UI for testing

echo ========================================
echo   Wox UI Windows - Development Mode
echo ========================================
echo.

REM Check if .NET SDK is installed
where dotnet >nul 2>&1
if %ERRORLEVEL% NEQ 0 (
    echo [ERROR] .NET SDK is not installed!
    echo.
    echo Please install .NET 8 SDK from:
    echo https://dotnet.microsoft.com/download/dotnet/8.0
    echo.
    pause
    exit /b 1
)

REM Display .NET version
echo .NET SDK Version:
dotnet --version
echo.

REM Check if wox.core is running
echo [INFO] Make sure wox.core is running before starting UI
echo.

REM Build the project
echo [BUILD] Building Wox.UI.Windows...
dotnet build --configuration Debug
if %ERRORLEVEL% NEQ 0 (
    echo.
    echo [ERROR] Build failed!
    pause
    exit /b 1
)

echo.
echo [SUCCESS] Build completed!
echo.

REM Run the application
echo [RUN] Starting Wox UI...
echo.
echo Command line args: <ServerPort> <ServerPid> <IsDev>
echo Using default values for development...
echo.

dotnet run --no-build --configuration Debug

pause
