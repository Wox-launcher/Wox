package woxview

import (
	"encoding/json"
	"strings"
	"testing"

	component "wox/ui/launcher/component"
	ui "wox/ui/runtime"
	widget "wox/ui/widget"
)

// TestTooltipKeepsInputAndTracksLocale tests window-level placement and trigger ownership.
func TestTooltipKeepsInputAndTracksLocale(t *testing.T) {
	v := NewView()
	v.SetTree(mustParse(t, `{"version":1,"root":{"type":"stack","children":[{"type":"button","key":"action","props":{"label":"Go","tooltip":"i18n:hint","onPress":{"$handler":"go"},"style":{"top":130,"left":130,"width":60}}}]}}`))
	translated := "First hint"
	var events []Event
	theme := component.ControlTheme{Surface: ui.Color{A: 255}, Text: ui.Color{R: 255, A: 255}}
	host := widget.NewHost(func(f ui.FrameInfo) widget.Widget {
		return v.Build(BuildProps{Width: f.Size.Width, Height: f.Size.Height, Theme: theme, Translate: func(string) string { return translated }, OnNodeEvent: func(e Event) { events = append(events, e) }})
	})
	host.AttachServices(fakeServices{})
	defer host.Dispose()
	id := "woxview/stack0/k:action/button"
	for _, scale := range []float32{1, 1.5, 2} {
		render := func() { renderLayout(host, 200, 180, scale) }
		render()
		before := automationBounds(t, host, id)
		host.Pointer(ui.PointerEvent{Kind: ui.PointerMove, Position: ui.Point{X: before.X + 10, Y: before.Y + 10}})
		render()
		hint := automationBounds(t, host, id+"/tooltip/hint")
		if !host.HasOverlay() || hint.X < 0 || hint.X+hint.Width > 200 || hint.Y+hint.Height > before.Y {
			t.Fatalf("tooltip failed edge flip at scale %v: %+v, trigger %+v", scale, hint, before)
		}
		tap(host, before)
		if len(events) == 0 || events[len(events)-1].Key != "action" || events[len(events)-1].Handler != "go" {
			t.Fatal("tooltip stole click or event lost key")
		}
		render()
		if after := automationBounds(t, host, id); after != before {
			t.Fatalf("tooltip changed layout: %+v -> %+v", before, after)
		}
		translated = "Translated hint"
		theme.Text = ui.Color{G: 255, A: 255}
		render()
		render()
		found := false
		for _, n := range host.Snapshot().Tree.Nodes {
			if n.AutomationID == id+"/tooltip/hint" && n.Label == translated {
				found = true
			}
		}
		if !found {
			t.Fatal("tooltip retained old translation")
		}
		host.Pointer(ui.PointerEvent{Kind: ui.PointerLeave})
		render()
		if host.HasOverlay() {
			t.Fatal("tooltip remained after leave")
		}
	}
	host.Pointer(ui.PointerEvent{Kind: ui.PointerMove, Position: ui.Point{X: 140, Y: 140}})
	renderLayout(host, 200, 180, 1)
	v.SetTree(mustParse(t, `{"version":1,"root":{"type":"text","props":{"text":"done"}}}`))
	renderLayout(host, 200, 180, 1)
	if host.HasOverlay() {
		t.Fatal("removed trigger retained tooltip")
	}
}

// TestMarkdownLinkEventOverridesExternalOpen exercises an actual link hit and additive metadata.
func TestMarkdownLinkEventOverridesExternalOpen(t *testing.T) {
	for _, handler := range []bool{false, true} {
		v := NewView()
		extra := ""
		if handler {
			extra = `,"onLinkPress":{"$handler":"link"}`
		}
		v.SetTree(mustParse(t, `{"version":1,"root":{"type":"markdown","key":"docs","props":{"content":"[Guide](https://example.com/guide)"`+extra+`}}}`))
		var events []Event
		opened := ""
		host := widget.NewHost(func(f ui.FrameInfo) widget.Widget {
			return v.Build(BuildProps{Width: f.Size.Width, Height: f.Size.Height, OnNodeEvent: func(e Event) { events = append(events, e) }, OnOpenLink: func(url string) { opened = url }})
		})
		host.AttachServices(fakeServices{})
		renderLayout(host, 300, 100, 1.5)
		found := false
		for _, n := range host.Snapshot().Tree.Nodes {
			if n.Role == ui.AccessibilityRoleLink {
				tap(host, n.Bounds)
				found = true
				break
			}
		}
		if !found {
			t.Fatal("link missing")
		}
		if handler {
			if opened != "" || len(events) != 1 || events[0].Key != "docs" || events[0].Handler != "link" || string(events[0].Args) != `["https://example.com/guide"]` {
				t.Fatalf("event %+v, opened %s", events, opened)
			}
			data, _ := json.Marshal(events[0])
			if !strings.Contains(string(data), `"Key":"docs"`) {
				t.Fatal("key absent from wire")
			}
		} else if opened != "https://example.com/guide" || len(events) != 0 {
			t.Fatalf("default open failed: %s", opened)
		}
		host.Dispose()
	}
}

// TestParseRejectsInvalidInteractionProps keeps the schema's string and handler shapes strict.
func TestParseRejectsInvalidInteractionProps(t *testing.T) {
	for _, root := range []string{
		`{"type":"text","props":{"tooltip":42}}`,
		`{"type":"markdown","props":{"onLinkPress":{"$handler":""}}}`,
		`{"type":"markdown","props":{"onLinkPress":"link"}}`,
		`{"type":"button","props":{"onDoubleTap":{"$handler":""}}}`,
		`{"type":"text","props":{"onHoverChange":false}}`,
		`{"type":"text-field","props":{"onFocusChange":"focus"}}`,
		`{"type":"view","props":{"onContextMenu":{"$handler":"menu","extra":1}}}`,
	} {
		if _, err := Parse([]byte(`{"version":1,"root":` + root + `}`)); err == nil {
			t.Fatalf("accepted %s", root)
		}
	}
}

// TestCommonEventsObserveControlsWithoutReplacingTheirActions exercises nested hit and focus routing.
func TestCommonEventsObserveControlsWithoutReplacingTheirActions(t *testing.T) {
	v := NewView()
	v.SetTree(mustParse(t, `{"version":1,"root":{"type":"column","props":{"onHoverChange":{"$handler":"hover"}},"children":[
		{"type":"button","key":"go","props":{"label":"Go","onPress":{"$handler":"press"},"onDoubleTap":{"$handler":"double"},"onContextMenu":{"$handler":"menu"},"onFocusChange":{"$handler":"focus"}}},
		{"type":"text-field","key":"edit","props":{"label":"Edit","onFocusChange":{"$handler":"editFocus"}}}
	]}}`))
	var events []Event
	host := widget.NewHost(func(f ui.FrameInfo) widget.Widget {
		return v.Build(BuildProps{Width: f.Size.Width, Height: f.Size.Height, OnNodeEvent: func(e Event) { events = append(events, e) }})
	})
	host.AttachServices(fakeServices{})
	defer host.Dispose()
	render := func() { renderLayout(host, 300, 200, 1.5) }
	render()
	bounds := automationBounds(t, host, "woxview/column0/k:go/button")
	host.Pointer(ui.PointerEvent{Kind: ui.PointerMove, Position: ui.Point{X: bounds.X + 5, Y: bounds.Y + 5}})
	tap(host, bounds)
	tap(host, bounds)
	render()
	host.Pointer(ui.PointerEvent{Kind: ui.PointerDown, Button: ui.PointerButtonSecondary, Position: ui.Point{X: bounds.X + 5, Y: bounds.Y + 5}})
	if !host.FocusAutomationID("woxview/column0/k:edit/text-field/field") {
		t.Fatal("lost child focus identity")
	}
	render()
	host.TextInput(ui.TextInputEvent{Kind: ui.TextInputCommit, Text: "still editable"})
	host.ClearFocus()
	host.Pointer(ui.PointerEvent{Kind: ui.PointerLeave})
	render()
	counts := map[HandlerID]int{}
	var focusArgs, editArgs []string
	for _, e := range events {
		counts[e.Handler]++
		if e.Handler == "focus" {
			focusArgs = append(focusArgs, string(e.Args))
		}
		if e.Handler == "editFocus" {
			editArgs = append(editArgs, string(e.Args))
		}
		if e.Handler == "double" || e.Handler == "menu" {
			if e.Key != "go" || string(e.Args) != "[]" {
				t.Fatalf("wrong event %+v", e)
			}
		}
	}
	if counts["press"] != 2 || counts["double"] != 1 || counts["menu"] != 1 || counts["hover"] != 2 {
		t.Fatalf("events: %+v", events)
	}
	if strings.Join(focusArgs, ",") != "[true],[false]" || strings.Join(editArgs, ",") != "[true],[false]" {
		t.Fatalf("focus events %+v", events)
	}
	if v.fieldValues["woxview/column0/k:edit/text-field"] != "still editable" {
		t.Fatal("observation broke editing")
	}
	for _, id := range []HandlerID{"press", "double", "menu", "hover", "focus", "editFocus"} {
		if !v.HasHandler(id) {
			t.Fatalf("missing common handler %s", id)
		}
	}
}
