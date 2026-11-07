package woxview

import (
	"fmt"
	"math"
	"unicode/utf8"

	component "wox/ui/launcher/component"
	ui "wox/ui/runtime"
	"wox/ui/widget"
)

// LineChartProps plots evenly spaced numeric samples in their supplied order.
type LineChartProps struct {
	Styled
	Values      []float64         `json:"values"`
	PointLabels []string          `json:"pointLabels,omitempty"`
	Label       string            `json:"label,omitempty"`
	Chart       *LineChartOptions `json:"chart,omitempty"`
}

// ChartAxisPosition selects the side on which numeric labels are displayed.
type ChartAxisPosition string

const (
	ChartAxisNone  ChartAxisPosition = "none"
	ChartAxisLeft  ChartAxisPosition = "left"
	ChartAxisRight ChartAxisPosition = "right"
)

func (ChartAxisPosition) enumValues() []string { return []string{"", "none", "left", "right"} }

// ChartTooltipPosition selects the legacy top readout or a point-anchored tooltip.
type ChartTooltipPosition string

const (
	ChartTooltipTop      ChartTooltipPosition = "top"
	ChartTooltipFloating ChartTooltipPosition = "floating"
)

func (ChartTooltipPosition) enumValues() []string { return []string{"", "top", "floating"} }

// LineChartOptions controls chart presentation without changing legacy defaults.
type LineChartOptions struct {
	ShowGrid        bool                 `json:"showGrid,omitempty"`
	ShowXAxis       bool                 `json:"showXAxis,omitempty"`
	YAxis           ChartAxisPosition    `json:"yAxis,omitempty"`
	YAxisWidth      *float32             `json:"yAxisWidth,omitempty"`
	GridLines       int                  `json:"gridLines,omitempty"`
	XTicks          int                  `json:"xTicks,omitempty"`
	GridColor       Color                `json:"gridColor,omitempty,omitzero"`
	LineWidth       *float32             `json:"lineWidth,omitempty"`
	ShowPoints      bool                 `json:"showPoints,omitempty"`
	Fill            bool                 `json:"fill,omitempty"`
	FillColor       Color                `json:"fillColor,omitempty,omitzero"`
	PlotPadding     *float32             `json:"plotPadding,omitempty"`
	Hover           *bool                `json:"hover,omitempty"`
	Crosshair       *bool                `json:"crosshair,omitempty"`
	HoverPoint      *bool                `json:"hoverPoint,omitempty"`
	Tooltip         *bool                `json:"tooltip,omitempty"`
	TooltipPosition ChartTooltipPosition `json:"tooltipPosition,omitempty"`
	ValueDecimals   *int                 `json:"valueDecimals,omitempty"`
	ValuePrefix     string               `json:"valuePrefix,omitempty"`
	ValueSuffix     string               `json:"valueSuffix,omitempty"`
}

// validate bounds paint work and prevents unreadable or invalid layout values.
func (o *LineChartOptions) validate(kind NodeType) error {
	if o == nil {
		return nil
	}
	if o.YAxis != "" && o.YAxis != ChartAxisNone && o.YAxis != ChartAxisLeft && o.YAxis != ChartAxisRight {
		return fmt.Errorf("invalid %s yAxis", kind)
	}
	if o.TooltipPosition != "" && o.TooltipPosition != ChartTooltipTop && o.TooltipPosition != ChartTooltipFloating {
		return fmt.Errorf("invalid %s tooltipPosition", kind)
	}
	if o.GridLines != 0 && (o.GridLines < 2 || o.GridLines > 12) {
		return fmt.Errorf("%s gridLines must be between 2 and 12", kind)
	}
	if o.XTicks != 0 && (o.XTicks < 2 || o.XTicks > 12) {
		return fmt.Errorf("%s xTicks must be between 2 and 12", kind)
	}
	if o.LineWidth != nil && (math.IsNaN(float64(*o.LineWidth)) || *o.LineWidth <= 0 || *o.LineWidth > 16) {
		return fmt.Errorf("%s lineWidth must be greater than 0 and at most 16", kind)
	}
	if o.PlotPadding != nil && (math.IsNaN(float64(*o.PlotPadding)) || *o.PlotPadding < 0 || *o.PlotPadding > 128) {
		return fmt.Errorf("%s plotPadding must be between 0 and 128", kind)
	}
	if o.YAxisWidth != nil && (math.IsNaN(float64(*o.YAxisWidth)) || *o.YAxisWidth < 0 || *o.YAxisWidth > 512) {
		return fmt.Errorf("%s yAxisWidth must be between 0 and 512", kind)
	}
	if o.ValueDecimals != nil && (*o.ValueDecimals < 0 || *o.ValueDecimals > 6) {
		return fmt.Errorf("%s valueDecimals must be between 0 and 6", kind)
	}
	if utf8.RuneCountInString(o.ValuePrefix) > 32 || utf8.RuneCountInString(o.ValueSuffix) > 32 {
		return fmt.Errorf("%s value prefix or suffix exceeds 32 characters", kind)
	}
	if err := o.GridColor.validate(); err != nil {
		return fmt.Errorf("%s gridColor: %w", kind, err)
	}
	if err := o.FillColor.validate(); err != nil {
		return fmt.Errorf("%s fillColor: %w", kind, err)
	}
	return nil
}

func (p *LineChartProps) prepare() error {
	if err := validateChartValues(NodeLineChart, p.Values, p.PointLabels, 4096); err != nil {
		return err
	}
	if err := p.Chart.validate(NodeLineChart); err != nil {
		return err
	}
	return p.Style.validate()
}

// validateChartValues enforces common untrusted data and label limits.
func validateChartValues(kind NodeType, values []float64, labels []string, limit int) error {
	if values == nil {
		return fmt.Errorf("%s requires values", kind)
	}
	if len(values) > limit {
		return fmt.Errorf("%s accepts at most %d values", kind, limit)
	}
	if len(labels) != 0 && len(labels) != len(values) {
		return fmt.Errorf("%s pointLabels must match values", kind)
	}
	for _, label := range labels {
		if utf8.RuneCountInString(label) > 128 {
			return fmt.Errorf("%s point label exceeds 128 characters", kind)
		}
	}
	for _, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) || math.Abs(value) > 1e100 {
			return fmt.Errorf("%s values must be finite and within ±1e100", kind)
		}
	}
	return nil
}

// lineChartPoints normalizes the data to the available logical bounds, including flat series.
func lineChartPoints(values []float64, bounds ui.Rect) []ui.Point {
	if len(values) == 0 || bounds.Width <= 0 || bounds.Height <= 0 {
		return nil
	}
	low, high := values[0], values[0]
	for _, value := range values[1:] {
		low, high = min(low, value), max(high, value)
	}
	points := make([]ui.Point, len(values))
	for i, value := range values {
		x, y := float64(.5), float64(.5)
		if len(values) > 1 {
			x = float64(i) / float64(len(values)-1)
		}
		if high != low {
			y = 1 - (value-low)/(high-low)
		}
		points[i] = ui.Point{X: bounds.X + float32(x)*bounds.Width, Y: bounds.Y + float32(y)*bounds.Height}
	}
	return points
}

type cartesianChartConfig struct {
	props       *LineChartProps
	id          string
	theme       component.ControlTheme
	kind        NodeType
	barWidth    float32
	pointRadius float32
	xValues     []float64
	colors      []Color
	xOptions    LineChartOptions
}

type cartesianChartState struct {
	selected  int
	plotLeft  float32
	plotWidth float32
	points    []ui.Point
}

func (s *cartesianChartState) InitState(_ widget.StateContext, _ any) { s.selected = -1 }
func (s *cartesianChartState) DidUpdateWidget(_ widget.StateContext, old, next any) {
	previous, config := old.(cartesianChartConfig), next.(cartesianChartConfig)
	p := config.props
	if previous.kind != config.kind || s.selected >= len(p.Values) || (p.Chart != nil && !chartOption(p.Chart.Hover)) {
		s.selected = -1
	}
}
func (s *cartesianChartState) Dispose() {}

// buildLineChart retains hover state locally, so pointer motion does not call the plugin.
func buildLineChart(b *builder, node *Node, id string) widget.Widget {
	return buildCartesianChart(b, cartesianChartConfig{props: node.Props.(*LineChartProps), id: id, kind: NodeLineChart})
}

// buildCartesianChart gives each chart an isolated local hover state and image semantics.
func buildCartesianChart(b *builder, config cartesianChartConfig) widget.Widget {
	p, id, theme := config.props, config.id, b.theme()
	config.theme = theme
	return p.Style.wrapControl(theme, widget.Semantics{Role: ui.AccessibilityRoleImage, Label: b.translate(p.Label), Child: widget.Stateful{
		Key: widget.Key(id), Type: (*cartesianChartState)(nil), Widget: config,
		CreateState: func() widget.State { return &cartesianChartState{} },
	}})
}

// chartOption defaults optional interaction switches to their legacy enabled state.
func chartOption(value *bool) bool { return value == nil || *value }

// chartValue uses scientific notation for extreme values to keep labels bounded.
func chartValue(value float64, options LineChartOptions) string {
	decimals := 2
	if options.ValueDecimals != nil {
		decimals = *options.ValueDecimals
	}
	format := "%.*f"
	if math.Abs(value) >= 1e9 || (value != 0 && math.Abs(value) < 1e-6) {
		format = "%.*g"
		decimals = max(decimals, 1)
	}
	return options.ValuePrefix + fmt.Sprintf(format, decimals, value) + options.ValueSuffix
}

// chartPlot reserves axis space inside the painter and keeps small charts nonnegative.
func chartPlot(bounds ui.Rect, options LineChartOptions, theme component.ControlTheme) ui.Rect {
	padding := min(float32(2), min(bounds.Width, bounds.Height)/2)
	if options.PlotPadding != nil {
		padding = *options.PlotPadding
	}
	left, right, top, bottom := padding, padding, padding, padding
	axisWidth := theme.Scaled(80)
	if options.YAxisWidth != nil {
		axisWidth = *options.YAxisWidth
	}
	if options.YAxis == ChartAxisLeft {
		left += axisWidth
	}
	if options.YAxis == ChartAxisRight {
		right += axisWidth
	}
	if options.ShowXAxis {
		bottom += theme.Scaled(20)
	}
	if options.YAxis == ChartAxisLeft || options.YAxis == ChartAxisRight {
		top += theme.Scaled(7)
		bottom += theme.Scaled(7)
	}
	left, right = min(left, bounds.Width/2), min(right, bounds.Width/2)
	top, bottom = min(top, bounds.Height/2), min(bottom, bounds.Height/2)
	return ui.Rect{X: bounds.X + left, Y: bounds.Y + top, Width: max(0, bounds.Width-left-right), Height: max(0, bounds.Height-top-bottom)}
}

func (s *cartesianChartState) Build(ctx widget.StateContext, value any) widget.Widget {
	config := value.(cartesianChartConfig)
	p, theme := config.props, config.theme
	options := LineChartOptions{}
	if p.Chart != nil {
		options = *p.Chart
	}
	style := p.Style.resolve(theme)
	height := style.Height
	if height <= 0 {
		height = 160
	}
	color := style.Color.resolve(theme, theme.Accent)
	lineWidth := float32(2)
	if options.LineWidth != nil {
		lineWidth = *options.LineWidth
	}
	return widget.TextLayoutBuilder{Build: func(available ui.Size, measurer widget.TextMeasurer) widget.Widget {
		width := widthOr(style, available)
		selected := s.selected
		painter := widget.Painter{Width: width, Height: height, Paint: func(list *ui.DisplayList, bounds ui.Rect) {
			list.PushClipRect(bounds)
			defer list.PopClipRect()
			plot := chartPlot(bounds, options, theme)
			if config.kind == NodeScatterChart {
				// Reserve room for markers at the extrema so their circles are not clipped.
				inset := min(config.pointRadius, plot.Width/2, plot.Height/2)
				plot.X, plot.Y = plot.X+inset, plot.Y+inset
				plot.Width, plot.Height = max(0, plot.Width-2*inset), max(0, plot.Height-2*inset)
			}
			// Pointer positions are local to the gesture, while painter bounds can have an origin.
			s.plotLeft, s.plotWidth = plot.X-bounds.X, plot.Width
			points, low, high, xLow, xHigh := cartesianPoints(p.Values, config.xValues, plot, config.kind)
			if config.kind == NodeScatterChart {
				s.points = make([]ui.Point, len(points))
				for i, point := range points {
					s.points[i] = ui.Point{X: point.X - bounds.X, Y: point.Y - bounds.Y}
				}
			}
			if len(points) == 0 {
				return
			}
			gridLines := options.GridLines
			if gridLines == 0 {
				gridLines = 5
			}
			gridColor := options.GridColor.resolve(theme, theme.Border)
			textStyle := ui.TextStyle{Size: theme.Scaled(12)}
			lastLabelY := float32(-math.MaxFloat32)
			for i := 0; i < gridLines; i++ {
				fraction := float32(i) / float32(gridLines-1)
				y := plot.Y + fraction*plot.Height
				if options.ShowGrid {
					list.FillRect(ui.Rect{X: plot.X, Y: y, Width: plot.Width, Height: 1}, gridColor)
				}
				if (options.YAxis == ChartAxisLeft || options.YAxis == ChartAxisRight) && (low != high || i == gridLines/2) {
					labelY := y
					if low == high {
						labelY = plot.Y + plot.Height/2
					}
					if labelY-lastLabelY < theme.Scaled(14) {
						continue
					}
					lastLabelY = labelY
					label := chartValue(high-(high-low)*float64(fraction), options)
					x := plot.X + plot.Width + theme.Scaled(6)
					if options.YAxis == ChartAxisLeft {
						x = bounds.X
					}
					labelWidth := bounds.X + bounds.Width - x
					if options.YAxis == ChartAxisLeft {
						labelWidth = plot.X - bounds.X - theme.Scaled(6)
					}
					labelWidth = max(0, labelWidth)
					list.PushClipRect(ui.Rect{X: x, Y: bounds.Y, Width: labelWidth, Height: bounds.Height})
					list.DrawText(label, ui.Rect{X: x, Y: labelY - theme.Scaled(7), Width: labelWidth, Height: theme.Scaled(14)}, textStyle, theme.TextSecondary)
					list.PopClipRect()
				}
			}
			if options.ShowXAxis && plot.Width > 0 {
				ticks := options.XTicks
				if ticks == 0 {
					ticks = 5
				}
				ticks = min(ticks, max(1, int(plot.Width/theme.Scaled(80))+1))
				if config.kind != NodeScatterChart {
					ticks = min(ticks, len(p.PointLabels))
				}
				if config.kind == NodeScatterChart && xLow == xHigh {
					ticks = 1
				}
				for i := 0; i < ticks; i++ {
					fraction := float64(0.5)
					if ticks > 1 {
						fraction = float64(i) / float64(ticks-1)
					}
					var x float32
					var label string
					if config.kind == NodeScatterChart {
						x = plot.X + float32(fraction)*plot.Width
						label = chartValue(xLow+(xHigh-xLow)*fraction, config.xOptions)
					} else {
						index := 0
						if ticks > 1 {
							index = int(math.Round(fraction * float64(len(points)-1)))
						}
						x, label = points[index].X, p.PointLabels[index]
					}
					labelWidth := min(theme.Scaled(80), bounds.Width)
					// Existing line charts retain their original label positioning for compatibility.
					if config.kind != NodeLineChart && measurer != nil {
						if metrics, err := measurer.MeasureText(label, textStyle); err == nil && metrics.Size.Width > 0 {
							labelWidth = min(labelWidth, metrics.Size.Width+1)
						}
					}
					if config.kind == NodeLineChart {
						x = min(max(x-labelWidth/2, bounds.X), bounds.X+bounds.Width-labelWidth)
					} else {
						labelWidth = min(labelWidth, plot.Width)
						x = min(max(x-labelWidth/2, plot.X), plot.X+plot.Width-labelWidth)
					}
					rect := ui.Rect{X: x, Y: plot.Y + plot.Height + theme.Scaled(4), Width: labelWidth, Height: theme.Scaled(14)}
					list.PushClipRect(rect)
					list.DrawText(label, rect, textStyle, theme.TextSecondary)
					list.PopClipRect()
				}
			}
			if config.kind == NodeLineChart && options.Fill && len(points) > 1 && plot.Width > 0 && plot.Height > 0 {
				fill := color
				fill.A = uint8(float32(fill.A) * 0.22)
				fill = options.FillColor.resolve(theme, fill)
				// The shared edge-fade compositor preserves translucent backgrounds on every renderer.
				list.BeginEdgeFade(plot, 0, plot.Height/2)
				for i := 1; i < len(points); i++ {
					a, b := points[i-1], points[i]
					bottom := plot.Y + plot.Height
					// A point on the baseline would duplicate a trapezoid vertex, which the renderer rejects.
					if a.Y == bottom && b.Y == bottom {
						continue
					}
					vertices := []ui.Point{a, b}
					if b.Y != bottom {
						vertices = append(vertices, ui.Point{X: b.X, Y: bottom})
					}
					if a.Y != bottom {
						vertices = append(vertices, ui.Point{X: a.X, Y: bottom})
					}
					list.FillConvexPolygon(vertices, fill)
				}
				list.EndEdgeFade(plot, 0, plot.Height/2)
			}
			if config.kind == NodeBarChart {
				baseline := plot.Y + plot.Height
				if high != low {
					baseline = plot.Y + float32(high/(high-low))*plot.Height
				}
				barWidth := plot.Width / float32(len(points)) * config.barWidth
				for i, point := range points {
					rect := ui.Rect{X: point.X - barWidth/2, Y: min(point.Y, baseline), Width: barWidth, Height: float32(math.Abs(float64(point.Y - baseline)))}
					list.FillRect(rect, chartItemColor(config.colors, i, theme, color))
				}
			} else if config.kind == NodeScatterChart {
				for i, point := range points {
					r := config.pointRadius
					list.FillRoundedRect(ui.Rect{X: point.X - r, Y: point.Y - r, Width: r * 2, Height: r * 2}, r, chartItemColor(config.colors, i, theme, color))
				}
			} else {
				if len(points) == 1 {
					list.FillRoundedRect(ui.Rect{X: points[0].X - 2, Y: points[0].Y - 2, Width: 4, Height: 4}, 2, color)
				}
				for i := 1; i < len(points); i++ {
					a, b := points[i-1], points[i]
					dx, dy := b.X-a.X, b.Y-a.Y
					length := float32(math.Hypot(float64(dx), float64(dy)))
					if length == 0 {
						continue
					}
					nx, ny := -dy/length*lineWidth/2, dx/length*lineWidth/2
					list.FillConvexPolygon([]ui.Point{{X: a.X + nx, Y: a.Y + ny}, {X: b.X + nx, Y: b.Y + ny}, {X: b.X - nx, Y: b.Y - ny}, {X: a.X - nx, Y: a.Y - ny}}, color)
				}
				if options.ShowPoints && len(points) > 1 {
					for _, point := range points {
						list.FillRoundedRect(ui.Rect{X: point.X - 2, Y: point.Y - 2, Width: 4, Height: 4}, 2, color)
					}
				}
			}
			if selected < 0 || selected >= len(points) || !chartOption(options.Hover) {
				return
			}
			point := points[selected]
			selectedColor := chartItemColor(config.colors, selected, theme, color)
			if chartOption(options.Crosshair) {
				list.FillRect(ui.Rect{X: point.X, Y: plot.Y, Width: 1, Height: plot.Height}, theme.TextSecondary)
			}
			if chartOption(options.HoverPoint) {
				radius := float32(4)
				if config.kind != NodeLineChart {
					radius = theme.Scaled(4)
				}
				if config.kind == NodeScatterChart {
					radius = max(radius, config.pointRadius)
				}
				if config.kind != NodeLineChart {
					outer := radius + theme.Scaled(2)
					list.FillRoundedRect(ui.Rect{X: point.X - outer, Y: point.Y - outer, Width: outer * 2, Height: outer * 2}, outer, theme.Text)
				}
				list.FillRoundedRect(ui.Rect{X: point.X - radius, Y: point.Y - radius, Width: radius * 2, Height: radius * 2}, radius, selectedColor)
			}
			if !chartOption(options.Tooltip) {
				return
			}
			label := fmt.Sprintf("%.2f", p.Values[selected])
			if options.ValueDecimals != nil || options.ValuePrefix != "" || options.ValueSuffix != "" {
				label = chartValue(p.Values[selected], options)
			}
			pointLabel := ""
			if len(p.PointLabels) > selected {
				pointLabel = p.PointLabels[selected]
			}
			if config.kind == NodeScatterChart {
				label = chartValue(config.xValues[selected], config.xOptions) + "  ·  " + chartValue(p.Values[selected], options)
			}
			drawChartTooltip(list, bounds, point, pointLabel, label, options.TooltipPosition, theme)

		}}
		return widget.Gesture{ID: config.id + "/chart", Child: painter, OnPointer: func(event ui.PointerEvent) bool {
			next := -1
			if chartOption(options.Hover) && event.Kind != ui.PointerLeave && len(p.Values) > 0 && s.plotWidth > 0 {
				ratio := min(max((event.Position.X-s.plotLeft)/s.plotWidth, 0), 1)
				switch config.kind {
				case NodeBarChart:
					next = min(int(ratio*float32(len(p.Values))), len(p.Values)-1)
				case NodeScatterChart:
					// XY points can be unordered or share an X value, so select by two-dimensional distance.
					distance := float64(max(theme.Scaled(12), config.pointRadius+theme.Scaled(4)))
					for i, point := range s.points {
						d := math.Hypot(float64(event.Position.X-point.X), float64(event.Position.Y-point.Y))
						if d <= distance {
							next, distance = i, d
						}
					}
				default:
					next = int(math.Round(float64(ratio) * float64(len(p.Values)-1)))
				}
			}
			if next != s.selected {
				ctx.SetState(func() { s.selected = next })
			}
			return false
		}}
	}}
}

// drawChartTooltip keeps local hover readouts inside the chart bounds.
func drawChartTooltip(list *ui.DisplayList, bounds ui.Rect, point ui.Point, pointLabel, label string, position ChartTooltipPosition, theme component.ControlTheme) {
	boxWidth := min(bounds.Width, theme.Scaled(210))
	boxHeight := min(bounds.Height, theme.Scaled(25))
	x := min(max(point.X+8, bounds.X), bounds.X+bounds.Width-boxWidth)
	y := bounds.Y
	if position == ChartTooltipFloating {
		boxWidth = min(bounds.Width, theme.Scaled(160))
		x = min(max(point.X+8, bounds.X), bounds.X+bounds.Width-boxWidth)
		if pointLabel != "" {
			boxHeight = min(bounds.Height, theme.Scaled(42))
		}
		if point.X+8+boxWidth > bounds.X+bounds.Width {
			x = max(bounds.X, point.X-8-boxWidth)
		}
		y = max(bounds.Y, point.Y-boxHeight-theme.Scaled(8))
		if point.Y-boxHeight-theme.Scaled(8) < bounds.Y {
			y = min(bounds.Y+bounds.Height-boxHeight, point.Y+theme.Scaled(8))
		}
	}
	box := ui.Rect{X: x, Y: y, Width: boxWidth, Height: boxHeight}
	list.FillRoundedRect(box, theme.Scaled(4), theme.Surface)
	list.PushClipRect(box)
	if position == ChartTooltipFloating && pointLabel != "" {
		list.DrawText(pointLabel, ui.Rect{X: x + theme.Scaled(6), Y: y + theme.Scaled(4), Width: boxWidth - theme.Scaled(12), Height: theme.Scaled(14)}, ui.TextStyle{Size: theme.Scaled(12)}, theme.TextSecondary)
		list.DrawText(label, ui.Rect{X: x + theme.Scaled(6), Y: y + theme.Scaled(21), Width: boxWidth - theme.Scaled(12), Height: theme.Scaled(16)}, ui.TextStyle{Size: theme.Scaled(12), Weight: ui.FontWeightSemibold}, theme.Text)
	} else {
		if pointLabel != "" {
			label = pointLabel + "  ·  " + label
		}
		list.DrawText(label, ui.Rect{X: x + theme.Scaled(6), Y: y + theme.Scaled(4), Width: boxWidth - theme.Scaled(12), Height: boxHeight - theme.Scaled(8)}, ui.TextStyle{Size: theme.Scaled(12)}, theme.Text)
	}
	list.PopClipRect()
}
