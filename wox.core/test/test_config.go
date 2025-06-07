package test

import (
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// TestConfig holds configuration for test execution
type TestConfig struct {
	// Network settings
	EnableNetworkTests bool
	NetworkTimeout     time.Duration

	// Test execution settings
	DefaultTimeout time.Duration
	MaxRetries     int
	RetryDelay     time.Duration

	// Parallel execution
	EnableParallel bool
	MaxParallel    int

	// Logging
	VerboseLogging   bool
	LogFailedQueries bool

	// Test environment directories
	TestDataDirectory string // Root directory for test data
	TestLogDirectory  string // Directory for test logs
	TestUserDirectory string // Directory for test user data
	CleanupAfterTest  bool   // Whether to cleanup test directories after tests
}

// DefaultTestConfig returns the default test configuration
func DefaultTestConfig() *TestConfig {
	// Create temporary test directories
	tempDir := os.TempDir()
	testDataDir := filepath.Join(tempDir, "wox-test-data")
	testLogDir := filepath.Join(testDataDir, "logs")
	testUserDir := filepath.Join(testDataDir, "user")

	return &TestConfig{
		EnableNetworkTests: true,
		NetworkTimeout:     45 * time.Second,
		DefaultTimeout:     30 * time.Second,
		MaxRetries:         3,
		RetryDelay:         1 * time.Second,
		EnableParallel:     false, // Disabled by default due to shared state
		MaxParallel:        4,
		VerboseLogging:     false,
		LogFailedQueries:   true,

		// Test environment directories
		TestDataDirectory: testDataDir,
		TestLogDirectory:  testLogDir,
		TestUserDirectory: testUserDir,
		CleanupAfterTest:  true, // Clean up by default
	}
}

// LoadTestConfigFromEnv loads test configuration from environment variables
func LoadTestConfigFromEnv() *TestConfig {
	config := DefaultTestConfig()

	// Network tests
	if val := os.Getenv("WOX_TEST_ENABLE_NETWORK"); val != "" {
		if enabled, err := strconv.ParseBool(val); err == nil {
			config.EnableNetworkTests = enabled
		}
	}

	// Network timeout
	if val := os.Getenv("WOX_TEST_NETWORK_TIMEOUT"); val != "" {
		if timeout, err := time.ParseDuration(val); err == nil {
			config.NetworkTimeout = timeout
		}
	}

	// Default timeout
	if val := os.Getenv("WOX_TEST_DEFAULT_TIMEOUT"); val != "" {
		if timeout, err := time.ParseDuration(val); err == nil {
			config.DefaultTimeout = timeout
		}
	}

	// Max retries
	if val := os.Getenv("WOX_TEST_MAX_RETRIES"); val != "" {
		if retries, err := strconv.Atoi(val); err == nil {
			config.MaxRetries = retries
		}
	}

	// Parallel execution
	if val := os.Getenv("WOX_TEST_ENABLE_PARALLEL"); val != "" {
		if enabled, err := strconv.ParseBool(val); err == nil {
			config.EnableParallel = enabled
		}
	}

	// Verbose logging
	if val := os.Getenv("WOX_TEST_VERBOSE"); val != "" {
		if verbose, err := strconv.ParseBool(val); err == nil {
			config.VerboseLogging = verbose
		}
	}

	// Test directories
	if val := os.Getenv("WOX_TEST_DATA_DIR"); val != "" {
		config.TestDataDirectory = val
		config.TestLogDirectory = filepath.Join(val, "logs")
		config.TestUserDirectory = filepath.Join(val, "user")
	}

	if val := os.Getenv("WOX_TEST_LOG_DIR"); val != "" {
		config.TestLogDirectory = val
	}

	if val := os.Getenv("WOX_TEST_USER_DIR"); val != "" {
		config.TestUserDirectory = val
	}

	// Cleanup setting
	if val := os.Getenv("WOX_TEST_CLEANUP"); val != "" {
		if cleanup, err := strconv.ParseBool(val); err == nil {
			config.CleanupAfterTest = cleanup
		}
	}

	return config
}

// GetTestConfig returns the current test configuration
func GetTestConfig() *TestConfig {
	return LoadTestConfigFromEnv()
}

// TestCategories defines different test categories
type TestCategory string

const (
	CategoryCalculator TestCategory = "calculator"
	CategoryConverter  TestCategory = "converter"
	CategoryPlugin     TestCategory = "plugin"
	CategoryTime       TestCategory = "time"
	CategoryNetwork    TestCategory = "network"
	CategorySystem     TestCategory = "system"
)

// TestFilter allows filtering tests by category or pattern
type TestFilter struct {
	Categories  []TestCategory
	Pattern     string
	SkipSlow    bool
	SkipNetwork bool
}

// ShouldRunTest determines if a test should run based on the filter
func (f *TestFilter) ShouldRunTest(category TestCategory, name string, isNetwork bool, isSlow bool) bool {
	// Check category filter
	if len(f.Categories) > 0 {
		found := false
		for _, cat := range f.Categories {
			if cat == category {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check network filter
	if f.SkipNetwork && isNetwork {
		return false
	}

	// Check slow test filter
	if f.SkipSlow && isSlow {
		return false
	}

	// Check pattern filter (simple contains check)
	if f.Pattern != "" && !contains(name, f.Pattern) {
		return false
	}

	return true
}

// SetupTestEnvironment creates and initializes test directories
func (c *TestConfig) SetupTestEnvironment() error {
	// Create test directories
	if err := os.MkdirAll(c.TestDataDirectory, 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(c.TestLogDirectory, 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(c.TestUserDirectory, 0755); err != nil {
		return err
	}

	// Create subdirectories that Wox expects
	subdirs := []string{
		"plugins", "themes", "settings", "cache", "images", "backup",
	}

	for _, subdir := range subdirs {
		if err := os.MkdirAll(filepath.Join(c.TestUserDirectory, subdir), 0755); err != nil {
			return err
		}
	}

	return nil
}

// CleanupTestEnvironment removes test directories if cleanup is enabled
func (c *TestConfig) CleanupTestEnvironment() error {
	if !c.CleanupAfterTest {
		return nil
	}

	return os.RemoveAll(c.TestDataDirectory)
}

// GetTestWoxDataDirectory returns the test equivalent of .wox directory
func (c *TestConfig) GetTestWoxDataDirectory() string {
	return c.TestDataDirectory
}

// GetTestUserDataDirectory returns the test user data directory
func (c *TestConfig) GetTestUserDataDirectory() string {
	return c.TestUserDirectory
}

// GetTestLogDirectory returns the test log directory
func (c *TestConfig) GetTestLogDirectory() string {
	return c.TestLogDirectory
}

// contains is a simple string contains check
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (len(substr) == 0 || s[len(s)-len(substr):] == substr ||
		s[:len(substr)] == substr || (len(s) > len(substr) &&
		func() bool {
			for i := 1; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}()))
}
