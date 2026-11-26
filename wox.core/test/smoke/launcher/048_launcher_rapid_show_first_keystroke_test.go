//go:build wox_ui_smoke && (windows || linux || darwin)

package query

import (
	"context"
	"fmt"
	"runtime"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxwidget "wox/ui/widget"
	"wox/util"
	"wox/util/keyboard"
	"wox/util/window"
)

// Test048LauncherRapidShowFirstKeystroke verifies native input immediately after launcher activation.
// Flow: hide Wox -> show the recent-items page -> send characters immediately through the OS.
// Evidence: the focused query editor contains every character in order after each activation.
func Test048LauncherRapidShowFirstKeystroke(t *testing.T) {
	if util.IsLinuxWaylandSession() {
		t.Skip("native keystroke injection in this fixture requires X11")
	}
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		smoke.ConfigureStartPage(t, ctx, client, 1)
		stats, err := client.RuntimeStats(ctx)
		if err != nil {
			t.Fatal(err)
		}
		for _, text := range []string{"abc", "xyz", "wox"} {
			if err := client.Show(ctx); err != nil {
				t.Fatal(err)
			}
			if runtime.GOOS == "windows" {
				if foreground := window.GetActiveWindowPid(); foreground != stats.PID {
					t.Fatalf("show returned before Windows granted foreground focus: PID %d, want %d", foreground, stats.PID)
				}
			}
			if runtime.GOOS != "linux" {
				if err := keyboard.SimulateType(text); err != nil {
					t.Fatal(err)
				}
			} else {
				for _, key := range text {
					if err := smoke.SendNativeKeyChord(string(key)); err != nil {
						t.Fatal(err)
					}
				}
			}
			snapshot, err := client.WaitForReason(ctx, func(snapshot woxwidget.AutomationSnapshot) (bool, string) {
				input, found := automationdriver.Find(snapshot, "launcher.query.input")
				return snapshot.Tree.WindowFocused && found && input.Focused && input.Value == text,
					fmt.Sprintf("window focused=%t, editor focused=%t, input=%q", snapshot.Tree.WindowFocused, input.Focused, input.Value)
			})
			if err != nil {
				t.Fatalf("wait for all native keystrokes %q: %v", text, err)
			}
			smoke.AssertNoDiagnostics(t, snapshot)
			if err := client.Hide(ctx); err != nil {
				t.Fatal(err)
			}
		}
	})
}
