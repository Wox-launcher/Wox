//go:build wox_ui_smoke

package gouismoke

import (
	"context"
	"fmt"
	"image/png"
	"math"
	"net"
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

// TestLauncherQuerySmoke covers the first native black-box product path without test routes.
func TestLauncherQuerySmoke(t *testing.T) {
	executable := strings.TrimSpace(os.Getenv("WOX_GO_UI_SMOKE_BINARY"))
	if executable == "" {
		t.Skip("WOX_GO_UI_SMOKE_BINARY is not configured")
	}
	absoluteExecutable, err := filepath.Abs(executable)
	if err != nil {
		t.Fatalf("resolve Wox binary: %v", err)
	}
	port := availablePort(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	process, err := automationdriver.Launch(ctx, absoluteExecutable, automationdriver.LaunchOptions{
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
	initialBounds, err := process.Client.Bounds(ctx)
	if err != nil {
		t.Fatalf("read initial launcher bounds: %v", err)
	}
	movedBounds := initialBounds
	movedBounds.X += 37
	movedBounds.Y += 29
	if err := process.Client.SetBounds(ctx, movedBounds); err != nil {
		t.Fatalf("move launcher before query: %v", err)
	}
	actualMovedBounds, err := process.Client.Bounds(ctx)
	if err != nil {
		t.Fatalf("read moved launcher bounds: %v", err)
	}
	assertWindowOrigin(t, actualMovedBounds, movedBounds)
	for _, query := range []string{"s", "sm", "smo", "smok", "smoke"} {
		if err := process.Client.Perform(ctx, "launcher.query.input", woxui.AccessibilityActionSetValue, query); err != nil {
			t.Fatalf("enter rapid query %q: %v", query, err)
		}
	}
	_, err = process.Client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		node, found := automationdriver.Find(snapshot, "launcher.query.input")
		return found && node.Value == "smoke"
	})
	if err != nil {
		t.Fatalf("wait for rapid query input: %v", err)
	}
	if err := process.Client.Perform(ctx, "launcher.query.input", woxui.AccessibilityActionSetValue, "1+1"); err != nil {
		t.Fatalf("enter calculator query: %v", err)
	}
	snapshot, err := process.Client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, found := calculatorResult(snapshot)
		return found
	})
	if err != nil {
		t.Fatalf("wait for query result: %v", err)
	}
	if len(snapshot.Diagnostics) > 0 {
		t.Fatalf("launcher semantics diagnostics: %v", snapshot.Diagnostics)
	}
	resultBounds, err := process.Client.Bounds(ctx)
	if err != nil {
		t.Fatalf("read launcher bounds after query: %v", err)
	}
	assertWindowOrigin(t, resultBounds, movedBounds)
	resultID, _ := calculatorResult(snapshot)
	if err := process.Client.Perform(ctx, resultID, woxui.AccessibilityActionFocus, ""); err != nil {
		t.Fatalf("focus first result: %v", err)
	}
	_, err = process.Client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		node, found := automationdriver.Find(snapshot, resultID)
		return found && node.Focused
	})
	if err != nil {
		t.Fatalf("wait for focused result: %v", err)
	}
	artifactDirectory := strings.TrimSpace(os.Getenv("WOX_GO_UI_ARTIFACT_DIR"))
	if artifactDirectory == "" {
		artifactDirectory = t.TempDir()
	}
	if err := os.MkdirAll(artifactDirectory, 0o755); err != nil {
		t.Fatalf("create visual artifact directory: %v", err)
	}
	capturePath := filepath.Join(artifactDirectory, "launcher-query-"+runtime.GOOS+".png")
	if err := process.Client.Capture(ctx, capturePath); err != nil {
		t.Fatalf("capture launcher visual: %v", err)
	}
	assertPNG(t, capturePath)
	assertVisualGolden(t, capturePath)
	if err := process.Client.Hide(ctx); err != nil {
		t.Fatalf("hide launcher: %v", err)
	}
}

func assertWindowOrigin(t *testing.T, actual, expected woxui.Rect) {
	t.Helper()
	if math.Abs(float64(actual.X-expected.X)) > 1 || math.Abs(float64(actual.Y-expected.Y)) > 1 {
		t.Fatalf("launcher origin = %.1f,%.1f, want %.1f,%.1f", actual.X, actual.Y, expected.X, expected.Y)
	}
}

func assertPNG(t *testing.T, path string) {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open launcher capture: %v", err)
	}
	defer file.Close()
	config, err := png.DecodeConfig(file)
	if err != nil {
		t.Fatalf("decode launcher capture: %v", err)
	}
	if config.Width < 600 || config.Height < 100 {
		t.Fatalf("launcher capture is unexpectedly small: %dx%d", config.Width, config.Height)
	}
}

func calculatorResult(snapshot woxwidget.AutomationSnapshot) (string, bool) {
	for _, node := range snapshot.Tree.Nodes {
		if strings.HasPrefix(node.AutomationID, "launcher.result.") && strings.TrimSpace(node.Label) == "2" {
			return node.AutomationID, true
		}
	}
	return "", false
}

func availablePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve control port: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatalf("release control port: %v", err)
	}
	return port
}
