package launcher

import (
	"log"
	"runtime"
	"strings"

	"wox/common"
	woxcomponent "wox/ui/launcher/component"
	launcherview "wox/ui/launcher/view"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const settingsTitleBarHeight = launcherview.SettingsTitleBarHeight

func (a *App) buildSettings(frame woxui.FrameInfo) woxwidget.Widget {
	snapshot := a.settingsSnapshot()
	items := settingItemsForSnapshot(snapshot)
	if snapshot.row >= len(items) && len(items) > 0 {
		snapshot.row = len(items) - 1
	}
	width := frame.Size.Width
	height := frame.Size.Height
	contentHeight := max(float32(0), height-settingsTitleBarHeight)
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
	var overlay woxwidget.Widget
	if snapshot.tableEditor != nil {
		overlay = a.buildFormTableOverlay(snapshot.tableEditor, snapshot.palette, width, height)
	} else if snapshot.modelManager != nil {
		overlay = a.buildModelManagerOverlay(snapshot.modelManager, snapshot.palette, width, height)
	} else if snapshot.choicePicker != nil {
		overlay = a.buildSettingChoicePickerOverlay(snapshot.choicePicker, snapshot.palette, width, height)
	} else if snapshot.cloudForm != nil {
		overlay = a.buildCloudFormOverlay(snapshot.cloudForm, snapshot.palette, width, height)
	} else if snapshot.privacySample != "" {
		overlay = a.buildPrivacySampleOverlay(snapshot, width, height)
	}
	return launcherview.SettingsWindow(launcherview.SettingsWindowProps{
		Width: width, Height: height, Theme: snapshot.palette.componentTheme(),
		TitleBar: a.buildSettingsTitleBar(snapshot, width, railWidth), Rail: a.buildSettingsRail(snapshot, railWidth, contentHeight), Page: page, Overlay: overlay,
	})
}

func (a *App) buildSettingsTitleBar(snapshot settingsSnapshot, width, railWidth float32) woxwidget.Widget {
	title := a.activeSettingsNavLabel(snapshot)
	titleStyle := woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}
	titleWidth := float32(160)
	if window := a.settingsNativeWindow(); window != nil {
		if metrics, err := window.MeasureText(title, titleStyle); err == nil {
			titleWidth = metrics.Size.Width + 24
		}
	}
	return launcherview.SettingsTitleBar(launcherview.SettingsTitleBarProps{
		Width: width, RailWidth: railWidth, Title: title, TitleWidth: titleWidth, Platform: runtime.GOOS, AppIcon: a.imageFor(fromCoreImage(common.WoxIcon)),
		Theme: snapshot.palette.componentTheme(),
		OnDrag: func() {
			if window := a.settingsNativeWindow(); window != nil {
				_ = window.StartDragging()
			}
		},
		OnMinimize: func() {
			if window := a.settingsNativeWindow(); window != nil {
				_ = window.Minimize()
			}
		},
		OnClose: func() {
			go func() {
				if err := a.closeSettings(); err != nil {
					log.Printf("close settings window: %v", err)
				}
			}()
		},
	})
}

// buildSettingsThemePage mounts theme catalogs and the shared live editor under one portable route.
func (a *App) buildSettingsThemePage(snapshot settingsSnapshot, width, height float32) woxwidget.Widget {
	innerWidth := max(float32(0), width-40)
	bodyHeight := max(float32(0), height-40)
	var body woxwidget.Widget
	if snapshot.themesMode != "editor" {
		body = a.buildThemeCatalog(snapshot, innerWidth, bodyHeight)
	} else {
		a.mu.RLock()
		theme := snapshotThemeEditorPreviewLocked(a.themeEditor)
		a.mu.RUnlock()
		if theme == nil {
			message := "Loading active theme…"
			if snapshot.themesError != "" {
				message = snapshot.themesError
			}
			body = launcherview.SettingsMessage(message, innerWidth, bodyHeight, snapshot.palette.componentTheme())
		} else {
			body = a.buildThemeEditorSettingsSurface(theme, snapshot.palette, innerWidth, bodyHeight)
		}
	}
	return launcherview.SettingsThemePage(launcherview.SettingsThemePageProps{
		Width: width, Height: height, Body: body,
	})
}

func (a *App) buildSettingsRail(snapshot settingsSnapshot, width, height float32) woxwidget.Widget {
	specs := settingNavSpecs(snapshot.isDev)
	activeID := activeSettingNavID(snapshot.tab, snapshot.pluginsStore, snapshot.themesMode)
	items := make([]launcherview.SettingsNavItem, 0, len(specs))
	var keepVisible *woxwidget.ScrollRange
	for index, spec := range specs {
		spec := spec
		foreground := snapshot.palette.toolbarText
		if spec.id == activeID {
			foreground = snapshot.palette.selectedTitle
		}
		var icon *woxui.Image
		if source := settingNavIconSource(spec.id); source.ImageData != "" {
			icon = a.imageForTint(source, &foreground, 24)
		}
		items = append(items, launcherview.SettingsNavItem{
			ID: spec.id, Label: a.settingNavLabel(spec), FallbackIcon: spec.icon, Icon: icon, Depth: spec.depth, Parent: spec.parent, Selected: spec.id == activeID,
			OnTap: func() { a.selectSettingsNavItem(spec) },
		})
		if spec.id == activeID {
			keepVisible = &woxwidget.ScrollRange{Start: float32(index * 50), End: float32(index*50 + 46)}
		}
	}
	innerWidth := width - 28
	searchAreaHeight := float32(58)
	viewportHeight := max(float32(1), height-searchAreaHeight-28)
	return launcherview.SettingsRail(launcherview.SettingsRailProps{
		Width: width, Height: height, Items: items, KeepVisible: keepVisible,
		SearchBox: a.buildSettingsSearchBox(snapshot, innerWidth), SearchPanel: a.buildSettingsSearchResultPanel(snapshot, innerWidth, viewportHeight),
		ShowSearch: snapshot.searchPanel && strings.TrimSpace(snapshot.searchQuery.Text) != "", Theme: snapshot.palette.componentTheme(),
	})
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
	placeholder := a.translate("i18n:ui_setting_search_placeholder")
	iconTint := snapshot.palette.resultSubtitle
	return launcherview.SettingsSearchBox(launcherview.SettingsSearchBoxProps{
		Width: width, Placeholder: placeholder, State: snapshot.searchQuery, Focused: snapshot.searchFocused,
		SearchIcon: a.imageForTint(settingControlIconSource("search"), &iconTint, 18), Window: a.settingsNativeWindow(), Theme: snapshot.palette.componentTheme(),
		OnFocus: func() { a.focusSettingsSearch(false) }, OnClear: a.clearSettingsSearch,
		OnKey: a.onSettingsSearchKey, OnFocusChange: a.setSettingsSearchFocused, OnChanged: func(value string) { _ = a.setSettingsSearchValue(value) }, OnSetValue: a.setSettingsSearchValue,
	})
}

// buildSettingsSearchResultPanel overlays navigation without shifting the rail while the query changes.
func (a *App) buildSettingsSearchResultPanel(snapshot settingsSnapshot, width, availableHeight float32) woxwidget.Widget {
	results := a.settingsSearchResults(snapshot)
	items := make([]launcherview.SettingsSearchResult, 0, len(results))
	for index, result := range results {
		index := index
		result := result
		items = append(items, launcherview.SettingsSearchResult{
			Title: result.title, Subtitle: a.settingsSearchResultTypeLabel(result.kind) + " · " + result.subtitle,
			OnHover: func() { a.selectSettingsSearchResult(index) }, OnTap: func() { a.activateSettingsSearchResult(result) },
		})
	}
	emptyMessage := a.translate("i18n:ui_setting_search_empty")
	if len(results) == 0 {
		if snapshot.searchLoading {
			emptyMessage = a.translate("i18n:ui_cloud_sync_plugin_exclusions_loading")
		} else if snapshot.searchError != "" {
			emptyMessage = snapshot.searchError
		}
	}
	return launcherview.SettingsSearchResults(launcherview.SettingsSearchResultsProps{
		Width: width, AvailableHeight: availableHeight, Results: items, Selected: snapshot.searchSelected,
		EmptyMessage: emptyMessage, Theme: snapshot.palette.componentTheme(),
	})
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
	children := make([]woxwidget.Widget, 0, len(items)+9)
	children = append(children, a.buildSettingsPageHeader(
		a.activeSettingsNavLabel(snapshot),
		a.settingsPageDescription(snapshot.tab),
		contentWidth,
		snapshot.palette,
	))
	contentHeight := woxcomponent.PageHeaderHeight
	var keepVisible *woxwidget.ScrollRange
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
		if index == snapshot.row {
			keepVisible = &woxwidget.ScrollRange{Start: contentHeight, End: contentHeight + 62}
		}
		children = append(children, a.buildSettingRow(snapshot, item, index, contentWidth, woxui.Color{}))
		contentHeight += 62
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
			rowHeight := formDefinitionHeight(definition, hotkeyForm.values)
			if hotkeyForm.active && index == hotkeyForm.focused {
				keepVisible = &woxwidget.ScrollRange{Start: contentHeight, End: contentHeight + rowHeight}
			}
			children = append(children, a.buildFormField(hotkeyForm, callbacks, snapshot.palette, index, definition, contentWidth, rowHeight))
			contentHeight += rowHeight
		}
	}
	note := snapshot.note
	if note != "" {
		children = append(children, launcherview.SettingsNote(note, contentWidth, snapshot.palette.componentTheme()))
		contentHeight += 34
	}
	return launcherview.SettingsPage(launcherview.SettingsPageProps{
		ID: "settings-page-" + snapshot.tab, Width: width, Height: height, Children: children, ContentHeight: contentHeight, KeepVisible: keepVisible,
	})
}

// buildSettingsPageHeader keeps built-in pages aligned with Flutter's wide settings form.
func (a *App) buildSettingsPageHeader(title, description string, width float32, palette uiPalette) woxwidget.Widget {
	return woxcomponent.WoxPageHeader(woxcomponent.PageHeaderProps{
		Title: title, Description: description, Width: width, Theme: palette.componentTheme(),
	})
}

func (a *App) settingsPageDescription(tab string) string {
	switch tab {
	case "general":
		return a.translate("i18n:ui_general_description")
	case "appearance":
		return a.translate("i18n:ui_ui_description")
	case "network":
		return a.translate("i18n:ui_network_description")
	case "debug":
		return a.translate("i18n:ui_debug_description")
	case "updates":
		return a.translate("i18n:ui_update_description")
	default:
		return ""
	}
}

func (a *App) buildSettingsSectionHeader(label string, width float32, palette uiPalette) woxwidget.Widget {
	return woxcomponent.WoxSectionHeader(woxcomponent.SectionHeaderProps{Label: label, Width: width, Theme: palette.componentTheme()})
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
	if tab == "updates" {
		return a.translate("i18n:ui_update_section_updates")
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
		"HttpProxyEnabled": {"ui_proxy_enabled", ""}, "HttpProxyUrl": {"ui_proxy_url", "ui_proxy_url_tips"},
		"CustomPythonPath": {"ui_runtime_python_path", "ui_runtime_python_path_tips"}, "CustomNodejsPath": {"ui_runtime_nodejs_path", "ui_runtime_nodejs_path_tips"},
		"EnableAutoUpdate": {"ui_enable_auto_update", "ui_enable_auto_update_tips"}, "ReleaseChannel": {"ui_release_channel", "ui_release_channel_tips"},
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
		"LaunchMode":     {"fresh": "ui_launch_mode_fresh", "continue": "ui_launch_mode_continue"},
		"StartPage":      {"blank": "ui_start_page_blank", "mru": "ui_start_page_mru"},
		"ShowPosition":   {"mouse_screen": "ui_show_position_mouse_screen", "active_screen": "ui_show_position_active_screen", "last_location": "ui_show_position_last_location"},
		"UiDensity":      {"compact": "ui_interface_size_compact", "normal": "ui_interface_size_normal", "comfortable": "ui_interface_size_comfortable"},
		"ReleaseChannel": {"stable": "ui_release_channel_stable", "beta": "ui_release_channel_beta"},
	}
	if valueKeys := choiceKeys[key]; valueKeys != nil {
		if labelKey := valueKeys[choice.value]; labelKey != "" {
			return a.translate("i18n:" + labelKey)
		}
	}
	return choice.label
}

func (a *App) localizedSettingChoiceTooltip(key string, choice settingChoice) string {
	tooltipKeys := map[string]map[string]string{
		"LaunchMode":     {"fresh": "ui_launch_mode_fresh_tips", "continue": "ui_launch_mode_continue_tips"},
		"StartPage":      {"blank": "ui_start_page_blank_tips", "mru": "ui_start_page_mru_tips"},
		"ReleaseChannel": {"stable": "ui_release_channel_stable_tips", "beta": "ui_release_channel_beta_tips"},
	}
	if valueKeys := tooltipKeys[key]; valueKeys != nil {
		if tooltipKey := valueKeys[choice.value]; tooltipKey != "" {
			return a.translate("i18n:" + tooltipKey)
		}
	}
	return ""
}

func (a *App) buildSettingRow(snapshot settingsSnapshot, item settingItem, index int, width float32, background woxui.Color) woxwidget.Widget {
	kind := "choice"
	value := settingValueLabel(item)
	state := woxui.TextEditingState{Text: item.value}
	focused := snapshot.editKey == item.key
	if item.text {
		kind = "text"
		if focused {
			state = snapshot.editing
		}
		value = item.value
	} else if isBooleanSettingItem(item) {
		kind = "bool"
		value = item.value
	}
	return launcherview.SettingRow(launcherview.SettingRowProps{
		ID: item.key, Title: item.title, Description: item.description, Value: value, ValueTrailing: item.trailers[item.value], Width: width, Background: background, Disabled: item.disabled,
		Kind: kind, ControlWidth: item.controlWidth, BrowseFile: item.browseFile, Editing: state, Focused: focused, Window: a.settingsNativeWindow(), Theme: snapshot.palette.componentTheme(),
		OnTap:       func() { a.selectSettingRow(index); a.openOrActivateSetting() },
		OnChoiceTap: func(anchor woxui.Rect) { a.selectSettingRow(index); a.openSettingChoicePickerAt(item, anchor) },
		OnFocus:     func() { a.selectSettingRow(index); a.startBuiltInSettingEdit(item, -1) },
		OnChanged:   func(value string) { a.setBuiltInSettingEditValue(item, value) }, OnKey: a.onBuiltInSettingsEditorKey,
		OnBrowse: func() { a.selectSettingRow(index); a.browseBuiltInSettingFile(item) },
	})
}
