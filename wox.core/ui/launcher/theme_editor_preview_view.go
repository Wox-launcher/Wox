package launcher

import (
	"strings"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// buildThemeEditorPreview combines a live portable sample with the shared color form.
func (a *App) buildThemeEditorPreview(result queryResult, preview queryPreview, palette uiPalette, width, height float32) woxwidget.Widget {
	state, err := a.ensureThemeEditorPreview(result, preview)
	if err != nil {
		return woxwidget.Container{Width: width, Height: height, Padding: woxwidget.UniformInsets(18), Child: woxwidget.TextBlock{
			Value: err.Error(), Width: max(float32(0), width-36), Height: max(float32(0), height-36), Style: woxui.TextStyle{Size: 13}, Color: woxui.Color{R: 232, G: 95, B: 95, A: 255},
		}}
	}
	return a.buildThemeEditorSurface(state, palette, width, height)
}

// buildThemeEditorSurface shares the entire editor between query preview and settings navigation.
func (a *App) buildThemeEditorSurface(state *themeEditorPreviewSnapshot, palette uiPalette, width, height float32) woxwidget.Widget {
	innerWidth := max(float32(0), width-32)
	innerHeight := max(float32(0), height-24)
	headerHeight := float32(34)
	sampleHeight := min(float32(150), max(float32(96), innerHeight*0.3))
	footerHeight := float32(48)
	errorHeight := float32(0)
	if state.error != "" {
		errorHeight = 30
	}
	bodyHeight := max(float32(72), innerHeight-headerHeight-sampleHeight-footerHeight-errorHeight)
	a.setThemeEditorViewport(state.key, bodyHeight)
	contentHeight := max(bodyHeight, formDefinitionsContentHeight(state.definitions))
	callbacks := formFieldCallbacks{idPrefix: "theme-editor", focus: a.focusThemeEditorField, setCaret: a.setThemeEditorCaret}
	rows := make([]woxwidget.Widget, 0, len(state.definitions))
	for index, definition := range state.definitions {
		rows = append(rows, a.buildFormField(state.formFieldsSnapshot, callbacks, palette, index, definition, innerWidth, formDefinitionHeight(definition)))
	}
	body := woxwidget.Gesture{ID: "theme-editor-scroll", OnScroll: func(delta woxui.Point) {
		a.scrollThemeEditorPreview(state.key, -delta.Y)
	}, Child: woxwidget.ScrollView{Width: innerWidth, Height: bodyHeight, ContentHeight: contentHeight, Offset: state.scroll, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows}}}

	dirty := false
	for key, value := range state.values {
		if value != state.initial[key] {
			dirty = true
			break
		}
	}
	saveLabel := a.translate("i18n:ui_save")
	if state.isSystem || strings.TrimSpace(state.values["ThemeName"]) != state.sourceName {
		saveLabel = "Save copy"
	}
	buttonColor := palette.selectedBackground
	if dirty && !state.saving {
		buttonColor = palette.actionSelected
	}
	if state.saving {
		saveLabel += "…"
	}
	button := woxwidget.Gesture{ID: "theme-editor-save", OnTap: a.submitThemeEditorPreview, Child: woxwidget.Container{Width: 116, Height: 36, Radius: 8, Color: buttonColor, Padding: woxwidget.Insets{Left: 22, Top: 10}, Child: woxwidget.Text{
		Value: saveLabel, Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, Color: palette.actionSelectedText,
	}}}
	footer := woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{woxwidget.Painter{Width: max(float32(0), innerWidth-116), Height: footerHeight}, button}}
	children := []woxwidget.Widget{
		woxwidget.Container{Width: innerWidth, Height: headerHeight, Child: woxwidget.Text{Value: "Theme editor · edit CSS colors directly", Style: woxui.TextStyle{Size: 16, Weight: woxui.FontWeightSemibold}, Color: palette.previewText}},
		a.buildThemeDraftSample(state.values, innerWidth, sampleHeight),
		body,
	}
	if errorHeight > 0 {
		children = append(children, woxwidget.Container{Width: innerWidth, Height: errorHeight, Padding: woxwidget.Insets{Top: 7}, Child: woxwidget.Text{Value: state.error, Style: woxui.TextStyle{Size: 11}, Color: woxui.Color{R: 232, G: 95, B: 95, A: 255}}})
	}
	children = append(children, footer)
	return woxwidget.Container{Width: width, Height: height, Padding: woxwidget.Insets{Left: 16, Top: 12, Right: 16, Bottom: 12}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: children}}
}

func (a *App) buildThemeDraftSample(values map[string]string, width, height float32) woxwidget.Widget {
	draft := themeEditorPalette(values)
	innerWidth := max(float32(0), width-20)
	queryHeight := float32(32)
	toolbarHeight := float32(24)
	rowHeight := max(float32(24), (height-queryHeight-toolbarHeight-20)/2)
	query := woxwidget.Container{Width: innerWidth, Height: queryHeight, Radius: 7, Color: draft.queryBackground, Padding: woxwidget.Insets{Left: 10, Top: 8}, Child: woxwidget.Text{Value: "WOX", Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, Color: draft.queryText}}
	row := func(selected bool, title, subtitle string) woxwidget.Widget {
		background := woxui.Color{}
		titleColor := draft.resultTitle
		subtitleColor := draft.resultSubtitle
		if selected {
			background = draft.selectedBackground
			titleColor = draft.selectedTitle
			subtitleColor = draft.selectedSubtitle
		}
		return woxwidget.Container{Width: innerWidth, Height: rowHeight, Radius: 7, Color: background, Padding: woxwidget.Insets{Left: 10, Top: 6}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{
			woxwidget.Text{Value: title, Style: woxui.TextStyle{Size: 11, Weight: woxui.FontWeightSemibold}, Color: titleColor},
			woxwidget.Text{Value: subtitle, Style: woxui.TextStyle{Size: 9}, Color: subtitleColor},
		}}}
	}
	toolbar := woxwidget.Container{Width: innerWidth, Height: toolbarHeight, Color: draft.toolbarBackground, Padding: woxwidget.Insets{Left: 10, Top: 6}, Child: woxwidget.Text{Value: "Open   ·   Actions", Style: woxui.TextStyle{Size: 9}, Color: draft.toolbarText}}
	return woxwidget.Container{Width: width, Height: height, Radius: 10, Color: draft.background, Padding: woxwidget.UniformInsets(10), Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 4, Children: []woxwidget.Widget{
		query,
		row(false, "Wox Go UI", "Portable GPU-rendered theme preview"),
		row(true, "Selected result", "Colors update as you type"),
		toolbar,
	}}}
}
