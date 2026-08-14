package view

import woxwidget "wox/ui/widget"

func focusedControlGesture(widget woxwidget.Widget) woxwidget.Gesture {
	child := widget.(woxwidget.Semantics).Child.(woxwidget.Focusable).Child
	if stateful, ok := child.(woxwidget.Stateful); ok {
		child = stateful.CreateState().Build(woxwidget.StateContext{}, stateful.Widget)
	}
	return child.(woxwidget.Gesture)
}
