package launcher

import (
	"testing"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func newSettingsWindowKeyTestApp() *App {
	return bindTestApp(&App{
		settingsWindow: settingsWindowController{open: true, tab: "general"},
		hotkeySettings: newHotkeySettingsController(CommonDeps{}),
	})
}

// TestSettingsEscapeClosesWindow covers Escape while an ordinary text field owns focus.
func TestSettingsEscapeClosesWindow(t *testing.T) {
	app := newSettingsWindowKeyTestApp()
	closed := 0
	app.settingsWindow.requestSettingsClose = func() { closed++ }
	host := woxwidget.NewHost(func(woxui.FrameInfo) woxwidget.Widget {
		return woxcomponent.WoxTextField(woxcomponent.TextFieldProps{
			ID: "settings-field", Label: "Setting", Width: 200, Height: 40, Autofocus: true,
			Controller: woxwidget.NewTextEditingController("value"),
		})
	})
	host.AttachServices(formTableHostServices{})
	defer host.Dispose()
	app.settingsWindow.host = host
	host.Frame(&woxui.DisplayList{}, woxui.FrameInfo{Size: woxui.Size{Width: 300, Height: 200}, Scale: 1})
	if !host.HasFocus("settings-field") {
		t.Fatal("text field did not receive focus")
	}
	if !app.settingsWindow.onKey(woxui.KeyEvent{Key: woxui.KeyEscape, Down: true}) || closed != 1 {
		t.Fatalf("close requests = %d, want 1", closed)
	}
}

// TestSettingsEscapeDismissesDialogsBeforeWindow exercises the complete window dispatch with nested modals.
func TestSettingsEscapeDismissesDialogsBeforeWindow(t *testing.T) {
	app := newSettingsWindowKeyTestApp()
	closed, dialogs := 0, 2
	app.settingsWindow.requestSettingsClose = func() { closed++ }
	host := woxwidget.NewHost(func(woxui.FrameInfo) woxwidget.Widget {
		var child woxwidget.Widget = woxwidget.Container{Width: 80, Height: 80}
		if dialogs == 2 {
			child = woxcomponent.WoxDialog(woxcomponent.DialogProps{
				ID: "inner", Width: 100, Height: 100, Child: child,
				OnEscape: func() { dialogs = 1 },
			})
		}
		if dialogs > 0 {
			child = woxcomponent.WoxDialog(woxcomponent.DialogProps{
				ID: "outer", Width: 200, Height: 200, Child: child,
				OnEscape: func() { dialogs = 0 },
			})
		}
		return child
	})
	host.AttachServices(formTableHostServices{})
	defer host.Dispose()
	app.settingsWindow.host = host
	frame := woxui.FrameInfo{Size: woxui.Size{Width: 300, Height: 300}, Scale: 1}
	escape := woxui.KeyEvent{Key: woxui.KeyEscape, Down: true}
	for remaining := 1; remaining >= 0; remaining-- {
		host.Frame(&woxui.DisplayList{}, frame)
		if !app.settingsWindow.onKey(escape) || dialogs != remaining || closed != 0 {
			t.Fatalf("dialogs = %d, close requests = %d; want %d and 0", dialogs, closed, remaining)
		}
		host.Frame(&woxui.DisplayList{}, frame)
		repeat := escape
		repeat.Repeat = true
		if !app.settingsWindow.onKey(repeat) || dialogs != remaining || closed != 0 {
			t.Fatal("held Escape dismissed another surface")
		}
	}
	if !app.settingsWindow.onKey(escape) || closed != 1 {
		t.Fatalf("close requests = %d, want 1 after both dialogs are cancelled", closed)
	}
}

// TestSettingsEscapeCancelsPageInteractionBeforeWindow protects controller-owned overlays and drafts.
func TestSettingsEscapeCancelsPageInteractionBeforeWindow(t *testing.T) {
	for _, name := range []string{"choice", "editor", "search", "table"} {
		t.Run(name, func(t *testing.T) {
			app := newSettingsWindowKeyTestApp()
			closed := 0
			app.settingsWindow.requestSettingsClose = func() { closed++ }
			var active func() bool
			switch name {
			case "choice":
				app.generalSettings.SetChoicePicker(&settingChoicePickerState{})
				active = func() bool { return app.generalSettings.ChoicePicker() != nil }
			case "editor":
				app.generalSettings.StartEdit("HttpProxyUrl", "unsaved", -1)
				active = func() bool { return app.generalSettings.EditKey() != "" }
			case "search":
				app.settingsSearch.SetEditor(woxwidget.NewTextEditingController("query"))
				app.settingsSearch.SetFocused(true)
				app.settingsSearch.SetPanel(true)
				active = func() bool { return app.settingsSearch.Editor().Text() != "" }
			case "table":
				fields := newFormFieldsState(nil, nil, true)
				app.generalSettings.SetForm(&fields)
				app.formTables.settingsEditor = &formTableEditorState{target: &fields, deletePending: -1}
				active = func() bool { return app.formTables.settingsEditor != nil }
			}
			escape := woxui.KeyEvent{Key: woxui.KeyEscape, Down: true}
			if !app.settingsWindow.onKey(escape) || active() || closed != 0 {
				t.Fatalf("active = %t, close requests = %d; want false and 0", active(), closed)
			}
			if !app.settingsWindow.onKey(escape) || closed != 1 {
				t.Fatalf("close requests = %d, want 1 after cancelling %s", closed, name)
			}
		})
	}
}

// TestSettingsEscapeLeavesOtherKeyEventsAlone protects composition and modifier combinations.
func TestSettingsEscapeLeavesOtherKeyEventsAlone(t *testing.T) {
	app := newSettingsWindowKeyTestApp()
	closed := 0
	app.settingsWindow.requestSettingsClose = func() { closed++ }
	for _, event := range []woxui.KeyEvent{
		{Key: woxui.KeyEscape},
		{Key: woxui.KeyEscape, Down: true, Composing: true},
		{Key: woxui.KeyEscape, Down: true, Modifiers: woxui.KeyModifierControl},
		{Key: "a", Down: true},
	} {
		if app.settingsWindow.onKey(event) || closed != 0 {
			t.Fatalf("unexpected handling of %+v; close requests = %d", event, closed)
		}
	}
}
