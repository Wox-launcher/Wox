//go:build wox_ui_smoke

package gouismoke

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"wox/test/automationdriver"
	woxwidget "wox/ui/widget"
)

// TestSettingsSmoke verifies General settings renders in the independent settings window.
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

	if err := process.Client.Show(ctx); err != nil {
		t.Fatalf("show launcher: %v", err)
	}
	_, err = process.Client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, found := automationdriver.Find(snapshot, "launcher.query.input")
		return found
	})
	if err != nil {
		t.Fatalf("wait for query input: %v", err)
	}

	if err := process.Client.OpenSettings(ctx, "/general"); err != nil {
		t.Fatalf("open General settings: %v", err)
	}
	_, err = process.Client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, windowFound := automationdriver.Find(snapshot, "settings.window")
		_, pageFound := automationdriver.Find(snapshot, "settings.page.general")
		return windowFound && pageFound
	})
	if err != nil {
		t.Fatalf("wait for General settings: %v", err)
	}
}
