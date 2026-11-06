package woxview

import (
	"fmt"
	"testing"

	component "wox/ui/launcher/component"
	ui "wox/ui/runtime"
	"wox/ui/widget"
)

// TestShadowProtocol verifies token mapping on boxes and shared controls, with
// density-scaled elevation, unchanged hit bounds, clipping, and group opacity.
func TestShadowProtocol(t *testing.T) {
	for _, kind := range []string{"view", "button"} {
		for _, token := range []string{"sm", "md", "lg"} {
			for _, density := range []float32{0.9, 1, 1.1} {
				v := NewView()
				v.SetTree(mustParse(t, fmt.Sprintf(`{"version":1,"root":{"type":"column","props":{"style":{"padding":30}},"children":[{"type":%q,"key":"card","props":{"label":"Card","onPress":{"$handler":"tap"},"style":{"width":80,"height":40,"overflow":"hidden","shadow":%q,"opacity":0.5}}}]}}`, kind, token)))
				taps := 0
				host := widget.NewHost(func(info ui.FrameInfo) widget.Widget {
					return v.Build(BuildProps{Width: info.Size.Width, Height: info.Size.Height, Theme: component.ControlTheme{DensityScale: density}, OnNodeEvent: func(Event) { taps++ }})
				})
				host.AttachServices(fakeServices{})
				for i, scale := range []float32{1, 1.5, 2, 1} {
					d := &ui.DisplayList{}
					host.Frame(d, ui.FrameInfo{Size: ui.Size{Width: 160, Height: 130}, PixelSize: ui.PixelSize{Width: int(160 * scale), Height: int(130 * scale)}, Scale: scale})
					id := "woxview/column0/k:card/" + kind
					if kind == "view" {
						id += "#press"
					}
					bounds := automationBounds(t, host, id)
					if bounds.X != 30 || bounds.Y != 30 || bounds.Width != 80 || bounds.Height != 40 {
						t.Fatalf("%s %s shadow changed geometry: %+v", kind, token, bounds)
					}
					d.TransformScene(scale, ui.Point{})
					r, _ := ui.NewSoftwareRenderer(int(160*scale), int(130*scale))
					if err := r.Render(d); err != nil {
						t.Fatal(err)
					}
					pixel := r.RGBA().RGBAAt(int(70*scale), int(71*scale))
					if pixel.A == 0 || pixel.A > 32 {
						t.Fatalf("%s %s density %v scale %v lost shadow or opacity: %+v", kind, token, density, scale, pixel)
					}
					tap(host, ui.Rect{X: 65, Y: 72, Width: 4, Height: 2})
					if taps != i {
						t.Fatal("shadow stole input outside the control")
					}
					tap(host, bounds)
					if taps != i+1 {
						t.Fatal("shadow blocked the control")
					}
				}
				host.Dispose()
			}
		}
	}
}

// TestShadowRejectsInvalidValues keeps the protocol restricted to elevation tokens.
func TestShadowRejectsInvalidValues(t *testing.T) {
	for _, value := range []string{`"xl"`, `"4px"`, `4`, `true`, `{}`} {
		if _, err := Parse([]byte(`{"version":1,"root":{"type":"view","props":{"style":{"shadow":` + value + `}}}}`)); err == nil {
			t.Fatalf("accepted shadow %s", value)
		}
	}
	if _, err := NewTree(&Node{Type: NodeView, Props: &BoxProps{Styled: Styled{Style: Style{Shadow: Shadow("xl")}}}}); err == nil {
		t.Fatal("accepted unknown programmatic shadow")
	}
}
