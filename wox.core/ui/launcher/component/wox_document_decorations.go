package component

import (
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const (
	documentBaseFontSize    = float32(14)
	documentCheckboxSize    = float32(13)
	documentCheckboxAdvance = float32(16)
	documentQuoteAdvance    = float32(12)
	documentQuoteBarWidth   = float32(2)
	documentRuleHeight      = float32(2)
)

// DocumentListMarkerColor is a fixed #1379D2 accent for task boxes and list prefixes.
// Theme cursor and body text can wash out on light or dark note surfaces, so this
// stays readable on both.
var DocumentListMarkerColor = woxui.Color{R: 0x13, G: 0x79, B: 0xD2, A: 255}

func documentDecorationScale(fontSize float32) float32 {
	return max(float32(0.75), fontSize/documentBaseFontSize)
}

func documentCheckboxWidth(fontSize float32) float32 {
	return documentCheckboxAdvance * documentDecorationScale(fontSize)
}

func documentCheckbox(fontSize, lineHeight float32, color woxui.Color, checked bool) woxwidget.Widget {
	return woxwidget.Painter{Width: documentCheckboxWidth(fontSize), Height: lineHeight, Paint: func(displayList *woxui.DisplayList, bounds woxui.Rect) {
		paintDocumentCheckbox(displayList, bounds, fontSize, color, checked)
	}}
}

// paintDocumentCheckbox keeps task markers identical in editable and read-only documents.
func paintDocumentCheckbox(displayList *woxui.DisplayList, bounds woxui.Rect, fontSize float32, color woxui.Color, checked bool) {
	scale := documentDecorationScale(fontSize)
	size := min(bounds.Height-4, documentCheckboxSize*scale)
	rect := woxui.Rect{X: bounds.X + (bounds.Width-size)/2, Y: bounds.Y + (bounds.Height-size)/2, Width: size, Height: size}
	displayList.StrokeRoundedRect(rect, 3*scale, max(float32(1), 2*scale), color)
	if checked {
		inset := 4 * scale
		displayList.FillRoundedRect(woxui.Rect{X: rect.X + inset, Y: rect.Y + inset, Width: rect.Width - inset*2, Height: rect.Height - inset*2}, scale, color)
	}
}

// documentQuote gives read-only Markdown the same leading-bar geometry as editable notes.
func documentQuote(width, fontSize float32, color woxui.Color, child woxwidget.Widget) woxwidget.Widget {
	scale := documentDecorationScale(fontSize)
	advance := documentQuoteAdvance * scale
	barWidth := documentQuoteBarWidth * scale
	barLeft := (advance - barWidth) / 2
	return woxwidget.Container{Width: width, Padding: woxwidget.Insets{Left: barLeft}, Child: woxwidget.Container{
		Width: max(float32(0), width-barLeft), Padding: woxwidget.Insets{Left: advance - barLeft}, LeftBorderColor: color, LeftBorderWidth: barWidth, Child: child,
	}}
}

func documentQuoteWidth(fontSize float32) float32 {
	return documentQuoteAdvance * documentDecorationScale(fontSize)
}

func paintDocumentQuoteBar(displayList *woxui.DisplayList, bounds woxui.Rect, fontSize float32, color woxui.Color) {
	barWidth := documentQuoteBarWidth * documentDecorationScale(fontSize)
	displayList.FillRect(woxui.Rect{X: bounds.X + (bounds.Width-barWidth)/2, Y: bounds.Y, Width: barWidth, Height: bounds.Height}, color)
}

func documentHorizontalRule(width float32, color woxui.Color) woxwidget.Widget {
	return woxwidget.Painter{Width: width, Height: documentRuleHeight, Paint: func(displayList *woxui.DisplayList, bounds woxui.Rect) {
		paintDocumentHorizontalRule(displayList, bounds, color)
	}}
}

func paintDocumentHorizontalRule(displayList *woxui.DisplayList, bounds woxui.Rect, color woxui.Color) {
	displayList.FillRect(bounds, color)
}
