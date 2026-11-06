package component

import (
	"testing"
	"time"

	ui "wox/ui/runtime"
	"wox/ui/widget"
)

// controlTransitionPixel checks composited pixels at physical display scale.
func controlTransitionPixel(t *testing.T, host *widget.Host, point ui.Point, scale float32) ui.Color {
	t.Helper()
	list := &ui.DisplayList{}
	host.Frame(list, ui.FrameInfo{Size: ui.Size{Width: 160, Height: 100}, Scale: scale})
	list.TransformScene(scale, ui.Point{})
	r, err := ui.NewSoftwareRenderer(int(160*scale), int(100*scale))
	if err != nil {
		t.Fatal(err)
	}
	if err = r.Render(list); err != nil {
		t.Fatal(err)
	}
	c := r.RGBA().RGBAAt(int(point.X*scale), int(point.Y*scale))
	return ui.Color{R: c.R, G: c.G, B: c.B, A: c.A}
}

// TestControlBackgroundTransition restores each control's actual native fill,
// including transparent defaults and separate table header/body surfaces.
func TestControlBackgroundTransition(t *testing.T) {
	for _, kind := range []string{"button", "icon", "field", "dropdown", "checkbox", "switch", "slider", "tag", "code", "hotkey", "table-header", "table-body", "list", "segment"} {
		t.Run(kind, func(t *testing.T) {
			clear, red := ui.Color{}, ui.Color{R: 255, A: 128}
			theme := ControlTheme{InputBackground: ui.Color{G: 255, A: 255}, Surface: ui.Color{B: 255, A: 255}, Text: ui.Color{G: 255, A: 255}, Border: ui.Color{B: 255, A: 255}}
			paint := &ControlPaint{Foreground: &clear, BackgroundTransition: &PaintTransition{Key: "test", Duration: 200 * time.Millisecond, Curve: widget.AnimationLinear}}
			point := ui.Point{X: 10, Y: 10}
			host := widget.NewHost(func(ui.FrameInfo) widget.Widget {
				var child widget.Widget
				switch kind {
				case "button":
					child = WoxButton(ButtonProps{ID: "control", Label: "test", Width: 100, Height: 40, Theme: theme, Paint: paint})
				case "icon":
					child = WoxIconButton(IconButtonProps{ID: "control", Width: 100, Height: 40, Background: theme.Surface, Paint: paint})
				case "field":
					child = WoxTextField(TextFieldProps{ID: "control", Width: 100, Height: 40, Theme: theme, Paint: paint})
				case "dropdown":
					child = WoxDropdown(DropdownProps{ID: "control", Width: 100, Height: 40, Theme: theme, Paint: paint})
				case "checkbox":
					child = WoxCheckbox(CheckboxProps{ID: "control", Theme: theme, Paint: paint})
				case "switch":
					point.X = 25
					child = WoxSwitch(SwitchProps{ID: "control", Theme: theme, Paint: paint})
				case "slider":
					point = ui.Point{X: 70, Y: 20}
					child = WoxSliderTrack(SliderTrackProps{Width: 100, Height: 40, Max: 100, Theme: theme, Paint: paint})
				case "tag":
					point.X = 4
					child = WoxTagWithPaint("test", clear, theme, false, paint)
				case "code":
					point.X = 4
					child = WoxCodeBlock(CodeBlockProps{ID: "control", Width: 100, Theme: theme, Paint: paint})
				case "hotkey":
					child, _ = WoxHotkeyRecorder(HotkeyRecorderProps{ID: "control", Theme: theme, Paint: paint})
				case "table-header", "table-body":
					if kind == "table-body" {
						point.Y = 60
					}
					child = WoxDataTable(DataTableProps{ID: "control", Width: 120, Height: 80, Columns: []DataTableColumn{{ID: "c"}}, Rows: []DataTableRow{{ID: "r", Cells: []string{""}}}, Theme: theme, Paint: paint})
				case "list":
					child = WoxListItem(ListItemProps{ID: "control", Width: 100, Height: 40, Theme: theme, Paint: paint})
				case "segment":
					child = WoxSegmentedButton(SegmentedButtonProps{ID: "control", Width: 100, Theme: theme, Paint: paint})
				}
				return widget.Flex{Axis: widget.Vertical, Children: []widget.Widget{child}}
			})
			host.AttachServices(&hotkeyRecorderHostServices{})
			defer host.Dispose()
			now := time.Unix(1, 0)
			host.SetAnimationClock(func() time.Time { return now })
			// Resolve fixture coordinates before sampling the first visible pixel.
			controlTransitionPixel(t, host, point, 1)
			initial := controlTransitionPixel(t, host, point, 1)
			paint.Background = &red
			if got := controlTransitionPixel(t, host, point, 1); got != initial {
				t.Fatalf("start jumped: %+v to %+v", initial, got)
			}
			now = now.Add(100 * time.Millisecond)
			mid := controlTransitionPixel(t, host, point, 1.5)
			if mid == initial || mid == (ui.Color{R: 128, A: 128}) {
				t.Fatalf("missing midpoint %+v", mid)
			}
			now = now.Add(100 * time.Millisecond)
			if got := controlTransitionPixel(t, host, point, 2); got != (ui.Color{R: 128, A: 128}) {
				t.Fatalf("target fill/compositing %+v", got)
			}
			paint.Background = nil
			if got := controlTransitionPixel(t, host, point, 2); got != (ui.Color{R: 128, A: 128}) {
				t.Fatalf("restoration jumped %+v", got)
			}
			now = now.Add(100 * time.Millisecond)
			if got := controlTransitionPixel(t, host, point, 1.5); got != mid {
				t.Fatalf("reverse midpoint %+v want %+v", got, mid)
			}
			now = now.Add(100 * time.Millisecond)
			if got := controlTransitionPixel(t, host, point, 1); got != initial {
				t.Fatalf("native fill lost %+v want %+v", got, initial)
			}
		})
	}
}
