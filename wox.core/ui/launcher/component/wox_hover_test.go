package component

import woxwidget "wox/ui/widget"

func buildHoverable(widget woxwidget.Widget, hovered bool) woxwidget.Widget {
	stateful := widget.(woxwidget.Stateful)
	return (&hoverableState{hovered: hovered}).Build(woxwidget.StateContext{}, stateful.Widget)
}
