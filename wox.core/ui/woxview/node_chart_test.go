package woxview

import (
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"

	component "wox/ui/launcher/component"
	ui "wox/ui/runtime"
	"wox/ui/widget"
)

// TestChartSchemaMatchesRuntime checks independently expressible limits and required fields.
func TestChartSchemaMatchesRuntime(t *testing.T) {
	var schema jsonschema.Schema
	if err := json.Unmarshal(JSONSchema(), &schema); err != nil {
		t.Fatal(err)
	}
	resolved, err := schema.Resolve(nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ kind, props string }{
		{"bar-chart", `{"values":[1,-1],"chart":{"barWidth":0.5,"yAxis":"left"}}`},
		{"bar-chart", `{}`}, {"bar-chart", `{"values":[1],"chart":{"barWidth":2}}`},
		{"pie-chart", `{"values":[1,0],"chart":{"innerRadius":0.5,"tooltipPosition":"floating"}}`},
		{"pie-chart", `{"values":[-1]}`}, {"pie-chart", `{"values":[1],"chart":{"innerRadius":1}}`},
		{"scatter-chart", `{"points":[{"x":1,"y":-1}],"chart":{"xValueDecimals":2,"showXAxis":true}}`},
		{"scatter-chart", `{"points":[{"x":1}]}`}, {"scatter-chart", `{"points":[{"x":null,"y":1}]}`},
		{"scatter-chart", `{"points":[{"x":1,"y":1}],"chart":{"pointRadius":17}}`},
	} {
		wire := []byte(`{"version":1,"root":{"type":"` + tc.kind + `","props":` + tc.props + `}}`)
		var document any
		if err := json.Unmarshal(wire, &document); err != nil {
			t.Fatal(err)
		}
		_, parseErr := Parse(wire)
		schemaErr := resolved.Validate(document)
		if (parseErr == nil) != (schemaErr == nil) {
			t.Errorf("%s %s: Core=%v Schema=%v", tc.kind, tc.props, parseErr, schemaErr)
		}
	}
}

// TestChartContracts covers unrelated use cases as well as invalid untrusted data.
func TestChartContracts(t *testing.T) {
	for _, tc := range []struct{ kind, props string }{
		{"bar-chart", `{"values":[-5,0,10],"pointLabels":["Loss","Flat","Gain"],"chart":{"showGrid":true,"yAxis":"left","barWidth":0.5,"colors":["#f00","$accent","#0f0"]}}`},
		{"scatter-chart", `{"points":[{"x":10,"y":4,"label":"A"},{"x":-2,"y":0}],"chart":{"showXAxis":true,"pointRadius":6,"xValueSuffix":" s","valueSuffix":" m"}}`},
		{"pie-chart", `{"values":[70,30,0],"pointLabels":["Used","Free","Other"],"chart":{"innerRadius":0.6,"colors":["#f00","#0f0","#00f"],"tooltipPosition":"floating"}}`},
		{"bar-chart", `{"values":[]}`}, {"scatter-chart", `{"points":[]}`}, {"pie-chart", `{"values":[0,0]}`},
	} {
		if _, err := Parse([]byte(`{"version":1,"root":{"type":"` + tc.kind + `","props":` + tc.props + `}}`)); err != nil {
			t.Errorf("%s %s: %v", tc.kind, tc.props, err)
		}
	}
	for _, tc := range []struct{ kind, props string }{
		{"bar-chart", `{}`}, {"bar-chart", `{"values":[1],"chart":{"barWidth":0}}`}, {"bar-chart", `{"values":[1,2],"chart":{"colors":["#f00"]}}`},
		{"bar-chart", `{"values":[1],"chart":{"gridLines":1}}`}, {"bar-chart", `{"values":[1],"chart":{"fill":true}}`},
		{"scatter-chart", `{}`}, {"scatter-chart", `{"points":[{"x":1}]}`}, {"scatter-chart", `{"points":[{"x":null,"y":1}]}`},
		{"scatter-chart", `{"points":[{"x":1e101,"y":1}]}`}, {"scatter-chart", `{"points":[{"x":1,"y":1}],"chart":{"pointRadius":0}}`},
		{"scatter-chart", `{"points":[{"x":1,"y":1}],"chart":{"xValueDecimals":7}}`},
		{"pie-chart", `{}`}, {"pie-chart", `{"values":[-1]}`}, {"pie-chart", `{"values":[1],"chart":{"innerRadius":1}}`},
		{"pie-chart", `{"values":[1],"chart":{"colors":["bad"]}}`}, {"pie-chart", `{"values":[` + strings.Repeat("1,", 128) + `1]}`},
	} {
		if _, err := Parse([]byte(`{"version":1,"root":{"type":"` + tc.kind + `","props":` + tc.props + `}}`)); err == nil {
			t.Errorf("accepted %s %s", tc.kind, tc.props)
		}
	}
}

// TestCartesianChartGeometry verifies zero baselines and unordered, repeated X values.
func TestCartesianChartGeometry(t *testing.T) {
	plot := ui.Rect{X: 10, Y: 20, Width: 120, Height: 100}
	points, lo, hi, _, _ := cartesianPoints([]float64{-2, 0, 4}, nil, plot, NodeBarChart)
	if lo != -2 || hi != 4 || points[0] != (ui.Point{X: 30, Y: 120}) || points[2] != (ui.Point{X: 110, Y: 20}) {
		t.Fatalf("bar geometry: %v %v %v", points, lo, hi)
	}
	points, lo, hi, _, _ = cartesianPoints([]float64{5, 5}, nil, plot, NodeBarChart)
	if lo != 0 || hi != 5 || points[0].Y != 20 {
		t.Fatalf("positive flat bars: %v %v %v", points, lo, hi)
	}
	points, _, _, xmin, xmax := cartesianPoints([]float64{1, 3, 2}, []float64{10, -10, 10}, plot, NodeScatterChart)
	if xmin != -10 || xmax != 10 || points[0] != (ui.Point{X: 130, Y: 120}) || points[1] != (ui.Point{X: 10, Y: 20}) || points[2] != (ui.Point{X: 130, Y: 70}) {
		t.Fatalf("scatter geometry: %v", points)
	}
	points, _, _, _, _ = cartesianPoints([]float64{1}, []float64{4}, plot, NodeScatterChart)
	if points[0] != (ui.Point{X: 70, Y: 70}) {
		t.Fatalf("single point: %v", points)
	}
}

// TestBarChartPaintAndHover checks positive/negative columns and categorical hit testing.
func TestBarChartPaintAndHover(t *testing.T) {
	v := NewView()
	v.SetTree(mustParse(t, `{"version":1,"root":{"type":"bar-chart","props":{"values":[2,-1],"pointLabels":["profit","loss"],"chart":{"colors":["#f00","#00f"],"tooltip":false},"style":{"height":120}}}}`))
	theme := component.ControlTheme{TextSecondary: ui.Color{G: 255, A: 255}}
	host := widget.NewHost(func(f ui.FrameInfo) widget.Widget {
		return v.Build(BuildProps{Width: f.Size.Width, Height: f.Size.Height, Theme: theme})
	})
	host.AttachServices(fakeServices{})
	defer host.Dispose()
	for _, scale := range []float32{1, 1.5, 2} {
		if p := statePixel(t, host, scale, 50, 30); p.R < 200 {
			t.Fatalf("scale %v positive bar: %+v", scale, p)
		}
		if p := statePixel(t, host, scale, 150, 100); p.B < 200 {
			t.Fatalf("scale %v negative bar: %+v", scale, p)
		}
		if p := statePixel(t, host, scale, 150, 30); p.A != 0 {
			t.Fatalf("scale %v wrong baseline: %+v", scale, p)
		}
	}
	host.Pointer(ui.PointerEvent{Kind: ui.PointerMove, Position: ui.Point{X: 50, Y: 40}})
	if p := statePixel(t, host, 1, 51, 100); p.G < 200 {
		t.Fatalf("bar crosshair: %+v", p)
	}
	host.Pointer(ui.PointerEvent{Kind: ui.PointerLeave})
	if p := statePixel(t, host, 1, 51, 100); p.A != 0 {
		t.Fatalf("bar hover remained: %+v", p)
	}
}

// TestScatterChartHoverUsesBothAxes ensures duplicate X coordinates select the nearest point.
func TestScatterChartHoverUsesBothAxes(t *testing.T) {
	v := NewView()
	v.SetTree(mustParse(t, `{"version":1,"root":{"type":"scatter-chart","props":{"points":[{"x":-1,"y":2},{"x":1,"y":2},{"x":0,"y":-2},{"x":0,"y":0}],"chart":{"tooltip":false},"style":{"height":120,"color":"#f00"}}}}`))
	theme := component.ControlTheme{TextSecondary: ui.Color{G: 255, A: 255}}
	host := widget.NewHost(func(f ui.FrameInfo) widget.Widget {
		return v.Build(BuildProps{Width: f.Size.Width, Height: f.Size.Height, Theme: theme})
	})
	host.AttachServices(fakeServices{})
	defer host.Dispose()
	statePixel(t, host, 1, 1, 1)
	host.Pointer(ui.PointerEvent{Kind: ui.PointerMove, Position: ui.Point{X: 100, Y: 60}})
	for _, scale := range []float32{1, 1.5, 2} {
		if p := statePixel(t, host, scale, 100, 35); p.G < 200 {
			t.Fatalf("scale %v scatter hover: %+v", scale, p)
		}
	}
	host.Pointer(ui.PointerEvent{Kind: ui.PointerMove, Position: ui.Point{X: 140, Y: 60}})
	if p := statePixel(t, host, 1, 100, 35); p.A != 0 {
		t.Fatalf("scatter selected distant point: %+v", p)
	}
}

// TestScatterChartExtremaKeepWholeMarkers checks the plot inset at each display scale.
func TestScatterChartExtremaKeepWholeMarkers(t *testing.T) {
	v := NewView()
	v.SetTree(mustParse(t, `{"version":1,"root":{"type":"scatter-chart","props":{"points":[{"x":0,"y":0},{"x":1,"y":1}],"style":{"height":120,"color":"#f00"}}}}`))
	var events []recordedEvent
	host := layoutHost(v, &events)
	defer host.Dispose()
	for _, scale := range []float32{1, 1.5, 2} {
		for _, point := range []ui.Point{{X: 3, Y: 114}, {X: 197, Y: 6}} {
			if p := statePixel(t, host, scale, point.X, point.Y); p.R < 200 {
				t.Fatalf("scale %v clipped marker at %+v: %+v", scale, point, p)
			}
		}
	}
}

// TestPieChartSlices verifies clockwise wedges and exclusion of zero categories.
func TestPieChartSlices(t *testing.T) {
	slices := pieSlices([]float64{1, 0, 3})
	if len(slices) != 3 || slices[1].start != slices[1].end || math.Abs(slices[0].end) > 1e-12 {
		t.Fatalf("slices: %+v", slices)
	}
	if len(pieSlices([]float64{0, 0})) != 0 {
		t.Fatal("zero total produced slices")
	}
}

// TestPieChartPaintAndHover checks colors, donut hit testing, and hover cleanup.
func TestPieChartPaintAndHover(t *testing.T) {
	v := NewView()
	v.SetTree(mustParse(t, `{"version":1,"root":{"type":"pie-chart","props":{"values":[1,1],"pointLabels":["used","free"],"chart":{"colors":["#f00","#00f"],"innerRadius":0.5,"tooltip":false},"style":{"height":120}}}}`))
	theme := component.ControlTheme{Text: ui.Color{R: 255, G: 255, B: 255, A: 255}}
	host := widget.NewHost(func(f ui.FrameInfo) widget.Widget {
		return v.Build(BuildProps{Width: f.Size.Width, Height: f.Size.Height, Theme: theme})
	})
	host.AttachServices(fakeServices{})
	defer host.Dispose()
	for _, scale := range []float32{1, 1.5, 2} {
		if p := statePixel(t, host, scale, 140, 60); p.R < 200 {
			t.Fatalf("scale %v right slice: %+v", scale, p)
		}
		if p := statePixel(t, host, scale, 60, 60); p.B < 200 {
			t.Fatalf("scale %v left slice: %+v", scale, p)
		}
		if p := statePixel(t, host, scale, 100, 60); p.A != 0 {
			t.Fatalf("scale %v donut hole: %+v", scale, p)
		}
	}
	host.Pointer(ui.PointerEvent{Kind: ui.PointerMove, Position: ui.Point{X: 140, Y: 60}})
	if p := statePixel(t, host, 1, 140, 60); p.G == 0 {
		t.Fatalf("selected slice not highlighted: %+v", p)
	}
	host.Pointer(ui.PointerEvent{Kind: ui.PointerMove, Position: ui.Point{X: 100, Y: 60}})
	if p := statePixel(t, host, 1, 140, 60); p.G != 0 {
		t.Fatalf("donut hole retained hover: %+v", p)
	}
}

// TestChartHoverTooltip checks local readouts and hover disabling for all new charts.
func TestChartHoverTooltip(t *testing.T) {
	for _, tc := range []struct {
		kind, data string
		x, y       float32
	}{
		{"bar-chart", `"values":[1],"pointLabels":["revenue"]`, 100, 60},
		{"scatter-chart", `"points":[{"x":3,"y":8,"label":"sample"}]`, 100, 60},
		{"pie-chart", `"values":[1],"pointLabels":["storage"]`, 140, 60},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			v := NewView()
			tree := func(chart string) *Tree {
				return mustParse(t, `{"version":1,"root":{"type":"`+tc.kind+`","props":{`+tc.data+`,"chart":`+chart+`,"style":{"height":120,"color":"#f00"}}}}`)
			}
			v.SetTree(tree(`{}`))
			theme := component.ControlTheme{Text: ui.Color{R: 255, G: 255, B: 255, A: 255}, Surface: ui.Color{B: 255, A: 255}}
			host := widget.NewHost(func(f ui.FrameInfo) widget.Widget {
				return v.Build(BuildProps{Width: f.Size.Width, Height: f.Size.Height, Theme: theme})
			})
			host.AttachServices(fakeServices{})
			defer host.Dispose()
			statePixel(t, host, 1, 5, 5)
			host.Pointer(ui.PointerEvent{Kind: ui.PointerMove, Position: ui.Point{X: tc.x, Y: tc.y}})
			if p := statePixel(t, host, 1, 5, 5); p.B < 200 {
				t.Fatalf("hover tooltip missing: %+v", p)
			}
			v.SetTree(tree(`{"hover":false}`))
			if p := statePixel(t, host, 1, 5, 5); p.B != 0 {
				t.Fatalf("disabled hover retained tooltip: %+v", p)
			}
		})
	}
}

// TestChartThemeColors resolves per-item colors for both launcher appearances.
func TestChartThemeColors(t *testing.T) {
	for _, dark := range []bool{false, true} {
		v := NewView()
		v.SetTree(mustParse(t, `{"version":1,"root":{"type":"bar-chart","props":{"values":[1],"chart":{"colors":[{"light":"#f00","dark":"#00f"}]},"style":{"height":120}}}}`))
		host := widget.NewHost(func(f ui.FrameInfo) widget.Widget {
			return v.Build(BuildProps{Width: f.Size.Width, Height: f.Size.Height, Theme: component.ControlTheme{Dark: dark}})
		})
		host.AttachServices(fakeServices{})
		p := statePixel(t, host, 1, 100, 60)
		host.Dispose()
		if (!dark && p.R < 200) || (dark && p.B < 200) {
			t.Fatalf("dark=%v color=%+v", dark, p)
		}
	}
}

// TestChartsNarrowAndAccessible exercises small bounds and the translated image label.
func TestChartsNarrowAndAccessible(t *testing.T) {
	for _, tc := range []struct{ kind, data string }{
		{"bar-chart", `"values":[1,-1],"chart":{"yAxis":"right","showXAxis":true}`},
		{"scatter-chart", `"points":[{"x":1,"y":2}],"chart":{"yAxis":"left","showXAxis":true}`},
		{"pie-chart", `"values":[1,2],"chart":{"innerRadius":0.6}`},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			v := NewView()
			v.SetTree(mustParse(t, `{"version":1,"root":{"type":"`+tc.kind+`","props":{`+tc.data+`,"label":"i18n:chart","style":{"width":20,"height":10}}}}`))
			host := widget.NewHost(func(f ui.FrameInfo) widget.Widget {
				return v.Build(BuildProps{Width: f.Size.Width, Height: f.Size.Height, Theme: component.ControlTheme{}, Translate: func(string) string { return "Chart label" }})
			})
			host.AttachServices(fakeServices{})
			defer host.Dispose()
			statePixel(t, host, 1.5, 5, 5)
			host.Pointer(ui.PointerEvent{Kind: ui.PointerMove, Position: ui.Point{X: 5, Y: 5}})
			statePixel(t, host, 2, 5, 5)
			found := false
			for _, node := range host.Snapshot().Tree.Nodes {
				if node.Role == ui.AccessibilityRoleImage && node.Label == "Chart label" {
					found = true
				}
			}
			if !found {
				t.Fatal("translated image label missing")
			}
		})
	}
}

// TestNewChartXAxisCentersLabels uses host font measurements to align short labels.
func TestNewChartXAxisCentersLabels(t *testing.T) {
	for _, tc := range []struct {
		kind, props string
		x           float32
	}{
		{"bar-chart", `"values":[1,2],"pointLabels":["A","B"]`, 51},
		{"scatter-chart", `"points":[{"x":-1,"y":1},{"x":1,"y":2}]`, 100},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			v := NewView()
			v.SetTree(mustParse(t, `{"version":1,"root":{"type":"`+tc.kind+`","props":{`+tc.props+`,"chart":{"showXAxis":true,"xTicks":3},"style":{"height":120,"color":"#f00"}}}}`))
			theme := component.ControlTheme{TextSecondary: ui.Color{G: 255, A: 255}}
			host := widget.NewHost(func(f ui.FrameInfo) widget.Widget {
				return v.Build(BuildProps{Width: f.Size.Width, Height: f.Size.Height, Theme: theme})
			})
			host.AttachServices(fakeServices{})
			defer host.Dispose()
			if p := statePixel(t, host, 1, tc.x, 108); p.G < 200 {
				t.Fatalf("axis label is not centered at %v: %+v", tc.x, p)
			}
		})
	}
}
