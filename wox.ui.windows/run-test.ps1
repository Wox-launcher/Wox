# Wox UI Windows - UI Test Mode
# This launches the test window without requiring wox.core

Write-Host "========================================"
Write-Host "  Wox UI Windows - UI Test Mode"
Write-Host "========================================"
Write-Host ""
Write-Host "This mode runs the UI with sample data"
Write-Host "No need to run wox.core"
Write-Host ""

Write-Host "[BUILD] Building..." -ForegroundColor Cyan
dotnet build --configuration Debug

if ($LASTEXITCODE -ne 0) {
    Write-Host "[ERROR] Build failed!" -ForegroundColor Red
    Read-Host "Press Enter to exit"
    exit 1
}

Write-Host ""
Write-Host "[RUN] Starting UI Test Window..." -ForegroundColor Green
Write-Host ""

dotnet run --no-build --configuration Debug -- --test

Read-Host "Press Enter to exit"
