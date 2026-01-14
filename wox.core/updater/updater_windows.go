package updater

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"wox/util"
	"wox/util/shell"
)

func init() {
	applyUpdaterInstance = &WindowsUpdater{}
}

type WindowsUpdater struct{}

const windowsUpdateScript = `@echo off
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
start "" "%TARGET%" "--update"
echo [%date% %time%] launched >> "%LOG%"
endlocal
del "%~f0" >nul 2>&1
`

func getExecutablePath() (string, error) {
	return os.Executable()
}

func (u *WindowsUpdater) ApplyUpdate(ctx context.Context, pid int, oldPath, newPath string, progress ApplyUpdateProgressCallback) error {
	backupPath := fmt.Sprintf("%s.old", oldPath)

	reportApplyProgress(progress, ApplyUpdateStageReplacing)
	util.GetLogger().Info(ctx, "replacing Windows executable in-place")
	if err := os.Rename(oldPath, backupPath); err != nil {
		return fmt.Errorf("failed to rename current executable: %w", err)
	} else {
		util.GetLogger().Info(ctx, "current executable renamed to backup successfully")
		hideBackupExecutable(ctx, backupPath)
	}

	util.GetLogger().Info(ctx, "moving new executable to current location")
	if err := os.Rename(newPath, oldPath); err != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("failed to move new executable to current location, attempting to restore backup: %v", err))

		restoreErr := os.Rename(backupPath, oldPath)
		if restoreErr != nil {
			util.GetLogger().Error(ctx, fmt.Sprintf("failed to restore backup executable: %v", restoreErr))
		} else {
			util.GetLogger().Info(ctx, "backup executable restored successfully")
		}

		return fmt.Errorf("failed to move new executable to current location: %w", err)
	} else {
		util.GetLogger().Info(ctx, "new executable moved to current location successfully")
	}

	reportApplyProgress(progress, ApplyUpdateStageRestarting)
	util.GetLogger().Info(ctx, "starting updated application")
	logPath := filepath.Join(util.GetLocation().GetLogDirectory(), "update.log")
	scriptPath, scriptErr := writeWindowsUpdateScript()
	if scriptErr != nil {
		return fmt.Errorf("failed to create windows update restart script: %w", scriptErr)
	}
	if _, err := shell.Run("cmd.exe", "/c", "call", scriptPath, oldPath, logPath); err != nil {
		return fmt.Errorf("failed to start updated application: %w", err)
	}

	util.GetLogger().Info(ctx, "exiting application for update to proceed")
	os.Exit(0)

	return nil // This line will never be reached due to os.Exit(0)
}

func hideBackupExecutable(ctx context.Context, path string) {
	ptr, ptrErr := syscall.UTF16PtrFromString(path)
	if ptrErr != nil {
		util.GetLogger().Warn(ctx, fmt.Sprintf("failed to encode backup executable path: %v", ptrErr))
		return
	}

	attrs, attrErr := syscall.GetFileAttributes(ptr)
	if attrErr != nil {
		util.GetLogger().Warn(ctx, fmt.Sprintf("failed to get backup executable attributes: %v", attrErr))
		return
	}

	if attrs&syscall.FILE_ATTRIBUTE_HIDDEN != 0 {
		return
	}

	if err := syscall.SetFileAttributes(ptr, attrs|syscall.FILE_ATTRIBUTE_HIDDEN); err != nil {
		util.GetLogger().Warn(ctx, fmt.Sprintf("failed to hide backup executable: %v", err))
	}
}

func writeWindowsUpdateScript() (string, error) {
	tempFile, err := os.CreateTemp("", "wox_update_*.cmd")
	if err != nil {
		return "", err
	}
	defer tempFile.Close()

	if _, err := tempFile.WriteString(windowsUpdateScript); err != nil {
		_ = os.Remove(tempFile.Name())
		return "", err
	}

	return tempFile.Name(), nil
}
