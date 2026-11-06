package woxview

import (
	"time"

	component "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	widget "wox/ui/widget"
)

// ProgressProps is a linear bar. Value is 0..1; Indeterminate animates a moving segment instead.
type ProgressProps struct {
	Styled
	Value         float32 `json:"value,omitempty"`
	Indeterminate bool    `json:"indeterminate,omitempty"`
}

func (p *ProgressProps) prepare() error {
	p.Value = min(max(p.Value, 0), 1)
	return p.Style.validate()
}

// LoadingProps is the rotating Wox loading ring; style.width sets its size.
type LoadingProps struct {
	Styled
}

// DividerProps is a one-unit rule. Vertical dividers need a height from style or default to 16.
type DividerProps struct {
	Styled
	Vertical bool `json:"vertical,omitempty"`
}

func buildProgress(b *builder, node *Node, id string) widget.Widget {
	props := node.Props.(*ProgressProps)
	theme := b.theme()
	style := props.Style.resolve(b.theme())
	height := style.Height
	if height <= 0 {
		height = 4
	}
	radius := style.BorderRadius
	if radius <= 0 {
		radius = height / 2
	}
	if props.Style.state != nil && props.Style.state.BorderRadius != nil {
		radius = style.BorderRadius
	}
	track := style.BackgroundColor.resolve(theme, theme.Border)
	fill := style.Color.resolve(theme, theme.Accent)
	bar := func(width, start, extent float32) widget.Widget {
		return widget.Stack{Width: width, Height: height, Children: []widget.StackChild{
			{Child: widget.Container{Width: width, Height: height, Radius: radius, Color: track}},
			{Left: start, Child: widget.Container{Width: extent, Height: height, Radius: radius, Color: fill}},
		}}
	}
	content := widget.LayoutBuilder{Build: func(available woxui.Size) widget.Widget {
		width := widthOr(style, available)
		if !props.Indeterminate {
			return bar(width, 0, width*props.Value)
		}
		return widget.LoopAnimation{Key: widget.Key(id), Duration: 1200 * time.Millisecond, Builder: func(progress float32) widget.Widget {
			// A 30%-wide segment sweeps from off the left edge to off the right edge.
			segment := width * 0.3
			start := -segment + (width+segment)*progress
			left := max(start, 0)
			return widget.Clip{Width: width, Height: height, Child: bar(width, left, min(start+segment, width)-left)}
		}}
	}}
	return props.Style.wrapControl(b.theme(), content)
}

func buildLoading(b *builder, node *Node, id string) widget.Widget {
	props := node.Props.(*LoadingProps)
	size := props.Style.Width.resolve(b.theme())
	if size <= 0 {
		size = 16
	}
	return props.Style.wrapOuter(b.theme(), component.WoxLoadingIndicator(size, props.Style.Color.resolve(b.theme(), b.theme().Text)))
}

func buildDivider(b *builder, node *Node, id string) widget.Widget {
	props := node.Props.(*DividerProps)
	style := props.Style.resolve(b.theme())
	color := style.Color.resolve(b.theme(), b.theme().Border)
	if props.Vertical {
		height := style.Height
		if height <= 0 {
			height = 16
		}
		return style.wrapOuter(b.theme(), widget.Container{Width: 1, Height: height, Color: color})
	}
	return style.wrapOuter(b.theme(), widget.LayoutBuilder{Build: func(available woxui.Size) widget.Widget {
		return widget.Container{Width: widthOr(style, available), Height: 1, Color: color}
	}})
}
