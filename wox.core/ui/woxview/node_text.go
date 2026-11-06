package woxview

import (
	"math"

	component "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	widget "wox/ui/widget"
)

// TextProps renders wrapped text with optional inline spans. Selectable text supports drag selection and copy.
type TextProps struct {
	Styled
	Text       string `json:"text"`
	Selectable bool   `json:"selectable,omitempty"`
}

// MarkdownProps renders GitHub-flavored Markdown with the native Markdown widgets.
type MarkdownProps struct {
	Styled
	Content     string   `json:"content"`
	OnLinkPress *Handler `json:"onLinkPress,omitempty"`
	document    component.MarkdownDocument
}

func (p *MarkdownProps) handlers() []namedHandler {
	return append(p.Styled.handlers(), []namedHandler{{"onLinkPress", p.OnLinkPress}}...)
}

// prepare parses Markdown off the UI thread; Build only lays out the parsed document.
func (p *MarkdownProps) prepare() error {
	p.document = component.ParseMarkdown(p.Content)
	return p.Style.validate()
}

func buildText(b *builder, node *Node, id string) widget.Widget {
	props := node.Props.(*TextProps)
	value := b.translate(props.Text)
	style := props.Style.resolve(b.theme())
	theme := b.theme()
	textStyle := style.textStyle(theme, 14)
	color := style.Color.resolve(theme, theme.Text)
	if len(node.Children) > 0 {
		return style.decorate(theme, widget.Stateful{Key: widget.Key(id + "/inline-state"), Type: (*richTextState)(nil),
			Widget: nodeVisualConfig{builder: *b, node: node, id: id}, CreateState: func() widget.State { return &richTextState{} }})
	}
	if props.Selectable {
		// The text field applies the Interface size itself, so it receives the unscaled style.
		return style.decorate(theme, selectableText(b, id, value, style.textStyle(component.ControlTheme{}, 14), color, props.Style.LineHeight.resolve(theme)))
	}
	centered := style.TextAlign == TextAlignCenter
	text := widget.TextBlock{
		Value: value, Style: textStyle, Color: color,
		LineHeight: props.Style.LineHeight.resolve(theme), MaxLines: style.MaxLines, Centered: centered, ShrinkWrap: !centered,
	}
	return style.decorate(theme, text)
}

// selectableText reuses a read-only text field sized to its wrapped content, because only the
// text field implements selection, IME-safe copy, and the context menu.
func selectableText(b *builder, id, value string, baseStyle woxui.TextStyle, color woxui.Color, lineHeight float32) widget.Widget {
	window, theme := b.props.Window, b.theme()
	scaled := baseStyle
	scaled.Size = theme.Scaled(baseStyle.Size)
	if lineHeight <= 0 {
		lineHeight = float32(math.Round(float64(scaled.Size * 1.5)))
	}
	return widget.LayoutBuilder{Build: func(available woxui.Size) widget.Widget {
		width := widthOr(resolvedStyle{}, available)
		lines := component.TextFieldVisualLineCount(value, window, scaled, width, nil)
		return component.WoxTextField(component.TextFieldProps{
			ID: id + "/selectable", Label: value, Width: width, Height: max(lineHeight, float32(lines)*lineHeight+1), Padding: widget.Insets{Bottom: 1},
			Transparent: true, DisableHover: true, Style: baseStyle, LineHeight: lineHeight, TextColor: color, Value: value,
			ReadOnly: true, MaxLines: max(8, lines+4), Window: window, Theme: theme, Paint: &component.ControlPaint{Foreground: &color},
		})
	}}
}

func buildMarkdown(b *builder, node *Node, id string) widget.Widget {
	props := node.Props.(*MarkdownProps)
	style := props.Style.resolve(b.theme())
	theme := b.theme()
	openLink := b.props.OnOpenLink
	if props.OnLinkPress != nil {
		emit := b.emit(props.OnLinkPress)
		openLink = func(url string) {
			if emit != nil {
				emit(url)
			}
		}
	}
	if style.Color != (Color{}) {
		color := style.Color.resolve(theme, theme.Text)
		theme.Text, theme.BodyText = color, color
	}
	// The decoration already subtracts width and padding, so the builder sees the content width.
	content := widget.LayoutBuilder{Build: func(available woxui.Size) widget.Widget {
		return component.WoxMarkdown(component.MarkdownProps{
			ID: id, Document: props.document, Width: widthOr(resolvedStyle{}, available), FontSize: style.FontSize,
			Theme: theme, Window: b.props.Window, OnOpenLink: openLink,
		})
	}}
	return style.decorate(theme, content)
}
