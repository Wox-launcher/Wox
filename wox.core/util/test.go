package util

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

const (
	TestWoxDataDirEnv       = "WOX_TEST_DATA_DIR"
	TestUserDataDirEnv      = "WOX_TEST_USER_DIR"
	TestServerPortEnv       = "WOX_TEST_SERVER_PORT"
	TestDisableTelemetryEnv = "WOX_TEST_DISABLE_TELEMETRY"
	TestSkipOnboardingEnv   = "WOX_TEST_SKIP_ONBOARDING"
	// TestFlowUserDataEnv points launcher import at one fixture directory.
	// Smoke sets it so onboarding does not read a developer's real Flow install.
	TestFlowUserDataEnv = "WOX_TEST_FLOW_USER_DATA"
	// TestRaycastInstallEnv points Raycast migration at a fake install marker.
	// Smoke sets it so onboarding does not scan a developer's real Raycast install.
	TestRaycastInstallEnv = "WOX_TEST_RAYCAST_INSTALL"
	// TestRaycastFileEnv is an optional .rayconfig path the onboarding unlock step can prefill.
	TestRaycastFileEnv = "WOX_TEST_RAYCAST_FILE"
)

func GetTestWoxDataDirectoryOverride() string {
	return strings.TrimSpace(os.Getenv(TestWoxDataDirEnv))
}

func GetTestUserDataDirectoryOverride() string {
	return strings.TrimSpace(os.Getenv(TestUserDataDirEnv))
}

func IsTestMode() bool {
	return GetTestWoxDataDirectoryOverride() != "" ||
		GetTestUserDataDirectoryOverride() != "" ||
		strings.TrimSpace(os.Getenv(TestServerPortEnv)) != ""
}

func GetTestServerPortOverride() (int, error) {
	portOverride := strings.TrimSpace(os.Getenv(TestServerPortEnv))
	port, err := strconv.Atoi(portOverride)
	if err != nil || port <= 0 {
		return 0, fmt.Errorf("invalid %s: %q", TestServerPortEnv, portOverride)
	}

	return port, nil
}

func ShouldDisableTelemetryForTest() bool {
	if !IsTestMode() {
		return false
	}

	return strings.EqualFold(strings.TrimSpace(os.Getenv(TestDisableTelemetryEnv)), "true")
}

// ShouldSkipOnboardingForTest keeps the bypass scoped to explicitly isolated test processes.
func ShouldSkipOnboardingForTest() bool {
	if !IsTestMode() {
		return false
	}

	return strings.EqualFold(strings.TrimSpace(os.Getenv(TestSkipOnboardingEnv)), "true")
}
