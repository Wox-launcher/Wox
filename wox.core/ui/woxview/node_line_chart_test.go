package woxview

import (
	"math"
	"strings"
	"testing"

	component "wox/ui/launcher/component"
	ui "wox/ui/runtime"
	"wox/ui/widget"
)

func TestLineChartValidationAndPoints(t *testing.T) {
	for _, props := range []string{`{}`, `{"values":null}`, `{"values":[1e101]}`, `{"values":[` + strings.Repeat("0,", 4096) + `0]}`} {
		if _, err := Parse([]byte(`{"version":1,"root":{"type":"line-chart","props":` + props + `}}`)); err == nil {
			t.Fatalf("accepted invalid chart: %s", props[:min(len(props), 80)])
		}
	}
	if err := (&LineChartProps{Values: []float64{math.Inf(1)}}).prepare(); err == nil {
		t.Fatal("accepted infinity")
	}
	for _, tc := range []struct {
		values []float64
		want   []ui.Point
	}{
		{[]float64{2, 4, 3}, []ui.Point{{X: 10, Y: 120}, {X: 60, Y: 20}, {X: 110, Y: 70}}},
		{[]float64{7, 7}, []ui.Point{{X: 10, Y: 70}, {X: 110, Y: 70}}},
		{[]float64{7}, []ui.Point{{X: 60, Y: 70}}},
	} {
		got := lineChartPoints(tc.values, ui.Rect{X: 10, Y: 20, Width: 100, Height: 100})
		for i := range tc.want {
			if got[i] != tc.want[i] {
				t.Fatalf("values %v: got %v, want %v", tc.values, got, tc.want)
			}
		}
	}
	if _, err := Parse([]byte(`{"version":1,"root":{"type":"line-chart","props":{"values":[]}}}`)); err != nil {
		t.Fatal(err)
	}
}

// TestLineChartPaint checks the line and its logical geometry at multiple display scales.
func TestLineChartPaint(t *testing.T) {
	v := NewView()
	v.SetTree(mustParse(t, `{"version":1,"root":{"type":"column","children":[{"type":"line-chart","props":{"values":[1,1],"label":"Price","style":{"width":100,"height":100,"color":"#ff0000"}}}]}}`))
	var events []recordedEvent
	host := layoutHost(v, &events)
	defer host.Dispose()
	for _, scale := range []float32{1, 1.5, 2} {
		if pixel := statePixel(t, host, scale, 50, 50); pixel.R < 200 || pixel.A < 200 {
			t.Fatalf("scale %v: missing line: %+v", scale, pixel)
		}
		if pixel := statePixel(t, host, scale, 50, 30); pixel.A != 0 {
			t.Fatalf("scale %v: unexpected paint: %+v", scale, pixel)
		}
	}
}

// TestLineChartHover paints a local crosshair and clears it on pointer leave.
func TestLineChartHover(t *testing.T) {
	v := NewView()
	v.SetTree(mustParse(t, `{"version":1,"root":{"type":"line-chart","props":{"values":[1,1,1],"pointLabels":["09:30","10:00","10:30"],"style":{"width":100,"height":100,"color":"#ff0000"}}}}`))
	theme := component.ControlTheme{TextSecondary: ui.Color{G: 255, A: 255}, Text: ui.Color{R: 255, G: 255, B: 255, A: 255}, Surface: ui.Color{B: 255, A: 255}}
	host := widget.NewHost(func(f ui.FrameInfo) widget.Widget {
		return v.Build(BuildProps{Width: f.Size.Width, Height: f.Size.Height, Theme: theme})
	})
	host.AttachServices(fakeServices{})
	defer host.Dispose()
	if pixel := statePixel(t, host, 1, 100, 75); pixel.A != 0 {
		t.Fatalf("unexpected crosshair before hover: %+v", pixel)
	}
	host.Pointer(ui.PointerEvent{Kind: ui.PointerMove, Position: ui.Point{X: 100, Y: 75}})
	for _, scale := range []float32{1, 1.5, 2} {
		if pixel := statePixel(t, host, scale, 100, 75); pixel.G < 200 {
			t.Fatalf("scale %v: crosshair missing on hover: %+v", scale, pixel)
		}
	}
	host.Pointer(ui.PointerEvent{Kind: ui.PointerLeave})
	if pixel := statePixel(t, host, 1, 100, 75); pixel.A != 0 {
		t.Fatalf("crosshair remained after leave: %+v", pixel)
	}
}

func TestLineChartPointLabelValidation(t *testing.T) {
	if _, err := Parse([]byte(`{"version":1,"root":{"type":"line-chart","props":{"values":[1,2],"pointLabels":["one"]}}}`)); err == nil {
		t.Fatal("accepted mismatched point labels")
	}
	if _, err := Parse([]byte(`{"version":1,"root":{"type":"line-chart","props":{"values":[1,2],"pointLabels":["one","two"]}}}`)); err != nil {
		t.Fatal(err)
	}
}

// TestLineChartOptionsValidation keeps the public wire contract within paint limits.
func TestLineChartOptionsValidation(t *testing.T) {
	for _, option := range []string{
		`{"yAxis":"center"}`, `{"tooltipPosition":"bottom"}`, `{"gridLines":1}`, `{"xTicks":13}`,
		`{"lineWidth":0}`, `{"plotPadding":129}`, `{"yAxisWidth":513}`, `{"valueDecimals":7}`, `{"fillColor":"invalid"}`,
		`{"valueSuffix":"` + strings.Repeat("a", 33) + `"}`,
	} {
		if _, err := Parse([]byte(`{"version":1,"root":{"type":"line-chart","props":{"values":[1,2],"chart":` + option + `}}}`)); err == nil {
			t.Fatalf("accepted invalid chart options %s", option)
		}
	}
	if _, err := Parse([]byte(`{"version":1,"root":{"type":"line-chart","props":{"values":[1,2],"chart":{"showGrid":true,"showXAxis":true,"yAxis":"right","lineWidth":3,"fill":true,"hover":false,"tooltipPosition":"floating","valueDecimals":0}}}}`)); err != nil {
		t.Fatal(err)
	}
}

// TestLineChartOptionsPaint exercises fill, grid and selection with a left axis.
func TestLineChartOptionsPaint(t *testing.T) {
	v := NewView()
	v.SetTree(mustParse(t, `{"version":1,"root":{"type":"line-chart","props":{"values":[1,2,1],"pointLabels":["a","b","c"],"chart":{"yAxis":"left","showGrid":true,"gridLines":3,"gridColor":"#0000ff","fill":true,"fillColor":"#ff000080","tooltip":false},"style":{"height":120,"color":"#ff0000"}}}}`))
	theme := component.ControlTheme{TextSecondary: ui.Color{G: 255, A: 255}}
	host := widget.NewHost(func(f ui.FrameInfo) widget.Widget {
		return v.Build(BuildProps{Width: f.Size.Width, Height: f.Size.Height, Theme: theme})
	})
	host.AttachServices(fakeServices{})
	defer host.Dispose()
	if pixel := statePixel(t, host, 1, 100, 9); pixel.B < 200 {
		t.Fatalf("grid missing: %+v", pixel)
	}
	if pixel := statePixel(t, host, 1, 140, 65); pixel.R == 0 {
		t.Fatalf("fill missing: %+v", pixel)
	}
	host.Pointer(ui.PointerEvent{Kind: ui.PointerMove, Position: ui.Point{X: 140, Y: 70}})
	for _, scale := range []float32{1, 1.5, 2} {
		if pixel := statePixel(t, host, scale, 140, 70); pixel.G < 200 {
			t.Fatalf("scale %v: wrong selected point: %+v", scale, pixel)
		}
	}
	host.Pointer(ui.PointerEvent{Kind: ui.PointerLeave})
	if pixel := statePixel(t, host, 1, 140, 70); pixel.G != 0 {
		t.Fatalf("crosshair remained: %+v", pixel)
	}
}

// TestLineChartDisabledHover checks that interaction switches suppress the corresponding paint.
func TestLineChartDisabledHover(t *testing.T) {
	for _, options := range []string{`{"hover":false}`, `{"crosshair":false,"hoverPoint":false,"tooltip":false}`} {
		v := NewView()
		v.SetTree(mustParse(t, `{"version":1,"root":{"type":"line-chart","props":{"values":[1,1,1],"chart":`+options+`,"style":{"height":100,"color":"#ff0000"}}}}`))
		theme := component.ControlTheme{TextSecondary: ui.Color{G: 255, A: 255}}
		host := widget.NewHost(func(f ui.FrameInfo) widget.Widget {
			return v.Build(BuildProps{Width: f.Size.Width, Height: f.Size.Height, Theme: theme})
		})
		host.AttachServices(fakeServices{})
		statePixel(t, host, 1, 100, 75)
		host.Pointer(ui.PointerEvent{Kind: ui.PointerMove, Position: ui.Point{X: 100, Y: 75}})
		if pixel := statePixel(t, host, 1, 100, 75); pixel.A != 0 {
			t.Fatalf("options %s: unexpected hover paint %+v", options, pixel)
		}
		host.Dispose()
	}
}

// TestLineChartFloatingTooltip keeps the tooltip in bounds at either edge.
func TestLineChartFloatingTooltip(t *testing.T) {
	v := NewView()
	v.SetTree(mustParse(t, `{"version":1,"root":{"type":"line-chart","props":{"values":[1,1,1],"pointLabels":["first","middle","last"],"chart":{"tooltipPosition":"floating","crosshair":false},"style":{"height":120,"color":"#ff0000"}}}}`))
	theme := component.ControlTheme{Surface: ui.Color{B: 255, A: 255}, Text: ui.Color{R: 255, G: 255, B: 255, A: 255}}
	host := widget.NewHost(func(f ui.FrameInfo) widget.Widget {
		return v.Build(BuildProps{Width: f.Size.Width, Height: f.Size.Height, Theme: theme})
	})
	host.AttachServices(fakeServices{})
	defer host.Dispose()
	statePixel(t, host, 1, 1, 1)
	for _, tc := range []struct{ pointer, sample float32 }{{2, 14}, {198, 185}} {
		host.Pointer(ui.PointerEvent{Kind: ui.PointerMove, Position: ui.Point{X: tc.pointer, Y: 60}})
		if pixel := statePixel(t, host, 1, tc.sample, 15); pixel.B < 200 {
			t.Fatalf("tooltip at %v missing or outside bounds: %+v", tc.pointer, pixel)
		}
	}
	host.Pointer(ui.PointerEvent{Kind: ui.PointerLeave})
	if pixel := statePixel(t, host, 1, 185, 15); pixel.A != 0 {
		t.Fatalf("tooltip remained: %+v", pixel)
	}
}

// TestLineChartFillFade checks that the optional fill fades toward the baseline.
func TestLineChartFillFade(t *testing.T) {
	v := NewView()
	v.SetTree(mustParse(t, `{"version":1,"root":{"type":"line-chart","props":{"values":[1,1],"chart":{"fill":true,"fillColor":"#ff000080"},"style":{"height":120,"color":"#ff0000"}}}}`))
	var events []recordedEvent
	host := layoutHost(v, &events)
	defer host.Dispose()
	for _, scale := range []float32{1, 1.5, 2} {
		top := statePixel(t, host, scale, 100, 70)
		bottom := statePixel(t, host, scale, 100, 100)
		if top.A <= bottom.A || bottom.A == 0 {
			t.Fatalf("scale %v: fill did not fade: top=%+v bottom=%+v", scale, top, bottom)
		}
	}
}

func TestLineChartPlotAndValue(t *testing.T) {
	zero := 0
	if got := chartValue(123.62, LineChartOptions{ValueDecimals: &zero, ValuePrefix: "$", ValueSuffix: " USD"}); got != "$124 USD" {
		t.Fatal(got)
	}
	axisWidth := float32(120)
	plot := chartPlot(ui.Rect{Width: 400, Height: 120}, LineChartOptions{YAxis: ChartAxisRight, YAxisWidth: &axisWidth}, component.ControlTheme{})
	if plot.Width != 276 {
		t.Fatalf("custom axis width not reserved: %+v", plot)
	}
	for _, bounds := range []ui.Rect{{Width: 20, Height: 10}, {Width: 0, Height: 0}} {
		plot := chartPlot(bounds, LineChartOptions{YAxis: ChartAxisRight, ShowXAxis: true}, component.ControlTheme{})
		if plot.Width < 0 || plot.Height < 0 || plot.X+plot.Width > bounds.Width || plot.Y+plot.Height > bounds.Height {
			t.Fatalf("invalid small plot: %+v", plot)
		}
	}
}
