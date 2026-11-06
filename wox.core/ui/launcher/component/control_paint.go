package component

import (
	"time"
	ui "wox/ui/runtime"
	"wox/ui/widget"
)

// ControlPaint overrides only an individual control's paint. Pointer values
// preserve explicit transparency and zero borders/radii without changing layout
// or replacing its disabled, focus, hover, or activation behavior.
type ControlPaint struct {
	BackgroundTransition *PaintTransition
	Background           *ui.Color
	Foreground           *ui.Color
	BorderColor          *ui.Color
	BorderWidth          *float32
	Radius               *float32
}

// PaintTransition lets a control animate its resolved native paint without the
// caller duplicating hover, disabled, selection or theme fallback rules.
type PaintTransition struct {
	Key      widget.Key
	Duration time.Duration
	Curve    widget.AnimationCurve
}

// animateBackground resolves the native target at the control that owns it.
// Parts use stable IDs because one group may paint several independent surfaces.
func (p *ControlPaint) animateBackground(part string, fallback ui.Color, build func(ui.Color) widget.Widget) widget.Widget {
	if p != nil && p.Background != nil {
		fallback = *p.Background
	}
	if p == nil || p.BackgroundTransition == nil {
		return build(fallback)
	}
	t := p.BackgroundTransition
	return widget.AnimatedColor{Key: widget.Key(string(t.Key) + "/" + part), Target: fallback, Duration: t.Duration, Curve: t.Curve, Builder: build}
}

// paintContainer applies authored chrome and animates only its resolved fill.
func (p *ControlPaint) paintContainer(part string, box widget.Container) widget.Widget {
	box = p.applyContainer(box)
	if p == nil || p.BackgroundTransition == nil {
		return box
	}
	return p.animateBackground(part, box.Color, func(color ui.Color) widget.Widget {
		current := box
		current.Color = color
		return current
	})
}

// applyContainer changes paint only; callers keep all dimensions, padding and children.
func (p *ControlPaint) applyContainer(box widget.Container) widget.Container {
	if p == nil {
		return box
	}
	if p.Background != nil {
		box.Color = *p.Background
	}
	if p.BorderColor != nil {
		box.BorderColor = *p.BorderColor
	}
	if p.BorderWidth != nil {
		box.BorderWidth = *p.BorderWidth
	}
	if p.Radius != nil {
		box.Radius = *p.Radius
	}
	return box
}

func (p *ControlPaint) foreground(fallback ui.Color) ui.Color {
	if p != nil && p.Foreground != nil {
		return *p.Foreground
	}
	return fallback
}

func (p *ControlPaint) textPaint() *ControlPaint {
	if p == nil || p.Foreground == nil {
		return nil
	}
	return &ControlPaint{Foreground: p.Foreground}
}

// withPart gives an ID-less nested control its own channel within a composite.
func (p *ControlPaint) withPart(part string) *ControlPaint {
	if p == nil || p.BackgroundTransition == nil {
		return p
	}
	paint, transition := *p, *p.BackgroundTransition
	transition.Key = widget.Key(string(transition.Key) + "/" + part)
	paint.BackgroundTransition = &transition
	return &paint
}

// choicePaint keeps the selected surface distinct. An authored group fill is
// painted once behind the rows, so unselected row surfaces become transparent.
func (p *ControlPaint) choicePaint(selected bool) *ControlPaint {
	if p == nil {
		return nil
	}
	paint := &ControlPaint{Foreground: p.Foreground, BackgroundTransition: p.BackgroundTransition}
	if p.Background != nil && !selected {
		clear := ui.Color{}
		paint.Background = &clear
	}
	return paint
}
