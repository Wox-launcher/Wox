package test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"wox/setting"
)

// ConfigBlackboxTestSuite tests configuration loading with various corrupted JSON files
type ConfigBlackboxTestSuite struct {
	*TestSuite
	testDir string
}

// NewConfigBlackboxTestSuite creates a new configuration blackbox test suite
func NewConfigBlackboxTestSuite(t *testing.T) *ConfigBlackboxTestSuite {
	suite := &ConfigBlackboxTestSuite{
		TestSuite: NewTestSuite(t),
	}

	// Create isolated test directory for config tests
	testConfig := GetCurrentTestConfig()
	suite.testDir = filepath.Join(testConfig.TestDataDirectory, "config_blackbox_test")
	err := os.MkdirAll(suite.testDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	return suite
}

// TestConfigBlackbox runs all configuration blackbox tests
func TestConfigBlackbox(t *testing.T) {
	suite := NewConfigBlackboxTestSuite(t)
	defer suite.cleanup()

	// Test loadWoxSetting with various corrupted JSON files
	t.Run("LoadWoxSetting", func(t *testing.T) {
		suite.testLoadWoxSettingBlackbox(t)
	})

	// TODO: Add loadWoxAppData tests later
	// t.Run("LoadWoxAppData", func(t *testing.T) {
	//	suite.testLoadWoxAppDataBlackbox(t)
	// })
}

// testLoadWoxSettingBlackbox tests loadWoxSetting method with various corrupted JSON scenarios
func (suite *ConfigBlackboxTestSuite) testLoadWoxSettingBlackbox(t *testing.T) {
	testCases := []struct {
		name           string
		jsonContent    string
		expectError    bool
		expectDefaults bool
		description    string
	}{
		{
			name:           "EmptyFile",
			jsonContent:    "",
			expectError:    false,
			expectDefaults: true,
			description:    "Empty configuration file should load with defaults",
		},
		{
			name:           "InvalidJSON_MissingBrace",
			jsonContent:    `{"MainHotkey": {"WinValue": "alt+space"`,
			expectError:    false,
			expectDefaults: true,
			description:    "Missing closing brace should be repaired and load with defaults",
		},
		{
			name:           "InvalidJSON_ExtraComma",
			jsonContent:    `{"MainHotkey": {"WinValue": "alt+space",}, "UsePinYin": true,}`,
			expectError:    false,
			expectDefaults: true,
			description:    "Extra trailing commas should be repaired and load successfully",
		},
		{
			name:           "ValidJSON_MissingFields",
			jsonContent:    `{"UsePinYin": true}`,
			expectError:    false,
			expectDefaults: true,
			description:    "Valid JSON with missing fields should load with defaults",
		},
		{
			name:           "ValidJSON_NullValues",
			jsonContent:    `{"MainHotkey": null, "UsePinYin": null, "QueryShortcuts": null}`,
			expectError:    false,
			expectDefaults: true,
			description:    "Null values should load with defaults",
		},
		{
			name:           "ValidJSON_ExtraFields",
			jsonContent:    `{"UsePinYin": true, "UnknownField": "value", "AnotherUnknown": 123}`,
			expectError:    false,
			expectDefaults: true,
			description:    "Extra unknown fields should be ignored",
		},
		{
			name:           "ValidJSON_PartialData",
			jsonContent:    `{"UsePinYin": true, "ShowTray": false, "AppWidth": 1200}`,
			expectError:    false,
			expectDefaults: true,
			description:    "Partial data should load with defaults for missing fields",
		},
		{
			name:           "InvalidJSON_WrongDataTypes",
			jsonContent:    `{"UsePinYin": "not_a_boolean", "AppWidth": "not_a_number"}`,
			expectError:    false,
			expectDefaults: true,
			description:    "Wrong data types should be ignored and load with defaults",
		},
		{
			name:           "InvalidJSON_CorruptedNestedObject",
			jsonContent:    `{"MainHotkey": {"WinValue": "alt+space", "MacValue": }, "UsePinYin": true}`,
			expectError:    false,
			expectDefaults: true,
			description:    "Corrupted nested object should be repaired and load with defaults",
		},
		{
			name:           "InvalidJSON_CorruptedArray",
			jsonContent:    `{"QueryShortcuts": [{"Shortcut": "test", "Query": "test"}, {"Shortcut": "test2",}], "UsePinYin": true}`,
			expectError:    false,
			expectDefaults: true,
			description:    "Corrupted array element should be repaired and load with defaults",
		},
		{
			name:           "ValidJSON_EmptyArrays",
			jsonContent:    `{"QueryShortcuts": [], "AIProviders": [], "UsePinYin": true}`,
			expectError:    false,
			expectDefaults: true,
			description:    "Empty arrays should load successfully",
		},
		{
			name:           "InvalidJSON_VeryLargeNumbers",
			jsonContent:    `{"AppWidth": 999999999999999999999999999999, "MaxResultCount": -999999999999999999999999999999}`,
			expectError:    false,
			expectDefaults: true,
			description:    "Numbers too large for int type should be sanitized to defaults",
		},
		{
			name:           "ValidJSON_ComplexNestedStructures",
			jsonContent:    `{"MainHotkey": {"WinValue": "ctrl+space", "MacValue": "cmd+space"}, "QueryShortcuts": [{"Shortcut": "g", "Query": "google {0}"}], "AIProviders": [{"Name": "openai", "ApiKey": "test", "Host": "api.openai.com"}]}`,
			expectError:    false,
			expectDefaults: true,
			description:    "Complex nested structures should load successfully",
		},
		{
			name:           "InvalidJSON_CorruptedPlatformSettings",
			jsonContent:    `{"MainHotkey": {"WinValue": "ctrl+space", "MacValue": }, "EnableAutostart": {"WinValue": "not_a_bool"}}`,
			expectError:    false,
			expectDefaults: true,
			description:    "Corrupted platform-specific settings should be repaired",
		},
		{
			name:           "ValidJSON_InvalidLangCode",
			jsonContent:    `{"LangCode": "invalid_lang", "UsePinYin": true}`,
			expectError:    false,
			expectDefaults: true,
			description:    "Invalid language code should be sanitized to default",
		},
		{
			name:           "ValidJSON_InvalidShowPosition",
			jsonContent:    `{"ShowPosition": "invalid_position", "AppWidth": 1000}`,
			expectError:    false,
			expectDefaults: true,
			description:    "Invalid show position should be sanitized to default",
		},
		{
			name:           "ValidJSON_InvalidLastQueryMode",
			jsonContent:    `{"LastQueryMode": "invalid_mode", "MaxResultCount": 15}`,
			expectError:    false,
			expectDefaults: true,
			description:    "Invalid last query mode should be sanitized to default",
		},
		{
			name:           "InvalidJSON_MalformedArrayElements",
			jsonContent:    `{"QueryShortcuts": [{"Shortcut": "g"}, {"Query": "test"}], "AIProviders": [{"Name": "openai"}, {"ApiKey": "test"}]}`,
			expectError:    false,
			expectDefaults: true,
			description:    "Malformed array elements should be handled gracefully",
		},
		{
			name:           "ValidJSON_BoundaryValues",
			jsonContent:    `{"AppWidth": 1, "MaxResultCount": 999, "UsePinYin": false}`,
			expectError:    false,
			expectDefaults: true,
			description:    "Boundary values should be accepted",
		},
		{
			name:           "ValidJSON_NegativeNumbers",
			jsonContent:    `{"AppWidth": -100, "MaxResultCount": -5}`,
			expectError:    false,
			expectDefaults: true,
			description:    "Negative numbers should be sanitized to defaults",
		},
		{
			name:           "InvalidJSON_UnterminatedString",
			jsonContent:    `{"ThemeId": "some-theme-id, "UsePinYin": true}`,
			expectError:    false,
			expectDefaults: true,
			description:    "Unterminated string should be repaired",
		},
		{
			name:           "InvalidJSON_MissingQuotes",
			jsonContent:    `{ThemeId: some-theme-id, UsePinYin: true}`,
			expectError:    false,
			expectDefaults: true,
			description:    "Missing quotes should be handled gracefully",
		},
		{
			name:           "ValidJSON_UnicodeContent",
			jsonContent:    `{"LangCode": "zh_CN", "QueryShortcuts": [{"Shortcut": "测试", "Query": "test 中文"}]}`,
			expectError:    false,
			expectDefaults: true,
			description:    "Unicode content should be handled correctly",
		},
		{
			name:           "InvalidJSON_OnlyBraces",
			jsonContent:    `{}`,
			expectError:    false,
			expectDefaults: true,
			description:    "Empty JSON object should load with all defaults",
		},
		{
			name:           "InvalidJSON_OnlyWhitespace",
			jsonContent:    "   \n\t  \r\n  ",
			expectError:    false,
			expectDefaults: true,
			description:    "Whitespace-only content should be treated as empty",
		},
		{
			name:           "InvalidJSON_BinaryData",
			jsonContent:    string([]byte{0x00, 0x01, 0x02, 0xFF, 0xFE}),
			expectError:    false,
			expectDefaults: true,
			description:    "Binary data should be handled gracefully",
		},
		{
			name:           "ValidJSON_ExtremelyLongString",
			jsonContent:    `{"ThemeId": "` + strings.Repeat("a", 10000) + `", "UsePinYin": true}`,
			expectError:    false,
			expectDefaults: true,
			description:    "Extremely long strings should be handled",
		},
		{
			name:           "InvalidJSON_NestedBracesOverflow",
			jsonContent:    `{"MainHotkey": ` + strings.Repeat("{", 100) + `"test"` + strings.Repeat("}", 100) + `}`,
			expectError:    false,
			expectDefaults: true,
			description:    "Deeply nested structures should be handled",
		},
		{
			name:           "ValidJSON_ZeroValues",
			jsonContent:    `{"AppWidth": 0, "MaxResultCount": 0, "UsePinYin": false, "ShowTray": false}`,
			expectError:    false,
			expectDefaults: true,
			description:    "Zero values should be replaced with defaults",
		},
		{
			name:           "InvalidJSON_SpecialCharacters",
			jsonContent:    `{"ThemeId": "theme\n\t\r\"\\\/\b\f", "UsePinYin": true}`,
			expectError:    false,
			expectDefaults: true,
			description:    "Special characters should be handled correctly",
		},
		{
			name:           "ValidJSON_FloatAsInt",
			jsonContent:    `{"AppWidth": 800.5, "MaxResultCount": 10.9}`,
			expectError:    false,
			expectDefaults: true,
			description:    "Float values for int fields should be converted",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			suite.runLoadWoxSettingTest(t, tc.name, tc.jsonContent, tc.expectError, tc.expectDefaults, tc.description)
		})
	}
}

// runLoadWoxSettingTest runs a single loadWoxSetting blackbox test
func (suite *ConfigBlackboxTestSuite) runLoadWoxSettingTest(t *testing.T, name, jsonContent string, expectError, expectDefaults bool, description string) {
	// Create a temporary setting file in the test location
	testLocation := GetTestLocation()
	if testLocation == nil {
		t.Fatalf("Test location not available")
	}

	settingPath := testLocation.GetWoxSettingPath()

	// Ensure the settings directory exists
	settingsDir := testLocation.GetPluginSettingDirectory()
	err := os.MkdirAll(settingsDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create settings directory: %v", err)
	}

	// Write the test JSON content
	err = os.WriteFile(settingPath, []byte(jsonContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test setting file: %v", err)
	}
	defer os.Remove(settingPath)

	t.Logf("Testing: %s - %s", name, description)
	t.Logf("JSON content: %s", jsonContent)
	t.Logf("Setting path: %s", settingPath)

	// Create a new manager and try to initialize it (which calls loadWoxSetting internally)
	manager := &setting.Manager{}
	ctx := context.Background()

	err = manager.Init(ctx)

	if expectError {
		if err == nil {
			t.Errorf("Expected error but got none for test case: %s", name)
		} else {
			t.Logf("Got expected error: %v", err)
		}
	} else {
		if err != nil {
			t.Errorf("Unexpected error for test case %s: %v", name, err)
		} else {
			t.Logf("Successfully loaded setting")

			// Verify the loaded setting
			loadedSetting := manager.GetWoxSetting(ctx)
			suite.validateLoadedWoxSetting(t, loadedSetting, expectDefaults, name)
		}
	}
}

// validateLoadedWoxSetting validates that a loaded WoxSetting has reasonable values
func (suite *ConfigBlackboxTestSuite) validateLoadedWoxSetting(t *testing.T, woxSetting *setting.WoxSetting, expectDefaults bool, testName string) {
	if woxSetting == nil {
		t.Errorf("Loaded setting is nil for test %s", testName)
		return
	}

	// Check critical fields have reasonable values
	if expectDefaults {
		// When expecting defaults, missing fields should be filled with default values
		if woxSetting.AppWidth.Get() == 0 {
			t.Errorf("AppWidth should have default value, got 0 in test %s", testName)
		}
		if woxSetting.MaxResultCount.Get() == 0 {
			t.Errorf("MaxResultCount should have default value, got 0 in test %s", testName)
		}
		if woxSetting.ThemeId.Get() == "" {
			t.Errorf("ThemeId should have default value, got empty string in test %s", testName)
		}
		if woxSetting.LangCode.Get() == "" {
			t.Errorf("LangCode should have default value, got empty string in test %s", testName)
		}
		if woxSetting.MainHotkey.Get() == "" {
			t.Errorf("MainHotkey should have default value, got empty string in test %s", testName)
		}
	}

	// Log the loaded values for debugging
	t.Logf("Loaded setting values for %s:", testName)
	t.Logf("  AppWidth: %d", woxSetting.AppWidth.Get())
	t.Logf("  MaxResultCount: %d", woxSetting.MaxResultCount.Get())
	t.Logf("  UsePinYin: %t", woxSetting.UsePinYin.Get())
	t.Logf("  ShowTray: %t", woxSetting.ShowTray.Get())
	t.Logf("  LangCode: %s", woxSetting.LangCode.Get())
	t.Logf("  ThemeId: %s", woxSetting.ThemeId.Get())
	t.Logf("  MainHotkey: %s", woxSetting.MainHotkey.Get())
}

// cleanup cleans up test resources
func (suite *ConfigBlackboxTestSuite) cleanup() {
	if suite.testDir != "" {
		os.RemoveAll(suite.testDir)
	}
}
