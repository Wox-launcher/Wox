package woxview

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"

	woxui "wox/ui/runtime"
	widget "wox/ui/widget"
)

// renderLayout exercises logical geometry against a non-unit physical display scale.
func renderLayout(host *widget.Host, width, height, scale float32) {
	host.Frame(&woxui.DisplayList{}, woxui.FrameInfo{Size: woxui.Size{Width: width, Height: height}, PixelSize: woxui.PixelSize{Width: int(width * scale), Height: int(height * scale)}, Scale: scale})
}

// layoutHost renders a view without tying layout tests to the common fixture's fixed size.
func layoutHost(v *View, events *[]recordedEvent) *widget.Host {
	host := widget.NewHost(func(info woxui.FrameInfo) widget.Widget {
		return v.Build(BuildProps{Width: info.Size.Width, Height: info.Size.Height, OnEvent: func(id HandlerID, args json.RawMessage) { *events = append(*events, recordedEvent{id, string(args)}) }})
	})
	host.AttachServices(fakeServices{})
	return host
}

// TestRowWrapReflowsAndKeepsKeyedFields exercises wrapping, resizing, and local field retention.
func TestRowWrapReflowsAndKeepsKeyedFields(t *testing.T) {
	for _, scale := range []float32{1, 1.5, 2} {
		v := NewView()
		v.SetTree(mustParse(t, `{"version":1,"root":{"type":"row","props":{"wrap":true,"style":{"gap":8}},"children":[
			{"type":"text-field","key":"a","props":{"label":"A","value":"","style":{"width":100}}},
			{"type":"text-field","key":"b","props":{"label":"B","style":{"width":100}}},
			{"type":"text-field","key":"c","props":{"label":"C","style":{"width":100}}}
		]}}`))
		var events []recordedEvent
		host := layoutHost(v, &events)
		t.Cleanup(host.Dispose)
		renderLayout(host, 220, 200, scale)
		first := automationBounds(t, host, "woxview/row0/k:a/text-field/field")
		second := automationBounds(t, host, "woxview/row0/k:b/text-field/field")
		third := automationBounds(t, host, "woxview/row0/k:c/text-field/field")
		if first.Width != 100 || second.X != 108 || second.Y != first.Y || third.X != 0 || third.Y != first.Height+8 {
			t.Fatalf("scale %v: wrap bounds %+v %+v %+v", scale, first, second, third)
		}
		host.FocusAutomationID("woxview/row0/k:c/text-field/field")
		host.TextInput(woxui.TextInputEvent{Kind: woxui.TextInputCommit, Text: "typed"})
		renderLayout(host, 340, 200, scale)
		third = automationBounds(t, host, "woxview/row0/k:c/text-field/field")
		if third.X != 216 || third.Y != 0 || v.fieldValues["woxview/row0/k:c/text-field"] != "typed" {
			t.Fatalf("resize lost layout or state: %+v, %+v", third, v.fieldValues)
		}
	}
}

// TestGridMeasuresRowsAndColumns checks asymmetric cell heights, gaps, padding, and DPI transitions.
func TestGridMeasuresRowsAndColumns(t *testing.T) {
	v := NewView()
	v.SetTree(mustParse(t, `{"version":1,"root":{"type":"grid","props":{"columns":2,"gap":10,"style":{"padding":10}},"children":[
		{"type":"view","key":"a","props":{"label":"A","onPress":{"$handler":"a"},"style":{"height":20}}},
		{"type":"view","key":"b","props":{"label":"B","onPress":{"$handler":"b"},"style":{"height":40}}},
		{"type":"view","key":"c","props":{"label":"C","onPress":{"$handler":"c"},"style":{"height":30}}}
	]}}`))
	var events []recordedEvent
	host := layoutHost(v, &events)
	t.Cleanup(host.Dispose)
	for _, scale := range []float32{1, 1.5, 2, 1} {
		renderLayout(host, 230, 200, scale)
		a := automationBounds(t, host, "woxview/grid0/k:a/view#press")
		b := automationBounds(t, host, "woxview/grid0/k:b/view#press")
		c := automationBounds(t, host, "woxview/grid0/k:c/view#press")
		if a != (woxui.Rect{X: 10, Y: 10, Width: 100, Height: 20}) || b != (woxui.Rect{X: 120, Y: 10, Width: 100, Height: 40}) || c != (woxui.Rect{X: 10, Y: 60, Width: 100, Height: 30}) {
			t.Fatalf("scale %v: grid bounds %+v %+v %+v", scale, a, b, c)
		}
		tap(host, c)
		if events[len(events)-1].handler != "c" {
			t.Fatal("grid hit testing used physical coordinates")
		}
	}
}

// scrollTree supplies content whose exact trailing edge is observable through a pressable final item.
func scrollTree(t *testing.T, extra string, count int) *Tree {
	t.Helper()
	children := make([]string, count)
	for i := range children {
		children[i] = fmt.Sprintf(`{"type":"view","key":"%d","props":{"label":"item","style":{"width":40,"height":40},"onPress":{"$handler":"item"}}}`, i)
	}
	return mustParse(t, `{"version":1,"root":{"type":"scroll","props":{"style":{"width":100,"height":100},`+extra+`},"children":[`+strings.Join(children, ",")+`]}}`)
}

// TestScrollEndReachedFiresOncePerVisit checks input routing, threshold crossing, appends, and listener replacement.
func TestScrollEndReachedFiresOncePerVisit(t *testing.T) {
	for _, horizontal := range []bool{false, true} {
		for _, scale := range []float32{1, 1.5, 2} {
			v := NewView()
			extra := fmt.Sprintf(`"horizontal":%t,"endThreshold":10,"onEndReached":{"$handler":"more"}`, horizontal)
			v.SetTree(scrollTree(t, extra, 10))
			var events []recordedEvent
			host := layoutHost(v, &events)
			t.Cleanup(host.Dispose)
			render := func() { renderLayout(host, 200, 200, scale) }
			wheel := func(delta float32) {
				host.Pointer(woxui.PointerEvent{Kind: woxui.PointerScroll, Position: woxui.Point{X: 10, Y: 10}, Scroll: woxui.Point{Y: delta}})
				render()
			}
			render()
			render()
			if len(events) != 0 {
				t.Fatalf("initial scroll sent events: %+v", events)
			}
			wheel(-280)
			if len(events) != 0 {
				t.Fatal("end event before threshold")
			}
			wheel(-10)
			if len(events) != 1 || events[0] != (recordedEvent{"more", "[]"}) {
				t.Fatalf("missing end event: %+v", events)
			}
			render()
			render()
			wheel(-10)
			if len(events) != 1 {
				t.Fatal("repeated frames/scroll spammed end event")
			}
			wheel(40)
			wheel(-40)
			if len(events) != 2 {
				t.Fatal("returning to end did not rearm")
			}
			v.SetTree(scrollTree(t, extra, 12))
			render()
			if len(events) != 2 {
				t.Fatal("append incorrectly reported end while far away")
			}
			wheel(-80)
			if len(events) != 3 {
				t.Fatal("new content end did not notify")
			}
			v.SetTree(scrollTree(t, strings.ReplaceAll(extra, "more", "next"), 12))
			render()
			if len(events) != 4 || events[3].handler != "next" || v.HasHandler("more") || !v.HasHandler("next") {
				t.Fatalf("handler replacement: %+v", events)
			}
		}
	}
}

// TestScrollFollowsTailAndLetsReadersScrollAway verifies the WoxView-to-widget stickToEnd mapping.
func TestScrollFollowsTailAndLetsReadersScrollAway(t *testing.T) {
	for _, horizontal := range []bool{false, true} {
		v := NewView()
		extra := fmt.Sprintf(`"horizontal":%t,"stickToEnd":true,"onEndReached":{"$handler":"more"}`, horizontal)
		v.SetTree(scrollTree(t, extra, 10))
		var events []recordedEvent
		host := layoutHost(v, &events)
		t.Cleanup(host.Dispose)
		render := func() { renderLayout(host, 200, 200, 1.5); renderLayout(host, 200, 200, 1.5) }
		render()
		last := automationBounds(t, host, "woxview/scroll0/k:9/view#press")
		end := last.Y + last.Height
		if horizontal {
			end = last.X + last.Width
		}
		if end != 100 || len(events) != 1 {
			t.Fatalf("did not start at end: %+v, %+v", last, events)
		}
		v.SetTree(scrollTree(t, extra, 12))
		render()
		if len(events) != 2 {
			t.Fatalf("append notification: %+v", events)
		}
		host.Pointer(woxui.PointerEvent{Kind: woxui.PointerScroll, Position: woxui.Point{X: 10, Y: 10}, Scroll: woxui.Point{Y: 80}})
		render()
		before := automationBounds(t, host, "woxview/scroll0/k:9/view#press")
		v.SetTree(scrollTree(t, extra, 14))
		render()
		after := automationBounds(t, host, "woxview/scroll0/k:9/view#press")
		if before != after || len(events) != 2 {
			t.Fatalf("append displaced reader: %+v -> %+v, %+v", before, after, events)
		}
	}
}

// TestShortScrollRequestsMoreOnlyOnce prevents undersized or empty content from creating an event loop.
func TestShortScrollRequestsMoreOnlyOnce(t *testing.T) {
	v := NewView()
	v.SetTree(scrollTree(t, `"onEndReached":{"$handler":"more"}`, 0))
	var events []recordedEvent
	host := layoutHost(v, &events)
	t.Cleanup(host.Dispose)
	for range 4 {
		renderLayout(host, 200, 200, 1)
	}
	if len(events) != 1 {
		t.Fatalf("empty scroll events: %+v", events)
	}
	v.SetTree(scrollTree(t, `"onEndReached":{"$handler":"more"}`, 1))
	renderLayout(host, 200, 200, 1)
	if len(events) != 2 {
		t.Fatal("added content that still fits must request another page")
	}
	v.SetTree(scrollTree(t, `"onEndReached":{"$handler":"more"}`, 1))
	for range 3 {
		renderLayout(host, 200, 200, 1)
	}
	if len(events) != 2 {
		t.Fatal("identical content repeatedly requested more")
	}
}

// TestNestedHorizontalScrollPreservesVerticalWheel prevents an inner strip from trapping its outer list.
func TestNestedHorizontalScrollPreservesVerticalWheel(t *testing.T) {
	v := NewView()
	v.SetTree(mustParse(t, `{"version":1,"root":{"type":"scroll","props":{"style":{"width":100,"height":100}},"children":[
		{"type":"scroll","key":"strip","props":{"horizontal":true,"style":{"width":100,"height":40}},"children":[
			{"type":"view","key":"first","props":{"style":{"width":80,"height":40},"onPress":{"$handler":"first"}}},
			{"type":"view","props":{"style":{"width":80,"height":40}}}
		]},
		{"type":"view","props":{"style":{"height":200}}}
	]}}`))
	var events []recordedEvent
	host := layoutHost(v, &events)
	t.Cleanup(host.Dispose)
	render := func() { renderLayout(host, 200, 200, 2); renderLayout(host, 200, 200, 2) }
	render()
	const id = "woxview/scroll0/k:strip/scroll/k:first/view#press"
	before := automationBounds(t, host, id)
	host.Pointer(woxui.PointerEvent{Kind: woxui.PointerScroll, Position: woxui.Point{X: 10, Y: 10}, Scroll: woxui.Point{X: -30}})
	render()
	afterHorizontal := automationBounds(t, host, id)
	if afterHorizontal.X != before.X-30 || afterHorizontal.Y != before.Y {
		t.Fatalf("horizontal wheel moved wrong axis: %+v -> %+v", before, afterHorizontal)
	}
	host.Pointer(woxui.PointerEvent{Kind: woxui.PointerScroll, Position: woxui.Point{X: 10, Y: 10}, Scroll: woxui.Point{Y: -20}})
	render()
	afterVertical := automationBounds(t, host, id)
	if afterVertical.X != afterHorizontal.X || afterVertical.Y != afterHorizontal.Y-20 {
		t.Fatalf("vertical wheel trapped by strip: %+v -> %+v", afterHorizontal, afterVertical)
	}
}

// TestScrollAxisChangeReplacesMeasuredGeometry rejects stale viewport dimensions from the old axis.
func TestScrollAxisChangeReplacesMeasuredGeometry(t *testing.T) {
	v := NewView()
	data := func(horizontal bool) string {
		return fmt.Sprintf(`{"version":1,"root":{"type":"scroll","props":{"horizontal":%t,"stickToEnd":true,"style":{"width":100,"height":60}},"children":[
		{"type":"view","props":{"style":{"width":120,"height":100}}},
		{"type":"view","key":"last","props":{"style":{"width":120,"height":100},"onPress":{"$handler":"last"}}}
	]}}`, horizontal)
	}
	var events []recordedEvent
	host := layoutHost(v, &events)
	t.Cleanup(host.Dispose)
	for _, horizontal := range []bool{false, true, false} {
		v.SetTree(mustParse(t, data(horizontal)))
		for range 3 {
			renderLayout(host, 200, 200, 1.5)
		}
		last := automationBounds(t, host, "woxview/scroll0/k:last/view#press")
		if horizontal {
			if last.X+last.Width != 100 {
				t.Fatalf("horizontal viewport retained vertical extent: %+v", last)
			}
		} else if last.Y+last.Height != 60 {
			t.Fatalf("vertical viewport retained horizontal extent: %+v", last)
		}
	}
}

// TestLayoutContractsRejectInvalidInputs keeps unsupported layout props from being silently ignored.
func TestLayoutContractsRejectInvalidInputs(t *testing.T) {
	for _, root := range []string{
		`{"type":"column","props":{"wrap":true}}`, `{"type":"row","props":{"wrap":"yes"}}`,
		`{"type":"grid"}`, `{"type":"grid","props":{"columns":0}}`, `{"type":"grid","props":{"columns":1.5}}`,
		`{"type":"grid","props":{"columns":5001}}`, `{"type":"grid","props":{"columns":2,"gap":-1}}`,
		`{"type":"scroll","props":{"horizontal":1}}`, `{"type":"scroll","props":{"stickToEnd":"yes"}}`,
		`{"type":"scroll","props":{"endThreshold":-1}}`, `{"type":"scroll","props":{"endThreshold":16385}}`,
		`{"type":"scroll","props":{"onEndReached":{"$handler":""}}}`, `{"type":"view","props":{"onEndReached":{"$handler":"x"}}}`,
	} {
		if _, err := Parse([]byte(`{"version":1,"root":` + root + `}`)); err == nil {
			t.Errorf("accepted invalid layout %s", root)
		}
	}
	for _, n := range []*Node{{Type: NodeGrid, Props: &GridProps{Columns: 2, Gap: Size{Value: float32(math.NaN())}}}, {Type: NodeScroll, Props: &ScrollProps{EndThreshold: Size{Value: float32(math.NaN())}}}} {
		if _, err := NewTree(n); err == nil {
			t.Fatal("accepted non-finite Go layout")
		}
	}
}
