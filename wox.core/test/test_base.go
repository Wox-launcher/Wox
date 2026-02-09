package test

import (
	"context"
	"strings"
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

type QueryTestFailure struct {
	Name           string
	Query          string
	ExpectedTitle  string
	ExpectedAction string
	HasTitleCheck  bool
	ActualActions  []string
	Reason         string
}

// RunQueryTest executes a single query test
func (ts *TestSuite) RunQueryTest(test QueryTest) (bool, *QueryTestFailure) {
	if test.ShouldSkip {
		ts.t.Skipf("Skipping test %s: %s", test.Name, test.SkipReason)
		return true, nil
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
		return false, &QueryTestFailure{
			Name:           test.Name,
			Query:          test.Query,
			ExpectedTitle:  test.ExpectedTitle,
			ExpectedAction: test.ExpectedAction,
			HasTitleCheck:  test.TitleCheck != nil,
			Reason:         "query_create_failed",
		}
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
			// Query completion only means all plugin goroutines have finished.
			// Drain buffered results to avoid dropping late-collected plugin results.
			for {
				select {
				case results := <-resultChan:
					allResults = append(allResults, results...)
				default:
					break CollectResults
				}
			}
		case <-time.After(timeout):
			ts.t.Errorf("Query timeout for test %s", test.Name)
			return false, &QueryTestFailure{
				Name:           test.Name,
				Query:          test.Query,
				ExpectedTitle:  test.ExpectedTitle,
				ExpectedAction: test.ExpectedAction,
				HasTitleCheck:  test.TitleCheck != nil,
				Reason:         "query_timeout",
			}
		}
	}

	// Try fallback results if no results found
	if len(allResults) == 0 {
		allResults = plugin.GetPluginManager().QueryFallback(ts.ctx, query, queryPlugin)
	}

	// Verify results
	if len(allResults) == 0 {
		ts.t.Errorf("No results returned for query: %s (test: %s)", test.Query, test.Name)
		return false, &QueryTestFailure{
			Name:           test.Name,
			Query:          test.Query,
			ExpectedTitle:  test.ExpectedTitle,
			ExpectedAction: test.ExpectedAction,
			HasTitleCheck:  test.TitleCheck != nil,
			Reason:         "no_results",
		}
	}

	// Find matching result
	found := false
	for _, result := range allResults {
		if test.TitleCheck != nil {
			if test.TitleCheck(result.Title) {
				found = true
				// Verify action
				ok, actualActions := ts.verifyAction(result, test.ExpectedAction, test.Name, test.Query)
				if !ok {
					return false, &QueryTestFailure{
						Name:           test.Name,
						Query:          test.Query,
						ExpectedTitle:  test.ExpectedTitle,
						ExpectedAction: test.ExpectedAction,
						HasTitleCheck:  test.TitleCheck != nil,
						ActualActions:  actualActions,
						Reason:         "action_mismatch",
					}
				}
				break
			}
		} else if result.Title == test.ExpectedTitle {
			found = true
			// Verify action
			ok, actualActions := ts.verifyAction(result, test.ExpectedAction, test.Name, test.Query)
			if !ok {
				return false, &QueryTestFailure{
					Name:           test.Name,
					Query:          test.Query,
					ExpectedTitle:  test.ExpectedTitle,
					ExpectedAction: test.ExpectedAction,
					HasTitleCheck:  test.TitleCheck != nil,
					ActualActions:  actualActions,
					Reason:         "action_mismatch",
				}
			}
			break
		}
	}

	if !found {
		expectedTitle := test.ExpectedTitle
		if test.TitleCheck != nil {
			expectedTitle = "custom TitleCheck"
		}
		ts.t.Errorf("[Test %s] Expected title (%s) not found for query %q", test.Name, expectedTitle, test.Query)
		ts.t.Errorf("[Test %s] Got %d result(s):", test.Name, len(allResults))
		for i, result := range allResults {
			actionNames := make([]string, 0, len(result.Actions))
			for _, action := range result.Actions {
				actionNames = append(actionNames, action.Name)
			}
			ts.t.Errorf("[Test %s] #%d title=%q subtitle=%q actions=%s", test.Name, i+1, result.Title, result.SubTitle, strings.Join(actionNames, ", "))
		}
		var actualActions []string
		if len(allResults) > 0 {
			for _, action := range allResults[0].Actions {
				actualActions = append(actualActions, action.Name)
			}
		}
		return false, &QueryTestFailure{
			Name:           test.Name,
			Query:          test.Query,
			ExpectedTitle:  test.ExpectedTitle,
			ExpectedAction: test.ExpectedAction,
			HasTitleCheck:  test.TitleCheck != nil,
			ActualActions:  actualActions,
			Reason:         "title_not_found",
		}
	}

	return true, nil
}

// RunQueryTests executes multiple query tests
func (ts *TestSuite) RunQueryTests(tests []QueryTest) {
	var failedTests []QueryTestFailure

	for _, test := range tests {
		ts.t.Run(test.Name, func(t *testing.T) {
			subSuite := &TestSuite{t: t, ctx: ts.ctx}
			if ok, failure := subSuite.RunQueryTest(test); !ok {
				if failure == nil {
					failure = &QueryTestFailure{
						Name:           test.Name,
						Query:          test.Query,
						ExpectedTitle:  test.ExpectedTitle,
						ExpectedAction: test.ExpectedAction,
						HasTitleCheck:  test.TitleCheck != nil,
						Reason:         "unknown",
					}
				}
				failedTests = append(failedTests, *failure)
			}
		})
	}

	if len(failedTests) > 0 {
		ts.t.Errorf("\nFailed tests (%d):", len(failedTests))
		for i, failure := range failedTests {
			expectedTitle := failure.ExpectedTitle
			if failure.HasTitleCheck {
				expectedTitle = "custom TitleCheck"
			}
			actualActions := "none"
			if len(failure.ActualActions) > 0 {
				actualActions = strings.Join(failure.ActualActions, ", ")
			}
			ts.t.Errorf("%d. %s | query=%q | expectedTitle=%s | expectedAction=%s | actualActions=%s | reason=%s", i+1, failure.Name, failure.Query, expectedTitle, failure.ExpectedAction, actualActions, failure.Reason)
		}
	}
}

func (ts *TestSuite) RunQueryTestsWithMaxDuration(tests []QueryTest, maxDurationMs int64) {
	startTime := time.Now()
	ts.RunQueryTests(tests)
	elapsed := time.Since(startTime).Milliseconds()
	if elapsed > maxDurationMs {
		ts.t.Errorf("Total test duration %v exceeded maximum allowed %v", elapsed, maxDurationMs)
	} else {
		ts.t.Logf("Total test duration %v within maximum allowed %v", elapsed, maxDurationMs)
	}
}

// verifyAction checks if the expected action exists in the result
func (ts *TestSuite) verifyAction(result plugin.QueryResultUI, expectedAction, testName, query string) (bool, []string) {
	actualActions := make([]string, 0, len(result.Actions))
	for _, action := range result.Actions {
		actualActions = append(actualActions, action.Name)
		if action.Name == expectedAction {
			return true, actualActions
		}
	}
	ts.t.Errorf("[Test %s] Expected action %q not found for query %q", testName, expectedAction, query)
	ts.t.Errorf("[Test %s] Actual result actions:", testName)
	for _, action := range result.Actions {
		ts.t.Errorf("[Test %s] %s", testName, action.Name)
	}
	return false, actualActions
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

		// Wait for all system plugins to fully initialize
		t.Logf("Waiting for system plugins to initialize...")
		plugin.GetPluginManager().WaitForSystemPlugins()
		t.Logf("All system plugins initialized")

		// Initialize selection
		selection.InitSelection()

		// Check if plugins are loaded
		instances := plugin.GetPluginManager().GetPluginInstances()
		t.Logf("Loaded %d plugin instances", len(instances))
		for _, instance := range instances {
			t.Logf("Plugin: %s (triggers: %v)", instance.Metadata.GetName(context.Background()), instance.GetTriggerKeywords())
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
