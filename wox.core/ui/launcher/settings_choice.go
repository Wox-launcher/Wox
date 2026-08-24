package launcher

import (
	"context"
	"strings"

	launcherview "wox/ui/launcher/view"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util"
)

type settingChoicePickerState struct {
	item     settingItem
	anchor   woxui.Rect
	onChoose func(settingChoice)
}

type settingChoicePickerSnapshot struct {
	item   settingItem
	anchor woxui.Rect
}

// buildSettingChoicePickerOverlay adapts controller state to the package-independent choice picker view.
func (a *App) buildSettingChoicePickerOverlay(snapshot *settingChoicePickerSnapshot, palette uiPalette, width, height, imageScale float32) woxwidget.Widget {
	choices := make([]launcherview.SettingsChoice, len(snapshot.item.choices))
	for index, choice := range snapshot.item.choices {
		var leading *woxui.Image
		if source := snapshot.item.icons[choice.value]; source.ImageData != "" {
			if snapshot.item.preserveIconColor {
				leading = a.imageForSize(source, physicalImageSize(18, imageScale))
			} else {
				leading = a.imageForTint(source, &palette.resultTitle, physicalImageSize(18, imageScale))
			}
		}
		choices[index] = launcherview.SettingsChoice{
			Value: choice.value, Label: choice.label, Leading: leading, Trailing: snapshot.item.trailers[choice.value],
			Tooltip: a.localizedSettingChoiceTooltip(snapshot.item.key, choice),
		}
	}
	searchIcon := a.imageForTint(settingControlIconSource("search"), &palette.resultSubtitle, physicalImageSize(16, imageScale))
	return launcherview.SettingsChoiceView(launcherview.SettingsChoiceProps{
		ID: "setting-choice-picker", Width: width, Height: height, Anchor: snapshot.anchor, Filterable: snapshot.item.filterable, Theme: palette.componentTheme(), Window: a.settingsNativeWindow(), Title: snapshot.item.title,
		FilterHint: a.translate("i18n:ui_filter_placeholder"), SearchIcon: searchIcon, CurrentValue: snapshot.item.value, Choices: choices, OnChoose: a.chooseSettingChoice, OnCancel: a.closeSettingChoicePicker, OnTooltip: a.setSettingChoiceTooltip,
	})
}

func snapshotSettingChoicePickerLocked(state *settingChoicePickerState) *settingChoicePickerSnapshot {
	if state == nil {
		return nil
	}
	item := state.item
	item.choices = append([]settingChoice(nil), state.item.choices...)
	return &settingChoicePickerSnapshot{item: item, anchor: state.anchor}
}

func (a *App) openOrActivateSetting() {
	snapshot := a.settingsSnapshot()
	items := settingItemsForSnapshot(snapshot)
	if snapshot.saving || snapshot.row < 0 || snapshot.row >= len(items) {
		return
	}
	item := a.localizedSettingItem(items[snapshot.row])
	if item.disabled {
		return
	}
	if item.text || isBooleanSettingItem(item) {
		a.activateSetting(1)
		return
	}
	a.openSettingChoicePicker(item)
}

func isBooleanSettingItem(item settingItem) bool {
	return len(item.choices) == 2 && item.choices[0].value == "false" && item.choices[1].value == "true"
}

func (a *App) openSettingChoicePicker(item settingItem) {
	host := a.settingsHost
	anchor := woxui.Rect{}
	if host != nil {
		anchor, _ = host.BoundsForKey(launcherview.SettingChoiceAnchorKey(item.key))
	}
	a.openSettingChoicePickerAt(item, anchor)
}

// openSettingChoicePickerAt anchors pointer-opened menus to the bounds from the exact hit-tested frame.
func (a *App) openSettingChoicePickerAt(item settingItem, anchor woxui.Rect) {
	if a.settingSaving || item.disabled || len(item.choices) == 0 {
		return
	}
	a.generalSettings.EndEdit()
	a.generalSettings.SetChoicePicker(&settingChoicePickerState{item: item, anchor: anchor})
	a.updateSettingsTextInput(false)
	a.invalidateSettingsWindow()
}

func (a *App) closeSettingChoicePicker() {
	closed := false
	if a.generalSettings.ChoicePicker() != nil {
		a.generalSettings.SetChoicePicker(nil)
		closed = true
	}
	if closed {
		a.setSettingChoiceTooltip(false, "", woxui.Rect{})
	}
	a.updateSettingsTextInput(false)
	a.invalidateSettingsWindow()
}

func (a *App) chooseSettingChoice(index int) {
	state := a.generalSettings.ChoicePicker()
	if state == nil || a.settingSaving {
		return
	}
	if index < 0 || index >= len(state.item.choices) {
		return
	}
	item := state.item
	choice := state.item.choices[index]
	a.generalSettings.SetChoicePicker(nil)
	if state.onChoose != nil {
		a.setSettingChoiceTooltip(false, "", woxui.Rect{})
		a.updateSettingsTextInput(false)
		state.onChoose(choice)
		a.invalidateSettingsWindow()
		return
	}
	a.beginSettingSave()
	a.setSettingChoiceTooltip(false, "", woxui.Rect{})
	a.updateSettingsTextInput(false)
	a.invalidateSettingsWindow()
	util.Go(a.lifecycleCtx, "save setting choice", func() {
		a.saveSetting(item, choice)
	})
}

func (a *App) setSettingChoiceTooltip(inside bool, text string, anchor woxui.Rect) {
	a.setSettingsHoverTooltip(inside, text, anchor, "top")
}

// setSettingsHoverTooltip shows one settings-window tooltip on the requested side.
func (a *App) setSettingsHoverTooltip(inside bool, text string, anchor woxui.Rect, side string) {
	if util.IsLinux() {
		a.scheduleLinuxInlineTooltip(inside, text, anchor, side)
		return
	}

	a.setNativeHoverTooltip(&a.choiceTooltipRevision, "go-ui-setting-choice", "update setting choice tooltip", inside, text, anchor, side, a.settingsNativeWindow)
}

func (a *App) scheduleLinuxInlineTooltip(inside bool, text string, anchor woxui.Rect, side string) {
	if !a.settingsOpen {
		a.settingsInlineTooltipRevision.Add(1)
		a.clearLinuxInlineTooltip()
		return
	}
	message := strings.TrimSpace(text)
	if !nativeHoverTooltipArmed(inside, message, anchor) {
		a.settingsInlineTooltipRevision.Add(1)
		a.clearLinuxInlineTooltip()
		return
	}
	next := settingsInlineTooltipState{Text: message, Side: side, Anchor: anchor}
	if current := a.settingsInlineTooltip; current != nil && current.Text == next.Text && current.Side == next.Side && current.Anchor == next.Anchor {
		return
	}
	revisionID := a.settingsInlineTooltipRevision.Add(1)
	util.Go(a.lifecycleCtx, "show settings inline tooltip after dwell", func() {
		if !a.waitHoverTooltipDelay(&a.settingsInlineTooltipRevision, revisionID) {
			return
		}
		apply := func() {
			if revisionID != a.settingsInlineTooltipRevision.Load() {
				return
			}
			if current := a.settingsInlineTooltip; current != nil && current.Text == next.Text && current.Side == next.Side && current.Anchor == next.Anchor {
				return
			}
			a.settingsInlineTooltip = &next
			a.invalidateSettingsWindow()
		}
		if err := a.runOnUI("show settings inline tooltip", apply); err != nil {
			apply()
		}
	})
}

func (a *App) clearLinuxInlineTooltip() {
	if a.settingsInlineTooltip == nil {
		return
	}
	a.settingsInlineTooltip = nil
	a.invalidateSettingsWindow()
}

// loadSystemFontFamilies keeps enumeration in core while the framework only consumes portable family names.
func (a *App) loadSystemFontFamilies() {
	a.appearanceSettings.ReloadFonts(context.Background(), a.services, a.sessionID)
}

func systemFontSettingItem(snapshot settingsSnapshot) settingItem {
	appearance := snapshot.appearance
	choices := make([]settingChoice, 0, len(appearance.FontFamilies)+2)
	choices = append(choices, settingChoice{value: "", label: "System default"})
	found := snapshot.general.Data.AppFontFamily == ""
	for _, family := range appearance.FontFamilies {
		choices = append(choices, settingChoice{value: family, label: family})
		if family == snapshot.general.Data.AppFontFamily {
			found = true
		}
	}
	if !found {
		choices = append([]settingChoice{{value: snapshot.general.Data.AppFontFamily, label: snapshot.general.Data.AppFontFamily}}, choices...)
	}
	description := "Font family used by Query and Settings windows"
	if appearance.FontsError != "" {
		description = "Could not load installed fonts: " + appearance.FontsError
	}
	return settingItem{key: "AppFontFamily", title: "Application font", description: description, value: snapshot.general.Data.AppFontFamily, choices: choices, filterable: true}
}
