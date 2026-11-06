package pluginwindow

import (
	"testing"
	"wox/plugin"
	ui "wox/ui/runtime"
	"wox/ui/widget"
	"wox/ui/woxview"
	"wox/util/overlay"
)

// TestTransparentWindowRendersOnlyItsContent checks alpha without relying on a desktop compositor.
func TestTransparentWindowRendersOnlyItsContent(t *testing.T) {
	tree, err := prepareContent(plugin.WindowContent{Type: plugin.WindowContentWoxView, Data: `{"version":1,"root":{"type":"stack","props":{"style":{"height":60}},"children":[{"type":"view","key":"shape","props":{"label":"Shape","onPress":{"$handler":"press"},"style":{"left":20,"top":20,"width":40,"height":20,"backgroundColor":"#ff0000"}}}]}}`})
	if err != nil {
		t.Fatal(err)
	}
	yes := true
	var events []plugin.WindowEvent
	s := &session{document: "doc", options: plugin.WindowOptions{Transparent: &yes, HideChrome: &yes}, view: woxview.NewView(), emit: func(e plugin.WindowEvent) { events = append(events, e) }}
	s.view.SetTree(tree)
	host := widget.NewHost(s.build)
	host.AttachServices(translationTestServices{})
	defer host.Dispose()
	list := &ui.DisplayList{}
	host.Frame(list, ui.FrameInfo{Size: ui.Size{Width: 100, Height: 60}, PixelSize: ui.PixelSize{Width: 100, Height: 60}, Scale: 1})
	renderer, err := ui.NewSoftwareRenderer(100, 60)
	if err != nil {
		t.Fatal(err)
	}
	if err := renderer.Render(list); err != nil {
		t.Fatal(err)
	}
	if pixel := renderer.RGBA().RGBAAt(5, 5); pixel.A != 0 {
		t.Fatalf("background is not transparent: %+v", pixel)
	}
	if pixel := renderer.RGBA().RGBAAt(30, 25); pixel.R != 255 || pixel.A != 255 {
		t.Fatalf("content did not render opaque red: %+v", pixel)
	}
	host.Pointer(ui.PointerEvent{Kind: ui.PointerDown, Button: ui.PointerButtonPrimary, Position: ui.Point{X: 30, Y: 25}})
	host.Pointer(ui.PointerEvent{Kind: ui.PointerUp, Button: ui.PointerButtonPrimary, Position: ui.Point{X: 30, Y: 25}})
	if len(events) != 1 || events[0].Type != plugin.WindowEventWoxView {
		t.Fatalf("transparent window lost content interaction: %+v", events)
	}
	s.options.Transparent = nil
	if box := s.build(ui.FrameInfo{Size: ui.Size{Width: 100, Height: 60}}).(widget.Container); box.Color != overlay.CurrentThemeChrome().Background {
		t.Fatal("default window no longer paints its theme background")
	}
}
