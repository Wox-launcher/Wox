package component

import (
	"reflect"

	ui "wox/ui/runtime"
	widget "wox/ui/widget"
)

// TooltipProps adds a noninteractive, theme-aware hint without changing its trigger's geometry.
type TooltipProps struct {
	Key     widget.Key
	Message string
	Theme   ControlTheme
	Child   widget.Widget
}

// WoxTooltip uses the shared hover path, including when a child control owns pointer input.
func WoxTooltip(p TooltipProps) widget.Widget {
	if p.Message == "" {
		return p.Child
	}
	return widget.Stateful{Key: p.Key, Type: (*tooltipState)(nil), Widget: p, CreateState: func() widget.State { return &tooltipState{} }}
}

type tooltipState struct {
	hovered  bool
	dirty    bool
	anchor   ui.Rect
	viewport ui.Size
	token    uint64
}

func (*tooltipState) InitState(widget.StateContext, any) {}
func (*tooltipState) Dispose()                           {}

// DidUpdateWidget refreshes translations and theme colors while a hint stays open.
func (s *tooltipState) DidUpdateWidget(_ widget.StateContext, old, next any) {
	a, b := old.(TooltipProps), next.(TooltipProps)
	s.dirty = s.dirty || a.Message != b.Message || !reflect.DeepEqual(a.Theme, b.Theme)
}

// Build owns only hover/overlay state; the child keeps its focus, gestures, and semantics.
func (s *tooltipState) Build(ctx widget.StateContext, value any) widget.Widget {
	p := value.(TooltipProps)
	key := widget.Key(string(p.Key) + "/trigger")
	show := func(anchor ui.Rect) {
		if owner := ctx.HostOverlayOwner(); owner != "" && owner != key {
			return
		}
		s.anchor, s.viewport, s.dirty = anchor, ctx.HostFrameSize(), false
		s.token = ctx.SetHostOverlay(key, widget.AnchoredOverlay{Anchor: anchor, Gap: p.Theme.Scaled(4), Margin: p.Theme.Scaled(8),
			Child: widget.Constrained{MaxWidth: p.Theme.Scaled(400), Child: widget.Container{
				Floating: true, Color: p.Theme.Surface, BorderColor: p.Theme.Border, BorderWidth: 1, Radius: p.Theme.Scaled(8),
				Padding: widget.Insets{Left: p.Theme.Scaled(8), Right: p.Theme.Scaled(8), Top: p.Theme.Scaled(8), Bottom: p.Theme.Scaled(8)},
				Child: widget.Semantics{AutomationID: string(p.Key) + "/hint", Label: p.Message,
					Child: widget.TextBlock{Value: p.Message, ShrinkWrap: true, Style: ui.TextStyle{Size: p.Theme.Scaled(SettingsHelpFontSize)}, Color: p.Theme.Text}},
			}},
		})
	}
	ctx.PostFrame(func() {
		if !s.hovered || !ctx.Mounted() || ctx.HostOverlayOwner() != key {
			return
		}
		if bounds, ok := ctx.HostBoundsForKey(key); ok && (bounds != s.anchor || ctx.HostFrameSize() != s.viewport || s.dirty) {
			show(bounds)
		}
	})
	return widget.Gesture{ID: string(key), CoverHover: true, OnHoverAt: func(inside bool, bounds ui.Rect) {
		s.hovered = inside
		if inside {
			show(bounds)
		} else {
			ctx.ClearHostOverlay(key, s.token)
		}
	}, Child: p.Child}
}
