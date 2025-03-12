package main

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
	"wox/i18n"
	"wox/plugin"
	"wox/resource"
	"wox/setting"
	"wox/share"
	"wox/ui"
	"wox/util"
	"wox/util/selection"
)

func TestCalculatorCrypto(t *testing.T) {
	tests := []queryTest{
		{
			name:           "BTC to USD",
			query:          "1BTC in USD",
			expectedTitle:  "$",
			expectedAction: "Copy result",
			titleCheck: func(title string) bool {
				return len(title) > 1 && strings.HasPrefix(title, "$") && title[1] >= '0' && title[1] <= '9'
			},
		},
		{
			name:           "BTC plus USD",
			query:          "1BTC + 1 USD",
			expectedTitle:  "$",
			expectedAction: "Copy result",
			titleCheck: func(title string) bool {
				return len(title) > 1 && strings.HasPrefix(title, "$") && title[1] >= '0' && title[1] <= '9'
			},
		},
		{
			name:           "ETH to USD",
			query:          "1 ETH to USD",
			expectedTitle:  "$",
			expectedAction: "Copy result",
			titleCheck: func(title string) bool {
				return len(title) > 1 && strings.HasPrefix(title, "$") && title[1] >= '0' && title[1] <= '9'
			},
		},
		{
			name:           "BTC + ETH",
			query:          "1BTC + 1ETH",
			expectedTitle:  "$",
			expectedAction: "Copy result",
			titleCheck: func(title string) bool {
				return len(title) > 1 && strings.HasPrefix(title, "$")
			},
		},
		{
			name:           "invalid crypto query",
			query:          "1btc dsfsdf",
			expectedTitle:  "Search for 1btc dsfsdf",
			expectedAction: "Search",
		},
		{
			name:           "BTC plus number",
			query:          "1btc + 1",
			expectedTitle:  "Search for 1btc + 1",
			expectedAction: "Search",
		},
	}
	runQueryTests(t, tests)
}

func TestCalculatorCurrency(t *testing.T) {
	tests := []queryTest{
		{
			name:           "USD to EUR",
			query:          "100 USD in EUR",
			expectedTitle:  "€",
			expectedAction: "Copy result",
			titleCheck: func(title string) bool {
				return len(title) > 1 && strings.HasPrefix(title, "€") && title[len("€")] >= '0' && title[len("€")] <= '9'
			},
		},
		{
			name:           "EUR to USD",
			query:          "50 EUR = ? USD",
			expectedTitle:  "$",
			expectedAction: "Copy result",
			titleCheck: func(title string) bool {
				return len(title) > 1 && strings.HasPrefix(title, "$") && title[1] >= '0' && title[1] <= '9'
			},
		},
		{
			name:           "USD to CNY",
			query:          "100 USD to CNY",
			expectedTitle:  "¥",
			expectedAction: "Copy result",
			titleCheck: func(title string) bool {
				return len(title) > 1 && strings.HasPrefix(title, "¥") && title[len("¥")] >= '0' && title[len("¥")] <= '9'
			},
		},
	}
	runQueryTests(t, tests)
}

func TestCalculatorBasic(t *testing.T) {
	tests := []queryTest{
		{
			name:           "Simple addition",
			query:          "1+2",
			expectedTitle:  "3",
			expectedAction: "Copy result",
		},
		{
			name:           "Complex expression",
			query:          "1+2*3",
			expectedTitle:  "7",
			expectedAction: "Copy result",
		},
		{
			name:           "Parentheses",
			query:          "(1+2)*3",
			expectedTitle:  "9",
			expectedAction: "Copy result",
		},
	}
	runQueryTests(t, tests)
}

func TestCalculatorTrigonometric(t *testing.T) {
	tests := []queryTest{
		{
			name:           "Sin with addition",
			query:          "sin(8) + 1",
			expectedTitle:  "1.9893582466233817",
			expectedAction: "Copy result",
		},
		{
			name:           "Sin with pi",
			query:          "sin(pi/4)",
			expectedTitle:  "0.7071067811865475",
			expectedAction: "Copy result",
		},
		{
			name:           "Complex expression with pi",
			query:          "2*pi + sin(pi/2)",
			expectedTitle:  "7.283185307179586",
			expectedAction: "Copy result",
		},
	}
	runQueryTests(t, tests)
}

func TestCalculatorAdvanced(t *testing.T) {
	tests := []queryTest{
		{
			name:           "Exponential",
			query:          "exp(2)",
			expectedTitle:  "7.38905609893065",
			expectedAction: "Copy result",
		},
		{
			name:           "Logarithm",
			query:          "log2(8)",
			expectedTitle:  "3",
			expectedAction: "Copy result",
		},
		{
			name:           "Power",
			query:          "pow(2,3)",
			expectedTitle:  "8",
			expectedAction: "Copy result",
		},
		{
			name:           "Square root",
			query:          "sqrt(16)",
			expectedTitle:  "4",
			expectedAction: "Copy result",
		},
		{
			name:           "Absolute value",
			query:          "abs(-42)",
			expectedTitle:  "42",
			expectedAction: "Copy result",
		},
		{
			name:           "Rounding",
			query:          "round(3.7)",
			expectedTitle:  "4",
			expectedAction: "Copy result",
		},
		{
			name:           "Nested functions",
			query:          "sqrt(pow(3,2) + pow(4,2))",
			expectedTitle:  "5",
			expectedAction: "Copy result",
		},
	}
	runQueryTests(t, tests)
}

func TestUrlPlugin(t *testing.T) {
	tests := []queryTest{
		{
			name:           "Domain only",
			query:          "google.com",
			expectedTitle:  "google.com",
			expectedAction: "Open",
		},
		{
			name:           "With https",
			query:          "https://www.google.com",
			expectedTitle:  "https://www.google.com",
			expectedAction: "Open",
		},
		{
			name:           "With path",
			query:          "github.com/Wox-launcher/Wox",
			expectedTitle:  "github.com/Wox-launcher/Wox",
			expectedAction: "Open",
		},
	}
	runQueryTests(t, tests)
}

func TestSystemPlugin(t *testing.T) {
	tests := []queryTest{
		{
			name:           "Lock command",
			query:          "lock",
			expectedTitle:  "Lock PC",
			expectedAction: "Execute",
		},
		{
			name:           "Settings command",
			query:          "settings",
			expectedTitle:  "Open Wox Settings",
			expectedAction: "Execute",
		},
	}
	runQueryTests(t, tests)
}

func TestWebSearchPlugin(t *testing.T) {
	tests := []queryTest{
		{
			name:           "Google search",
			query:          "g wox launcher",
			expectedTitle:  "Search for wox launcher",
			expectedAction: "Search",
		},
	}
	runQueryTests(t, tests)
}

func TestFilePlugin(t *testing.T) {
	tests := []queryTest{
		{
			name:           "Search by name",
			query:          "f main.go",
			expectedTitle:  "main.go",
			expectedAction: "Open",
		},
	}
	runQueryTests(t, tests)
}

func TestCalculatorTime(t *testing.T) {
	now := time.Now()

	// Get current time
	hour := now.Hour()
	ampm := "AM"
	if hour >= 12 {
		ampm = "PM"
		if hour > 12 {
			hour -= 12
		}
	}
	if hour == 0 {
		hour = 12
	}
	expectedTime := fmt.Sprintf("%d:%02d %s", hour, now.Minute(), ampm)
	// expectedTimePlusOneHour := fmt.Sprintf("%d:%02d %s", hour+1, now.Minute(), ampm)

	// Calculate expected date for "monday in 10 days"
	targetDate := now.AddDate(0, 0, 10)
	for targetDate.Weekday() != time.Monday {
		targetDate = targetDate.AddDate(0, 0, 1)
	}
	expectedMonday := fmt.Sprintf("%s (Monday)", targetDate.Format("2006-01-02"))

	// Calculate expected days until Christmas 2025
	christmas := time.Date(2025, time.December, 25, 0, 0, 0, 0, time.Local)
	daysUntilChristmas := int(christmas.Sub(now).Hours() / 24)
	expectedDaysUntil := fmt.Sprintf("%d days", daysUntilChristmas)

	tests := []queryTest{
		{
			name:           "Time in location",
			query:          "time in Shanghai",
			expectedTitle:  expectedTime,
			expectedAction: "Copy result",
		},
		{
			name:           "Weekday in future",
			query:          "monday in 10 days",
			expectedTitle:  expectedMonday,
			expectedAction: "Copy result",
		},
		{
			name:           "Days until Christmas",
			query:          "days until 25 Dec 2025",
			expectedTitle:  expectedDaysUntil,
			expectedAction: "Copy result",
		},
		{
			name:           "Specific time in location",
			query:          "3:30 pm in tokyo",
			expectedTitle:  "2:30 PM",
			expectedAction: "Copy result",
		},
		{
			name:           "Simple time unit",
			query:          "100ms",
			expectedTitle:  "100 milliseconds",
			expectedAction: "Copy result",
		},
	}
	runQueryTests(t, tests)
}

func initServices() {
	ctx := context.Background()

	// Initialize location
	err := util.GetLocation().Init()
	if err != nil {
		panic(err)
	}

	// Extract resources
	err = resource.Extract(ctx)
	if err != nil {
		panic(err)
	}

	// Initialize settings
	err = setting.GetSettingManager().Init(ctx)
	if err != nil {
		panic(err)
	}

	// Initialize i18n
	err = i18n.GetI18nManager().UpdateLang(ctx, i18n.LangCodeEnUs)
	if err != nil {
		panic(err)
	}

	// Initialize UI
	err = ui.GetUIManager().Start(ctx)
	if err != nil {
		panic(err)
	}

	// Initialize plugin system with UI
	plugin.GetPluginManager().Start(ctx, ui.GetUIManager().GetUI(ctx))

	// Wait for plugins to initialize
	time.Sleep(time.Second * 10)

	// Initialize selection
	selection.InitSelection()
}

type queryTest struct {
	name           string
	query          string
	expectedTitle  string
	expectedAction string
	titleCheck     func(string) bool
}

func runQueryTests(t *testing.T, tests []queryTest) {
	ctx := util.NewTraceContext()
	var failedTests []string

	initServices()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			success := true
			// Create query
			plainQuery := share.PlainQuery{
				QueryType: plugin.QueryTypeInput,
				QueryText: tt.query,
			}

			// Execute query
			query, queryPlugin, err := plugin.GetPluginManager().NewQuery(ctx, plainQuery)
			if err != nil {
				t.Errorf("Failed to create query: %v", err)
				failedTests = append(failedTests, tt.name)
				return
			}

			resultChan, doneChan := plugin.GetPluginManager().Query(ctx, query)

			// Collect all results
			var allResults []plugin.QueryResultUI

		CollectResults:
			for {
				select {
				case results := <-resultChan:
					allResults = append(allResults, results...)
				case <-doneChan:
					break CollectResults
				case <-time.After(time.Second * 30):
					t.Errorf("Query timeout")
					failedTests = append(failedTests, tt.name)
					return
				}
			}

			// Try fallback results if no results found
			if len(allResults) == 0 {
				allResults = plugin.GetPluginManager().QueryFallback(ctx, query, queryPlugin)
			}

			// Verify results
			if len(allResults) == 0 {
				t.Errorf("No results returned for query: %s", tt.query)
				failedTests = append(failedTests, tt.name)
				return
			}

			// Find matching result
			found := false
			for _, result := range allResults {
				if tt.titleCheck != nil {
					if tt.titleCheck(result.Title) {
						found = true
						// Verify action
						actionFound := false
						for _, action := range result.Actions {
							if action.Name == tt.expectedAction {
								actionFound = true
								break
							}
						}
						if !actionFound {
							t.Errorf("Expected action %q not found in result actions for title %q", tt.expectedAction, result.Title)
							t.Errorf("Actual result actions:")
							for _, action := range result.Actions {
								t.Errorf("  %s", action.Name)
							}
							success = false
						}
						break
					}
				} else if result.Title == tt.expectedTitle {
					found = true
					// Verify action
					actionFound := false
					for _, action := range result.Actions {
						if action.Name == tt.expectedAction {
							actionFound = true
							break
						}
					}
					if !actionFound {
						t.Errorf("Expected action %q not found in result actions for title %q", tt.expectedAction, result.Title)
						success = false
					}
					break
				}
			}

			if !found {
				t.Errorf("Expected title (%s) format not found in results. Got results for query %q:", tt.expectedTitle, tt.query)
				for i, result := range allResults {
					t.Errorf("Result %d:", i+1)
					t.Errorf("  Title: %s", result.Title)
					t.Errorf("  SubTitle: %s", result.SubTitle)
					t.Errorf("  Actions:")
					for j, action := range result.Actions {
						t.Errorf("    %d. %s", j+1, action.Name)
					}
				}
				success = false
			}

			if !success {
				failedTests = append(failedTests, tt.name)
			}
		})
	}

	if len(failedTests) > 0 {
		t.Errorf("\nFailed tests (%d):", len(failedTests))
		for i, name := range failedTests {
			t.Errorf("%d. %s", i+1, name)
		}
	}
}
