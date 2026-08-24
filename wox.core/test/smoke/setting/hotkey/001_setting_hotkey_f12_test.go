//go:build wox_ui_smoke

package hotkey

import (
	"context"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const mainHotkeyFieldID = "hotkey-settings-field-0"

// Test001SettingHotkeyF12Recording verifies that standalone F12 can be recorded as the main hotkey.
func Test001SettingHotkeyF12Recording(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		testMainHotkeyRecording(t, ctx, client, "f12")
	})
}

// Test001SettingHotkeyCtrlF12Recording verifies that Ctrl+F12 can be recorded as the main hotkey.
func Test001SettingHotkeyCtrlF12Recording(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		testMainHotkeyRecording(t, ctx, client, "ctrl+f12")
	})
}

// testMainHotkeyRecording records a candidate through the native Windows keyboard path.
func testMainHotkeyRecording(t *testing.T, ctx context.Context, client *automationdriver.Client, hotkey string) {
	t.Helper()
	previous := openMainHotkeySettings(t, ctx, client)
	t.Cleanup(func() { restoreMainHotkey(t, client, previous) })

	recorderDescription := startMainHotkeyRecording(t, ctx, client)
	sendNativeHotkey(t, hotkey)
	waitForMainHotkeyValue(t, ctx, client, hotkey)
	stopMainHotkeyRecording(t, ctx, client, recorderDescription)
}

func openMainHotkeySettings(t *testing.T, ctx context.Context, client *automationdriver.Client) string {
	t.Helper()
	if err := client.OpenSettings(ctx, "/general"); err != nil {
		t.Fatalf("open General settings: %v", err)
	}
	if err := client.Perform(ctx, "settings-search-field", woxui.AccessibilityActionSetValue, "main hotkey"); err != nil {
		t.Fatalf("search for the main hotkey setting: %v", err)
	}
	if err := client.PressKey(ctx, woxui.KeyEnter, 0); err != nil {
		t.Fatalf("open the main hotkey setting from search: %v", err)
	}
	snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, pageFound := automationdriver.Find(snapshot, "settings.page.general")
		field, fieldFound := automationdriver.Find(snapshot, mainHotkeyFieldID)
		return pageFound && fieldFound && field.Value != ""
	})
	if err != nil {
		t.Fatalf("wait for main hotkey field: %v", err)
	}
	field, _ := automationdriver.Find(snapshot, mainHotkeyFieldID)
	return field.Value
}

func startMainHotkeyRecording(t *testing.T, ctx context.Context, client *automationdriver.Client) string {
	t.Helper()
	snapshot, err := client.Snapshot(ctx)
	if err != nil {
		t.Fatalf("read main hotkey recorder before activation: %v", err)
	}
	field, found := automationdriver.Find(snapshot, mainHotkeyFieldID)
	if !found {
		t.Fatalf("main hotkey recorder was not found")
	}
	previousDescription := field.Description
	if err := client.Perform(ctx, mainHotkeyFieldID, woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("start main hotkey recording: %v", err)
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		field, found := automationdriver.Find(snapshot, mainHotkeyFieldID)
		return found && field.Description != previousDescription
	}); err != nil {
		t.Fatalf("wait for main hotkey recording state: %v", err)
	}
	return previousDescription
}

func waitForMainHotkeyValue(t *testing.T, ctx context.Context, client *automationdriver.Client, expected string) {
	t.Helper()
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		field, found := automationdriver.Find(snapshot, mainHotkeyFieldID)
		return found && field.Value == expected
	}); err != nil {
		t.Fatalf("wait for main hotkey %q: %v", expected, err)
	}
}

func stopMainHotkeyRecording(t *testing.T, ctx context.Context, client *automationdriver.Client, description string) {
	t.Helper()
	if err := client.PressKey(ctx, woxui.KeyEscape, 0); err != nil {
		t.Fatalf("stop main hotkey recording: %v", err)
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		field, found := automationdriver.Find(snapshot, mainHotkeyFieldID)
		return found && field.Description == description
	}); err != nil {
		t.Fatalf("wait for main hotkey recorder to stop: %v", err)
	}
}

func restoreMainHotkey(t *testing.T, client *automationdriver.Client, previous string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), automationdriver.ActionTimeout)
	defer cancel()
	if err := client.Hide(ctx); err != nil {
		t.Errorf("hide active window before restoring main hotkey: %v", err)
	}
	current := openMainHotkeySettings(t, ctx, client)
	if current == previous {
		if err := client.Hide(ctx); err != nil {
			t.Errorf("close settings after confirming main hotkey restore: %v", err)
		}
		return
	}
	recorderDescription := startMainHotkeyRecording(t, ctx, client)
	sendNativeHotkey(t, previous)
	waitForMainHotkeyValue(t, ctx, client, previous)
	stopMainHotkeyRecording(t, ctx, client, recorderDescription)
	if err := client.Hide(ctx); err != nil {
		t.Errorf("close settings after restoring main hotkey: %v", err)
	}
}
