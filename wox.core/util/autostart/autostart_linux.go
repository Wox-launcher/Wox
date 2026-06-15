package autostart

import (
	"fmt"
	"os"
	"wox/util"
)

func setAutostart(enable bool) error {
	desktopFilePath, err := util.LinuxAutostartDesktopEntryPath()
	if err != nil {
		return err
	}

	if enable {
		if err := util.WriteLinuxDesktopEntry(desktopFilePath, false, true); err != nil {
			return err
		}
		return nil
	}

	return removeFileIfExists(desktopFilePath)
}

func isAutostart() (bool, error) {
	desktopFilePath, err := util.LinuxAutostartDesktopEntryPath()
	if err != nil {
		return false, err
	}

	return fileExists(desktopFilePath)
}

func fileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to check file: %w", err)
	}
	return true, nil
}

func removeFileIfExists(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove file: %w", err)
	}
	return nil
}
