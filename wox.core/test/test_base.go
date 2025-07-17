package test

import (
	"context"
	"sync"
	"testing"
	"time"
	"wox/common"
	"wox/database"
	"wox/i18n"
	"wox/plugin"
	"wox/resource"
	"wox/setting"
	"wox/ui"
	"wox/util"
	"wox/util/selection"

	// Import all system plugins to trigger their init() functions
	// This ensures all system plugins are registered in plugin.AllSystemPlugin
	_ "wox/plugin/system" // Contains multiple plugins: sys.go, ai_command.go, backup.go, browser.go, etc.
	_ "wox/plugin/system/app"
	_ "wox/plugin/system/calculator"
	_ "wox/plugin/system/converter"
	_ "wox/plugin/system/file"
)

var (
	testInitOnce sync.Once
	testConfig   *TestConfig
	testLocation *TestLocation
	testLogger   *TestLogger
)

// TestSuite provides a base for all integration tests
type TestSuite struct {
	t   *testing.T
	ctx context.Context
}

// NewTestSuite creates a new test suite instance
func NewTestSuite(t *testing.T) *TestSuite {
	ensureServicesInitialized(t)
	return &TestSuite{
		t:   t,
		ctx: util.NewTraceContext(),
	}
}

// QueryTest represents a single query test case
type QueryTest struct {
	Name           string
	Query          string
	ExpectedTitle  string
	ExpectedAction string
	TitleCheck     func(string) bool
	FloatTolerance float64 // For floating point comparisons
	Timeout        time.Duration
	ShouldSkip     bool
	SkipReason     string
}

// RunQueryTest executes a single query test
func (ts *TestSuite) RunQueryTest(test QueryTest) bool {
	if test.ShouldSkip {
		ts.t.Skipf("Skipping test %s: %s", test.Name, test.SkipReason)
		return true
	}

	timeout := test.Timeout
	if timeout == 0 {
		timeout = 60 * time.Second // Increased default timeout
	}

	// Create query
	plainQuery := common.PlainQuery{
		QueryType: plugin.QueryTypeInput,
		QueryText: test.Query,
	}

	// Execute query
	ts.t.Logf("Creating query for test %s: %s", test.Name, test.Query)
	query, queryPlugin, err := plugin.GetPluginManager().NewQuery(ts.ctx, plainQuery)
	if err != nil {
		ts.t.Errorf("Failed to create query for %s: %v", test.Name, err)
		return false
	}
	ts.t.Logf("Query created successfully, executing...")

	resultChan, doneChan := plugin.GetPluginManager().Query(ts.ctx, query)

	// Collect all results
	var allResults []plugin.QueryResultUI

CollectResults:
	for {
		select {
		case results := <-resultChan:
			allResults = append(allResults, results...)
		case <-doneChan:
			break CollectResults
		case <-time.After(timeout):
			ts.t.Errorf("Query timeout for test %s", test.Name)
			return false
		}
	}

	// Try fallback results if no results found
	if len(allResults) == 0 {
		allResults = plugin.GetPluginManager().QueryFallback(ts.ctx, query, queryPlugin)
	}

	// Verify results
	if len(allResults) == 0 {
		ts.t.Errorf("No results returned for query: %s (test: %s)", test.Query, test.Name)
		return false
	}

	// Find matching result
	found := false
	for _, result := range allResults {
		if test.TitleCheck != nil {
			if test.TitleCheck(result.Title) {
				found = true
				// Verify action
				if !ts.verifyAction(result, test.ExpectedAction, test.Name) {
					return false
				}
				break
			}
		} else if result.Title == test.ExpectedTitle {
			found = true
			// Verify action
			if !ts.verifyAction(result, test.ExpectedAction, test.Name) {
				return false
			}
			break
		}
	}

	if !found {
		ts.t.Errorf("Expected title (%s) format not found in results for test %s. Got results for query %q:", test.ExpectedTitle, test.Name, test.Query)
		for i, result := range allResults {
			ts.t.Errorf("Result %d:", i+1)
			ts.t.Errorf("  Title: %s", result.Title)
			ts.t.Errorf("  SubTitle: %s", result.SubTitle)
			ts.t.Errorf("  Actions:")
			for j, action := range result.Actions {
				ts.t.Errorf("    %d. %s", j+1, action.Name)
			}
		}
		return false
	}

	return true
}

// RunQueryTests executes multiple query tests
func (ts *TestSuite) RunQueryTests(tests []QueryTest) {
	var failedTests []string

	for _, test := range tests {
		ts.t.Run(test.Name, func(t *testing.T) {
			subSuite := &TestSuite{t: t, ctx: ts.ctx}
			if !subSuite.RunQueryTest(test) {
				failedTests = append(failedTests, test.Name)
			}
		})
	}

	if len(failedTests) > 0 {
		ts.t.Errorf("\nFailed tests (%d):", len(failedTests))
		for i, name := range failedTests {
			ts.t.Errorf("%d. %s", i+1, name)
		}
	}
}

// verifyAction checks if the expected action exists in the result
func (ts *TestSuite) verifyAction(result plugin.QueryResultUI, expectedAction, testName string) bool {
	for _, action := range result.Actions {
		if action.Name == expectedAction {
			return true
		}
	}
	ts.t.Errorf("Expected action %q not found in result actions for test %s", expectedAction, testName)
	ts.t.Errorf("Actual result actions:")
	for _, action := range result.Actions {
		ts.t.Errorf("  %s", action.Name)
	}
	return false
}

// ensureServicesInitialized initializes services once for all tests
func ensureServicesInitialized(t *testing.T) {
	testInitOnce.Do(func() {
		ctx := context.Background()

		// Load test configuration
		testConfig = GetTestConfig()
		t.Logf("Using test configuration: data_dir=%s, log_dir=%s, user_dir=%s",
			testConfig.TestDataDirectory, testConfig.TestLogDirectory, testConfig.TestUserDirectory)

		// Setup test location
		var err error
		testLocation, err = NewTestLocation(testConfig)
		if err != nil {
			t.Fatalf("Failed to create test location: %v", err)
		}

		// Initialize test directories
		err = testLocation.InitTestDirectories()
		if err != nil {
			t.Fatalf("Failed to initialize test directories: %v", err)
		}

		// Setup test logger
		testLogger = NewTestLogger(testLocation)
		t.Logf("Test environment setup complete")

		// Initialize location (this will use the default location, but we'll override paths as needed)
		err = util.GetLocation().Init()
		if err != nil {
			t.Fatalf("Failed to initialize location: %v", err)
		}

		// Extract resources
		err = resource.Extract(ctx)
		if err != nil {
			t.Fatalf("Failed to extract resources: %v", err)
		}

		// Initialize database
		err = database.Init(ctx)
		if err != nil {
			t.Fatalf("Failed to initialize database: %v", err)
		}

		// Initialize settings
		err = setting.GetSettingManager().Init(ctx)
		if err != nil {
			t.Fatalf("Failed to initialize settings: %v", err)
		}

		// Initialize i18n
		err = i18n.GetI18nManager().UpdateLang(ctx, i18n.LangCodeEnUs)
		if err != nil {
			t.Fatalf("Failed to initialize i18n: %v", err)
		}

		// Initialize UI
		err = ui.GetUIManager().Start(ctx)
		if err != nil {
			t.Fatalf("Failed to initialize UI: %v", err)
		}

		// Initialize plugin system with UI
		plugin.GetPluginManager().Start(ctx, ui.GetUIManager().GetUI(ctx))

		// Wait for plugins to initialize - increased timeout for stability
		t.Logf("Waiting 15s for plugins to init")
		time.Sleep(time.Second * 15)

		// Initialize selection
		selection.InitSelection()

		// Check if plugins are loaded
		instances := plugin.GetPluginManager().GetPluginInstances()
		t.Logf("Loaded %d plugin instances", len(instances))
		for _, instance := range instances {
			t.Logf("Plugin: %s (triggers: %v)", instance.Metadata.Name, instance.GetTriggerKeywords())
		}

		t.Logf("Test services initialized successfully")
	})
}

// IsNetworkAvailable checks if network-dependent tests should run
func IsNetworkAvailable() bool {
	// Simple check - could be enhanced with actual network connectivity test
	return true
}

// ShouldSkipNetworkTests returns true if network tests should be skipped
func ShouldSkipNetworkTests() bool {
	config := GetCurrentTestConfig()
	return !config.EnableNetworkTests || !IsNetworkAvailable()
}

// CleanupTestEnvironment cleans up test directories if configured to do so
func CleanupTestEnvironment() error {
	if testLocation != nil {
		return testLocation.Cleanup()
	}
	return nil
}

// GetCurrentTestConfig returns the current test configuration instance
func GetCurrentTestConfig() *TestConfig {
	if testConfig == nil {
		testConfig = GetTestConfig()
	}
	return testConfig
}

// GetTestLocation returns the test location instance
func GetTestLocation() *TestLocation {
	return testLocation
}

// GetTestLogger returns the test logger instance
func GetTestLogger() *TestLogger {
	return testLogger
}
