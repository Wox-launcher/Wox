package woxview

import (
	"fmt"
	"strings"
	"testing"

	ui "wox/ui/runtime"
	widget "wox/ui/widget"
)

// selectDocument places a sibling below the trigger so an inline expansion cannot pass unnoticed.
func selectDocument(t *testing.T, disabled bool) *Tree {
	t.Helper()
	return mustParse(t, fmt.Sprintf(`{"version":1,"root":{"type":"column","children":[
		{"type":"select","key":"mode","props":{"label":"Mode","value":"a","disabled":%t,"style":{"width":160},"onChange":{"$handler":"choose"},"options":[{"value":"a","label":"Alpha"},{"value":"b","label":"Beta","disabled":true},{"value":"c","label":"Charlie"},{"value":"d","label":"Delta"}]}},
		{"type":"button","key":"after","props":{"label":"After","onPress":{"$handler":"after"}}}
	]}}`, disabled))
}

// TestSelectPopupKeepsLayoutAndSupportsKeyboard verifies real overlay, navigation, dismissal, and focus restoration.
func TestSelectPopupKeepsLayoutAndSupportsKeyboard(t *testing.T) {
	for _, scale := range []float32{1, 1.5, 2} {
		v := NewView()
		v.SetTree(selectDocument(t, false))
		var events []recordedEvent
		host := layoutHost(v, &events)
		t.Cleanup(host.Dispose)
		render := func() { renderLayout(host, 280, 240, scale) }
		key := func(k ui.Key) {
			if !host.Key(ui.KeyEvent{Key: k, Down: true}) {
				t.Fatalf("unhandled key %s", k)
			}
			render()
		}
		id := "woxview/column0/k:mode/select"
		render()
		before := automationBounds(t, host, "woxview/column0/k:after/button")
		if !host.FocusAutomationID(id) {
			t.Fatal("trigger missing")
		}
		key(ui.KeyEnter)
		if !host.HasOverlay() || automationBounds(t, host, "woxview/column0/k:after/button") != before {
			t.Fatal("popup changed document layout")
		}
		if host.FocusedKey() != widget.Key(id+"/option:YQ") {
			t.Fatalf("initial focus %s", host.FocusedKey())
		}
		key(ui.KeyArrowDown)
		if host.FocusedKey() != widget.Key(id+"/option:Yw") {
			t.Fatalf("disabled row not skipped: %s", host.FocusedKey())
		}
		key(ui.Key("d"))
		key(ui.KeyEnter)
		if host.HasOverlay() || host.FocusedKey() != widget.Key(id) || len(events) != 1 || events[0].args != `["d"]` {
			t.Fatalf("activation/focus: %s %+v", host.FocusedKey(), events)
		}
		key(ui.KeyArrowDown)
		key(ui.KeyHome)
		key(ui.KeyEscape)
		if host.HasOverlay() || len(events) != 1 {
			t.Fatal("Escape selected a value")
		}
		key(ui.KeyEnter)
		host.Pointer(ui.PointerEvent{Kind: ui.PointerDown, Button: ui.PointerButtonPrimary, Position: ui.Point{X: 250, Y: 200}})
		host.Pointer(ui.PointerEvent{Kind: ui.PointerUp, Button: ui.PointerButtonPrimary, Position: ui.Point{X: 250, Y: 200}})
		render()
		if host.HasOverlay() || len(events) != 1 {
			t.Fatal("outside click was not consumed")
		}
		key(ui.KeyEnter)
		v.SetTree(selectDocument(t, true))
		render()
		if host.HasOverlay() {
			t.Fatal("disabled trigger retained open menu")
		}
	}
}

// TestMenuListNavigationAndScrollVisibility keeps active rows reachable in a bounded long list.
func TestMenuListNavigationAndScrollVisibility(t *testing.T) {
	for _, kind := range []string{"list", "select"} {
		options := make([]string, 40)
		for i := range options {
			options[i] = fmt.Sprintf(`{"value":"%d","label":"Item %02d"}`, i, i)
		}
		v := NewView()
		v.SetTree(mustParse(t, `{"version":1,"root":{"type":"`+kind+`","props":{"label":"Items","style":{"width":150,"height":100},"onChange":{"$handler":"change"},"options":[`+strings.Join(options, ",")+`]}}}`))
		var events []recordedEvent
		host := layoutHost(v, &events)
		defer host.Dispose()
		render := func() { renderLayout(host, 220, 180, 1.5) }
		render()
		id := "woxview/" + kind + "0"
		if kind == "select" {
			host.FocusAutomationID(id)
			host.Key(ui.KeyEvent{Key: ui.KeyEnter, Down: true})
			render()
		} else {
			host.FocusAutomationID(id + "/option:MA")
		}
		if !host.Key(ui.KeyEvent{Key: ui.KeyEnd, Down: true}) {
			t.Fatal("End not handled")
		}
		render()
		render()
		last := automationBounds(t, host, id+"/option:Mzk")
		if last.Y < 0 || last.Y+last.Height > 180 {
			t.Fatalf("focused row outside viewport: %+v", last)
		}
		if !host.Key(ui.KeyEvent{Key: ui.KeyEnter, Down: true}) {
			t.Fatal("Enter not handled")
		}
		if len(events) != 1 || events[0].args != `["39"]` {
			t.Fatalf("list selection %+v", events)
		}
	}
}

// TestContextMenuUpdatesHandlersAndPreservesDisabledRows exercises local popup behavior and translation.
func TestContextMenuUpdatesHandlersAndPreservesDisabledRows(t *testing.T) {
	v := NewView()
	tree := func(handler string) *Tree {
		return mustParse(t, fmt.Sprintf(`{"version":1,"root":{"type":"button","key":"row","props":{"label":"Row","onPress":{"$handler":"press"},"contextMenu":[{"label":"i18n:open","icon":"action.execute","onPress":{"$handler":%q}},{"label":"Blocked","disabled":true,"onPress":{"$handler":"blocked"}}]}}}`, handler))
	}
	v.SetTree(tree("old"))
	var events []Event
	translated := "Open item"
	host := widget.NewHost(func(f ui.FrameInfo) widget.Widget {
		return v.Build(BuildProps{Width: f.Size.Width, Height: f.Size.Height, Translate: func(string) string { return translated }, OnNodeEvent: func(e Event) { events = append(events, e) }})
	})
	host.AttachServices(fakeServices{})
	defer host.Dispose()
	render := func() { renderLayout(host, 280, 180, 2) }
	render()
	id := "woxview/k:row/button/context-menu"
	host.FocusAutomationID("woxview/k:row/button")
	host.Pointer(ui.PointerEvent{Kind: ui.PointerDown, Button: ui.PointerButtonSecondary, Position: ui.Point{X: 10, Y: 10}})
	render()
	if !host.HasOverlay() || len(events) != 0 {
		t.Fatal("opening a menu invoked plugin code")
	}
	tap(host, automationBounds(t, host, id+"/item:1"))
	if len(events) != 0 || !host.HasOverlay() {
		t.Fatal("disabled item activated")
	}
	v.SetTree(tree("new"))
	translated = "Updated label"
	render()
	found := false
	for _, n := range host.Snapshot().Tree.Nodes {
		if n.AutomationID == id+"/item:0" && n.Label == translated {
			found = true
		}
	}
	if !found || v.HasHandler("old") || !v.HasHandler("new") {
		t.Fatal("open menu retained stale contract")
	}
	host.Key(ui.KeyEvent{Key: ui.KeyHome, Down: true})
	host.Key(ui.KeyEvent{Key: ui.KeyEnter, Down: true})
	render()
	if host.HasOverlay() || len(events) != 1 || events[0].Handler != "new" || events[0].Key != "row" || string(events[0].Args) != "[]" {
		t.Fatalf("menu events %+v", events)
	}
	if host.FocusedKey() != "woxview/k:row/button" {
		t.Fatalf("focus not restored: %s", host.FocusedKey())
	}
	host.Pointer(ui.PointerEvent{Kind: ui.PointerDown, Button: ui.PointerButtonSecondary, Position: ui.Point{X: 10, Y: 10}})
	render()
	v.SetTree(mustParse(t, `{"version":1,"root":{"type":"text","props":{"text":"gone"}}}`))
	render()
	if host.HasOverlay() {
		t.Fatal("removed owner retained popup")
	}
}

// TestContextMenuRejectsInvalidContracts covers nested callbacks, icons, and required labels.
func TestContextMenuRejectsInvalidContracts(t *testing.T) {
	for _, item := range []string{`{}`, `{"label":"X"}`, `{"label":"","onPress":{"$handler":"go"}}`, `{"label":"X","onPress":null}`, `{"label":"X","onPress":{"$handler":""}}`, `{"label":"X","icon":"missing-icon","onPress":{"$handler":"go"}}`, `{"label":"X","disabled":"true","onPress":{"$handler":"go"}}`} {
		if _, err := Parse([]byte(`{"version":1,"root":{"type":"text","props":{"contextMenu":[` + item + `]}}}`)); err == nil {
			t.Fatalf("accepted %s", item)
		}
	}
}
