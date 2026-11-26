//go:build wox_ui_smoke && linux

package query

import (
	"context"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxwidget "wox/ui/widget"
	"wox/util"
)

// Test046LauncherGTKCompositionEscape verifies that Escape cancels GTK Unicode preedit.
// Flow: focus Wox -> start the GTK Unicode input sequence -> enter a code point -> press Escape twice.
// Evidence: the first Escape cancels the preedit without committing it and the second Escape hides Wox.
func Test046LauncherGTKCompositionEscape(t *testing.T) {
	if util.IsLinuxWaylandSession() {
		t.Skip("GTK Unicode preedit fixture injects native X11 keys")
	}
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		smoke.ConfigureStartPage(t, ctx, client, 0)
		smoke.ShowLauncher(t, ctx, client)
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			input, found := automationdriver.Find(snapshot, "launcher.query.input")
			return snapshot.Tree.WindowFocused && found && input.Focused
		}); err != nil {
			t.Fatal(err)
		}
		if err := smoke.SendNativeKeyChord("Control_L", "Shift_L", "u"); err != nil {
			t.Fatal(err)
		}
		for _, key := range []string{"4", "e", "2", "d", "Escape", "x"} {
			if err := smoke.SendNativeKeyChord(key); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			input, found := automationdriver.Find(snapshot, "launcher.query.input")
			return snapshot.Tree.WindowFocused && found && input.Value == "x"
		}); err != nil {
			t.Fatalf("Escape failed to cancel GTK preedit without hiding Wox: %v", err)
		}
		if err := smoke.SendNativeKeyChord("Escape"); err != nil {
			t.Fatal(err)
		}
		if _, err := client.WaitForWindowState(ctx, "primary", func(state automationdriver.WindowState) bool {
			return !state.Visible
		}); err != nil {
			t.Fatalf("second Escape did not hide Wox: %v", err)
		}
	})
}
