package main

import (
	"context"
	"testing"
	"time"
	"wox/i18n"
	"wox/plugin"
	"wox/resource"
	"wox/setting"
	"wox/share"
	"wox/ui"
	"wox/util"
)

func init() {
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
	woxSetting := setting.GetSettingManager().GetWoxSetting(ctx)
	err = i18n.GetI18nManager().UpdateLang(ctx, woxSetting.LangCode)
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

	// Initialize selection
	util.InitSelection()
}

func TestQuery(t *testing.T) {
	ctx := util.NewTraceContext()

	tests := []struct {
		name           string
		query          string
		expectedTitle  string
		expectedAction string
	}{
		// Calculator plugin tests
		{
			name:           "Calculator plugin - simple addition",
			query:          "1+2",
			expectedTitle:  "1+2 = 3",
			expectedAction: "Copy result",
		},
		{
			name:           "Calculator plugin - complex expression",
			query:          "1+2*3",
			expectedTitle:  "1+2*3 = 7",
			expectedAction: "Copy result",
		},
		{
			name:           "Calculator plugin - parentheses",
			query:          "(1+2)*3",
			expectedTitle:  "(1+2)*3 = 9",
			expectedAction: "Copy result",
		},

		// URL plugin tests
		{
			name:           "URL plugin - domain only",
			query:          "google.com",
			expectedTitle:  "google.com",
			expectedAction: "Open",
		},
		{
			name:           "URL plugin - with https",
			query:          "https://www.google.com",
			expectedTitle:  "https://www.google.com",
			expectedAction: "Open",
		},
		{
			name:           "URL plugin - with path",
			query:          "github.com/Wox-launcher/Wox",
			expectedTitle:  "github.com/Wox-launcher/Wox",
			expectedAction: "Open",
		},

		// System plugin tests
		{
			name:           "System plugin - lock",
			query:          "lock",
			expectedTitle:  "Lock PC",
			expectedAction: "Execute",
		},
		{
			name:           "System plugin - settings",
			query:          "settings",
			expectedTitle:  "Open Wox Settings",
			expectedAction: "Execute",
		},

		// Web search plugin tests
		{
			name:           "Web search plugin - google",
			query:          "g wox launcher",
			expectedTitle:  "Search for wox launcher",
			expectedAction: "Search",
		},

		// File plugin tests
		{
			name:           "File plugin - search by name",
			query:          "f main.go",
			expectedTitle:  "main.go",
			expectedAction: "Open",
		},

		// Clipboard plugin tests
		{
			name:           "Clipboard plugin - show history",
			query:          "cb",
			expectedTitle:  "cb",
			expectedAction: "Activate",
		},
	}

	var failedTests []string
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
			timeout := time.After(time.Second * 5)

		CollectResults:
			for {
				select {
				case results := <-resultChan:
					allResults = append(allResults, results...)
				case <-doneChan:
					break CollectResults
				case <-timeout:
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
				if result.Title == tt.expectedTitle {
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
						t.Errorf("Expected action %q not found in result actions:", tt.expectedAction)
						t.Errorf("Got results for query %q:", tt.query)
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
					break
				}
			}

			if !found {
				t.Errorf("Expected title %q not found in results:", tt.expectedTitle)
				t.Errorf("Got results for query %q:", tt.query)
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

	// 在所有测试完成后，显示失败的测试列表
	if len(failedTests) > 0 {
		t.Errorf("\nFailed tests (%d):", len(failedTests))
		for i, name := range failedTests {
			t.Errorf("%d. %s", i+1, name)
		}
	}
}
