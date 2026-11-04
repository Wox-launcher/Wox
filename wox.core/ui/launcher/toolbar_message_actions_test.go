package launcher

import (
	"context"
	"testing"
	"time"

	"wox/common"
	woxui "wox/ui/runtime"
)

func TestTruncatedMessageWithoutPluginActionsOffersCopyAndFullToast(t *testing.T) {
	app := bindTestApp(&App{lifecycleCtx: context.Background(), querySession: queryController{editor: woxui.NewTextEditor("")}, toolbar: toolbarController{toolbarMsg: &toolbarMessage{ID: "long", Title: "The full long message"}, messageTruncated: true}})
	shown := make(chan string, 1)
	app.toolbar.showFullMessage = func(text string) bool { shown <- text; return true }
	app.toggleToolbarPanel()
	entries := app.toolbarActionPanelEntries()
	if len(entries) != 2 || entries[0].ID != "toolbar-copy-message" || entries[1].ID != "toolbar-show-message" {
		t.Fatalf("message actions: %+v", entries)
	}
	app.activateActionPanelEntry(entries[1])
	select {
	case text := <-shown:
		if text != "The full long message" {
			t.Fatalf("toast text = %q", text)
		}
	case <-time.After(time.Second):
		t.Fatal("full message toast was not shown")
	}
	if app.actions.actionPanel {
		t.Fatal("showing the toast should close the panel")
	}
}

func TestFallbackToolbarActionOpensMainHotkeySettings(t *testing.T) {
	message := fromSystemToolbarMessage(common.SystemToolbarMsg{Id: "warning", Title: "Hotkey failed", Actions: []common.SystemToolbarAction{{Id: "open", Name: "Change hotkey", SettingsPath: "/hotkeys", SettingsParam: "MainHotkey"}}})
	app := bindTestApp(&App{lifecycleCtx: context.Background(), querySession: queryController{editor: woxui.NewTextEditor("")}})
	app.toolbar.applySystemToolbarMessage(message, common.SystemToolbarPriorityWarning, 0)
	opened := make(chan common.SettingWindowContext, 1)
	app.toolbar.openSettings = func(_ context.Context, target common.SettingWindowContext) error { opened <- target; return nil }
	app.toggleToolbarPanel()
	entries := app.toolbarActionPanelEntries()
	app.activateActionPanelEntry(entries[0])
	select {
	case target := <-opened:
		if target.Path != "/hotkeys" || target.Param != "MainHotkey" {
			t.Fatalf("target = %+v", target)
		}
	case <-time.After(time.Second):
		t.Fatal("hotkey settings did not open")
	}
	app.toggleToolbarPanel()
	app.toolbar.clearSystemToolbarMessage("warning")
	if app.actions.actionPanel || app.toolbar.effectiveToolbarMessage() != nil {
		t.Fatal("clearing the fallback must close its action panel")
	}
}

func TestFocusRequestedHotkeySetting(t *testing.T) {
	app := bindTestApp(&App{hotkeySettings: newHotkeySettingsController(CommonDeps{})})
	form := newHotkeySettingsForm(settingsData{})
	app.hotkeySettings.SetForm(&form)
	for _, key := range []string{"MainHotkey", "ResultBindings"} {
		app.settingsSearch.SetFocused(true)
		app.focusHotkeySetting(key)
		if form.focused < 0 || form.definitions[form.focused].Value.Key != key || !app.hotkeySettings.Focused() || app.settingsSearch.Focused() {
			t.Fatalf("%s was not focused: form=%d", key, form.focused)
		}
	}
}

func TestTruncatedMessageReservesShortcutAndKeepsPanelStable(t *testing.T) {
	app := bindTestApp(&App{querySession: queryController{editor: woxui.NewTextEditor("")}, resultList: resultController{results: []queryResult{{Actions: []resultAction{{ID: "conflict", Hotkey: primaryHotkey("t")}}}}}, toolbar: toolbarController{toolbarMsg: &toolbarMessage{ID: "long", Title: "Long text"}, messageTruncated: true}})
	if entries := app.resultActionPanelEntries(); len(entries) != 1 || entries[0].Hotkey != "" {
		t.Fatalf("conflicting result action = %+v", entries)
	}
	app.toggleToolbarPanel()
	app.toolbar.messageTruncated = false
	if entries := app.toolbarActionPanelEntries(); len(entries) != 2 || entries[1].ID != "toolbar-show-message" {
		t.Fatalf("panel changed after resize: %+v", entries)
	}
	app.actions.hideActionPanel()
	if entries := app.resultActionPanelEntries(); len(entries) != 1 || entries[0].Hotkey != primaryHotkey("t") {
		t.Fatalf("result shortcut not restored: %+v", entries)
	}
}

// TestDelayedToolbarClearPreservesNewMessage covers reused message IDs across plugin changes.
func TestDelayedToolbarClearPreservesNewMessage(t *testing.T) {
	current := true
	app := bindTestApp(&App{lifecycleCtx: context.Background(), toolbar: toolbarController{toolbarMsg: &toolbarMessage{ID: "shared", Title: "new", IsCurrent: func() bool { return current }}}})
	if err := app.ClearToolbarMessage(context.Background(), "shared"); err != nil {
		t.Fatal(err)
	}
	if app.toolbar.toolbarMsg == nil {
		t.Fatal("delayed clear erased the newer message")
	}
	current = false
	if err := app.ClearToolbarMessage(context.Background(), "shared"); err != nil {
		t.Fatal(err)
	}
	if app.toolbar.toolbarMsg != nil {
		t.Fatal("current clear did not remove the message")
	}
}

// TestSystemToolbarMessagePriorityAndRestoration keeps every system and plugin message independently.
func TestSystemToolbarMessagePriorityAndRestoration(t *testing.T) {
	app := bindTestApp(&App{lifecycleCtx: context.Background(), querySession: queryController{editor: woxui.NewTextEditor("")}})
	valid := true
	app.toolbar.applyToolbarMessage(toolbarMessage{ID: "plugin", Title: "Initial", IsCurrent: func() bool { return valid }})
	app.toolbar.applySystemToolbarMessage(toolbarMessage{ID: "warning", Title: "Hotkey failed"}, common.SystemToolbarPriorityWarning, 0)
	app.toolbar.applySystemToolbarMessage(toolbarMessage{ID: "updated", Title: "Updated"}, common.SystemToolbarPriorityConfirmation, 0)
	firstRevision := app.toolbar.systemMessages["updated"].revision
	app.toolbar.applySystemToolbarMessage(toolbarMessage{ID: "updated", Title: "Updated again"}, common.SystemToolbarPriorityConfirmation, 0)
	app.toolbar.expireSystemToolbarMessage("updated", firstRevision)
	app.toolbar.applyToolbarMessage(toolbarMessage{ID: "plugin", Title: "Latest", IsCurrent: func() bool { return valid }})
	if message := app.toolbar.effectiveToolbarMessage(); message == nil || message.Title != "Updated again" {
		t.Fatalf("confirmation lost priority: %+v", message)
	}
	app.toolbar.expireSystemToolbarMessage("updated", app.toolbar.systemMessages["updated"].revision)
	if message := app.toolbar.effectiveToolbarMessage(); message == nil || message.ID != "warning" {
		t.Fatalf("warning not restored: %+v", message)
	}
	app.toolbar.clearSystemToolbarMessage("warning")
	if message := app.toolbar.effectiveToolbarMessage(); message == nil || message.Title != "Latest" {
		t.Fatalf("latest plugin status not restored: %+v", message)
	}
	app.toolbar.applySystemToolbarMessage(toolbarMessage{ID: "warning", Title: "Hotkey failed"}, common.SystemToolbarPriorityWarning, 0)
	valid = false
	app.toolbar.clearSystemToolbarMessage("warning")
	if message := app.toolbar.effectiveToolbarMessage(); message != nil {
		t.Fatalf("stale plugin status restored: %+v", message)
	}
}

// TestSystemToolbarMessageExpires verifies the real timer reveals the lower-priority warning.
func TestSystemToolbarMessageExpires(t *testing.T) {
	app := bindTestApp(&App{lifecycleCtx: context.Background(), querySession: queryController{editor: woxui.NewTextEditor("")}})
	app.toolbar.applySystemToolbarMessage(toolbarMessage{ID: "warning", Title: "Hotkey failed"}, common.SystemToolbarPriorityWarning, 0)
	dispatched := make(chan struct{}, 1)
	app.toolbar.runOnUI = func(_ string, fn func()) error { fn(); dispatched <- struct{}{}; return nil }
	app.toolbar.applySystemToolbarMessage(toolbarMessage{ID: "updated", Title: "Updated"}, common.SystemToolbarPriorityConfirmation, 20*time.Millisecond)
	select {
	case <-dispatched:
	case <-time.After(time.Second):
		t.Fatal("notice timer did not fire")
	}
	if message := app.toolbar.effectiveToolbarMessage(); message == nil || message.ID != "warning" {
		t.Fatalf("warning not restored: %+v", message)
	}
}

// TestSystemMessagePriorityTiesUseMostRecent ensures that clearing one message reveals the others.
func TestSystemMessagePriorityTiesUseMostRecent(t *testing.T) {
	app := bindTestApp(&App{lifecycleCtx: context.Background()})
	app.toolbar.applySystemToolbarMessage(toolbarMessage{ID: "first"}, common.SystemToolbarPriorityWarning, 0)
	app.toolbar.applySystemToolbarMessage(toolbarMessage{ID: "second"}, common.SystemToolbarPriorityWarning, 0)
	if got := app.toolbar.effectiveToolbarMessage(); got == nil || got.ID != "second" {
		t.Fatalf("newest message = %+v", got)
	}
	app.toolbar.clearSystemToolbarMessage("second")
	if got := app.toolbar.effectiveToolbarMessage(); got == nil || got.ID != "first" {
		t.Fatalf("remaining message = %+v", got)
	}
}

// TestSystemNoticeBlocksHiddenPluginShortcuts prevents actions for an obscured toolbar message.
func TestSystemNoticeBlocksHiddenPluginShortcuts(t *testing.T) {
	app := bindTestApp(&App{lifecycleCtx: context.Background(), toolbar: toolbarController{toolbarMsg: &toolbarMessage{ID: "plugin", Actions: []toolbarMessageAction{{ID: "run", Hotkey: "ctrl+k"}}}}})
	app.toolbar.applySystemToolbarMessage(toolbarMessage{ID: "updated"}, common.SystemToolbarPriorityConfirmation, 0)
	if app.toolbar.onToolbarKey(woxui.KeyEvent{Key: woxui.Key("k"), Modifiers: woxui.KeyModifierControl}) {
		t.Fatal("hidden plugin action received its shortcut")
	}
}

// TestPluginClearDoesNotRemoveSystemNotice covers an ID reused by a plugin.
func TestPluginClearDoesNotRemoveSystemNotice(t *testing.T) {
	app := bindTestApp(&App{lifecycleCtx: context.Background(), querySession: queryController{editor: woxui.NewTextEditor("")}})
	app.toolbar.applySystemToolbarMessage(toolbarMessage{ID: "shared", Title: "Updated"}, common.SystemToolbarPriorityConfirmation, 0)
	app.toolbar.applyToolbarMessage(toolbarMessage{ID: "shared", Title: "Plugin"})
	app.toolbar.clearToolbarMessageByID("shared")
	if app.toolbar.toolbarMsg != nil || app.toolbar.effectiveToolbarMessage() == nil || app.toolbar.effectiveToolbarMessage().Title != "Updated" {
		t.Fatalf("plugin clear changed system notice: %+v", app.toolbar.effectiveToolbarMessage())
	}
	app.toolbar.clearSystemToolbarMessage("shared")
	if app.toolbar.effectiveToolbarMessage() != nil {
		t.Fatal("cleared plugin status was restored")
	}
}
