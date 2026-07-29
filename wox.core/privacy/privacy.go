package privacy

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"wox/database"
	"wox/diagnostic"
	"wox/setting"
	"wox/util"
	"wox/util/shell"
)

const (
	CleanupArg           = "--privacy-cleanup"
	profileFileName      = "privacy.json"
	localIdentityStateID = 1
)

var profileMu sync.Mutex

type Profile struct {
	PreservedSettings PreservedSettings `json:"preserved_settings"`
	DeviceID          string            `json:"device_id,omitempty"`
	TelemetryState    *TelemetryState   `json:"telemetry_state,omitempty"`
}

type PreservedSettings struct {
	LangCode                  string             `json:"lang_code"`
	MainHotkey                string             `json:"main_hotkey"`
	SelectionHotkey           string             `json:"selection_hotkey"`
	UiDensity                 string             `json:"ui_density"`
	EnableAnonymousUsageStats bool               `json:"enable_anonymous_usage_stats"`
	EnableGlance              *bool              `json:"enable_glance,omitempty"`
	PrimaryGlance             *setting.GlanceRef `json:"primary_glance,omitempty"`
	HideGlanceIcon            *bool              `json:"hide_glance_icon,omitempty"`
}

type TelemetryState struct {
	InstallID       string `json:"install_id"`
	LastSentAt      int64  `json:"last_sent_at"`
	LastSentVersion string `json:"last_sent_version"`
}

func IsCleanupProcess(args []string) bool {
	if len(args) != 3 || args[1] != CleanupArg {
		return false
	}
	parentPID, err := strconv.Atoi(args[2])
	return err == nil && parentPID > 0
}

// PrepareAtStartup removes data left by the previous private session before Wox opens logs or databases.
func PrepareAtStartup() error {
	root, err := dataDirectory()
	if err != nil {
		return err
	}
	if !profileExists(root) {
		return nil
	}
	return cleanup(root)
}

func IsEnabled() bool {
	root, err := dataDirectory()
	if err != nil {
		return false
	}
	return profileExists(root)
}

// SetEnabled creates or removes the profile file that represents private mode.
func SetEnabled(enabled bool, woxSetting *setting.WoxSetting) error {
	root, err := dataDirectory()
	if err != nil {
		return err
	}
	if !enabled {
		err := os.Remove(filepath.Join(root, profileFileName))
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if err := validateUserDataLocation(); err != nil {
		return err
	}

	profile := Profile{
		PreservedSettings: captureSettings(woxSetting),
	}
	captureIdentityState(&profile)
	return saveProfile(profile)
}

// RefreshPreservedSettings keeps the fixed safe setting allowlist current while private mode is enabled.
func RefreshPreservedSettings(woxSetting *setting.WoxSetting) error {
	root, err := dataDirectory()
	if err != nil {
		return err
	}
	if !profileExists(root) {
		return nil
	}
	profile, err := loadProfile(root)
	if err != nil {
		return err
	}
	profile.PreservedSettings = captureSettings(woxSetting)
	captureIdentityState(&profile)
	return saveProfile(profile)
}

// ApplyPreservedSettings restores only the fixed safe allowlist into the fresh session database.
func ApplyPreservedSettings(woxSetting *setting.WoxSetting) error {
	root, err := dataDirectory()
	if err != nil {
		return err
	}
	if !profileExists(root) {
		return nil
	}
	profile, err := loadProfile(root)
	if err != nil {
		return err
	}

	preserved := profile.PreservedSettings
	var applyErrors []error
	if preserved.LangCode != "" {
		applyErrors = appendIfError(applyErrors, woxSetting.LangCode.SetFromString(preserved.LangCode))
	}
	if preserved.MainHotkey != "" {
		applyErrors = appendIfError(applyErrors, woxSetting.MainHotkey.SetFromString(preserved.MainHotkey))
	}
	if preserved.SelectionHotkey != "" {
		applyErrors = appendIfError(applyErrors, woxSetting.SelectionHotkey.SetFromString(preserved.SelectionHotkey))
	}
	if preserved.UiDensity != "" {
		applyErrors = appendIfError(applyErrors, woxSetting.UiDensity.SetFromString(preserved.UiDensity))
	}
	applyErrors = appendIfError(applyErrors, woxSetting.EnableAnonymousUsageStats.SetLocal(preserved.EnableAnonymousUsageStats))
	if preserved.EnableGlance != nil {
		applyErrors = appendIfError(applyErrors, woxSetting.EnableGlance.SetLocal(*preserved.EnableGlance))
	}
	if preserved.PrimaryGlance != nil {
		applyErrors = appendIfError(applyErrors, woxSetting.PrimaryGlance.SetLocal(*preserved.PrimaryGlance))
	}
	if preserved.HideGlanceIcon != nil {
		applyErrors = appendIfError(applyErrors, woxSetting.HideGlanceIcon.SetLocal(*preserved.HideGlanceIcon))
	}
	// A privacy profile can only be enabled after onboarding, so a cleaned session must not show it again.
	applyErrors = appendIfError(applyErrors, woxSetting.OnboardingFinished.SetLocal(true))

	db := database.GetDB()
	if db != nil && profile.DeviceID != "" {
		applyErrors = appendIfError(applyErrors, db.Save(&database.DeviceIdentity{ID: localIdentityStateID, DeviceID: profile.DeviceID}).Error)
	}
	if db != nil && profile.TelemetryState != nil && profile.TelemetryState.InstallID != "" {
		state := profile.TelemetryState
		applyErrors = appendIfError(applyErrors, db.Save(&database.TelemetryState{
			ID:              localIdentityStateID,
			InstallID:       state.InstallID,
			LastSentAt:      state.LastSentAt,
			LastSentVersion: state.LastSentVersion,
		}).Error)
	}
	return errors.Join(applyErrors...)
}

// StartExitCleanup starts a short-lived copy of Wox that cleans data after this process releases its files.
func StartExitCleanup(woxSetting *setting.WoxSetting) error {
	root, err := dataDirectory()
	if err != nil {
		return err
	}
	if !profileExists(root) {
		return nil
	}
	profile, err := loadProfile(root)
	if err != nil {
		return err
	}
	profile.PreservedSettings = captureSettings(woxSetting)
	captureIdentityState(&profile)
	if err := saveProfile(profile); err != nil {
		return err
	}

	executable, err := os.Executable()
	if err != nil {
		return err
	}
	return shell.BuildCommand(executable, nil, CleanupArg, strconv.Itoa(os.Getpid())).Start()
}

// RunCleanupProcess retries until the exiting Wox process and UI release their files.
func RunCleanupProcess(args []string) int {
	root, err := dataDirectory()
	if err != nil {
		return 1
	}
	parentPID, _ := strconv.Atoi(args[2])
	deadline := time.Now().Add(30 * time.Second)
	for diagnostic.IsProcessRunning(parentPID) && time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
	}
	for time.Now().Before(deadline) {
		if !profileExists(root) {
			return 0
		}
		if err := cleanup(root); err == nil {
			return 0
		}
		time.Sleep(200 * time.Millisecond)
	}
	return 1
}

func captureSettings(woxSetting *setting.WoxSetting) PreservedSettings {
	enableGlance := woxSetting.EnableGlance.Get()
	primaryGlance := woxSetting.PrimaryGlance.Get()
	hideGlanceIcon := woxSetting.HideGlanceIcon.Get()
	return PreservedSettings{
		LangCode:                  string(woxSetting.LangCode.Get()),
		MainHotkey:                woxSetting.MainHotkey.Get(),
		SelectionHotkey:           woxSetting.SelectionHotkey.Get(),
		UiDensity:                 string(woxSetting.UiDensity.Get()),
		EnableAnonymousUsageStats: woxSetting.EnableAnonymousUsageStats.Get(),
		EnableGlance:              &enableGlance,
		PrimaryGlance:             &primaryGlance,
		HideGlanceIcon:            &hideGlanceIcon,
	}
}

func captureIdentityState(profile *Profile) {
	db := database.GetDB()
	if db == nil {
		return
	}

	profile.DeviceID = ""
	profile.TelemetryState = nil
	var identity database.DeviceIdentity
	if err := db.First(&identity, localIdentityStateID).Error; err == nil {
		profile.DeviceID = identity.DeviceID
	}

	var telemetry database.TelemetryState
	if err := db.First(&telemetry, localIdentityStateID).Error; err == nil && telemetry.InstallID != "" {
		profile.TelemetryState = &TelemetryState{
			InstallID:       telemetry.InstallID,
			LastSentAt:      telemetry.LastSentAt,
			LastSentVersion: telemetry.LastSentVersion,
		}
	}
}

func appendIfError(errs []error, err error) []error {
	if err != nil {
		return append(errs, err)
	}
	return errs
}

func dataDirectory() (string, error) {
	if override := util.GetTestWoxDataDirectoryOverride(); override != "" {
		return filepath.Abs(override)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".wox"), nil
}

func validateUserDataLocation() error {
	return ValidateUserDataDirectory(util.GetLocation().GetUserDataDirectory())
}

// ValidateUserDataDirectory prevents private mode from claiming it cleans data outside Wox's managed root.
func ValidateUserDataDirectory(userData string) error {
	root, err := dataDirectory()
	if err != nil {
		return err
	}
	relative, err := filepath.Rel(root, userData)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("privacy mode requires the user data location to be inside %s", root)
	}
	return nil
}

func loadProfile(root string) (Profile, error) {
	profileMu.Lock()
	defer profileMu.Unlock()

	data, err := os.ReadFile(filepath.Join(root, profileFileName))
	if os.IsNotExist(err) {
		return Profile{}, nil
	}
	if err != nil {
		return Profile{}, err
	}
	var profile Profile
	if err := json.Unmarshal(data, &profile); err != nil {
		return Profile{}, fmt.Errorf("failed to read privacy profile: %w", err)
	}
	return profile, nil
}

func profileExists(root string) bool {
	_, err := os.Stat(filepath.Join(root, profileFileName))
	return err == nil
}

func saveProfile(profile Profile) error {
	profileMu.Lock()
	defer profileMu.Unlock()

	root, err := dataDirectory()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(root, profileFileName), data, 0600)
}

func cleanup(root string) error {
	info, err := os.Lstat(root)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing to clean symlinked Wox data directory: %s", root)
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	var cleanupErrors []error
	for _, entry := range entries {
		if entry.Name() == profileFileName {
			continue
		}
		if err := os.RemoveAll(filepath.Join(root, entry.Name())); err != nil {
			cleanupErrors = append(cleanupErrors, err)
		}
	}
	return errors.Join(cleanupErrors...)
}
