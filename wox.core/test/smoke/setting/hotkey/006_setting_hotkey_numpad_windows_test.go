//go:build wox_ui_smoke

package hotkey

import (
	"context"
	"errors"
	"testing"
	"time"

	"wox/test/automationdriver"
	"wox/test/smoke"
)

// Test006SettingHotkeyNumpad records a keypad binding and checks that only that chord opens Wox.
func Test006SettingHotkeyNumpad(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		const hotkey = "ctrl+numpad0"
		ensureMainHotkey(t, ctx, client, hotkey)
		if err := client.Hide(ctx); err != nil {
			t.Fatal(err)
		}
		visible := func(state automationdriver.WindowState) bool { return state.Visible }
		sendNativeHotkey(t, "ctrl+0")
		digitCtx, cancel := context.WithTimeout(ctx, 400*time.Millisecond)
		_, err := client.WaitForWindowState(digitCtx, "primary", visible)
		cancel()
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("top-row Ctrl+0 should leave Wox hidden: %v", err)
		}
		sendNativeHotkey(t, hotkey)
		if _, err := client.WaitForWindowState(ctx, "primary", visible); err != nil {
			t.Fatalf("%s did not show Wox: %v", hotkey, err)
		}
	})
}
