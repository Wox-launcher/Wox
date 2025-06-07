package test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestEnvironmentSetup tests that the test environment is properly configured
func TestEnvironmentSetup(t *testing.T) {
	config := GetTestConfig()

	t.Logf("Test configuration loaded:")
	t.Logf("  Data Directory: %s", config.TestDataDirectory)
	t.Logf("  Log Directory: %s", config.TestLogDirectory)
	t.Logf("  User Directory: %s", config.TestUserDirectory)
	t.Logf("  Network Tests: %v", config.EnableNetworkTests)
	t.Logf("  Cleanup: %v", config.CleanupAfterTest)

	// Verify that test directories are different from production directories
	if config.TestDataDirectory == "" {
		t.Error("Test data directory should not be empty")
	}

	// Check that test directories contain "test" in the path
	if !containsTestInPath(config.TestDataDirectory) {
		t.Logf("Warning: Test data directory doesn't contain 'test' in path: %s", config.TestDataDirectory)
	}

	// Verify directories exist after initialization
	suite := NewTestSuite(t)
	if suite == nil {
		t.Fatal("Failed to create test suite")
	}

	// Check that directories were created
	if _, err := os.Stat(config.TestDataDirectory); os.IsNotExist(err) {
		t.Errorf("Test data directory was not created: %s", config.TestDataDirectory)
	}

	if _, err := os.Stat(config.TestLogDirectory); os.IsNotExist(err) {
		t.Errorf("Test log directory was not created: %s", config.TestLogDirectory)
	}

	if _, err := os.Stat(config.TestUserDirectory); os.IsNotExist(err) {
		t.Errorf("Test user directory was not created: %s", config.TestUserDirectory)
	}

	// Check that subdirectories were created
	expectedSubdirs := []string{
		"plugins", "themes", "settings", "cache", "images", "backup",
	}

	for _, subdir := range expectedSubdirs {
		subdirPath := filepath.Join(config.TestUserDirectory, subdir)
		if _, err := os.Stat(subdirPath); os.IsNotExist(err) {
			t.Errorf("Expected subdirectory was not created: %s", subdirPath)
		}
	}

	t.Logf("Test environment setup validation passed")
}

// TestEnvironmentIsolation tests that test environment doesn't interfere with production
func TestEnvironmentIsolation(t *testing.T) {
	config := GetTestConfig()

	// Test directories should be in temp or contain "test"
	testDirs := []string{
		config.TestDataDirectory,
		config.TestLogDirectory,
		config.TestUserDirectory,
	}

	for _, dir := range testDirs {
		if !isTestDirectory(dir) {
			t.Errorf("Directory doesn't appear to be a test directory: %s", dir)
		}
	}

	t.Logf("Test environment isolation validation passed")
}

// TestEnvironmentCleanup tests the cleanup functionality
func TestEnvironmentCleanup(t *testing.T) {
	// Create a temporary test config with cleanup enabled
	config := DefaultTestConfig()
	config.TestDataDirectory = filepath.Join(os.TempDir(), "wox-test-cleanup-test")
	config.CleanupAfterTest = true

	// Setup environment
	if err := config.SetupTestEnvironment(); err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}

	// Verify directory exists
	if _, err := os.Stat(config.TestDataDirectory); os.IsNotExist(err) {
		t.Fatalf("Test directory was not created: %s", config.TestDataDirectory)
	}

	// Cleanup
	if err := config.CleanupTestEnvironment(); err != nil {
		t.Errorf("Failed to cleanup test environment: %v", err)
	}

	// Verify directory was removed
	if _, err := os.Stat(config.TestDataDirectory); !os.IsNotExist(err) {
		t.Errorf("Test directory was not cleaned up: %s", config.TestDataDirectory)
	}

	t.Logf("Test environment cleanup validation passed")
}

// Helper functions
func containsTestInPath(path string) bool {
	return filepath.Base(path) == "test" ||
		filepath.Base(filepath.Dir(path)) == "test" ||
		filepath.Base(path) == "wox-test" ||
		filepath.Base(path) == "wox-test-data"
}

func isTestDirectory(path string) bool {
	// Check if path is in temp directory or contains "test"
	tempDir := os.TempDir()
	if strings.HasPrefix(path, tempDir) {
		return true
	}

	// Check if path contains "test" somewhere
	for p := path; p != "/" && p != "."; p = filepath.Dir(p) {
		base := filepath.Base(p)
		if base == "test" ||
			base == "wox-test" ||
			base == "wox-test-data" ||
			base == "wox-test-isolated" ||
			base == "wox-test-custom" ||
			base == "wox-test-debug" ||
			strings.HasPrefix(base, "wox-test-") {
			return true
		}
	}

	return false
}
