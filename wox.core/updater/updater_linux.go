package updater

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"wox/util"
)

func init() {
	applyUpdaterInstance = &LinuxUpdater{}
}

type LinuxUpdater struct{}

const linuxUpdateScript = `
PID="$1"
OLD_PATH="$2"
NEW_PATH="$3"
LOG_FILE="$4"

trap 'rm -f "$0"' EXIT

if [ -z "$PID" ] || [ -z "$OLD_PATH" ] || [ -z "$NEW_PATH" ]; then
  echo "$(date "+%Y-%m-%d %H:%M:%S") missing required args" >> "$LOG_FILE"
  exit 1
fi

if [ -z "$LOG_FILE" ]; then
  LOG_FILE="/tmp/wox_update.log"
fi

log() {
  local now
  now=$(date "+%Y-%m-%d %H:%M:%S")
  echo "$now $1" >> "$LOG_FILE"
  echo "$1"
}

log "Update process started"
log "Old path: $OLD_PATH"
log "New path: $NEW_PATH"

WAIT_COUNT=0
while ps -p "$PID" > /dev/null 2>&1; do
	if [ $((WAIT_COUNT % 5)) -eq 0 ]; then
		log "Waiting for application process $PID to exit"
	fi
  sleep 1
	WAIT_COUNT=$((WAIT_COUNT + 1))
done

log "Application exited, replacing executable"
# Remove the old file first to avoid ETXTBSY error (AppImage FUSE mount may still hold the file open)
rm -f "$OLD_PATH"
CP_OUTPUT=$(cp "$NEW_PATH" "$OLD_PATH" 2>&1)
if [ $? -ne 0 ]; then
  log "Failed to copy executable: $CP_OUTPUT"
  log "ERROR: Failed to copy executable"
  exit 1
fi

chmod +x "$OLD_PATH"

log "Launching updated application"
"$OLD_PATH" --updated >> "$LOG_FILE" 2>&1 &
log "Update process completed"
`

func getExecutablePath() (string, error) {
	if appImagePath := os.Getenv("APPIMAGE"); appImagePath != "" {
		return appImagePath, nil
	}
	return os.Executable()
}

func (u *LinuxUpdater) ApplyUpdate(ctx context.Context, pid int, oldPath, newPath string, progress ApplyUpdateProgressCallback) error {
	// Execute the shell script
	reportApplyProgress(progress, ApplyUpdateStageRestarting)
	util.GetLogger().Info(ctx, "starting Linux update process")
	logPath := filepath.Join(util.GetLocation().GetLogDirectory(), "update.log")
	scriptPath, err := writeLinuxUpdateScript()
	if err != nil {
		return fmt.Errorf("failed to create Linux update script: %w", err)
	}
	cmd := exec.Command("bash", scriptPath, fmt.Sprintf("%d", pid), oldPath, newPath, logPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		_ = os.Remove(scriptPath)
		return fmt.Errorf("failed to start update process: %w", err)
	}
	if cmd.Process != nil {
		_ = cmd.Process.Release()
	}

	// Exit the application
	util.GetLogger().Info(ctx, "exiting application for update to proceed")
	os.Exit(0)

	return nil // This line will never be reached due to os.Exit(0)
}

func cleanupBackupExecutable(_ context.Context) {
}

func writeLinuxUpdateScript() (string, error) {
	tempFile, err := os.CreateTemp("", "wox_update_*.sh")
	if err != nil {
		return "", err
	}

	if _, err := tempFile.WriteString(linuxUpdateScript); err != nil {
		_ = tempFile.Close()
		_ = os.Remove(tempFile.Name())
		return "", err
	}

	if err := tempFile.Close(); err != nil {
		_ = os.Remove(tempFile.Name())
		return "", err
	}

	if err := os.Chmod(tempFile.Name(), 0o700); err != nil {
		_ = os.Remove(tempFile.Name())
		return "", err
	}

	return tempFile.Name(), nil
}
