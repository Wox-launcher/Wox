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
func (o *LineChartOptions) validate() error {
	if o == nil {
		return nil
	}
	if o.YAxis != "" && o.YAxis != ChartAxisNone && o.YAxis != ChartAxisLeft && o.YAxis != ChartAxisRight {
		return fmt.Errorf("invalid line-chart yAxis")
	}
	if o.TooltipPosition != "" && o.TooltipPosition != ChartTooltipTop && o.TooltipPosition != ChartTooltipFloating {
		return fmt.Errorf("invalid line-chart tooltipPosition")
	}
	if o.GridLines != 0 && (o.GridLines < 2 || o.GridLines > 12) {
		return fmt.Errorf("line-chart gridLines must be between 2 and 12")
	}
	if o.XTicks != 0 && (o.XTicks < 2 || o.XTicks > 12) {
		return fmt.Errorf("line-chart xTicks must be between 2 and 12")
	}
	if o.LineWidth != nil && (math.IsNaN(float64(*o.LineWidth)) || *o.LineWidth <= 0 || *o.LineWidth > 16) {
		return fmt.Errorf("line-chart lineWidth must be greater than 0 and at most 16")
	}
	if o.PlotPadding != nil && (math.IsNaN(float64(*o.PlotPadding)) || *o.PlotPadding < 0 || *o.PlotPadding > 128) {
		return fmt.Errorf("line-chart plotPadding must be between 0 and 128")
	}
	if o.YAxisWidth != nil && (math.IsNaN(float64(*o.YAxisWidth)) || *o.YAxisWidth < 0 || *o.YAxisWidth > 512) {
		return fmt.Errorf("line-chart yAxisWidth must be between 0 and 512")
	}
	if o.ValueDecimals != nil && (*o.ValueDecimals < 0 || *o.ValueDecimals > 6) {
		return fmt.Errorf("line-chart valueDecimals must be between 0 and 6")
	}
	if utf8.RuneCountInString(o.ValuePrefix) > 32 || utf8.RuneCountInString(o.ValueSuffix) > 32 {
		return fmt.Errorf("line-chart value prefix or suffix exceeds 32 characters")
	}
	if err := o.GridColor.validate(); err != nil {
		return fmt.Errorf("line-chart gridColor: %w", err)
	}
	if err := o.FillColor.validate(); err != nil {
		return fmt.Errorf("line-chart fillColor: %w", err)
	}
	return nil
}

func (p *LineChartProps) prepare() error {
	if p.Values == nil {
		return fmt.Errorf("line-chart requires values")
	}
	if len(p.Values) > 4096 {
		return fmt.Errorf("line-chart accepts at most 4096 values")
	}
	if len(p.PointLabels) != 0 && len(p.PointLabels) != len(p.Values) {
		return fmt.Errorf("line-chart pointLabels must match values")
	}
	for _, label := range p.PointLabels {
		if utf8.RuneCountInString(label) > 128 {
			return fmt.Errorf("line-chart point label exceeds 128 characters")
		}
	}
	for _, value := range p.Values {
		if math.IsNaN(value) || math.IsInf(value, 0) || math.Abs(value) > 1e100 {
			return fmt.Errorf("line-chart values must be finite and within ±1e100")
		}
	}
	if err := p.Chart.validate(); err != nil {
		return err
	}
	return p.Style.validate()
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

type lineChartConfig struct {
	props *LineChartProps
	id    string
	theme component.ControlTheme
}

type lineChartState struct {
	selected  int
	plotLeft  float32
	plotWidth float32
}

func (s *lineChartState) InitState(_ widget.StateContext, _ any) { s.selected = -1 }
func (s *lineChartState) DidUpdateWidget(_ widget.StateContext, _, next any) {
	p := next.(lineChartConfig).props
	if s.selected >= len(p.Values) || (p.Chart != nil && !chartOption(p.Chart.Hover)) {
		s.selected = -1
	}
}
func (s *lineChartState) Dispose() {}

// buildLineChart retains hover state locally, so pointer motion does not call the plugin.
func buildLineChart(b *builder, node *Node, id string) widget.Widget {
	p := node.Props.(*LineChartProps)
	theme := b.theme()
	config := lineChartConfig{props: p, id: id, theme: theme}
	return p.Style.wrapControl(theme, widget.Semantics{Role: ui.AccessibilityRoleImage, Label: b.translate(p.Label), Child: widget.Stateful{
		Key: widget.Key(id), Type: (*lineChartState)(nil), Widget: config,
		CreateState: func() widget.State { return &lineChartState{} },
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

func (s *lineChartState) Build(ctx widget.StateContext, value any) widget.Widget {
	config := value.(lineChartConfig)
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
	return widget.LayoutBuilder{Build: func(available ui.Size) widget.Widget {
		width := widthOr(style, available)
		selected := s.selected
		painter := widget.Painter{Width: width, Height: height, Paint: func(list *ui.DisplayList, bounds ui.Rect) {
			list.PushClipRect(bounds)
			defer list.PopClipRect()
			plot := chartPlot(bounds, options, theme)
			// Pointer positions are local to the gesture, while painter bounds can have an origin.
			s.plotLeft, s.plotWidth = plot.X-bounds.X, plot.Width
			points := lineChartPoints(p.Values, plot)
			if len(points) == 0 {
				return
			}
			low, high := p.Values[0], p.Values[0]
			for _, value := range p.Values[1:] {
				low, high = min(low, value), max(high, value)
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
			if options.ShowXAxis && len(p.PointLabels) > 0 && plot.Width > 0 {
				ticks := options.XTicks
				if ticks == 0 {
					ticks = 5
				}
				ticks = min(ticks, len(points), max(1, int(plot.Width/theme.Scaled(80))+1))
				for i := 0; i < ticks; i++ {
					index := 0
					if ticks > 1 {
						index = int(math.Round(float64(i) * float64(len(points)-1) / float64(ticks-1)))
					}
					labelWidth := min(theme.Scaled(80), bounds.Width)
					x := min(max(points[index].X-labelWidth/2, bounds.X), bounds.X+bounds.Width-labelWidth)
					rect := ui.Rect{X: x, Y: plot.Y + plot.Height + theme.Scaled(4), Width: labelWidth, Height: theme.Scaled(14)}
					list.PushClipRect(rect)
					list.DrawText(p.PointLabels[index], rect, textStyle, theme.TextSecondary)
					list.PopClipRect()
				}
			}
			if options.Fill && len(points) > 1 && plot.Width > 0 && plot.Height > 0 {
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
			if selected < 0 || selected >= len(points) || !chartOption(options.Hover) {
				return
			}
			point := points[selected]
			if chartOption(options.Crosshair) {
				list.FillRect(ui.Rect{X: point.X, Y: plot.Y, Width: 1, Height: plot.Height}, theme.TextSecondary)
			}
			if chartOption(options.HoverPoint) {
				list.FillRoundedRect(ui.Rect{X: point.X - 4, Y: point.Y - 4, Width: 8, Height: 8}, 4, color)
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
			boxWidth := min(bounds.Width, theme.Scaled(210))
			boxHeight := min(bounds.Height, theme.Scaled(25))
			x := min(max(point.X+8, bounds.X), bounds.X+bounds.Width-boxWidth)
			y := bounds.Y
			if options.TooltipPosition == ChartTooltipFloating {
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
			if options.TooltipPosition == ChartTooltipFloating && pointLabel != "" {
				list.DrawText(pointLabel, ui.Rect{X: x + theme.Scaled(6), Y: y + theme.Scaled(4), Width: boxWidth - theme.Scaled(12), Height: theme.Scaled(14)}, textStyle, theme.TextSecondary)
				list.DrawText(label, ui.Rect{X: x + theme.Scaled(6), Y: y + theme.Scaled(21), Width: boxWidth - theme.Scaled(12), Height: theme.Scaled(16)}, ui.TextStyle{Size: theme.Scaled(12), Weight: ui.FontWeightSemibold}, theme.Text)
			} else {
				if pointLabel != "" {
					label = pointLabel + "  ·  " + label
				}
				list.DrawText(label, ui.Rect{X: x + theme.Scaled(6), Y: y + theme.Scaled(4), Width: boxWidth - theme.Scaled(12), Height: boxHeight - theme.Scaled(8)}, textStyle, theme.Text)
			}
			list.PopClipRect()
		}}
		return widget.Gesture{ID: config.id + "/chart", Child: painter, OnPointer: func(event ui.PointerEvent) bool {
			next := -1
			if chartOption(options.Hover) && event.Kind != ui.PointerLeave && len(p.Values) > 0 && s.plotWidth > 0 {
				ratio := min(max((event.Position.X-s.plotLeft)/s.plotWidth, 0), 1)
				next = int(math.Round(float64(ratio) * float64(len(p.Values)-1)))
			}
			if next != s.selected {
				ctx.SetState(func() { s.selected = next })
			}
			return false
		}}
	}}
}
