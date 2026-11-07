package woxview

import (
	"fmt"
	"math"
	"unicode/utf8"

	component "wox/ui/launcher/component"
	ui "wox/ui/runtime"
	"wox/ui/widget"
)

// CartesianChartOptions contains presentation shared by categorical and XY charts.
type CartesianChartOptions struct {
	ShowGrid        bool                 `json:"showGrid,omitempty"`
	ShowXAxis       bool                 `json:"showXAxis,omitempty"`
	YAxis           ChartAxisPosition    `json:"yAxis,omitempty"`
	YAxisWidth      *float32             `json:"yAxisWidth,omitempty"`
	GridLines       int                  `json:"gridLines,omitempty"`
	XTicks          int                  `json:"xTicks,omitempty"`
	GridColor       Color                `json:"gridColor,omitempty,omitzero"`
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

// lineOptions reuses the existing Cartesian rendering and validation contract.
func (o CartesianChartOptions) lineOptions() LineChartOptions {
	return LineChartOptions{ShowGrid: o.ShowGrid, ShowXAxis: o.ShowXAxis, YAxis: o.YAxis, YAxisWidth: o.YAxisWidth,
		GridLines: o.GridLines, XTicks: o.XTicks, GridColor: o.GridColor, PlotPadding: o.PlotPadding,
		Hover: o.Hover, Crosshair: o.Crosshair, HoverPoint: o.HoverPoint, Tooltip: o.Tooltip,
		TooltipPosition: o.TooltipPosition, ValueDecimals: o.ValueDecimals, ValuePrefix: o.ValuePrefix, ValueSuffix: o.ValueSuffix}
}

// BarChartProps displays one bar per categorical value, using a zero baseline.
type BarChartProps struct {
	Styled
	Values      []float64        `json:"values"`
	PointLabels []string         `json:"pointLabels,omitempty"`
	Label       string           `json:"label,omitempty"`
	Chart       *BarChartOptions `json:"chart,omitempty"`
}

// BarChartOptions extends the shared Cartesian options with category appearance.
type BarChartOptions struct {
	CartesianChartOptions
	BarWidth *float32 `json:"barWidth,omitempty"`
	Colors   []Color  `json:"colors,omitempty"`
}

func (p *BarChartProps) prepare() error {
	if err := validateChartValues(NodeBarChart, p.Values, p.PointLabels, 4096); err != nil {
		return err
	}
	options := LineChartOptions{}
	if p.Chart != nil {
		options = p.Chart.lineOptions()
		if p.Chart.BarWidth != nil && (math.IsNaN(float64(*p.Chart.BarWidth)) || *p.Chart.BarWidth <= 0 || *p.Chart.BarWidth > 1) {
			return fmt.Errorf("bar-chart barWidth must be greater than 0 and at most 1")
		}
		if err := validateChartColors(p.Chart.Colors, len(p.Values)); err != nil {
			return err
		}
	}
	if err := options.validate(NodeBarChart); err != nil {
		return err
	}
	return p.Style.validate()
}

// buildBarChart shares Cartesian geometry and interaction with the line chart.
func buildBarChart(b *builder, node *Node, id string) widget.Widget {
	p := node.Props.(*BarChartProps)
	options := LineChartOptions{}
	var colors []Color
	width := float32(0.7)
	if p.Chart != nil {
		options = p.Chart.lineOptions()
		colors = p.Chart.Colors
		if p.Chart.BarWidth != nil {
			width = *p.Chart.BarWidth
		}
	}
	props := &LineChartProps{Styled: p.Styled, Values: p.Values, PointLabels: p.PointLabels, Label: p.Label, Chart: &options}
	return buildCartesianChart(b, cartesianChartConfig{props: props, id: id, kind: NodeBarChart, barWidth: width, colors: colors})
}

// ScatterPoint is one independent XY observation, optionally labeled for hover.
type ScatterPoint struct {
	X     *float64 `json:"x"`
	Y     *float64 `json:"y"`
	Label string   `json:"label,omitempty"`
}

// ScatterChartProps displays numeric XY observations in any order.
type ScatterChartProps struct {
	Styled
	Points []ScatterPoint       `json:"points"`
	Label  string               `json:"label,omitempty"`
	Chart  *ScatterChartOptions `json:"chart,omitempty"`
}

// ScatterChartOptions controls point appearance and independent X value formatting.
type ScatterChartOptions struct {
	CartesianChartOptions
	PointRadius    *float32 `json:"pointRadius,omitempty"`
	Colors         []Color  `json:"colors,omitempty"`
	XValueDecimals *int     `json:"xValueDecimals,omitempty"`
	XValuePrefix   string   `json:"xValuePrefix,omitempty"`
	XValueSuffix   string   `json:"xValueSuffix,omitempty"`
}

func (p *ScatterChartProps) prepare() error {
	if p.Points == nil {
		return fmt.Errorf("scatter-chart requires points")
	}
	if len(p.Points) > 4096 {
		return fmt.Errorf("scatter-chart accepts at most 4096 points")
	}
	values := make([]float64, len(p.Points))
	labels := make([]string, len(p.Points))
	for i, point := range p.Points {
		if point.X == nil || point.Y == nil || math.IsNaN(*point.X) || math.IsInf(*point.X, 0) || math.Abs(*point.X) > 1e100 {
			return fmt.Errorf("scatter-chart points require finite x and y within ±1e100")
		}
		values[i], labels[i] = *point.Y, point.Label
	}
	options := LineChartOptions{}
	if p.Chart != nil {
		options = p.Chart.lineOptions()
		if p.Chart.PointRadius != nil && (math.IsNaN(float64(*p.Chart.PointRadius)) || *p.Chart.PointRadius < 1 || *p.Chart.PointRadius > 16) {
			return fmt.Errorf("scatter-chart pointRadius must be between 1 and 16")
		}
		if p.Chart.XValueDecimals != nil && (*p.Chart.XValueDecimals < 0 || *p.Chart.XValueDecimals > 6) {
			return fmt.Errorf("scatter-chart xValueDecimals must be between 0 and 6")
		}
		if utf8.RuneCountInString(p.Chart.XValuePrefix) > 32 || utf8.RuneCountInString(p.Chart.XValueSuffix) > 32 {
			return fmt.Errorf("scatter-chart x value prefix or suffix exceeds 32 characters")
		}
		if err := validateChartColors(p.Chart.Colors, len(p.Points)); err != nil {
			return err
		}
	}
	if err := validateChartValues(NodeScatterChart, values, labels, 4096); err != nil {
		return err
	}
	if err := options.validate(NodeScatterChart); err != nil {
		return err
	}
	return p.Style.validate()
}

// buildScatterChart adapts XY points to the shared Cartesian renderer.
func buildScatterChart(b *builder, node *Node, id string) widget.Widget {
	p := node.Props.(*ScatterChartProps)
	values, xs, labels := make([]float64, len(p.Points)), make([]float64, len(p.Points)), make([]string, len(p.Points))
	for i, point := range p.Points {
		xs[i], values[i], labels[i] = *point.X, *point.Y, point.Label
	}
	options, xOptions := LineChartOptions{}, LineChartOptions{}
	radius := b.theme().Scaled(4)
	var colors []Color
	if p.Chart != nil {
		options = p.Chart.lineOptions()
		xOptions.ValueDecimals, xOptions.ValuePrefix, xOptions.ValueSuffix = p.Chart.XValueDecimals, p.Chart.XValuePrefix, p.Chart.XValueSuffix
		colors = p.Chart.Colors
		if p.Chart.PointRadius != nil {
			radius = *p.Chart.PointRadius
		}
	}
	props := &LineChartProps{Styled: p.Styled, Values: values, PointLabels: labels, Label: p.Label, Chart: &options}
	return buildCartesianChart(b, cartesianChartConfig{props: props, id: id, kind: NodeScatterChart, pointRadius: radius, xValues: xs, colors: colors, xOptions: xOptions})
}

// validateChartColors keeps per-item colors aligned with their observations.
func validateChartColors(colors []Color, count int) error {
	if len(colors) != 0 && len(colors) != count {
		return fmt.Errorf("chart colors must match the number of values")
	}
	for _, color := range colors {
		if err := color.validate(); err != nil {
			return fmt.Errorf("chart color: %w", err)
		}
	}
	return nil
}

func chartItemColor(colors []Color, index int, theme component.ControlTheme, fallback ui.Color) ui.Color {
	if len(colors) > index {
		return colors[index].resolve(theme, fallback)
	}
	return fallback
}

// cartesianPoints maps bars from zero and scatter points from their numeric X/Y domains.
func cartesianPoints(values, xs []float64, plot ui.Rect, kind NodeType) (points []ui.Point, low, high, xLow, xHigh float64) {
	if len(values) == 0 || plot.Width <= 0 || plot.Height <= 0 {
		return
	}
	low, high = values[0], values[0]
	for _, value := range values[1:] {
		low, high = min(low, value), max(high, value)
	}
	if kind == NodeLineChart {
		return lineChartPoints(values, plot), low, high, 0, 0
	}
	if kind == NodeBarChart {
		low, high = min(0, low), max(0, high)
	}
	if kind == NodeScatterChart {
		xLow, xHigh = xs[0], xs[0]
		for _, x := range xs[1:] {
			xLow, xHigh = min(xLow, x), max(xHigh, x)
		}
	}
	points = make([]ui.Point, len(values))
	for i, value := range values {
		x, y := float64(0.5), float64(0.5)
		if kind == NodeBarChart {
			x = (float64(i) + 0.5) / float64(len(values))
			y = 1
		}
		if kind == NodeScatterChart && xLow != xHigh {
			x = (xs[i] - xLow) / (xHigh - xLow)
		}
		if high != low {
			y = 1 - (value-low)/(high-low)
		}
		points[i] = ui.Point{X: plot.X + float32(x)*plot.Width, Y: plot.Y + float32(y)*plot.Height}
	}
	return
}
