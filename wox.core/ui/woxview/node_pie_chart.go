package woxview

import (
	"fmt"
	"math"

	component "wox/ui/launcher/component"
	ui "wox/ui/runtime"
	"wox/ui/widget"
)

// PieChartProps describes nonnegative parts of a whole in supplied order.
type PieChartProps struct {
	Styled
	Values      []float64        `json:"values"`
	PointLabels []string         `json:"pointLabels,omitempty"`
	Label       string           `json:"label,omitempty"`
	Chart       *PieChartOptions `json:"chart,omitempty"`
}

// PieChartOptions configures a pie or donut without Cartesian-only options.
type PieChartOptions struct {
	Colors          []Color              `json:"colors,omitempty"`
	InnerRadius     *float32             `json:"innerRadius,omitempty"`
	Hover           *bool                `json:"hover,omitempty"`
	Highlight       *bool                `json:"highlight,omitempty"`
	Tooltip         *bool                `json:"tooltip,omitempty"`
	TooltipPosition ChartTooltipPosition `json:"tooltipPosition,omitempty"`
	ValueDecimals   *int                 `json:"valueDecimals,omitempty"`
	ValuePrefix     string               `json:"valuePrefix,omitempty"`
	ValueSuffix     string               `json:"valueSuffix,omitempty"`
}

// lineOptions maps shared tooltip formatting and validation to the line contract.
func (o PieChartOptions) lineOptions() LineChartOptions {
	return LineChartOptions{Hover: o.Hover, Tooltip: o.Tooltip, TooltipPosition: o.TooltipPosition, ValueDecimals: o.ValueDecimals, ValuePrefix: o.ValuePrefix, ValueSuffix: o.ValueSuffix}
}

func (p *PieChartProps) prepare() error {
	if err := validateChartValues(NodePieChart, p.Values, p.PointLabels, 128); err != nil {
		return err
	}
	for _, value := range p.Values {
		if value < 0 {
			return fmt.Errorf("pie-chart values must be nonnegative")
		}
	}
	options := LineChartOptions{}
	if p.Chart != nil {
		options = p.Chart.lineOptions()
		if p.Chart.InnerRadius != nil && (math.IsNaN(float64(*p.Chart.InnerRadius)) || *p.Chart.InnerRadius < 0 || *p.Chart.InnerRadius > 0.9) {
			return fmt.Errorf("pie-chart innerRadius must be between 0 and 0.9")
		}
		if err := validateChartColors(p.Chart.Colors, len(p.Values)); err != nil {
			return err
		}
	}
	if err := options.validate(NodePieChart); err != nil {
		return err
	}
	return p.Style.validate()
}

type pieSlice struct{ start, end float64 }
type pieChartConfig struct {
	props *PieChartProps
	id    string
	theme component.ControlTheme
}
type pieChartState struct {
	selected    int
	center      ui.Point
	radius      float32
	innerRadius float32
	slices      []pieSlice
}

func (s *pieChartState) InitState(_ widget.StateContext, _ any) { s.selected = -1 }
func (s *pieChartState) Dispose()                               {}
func (s *pieChartState) DidUpdateWidget(_ widget.StateContext, _, next any) {
	p := next.(pieChartConfig).props
	if s.selected >= len(p.Values) || (s.selected >= 0 && p.Values[s.selected] == 0) || (p.Chart != nil && !chartOption(p.Chart.Hover)) {
		s.selected = -1
	}
}

// pieSlices keeps zero-valued categories in place without giving them a hit area.
func pieSlices(values []float64) []pieSlice {
	total := float64(0)
	for _, value := range values {
		total += value
	}
	if total == 0 {
		return nil
	}
	slices := make([]pieSlice, len(values))
	start := -math.Pi / 2
	for i, value := range values {
		end := start + 2*math.Pi*value/total
		slices[i] = pieSlice{start: start, end: end}
		start = end
	}
	return slices
}

// buildPieChart owns its local hit state without sending pointer events to plugins.
func buildPieChart(b *builder, node *Node, id string) widget.Widget {
	p := node.Props.(*PieChartProps)
	theme := b.theme()
	return p.Style.wrapControl(theme, widget.Semantics{Role: ui.AccessibilityRoleImage, Label: b.translate(p.Label), Child: widget.Stateful{
		Key: widget.Key(id), Type: (*pieChartState)(nil), Widget: pieChartConfig{props: p, id: id, theme: theme},
		CreateState: func() widget.State { return &pieChartState{} },
	}})
}

// pieColor derives a restrained theme-aware series when a plugin supplies no colors.
func pieColor(index int, theme component.ControlTheme, base ui.Color) ui.Color {
	mix := float32(index%4) * 0.18
	base.R = uint8(float32(base.R)*(1-mix) + float32(theme.Text.R)*mix)
	base.G = uint8(float32(base.G)*(1-mix) + float32(theme.Text.G)*mix)
	base.B = uint8(float32(base.B)*(1-mix) + float32(theme.Text.B)*mix)
	return base
}

// paintPieSlice tessellates each sector into convex wedges for all renderers.
func paintPieSlice(list *ui.DisplayList, center ui.Point, radius, inner float32, slice pieSlice, color ui.Color) {
	span := slice.end - slice.start
	if span <= 0 || radius <= 0 {
		return
	}
	segments := max(1, int(math.Ceil(span/(2*math.Pi)*120)))
	point := func(angle float64, r float32) ui.Point {
		return ui.Point{X: center.X + float32(math.Cos(angle))*r, Y: center.Y + float32(math.Sin(angle))*r}
	}
	for i := 0; i < segments; i++ {
		a := slice.start + span*float64(i)/float64(segments)
		b := slice.start + span*float64(i+1)/float64(segments)
		outerA, outerB := point(a, radius), point(b, radius)
		if inner <= 0 {
			list.FillConvexPolygon([]ui.Point{center, outerA, outerB}, color)
		} else {
			list.FillConvexPolygon([]ui.Point{point(a, inner), outerA, outerB, point(b, inner)}, color)
		}
	}
}

func (s *pieChartState) Build(ctx widget.StateContext, value any) widget.Widget {
	config := value.(pieChartConfig)
	p, theme := config.props, config.theme
	options := PieChartOptions{}
	if p.Chart != nil {
		options = *p.Chart
	}
	style := p.Style.resolve(theme)
	height := style.Height
	if height <= 0 {
		height = 160
	}
	color := style.Color.resolve(theme, theme.Accent)
	return widget.LayoutBuilder{Build: func(available ui.Size) widget.Widget {
		width := widthOr(style, available)
		selected := s.selected
		painter := widget.Painter{Width: width, Height: height, Paint: func(list *ui.DisplayList, bounds ui.Rect) {
			list.PushClipRect(bounds)
			defer list.PopClipRect()
			radius := max(float32(0), min(bounds.Width, bounds.Height)/2-theme.Scaled(4))
			center := ui.Point{X: bounds.X + bounds.Width/2, Y: bounds.Y + bounds.Height/2}
			inner := float32(0)
			if options.InnerRadius != nil {
				inner = radius * *options.InnerRadius
			}
			slices := pieSlices(p.Values)
			s.center, s.radius, s.innerRadius, s.slices = ui.Point{X: center.X - bounds.X, Y: center.Y - bounds.Y}, radius, inner, slices
			for i, slice := range slices {
				c := chartItemColor(options.Colors, i, theme, pieColor(i, theme, color))
				paintPieSlice(list, center, radius, inner, slice, c)
				if i == selected && chartOption(options.Hover) && chartOption(options.Highlight) {
					highlight := theme.Text
					highlight.A = uint8(float32(highlight.A) * 0.16)
					paintPieSlice(list, center, radius, inner, slice, highlight)
				}
			}
			if selected < 0 || selected >= len(slices) || !chartOption(options.Hover) || !chartOption(options.Tooltip) {
				return
			}
			slice := slices[selected]
			angle := (slice.start + slice.end) / 2
			distance := (radius + inner) / 2
			anchor := ui.Point{X: center.X + float32(math.Cos(angle))*distance, Y: center.Y + float32(math.Sin(angle))*distance}
			label := fmt.Sprintf("%.2f", p.Values[selected])
			if options.ValueDecimals != nil || options.ValuePrefix != "" || options.ValueSuffix != "" {
				label = chartValue(p.Values[selected], options.lineOptions())
			}
			pointLabel := ""
			if len(p.PointLabels) > selected {
				pointLabel = p.PointLabels[selected]
			}
			drawChartTooltip(list, bounds, anchor, pointLabel, label, options.TooltipPosition, theme)
		}}
		return widget.Gesture{ID: config.id + "/chart", Child: painter, OnPointer: func(event ui.PointerEvent) bool {
			next := -1
			if chartOption(options.Hover) && event.Kind != ui.PointerLeave && s.radius > 0 {
				dx, dy := event.Position.X-s.center.X, event.Position.Y-s.center.Y
				distance := float32(math.Hypot(float64(dx), float64(dy)))
				if distance <= s.radius && distance >= s.innerRadius {
					angle := math.Atan2(float64(dy), float64(dx))
					if angle < -math.Pi/2 {
						angle += 2 * math.Pi
					}
					for i, slice := range s.slices {
						if angle >= slice.start && angle < slice.end {
							next = i
							break
						}
					}
				}
			}
			if next != s.selected {
				ctx.SetState(func() { s.selected = next })
			}
			return false
		}}
	}}
}
