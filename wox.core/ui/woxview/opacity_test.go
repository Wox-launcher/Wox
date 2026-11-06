package woxview

import (
	"fmt"
	"math"
	"testing"

	ui "wox/ui/runtime"
)

// TestOpacityProtocolCompositesNestedChildren checks rendered pixels after keyed tree updates.
func TestOpacityProtocolCompositesNestedChildren(t *testing.T) {
	v := NewView()
	var events []recordedEvent
	host := layoutHost(v, &events)
	defer host.Dispose()
	for i, alpha := range []float32{0.5, 0, 1, 0.25} {
		v.SetTree(mustParse(t, fmt.Sprintf(`{"version":1,"root":{"type":"column","props":{"style":{"backgroundColor":"#00f"}},"children":[{"type":"stack","key":"group","props":{"style":{"width":80,"height":40,"opacity":%v}},"children":[{"type":"view","props":{"style":{"width":60,"height":40,"backgroundColor":"#f00"}}},{"type":"view","key":"hit","props":{"label":"Hit","onPress":{"$handler":"hit"},"style":{"left":20,"width":60,"height":40,"backgroundColor":"#0f0"}}}]}]}}`, alpha)))
		list := &ui.DisplayList{}
		scale := []float32{1, 1.5, 2, 1}[i]
		host.Frame(list, ui.FrameInfo{Size: ui.Size{Width: 100, Height: 60}, PixelSize: ui.PixelSize{Width: int(100 * scale), Height: int(60 * scale)}, Scale: scale})
		r, _ := ui.NewSoftwareRenderer(100, 60)
		if err := r.Render(list); err != nil {
			t.Fatal(err)
		}
		pixel := r.RGBA().RGBAAt(30, 20)
		if pixel.R != 0 || math.Abs(float64(pixel.G)-float64(alpha*255)) > 1 || math.Abs(float64(pixel.B)-float64((1-alpha)*255)) > 1 || pixel.A != 255 {
			t.Fatalf("opacity %v applied per child: %+v", alpha, pixel)
		}
		tap(host, automationBounds(t, host, "woxview/column0/k:group/stack/k:hit/view#press"))
		if len(events) != i+1 {
			t.Fatal("opacity changed event routing")
		}
	}
}

// TestOpacityRejectsInvalidValues covers JSON types and nonfinite programmatic trees.
func TestOpacityRejectsInvalidValues(t *testing.T) {
	for _, value := range []string{`-0.1`, `1.1`, `"0.5"`, `true`} {
		if _, err := Parse([]byte(`{"version":1,"root":{"type":"view","props":{"style":{"opacity":` + value + `}}}}`)); err == nil {
			t.Fatalf("accepted opacity %s", value)
		}
	}
	for _, value := range []float32{float32(math.NaN()), float32(math.Inf(1))} {
		if _, err := NewTree(&Node{Type: NodeView, Props: &BoxProps{Styled: Styled{Style: Style{Opacity: &value}}}}); err == nil {
			t.Fatal("accepted nonfinite opacity")
		}
	}
}
