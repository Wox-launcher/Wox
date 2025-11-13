package system

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"wox/common"
	"wox/plugin"
	"wox/setting/definition"

	"github.com/stretchr/testify/assert"
)

func TestBrowserBookmarkPlugin_loadBookmarkFromFile(t *testing.T) {
	// Create a temporary bookmark file for testing
	tempDir := t.TempDir()
	bookmarkFile := filepath.Join(tempDir, "Bookmarks")

	// Sample bookmark JSON content (more realistic format)
	bookmarkContent := `{
		"roots": {
			"bookmark_bar": {
				"children": [
					{
						"date_added": "13285874237000000",
						"date_last_used": "0",
						"guid": "00000000-0000-4000-A000-000000000001",
						"id": "5",
						"name": "Google",
						"type": "url",
						"url": "https://www.google.com"
					},
					{
						"date_added": "13285874237000000",
						"date_last_used": "0",
						"guid": "00000000-0000-4000-A000-000000000002",
						"id": "6",
						"name": "GitHub",
						"type": "url",
						"url": "https://github.com"
					}
				]
			}
		}
	}`

	err := os.WriteFile(bookmarkFile, []byte(bookmarkContent), 0644)
	assert.NoError(t, err)

	// Create plugin instance
	plugin := &BrowserBookmarkPlugin{}
	plugin.api = &mockAPI{}

	// Test loading bookmarks
	ctx := context.Background()

	bookmarks := plugin.loadBookmarkFromFile(ctx, bookmarkFile, "TestBrowser")

	// Verify results
	assert.Len(t, bookmarks, 2)
	if len(bookmarks) >= 2 {
		assert.Equal(t, "Google", bookmarks[0].Name)
		assert.Equal(t, "https://www.google.com", bookmarks[0].Url)
		assert.Equal(t, "GitHub", bookmarks[1].Name)
		assert.Equal(t, "https://github.com", bookmarks[1].Url)
	}
}

func TestBrowserBookmarkPlugin_ChromeBookmarkPaths(t *testing.T) {
	plugin := &BrowserBookmarkPlugin{}
	plugin.api = &mockAPI{}
	ctx := context.Background()

	// Test Windows Chrome path - only if Chrome is installed
	if os.Getenv("LOCALAPPDATA") != "" {
		chromeDir := filepath.Join(os.Getenv("LOCALAPPDATA"), "Google", "Chrome", "User Data")
		if _, err := os.Stat(chromeDir); err == nil {
			t.Log("Chrome detected on Windows, testing Chrome bookmark loading")
			bookmarks := plugin.loadChromeBookmarkInWindows(ctx, "Default")
			// Should return a slice (even if empty) and not panic
			assert.NotNil(t, bookmarks)
			assert.IsType(t, []Bookmark{}, bookmarks)
			t.Logf("Chrome Windows test: found %d bookmarks", len(bookmarks))
		} else {
			t.Log("Chrome not found on Windows, skipping Chrome bookmark test")
		}
	}

	// Test macOS Chrome path - only if Chrome is installed
	homeDir, _ := os.UserHomeDir()
	if homeDir != "" {
		chromeDir := filepath.Join(homeDir, "Library", "Application Support", "Google", "Chrome")
		if _, err := os.Stat(chromeDir); err == nil {
			t.Log("Chrome detected on macOS, testing Chrome bookmark loading")
			bookmarks := plugin.loadChromeBookmarkInMacos(ctx, "Default")
			assert.NotNil(t, bookmarks)
		} else {
			t.Log("Chrome not found on macOS, skipping Chrome bookmark test")
		}
	}

	// Test Linux Chrome path - only if Chrome is installed
	if homeDir, _ := os.UserHomeDir(); homeDir != "" {
		chromeDir := filepath.Join(homeDir, ".config", "google-chrome")
		if _, err := os.Stat(chromeDir); err == nil {
			t.Log("Chrome detected on Linux, testing Chrome bookmark loading")
			bookmarks := plugin.loadChromeBookmarkInLinux(ctx, "Default")
			assert.NotNil(t, bookmarks)
		} else {
			t.Log("Chrome not found on Linux, skipping Chrome bookmark test")
		}
	}
}

func TestBrowserBookmarkPlugin_EdgeBookmarkPaths(t *testing.T) {
	plugin := &BrowserBookmarkPlugin{}
	plugin.api = &mockAPI{}
	ctx := context.Background()

	// Test Windows Edge path - only if Edge is installed
	if os.Getenv("LOCALAPPDATA") != "" {
		edgeDir := filepath.Join(os.Getenv("LOCALAPPDATA"), "Microsoft", "Edge", "User Data")
		if _, err := os.Stat(edgeDir); err == nil {
			t.Log("Edge detected on Windows, testing Edge bookmark loading")

			// Check if Default profile exists
			defaultProfilePath := filepath.Join(edgeDir, "Default")
			if _, err := os.Stat(defaultProfilePath); err == nil {
				t.Logf("Default profile found at: %s", defaultProfilePath)

				// Check if Bookmarks file exists
				bookmarkFile := filepath.Join(defaultProfilePath, "Bookmarks")
				if _, err := os.Stat(bookmarkFile); err == nil {
					t.Logf("Bookmarks file found at: %s", bookmarkFile)

					// Read and log first 500 characters of bookmark file for debugging
					content, err := os.ReadFile(bookmarkFile)
					if err == nil {
						contentStr := string(content)
						if len(contentStr) > 500 {
							contentStr = contentStr[:500] + "..."
						}
						t.Logf("Bookmark file content preview: %s", contentStr)
					} else {
						t.Logf("Error reading bookmark file: %v", err)
					}
				} else {
					t.Logf("Bookmarks file not found at: %s", bookmarkFile)
				}
			} else {
				t.Logf("Default profile not found at: %s", defaultProfilePath)

				// List available profiles
				entries, err := os.ReadDir(edgeDir)
				if err == nil {
					t.Log("Available profiles:")
					for _, entry := range entries {
						if entry.IsDir() {
							t.Logf("  - %s", entry.Name())
						}
					}
				}
			}

			bookmarks := plugin.loadEdgeBookmarkInWindows(ctx, "Default")
			// Should return a slice (even if empty) and not panic
			assert.NotNil(t, bookmarks)
			t.Logf("Edge Windows test: found %d bookmarks", len(bookmarks))
		} else {
			t.Log("Edge not found on Windows, skipping Edge bookmark test")
		}
	}

	// Test macOS Edge path - only if Edge is installed
	homeDir, _ := os.UserHomeDir()
	if homeDir != "" {
		edgeDir := filepath.Join(homeDir, "Library", "Application Support", "Microsoft Edge")
		if _, err := os.Stat(edgeDir); err == nil {
			t.Log("Edge detected on macOS, testing Edge bookmark loading")
			bookmarks := plugin.loadEdgeBookmarkInMacos(ctx, "Default")
			assert.NotNil(t, bookmarks)
		} else {
			t.Log("Edge not found on macOS, skipping Edge bookmark test")
		}
	}

	// Test Linux Edge path - only if Edge is installed
	if homeDir, _ := os.UserHomeDir(); homeDir != "" {
		edgeDir := filepath.Join(homeDir, ".config", "microsoft-edge")
		if _, err := os.Stat(edgeDir); err == nil {
			t.Log("Edge detected on Linux, testing Edge bookmark loading")
			bookmarks := plugin.loadEdgeBookmarkInLinux(ctx, "Default")
			assert.NotNil(t, bookmarks)
		} else {
			t.Log("Edge not found on Linux, skipping Edge bookmark test")
		}
	}
}

func TestBrowserBookmarkPlugin_RemoveDuplicateBookmarks(t *testing.T) {
	plugin := &BrowserBookmarkPlugin{}
	plugin.api = &mockAPI{}

	// Create test bookmarks with duplicates
	bookmarks := []Bookmark{
		{Name: "Google", Url: "https://www.google.com"},
		{Name: "GitHub", Url: "https://github.com"},
		{Name: "Google", Url: "https://www.google.com"}, // Duplicate
		{Name: "Stack Overflow", Url: "https://stackoverflow.com"},
		{Name: "GitHub", Url: "https://github.com"},            // Duplicate
		{Name: "Google", Url: "https://www.google.cn"},         // Different URL, should keep
		{Name: "Google Search", Url: "https://www.google.com"}, // Different name, should keep
	}

	// Remove duplicates
	result := plugin.removeDuplicateBookmarks(bookmarks)

	// Verify results
	assert.Len(t, result, 5) // Should have 5 unique bookmarks

	// Verify each bookmark exists and is unique
	seen := make(map[string]bool)
	for _, bookmark := range result {
		key := bookmark.Name + "|" + bookmark.Url
		assert.False(t, seen[key], "Duplicate bookmark found: %s", key)
		seen[key] = true
	}

	// Verify that we have the expected unique combinations
	expectedKeys := []string{
		"Google|https://www.google.com",
		"GitHub|https://github.com",
		"Stack Overflow|https://stackoverflow.com",
		"Google|https://www.google.cn",
		"Google Search|https://www.google.com",
	}

	for _, expectedKey := range expectedKeys {
		assert.True(t, seen[expectedKey], "Expected bookmark not found: %s", expectedKey)
	}
}

// Mock API for testing
type mockAPI struct{}

func (m *mockAPI) Log(ctx context.Context, level plugin.LogLevel, msg string) {
	// Output log messages during testing
	if testing.Verbose() {
		println("LOG:", msg)
	}
}
func (m *mockAPI) Notify(ctx context.Context, msg string)                {}
func (m *mockAPI) GetTranslation(ctx context.Context, key string) string { return key }
func (m *mockAPI) GetSetting(ctx context.Context, key string) string     { return "" }
func (m *mockAPI) SaveSetting(ctx context.Context, key string, value string, isGlobal bool) {
}
func (m *mockAPI) GetAllSettings(ctx context.Context) map[string]string                          { return nil }
func (m *mockAPI) OpenSettingDialog(ctx context.Context)                                         {}
func (m *mockAPI) HideApp(ctx context.Context)                                                   {}
func (m *mockAPI) ShowApp(ctx context.Context)                                                   {}
func (m *mockAPI) ChangeQuery(ctx context.Context, query common.PlainQuery)                      {}
func (m *mockAPI) RestartApp(ctx context.Context)                                                {}
func (m *mockAPI) ReloadPlugin(ctx context.Context, pluginId string)                             {}
func (m *mockAPI) RemovePlugin(ctx context.Context, pluginId string)                             {}
func (m *mockAPI) OnSettingChanged(ctx context.Context, callback func(key string, value string)) {}
func (m *mockAPI) OnGetDynamicSetting(context.Context, func(string) definition.PluginSettingDefinitionItem) {
}
func (m *mockAPI) OnDeepLink(ctx context.Context, callback func(arguments map[string]string))   {}
func (m *mockAPI) OnUnload(ctx context.Context, callback func())                                {}
func (m *mockAPI) RegisterQueryCommands(ctx context.Context, commands []plugin.MetadataCommand) {}
func (m *mockAPI) AIChatStream(ctx context.Context, model common.Model, conversations []common.Conversation, options common.ChatOptions, callback common.ChatStreamFunc) error {
	return nil
}
func (m *mockAPI) OnMRURestore(ctx context.Context, callback func(mruData plugin.MRUData) (*plugin.QueryResult, error)) {
}
func (m *mockAPI) UpdateResult(ctx context.Context, result plugin.UpdatableResult) bool {
	return false
}

func (m *mockAPI) GetUpdatableResult(ctx context.Context, resultId string) *plugin.UpdatableResult {
	return nil
}

func (m *mockAPI) IsVisible(ctx context.Context) bool {
	return false
}
