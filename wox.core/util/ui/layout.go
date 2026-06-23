package ui

import "fmt"

// LayoutEngine converts a widget tree into a CommandList.
// It measures text via the native TextMeasurer, computes positions
// for all widgets, and emits draw commands for the visible region.
type LayoutEngine struct {
	Theme    Theme
	Measurer TextMeasurer
}

// LayoutResult holds the computed geometry and draw commands.
type LayoutResult struct {
	Commands CommandList
	Width    float32
	Height   float32
}

// Layout produces a full draw command list for the given widget tree
// within the specified window dimensions. The list always starts with a
// Clear command so the previous frame is fully erased.
func (e *LayoutEngine) Layout(root Widget, winW, winH float32) LayoutResult {
	result := LayoutResult{Width: winW, Height: winH}
	ctx := layoutCtx{
		engine:   e,
		commands: &result.Commands,
		clip:     clipStack{{0, 0, winW, winH}},
	}

	// Use the theme background color directly, including its alpha. A
	// translucent background (e.g. glass-dark rgba 0.52) tints the Mica
	// backdrop with the theme's color instead of leaving it fully transparent.
	bg := e.Theme.WindowBg
	result.Commands.Clear(bg.R, bg.G, bg.B, bg.A)

	// Root widget must be a container (VBox or HBox).
	e.layoutWidget(&ctx, root, 0, 0, winW, winH)
	return result
}

// clipRect is a stack entry for nested clipping.
type clipRect struct{ X, Y, W, H float32 }

type clipStack []clipRect

func (s *clipStack) push(r clipRect) { *s = append(*s, r) }
func (s *clipStack) pop() clipRect {
	if len(*s) == 0 {
		return clipRect{}
	}
	r := (*s)[len(*s)-1]
	*s = (*s)[:len(*s)-1]
	return r
}
func (s *clipStack) current() clipRect {
	if len(*s) == 0 {
		return clipRect{}
	}
	return (*s)[len(*s)-1]
}

// layoutCtx carries state through the recursive layout pass.
type layoutCtx struct {
	engine   *LayoutEngine
	commands *CommandList
	clip     clipStack
}

// layoutWidget dispatches to the appropriate layout function based on widget type.
func (e *LayoutEngine) layoutWidget(ctx *layoutCtx, w Widget, x, y, w_, h float32) {
	switch widget := w.(type) {
	case VBox:
		e.layoutVBox(ctx, widget, x, y, w_, h)
	case HBox:
		e.layoutHBox(ctx, widget, x, y, w_, h)
	case Text:
		e.layoutText(ctx, widget, x, y, w_, h)
	case TextBox:
		e.layoutTextBox(ctx, widget, x, y, w_, h)
	case ListBox:
		e.layoutListBox(ctx, widget, x, y, w_, h)
	case Image:
		e.layoutImage(ctx, widget, x, y, w_, h)
	case Separator:
		e.layoutSeparator(ctx, widget, x, y, w_, h)
	case Spacer:
		// Spacer contributes to flex sizing but draws nothing.
	}
}

// layoutVBox lays out children vertically (top to bottom).
func (e *LayoutEngine) layoutVBox(ctx *layoutCtx, vbox VBox, x, y, w, h float32) {
	if vbox.BgColor != nil {
		c := *vbox.BgColor
		ctx.commands.DrawRect(x, y, w, h, c.R, c.G, c.B, c.A)
	}

	pad := vbox.Padding
	innerX := x + pad
	innerY := y + pad
	innerW := w - pad*2
	innerH := h - pad*2

	// First pass: measure fixed-size children and count flex children.
	flexCount := 0
	fixedH := float32(0)
	for _, child := range vbox.Children {
		if _, ok := child.(Spacer); ok {
			flexCount++
			fixedH += child.(Spacer).Size
		} else {
			cw, ch := e.measureWidget(child, innerW)
			_ = cw
			fixedH += ch
		}
	}

	gapTotal := float32(len(vbox.Children)-1) * vbox.Gap
	if gapTotal < 0 {
		gapTotal = 0
	}
	availFlex := innerH - fixedH - gapTotal
	flexSize := float32(0)
	if flexCount > 0 && availFlex > 0 {
		flexSize = availFlex / float32(flexCount)
	}

	// Second pass: position and draw children.
	cy := innerY
	for _, child := range vbox.Children {
		_, ch := e.measureWidget(child, innerW)
		if sp, ok := child.(Spacer); ok {
			ch = sp.Size + flexSize
		}
		e.layoutWidget(ctx, child, innerX, cy, innerW, ch)
		cy += ch + vbox.Gap
	}
}

// layoutHBox lays out children horizontally (left to right).
func (e *LayoutEngine) layoutHBox(ctx *layoutCtx, hbox HBox, x, y, w, h float32) {
	if hbox.BgColor != nil {
		c := *hbox.BgColor
		ctx.commands.DrawRect(x, y, w, h, c.R, c.G, c.B, c.A)
	}

	pad := hbox.Padding
	innerX := x + pad
	innerY := y + pad
	innerW := w - pad*2
	innerH := h - pad*2

	flexCount := 0
	fixedW := float32(0)
	for _, child := range hbox.Children {
		if _, ok := child.(Spacer); ok {
			flexCount++
			fixedW += child.(Spacer).Size
		} else {
			cw, _ := e.measureWidget(child, innerH)
			fixedW += cw
		}
	}

	gapTotal := float32(len(hbox.Children)-1) * hbox.Gap
	if gapTotal < 0 {
		gapTotal = 0
	}
	availFlex := innerW - fixedW - gapTotal
	flexSize := float32(0)
	if flexCount > 0 && availFlex > 0 {
		flexSize = availFlex / float32(flexCount)
	}

	cx := innerX
	for _, child := range hbox.Children {
		cw, _ := e.measureWidget(child, innerH)
		if sp, ok := child.(Spacer); ok {
			cw = sp.Size + flexSize
		}
		e.layoutWidget(ctx, child, cx, innerY, cw, innerH)
		cx += cw + hbox.Gap
	}
}

// layoutText draws a static text label.
func (e *LayoutEngine) layoutText(ctx *layoutCtx, t Text, x, y, w, h float32) {
	c := t.FontColor
	ctx.commands.DrawText(x, y, w, h, c.R, c.G, c.B, c.A, t.Content, t.FontSize, t.FontFamily)
}

// layoutTextBox draws a text input field with background and cursor.
func (e *LayoutEngine) layoutTextBox(ctx *layoutCtx, tb TextBox, x, y, w, h float32) {
	bg := tb.BgColor
	ctx.commands.DrawRoundedRect(x, y, w, h, tb.CornerRadius, bg.R, bg.G, bg.B, bg.A)

	textX := x + 12 // left padding
	textY := y
	textW := w - 24
	textH := h

	if tb.Value == "" && tb.Placeholder != "" {
		p := e.Theme.TextPlaceholder
		ctx.commands.DrawText(textX, textY, textW, textH, p.R, p.G, p.B, p.A, tb.Placeholder, tb.FontSize, e.Theme.FontFamily)
	} else {
		c := tb.FontColor
		ctx.commands.DrawText(textX, textY, textW, textH, c.R, c.G, c.B, c.A, tb.Value, tb.FontSize, e.Theme.FontFamily)
	}

	// Cursor — simple vertical bar at end of text (or start if empty).
	if tb.Focused {
		cursorX := textX + 2
		if tb.Value != "" {
			tw, _ := e.Measurer.MeasureText(tb.Value, tb.FontSize, "")
			cursorX = textX + tw + 2
		}
		cc := tb.CursorColor
		ctx.commands.DrawLine(cursorX, y+8, cursorX, y+h-8, 1.5, cc.R, cc.G, cc.B, cc.A)
	}
}

// layoutListBox draws a scrollable list of items.
func (e *LayoutEngine) layoutListBox(ctx *layoutCtx, lb ListBox, x, y, w, h float32) {
	if lb.BgColor != nil {
		c := *lb.BgColor
		ctx.commands.DrawRect(x, y, w, h, c.R, c.G, c.B, c.A)
	}

	// Clip to list viewport.
	ctx.commands.PushClip(x, y, w, h)
	defer ctx.commands.PopClip()

	itemH := lb.ItemHeight
	if itemH == 0 {
		itemH = e.Theme.ListItemHeight
	}

	// Only draw visible items.
	startIdx := int(lb.ScrollOffset / itemH)
	if startIdx < 0 {
		startIdx = 0
	}
	endIdx := startIdx + int(h/itemH) + 1
	if endIdx > len(lb.Items) {
		endIdx = len(lb.Items)
	}

	for i := startIdx; i < endIdx; i++ {
		itemY := y + float32(i)*itemH - lb.ScrollOffset
		if itemY+itemH < y || itemY > y+h {
			continue
		}

		// Selected highlight.
		if i == lb.Selected && lb.SelectedColor != nil {
			c := *lb.SelectedColor
			ctx.commands.DrawRoundedRect(x, itemY, w, itemH-4, 6, c.R, c.G, c.B, c.A)
		}

		item := lb.Items[i]

		// Icon (left side).
		iconX := x + 12
		iconY := itemY + 6
		iconSize := float32(36)
		if len(item.IconPNG) > 0 {
			ctx.commands.DrawImageWithKey(iconX, iconY, iconSize, iconSize, item.IconKey, item.IconPNG)
		}

		// Title.
		titleX := iconX + iconSize + 8
		titleColor := e.Theme.TextPrimary
		ctx.commands.DrawText(titleX, itemY+8, w-iconSize-40, 20, titleColor.R, titleColor.G, titleColor.B, titleColor.A, item.Title, e.Theme.FontSize, e.Theme.FontFamily)

		// Subtitle.
		subColor := e.Theme.TextSecondary
		ctx.commands.DrawText(titleX, itemY+28, w-iconSize-40, 16, subColor.R, subColor.G, subColor.B, subColor.A, item.Subtitle, e.Theme.FontSize-3, e.Theme.FontFamily)
	}

	// Scrollbar.
	contentH := float32(len(lb.Items)) * itemH
	if contentH > h {
		scrollbarX := x + w - 8
		scrollbarH := h * (h / contentH)
		scrollbarY := y + (h-scrollbarH)*(lb.ScrollOffset/(contentH-h))
		ctx.commands.DrawRoundedRect(scrollbarX, scrollbarY, 4, scrollbarH, 2, 1, 1, 1, 0.15)
	}
}

// layoutImage draws a PNG image.
func (e *LayoutEngine) layoutImage(ctx *layoutCtx, img Image, x, y, w, h float32) {
	iw := img.Width
	ih := img.Height
	if iw == 0 {
		iw = w
	}
	if ih == 0 {
		ih = h
	}
	if len(img.PNGData) > 0 {
		ctx.commands.DrawImageWithKey(x, y, iw, ih, img.ImageKey, img.PNGData)
	}
}

// layoutSeparator draws a divider line.
func (e *LayoutEngine) layoutSeparator(ctx *layoutCtx, sep Separator, x, y, w, h float32) {
	c := sep.Color
	thickness := sep.Thickness
	if thickness == 0 {
		thickness = 1
	}
	if sep.Orientation == OrientHorizontal {
		ctx.commands.DrawLine(x, y+h/2, x+w, y+h/2, thickness, c.R, c.G, c.B, c.A)
	} else {
		ctx.commands.DrawLine(x+w/2, y, x+w/2, y+h, thickness, c.R, c.G, c.B, c.A)
	}
}

// measureWidget returns the natural width and height of a widget
// for layout purposes. Containers measure their children recursively.
func (e *LayoutEngine) measureWidget(w Widget, avail float32) (width, height float32) {
	switch widget := w.(type) {
	case VBox:
		maxChildW := float32(0)
		totalH := float32(0)
		for i, child := range widget.Children {
			cw, ch := e.measureWidget(child, avail-widget.Padding*2)
			if cw > maxChildW {
				maxChildW = cw
			}
			totalH += ch
			if i < len(widget.Children)-1 {
				totalH += widget.Gap
			}
		}
		return maxChildW + widget.Padding*2, totalH + widget.Padding*2
	case HBox:
		totalW := float32(0)
		maxChildH := float32(0)
		for i, child := range widget.Children {
			cw, ch := e.measureWidget(child, avail)
			totalW += cw
			if ch > maxChildH {
				maxChildH = ch
			}
			if i < len(widget.Children)-1 {
				totalW += widget.Gap
			}
		}
		return totalW + widget.Padding*2, maxChildH + widget.Padding*2
	case Text:
		w, h := e.Measurer.MeasureText(widget.Content, widget.FontSize, widget.FontFamily)
		return w, h
	case TextBox:
		return avail, e.Theme.QueryBoxHeight
	case ListBox:
		return avail, float32(len(widget.Items)) * widget.ItemHeight
	case Image:
		return widget.Width, widget.Height
	case Separator:
		if widget.Orientation == OrientHorizontal {
			return avail, widget.Thickness
		}
		return widget.Thickness, avail
	case Spacer:
		return widget.Size, widget.Size
	}
	return 0, 0
}

// string helper for debugging layout.
func rectStr(x, y, w, h float32) string {
	return fmt.Sprintf("(%.0f,%.0f %.0fx%.0f)", x, y, w, h)
}
