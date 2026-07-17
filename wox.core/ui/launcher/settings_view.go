package launcher

import (
	"fmt"
	"log"
	"runtime"
	"strings"
	"sync"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const settingsTitleBarHeight = 42

var (
	settingNavIconPaths     map[string]string
	settingNavIconPathsOnce sync.Once
)

func (a *App) buildSettings(frame woxui.FrameInfo) woxwidget.Widget {
	snapshot := a.settingsSnapshot()
	items := settingItemsForSnapshot(snapshot)
	if snapshot.row >= len(items) && len(items) > 0 {
		snapshot.row = len(items) - 1
	}
	width := frame.Size.Width
	height := frame.Size.Height
	contentHeight := max(float32(0), height-settingsTitleBarHeight)
	surfaceRadius := appSurfaceRadius()
	railWidth := min(float32(250), max(float32(210), width*0.22))
	var page woxwidget.Widget
	if snapshot.tab == "plugins" {
		page = a.buildPluginSettingsPage(snapshot, width-railWidth, contentHeight)
	} else if snapshot.tab == "theme" {
		page = a.buildSettingsThemePage(snapshot, width-railWidth, contentHeight)
	} else if snapshot.tab == "ai" {
		page = a.buildAISettingsPage(snapshot, width-railWidth, contentHeight)
	} else if snapshot.tab == "data" {
		page = a.buildDataSettingsPage(snapshot, width-railWidth, contentHeight)
	} else if snapshot.tab == "cloud" {
		page = a.buildCloudSettingsPage(snapshot, width-railWidth, contentHeight)
	} else if snapshot.tab == "runtime" {
		page = a.buildRuntimeSettingsPage(snapshot, items, width-railWidth, contentHeight)
	} else if snapshot.tab == "usage" {
		page = a.buildUsageSettingsPage(snapshot, width-railWidth, contentHeight)
	} else if snapshot.tab == "about" {
		page = a.buildAboutSettingsPage(snapshot, width-railWidth, contentHeight)
	} else if snapshot.tab == "privacy" {
		page = a.buildPrivacySettingsPage(snapshot, width-railWidth, contentHeight)
	} else {
		page = a.buildSettingsPage(snapshot, items, width-railWidth, contentHeight)
	}
	content := woxwidget.Container{
		Width: width, Height: contentHeight, Color: snapshot.palette.background,
		Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{
			a.buildSettingsRail(snapshot, railWidth, contentHeight),
			page,
		}},
	}
	body := woxwidget.Container{Width: width, Height: height, Color: snapshot.palette.background, Radius: surfaceRadius, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{
		a.buildSettingsTitleBar(snapshot, width),
		content,
	}}}
	if snapshot.tableEditor != nil {
		return woxwidget.Container{Width: width, Height: height, Color: snapshot.palette.background, Radius: surfaceRadius, Child: woxwidget.Stack{Width: width, Height: height, Children: []woxwidget.StackChild{
			{Child: body},
			{Child: a.buildFormTableOverlay(snapshot.tableEditor, snapshot.palette, width, height)},
		}}}
	}
	if snapshot.modelManager != nil {
		return woxwidget.Container{Width: width, Height: height, Color: snapshot.palette.background, Radius: surfaceRadius, Child: woxwidget.Stack{Width: width, Height: height, Children: []woxwidget.StackChild{
			{Child: body},
			{Child: a.buildModelManagerOverlay(snapshot.modelManager, snapshot.palette, width, height)},
		}}}
	}
	if snapshot.choicePicker != nil {
		return woxwidget.Container{Width: width, Height: height, Color: snapshot.palette.background, Radius: surfaceRadius, Child: woxwidget.Stack{Width: width, Height: height, Children: []woxwidget.StackChild{
			{Child: body},
			{Child: a.buildSettingChoicePickerOverlay(snapshot.choicePicker, snapshot.palette, width, height)},
		}}}
	}
	if snapshot.cloudForm != nil {
		return woxwidget.Container{Width: width, Height: height, Color: snapshot.palette.background, Radius: surfaceRadius, Child: woxwidget.Stack{Width: width, Height: height, Children: []woxwidget.StackChild{
			{Child: body},
			{Child: a.buildCloudFormOverlay(snapshot.cloudForm, snapshot.palette, width, height)},
		}}}
	}
	return body
}

func (a *App) buildSettingsTitleBar(snapshot settingsSnapshot, width float32) woxwidget.Widget {
	title := a.activeSettingsNavLabel(snapshot)
	titleStyle := woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}
	titleWidth := float32(160)
	if window := a.settingsNativeWindow(); window != nil {
		if metrics, err := window.MeasureText(title, titleStyle); err == nil {
			titleWidth = metrics.Size.Width + 24
		}
	}
	dragArea := woxwidget.Gesture{ID: "settings-title-drag", OnDragStart: func() {
		if window := a.settingsNativeWindow(); window != nil {
			_ = window.StartDragging()
		}
	}, Child: woxwidget.Container{Width: width, Height: settingsTitleBarHeight, Color: snapshot.palette.toolbarBackground}}
	children := []woxwidget.StackChild{
		{Child: dragArea},
		{Left: max(float32(0), (width-titleWidth)/2), Top: 12, Child: woxwidget.Container{Width: titleWidth, Height: 24, Child: woxwidget.Text{Value: title, Style: titleStyle, Color: snapshot.palette.toolbarText}}},
	}
	if runtime.GOOS == "darwin" {
		return woxwidget.Stack{Width: width, Height: settingsTitleBarHeight, Children: children}
	}
	closeWidth := float32(52)
	closeButton := woxwidget.Gesture{ID: "settings-window-close", OnTap: func() {
		go func() {
			if err := a.closeSettings(); err != nil {
				log.Printf("close settings window: %v", err)
			}
		}()
	}, Child: woxwidget.Container{Width: closeWidth, Height: settingsTitleBarHeight, Color: snapshot.palette.toolbarBackground, Padding: woxwidget.Insets{Left: 20, Top: 10}, Child: woxwidget.Text{
		Value: "×", Style: woxui.TextStyle{Size: 18, Weight: woxui.FontWeightSemibold}, Color: snapshot.palette.toolbarText,
	}}}
	children = append(children, woxwidget.StackChild{Left: max(float32(0), width-closeWidth), Child: closeButton})
	return woxwidget.Stack{Width: width, Height: settingsTitleBarHeight, Children: children}
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
	specs := settingNavSpecs(snapshot.isDev)
	activeID := activeSettingNavID(snapshot.tab, snapshot.pluginsStore, snapshot.themesMode)
	items := make([]woxwidget.Widget, 0, len(specs))
	for _, spec := range specs {
		spec := spec
		label := a.settingNavLabel(spec)
		color := woxui.Color{}
		foreground := snapshot.palette.toolbarText
		if spec.id == activeID {
			color = snapshot.palette.selectedBackground
			foreground = snapshot.palette.selectedTitle
		}
		labelStyle := woxui.TextStyle{Size: 13}
		if spec.parent {
			labelStyle.Weight = woxui.FontWeightSemibold
		}
		leftPadding := float32(10 + spec.depth*18)
		trailing := ""
		if spec.parent {
			trailing = "⌄"
		}
		var icon woxwidget.Widget = woxwidget.Text{Value: spec.icon, Style: woxui.TextStyle{Size: 15}, Color: foreground}
		if source := settingNavIconSource(spec.id); source.ImageData != "" {
			if image := a.imageForTint(source, &foreground, 24); image != nil {
				icon = woxwidget.Container{Width: 18, Height: 22, Padding: woxwidget.Insets{Top: 2}, Child: woxwidget.Image{Source: image, Width: 18, Height: 18}}
			}
		}
		row := woxwidget.Container{
			Width: width - 28, Height: 46, Radius: 6, Color: color,
			Padding: woxwidget.Insets{Left: leftPadding, Top: 12, Right: 10},
			Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 10, Children: []woxwidget.Widget{
				woxwidget.Container{Width: 22, Height: 24, Child: icon},
				woxwidget.Container{Width: max(float32(0), width-leftPadding-98), Height: 24, Child: woxwidget.Text{Value: label, Style: labelStyle, Color: foreground}},
				woxwidget.Container{Width: 18, Height: 24, Child: woxwidget.Text{Value: trailing, Style: woxui.TextStyle{Size: 13}, Color: snapshot.palette.resultSubtitle}},
			}},
		}
		items = append(items, woxwidget.Gesture{
			ID:    "settings-nav-" + spec.id,
			OnTap: func() { a.selectSettingsNavItem(spec) },
			Child: row,
		})
	}
	innerWidth := width - 28
	searchAreaHeight := float32(58)
	viewportHeight := max(float32(1), height-searchAreaHeight-28)
	a.setSettingsRailViewport(viewportHeight)
	nav := woxwidget.Gesture{ID: "settings-rail-scroll", OnScroll: func(delta woxui.Point) { a.scrollSettingsRail(-delta.Y) }, Child: woxwidget.ScrollView{
		Width: innerWidth, Height: viewportHeight, ContentHeight: max(viewportHeight, settingsRailContentHeight(len(items))), Offset: snapshot.railScroll,
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 4, Children: items},
	}}
	stackChildren := []woxwidget.StackChild{{Child: nav}}
	if snapshot.searchPanel && strings.TrimSpace(snapshot.searchQuery.Text) != "" {
		stackChildren = append(stackChildren, woxwidget.StackChild{Child: a.buildSettingsSearchResultPanel(snapshot, innerWidth, viewportHeight)})
	}
	return woxwidget.Container{
		Width: width, Height: height, Color: snapshot.palette.toolbarBackground,
		Padding: woxwidget.Insets{Left: 14, Top: 14, Right: 14, Bottom: 14},
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{
			a.buildSettingsSearchBox(snapshot, innerWidth),
			woxwidget.Stack{Width: innerWidth, Height: viewportHeight, Children: stackChildren},
		}},
	}
}

// settingNavIconSource maps the Flutter rail's line-icon semantics onto portable monochrome SVGs.
func settingNavIconSource(id string) woxImage {
	const start = `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="#fff" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">`
	const end = `</svg>`
	settingNavIconPathsOnce.Do(func() {
		settingNavIconPaths = map[string]string{
			"general":           `<path d="M12 15.5a3.5 3.5 0 1 0 0-7 3.5 3.5 0 0 0 0 7z"/><path d="M19.4 15a1.7 1.7 0 0 0 .34 1.88l.06.06-2.83 2.83-.06-.06A1.7 1.7 0 0 0 15 19.4a1.7 1.7 0 0 0-1 .6 1.7 1.7 0 0 0-.4 1.1V21H9.6v-.1A1.7 1.7 0 0 0 8.5 19.4a1.7 1.7 0 0 0-1.88.34l-.06.06-2.83-2.83.06-.06A1.7 1.7 0 0 0 4.6 15a1.7 1.7 0 0 0-.6-1 1.7 1.7 0 0 0-1.1-.4H3V9.6h.1A1.7 1.7 0 0 0 4.6 8.5a1.7 1.7 0 0 0-.34-1.88l-.06-.06 2.83-2.83.06.06A1.7 1.7 0 0 0 9 4.6a1.7 1.7 0 0 0 1-.6 1.7 1.7 0 0 0 .4-1.1V3h4v.1A1.7 1.7 0 0 0 15.5 4.6a1.7 1.7 0 0 0 1.88-.34l.06-.06 2.83 2.83-.06.06A1.7 1.7 0 0 0 19.4 9c.4.28.75.62 1 .99.25.38.39.82.4 1.27v1.48c-.01.45-.15.9-.4 1.27-.25.37-.6.71-1 .99z"/>`,
			"ui":                `<path d="M12 3a9 9 0 1 0 0 18h1.5a1.5 1.5 0 0 0 0-3H12a1.5 1.5 0 0 1 0-3h2a7 7 0 0 0 7-7c0-2.76-4.03-5-9-5z"/><path d="M7.5 10.5h.01M9.5 6.5h.01M14.5 6.5h.01M17 10h.01"/>`,
			"ai":                `<path d="M9.5 18H8a4 4 0 0 1-4-4 3.5 3.5 0 0 1 2-3.15V9a4 4 0 0 1 7.5-1.94A3.5 3.5 0 0 1 20 9v1.85A3.5 3.5 0 0 1 19.5 17H18"/><path d="M9 13h6M10 17h4M11 21h2"/>`,
			"network":           `<circle cx="12" cy="12" r="9"/><path d="M3 12h18M12 3a14 14 0 0 1 0 18M12 3a14 14 0 0 0 0 18"/>`,
			"data":              `<path d="M3 7.5h6l2-2h10v13H3z"/>`,
			"data.backup":       `<path d="M7 18h10a4 4 0 0 0 .6-7.96A6 6 0 0 0 6.2 8.6 4.7 4.7 0 0 0 7 18z"/><path d="m9 13 3-3 3 3M12 10v6"/>`,
			"data.cloudsync":    `<path d="M7 18h10a4 4 0 0 0 .6-7.96A6 6 0 0 0 6.2 8.6 4.7 4.7 0 0 0 7 18z"/>`,
			"plugins":           `<path d="M8.5 3v4H5a2 2 0 0 0-2 2v3.5h4a2 2 0 1 1 0 4H3V21h6a2 2 0 0 0 2-2v-3.5h3.5a2 2 0 1 0 4 0H21V9a2 2 0 0 0-2-2h-3.5V3a2 2 0 1 0-4 0z"/>`,
			"plugins.store":     `<path d="M6 8h12l1 13H5zM9 8V6a3 3 0 0 1 6 0v2"/>`,
			"plugins.installed": `<rect x="4" y="4" width="6" height="6"/><rect x="14" y="4" width="6" height="6"/><rect x="4" y="14" width="6" height="6"/><path d="M17 14v6M14 17h6"/>`,
			"plugins.runtime":   `<rect x="3" y="5" width="18" height="14" rx="2"/><path d="m7 10 2 2-2 2M12 15h4"/>`,
			"themes":            `<path d="M12 3a9 9 0 1 0 0 18h1.5a1.5 1.5 0 0 0 0-3H12a1.5 1.5 0 0 1 0-3h2a7 7 0 0 0 7-7c0-2.76-4.03-5-9-5z"/><path d="M7.5 10.5h.01M9.5 6.5h.01M14.5 6.5h.01M17 10h.01"/>`,
			"themes.store":      `<path d="M6 8h12l1 13H5zM9 8V6a3 3 0 0 1 6 0v2"/>`,
			"themes.installed":  `<path d="m4 20 5-5M14 4l6 6-9 9-6-6z"/>`,
			"themes.edit":       `<path d="M4 20h6M7 17v-7h10v7M9 10V6h6v4"/>`,
			"usage":             `<path d="M4 19V9M9 19V5M14 19v-7M19 19V3"/>`,
			"debug":             `<path d="M8 9h8M9 4h6l1 3H8zM6 12h12v5a6 6 0 0 1-12 0zM3 14h3M18 14h3M4 20l3-2M20 20l-3-2"/>`,
			"update":            `<path d="M20 11a8 8 0 1 0-2.34 5.66M20 4v7h-7"/>`,
			"privacy":           `<path d="M12 3 5 6v5c0 4.8 2.9 8.2 7 10 4.1-1.8 7-5.2 7-10V6z"/>`,
			"about":             `<circle cx="12" cy="12" r="9"/><path d="M12 11v6M12 7h.01"/>`,
		}
	})
	path := settingNavIconPaths[id]
	if path == "" {
		return woxImage{}
	}
	return woxImage{ImageType: "svg", ImageData: start + path + end}
}

func (a *App) settingNavLabel(spec settingNavSpec) string {
	translated := a.translate("i18n:" + spec.labelKey)
	if translated == "" || translated == strings.ReplaceAll(spec.labelKey, "_", " ") {
		return spec.fallback
	}
	return translated
}

func (a *App) activeSettingsNavLabel(snapshot settingsSnapshot) string {
	activeID := activeSettingNavID(snapshot.tab, snapshot.pluginsStore, snapshot.themesMode)
	for _, spec := range settingNavSpecs(snapshot.isDev) {
		if spec.id == activeID {
			return a.settingNavLabel(spec)
		}
	}
	return a.translate("i18n:ui_tray_open_setting_window")
}

// buildSettingsSearchBox owns the settings window's default text-input focus and native IME cursor.
func (a *App) buildSettingsSearchBox(snapshot settingsSnapshot, width float32) woxwidget.Widget {
	window := a.settingsNativeWindow()
	style := woxui.TextStyle{Size: 13}
	clearWidth := float32(0)
	if snapshot.searchQuery.Text != "" {
		clearWidth = 34
	}
	editorWidth := max(float32(40), width-38-clearWidth)
	editor := woxwidget.Gesture{ID: "settings-search-field", OnTapAt: func(position woxui.Point) {
		offset := formTextOffsetAt(snapshot.searchQuery, window, style, 1, editorWidth-8, woxui.Point{X: max(float32(0), position.X-2), Y: position.Y})
		a.setSettingsSearchCaret(offset)
	}, Child: woxwidget.Painter{Width: editorWidth, Height: 38, Paint: func(displayList *woxui.DisplayList, bounds woxui.Rect) {
		if snapshot.searchQuery.Text == "" {
			displayList.DrawText(a.translate("i18n:ui_setting_search_placeholder"), bounds, style, snapshot.palette.resultSubtitle)
		}
		drawFormEditor(displayList, bounds, snapshot.searchQuery, style, snapshot.palette, snapshot.searchFocused, 1, window)
	}}}
	children := []woxwidget.Widget{
		woxwidget.Gesture{ID: "settings-search-icon", OnTap: func() { a.focusSettingsSearch(false) }, Child: woxwidget.Container{Width: 38, Height: 42, Padding: woxwidget.Insets{Left: 12, Top: 11}, Child: woxwidget.Text{
			Value: "⌕", Style: woxui.TextStyle{Size: 18, Weight: woxui.FontWeightSemibold}, Color: snapshot.palette.resultSubtitle,
		}}},
		editor,
	}
	if clearWidth > 0 {
		children = append(children, woxwidget.Gesture{ID: "settings-search-clear", OnTap: a.clearSettingsSearch, Child: woxwidget.Container{Width: clearWidth, Height: 42, Padding: woxwidget.Insets{Left: 10, Top: 10}, Child: woxwidget.Text{
			Value: "×", Style: woxui.TextStyle{Size: 17, Weight: woxui.FontWeightSemibold}, Color: snapshot.palette.resultSubtitle,
		}}})
	}
	fieldColor := snapshot.palette.queryBackground
	if snapshot.searchFocused {
		fieldColor = snapshot.palette.actionQueryBackground
	}
	return woxwidget.Container{Width: width, Height: 50, Child: woxwidget.Container{Width: width, Height: 42, Radius: 6, Color: fieldColor,
		Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Children: children},
	}}
}

// buildSettingsSearchResultPanel overlays navigation without shifting the rail while the query changes.
func (a *App) buildSettingsSearchResultPanel(snapshot settingsSnapshot, width, availableHeight float32) woxwidget.Widget {
	results := a.settingsSearchResults(snapshot)
	selected := 0
	if len(results) > 0 {
		selected = min(max(0, snapshot.searchSelected), len(results)-1)
	}
	panelHeight := min(float32(280), availableHeight)
	if len(results) > 0 {
		panelHeight = min(panelHeight, float32(len(results))*settingsSearchResultRowHeight+12)
	} else {
		panelHeight = min(panelHeight, float32(58))
	}
	viewportHeight := max(float32(1), panelHeight-12)
	a.setSettingsSearchViewport(viewportHeight, len(results))
	background := snapshot.palette.toolbarBackground
	background.A = 255
	if len(results) == 0 {
		message := a.translate("i18n:ui_setting_search_empty")
		if snapshot.searchLoading {
			message = a.translate("i18n:ui_cloud_sync_plugin_exclusions_loading")
		} else if snapshot.searchError != "" {
			message = snapshot.searchError
		}
		return woxwidget.Container{Width: width, Height: panelHeight, Radius: 7, Color: background,
			Padding: woxwidget.Insets{Left: 12, Top: 18, Right: 12}, Child: woxwidget.Text{Value: message, Style: woxui.TextStyle{Size: 12}, Color: snapshot.palette.resultSubtitle},
		}
	}
	rows := make([]woxwidget.Widget, 0, len(results))
	for index, result := range results {
		index := index
		result := result
		rowBackground := background
		titleColor := snapshot.palette.resultTitle
		if index == selected {
			rowBackground = snapshot.palette.selectedBackground
			titleColor = snapshot.palette.selectedTitle
		}
		rows = append(rows, woxwidget.Gesture{ID: fmt.Sprintf("settings-search-result-%d", index), OnHover: func(inside bool) {
			if inside {
				a.selectSettingsSearchResult(index)
			}
		}, OnTap: func() { a.activateSettingsSearchResult(result) }, Child: woxwidget.Container{Width: width - 12, Height: settingsSearchResultRowHeight, Radius: 5, Color: rowBackground, Padding: woxwidget.Insets{Left: 10, Top: 8, Right: 10}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 3, Children: []woxwidget.Widget{
			woxwidget.Text{Value: result.title, Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, Color: titleColor},
			woxwidget.Text{Value: a.settingsSearchResultTypeLabel(result.kind) + " · " + result.subtitle, Style: woxui.TextStyle{Size: 10}, Color: snapshot.palette.resultSubtitle},
		}}}})
	}
	return woxwidget.Container{Width: width, Height: panelHeight, Radius: 7, Color: background, Padding: woxwidget.UniformInsets(6), Child: woxwidget.Gesture{
		ID: "settings-search-results", OnScroll: func(delta woxui.Point) { a.scrollSettingsSearch(-delta.Y, len(results)) }, Child: woxwidget.ScrollView{
			Width: width - 12, Height: viewportHeight, ContentHeight: max(viewportHeight, float32(len(results))*settingsSearchResultRowHeight), Offset: snapshot.searchScroll,
			Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows},
		},
	}}
}

func (a *App) settingsSearchResultTypeLabel(kind settingsSearchResultKind) string {
	switch kind {
	case settingsSearchPlugin:
		return a.translate("i18n:ui_setting_search_type_plugin")
	case settingsSearchPluginSetting:
		return a.translate("i18n:ui_setting_search_type_plugin_setting")
	case settingsSearchSection:
		return a.translate("i18n:ui_setting_search_type_setting")
	default:
		return a.translate("i18n:ui_setting_search_type_setting")
	}
}

func (a *App) buildSettingsPage(snapshot settingsSnapshot, items []settingItem, width, height float32) woxwidget.Widget {
	contentWidth := max(float32(0), width-82)
	children := make([]woxwidget.Widget, 0, len(items)+8)
	contentHeight := float32(0)
	currentSection := ""
	for index, item := range items {
		index := index
		item = a.localizedSettingItem(item)
		section := a.settingsSectionLabel(snapshot.tab, item.key)
		if section != currentSection {
			currentSection = section
			children = append(children, a.buildSettingsSectionHeader(section, contentWidth, snapshot.palette))
			contentHeight += 43
		}
		selected := index == snapshot.row
		background := woxui.Color{}
		if selected {
			background = snapshot.palette.selectedBackground
		}
		children = append(children, woxwidget.Gesture{
			ID: "setting-" + item.key,
			OnTap: func() {
				a.selectSettingRow(index)
				a.openOrActivateSetting()
			},
			OnScroll: func(delta woxui.Point) {
				a.scrollSettingsPage(-delta.Y)
			},
			Child: a.buildSettingRow(snapshot, item, contentWidth, background),
		})
		contentHeight += 74
	}
	if snapshot.tab == "general" && snapshot.hotkeyForm != nil {
		children = append(children, a.buildSettingsSectionHeader(a.translate("i18n:ui_general_section_hotkeys"), contentWidth, snapshot.palette))
		contentHeight += 43
		hotkeyForm := *snapshot.hotkeyForm
		hotkeyForm.active = snapshot.hotkeyFocused
		callbacks := formFieldCallbacks{
			idPrefix: "hotkey-settings", focus: a.focusHotkeySettingsField, openTable: a.openHotkeySettingsTable, recordKey: a.recordHotkeySettingsField,
		}
		for index, definition := range hotkeyForm.definitions {
			rowHeight := formDefinitionHeight(definition)
			children = append(children, a.buildFormField(hotkeyForm, callbacks, snapshot.palette, index, definition, contentWidth, rowHeight))
			contentHeight += rowHeight
		}
	}
	note := snapshot.note
	if note != "" {
		children = append(children, woxwidget.Container{Width: contentWidth, Height: 34, Padding: woxwidget.Insets{Top: 10}, Child: woxwidget.Text{Value: note, Style: woxui.TextStyle{Size: 11}, Color: snapshot.palette.resultSubtitle}})
		contentHeight += 34
	}
	viewportHeight := max(float32(1), height-58)
	a.setSettingsPageGeometry(viewportHeight, contentHeight, len(items))
	content := woxwidget.Flex{Axis: woxwidget.Vertical, Children: children}
	return woxwidget.Container{
		Width: width, Height: height,
		Padding: woxwidget.Insets{Left: 38, Top: 34, Right: 44, Bottom: 24},
		Child: woxwidget.Gesture{ID: "settings-page-scroll", OnScroll: func(delta woxui.Point) { a.scrollSettingsPage(-delta.Y) }, Child: woxwidget.ScrollView{
			Width: contentWidth, Height: viewportHeight, ContentHeight: max(viewportHeight, contentHeight), Offset: snapshot.pageScroll, Child: content,
		}},
	}
}

func (a *App) buildSettingsSectionHeader(label string, width float32, palette uiPalette) woxwidget.Widget {
	return woxwidget.Container{Width: width, Height: 43, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{
		woxwidget.Container{Width: width, Height: 1, Color: palette.previewSplit},
		woxwidget.Container{Width: width, Height: 42, Padding: woxwidget.Insets{Top: 14}, Child: woxwidget.Text{Value: strings.ToUpper(label), Style: woxui.TextStyle{Size: 11, Weight: woxui.FontWeightSemibold}, Color: palette.resultSubtitle}},
	}}}
}

func (a *App) settingsSectionLabel(tab, key string) string {
	if tab == "general" {
		switch key {
		case "EnableAutostart", "HideOnStart":
			return a.translate("i18n:ui_general_section_startup")
		case "LangCode":
			return a.translate("i18n:ui_general_section_language")
		default:
			return a.translate("i18n:ui_general_section_launch")
		}
	}
	if tab == "appearance" {
		switch key {
		case "MaxResultCount":
			return a.translate("i18n:ui_ui_section_results")
		case "EnableGlance", "HideGlanceIcon", "PrimaryGlance":
			return a.translate("i18n:ui_ui_section_glance")
		default:
			return a.translate("i18n:ui_ui_section_launcher")
		}
	}
	return a.activeSettingsNavLabel(a.settingsSnapshot())
}

func (a *App) localizedSettingItem(item settingItem) settingItem {
	keys := map[string][2]string{
		"EnableAutostart": {"ui_autostart", "ui_autostart_tips"}, "HideOnStart": {"ui_hide_on_start", "ui_hide_on_start_tips"},
		"LaunchMode": {"ui_launch_mode", "ui_launch_mode_tips"}, "StartPage": {"ui_start_page", "ui_start_page_tips"},
		"HideOnLostFocus": {"ui_hide_on_lost_focus", "ui_hide_on_lost_focus_tips"}, "UsePinYin": {"ui_use_pinyin", "ui_use_pinyin_tips"},
		"SwitchInputMethodABC": {"ui_switch_input_method_abc", "ui_switch_input_method_abc_tips"}, "LangCode": {"ui_lang", ""},
		"ShowPosition": {"ui_show_position", "ui_show_position_tips"}, "ShowTray": {"ui_show_tray", "ui_show_tray_tips"},
		"AppWidth": {"ui_app_width", "ui_app_width_tips"}, "UiDensity": {"ui_interface_size", "ui_interface_size_tips"},
		"AppFontFamily": {"ui_app_font_family", "ui_app_font_family_tips"}, "EnableQueryCompletionHint": {"ui_query_completion_hint", "ui_query_completion_hint_tips"},
		"MaxResultCount": {"ui_max_result_count", "ui_max_result_count_tips"}, "EnableGlance": {"ui_glance_enable", "ui_glance_enable_tips"},
		"HideGlanceIcon": {"ui_glance_hide_icon", "ui_glance_hide_icon_tips"}, "PrimaryGlance": {"ui_glance_primary", "ui_glance_primary_tips"},
		"HttpProxyEnabled": {"ui_proxy_enabled", ""}, "HttpProxyUrl": {"ui_proxy_url", ""},
	}
	if pair, ok := keys[item.key]; ok {
		item.title = a.translate("i18n:" + pair[0])
		if pair[1] != "" {
			item.description = a.translate("i18n:" + pair[1])
		}
	}
	for index := range item.choices {
		item.choices[index].label = a.localizedSettingChoiceLabel(item.key, item.choices[index])
	}
	return item
}

func (a *App) localizedSettingChoiceLabel(key string, choice settingChoice) string {
	choiceKeys := map[string]map[string]string{
		"LaunchMode":   {"fresh": "ui_launch_mode_fresh", "continue": "ui_launch_mode_continue"},
		"StartPage":    {"blank": "ui_start_page_blank", "mru": "ui_start_page_mru"},
		"ShowPosition": {"mouse_screen": "ui_show_position_mouse_screen", "active_screen": "ui_show_position_active_screen", "last_location": "ui_show_position_last_location"},
		"UiDensity":    {"compact": "ui_interface_size_compact", "normal": "ui_interface_size_normal", "comfortable": "ui_interface_size_comfortable"},
	}
	if valueKeys := choiceKeys[key]; valueKeys != nil {
		if labelKey := valueKeys[choice.value]; labelKey != "" {
			return a.translate("i18n:" + labelKey)
		}
	}
	return choice.label
}

func (a *App) buildSettingRow(snapshot settingsSnapshot, item settingItem, width float32, background woxui.Color) woxwidget.Widget {
	window := a.settingsNativeWindow()
	palette := snapshot.palette
	foreground := palette.resultTitle
	subtitle := palette.resultSubtitle
	valueColor := palette.cursor
	if item.disabled {
		foreground = palette.resultSubtitle
		valueColor = palette.resultSubtitle
	}
	valueWidth := min(float32(280), max(float32(190), width*0.32))
	if item.text {
		valueWidth = min(float32(440), max(float32(280), width*0.46))
	}
	if isBooleanSettingItem(item) {
		valueWidth = 42
	}
	labelWidth := max(float32(180), width-valueWidth-32)
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
			offset := formTextOffsetAt(state, window, style, 1, inputWidth-24, woxui.Point{X: max(float32(0), position.X-12), Y: max(float32(0), position.Y-8)})
			a.startBuiltInSettingEdit(item, offset)
		}, Child: woxwidget.Container{Width: inputWidth, Height: 38, Radius: 8, Color: palette.toolbarBackground, Padding: woxwidget.Insets{Left: 12, Top: 8, Right: 12, Bottom: 6}, Child: woxwidget.Clip{
			Width: inputWidth - 24, Height: 24, Child: woxwidget.Painter{Width: inputWidth - 24, Height: 24, Paint: func(displayList *woxui.DisplayList, bounds woxui.Rect) {
				drawFormEditor(displayList, bounds, state, style, palette, focused, 1, window)
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
	} else if isBooleanSettingItem(item) {
		enabled := item.value == "true"
		trackColor := palette.previewSplit
		knobLeft := float32(2)
		if enabled {
			trackColor = palette.cursor
			knobLeft = 22
		}
		valueField = woxwidget.Container{Width: valueWidth, Height: 52, Padding: woxwidget.Insets{Top: 15}, Child: woxwidget.Stack{Width: 42, Height: 22, Children: []woxwidget.StackChild{
			{Child: woxwidget.Container{Width: 42, Height: 22, Radius: 11, Color: trackColor}},
			{Left: knobLeft, Top: 2, Child: woxwidget.Container{Width: 18, Height: 18, Radius: 9, Color: woxui.Color{R: 248, G: 248, B: 248, A: 255}}},
		}}}
	} else {
		valueField = woxwidget.Container{
			Width: valueWidth, Height: 38, Radius: 6, Color: palette.toolbarBackground,
			Padding: woxwidget.Insets{Left: 14, Top: 10, Right: 12},
			Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{
				woxwidget.Container{Width: max(float32(0), valueWidth-42), Height: 24, Child: woxwidget.Text{Value: settingValueLabel(item), Style: woxui.TextStyle{Size: 13}, Color: valueColor}},
				woxwidget.Container{Width: 16, Height: 24, Child: woxwidget.Text{Value: "▾", Style: woxui.TextStyle{Size: 11}, Color: subtitle}},
			}},
		}
	}
	return woxwidget.Container{
		Width: width, Height: 74, Radius: 6, Color: background,
		Padding: woxwidget.Insets{Left: 2, Top: 11, Right: 2, Bottom: 11},
		Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 28, Children: []woxwidget.Widget{
			woxwidget.Container{Width: labelWidth, Height: 52, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 5, Children: []woxwidget.Widget{
				woxwidget.Text{Value: item.title, Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: foreground},
				woxwidget.Text{Value: item.description, Style: woxui.TextStyle{Size: 11}, Color: subtitle},
			}}},
			valueField,
		}},
	}
}
