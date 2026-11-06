package woxview

import (
	"testing"

	component "wox/ui/launcher/component"
	ui "wox/ui/runtime"
	"wox/ui/widget"
)

// sizeTextServices records actual text measurements rather than inspecting the size resolver.
type sizeTextServices struct {
	fakeServices
	sizes map[string]float32
}

func (s sizeTextServices) MeasureText(text string, style ui.TextStyle) (ui.TextMetrics, error) {
	s.sizes[text] = style.Size
	return s.fakeServices.MeasureText(text, style)
}

// TestSizeFontTokensScaleOnce exercises both widget-owned and component-owned typography.
func TestSizeFontTokensScaleOnce(t *testing.T) {
	v := NewView()
	v.SetTree(mustParse(t, `{"version":1,"root":{"type":"column","children":[
		{"type":"text","props":{"text":"Plain","style":{"fontSize":"$fontMd","lineHeight":"$fontTitle"}}},
		{"type":"button","props":{"label":"Button","style":{"fontSize":"$fontMd"}}},
		{"type":"text","props":{"text":"","style":{"fontSize":"$fontMd","lineHeight":"$fontTitle"}},"children":[{"type":"span","props":{"text":"Rich"}}]},
		{"type":"text","key":"selected","props":{"text":"First\nSecond","selectable":true,"style":{"fontSize":"$fontMd","lineHeight":"$fontTitle"}}}
	]}}`))
	theme := component.ControlTheme{Text: ui.Color{A: 255}}
	services := sizeTextServices{sizes: map[string]float32{}}
	host := widget.NewHost(func(info ui.FrameInfo) widget.Widget {
		return v.Build(BuildProps{Width: info.Size.Width, Height: info.Size.Height, Theme: theme})
	})
	host.AttachServices(services)
	defer host.Dispose()
	for i, density := range []float32{0.9, 1, 1.1, 1} {
		theme.DensityScale = density
		clear(services.sizes)
		renderLayout(host, 300, 300, []float32{1, 1.5, 2, 1}[i])
		for _, label := range []string{"Plain", "Button", "Rich"} {
			if got := services.sizes[label]; got != theme.Scaled(13) {
				t.Fatalf("%s density %v: font size %v, want %v", label, density, got, theme.Scaled(13))
			}
		}
		bounds := automationBounds(t, host, "woxview/column0/k:selected/text/selectable")
		if bounds.Height != 2*theme.Scaled(22)+1 {
			t.Fatalf("selectable text line box did not follow density %v: %+v", density, bounds)
		}
	}
}

// TestSizeBorderTokensPaint compares the complete command stream, including explicit zero sides.
func TestSizeBorderTokensPaint(t *testing.T) {
	v := NewView()
	v.SetTree(mustParse(t, `{"version":1,"root":{"type":"column","children":[{"type":"view","props":{"style":{"width":80,"height":40,"borderWidth":"$space1","borderLeftWidth":0,"borderRadius":"$radiusMd","borderColor":"#f00","backgroundColor":"#00f"}}}]}}`))
	theme := component.ControlTheme{}
	host := widget.NewHost(func(info ui.FrameInfo) widget.Widget {
		return v.Build(BuildProps{Width: info.Size.Width, Height: info.Size.Height, Theme: theme})
	})
	host.AttachServices(fakeServices{})
	defer host.Dispose()
	for i, density := range []float32{0.9, 1, 1.1} {
		theme.DensityScale = density
		actual, expected := &ui.DisplayList{}, &ui.DisplayList{}
		scale := []float32{1, 1.5, 2}[i]
		host.Frame(actual, ui.FrameInfo{Size: ui.Size{Width: 200, Height: 100}, Scale: scale})
		bounds, red := ui.Rect{Width: 80, Height: 40}, ui.Color{R: 255, A: 255}
		width, radius := theme.Scaled(4), theme.Scaled(8)
		expected.Clear(ui.Color{})
		expected.FillRoundedRect(bounds, radius, ui.Color{B: 255, A: 255})
		for _, strip := range []ui.Rect{{Width: 80, Height: width}, {Y: 40 - width, Width: 80, Height: width}, {X: 80 - width, Y: width, Width: width, Height: 40 - 2*width}} {
			expected.PushClipRect(strip)
			expected.FillRoundedRect(bounds, radius, red)
			expected.PopClipRect()
		}
		// Desktop translation belongs to the renderer; logical tokens must survive it unchanged.
		origin := ui.Point{X: -120, Y: -40}
		actual.TransformScene(scale, origin)
		expected.TransformScene(scale, origin)
		if err := actual.Compare(expected); err != nil {
			t.Fatalf("density %v DPI %v: %v", density, scale, err)
		}
	}
}
