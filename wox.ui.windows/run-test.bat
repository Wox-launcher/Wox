@echo off
REM Wox UI Windows - UI Test Mode
REM This launches the test window without requiring wox.core

echo ========================================
echo   Wox UI Windows - UI Test Mode
echo ========================================
echo.
echo This mode runs the UI with sample data
echo No need to run wox.core
echo.

dotnet build --configuration Debug
if %ERRORLEVEL% NEQ 0 (
    echo [ERROR] Build failed!
    pause
    exit /b 1
)

echo.
echo [RUN] Starting UI Test Window...
echo.

dotnet run --no-build --configuration Debug -- --test

pause
