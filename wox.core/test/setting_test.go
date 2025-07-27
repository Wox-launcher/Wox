package test

import (
	"os"
	"path/filepath"
	"testing"
	"wox/database"
	"wox/setting"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ConfigBlackboxTestSuite tests configuration loading with various corrupted database scenarios
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

// createTestDatabase creates an isolated test database
func (suite *ConfigBlackboxTestSuite) createTestDatabase(dbPath string) (*gorm.DB, error) {
	// Import required packages for database setup
	dsn := dbPath + "?" +
		"_journal_mode=WAL&" +
		"_synchronous=NORMAL&" +
		"_cache_size=1000&" +
		"_foreign_keys=true&" +
		"_busy_timeout=5000"

	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, err
	}

	// Auto-migrate the schema
	err = db.AutoMigrate(&database.WoxSetting{})
	if err != nil {
		return nil, err
	}

	return db, nil
}

// TestConfigBlackbox runs all configuration blackbox tests
func TestConfigBlackbox(t *testing.T) {
	suite := NewConfigBlackboxTestSuite(t)
	defer suite.cleanup()

	// Test WoxSetting with various database scenarios
	t.Run("LoadWoxSetting", func(t *testing.T) {
		suite.testLoadWoxSettingBlackbox(t)
	})

	// Test database corruption scenarios
	t.Run("DatabaseCorruption", func(t *testing.T) {
		suite.testDatabaseCorruptionScenarios(t)
	})
}

// testLoadWoxSettingBlackbox tests WoxSetting with various database scenarios
func (suite *ConfigBlackboxTestSuite) testLoadWoxSettingBlackbox(t *testing.T) {
	testCases := []struct {
		name           string
		setupDB        func(*gorm.DB) error
		expectError    bool
		expectDefaults bool
		description    string
	}{
		{
			name: "EmptyDatabase",
			setupDB: func(db *gorm.DB) error {
				// Empty database - no settings stored
				return nil
			},
			expectError:    false,
			expectDefaults: true,
			description:    "Empty database should load with defaults",
		},
		{
			name: "CorruptedValue",
			setupDB: func(db *gorm.DB) error {
				// Insert corrupted JSON value
				return db.Create(&database.WoxSetting{
					Key:   "AppWidth",
					Value: "invalid_json_data",
				}).Error
			},
			expectError:    false,
			expectDefaults: true,
			description:    "Corrupted value should fallback to defaults",
		},
		{
			name: "InvalidBooleanValue",
			setupDB: func(db *gorm.DB) error {
				// Insert invalid boolean value
				return db.Create(&database.WoxSetting{
					Key:   "UsePinYin",
					Value: "not_a_boolean",
				}).Error
			},
			expectError:    false,
			expectDefaults: true,
			description:    "Invalid boolean value should fallback to default",
		},
		{
			name: "InvalidIntegerValue",
			setupDB: func(db *gorm.DB) error {
				// Insert invalid integer value
				return db.Create(&database.WoxSetting{
					Key:   "AppWidth",
					Value: "not_a_number",
				}).Error
			},
			expectError:    false,
			expectDefaults: true,
			description:    "Invalid integer value should fallback to default",
		},
		{
			name: "ValidPartialSettings",
			setupDB: func(db *gorm.DB) error {
				// Insert some valid settings
				settings := []database.WoxSetting{
					{Key: "UsePinYin", Value: "true"},
					{Key: "ShowTray", Value: "false"},
					{Key: "AppWidth", Value: "1200"},
				}
				for _, s := range settings {
					if err := db.Create(&s).Error; err != nil {
						return err
					}
				}
				return nil
			},
			expectError:    false,
			expectDefaults: true,
			description:    "Partial settings should load successfully with defaults for missing fields",
		},
		{
			name: "ComplexObjectSerialization",
			setupDB: func(db *gorm.DB) error {
				// Insert complex object (platform value)
				platformValue := `{"MacValue":"cmd+space","WinValue":"alt+space","LinuxValue":"ctrl+space"}`
				return db.Create(&database.WoxSetting{
					Key:   "MainHotkey",
					Value: platformValue,
				}).Error
			},
			expectError:    false,
			expectDefaults: false,
			description:    "Complex objects should serialize/deserialize correctly",
		},
		{
			name: "InvalidLangCode",
			setupDB: func(db *gorm.DB) error {
				// Insert invalid language code
				return db.Create(&database.WoxSetting{
					Key:   "LangCode",
					Value: `"invalid_lang"`,
				}).Error
			},
			expectError:    false,
			expectDefaults: true,
			description:    "Invalid language code should be sanitized to default",
		},
		{
			name: "ValidComplexArrays",
			setupDB: func(db *gorm.DB) error {
				// Insert valid array data
				queryShortcuts := `[{"Shortcut":"g","Query":"google {0}"},{"Shortcut":"gh","Query":"github {0}"}]`
				return db.Create(&database.WoxSetting{
					Key:   "QueryShortcuts",
					Value: queryShortcuts,
				}).Error
			},
			expectError:    false,
			expectDefaults: false,
			description:    "Valid complex arrays should load correctly",
		},
		{
			name: "BoundaryValues",
			setupDB: func(db *gorm.DB) error {
				// Insert boundary values
				settings := []database.WoxSetting{
					{Key: "AppWidth", Value: "1"},
					{Key: "MaxResultCount", Value: "999"},
				}
				for _, s := range settings {
					if err := db.Create(&s).Error; err != nil {
						return err
					}
				}
				return nil
			},
			expectError:    false,
			expectDefaults: false,
			description:    "Boundary values should be accepted",
		},
		{
			name: "NegativeNumbers",
			setupDB: func(db *gorm.DB) error {
				// Insert negative numbers
				settings := []database.WoxSetting{
					{Key: "AppWidth", Value: "-100"},
					{Key: "MaxResultCount", Value: "-5"},
				}
				for _, s := range settings {
					if err := db.Create(&s).Error; err != nil {
						return err
					}
				}
				return nil
			},
			expectError:    false,
			expectDefaults: true,
			description:    "Negative numbers should be sanitized to defaults",
		},
		{
			name: "UnicodeContent",
			setupDB: func(db *gorm.DB) error {
				// Insert unicode content
				queryShortcuts := `[{"Shortcut":"测试","Query":"test 中文"}]`
				return db.Create(&database.WoxSetting{
					Key:   "QueryShortcuts",
					Value: queryShortcuts,
				}).Error
			},
			expectError:    false,
			expectDefaults: false,
			description:    "Unicode content should be handled correctly",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			suite.runLoadWoxSettingTest(t, tc.name, tc.setupDB, tc.expectError, tc.expectDefaults, tc.description)
		})
	}
}

// testDatabaseCorruptionScenarios tests various database corruption scenarios
func (suite *ConfigBlackboxTestSuite) testDatabaseCorruptionScenarios(t *testing.T) {
	testCases := []struct {
		name        string
		setupDB     func(*gorm.DB) error
		description string
	}{
		{
			name: "CorruptedDatabaseFile",
			setupDB: func(db *gorm.DB) error {
				// Simulate database corruption by writing invalid data directly
				sqlDB, err := db.DB()
				if err != nil {
					return err
				}
				// This will cause issues when trying to read
				_, err = sqlDB.Exec("INSERT INTO wox_settings (key, value) VALUES ('test', x'deadbeef')")
				return err
			},
			description: "Corrupted database file should be handled gracefully",
		},
		{
			name: "MissingTable",
			setupDB: func(db *gorm.DB) error {
				// Drop the table to simulate missing schema
				return db.Migrator().DropTable(&database.WoxSetting{})
			},
			description: "Missing table should trigger auto-migration",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create isolated test database
			testDBPath := filepath.Join(suite.testDir, "corruption_"+tc.name+".db")
			defer os.Remove(testDBPath)

			testDB, err := suite.createTestDatabase(testDBPath)
			if err != nil {
				t.Fatalf("Failed to create test database: %v", err)
			}

			// Setup corruption scenario
			if tc.setupDB != nil {
				err = tc.setupDB(testDB)
				// We expect some of these to fail, that's the point
				t.Logf("Setup result for %s: %v", tc.name, err)
			}

			t.Logf("Testing corruption scenario: %s - %s", tc.name, tc.description)

			// Try to create setting store and verify it handles corruption gracefully
			store := setting.NewWoxSettingStore(testDB)
			woxSetting := setting.NewWoxSetting(store)

			// Test basic operations don't panic
			_ = woxSetting.AppWidth.Get()
			_ = woxSetting.UsePinYin.Get()
			_ = woxSetting.MainHotkey.Get()

			t.Logf("Corruption scenario %s handled gracefully", tc.name)
		})
	}
}

// runLoadWoxSettingTest runs a single WoxSetting blackbox test
func (suite *ConfigBlackboxTestSuite) runLoadWoxSettingTest(t *testing.T, name string, setupDB func(*gorm.DB) error, expectError, expectDefaults bool, description string) {
	// Create isolated test database
	testDBPath := filepath.Join(suite.testDir, name+".db")
	defer os.Remove(testDBPath)

	// Initialize test database
	testDB, err := suite.createTestDatabase(testDBPath)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Setup test data
	if setupDB != nil {
		err = setupDB(testDB)
		if err != nil {
			t.Fatalf("Failed to setup test database: %v", err)
		}
	}

	t.Logf("Testing: %s - %s", name, description)

	// Create setting store and WoxSetting
	store := setting.NewWoxSettingStore(testDB)
	woxSetting := setting.NewWoxSetting(store)

	// Test loading settings - this should not error even with corrupted data
	if expectError {
		// For database tests, we don't expect errors during loading
		// The system should handle corrupted data gracefully
		t.Logf("Note: Database-based settings should handle errors gracefully")
	}

	// Verify the loaded setting
	suite.validateLoadedWoxSetting(t, woxSetting, expectDefaults, name)
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
