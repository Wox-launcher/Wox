package audio

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"wox/util"
)

// getSystemVolume returns the current system output volume (0-100) on macOS
// via osascript.
func getSystemVolume() (int, error) {
	out, err := exec.Command("osascript", "-e", "output volume of (get volume settings)").Output()
	if err != nil {
		util.GetLogger().Error(context.Background(), fmt.Sprintf("audio: getSystemVolume osascript failed: %s, stderr: %s", err.Error(), string(out)))
		return 0, fmt.Errorf("failed to get system volume: %w", err)
	}
	vol, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 0, fmt.Errorf("failed to parse volume %q: %w", strings.TrimSpace(string(out)), err)
	}
	util.GetLogger().Info(context.Background(), fmt.Sprintf("audio: getSystemVolume returned %d", vol))
	return vol, nil
}

// setSystemVolume sets the system output volume (0-100) on macOS via osascript.
func setSystemVolume(volume int) error {
	out, err := exec.Command("osascript", "-e", fmt.Sprintf("set volume output volume %d", volume)).CombinedOutput()
	if err != nil {
		util.GetLogger().Error(context.Background(), fmt.Sprintf("audio: setSystemVolume osascript failed: %s, output: %s", err.Error(), string(out)))
		return fmt.Errorf("failed to set system volume: %w", err)
	}
	util.GetLogger().Info(context.Background(), fmt.Sprintf("audio: setSystemVolume set to %d, output: %s", volume, strings.TrimSpace(string(out))))
	return nil
}