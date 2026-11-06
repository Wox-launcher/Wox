package woxview

import (
	ui "wox/ui/runtime"
	widget "wox/ui/widget"
)

// interactions observes a subtree without inserting a competing focusable control.
func (b *builder) interactions(id string, p CommonProps, child widget.Widget) widget.Widget {
	if p.OnHoverChange == nil && p.OnDoubleTap == nil && p.OnContextMenu == nil && p.OnFocusChange == nil {
		return child
	}
	g := widget.Gesture{ID: id + "/interactions", CoverHover: true, ObserveDescendants: true, OnDoubleTap: b.tap(p.OnDoubleTap), Child: widget.Container{Child: child}}
	if emit := b.emit(p.OnHoverChange); emit != nil {
		g.OnHover = func(inside bool) { emit(inside) }
	}
	if emit := b.emit(p.OnContextMenu); emit != nil {
		g.OnSecondaryTapDown = func(ui.Point) { emit() }
	}
	if emit := b.emit(p.OnFocusChange); emit != nil {
		return widget.FocusObserver{Key: widget.Key(id + "/focus-observer"), OnChange: func(focused bool) { emit(focused) }, Child: g}
	}
	return g
}
