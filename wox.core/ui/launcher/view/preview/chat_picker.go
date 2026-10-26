package preview

import (
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// ChatPickerSectionGap separates providers with whitespace and content types with a divider.
const ChatPickerSectionGap = float32(12)

// ChatCatalogFilter carries a localized category and its stable selection callback.
type ChatCatalogFilter struct {
	ID, Label string
	Selected  bool
	OnSelect  func()
}

// ChatPickerHeight uses the same density-scaled header and padding as the picker.
func ChatPickerHeight(contentHeight, scale, available float32) float32 {
	return min(max(ChatScaledSize(scale, 40), contentHeight)+ChatPickerChromeHeight(scale), ChatScaledSize(scale, 420), max(0, available))
}

// ChatPickerChromeHeight sums rounded parts so density never introduces cumulative scroll drift.
func ChatPickerChromeHeight(scale float32) float32 {
	return 2*ChatScaledSize(scale, 10) + ChatScaledSize(scale, 32) + ChatScaledSize(scale, 6)
}

// chatPickerCatalog keeps the category/search header fixed above one complete scrolling list.
func chatPickerCatalog(props ChatCatalogProps) woxwidget.Widget {
	s := func(value float32) float32 { return ChatScaledSize(props.DensityScale, value) }
	width := max(float32(0), props.Width-s(20))
	viewport := max(float32(0), props.Height-ChatPickerChromeHeight(props.DensityScale))
	controls := props.Theme.Controls
	controls.DensityScale = props.DensityScale
	var header woxwidget.Widget
	if props.Search != nil {
		search := *props.Search
		search.Width, search.Height = width, s(32)
		search.Theme = controls
		search.Padding = woxwidget.Insets{Left: s(10), Right: s(10), Top: s(6), Bottom: s(6)}
		search.LineHeight = s(20)
		header = woxcomponent.WoxTextField(search)
	} else {
		buttons := make([]woxwidget.Widget, 0, len(props.Filters))
		for _, filter := range props.Filters {
			variant := woxcomponent.ButtonText
			if filter.Selected {
				variant = woxcomponent.ButtonSelected
			}
			buttons = append(buttons, woxcomponent.WoxButton(woxcomponent.ButtonProps{
				ID: filter.ID, Label: filter.Label, Width: min(s(72), max(0, (width-s(8))/3)), Height: s(32),
				Variant: variant, Selected: filter.Selected, Theme: controls,
				OnTap: filter.OnSelect,
			}))
		}
		header = woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: s(4), Children: buttons}
	}
	rows := make([]woxwidget.Widget, 0, len(props.Items)*2)
	border := props.Theme.ResultSubtitle
	border.A = uint8(float32(border.A) * 0.18)
	section, group := "", ""
	for _, item := range props.Items {
		if item.Kind != section {
			if section != "" {
				rows = append(rows, woxwidget.Align{Width: width, Height: s(ChatPickerSectionGap), Vertical: 0.5,
					Child: woxwidget.Container{Width: width, Height: 1, Color: border}})
			}
			section, group = item.Kind, ""
			rows = append(rows, woxwidget.Container{Width: width, Height: s(28), Padding: woxwidget.Insets{Left: s(10)},
				Child: woxwidget.Align{Height: s(28), Vertical: 0.5, Child: woxwidget.Text{Value: item.SectionLabel,
					Style: woxui.TextStyle{Size: s(woxcomponent.SettingsHelpFontSize), Weight: woxui.FontWeightSemibold}, Color: props.Theme.ResultSubtitle}}})
		}
		if item.Kind == "models" && item.GroupLabel != "" && item.GroupLabel != group {
			if group != "" {
				rows = append(rows, woxwidget.Container{Width: width, Height: s(ChatPickerSectionGap)})
			}
			group = item.GroupLabel
			// Keep the icon slot stable while the shared provider catalog/image is loading.
			var icon woxwidget.Widget = woxcomponent.SparklesGlyph(s(20), props.Theme.ResultSubtitle)
			if item.GroupIcon != nil {
				icon = woxwidget.Image{Source: item.GroupIcon, Width: s(20), Height: s(20)}
			}
			label := []woxwidget.Widget{icon, woxwidget.Expanded{Child: woxwidget.Text{Value: item.GroupLabel,
				Style: woxui.TextStyle{Size: s(woxcomponent.SettingsHelpFontSize), Weight: woxui.FontWeightSemibold}, Color: props.Theme.ResultSubtitle}}}
			rows = append(rows, woxwidget.Container{Width: width, Height: s(28), Padding: woxwidget.Insets{Left: s(10)},
				Child: woxwidget.Flex{Axis: woxwidget.Horizontal, CrossAxisAlignment: woxwidget.CrossAxisCenter, Gap: s(8), Children: label}})
		}
		item.DensityScale = props.DensityScale
		if item.Placeholder {
			rows = append(rows, woxwidget.Container{Width: width, Height: s(item.RowHeight), Padding: woxwidget.Insets{Left: s(38)},
				Child: woxwidget.Align{Height: s(item.RowHeight), Vertical: 0.5, Child: woxwidget.Text{
					Value: item.Title, Style: woxui.TextStyle{Size: s(woxcomponent.SettingsHelpFontSize)}, Color: props.Theme.ResultSubtitle}}})
			continue
		}
		rows = append(rows, ChatCatalogItem(item, width, s(item.RowHeight), props.Theme))
	}
	if len(rows) == 0 {
		rows = append(rows, woxwidget.Container{Width: width, Height: viewport, Padding: woxwidget.Insets{Left: s(10), Top: s(12)},
			Child: woxwidget.Text{Value: props.EmptyMessage, Style: woxui.TextStyle{Size: s(woxcomponent.SettingsHelpFontSize)}, Color: props.Theme.ResultSubtitle}})
	}
	return woxwidget.Container{Width: props.Width, Height: props.Height, Radius: s(12), Floating: true,
		Color: props.Theme.ActionBackground, BorderColor: border, BorderWidth: 1, Padding: woxwidget.UniformInsets(s(10)),
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: s(6), Children: []woxwidget.Widget{
			header,
			woxcomponent.WoxScrollView(woxcomponent.ScrollViewProps{
				Key: woxwidget.Key("chat-catalog-scroll-" + props.Key), Width: width, Height: viewport,
				ContentHeight: props.ContentHeight, Offset: props.Scroll, Theme: props.Theme.Controls,
				ThumbColor: props.Theme.ResultSubtitle, OnScroll: props.OnScroll,
				Content: woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows},
			}),
		}}}
}

// chatPickerItem separates the active model check from keyboard selection and pointer hover.
func chatPickerItem(item ChatCatalogItemProps, width, height float32, theme woxcomponent.Theme, hovered bool, onHover func(bool)) woxwidget.Widget {
	s := func(value float32) float32 { return ChatScaledSize(item.DensityScale, value) }
	background, foreground := woxui.Color{}, theme.PreviewText
	if item.Selected {
		background, foreground = theme.SelectedBackground, theme.SelectedTitle
	} else if hovered {
		background = theme.SelectedBackground
		background.A = uint8(float32(background.A) * 0.5)
	}
	texts := []woxwidget.Widget{woxwidget.Text{Value: item.Title, Style: woxui.TextStyle{Size: s(woxcomponent.SettingsControlFontSize)}, Color: foreground}}
	if item.Kind == "skills" && item.Subtitle != "" {
		texts = append(texts, woxwidget.Text{Value: item.Subtitle, Style: woxui.TextStyle{Size: s(woxcomponent.SettingsHelpFontSize)}, Color: theme.ResultSubtitle})
	}
	content := []woxwidget.Widget{}
	if item.Icon != nil {
		content = append(content, woxwidget.Image{Source: item.Icon, Width: s(20), Height: s(20)})
	} else if item.Kind == "skills" {
		content = append(content, woxcomponent.ExtensionGlyph(s(18), foreground))
	}
	content = append(content, woxwidget.Expanded{Child: woxwidget.Flex{Axis: woxwidget.Vertical, MainAxisAlignment: woxwidget.MainAxisCenter, Gap: s(2), Children: texts}})
	if item.Current {
		content = append(content, woxcomponent.CheckGlyph(s(16), foreground))
	}
	leftPadding := s(12)
	if item.Kind == "models" {
		leftPadding = s(38)
	}
	return woxwidget.Semantics{AutomationID: item.SelectID, Role: woxui.AccessibilityRoleListItem, Label: item.Title,
		Description: item.Subtitle, Selected: item.Selected, Actions: []woxui.AccessibilityAction{woxui.AccessibilityActionActivate},
		Child: woxwidget.Gesture{ID: item.SelectID, OnTap: item.OnSelect, OnHover: onHover,
			Child: woxwidget.Container{Width: width, Height: height, Radius: s(7), Color: background,
				Padding: woxwidget.Insets{Left: leftPadding, Right: s(12)},
				Child:   woxwidget.Flex{Axis: woxwidget.Horizontal, CrossAxisAlignment: woxwidget.CrossAxisCenter, Gap: s(10), Children: content}}}}
}
