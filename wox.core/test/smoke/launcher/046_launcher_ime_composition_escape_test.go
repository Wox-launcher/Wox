//go:build wox_ui_smoke && windows

package query

import (
	"context"
	"testing"

	"github.com/lxn/win"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxwidget "wox/ui/widget"
	"wox/util/keyboard"
)

// Test046LauncherIMECompositionEscape verifies that one composing Escape never closes the launcher.
// Flow: show and focus Wox -> deliver the native end-before-Escape ordering and a repeat -> press Escape again.
// Evidence: typing still works after the first physical press and the next press hides Wox.
func Test046LauncherIMECompositionEscape(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		smoke.ConfigureStartPage(t, ctx, client, 0)
		smoke.ShowLauncher(t, ctx, client)
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			input, found := automationdriver.Find(snapshot, "launcher.query.input")
			return snapshot.Tree.WindowFocused && found && input.Focused
		}); err != nil {
			t.Fatal(err)
		}
		hwnd := win.GetForegroundWindow()
		var foregroundPID uint32
		win.GetWindowThreadProcessId(hwnd, &foregroundPID)
		stats, err := client.RuntimeStats(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if hwnd == 0 || int(foregroundPID) != stats.PID {
			t.Fatalf("foreground HWND %v belongs to PID %d, want Wox PID %d", hwnd, foregroundPID, stats.PID)
		}
		const wmIMEStartComposition = 0x010d
		const wmIMEEndComposition = 0x010e
		win.SendMessage(hwnd, wmIMEStartComposition, 0, 0)
		win.SendMessage(hwnd, wmIMEEndComposition, 0, 0)
		win.SendMessage(hwnd, win.WM_KEYDOWN, win.VK_ESCAPE, 1)
		win.SendMessage(hwnd, win.WM_KEYDOWN, win.VK_ESCAPE, 1|(1<<30))
		win.SendMessage(hwnd, win.WM_KEYUP, win.VK_ESCAPE, 1|(1<<30)|(1<<31))
		if err := keyboard.SimulateType("x"); err != nil {
			t.Fatal(err)
		}
		snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			input, found := automationdriver.Find(snapshot, "launcher.query.input")
			return snapshot.Tree.WindowFocused && found && input.Value == "x"
		})
		if err != nil {
			input, _ := automationdriver.Find(snapshot, "launcher.query.input")
			t.Fatalf("launcher after composing Escape: focused=%t input=%q: %v", snapshot.Tree.WindowFocused, input.Value, err)
		}
		if err := smoke.SendNativeKeyChord("escape"); err != nil {
			t.Fatal(err)
		}
		if _, err := client.WaitForWindowState(ctx, "primary", func(state automationdriver.WindowState) bool {
			return !state.Visible
		}); err != nil {
			t.Fatalf("second Escape did not hide the launcher: %v", err)
		}
	})
}
