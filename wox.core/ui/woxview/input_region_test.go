package woxview

import (
	"encoding/json"
	"testing"

	ui "wox/ui/runtime"
	"wox/ui/widget"
)

// TestInputRegionHover verifies that an explicitly interactive empty gap receives hover without painting it.
func TestInputRegionHover(t *testing.T) {
	doc := `{"version":1,"root":{"type":"stack","props":{"style":{"width":208,"height":40}},"children":[{"type":"view","key":"panel","props":{"onHoverChange":{"$handler":"hover"},"style":{"left":0,"top":0,"width":164,"height":40,"backgroundColor":"#fff"}}},{"type":"view","key":"gap","props":{"inputRegion":true,"onHoverChange":{"$handler":"hover"},"style":{"left":164,"top":0,"width":10,"height":40}}},{"type":"icon-button","key":"button","props":{"icon":"control.close","label":"Close","onPress":{"$handler":"cancel"},"onHoverChange":{"$handler":"hover"},"style":{"left":174,"top":0,"width":32,"height":40}}}]}}`
	v := NewView()
	v.SetTree(mustParse(t, doc))
	var events []Event
	host := widget.NewHost(func(info ui.FrameInfo) widget.Widget {
		return v.Build(BuildProps{Width: info.Size.Width, Height: info.Size.Height, OnNodeEvent: func(event Event) { events = append(events, event) }})
	})
	host.AttachServices(fakeServices{})
	defer host.Dispose()
	host.Frame(&ui.DisplayList{}, ui.FrameInfo{Size: ui.Size{Width: 208, Height: 40}, Scale: 1})
	host.Pointer(ui.PointerEvent{Kind: ui.PointerMove, Position: ui.Point{X: 207, Y: 20}})
	if len(events) != 0 {
		t.Fatalf("transparent area emitted hover: %+v", events)
	}
	host.Pointer(ui.PointerEvent{Kind: ui.PointerMove, Position: ui.Point{X: 80, Y: 20}})
	host.Pointer(ui.PointerEvent{Kind: ui.PointerMove, Position: ui.Point{X: 168, Y: 20}})
	if len(events) != 3 || events[1].Key != "panel" || events[2].Key != "gap" {
		t.Fatalf("panel-to-gap hover: %+v", events)
	}
	var args []bool
	if err := json.Unmarshal(events[2].Args, &args); err != nil || len(args) != 1 || !args[0] {
		t.Fatalf("hover arguments: %s, %v", events[2].Args, err)
	}
	host.Pointer(ui.PointerEvent{Kind: ui.PointerMove, Position: ui.Point{X: 190, Y: 20}})
	if len(events) != 5 || events[3].Key != "gap" || events[4].Key != "button" {
		t.Fatalf("gap-to-button hover: %+v", events)
	}
}
