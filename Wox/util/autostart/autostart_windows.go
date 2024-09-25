package autostart

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows/registry"
)

func setAutostart(enable bool) error {
	key, _, err := registry.CreateKey(
		registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Run`,
		registry.ALL_ACCESS,
	)
	if err != nil {
		return fmt.Errorf("failed to access registry: %w", err)
	}
	defer key.Close()

	valueName := "WoxLauncher"

	if enable {
		exePath, err := os.Executable()
		if err != nil {
			return fmt.Errorf("failed to get executable path: %w", err)
		}
		err = key.SetStringValue(valueName, exePath)
		if err != nil {
			return fmt.Errorf("failed to set registry value: %w", err)
		}
	} else {
		err = key.DeleteValue(valueName)
		if err != nil && err != registry.ErrNotExist {
			return fmt.Errorf("failed to delete registry value: %w", err)
		}
	}

	return nil
}
