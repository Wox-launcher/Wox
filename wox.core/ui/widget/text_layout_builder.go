package widget

import ui "wox/ui/runtime"

// TextMeasurer is the current surface's logical font measurement capability.
type TextMeasurer interface {
	MeasureText(string, ui.TextStyle) (ui.TextMetrics, error)
}

// TextLayoutBuilder lets shared text controls use exactly the Host's font metrics
// without depending on a native Window or approximating widths in headless hosts.
type TextLayoutBuilder struct {
	Build func(ui.Size, TextMeasurer) Widget
}

func (w TextLayoutBuilder) layout(ctx context, available constraints) *node {
	if w.Build == nil {
		return &node{}
	}
	child := w.Build(ui.Size{Width: available.width, Height: available.height}, ctx.window)
	if child == nil {
		return &node{}
	}
	return child.layout(ctx, available)
}
