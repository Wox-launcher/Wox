package preview

import (
	"strconv"
	"strings"
	"unicode/utf8"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// UpdatePreviewProps contains the update state, translated labels, and launcher-owned Markdown renderer.
type UpdatePreviewProps struct {
	ID                  string
	Width               float32
	Height              float32
	Scale               float32
	Theme               woxcomponent.Theme
	Title               string
	Error               string
	BetaLabel           string
	StatusLabel         string
	StatusColor         woxui.Color
	AutoUpdateEnabled   bool
	DisabledTitle       string
	DisabledDescription string
	DisabledAction      string
	OnPrimaryAction     func()
	ReleaseNotes        string
	NoReleaseNotes      string
	SectionNew          string
	SectionImprovements string
	SectionFixes        string
	SectionChanged      string
	SectionRemoved      string
	SectionSecurity     string
	MeasureText         func(string, woxui.TextStyle) float32
	RenderMarkdown      func(string, string, float32) woxwidget.Widget
}

type updateReleaseNotes struct {
	intro    string
	sections []updateReleaseNotesSection
}

type updateReleaseNotesSection struct {
	title string
	items []updateReleaseNotesItem
}

type updateReleaseNotesItem struct {
	tag          string
	summary      string
	continuation string
}

// UpdatePreviewView builds the dedicated update surface used by the launcher preview.
func UpdatePreviewView(props UpdatePreviewProps) woxwidget.Widget {
	scale := props.Scale
	if scale <= 0 {
		scale = 1
	}
	scaled := func(value float32) float32 { return float32(int(value*scale + 0.5)) }
	if !props.AutoUpdateEnabled {
		return disabledUpdatePreview(props, scaled)
	}

	innerWidth := max(float32(0), props.Width-scaled(40))
	headerHeight := scaled(24)
	if strings.TrimSpace(props.Error) != "" {
		headerHeight += scaled(22)
	}
	statusWidth := updatePillWidth(props, props.StatusLabel, scaled)
	betaWidth := float32(0)
	if strings.TrimSpace(props.BetaLabel) != "" {
		betaWidth = updatePillWidth(props, props.BetaLabel, scaled)
	}
	pillsWidth := statusWidth
	if betaWidth > 0 {
		pillsWidth += betaWidth + scaled(8)
	}
	titleWidth := max(float32(0), innerWidth-pillsWidth-scaled(12))
	headerChildren := []woxwidget.StackChild{
		{Child: woxwidget.TextBlock{Value: props.Title, Width: titleWidth, Height: scaled(22), MaxLines: 2, Style: woxui.TextStyle{Size: scaled(18), Weight: woxui.FontWeightSemibold}, LineHeight: scaled(20), Color: props.Theme.PreviewText}},
	}
	if strings.TrimSpace(props.Error) != "" {
		headerChildren = append(headerChildren, woxwidget.StackChild{Top: scaled(28), Child: woxwidget.TextBlock{Value: props.Error, Width: titleWidth, Height: scaled(18), MaxLines: 2, Style: woxui.TextStyle{Size: scaled(11)}, LineHeight: scaled(16), Color: props.Theme.ErrorText}})
	}
	right := innerWidth - statusWidth
	headerChildren = append(headerChildren, woxwidget.StackChild{Left: right, Child: updateStatusPill(props.StatusLabel, statusWidth, props.StatusColor, scaled)})
	if betaWidth > 0 {
		headerChildren = append(headerChildren, woxwidget.StackChild{Left: right - scaled(8) - betaWidth, Child: updateStatusPill(props.BetaLabel, betaWidth, woxui.Color{R: 33, G: 150, B: 243, A: 255}, scaled)})
	}

	bodyHeight := max(float32(0), props.Height-scaled(40)-headerHeight-scaled(27))
	releaseNotes := strings.TrimSpace(props.ReleaseNotes)
	if releaseNotes == "" {
		releaseNotes = props.NoReleaseNotes
	}
	body := buildUpdateReleaseNotes(props, releaseNotes, innerWidth-scaled(8), scaled)
	return woxwidget.Container{
		Width: props.Width, Height: props.Height, Padding: woxwidget.UniformInsets(scaled(20)),
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{
			woxwidget.Stack{Width: innerWidth, Height: headerHeight, Children: headerChildren},
			woxwidget.Container{Width: innerWidth, Height: scaled(15), Padding: woxwidget.Insets{Top: scaled(14)}, Child: woxwidget.Container{Width: innerWidth, Height: 1, Color: props.Theme.PreviewSplit}},
			woxwidget.Container{Width: innerWidth, Height: scaled(12)},
			woxwidget.ScrollView{Key: woxwidget.Key("update-preview-scroll-" + props.ID), ID: "update-preview-scroll-" + props.ID, Width: innerWidth, Height: bodyHeight, ContentHeight: bodyHeight, Child: body},
		}},
	}
}

func disabledUpdatePreview(props UpdatePreviewProps, scaled func(float32) float32) woxwidget.Widget {
	cardWidth := min(max(float32(0), props.Width-scaled(40)), scaled(760))
	innerWidth := max(float32(0), cardWidth-scaled(40))
	button := woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "update-enable-auto-update", Label: props.DisabledAction, IntrinsicWidth: true, Variant: woxcomponent.ButtonPrimary, OnTap: props.OnPrimaryAction, Theme: props.Theme})
	card := woxwidget.Container{
		Width: cardWidth, Padding: woxwidget.UniformInsets(scaled(20)), Radius: scaled(14), Color: updateColorAlpha(props.Theme.Background, 0.35), BorderColor: updateColorAlpha(props.Theme.PreviewSplit, 0.6), BorderWidth: 1,
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: scaled(12), Children: []woxwidget.Widget{
			woxwidget.Text{Value: props.DisabledTitle, Style: woxui.TextStyle{Size: scaled(18), Weight: woxui.FontWeightSemibold}, Color: props.Theme.PreviewText},
			woxwidget.TextBlock{Value: props.DisabledDescription, Width: innerWidth, MaxLines: 4, Style: woxui.TextStyle{Size: scaled(12)}, LineHeight: scaled(18), Color: updateColorAlpha(props.Theme.PreviewText, 0.8)},
			button,
		}},
	}
	return woxwidget.Align{Width: props.Width, Height: props.Height, Horizontal: 0.5, Vertical: 0.5, Child: card}
}

// buildUpdateReleaseNotes preserves the changelog structure while delegating each Markdown fragment to the shared renderer.
func buildUpdateReleaseNotes(props UpdatePreviewProps, markdown string, width float32, scaled func(float32) float32) woxwidget.Widget {
	parsed := parseUpdateReleaseNotes(markdown)
	if len(parsed.sections) == 0 {
		return renderUpdateMarkdown(props, props.ID+"-raw", markdown, width)
	}

	children := make([]woxwidget.Widget, 0, len(parsed.sections)*2+2)
	if parsed.intro != "" {
		children = append(children, woxwidget.Container{
			Width: width, Padding: woxwidget.Insets{Left: scaled(12), Top: scaled(10), Right: scaled(12), Bottom: scaled(10)}, Radius: scaled(8),
			Color: updateColorAlpha(props.Theme.PreviewText, 0.05), BorderColor: updateColorAlpha(props.Theme.PreviewSplit, 0.45), BorderWidth: 1,
			Child: renderUpdateMarkdown(props, props.ID+"-intro", parsed.intro, max(float32(0), width-scaled(24))),
		})
	}
	tagWidth := updateTagColumnWidth(props, parsed.sections, scaled)
	for sectionIndex, section := range parsed.sections {
		if len(section.items) == 0 {
			continue
		}
		if len(children) > 0 {
			children = append(children, woxwidget.Container{Width: width, Height: scaled(16)})
		}
		sectionChildren := []woxwidget.Widget{
			woxwidget.Text{Value: updateSectionTitle(props, section.title), Style: woxui.TextStyle{Size: scaled(18), Weight: woxui.FontWeightSemibold}, Color: props.Theme.PreviewText},
			woxwidget.Container{Width: width, Height: scaled(10)},
		}
		for itemIndex, item := range section.items {
			bodyWidth := width
			rowChildren := make([]woxwidget.Widget, 0, 2)
			if tagWidth > 0 {
				bodyWidth = max(float32(0), width-tagWidth-scaled(10))
				rowChildren = append(rowChildren, woxwidget.Container{Width: tagWidth, Padding: woxwidget.Insets{Top: scaled(2)}, Child: woxwidget.Text{Value: item.tag, Style: woxui.TextStyle{Size: scaled(11), Weight: woxui.FontWeightSemibold}, Color: updateColorAlpha(updateSectionColor(section.title, props.Theme.PreviewText), 0.9)}})
			}
			itemMarkdown := item.summary
			if item.continuation != "" {
				itemMarkdown += "\n\n" + item.continuation
			}
			rowChildren = append(rowChildren, woxwidget.Container{Width: bodyWidth, Child: renderUpdateMarkdown(props, props.ID+"-item-"+strconv.Itoa(sectionIndex)+"-"+strconv.Itoa(itemIndex), itemMarkdown, bodyWidth)})
			sectionChildren = append(sectionChildren, woxwidget.Flex{Axis: woxwidget.Horizontal, CrossAxisAlignment: woxwidget.CrossAxisStart, Gap: scaled(10), Children: rowChildren}, woxwidget.Container{Width: width, Height: scaled(10)})
		}
		children = append(children, woxwidget.Flex{Axis: woxwidget.Vertical, Children: sectionChildren})
	}
	return woxwidget.Container{Width: width, Padding: woxwidget.Insets{Right: scaled(8), Bottom: scaled(10)}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: children}}
}

func renderUpdateMarkdown(props UpdatePreviewProps, id, markdown string, width float32) woxwidget.Widget {
	if props.RenderMarkdown != nil {
		return props.RenderMarkdown(id, markdown, width)
	}
	return woxwidget.TextBlock{Value: markdown, Width: width, Style: woxui.TextStyle{Size: 12}, LineHeight: 18, Color: props.Theme.PreviewText}
}

// parseUpdateReleaseNotes recognizes the stable Wox changelog headings and optional area tags.
func parseUpdateReleaseNotes(markdown string) updateReleaseNotes {
	intro := make([]string, 0)
	sections := make([]updateReleaseNotesSection, 0)
	sectionIndex := -1
	itemIndex := -1
	seenSection := false
	for _, rawLine := range strings.Split(strings.ReplaceAll(strings.ReplaceAll(markdown, "\r\n", "\n"), "\r", "\n"), "\n") {
		line := strings.TrimRight(rawLine, " \t")
		if strings.TrimSpace(line) == "" {
			continue
		}
		if title, ok := parseUpdateSectionTitle(line); ok {
			sections = append(sections, updateReleaseNotesSection{title: title})
			sectionIndex = len(sections) - 1
			itemIndex = -1
			seenSection = true
			continue
		}
		if sectionIndex >= 0 {
			if item, ok := parseUpdateReleaseNotesItem(line); ok {
				sections[sectionIndex].items = append(sections[sectionIndex].items, item)
				itemIndex = len(sections[sectionIndex].items) - 1
				continue
			}
		}
		if sectionIndex >= 0 && itemIndex >= 0 {
			item := &sections[sectionIndex].items[itemIndex]
			if item.continuation != "" {
				item.continuation += "\n"
			}
			item.continuation += strings.TrimLeft(line, " \t")
		} else if !seenSection {
			intro = append(intro, line)
		}
	}
	structured := make([]updateReleaseNotesSection, 0, len(sections))
	for _, section := range sections {
		if len(section.items) > 0 {
			structured = append(structured, section)
		}
	}
	return updateReleaseNotes{intro: strings.Join(intro, "\n"), sections: structured}
}

func parseUpdateSectionTitle(line string) (string, bool) {
	if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") || !strings.HasPrefix(line, "- ") {
		return "", false
	}
	title := strings.TrimSpace(strings.TrimPrefix(line, "- "))
	switch strings.ToLower(title) {
	case "add", "added", "new", "improve", "improvements", "fix", "fixed", "fixes", "changed", "change", "remove", "removed", "security":
		return title, true
	default:
		return "", false
	}
}

func parseUpdateReleaseNotesItem(line string) (updateReleaseNotesItem, bool) {
	trimmed := strings.TrimLeft(line, " \t")
	if !strings.HasPrefix(trimmed, "- ") || trimmed == line {
		return updateReleaseNotesItem{}, false
	}
	value := strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))
	item := updateReleaseNotesItem{summary: value}
	if strings.HasPrefix(value, "[`") {
		if end := strings.Index(value[2:], "`]"); end >= 0 {
			item.tag = strings.TrimSpace(value[2 : end+2])
			item.summary = strings.TrimSpace(value[end+4:])
		}
	}
	return item, true
}

func updateSectionTitle(props UpdatePreviewProps, raw string) string {
	switch strings.ToLower(raw) {
	case "add", "added", "new":
		return props.SectionNew
	case "improve", "improvements":
		return props.SectionImprovements
	case "fix", "fixed", "fixes":
		return props.SectionFixes
	case "changed", "change":
		return props.SectionChanged
	case "remove", "removed":
		return props.SectionRemoved
	case "security":
		return props.SectionSecurity
	default:
		return raw
	}
}

func updateSectionColor(raw string, fallback woxui.Color) woxui.Color {
	switch strings.ToLower(raw) {
	case "add", "added", "new":
		return woxui.Color{R: 91, G: 203, B: 123, A: 255}
	case "improve", "improvements":
		return woxui.Color{R: 88, G: 166, B: 255, A: 255}
	case "fix", "fixed", "fixes":
		return woxui.Color{R: 255, G: 180, B: 84, A: 255}
	case "security":
		return woxui.Color{R: 255, G: 107, B: 122, A: 255}
	case "remove", "removed":
		return woxui.Color{R: 255, G: 122, B: 102, A: 255}
	case "changed", "change":
		return woxui.Color{R: 180, G: 140, B: 255, A: 255}
	default:
		return fallback
	}
}

func updateTagColumnWidth(props UpdatePreviewProps, sections []updateReleaseNotesSection, scaled func(float32) float32) float32 {
	widest := float32(0)
	style := woxui.TextStyle{Size: scaled(11), Weight: woxui.FontWeightSemibold}
	for _, section := range sections {
		for _, item := range section.items {
			width := scaled(float32(utf8.RuneCountInString(item.tag)) * 7)
			if props.MeasureText != nil {
				width = props.MeasureText(item.tag, style)
			}
			widest = max(widest, width)
		}
	}
	return min(scaled(180), widest+scaled(2))
}

func updatePillWidth(props UpdatePreviewProps, label string, scaled func(float32) float32) float32 {
	textWidth := scaled(float32(utf8.RuneCountInString(label)) * 7)
	if props.MeasureText != nil {
		textWidth = props.MeasureText(label, woxui.TextStyle{Size: scaled(11), Weight: woxui.FontWeightSemibold})
	}
	return max(scaled(36), textWidth+scaled(20))
}

func updateStatusPill(label string, width float32, color woxui.Color, scaled func(float32) float32) woxwidget.Widget {
	return woxwidget.Container{
		Width: width, Height: scaled(24), Radius: scaled(12), Color: updateColorAlpha(color, 0.15), BorderColor: updateColorAlpha(color, 0.4), BorderWidth: 1,
		Padding: woxwidget.Insets{Left: scaled(10), Top: scaled(5), Right: scaled(10), Bottom: scaled(4)},
		Child:   woxwidget.Text{Value: label, Style: woxui.TextStyle{Size: scaled(11), Weight: woxui.FontWeightSemibold}, Color: color},
	}
}

func updateColorAlpha(color woxui.Color, opacity float32) woxui.Color {
	color.A = uint8(min(max(float32(0), opacity), float32(1))*255 + 0.5)
	return color
}
