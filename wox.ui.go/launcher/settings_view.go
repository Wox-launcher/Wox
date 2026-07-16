package launcher

import (
	woxui "github.com/Wox-launcher/wox.ui.go"
	woxwidget "github.com/Wox-launcher/wox.ui.go/widget"
)

func (a *App) buildSettings(frame woxui.FrameInfo) woxwidget.Widget {
	snapshot := a.settingsSnapshot()
	items := settingItemsForSnapshot(snapshot)
	if snapshot.row >= len(items) && len(items) > 0 {
		snapshot.row = len(items) - 1
	}
	width := frame.Size.Width
	height := frame.Size.Height
	railWidth := min(float32(250), max(float32(210), width*0.22))
	var page woxwidget.Widget
	if snapshot.tab == "plugins" {
		page = a.buildPluginSettingsPage(snapshot, width-railWidth, height)
	} else if snapshot.tab == "hotkeys" {
		page = a.buildHotkeySettingsPage(snapshot, width-railWidth, height)
	} else if snapshot.tab == "theme" {
		page = a.buildSettingsThemePage(snapshot, width-railWidth, height)
	} else if snapshot.tab == "ai" {
		page = a.buildAISettingsPage(snapshot, width-railWidth, height)
	} else if snapshot.tab == "data" {
		page = a.buildDataSettingsPage(snapshot, width-railWidth, height)
	} else if snapshot.tab == "cloud" {
		page = a.buildCloudSettingsPage(snapshot, width-railWidth, height)
	} else if snapshot.tab == "runtime" {
		page = a.buildRuntimeSettingsPage(snapshot, items, width-railWidth, height)
	} else if snapshot.tab == "usage" {
		page = a.buildUsageSettingsPage(snapshot, width-railWidth, height)
	} else if snapshot.tab == "about" {
		page = a.buildAboutSettingsPage(snapshot, width-railWidth, height)
	} else if snapshot.tab == "privacy" {
		page = a.buildPrivacySettingsPage(snapshot, width-railWidth, height)
	} else {
		page = a.buildSettingsPage(snapshot, items, width-railWidth, height)
	}
	body := woxwidget.Container{
		Width: width, Height: height, Color: snapshot.palette.background, Radius: 14,
		Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{
			a.buildSettingsRail(snapshot, railWidth, height),
			page,
		}},
	}
	if snapshot.tableEditor != nil {
		return woxwidget.Container{Width: width, Height: height, Color: snapshot.palette.background, Radius: 14, Child: woxwidget.Stack{Width: width, Height: height, Children: []woxwidget.StackChild{
			{Child: body},
			{Child: a.buildFormTableOverlay(snapshot.tableEditor, snapshot.palette, width, height)},
		}}}
	}
	if snapshot.modelManager != nil {
		return woxwidget.Container{Width: width, Height: height, Color: snapshot.palette.background, Radius: 14, Child: woxwidget.Stack{Width: width, Height: height, Children: []woxwidget.StackChild{
			{Child: body},
			{Child: a.buildModelManagerOverlay(snapshot.modelManager, snapshot.palette, width, height)},
		}}}
	}
	if snapshot.choicePicker != nil {
		return woxwidget.Container{Width: width, Height: height, Color: snapshot.palette.background, Radius: 14, Child: woxwidget.Stack{Width: width, Height: height, Children: []woxwidget.StackChild{
			{Child: body},
			{Child: a.buildSettingChoicePickerOverlay(snapshot.choicePicker, snapshot.palette, width, height)},
		}}}
	}
	if snapshot.cloudForm != nil {
		return woxwidget.Container{Width: width, Height: height, Color: snapshot.palette.background, Radius: 14, Child: woxwidget.Stack{Width: width, Height: height, Children: []woxwidget.StackChild{
			{Child: body},
			{Child: a.buildCloudFormOverlay(snapshot.cloudForm, snapshot.palette, width, height)},
		}}}
	}
	return body
}

// buildSettingsThemePage mounts theme catalogs and the shared live editor under one portable route.
func (a *App) buildSettingsThemePage(snapshot settingsSnapshot, width, height float32) woxwidget.Widget {
	innerWidth := max(float32(0), width-48)
	headerHeight := float32(68)
	modeLabel := "Installed themes"
	if snapshot.themesMode == "store" {
		modeLabel = "Browse and install themes from the Wox store"
	} else if snapshot.themesMode == "editor" {
		modeLabel = "Edit the active theme and save a portable copy"
	}
	header := woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: []woxwidget.Widget{
		woxwidget.Container{Width: max(float32(0), innerWidth-310), Height: headerHeight, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 6, Children: []woxwidget.Widget{
			woxwidget.Text{Value: "Themes", Style: woxui.TextStyle{Size: 24, Weight: woxui.FontWeightSemibold}, Color: snapshot.palette.queryText},
			woxwidget.Text{Value: modeLabel, Style: woxui.TextStyle{Size: 13}, Color: snapshot.palette.resultSubtitle},
		}}},
		a.buildFormTableButton("themes-mode-installed", "Installed", 98, !snapshot.themesLoading && snapshot.themeOperation == "", snapshot.themesMode == "installed", func() { a.switchThemeSettingsMode("installed") }, snapshot.palette),
		a.buildFormTableButton("themes-mode-store", "Store", 88, !snapshot.themesLoading && snapshot.themeOperation == "", snapshot.themesMode == "store", func() { a.switchThemeSettingsMode("store") }, snapshot.palette),
		a.buildFormTableButton("themes-mode-editor", "Editor", 88, !snapshot.themesLoading && snapshot.themeOperation == "", snapshot.themesMode == "editor", func() { a.switchThemeSettingsMode("editor") }, snapshot.palette),
	}}
	bodyHeight := max(float32(0), height-48-headerHeight)
	var body woxwidget.Widget
	if snapshot.themesMode != "editor" {
		body = a.buildThemeCatalog(snapshot, innerWidth, bodyHeight)
		return woxwidget.Container{Width: width, Height: height, Padding: woxwidget.Insets{Left: 24, Top: 24, Right: 24, Bottom: 24}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{header, body}}}
	}
	a.mu.RLock()
	theme := snapshotThemeEditorPreviewLocked(a.themeEditor)
	a.mu.RUnlock()
	if theme == nil {
		message := "Loading active theme…"
		if snapshot.themesError != "" {
			message = snapshot.themesError
		}
		body = woxwidget.Container{Width: innerWidth, Height: bodyHeight, Padding: woxwidget.Insets{Top: 24}, Child: woxwidget.TextBlock{
			Value: message, Width: innerWidth, Height: 80, Style: woxui.TextStyle{Size: 13}, LineHeight: 19, Color: snapshot.palette.resultSubtitle,
		}}
	} else {
		body = a.buildThemeEditorSurface(theme, snapshot.palette, innerWidth, bodyHeight)
	}
	return woxwidget.Container{Width: width, Height: height, Padding: woxwidget.Insets{Left: 24, Top: 24, Right: 24, Bottom: 24}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{header, body}}}
}

func (a *App) buildSettingsRail(snapshot settingsSnapshot, width, height float32) woxwidget.Widget {
	visibleTabs := settingTabs(snapshot.isDev)
	tabs := make([]woxwidget.Widget, 0, len(visibleTabs))
	for _, tab := range visibleTabs {
		tab := tab
		color := woxui.Color{}
		foreground := snapshot.palette.toolbarText
		if tab.id == snapshot.tab {
			color = snapshot.palette.selectedBackground
			foreground = snapshot.palette.selectedTitle
		}
		tabs = append(tabs, woxwidget.Gesture{
			ID:    "settings-tab-" + tab.id,
			OnTap: func() { a.selectSettingTab(tab.id) },
			Child: woxwidget.Container{
				Width: width - 32, Height: 48, Radius: 9, Color: color,
				Padding: woxwidget.Insets{Left: 16, Top: 14},
				Child:   woxwidget.Text{Value: tab.label, Style: woxui.TextStyle{Size: 15, Weight: woxui.FontWeightSemibold}, Color: foreground},
			},
		})
	}
	contentHeight := settingsRailContentHeight(len(tabs))
	viewportHeight := max(float32(1), height-48)
	a.setSettingsRailViewport(viewportHeight)
	return woxwidget.Container{
		Width: width, Height: height, Color: snapshot.palette.toolbarBackground,
		Padding: woxwidget.Insets{Left: 16, Top: 28, Right: 16, Bottom: 20},
		Child: woxwidget.Gesture{ID: "settings-rail-scroll", OnScroll: func(delta woxui.Point) { a.scrollSettingsRail(-delta.Y) }, Child: woxwidget.ScrollView{
			Width: width - 32, Height: viewportHeight, ContentHeight: contentHeight, Offset: snapshot.railScroll, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 8, Children: append([]woxwidget.Widget{
				woxwidget.Container{Width: width - 32, Height: 58, Padding: woxwidget.Insets{Left: 10}, Child: woxwidget.Text{Value: "WOX SETTINGS", Style: woxui.TextStyle{Size: 20, Weight: woxui.FontWeightSemibold}, Color: snapshot.palette.queryText}},
			}, tabs...)},
		}},
	}
}

func (a *App) buildSettingsPage(snapshot settingsSnapshot, items []settingItem, width, height float32) woxwidget.Widget {
	pageTitle := "General"
	for _, tab := range settingTabs(snapshot.isDev) {
		if tab.id == snapshot.tab {
			pageTitle = tab.label
			break
		}
	}
	contentWidth := max(float32(0), width-72)
	rows := make([]woxwidget.Widget, 0, len(items))
	for index, item := range items {
		index := index
		item := item
		selected := index == snapshot.row
		background := snapshot.palette.queryBackground
		if selected {
			background = snapshot.palette.selectedBackground
		}
		rows = append(rows, woxwidget.Gesture{
			ID: "setting-" + item.key,
			OnHover: func(inside bool) {
				if inside {
					a.selectSettingRow(index)
				}
			},
			OnTap: func() {
				a.selectSettingRow(index)
				a.openOrActivateSetting()
			},
			OnScroll: func(delta woxui.Point) {
				a.scrollSettingsPage(-delta.Y)
			},
			Child: a.buildSettingRow(snapshot, item, contentWidth, background),
		})
	}
	note := snapshot.note
	if note == "" {
		note = "Arrow keys select · Enter changes · Esc returns"
	}
	viewportHeight := max(float32(1), height-52)
	a.setSettingsPageViewport(viewportHeight, len(items))
	content := woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 12, Children: []woxwidget.Widget{
		woxwidget.Container{Width: contentWidth, Height: 62, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 7, Children: []woxwidget.Widget{
			woxwidget.Text{Value: pageTitle, Style: woxui.TextStyle{Size: 24, Weight: woxui.FontWeightSemibold}, Color: snapshot.palette.queryText},
			woxwidget.Text{Value: "Native Go UI settings backed by Wox core", Style: woxui.TextStyle{Size: 13}, Color: snapshot.palette.resultSubtitle},
		}}},
		woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 9, Children: rows},
		woxwidget.Container{Width: contentWidth, Height: 34, Padding: woxwidget.Insets{Top: 10}, Child: woxwidget.Text{Value: note, Style: woxui.TextStyle{Size: 12}, Color: snapshot.palette.resultSubtitle}},
	}}
	return woxwidget.Container{
		Width: width, Height: height,
		Padding: woxwidget.Insets{Left: 36, Top: 30, Right: 36, Bottom: 22},
		Child: woxwidget.Gesture{ID: "settings-page-scroll", OnScroll: func(delta woxui.Point) { a.scrollSettingsPage(-delta.Y) }, Child: woxwidget.ScrollView{
			Width: contentWidth, Height: viewportHeight, ContentHeight: max(viewportHeight, settingsPageContentHeight(len(items))), Offset: snapshot.pageScroll, Child: content,
		}},
	}
}

func settingsPageContentHeight(itemCount int) float32 {
	return 62 + 12 + float32(itemCount*79) + 12 + 34
}

func (a *App) buildSettingRow(snapshot settingsSnapshot, item settingItem, width float32, background woxui.Color) woxwidget.Widget {
	palette := snapshot.palette
	foreground := palette.resultTitle
	subtitle := palette.resultSubtitle
	valueColor := palette.cursor
	if item.disabled {
		foreground = palette.resultSubtitle
		valueColor = palette.resultSubtitle
	}
	valueWidth := min(float32(170), max(float32(110), width*0.22))
	if item.text {
		valueWidth = min(float32(440), max(float32(280), width*0.46))
	}
	labelWidth := max(float32(180), width-valueWidth-48)
	var valueField woxwidget.Widget
	if item.text {
		focused := snapshot.editKey == item.key
		state := woxui.TextEditingState{Text: item.value}
		if focused {
			state = snapshot.editing
		}
		style := woxui.TextStyle{Size: 13}
		inputWidth := valueWidth
		if item.browseFile {
			inputWidth = max(float32(120), valueWidth-82)
		}
		input := woxwidget.Gesture{ID: "setting-text-" + item.key, OnTapAt: func(position woxui.Point) {
			offset := formTextOffsetAt(state, a.window, style, 1, inputWidth-24, woxui.Point{X: max(float32(0), position.X-12), Y: max(float32(0), position.Y-8)})
			a.startBuiltInSettingEdit(item, offset)
		}, Child: woxwidget.Container{Width: inputWidth, Height: 38, Radius: 8, Color: palette.toolbarBackground, Padding: woxwidget.Insets{Left: 12, Top: 8, Right: 12, Bottom: 6}, Child: woxwidget.Clip{
			Width: inputWidth - 24, Height: 24, Child: woxwidget.Painter{Width: inputWidth - 24, Height: 24, Paint: func(displayList *woxui.DisplayList, bounds woxui.Rect) {
				drawFormEditor(displayList, bounds, state, style, palette, focused, 1, a.window)
			}},
		}}}
		valueField = input
		if item.browseFile {
			valueField = woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: []woxwidget.Widget{
				input,
				woxwidget.Gesture{ID: "setting-browse-" + item.key, OnTap: func() { a.browseBuiltInSettingFile(item) }, Child: woxwidget.Container{
					Width: 74, Height: 38, Radius: 8, Color: palette.toolbarBackground, Padding: woxwidget.Insets{Left: 13, Top: 11}, Child: woxwidget.Text{Value: "Browse", Style: woxui.TextStyle{Size: 10, Weight: woxui.FontWeightSemibold}, Color: palette.cursor},
				}},
			}}
		}
	} else {
		valueField = woxwidget.Container{
			Width: valueWidth, Height: 38, Radius: 8, Color: palette.toolbarBackground,
			Padding: woxwidget.Insets{Left: 14, Top: 10, Right: 12},
			Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: []woxwidget.Widget{
				woxwidget.Text{Value: settingValueLabel(item), Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: valueColor},
				woxwidget.Text{Value: "‹  ›", Style: woxui.TextStyle{Size: 13}, Color: subtitle},
			}},
		}
	}
	return woxwidget.Container{
		Width: width, Height: 70, Radius: 10, Color: background,
		Padding: woxwidget.Insets{Left: 18, Top: 9, Right: 16, Bottom: 9},
		Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 14, Children: []woxwidget.Widget{
			woxwidget.Container{Width: labelWidth, Height: 52, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 5, Children: []woxwidget.Widget{
				woxwidget.Text{Value: item.title, Style: woxui.TextStyle{Size: 16, Weight: woxui.FontWeightSemibold}, Color: foreground},
				woxwidget.Text{Value: item.description, Style: woxui.TextStyle{Size: 12}, Color: subtitle},
			}}},
			valueField,
		}},
	}
}
