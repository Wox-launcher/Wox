@echo off
echo === Wox uninstallation utility ===
echo Now running Squirrel uninstaller
%LOCALAPPDATA%\Wox\Update.exe --uninstall .
echo Deleting Wox application files
rmdir %LOCALAPPDATA%\Wox /S /Q

echo Deleting Wox configuration files
rmdir %APPDATA%\Wox /S /Q

echo Wox uninstalled and residous folders deleted. 
pause