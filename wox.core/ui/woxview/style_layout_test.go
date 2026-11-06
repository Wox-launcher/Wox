package woxview

import (
	"fmt"
	"math"
	"testing"

	component "wox/ui/launcher/component"
	ui "wox/ui/runtime"
	"wox/ui/widget"
)

// TestAlignSelfProtocolAndLayout verifies the protocol mapping through live hit testing at each DPI.
func TestAlignSelfProtocolAndLayout(t *testing.T) {
	for _, nodeType := range []string{"row", "column"} {
		for _, scale := range []float32{1, 1.5, 2} {
			v := NewView()
			v.SetTree(mustParse(t, fmt.Sprintf(`{"version":1,"root":{"type":%q,"props":{"style":{"width":120,"height":100,"alignItems":"stretch"}},"children":[
				{"type":"view","key":"fixed","props":{"style":{"width":20,"height":20}}},
				{"type":"view","key":"end","props":{"label":"End","onPress":{"$handler":"end"},"style":{"width":20,"height":20,"alignSelf":"end","flex":1}}}
			]}}`, nodeType)))
			var events []recordedEvent
			host := layoutHost(v, &events)
			renderLayout(host, 120, 100, scale)
			bounds := automationBounds(t, host, "woxview/"+nodeType+"0/k:end/view#press")
			if nodeType == "row" && (bounds.Y != 80 || bounds.Height != 20 || bounds.Width != 100) || nodeType == "column" && (bounds.X != 100 || bounds.Width != 20 || bounds.Height != 80) {
				t.Fatalf("%s scale %v: %+v", nodeType, scale, bounds)
			}
			tap(host, bounds)
			if len(events) != 1 || events[0].handler != "end" {
				t.Fatalf("aligned target did not receive the click: %+v", events)
			}
			host.Dispose()
		}
	}
	for _, value := range []string{`"baseline"`, `true`, `1`} {
		if _, err := Parse([]byte(`{"version":1,"root":{"type":"view","props":{"style":{"alignSelf":` + value + `}}}}`)); err == nil {
			t.Fatalf("accepted invalid alignSelf %s", value)
		}
	}
}

// TestAspectRatioBoxConstraints covers preferred sizes, padding, min/max bounds, and live DPI changes.
func TestAspectRatioBoxConstraints(t *testing.T) {
	for _, tc := range []struct {
		name, style   string
		width, height float32
	}{
		{"width", `"width":120`, 120, 60},
		{"height", `"height":50`, 100, 50},
		{"available", `"padding":8`, 200, 100},
		{"max-width", `"maxWidth":80`, 80, 40},
		{"max-height", `"width":120,"maxHeight":30`, 60, 30},
		{"minimum", `"width":120,"minWidth":150`, 150, 75},
		{"both-explicit", `"width":100,"height":35`, 100, 35},
		{"margin", `"margin":10`, 180, 90},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := NewView()
			v.SetTree(mustParse(t, `{"version":1,"root":{"type":"column","children":[{"type":"view","key":"box","props":{"style":{"aspectRatio":2,`+tc.style+`}},"children":[{"type":"view","key":"fill","props":{"label":"Fill","onPress":{"$handler":"fill"},"style":{"width":200,"height":200}}}]}]}}`))
			var events []recordedEvent
			host := layoutHost(v, &events)
			defer host.Dispose()
			for _, scale := range []float32{1, 1.5, 2, 1} {
				renderLayout(host, 200, 200, scale)
				bounds := automationBounds(t, host, "woxview/column0/k:box/view/k:fill/view#press")
				width, height := tc.width, tc.height
				if tc.name == "available" {
					width, height = width-16, height-16
				}
				if bounds.Width != width || bounds.Height != height {
					t.Fatalf("scale %v: content %+v, want %vx%v", scale, bounds, width, height)
				}
				tap(host, bounds)
				if events[len(events)-1].handler != "fill" {
					t.Fatal("ratio layout lost input")
				}
			}
		})
	}
}

// TestAspectRatioControlsAndScroll checks the paths that own their dimensions internally.
func TestAspectRatioControlsAndScroll(t *testing.T) {
	for _, nodeType := range []string{"button", "scroll", "lazy-list"} {
		extra, suffix := `"label":"Target","onPress":{"$handler":"press"},`, ""
		if nodeType != "button" {
			suffix = "#press"
		}
		if nodeType == "lazy-list" {
			extra += `"itemHeight":20,`
		}
		v := NewView()
		v.SetTree(mustParse(t, fmt.Sprintf(`{"version":1,"root":{"type":"column","children":[{"type":%q,"key":"target","props":{%s"style":{"width":120,"aspectRatio":2}}}]}}`, nodeType, extra)))
		var events []recordedEvent
		host := layoutHost(v, &events)
		renderLayout(host, 200, 200, 1.5)
		bounds := automationBounds(t, host, "woxview/column0/k:target/"+nodeType+suffix)
		if bounds.Width != 120 || bounds.Height != 60 {
			t.Fatalf("%s ignored ratio: %+v", nodeType, bounds)
		}
		host.Dispose()
	}
}

// TestBoxClipBorderPaintAndHit verifies that clipping precedes margin and borders survive child paint.
func TestBoxClipBorderPaintAndHit(t *testing.T) {
	v := NewView()
	v.SetTree(mustParse(t, `{"version":1,"root":{"type":"column","children":[{"type":"row","props":{"style":{"width":60,"height":30,"margin":10,"overflow":"hidden","borderTopWidth":2,"borderColor":"#f00"}},"children":[{"type":"view","props":{"style":{"width":40,"height":20}}},{"type":"view","key":"overflow","props":{"label":"Overflow","onPress":{"$handler":"hit"},"style":{"width":50,"height":20,"backgroundColor":"#00f"}}}]}]}}`))
	var events []recordedEvent
	host := layoutHost(v, &events)
	defer host.Dispose()
	for _, scale := range []float32{1, 1.5, 2} {
		actual, expected := &ui.DisplayList{}, &ui.DisplayList{}
		host.Frame(actual, ui.FrameInfo{Size: ui.Size{Width: 200, Height: 100}, PixelSize: ui.PixelSize{Width: int(200 * scale), Height: int(100 * scale)}, Scale: scale})
		expected.Clear(ui.Color{})
		expected.PushClipRect(ui.Rect{X: 10, Y: 10, Width: 60, Height: 30})
		expected.FillRoundedRect(ui.Rect{X: 50, Y: 10, Width: 50, Height: 20}, 0, ui.Color{B: 255, A: 255})
		expected.FillRect(ui.Rect{X: 10, Y: 10, Width: 60, Height: 2}, ui.Color{R: 255, A: 255})
		expected.PopClipRect()
		if err := actual.Compare(expected); err != nil {
			t.Fatalf("scale %v: %v", scale, err)
		}
		before := len(events)
		for _, point := range []ui.Point{{X: 60, Y: 20}, {X: 80, Y: 20}, {X: 5, Y: 20}} {
			host.Pointer(ui.PointerEvent{Kind: ui.PointerDown, Button: ui.PointerButtonPrimary, Position: point})
			host.Pointer(ui.PointerEvent{Kind: ui.PointerUp, Button: ui.PointerButtonPrimary, Position: point})
		}
		if len(events) != before+1 {
			t.Fatal("overflow or margin activated the clipped child")
		}
	}
}

// TestFloatingStyleUsesMaterialAndEdgeOverrides checks actual renderer commands, including a zero side.
func TestFloatingStyleUsesMaterialAndEdgeOverrides(t *testing.T) {
	v := NewView()
	v.SetTree(mustParse(t, `{"version":1,"root":{"type":"column","children":[{"type":"view","props":{"style":{"width":80,"height":40,"floating":true,"borderWidth":2,"borderLeftWidth":0,"borderColor":"#f00","backgroundColor":"#1238ab","borderRadius":4}}}]}}`))
	var events []recordedEvent
	host := layoutHost(v, &events)
	defer host.Dispose()
	actual, expected := &ui.DisplayList{}, &ui.DisplayList{}
	host.Frame(actual, ui.FrameInfo{Size: ui.Size{Width: 200, Height: 100}, Scale: 1.5})
	bounds, red := ui.Rect{Width: 80, Height: 40}, ui.Color{R: 255, A: 255}
	expected.Clear(ui.Color{})
	expected.FloatingMaterial(bounds, 4, ui.Color{R: 0x12, G: 0x38, B: 0xab, A: 255}, ui.Color{})
	for _, strip := range []ui.Rect{{Width: 80, Height: 2}, {Y: 38, Width: 80, Height: 2}, {X: 78, Y: 2, Width: 2, Height: 36}} {
		expected.PushClipRect(strip)
		expected.FillRoundedRect(bounds, 4, red)
		expected.PopClipRect()
	}
	if err := actual.Compare(expected); err != nil {
		t.Fatal(err)
	}
}

// TestBoxStyleRejectsInvalidValues keeps parser and programmatic tree bounds explicit.
func TestBoxStyleRejectsInvalidValues(t *testing.T) {
	for _, style := range []string{`"aspectRatio":0`, `"aspectRatio":-1`, `"aspectRatio":16385`, `"aspectRatio":"wide"`, `"overflow":"scroll"`, `"overflow":true`, `"floating":1`, `"borderTopWidth":-1`, `"borderLeftWidth":16385`, `"borderBottomWidth":"thin"`} {
		if _, err := Parse([]byte(`{"version":1,"root":{"type":"view","props":{"style":{` + style + `}}}}`)); err == nil {
			t.Fatalf("accepted %s", style)
		}
	}
	for _, value := range []float32{float32(math.NaN()), float32(math.Inf(1)), -1, 0} {
		if _, err := NewTree(&Node{Type: NodeView, Props: &BoxProps{Styled: Styled{Style: Style{AspectRatio: &value}}}}); err == nil {
			t.Fatalf("programmatic tree accepted ratio %v", value)
		}
	}
}

// TestControlSideBorderPaintsAboveControl protects foreground edge placement on opaque shared controls.
func TestControlSideBorderPaintsAboveControl(t *testing.T) {
	var frames []*ui.DisplayList
	for _, extra := range []string{"", `,"borderTopWidth":2,"borderColor":"#f00"`} {
		v := NewView()
		v.SetTree(mustParse(t, `{"version":1,"root":{"type":"column","children":[{"type":"button","props":{"label":"Button","style":{"width":80,"height":32`+extra+`}}}]}}`))
		var events []recordedEvent
		host := layoutHost(v, &events)
		list := &ui.DisplayList{}
		host.Frame(list, ui.FrameInfo{Size: ui.Size{Width: 200, Height: 100}, Scale: 1.5})
		frames = append(frames, list)
		host.Dispose()
	}
	frames[0].FillRect(ui.Rect{Width: 80, Height: 2}, ui.Color{R: 255, A: 255})
	if err := frames[1].Compare(frames[0]); err != nil {
		t.Fatal(err)
	}
}

// TestAspectRatioUnboundedAxis uses the bounded height of a horizontal scroller.
func TestAspectRatioUnboundedAxis(t *testing.T) {
	ratio := float32(2)
	var measured ui.Size
	host := widget.NewHost(func(ui.FrameInfo) widget.Widget {
		return widget.ScrollView{Width: 200, Height: 60, Horizontal: true, Child: (Style{AspectRatio: &ratio}).finishBox(component.ControlTheme{}, widget.LayoutBuilder{Build: func(size ui.Size) widget.Widget {
			measured = size
			return widget.Container{}
		}})}
	})
	host.AttachServices(fakeServices{})
	defer host.Dispose()
	renderLayout(host, 200, 200, 2)
	if measured.Width != 120 || measured.Height != 60 {
		t.Fatalf("unbounded-width ratio constraints = %+v", measured)
	}
}
