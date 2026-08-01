//go:build wox_ui_smoke

package gouismoke

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"wox/test/automationdriver"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// TestSettingsSmoke verifies built-in pages and shared choice menus in the independent settings window.
func TestSettingsSmoke(t *testing.T) {
	executable := strings.TrimSpace(os.Getenv("WOX_GO_UI_SMOKE_BINARY"))
	if executable == "" {
		t.Skip("WOX_GO_UI_SMOKE_BINARY is not configured")
	}
	absolute, err := filepath.Abs(executable)
	if err != nil {
		t.Fatalf("resolve Wox binary: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	port := availablePort(t)
	process, err := automationdriver.Launch(ctx, absolute, automationdriver.LaunchOptions{
		Environment: []string{
			"WOX_TEST_DATA_DIR=" + t.TempDir(),
			"WOX_TEST_USER_DIR=" + t.TempDir(),
			fmt.Sprintf("WOX_TEST_SERVER_PORT=%d", port),
			"WOX_TEST_DISABLE_TELEMETRY=true",
		},
		StartupTimeout: 45 * time.Second,
	})
	if err != nil {
		t.Fatalf("launch Wox: %v", err)
	}
	defer process.Close()

	showLauncher(t, ctx, process.Client)

	if err := process.Client.OpenSettings(ctx, "/data"); err != nil {
		t.Fatalf("open Data settings: %v", err)
	}
	_, err = process.Client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, windowFound := automationdriver.Find(snapshot, "settings.window")
		_, pageFound := automationdriver.Find(snapshot, "settings.page.data")
		_, logLevelFound := automationdriver.Find(snapshot, "data-log-level")
		return windowFound && pageFound && logLevelFound
	})
	if err != nil {
		t.Fatalf("wait for Data settings: %v", err)
	}

	if err := process.Client.Perform(ctx, "data-log-level", woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("open log level dropdown: %v", err)
	}
	if _, err := process.Client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, menuFound := automationdriver.Find(snapshot, "setting-choice-menu")
		info, infoFound := automationdriver.Find(snapshot, "setting-choice-0")
		debug, debugFound := automationdriver.Find(snapshot, "setting-choice-1")
		return menuFound && infoFound && debugFound && info.Label == "INFO" && debug.Label == "DEBUG"
	}); err != nil {
		t.Fatalf("wait for log level dropdown choices: %v", err)
	}
	artifactDirectory := strings.TrimSpace(os.Getenv("WOX_GO_UI_ARTIFACT_DIR"))
	if artifactDirectory == "" {
		artifactDirectory = t.TempDir()
	}
	if err := os.MkdirAll(artifactDirectory, 0o755); err != nil {
		t.Fatalf("create settings artifact directory: %v", err)
	}
	capturePath := filepath.Join(artifactDirectory, "settings-data-log-level-"+runtime.GOOS+".png")
	if err := process.Client.Capture(ctx, capturePath); err != nil {
		t.Fatalf("capture log level dropdown: %v", err)
	}
	assertPNG(t, capturePath)
}
