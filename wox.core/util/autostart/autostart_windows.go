package autostart

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows/registry"
)

func setAutostart(enable bool) error {
	// Set the main Run registry entry
	runKey, _, err := registry.CreateKey(
		registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Run`,
		registry.ALL_ACCESS,
	)
	if err != nil {
		return fmt.Errorf("failed to access Run registry: %w", err)
	}
	defer runKey.Close()

	valueName := "WoxLauncher"

	if enable {
		exePath, err := os.Executable()
		if err != nil {
			return fmt.Errorf("failed to get executable path: %w", err)
		}

		err = runKey.SetStringValue(valueName, exePath)
		if err != nil {
			return fmt.Errorf("failed to set Run registry value: %w", err)
		}

		// Verify the value was set correctly
		verifyValue, _, verifyErr := runKey.GetStringValue(valueName)
		if verifyErr != nil {
			return fmt.Errorf("failed to verify Run registry value: %w", verifyErr)
		}
		if verifyValue != exePath {
			return fmt.Errorf("Run registry value verification failed: expected %s, got %s", exePath, verifyValue)
		}

		// Set the StartupApproved entry to enable the startup app in Windows Settings
		err = setStartupApproved(valueName, true)
		if err != nil {
			return fmt.Errorf("failed to set StartupApproved: %w", err)
		}
	} else {
		err = runKey.DeleteValue(valueName)
		if err != nil && err != registry.ErrNotExist {
			return fmt.Errorf("failed to delete Run registry value: %w", err)
		}

		// Remove the StartupApproved entry
		err = removeStartupApproved(valueName)
		if err != nil {
			return fmt.Errorf("failed to remove StartupApproved: %w", err)
		}
	}

	return nil
}

func isAutostart() (bool, error) {
	runKey, _, err := registry.CreateKey(
		registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Run`,
		registry.QUERY_VALUE,
	)
	if err != nil {
		return false, fmt.Errorf("failed to access Run registry: %w", err)
	}
	defer runKey.Close()

	valueName := "WoxLauncher"
	value, _, err := runKey.GetStringValue(valueName)
	if err == registry.ErrNotExist {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to get Run registry value: %w", err)
	}

	// Check if the executable path still exists
	if _, statErr := os.Stat(value); os.IsNotExist(statErr) {
		// The registered executable doesn't exist, this means autostart is broken
		// We should return false so the system can detect and fix this
		return false, nil
	}

	// Check StartupApproved status - if it exists and is disabled, return false
	approved, err := isStartupApproved(valueName)
	if err != nil {
		// If we can't read StartupApproved, assume it's enabled (backward compatibility)
		return true, nil
	}

	return approved, nil
}

// setStartupApproved sets the StartupApproved registry entry to enable/disable startup app in Windows Settings
func setStartupApproved(valueName string, enabled bool) error {
	approvedKey, _, err := registry.CreateKey(
		registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Explorer\StartupApproved\Run`,
		registry.ALL_ACCESS,
	)
	if err != nil {
		return fmt.Errorf("failed to access StartupApproved registry: %w", err)
	}
	defer approvedKey.Close()

	// The StartupApproved value is a binary value
	// Enabled: starts with 0x02 0x00 0x00 0x00 0x00 0x00 0x00 0x00 0x00 0x00 0x00 0x00
	// Disabled: starts with 0x03 0x00 0x00 0x00 0x00 0x00 0x00 0x00 0x00 0x00 0x00 0x00
	var binaryValue []byte
	if enabled {
		// Enabled state - first byte is 0x02
		binaryValue = []byte{0x02, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	} else {
		// Disabled state - first byte is 0x03
		binaryValue = []byte{0x03, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	}

	err = approvedKey.SetBinaryValue(valueName, binaryValue)
	if err != nil {
		return fmt.Errorf("failed to set StartupApproved binary value: %w", err)
	}

	return nil
}

// removeStartupApproved removes the StartupApproved registry entry
func removeStartupApproved(valueName string) error {
	approvedKey, _, err := registry.CreateKey(
		registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Explorer\StartupApproved\Run`,
		registry.ALL_ACCESS,
	)
	if err != nil {
		return fmt.Errorf("failed to access StartupApproved registry: %w", err)
	}
	defer approvedKey.Close()

	err = approvedKey.DeleteValue(valueName)
	if err != nil && err != registry.ErrNotExist {
		return fmt.Errorf("failed to delete StartupApproved value: %w", err)
	}

	return nil
}

// isStartupApproved checks if the startup app is approved (enabled) in Windows Settings
func isStartupApproved(valueName string) (bool, error) {
	approvedKey, _, err := registry.CreateKey(
		registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Explorer\StartupApproved\Run`,
		registry.QUERY_VALUE,
	)
	if err != nil {
		return false, fmt.Errorf("failed to access StartupApproved registry: %w", err)
	}
	defer approvedKey.Close()

	binaryValue, _, err := approvedKey.GetBinaryValue(valueName)
	if err == registry.ErrNotExist {
		// If StartupApproved entry doesn't exist, assume it's enabled
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to get StartupApproved binary value: %w", err)
	}

	// Check the first byte: 0x02 = enabled, 0x03 = disabled
	if len(binaryValue) > 0 {
		return binaryValue[0] == 0x02, nil
	}

	// If binary value is empty, assume enabled
	return true, nil
}
