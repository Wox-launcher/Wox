@echo off
setlocal

set "TARGET=%~1"
set "LOG=%~2"

echo [%date% %time%] restart script start >> "%LOG%"
echo [%date% %time%] args: %* >> "%LOG%"
if "%TARGET%"=="" (
  echo [%date% %time%] missing target >> "%LOG%"
  endlocal
  exit /b 1
)
echo [%date% %time%] target: %TARGET% >> "%LOG%"
echo [%date% %time%] killing wox-ui.exe >> "%LOG%"
taskkill /T /F /IM wox-ui.exe >> "%LOG%" 2>&1
timeout /t 1 /nobreak >nul
echo [%date% %time%] removing backup >> "%LOG%"
if exist "%TARGET%.old" (
  attrib -H -S -R "%TARGET%.old" >> "%LOG%" 2>&1
  del /f /q "%TARGET%.old" >> "%LOG%" 2>&1
) else (
  echo [%date% %time%] backup not found: %TARGET%.old >> "%LOG%"
)
echo [%date% %time%] launching >> "%LOG%"
start "" "%TARGET%"
echo [%date% %time%] launched >> "%LOG%"
endlocal
