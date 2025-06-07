package test

import (
	"fmt"
	"os"
	"path/filepath"
	"wox/util"
)

// TestLocation is a test-specific implementation of Location that uses isolated directories
type TestLocation struct {
	config *TestConfig
	*util.Location // Embed the original Location
}

// NewTestLocation creates a new test location with isolated directories
func NewTestLocation(config *TestConfig) (*TestLocation, error) {
	// Setup test environment
	if err := config.SetupTestEnvironment(); err != nil {
		return nil, fmt.Errorf("failed to setup test environment: %w", err)
	}
	
	// Create a new location instance
	originalLocation := util.GetLocation()
	
	testLocation := &TestLocation{
		config:   config,
		Location: originalLocation,
	}
	
	return testLocation, nil
}

// Override methods to use test directories
func (tl *TestLocation) GetWoxDataDirectory() string {
	return tl.config.TestDataDirectory
}

func (tl *TestLocation) GetLogDirectory() string {
	return tl.config.TestLogDirectory
}

func (tl *TestLocation) GetUserDataDirectory() string {
	return tl.config.TestUserDirectory
}

func (tl *TestLocation) GetLogPluginDirectory() string {
	return filepath.Join(tl.GetLogDirectory(), "plugins")
}

func (tl *TestLocation) GetLogHostsDirectory() string {
	return filepath.Join(tl.GetLogDirectory(), "hosts")
}

func (tl *TestLocation) GetPluginDirectory() string {
	return filepath.Join(tl.GetUserDataDirectory(), "plugins")
}

func (tl *TestLocation) GetUserScriptPluginsDirectory() string {
	return filepath.Join(tl.GetPluginDirectory(), "scripts")
}

func (tl *TestLocation) GetThemeDirectory() string {
	return filepath.Join(tl.GetUserDataDirectory(), "themes")
}

func (tl *TestLocation) GetPluginSettingDirectory() string {
	return filepath.Join(tl.GetUserDataDirectory(), "settings")
}

func (tl *TestLocation) GetWoxSettingPath() string {
	return filepath.Join(tl.GetPluginSettingDirectory(), "wox.json")
}

func (tl *TestLocation) GetWoxAppDataPath() string {
	return filepath.Join(tl.GetPluginSettingDirectory(), "wox.data.json")
}

func (tl *TestLocation) GetHostDirectory() string {
	return filepath.Join(tl.GetWoxDataDirectory(), "hosts")
}

func (tl *TestLocation) GetUpdatesDirectory() string {
	return filepath.Join(tl.GetWoxDataDirectory(), "updates")
}

func (tl *TestLocation) GetUIDirectory() string {
	return filepath.Join(tl.GetWoxDataDirectory(), "ui")
}

func (tl *TestLocation) GetOthersDirectory() string {
	return filepath.Join(tl.GetWoxDataDirectory(), "others")
}

func (tl *TestLocation) GetScriptPluginTemplatesDirectory() string {
	return filepath.Join(tl.GetWoxDataDirectory(), "script_plugin_templates")
}

func (tl *TestLocation) GetCacheDirectory() string {
	return filepath.Join(tl.GetWoxDataDirectory(), "cache")
}

func (tl *TestLocation) GetImageCacheDirectory() string {
	return filepath.Join(tl.GetCacheDirectory(), "images")
}

func (tl *TestLocation) GetBackupDirectory() string {
	return filepath.Join(tl.GetWoxDataDirectory(), "backup")
}

func (tl *TestLocation) GetAppLockPath() string {
	return filepath.Join(tl.GetWoxDataDirectory(), "wox.lock")
}

func (tl *TestLocation) GetUserDataDirectoryShortcutPath() string {
	return filepath.Join(tl.GetWoxDataDirectory(), ".userdata.location")
}

// InitTestDirectories creates all necessary test directories
func (tl *TestLocation) InitTestDirectories() error {
	directories := []string{
		tl.GetWoxDataDirectory(),
		tl.GetLogDirectory(),
		tl.GetUserDataDirectory(),
		tl.GetLogPluginDirectory(),
		tl.GetLogHostsDirectory(),
		tl.GetPluginDirectory(),
		tl.GetUserScriptPluginsDirectory(),
		tl.GetThemeDirectory(),
		tl.GetPluginSettingDirectory(),
		tl.GetHostDirectory(),
		tl.GetUpdatesDirectory(),
		tl.GetUIDirectory(),
		tl.GetOthersDirectory(),
		tl.GetScriptPluginTemplatesDirectory(),
		tl.GetCacheDirectory(),
		tl.GetImageCacheDirectory(),
		tl.GetBackupDirectory(),
	}
	
	for _, dir := range directories {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}
	
	// Create the shortcut file
	shortcutPath := tl.GetUserDataDirectoryShortcutPath()
	if err := os.WriteFile(shortcutPath, []byte(tl.GetUserDataDirectory()), 0644); err != nil {
		return fmt.Errorf("failed to create shortcut file: %w", err)
	}
	
	return nil
}

// Cleanup removes all test directories
func (tl *TestLocation) Cleanup() error {
	return tl.config.CleanupTestEnvironment()
}
