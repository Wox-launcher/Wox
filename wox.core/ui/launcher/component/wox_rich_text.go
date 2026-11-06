package component

import (
	"math"
	"strings"
	"unicode/utf8"

	ui "wox/ui/runtime"
	"wox/ui/widget"
)

// RichTextRun owns one inline range. Style sizes are unscaled; Decorate receives
// its laid-out fragment group, allowing an embedder to attach node-level behavior.
type RichTextRun struct {
	ID        string
	Text      string
	Style     ui.TextStyle
	Color     ui.Color
	Underline bool
	OnPress   func()
	Decorate  func(widget.Widget) widget.Widget
	// Interaction observers use the measured fragments without adding hit targets
	// above the paragraph editor. The caller owns the resulting visual state.
	OnHoverChange func(bool)
	OnPressChange func(bool)
	DecoratePaint func(list *ui.DisplayList, bounds ui.Rect, selected bool, draw func())
	// OnSelectionPaint observes the native highlight separately from glyph
	// decoration, so selection is never tinted or transformed by local styles.
	OnSelectionPaint func(*ui.PaintSnapshot)
}

type RichTextProps struct {
	ID         string
	Runs       []RichTextRun
	Style      ui.TextStyle
	Color      ui.Color
	LineHeight float32
	MaxLines   int
	Centered   bool
	Selectable bool
	Window     *ui.Window
	Theme      ControlTheme
}

type richTextFragment struct {
	start, end int
	bounds     ui.Rect
}

// richTextLayout shares grapheme-safe line breaking with editable rich text.
// Every fragment uses logical coordinates measured by the current Host.
type richTextLayout struct {
	value      string
	runes      []rune
	runs       []TextFieldRichRun
	lines      []textFieldLine
	fragments  [][]richTextFragment
	width      float32
	height     float32
	lineHeight float32
}

// layoutRichText computes wrapping once for painting, links, and selection.
func layoutRichText(p RichTextProps, available ui.Size, measure widget.TextMeasurer) richTextLayout {
	layout := richTextLayout{fragments: make([][]richTextFragment, len(p.Runs))}
	base := p.Style
	base.Size = p.Theme.Scaled(base.Size)
	var text strings.Builder
	offset := 0
	lineHeight := float32(0)
	for _, run := range p.Runs {
		style := run.Style
		if style.Size <= 0 {
			style.Size = p.Style.Size
		}
		style.Size = p.Theme.Scaled(style.Size)
		metrics, _ := measure.MeasureText("Mg", style)
		lineHeight = max(lineHeight, metrics.Size.Height, style.Size*1.5)
		end := offset + utf8.RuneCountInString(run.Text)
		layout.runs = append(layout.runs, TextFieldRichRun{Start: offset, End: end, Style: style, Color: run.Color, HasColor: true, Underline: run.Underline, DecoratePaint: run.DecoratePaint, OnSelectionPaint: run.OnSelectionPaint})
		text.WriteString(run.Text)
		offset = end
	}
	if p.LineHeight > 0 {
		lineHeight = p.Theme.Scaled(p.LineHeight)
	}
	layout.lineHeight = max(float32(1), lineHeight)
	layout.value = text.String()
	layout.runes = []rune(layout.value)
	layout.lines = textFieldRichLines(layout.value, measure, base, available.Width, true, layout.runs)
	limit := len(layout.lines)
	if p.MaxLines > 0 {
		limit = min(limit, p.MaxLines)
	}
	if available.Height < math.MaxFloat32 {
		limit = min(limit, max(1, int(available.Height/layout.lineHeight)))
	}
	layout.lines = layout.lines[:limit]
	for _, line := range layout.lines {
		layout.width = max(layout.width, textFieldMeasureRange(measure, layout.runes, line.start, line.end, base, layout.runs))
	}
	layout.width = min(layout.width, available.Width)
	if p.Centered || p.Selectable {
		layout.width = min(available.Width, max(layout.width, float32(1)))
		if available.Width < math.MaxFloat32 {
			layout.width = available.Width
		}
	}
	layout.height = min(available.Height, float32(max(1, limit))*layout.lineHeight)
	var insets []TextFieldRichRun
	for i, line := range layout.lines {
		x := float32(0)
		if p.Centered {
			width := textFieldMeasureRange(measure, layout.runes, line.start, line.end, base, layout.runs)
			x = max(float32(0), (layout.width-width)/2)
			insets = append(insets, TextFieldRichRun{Start: line.start, End: line.start, LineInset: x})
		}
		for index, run := range layout.runs {
			start, end := max(line.start, run.Start), min(line.end, run.End)
			if start >= end {
				continue
			}
			metrics, _ := measure.MeasureText(string(layout.runes[start:end]), run.Style)
			bounds := ui.Rect{X: x, Y: float32(i) * layout.lineHeight, Width: metrics.Size.Width, Height: layout.lineHeight}
			layout.fragments[index] = append(layout.fragments[index], richTextFragment{start: start, end: end, bounds: bounds})
			x += bounds.Width
		}
	}
	layout.runs = append(layout.runs, insets...)
	return layout
}

// WoxRichText draws inline runs as a paragraph, preserving whitespace and exact
// copy text. Each actionable run has one keyboard/accessibility target even when
// wrapping creates several disjoint pointer regions.
func WoxRichText(p RichTextProps) widget.Widget {
	if p.Style.Size <= 0 {
		p.Style.Size = SettingsControlFontSize
	}
	return widget.TextLayoutBuilder{Build: func(available ui.Size, measure widget.TextMeasurer) widget.Widget {
		layout := layoutRichText(p, available, measure)
		children := []widget.StackChild{}
		if p.Selectable {
			children = append(children, widget.StackChild{Child: WoxTextField(TextFieldProps{
				ID: p.ID + "/selectable", Label: layout.value, Width: layout.width, Height: layout.height + 1,
				Padding: widget.Insets{Bottom: 1}, Transparent: true, DisableHover: true, Style: p.Style,
				RichRuns: layout.runs, LineHeight: layout.lineHeight, TextColor: p.Color, Value: layout.value,
				ReadOnly: true, FixedViewport: true, MaxLines: max(8, len(layout.lines)+4), Window: p.Window, Measurer: measure, Theme: p.Theme,
			})})
		}
		for index, run := range p.Runs {
			fragments := layout.fragments[index]
			if len(fragments) == 0 {
				continue
			}
			bounds := fragments[0].bounds
			for _, f := range fragments[1:] {
				left, top := min(bounds.X, f.bounds.X), min(bounds.Y, f.bounds.Y)
				right, bottom := max(bounds.X+bounds.Width, f.bounds.X+f.bounds.Width), max(bounds.Y+bounds.Height, f.bounds.Y+f.bounds.Height)
				bounds = ui.Rect{X: left, Y: top, Width: right - left, Height: bottom - top}
			}
			pieces := []widget.StackChild{}
			for _, f := range fragments {
				var piece widget.Widget = widget.Container{Width: f.bounds.Width, Height: f.bounds.Height}
				if !p.Selectable {
					piece = widget.Painter{Width: f.bounds.Width, Height: f.bounds.Height, Paint: func(list *ui.DisplayList, rect ui.Rect) {
						drawTextFieldRichRange(list, measure, layout.runes, f.start, f.end, rect.X, rect.X+rect.Width, rect.Y, layout.lineHeight, p.Style, layout.runs, p.Color, true, 0)
					}}
				}
				if run.OnPress != nil {
					piece = widget.Gesture{Cursor: ui.PointerCursorHand, OnTap: run.OnPress, Child: piece}
				}
				pieces = append(pieces, widget.StackChild{Left: f.bounds.X - bounds.X, Top: f.bounds.Y - bounds.Y, Child: piece})
			}
			var content widget.Widget = widget.Container{Width: bounds.Width, Height: bounds.Height, Child: widget.Stack{Children: pieces}}
			semantic := widget.Semantics{Key: widget.Key(run.ID), AutomationID: run.ID, Label: run.Text, Role: ui.AccessibilityRoleText, Child: content}
			if run.OnPress != nil {
				semantic.Role = ui.AccessibilityRoleLink
				semantic.Actions = []ui.AccessibilityAction{ui.AccessibilityActionActivate}
				semantic.OnAction = func(action ui.AccessibilityAction, _ string) error {
					if action == ui.AccessibilityActionActivate {
						run.OnPress()
					}
					return nil
				}
				semantic.Child = widget.Focusable{Key: widget.Key(run.ID), FocusRingColor: p.Theme.Focus, FocusRingRadius: p.Theme.Scaled(2), OnKey: func(event ui.KeyEvent) bool {
					if event.Composing || event.Key != ui.KeyEnter && event.Key != ui.KeySpace {
						return false
					}
					if event.Down && !event.Repeat {
						run.OnPress()
					}
					return true
				}, Child: content}
			}
			content = semantic
			if run.Decorate != nil {
				content = run.Decorate(content)
			}
			children = append(children, widget.StackChild{Left: bounds.X, Top: bounds.Y, Child: content})
		}
		var paragraph widget.Widget = widget.Clip{Width: layout.width, Height: layout.height + 1, Child: widget.Container{Width: layout.width, Height: layout.height + 1, Child: widget.Stack{Children: children}}}
		for _, run := range p.Runs {
			if run.OnHoverChange != nil || run.OnPressChange != nil {
				paragraph = observeRichText(p, layout, paragraph)
				break
			}
		}
		return paragraph
	}}
}

// observeRichText shares layout geometry with selection, including disjoint
// fragments of a wrapped run, without inserting per-run pointer interception.
func observeRichText(p RichTextProps, layout richTextLayout, child widget.Widget) widget.Widget {
	return widget.Gesture{ID: p.ID + "/run-observer", CoverHover: true, ObserveDescendants: true,
		OnHover: func(inside bool) {
			if !inside {
				for _, run := range p.Runs {
					if run.OnHoverChange != nil {
						run.OnHoverChange(false)
					}
				}
			}
		},
		OnPressChange: func(pressed bool) {
			if !pressed {
				for _, run := range p.Runs {
					if run.OnPressChange != nil {
						run.OnPressChange(false)
					}
				}
			}
		},
		OnPointerObserve: func(e ui.PointerEvent) {
			for i, run := range p.Runs {
				inside := false
				for _, f := range layout.fragments[i] {
					r := f.bounds
					if e.Position.X >= r.X && e.Position.X < r.X+r.Width && e.Position.Y >= r.Y && e.Position.Y < r.Y+r.Height {
						inside = true
						break
					}
				}
				if run.OnHoverChange != nil {
					run.OnHoverChange(inside)
				}
				if e.Kind == ui.PointerDown && e.Button == ui.PointerButtonPrimary && run.OnPressChange != nil {
					run.OnPressChange(inside)
				}
			}
		},
		Child: widget.Container{Child: child},
	}
}
