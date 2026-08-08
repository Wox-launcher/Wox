package preview

import (
	"fmt"
	"strings"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

var hotkeyOverviewAccent = woxui.Color{R: 59, G: 130, B: 246, A: 255}

// HotkeyOverviewPreviewEntry is one searchable shortcut row in the preview.
type HotkeyOverviewPreviewEntry struct {
	RawShortcut string
	Labels      []string
	Action      string
	Detail      string
	Scope       string
	Source      string
}

// HotkeyOverviewPreviewSection groups shortcuts under the Flutter section headings.
type HotkeyOverviewPreviewSection struct {
	Title   string
	Entries []HotkeyOverviewPreviewEntry
}

// HotkeyOverviewPreviewProps contains the immutable shortcut overview state.
type HotkeyOverviewPreviewProps struct {
	Width    float32
	Height   float32
	Scale    float32
	Search   string
	Title    string
	Subtitle string
	Count    string
	Empty    string
	Sections []HotkeyOverviewPreviewSection
	Theme    woxcomponent.Theme
}

// HotkeyOverviewPreviewView builds the grouped shortcut cards from prepared props.
func HotkeyOverviewPreviewView(props HotkeyOverviewPreviewProps) woxwidget.Widget {
	scale := props.Scale
	if scale <= 0 {
		scale = 1
	}
	scaled := func(value float32) float32 { return value * scale }
	textColor := props.Theme.PreviewText
	mutedColor := hotkeyOverviewWithOpacity(props.Theme.PreviewPropertyContent, 0.72)
	borderColor := hotkeyOverviewWithOpacity(props.Theme.PreviewSplit, 0.42)
	filtered := hotkeyOverviewFilteredSections(props.Sections, props.Search)
	count := 0
	for _, section := range filtered {
		count += len(section.Entries)
	}

	innerWidth := max(float32(0), props.Width-scaled(34))
	headerHeight := scaled(42)
	bodyHeight := max(float32(0), props.Height-scaled(16+14+14)-headerHeight)
	var content woxwidget.Widget
	if count == 0 {
		content = woxwidget.Align{Width: innerWidth, Height: bodyHeight, Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.Text{Value: props.Empty, Style: woxui.TextStyle{Size: scaled(12)}, Color: mutedColor}}
	} else {
		sections := make([]woxwidget.Widget, 0, len(filtered))
		contentHeight := float32(0)
		for index, section := range filtered {
			sectionWidget, sectionHeight := hotkeyOverviewSection(section, innerWidth, scale, textColor, mutedColor, borderColor)
			sections = append(sections, sectionWidget)
			contentHeight += sectionHeight
			if index < len(filtered)-1 {
				contentHeight += scaled(14)
			}
		}
		content = woxcomponent.WoxScrollView(woxcomponent.ScrollViewProps{
			Key: "hotkey-overview-scroll", Width: innerWidth, Height: bodyHeight, ContentHeight: contentHeight,
			ThumbColor: mutedColor, Content: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: scaled(14), Children: sections},
		})
	}

	header := hotkeyOverviewHeader(props, innerWidth, headerHeight, scale, textColor, mutedColor, hotkeyOverviewAccent, count)
	return woxwidget.Container{
		Width: props.Width, Height: props.Height,
		Padding: woxwidget.Insets{Left: scaled(18), Top: scaled(16), Right: scaled(16), Bottom: scaled(14)},
		Child:   woxwidget.Flex{Axis: woxwidget.Vertical, Gap: scaled(14), Children: []woxwidget.Widget{header, content}},
	}
}

func hotkeyOverviewHeader(props HotkeyOverviewPreviewProps, width, height, scale float32, textColor, mutedColor, accent woxui.Color, count int) woxwidget.Widget {
	pillText := strings.ReplaceAll(props.Count, "{count}", formatHotkeyOverviewCount(count))
	pillWidth := max(58*scale, float32(len([]rune(pillText))*7+20)*scale)
	textWidth := max(float32(0), width-46*scale-pillWidth-10*scale)
	icon := woxwidget.Container{Width: 34 * scale, Height: 34 * scale, Radius: 8 * scale, Color: accent, Child: woxwidget.Align{Width: 34 * scale, Height: 34 * scale, Horizontal: 0.5, Vertical: 0.5, Child: woxcomponent.KeyboardGlyph(20*scale, woxui.Color{R: 255, G: 255, B: 255, A: 255})}}
	title := woxwidget.Container{Width: textWidth, Height: 19 * scale, Child: woxwidget.Text{Value: props.Title, Style: woxui.TextStyle{Size: 14 * scale, Weight: woxui.FontWeightSemibold}, Color: textColor}}
	subtitle := woxwidget.Container{Width: max(float32(0), width-46*scale), Height: 18 * scale, Child: woxwidget.TextBlock{Value: props.Subtitle, Width: max(float32(0), width-46*scale), Height: 18 * scale, MaxLines: 2, LineHeight: 15 * scale, Style: woxui.TextStyle{Size: 11 * scale}, Color: mutedColor}}
	text := woxwidget.Container{
		Width: max(float32(0), width-46*scale), Height: height,
		Child: woxwidget.Flex{
			Axis: woxwidget.Vertical, Gap: 3 * scale,
			Children: []woxwidget.Widget{
				woxwidget.Stack{
					Width: max(float32(0), width-46*scale), Height: 19 * scale,
					Children: []woxwidget.StackChild{
						{Child: title},
						{Right: 0, AnchorRight: true, Child: hotkeyOverviewCountTag(pillText, pillWidth, scale, accent)},
					},
				},
				subtitle,
			},
		},
	}
	return woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 12 * scale, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{icon, text}}
}

// hotkeyOverviewCountTag builds the compact rectangular shortcut count tag.
func hotkeyOverviewCountTag(text string, width, scale float32, accent woxui.Color) woxwidget.Widget {
	return woxwidget.Container{
		Width: width, Height: 24 * scale, Radius: 5 * scale, Color: accent,
		Padding: woxwidget.Insets{Left: 10 * scale, Top: 5 * scale, Right: 10 * scale, Bottom: 5 * scale},
		Child:   woxwidget.Text{Value: text, Style: woxui.TextStyle{Size: 10 * scale, Weight: woxui.FontWeightSemibold}, Color: woxui.Color{R: 255, G: 255, B: 255, A: 255}},
	}
}

func hotkeyOverviewSection(section HotkeyOverviewPreviewSection, width, scale float32, textColor, mutedColor, borderColor woxui.Color) (woxwidget.Widget, float32) {
	rows := make([]woxwidget.Widget, 0, len(section.Entries)*2)
	cardHeight := float32(0)
	for index, entry := range section.Entries {
		rowHeight := 38 * scale
		if strings.TrimSpace(entry.Detail) != "" {
			rowHeight = 50 * scale
		}
		rows = append(rows, hotkeyOverviewEntryRow(entry, width, rowHeight, scale, textColor, mutedColor))
		cardHeight += rowHeight
		if index < len(section.Entries)-1 {
			rows = append(rows, woxwidget.Container{Width: width, Height: 1, Color: borderColor})
			cardHeight += 1
		}
	}
	card := woxwidget.Container{Width: width, Height: cardHeight, Radius: 8 * scale, BorderColor: borderColor, BorderWidth: 1, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows}}
	sectionHeight := 17*scale + 7*scale + cardHeight
	return woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 7 * scale, Children: []woxwidget.Widget{woxwidget.Container{Width: width, Height: 17 * scale, Child: woxwidget.Text{Value: section.Title, Style: woxui.TextStyle{Size: 12 * scale, Weight: woxui.FontWeightSemibold}, Color: textColor}}, card}}, sectionHeight
}

func hotkeyOverviewEntryRow(entry HotkeyOverviewPreviewEntry, width, height, scale float32, textColor, mutedColor woxui.Color) woxwidget.Widget {
	shortcutWidth := 220 * scale
	sourceWidth := 64 * scale
	actionWidth := max(float32(0), width-shortcutWidth-10*scale-sourceWidth-10*scale)
	chips := hotkeyOverviewChips(entry.Labels, entry.RawShortcut, shortcutWidth-20*scale, scale, textColor)
	actionChildren := []woxwidget.Widget{woxwidget.Container{Width: actionWidth, Height: 18 * scale, Child: woxwidget.Text{Value: entry.Action, Style: woxui.TextStyle{Size: 12 * scale, Weight: woxui.FontWeightSemibold}, Color: textColor}}}
	if strings.TrimSpace(entry.Detail) != "" {
		actionChildren = append(actionChildren, woxwidget.Container{Width: actionWidth, Height: 16 * scale, Padding: woxwidget.Insets{Top: 2 * scale}, Child: woxwidget.Text{Value: entry.Detail, Style: woxui.TextStyle{Size: 10 * scale}, Color: mutedColor}})
	}
	return woxwidget.Container{Width: width, Height: height, Padding: woxwidget.Insets{Left: 10 * scale, Top: 7 * scale, Right: 10 * scale, Bottom: 7 * scale}, Child: woxwidget.Stack{Width: max(float32(0), width-20*scale), Height: max(float32(0), height-14*scale), Children: []woxwidget.StackChild{
		{Child: chips},
		{Left: shortcutWidth + 10*scale, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 0, Children: actionChildren}},
		{Right: 0, AnchorRight: true, Child: woxwidget.Container{Width: sourceWidth, Height: 16 * scale, Child: woxwidget.Text{Value: entry.Source, Style: woxui.TextStyle{Size: 10 * scale, Weight: woxui.FontWeightSemibold}, Color: mutedColor}}},
	}}}
}

func hotkeyOverviewChips(labels []string, raw string, width, scale float32, textColor woxui.Color) woxwidget.Widget {
	if len(labels) == 0 {
		labels = []string{raw}
	}
	children := make([]woxwidget.Widget, 0, len(labels))
	contentWidth := float32(0)
	for index, label := range labels {
		if index > 0 {
			contentWidth += 4 * scale
		}
		chipWidth := max(28*scale, float32(len([]rune(label))*7+14)*scale)
		children = append(children, woxwidget.Container{Width: chipWidth, Height: 22 * scale, Radius: 5 * scale, Padding: woxwidget.Insets{Left: 7 * scale, Top: 4 * scale, Right: 7 * scale, Bottom: 4 * scale}, Color: hotkeyOverviewWithOpacity(textColor, 0.06), BorderColor: hotkeyOverviewWithOpacity(textColor, 0.18), BorderWidth: 1, Child: woxwidget.Align{Width: chipWidth - 14*scale, Height: 14 * scale, Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.Text{Value: label, Style: woxui.TextStyle{Size: 10 * scale, Weight: woxui.FontWeightSemibold}, Color: hotkeyOverviewWithOpacity(textColor, 0.9)}}})
		contentWidth += chipWidth
	}
	return woxwidget.ScrollView{Width: width, Height: 22 * scale, ContentWidth: max(width, contentWidth), Horizontal: true, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 4 * scale, Children: children}}
}

func hotkeyOverviewFilteredSections(sections []HotkeyOverviewPreviewSection, search string) []HotkeyOverviewPreviewSection {
	search = strings.ToLower(strings.TrimSpace(search))
	filtered := make([]HotkeyOverviewPreviewSection, 0, len(sections))
	for _, section := range sections {
		entries := make([]HotkeyOverviewPreviewEntry, 0, len(section.Entries))
		for _, entry := range section.Entries {
			if strings.TrimSpace(entry.RawShortcut) == "" {
				continue
			}
			values := []string{entry.RawShortcut, strings.Join(entry.Labels, " "), entry.Action, entry.Scope, entry.Source, entry.Detail}
			if search != "" {
				matched := false
				for _, value := range values {
					if strings.Contains(strings.ToLower(value), search) || strings.Contains(hotkeyOverviewNormalize(value), hotkeyOverviewNormalize(search)) {
						matched = true
						break
					}
				}
				if !matched {
					continue
				}
			}
			entries = append(entries, entry)
		}
		if len(entries) > 0 {
			filtered = append(filtered, HotkeyOverviewPreviewSection{Title: section.Title, Entries: entries})
		}
	}
	return filtered
}

func hotkeyOverviewNormalize(value string) string {
	value = strings.ToLower(value)
	value = strings.NewReplacer(" ", "", "_", "", "+", "", "-", "").Replace(value)
	return value
}

func formatHotkeyOverviewCount(count int) string {
	return fmt.Sprintf("%d", count)
}

func hotkeyOverviewWithOpacity(color woxui.Color, opacity float32) woxui.Color {
	color.A = uint8(min(float32(1), max(float32(0), opacity))*255 + 0.5)
	return color
}
