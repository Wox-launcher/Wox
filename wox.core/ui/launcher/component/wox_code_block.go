package component

import (
	"math"

	ui "wox/ui/runtime"
	widget "wox/ui/widget"
)

// CodeBlockProps carries literal source, independent of Markdown parsing or fence escaping.
type CodeBlockProps struct {
	ID, Code, Language string
	Width, FontSize    float32
	Theme              ControlTheme
	Window             *ui.Window
	Paint              *ControlPaint
}

// WoxCodeBlock reuses native text selection and the Markdown copy feedback control.
func WoxCodeBlock(p CodeBlockProps) widget.Widget {
	return widget.LayoutBuilder{Build: func(available ui.Size) widget.Widget {
		width := p.Width
		if width <= 0 {
			width = available.Width
		}
		width = min(width, available.Width)
		padding := p.Theme.Scaled(12)
		inner := max(float32(0), width-2*padding)
		font := p.FontSize
		if font <= 0 {
			font = SettingsControlFontSize
		}
		style := ui.TextStyle{Size: p.Theme.Scaled(font), Family: ui.FontFamilyMonospace}
		lineHeight := float32(math.Round(float64(style.Size * 1.5)))
		lines := TextFieldVisualLineCount(p.Code, p.Window, style, inner, nil)
		copySize := min(inner, p.Theme.Scaled(28))
		header := widget.Flex{Axis: widget.Horizontal, CrossAxisAlignment: widget.CrossAxisCenter, Children: []widget.Widget{
			widget.Expanded{Child: widget.TextBlock{Value: p.Language, MaxLines: 1, Style: ui.TextStyle{Size: p.Theme.Scaled(SettingsSecondaryFontSize)}, Color: p.Paint.foreground(p.Theme.TextSecondary)}},
			codeCopyButton(codeCopyProps{id: p.ID + "/copy", code: p.Code, size: copySize, theme: p.Theme}),
		}}
		// WoxTextField scales the base font itself; line height and content size are already logical measurements.
		code := WoxTextField(TextFieldProps{
			ID: p.ID + "/text", Label: p.Code, Value: p.Code, ReadOnly: true, DisableHover: true, Transparent: true,
			Width: inner, Height: max(lineHeight, float32(lines)*lineHeight+1), LineHeight: lineHeight,
			Style: ui.TextStyle{Size: font, Family: ui.FontFamilyMonospace}, TextColor: p.Theme.Text,
			MaxLines: max(2, lines+1), Theme: p.Theme, Window: p.Window, Paint: p.Paint.textPaint(),
		})
		return p.Paint.paintContainer(p.ID, widget.Container{Width: width, Padding: widget.UniformInsets(padding), Radius: p.Theme.Scaled(4), Color: p.Theme.Surface, BorderColor: p.Theme.Border, BorderWidth: 1,
			Child: widget.Flex{Axis: widget.Vertical, Gap: p.Theme.Scaled(8), Children: []widget.Widget{header, code}}})
	}}
}
