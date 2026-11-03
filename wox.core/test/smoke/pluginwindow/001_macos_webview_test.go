//go:build wox_ui_smoke

package pluginwindow

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"testing"
	"time"

	"wox/test/automationdriver"
	"wox/test/smoke"
	"wox/util"
)

// Test001MacOSWebView checks a draggable plugin window using AppKit events and WKWebView.
// The fixture verifies transparent rendered pixels, hover enter/leave, and a close click.
func Test001MacOSWebView(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("the native WKWebView smoke runs on macOS")
	}

	smoke.CaseWithTimeout(t, 90*time.Second, func(ctx context.Context, _ *automationdriver.Client) {
		command := exec.CommandContext(ctx, "go", "run", "./testdata")
		command.Env = append(os.Environ(), util.TestWoxDataDirEnv+"="+t.TempDir(), util.TestUserDataDirEnv+"="+t.TempDir())
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("native plugin-window WebView smoke: %v\n%s", err, output)
		}
	})
}
