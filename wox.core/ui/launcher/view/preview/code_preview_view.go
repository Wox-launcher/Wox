package preview

import (
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/alecthomas/chroma/v2"
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// CodePreviewProps carries bounded source and tokens prepared by the file loader.
type CodePreviewProps struct {
	ID                           string
	Value                        string
	Tokens                       []chroma.Token
	Note                         string
	Width, Height, InitialOffset float32
	Window                       *woxui.Window
	Theme                        woxcomponent.Theme
}

// CodePreview keeps numbers outside the selectable value and scrolls both columns together.
func CodePreview(props CodePreviewProps) woxwidget.Widget {
	scaled := props.Theme.Controls.Scaled
	style := woxui.TextStyle{Size: scaled(woxcomponent.PreviewCodeFontSize), Family: woxui.FontFamilyMonospace}
	lineHeight, padding, gap := scaled(20), scaled(12), scaled(12)
	lines := strings.Split(props.Value, "\n")
	gutterWidth := scaled(24)
	if props.Window != nil {
		metrics, _ := props.Window.MeasureText(strconv.Itoa(len(lines)), style)
		gutterWidth = max(gutterWidth, metrics.Size.Width)
	}
	scroll := woxcomponent.ScrollViewProps{
		Key: woxwidget.Key("preview-scroll-" + props.ID), Offset: props.InitialOffset, FillWidth: true, FillHeight: true,
		Width: max(float32(0), props.Width-padding*2), ReserveScrollbarSpace: true,
		Theme: props.Theme.Controls, ThumbColor: props.Theme.PreviewText,
	}
	textWidth := max(float32(1), scroll.ContentViewportWidth()-gutterWidth-gap)
	runs := codePreviewRuns(props.Tokens, props.Theme.PreviewText, utf8.RuneCountInString(props.Value))
	numbers := make([]woxwidget.Widget, 0, len(lines))
	offset, visualLines := 0, 0
	for index, line := range lines {
		end := offset + utf8.RuneCountInString(line)
		var lineRuns []woxcomponent.TextFieldRichRun
		for _, run := range runs {
			if run.Start < end && run.End > offset {
				run.Start, run.End = max(0, run.Start-offset), min(end, run.End)-offset
				lineRuns = append(lineRuns, run)
			}
		}
		count := woxcomponent.TextFieldVisualLineCount(line, props.Window, style, textWidth, lineRuns)
		visualLines += count
		numbers = append(numbers, woxwidget.Container{Width: gutterWidth, Height: float32(count) * lineHeight,
			Child: woxwidget.Align{Horizontal: 1, Child: woxwidget.TextBlock{
				Value: strconv.Itoa(index + 1), Style: style, Color: previewColorWithOpacity(props.Theme.PreviewText, 0.55),
				LineHeight: lineHeight, MaxLines: 1, ShrinkWrap: true,
			}},
		})
		offset = end + 1
	}
	height := float32(visualLines)*lineHeight + 1
	field := woxcomponent.WoxTextField(woxcomponent.TextFieldProps{
		ID: previewTextFieldID(props.ID, "code"), Label: props.Value, Value: props.Value,
		Width: textWidth, Height: height, Padding: woxwidget.Insets{Bottom: 1},
		Style: style, LineHeight: lineHeight, RichRuns: runs, TextColor: props.Theme.PreviewText,
		Transparent: true, DisableHover: true, ReadOnly: true, MaxLines: max(8, visualLines+4),
		Window: props.Window, Theme: props.Theme.Controls,
	})
	children := []woxwidget.Widget{woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: gap, Children: []woxwidget.Widget{
		woxwidget.Container{Width: gutterWidth, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: numbers}}, field,
	}}}
	if props.Note != "" {
		children = append(children, woxwidget.TextBlock{Value: props.Note, Width: scroll.ContentViewportWidth(),
			Style: woxui.TextStyle{Size: scaled(woxcomponent.PreviewCodeFontSize)}, LineHeight: lineHeight,
			Color: previewColorWithOpacity(props.Theme.PreviewText, 0.65)})
	}
	scroll.Content = woxwidget.Flex{Axis: woxwidget.Vertical, Gap: gap, Children: children}
	return woxwidget.Container{Width: props.Width, Height: props.Height, Padding: woxwidget.UniformInsets(padding),
		Child: woxcomponent.WoxScrollView(scroll),
	}
}

// codePreviewRuns uses three muted syntax hues; other identifiers retain the theme's text color.
func codePreviewRuns(tokens []chroma.Token, foreground woxui.Color, length int) []woxcomponent.TextFieldRichRun {
	keyword, literal, number := woxui.Color{R: 185, G: 161, B: 213, A: 255}, woxui.Color{R: 164, G: 190, B: 160, A: 255}, woxui.Color{R: 201, G: 180, B: 143, A: 255}
	// Choose ink for light themes from the foreground, including translucent surfaces.
	if int(foreground.R)+int(foreground.G)+int(foreground.B) < 384 {
		keyword, literal, number = woxui.Color{R: 112, G: 77, B: 142, A: 255}, woxui.Color{R: 65, G: 109, B: 67, A: 255}, woxui.Color{R: 132, G: 95, B: 44, A: 255}
	}
	var runs []woxcomponent.TextFieldRichRun
	offset := 0
	for _, token := range tokens {
		end := min(length, offset+utf8.RuneCountInString(token.Value))
		color := woxui.Color{}
		switch {
		case token.Type.InCategory(chroma.Comment):
			color = previewColorWithOpacity(foreground, 0.65)
		case token.Type.InCategory(chroma.Keyword):
			color = keyword
		case token.Type.InSubCategory(chroma.LiteralString):
			color = literal
		case token.Type.InSubCategory(chroma.LiteralNumber):
			color = number
		}
		if color.A != 0 && end > offset {
			runs = append(runs, woxcomponent.TextFieldRichRun{Start: offset, End: end, Color: color})
		}
		offset = end
		if offset >= length {
			break
		}
	}
	return runs
}
