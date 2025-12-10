package test

import (
	"fmt"
	"os"
	"testing"
)

// TestRunner manages the test execution lifecycle
type TestRunner struct {
	config *TestConfig
}

// NewTestRunner creates a new test runner with the given configuration
func NewTestRunner(config *TestConfig) *TestRunner {
	return &TestRunner{
		config: config,
	}
}

// RunWithCleanup runs a test function and ensures cleanup is performed
func (tr *TestRunner) RunWithCleanup(t *testing.T, testFunc func(*testing.T)) {
	// Setup test environment
	if err := tr.config.SetupTestEnvironment(); err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}

	// Ensure cleanup happens even if test panics
	defer func() {
		if err := tr.config.CleanupTestEnvironment(); err != nil {
			t.Errorf("Failed to cleanup test environment: %v", err)
		}
	}()

	// Run the test
	testFunc(t)
}

// PrintTestEnvironmentInfo prints information about the test environment
func (tr *TestRunner) PrintTestEnvironmentInfo(t *testing.T) {
	t.Logf("=== Test Environment Configuration ===")
	t.Logf("Test Data Directory: %s", tr.config.TestDataDirectory)
	t.Logf("Test Log Directory: %s", tr.config.TestLogDirectory)
	t.Logf("Test User Directory: %s", tr.config.TestUserDirectory)
	t.Logf("Network Tests Enabled: %v", tr.config.EnableNetworkTests)
	t.Logf("Cleanup After Test: %v", tr.config.CleanupAfterTest)
	t.Logf("Verbose Logging: %v", tr.config.VerboseLogging)
	t.Logf("=====================================")
}

// Example usage function
func ExampleTestWithIsolatedEnvironment(t *testing.T) {
	// Create a custom test configuration
	config := DefaultTestConfig()
	config.TestDataDirectory = "/tmp/wox-test-custom"
	config.CleanupAfterTest = true
	config.VerboseLogging = true

	// Create test runner
	runner := NewTestRunner(config)
	runner.PrintTestEnvironmentInfo(t)

	// Run test with automatic cleanup
	runner.RunWithCleanup(t, func(t *testing.T) {
		// Your test code here
		suite := NewTestSuite(t)

		test := QueryTest{
			Name:           "Example test",
			Query:          "1+1",
			ExpectedTitle:  "2",
			ExpectedAction: "Copy",
		}

		if !suite.RunQueryTest(test) {
			t.Errorf("Example test failed")
		}
	})
}

// SetupTestEnvironmentFromEnv sets up test environment based on environment variables
func SetupTestEnvironmentFromEnv() error {
	config := GetTestConfig()
	return config.SetupTestEnvironment()
}

// CleanupTestEnvironmentFromEnv cleans up test environment based on environment variables
func CleanupTestEnvironmentFromEnv() error {
	config := GetTestConfig()
	return config.CleanupTestEnvironment()
}

// ValidateTestEnvironment checks if the test environment is properly configured
func ValidateTestEnvironment(t *testing.T) error {
	config := GetTestConfig()

	// Check if directories exist
	if _, err := os.Stat(config.TestDataDirectory); os.IsNotExist(err) {
		return fmt.Errorf("test data directory does not exist: %s", config.TestDataDirectory)
	}

	if _, err := os.Stat(config.TestLogDirectory); os.IsNotExist(err) {
		return fmt.Errorf("test log directory does not exist: %s", config.TestLogDirectory)
	}

	if _, err := os.Stat(config.TestUserDirectory); os.IsNotExist(err) {
		return fmt.Errorf("test user directory does not exist: %s", config.TestUserDirectory)
	}

	t.Logf("Test environment validation passed")
	return nil
}
