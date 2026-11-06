package component

import (
	"testing"

	ui "wox/ui/runtime"
	"wox/ui/widget"
)

// TestControlPaintTransparency survives each control's hover and default-color
// fallbacks, including a zero radius and border on an otherwise outlined surface.
func TestControlPaintTransparency(t *testing.T) {
	clear, zero := ui.Color{}, float32(0)
	theme := ControlTheme{InputBackground: ui.Color{G: 255, A: 255}, Text: ui.Color{R: 255, A: 255}, ControlText: ui.Color{R: 255, A: 255}}
	for _, kind := range []string{"button", "field", "dropdown"} {
		t.Run(kind, func(t *testing.T) {
			paint := &ControlPaint{Background: &clear, Foreground: &clear, BorderColor: &clear, BorderWidth: &zero, Radius: &zero}
			host := widget.NewHost(func(ui.FrameInfo) widget.Widget {
				var child widget.Widget
				switch kind {
				case "button":
					child = WoxButton(ButtonProps{ID: "control", Label: "Test", Width: 100, Height: 40, Theme: theme, Variant: ButtonOutline, Paint: paint})
				case "field":
					child = WoxTextField(TextFieldProps{ID: "control", Label: "Test", Value: "", Width: 100, Height: 40, Theme: theme, BorderColor: theme.Text, BorderWidth: 1, Paint: paint})
				case "dropdown":
					child = WoxDropdown(DropdownProps{ID: "control", Label: "Test", Width: 100, Height: 40, Foreground: theme.Text, Theme: theme, Paint: paint, OnTap: func() {}})
				}
				return widget.Flex{Axis: widget.Vertical, Children: []widget.Widget{child}}
			})
			host.AttachServices(&hotkeyRecorderHostServices{})
			defer host.Dispose()
			for _, scale := range []float32{1, 1.5, 2} {
				for range 2 {
					list := &ui.DisplayList{}
					host.Frame(list, ui.FrameInfo{Size: ui.Size{Width: 150, Height: 80}, Scale: scale})
					list.TransformScene(scale, ui.Point{})
					r, err := ui.NewSoftwareRenderer(int(150*scale), int(80*scale))
					if err != nil {
						t.Fatal(err)
					}
					if err := r.Render(list); err != nil {
						t.Fatal(err)
					}
					for _, point := range []ui.Point{{X: 1, Y: 1}, {X: 10, Y: 30}, {X: 98, Y: 38}} {
						if got := r.RGBA().RGBAAt(int(point.X*scale), int(point.Y*scale)); got.A != 0 {
							t.Fatalf("DPI %v at %+v: %+v", scale, point, got)
						}
					}
					host.Pointer(ui.PointerEvent{Kind: ui.PointerMove, Position: ui.Point{X: 10, Y: 30}})
				}
			}
		})
	}
}

// TestDataTablePaintFrame replaces the native frame rather than stacking another
// stroke on top, while keeping cell separators independent of the outer border.
func TestDataTablePaintFrame(t *testing.T) {
	red, green, blue, clear := ui.Color{R: 255, A: 128}, ui.Color{G: 255, A: 128}, ui.Color{B: 255, A: 255}, ui.Color{}
	for _, tc := range []struct {
		name   string
		width  float32
		border *ui.Color
		x      float32
		want   ui.Color
	}{
		{"removed", 0, nil, 0, ui.Color{R: 128, A: 128}},
		{"theme-fallback", 4, nil, 2, blue},
		{"single-alpha-stroke", 4, &green, 0, ui.Color{R: 63, G: 128, A: 191}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			paint := &ControlPaint{Background: &red, Foreground: &clear, BorderColor: tc.border, BorderWidth: &tc.width}
			host := widget.NewHost(func(ui.FrameInfo) widget.Widget {
				return WoxDataTable(DataTableProps{ID: "table", Width: 120, Height: 80, Columns: []DataTableColumn{{ID: "c"}}, Rows: []DataTableRow{{ID: "r", Cells: []string{""}}}, Theme: ControlTheme{Border: blue}, Paint: paint})
			})
			host.AttachServices(&hotkeyRecorderHostServices{})
			defer host.Dispose()
			for _, scale := range []float32{1, 1.5, 2} {
				list := &ui.DisplayList{}
				host.Frame(list, ui.FrameInfo{Size: ui.Size{Width: 120, Height: 80}, Scale: scale})
				list.TransformScene(scale, ui.Point{})
				r, err := ui.NewSoftwareRenderer(int(120*scale), int(80*scale))
				if err != nil {
					t.Fatal(err)
				}
				if err = r.Render(list); err != nil {
					t.Fatal(err)
				}
				c := r.RGBA().RGBAAt(int(tc.x*scale), int(60*scale))
				if got := (ui.Color{R: c.R, G: c.G, B: c.B, A: c.A}); got != tc.want {
					t.Fatalf("DPI %v frame %+v want %+v", scale, got, tc.want)
				}
				separator := r.RGBA().RGBAAt(int(60*scale), int(39*scale))
				if separator.B != 255 || separator.A != 255 {
					t.Fatalf("outer paint changed separator: %+v", separator)
				}
			}
		})
	}
}
