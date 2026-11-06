package woxview

import (
	"fmt"
	"strings"
	"testing"

	ui "wox/ui/runtime"
	widget "wox/ui/widget"
)

// lazyForm supplies enough rows to actually unmount the first edited field.
func lazyForm(t *testing.T, revision int, extra string, count int) *Tree {
	t.Helper()
	items := make([]string, count)
	for i := range items {
		items[i] = fmt.Sprintf(`{"type":"text-field","key":"%d","props":{"name":"f%d","revision":%d,"style":{"height":32}}}`, i, i, revision)
	}
	return mustParse(t, `{"version":1,"root":{"type":"form","children":[{"type":"lazy-list","key":"rows","props":{"itemHeight":40,"style":{"width":200,"height":100}`+extra+`},"children":[`+strings.Join(items, ",")+`]}]}}`)
}

// hasAutomation checks absence without failing, so virtualization cannot pass by hiding all rows.
func hasAutomation(host *widget.Host, id string) bool {
	for _, n := range host.Snapshot().Tree.Nodes {
		if n.AutomationID == id {
			return true
		}
	}
	return false
}

// TestLazyListRetainsEditsAndResetsOffscreen covers unmount, stale echo, revision, and removal.
func TestLazyListRetainsEditsAndResetsOffscreen(t *testing.T) {
	for _, scale := range []float32{1, 1.5, 2} {
		v := NewView()
		v.SetTree(lazyForm(t, 0, "", 100))
		var events []recordedEvent
		host := layoutHost(v, &events)
		t.Cleanup(host.Dispose)
		render := func() { renderLayout(host, 250, 150, scale) }
		wheel := func(delta float32) {
			host.Pointer(ui.PointerEvent{Kind: ui.PointerScroll, Position: ui.Point{X: 10, Y: 10}, Scroll: ui.Point{Y: delta}})
			render()
		}
		id := "woxview/form0/k:rows/lazy-list/k:0/text-field"
		render()
		if !host.FocusAutomationID(id + "/field") {
			t.Fatal("first row missing")
		}
		host.TextInput(ui.TextInputEvent{Kind: ui.TextInputCommit, Text: "typed"})
		render()
		wheel(-2000)
		if hasAutomation(host, id+"/field") || len(v.fieldStates) >= 30 {
			t.Fatalf("list did not virtualize: %d fields", len(v.fieldStates))
		}
		v.SetTree(lazyForm(t, 0, "", 100))
		render()
		if v.formValues(v.Tree().Root, "woxview/form0")["f0"] != "typed" {
			t.Fatal("offscreen form lost local value")
		}
		wheel(10000)
		host.FocusAutomationID(id + "/field")
		host.TextInput(ui.TextInputEvent{Kind: ui.TextInputCommit, Text: "!"})
		render()
		if v.fieldValues[id] != "typed!" {
			t.Fatalf("remount lost edit: %v", v.fieldValues[id])
		}
		wheel(-2000)
		v.SetTree(lazyForm(t, 1, "", 100))
		if v.formValues(v.Tree().Root, "woxview/form0")["f0"] != "" {
			t.Fatal("offscreen reset not applied to form")
		}
		render()
		wheel(10000)
		host.FocusAutomationID(id + "/field")
		host.TextInput(ui.TextInputEvent{Kind: ui.TextInputCommit, Text: "new"})
		render()
		if v.fieldValues[id] != "new" {
			t.Fatal("reset not restored when remounted")
		}
		v.SetTree(lazyForm(t, 1, "", 0))
		if len(v.fieldStates) != 0 || len(v.fieldValues) != 0 {
			t.Fatal("removed rows retained document state")
		}
	}
}

// TestLazyListTailAndEndEvents exercises append, pause, resumption, and display transitions.
func TestLazyListTailAndEndEvents(t *testing.T) {
	v := NewView()
	extra := `,"stickToEnd":true,"onEndReached":{"$handler":"more"}`
	v.SetTree(lazyForm(t, 0, extra, 100))
	var events []recordedEvent
	host := layoutHost(v, &events)
	defer host.Dispose()
	render := func(scale float32) { renderLayout(host, 250, 150, scale); renderLayout(host, 250, 150, scale) }
	render(1.5)
	last := "woxview/form0/k:rows/lazy-list/k:99/text-field/field"
	if !hasAutomation(host, last) || len(events) != 1 {
		t.Fatalf("initial tail: %v", events)
	}
	host.Pointer(ui.PointerEvent{Kind: ui.PointerScroll, Position: ui.Point{X: 10, Y: 10}, Scroll: ui.Point{Y: 1000}})
	render(2)
	v.SetTree(lazyForm(t, 0, extra, 110))
	render(2)
	if hasAutomation(host, "woxview/form0/k:rows/lazy-list/k:109/text-field/field") || len(events) != 1 {
		t.Fatal("append moved a reader away from their position")
	}
	host.Pointer(ui.PointerEvent{Kind: ui.PointerScroll, Position: ui.Point{X: 10, Y: 10}, Scroll: ui.Point{Y: -10000}})
	render(1)
	if len(events) != 2 {
		t.Fatalf("tail return events: %v", events)
	}
	v.SetTree(lazyForm(t, 0, extra, 111))
	render(1.5)
	if !hasAutomation(host, "woxview/form0/k:rows/lazy-list/k:110/text-field/field") || len(events) != 3 {
		t.Fatal("tail append did not follow and notify")
	}
}

// TestLazyListRejectsInvalidContracts verifies the fixed-extent and stable-key requirements.
func TestLazyListRejectsInvalidContracts(t *testing.T) {
	for _, raw := range []string{
		`{"type":"lazy-list"}`,
		`{"type":"lazy-list","props":{"itemHeight":0}}`,
		`{"type":"lazy-list","props":{"itemHeight":-1}}`,
		`{"type":"lazy-list","props":{"itemHeight":16385}}`,
		`{"type":"lazy-list","props":{"itemHeight":"40"}}`,
		`{"type":"lazy-list","props":{"itemHeight":40},"children":[{"type":"text"}]}`,
		`{"type":"lazy-list","props":{"itemHeight":40},"children":[{"type":"text","key":"a"},{"type":"text","key":"a"}]}`,
		`{"type":"lazy-list","props":{"itemHeight":40,"onEndReached":{"$handler":""}}}`,
	} {
		if _, err := Parse([]byte(`{"version":1,"root":` + raw + `}`)); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
}
