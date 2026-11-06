package woxview

import (
	"runtime"
	"testing"

	ui "wox/ui/runtime"
	widget "wox/ui/widget"
)

// TestShortcutScopes isolates siblings, gives the nearest scope precedence, and supports no-focus window shortcuts.
func TestShortcutScopes(t *testing.T) {
	v := NewView()
	v.SetTree(mustParse(t, `{"version":1,"root":{"type":"column","key":"window","props":{"shortcuts":[{"key":"mod+enter","onPress":{"$handler":"global"}}]},"children":[
		{"type":"column","key":"group","props":{"shortcuts":[{"key":"mod+enter","onPress":{"$handler":"local"}}]},"children":[{"type":"text-field","key":"editor","props":{"label":"Editor"}}]},
		{"type":"button","key":"sibling","props":{"label":"Sibling","onPress":{"$handler":"press"}}},
		{"type":"select","key":"select","props":{"label":"Mode","options":[{"value":"a","label":"A"}]}}
	]}}`))
	var events []Event
	host := widget.NewHost(func(f ui.FrameInfo) widget.Widget {
		return v.Build(BuildProps{Width: f.Size.Width, Height: f.Size.Height, OnNodeEvent: func(e Event) { events = append(events, e) }})
	})
	host.AttachServices(fakeServices{})
	defer host.Dispose()
	render := func() { renderLayout(host, 300, 240, 1.5) }
	render()
	modifier := ui.KeyModifierControl
	if runtime.GOOS == "darwin" {
		modifier = ui.KeyModifierMeta
	}
	shortcut := ui.KeyEvent{Key: ui.KeyEnter, Modifiers: modifier, Down: true}
	if !host.Key(shortcut) || len(events) != 1 || events[0].Handler != "global" {
		t.Fatal("window shortcut required focus")
	}
	host.FocusAutomationID("woxview/k:window/column/k:group/column/k:editor/text-field/field")
	if !host.Key(shortcut) || len(events) != 2 || events[1].Handler != "local" || events[1].Key != "group" {
		t.Fatalf("local precedence %+v", events)
	}
	host.FocusAutomationID("woxview/k:window/column/k:sibling/button")
	host.Key(shortcut)
	if len(events) != 3 || events[2].Handler != "global" {
		t.Fatal("sibling scope leaked")
	}
	host.FocusAutomationID("woxview/k:window/column/k:group/column/k:editor/text-field/field")
	composing := shortcut
	composing.Composing = true
	host.Key(composing)
	up := shortcut
	up.Down = false
	host.Key(up)
	if len(events) != 3 {
		t.Fatal("composition/key-up triggered shortcut")
	}
	host.FocusAutomationID("woxview/k:window/column/k:select/select")
	host.Key(ui.KeyEvent{Key: ui.KeyEnter, Down: true})
	render()
	host.Key(shortcut)
	if len(events) != 3 {
		t.Fatal("modal allowed background shortcut")
	}
	host.Key(ui.KeyEvent{Key: ui.KeyEscape, Down: true})
	render()
	if !v.HasHandler("local") || !v.HasHandler("global") {
		t.Fatal("shortcut handler omitted from ownership")
	}
}

// TestShortcutPlatformAlias checks both platform branches regardless of the test machine.
func TestShortcutPlatformAlias(t *testing.T) {
	for _, platform := range []string{"windows", "linux", "darwin"} {
		combo, ok := parseShortcut("mod+shift+enter", platform)
		want := ui.KeyModifierControl | ui.KeyModifierShift
		if platform == "darwin" {
			want = ui.KeyModifierMeta | ui.KeyModifierShift
		}
		if !ok || combo.Key != ui.KeyEnter || combo.Modifiers != want {
			t.Fatalf("%s: %+v", platform, combo)
		}
	}
}

// TestShortcutRejectsInvalidContracts verifies strict input and equivalent-key duplicate detection.
func TestShortcutRejectsInvalidContracts(t *testing.T) {
	for _, items := range []string{`[{}]`, `[{"key":"ctrl","onPress":{"$handler":"x"}}]`, `[{"key":"ctrl+enter"}]`, `[{"key":"ctrl+enter","onPress":{"$handler":""}}]`, `[{"key":"ctrl+enter","onPress":{"$handler":"x"}},{"key":"control+return","onPress":{"$handler":"y"}}]`, `[{"key":"hold:ctrl+a","onPress":{"$handler":"x"}}]`, `[{"key":"left_ctrl+a","onPress":{"$handler":"x"}}]`} {
		if _, err := Parse([]byte(`{"version":1,"root":{"type":"text","props":{"shortcuts":` + items + `}}}`)); err == nil {
			t.Fatalf("accepted %s", items)
		}
	}
}
