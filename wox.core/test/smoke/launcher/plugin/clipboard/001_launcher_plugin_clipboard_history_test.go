//go:build wox_ui_smoke

package clipboard

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	woxclipboard "wox/util/clipboard"
)

// Test001LauncherPluginClipboardHistory verifies system clipboard capture, search, and result copy.
// Flow: inject external clipboard text -> query Clipboard -> open result actions -> activate Copy.
// Evidence: the plugin exposes the injected text and the Copy action restores it to the system clipboard.
func Test001LauncherPluginClipboardHistory(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		smoke.PreserveClipboard(t)
		smoke.ShowLauncher(t, ctx, client)

		marker := fmt.Sprintf("wox-smoke-clipboard-%d", time.Now().UnixNano())
		if err := writeExternalClipboard(marker); err != nil {
			t.Fatalf("write external clipboard text: %v", err)
		}

		snapshot := waitForClipboardResult(t, ctx, client, marker)
		resultID, found := smoke.FindLauncherResult(snapshot, marker)
		if !found {
			t.Fatalf("Clipboard result %q was not found", marker)
		}
		smoke.SelectLauncherResult(t, ctx, client, resultID)

		snapshot = smoke.OpenResultActionPanel(t, ctx, client)
		copyAction, found := automationdriver.FindByAutomationIDPrefix(snapshot, "action-result-")
		if !found {
			t.Fatal("Clipboard Copy action was not exposed")
		}
		if err := client.Perform(ctx, copyAction.AutomationID, woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("activate Clipboard Copy action: %v", err)
		}
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			copied, readErr := woxclipboard.ReadText()
			return readErr == nil && copied == marker
		}); err != nil {
			t.Fatalf("wait for Clipboard Copy result: %v", err)
		}

		copied, err := woxclipboard.ReadText()
		if err != nil || copied != marker {
			t.Fatalf("clipboard after Clipboard Copy = %q err %v, want %q", copied, err, marker)
		}
		snapshot, err = client.Snapshot(ctx)
		if err != nil {
			t.Fatalf("read Clipboard smoke semantics: %v", err)
		}
		smoke.AssertNoDiagnostics(t, snapshot)
	})
}

// waitForClipboardResult retries completed queries until the asynchronous clipboard watcher has persisted the marker.
func waitForClipboardResult(t *testing.T, ctx context.Context, client *automationdriver.Client, marker string) woxwidget.AutomationSnapshot {
	t.Helper()
	queries := []string{"cb " + marker, "cb " + marker + " "}
	for attempt := 0; ; attempt++ {
		query := queries[attempt%len(queries)]
		if err := client.Perform(ctx, "launcher.query.input", woxui.AccessibilityActionSetValue, query); err != nil {
			t.Fatalf("enter Clipboard query %q: %v", query, err)
		}
		snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			input, inputFound := automationdriver.Find(snapshot, "launcher.query.input")
			return inputFound && input.Value == query
		})
		if err != nil {
			t.Fatalf("wait for Clipboard query %q: %v", query, err)
		}
		results, resultsFound := automationdriver.Find(snapshot, "launcher.results")
		if resultsFound && results.Value == "complete" {
			if _, found := smoke.FindLauncherResult(snapshot, marker); found {
				return snapshot
			}
		}
	}
}

// writeExternalClipboard changes the OS clipboard outside Wox so its watcher treats the update as user-originated.
func writeExternalClipboard(text string) error {
	var command *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		command = exec.Command("clip.exe")
	case "darwin":
		command = exec.Command("pbcopy")
	case "linux":
		for _, candidate := range []struct {
			name string
			args []string
		}{
			{name: "wl-copy"},
			{name: "xclip", args: []string{"-selection", "clipboard"}},
			{name: "xsel", args: []string{"--clipboard", "--input"}},
		} {
			path, err := exec.LookPath(candidate.name)
			if err == nil {
				command = exec.Command(path, candidate.args...)
				break
			}
		}
		if command == nil {
			return fmt.Errorf("no supported Linux clipboard command found")
		}
	default:
		return fmt.Errorf("unsupported OS %q", runtime.GOOS)
	}
	command.Stdin = strings.NewReader(text)
	return command.Run()
}
