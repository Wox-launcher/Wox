package ui

import (
	"fmt"
	"strings"
)

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
	case PreviewPanel:
		e.layoutPreviewPanel(ctx, widget, x, y, w_, h)
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

// layoutPreviewPanel renders the preview surface for the active query result.
// It draws a background, a vertical split line on the left edge, the preview
// content (text/markdown/image) clipped to the panel, a tag row at the bottom,
// and a scrollbar when content exceeds the panel height.
func (e *LayoutEngine) layoutPreviewPanel(ctx *layoutCtx, p PreviewPanel, x, y, w, h float32) {
	// Background.
	if p.BgColor != nil {
		c := *p.BgColor
		ctx.commands.DrawRoundedRect(x, y, w, h, 8, c.R, c.G, c.B, c.A)
	}

	// Left split line separating results and preview.
	sc := p.SplitColor
	ctx.commands.DrawLine(x, y+4, x, y+h-4, 1, sc.R, sc.G, sc.B, sc.A)

	const pad = 12.0
	const tagHeight = 22.0
	contentX := x + pad
	contentY := y + pad
	contentW := w - pad*2
	contentH := h - pad*2 - tagHeight

	// Measure the full content height so we can clamp ScrollOffset and draw a
	// scrollbar. This is a pre-pass that does not emit commands.
	contentTotalH := e.measurePreviewContentHeight(p, contentW)
	maxScroll := contentTotalH - contentH
	if maxScroll < 0 {
		maxScroll = 0
	}
	scroll := p.ScrollOffset
	if scroll < 0 {
		scroll = 0
	}
	if scroll > maxScroll {
		scroll = maxScroll
	}

	// Clip to the content area so overflow text is not drawn over the tag row.
	ctx.commands.PushClip(contentX, contentY, contentW, contentH)

	fontColor := p.FontColor
	fontSize := p.FontSize
	if fontSize == 0 {
		fontSize = e.Theme.FontSize
	}
	fontFamily := p.FontFamily
	if fontFamily == "" {
		fontFamily = e.Theme.FontFamily
	}

	switch p.PreviewType {
	case "", "remote":
		ctx.commands.DrawText(contentX, contentY-scroll, contentW, contentH,
			fontColor.R, fontColor.G, fontColor.B, fontColor.A,
			"Loading...", fontSize, fontFamily)
	case "text":
		e.drawPreviewText(ctx, p.PreviewData, contentX, contentY, contentW, contentH, scroll,
			fontColor, fontSize, fontFamily)
	case "markdown":
		e.drawPreviewMarkdown(ctx, p.PreviewData, contentX, contentY, contentW, contentH, scroll,
			fontColor, fontSize, fontFamily)
	case "image":
		if len(p.ImagePNG) > 0 {
			// Fit image into contentW x contentH. The native layer scales the
			// decoded PNG to the given rect, preserving aspect ratio is not
			// available here, so we use the full content area as the draw rect.
			ctx.commands.DrawImageWithKey(contentX, contentY-scroll, contentW, contentH, p.ImageKey, p.ImagePNG)
		} else {
			ctx.commands.DrawText(contentX, contentY-scroll, contentW, contentH,
				fontColor.R, fontColor.G, fontColor.B, fontColor.A,
				"Loading image...", fontSize, fontFamily)
		}
	default:
		// Unsupported preview types render as a short notice so the user knows
		// the preview exists but this UI can't render it.
		ctx.commands.DrawText(contentX, contentY-scroll, contentW, contentH,
			fontColor.R, fontColor.G, fontColor.B, fontColor.A,
			"(preview type \""+p.PreviewType+"\" is not supported in the native UI)",
			fontSize, fontFamily)
	}

	ctx.commands.PopClip()

	// Tag row at the bottom.
	if len(p.PreviewTags) > 0 {
		tagY := y + h - pad - tagHeight + 4
		labels := make([]string, 0, len(p.PreviewTags))
		for _, t := range p.PreviewTags {
			if t.Label != "" {
				labels = append(labels, t.Label)
			}
		}
		tagText := strings.Join(labels, "  ·  ")
		tc := e.Theme.PreviewPropertyContent
		ctx.commands.DrawText(contentX, tagY, contentW, tagHeight,
			tc.R, tc.G, tc.B, tc.A, tagText, fontSize-3, fontFamily)
	}

	// Scrollbar.
	if contentTotalH > contentH {
		scrollbarX := x + w - 8
		scrollbarH := contentH * (contentH / contentTotalH)
		if scrollbarH < 12 {
			scrollbarH = 12
		}
		scrollbarY := contentY + (contentH-scrollbarH)*(scroll/maxScroll)
		ctx.commands.DrawRoundedRect(scrollbarX, scrollbarY, 4, scrollbarH, 2, 1, 1, 1, 0.15)
	}
}

// measurePreviewContentHeight returns the total height of the preview content
// (text, markdown or image) without emitting draw commands. Used for scrollbar
// math and ScrollOffset clamping.
func (e *LayoutEngine) measurePreviewContentHeight(p PreviewPanel, contentW float32) float32 {
	fontSize := p.FontSize
	if fontSize == 0 {
		fontSize = e.Theme.FontSize
	}
	lineH := fontSize * 1.5

	switch p.PreviewType {
	case "text":
		return e.measurePreviewTextHeight(p.PreviewData, contentW, fontSize, lineH)
	case "markdown":
		return e.measurePreviewMarkdownHeight(p.PreviewData, contentW, fontSize, lineH)
	case "image":
		// Approximate image height as the content width — a square-ish default.
		// The real aspect ratio is unknown until native-decoded; this is a
		// reasonable placeholder for scrollbar sizing.
		return contentW
	default:
		return lineH
	}
}

// measurePreviewTextHeight computes the wrapped height of plain text.
func (e *LayoutEngine) measurePreviewTextHeight(text string, contentW, fontSize, lineH float32) float32 {
	if text == "" {
		return lineH
	}
	paragraphs := strings.Split(text, "\n")
	total := float32(0)
	for _, para := range paragraphs {
		if para == "" {
			total += lineH // blank line spacing
			continue
		}
		lines := e.wrapText(para, contentW, fontSize)
		total += float32(len(lines)) * lineH
	}
	return total
}

// measurePreviewMarkdownHeight computes the rendered height of parsed markdown.
func (e *LayoutEngine) measurePreviewMarkdownHeight(src string, contentW, fontSize, lineH float32) float32 {
	lines := ParseMarkdown(src)
	total := float32(0)
	for _, ml := range lines {
		switch ml.Style {
		case MDSeparator:
			total += lineH
		case MDHeading1:
			total += lineH * 1.6
		case MDHeading2:
			total += lineH * 1.35
		case MDHeading3:
			total += lineH * 1.15
		case MDCode:
			// Code lines measure with the code font; approximate by wrapping with
			// the same font size (the native layer uses the default family).
			wrapped := e.wrapText(ml.Text, contentW-16, fontSize)
			total += float32(len(wrapped)) * lineH
		default:
			if ml.Text == "" {
				total += lineH
			} else {
				indentW := contentW - float32(ml.Indent)*16
				wrapped := e.wrapText(ml.Text, indentW, fontSize)
				total += float32(len(wrapped)) * lineH
			}
		}
	}
	return total
}

// drawPreviewText renders plain text with automatic wrapping, applying the
// vertical scroll offset so only the visible portion is drawn (the caller has
// already pushed a clip rect).
func (e *LayoutEngine) drawPreviewText(ctx *layoutCtx, text string, x, y, w, h, scroll float32,
	color Color, fontSize float32, fontFamily string) {
	if text == "" {
		return
	}
	lineH := fontSize * 1.5
	paragraphs := strings.Split(text, "\n")
	cy := y - scroll
	for _, para := range paragraphs {
		if para == "" {
			cy += lineH
			continue
		}
		lines := e.wrapText(para, w, fontSize)
		for _, ln := range lines {
			if cy+lineH >= y && cy <= y+h {
				ctx.commands.DrawText(x, cy, w, lineH, color.R, color.G, color.B, color.A, ln, fontSize, fontFamily)
			}
			cy += lineH
		}
	}
}

// drawPreviewMarkdown renders parsed markdown lines with per-style sizing,
// applying the vertical scroll offset. Unsupported inline styling is already
// stripped by ParseMarkdown.
func (e *LayoutEngine) drawPreviewMarkdown(ctx *layoutCtx, src string, x, y, w, h, scroll float32,
	color Color, fontSize float32, fontFamily string) {
	lines := ParseMarkdown(src)
	cy := y - scroll
	for _, ml := range lines {
		switch ml.Style {
		case MDSeparator:
			if cy+8 >= y && cy <= y+h {
				ctx.commands.DrawLine(x, cy+4, x+w, cy+4, 1, color.R, color.G, color.B, color.A*0.4)
			}
			cy += fontSize * 1.5
		case MDHeading1:
			drawMarkdownLine(ctx, ml.Text, x, &cy, w, y, h, color, fontSize*1.6, fontFamily, true)
		case MDHeading2:
			drawMarkdownLine(ctx, ml.Text, x, &cy, w, y, h, color, fontSize*1.35, fontFamily, true)
		case MDHeading3:
			drawMarkdownLine(ctx, ml.Text, x, &cy, w, y, h, color, fontSize*1.15, fontFamily, true)
		case MDList:
			indentX := x + float32(ml.Indent)*16
			indentW := w - float32(ml.Indent)*16
			drawMarkdownLineWithBullet(ctx, ml.Text, indentX, &cy, indentW, y, h, color, fontSize, fontFamily)
		case MDCode:
			// Render code lines in a slightly inset block with a dimmer color.
			codeColor := color
			codeColor.A *= 0.85
			wrapped := e.wrapText(ml.Text, w-16, fontSize)
			for _, ln := range wrapped {
				if cy+fontSize*1.5 >= y && cy <= y+h {
					ctx.commands.DrawText(x+8, cy, w-16, fontSize*1.5, codeColor.R, codeColor.G, codeColor.B, codeColor.A, ln, fontSize, fontFamily)
				}
				cy += fontSize * 1.5
			}
		case MDQuote:
			qc := color
			qc.A *= 0.8
			drawMarkdownLineWithQuote(ctx, ml.Text, x, &cy, w, y, h, qc, fontSize, fontFamily)
		default:
			if ml.Text == "" {
				cy += fontSize * 1.5
			} else {
				drawMarkdownLine(ctx, ml.Text, x, &cy, w, y, h, color, fontSize, fontFamily, false)
			}
		}
	}
}

// drawMarkdownLine wraps a single markdown line and draws each wrapped row,
// advancing cy. Only rows intersecting [y, y+h] are drawn.
func drawMarkdownLine(ctx *layoutCtx, text string, x float32, cy *float32, w, y, h float32,
	color Color, fontSize float32, fontFamily string, _ bool) {
	lineH := fontSize * 1.5
	wrapped := ctx.engine.wrapText(text, w, fontSize)
	for _, ln := range wrapped {
		if *cy+lineH >= y && *cy <= y+h {
			ctx.commands.DrawText(x, *cy, w, lineH, color.R, color.G, color.B, color.A, ln, fontSize, fontFamily)
		}
		*cy += lineH
	}
}

// drawMarkdownLineWithBullet draws a list item with a "•" prefix and indent.
func drawMarkdownLineWithBullet(ctx *layoutCtx, text string, x float32, cy *float32, w, y, h float32,
	color Color, fontSize float32, fontFamily string) {
	lineH := fontSize * 1.5
	bulletW := float32(14)
	wrapped := ctx.engine.wrapText(text, w-bulletW, fontSize)
	for i, ln := range wrapped {
		if *cy+lineH >= y && *cy <= y+h {
			if i == 0 {
				ctx.commands.DrawText(x, *cy, bulletW, lineH, color.R, color.G, color.B, color.A, "•", fontSize, fontFamily)
			}
			ctx.commands.DrawText(x+bulletW, *cy, w-bulletW, lineH, color.R, color.G, color.B, color.A, ln, fontSize, fontFamily)
		}
		*cy += lineH
	}
}

// drawMarkdownLineWithQuote draws a quote line with a left bar.
func drawMarkdownLineWithQuote(ctx *layoutCtx, text string, x float32, cy *float32, w, y, h float32,
	color Color, fontSize float32, fontFamily string) {
	lineH := fontSize * 1.5
	barW := float32(3)
	wrapped := ctx.engine.wrapText(text, w-barW-8, fontSize)
	for _, ln := range wrapped {
		if *cy+lineH >= y && *cy <= y+h {
			ctx.commands.DrawLine(x, *cy+2, x, *cy+lineH-2, barW, color.R, color.G, color.B, color.A*0.5)
			ctx.commands.DrawText(x+8, *cy, w-barW-8, lineH, color.R, color.G, color.B, color.A, ln, fontSize, fontFamily)
		}
		*cy += lineH
	}
}

// wrapText breaks a single paragraph into lines that fit within width w,
// using the native text measurer. It splits on whitespace; very long words
// longer than the line are hard-broken at the character level.
func (e *LayoutEngine) wrapText(text string, w, fontSize float32) []string {
	if text == "" {
		return []string{""}
	}
	if w <= 0 {
		return []string{text}
	}
	words := strings.Fields(text)
	var lines []string
	var current string
	for _, word := range words {
		if current == "" {
			current = word
			continue
		}
		trial := current + " " + word
		tw, _ := e.Measurer.MeasureText(trial, fontSize, "")
		if tw <= w {
			current = trial
		} else {
			lines = append(lines, current)
			// Hard-break words wider than the line.
			ww, _ := e.Measurer.MeasureText(word, fontSize, "")
			if ww > w {
				runes := []rune(word)
				start := 0
				for start < len(runes) {
					end := len(runes)
					for end > start+1 {
						tw, _ := e.Measurer.MeasureText(string(runes[start:end]), fontSize, "")
						if tw <= w {
							break
						}
						end--
					}
					lines = append(lines, string(runes[start:end]))
					start = end
				}
				current = ""
			} else {
				current = word
			}
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	if len(lines) == 0 {
		lines = []string{""}
	}
	return lines
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
		if widget.Width > 0 {
			return widget.Width, float32(len(widget.Items)) * widget.ItemHeight
		}
		return avail, float32(len(widget.Items)) * widget.ItemHeight
	case Image:
		return widget.Width, widget.Height
	case PreviewPanel:
		if widget.Width > 0 {
			return widget.Width, e.Theme.ListItemHeight * 8
		}
		return 0, e.Theme.ListItemHeight * 8
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
